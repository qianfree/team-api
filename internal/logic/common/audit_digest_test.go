package common

import (
	"strings"
	"testing"
)

// ============================================================
// 会话关联摘要测试：核心是「双侧一致性」——同一轮的响应与下一轮请求中
// 回传的 assistant 消息，两侧哈希必须相等，衔接链才成立。
// 该测试是 M1 的一票否决门禁（见 docs/会话关联开发计划.md T1.4）。
// ============================================================

// oaiChainTurn1Req / oaiChainTurn1Resp / oaiChainTurn2Req OpenAI chat 三件套：
// 第 1 轮请求 + 响应，第 2 轮请求回传第 1 轮响应
const oaiChainTurn1Req = `{"model":"gpt-4o","messages":[` +
	`{"role":"system","content":"You are a helpful assistant."},` +
	`{"role":"user","content":"帮我写一个快速排序"}]}`

const oaiChainTurn1Resp = `{"choices":[{"message":{"role":"assistant","content":"好的，以下是快速排序实现：\nfunc quickSort..."}}]}`

const oaiChainTurn2Req = `{"model":"gpt-4o","messages":[` +
	`{"role":"system","content":"You are a helpful assistant."},` +
	`{"role":"user","content":"帮我写一个快速排序"},` +
	`{"role":"assistant","content":"好的，以下是快速排序实现：\nfunc quickSort..."},` +
	`{"role":"user","content":"改成 Python"}]}`

// oaiChainTurn1RespSSE 流式形态的第 1 轮响应（与非流式内容一致）
const oaiChainTurn1RespSSE = "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"好的，以下是快速排序实现：\\n\"}}]}\n\n" +
	"data: {\"choices\":[{\"delta\":{\"content\":\"func quickSort...\"}}]}\n\n" +
	"data: [DONE]\n\n"

func TestOpenAIChatDigestChain(t *testing.T) {
	d1 := ExtractSessionDigest("/chat/completions", oaiChainTurn1Req, oaiChainTurn1Resp, false, "", 1, "gpt-4o")
	d2 := ExtractSessionDigest("/chat/completions", oaiChainTurn2Req, "", false, "", 1, "gpt-4o")
	d1s := ExtractSessionDigest("/chat/completions", oaiChainTurn1Req, oaiChainTurn1RespSSE, true, "", 1, "gpt-4o")

	if d1.ConvFingerprint == "" {
		t.Fatal("第 1 轮应产生会话指纹")
	}
	if d1.ConvFingerprint != d2.ConvFingerprint {
		t.Errorf("同会话两轮指纹应相等: %s != %s", d1.ConvFingerprint, d2.ConvFingerprint)
	}
	if d1.RespMsgHash == "" {
		t.Fatal("第 1 轮响应应产生衔接哈希")
	}
	if d1.RespMsgHash != d2.LastAsstMsgHash {
		t.Errorf("衔接哈希不匹配（非流式）: resp=%s last_asst=%s", d1.RespMsgHash, d2.LastAsstMsgHash)
	}
	if d1s.RespMsgHash != d1.RespMsgHash {
		t.Errorf("流式与非流式汇聚结果应一致: stream=%s nonstream=%s", d1s.RespMsgHash, d1.RespMsgHash)
	}
	if d1.LastAsstMsgHash != "" {
		t.Errorf("首轮无 assistant 消息，LastAsstMsgHash 应为空，实际 %s", d1.LastAsstMsgHash)
	}
}

// oaiToolResp / oaiToolTurnReq 工具调用链：响应含 tool_calls，
// 下一轮请求回传该 assistant 消息（content 为 null）
const oaiToolResp = `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[` +
	`{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"北京\"}"}}]}}]}`

const oaiToolRespSSE = "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"\"}}]}}]}\n\n" +
	"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"city\\\"\"}}]}}]}\n\n" +
	"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\":\\\"北京\\\"}\"}}]}}]}\n\n" +
	"data: [DONE]\n\n"

const oaiToolTurnReq = `{"model":"gpt-4o","messages":[` +
	`{"role":"user","content":"查北京天气"},` +
	`{"role":"assistant","content":null,"tool_calls":[` +
	`{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"北京\"}"}}]},` +
	`{"role":"tool","tool_call_id":"call_1","content":"{\"temp\":25}"},` +
	`{"role":"user","content":"谢谢，明天呢"}]}`

