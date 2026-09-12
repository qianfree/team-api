package register

// capability_invariant_test.go — 能力守恒不变量测试。
//
// 目的：防止协议转换在某个方向上「静默丢能力」（工具定义丢失、联网搜索被吞、
// 图片被丢、多轮工具循环断链、系统提示消失等）。与金样本（精确字节比对）不同，
// 本测试断言的是结构性不变量：一份「满配」入站请求经每个已注册方向转换后，
// 目标格式请求中各能力构件的数量与标识必须符合该方向的期望档案。
//
// 关键设计：
//   - 输入用 wire JSON 并以宿主 parseInboundRequest 同款方式（裸 json.Unmarshal
//     进 DTO）解码——转换器收到的形状与线上完全一致（如 OpenAI 多模态 content
//     实际是 []any 而非 []dto.ContentPart）；
//   - 断言在转换结果 marshal 回 JSON 之后进行——检查的是上游真正收到的字节形状；
//   - 期望档案按转换器 ID 显式登记（capExpectations）。测试枚举注册表全部请求
//     转换器，未登记期望的方向直接失败并打印实测档案——新增方向必须同步声明
//     其能力预期，堵住「矩阵接管但能力静默丢失」的盲区；
//   - 已知的合理能力缺口（目标格式无法表达）以注释形式写在期望条目上，
//     改动导致缺口扩大时测试失败。

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

// ---------- 满配入站请求（wire JSON，形状与真实客户端一致） ----------

// 各 marker 全局唯一，用于跨格式检查文本是否幸存。
const (
	mkSystem     = "CAP_MARKER_SYSTEM"
	mkUser       = "CAP_MARKER_USER"
	mkToolResult = "CAP_MARKER_TOOL_RESULT"
	mkFollowup   = "CAP_MARKER_FOLLOWUP"
	imgDataURL   = "data:image/jpeg;base64,/9j/4AAQSkZJRg=="
)

var fullWireRequests = map[types.RelayFormat]string{
	types.RelayFormatOpenAI: `{
		"model": "origin-model",
		"messages": [
			{"role": "system", "content": "` + mkSystem + ` you are helpful"},
			{"role": "user", "content": [
				{"type": "text", "text": "` + mkUser + ` describe this image"},
				{"type": "image_url", "image_url": {"url": "` + imgDataURL + `"}}
			]},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"location\": \"Boston\"}"}}
			]},
			{"role": "tool", "tool_call_id": "call_1", "content": "` + mkToolResult + ` 68F"},
			{"role": "user", "content": "` + mkFollowup + ` thanks"}
		],
		"tools": [
			{"type": "function", "function": {"name": "get_weather", "description": "get weather", "parameters": {"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}}},
			{"type": "function", "function": {"name": "lookup_stock", "description": "stock quote", "parameters": {"type": "object", "properties": {"symbol": {"type": "string"}}}}}
		],
		"tool_choice": {"type": "function", "function": {"name": "get_weather"}},
		"max_tokens": 1024,
		"stop": ["END_TOKEN"],
		"temperature": 0.7,
		"stream": true
	}`,
	types.RelayFormatClaude: `{
		"model": "origin-model",
		"max_tokens": 1024,
		"system": "` + mkSystem + ` you are helpful",
		"messages": [
			{"role": "user", "content": [
				{"type": "text", "text": "` + mkUser + ` describe this image"},
				{"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "/9j/4AAQSkZJRg=="}}
			]},
			{"role": "assistant", "content": [
				{"type": "text", "text": "checking weather"},
				{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"location": "Boston"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_1", "content": "` + mkToolResult + ` 68F"}
			]},
			{"role": "user", "content": "` + mkFollowup + ` thanks"}
		],
		"tools": [
			{"name": "get_weather", "description": "get weather", "input_schema": {"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}},
			{"name": "lookup_stock", "description": "stock quote", "input_schema": {"type": "object", "properties": {"symbol": {"type": "string"}}}}
		],
		"tool_choice": {"type": "tool", "name": "get_weather"},
		"stop_sequences": ["END_TOKEN"],
		"temperature": 0.7,
		"stream": true
	}`,
	types.RelayFormatGemini: `{
		"systemInstruction": {"parts": [{"text": "` + mkSystem + ` you are helpful"}]},
		"contents": [
			{"role": "user", "parts": [
				{"text": "` + mkUser + ` describe this image"},
				{"inlineData": {"mimeType": "image/jpeg", "data": "/9j/4AAQSkZJRg=="}}
			]},
			{"role": "model", "parts": [
				{"text": "checking weather"},
				{"functionCall": {"name": "get_weather", "args": {"location": "Boston"}}}
			]},
			{"role": "user", "parts": [
				{"functionResponse": {"name": "get_weather", "response": {"result": "` + mkToolResult + ` 68F"}}}
			]},
			{"role": "user", "parts": [{"text": "` + mkFollowup + ` thanks"}]}
		],
		"tools": [
			{"functionDeclarations": [
				{"name": "get_weather", "description": "get weather", "parameters": {"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}},
				{"name": "lookup_stock", "description": "stock quote", "parameters": {"type": "object", "properties": {"symbol": {"type": "string"}}}}
			]}
		],
		"toolConfig": {"functionCallingConfig": {"mode": "ANY", "allowedFunctionNames": ["get_weather"]}},
		"generationConfig": {"maxOutputTokens": 1024, "stopSequences": ["END_TOKEN"], "temperature": 0.7}
	}`,
	types.RelayFormatOpenAIResponses: `{
		"model": "origin-model",
		"instructions": "` + mkSystem + ` you are helpful",
		"input": [
			{"role": "user", "content": [
				{"type": "input_text", "text": "` + mkUser + ` describe this image"},
				{"type": "input_image", "image_url": "` + imgDataURL + `"}
			]},
			{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": "{\"location\": \"Boston\"}"},
			{"type": "function_call_output", "call_id": "call_1", "output": "` + mkToolResult + ` 68F"},
			{"role": "user", "content": [{"type": "input_text", "text": "` + mkFollowup + ` thanks"}]}
		],
		"tools": [
			{"type": "function", "name": "get_weather", "description": "get weather", "parameters": {"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}},
			{"type": "function", "name": "lookup_stock", "description": "stock quote", "parameters": {"type": "object", "properties": {"symbol": {"type": "string"}}}}
		],
		"tool_choice": {"type": "function", "name": "get_weather"},
		"max_output_tokens": 1024,
		"temperature": 0.7,
		"stream": true
	}`,
}

