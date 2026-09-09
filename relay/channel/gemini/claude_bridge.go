package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// ========== Claude 入站桥接：Gemini → Claude Messages ==========
//
// 请求侧由 ConvertClaudeToGemini 完成（Claude → OpenAI → Gemini），
// 这里做响应侧：把 Gemini 上游的响应/SSE 转回 Claude Messages 格式。
// 与 openai 包的 Claude 出站（openai/claude_response.go）事件序列保持一致，
// 差别只在前端解析的是 Gemini candidates/parts 而非 OpenAI choices/delta。
//
// 协议落差（Gemini 侧无对应物，转换时按下列口径处理）：
//   - functionCall 没有调用 ID，Claude 的 tool_use 必须带 id → 本层合成，
//     客户端回传的 tool_result.tool_use_id 在请求侧按函数名还原（与
//     openai/converter.go 的 g2oConvertContent 同一套约定）。
//   - inlineData / executableCode / codeExecutionResult / fileData 在 Claude
//     的 assistant 内容块里没有对应类型 → 按既有 Gemini→OpenAI 口径渲染为文本，
//     信息保留而非丢弃。
//   - thoughtSignature 只在 thinking 块上有落点（Claude 的 signature 字段）。
//     挂在 functionCall part 上的签名（Gemini 3 函数调用）无处安放，只能丢弃；
//     且客户端回传的 signature 在请求侧 Claude→OpenAI→Gemini 的 chat 中间格式里
//     同样没有字段承载，故本方向的签名往返目前不闭合。

