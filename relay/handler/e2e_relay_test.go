package handler

// e2e_relay_test.go — 协议代理链路的端到端集成测试（假上游）。
//
// 为什么需要这一层：既有三层防线（wire 保真 / 能力守恒 / 金样本）与矩阵守卫都是
// 静态或单元维度的——分别验证「DTO 形状对不对」「转换器输出对不对」「该不该转、
// URL 拿不拿得到」。但没有任何测试走过完整链路，于是「每个单元各自正确、缝隙处错配」
// 的问题能逃过全部测试：典型如 vertex（URL 不报错但端点吃的是原生格式，矩阵却发 chat 体）
// 与 GeminiChat mode 漏配（转换器和响应桥都就位、DoRequest 处 0ms 断链）。
//
// 本文件用 httptest 假上游驱动真实链路：
//
//	convertRequestBody → adaptor.DoRequest → 假上游（捕获 URL/Header/Body）
//	  → 返回上游格式响应 → adaptor.DoResponse → 客户端可见字节
//
// 断言三条单元测试给不了的性质：
//  1. 体格式 = 端点格式：上游收到的请求体必须符合该端点期望的协议格式；
//  2. 响应闭环：客户端最终收到的字节必须符合入站协议的 wire 契约（含流式终止语义）；
//  3. 错误映射：上游故障时客户端拿到的是入站协议的原生错误格式。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// ---------- 假上游 ----------

// fakeUpstream 捕获网关实际发出的请求，并按配置返回响应。
type fakeUpstream struct {
	srv *httptest.Server

	mu         sync.Mutex
	gotPath    string
	gotQuery   string
	gotBody    []byte
	gotHeaders http.Header
	gotCount   int

	status      int
	contentType string
	body        string
}

func newFakeUpstream(status int, contentType, body string) *fakeUpstream {
	up := &fakeUpstream{status: status, contentType: contentType, body: body}
	up.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)

		up.mu.Lock()
		up.gotPath = r.URL.Path
		up.gotQuery = r.URL.RawQuery
		up.gotBody = raw
		up.gotHeaders = r.Header.Clone()
		up.gotCount++
		status, ct, body := up.status, up.contentType, up.body
		up.mu.Unlock()

		w.Header().Set("Content-Type", ct)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	return up
}

func (u *fakeUpstream) close() { u.srv.Close() }

func (u *fakeUpstream) request() (path, query string, body []byte, headers http.Header, count int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.gotPath, u.gotQuery, u.gotBody, u.gotHeaders, u.gotCount
}

// ---------- 链路驱动 ----------

type e2eCase struct {
	name     string
	provider constant.ProviderType
	inbound  constant.RelayFormat
	mode     constant.RelayMode
	stream   bool

	// 渠道能力开关（Responses 桥接方向用）
	chatViaResponses  bool
	supportsResponses bool

	clientBody string

	// 上游侧期望
	wantUpstreamFormat constant.RelayFormat
	wantPathContains   string

	// 假上游返回
	upstreamStatus      int
	upstreamContentType string
	upstreamBody        string
}

type e2eResult struct {
	upstream     *fakeUpstream
	clientStatus int
	clientBody   string
	clientHeader http.Header
	usage        *common.Usage
	err          error
}

// runE2E 驱动一次完整代理链路，返回上游侧与客户端侧的可观测结果。
func runE2E(t *testing.T, c e2eCase) e2eResult {
	t.Helper()

	ct := c.upstreamContentType
	if ct == "" {
		ct = "application/json"
	}
	status := c.upstreamStatus
	if status == 0 {
		status = http.StatusOK
	}
	up := newFakeUpstream(status, ct, c.upstreamBody)
	t.Cleanup(up.close)

	info := &common.RelayInfo{
		Context:         context.Background(),
		RequestID:       "e2e-req-1",
		RelayMode:       int(c.mode),
		IsStream:        c.stream,
		OriginModelName: "origin-model",
		InboundFormat:   c.inbound,
		ClientFormat:    c.inbound,
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         1,
			ChannelType:       int(c.provider),
			ChannelName:       "e2e-channel",
			BaseURL:           up.srv.URL,
			ApiKey:            "sk-e2e-test",
			UpstreamModelName: "upstream-model",
			ChatViaResponses:  c.chatViaResponses,
			SupportsResponses: c.supportsResponses,
		},
	}
	// chat_via_responses 渠道的 chat 入站：由 relay_handler 置位，此处按真实链路模拟
	if c.chatViaResponses && c.mode == constant.RelayModeChatCompletions {
		info.UseResponsesAPI = true
	}

	ctx := context.Background()
	adaptor := channel.GetAdaptor(int(c.provider))
	if adaptor == nil {
		t.Fatalf("provider %d 没有注册 adaptor", c.provider)
	}
	adaptor.Init(info)

	reqBody, err := convertRequestBody(ctx, info, []byte(c.clientBody), adaptor)
	if err != nil {
		return e2eResult{upstream: up, err: err}
	}

	resp, err := adaptor.DoRequest(ctx, info, reqBody)
	if err != nil {
		return e2eResult{upstream: up, err: err}
	}

	rec := httptest.NewRecorder()
	usage, err := adaptor.DoResponse(ctx, resp, info, rec)

	return e2eResult{
		upstream:     up,
		clientStatus: rec.Code,
		clientBody:   rec.Body.String(),
		clientHeader: rec.Header(),
		usage:        usage,
		err:          err,
	}
}

