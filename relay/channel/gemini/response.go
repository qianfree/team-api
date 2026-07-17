package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// codeAssistWrapper Code Assist 响应包装层
type codeAssistWrapper struct {
	Response json.RawMessage `json:"response"`
}

// unwrapCodeAssistData 解包 Code Assist 的响应格式
// Code Assist 格式：{"response": {GeminiChatResponse}, "traceId": "...", ...}
// 标准格式：{GeminiChatResponse}
func unwrapCodeAssistData(data []byte) []byte {
	var wrapper codeAssistWrapper
	if err := json.Unmarshal(data, &wrapper); err == nil && wrapper.Response != nil {
		return wrapper.Response
	}
	return data
}

// handleGeminiNativeResponse 处理 Gemini 原生透传响应（Gemini 客户端 → Gemini 上游）
func (a *Adaptor) handleGeminiNativeResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if info.IsStream {
		return a.handleGeminiNativeStream(ctx, resp, info, writer)
	}
	return a.handleGeminiNativeNonStream(ctx, resp, info, writer)
}

// handleGeminiNativeNonStream Gemini 原生非流式透传
func (a *Adaptor) handleGeminiNativeNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Gemini 原生格式透传：已写入完整上游错误响应，标记以防上层二次写入
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		err := constant.NewUpstreamError(resp.StatusCode, string(body), nil)
		err.ResponseWritten = true
		return nil, err
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	var geminiResp dto.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err == nil && geminiResp.UsageMetadata != nil {
		return geminiUsageToCommon(geminiResp.UsageMetadata), nil
	}
	return &common.Usage{}, nil
}

// handleGeminiNativeStream Gemini 原生流式透传
func (a *Adaptor) handleGeminiNativeStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Gemini 原生格式透传：已写入完整上游错误响应，标记以防上层二次写入
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		err := constant.NewUpstreamError(resp.StatusCode, string(body), nil)
		err.ResponseWritten = true
		return nil, err
	}

	helper.SetEventStreamHeaders(writer)

	isCA := a.isCodeAssistActive()
	reader := bufio.NewReader(resp.Body)

	var totalUsage dto.GeminiUsageMetadata

	for {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return &common.Usage{}, common.ErrStreamInterrupted
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			outputLine := line

			// 解析 data: 行以收集 usage 并解包 Code Assist 响应
			trimmed := strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))

				if data != "" && data != "[DONE]" {
					info.SetFirstResponseTime()
				}

				if data == "[DONE]" {
					_, _ = io.WriteString(writer, outputLine)
					if f, ok := writer.(http.Flusher); ok {
						f.Flush()
					}
					break
				}

				if data != "" {
					rawData := []byte(data)
					// Code Assist 模式：解包 response 字段并替换 SSE data
					if isCA {
						unwrapped := unwrapCodeAssistData(rawData)
						outputLine = "data: " + string(unwrapped) + "\n"
						rawData = unwrapped
					}
					var geminiResp dto.GeminiChatResponse
					if jsonErr := json.Unmarshal(rawData, &geminiResp); jsonErr == nil {
						if geminiResp.UsageMetadata != nil {
							totalUsage = *geminiResp.UsageMetadata
						}
					}
				}
			}

			// 输出 SSE 行（解包后的）
			_, _ = io.WriteString(writer, outputLine)
			if f, ok := writer.(http.Flusher); ok {
				f.Flush()
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			break
		}
	}

	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)

	return geminiUsageToCommon(&totalUsage), nil
}

// handleNonStreamToOpenAI 将 Gemini 非流式响应转换为 OpenAI 格式
func (a *Adaptor) handleNonStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 不写响应：交上层 WriteRelayError 统一写入，避免双重写入与重试时的响应污染
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	var geminiResp dto.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}

	// 检查 promptFeedback.blockReason（Gemini 安全过滤）
	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		return nil, constant.NewRequestError(
			fmt.Sprintf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason), nil,
		)
	}

	openaiResp := geminiToOpenAIResponse(&geminiResp, info)

	respBody, _ := json.Marshal(openaiResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	usage := &common.Usage{}
	if geminiResp.UsageMetadata != nil {
		usage = geminiUsageToCommon(geminiResp.UsageMetadata)
	}
	return usage, nil
}

