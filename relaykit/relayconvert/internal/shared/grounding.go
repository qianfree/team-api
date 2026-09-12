package shared

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
)

// 服务端联网搜索的**响应侧**证据（引用来源）跨协议中间表示。
//
// 请求侧的能力映射见 websearch.go：客户端要求联网 → 各格式的原生搜索构件。
// 但上游真的去搜了之后，「搜到了什么」同样需要还原回客户端协议，否则客户端只拿到
// 一段被搜索增强的纯文本——**看不到来源、无法核查、也无法展示引用**，而这正是
// 联网搜索的主要价值。各格式的表达方式：
//
//	Gemini     candidate.groundingMetadata：groundingChunks（来源）+
//	           groundingSupports（文本片段→来源索引）+ webSearchQueries（实际查询）
//	Claude     content 块对：server_tool_use（发起搜索）+ web_search_tool_result（结果列表）
//	           另有 usage.server_tool_use.web_search_requests 供按次计费
//	OpenAI     message.annotations：[{type:"url_citation", url_citation:{url,title,start_index,end_index}}]
//	Responses  output_text.annotations：[{type:"url_citation", url, title, start_index, end_index}]
//
// 本文件只做协议还原；按次计费的定价挂接属宿主计费层，此处仅把
// web_search_requests 计数透出到 usage，使计费层有据可依。

// GroundingSource 一条被引用的网页来源。
type GroundingSource struct {
	URL   string
	Title string
}

// GroundingSupport 回答文本的一个片段与其来源的对应关系。
// StartIndex / EndIndex 是**字节**偏移（与 Gemini 一致，Go 字符串切片同为字节口径）。
type GroundingSupport struct {
	StartIndex    int
	EndIndex      int
	Text          string // 片段原文，用于校验偏移是否仍然对齐
	SourceIndices []int  // 指向 GroundingCitations.Sources
}

// GroundingCitations 一次带搜索的回答所附带的全部引用证据。
type GroundingCitations struct {
	Queries  []string
	Sources  []GroundingSource
	Supports []GroundingSupport
}

// IsEmpty 无来源即视为没有可还原的引用证据（只有 queries 而无来源时，
// 客户端拿不到任何可核查的东西，不值得合成空的工具结果块）。
func (c *GroundingCitations) IsEmpty() bool {
	return c == nil || len(c.Sources) == 0
}

// SearchRequestCount 本次回答实际发起的搜索次数，供计费层按次计价。
// Gemini 以 webSearchQueries 列出实际查询；缺失但有来源时按 1 次计。
func (c *GroundingCitations) SearchRequestCount() int {
	if c.IsEmpty() {
		return 0
	}
	if n := len(c.Queries); n > 0 {
		return n
	}
	return 1
}

// queryText 把多个查询合并为单个 query 字符串。
// Claude 的 server_tool_use.input 是单个 query，而 Gemini 把多次查询的结果**汇集**在
// 一组 groundingChunks 里、无法回溯每条来源属于哪次查询，故合并表达而非拆成多对
// （拆开会导致来源在各对之间重复，虚增引用数）。真实次数由 SearchRequestCount 承载。
func (c *GroundingCitations) queryText() string {
	if len(c.Queries) == 0 {
		return ""
	}
	return strings.Join(c.Queries, ", ")
}

// ---------- 解析：Gemini groundingMetadata ----------

// geminiGroundingMetadata 是 groundingMetadata 的解析形态。
// DTO 侧保持 `any` 以保证原生透传方向的 wire 保真（未知字段不丢），
// 这里按需重解析出所需字段。
type geminiGroundingMetadata struct {
	WebSearchQueries []string `json:"webSearchQueries"`
	GroundingChunks  []struct {
		Web *struct {
			URI   string `json:"uri"`
			Title string `json:"title"`
		} `json:"web"`
	} `json:"groundingChunks"`
	GroundingSupports []struct {
		Segment *struct {
			StartIndex int    `json:"startIndex"`
			EndIndex   int    `json:"endIndex"`
			Text       string `json:"text"`
		} `json:"segment"`
		GroundingChunkIndices []int `json:"groundingChunkIndices"`
	} `json:"groundingSupports"`
}

// ParseGeminiGrounding 从 candidate.groundingMetadata 提取引用证据。
// 无 grounding、解析失败、或无任何网页来源时返回 nil（调用方据此不做还原）。
func ParseGeminiGrounding(meta any) *GroundingCitations {
	if meta == nil {
		return nil
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	var parsed geminiGroundingMetadata
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}

	citations := &GroundingCitations{Queries: parsed.WebSearchQueries}
	// chunk 下标 → Sources 下标的重映射：非 web 类型的 chunk（如检索库片段）
	// 不产生来源，会让两侧下标错位，必须显式重映射而非直接沿用原下标
	chunkToSource := make(map[int]int, len(parsed.GroundingChunks))
	for i, chunk := range parsed.GroundingChunks {
		if chunk.Web == nil || chunk.Web.URI == "" {
			continue
		}
		chunkToSource[i] = len(citations.Sources)
		citations.Sources = append(citations.Sources, GroundingSource{
			URL:   chunk.Web.URI,
			Title: chunk.Web.Title,
		})
	}
	if len(citations.Sources) == 0 {
		return nil
	}

	for _, support := range parsed.GroundingSupports {
		if support.Segment == nil {
			continue
		}
		indices := make([]int, 0, len(support.GroundingChunkIndices))
		for _, ci := range support.GroundingChunkIndices {
			if si, ok := chunkToSource[ci]; ok {
				indices = append(indices, si)
			}
		}
		if len(indices) == 0 {
			continue
		}
		citations.Supports = append(citations.Supports, GroundingSupport{
			StartIndex:    support.Segment.StartIndex,
			EndIndex:      support.Segment.EndIndex,
			Text:          support.Segment.Text,
			SourceIndices: indices,
		})
	}
	return citations
}

