package shared

import (
	"encoding/json"
	"testing"
)

// geminiGroundingFixture 一份贴合真实形态的 groundingMetadata（含两个来源、
// 一个覆盖片段、两次查询，另含一个非 web 类型的 chunk 用于验证下标重映射）。
const geminiGroundingFixture = `{
	"searchEntryPoint": {"renderedContent": "<div>Google Search Suggestions</div>"},
	"webSearchQueries": ["weather boston", "boston forecast"],
	"groundingChunks": [
		{"web": {"uri": "https://weather.example/boston", "title": "weather.example"}},
		{"retrievedContext": {"title": "内部文档"}},
		{"web": {"uri": "https://forecast.example/ma", "title": "forecast.example"}}
	],
	"groundingSupports": [
		{
			"segment": {"startIndex": 0, "endIndex": 11, "text": "It is 68F in"},
			"groundingChunkIndices": [0, 2]
		}
	]
}`

func parseFixture(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("语料不是合法 JSON: %v", err)
	}
	return v
}

func TestParseGeminiGrounding(t *testing.T) {
	c := ParseGeminiGrounding(parseFixture(t, geminiGroundingFixture))
	if c.IsEmpty() {
		t.Fatal("应解析出引用证据")
	}
	if len(c.Sources) != 2 {
		t.Fatalf("Sources 数 = %d, want 2（非 web 类型 chunk 不产生来源）: %+v", len(c.Sources), c.Sources)
	}
	if c.Sources[0].URL != "https://weather.example/boston" || c.Sources[1].URL != "https://forecast.example/ma" {
		t.Errorf("来源顺序/内容不符: %+v", c.Sources)
	}
	if len(c.Queries) != 2 {
		t.Errorf("Queries = %v, want 2 条", c.Queries)
	}

	// 关键：chunk 下标 [0,2] 必须重映射为 Sources 下标 [0,1]。
	// 直接沿用原下标会让第二条引用指向不存在的 Sources[2]（越界或错位到别的来源）。
	if len(c.Supports) != 1 {
		t.Fatalf("Supports 数 = %d, want 1", len(c.Supports))
	}
	got := c.Supports[0].SourceIndices
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("chunk 下标未正确重映射: got %v, want [0 1]", got)
	}
}

