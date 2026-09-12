package corpus

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func scrubJSON(t *testing.T, body string, opts ...ScrubOptions) string {
	t.Helper()
	o := ScrubOptions{Enabled: true}
	if len(opts) > 0 {
		o = opts[0]
	}
	return string(NewScrubber(o).ScrubJSON([]byte(body)))
}

// TestScrub_ToolCallIDReferenceConsistency 最关键的不变量：
// tool_call_id 在 assistant.tool_calls 与后续 tool_result 之间必须替换后仍相等。
//
// 打断这条引用，工具循环链就断了——而工具循环恰是转换最容易出 bug 的地方，
// 语料反而会变成假阳性来源（测试报「工具循环丢失」，实际是语料自己坏了）。
func TestScrub_ToolCallIDReferenceConsistency(t *testing.T) {
	body := `{
		"messages": [
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_abc123","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Boston\"}"}},
				{"id":"call_def456","type":"function","function":{"name":"lookup_stock","arguments":"{\"symbol\":\"AAPL\"}"}}
			]},
			{"role":"tool","tool_call_id":"call_abc123","content":"68F"},
			{"role":"tool","tool_call_id":"call_def456","content":"180.5"}
		]
	}`
	out := scrubJSON(t, body)

	var parsed struct {
		Messages []struct {
			ToolCalls []struct {
				ID string `json:"id"`
			} `json:"tool_calls"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("脱敏结果不是合法 JSON: %v\n%s", err, out)
	}

	calls := parsed.Messages[0].ToolCalls
	if len(calls) != 2 {
		t.Fatalf("工具调用数变化: %+v", calls)
	}
	if calls[0].ID == "call_abc123" {
		t.Error("标识符未被脱敏")
	}
	// 不同标识符不得映射成同一个值，否则两条工具结果会挂到同一个调用上
	if calls[0].ID == calls[1].ID {
		t.Errorf("不同标识符被映射成同一个值: %s", calls[0].ID)
	}
	if parsed.Messages[1].ToolCallID != calls[0].ID {
		t.Errorf("引用断裂: tool_call_id=%q 但 tool_calls[0].id=%q",
			parsed.Messages[1].ToolCallID, calls[0].ID)
	}
	if parsed.Messages[2].ToolCallID != calls[1].ID {
		t.Errorf("引用断裂: tool_call_id=%q 但 tool_calls[1].id=%q",
			parsed.Messages[2].ToolCallID, calls[1].ID)
	}
}

// TestScrub_ClaudeToolUseReference Claude 用 tool_use.id ←→ tool_result.tool_use_id 关联。
func TestScrub_ClaudeToolUseReference(t *testing.T) {
	body := `{
		"messages": [
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_01X","name":"get_weather","input":{"city":"Boston"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01X","content":"68F"}]}
		]
	}`
	out := scrubJSON(t, body)

	if strings.Contains(out, "toolu_01X") {
		t.Error("标识符未被脱敏")
	}
	// 前缀保留：部分转换器与测试按前缀模式识别合成 ID
	ids := regexp.MustCompile(`toolu_\d+`).FindAllString(out, -1)
	if len(ids) != 2 {
		t.Fatalf("应保留 toolu_ 前缀且出现两次: %v\n%s", ids, out)
	}
	if ids[0] != ids[1] {
		t.Errorf("Claude 工具引用断裂: %v", ids)
	}
}

// TestScrub_GeminiFunctionNameReference Gemini 的 functionCall ↔ functionResponse
// **靠函数名关联**（没有 id），因此工具名也必须一致映射。
func TestScrub_GeminiFunctionNameReference(t *testing.T) {
	body := `{
		"contents": [
			{"role":"model","parts":[{"functionCall":{"name":"query_internal_crm","args":{"id":"42"}}}]},
			{"role":"user","parts":[{"functionResponse":{"name":"query_internal_crm","response":{"result":"ok"}}}]}
		]
	}`
	out := scrubJSON(t, body)

	if strings.Contains(out, "query_internal_crm") {
		t.Error("自定义工具名未被脱敏（可能泄漏业务逻辑）")
	}
	names := regexp.MustCompile(`"name":"([^"]+)"`).FindAllStringSubmatch(out, -1)
	if len(names) != 2 {
		t.Fatalf("函数名应出现两次: %v\n%s", names, out)
	}
	if names[0][1] != names[1][1] {
		t.Errorf("Gemini 函数名引用断裂: %s vs %s", names[0][1], names[1][1])
	}
}