// ---------- 格式签名断言 ----------

// formatSignature 各协议格式请求体的结构签名：required 为必须存在的顶层 key，
// forbidden 为「其他格式独有、本格式绝不该出现」的顶层 key。
// 体与端点错配时（如把 chat 体发到 Gemini 的 :generateContent），forbidden 必然命中。
var requestSignature = map[constant.RelayFormat]struct {
	required  []string
	forbidden []string
}{
	constant.RelayFormatOpenAI: {
		required:  []string{"model", "messages"},
		forbidden: []string{"contents", "generationConfig", "input", "system"},
	},
	constant.RelayFormatClaude: {
		required:  []string{"model", "messages", "max_tokens"},
		forbidden: []string{"contents", "generationConfig", "input", "stream_options"},
	},
	constant.RelayFormatGemini: {
		required:  []string{"contents"},
		forbidden: []string{"messages", "input", "max_tokens", "system"},
	},
	constant.RelayFormatResponses: {
		required:  []string{"model", "input"},
		forbidden: []string{"messages", "contents", "generationConfig"},
	},
	constant.RelayFormatOllama: {
		required:  []string{"model", "messages"},
		forbidden: []string{"contents", "input", "max_tokens", "system"},
	},
}

// responseSignature 各协议格式响应体的结构签名（客户端可见字节）。
var responseSignature = map[constant.RelayFormat]struct {
	required  []string
	forbidden []string
}{
	constant.RelayFormatOpenAI: {
		required:  []string{"choices"},
		forbidden: []string{"candidates", "output", "content"},
	},
	constant.RelayFormatClaude: {
		required:  []string{"type", "content"},
		forbidden: []string{"choices", "candidates", "output"},
	},
	constant.RelayFormatGemini: {
		required:  []string{"candidates"},
		forbidden: []string{"choices", "output", "content"},
	},
	constant.RelayFormatResponses: {
		required:  []string{"output", "status"},
		forbidden: []string{"choices", "candidates"},
	},
}

func assertTopLevelShape(t *testing.T, what string, format constant.RelayFormat, body []byte,
	sigs map[constant.RelayFormat]struct {
		required  []string
		forbidden []string
	}) {
	t.Helper()

	sig, ok := sigs[format]
	if !ok {
		t.Fatalf("%s: 格式 %s 没有登记结构签名", what, format)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("%s: 不是合法 JSON 对象（期望 %s 格式）: %v\n实际内容: %s", what, format, err, truncate(body))
	}

	for _, k := range sig.required {
		if _, exists := obj[k]; !exists {
			t.Errorf("%s: 期望 %s 格式，但缺少必需字段 %q\n实际内容: %s", what, format, k, truncate(body))
		}
	}
	for _, k := range sig.forbidden {
		if _, exists := obj[k]; exists {
			t.Errorf("%s: 期望 %s 格式，却出现了其他格式独有的字段 %q——体与端点错配\n实际内容: %s",
				what, format, k, truncate(body))
		}
	}
}

func truncate(b []byte) string {
	const max = 600
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}

// ---------- 语料 ----------

