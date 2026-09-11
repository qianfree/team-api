package handler

// e2e_relay_stream_test.go — 端到端流式与错误链路（假上游）。
//
// 与 e2e_relay_test.go 共用驱动（runE2E），覆盖非流式测不到的两类性质：
//   - 流式终止语义：各客户端协议的收尾契约互不相同，写错会让官方 SDK 直接报错
//     （Gemini 客户端多发一个 [DONE] 会让 @google/genai 的 JSON.parse 抛 SyntaxError，
//     Responses 客户端缺 response.completed 会让 codex 一直挂起等待）；
//   - 错误映射：上游故障时客户端必须收到**入站协议的原生错误格式**，
//     而不是网关统一 JSON 或另一种协议的错误体。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

// ---------- 流式上游语料 ----------

var e2eUpstreamStreams = map[constant.RelayFormat]string{
	constant.RelayFormatOpenAI: "data: {\"id\":\"chatcmpl-up\",\"object\":\"chat.completion.chunk\",\"created\":1700000000,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-up\",\"object\":\"chat.completion.chunk\",\"created\":1700000000,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi there!\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-up\",\"object\":\"chat.completion.chunk\",\"created\":1700000000,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":7,\"total_tokens\":19}}\n\n" +
		"data: [DONE]\n\n",

	constant.RelayFormatClaude: "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_up\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"upstream-model\",\"stop_reason\":null,\"usage\":{\"input_tokens\":12,\"output_tokens\":1}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi there!\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":7}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",

	constant.RelayFormatGemini: "data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hi there!\"}]}}]}\n\n" +
		"data: {\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":12,\"candidatesTokenCount\":7,\"totalTokenCount\":19}}\n\n",

	constant.RelayFormatResponses: "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_up\",\"object\":\"response\",\"created_at\":1700000000,\"status\":\"in_progress\",\"model\":\"upstream-model\"}}\n\n" +
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_up\",\"output_index\":0,\"content_index\":0,\"delta\":\"Hi there!\"}\n\n" +
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_up\",\"object\":\"response\",\"created_at\":1700000000,\"status\":\"completed\",\"model\":\"upstream-model\",\"usage\":{\"input_tokens\":12,\"output_tokens\":7,\"total_tokens\":19}}}\n\n" +
		"data: [DONE]\n\n",

	constant.RelayFormatOllama: "{\"model\":\"upstream-model\",\"created_at\":\"2026-01-01T00:00:00Z\",\"message\":{\"role\":\"assistant\",\"content\":\"Hi there!\"},\"done\":false}\n" +
		"{\"model\":\"upstream-model\",\"created_at\":\"2026-01-01T00:00:01Z\",\"message\":{\"role\":\"assistant\",\"content\":\"\"},\"done\":true,\"done_reason\":\"stop\",\"prompt_eval_count\":12,\"eval_count\":7}\n",
}

// ---------- SSE 解析 ----------

type sseFrame struct {
	event string
	data  string
}

// parseSSE 把客户端收到的字节流拆成帧（按空行分隔，聚合 event/data 行）。
func parseSSE(raw string) []sseFrame {
	var frames []sseFrame
	for _, block := range strings.Split(raw, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var f sseFrame
		var dataLines []string
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event:"):
				f.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if len(dataLines) == 0 && f.event == "" {
			continue // 保活注释等非事件块
		}
		f.data = strings.Join(dataLines, "\n")
		frames = append(frames, f)
	}
	return frames
}

func eventNames(frames []sseFrame) []string {
	out := make([]string, 0, len(frames))
	for _, f := range frames {
		out = append(out, f.event)
	}
	return out
}

// ---------- 流式方向矩阵 ----------