func TestOpenAIToolCallDigestChain(t *testing.T) {
	d1 := ExtractSessionDigest("/chat/completions", oaiToolTurnReq, oaiToolResp, false, "", 1, "gpt-4o")
	d1s := ExtractSessionDigest("/chat/completions", oaiToolTurnReq, oaiToolRespSSE, true, "", 1, "gpt-4o")

	if d1.RespMsgHash == "" {
		t.Fatal("含 tool_calls 的响应应产生衔接哈希")
	}
	if d1.RespMsgHash != d1.LastAsstMsgHash {
		t.Errorf("同一条记录里请求末条 assistant（回传上一轮响应）与响应本身应同哈希: %s != %s",
			d1.RespMsgHash, d1.LastAsstMsgHash)
	}
	if d1s.RespMsgHash != d1.RespMsgHash {
		t.Errorf("流式工具调用分片拼接应与非流式一致: stream=%s nonstream=%s", d1s.RespMsgHash, d1.RespMsgHash)
	}
}

// Claude 链：thinking 块两侧一致跳过、tool_use 进工具结构、流式按事件汇聚
const claudeChainTurn1Req = `{"model":"claude-sonnet-4","system":"你是一个编程助手",` +
	`"messages":[{"role":"user","content":"解释一下闭包"}]}`

const claudeChainTurn1Resp = `{"content":[{"type":"text","text":"闭包是指..."}]}`

const claudeChainTurn2Req = `{"model":"claude-sonnet-4","system":"你是一个编程助手","messages":[` +
	`{"role":"user","content":"解释一下闭包"},` +
	`{"role":"assistant","content":[` +
	`{"type":"thinking","thinking":"用户问闭包的定义","signature":"sig-abc"},` +
	`{"type":"text","text":"闭包是指..."}]},` +
	`{"role":"user","content":"再举个例子"}]}`

const claudeToolResp = `{"content":[` +
	`{"type":"text","text":"我查一下"},` +
	`{"type":"tool_use","id":"toolu_1","name":"search","input":{"q":"closure"}}]}`

const claudeToolRespSSE = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{}}\n\n" +
	"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
	"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"我查一下\"}}\n\n" +
	"event: content_block_end\ndata: {\"type\":\"content_block_end\",\"index\":0}\n\n" +
	"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"search\",\"input\":{}}}\n\n" +
	"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"q\\\":\"}}\n\n" +
	"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"closure\\\"}\"}}\n\n" +
	"event: content_block_end\ndata: {\"type\":\"content_block_end\",\"index\":1}\n\n" +
	"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\n" +
	"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

const claudeToolTurnReq = `{"model":"claude-sonnet-4","messages":[` +
	`{"role":"user","content":"什么是闭包"},` +
	`{"role":"assistant","content":[` +
	`{"type":"text","text":"我查一下"},` +
	`{"type":"tool_use","id":"toolu_1","name":"search","input":{"q":"closure"}}]},` +
	`{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"找到 3 篇"}]},` +
	`{"role":"user","content":"总结一下"}]}`

func TestClaudeDigestChain(t *testing.T) {
	d1 := ExtractSessionDigest("/messages", claudeChainTurn1Req, claudeChainTurn1Resp, false, "", 1, "claude-sonnet-4")
	d2 := ExtractSessionDigest("/messages", claudeChainTurn2Req, "", false, "", 1, "claude-sonnet-4")

	if d1.ConvFingerprint == "" || d1.ConvFingerprint != d2.ConvFingerprint {
		t.Errorf("Claude 同会话指纹应相等: %s != %s", d1.ConvFingerprint, d2.ConvFingerprint)
	}
	// thinking 块两侧一致跳过：请求侧回传含 thinking 的 assistant 消息，
	// 响应侧（无 thinking）哈希仍应匹配
	if d1.RespMsgHash != d2.LastAsstMsgHash {
		t.Errorf("Claude 衔接哈希不匹配（thinking 应被两侧一致跳过）: %s != %s",
			d1.RespMsgHash, d2.LastAsstMsgHash)
	}

	dt := ExtractSessionDigest("/messages", claudeToolTurnReq, claudeToolResp, false, "", 1, "claude-sonnet-4")
	dts := ExtractSessionDigest("/messages", claudeToolTurnReq, claudeToolRespSSE, true, "", 1, "claude-sonnet-4")
	if dt.RespMsgHash == "" || dt.RespMsgHash != dt.LastAsstMsgHash {
		t.Errorf("Claude tool_use 哈希不匹配: resp=%s last_asst=%s", dt.RespMsgHash, dt.LastAsstMsgHash)
	}
	if dts.RespMsgHash != dt.RespMsgHash {
		t.Errorf("Claude 流式事件汇聚应与非流式一致: stream=%s nonstream=%s", dts.RespMsgHash, dt.RespMsgHash)
	}
}

