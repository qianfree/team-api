package claude

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

// handleNonStreamToOpenAI 将 Claude 非流式响应转换为 OpenAI 格式
func (a *Adaptor) handleNonStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}

	// 转换为 OpenAI 格式
	openaiResp := claudeToOpenAIResponse(&claudeResp, info)

	respBody, _ := json.Marshal(openaiResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	usage := &common.Usage{}
	if claudeResp.Usage != nil {
		usage.PromptTokens = claudeResp.Usage.InputTokens
		usage.CompletionTokens = claudeResp.Usage.OutputTokens
		usage.TotalTokens = claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens
		usage.CacheCreationTokens = claudeResp.Usage.CacheCreationInputTokens
		usage.PromptTokensDetails = claudeUsageToTokenDetails(claudeResp.Usage)
	}
	return usage, nil
}

// handleStreamToOpenAI 将 Claude 流式响应转换为 OpenAI SSE 格式
func (a *Adaptor) handleStreamToOpenAI(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	helper.SetEventStreamHeaders(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	responseID := fmt.Sprintf("chatcmpl-%s", info.RequestID)
	createdAt := time.Now().Unix()

	var (
		usage           dto.ClaudeUsage
		modelName       string
		finishReason    string
		toolCallIdx     int
		roleChunkSent   bool
		responseTextBuf strings.Builder
	)

	newChunk := func(delta dto.Message) *dto.ChatCompletionStreamResponse {
		m := modelName
		if m == "" {
			m = info.OriginModelName
		}
		return &dto.ChatCompletionStreamResponse{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   m,
			Choices: []dto.StreamChoice{{
				Index: 0,
				Delta: delta,
			}},
		}
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return buildUsageFromClaude(&usage), common.ErrStreamInterrupted
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

		var event dto.ClaudeResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				modelName = event.Message.Model
				if modelName == "" {
					modelName = info.OriginModelName
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}

			if !roleChunkSent {
				emptyContent := ""
				writeStreamChunk(writer, newChunk(dto.Message{
					Role:    "assistant",
					Content: &emptyContent,
				}))
				roleChunkSent = true
			}

		case "content_block_start":
			if event.ContentBlock == nil {
				continue
			}
			switch event.ContentBlock.Type {
			case "text":
			case "thinking":
			case "redacted_thinking":
				// 脱敏思考，OpenAI 格式无等价物，忽略
			case "tool_use":
				toolCall := dto.ToolCall{
					ID:   event.ContentBlock.ID,
					Type: "function",
					Function: dto.FunctionCall{
						Name:      event.ContentBlock.Name,
						Arguments: "",
					},
				}
				writeStreamChunk(writer, newChunk(dto.Message{
					ToolCalls: []dto.ToolCall{toolCall},
				}))
				toolCallIdx++
			}

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != nil && *event.Delta.Text != "" {
					responseTextBuf.WriteString(*event.Delta.Text)
					writeStreamChunk(writer, newChunk(dto.Message{
						Content: *event.Delta.Text,
					}))
				}
			case "thinking_delta":
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					writeStreamChunk(writer, newChunk(dto.Message{
						ReasoningContent: event.Delta.Thinking,
					}))
				}
			case "input_json_delta":
				if event.Delta.PartialJSON != nil && *event.Delta.PartialJSON != "" {
					writeStreamChunk(writer, newChunk(dto.Message{
						ToolCalls: []dto.ToolCall{{
							Index: toolCallIdx - 1,
							Function: dto.FunctionCall{
								Arguments: *event.Delta.PartialJSON,
							},
						}},
					}))
				}
			case "signature_delta":
			}

		case "content_block_stop":

		case "error":
			errMsg := "claude stream error"
			if event.Error != nil {
				if b, err := json.Marshal(event.Error); err == nil {
					errMsg = fmt.Sprintf("claude stream error: %s", string(b))
				}
			}
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("%s", errMsg))

		case "message_delta":
			if event.Delta != nil {
				if event.Delta.StopReason != nil {
					finishReason = common.ClaudeStopReasonToOpenAI(*event.Delta.StopReason)
				}
			}
			if event.Usage != nil {
				if event.Usage.InputTokens > 0 {
					usage.InputTokens = event.Usage.InputTokens
				}
				usage.OutputTokens = event.Usage.OutputTokens
				if event.Usage.CacheReadInputTokens > 0 {
					usage.CacheReadInputTokens = event.Usage.CacheReadInputTokens
				}
				if event.Usage.CacheCreationInputTokens > 0 {
					usage.CacheCreationInputTokens = event.Usage.CacheCreationInputTokens
				}
				if event.Usage.CacheCreation != nil {
					usage.CacheCreation = event.Usage.CacheCreation
				}
			}

		case "message_stop":
			reason := finishReason
			if reason == "" {
				reason = "stop"
			}
			usageObj := &dto.UsageWithDetails{
				PromptTokens:        usage.InputTokens,
				CompletionTokens:    usage.OutputTokens,
				TotalTokens:         usage.InputTokens + usage.OutputTokens,
				PromptTokensDetails: common.CommonTokenDetailsToDto(claudeUsageToTokenDetails(&usage)),
			}
			if usageObj.CompletionTokens == 0 {
				estimated := responseTextBuf.Len() / 4
				if estimated > 0 {
					usageObj.CompletionTokens = estimated
					usageObj.TotalTokens = usageObj.PromptTokens + usageObj.CompletionTokens
				}
			}

			chunk := &dto.ChatCompletionStreamResponse{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   modelName,
				Choices: []dto.StreamChoice{{
					Index:        0,
					FinishReason: &reason,
				}},
				Usage: usageObj,
			}
			if chunk.Model == "" {
				chunk.Model = info.OriginModelName
			}
			writeStreamChunk(writer, chunk)
			helper.WriteSSEData(writer, "[DONE]")
			info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return &common.Usage{}, fmt.Errorf("stream scanner error: %w", err)
	}

	if info.StreamStatus.GetEndReason() == "" {
		reason := "stop"
		if finishReason != "" {
			reason = finishReason
		}
		usageObj := &dto.UsageWithDetails{
			PromptTokens:        usage.InputTokens,
			CompletionTokens:    usage.OutputTokens,
			TotalTokens:         usage.InputTokens + usage.OutputTokens,
			PromptTokensDetails: common.CommonTokenDetailsToDto(claudeUsageToTokenDetails(&usage)),
		}
		if usageObj.CompletionTokens == 0 {
			estimated := responseTextBuf.Len() / 4
			if estimated > 0 {
				usageObj.CompletionTokens = estimated
				usageObj.TotalTokens = usageObj.PromptTokens + usageObj.CompletionTokens
			}
		}
		chunk := &dto.ChatCompletionStreamResponse{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   modelName,
			Choices: []dto.StreamChoice{{
				Index:        0,
				FinishReason: &reason,
			}},
			Usage: usageObj,
		}
		if chunk.Model == "" {
			chunk.Model = info.OriginModelName
		}
		writeStreamChunk(writer, chunk)
		helper.WriteSSEData(writer, "[DONE]")
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	return &common.Usage{
		PromptTokens:        usage.InputTokens,
		CompletionTokens:    usage.OutputTokens,
		TotalTokens:         usage.InputTokens + usage.OutputTokens,
		CacheCreationTokens: usage.CacheCreationInputTokens,
		PromptTokensDetails: claudeUsageToTokenDetails(&usage),
	}, nil
}