// handleStreamToOpenAI 将 Gemini 流式响应转换为 OpenAI SSE 格式
func (a *Adaptor) handleStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 不写响应：交上层 WriteRelayError 统一写入，避免双重写入与重试时的响应污染
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	isCA := a.isCodeAssistActive()
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var (
		totalUsage   dto.GeminiUsageMetadata
		finishReason string
		toolCallIdx  int
		modelName    string
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return &common.Usage{}, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data, _ := helper.ExtractSSEData(line)

		if data != "" && data != "[DONE]" {
			info.SetFirstResponseTime()
		}

		if data == "[DONE]" {
			break
		}

		var geminiResp dto.GeminiChatResponse
		rawData := []byte(data)
		if isCA {
			rawData = unwrapCodeAssistData(rawData)
		}
		if err := json.Unmarshal(rawData, &geminiResp); err != nil {
			continue
		}

		// 收集用量
		if geminiResp.UsageMetadata != nil {
			totalUsage = *geminiResp.UsageMetadata
		}

		// 检查 promptFeedback.blockReason（流式安全过滤）
		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			// SSE 头已发送，需先发送结束 chunk 和 [DONE] 避免客户端挂起
			filterReason := "content_filter"
			endChunk := dto.ChatCompletionStreamResponse{
				ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
				Object: "chat.completion.chunk",
				Model:  info.OriginModelName,
				Choices: []dto.StreamChoice{{
					Index:        0,
					FinishReason: &filterReason,
				}},
			}
			writeStreamChunk(writer, &endChunk)
			_ = helper.WriteSSEData(writer, "[DONE]")
			info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
			return nil, constant.NewRequestError(
				fmt.Sprintf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason), nil,
			)
		}

		// 收集模型名
		if geminiResp.ModelName != "" {
			modelName = geminiResp.ModelName
		}

		for _, candidate := range geminiResp.Candidates {
			if candidate.FinishReason != "" {
				finishReason = common.GeminiFinishReasonToOpenAI(candidate.FinishReason)
			}

			if candidate.Content == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				isThought := part.Thought != nil && *part.Thought

				// 文本内容
				if part.Text != "" {
					if isThought {
						// 思考内容 → reasoning_content
						text := part.Text
						chunk := dto.ChatCompletionStreamResponse{
							ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
							Object: "chat.completion.chunk",
							Model:  modelName,
							Choices: []dto.StreamChoice{{
								Index: 0,
								Delta: dto.Message{
									Role:             "assistant",
									ReasoningContent: &text,
								},
							}},
						}
						if modelName == "" {
							chunk.Model = info.OriginModelName
						}
						writeStreamChunk(writer, &chunk)
					} else {
						chunk := dto.ChatCompletionStreamResponse{
							ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
							Object: "chat.completion.chunk",
							Model:  modelName,
							Choices: []dto.StreamChoice{{
								Index: 0,
								Delta: dto.Message{
									Role:    "assistant",
									Content: part.Text,
								},
							}},
						}
						if modelName == "" {
							chunk.Model = info.OriginModelName
						}
						writeStreamChunk(writer, &chunk)
					}
				}

				// inline image data
				if part.InlineData != nil {
					imageText := fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data)
					imageChunk := dto.ChatCompletionStreamResponse{
						ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
						Object: "chat.completion.chunk",
						Model:  modelName,
						Choices: []dto.StreamChoice{{
							Index: 0,
							Delta: dto.Message{
								Role:    "assistant",
								Content: imageText,
							},
						}},
					}
					if modelName == "" {
						imageChunk.Model = info.OriginModelName
					}
					writeStreamChunk(writer, &imageChunk)
				}

				// executable code
				if part.ExecutableCode != nil {
					codeText := fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code)
					codeChunk := dto.ChatCompletionStreamResponse{
						ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
						Object: "chat.completion.chunk",
						Model:  modelName,
						Choices: []dto.StreamChoice{{
							Index: 0,
							Delta: dto.Message{
								Role:    "assistant",
								Content: codeText,
							},
						}},
					}
					if modelName == "" {
						codeChunk.Model = info.OriginModelName
					}
					writeStreamChunk(writer, &codeChunk)
				}

				// code execution result
				if part.CodeExecutionResult != nil {
					resultText := fmt.Sprintf("Execution %s:\n%s", part.CodeExecutionResult.Outcome, part.CodeExecutionResult.Output)
					resultChunk := dto.ChatCompletionStreamResponse{
						ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
						Object: "chat.completion.chunk",
						Model:  modelName,
						Choices: []dto.StreamChoice{{
							Index: 0,
							Delta: dto.Message{
								Role:    "assistant",
								Content: resultText,
							},
						}},
					}
					if modelName == "" {
						resultChunk.Model = info.OriginModelName
					}
					writeStreamChunk(writer, &resultChunk)
				}

				// file data
				if part.FileData != nil {
					fileText := fmt.Sprintf("[file](%s)", part.FileData.FileURI)
					fileChunk := dto.ChatCompletionStreamResponse{
						ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
						Object: "chat.completion.chunk",
						Model:  modelName,
						Choices: []dto.StreamChoice{{
							Index: 0,
							Delta: dto.Message{
								Role:    "assistant",
								Content: fileText,
							},
						}},
					}
					if modelName == "" {
						fileChunk.Model = info.OriginModelName
					}
					writeStreamChunk(writer, &fileChunk)
				}

				// function call
				if part.FunctionCall != nil {
					argsJSON, _ := json.Marshal(part.FunctionCall.Arguments)
					chunk := dto.ChatCompletionStreamResponse{
						ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
						Object: "chat.completion.chunk",
						Model:  modelName,
						Choices: []dto.StreamChoice{{
							Index: 0,
							Delta: dto.Message{
								ToolCalls: []dto.ToolCall{{
									ID:   fmt.Sprintf("call_%s_%d", info.RequestID, toolCallIdx),
									Type: "function",
									Function: dto.FunctionCall{
										Name:      part.FunctionCall.FunctionName,
										Arguments: string(argsJSON),
									},
								}},
							},
						}},
					}
					if modelName == "" {
						chunk.Model = info.OriginModelName
					}
					writeStreamChunk(writer, &chunk)
					toolCallIdx++
				}
			}
		}
	}

	// 发送结束 chunk
	reason := finishReason
	if reason == "" {
		reason = "stop"
	}
	endChunk := dto.ChatCompletionStreamResponse{
		ID:     fmt.Sprintf("chatcmpl-%s", info.RequestID),
		Object: "chat.completion.chunk",
		Model:  modelName,
		Choices: []dto.StreamChoice{{
			Index:        0,
			FinishReason: &reason,
		}},
	}
	if modelName == "" {
		endChunk.Model = info.OriginModelName
	}

	if totalUsage.PromptTokenCount > 0 || totalUsage.CandidatesTokenCount > 0 {
		endChunk.Usage = &dto.UsageWithDetails{
			PromptTokens:     totalUsage.PromptTokenCount,
			CompletionTokens: totalUsage.CandidatesTokenCount,
			TotalTokens:      totalUsage.TotalTokenCount,
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		writeStreamChunk(writer, &endChunk)
		_ = helper.WriteSSEData(writer, "[DONE]")
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return geminiUsageToCommon(&totalUsage), fmt.Errorf("stream scanner error: %w", err)
	}

	writeStreamChunk(writer, &endChunk)
	_ = helper.WriteSSEData(writer, "[DONE]")
	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)

	return geminiUsageToCommon(&totalUsage), nil
}