// parseWireRequest 与宿主 relay/handler/relaykit_bridge.go parseInboundRequest 同款：
// 裸 json.Unmarshal 进对应 DTO 指针，保证转换器收到的形状与线上一致。
func parseWireRequest(t *testing.T, format types.RelayFormat, body string) any {
	t.Helper()
	switch format {
	case types.RelayFormatOpenAI:
		var req dto.GeneralOpenAIRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		return &req
	case types.RelayFormatClaude:
		var req dto.ClaudeRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		return &req
	case types.RelayFormatGemini:
		var req dto.GeminiChatRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		return &req
	case types.RelayFormatOpenAIResponses:
		var req dto.OpenAIResponsesRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		return &req
	}
	t.Fatalf("未支持的入站格式: %s", format)
	return nil
}

// ---------- 能力画像与提取器 ----------

// capProfile 是从某格式请求 JSON 中提取出的标准化能力画像。
type capProfile struct {
	ToolNames       []string // 自定义函数工具名（升序）
	ForcedToolName  string   // 强制 tool_choice 指向的函数名（无法表达/未强制为空）
	HasWebSearch    bool     // 目标格式的原生服务端搜索构件是否存在
	ImageCount      int      // 图片内容块数量
	ToolCallCount   int      // 模型侧函数调用数量（多轮历史中的 assistant tool_call）
	ToolResultCount int      // 工具结果块数量
	StopCount       int      // stop 序列数量
	MaxTokensSet    bool     // 输出上限是否已表达
	SystemMarker    bool     // 系统提示 marker 是否幸存（不限承载位置）
	UserMarker      bool     // 首条用户文本 marker 是否幸存
	ToolResultText  bool     // 工具结果文本 marker 是否幸存
	FollowupMarker  bool     // 多轮后续用户消息 marker 是否幸存
}

// extractProfile 将转换结果 marshal 回 JSON 后按目标格式提取能力画像。
func extractProfile(t *testing.T, target types.RelayFormat, converted any) capProfile {
	t.Helper()
	raw, err := json.Marshal(converted)
	require.NoError(t, err, "marshal 转换结果")
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))

	var p capProfile
	body := string(raw)
	p.SystemMarker = strings.Contains(body, mkSystem)
	p.UserMarker = strings.Contains(body, mkUser)
	p.ToolResultText = strings.Contains(body, mkToolResult)
	p.FollowupMarker = strings.Contains(body, mkFollowup)

	switch target {
	case types.RelayFormatOpenAI:
		extractOpenAIStructure(doc, &p)
	case types.RelayFormatClaude:
		extractClaudeStructure(doc, &p)
	case types.RelayFormatGemini:
		extractGeminiStructure(doc, &p)
	case types.RelayFormatOpenAIResponses:
		extractResponsesStructure(doc, &p)
	case types.RelayFormatOllama:
		extractOllamaStructure(doc, &p)
	default:
		t.Fatalf("未支持的目标格式: %s", target)
	}
	sort.Strings(p.ToolNames)
	return p
}

// JSON 取值助手
func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }
func asArr(v any) []any          { a, _ := v.([]any); return a }
func asStr(v any) string         { s, _ := v.(string); return s }
func stopLen(v any) int {
	switch s := v.(type) {
	case string:
		if s != "" {
			return 1
		}
	case []any:
		return len(s)
	}
	return 0
}