// handleClaudeNativeResponse 直通 Claude 原生格式响应
func (a *Adaptor) handleClaudeNativeResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if info.IsStream {
		return a.handleClaudeNativeStream(ctx, resp, info, writer)
	}
	return a.handleClaudeNativeNonStream(ctx, resp, info, writer)
}

// handleClaudeNativeNonStream 直通 Claude 非流式响应
func (a *Adaptor) handleClaudeNativeNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	if info.ChannelMeta.IsModelMapped {
		body = replaceModelName(body, info.OriginModelName)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(resp.StatusCode)
	_, _ = writer.Write(body)

	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err == nil && claudeResp.Usage != nil {
		return &common.Usage{
			PromptTokens:        claudeResp.Usage.InputTokens,
			CompletionTokens:    claudeResp.Usage.OutputTokens,
			TotalTokens:         claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
			CacheCreationTokens: claudeResp.Usage.CacheCreationInputTokens,
			PromptTokensDetails: claudeUsageToTokenDetails(claudeResp.Usage),
		}, nil
	}

	return &common.Usage{}, nil
}

// handleClaudeNativeStream 直通 Claude 流式响应
func (a *Adaptor) handleClaudeNativeStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	helper.SetEventStreamHeaders(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	reader := bufio.NewReaderSize(resp.Body, 64*1024)
	var usage dto.ClaudeUsage

	for {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return buildUsageFromClaude(&usage), common.ErrStreamInterrupted
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			return buildUsageFromClaude(&usage), fmt.Errorf("stream read error: %w", err)
		}

		if strings.HasPrefix(line, "data:") {
			data, _ := helper.ExtractSSEData(line)

			if data != "" && data != "[DONE]" {
				info.SetFirstResponseTime()
			}

			var event dto.ClaudeResponse
			if json.Unmarshal([]byte(data), &event) == nil {
				switch event.Type {
				case "message_start":
					if event.Message != nil && event.Message.Usage != nil {
						usage = *event.Message.Usage
					}
				case "message_delta":
					if event.Usage != nil {
						if event.Usage.InputTokens > 0 {
							usage.InputTokens = event.Usage.InputTokens
						}
						usage.OutputTokens = event.Usage.OutputTokens
						if event.Usage.CacheReadInputTokens > 0 {
							usage.CacheReadInputTokens = event.Usage.CacheReadInputTokens
						}
						if event.Usage.CacheCreationInputTokens > 0 {
							usage.CacheCreationInputTokens = event.Usage.CacheCreationInputTokens
						}
						if event.Usage.CacheCreation != nil {
							usage.CacheCreation = event.Usage.CacheCreation
						}
					}
				case "error":
					info.StreamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("claude upstream stream error"))
				}
			}

			if info.ChannelMeta.IsModelMapped {
				replaced := string(replaceModelName([]byte(data), info.OriginModelName))
				line = fmt.Sprintf("data: %s\n", replaced)
			}
		}

		if _, err := writer.Write([]byte(line)); err != nil {
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, err)
			return buildUsageFromClaude(&usage), common.ErrStreamInterrupted
		}

		if len(line) == 1 && line[0] == '\n' {
			if f, ok := writer.(http.Flusher); ok {
				f.Flush()
			}
		}
	}

	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)

	return buildUsageFromClaude(&usage), nil
}