// AlignSupports 校正片段偏移，使其对齐**转换后**的回答文本。
//
// 为什么必须校正：Gemini 的偏移是相对上游原始候选文本的，而转换后的正文可能与之不同
// （思考 part 被剥离、多个 text part 被拼接、前缀被裁剪）。偏移错位会让客户端把引用
// 高亮在错误的位置上——比没有引用更糟。
//
// 策略：segment.text 给出了片段原文，据此校验；对不上就在正文里重新定位；
// 仍找不到则丢弃该片段的偏移（保留来源本身，只是不带位置）。
func (c *GroundingCitations) AlignSupports(text string) {
	if c == nil || len(c.Supports) == 0 {
		return
	}
	aligned := make([]GroundingSupport, 0, len(c.Supports))
	for _, s := range c.Supports {
		if s.Text == "" {
			continue // 无原文可校验，不敢保留偏移
		}
		// 原偏移仍然对齐：直接沿用
		if s.StartIndex >= 0 && s.EndIndex <= len(text) && s.StartIndex < s.EndIndex &&
			text[s.StartIndex:s.EndIndex] == s.Text {
			aligned = append(aligned, s)
			continue
		}
		// 否则在正文中重新定位
		if idx := strings.Index(text, s.Text); idx >= 0 {
			s.StartIndex = idx
			s.EndIndex = idx + len(s.Text)
			aligned = append(aligned, s)
		}
		// 找不到：该片段不带偏移地丢弃，来源本身仍在 Sources 里
	}
	c.Supports = aligned
}

// ---------- 产出：Claude ----------

// ToClaudeSearchBlocks 产出 Claude 的 server_tool_use + web_search_tool_result 块对。
// Gemini 不提供 Anthropic 的 encrypted_content / page_age，仅还原 url + title——
// 能力（来源可见、可核查）保住，细粒度字段静默降级。
func (c *GroundingCitations) ToClaudeSearchBlocks(idSeed string) []dto.ClaudeContentBlock {
	if c.IsEmpty() {
		return nil
	}
	toolUseID := fmt.Sprintf("srvtoolu_%s", idSeed)

	results := make([]map[string]any, 0, len(c.Sources))
	for _, src := range c.Sources {
		results = append(results, map[string]any{
			"type":  "web_search_result",
			"url":   src.URL,
			"title": src.Title,
		})
	}

	return []dto.ClaudeContentBlock{
		{
			Type:  "server_tool_use",
			ID:    toolUseID,
			Name:  "web_search",
			Input: map[string]any{"query": c.queryText()},
		},
		{
			Type:      "web_search_tool_result",
			ToolUseID: toolUseID,
			Content:   results,
		},
	}
}

// ---------- 产出：OpenAI chat / Responses 的 url_citation ----------

// ToOpenAIAnnotations 产出 chat 的 annotations（url_citation 形态）。
// 有对齐后的片段时按「片段 × 来源」逐条产出并带偏移，否则每个来源产出一条无偏移的引用。
func (c *GroundingCitations) ToOpenAIAnnotations() []any {
	if c.IsEmpty() {
		return nil
	}
	out := make([]any, 0, len(c.Sources))
	emit := func(src GroundingSource, start, end int, withRange bool) {
		citation := map[string]any{"url": src.URL, "title": src.Title}
		if withRange {
			citation["start_index"] = start
			citation["end_index"] = end
		}
		out = append(out, map[string]any{"type": "url_citation", "url_citation": citation})
	}

	if len(c.Supports) > 0 {
		for _, s := range c.Supports {
			for _, si := range s.SourceIndices {
				if si < len(c.Sources) {
					emit(c.Sources[si], s.StartIndex, s.EndIndex, true)
				}
			}
		}
		return out
	}
	for _, src := range c.Sources {
		emit(src, 0, 0, false)
	}
	return out
}

// ToResponsesAnnotations 产出 Responses 的 output_text.annotations（url_citation 形态）。
func (c *GroundingCitations) ToResponsesAnnotations() []dto.ResponsesAnnotation {
	if c.IsEmpty() {
		return nil
	}
	out := make([]dto.ResponsesAnnotation, 0, len(c.Sources))
	if len(c.Supports) > 0 {
		for _, s := range c.Supports {
			for _, si := range s.SourceIndices {
				if si >= len(c.Sources) {
					continue
				}
				out = append(out, dto.ResponsesAnnotation{
					Type:       "url_citation",
					URL:        c.Sources[si].URL,
					Title:      c.Sources[si].Title,
					StartIndex: s.StartIndex,
					EndIndex:   s.EndIndex,
				})
			}
		}
		return out
	}
	for _, src := range c.Sources {
		out = append(out, dto.ResponsesAnnotation{Type: "url_citation", URL: src.URL, Title: src.Title})
	}
	return out
}

// ToResponsesWebSearchCallItem 产出 Responses 的 web_search_call 输出项，
// 让客户端看到「模型发起过搜索」这一事实（与 annotations 互补：前者是动作，后者是引用）。
func (c *GroundingCitations) ToResponsesWebSearchCallItem(idSeed string) map[string]any {
	if c.IsEmpty() {
		return nil
	}
	return map[string]any{
		"type":   "web_search_call",
		"id":     fmt.Sprintf("ws_%s", idSeed),
		"status": "completed",
		"action": map[string]any{"type": "search", "query": c.queryText()},
	}
}
