package ali

import (
	"encoding/json"
	"testing"
)

func TestIsQwenThinkingModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"qwen-plus", "qwen-plus", true},
		{"qwen-max", "qwen-max", true},
		{"qwq-32b", "qwq-32b", true},
		{"HF path Qwen", "Qwen/Qwen3-235B-A22B-Thinking-2507", true},
		{"HF path Qwq", "Qwen/QwQ-32B", true},
		{"case insensitive", "QWEN-Plus", true},
		{"deepseek-r1", "deepseek-r1", false},
		{"claude", "claude-3-5-sonnet", false},
		{"empty", "", false},
		{"whitespace only", "   ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isQwenThinkingModel(tt.model); got != tt.want {
				t.Errorf("isQwenThinkingModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// TestConvertRequest_ThinkingBudgetStripping 验证 thinking_budget 仅在
// 上游为 Qwen/Qwq 思考模型时透传，其余情况剥离；显式 0 必须保留。
// 场景对齐 new-api #5836。
func TestConvertRequest_ThinkingBudgetStripping(t *testing.T) {
	tests := []struct {
		name             string
		upstreamModel    string // 传入 convertRequest 的映射后上游模型名
		bodyModel        string // 请求体内的 model 字段（upstream 为空时作为回退判据）
		thinkingBudget   string // thinking_budget 的原始 JSON 值，"" 表示省略
		wantBudgetExists bool
		wantBudgetValue  string // 存在时的期望原始值
	}{
		{
			name:             "qwen upstream keeps budget",
			upstreamModel:    "qwen-plus",
			bodyModel:        "qwen-plus",
			thinkingBudget:   "128",
			wantBudgetExists: true,
			wantBudgetValue:  "128",
		},
		{
			name:             "qwq explicit zero preserved",
			upstreamModel:    "qwq-32b",
			bodyModel:        "qwq-32b",
			thinkingBudget:   "0",
			wantBudgetExists: true,
			wantBudgetValue:  "0",
		},
		{
			name:             "non-qwen upstream strips budget",
			upstreamModel:    "deepseek-r1",
			bodyModel:        "qwen-plus",
			thinkingBudget:   "128",
			wantBudgetExists: false,
		},
		{
			name:             "empty upstream falls back to qwen body model keeps",
			upstreamModel:    "",
			bodyModel:        "qwen-max",
			thinkingBudget:   "256",
			wantBudgetExists: true,
			wantBudgetValue:  "256",
		},
		{
			name:             "empty upstream falls back to non-qwen body model strips",
			upstreamModel:    "",
			bodyModel:        "deepseek-r1",
			thinkingBudget:   "256",
			wantBudgetExists: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]json.RawMessage{
				"model":           json.RawMessage(`"` + tt.bodyModel + `"`),
				"enable_thinking": json.RawMessage(`true`), // 验证 enable_thinking 永不被剥离
			}
			if tt.thinkingBudget != "" {
				body["thinking_budget"] = json.RawMessage(tt.thinkingBudget)
			}

			requestBody, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}

			converted, err := convertRequest(requestBody, tt.upstreamModel)
			if err != nil {
				t.Fatalf("convertRequest: %v", err)
			}

			var out map[string]json.RawMessage
			if err := json.Unmarshal(converted, &out); err != nil {
				t.Fatalf("unmarshal converted: %v", err)
			}

			budgetRaw, hasBudget := out["thinking_budget"]
			if hasBudget != tt.wantBudgetExists {
				t.Errorf("thinking_budget exists = %v, want %v (raw=%q)", hasBudget, tt.wantBudgetExists, string(budgetRaw))
			}
			if tt.wantBudgetExists && string(budgetRaw) != tt.wantBudgetValue {
				t.Errorf("thinking_budget = %q, want %q", string(budgetRaw), tt.wantBudgetValue)
			}

			// enable_thinking 不应被剥离（仅 thinking_budget 剥离）
			if _, ok := out["enable_thinking"]; !ok {
				t.Errorf("enable_thinking was unexpectedly stripped")
			}
		})
	}
}

// TestConvertRequest_TopPCappingRegression 确保新增 thinking_budget 剥离逻辑
// 不影响既有的 top_p 裁剪行为。
func TestConvertRequest_TopPCappingRegression(t *testing.T) {
	body := map[string]json.RawMessage{
		"model":          json.RawMessage(`"qwen-plus"`),
		"top_p":          json.RawMessage(`1.5`),
		"thinking_budget": json.RawMessage(`128`),
	}
	requestBody, _ := json.Marshal(body)

	converted, err := convertRequest(requestBody, "qwen-plus")
	if err != nil {
		t.Fatalf("convertRequest: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(converted, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var topP float64
	if err := json.Unmarshal(out["top_p"], &topP); err != nil {
		t.Fatalf("unmarshal top_p: %v", err)
	}
	if topP != 0.999 {
		t.Errorf("top_p = %v, want 0.999", topP)
	}
}