// Gemini 链：role model→assistant、systemInstruction、流式分片
const geminiTurn1Req = `{"contents":[{"role":"user","parts":[{"text":"你好"}]}],` +
	`"systemInstruction":{"parts":[{"text":"你是助手"}]}}`

const geminiTurn1Resp = `{"candidates":[{"content":{"role":"model","parts":[{"text":"你好！"}]}}]}`

const geminiTurn1RespSSE = "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"你\"}]}}]}\n\n" +
	"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"好！\"}]}}]}\n\n"

const geminiTurn2Req = `{"contents":[` +
	`{"role":"user","parts":[{"text":"你好"}]},` +
	`{"role":"model","parts":[{"text":"你好！"}]},` +
	`{"role":"user","parts":[{"text":"再见"}]}],` +
	`"systemInstruction":{"parts":[{"text":"你是助手"}]}}`

func TestGeminiDigestChain(t *testing.T) {
	d1 := ExtractSessionDigest("/models/gemini-2.0-flash:generateContent", geminiTurn1Req, geminiTurn1Resp, false, "", 1, "gemini-2.0-flash")
	d2 := ExtractSessionDigest("/models/gemini-2.0-flash:generateContent", geminiTurn2Req, "", false, "", 1, "gemini-2.0-flash")
	d1s := ExtractSessionDigest("/models/gemini-2.0-flash:streamGenerateContent", geminiTurn1Req, geminiTurn1RespSSE, true, "", 1, "gemini-2.0-flash")

	if d1.ConvFingerprint == "" || d1.ConvFingerprint != d2.ConvFingerprint {
		t.Errorf("Gemini 同会话指纹应相等: %s != %s", d1.ConvFingerprint, d2.ConvFingerprint)
	}
	if d1.RespMsgHash != d2.LastAsstMsgHash {
		t.Errorf("Gemini 衔接哈希不匹配: %s != %s", d1.RespMsgHash, d2.LastAsstMsgHash)
	}
	if d1s.RespMsgHash != d1.RespMsgHash {
		t.Errorf("Gemini 流式汇聚应与非流式一致: stream=%s nonstream=%s", d1s.RespMsgHash, d1.RespMsgHash)
	}
}

// Responses 链：input 字符串与 item 数组两种形态、instructions、response.completed
const responsesTurn1Req = `{"model":"gpt-5","instructions":"你是助手","input":"你好"}`

const responsesTurn1Resp = `{"output":[` +
	`{"type":"reasoning","summary":[]},` +
	`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"你好！有什么问题？"}]}]}`

const responsesTurn1RespSSE = "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"你好！\"}\n\n" +
	"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[" +
	"{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"你好！有什么问题？\"}]}]}}\n\n"

const responsesTurn2Req = `{"model":"gpt-5","instructions":"你是助手","input":[` +
	`{"type":"message","role":"user","content":[{"type":"input_text","text":"你好"}]},` +
	`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"你好！有什么问题？"}]},` +
	`{"type":"message","role":"user","content":[{"type":"input_text","text":"第二个问题"}]}]}`

func TestResponsesDigestChain(t *testing.T) {
	d1 := ExtractSessionDigest("/responses", responsesTurn1Req, responsesTurn1Resp, false, "", 1, "gpt-5")
	d2 := ExtractSessionDigest("/responses", responsesTurn2Req, "", false, "", 1, "gpt-5")
	d1s := ExtractSessionDigest("/responses", responsesTurn1Req, responsesTurn1RespSSE, true, "", 1, "gpt-5")

	// input 字符串形态与 item 数组形态的首条 user 规范化结果应一致（指纹相等）
	if d1.ConvFingerprint == "" || d1.ConvFingerprint != d2.ConvFingerprint {
		t.Errorf("Responses 两种 input 形态指纹应相等: %s != %s", d1.ConvFingerprint, d2.ConvFingerprint)
	}
	if d1.RespMsgHash != d2.LastAsstMsgHash {
		t.Errorf("Responses 衔接哈希不匹配: %s != %s", d1.RespMsgHash, d2.LastAsstMsgHash)
	}
	if d1s.RespMsgHash != d1.RespMsgHash {
		t.Errorf("Responses 流式（response.completed）应与非流式一致: stream=%s nonstream=%s",
			d1s.RespMsgHash, d1.RespMsgHash)
	}
}