func e2eStreamCases() []e2eCase {
	mk := func(name string, provider constant.ProviderType, inbound constant.RelayFormat,
		mode constant.RelayMode, upstreamFormat constant.RelayFormat) e2eCase {
		return e2eCase{
			name:                name,
			provider:            provider,
			inbound:             inbound,
			mode:                mode,
			stream:              true,
			clientBody:          e2eClientRequests[inbound],
			wantUpstreamFormat:  upstreamFormat,
			upstreamContentType: "text/event-stream",
			upstreamBody:        e2eUpstreamStreams[upstreamFormat],
		}
	}

	return []e2eCase{
		// OpenAI 客户端（收尾需 [DONE]）
		mk("openai客户端←claude上游", constant.ProviderClaude, constant.RelayFormatOpenAI,
			constant.RelayModeChatCompletions, constant.RelayFormatClaude),
		mk("openai客户端←gemini上游", constant.ProviderGemini, constant.RelayFormatOpenAI,
			constant.RelayModeChatCompletions, constant.RelayFormatGemini),
		mk("openai客户端←ollama上游", constant.ProviderOllama, constant.RelayFormatOpenAI,
			constant.RelayModeChatCompletions, constant.RelayFormatOllama),

		// Claude 客户端（收尾需 message_stop，禁 [DONE]）
		mk("claude客户端←openai上游", constant.ProviderOpenAI, constant.RelayFormatClaude,
			constant.RelayModeClaudeMessages, constant.RelayFormatOpenAI),
		mk("claude客户端←gemini上游", constant.ProviderGemini, constant.RelayFormatClaude,
			constant.RelayModeClaudeMessages, constant.RelayFormatGemini),

		// Gemini 客户端（收尾禁 [DONE]——官方 SDK 对每个 data 帧 JSON.parse）
		mk("gemini客户端←openai上游", constant.ProviderOpenAI, constant.RelayFormatGemini,
			constant.RelayModeGeminiChat, constant.RelayFormatOpenAI),
		mk("gemini客户端←claude上游", constant.ProviderClaude, constant.RelayFormatGemini,
			constant.RelayModeGeminiChat, constant.RelayFormatClaude),

		// Responses 客户端（收尾需 response.completed）
		mk("responses客户端←openai上游", constant.ProviderOpenAI, constant.RelayFormatResponses,
			constant.RelayModeResponses, constant.RelayFormatOpenAI),
		mk("responses客户端←claude上游", constant.ProviderClaude, constant.RelayFormatResponses,
			constant.RelayModeResponses, constant.RelayFormatClaude),
		mk("responses客户端←gemini上游", constant.ProviderGemini, constant.RelayFormatResponses,
			constant.RelayModeResponses, constant.RelayFormatGemini),
	}
}

