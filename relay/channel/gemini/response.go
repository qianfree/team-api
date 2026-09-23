package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
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
	if err := json.Unmarshal(data, &wrapper); err != nil {
		// 解析失败：日志记录，返回原数据（可能是标准格式或格式错误）
		// 不在这里打 Debug 日志，因为标准格式也会走到这个分支（预期行为）
		return data
	}
	if wrapper.Response != nil {
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
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		// Usage 解析失败，返回空 Usage（静默处理，非致命错误）
		return &common.Usage{}, nil
	}
	if geminiResp.UsageMetadata != nil {
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

	// 按帧攒齐再写：本路径当前没有并发的保活 ping，但逐行写既不是帧原子（将来接入 ping
	// 或多行 data 时会被空行劈开），也会按行 Flush 产生多余系统调用；统一走帧写入器。
	frame := helper.NewSSEFrameWriter(writer)

	var totalUsage dto.GeminiUsageMetadata

	for {
		select {
		case <-ctx.Done():
			// 未完成的半帧尚未出网，丢弃即可
			frame.Discard()
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断：返回已累计的 usage（Gemini 每个 chunk 携带累计值），输入缺失用请求侧估算补齐
			interruptedUsage := geminiUsageToCommon(&totalUsage)
			helper.ApplyInterruptedUsageFallback(info, interruptedUsage, 0)
			return interruptedUsage, common.ErrStreamInterrupted
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
					if werr := frame.WriteLine(outputLine); werr != nil {
						info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, werr)
						break
					}
					// [DONE] 后直接收尾，须冲刷缓冲，否则这一行发不出去
					_ = frame.FlushPartial()
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
					if jsonErr := json.Unmarshal(rawData, &geminiResp); jsonErr != nil {
						// JSON 解析失败：静默跳过
					} else {
						if geminiResp.UsageMetadata != nil {
							totalUsage = *geminiResp.UsageMetadata
						}
					}
				}
			}

			// 输出 SSE 行（解包后的），遇帧分隔空行整帧写出
			if werr := frame.WriteLine(outputLine); werr != nil {
				info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, werr)
				break
			}
		}

		if err == io.EOF {
			// 上游未以空行结尾时冲刷残留，避免丢掉最后一帧
			_ = frame.FlushPartial()
			break
		}
		if err != nil {
			// 上游读取出错：缓冲里的残留是不完整的帧，写给客户端只会造成非法 SSE，丢弃
			frame.Discard()
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

	// relaykit 响应转换（唯一路径，hard-fail：解析/转换失败即向上返回错误，不再回退旧实现）
	convertedBody, _, handled, convErr := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !handled {
		return nil, constant.NewChannelError("gemini adaptor: no relaykit converter for openai client response", nil)
	}
	if convErr != nil {
		return nil, relaykit_bridge.ResponseConvertError(convErr, resp.StatusCode, resp.Header)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// relaykit 转换器返回的 Usage 为 nil（ResponseConverterFunc 签名约束），从原始 Gemini 响应提取
	var geminiResp dto.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err == nil && geminiResp.UsageMetadata != nil {
		return geminiUsageToCommon(geminiResp.UsageMetadata), nil
	}
	// 如果 Usage 解析失败，返回空 Usage（已写响应，不能重试）
	return &common.Usage{}, nil
}

// handleStreamToOpenAI 将 Gemini 流式响应转换为 OpenAI SSE 格式
func (a *Adaptor) handleStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 不写响应：交上层 WriteRelayError 统一写入，避免双重写入与重试时的响应污染
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	// relaykit 流式转换（唯一路径，hard-fail）：写入前失败由桥接层返回未接管，此处显式报错
	if usage, ok, err := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer); ok {
		// 桥接层已写出 SSE 并完成收尾裁决：err 非空即流以错误/客户端中断结束
		//（已带 ResponseWritten，上层只记账与上报调度，不重写响应体、不换渠道重试），
		// usage 中断兜底亦已在桥接层完成，此处原样上抛即可。
		return usage, err
	}
	return nil, constant.NewChannelError("gemini adaptor: relaykit stream converter unavailable for openai client", nil)
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
			// Gemini 内联图片数据
			if part.InlineData != nil {
				textParts = append(textParts, fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data))
			}
			// 文件数据
			if part.FileData != nil {
				textParts = append(textParts, fmt.Sprintf("[file](%s)", part.FileData.FileURI))
			}
			// 可执行代码
			if part.ExecutableCode != nil {
				textParts = append(textParts, fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code))
			}
			// 代码执行结果
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
		// OpenAI 口径：prompt 含 cached（子集），completion 含 thoughts（子集），
		// 与计费用量 geminiUsageToCommon 保持同一换算
		resp.Usage = dto.UsageWithDetails{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount + geminiResp.UsageMetadata.ThoughtsTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
			PromptTokensDetails: &dto.TokenDetails{
				CachedTokens: geminiResp.UsageMetadata.CachedContentTokenCount,
			},
			CompletionTokenDetails: &dto.TokenDetails{
				ReasoningTokens: geminiResp.UsageMetadata.ThoughtsTokenCount,
			},
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
		PromptTokens: um.PromptTokenCount,
		// Gemini 的 candidatesTokenCount 不含思考 token，thoughtsTokenCount 是输出侧
		// 独立字段（按输出价计费）。OpenAI 口径的 completion 含 reasoning（子集语义），
		// 计费用量必须 candidates+thoughts 合计，否则思考 token 漏计费
		CompletionTokens: um.CandidatesTokenCount + um.ThoughtsTokenCount,
		TotalTokens:      um.TotalTokenCount,
		// Gemini 的 promptTokenCount 已含 cachedContentTokenCount（cached 为其子集），
		// 置 true 让计费先扣减缓存部分，避免「input 全价 + cache 价」双重计费
		CacheIncludedInPrompt: true,
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