// 各入站格式的客户端请求（均带 system 提示词：它在 OpenAI 侧进 messages、
// 在 Claude 侧升为顶层 system，是两种相似结构的关键判别点）。
var e2eClientRequests = map[constant.RelayFormat]string{
	constant.RelayFormatOpenAI: `{
		"model": "origin-model",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello"}
		],
		"max_tokens": 1024,
		"temperature": 0.7
	}`,
	constant.RelayFormatClaude: `{
		"model": "origin-model",
		"system": "You are helpful.",
		"messages": [{"role": "user", "content": "Hello"}],
		"max_tokens": 1024
	}`,
	constant.RelayFormatGemini: `{
		"systemInstruction": {"parts": [{"text": "You are helpful."}]},
		"contents": [{"role": "user", "parts": [{"text": "Hello"}]}],
		"generationConfig": {"maxOutputTokens": 1024, "temperature": 0.7}
	}`,
	constant.RelayFormatResponses: `{
		"model": "origin-model",
		"instructions": "You are helpful.",
		"input": [{"role": "user", "content": [{"type": "input_text", "text": "Hello"}]}],
		"max_output_tokens": 1024
	}`,
}

// 各上游格式的非流式响应语料（含用量，供计费闭环断言）。
var e2eUpstreamResponses = map[constant.RelayFormat]string{
	constant.RelayFormatOpenAI: `{
		"id": "chatcmpl-up", "object": "chat.completion", "created": 1700000000, "model": "upstream-model",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi there!"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 12, "completion_tokens": 7, "total_tokens": 19}
	}`,
	constant.RelayFormatClaude: `{
		"id": "msg_up", "type": "message", "role": "assistant", "model": "upstream-model",
		"content": [{"type": "text", "text": "Hi there!"}],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 12, "output_tokens": 7}
	}`,
	constant.RelayFormatGemini: `{
		"candidates": [{"index": 0, "content": {"role": "model", "parts": [{"text": "Hi there!"}]}, "finishReason": "STOP"}],
		"usageMetadata": {"promptTokenCount": 12, "candidatesTokenCount": 7, "totalTokenCount": 19}
	}`,
	constant.RelayFormatResponses: `{
		"id": "resp_up", "object": "response", "created_at": 1700000000, "status": "completed", "model": "upstream-model",
		"output": [{"type": "message", "id": "msg_up", "status": "completed", "role": "assistant",
			"content": [{"type": "output_text", "text": "Hi there!"}]}],
		"usage": {"input_tokens": 12, "output_tokens": 7, "total_tokens": 19}
	}`,
	constant.RelayFormatOllama: `{
		"model": "upstream-model", "created_at": "2026-01-01T00:00:00Z",
		"message": {"role": "assistant", "content": "Hi there!"},
		"done": true, "done_reason": "stop",
		"prompt_eval_count": 12, "eval_count": 7
	}`,
}

// ---------- 非流式方向矩阵 ----------

// e2eNonStreamCases 覆盖全部已注册转换方向的真实链路（含同格式直连基线）。
func e2eNonStreamCases() []e2eCase {
	mk := func(name string, provider constant.ProviderType, inbound constant.RelayFormat,
		mode constant.RelayMode, upstreamFormat constant.RelayFormat, pathContains string) e2eCase {
		return e2eCase{
			name:               name,
			provider:           provider,
			inbound:            inbound,
			mode:               mode,
			clientBody:         e2eClientRequests[inbound],
			wantUpstreamFormat: upstreamFormat,
			wantPathContains:   pathContains,
			upstreamBody:       e2eUpstreamResponses[upstreamFormat],
		}
	}

	return []e2eCase{
		// OpenAI 入站 → 各原生上游
		mk("openai→claude", constant.ProviderClaude, constant.RelayFormatOpenAI,
			constant.RelayModeChatCompletions, constant.RelayFormatClaude, "/v1/messages"),
		mk("openai→gemini", constant.ProviderGemini, constant.RelayFormatOpenAI,
			constant.RelayModeChatCompletions, constant.RelayFormatGemini, ":generateContent"),
		mk("openai→ollama", constant.ProviderOllama, constant.RelayFormatOpenAI,
			constant.RelayModeChatCompletions, constant.RelayFormatOllama, "/api/chat"),

		// 非 OpenAI 入站 → OpenAI 上游（反向）
		mk("claude→openai", constant.ProviderOpenAI, constant.RelayFormatClaude,
			constant.RelayModeClaudeMessages, constant.RelayFormatOpenAI, "/v1/chat/completions"),
		mk("gemini→openai", constant.ProviderOpenAI, constant.RelayFormatGemini,
			constant.RelayModeGeminiChat, constant.RelayFormatOpenAI, "/v1/chat/completions"),
		mk("responses→openai", constant.ProviderOpenAI, constant.RelayFormatResponses,
			constant.RelayModeResponses, constant.RelayFormatOpenAI, "/v1/chat/completions"),

		// 跨原生方向
		mk("claude→gemini", constant.ProviderGemini, constant.RelayFormatClaude,
			constant.RelayModeClaudeMessages, constant.RelayFormatGemini, ":generateContent"),
		mk("gemini→claude", constant.ProviderClaude, constant.RelayFormatGemini,
			constant.RelayModeGeminiChat, constant.RelayFormatClaude, "/v1/messages"),
		mk("responses→claude", constant.ProviderClaude, constant.RelayFormatResponses,
			constant.RelayModeResponses, constant.RelayFormatClaude, "/v1/messages"),
		mk("responses→gemini", constant.ProviderGemini, constant.RelayFormatResponses,
			constant.RelayModeResponses, constant.RelayFormatGemini, ":generateContent"),

		// 同格式直连基线（矩阵不接管，走 adaptor 原生路径）
		mk("openai→openai 直连", constant.ProviderOpenAI, constant.RelayFormatOpenAI,
			constant.RelayModeChatCompletions, constant.RelayFormatOpenAI, "/v1/chat/completions"),
		mk("claude→claude 直连", constant.ProviderClaude, constant.RelayFormatClaude,
			constant.RelayModeClaudeMessages, constant.RelayFormatClaude, "/v1/messages"),
	}
}