func TestSessionKeySources(t *testing.T) {
	anthropicUserID := "user_abc_account-def_session_550e8400-e29b-41d4-a716-446655440000"
	cases := []struct {
		name       string
		path       string
		body       string
		header     string
		wantSource string
		wantEmpty  bool
	}{
		{"显式头", "/chat/completions", oaiChainTurn1Req, "sess-abc-123", "header", false},
		{"Claude metadata", "/messages",
			`{"metadata":{"user_id":"` + anthropicUserID + `"},"messages":[]}`, "", "anthropic", false},
		{"Responses conversation_id", "/responses",
			`{"conversation_id":"conv_abc","input":"hi"}`, "", "openai", false},
		// previous_response_id 每轮漂移，不产生审计会话键（调度亲和键专用）；
		// 有状态 Responses 无 conversation_id 时落到指纹/身份层
		{"仅 previous_response_id", "/responses",
			`{"previous_response_id":"resp_123","input":"hi"}`, "", "", true},
		{"无信号", "/chat/completions", oaiChainTurn1Req, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := ExtractSessionDigest(tc.path, tc.body, "", false, tc.header, 7, "gpt-4o")
			if tc.wantEmpty {
				if d.SessionKey != "" || d.SessionSource != "" {
					t.Errorf("无显式信号时不应落会话键（身份回退不落库）: key=%s source=%s",
						d.SessionKey, d.SessionSource)
				}
				return
			}
			if !strings.HasPrefix(d.SessionKey, "sk:") {
				t.Errorf("会话键应带 sk: 前缀: %s", d.SessionKey)
			}
			if d.SessionSource != tc.wantSource {
				t.Errorf("来源不符: want=%s got=%s", tc.wantSource, d.SessionSource)
			}
			// 同信号不同租户应得到不同键（哈希材料带租户命名空间）
			d2 := ExtractSessionDigest(tc.path, tc.body, "", false, tc.header, 8, "gpt-4o")
			if d2.SessionKey == d.SessionKey {
				t.Errorf("不同租户同信号会话键应不同: %s", d.SessionKey)
			}
		})
	}
}

func TestConversationKeyStableAcrossTurns(t *testing.T) {
	// 会话稳定键的核心性质：同会话各轮键相同、跨会话键不同。
	// previous_response_id 每轮变化（曾导致 Codex 有状态会话聚合失败），
	// 只有 conversation_id / 显式头 / session UUID 参与
	t1 := ExtractSessionDigest("/responses",
		`{"conversation_id":"conv_same","previous_response_id":"resp_1","input":"第一轮"}`,
		"", false, "", 7, "gpt-5")
	t2 := ExtractSessionDigest("/responses",
		`{"conversation_id":"conv_same","previous_response_id":"resp_2","input":"第二轮"}`,
		"", false, "", 7, "gpt-5")
	t3 := ExtractSessionDigest("/responses",
		`{"conversation_id":"conv_other","input":"另一会话"}`,
		"", false, "", 7, "gpt-5")

	if t1.SessionKey == "" || t1.SessionKey != t2.SessionKey {
		t.Errorf("同 conversation_id 各轮键应相等: %s != %s", t1.SessionKey, t2.SessionKey)
	}
	if t3.SessionKey == t1.SessionKey {
		t.Errorf("不同会话键应不同: %s", t1.SessionKey)
	}
}

func TestNonConversationPath(t *testing.T) {
	// 非对话端点只解析会话键，内容字段留空
	d := ExtractSessionDigest("/embeddings", `{"model":"text-embedding-3-small","input":"hello"}`,
		`{"data":[{"embedding":[0.1]}]}`, false, "sess-1", 1, "text-embedding-3-small")
	if d.SessionKey == "" || d.SessionSource != "header" {
		t.Errorf("非对话端点仍应有会话键: key=%s source=%s", d.SessionKey, d.SessionSource)
	}
	if d.ConvFingerprint != "" || d.RespMsgHash != "" || d.LastAsstMsgHash != "" {
		t.Errorf("非对话端点不应产生内容摘要: %+v", d)
	}

	// 任务路径（真实 URL path 形态）同理
	dt := ExtractSessionDigest("/v1/video/generations", `{"model":"kling-v1","prompt":"cat"}`,
		`{"task_id":"t_1"}`, false, "", 1, "kling-v1")
	if dt.SessionKey != "" || dt.ConvFingerprint != "" {
		t.Errorf("任务路径无信号时不应有任何键: %+v", dt)
	}
}

