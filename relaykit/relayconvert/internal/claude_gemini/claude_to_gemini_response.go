package claude_gemini

import (
	"context"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ClaudeToGeminiResponseConverter 将 Claude Messages 非流式响应转换为 Gemini GenerateContent 响应。
//
// 协议落差（转换时按下列口径处理）：
//   - Claude 的 output_tokens 含思考 token 且不单独拆分，Gemini 的 candidatesTokenCount
//     语义上不含 thoughts → 无法还原拆分，thoughtsTokenCount 记 0、全部计入 candidates。
//   - redacted_thinking 是 Claude 的加密思考块，Gemini 无对应物 → 丢弃。
//   - thinking.signature 搬到 Gemini 的 thoughtSignature 上（单向出站）。客户端回传的
//     签名在请求侧 Gemini→OpenAI→Claude 的 chat 中间格式里没有字段承载，故本方向的
//     签名往返不闭合；这是已知取舍，不是遗漏。
type ClaudeToGeminiResponseConverter struct{}

func (c *ClaudeToGeminiResponseConverter) ID() string {
	return relayconvert.ResponseConverterClaudeMessagesToGeminiChat
}

func (c *ClaudeToGeminiResponseConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToGeminiResponseConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *ClaudeToGeminiResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 *dto.ClaudeResponse 转换为 *dto.GeminiChatResponse。
func (c *ClaudeToGeminiResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	claudeResp, ok := response.(*dto.ClaudeResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ClaudeResponse, got %T", response)
	}
	return claudeToGeminiResponse(claudeResp, info), nil
}

// claudeToGeminiResponse 将 Claude 非流式响应转换为 Gemini 响应。
// 内容块顺序保持 Claude 的原始顺序（模型已按语义排好 thinking / 正文 / 工具调用的先后）。
func claudeToGeminiResponse(claudeResp *dto.ClaudeResponse, info convmeta.Meta) *dto.GeminiChatResponse {
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
					// Gemini 的 functionCall 支持可选 id，直接搬运 Claude 的 tool_use.id
					ID:           block.ID,
					FunctionName: block.Name,
					Arguments:    claudeToolInput(block.Input),
				},
			})
		}
	}

	modelName := claudeResp.Model
	if modelName == "" || isModelMapped(info) {
		modelName = originModelName(info)
	}

	resp := &dto.GeminiChatResponse{
		ModelName: modelName,
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model", Parts: parts},
			FinishReason: claudeStopReasonToGemini(claudeResp.StopReason),
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