// buildUsageFromClaude 从已累积的 ClaudeUsage 构建 common.Usage，保留 cache 字段
func buildUsageFromClaude(u *dto.ClaudeUsage) *common.Usage {
	return &common.Usage{
		PromptTokens:        u.InputTokens,
		CompletionTokens:    u.OutputTokens,
		TotalTokens:         u.InputTokens + u.OutputTokens,
		CacheCreationTokens: u.CacheCreationInputTokens,
		PromptTokensDetails: claudeUsageToTokenDetails(u),
	}
}

// claudeUsageToTokenDetails 将 ClaudeUsage 转换为 TokenDetails（含 cache token 细分）
func claudeUsageToTokenDetails(u *dto.ClaudeUsage) *common.TokenDetails {
	if u == nil {
		return nil
	}
	td := &common.TokenDetails{
		CachedTokens:         u.CacheReadInputTokens,
		CachedCreationTokens: u.CacheCreationInputTokens,
	}
	if u.CacheCreation != nil {
		td.CachedCreation5mTokens = u.CacheCreation.Ephemeral5mInputTokens
		td.CachedCreation1hTokens = u.CacheCreation.Ephemeral1hInputTokens
	}
	return td
}

// claudeToOpenAIResponse 将 Claude 非流式响应转换为 OpenAI ChatCompletion 格式
func claudeToOpenAIResponse(claudeResp *dto.ClaudeResponse, info *common.RelayInfo) dto.ChatCompletionResponse {
	resp := dto.ChatCompletionResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   claudeResp.Model,
		Choices: []dto.Choice{{
			Index:        0,
			FinishReason: common.ClaudeStopReasonToOpenAI(claudeResp.StopReason),
		}},
	}

	if info.ChannelMeta.IsModelMapped {
		resp.Model = info.OriginModelName
	}

	var textParts []string
	var thinkingParts []string
	var toolCalls []dto.ToolCall

	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				textParts = append(textParts, *block.Text)
			}
		case "thinking":
			if block.Thinking != nil {
				thinkingParts = append(thinkingParts, *block.Thinking)
			}
		case "redacted_thinking":
			// 脱敏思考，OpenAI 格式无等价物，忽略
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, dto.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: dto.FunctionCall{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	message := dto.Message{
		Role:    "assistant",
		Content: joinTextPartsResponse(textParts),
	}
	if len(thinkingParts) > 0 {
		thinking := strings.Join(thinkingParts, "")
		message.ReasoningContent = &thinking
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}
	resp.Choices[0].Message = message

	if claudeResp.Usage != nil {
		resp.Usage = dto.UsageWithDetails{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		}
	}

	return resp
}

// writeStreamChunk 写入流式 chunk
func writeStreamChunk(w http.ResponseWriter, chunk *dto.ChatCompletionStreamResponse) {
	data, _ := json.Marshal(chunk)
	_ = helper.WriteSSEData(w, string(data))
}

// replaceModelName 替换 JSON 中 "model" 字段的值
func replaceModelName(body []byte, modelName string) []byte {
	fieldPrefix := []byte(`"model":"`)
	replacement := append([]byte(`"model":"`), modelName...)
	replacement = append(replacement, '"')

	result := make([]byte, 0, len(body))
	i := 0
	for i < len(body) {
		idx := indexOf(body[i:], fieldPrefix)
		if idx == -1 {
			result = append(result, body[i:]...)
			break
		}
		result = append(result, body[i:i+idx+len(fieldPrefix)]...)
		i += idx + len(fieldPrefix)

		endQuote := indexOf(body[i:], []byte{'"'})
		if endQuote == -1 {
			result = append(result, body[i:]...)
			break
		}
		i += endQuote + 1
		result = append(result, replacement...)
	}
	return result
}

func indexOf(s, sep []byte) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if equal(s[i:i+len(sep)], sep) {
			return i
		}
	}
	return -1
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func joinTextPartsResponse(parts []string) any {
	if len(parts) == 0 {
		return nil
	}
	result := make([]byte, 0, 64)
	for i, p := range parts {
		if i > 0 {
			result = append(result, '\n')
		}
		result = append(result, p...)
	}
	return string(result)
}