// geminiToOpenAIResponse 将 Gemini 非流式响应转换为 OpenAI ChatCompletion 格式
func geminiToOpenAIResponse(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) dto.ChatCompletionResponse {
	resp := dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", info.RequestID),
		Object:  "chat.completion",
		Created: 0,
		Model:   info.OriginModelName,
		Choices: make([]dto.Choice, 0),
	}

	if len(geminiResp.Candidates) == 0 {
		return resp
	}

	candidate := geminiResp.Candidates[0]
	choice := dto.Choice{
		Index:        candidate.Index,
		FinishReason: common.GeminiFinishReasonToOpenAI(candidate.FinishReason),
	}

	if candidate.Content != nil {
		var textParts []string
		var thinkingParts []string
		var toolCalls []dto.ToolCall
		toolIdx := 0

		for _, part := range candidate.Content.Parts {
			isThought := part.Thought != nil && *part.Thought

			if part.Text != "" {
				if isThought {
					thinkingParts = append(thinkingParts, part.Text)
				} else {
					textParts = append(textParts, part.Text)
				}
			}
			// Gemini inline image data
			if part.InlineData != nil {
				textParts = append(textParts, fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data))
			}
			// file data
			if part.FileData != nil {
				textParts = append(textParts, fmt.Sprintf("[file](%s)", part.FileData.FileURI))
			}
			// executable code
			if part.ExecutableCode != nil {
				textParts = append(textParts, fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code))
			}
			// code execution result
			if part.CodeExecutionResult != nil {
				textParts = append(textParts, fmt.Sprintf("Execution %s:\n%s", part.CodeExecutionResult.Outcome, part.CodeExecutionResult.Output))
			}
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Arguments)
				toolCalls = append(toolCalls, dto.ToolCall{
					ID:   fmt.Sprintf("call_%s_%d", info.RequestID, toolIdx),
					Type: "function",
					Function: dto.FunctionCall{
						Name:      part.FunctionCall.FunctionName,
						Arguments: string(argsJSON),
					},
				})
				toolIdx++
			}
		}

		message := dto.Message{
			Role:    "assistant",
			Content: strings.Join(textParts, ""),
		}
		if len(thinkingParts) > 0 {
			joined := strings.Join(thinkingParts, "\n")
			message.ReasoningContent = &joined
		}
		if len(toolCalls) > 0 {
			message.ToolCalls = toolCalls
			choice.FinishReason = "tool_calls"
		}
		choice.Message = message
	}

	resp.Choices = append(resp.Choices, choice)

	if geminiResp.UsageMetadata != nil {
		resp.Usage = dto.UsageWithDetails{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return resp
}

// writeStreamChunk 写入流式 chunk
func writeStreamChunk(w http.ResponseWriter, chunk *dto.ChatCompletionStreamResponse) {
	data, _ := json.Marshal(chunk)
	_ = helper.WriteSSEData(w, string(data))
}

// geminiUsageToCommon 将 Gemini UsageMetadata 转换为 common.Usage
func geminiUsageToCommon(um *dto.GeminiUsageMetadata) *common.Usage {
	if um == nil {
		return &common.Usage{}
	}
	usage := &common.Usage{
		PromptTokens:     um.PromptTokenCount,
		CompletionTokens: um.CandidatesTokenCount,
		TotalTokens:      um.TotalTokenCount,
		PromptTokensDetails: &common.TokenDetails{
			CachedTokens: um.CachedContentTokenCount,
		},
		CompletionTokenDetails: &common.TokenDetails{
			ReasoningTokens: um.ThoughtsTokenCount,
		},
	}

	// 转换模态 Token 明细
	if len(um.PromptTokensDetails) > 0 || len(um.CandidatesTokensDetails) > 0 {
		if usage.PromptTokensDetails == nil {
			usage.PromptTokensDetails = &common.TokenDetails{}
		}
		for _, mtc := range um.PromptTokensDetails {
			geminiModalityToTokenDetails(mtc, usage.PromptTokensDetails)
		}
		for _, mtc := range um.CandidatesTokensDetails {
			geminiModalityToTokenDetails(mtc, usage.CompletionTokenDetails)
		}
	}

	return usage
}

// geminiModalityToTokenDetails 将 Gemini 模态 Token 计数转换为 OpenAI TokenDetails 字段
func geminiModalityToTokenDetails(mtc dto.GeminiModalityTokenCount, td *common.TokenDetails) {
	if td == nil {
		return
	}
	switch mtc.Modality {
	case "TEXT":
		td.TextTokens += mtc.TokenCount
	case "IMAGE":
		td.ImageTokens += mtc.TokenCount
	case "AUDIO":
		td.AudioTokens += mtc.TokenCount
	}
}

// ===== 上游错误格式转换 =====

// parseGeminiError 解析 Gemini RPC Status 错误格式
func parseGeminiError(body []byte) (code int, status string, message string) {
	var rpcStatus struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcStatus); err == nil && rpcStatus.Error.Code != 0 {
		return rpcStatus.Error.Code, rpcStatus.Error.Status, rpcStatus.Error.Message
	}
	return 0, "", string(body)
}