func extractOpenAIStructure(doc map[string]any, p *capProfile) {
	for _, tl := range asArr(doc["tools"]) {
		tm := asMap(tl)
		if asStr(tm["type"]) == "function" {
			if name := asStr(asMap(tm["function"])["name"]); name != "" {
				p.ToolNames = append(p.ToolNames, name)
			}
		}
	}
	if tc := asMap(doc["tool_choice"]); tc != nil && asStr(tc["type"]) == "function" {
		p.ForcedToolName = asStr(asMap(tc["function"])["name"])
	}
	p.HasWebSearch = doc["web_search_options"] != nil
	for _, msg := range asArr(doc["messages"]) {
		mm := asMap(msg)
		for _, part := range asArr(mm["content"]) {
			if asStr(asMap(part)["type"]) == "image_url" {
				p.ImageCount++
			}
		}
		p.ToolCallCount += len(asArr(mm["tool_calls"]))
		if asStr(mm["role"]) == "tool" {
			p.ToolResultCount++
		}
	}
	p.StopCount = stopLen(doc["stop"])
	p.MaxTokensSet = doc["max_tokens"] != nil || doc["max_completion_tokens"] != nil
}

func extractClaudeStructure(doc map[string]any, p *capProfile) {
	for _, tl := range asArr(doc["tools"]) {
		tm := asMap(tl)
		typ := asStr(tm["type"])
		if strings.HasPrefix(typ, "web_search") {
			p.HasWebSearch = true
			continue
		}
		if typ == "" || typ == "custom" {
			if name := asStr(tm["name"]); name != "" {
				p.ToolNames = append(p.ToolNames, name)
			}
		}
	}
	if tc := asMap(doc["tool_choice"]); tc != nil && asStr(tc["type"]) == "tool" {
		p.ForcedToolName = asStr(tc["name"])
	}
	for _, msg := range asArr(doc["messages"]) {
		for _, blk := range asArr(asMap(msg)["content"]) {
			switch asStr(asMap(blk)["type"]) {
			case "image":
				p.ImageCount++
			case "tool_use":
				p.ToolCallCount++
			case "tool_result":
				p.ToolResultCount++
			}
		}
	}
	p.StopCount = len(asArr(doc["stop_sequences"]))
	p.MaxTokensSet = doc["max_tokens"] != nil
}

func extractGeminiStructure(doc map[string]any, p *capProfile) {
	for _, tl := range asArr(doc["tools"]) {
		tm := asMap(tl)
		if tm["googleSearch"] != nil || tm["googleSearchRetrieval"] != nil {
			p.HasWebSearch = true
		}
		for _, fd := range asArr(tm["functionDeclarations"]) {
			if name := asStr(asMap(fd)["name"]); name != "" {
				p.ToolNames = append(p.ToolNames, name)
			}
		}
	}
	if fcc := asMap(asMap(doc["toolConfig"])["functionCallingConfig"]); fcc != nil {
		if asStr(fcc["mode"]) == "ANY" {
			if names := asArr(fcc["allowedFunctionNames"]); len(names) == 1 {
				p.ForcedToolName = asStr(names[0])
			}
		}
	}
	for _, content := range asArr(doc["contents"]) {
		for _, part := range asArr(asMap(content)["parts"]) {
			pm := asMap(part)
			if pm["inlineData"] != nil {
				p.ImageCount++
			}
			if pm["functionCall"] != nil {
				p.ToolCallCount++
			}
			if pm["functionResponse"] != nil {
				p.ToolResultCount++
			}
		}
	}
	gc := asMap(doc["generationConfig"])
	p.StopCount = len(asArr(gc["stopSequences"]))
	p.MaxTokensSet = gc["maxOutputTokens"] != nil
}

func extractResponsesStructure(doc map[string]any, p *capProfile) {
	for _, tl := range asArr(doc["tools"]) {
		tm := asMap(tl)
		typ := asStr(tm["type"])
		if strings.HasPrefix(typ, "web_search") {
			p.HasWebSearch = true
			continue
		}
		if typ == "function" {
			if name := asStr(tm["name"]); name != "" {
				p.ToolNames = append(p.ToolNames, name)
			}
		}
	}
	if tc := asMap(doc["tool_choice"]); tc != nil && asStr(tc["type"]) == "function" {
		p.ForcedToolName = asStr(tc["name"])
	}
	for _, item := range asArr(doc["input"]) {
		im := asMap(item)
		switch asStr(im["type"]) {
		case "function_call":
			p.ToolCallCount++
		case "function_call_output":
			p.ToolResultCount++
		}
		for _, part := range asArr(im["content"]) {
			if asStr(asMap(part)["type"]) == "input_image" {
				p.ImageCount++
			}
		}
	}
	p.MaxTokensSet = doc["max_output_tokens"] != nil
	// Responses API 无请求级 stop 序列，StopCount 恒 0
}