// TestE2E_StreamTerminalSemantics 断言 3（流式闭环）：客户端收到的 SSE 必须符合
// 入站协议的收尾契约，且每个 data 帧都是可解析的 JSON、用量可被计费捕获。
func TestE2E_StreamTerminalSemantics(t *testing.T) {
	for _, c := range e2eStreamCases() {
		t.Run(c.name, func(t *testing.T) {
			res := runE2E(t, c)
			if res.err != nil {
				t.Fatalf("链路失败: %v", res.err)
			}

			frames := parseSSE(res.clientBody)
			if len(frames) == 0 {
				t.Fatalf("客户端未收到任何 SSE 帧\n原始输出: %s", truncate([]byte(res.clientBody)))
			}

			// 每个 data 帧必须是合法 JSON（[DONE] 哨兵除外）：
			// 写出空 data 或畸形 JSON 会让客户端 SDK 解析失败并中止整条请求，
			// 而网关侧只会看到 ctx 取消、记成 client_gone（问题被掩盖）
			hasDone := false
			for i, f := range frames {
				if f.data == "[DONE]" {
					hasDone = true
					continue
				}
				if f.data == "" {
					t.Errorf("第 %d 帧（event=%q）的 data 为空——客户端 JSON.parse(\"\") 会抛错", i, f.event)
					continue
				}
				var probe any
				if err := json.Unmarshal([]byte(f.data), &probe); err != nil {
					t.Errorf("第 %d 帧（event=%q）的 data 不是合法 JSON: %v\n内容: %s", i, f.event, err, truncate([]byte(f.data)))
				}
			}

			// 收尾契约按客户端协议区分
			switch c.inbound {
			case constant.RelayFormatOpenAI:
				if !hasDone {
					t.Error("OpenAI 客户端流缺少 data: [DONE] 收尾哨兵")
				}
			case constant.RelayFormatGemini:
				if hasDone {
					t.Error("Gemini 客户端流写出了 [DONE]——官方 SDK（@google/genai）对每个 data 帧做 JSON.parse，会抛 SyntaxError")
				}
				if !strings.Contains(res.clientBody, "finishReason") {
					t.Errorf("Gemini 客户端流缺少带 finishReason 的终止帧\n原始输出: %s", truncate([]byte(res.clientBody)))
				}
			case constant.RelayFormatClaude:
				if hasDone {
					t.Error("Claude 客户端流写出了 [DONE]——Claude 协议以 message_stop 事件收尾，不使用该哨兵")
				}
				if !containsEvent(frames, "message_stop") {
					t.Errorf("Claude 客户端流缺少 message_stop 终止事件（客户端会一直等待）\n事件序列: %v", eventNames(frames))
				}
			case constant.RelayFormatResponses:
				if !containsEvent(frames, "response.completed") {
					t.Errorf("Responses 客户端流缺少 response.completed 终止事件（codex 等客户端会挂起）\n事件序列: %v", eventNames(frames))
				}
			}

			if !strings.Contains(res.clientBody, "Hi there!") {
				t.Errorf("上游回答内容在流式转换中丢失\n原始输出: %s", truncate([]byte(res.clientBody)))
			}

			// 计费闭环
			if res.usage == nil {
				t.Fatal("流式未返回用量（计费将按 0 token 结算）")
			}
			if res.usage.PromptTokens == 0 || res.usage.CompletionTokens == 0 {
				t.Errorf("流式用量提取不完整: prompt=%d completion=%d（上游语料为 12/7）",
					res.usage.PromptTokens, res.usage.CompletionTokens)
			}
		})
	}
}

func containsEvent(frames []sseFrame, name string) bool {
	for _, f := range frames {
		if f.event == name {
			return true
		}
		// 无 event 行时退回按 data 里的 type 字段判定（部分方向以纯 data 帧承载事件）
		if f.event == "" && strings.Contains(f.data, `"type":"`+name+`"`) {
			return true
		}
	}
	return false
}

// ---------- 错误映射 ----------

