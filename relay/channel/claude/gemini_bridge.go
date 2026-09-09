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

// ========== Gemini 入站桥接：Claude Messages → Gemini GenerateContent ==========
//
// 请求侧由 ConvertGeminiToClaude 完成（Gemini → OpenAI → Claude），
// 这里做响应侧：把 Claude 上游的响应/SSE 转回 Gemini 格式。
// 事件形态与 openai 包的 Gemini 出站（openai/gemini_response.go）保持一致：
// 逐 chunk 写 data 行、末尾补 finishReason + usageMetadata 的收尾 chunk，再发 [DONE]。
//
// 协议落差（转换时按下列口径处理）：
//   - Claude 的 output_tokens 含思考 token 且不单独拆分，Gemini 的 candidatesTokenCount
//     语义上不含 thoughts → 无法还原拆分，thoughtsTokenCount 记 0、全部计入 candidates。
//   - redacted_thinking 是 Claude 的加密思考块，Gemini 无对应物 → 丢弃。
//   - thinking.signature 搬到 Gemini 的 thoughtSignature 上（单向出站）。客户端回传的
//     签名在请求侧 Gemini→OpenAI→Claude 的 chat 中间格式里没有字段承载，故本方向的
//     签名往返不闭合；这是已知取舍，不是遗漏。

// geminiToolCallState 流式聚合中的工具调用（Claude tool_use 块）。
// Claude 的参数按 input_json_delta 分片下发，而 Gemini 的 functionCall.args 必须是
// 完整对象，因此必须缓冲到 content_block_stop 才能发出。
type geminiToolCallState struct {
	id   string
	name string
	args strings.Builder
}

// handleNonStreamToGemini 将 Claude 非流式响应转换为 Gemini 格式
func (a *Adaptor) handleNonStreamToGemini(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}
	if resp.StatusCode != http.StatusOK {
		// 不写响应：Gemini 入站由上层 WriteGeminiRelayError 统一写入，避免双写与重试时的响应污染
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	geminiResp := claudeToGeminiResponse(&claudeResp, info)

	respBody, _ := json.Marshal(geminiResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	// 计费用量走 Claude 口径（input 不含缓存、cache_creation 独立计价），
	// 与本适配器其他出站路径一致；上面写给客户端的是 Gemini 协议口径。
	if claudeResp.Usage != nil {
		return buildUsageFromClaude(claudeResp.Usage), nil
	}
	return &common.Usage{}, nil
}

// handleStreamToGemini 将 Claude SSE 流转换为 Gemini SSE 流
func (a *Adaptor) handleStreamToGemini(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 不写响应：交上层 WriteGeminiRelayError 统一写入
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	modelName := info.OriginModelName

	var (
		usage              dto.ClaudeUsage
		stopReason         string
		currentTool        *geminiToolCallState
		transferredTextLen int // 已转发的文本/思考内容长度，供流中断输出估算
	)

	// emitParts 发出一个仅含 content parts 的 Gemini chunk
	emitParts := func(parts []dto.GeminiPart) {
		if len(parts) == 0 {
			return
		}
		chunk := dto.GeminiChatResponse{
			ModelName: modelName,
			Candidates: []dto.GeminiCandidate{{
				Content: &dto.GeminiContent{Role: "model", Parts: parts},
			}},
		}
		// 传指针而非结构体值：值传进 any 会额外堆分配一份拷贝用于装箱
		_ = helper.WriteSSEDataJSON(writer, &chunk)
	}

	// flushTool 把缓冲的工具调用作为 functionCall part 发出
	flushTool := func() {
		if currentTool == nil {
			return
		}
		var args any
		if raw := currentTool.args.String(); raw != "" {
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				// 参数分片拼接后仍非法：降级为空对象，保留函数名让客户端可见调用意图
				args = map[string]any{}
			}
		} else {
			args = map[string]any{}
		}
		emitParts([]dto.GeminiPart{{
			FunctionCall: &dto.GeminiFunctionCall{
				// Gemini 的 functionCall 支持可选 id，直接搬运 Claude 的 tool_use.id
				ID:           currentTool.id,
				FunctionName: currentTool.name,
				Arguments:    args,
			},
		}})
		currentTool = nil
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断计费兜底：输出缺失按已转发文本估算，输入用请求侧估算值补齐
			interruptedUsage := buildUsageFromClaude(&usage)
			helper.ApplyInterruptedUsageFallback(info, interruptedUsage, transferredTextLen)
			return interruptedUsage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "event:") {
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

		var event dto.ClaudeResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			// JSON 解析失败：静默跳过（允许部分格式异常）
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				if event.Message.Model != "" {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}

		case "content_block_start":
			if event.ContentBlock == nil {
				continue
			}
			if event.ContentBlock.Type == "tool_use" {
				// 上一个工具块理论上已在 content_block_stop 冲刷，这里兜底防止串块
				flushTool()
				currentTool = &geminiToolCallState{
					id:   event.ContentBlock.ID,
					name: event.ContentBlock.Name,
				}
			}

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != nil && *event.Delta.Text != "" {
					transferredTextLen += len(*event.Delta.Text)
					emitParts([]dto.GeminiPart{{Text: *event.Delta.Text}})
				}
			case "thinking_delta":
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					transferredTextLen += len(*event.Delta.Thinking)
					emitParts([]dto.GeminiPart{{Text: *event.Delta.Thinking, Thought: boolPtr(true)}})
				}
			case "signature_delta":
				if event.Delta.Signature != "" {
					// 思考签名单独成 part（无文本），与 Gemini 把签名挂在 thought part 上的形态一致
					emitParts([]dto.GeminiPart{{Thought: boolPtr(true), ThoughtSignature: event.Delta.Signature}})
				}
			case "input_json_delta":
				if currentTool != nil && event.Delta.PartialJSON != nil {
					currentTool.args.WriteString(*event.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			flushTool()

		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != nil {
				stopReason = *event.Delta.StopReason
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

		case "error":
			errMsg := "claude stream error"
			if event.Error != nil {
				if b, err := json.Marshal(event.Error); err == nil {
					errMsg = fmt.Sprintf("claude stream error: %s", string(b))
				}
			}
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("%s", errMsg))

		case "message_stop":
			// 收尾在循环外统一处理，保证异常结束路径也走同一套
		}
	}

	// 未闭合的工具块（上游异常断流）：兜底冲刷，避免整个调用丢失
	flushTool()

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		// 读流出错：带上已收到的 usage，避免整段按零值计费漏计
		return buildUsageFromClaude(&usage), fmt.Errorf("stream scanner error: %w", err)
	}

	// 收尾 chunk：finishReason + usageMetadata（与 Gemini 出站的既有形态一致）
	finalChunk := dto.GeminiChatResponse{
		ModelName: modelName,
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model"},
			FinishReason: common.ClaudeStopReasonToGemini(stopReason),
		}},
	}
	if um := geminiUsageFromClaude(&usage); um != nil {
		finalChunk.UsageMetadata = um
	}
	_ = helper.WriteSSEDataJSON(writer, &finalChunk)

	_ = helper.WriteSSEData(writer, "[DONE]")
	if info.StreamStatus.GetEndReason() == "" {
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	return buildUsageFromClaude(&usage), nil
}