func extractOllamaStructure(doc map[string]any, p *capProfile) {
	for _, tl := range asArr(doc["tools"]) {
		tm := asMap(tl)
		if asStr(tm["type"]) == "function" {
			if name := asStr(asMap(tm["function"])["name"]); name != "" {
				p.ToolNames = append(p.ToolNames, name)
			}
		}
	}
	for _, msg := range asArr(doc["messages"]) {
		mm := asMap(msg)
		p.ImageCount += len(asArr(mm["images"]))
		p.ToolCallCount += len(asArr(mm["tool_calls"]))
		if asStr(mm["role"]) == "tool" {
			p.ToolResultCount++
		}
	}
	opts := asMap(doc["options"])
	p.StopCount = stopLen(opts["stop"])
	p.MaxTokensSet = opts["num_predict"] != nil
	// Ollama 无 tool_choice / 服务端搜索
}

// ---------- 期望档案表 ----------

// capMeta 构造转换所需 Meta（与宿主构造方式对齐：模型名 + 渠道选项）。
func capMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "origin-model",
		UpstreamModelName:   "upstream-model",
		Options: &convmeta.Options{
			Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 4096 }},
			Gemini: convmeta.GeminiOptions{WebSearchToGoogleSearch: true},
		},
	}
}

// capExpectations 每个已注册请求转换器在满配请求下的期望能力画像。
// 新注册方向必须在此登记（TestCapabilityInvariants_Full 枚举注册表并强制检查），
// 已知合理缺口必须注释说明原因。
var capExpectations = map[string]capProfile{
	// 满配画像基准：两个自定义工具 + 强制工具选择 + 1 图片 + 1 轮工具循环 + stop + max_tokens + 全部文本 marker
	relayconvert.ConverterOpenAIChatToClaudeMessages: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 1, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterOpenAIChatToGeminiContent: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 1, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterOpenAIChatToOllama: {
		// 缺口：Ollama /api/chat 无 tool_choice 语义，强制工具选择无法表达
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 1, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterOpenAIChatToOpenAIResponses: {
		// 缺口：Responses API 无请求级 stop 参数
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 0, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterClaudeMessagesToOpenAIChat: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 1, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterClaudeMessagesToGeminiContent: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 1, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterGeminiContentToOpenAIChat: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 1, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterGeminiContentToClaudeMessages: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 1, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterOpenAIResponsesToOpenAIChat: {
		// StopCount 0：Responses 入站请求本身无 stop 概念（非转换丢失）
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 0, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterResponsesToClaudeMessages: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 0, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
	relayconvert.ConverterOpenAIResponsesToGemini: {
		ToolNames: []string{"get_weather", "lookup_stock"}, ForcedToolName: "get_weather",
		ImageCount: 1, ToolCallCount: 1, ToolResultCount: 1, StopCount: 0, MaxTokensSet: true,
		SystemMarker: true, UserMarker: true, ToolResultText: true, FollowupMarker: true,
	},
}

func TestCapabilityInvariants_Full(t *testing.T) {
	ctx := context.Background()

	for _, id := range relayconvert.ListRequestConverterIDs() {
		spec, ok := relayconvert.LookupRequestConverter(id)
		require.True(t, ok)

		input, hasInput := fullWireRequests[spec.From]
		if !hasInput {
			t.Errorf("转换器 %s 的入站格式 %s 没有满配请求语料，请在 fullWireRequests 补充", id, spec.From)
			continue
		}

		t.Run(id, func(t *testing.T) {
			parsed := parseWireRequest(t, spec.From, input)
			converted, err := relayconvert.ConvertRequestByID(ctx, capMeta(), id, parsed)
			require.NoError(t, err, "转换失败")

			actual := extractProfile(t, spec.To, converted)

			expected, registered := capExpectations[id]
			if !registered {
				t.Errorf("转换器 %s 未登记能力期望档案。实测档案（确认合理后粘贴进 capExpectations）：\n%s: %s,",
					id, fmt.Sprintf("%q", id), profileLiteral(actual))
				return
			}
			require.Equal(t, expected, actual, "能力画像与期望不符（若为有意变更，请同步更新 capExpectations 并说明缺口原因）")
		})
	}
}

// profileLiteral 输出可直接粘贴进期望表的 Go 字面量。
func profileLiteral(p capProfile) string {
	return fmt.Sprintf("{ToolNames: %#v, ForcedToolName: %q, HasWebSearch: %v, ImageCount: %d, ToolCallCount: %d, ToolResultCount: %d, StopCount: %d, MaxTokensSet: %v, SystemMarker: %v, UserMarker: %v, ToolResultText: %v, FollowupMarker: %v}",
		p.ToolNames, p.ForcedToolName, p.HasWebSearch, p.ImageCount, p.ToolCallCount, p.ToolResultCount, p.StopCount, p.MaxTokensSet, p.SystemMarker, p.UserMarker, p.ToolResultText, p.FollowupMarker)
}

// ---------- 响应侧能力守恒 ----------

// 响应侧 marker
const (
	mkAssistant = "CAP_MARKER_ASSISTANT_TEXT"
	mkThinking  = "CAP_MARKER_THINKING"
)

// upstreamWireResponses 各上游格式的满配非流式响应（wire JSON）：
// assistant 文本 + thinking（格式支持时）+ 1 个函数调用 + usage。
var upstreamWireResponses = map[types.RelayFormat]string{
	types.RelayFormatOpenAI: `{
		"id": "chatcmpl-abc123",
		"object": "chat.completion",
		"created": 1741569952,
		"model": "upstream-model",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "` + mkAssistant + ` the weather is 68F",
				"reasoning_content": "` + mkThinking + ` user asks weather",
				"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"location\": \"Boston\"}"}}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}
	}`,
	types.RelayFormatClaude: `{
		"id": "msg_abc123",
		"type": "message",
		"role": "assistant",
		"model": "upstream-model",
		"content": [
			{"type": "thinking", "thinking": "` + mkThinking + ` user asks weather", "signature": "sig_abc"},
			{"type": "text", "text": "` + mkAssistant + ` the weather is 68F"},
			{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"location": "Boston"}}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 100, "output_tokens": 50}
	}`,
	types.RelayFormatGemini: `{
		"candidates": [{
			"content": {"role": "model", "parts": [
				{"text": "` + mkThinking + ` user asks weather", "thought": true},
				{"text": "` + mkAssistant + ` the weather is 68F", "thoughtSignature": "sig_abc"},
				{"functionCall": {"name": "get_weather", "args": {"location": "Boston"}}}
			]},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {"promptTokenCount": 100, "candidatesTokenCount": 50, "totalTokenCount": 150},
		"modelVersion": "upstream-model"
	}`,
	types.RelayFormatOpenAIResponses: `{
		"id": "resp_abc123",
		"object": "response",
		"created_at": 1741476542,
		"status": "completed",
		"model": "upstream-model",
		"output": [
			{"type": "reasoning", "id": "rs_1", "summary": [{"type": "summary_text", "text": "` + mkThinking + ` user asks weather"}]},
			{"type": "message", "id": "msg_1", "status": "completed", "role": "assistant", "content": [{"type": "output_text", "text": "` + mkAssistant + ` the weather is 68F", "annotations": []}]},
			{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "get_weather", "arguments": "{\"location\": \"Boston\"}", "status": "completed"}
		],
		"usage": {"input_tokens": 100, "output_tokens": 50, "total_tokens": 150}
	}`,
	types.RelayFormatOllama: `{
		"model": "upstream-model",
		"created_at": "2026-01-01T00:00:00Z",
		"message": {
			"role": "assistant",
			"content": "` + mkAssistant + ` the weather is 68F",
			"thinking": "` + mkThinking + ` user asks weather",
			"tool_calls": [{"function": {"name": "get_weather", "arguments": {"location": "Boston"}}}]
		},
		"done": true,
		"done_reason": "stop",
		"prompt_eval_count": 100,
		"eval_count": 50
	}`,
}

// parseWireResponse 按上游格式把响应体解析为对应 DTO 指针（与宿主响应解析一致）。
func parseWireResponse(t *testing.T, format types.RelayFormat, body string) any {
	t.Helper()
	switch format {
	case types.RelayFormatOpenAI:
		var resp dto.ChatCompletionResponse
		require.NoError(t, json.Unmarshal([]byte(body), &resp))
		return &resp
	case types.RelayFormatClaude:
		var resp dto.ClaudeResponse
		require.NoError(t, json.Unmarshal([]byte(body), &resp))
		return &resp
	case types.RelayFormatGemini:
		var resp dto.GeminiChatResponse
		require.NoError(t, json.Unmarshal([]byte(body), &resp))
		return &resp
	case types.RelayFormatOpenAIResponses:
		var resp dto.OpenAIResponsesResponse
		require.NoError(t, json.Unmarshal([]byte(body), &resp))
		return &resp
	case types.RelayFormatOllama:
		var resp dto.OllamaChatResponse
		require.NoError(t, json.Unmarshal([]byte(body), &resp))
		return &resp
	}
	t.Fatalf("未支持的上游格式: %s", format)
	return nil
}

// respProfile 客户端格式响应的能力画像。
type respProfile struct {
	ToolCallNames []string // 函数调用名（升序）
	TextMarker    bool     // assistant 文本 marker 幸存
	ThinkMarker   bool     // thinking 内容 marker 幸存（承载位置不限）
	UsageIn       bool     // 输入 token 数 > 0
	UsageOut      bool     // 输出 token 数 > 0
}

func extractRespProfile(t *testing.T, client types.RelayFormat, converted any) respProfile {
	t.Helper()
	raw, err := json.Marshal(converted)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))

	var p respProfile
	body := string(raw)
	p.TextMarker = strings.Contains(body, mkAssistant)
	p.ThinkMarker = strings.Contains(body, mkThinking)

	num := func(m map[string]any, key string) float64 { f, _ := m[key].(float64); return f }

	switch client {
	case types.RelayFormatOpenAI:
		for _, ch := range asArr(doc["choices"]) {
			msg := asMap(asMap(ch)["message"])
			for _, tc := range asArr(msg["tool_calls"]) {
				if name := asStr(asMap(asMap(tc)["function"])["name"]); name != "" {
					p.ToolCallNames = append(p.ToolCallNames, name)
				}
			}
		}
		usage := asMap(doc["usage"])
		p.UsageIn = num(usage, "prompt_tokens") > 0
		p.UsageOut = num(usage, "completion_tokens") > 0
	case types.RelayFormatClaude:
		for _, blk := range asArr(doc["content"]) {
			bm := asMap(blk)
			if asStr(bm["type"]) == "tool_use" {
				if name := asStr(bm["name"]); name != "" {
					p.ToolCallNames = append(p.ToolCallNames, name)
				}
			}
		}
		usage := asMap(doc["usage"])
		p.UsageIn = num(usage, "input_tokens") > 0
		p.UsageOut = num(usage, "output_tokens") > 0
	case types.RelayFormatGemini:
		for _, cand := range asArr(doc["candidates"]) {
			for _, part := range asArr(asMap(asMap(cand)["content"])["parts"]) {
				if fc := asMap(asMap(part)["functionCall"]); fc != nil {
					if name := asStr(fc["name"]); name != "" {
						p.ToolCallNames = append(p.ToolCallNames, name)
					}
				}
			}
		}
		usage := asMap(doc["usageMetadata"])
		p.UsageIn = num(usage, "promptTokenCount") > 0
		p.UsageOut = num(usage, "candidatesTokenCount") > 0
	case types.RelayFormatOpenAIResponses:
		for _, item := range asArr(doc["output"]) {
			im := asMap(item)
			if asStr(im["type"]) == "function_call" {
				if name := asStr(im["name"]); name != "" {
					p.ToolCallNames = append(p.ToolCallNames, name)
				}
			}
		}
		usage := asMap(doc["usage"])
		p.UsageIn = num(usage, "input_tokens") > 0
		p.UsageOut = num(usage, "output_tokens") > 0
	default:
		t.Fatalf("未支持的客户端格式: %s", client)
	}
	sort.Strings(p.ToolCallNames)
	return p
}

// respExpectations 各方向（响应转换：上游 To 格式 → 客户端 From 格式）的期望画像。
// 新注册方向必须在此登记；已知缺口注明原因，修复后同步翻转期望值。
var respExpectations = map[string]respProfile{
	relayconvert.ConverterOpenAIChatToClaudeMessages: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterOpenAIChatToGeminiContent: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterOpenAIChatToOllama: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterOpenAIChatToOpenAIResponses: {
		// Responses 上游的 reasoning summary 回填 chat 的 reasoning_content
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterClaudeMessagesToOpenAIChat: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterClaudeMessagesToGeminiContent: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterGeminiContentToOpenAIChat: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterGeminiContentToClaudeMessages: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	// Responses 客户端方向：上游 thinking（chat reasoning_content / Claude thinking 块 /
	// Gemini thought part）合成为 Responses 的 reasoning 输出项
	//（{type:"reasoning", summary:[{type:"summary_text", text:...}]}，排在 message 之前）
	relayconvert.ConverterOpenAIResponsesToOpenAIChat: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterResponsesToClaudeMessages: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
	relayconvert.ConverterOpenAIResponsesToGemini: {
		ToolCallNames: []string{"get_weather"}, TextMarker: true, ThinkMarker: true, UsageIn: true, UsageOut: true,
	},
}

func TestCapabilityInvariants_Response(t *testing.T) {
	ctx := context.Background()

	for _, id := range relayconvert.ListResponseConverterIDs() {
		spec, ok := relayconvert.LookupResponseConverter(id)
		require.True(t, ok)
		if spec.Convert == nil {
			continue // 纯流式登记（无非流式转换器）不在本测试范围
		}

		input, hasInput := upstreamWireResponses[spec.To]
		if !hasInput {
			t.Errorf("响应转换器 %s 的上游格式 %s 没有响应语料，请在 upstreamWireResponses 补充", id, spec.To)
			continue
		}

		t.Run(id, func(t *testing.T) {
			parsed := parseWireResponse(t, spec.To, input)
			converted, _, err := spec.Convert(ctx, capMeta(), parsed)
			require.NoError(t, err, "响应转换失败")

			actual := extractRespProfile(t, spec.From, converted)

			expected, registered := respExpectations[id]
			if !registered {
				t.Errorf("响应转换器 %s 未登记期望档案。实测：\n%q: {ToolCallNames: %#v, TextMarker: %v, ThinkMarker: %v, UsageIn: %v, UsageOut: %v},",
					id, id, actual.ToolCallNames, actual.TextMarker, actual.ThinkMarker, actual.UsageIn, actual.UsageOut)
				return
			}
			require.Equal(t, expected, actual, "响应能力画像与期望不符（若为有意变更请更新 respExpectations 并注明原因）")
		})
	}
}

// ---------- 联网搜索场景 ----------

// webSearchWireRequests 各入站格式的「仅服务端搜索工具」请求（无自定义函数——
// Claude→Gemini 的 googleSearch 映射采取保守策略，与 functionDeclarations 共存时不附加）。
var webSearchWireRequests = map[types.RelayFormat]string{
	types.RelayFormatOpenAI: `{
		"model": "origin-model",
		"messages": [{"role": "user", "content": "` + mkUser + ` search the latest news"}],
		"web_search_options": {},
		"max_tokens": 1024
	}`,
	types.RelayFormatClaude: `{
		"model": "origin-model",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "` + mkUser + ` search the latest news"}],
		"tools": [{"type": "web_search_20250305", "name": "web_search", "max_uses": 5}]
	}`,
	types.RelayFormatGemini: `{
		"contents": [{"role": "user", "parts": [{"text": "` + mkUser + ` search the latest news"}]}],
		"tools": [{"googleSearch": {}}],
		"generationConfig": {"maxOutputTokens": 1024}
	}`,
	types.RelayFormatOpenAIResponses: `{
		"model": "origin-model",
		"input": [{"role": "user", "content": [{"type": "input_text", "text": "` + mkUser + ` search the latest news"}]}],
		"tools": [{"type": "web_search"}],
		"max_output_tokens": 1024
	}`,
}

// webSearchExpectations 各方向转换后目标格式是否保留了服务端搜索能力。
//
// 这是用户最担心的静默丢失能力之一：客户端明确要求联网、请求却以「没搜索」的方式
// 成功返回，比直接报错更难排查。四种文本协议都有原生构件（chat 的 web_search_options、
// Claude 的 web_search_* 工具、Gemini 的 googleSearch、Responses 的 web_search 工具），
// 由 shared/websearch.go 的 WebSearchSpec 做跨协议中间表示；chat 作为两跳链的中枢
// 承载该能力，因此跨原生方向（Gemini→Claude 等）也能保住。
//
// 为 false 的方向必须是**目标协议真实无此能力**，并注明原因；单纯「没实现」不得置 false。
var webSearchExpectations = map[string]bool{
	// OpenAI chat 入站 web_search_options
	relayconvert.ConverterOpenAIChatToClaudeMessages:  true,  // → Claude web_search 内置工具
	relayconvert.ConverterOpenAIChatToGeminiContent:   true,  // → googleSearch（渠道开启 WebSearchToGoogleSearch 时；本测试开启）
	relayconvert.ConverterOpenAIChatToOllama:          false, // Ollama 无服务端搜索能力（协议真实缺失）
	relayconvert.ConverterOpenAIChatToOpenAIResponses: true,  // → Responses web_search 工具

	// Claude 入站 web_search_* 工具
	relayconvert.ConverterClaudeMessagesToOpenAIChat:    true, // → chat web_search_options
	relayconvert.ConverterClaudeMessagesToGeminiContent: true, // → googleSearch（直连方向，渠道开关控制）

	// Gemini 入站 googleSearch 工具
	relayconvert.ConverterGeminiContentToOpenAIChat:     true, // → chat web_search_options
	relayconvert.ConverterGeminiContentToClaudeMessages: true, // 两跳链 g2o→o2c，经中枢 web_search_options 保住

	// Responses 入站 web_search 工具
	relayconvert.ConverterOpenAIResponsesToOpenAIChat: true, // → chat web_search_options
	relayconvert.ConverterResponsesToClaudeMessages:   true, // 两跳链 r2o→o2c
	relayconvert.ConverterOpenAIResponsesToGemini:     true, // 两跳链 r2o→o2g
}

func TestCapabilityInvariants_WebSearch(t *testing.T) {
	ctx := context.Background()

	for _, id := range relayconvert.ListRequestConverterIDs() {
		spec, ok := relayconvert.LookupRequestConverter(id)
		require.True(t, ok)

		input, hasInput := webSearchWireRequests[spec.From]
		if !hasInput {
			t.Errorf("转换器 %s 的入站格式 %s 没有搜索场景语料，请在 webSearchWireRequests 补充", id, spec.From)
			continue
		}

		t.Run(id, func(t *testing.T) {
			parsed := parseWireRequest(t, spec.From, input)
			converted, err := relayconvert.ConvertRequestByID(ctx, capMeta(), id, parsed)
			require.NoError(t, err, "转换失败")

			actual := extractProfile(t, spec.To, converted)

			expected, registered := webSearchExpectations[id]
			if !registered {
				t.Errorf("转换器 %s 未登记搜索能力期望（实测 HasWebSearch=%v），请在 webSearchExpectations 补充", id, actual.HasWebSearch)
				return
			}
			require.Equal(t, expected, actual.HasWebSearch,
				"服务端搜索能力保留情况与期望不符（若为有意变更请更新 webSearchExpectations 并注明原因）")
			// 搜索场景无自定义函数：不得凭空多出函数工具（如把 web_search 错当自定义函数转换）
			require.Empty(t, actual.ToolNames, "搜索场景不应产出自定义函数工具")
			// 用户文本必须幸存
			require.True(t, actual.UserMarker, "用户文本在搜索场景转换后丢失")
		})
	}
}

// ---------- 响应侧 grounding 还原 ----------

// geminiGroundedResponse 带 groundingMetadata 的 Gemini 上游响应（满配搜索场景）：
// 模型真的去搜了，回答附带两条网页来源与一个覆盖片段。
const geminiGroundedResponse = `{
	"candidates": [{
		"content": {"role": "model", "parts": [
			{"text": "` + mkAssistant + ` the weather is 68F"}
		]},
		"finishReason": "STOP",
		"index": 0,
		"groundingMetadata": {
			"webSearchQueries": ["boston weather"],
			"groundingChunks": [
				{"web": {"uri": "https://weather.example/boston", "title": "weather.example"}},
				{"web": {"uri": "https://forecast.example/ma", "title": "forecast.example"}}
			],
			"groundingSupports": [{
				"segment": {"startIndex": 0, "endIndex": 5, "text": "` + mkAssistant + `"},
				"groundingChunkIndices": [0, 1]
			}]
		}
	}],
	"usageMetadata": {"promptTokenCount": 100, "candidatesTokenCount": 50, "totalTokenCount": 150},
	"modelVersion": "upstream-model"
}`

// groundingRestoreExpectations 各客户端方向是否把上游的搜索来源还原为本协议的引用构件。
//
// 与请求侧（webSearchExpectations）对称：请求侧保住「要求联网」的能力，
// 响应侧保住「搜到了什么」的证据。只还原请求、不还原响应，客户端拿到的是一段
// 被搜索增强却无出处的文本——看不到来源、无法核查，联网搜索的主要价值随之丢失。
//
// 键为**响应转换方向的配对转换器 ID**（From=客户端格式、To=Gemini 上游）。
var groundingRestoreExpectations = map[string]bool{
	relayconvert.ConverterOpenAIChatToGeminiContent:     true, // → chat message.annotations（url_citation）
	relayconvert.ConverterClaudeMessagesToGeminiContent: true, // → server_tool_use + web_search_tool_result 块
	relayconvert.ConverterOpenAIResponsesToGemini:       true, // → web_search_call 项 + output_text.annotations
}

// TestCapabilityInvariants_GroundingRestore 响应侧搜索证据还原：
// 上游 Gemini 返回带 groundingMetadata 的响应时，来源 URL 必须出现在客户端可见的响应里。
func TestCapabilityInvariants_GroundingRestore(t *testing.T) {
	ctx := context.Background()

	for _, id := range relayconvert.ListResponseConverterIDs() {
		spec, ok := relayconvert.LookupResponseConverter(id)
		require.True(t, ok)
		// 只考察上游为 Gemini 的方向（grounding 是 Gemini 侧的构件）
		if spec.Convert == nil || spec.To != types.RelayFormatGemini {
			continue
		}

		t.Run(id, func(t *testing.T) {
			parsed := parseWireResponse(t, types.RelayFormatGemini, geminiGroundedResponse)
			converted, _, err := spec.Convert(ctx, capMeta(), parsed)
			require.NoError(t, err, "响应转换失败")

			raw, err := json.Marshal(converted)
			require.NoError(t, err)
			body := string(raw)

			expected, registered := groundingRestoreExpectations[id]
			if !registered {
				t.Errorf("响应转换器 %s 未登记 grounding 还原期望（实测含来源=%v），请在 groundingRestoreExpectations 补充",
					id, strings.Contains(body, "weather.example/boston"))
				return
			}

			hasSource := strings.Contains(body, "https://weather.example/boston") &&
				strings.Contains(body, "https://forecast.example/ma")
			require.Equal(t, expected, hasSource,
				"搜索来源还原情况与期望不符（若为有意变更请更新 groundingRestoreExpectations 并注明原因）\n实际响应: %s", body)

			// 回答正文不得丢失（还原引用不能以吞掉正文为代价）
			require.Contains(t, body, mkAssistant, "回答正文在 grounding 还原后丢失")
		})
	}
}

// TestCapabilityInvariants_GroundingSearchCountForBilling Claude 客户端方向必须把
// 搜索次数透出到 usage.server_tool_use.web_search_requests：
// Google 在 token 之外按搜索次数单独计价，不透出则计费层无从得知发起过几次搜索。
func TestCapabilityInvariants_GroundingSearchCountForBilling(t *testing.T) {
	ctx := context.Background()

	spec, ok := relayconvert.LookupResponseConverter(relayconvert.ConverterClaudeMessagesToGeminiContent)
	require.True(t, ok)
	require.NotNil(t, spec.Convert)

	parsed := parseWireResponse(t, types.RelayFormatGemini, geminiGroundedResponse)
	converted, _, err := spec.Convert(ctx, capMeta(), parsed)
	require.NoError(t, err)

	claudeResp, ok := converted.(*dto.ClaudeResponse)
	require.True(t, ok, "期望 *dto.ClaudeResponse, got %T", converted)
	require.NotNil(t, claudeResp.Usage, "usage 缺失")
	require.NotNil(t, claudeResp.Usage.ServerToolUse, "usage.server_tool_use 缺失（按次计费无据可依）")
	require.Equal(t, 1, claudeResp.Usage.ServerToolUse.WebSearchRequests,
		"web_search_requests 应等于 webSearchQueries 条数")
}
