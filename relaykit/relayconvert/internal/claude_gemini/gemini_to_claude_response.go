package claude_gemini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// GeminiToClaudeResponseConverter 将 Gemini GenerateContent 非流式响应转换为 Claude Messages 响应。
//
// 协议落差（Gemini 侧无对应物，转换时按下列口径处理）：
//   - functionCall 没有调用 ID，Claude 的 tool_use 必须带 id → 本层合成，
//     客户端回传的 tool_result.tool_use_id 在请求侧按函数名还原。
//   - inlineData / executableCode / codeExecutionResult / fileData 在 Claude
//     的 assistant 内容块里没有对应类型 → 按既有 Gemini→OpenAI 口径渲染为文本，
//     信息保留而非丢弃。
//   - thoughtSignature 只在 thinking 块上有落点（Claude 的 signature 字段）。
//     挂在 functionCall part 上的签名（Gemini 3 函数调用）无处安放，只能丢弃；
//     且客户端回传的 signature 在请求侧 Claude→OpenAI→Gemini 的 chat 中间格式里
//     同样没有字段承载，故本方向的签名往返目前不闭合。
type GeminiToClaudeResponseConverter struct{}

func (c *GeminiToClaudeResponseConverter) ID() string {
	return relayconvert.ResponseConverterGeminiChatToClaudeMessages
}

func (c *GeminiToClaudeResponseConverter) From() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *GeminiToClaudeResponseConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *GeminiToClaudeResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 *dto.GeminiChatResponse 转换为 *dto.ClaudeResponse。
// promptFeedback.blockReason 命中（Gemini 安全过滤）时返回错误，与旧桥的请求错误分类一致。
func (c *GeminiToClaudeResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	geminiResp, ok := response.(*dto.GeminiChatResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeminiChatResponse, got %T", response)
	}

	// 检查 promptFeedback.blockReason（Gemini 安全过滤）
	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		return nil, fmt.Errorf("request blocked by Gemini safety filter: %s: %w", geminiResp.PromptFeedback.BlockReason, relayconvert.ErrContentBlocked)
	}

	return geminiToClaudeResponse(geminiResp, info), nil
}

// geminiToClaudeResponse 将 Gemini 非流式响应转换为 Claude Messages 响应。
// 内容块顺序保持 Gemini parts 的原始顺序（模型已按语义排好，重排会改变思考与正文的先后）。
func geminiToClaudeResponse(geminiResp *dto.GeminiChatResponse, info convmeta.Meta) *dto.ClaudeResponse {
	// 消息/工具 ID 基准：宿主 RequestID 未随 Meta 下沉，以纳秒时间戳保证单响应内唯一
	idBase := fmt.Sprintf("%d", time.Now().UnixNano())

	content := make([]dto.ClaudeContentBlock, 0)
	var (
		finishReason string
		hasToolUse   bool
		toolIdx      int
		citations    *shared.GroundingCitations
		answerText   strings.Builder
	)

	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		finishReason = candidate.FinishReason
		// 服务端搜索证据：上游真的去搜了，来源必须还原成 Claude 的搜索工具块，
		// 否则客户端只拿到一段被搜索增强的纯文本，看不到来源、无法核查
		citations = shared.ParseGeminiGrounding(candidate.GroundingMetadata)

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
						answerText.WriteString(part.Text)
						content = append(content, dto.ClaudeContentBlock{
							Type: "text",
							Text: strPtr(part.Text),
						})
					}
				}

				if part.FunctionCall != nil {
					content = append(content, dto.ClaudeContentBlock{
						Type:  "tool_use",
						ID:    claudeToolUseID(idBase, toolIdx),
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

	// 搜索块排在正文之前：Claude 的语义是「先发起搜索、拿到结果，再据此作答」
	if !citations.IsEmpty() {
		citations.AlignSupports(answerText.String())
		content = append(citations.ToClaudeSearchBlocks(idBase), content...)
	}

	// Claude 协议要求 content 非空
	if len(content) == 0 {
		content = append(content, dto.ClaudeContentBlock{Type: "text", Text: strPtr("")})
	}

	stopReason := geminiFinishReasonToClaude(finishReason)
	if hasToolUse {
		// Gemini 带 functionCall 时 finishReason 仍是 STOP，Claude 协议要求 tool_use
		stopReason = claudeToolUse
	}

	modelName := geminiResp.ModelName
	if modelName == "" || isModelMapped(info) {
		modelName = originModelName(info)
	}

	usage := claudeUsageFromGemini(geminiResp.UsageMetadata)
	// 搜索次数透出到 usage：Google 在 token 之外按搜索次数单独计价，
	// 不透出则计费层无从得知本次回答发起过几次搜索（定价挂接由宿主计费层完成）
	if n := citations.SearchRequestCount(); n > 0 {
		usage.ServerToolUse = &dto.ClaudeServerToolUsage{WebSearchRequests: n}
	}

	return &dto.ClaudeResponse{
		ID:           fmt.Sprintf("msg_%s", idBase),
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		StopReason:   stopReason,
		StopSequence: nil,
		Model:        modelName,
		Usage:        usage,
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
// 按函数名还原，故此处只需在单次响应内唯一且可被客户端原样回传。
func claudeToolUseID(idBase string, idx int) string {
	return fmt.Sprintf("toolu_%s_%d", idBase, idx)
}

// geminiPartFallbackText 将 Claude 无对应内容块类型的 Gemini part 渲染为文本。
// 渲染口径与 Gemini→OpenAI 出站一致。
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
