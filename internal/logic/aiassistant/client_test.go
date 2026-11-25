package aiassistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient 构造指向 mock 上游的客户端。
func newTestClient(t *testing.T, mode, path string, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := defaultConfig()
	cfg.BaseURL = srv.URL + path
	cfg.EndpointMode = mode
	cfg.APIKey = "sk-test"
	cfg.Model = "test-model"
	return NewClient(cfg)
}

// TestChatStreamDeltas 流式分片解析：delta 逐段回调、[DONE] 终止、usage 采集。
func TestChatStreamDeltas(t *testing.T) {
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("base 模式应拼接 /chat/completions，得到 %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("缺少鉴权头")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n"))
		_, _ = w.Write([]byte(": keepalive\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	var got strings.Builder
	res, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 100, 0.5, nil, func(delta string) {
		got.WriteString(delta)
	}, nil, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if got.String() != "你好" {
		t.Fatalf("增量不符: %q", got.String())
	}
	if res.Usage.TotalTokens != 12 {
		t.Fatalf("usage 不符: %+v", res.Usage)
	}
}

// TestChatStreamNoDoneFlag 流未发 [DONE] 自然结束（部分兼容端点形态）。
func TestChatStreamNoDoneFlag(t *testing.T) {
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
	})
	var got strings.Builder
	if _, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 0, 0, nil, func(s string) { got.WriteString(s) }, nil, nil); err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if got.String() != "ok" {
		t.Fatalf("增量不符: %q", got.String())
	}
}

// TestChatStreamHTTPError 上游非 2xx：错误信息带状态码与 message。
func TestChatStreamHTTPError(t *testing.T) {
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	})
	_, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 0, 0, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("错误信息不符: %v", err)
	}
}

// TestClientFullEndpointMode full 模式端点原样使用（不拼接路径）。
func TestClientFullEndpointMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/path" {
			t.Errorf("full 模式应原样请求，得到 %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	cfg := defaultConfig()
	cfg.BaseURL = srv.URL + "/custom/path"
	cfg.EndpointMode = EndpointModeFull
	_, err := NewClient(cfg).ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 0, 0, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
}

// TestTestConnection 连通性测试成功路径。
func TestTestConnection(t *testing.T) {
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"total_tokens":8}}`))
	})
	res := client.TestConnection(context.Background())
	if !res.OK || res.Reply != "ok" || res.TokensUsed != 8 {
		t.Fatalf("结果不符: %+v", res)
	}
}

// TestChatStreamReasoning 推理型模型：思考内容走 onReasoning，正文走 onDelta，
// 两条通道互不混入（前端据此渲染「思考过程」折叠块）。
func TestChatStreamReasoning(t *testing.T) {
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"先看契约\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning\":\"再动手\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好了\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	var think, body strings.Builder
	_, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 100, 0.5, nil,
		func(s string) { body.WriteString(s) }, func(s string) { think.WriteString(s) }, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if think.String() != "先看契约再动手" {
		t.Fatalf("思考增量不符: %q", think.String())
	}
	if body.String() != "好了" {
		t.Fatalf("正文增量不符: %q", body.String())
	}
}

// TestChatStreamFinishReason 截断信号：finish_reason=length 必须被采集，
// 上层据此发 truncated 事件（否则截断的代码会被当成完整代码校验/展示）。
func TestChatStreamFinishReason(t *testing.T) {
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"const a =\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	res, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 4096, 0.2, nil, func(string) {}, nil, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if !res.Truncated() {
		t.Fatalf("应识别为截断，finish_reason=%q", res.FinishReason)
	}
}

// TestIsMaxTokensRejected 上游拒绝 max_tokens 的识别（用于回退重试）。
func TestIsMaxTokensRejected(t *testing.T) {
	cases := map[string]bool{
		"AI 助手端点返回非成功状态（HTTP 400）: max_tokens is too large":                     true,
		"AI 助手端点返回非成功状态（HTTP 400）: This model's maximum context length is 8192": true,
		"AI 助手端点返回非成功状态（HTTP 401）: invalid api key":                             false,
		"context canceled": false,
	}
	for msg, want := range cases {
		if got := IsMaxTokensRejected(errors.New(msg)); got != want {
			t.Fatalf("%q => %v, want %v", msg, got, want)
		}
	}
	if IsMaxTokensRejected(nil) {
		t.Fatal("nil 不应判定为拒绝")
	}
}

// TestChatStreamToolCalls 流式工具调用：分片聚合（id/name 首片、arguments 逐片拼接）、
// 多调用按 index 排序、finish_reason=tool_calls 采集、onToolDelta 增量回调、
// 请求体携带 tools 定义（agent loop 的必备链路）。
func TestChatStreamToolCalls(t *testing.T) {
	var gotReq struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name       string `json:"name"`
				Parameters struct {
					Type string `json:"type"`
				} `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
		ToolChoice string `json:"tool_choice"`
	}
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "text/event-stream")
		// 工具调用分片（OpenAI 流式形态）：首片带 id/name，arguments 逐片拼接
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"write_code","arguments":""}}]}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"code\":\"// he"}}]}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"llo\"}"}}]}}]}` + "\n\n"))
		// 第二个调用（index 1）
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"validate_code","arguments":"{}"}}]}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	tools := []ToolDef{PluginCodeTools[0], PluginCodeTools[3]}
	type delta struct {
		index          int
		id, name, args string
	}
	var deltas []delta
	res, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 4096, 0.2,
		tools, nil, nil, func(index int, id, name, argsDelta string) {
			deltas = append(deltas, delta{index, id, name, argsDelta})
		})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if res.FinishReason != FinishReasonToolCalls {
		t.Fatalf("finish_reason 应为 tool_calls，得到 %q", res.FinishReason)
	}
	if len(res.ToolCalls) != 2 {
		t.Fatalf("应聚合出 2 个工具调用，得到 %d", len(res.ToolCalls))
	}
	a, b := res.ToolCalls[0], res.ToolCalls[1]
	if a.ID != "call_a" || a.Function == nil || a.Function.Name != "write_code" || a.Function.Arguments != `{"code":"// hello"}` {
		t.Fatalf("第一个调用聚合异常: %+v", a)
	}
	if b.ID != "call_b" || b.Function.Name != "validate_code" || b.Function.Arguments != "{}" {
		t.Fatalf("第二个调用聚合异常: %+v", b)
	}
	if len(deltas) != 4 || deltas[0].name != "write_code" || deltas[0].id != "call_a" || deltas[3].name != "validate_code" {
		t.Fatalf("增量回调异常: %+v", deltas)
	}
	if len(gotReq.Tools) != 2 || gotReq.Tools[0].Function.Name != "write_code" || gotReq.Tools[0].Function.Parameters.Type != "object" {
		t.Fatalf("请求未携带 tools 定义: %+v", gotReq.Tools)
	}
	if gotReq.ToolChoice != "auto" {
		t.Fatalf("tool_choice 应为 auto: %q", gotReq.ToolChoice)
	}
}
