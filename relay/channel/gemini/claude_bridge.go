package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ========== Claude 入站桥接：Gemini → Claude Messages ==========
//
// 请求侧由 relaykit 的步骤链完成（Claude → OpenAI 中枢 → Gemini），
// 响应侧同样由 relaykit 转换器完成（Gemini → Claude，含 SSE 事件序列产出）。
// 本文件只保留宿主侧接线：HTTP 状态码/安全过滤错误处理、响应写出与计费用量提取，
// 以及 Code Assist 强制流式聚合路径复用的非流式装配函数（geminiToClaudeResponse 及其助手，
// 由 adaptor.go 的 handleCodeAssistAggregatedStream 调用）。
//
// 协议落差（functionCall 无调用 ID、inlineData 等无对应块类型、thoughtSignature 往返）
// 的处理口径已随转换逻辑迁入 relaykit（relaykit/relayconvert/internal/claude_gemini），
// 此处保留的聚合装配函数沿用同一套约定。
//
// 计费口径：写给客户端的是 Claude 协议口径的 usage（由转换器产出），
// 返回给上层计费的是 Gemini 原生口径（candidates+thoughts 合计、prompt 含 cached），
// 因此非流式路径仍从**原始上游响应体**提取 usage。

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

	// 安全过滤（promptFeedback.blockReason）属上游错误语义，先于协议转换判定
	var geminiResp dto.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}
	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		return nil, constant.NewRequestError(
			fmt.Sprintf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason), nil,
		)
	}

	// relaykit 响应转换（唯一路径，hard-fail：解析/转换失败即向上返回错误，不再回退旧实现）
	convertedBody, _, handled, convErr := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !handled {
		return nil, constant.NewChannelError("gemini adaptor: no relaykit converter for claude client response", nil)
	}
	if convErr != nil {
		return nil, relaykit_bridge.ResponseConvertError(convErr, resp.StatusCode, resp.Header)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

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

	// relaykit 流式转换（唯一路径，hard-fail）：写入前失败由桥接层返回未接管，此处显式报错
	if usage, ok, err := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer); ok {
		// 桥接层已写出 SSE 并完成收尾裁决：err 非空即流以错误/客户端中断结束
		//（已带 ResponseWritten，上层只记账与上报调度，不重写响应体、不换渠道重试），
		// usage 中断兜底亦已在桥接层完成，此处原样上抛即可。
		return usage, err
	}
	return nil, constant.NewChannelError("gemini adaptor: relaykit stream converter unavailable for claude client", nil)
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
// （relaykit 步骤链 Claude → OpenAI → Gemini）按函数名还原，故此处只需
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

// intPtr 返回 int 的指针
func intPtr(v int) *int { return &v }

// strPtr 返回 string 的指针
func strPtr(v string) *string { return &v }
