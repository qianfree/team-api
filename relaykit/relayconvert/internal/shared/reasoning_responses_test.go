package shared

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// Responses 思考内容的构造与提取：任一侧失准都会让推理模型的思考过程在转换中静默消失。

func TestBuildResponsesReasoningOutput(t *testing.T) {
	item := BuildResponsesReasoningOutput("abc", "让我想想")
	if item == nil {
		t.Fatal("非空思考内容应产出 reasoning 项")
	}
	if item.Type != "reasoning" {
		t.Errorf("Type = %q, want \"reasoning\"", item.Type)
	}
	if item.ID != "rs_abc" {
		t.Errorf("ID = %q, want \"rs_abc\"（id 须由 seed 稳定派生，金样本比对依赖此性质）", item.ID)
	}
	if len(item.Summary) != 1 || item.Summary[0].Type != "summary_text" || item.Summary[0].Text != "让我想想" {
		t.Errorf("Summary 形态不符: %+v", item.Summary)
	}

	// 空思考内容不得产出空壳
	if BuildResponsesReasoningOutput("abc", "") != nil {
		t.Error("空思考内容应返回 nil，避免产出空 reasoning 壳")
	}
}

func TestExtractResponsesReasoning(t *testing.T) {
	tests := []struct {
		name    string
		outputs []dto.ResponsesOutput
		want    string
	}{
		{
			name: "单个 reasoning 项",
			outputs: []dto.ResponsesOutput{
				{Type: "reasoning", Summary: []dto.ResponsesSummaryPart{{Type: "summary_text", Text: "思考"}}},
				{Type: "message"},
			},
			want: "思考",
		},
		{
			name: "多段 summary 按序拼接",
			outputs: []dto.ResponsesOutput{
				{Type: "reasoning", Summary: []dto.ResponsesSummaryPart{
					{Type: "summary_text", Text: "第一段"},
					{Type: "summary_text", Text: "第二段"},
				}},
			},
			want: "第一段第二段",
		},
		{
			name: "多个 reasoning 项按序拼接",
			outputs: []dto.ResponsesOutput{
				{Type: "reasoning", Summary: []dto.ResponsesSummaryPart{{Text: "A"}}},
				{Type: "message"},
				{Type: "reasoning", Summary: []dto.ResponsesSummaryPart{{Text: "B"}}},
			},
			want: "AB",
		},
		{name: "无 reasoning 项", outputs: []dto.ResponsesOutput{{Type: "message"}}, want: ""},
		{name: "空 summary", outputs: []dto.ResponsesOutput{{Type: "reasoning"}}, want: ""},
		{name: "空输出", outputs: nil, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractResponsesReasoning(tt.outputs); got != tt.want {
				t.Errorf("ExtractResponsesReasoning() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractClaudeThinking(t *testing.T) {
	think := "想一想"
	empty := ""
	blocks := []dto.ClaudeContentBlock{
		{Type: "thinking", Thinking: &think},
		{Type: "redacted_thinking", Thinking: &empty},
		{Type: "text"},
	}
	if got := ExtractClaudeThinking(blocks); got != "想一想" {
		t.Errorf("ExtractClaudeThinking() = %q, want %q", got, "想一想")
	}
	// 无 thinking 块
	if got := ExtractClaudeThinking([]dto.ClaudeContentBlock{{Type: "text"}}); got != "" {
		t.Errorf("无思考块应返回空串, got %q", got)
	}
	// nil Thinking 指针不得 panic
	if got := ExtractClaudeThinking([]dto.ClaudeContentBlock{{Type: "thinking"}}); got != "" {
		t.Errorf("nil Thinking 应返回空串, got %q", got)
	}
}

// TestResponsesReasoning_RoundTrip 构造后再提取必须还原原文——
// 这是「chat reasoning_content ↔ Responses reasoning」双向转换不丢内容的前提。
func TestResponsesReasoning_RoundTrip(t *testing.T) {
	const original = "第一步：分析问题\n第二步：得出结论"
	item := BuildResponsesReasoningOutput("seed", original)
	if item == nil {
		t.Fatal("构造失败")
	}
	if got := ExtractResponsesReasoning([]dto.ResponsesOutput{*item}); got != original {
		t.Errorf("往返后思考内容变化:\n got=%q\nwant=%q", got, original)
	}
}