// geminiStatusToOpenAIType 将 Gemini RPC status 映射为 OpenAI error type
func geminiStatusToOpenAIType(status string) string {
	switch status {
	case "UNAUTHENTICATED":
		return "authentication_error"
	case "PERMISSION_DENIED":
		return "permission_error"
	case "INVALID_ARGUMENT":
		return "invalid_request_error"
	case "NOT_FOUND":
		return "invalid_request_error"
	case "RESOURCE_EXHAUSTED", "RATE_LIMIT_EXCEEDED":
		return "rate_limit_error"
	case "INTERNAL":
		return "internal_error"
	case "UNAVAILABLE":
		return "server_error"
	case "DEADLINE_EXCEEDED":
		return "timeout_error"
	default:
		return "api_error"
	}
}

// buildGeminiUpstreamError 解析 Gemini 上游错误，构造携带正确 type 的 RelayError。
// 用于 OpenAI 出站路径：adaptor 不直接写响应，交上层错误写入器（WriteRelayError）统一写入一次，
// 既消除双重写入，又避免非流式可重试错误提前 WriteHeader 造成的重试响应污染。
func buildGeminiUpstreamError(body []byte, defaultStatusCode int) *constant.RelayError {
	code, status, message := parseGeminiError(body)
	if code == 0 {
		code = defaultStatusCode
	}
	if code < 100 || code > 599 {
		code = 500
	}
	return &constant.RelayError{
		StatusCode: code,
		Message:    message,
		Type:       geminiStatusToOpenAIType(status),
	}
}