// handleNonStreamToClaude 将 Gemini 非流式响应转换为 Claude Messages 格式
func (a *Adaptor) handleNonStreamToClaude(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 不写响应：Claude 入站由上层 WriteClaudeRelayError 统一写入，避免双重写入与重试时的响应污染
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

	claudeResp := geminiToClaudeResponse(&geminiResp, info)

	respBody, _ := json.Marshal(claudeResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	// 计费用量仍走 Gemini 口径（candidates+thoughts 合计、prompt 含 cached），
	// 与本适配器其他出站路径一致；上面写给客户端的是 Claude 协议口径。
	if geminiResp.UsageMetadata != nil {
		return geminiUsageToCommon(geminiResp.UsageMetadata), nil
	}
	return &common.Usage{}, nil
}

// handleStreamToClaude 将 Gemini 流式响应转换为 Claude Messages SSE 格式
func (a *Adaptor) handleStreamToClaude(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 不写响应：交上层 WriteClaudeRelayError 统一写入，避免双重写入与重试时的响应污染
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	isCA := a.isCodeAssistActive()
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	msgID := fmt.Sprintf("msg_%s", info.RequestID)
	modelName := info.OriginModelName

	var (
		totalUsage         dto.GeminiUsageMetadata
		startSent          bool
		finishReason       string
		hasToolUse         bool
		contentIndex       int
		currentBlockType   string
		toolIdx            int
		transferredTextLen int // 已转发的文本/思考内容长度，供流中断输出估算
	)

	// closeBlock 关闭当前 content block 并推进索引
	closeBlock := func() {
		if currentBlockType == "" {
			return
		}
		writeClaudeEvent(ctx, writer, "content_block_stop", &dto.ClaudeResponse{
			Type:  "content_block_stop",
			Index: intPtr(contentIndex),
		})
		contentIndex++
		currentBlockType = ""
	}

	// openTextLikeBlock 开启 text/thinking 块；同类型连续增量复用当前块
	openTextLikeBlock := func(blockType string, block *dto.ClaudeContentBlock) {
		if currentBlockType == blockType {
			return
		}
		closeBlock()
		writeClaudeEvent(ctx, writer, "content_block_start", &dto.ClaudeResponse{
			Type:         "content_block_start",
			Index:        intPtr(contentIndex),
			ContentBlock: block,
		})
		currentBlockType = blockType
	}

	// emitFinal 发送 message_delta（stop_reason + 最终用量）与 message_stop
	emitFinal := func() {
		closeBlock()

		stopReason := common.GeminiFinishReasonToClaude(finishReason)
		if hasToolUse {
			// Gemini 带 functionCall 时 finishReason 仍是 STOP，Claude 协议要求 tool_use，
			// 否则客户端不会进入工具调用回合
			stopReason = common.ClaudeToolUse
		}

		writeClaudeEvent(ctx, writer, "message_delta", &dto.ClaudeResponse{
			Type:  "message_delta",
			Delta: &dto.ClaudeDelta{StopReason: strPtr(stopReason)},
			Usage: claudeUsageFromGemini(&totalUsage),
		})
		writeClaudeEvent(ctx, writer, "message_stop", &dto.ClaudeResponse{Type: "message_stop"})
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断：Gemini 每个 chunk 携带累计 usage，缺失部分按已转发文本与请求侧估算补齐
			interruptedUsage := geminiUsageToCommon(&totalUsage)
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

		var geminiResp dto.GeminiChatResponse
		rawData := []byte(data)
		if isCA {
			rawData = unwrapCodeAssistData(rawData)
		}
		if err := json.Unmarshal(rawData, &geminiResp); err != nil {
			continue
		}

		if geminiResp.UsageMetadata != nil {
			totalUsage = *geminiResp.UsageMetadata
		}
		if geminiResp.ModelName != "" {
			modelName = geminiResp.ModelName
		}

		// 检查 promptFeedback.blockReason（流式安全过滤）
		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			// SSE 头已发送，必须补齐 Claude 的收尾事件，否则客户端挂起等 message_stop
			finishReason = "SAFETY"
			emitFinal()
			return nil, constant.NewRequestError(
				fmt.Sprintf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason), nil,
			)
		}

		// message_start：首个有效 chunk 时发送。Gemini 的 usageMetadata 逐块累计，
		// 首块的 promptTokenCount 已可用，直接带上
		if !startSent {
			writeClaudeEvent(ctx, writer, "message_start", &dto.ClaudeResponse{
				Type: "message_start",
				Message: &dto.ClaudeMessageInfo{
					ID:           msgID,
					Type:         "message",
					Role:         "assistant",
					Content:      []dto.ClaudeContentBlock{},
					Model:        modelName,
					StopReason:   nil,
					StopSequence: nil,
					Usage: &dto.ClaudeUsage{
						InputTokens:  claudeInputTokens(&totalUsage),
						OutputTokens: 0,
					},
				},
			})
			startSent = true
		}

		for _, candidate := range geminiResp.Candidates {
			if candidate.FinishReason != "" {
				finishReason = candidate.FinishReason
			}
			if candidate.Content == nil {
				continue
			}

			for i := range candidate.Content.Parts {
				part := &candidate.Content.Parts[i]
				isThought := part.Thought != nil && *part.Thought

				// 文本 / 思考增量
				if part.Text != "" {
					transferredTextLen += len(part.Text)
					if isThought {
						openTextLikeBlock("thinking", &dto.ClaudeContentBlock{
							Type:     "thinking",
							Thinking: strPtr(""),
						})
						writeClaudeEvent(ctx, writer, "content_block_delta", &dto.ClaudeResponse{
							Type:  "content_block_delta",
							Index: intPtr(contentIndex),
							Delta: &dto.ClaudeDelta{Type: "thinking_delta", Thinking: strPtr(part.Text)},
						})
					} else {
						openTextLikeBlock("text", &dto.ClaudeContentBlock{
							Type: "text",
							Text: strPtr(""),
						})
						writeClaudeEvent(ctx, writer, "content_block_delta", &dto.ClaudeResponse{
							Type:  "content_block_delta",
							Index: intPtr(contentIndex),
							Delta: &dto.ClaudeDelta{Type: "text_delta", Text: strPtr(part.Text)},
						})
					}
				}

				// thoughtSignature 与 Claude 的 thinking.signature 同为对客户端不透明的载体，
				// 原样搬运；仅在 thinking 块内有意义
				if part.ThoughtSignature != "" && currentBlockType == "thinking" {
					writeClaudeEvent(ctx, writer, "content_block_delta", &dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{Type: "signature_delta", Signature: part.ThoughtSignature},
					})
				}

				// 工具调用：Gemini 一次给全量参数，不做增量拼接
				if part.FunctionCall != nil {
					closeBlock()
					writeClaudeEvent(ctx, writer, "content_block_start", &dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: intPtr(contentIndex),
						ContentBlock: &dto.ClaudeContentBlock{
							Type:  "tool_use",
							ID:    claudeToolUseID(info, toolIdx),
							Name:  part.FunctionCall.FunctionName,
							Input: map[string]any{},
						},
					})
					currentBlockType = "tool_use"
					toolIdx++
					hasToolUse = true

					argsJSON, err := json.Marshal(geminiFunctionArgs(part.FunctionCall))
					if err != nil {
						argsJSON = []byte("{}")
					}
					writeClaudeEvent(ctx, writer, "content_block_delta", &dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{Type: "input_json_delta", PartialJSON: strPtr(string(argsJSON))},
					})
					closeBlock()
				}

				// Claude assistant 内容块无对应类型，按既有 Gemini→OpenAI 口径渲染为文本
				if fallback := geminiPartFallbackText(part); fallback != "" {
					transferredTextLen += len(fallback)
					openTextLikeBlock("text", &dto.ClaudeContentBlock{Type: "text", Text: strPtr("")})
					writeClaudeEvent(ctx, writer, "content_block_delta", &dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{Type: "text_delta", Text: strPtr(fallback)},
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		// 读流出错：带上已累计的 usage，避免整段按零值计费漏计
		return geminiUsageToCommon(&totalUsage), fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游一个有效 chunk 都没给：仍需补齐 message_start，否则客户端拿不到完整事件序列
	if !startSent {
		writeClaudeEvent(ctx, writer, "message_start", &dto.ClaudeResponse{
			Type: "message_start",
			Message: &dto.ClaudeMessageInfo{
				ID:      msgID,
				Type:    "message",
				Role:    "assistant",
				Content: []dto.ClaudeContentBlock{},
				Model:   modelName,
				Usage:   &dto.ClaudeUsage{},
			},
		})
	}

	emitFinal()
	return geminiUsageToCommon(&totalUsage), nil
}

// geminiToClaudeResponse 将 Gemini 非流式响应转换为 Claude Messages 响应。
// 内容块顺序保持 Gemini parts 的原始顺序（模型已按语义排好，重排会改变思考与正文的先后）。
func geminiToClaudeResponse(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) dto.ClaudeResponse {
	content := make([]dto.ClaudeContentBlock, 0)
	var (
		finishReason string
		hasToolUse   bool
		toolIdx      int
	)

	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		finishReason = candidate.FinishReason

		if candidate.Content != nil {
			for i := range candidate.Content.Parts {
				part := &candidate.Content.Parts[i]
				isThought := part.Thought != nil && *part.Thought

				if part.Text != "" {
					if isThought {
						content = append(content, dto.ClaudeContentBlock{
							Type:      "thinking",
							Thinking:  strPtr(part.Text),
							Signature: part.ThoughtSignature,
						})
					} else {
						content = append(content, dto.ClaudeContentBlock{
							Type: "text",
							Text: strPtr(part.Text),
						})
					}
				}

				if part.FunctionCall != nil {
					content = append(content, dto.ClaudeContentBlock{
						Type:  "tool_use",
						ID:    claudeToolUseID(info, toolIdx),
						Name:  part.FunctionCall.FunctionName,
						Input: geminiFunctionArgs(part.FunctionCall),
					})
					toolIdx++
					hasToolUse = true
				}

				if fallback := geminiPartFallbackText(part); fallback != "" {
					content = append(content, dto.ClaudeContentBlock{
						Type: "text",
						Text: strPtr(fallback),
					})
				}
			}
		}
	}

	// Claude 协议要求 content 非空
	if len(content) == 0 {
		content = append(content, dto.ClaudeContentBlock{Type: "text", Text: strPtr("")})
	}

	stopReason := common.GeminiFinishReasonToClaude(finishReason)
	if hasToolUse {
		// Gemini 带 functionCall 时 finishReason 仍是 STOP，Claude 协议要求 tool_use
		stopReason = common.ClaudeToolUse
	}

	modelName := geminiResp.ModelName
	if modelName == "" || info.ChannelMeta.IsModelMapped {
		modelName = info.OriginModelName
	}

	return dto.ClaudeResponse{
		ID:           fmt.Sprintf("msg_%s", info.RequestID),
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		StopReason:   stopReason,
		StopSequence: nil,
		Model:        modelName,
		Usage:        claudeUsageFromGemini(geminiResp.UsageMetadata),
	}
}