func TestParseGeminiGrounding_NoSources(t *testing.T) {
	cases := map[string]string{
		"nil":           "",
		"空对象":           `{}`,
		"只有查询无来源":       `{"webSearchQueries":["q"]}`,
		"chunk 全为非 web": `{"groundingChunks":[{"retrievedContext":{"title":"x"}}]}`,
		"web 缺 uri":     `{"groundingChunks":[{"web":{"title":"x"}}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var meta any
			if raw != "" {
				meta = parseFixture(t, raw)
			}
			if c := ParseGeminiGrounding(meta); !c.IsEmpty() {
				t.Errorf("无可核查来源时应视为空, got %+v", c)
			}
		})
	}
}

func TestSearchRequestCount(t *testing.T) {
	c := ParseGeminiGrounding(parseFixture(t, geminiGroundingFixture))
	if got := c.SearchRequestCount(); got != 2 {
		t.Errorf("SearchRequestCount = %d, want 2（按 webSearchQueries 条数）", got)
	}
	// 有来源但无 queries：按 1 次计，不能算 0（否则按次计费漏记）
	noQueries := ParseGeminiGrounding(parseFixture(t, `{"groundingChunks":[{"web":{"uri":"https://a.example"}}]}`))
	if got := noQueries.SearchRequestCount(); got != 1 {
		t.Errorf("无 queries 但有来源时 = %d, want 1", got)
	}
	var empty *GroundingCitations
	if got := empty.SearchRequestCount(); got != 0 {
		t.Errorf("空证据 = %d, want 0", got)
	}
}

// TestAlignSupports 片段偏移校正：Gemini 的偏移相对上游原始文本，
// 转换后的正文可能不同（剥离思考 part、拼接多个 text part），
// 错位的偏移会让客户端把引用高亮在错误位置——比没有引用更糟。
func TestAlignSupports(t *testing.T) {
	t.Run("偏移已对齐则沿用", func(t *testing.T) {
		c := &GroundingCitations{
			Sources:  []GroundingSource{{URL: "https://a.example"}},
			Supports: []GroundingSupport{{StartIndex: 0, EndIndex: 5, Text: "Hello", SourceIndices: []int{0}}},
		}
		c.AlignSupports("Hello world")
		if len(c.Supports) != 1 || c.Supports[0].StartIndex != 0 || c.Supports[0].EndIndex != 5 {
			t.Errorf("对齐的偏移不应被改动: %+v", c.Supports)
		}
	})

	t.Run("偏移错位则按原文重新定位", func(t *testing.T) {
		c := &GroundingCitations{
			Sources: []GroundingSource{{URL: "https://a.example"}},
			// 上游偏移 0..5，但转换后正文前面多了前缀
			Supports: []GroundingSupport{{StartIndex: 0, EndIndex: 5, Text: "Hello", SourceIndices: []int{0}}},
		}
		c.AlignSupports("prefix: Hello world")
		if len(c.Supports) != 1 {
			t.Fatalf("应保留并校正该片段: %+v", c.Supports)
		}
		if c.Supports[0].StartIndex != 8 || c.Supports[0].EndIndex != 13 {
			t.Errorf("重新定位错误: start=%d end=%d, want 8/13", c.Supports[0].StartIndex, c.Supports[0].EndIndex)
		}
	})

	t.Run("正文中找不到则丢弃偏移但保留来源", func(t *testing.T) {
		c := &GroundingCitations{
			Sources:  []GroundingSource{{URL: "https://a.example"}},
			Supports: []GroundingSupport{{StartIndex: 0, EndIndex: 5, Text: "Hello", SourceIndices: []int{0}}},
		}
		c.AlignSupports("完全不同的正文")
		if len(c.Supports) != 0 {
			t.Errorf("对不上的片段应丢弃偏移: %+v", c.Supports)
		}
		if len(c.Sources) != 1 {
			t.Error("来源本身必须保留（客户端仍应看到引用了哪些网页）")
		}
	})

	t.Run("越界偏移不得 panic", func(t *testing.T) {
		c := &GroundingCitations{
			Sources:  []GroundingSource{{URL: "https://a.example"}},
			Supports: []GroundingSupport{{StartIndex: 100, EndIndex: 999, Text: "x", SourceIndices: []int{0}}},
		}
		c.AlignSupports("短")
	})

	t.Run("多字节字符按字节偏移", func(t *testing.T) {
		c := &GroundingCitations{
			Sources:  []GroundingSource{{URL: "https://a.example"}},
			Supports: []GroundingSupport{{Text: "波士顿", SourceIndices: []int{0}}},
		}
		answer := "今天波士顿很冷"
		c.AlignSupports(answer)
		if len(c.Supports) != 1 {
			t.Fatal("应定位到多字节片段")
		}
		s := c.Supports[0]
		if answer[s.StartIndex:s.EndIndex] != "波士顿" {
			t.Errorf("多字节偏移错误: %q", answer[s.StartIndex:s.EndIndex])
		}
	})
}

func TestToClaudeSearchBlocks(t *testing.T) {
	c := ParseGeminiGrounding(parseFixture(t, geminiGroundingFixture))
	blocks := c.ToClaudeSearchBlocks("seed1")
	if len(blocks) != 2 {
		t.Fatalf("应产出 server_tool_use + web_search_tool_result 两块: %+v", blocks)
	}
	if blocks[0].Type != "server_tool_use" || blocks[0].Name != "web_search" {
		t.Errorf("首块形态不符: %+v", blocks[0])
	}
	if blocks[1].Type != "web_search_tool_result" {
		t.Errorf("次块形态不符: %+v", blocks[1])
	}
	// 两块必须以同一个 id 关联，否则 Anthropic SDK 无法把结果挂到调用上
	if blocks[1].ToolUseID != blocks[0].ID {
		t.Errorf("tool_use_id (%s) 与 server_tool_use.id (%s) 不一致", blocks[1].ToolUseID, blocks[0].ID)
	}
	results, ok := blocks[1].Content.([]map[string]any)
	if !ok || len(results) != 2 {
		t.Fatalf("结果列表应含 2 条来源: %+v", blocks[1].Content)
	}
	if results[0]["type"] != "web_search_result" || results[0]["url"] != "https://weather.example/boston" {
		t.Errorf("结果项形态不符: %+v", results[0])
	}

	var empty *GroundingCitations
	if empty.ToClaudeSearchBlocks("s") != nil {
		t.Error("空证据不应产出块")
	}
}

func TestToOpenAIAnnotations(t *testing.T) {
	c := ParseGeminiGrounding(parseFixture(t, geminiGroundingFixture))
	c.AlignSupports("It is 68F in Boston today")

	anns := c.ToOpenAIAnnotations()
	if len(anns) != 2 {
		t.Fatalf("一个片段引用两个来源应产出 2 条 annotation: %+v", anns)
	}
	first := anns[0].(map[string]any)
	if first["type"] != "url_citation" {
		t.Errorf("type = %v, want url_citation", first["type"])
	}
	citation := first["url_citation"].(map[string]any)
	if citation["url"] != "https://weather.example/boston" {
		t.Errorf("url 不符: %+v", citation)
	}
	if _, ok := citation["start_index"]; !ok {
		t.Error("对齐后的片段应带偏移")
	}

	// 无片段时每个来源一条、不带偏移（宁可无位置，也不给错误位置）
	noSupport := &GroundingCitations{Sources: []GroundingSource{{URL: "https://a.example"}}}
	anns = noSupport.ToOpenAIAnnotations()
	if len(anns) != 1 {
		t.Fatalf("应按来源产出: %+v", anns)
	}
	citation = anns[0].(map[string]any)["url_citation"].(map[string]any)
	if _, ok := citation["start_index"]; ok {
		t.Error("无可信片段时不得附带偏移")
	}
}

func TestToResponsesAnnotationsAndSearchCall(t *testing.T) {
	c := ParseGeminiGrounding(parseFixture(t, geminiGroundingFixture))
	c.AlignSupports("It is 68F in Boston today")

	anns := c.ToResponsesAnnotations()
	if len(anns) != 2 {
		t.Fatalf("annotations 数 = %d, want 2: %+v", len(anns), anns)
	}
	if anns[0].Type != "url_citation" || anns[0].URL != "https://weather.example/boston" {
		t.Errorf("annotation 形态不符: %+v", anns[0])
	}

	item := c.ToResponsesWebSearchCallItem("seed1")
	if item == nil {
		t.Fatal("应产出 web_search_call 动作项")
	}
	if item["type"] != "web_search_call" || item["status"] != "completed" {
		t.Errorf("动作项形态不符: %+v", item)
	}
	action := item["action"].(map[string]any)
	if action["query"] != "weather boston, boston forecast" {
		t.Errorf("query 合并结果 = %v", action["query"])
	}

	var empty *GroundingCitations
	if empty.ToResponsesWebSearchCallItem("s") != nil {
		t.Error("空证据不应产出动作项")
	}
}
