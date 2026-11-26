package jsbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// TestValidateUpstreamTarget 绝对 URL host 白名单表驱动：相对放行、渠道 host 命中、
// manifest.allowedHosts 命中（含端口语义）、未命中拒绝。
func TestValidateUpstreamTarget(t *testing.T) {
	m := &contract.Manifest{AllowedHosts: []string{"cdn.vendor.example", "api.vendor.example:8443"}}
	cases := []struct {
		name    string
		baseURL string
		url     string
		wantErr bool
	}{
		{"相对路径放行", "https://api.vendor.example", "/v1/videos", false},
		{"渠道 host 命中", "https://api.vendor.example", "https://api.vendor.example/v1/videos", false},
		{"渠道 host 带端口命中", "http://10.0.0.5:9000", "http://10.0.0.5:9000/api/x", false},
		{"allowedHosts 无端口条目匹配任意端口", "https://api.vendor.example", "https://cdn.vendor.example:9443/file", false},
		{"allowedHosts 端口条目精确匹配", "https://api.vendor.example", "https://api.vendor.example:8443/x", false},
		{"未命中 host 拒绝", "https://api.vendor.example", "https://evil.example/v1/videos", true},
		{"渠道 host 不同端口且条目带端口拒绝", "https://api.vendor.example:8443", "https://api.vendor.example:9999/x", true},
		{"非 http(s) 绝对形态放行（按相对处理）", "https://api.vendor.example", "ftp://x/y", false},
	}
	for _, c := range cases {
		err := validateUpstreamTarget(c.baseURL, m, c.url)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: validateUpstreamTarget(%s, %s) err=%v, wantErr=%v", c.name, c.baseURL, c.url, err, c.wantErr)
		}
	}
}

// TestResolveUpstreamURL 相对路径拼接与空 baseUrl 前置报错：绝对 URL 原样透传、
// 相对路径拼渠道 baseUrl，baseUrl 缺失时报明确的配置错误（而非 http 层晦涩的
// `unsupported protocol scheme ""`——/v1/videos 线上报错的根因之一）。
func TestResolveUpstreamURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		target  string
		want    string
		wantErr bool
	}{
		{"绝对 URL 原样透传", "https://api.vendor.example", "https://cdn.vendor.example/v1/x", "https://cdn.vendor.example/v1/x", false},
		{"相对路径拼接", "https://api.vendor.example", "/v1/api/generate", "https://api.vendor.example/v1/api/generate", false},
		{"baseUrl 尾斜杠剥除", "https://api.vendor.example/", "/v1/api/generate", "https://api.vendor.example/v1/api/generate", false},
		{"无前导斜杠归一后拼接", "https://api.vendor.example", "v1/api/generate", "https://api.vendor.example/v1/api/generate", false},
		{"空 baseUrl 相对路径报配置错误", "", "/v1/api/generate", "", true},
		{"空白 baseUrl 同样报配置错误", "  ", "/v1/api/generate", "", true},
	}
	for _, c := range cases {
		got, err := resolveUpstreamURL(c.baseURL, c.target)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: resolveUpstreamURL(%q, %q) err=%v, wantErr=%v", c.name, c.baseURL, c.target, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("%s: resolveUpstreamURL(%q, %q) = %q, want %q", c.name, c.baseURL, c.target, got, c.want)
		}
	}

	// 错误消息须指向渠道配置缺失，可定位排障方向
	if _, err := resolveUpstreamURL("", "/v1/api/generate"); err == nil || !strings.Contains(err.Error(), "base_url") {
		t.Fatalf("空 baseUrl 应报配置缺失错误: %v", err)
	}
}

// absURLPlugin 构造一个 buildQueryRequest 直接返回任务 ID 作为绝对 URL 的插件源码
// （allowedHosts 由调用方注入，用于测试 manifest 声明面）。
func absURLPlugin(key string, allowedHosts string) string {
	return fmt.Sprintf(`export const meta = {
  apiVersion: 1, key: %q, name: "AbsURL", version: "1.0.0",
  models: ["abs_m"],
  allowedHosts: [%s],
  routes: [{ method: "GET", path: "/q", type: "query" }],
};
export function buildQueryRequest(ctx) {
  return { url: ctx.tasks[0].taskId, method: "GET" };
}
export function parseQueryResponse(ctx) {
  return { items: [{ taskId: ctx.tasks[0].taskId, done: true, taskData: { ok: 1 } }] };
}
`, key, allowedHosts)
}