// errorShapeCheck 各入站协议的原生错误体判据：客户端必须拿到自己协议的错误格式，
// 而不是网关统一 JSON、也不是另一种协议的错误体（SDK 会解析失败）。
var errorShapeCheck = map[constant.RelayFormat]func(t *testing.T, body string){
	constant.RelayFormatOpenAI: func(t *testing.T, body string) {
		t.Helper()
		var obj struct {
			Error *struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &obj); err != nil || obj.Error == nil {
			t.Errorf("OpenAI 客户端应收到 {\"error\":{...}} 形态的错误体（err=%v）\n实际内容: %s", err, truncate([]byte(body)))
		}
	},
	constant.RelayFormatClaude: func(t *testing.T, body string) {
		t.Helper()
		var obj struct {
			Type  string `json:"type"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &obj); err != nil || obj.Error == nil || obj.Type != "error" {
			t.Errorf("Claude 客户端应收到 {\"type\":\"error\",\"error\":{...}} 形态的错误体（err=%v）\n实际内容: %s",
				err, truncate([]byte(body)))
		}
	},
	constant.RelayFormatGemini: func(t *testing.T, body string) {
		t.Helper()
		// Gemini 错误体为 {"error":{"code","message","status"}}（也接受数组包装形态）
		trimmed := strings.TrimSpace(body)
		if strings.HasPrefix(trimmed, "[") {
			trimmed = strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
		}
		var obj struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil || obj.Error == nil {
			t.Errorf("Gemini 客户端应收到 {\"error\":{...}} 形态的错误体（err=%v）\n实际内容: %s", err, truncate([]byte(body)))
		}
	},
	constant.RelayFormatResponses: func(t *testing.T, body string) {
		t.Helper()
		var obj struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &obj); err != nil || obj.Error == nil {
			t.Errorf("Responses 客户端应收到 {\"error\":{...}} 形态的错误体（err=%v）\n实际内容: %s", err, truncate([]byte(body)))
		}
	},
}

// hostErrorWriterFor 返回宿主按入站格式分派的错误写出器
// （与 internal/handler/relay 的真实接线一致：/v1/messages → Claude、
// /v1beta/* → Gemini、chat 与 responses → OpenAI）。
func hostErrorWriterFor(inbound constant.RelayFormat) func(http.ResponseWriter, error) {
	switch inbound {
	case constant.RelayFormatClaude:
		return WriteClaudeRelayError
	case constant.RelayFormatGemini:
		return WriteGeminiRelayError
	default:
		return WriteRelayError
	}
}

// TestE2E_UpstreamErrorMapping 断言 4（错误映射）：上游返回 4xx/5xx 时，
// 客户端最终收到的必须是入站协议的原生错误格式，且链路向上报错
// （供调度按上游故障扣健康分 / 换渠道）。
//
// 两种写出路径都要覆盖，否则测试会在「adaptor 没写、宿主才写」的方向上静默空过：
//   - adaptor 内写出（置 ResponseWritten）：直接校验其字节；
//   - adaptor 只上抛（未写出）：按宿主真实接线调用 Write*RelayError 再校验。
//
// 两条路径的判定结果也一并断言（expectAdaptorWrites），行为漂移时立即可见。
func TestE2E_UpstreamErrorMapping(t *testing.T) {
	upstreamErrBodies := map[constant.RelayFormat]string{
		constant.RelayFormatOpenAI: `{"error":{"message":"rate limit exceeded","type":"rate_limit_error","code":"rate_limit"}}`,
		constant.RelayFormatClaude: `{"type":"error","error":{"type":"rate_limit_error","message":"rate limit exceeded"}}`,
		constant.RelayFormatGemini: `{"error":{"code":429,"message":"rate limit exceeded","status":"RESOURCE_EXHAUSTED"}}`,
	}

	// 当前各方向的错误写出归属：OpenAI 上游的三个入站方向由 adaptor 就地改写为
	// 入站协议错误体（置 ResponseWritten，上层不再重写、也不换渠道重试）；
	// Claude/Gemini 上游方向只上抛错误，由宿主统一写出（保留换渠道重试的可能）。
	expectAdaptorWrites := map[string]bool{
		"claude→openai":    true,
		"gemini→openai":    true,
		"responses→openai": true,
	}

	for _, c := range e2eNonStreamCases() {
		// 同格式方向不参与：上游错误原样透传，没有跨协议映射行为可测
		//（openai→openai / claude→claude 直连基线、gemini→vertex(gemini) 等）
		if c.inbound == c.wantUpstreamFormat {
			continue
		}
		errBody, ok := upstreamErrBodies[c.wantUpstreamFormat]
		if !ok {
			continue // 上游格式无错误语料（ollama 等）
		}

		t.Run(c.name, func(t *testing.T) {
			ec := c
			ec.upstreamStatus = http.StatusTooManyRequests
			ec.upstreamBody = errBody
			res := runE2E(t, ec)

			// 上游故障必须向上报错：吞掉错误会让调度器把故障渠道判成健康
			if res.err == nil {
				t.Fatal("上游 429 未向上返回错误（调度器无法据此扣健康分 / 换渠道）")
			}

			adaptorWrote := res.clientBody != ""
			if want := expectAdaptorWrites[c.name]; adaptorWrote != want {
				t.Errorf("错误写出归属变化：adaptor 写出 = %v，期望 %v\n"+
					"→ 若为有意变更请更新 expectAdaptorWrites；否则可能是 ResponseWritten 标记漏置/误置"+
					"（漏置导致错误体被写两次，误置导致本可重试的请求不再换渠道）", adaptorWrote, want)
			}

			// 取客户端最终可见的字节：adaptor 已写出就用它，否则按宿主接线补写
			clientBody := res.clientBody
			if !adaptorWrote {
				rec := httptest.NewRecorder()
				hostErrorWriterFor(c.inbound)(rec, res.err)
				clientBody = rec.Body.String()
				if clientBody == "" {
					t.Fatal("adaptor 与宿主都没有写出错误体，客户端将收到空响应")
				}
			}

			if check, ok := errorShapeCheck[c.inbound]; ok {
				check(t, clientBody)
			}
			// 上游的人类可读错误消息必须传达到客户端，而不是被替换成通用文案或整坨转义 JSON
			if !strings.Contains(clientBody, "rate limit exceeded") {
				t.Errorf("上游错误消息在映射中丢失\n实际内容: %s", truncate([]byte(clientBody)))
			}
			if strings.Contains(clientBody, `\"error\"`) {
				t.Errorf("错误体被双重编码（上游 JSON 原文被塞进 message 字段），客户端拿到的是一坨转义 JSON 而非可读消息\n实际内容: %s",
					truncate([]byte(clientBody)))
			}
		})
	}
}

