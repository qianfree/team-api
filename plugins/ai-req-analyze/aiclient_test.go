package aireqanalyze

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClientConfig(baseURL string) *Config {
	return &Config{
		AIBaseURL:   baseURL,
		AIAPIKey:    "sk-test",
		AIModel:     "test-model",
		AITimeoutMs: 5000,
		AIMaxTokens: 128,
	}
}

// openAIHandler 模拟 OpenAI 兼容端点：记录请求供断言，按 stub 返回响应。
type openAIHandler struct {
	gotAuthHeader  string
	gotRequestBody map[string]any
	stubStatus     int
	stubBody       string
}

func (h *openAIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.gotAuthHeader = r.Header.Get("Authorization")
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	h.gotRequestBody = body
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.stubStatus)
	_, _ = w.Write([]byte(h.stubBody))
}

func TestAIClientFullEndpointMode(t *testing.T) {
	// full 模式：配置值即完整端点，不追加 /chat/completions（应对非标准路径）
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	cfg := testClientConfig(srv.URL + "/ai/custom-endpoint")
	cfg.AIEndpointMode = EndpointModeFull
	res, err := NewAIClient(cfg).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{})
	if err != nil {
		t.Fatalf("full 模式请求失败: %v", err)
	}
	if res.Content != "ok" {
		t.Fatalf("响应解析错误: %q", res.Content)
	}
	if gotPath != "/ai/custom-endpoint" {
		t.Fatalf("full 模式请求路径应为配置原值，实际 %q", gotPath)
	}

	// base 模式（默认）追加 /chat/completions
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv2.Close()

	cfg2 := testClientConfig(srv2.URL + "/v1")
	if _, err := NewAIClient(cfg2).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{}); err != nil {
		t.Fatalf("base 模式请求失败: %v", err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("base 模式应追加 /chat/completions，实际 %q", gotPath)
	}
}

func TestAIClientChatSuccess(t *testing.T) {
	h := &openAIHandler{
		stubStatus: 200,
		stubBody: `{"choices":[{"message":{"role":"assistant","content":"分析结论"}}],
			"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120}}`,
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	client := NewAIClient(testClientConfig(srv.URL))
	res, err := client.Chat(context.Background(), []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
	}, ChatOptions{MaxTokens: 64, Temperature: 0})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if res.Content != "分析结论" {
		t.Fatalf("内容解析错误: %q", res.Content)
	}
	if res.TotalTokens != 120 || res.PromptTokens != 100 || res.CompletionTokens != 20 {
		t.Fatalf("usage 解析错误: %+v", res)
	}

	if h.gotAuthHeader != "Bearer sk-test" {
		t.Fatalf("Authorization 头错误: %q", h.gotAuthHeader)
	}
	if h.gotRequestBody["model"] != "test-model" {
		t.Fatalf("模型名错误: %v", h.gotRequestBody["model"])
	}
	if h.gotRequestBody["max_tokens"] != float64(64) {
		t.Fatalf("max_tokens 错误: %v", h.gotRequestBody["max_tokens"])
	}
	if _, has := h.gotRequestBody["response_format"]; has {
		t.Fatal("未请求 JSON 模式时不应携带 response_format")
	}
}

func TestAIClientChatJSONMode(t *testing.T) {
	h := &openAIHandler{stubStatus: 200, stubBody: `{"choices":[{"message":{"content":"{}"}}]}`}
	srv := httptest.NewServer(h)
	defer srv.Close()

	client := NewAIClient(testClientConfig(srv.URL))
	if _, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "x"}},
		ChatOptions{ResponseJSON: true}); err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}

	rf, ok := h.gotRequestBody["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_object" {
		t.Fatalf("response_format 未正确下发: %v", h.gotRequestBody["response_format"])
	}
}

func TestAIClientChatUsageFallback(t *testing.T) {
	// 端点未返回 total_tokens 时按 prompt+completion 兜底
	h := &openAIHandler{stubStatus: 200,
		stubBody: `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`}
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, err := NewAIClient(testClientConfig(srv.URL)).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if res.TotalTokens != 10 {
		t.Fatalf("total 兜底错误: %d", res.TotalTokens)
	}
}

