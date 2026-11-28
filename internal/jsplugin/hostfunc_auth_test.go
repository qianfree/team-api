package jsplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// fetchAuthFixture 鉴权探针插件：probe 钩子按入参 url / headers 发起 utils.fetch，
// 回传服务端看到的 Authorization（不参与任务链，仅探测宿主能力行为）。
func fetchAuthFixture(t *testing.T) *CompiledPlugin {
	t.Helper()
	src := `
export const meta = {
  apiVersion: 1, key: "fetchauth", name: "fetch auth", version: "1.0.0",
  models: ["fetchauth_model"],
  routes: [
    { method: "POST", path: "/submit/:action", type: "submit" },
    { method: "GET",  path: "/fetch/:task_id",  type: "query" },
  ],
};
export function decodeRequest(ctx) { return { kind: "submit", model: "fetchauth_model", requestBody: ctx.body || {} }; }
export function buildSubmitRequest(ctx) { return { url: "/v1/create", method: "POST", body: ctx.body || {} }; }
export function parseSubmitResponse() { return { taskId: "t1" }; }
export function buildQueryRequest() { return { url: "/v1/query", method: "GET" }; }
export function parseQueryResponse() { return { items: [{ taskId: "t1", done: true }] }; }
export function probe(ctx) {
  const request = { url: ctx.url };
  if (ctx.headers) request.headers = ctx.headers;
  const response = utils.fetch(request);
  return { auth: response.body.auth };
}
`
	return mustCompile(t, src)
}

// newAuthEchoServer 把请求的 Authorization 头原样回显为 {"auth": "..."} 的测试服务器。
func newAuthEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"auth": r.Header.Get("Authorization")})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestFetchSameOriginChannelAuth utils.fetch 的鉴权语义：
//   - 与渠道 baseUrl 同源（scheme + host[:port]）的请求注入 "Bearer <渠道密钥>"，
//     相对路径与绝对 URL 同口径——渠道自己的接口需要鉴权时插件才能二跳（如换下载直链）
//   - 插件自设的 Authorization 一律剔除：同源请求看到的是渠道密钥，跨域请求不带任何鉴权
//     （渠道密钥不出渠道自己的域，meta.allowedHosts 的第三方域拿不到）
//   - 无渠道上下文（decode/render 等钩子）不注入
func TestFetchSameOriginChannelAuth(t *testing.T) {
	channel := newAuthEchoServer(t)
	third := newAuthEchoServer(t)
	hosts := []string{hostPortOf(channel.URL), hostPortOf(third.URL)}

	inst, err := NewGojaEngine().NewInstance(fetchAuthFixture(t), InstanceOptions{
		CallTimeout:  5 * time.Second,
		AllowedHosts: hosts,
	})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	t.Cleanup(inst.Close)
	probe := func(ctx context.Context, fetchURL string, headers map[string]string) string {
		t.Helper()
		args, _ := json.Marshal(map[string]any{"url": fetchURL, "headers": headers})
		out, err := inst.Call(ctx, "probe", args)
		if err != nil {
			t.Fatalf("probe 失败（%s）: %v", fetchURL, err)
		}
		var res struct {
			Auth string `json:"auth"`
		}
		if err := json.Unmarshal(out, &res); err != nil {
			t.Fatalf("probe 返回解析失败: %v", err)
		}
		return res.Auth
	}
	withKey := WithChannelKey(WithBaseURL(WithAllowedHosts(context.Background(), hosts), channel.URL), "sk-channel")
	noKey := WithBaseURL(WithAllowedHosts(context.Background(), hosts), channel.URL)
	pluginAuth := map[string]string{"Authorization": "Bearer plugin-supplied"}

	// 同源：相对路径 / 绝对 URL 都注入渠道 Bearer
	if got := probe(withKey, "/v1/ping", nil); got != "Bearer sk-channel" {
		t.Fatalf("同源相对路径应注入渠道 Bearer，实际 %q", got)
	}
	if got := probe(withKey, channel.URL+"/v1/ping", nil); got != "Bearer sk-channel" {
		t.Fatalf("同源绝对 URL 应注入渠道 Bearer，实际 %q", got)
	}
	// 插件自设 Authorization：同源被渠道密钥覆盖
	if got := probe(withKey, "/v1/ping", pluginAuth); got != "Bearer sk-channel" {
		t.Fatalf("插件自设 Authorization 应被渠道密钥覆盖，实际 %q", got)
	}
	// 跨域（allowedHosts 第三方域）：不带任何鉴权
	if got := probe(withKey, third.URL+"/v1/ping", nil); got != "" {
		t.Fatalf("跨域请求不得携带渠道密钥，实际 %q", got)
	}
	if got := probe(withKey, third.URL+"/v1/ping", pluginAuth); got != "" {
		t.Fatalf("跨域请求不得透传插件自设 Authorization，实际 %q", got)
	}
	// 无渠道上下文：不注入
	if got := probe(noKey, "/v1/ping", nil); got != "" {
		t.Fatalf("未注入渠道密钥时不得带鉴权头，实际 %q", got)
	}
}

// TestSameOrigin 同源判定：scheme 与 host[:port] 均一致（大小写不敏感）才算同源；
// 协议降级、端口不同、baseUrl 缺失或非法一律视为不同源（保守，不注入密钥）。
func TestSameOrigin(t *testing.T) {
	parse := func(raw string) *url.URL {
		t.Helper()
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("解析 %q 失败: %v", raw, err)
		}
		return u
	}
	cases := []struct {
		base, target string
		want         bool
	}{
		{"https://api.example.com", "https://api.example.com/v1/x", true},
		{"https://API.example.com/", "https://api.Example.com/v1", true},
		{"https://api.example.com:8443", "https://api.example.com:8443/v1", true},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080/v1", true},
		{"https://api.example.com", "http://api.example.com/v1", false}, // 协议降级不带密钥
		{"https://api.example.com", "https://api.example.com:8443/v1", false},
		{"https://api.example.com", "https://cdn.example.com/v1", false},
		{"https://api.example.com", "https://api.example.com.evil.com/v1", false},
		{"", "https://api.example.com/v1", false},
		{"::not a url::", "https://api.example.com/v1", false},
	}
	for _, c := range cases {
		if got := sameOrigin(c.base, parse(c.target)); got != c.want {
			t.Errorf("sameOrigin(base=%q, target=%q) = %v, want %v", c.base, c.target, got, c.want)
		}
	}
	if sameOrigin("https://api.example.com", nil) {
		t.Error("nil 目标应为不同源")
	}
}