// claudeUsageFromGemini 按 Claude 协议口径换算用量（客户端可见值，非计费值）。
//   - Claude 的 input_tokens 不含缓存命中，Gemini 的 promptTokenCount 已含 cachedContentTokenCount，需扣减
//   - Claude 的 output_tokens 含思考，Gemini 的 candidatesTokenCount 不含 thoughtsTokenCount，需合计
func claudeUsageFromGemini(um *dto.GeminiUsageMetadata) *dto.ClaudeUsage {
	if um == nil {
		return &dto.ClaudeUsage{}
	}
	return &dto.ClaudeUsage{
		InputTokens:          claudeInputTokens(um),
		OutputTokens:         um.CandidatesTokenCount + um.ThoughtsTokenCount,
		CacheReadInputTokens: um.CachedContentTokenCount,
	}
}

// claudeInputTokens 计算 Claude 口径的 input_tokens（扣除缓存命中部分）
func claudeInputTokens(um *dto.GeminiUsageMetadata) int {
	if um == nil {
		return 0
	}
	input := um.PromptTokenCount - um.CachedContentTokenCount
	if input < 0 {
		return 0
	}
	return input
}

// geminiFunctionArgs 提取 functionCall 参数，nil 时回退为空对象
// （Claude 的 tool_use.input 不允许为 null）
func geminiFunctionArgs(fc *dto.GeminiFunctionCall) any {
	if fc == nil || fc.Arguments == nil {
		return map[string]any{}
	}
	return fc.Arguments
}