// TestE2E_ClaudeInboundErrorTypeMapping Claude 入站错误类型映射：
// error.type 是 Anthropic SDK 的分类与退避依据，必须按上游真实错误类型/状态码映射，
// 不能一律 api_error（限流被当成服务端故障会让客户端退避策略失效）。
func TestE2E_ClaudeInboundErrorTypeMapping(t *testing.T) {
	cases := []struct {
		name           string
		upstreamStatus int
		upstreamBody   string
		wantType       string
	}{
		{"限流", http.StatusTooManyRequests,
			`{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`, "rate_limit_error"},
		{"鉴权失败", http.StatusUnauthorized,
			`{"error":{"message":"invalid api key","type":"authentication_error"}}`, "authentication_error"},
		{"参数非法", http.StatusBadRequest,
			`{"error":{"message":"bad param","type":"invalid_request_error"}}`, "invalid_request_error"},
		{"服务端故障", http.StatusInternalServerError,
			`{"error":{"message":"boom","type":"server_error"}}`, "api_error"},
		// 上游未给出 type：按状态码兜底，仍优于一律 api_error
		{"无类型按状态码兜底", http.StatusTooManyRequests,
			`{"error":{"message":"slow down"}}`, "rate_limit_error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runE2E(t, e2eCase{
				provider:       constant.ProviderOpenAI,
				inbound:        constant.RelayFormatClaude,
				mode:           constant.RelayModeClaudeMessages,
				clientBody:     e2eClientRequests[constant.RelayFormatClaude],
				upstreamStatus: tc.upstreamStatus,
				upstreamBody:   tc.upstreamBody,
			})
			if res.clientBody == "" {
				t.Fatal("adaptor 未写出 Claude 错误体")
			}

			var obj struct {
				Type  string `json:"type"`
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(res.clientBody), &obj); err != nil {
				t.Fatalf("错误体不是合法 JSON: %v\n内容: %s", err, res.clientBody)
			}
			if obj.Type != "error" {
				t.Errorf("顶层 type = %q，期望 \"error\"", obj.Type)
			}
			if obj.Error.Type != tc.wantType {
				t.Errorf("error.type = %q，期望 %q（SDK 据此分类与退避）", obj.Error.Type, tc.wantType)
			}
			if strings.HasPrefix(obj.Error.Message, "{") {
				t.Errorf("error.message 是 JSON 原文而非可读消息: %s", obj.Error.Message)
			}
			if res.clientStatus != tc.upstreamStatus {
				t.Errorf("客户端状态码 = %d，期望 %d", res.clientStatus, tc.upstreamStatus)
			}
		})
	}
}