func fetchTaskWith(t *testing.T, key, baseURL, targetURL string) (*http.Response, error) {
	t.Helper()
	adaptor := &Adaptor{}
	taskData, _ := json.Marshal(map[string]any{
		"task_id": targetURL, "platform": "js:" + key, "model": "abs_m",
	})
	return adaptor.FetchTask(context.Background(), baseURL, "sk-test", taskData)
}

// TestFetchTaskAbsoluteURLWhitelist 绝对 URL 主链路白名单：渠道 host 命中放行、
// 未命中 host 在发请求前被拒（等效白名单绕过封堵）。
func TestFetchTaskAbsoluteURLWhitelist(t *testing.T) {
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"done"}`))
	}))
	t.Cleanup(vendor.Close)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"evil"}`))
	}))
	t.Cleanup(other.Close)

	cleanup, err := jsplugin.PublishForTest(absURLPlugin("absurl", ""))
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	// 渠道 host 命中：绝对 URL 指向渠道自身
	resp, err := fetchTaskWith(t, "absurl", vendor.URL, vendor.URL+"/v1/q")
	if err != nil {
		t.Fatalf("渠道 host 绝对 URL 应放行: %v", err)
	}
	resp.Body.Close()

	// 未命中：指向另一台服务器，须在发请求前被拒
	if _, err := fetchTaskWith(t, "absurl", vendor.URL, other.URL+"/v1/q"); err == nil {
		t.Fatal("白名单外绝对 URL 应当被拒绝")
	} else if !strings.Contains(err.Error(), "白名单") {
		t.Fatalf("错误消息应说明白名单拒绝: %v", err)
	}
}

// TestFetchTaskAllowedHostsExtension manifest.allowedHosts 扩展白名单面：
// 声明的 host 即使不是渠道 host 也放行。
func TestFetchTaskAllowedHostsExtension(t *testing.T) {
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"done"}`))
	}))
	t.Cleanup(vendor.Close)
	// 渠道 base 指向不可达地址，绝对 URL 指向 vendor——只有 allowedHosts 声明才能命中
	host := strings.TrimPrefix(vendor.URL, "http://")
	src := absURLPlugin("absurl2", fmt.Sprintf("%q", host))
	cleanup, err := jsplugin.PublishForTest(src)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	resp, err := fetchTaskWith(t, "absurl2", "http://channel.invalid", vendor.URL+"/v1/q")
	if err != nil {
		t.Fatalf("allowedHosts 声明的 host 应放行: %v", err)
	}
	resp.Body.Close()
}

// TestFetchTaskRedirectGuard 重定向治理：白名单内跳转跟随、跳出白名单中止。
func TestFetchTaskRedirectGuard(t *testing.T) {
	outside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"evil":true}`))
	}))
	t.Cleanup(outside.Close)

	mux := http.NewServeMux()
	// 同 host 相对路径跳转（白名单内）
	mux.HandleFunc("/v1/ok1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/v1/final")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/v1/final", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"done"}`))
	})
	// 跳出白名单
	mux.HandleFunc("/v1/hop-out", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", outside.URL+"/steal")
		w.WriteHeader(http.StatusFound)
	})
	vendor := httptest.NewServer(mux)
	t.Cleanup(vendor.Close)

	cleanup, err := jsplugin.PublishForTest(absURLPlugin("absurl3", ""))
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	// 白名单内跳转：跟随到最终 200
	resp, err := fetchTaskWith(t, "absurl3", vendor.URL, vendor.URL+"/v1/ok1")
	if err != nil {
		t.Fatalf("白名单内重定向应跟随: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "done") {
		t.Fatalf("应跟随到最终响应: status=%d body=%s", resp.StatusCode, body)
	}

	// 跳出白名单：CheckRedirect 中止
	if _, err := fetchTaskWith(t, "absurl3", vendor.URL, vendor.URL+"/v1/hop-out"); err == nil {
		t.Fatal("跳出白名单的重定向应当被中止")
	}
}
