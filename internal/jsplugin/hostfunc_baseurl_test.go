package jsplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fetchRelativeFixture 相对路径 fetch 的最小插件：probe 钩子按入参 url 发起 utils.fetch
// 并回传 {status, body}（不参与任务链，仅探测宿主能力行为）。
func fetchRelativeFixture(t *testing.T) *CompiledPlugin {
	t.Helper()
	src := `
export const meta = {
  apiVersion: 1, key: "fetchrel", name: "fetch relative", version: "1.0.0",
  models: ["fetchrel_model"],
  routes: [
    { method: "POST", path: "/submit/:action", type: "submit" },
    { method: "GET",  path: "/fetch/:task_id",  type: "query" },
  ],
};
export function decodeRequest(ctx) { return { kind: "submit", model: "fetchrel_model", requestBody: ctx.body || {} }; }
export function buildSubmitRequest(ctx) { return { url: "/v1/create", method: "POST", body: ctx.body || {} }; }
export function parseSubmitResponse(ctx) { return { taskId: "t1" }; }
export function buildQueryRequest(ctx) { return { url: "/v1/query", method: "GET" }; }
export function parseQueryResponse(ctx) { return { items: [{ taskId: "t1", done: true }] }; }
export function probe(ctx) {
  const response = utils.fetch({ url: ctx.url });
  return { status: response.status, body: response.body };
}
`
	return mustCompile(t, src)
}

// TestFetchRelativeURLResolvedAgainstBaseURL 相对路径 utils.fetch：
//   - 注入 WithBaseURL + WithAllowedHosts（桥接层同款）→ 按渠道 baseUrl 拼接后放行
//   - 未注入 WithBaseURL（decode/render 等无渠道上下文的钩子）→ 明确拒绝
//   - 白名单仍对最终绝对 URL 生效（拼接 host=渠道 host 天然命中，外部绝对 URL 照拒）
func TestFetchRelativeURLResolvedAgainstBaseURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/retrieve", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("file_id") != "f1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"file":{"download_url":"https://vid.example/s.mp4"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("解析测试服务器地址失败: %v", err)
	}
	host := u.Host

	inst, err := NewGojaEngine().NewInstance(fetchRelativeFixture(t), InstanceOptions{
		CallTimeout:  5 * time.Second,
		AllowedHosts: []string{host}, // 实例级白名单（等价 manifest.allowedHosts 场景）
	})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	t.Cleanup(inst.Close)

	call := func(ctx context.Context, fetchURL string) error {
		args, _ := json.Marshal(map[string]any{"url": fetchURL})
		_, err := inst.Call(ctx, "probe", args)
		return err
	}

	// 相对路径 + 注入渠道 baseUrl → 解析命中测试服务器
	ctx := WithBaseURL(WithAllowedHosts(context.Background(), []string{host}), srv.URL)
	args, _ := json.Marshal(map[string]any{"url": "/v1/files/retrieve?file_id=f1"})
	out, err := inst.Call(ctx, "probe", args)
	if err != nil {
		t.Fatalf("相对路径 fetch 失败: %v", err)
	}
	var probe struct {
		Status int `json:"status"`
		Body   struct {
			File struct {
				DownloadURL string `json:"download_url"`
			} `json:"file"`
		} `json:"body"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatalf("probe 返回解析失败: %v", err)
	}
	if probe.Status != http.StatusOK || probe.Body.File.DownloadURL != "https://vid.example/s.mp4" {
		t.Fatalf("相对路径解析异常: status=%d body=%+v", probe.Status, probe.Body)
	}

	// 无渠道上下文 → 明确拒绝（不能静默拼到任意地址）
	if err := call(WithAllowedHosts(context.Background(), []string{host}), "/v1/files/retrieve?file_id=f1"); err == nil || !strings.Contains(err.Error(), "渠道上下文") {
		t.Fatalf("未注入渠道 baseUrl 应明确拒绝，实际: %v", err)
	}

	// 白名单外的绝对 URL 仍被拒（相对路径支持未扩大出网面）
	if err := call(ctx, "https://evil.example.com/x"); err == nil || !strings.Contains(err.Error(), "白名单") {
		t.Fatalf("白名单外绝对 URL 应被拒，实际: %v", err)
	}

	// baseUrl 缺斜杠边界：TrimRight 语义下拼接不产生双斜杠路径
	served := false
	mux.HandleFunc("/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		served = true
		_, _ = w.Write([]byte(`{}`))
	})
	ctxNoSlash := WithBaseURL(WithAllowedHosts(context.Background(), []string{host}), srv.URL+"/")
	argsPing, _ := json.Marshal(map[string]any{"url": "v1/ping"})
	if _, err := inst.Call(ctxNoSlash, "probe", argsPing); err != nil {
		t.Fatalf("无前导斜杠相对路径应补斜杠后放行: %v", err)
	}
	if !served {
		t.Fatal("无前导斜杠相对路径未命中测试服务器")
	}
}
