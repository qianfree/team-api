package shared

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// 服务端联网搜索的跨协议中间表示：探测与产出两侧都要覆盖，
// 任一侧失准都会让能力在转换中静默丢失（客户端要求联网、却以没搜索的方式成功返回）。

func TestDetectWebSearchFromOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		opts     string
		wantNil  bool
		wantSize string
	}{
		{"空对象即启用", `{}`, false, ""},
		{"带上下文规模", `{"search_context_size":"high"}`, false, "high"},
		{"带位置", `{"user_location":{"type":"approximate","approximate":{"city":"SH"}}}`, false, ""},
		{"字段缺失", ``, true, ""},
		{"显式 null", `null`, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectWebSearchFromOpenAI(json.RawMessage(tt.opts))
			if tt.wantNil {
				if got != nil {
					t.Fatalf("期望未探测到搜索，got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("应探测到搜索能力")
			}
			if got.SearchContextSize != tt.wantSize {
				t.Errorf("SearchContextSize = %q, want %q", got.SearchContextSize, tt.wantSize)
			}
		})
	}
}

func TestDetectWebSearchFromClaudeTools(t *testing.T) {
	maxUses := 5
	tools := []dto.ClaudeTool{
		{Name: "get_weather", InputSchema: map[string]any{}},
		{Type: "web_search_20250305", Name: "web_search", MaxUses: &maxUses, AllowedDomains: []string{"example.com"}},
	}
	got := DetectWebSearchFromClaudeTools(tools)
	if got == nil {
		t.Fatal("应在混合工具列表中探测到 web_search")
	}
	if got.MaxUses == nil || *got.MaxUses != 5 {
		t.Errorf("MaxUses 丢失: %v", got.MaxUses)
	}
	if len(got.AllowedDomains) != 1 || got.AllowedDomains[0] != "example.com" {
		t.Errorf("AllowedDomains 丢失: %v", got.AllowedDomains)
	}

	// 仅自定义函数时不得误报
	if DetectWebSearchFromClaudeTools([]dto.ClaudeTool{{Name: "f"}}) != nil {
		t.Error("纯自定义函数不应被识别为搜索")
	}
	// 其他内置工具不得误报
	if DetectWebSearchFromClaudeTools([]dto.ClaudeTool{{Type: "code_execution_20250522", Name: "code"}}) != nil {
		t.Error("code_execution 不应被识别为搜索")
	}
}

func TestDetectWebSearchFromGeminiTools(t *testing.T) {
	cases := []struct {
		name  string
		tools string
		want  bool
	}{
		{"googleSearch", `[{"googleSearch":{}}]`, true},
		{"旧版 googleSearchRetrieval", `[{"googleSearchRetrieval":{}}]`, true},
		{"与函数声明共存", `[{"functionDeclarations":[{"name":"f"}]},{"googleSearch":{}}]`, true},
		{"仅函数声明", `[{"functionDeclarations":[{"name":"f"}]}]`, false},
		{"空", ``, false},
		{"畸形 JSON", `{not json`, false},
		// 对象形态（非数组）不是合法 Gemini tools，不应误判
		{"对象形态", `{"googleSearch":{}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectWebSearchFromGeminiTools(json.RawMessage(tc.tools)) != nil
			if got != tc.want {
				t.Errorf("探测结果 = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectWebSearchFromResponsesTools(t *testing.T) {
	cases := []struct {
		name  string
		tools string
		want  bool
	}{
		{"web_search", `[{"type":"web_search"}]`, true},
		{"web_search_preview 变体", `[{"type":"web_search_preview"}]`, true},
		{"与函数共存", `[{"type":"function","name":"f"},{"type":"web_search"}]`, true},
		{"仅函数", `[{"type":"function","name":"f"}]`, false},
		{"空", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectWebSearchFromResponsesTools(json.RawMessage(tc.tools)) != nil
			if got != tc.want {
				t.Errorf("探测结果 = %v, want %v", got, tc.want)
			}
		})
	}

	spec := DetectWebSearchFromResponsesTools(json.RawMessage(`[{"type":"web_search","search_context_size":"low"}]`))
	if spec == nil || spec.SearchContextSize != "low" {
		t.Errorf("search_context_size 未提取: %+v", spec)
	}
}

// TestWebSearchSpec_RoundTripAcrossFormats 跨格式往返：任一格式产出的构件
// 再被对应的探测器识别，能力都不得丢失——这是两跳链（经 chat 中枢）能保住搜索的前提。
func TestWebSearchSpec_RoundTripAcrossFormats(t *testing.T) {
	maxUses := 3
	origin := &WebSearchSpec{MaxUses: &maxUses, SearchContextSize: "high"}

	// → chat → 回探测
	chatOpts := origin.ToOpenAIOptions()
	back := DetectWebSearchFromOpenAI(chatOpts)
	if back == nil {
		t.Fatal("chat 往返后能力丢失")
	}
	if back.SearchContextSize != "high" {
		t.Errorf("chat 往返丢失 search_context_size: %+v", back)
	}

	// → Claude → 回探测
	claudeTool := origin.ToClaudeTool()
	back = DetectWebSearchFromClaudeTools([]dto.ClaudeTool{claudeTool})
	if back == nil {
		t.Fatal("Claude 往返后能力丢失")
	}
	if back.MaxUses == nil || *back.MaxUses != 3 {
		t.Errorf("Claude 往返丢失 max_uses: %+v", back.MaxUses)
	}

	// → Responses → 回探测
	respToolRaw, err := json.Marshal([]any{origin.ToResponsesTool()})
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	if DetectWebSearchFromResponsesTools(respToolRaw) == nil {
		t.Fatal("Responses 往返后能力丢失")
	}

	// → Gemini → 回探测（Gemini 侧无可承载的约束字段，只保能力本身）
	geminiRaw, err := json.Marshal([]any{GeminiGoogleSearchEntry()})
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	if DetectWebSearchFromGeminiTools(geminiRaw) == nil {
		t.Fatal("Gemini 往返后能力丢失")
	}
}

// TestWebSearchSpec_ToOpenAIOptions_EmptyIsEnabled 无约束时必须产出空对象而非 null：
// OpenAI 的约定是「字段存在即启用」，产出 null 等于没启用。
func TestWebSearchSpec_ToOpenAIOptions_EmptyIsEnabled(t *testing.T) {
	opts := (&WebSearchSpec{}).ToOpenAIOptions()
	if string(opts) != `{}` {
		t.Errorf("空 spec 应产出 {}, got %s", opts)
	}
	if DetectWebSearchFromOpenAI(opts) == nil {
		t.Error("产出的 options 应能被自身探测器识别为启用")
	}
	// nil 接收者产出 nil（调用方据此判断不注入字段）
	var nilSpec *WebSearchSpec
	if nilSpec.ToOpenAIOptions() != nil {
		t.Error("nil spec 应产出 nil")
	}
}