// claudeToolUseID 为 Gemini functionCall 合成 Claude tool_use ID。
// Gemini 协议不带调用 ID，客户端回传的 tool_result.tool_use_id 在请求侧
// （ConvertClaudeToGemini → OpenAI → Gemini）按函数名还原，故此处只需
// 在单次响应内唯一且可被客户端原样回传。
func claudeToolUseID(info *common.RelayInfo, idx int) string {
	return fmt.Sprintf("toolu_%s_%d", info.RequestID, idx)
}

// geminiPartFallbackText 将 Claude 无对应内容块类型的 Gemini part 渲染为文本。
// 渲染口径与 Gemini→OpenAI 出站一致（response.go 的 handleStreamToOpenAI）。
func geminiPartFallbackText(part *dto.GeminiPart) string {
	switch {
	case part.InlineData != nil:
		return fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data)
	case part.ExecutableCode != nil:
		return fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code)
	case part.CodeExecutionResult != nil:
		return fmt.Sprintf("Execution %s:\n%s", part.CodeExecutionResult.Outcome, part.CodeExecutionResult.Output)
	case part.FileData != nil:
		return fmt.Sprintf("[file](%s)", part.FileData.FileURI)
	}
	return ""
}

// writeClaudeEvent 写入 Claude 格式的 SSE 事件（池化缓冲，整帧单次写出）。
// 序列化失败时跳过该事件并记日志：写出 data 为空的事件会让客户端 JSON.parse 抛错并中止请求。
func writeClaudeEvent(ctx context.Context, w http.ResponseWriter, eventType string, data any) {
	if err := helper.WriteSSEEventJSON(w, eventType, data); errors.Is(err, helper.ErrSSEPayloadMarshal) {
		g.Log().Errorf(ctx, "[Gemini→Claude] marshal %s event failed, event skipped: %v", eventType, err)
	}
}

// intPtr 返回 int 的指针
func intPtr(v int) *int { return &v }

// strPtr 返回 string 的指针
func strPtr(v string) *string { return &v }
