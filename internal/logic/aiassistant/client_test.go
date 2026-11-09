package aiassistant

import (
	"context"
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
	usage, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 100, 0.5, func(delta string) {
		got.WriteString(delta)
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if got.String() != "你好" {
		t.Fatalf("增量不符: %q", got.String())
	}
	if usage.TotalTokens != 12 {
		t.Fatalf("usage 不符: %+v", usage)
	}
}

// TestChatStreamNoDoneFlag 流未发 [DONE] 自然结束（部分兼容端点形态）。
func TestChatStreamNoDoneFlag(t *testing.T) {
	client := newTestClient(t, EndpointModeBase, "", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
	})
	var got strings.Builder
	if _, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 0, 0, func(s string) { got.WriteString(s) }); err != nil {
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
	_, err := client.ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 0, 0, nil)
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
	_, err := NewClient(cfg).ChatStream(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, 0, 0, nil)
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