func TestAIClientChatHTTPError(t *testing.T) {
	h := &openAIHandler{stubStatus: 401, stubBody: `{"error":{"message":"invalid api key","type":"auth"}}`}
	srv := httptest.NewServer(h)
	defer srv.Close()

	_, err := NewAIClient(testClientConfig(srv.URL)).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{})
	if err == nil {
		t.Fatal("401 应返回错误")
	}
}

func TestAIClientRejectClassified(t *testing.T) {
	// 4xx 带错误消息 → AIRejectedError（调用方据此降级逐单元定位被拒内容）
	h := &openAIHandler{stubStatus: 400,
		stubBody: `{"error":{"message":"系统检测到输入或生成内容可能包含不安全或敏感内容","type":"content_filter"}}`}
	srv := httptest.NewServer(h)
	defer srv.Close()

	_, err := NewAIClient(testClientConfig(srv.URL)).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{})
	var rejected *AIRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("4xx 应分类为 AIRejectedError，实际 %T: %v", err, err)
	}
	if rejected.Status != 400 || !strings.Contains(rejected.Message, "敏感内容") {
		t.Fatalf("拒绝错误信息错误: %+v", rejected)
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("错误文案应含状态码: %v", err)
	}

	// 非 JSON 错误体（网关 HTML 错误页）也能分类并带出片段
	h2 := &openAIHandler{stubStatus: 400, stubBody: `<html>Bad Request by WAF</html>`}
	srv2 := httptest.NewServer(h2)
	defer srv2.Close()
	_, err = NewAIClient(testClientConfig(srv2.URL)).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{})
	if !errors.As(err, &rejected) || !strings.Contains(rejected.Message, "Bad Request") {
		t.Fatalf("非 JSON 错误体应分类并带片段: %T %v", err, err)
	}

	// 5xx 不分类为拒绝（服务端故障不降级，避免放大调用）
	h3 := &openAIHandler{stubStatus: 502, stubBody: `bad gateway`}
	srv3 := httptest.NewServer(h3)
	defer srv3.Close()
	_, err = NewAIClient(testClientConfig(srv3.URL)).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{})
	if errors.As(err, &rejected) {
		t.Fatal("5xx 不应分类为 AIRejectedError")
	}
}

func TestAIClientChatNoChoices(t *testing.T) {
	h := &openAIHandler{stubStatus: 200, stubBody: `{"choices":[]}`}
	srv := httptest.NewServer(h)
	defer srv.Close()

	if _, err := NewAIClient(testClientConfig(srv.URL)).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{}); err == nil {
		t.Fatal("空 choices 应返回错误")
	}
}

func TestAIClientChatTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"late"}}]}`))
	}))
	defer srv.Close()

	cfg := testClientConfig(srv.URL)
	cfg.AITimeoutMs = 50 // 50ms 超时
	if _, err := NewAIClient(cfg).Chat(context.Background(),
		[]ChatMessage{{Role: "user", Content: "x"}}, ChatOptions{}); err == nil {
		t.Fatal("超时应返回错误")
	}
}

func TestAIClientTestConnection(t *testing.T) {
	h := &openAIHandler{stubStatus: 200, stubBody: `{"choices":[{"message":{"content":"ok"}}],"usage":{"total_tokens":9}}`}
	srv := httptest.NewServer(h)
	defer srv.Close()

	res := NewAIClient(testClientConfig(srv.URL)).TestConnection(context.Background())
	if !res.OK || res.TokensUsed != 9 || res.Reply != "ok" {
		t.Fatalf("连通性测试成功路径错误: %+v", res)
	}
	if res.LatencyMs < 0 {
		t.Fatalf("延迟不应为负: %d", res.LatencyMs)
	}

	// 失败路径：错误是有效结果而非 panic
	res = NewAIClient(testClientConfig("http://127.0.0.1:1")).TestConnection(context.Background())
	if res.OK || res.Error == "" {
		t.Fatalf("失败路径应携带错误信息: %+v", res)
	}
}