// claudeToGeminiResponse 将 Claude 非流式响应转换为 Gemini 响应。
// 内容块顺序保持 Claude 的原始顺序（模型已按语义排好 thinking / 正文 / 工具调用的先后）。
func claudeToGeminiResponse(claudeResp *dto.ClaudeResponse, info *common.RelayInfo) dto.GeminiChatResponse {
	parts := make([]dto.GeminiPart, 0, len(claudeResp.Content))

	for i := range claudeResp.Content {
		block := &claudeResp.Content[i]
		switch block.Type {
		case "text":
			if block.Text != nil && *block.Text != "" {
				parts = append(parts, dto.GeminiPart{Text: *block.Text})
			}
		case "thinking":
			if block.Thinking != nil && *block.Thinking != "" {
				parts = append(parts, dto.GeminiPart{
					Text:             *block.Thinking,
					Thought:          boolPtr(true),
					ThoughtSignature: block.Signature,
				})
			}
		case "redacted_thinking":
			// Claude 的加密思考块，Gemini 无对应物，丢弃
		case "tool_use":
			parts = append(parts, dto.GeminiPart{
				FunctionCall: &dto.GeminiFunctionCall{
					ID:           block.ID,
					FunctionName: block.Name,
					Arguments:    claudeToolInput(block.Input),
				},
			})
		}
	}

	modelName := claudeResp.Model
	if modelName == "" || info.ChannelMeta.IsModelMapped {
		modelName = info.OriginModelName
	}

	resp := dto.GeminiChatResponse{
		ModelName: modelName,
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model", Parts: parts},
			FinishReason: common.ClaudeStopReasonToGemini(claudeResp.StopReason),
		}},
	}
	if um := geminiUsageFromClaude(claudeResp.Usage); um != nil {
		resp.UsageMetadata = um
	}
	return resp
}

// geminiUsageFromClaude 按 Gemini 协议口径换算用量（客户端可见值，非计费值）。
//   - Gemini 的 promptTokenCount 含缓存，Claude 的 input_tokens 与两项 cache 并列，需相加
//   - Gemini 的 candidatesTokenCount 语义上不含 thoughts，但 Claude 的 output_tokens 含思考
//     且不单独拆分 → 无法还原，全部计入 candidates，thoughtsTokenCount 保持 0
func geminiUsageFromClaude(u *dto.ClaudeUsage) *dto.GeminiUsageMetadata {
	if u == nil {
		return nil
	}
	promptTotal := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
	if promptTotal == 0 && u.OutputTokens == 0 {
		return nil
	}
	return &dto.GeminiUsageMetadata{
		PromptTokenCount:        promptTotal,
		CandidatesTokenCount:    u.OutputTokens,
		TotalTokenCount:         promptTotal + u.OutputTokens,
		CachedContentTokenCount: u.CacheReadInputTokens,
	}
}

// claudeToolInput 归一化 tool_use.input：Gemini 的 functionCall.args 期望对象，
// Claude 允许 null，此时回退为空对象。
func claudeToolInput(input any) any {
	if input == nil {
		return map[string]any{}
	}
	return input
}

// boolPtr 返回 bool 的指针
func boolPtr(v bool) *bool { return &v }