// TestE2E_RequestBodyMatchesEndpointFormat 断言 1（体格式 = 端点格式）：
// 上游实际收到的请求体必须符合该端点期望的协议格式，且用户内容不丢。
//
// 这是单元测试结构上够不到的性质——转换器输出正确、URL 计算正确，但两者配错时
// 只有真实链路能发现（vertex 类问题的检测面）。
func TestE2E_RequestBodyMatchesEndpointFormat(t *testing.T) {
	for _, c := range e2eNonStreamCases() {
		t.Run(c.name, func(t *testing.T) {
			res := runE2E(t, c)
			if res.err != nil {
				t.Fatalf("链路失败: %v", res.err)
			}

			path, query, body, _, count := res.upstream.request()
			if count != 1 {
				t.Fatalf("上游收到 %d 次请求，期望 1 次", count)
			}
			full := path
			if query != "" {
				full += "?" + query
			}
			if !strings.Contains(full, c.wantPathContains) {
				t.Errorf("上游端点 = %q，期望包含 %q", full, c.wantPathContains)
			}

			assertTopLevelShape(t, "上游请求体（端点 "+full+"）", c.wantUpstreamFormat, body, requestSignature)

			if !strings.Contains(string(body), "Hello") {
				t.Errorf("用户消息内容在转换链路中丢失\n实际内容: %s", truncate(body))
			}
		})
	}
}

// TestE2E_ResponseMatchesInboundFormat 断言 2（响应闭环）：
// 客户端最终收到的字节必须符合入站协议的响应 wire 契约，内容与用量都不丢。
func TestE2E_ResponseMatchesInboundFormat(t *testing.T) {
	for _, c := range e2eNonStreamCases() {
		t.Run(c.name, func(t *testing.T) {
			res := runE2E(t, c)
			if res.err != nil {
				t.Fatalf("链路失败: %v", res.err)
			}
			if res.clientStatus != http.StatusOK {
				t.Fatalf("客户端状态码 = %d，期望 200\n响应体: %s", res.clientStatus, truncate([]byte(res.clientBody)))
			}

			assertTopLevelShape(t, "客户端响应体", c.inbound, []byte(res.clientBody), responseSignature)

			if !strings.Contains(res.clientBody, "Hi there!") {
				t.Errorf("上游回答内容在响应转换中丢失\n实际内容: %s", truncate([]byte(res.clientBody)))
			}

			// 计费闭环：用量必须被提取（否则整条请求按 0 token 结算）
			if res.usage == nil {
				t.Fatal("未返回用量（计费将按 0 token 结算）")
			}
			if res.usage.PromptTokens == 0 || res.usage.CompletionTokens == 0 {
				t.Errorf("用量提取不完整: prompt=%d completion=%d（上游语料为 12/7）",
					res.usage.PromptTokens, res.usage.CompletionTokens)
			}
		})
	}
}
