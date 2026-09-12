package shared

import (
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
)

// Responses 协议的「思考内容」跨协议映射。
//
// 四种协议表达模型思考过程的方式各不相同：
//
//	OpenAI chat  message.reasoning_content（字符串）
//	Claude       content 块 {type: "thinking", thinking: "...", signature: "..."}
//	Gemini       part {text: "...", thought: true}
//	Responses    输出项 {type: "reasoning", id: "rs_x", summary: [{type: "summary_text", text: "..."}]}
//
// 非流式的 Responses 方向此前一律跳过思考内容，理由注释写的是「Responses 非流式
// 无对应物」——但这是误判：reasoning 输出项本身就是非流式形态（流式侧的
// response.reasoning_summary_text.delta 只是它的增量表达）。于是推理模型经这些方向
// 转换后，客户端完全看不到思考过程，且**看不出丢过东西**。

// responsesReasoningItemType Responses 输出项中思考项的 type 值。
const responsesReasoningItemType = "reasoning"

// responsesSummaryTextType reasoning.summary 数组元素的 type 值。
const responsesSummaryTextType = "summary_text"

// BuildResponsesReasoningOutput 把一段思考文本构造为 Responses 的 reasoning 输出项。
// thinking 为空时返回 nil（调用方据此不追加输出项，避免产出空 reasoning 壳）。
//
// idSeed 用于生成稳定的项 id：同一响应内多次调用应传入同一 seed，
// 使 id 可复现（金样本比对依赖此性质）。
func BuildResponsesReasoningOutput(idSeed, thinking string) *dto.ResponsesOutput {
	if thinking == "" {
		return nil
	}
	return &dto.ResponsesOutput{
		Type: responsesReasoningItemType,
		ID:   fmt.Sprintf("rs_%s", idSeed),
		Summary: []dto.ResponsesSummaryPart{{
			Type: responsesSummaryTextType,
			Text: thinking,
		}},
	}
}

// ExtractResponsesReasoning 从 Responses 输出项中提取思考文本，
// 多个 reasoning 项与多段 summary 按顺序拼接；无思考内容返回空串。
func ExtractResponsesReasoning(outputs []dto.ResponsesOutput) string {
	var parts []string
	for _, out := range outputs {
		if out.Type != responsesReasoningItemType {
			continue
		}
		for _, s := range out.Summary {
			if s.Text != "" {
				parts = append(parts, s.Text)
			}
		}
	}
	return strings.Join(parts, "")
}

// ExtractClaudeThinking 从 Claude content 块中提取思考文本（thinking / redacted_thinking）。
func ExtractClaudeThinking(blocks []dto.ClaudeContentBlock) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "thinking", "redacted_thinking":
			if b.Thinking != nil && *b.Thinking != "" {
				parts = append(parts, *b.Thinking)
			}
		}
	}
	return strings.Join(parts, "")
}
