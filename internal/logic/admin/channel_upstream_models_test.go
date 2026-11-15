package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/relay/constant"
)

func TestJoinModelsURL(t *testing.T) {
	cases := []struct {
		name   string
		base   string
		suffix string
		want   string
	}{
		{"裸域名补版本段", "https://api.openai.com", "/v1/models", "https://api.openai.com/v1/models"},
		{"末尾斜杠", "https://api.openai.com/", "/v1/models", "https://api.openai.com/v1/models"},
		{"中转站已带 v1", "https://relay.example.com/v1", "/v1/models", "https://relay.example.com/v1/models"},
		{"智谱 paas v4 已带版本段", "https://open.bigmodel.cn/api/paas/v4", "/v1/models", "https://open.bigmodel.cn/api/paas/v4/models"},
		{"阿里 compatible-mode", "https://dashscope.aliyuncs.com/compatible-mode/v1", "/v1/models", "https://dashscope.aliyuncs.com/compatible-mode/v1/models"},
		{"gemini 裸域名", "https://generativelanguage.googleapis.com", "/v1beta/models", "https://generativelanguage.googleapis.com/v1beta/models"},
		{"gemini 已带版本段", "https://generativelanguage.googleapis.com/v1beta", "/v1beta/models", "https://generativelanguage.googleapis.com/v1beta/models"},
		{"主机名含 v 不误判", "https://v2.example.com", "/v1/models", "https://v2.example.com/v1/models"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := joinModelsURL(c.base, c.suffix); got != c.want {
				t.Fatalf("joinModelsURL(%q, %q) = %q, want %q", c.base, c.suffix, got, c.want)
			}
		})
	}
}

func TestParseUpstreamModelList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "OpenAI data[].id",
			raw:  `{"object":"list","data":[{"id":"gpt-4o","owned_by":"openai"},{"id":"gpt-4o-mini","owned_by":"openai"}]}`,
			want: []string{"gpt-4o", "gpt-4o-mini"},
		},
		{
			name: "Gemini models[].name 去前缀",
			raw:  `{"models":[{"name":"models/gemini-2.0-flash"},{"name":"models/gemini-1.5-pro"}]}`,
			want: []string{"gemini-1.5-pro", "gemini-2.0-flash"},
		},
		{
			name: "根数组对象",
			raw:  `[{"id":"b-model"},{"id":"a-model"}]`,
			want: []string{"a-model", "b-model"},
		},
		{
			name: "根数组字符串",
			raw:  `["model-x","model-y"]`,
			want: []string{"model-x", "model-y"},
		},
		{
			name: "去重且排序",
			raw:  `{"data":[{"id":"b"},{"id":"a"},{"id":"b"}]}`,
			want: []string{"a", "b"},
		},
		{
			name: "忽略空 ID",
			raw:  `{"data":[{"id":""},{"id":"  "},{"id":"ok"}]}`,
			want: []string{"ok"},
		},
		{
			name: "无法识别",
			raw:  `{"error":{"message":"invalid api key"}}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUpstreamModelList([]byte(tt.raw))
			if len(got) != len(tt.want) {
				t.Fatalf("解析出 %d 条（%v），期望 %d 条（%v）", len(got), ids(got), len(tt.want), tt.want)
			}
			for i, id := range tt.want {
				if got[i].ID != id {
					t.Fatalf("第 %d 条 = %q, want %q（全部：%v）", i, got[i].ID, id, ids(got))
				}
			}
		})
	}
}

func ids(list []v1.UpstreamModelItem) []string {
	out := make([]string, len(list))
	for i, m := range list {
		out[i] = m.ID
	}
	return out
}

// TestFetchUpstreamModelList 走真实 HTTP，校验协议分支的路径与鉴权头。
func TestFetchUpstreamModelList(t *testing.T) {
	tests := []struct {
		name         string
		provider     constant.ProviderType
		wantPath     string
		wantProtocol string
		checkHeader  func(t *testing.T, r *http.Request)
		body         string
	}{
		{
			name:         "OpenAI 兼容",
			provider:     constant.ProviderOpenAI,
			wantPath:     "/v1/models",
			wantProtocol: "openai",
			checkHeader: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
					t.Fatalf("Authorization = %q, want Bearer sk-test", got)
				}
			},
			body: `{"object":"list","data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`,
		},
		{
			name:         "Claude 用 x-api-key",
			provider:     constant.ProviderClaude,
			wantPath:     "/v1/models",
			wantProtocol: "claude",
			checkHeader: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("x-api-key"); got != "sk-test" {
					t.Fatalf("x-api-key = %q, want sk-test", got)
				}
				if got := r.Header.Get("anthropic-version"); got == "" {
					t.Fatal("缺少 anthropic-version 头")
				}
			},
			body: `{"data":[{"id":"claude-3-5-sonnet"}]}`,
		},
		{
			name:         "Gemini 走 v1beta 与 x-goog-api-key",
			provider:     constant.ProviderGemini,
			wantPath:     "/v1beta/models",
			wantProtocol: "gemini",
			checkHeader: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("x-goog-api-key"); got != "sk-test" {
					t.Fatalf("x-goog-api-key = %q, want sk-test", got)
				}
			},
			body: `{"models":[{"name":"models/gemini-2.0-flash"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.wantPath {
					t.Errorf("请求路径 = %q, want %q", r.URL.Path, tt.wantPath)
				}
				tt.checkHeader(t, r)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			list, protocol, err := fetchUpstreamModelList(context.Background(), tt.provider, srv.URL, "sk-test", false)
			if err != nil {
				t.Fatalf("拉取失败: %v", err)
			}
			if protocol != tt.wantProtocol {
				t.Fatalf("协议 = %q, want %q", protocol, tt.wantProtocol)
			}
			if len(list) == 0 {
				t.Fatal("返回空列表")
			}
		})
	}
}

// TestFetchUpstreamModelListErrors 上游报错时必须返回可读错误，让前端能降级为手动输入。
func TestFetchUpstreamModelListErrors(t *testing.T) {
	t.Run("非 2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
		}))
		defer srv.Close()

		_, _, err := fetchUpstreamModelList(context.Background(), constant.ProviderOpenAI, srv.URL, "bad", false)
		if err == nil {
			t.Fatal("期望报错，实际成功")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Fatalf("错误信息应包含状态码 401，实际: %v", err)
		}
	})

	t.Run("空 base_url", func(t *testing.T) {
		_, _, err := fetchUpstreamModelList(context.Background(), constant.ProviderOpenAI, "  ", "k", false)
		if err == nil {
			t.Fatal("base_url 为空时应报错")
		}
	})

	t.Run("不可识别的响应体", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`<html>gateway</html>`))
		}))
		defer srv.Close()

		_, _, err := fetchUpstreamModelList(context.Background(), constant.ProviderOpenAI, srv.URL, "k", false)
		if err == nil {
			t.Fatal("响应体无法解析时应报错")
		}
	})
}