func TestMediaPlaceholderDeterminism(t *testing.T) {
	// 同结构不同 base64 载荷 → 媒体占位与长度无关，指纹应一致
	req1 := `{"messages":[{"role":"user","content":[` +
		`{"type":"text","text":"这张图是什么"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("A", 800) + `"}}]}]}`
	req2 := `{"messages":[{"role":"user","content":[` +
		`{"type":"text","text":"这张图是什么"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("B", 300) + `"}}]}]}`
	d1 := ExtractSessionDigest("/chat/completions", req1, "", false, "", 1, "gpt-4o")
	d2 := ExtractSessionDigest("/chat/completions", req2, "", false, "", 1, "gpt-4o")
	if d1.ConvFingerprint == "" || d1.ConvFingerprint != d2.ConvFingerprint {
		t.Errorf("媒体载荷长度不同不应影响指纹: %s != %s", d1.ConvFingerprint, d2.ConvFingerprint)
	}
}

func TestContextResetMarkers(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"Cline 摘要", `{"messages":[{"role":"user","content":"[Conversation summarized as of 10:00] 继续"}]}`, true},
		{"Claude Code 压缩中断", `{"messages":[{"role":"user","content":"[Request interrupted by user for compaction]"}]}`, true},
		{"Claude Code 续接", `{"messages":[{"role":"user","content":"This session is being continued from a previous conversation..."}]}`, true},
		{"普通对话", `{"messages":[{"role":"user","content":"正常内容"}]}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := ExtractSessionDigest("/chat/completions", tc.body, "", false, "", 1, "gpt-4o")
			if d.IsContextReset != tc.want {
				t.Errorf("IsContextReset: want=%v got=%v", tc.want, d.IsContextReset)
			}
		})
	}
}

func TestDigestDegenerateAndInvalid(t *testing.T) {
	// 空文本且无工具调用的 assistant 消息不产哈希（防大量空哈希互相误链）
	req := `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":null},{"role":"user","content":"again"}]}`
	d := ExtractSessionDigest("/chat/completions", req, "", false, "", 1, "gpt-4o")
	if d.LastAsstMsgHash != "" {
		t.Errorf("退化 assistant 消息不应产生哈希: %s", d.LastAsstMsgHash)
	}

	// 二进制占位体 / 非法 JSON：不 panic，只出会话键
	d2 := ExtractSessionDigest("/chat/completions", "[binary body omitted]", "", false, "sess-x", 1, "gpt-4o")
	if d2.SessionSource != "header" || d2.ConvFingerprint != "" {
		t.Errorf("二进制体应只出会话键: %+v", d2)
	}
	d3 := ExtractSessionDigest("/chat/completions", "{not-json", "", false, "", 1, "gpt-4o")
	if d3.ConvFingerprint != "" || d3.SessionKey != "" {
		t.Errorf("非法 JSON 不应产生任何键: %+v", d3)
	}

	// 响应非法 JSON / 空响应：衔接哈希留空不 panic
	d4 := ExtractSessionDigest("/chat/completions", oaiChainTurn1Req, "{bad", false, "", 1, "gpt-4o")
	if d4.RespMsgHash != "" {
		t.Errorf("非法响应不应产生衔接哈希: %s", d4.RespMsgHash)
	}
	d5 := ExtractSessionDigest("/chat/completions", oaiChainTurn1Req, "", true, "", 1, "gpt-4o")
	if d5.RespMsgHash != "" {
		t.Errorf("空响应不应产生衔接哈希: %s", d5.RespMsgHash)
	}
}

func TestUserFieldAndTruncatedStream(t *testing.T) {
	// user 字段提取（hint）
	req := `{"model":"gpt-4o","user":"user-789","messages":[{"role":"user","content":"hi"}]}`
	d := ExtractSessionDigest("/chat/completions", req, "", false, "", 1, "gpt-4o")
	if d.UserField != "user-789" {
		t.Errorf("user 字段提取失败: %q", d.UserField)
	}

	// 流被截断（完整行 + 半行，无 [DONE]）：完整行仍可汇聚，半行跳过，不 panic
	truncated := "data: {\"choices\":[{\"delta\":{\"content\":\"部分内容\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"cont"
	d2 := ExtractSessionDigest("/chat/completions", req, truncated, true, "", 1, "gpt-4o")
	if d2.RespMsgHash == "" {
		t.Error("截断流的部分文本也应产生哈希（确定性降级而非丢弃）")
	}
}