// TestScrub_ToolArgumentsStayValidJSON tool arguments 是**JSON 字符串**，
// 替换后必须仍是合法 JSON——否则转换器走进解析失败分支，语料反映的不是线上行为。
func TestScrub_ToolArgumentsStayValidJSON(t *testing.T) {
	body := `{"tool_calls":[{"id":"call_1","function":{"name":"f","arguments":"{\"city\":\"Boston\",\"unit\":\"celsius\",\"days\":7}"}}]}`
	out := scrubJSON(t, body)

	var parsed struct {
		ToolCalls []struct {
			Function struct {
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("外层不是合法 JSON: %v", err)
	}

	args := parsed.ToolCalls[0].Function.Arguments
	var inner map[string]any
	if err := json.Unmarshal([]byte(args), &inner); err != nil {
		t.Fatalf("arguments 脱敏后不再是合法 JSON: %v\n%s", err, args)
	}
	if _, ok := inner["city"]; !ok {
		t.Errorf("arguments 的 key 结构应保留: %v", inner)
	}
	if inner["days"] != float64(7) {
		t.Errorf("数值应保留（影响转换行为）: %v", inner["days"])
	}
	if inner["city"] == "Boston" {
		t.Error("arguments 内的用户数据未被脱敏")
	}
}

// TestScrub_PreservesBehaviorFields 判别式与行为相关字段必须原样保留，
// 否则语料不再触发对应的转换分支（model 决定 Claude 默认 max_tokens 与 Vertex 端点选择）。
func TestScrub_PreservesBehaviorFields(t *testing.T) {
	body := `{
		"model":"claude-3-5-sonnet-20241022",
		"max_tokens":1024,
		"temperature":0.7,
		"messages":[{"role":"user","content":[{"type":"text","text":"你好世界"}]}]
	}`
	out := scrubJSON(t, body)

	for _, must := range []string{
		`"model":"claude-3-5-sonnet-20241022"`, // 模型名影响分档与路由
		`"role":"user"`,                        // 角色是判别式
		`"type":"text"`,                        // 内容块类型是判别式
		`"max_tokens":1024`,                    // 数值影响转换
	} {
		if !strings.Contains(out, must) {
			t.Errorf("行为相关字段被破坏，缺少 %s\n%s", must, out)
		}
	}
	if strings.Contains(out, "你好世界") {
		t.Error("用户文本未被脱敏")
	}
}

// TestScrub_PreservesReservedToolNames 协议内置工具名必须保留——
// 转换器按名识别服务端工具，改掉会让语料不再触发 web_search 等能力分支。
func TestScrub_PreservesReservedToolNames(t *testing.T) {
	body := `{"tools":[
		{"type":"web_search_20250305","name":"web_search","max_uses":5},
		{"name":"my_private_tool","description":"查询内部客户系统"}
	]}`
	out := scrubJSON(t, body)

	if !strings.Contains(out, `"name":"web_search"`) {
		t.Errorf("内置工具名必须保留: %s", out)
	}
	if !strings.Contains(out, `"type":"web_search_20250305"`) {
		t.Errorf("内置工具类型必须保留: %s", out)
	}
	if strings.Contains(out, "my_private_tool") {
		t.Error("自定义工具名应被替换")
	}
	if strings.Contains(out, "查询内部客户系统") {
		t.Error("工具描述应被脱敏")
	}
}

// TestScrub_LengthPreserved 自由文本按**字节长度**等长替换：
// token 估算、截断阈值、grounding 字节偏移校正都与长度相关，
// 长度塌缩会让语料不再覆盖这些路径。
func TestScrub_LengthPreserved(t *testing.T) {
	original := "这是一段中文提示词，包含若干内容用于测试长度保留是否正确"
	body := `{"messages":[{"role":"user","content":"` + original + `"}]}`
	out := scrubJSON(t, body)

	var parsed struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("不是合法 JSON: %v", err)
	}
	if got := len(parsed.Messages[0].Content); got != len(original) {
		t.Errorf("脱敏后字节长度 = %d, want %d", got, len(original))
	}
}

// TestScrub_Secrets 凭证形态无条件脱敏——用户完全可能把密钥粘进提示词。
func TestScrub_Secrets(t *testing.T) {
	for _, secret := range []string{
		"sk-proj-abcdefghijklmnopqrstuvwxyz123456",
		"AIzaSyD-abcdefghijklmnopqrstuvwxyz12345",
		"ghp_abcdefghijklmnopqrstuvwxyz1234567890",
	} {
		out := scrubJSON(t, `{"messages":[{"role":"user","content":"我的密钥是 `+secret+`"}]}`)
		if strings.Contains(out, secret) {
			t.Errorf("凭证未被脱敏: %s", secret)
		}
	}
	// 即使出现在「短且无空格、看起来像枚举」的位置也要脱敏
	out := scrubJSON(t, `{"custom_field":"sk-proj-abcdefghijklmnopqrstuvwxyz123456"}`)
	if strings.Contains(out, "sk-proj-abcdefghij") {
		t.Errorf("枚举位置的凭证未被脱敏: %s", out)
	}
}

// TestScrub_SessionIdentifiers 会话级标识符必须脱敏：codex 把同一会话 UUID 塞进
// 大量字段（含 x-codex-* 连字符 key 与任意自定义 key），key 名不可枚举——
// 已知 key 走 idKeys 表，未知 key 靠裸 UUID 值形态兜底。同值字段映射后仍应相等。
func TestScrub_SessionIdentifiers(t *testing.T) {
	const sessionUUID = "01a0916c-85ba-7121-8819-8d582bfe20e6"
	const installUUID = "aa11bb22-cc33-dd44-ee55-ff6677889900"
	body := `{"session_id":"` + sessionUUID + `","thread_id":"` + sessionUUID +
		`","turn_id":"` + sessionUUID + `","root_turn_id":"` + sessionUUID +
		`","prompt_cache_key":"` + sessionUUID +
		`","x-codex-window-id":"` + sessionUUID + `:0"` +
		`,"x-codex-installation-id":"` + installUUID +
		`","some_future_client_id_field":"` + installUUID +
		`","model":"gpt-5"}`

	out := scrubJSON(t, body)
	if strings.Contains(out, sessionUUID) || strings.Contains(out, installUUID) {
		t.Errorf("会话/安装标识符未被脱敏: %s", out)
	}

	var parsed struct {
		SessionID string `json:"session_id"`
		ThreadID  string `json:"thread_id"`
		TurnID    string `json:"turn_id"`
		Model     string `json:"model"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("不是合法 JSON: %v", err)
	}
	if parsed.SessionID != parsed.ThreadID || parsed.SessionID != parsed.TurnID {
		t.Errorf("同值字段映射后应仍相等: session=%s thread=%s turn=%s",
			parsed.SessionID, parsed.ThreadID, parsed.TurnID)
	}
	if parsed.Model != "gpt-5" {
		t.Errorf("行为字段 model 不应被改写: %s", parsed.Model)
	}
}

// TestScrub_InlineImageData 内联图片换成可解码的占位 PNG，
// data URL 的 mime 前缀保留（图片处理路径靠它判断格式）。
func TestScrub_InlineImageData(t *testing.T) {
	blob := strings.Repeat("QUJDREVG", 200)
	body := `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,` + blob + `"}}]}]}`
	out := scrubJSON(t, body)

	if strings.Contains(out, blob) {
		t.Error("图片数据未被替换")
	}
	if !strings.Contains(out, "data:image/jpeg;base64,") {
		t.Errorf("data URL 的 mime 前缀应保留: %s", out)
	}
	if !strings.Contains(out, placeholderPNG) {
		t.Error("应替换为可解码的占位图片")
	}

	out = scrubJSON(t, `{"image_url":{"url":"https://internal.corp.example/private/screenshot.png"}}`)
	if strings.Contains(out, "internal.corp.example") {
		t.Errorf("远程 URL 未被脱敏: %s", out)
	}
}

// TestScrub_Stream 流式脱敏保留事件名与帧结构，[DONE] 哨兵不动。
func TestScrub_Stream(t *testing.T) {
	stream := "event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"敏感回答内容\"}}\n\n" +
		"data: [DONE]\n\n"
	out := string(NewScrubber(ScrubOptions{Enabled: true}).ScrubStream([]byte(stream)))

	if !strings.Contains(out, "event: content_block_delta") {
		t.Errorf("事件名应保留: %s", out)
	}
	if !strings.Contains(out, `"type":"content_block_delta"`) {
		t.Errorf("判别式字段应保留: %s", out)
	}
	if strings.Contains(out, "敏感回答内容") {
		t.Error("流式文本未被脱敏")
	}
	if !strings.Contains(out, "data: [DONE]") {
		t.Errorf("[DONE] 哨兵应原样保留（它是协议的一部分）: %s", out)
	}
}

func TestScrub_Disabled(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"原文"}]}`
	if got := scrubJSON(t, body, ScrubOptions{Enabled: false}); got != body {
		t.Errorf("未启用脱敏应原样返回: %s", got)
	}
}

func TestScrub_KeepToolNames(t *testing.T) {
	out := scrubJSON(t, `{"tools":[{"name":"my_tool"}]}`, ScrubOptions{Enabled: true, KeepToolNames: true})
	if !strings.Contains(out, "my_tool") {
		t.Errorf("KeepToolNames=true 时应保留: %s", out)
	}
}

// TestScrub_SharedAcrossRequestAndResponse 同一记录的请求与响应共用 Scrubber，
// 标识符映射一致——否则两个语料文件之间的引用关系会对不上。
func TestScrub_SharedAcrossRequestAndResponse(t *testing.T) {
	r := Record{
		UpstreamStatus:   200,
		ClientReqBody:    []byte(`{"messages":[{"role":"tool","tool_call_id":"call_shared"}]}`),
		UpstreamRespBody: []byte(`{"choices":[{"message":{"tool_calls":[{"id":"call_shared"}]}}]}`),
		Conversion:       Conversion{ClientFormat: "openai", UpstreamFormat: "openai"},
	}
	samples := Build(r, NewScrubber(ScrubOptions{Enabled: true}))
	if len(samples) != 2 {
		t.Fatalf("应产出请求与响应两条语料: %d", len(samples))
	}

	ids := regexp.MustCompile(`call_\d+`)
	reqID := ids.FindString(string(samples[0].Body))
	respID := ids.FindString(string(samples[1].Body))
	if reqID == "" || reqID != respID {
		t.Errorf("请求与响应的同一标识符映射应一致: req=%q resp=%q", reqID, respID)
	}
}

// TestScrub_DedupStillWorks 脱敏后结构不变，因此同形状的记录仍应被去重。
// 若脱敏破坏了结构（如把数组换成字符串），去重率会骤降——这是回归信号。
func TestScrub_DedupStillWorks(t *testing.T) {
	c := NewCollector(1, ScrubOptions{Enabled: true})
	c.Add(mkRecord(1, `{"messages":[{"role":"user","content":"第一个问题"}]}`, "", false))
	c.Add(mkRecord(2, `{"messages":[{"role":"user","content":"另一个完全不同的问题内容"}]}`, "", false))

	if got := len(c.Samples()); got != 1 {
		t.Errorf("脱敏后同形状仍应去重为 1 条，实际 %d", got)
	}
}

// TestScrub_CJKFreeText 短中文必须脱敏：中文通常**不含空格**，「短且无空格即枚举」
// 的启发式对 CJK 失效，会把用户输入、思考内容原样放进语料。
func TestScrub_CJKFreeText(t *testing.T) {
	const zh = "先看仓库结构再改"
	out := scrubJSON(t, `{"custom_field":"`+zh+`","finish_reason":"stop"}`)
	if strings.Contains(out, zh) {
		t.Errorf("短中文未被脱敏: %s", out)
	}
	var parsed struct {
		CustomField  string `json:"custom_field"`
		FinishReason string `json:"finish_reason"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("不是合法 JSON: %v", err)
	}
	if parsed.FinishReason != "stop" {
		t.Errorf("ASCII 枚举不应被误伤: %s", parsed.FinishReason)
	}
	if len(parsed.CustomField) != len(zh) {
		t.Errorf("替换应等长: got %d want %d", len(parsed.CustomField), len(zh))
	}
}
