// jsdispatcher_headers_test.go 解码钩子 ctx.headers 的分发器单测。
package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
)

// headerProbePluginSource 解码钩子回显 ctx.headers 的探针插件：只回显头名列表与一个
// 普通头的值——头名不会被消息脱敏规则抹值，「凭据头是否被剔除」的断言才是有效的
// （回显值的话，脱敏规则同样会把 Authorization / Cookie 的值抹成 ***，断言会空过）。
const headerProbePluginSource = `export const meta = {
  apiVersion: 1, key: "hdrprobe", name: "Header Probe", version: "1.0.0",
  models: ["hdr_model"],
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeRequest(ctx) {
  const headers = ctx.headers || {};
  throw new Error("KEYS:" + JSON.stringify(Object.keys(headers).sort()) + " TRACE:" + headers["X-Trace-Id"]);
}
export function buildSubmitRequest(ctx) { return { url: "/submit" }; }
export function parseSubmitResponse(ctx) { return { taskId: "t1" }; }
export function buildQueryRequest(ctx) { return { url: "/q" }; }
export function parseQueryResponse(ctx) { return { items: [{ taskId: ctx.tasks[0].taskId, done: false }] }; }
`

// TestJSDispatch_DecodeSeesClientHeaders 解码钩子的 ctx.headers：分发器把客户端请求头
// （首值）传给 decodeRequest，凭据类头（网关 API Key 的 Authorization、Cookie）不进插件。
// 此前分发器从未填充该字段——契约把它列为入参，按文档读取的插件恒拿到 undefined。
func TestJSDispatch_DecodeSeesClientHeaders(t *testing.T) {
	base := setupJSDispatcher(t, nil)
	cleanup, err := jsplugin.PublishForTest(headerProbePluginSource)
	if err != nil {
		t.Fatalf("发布探针插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	req, err := http.NewRequest(http.MethodPost, base+"/pjs/hdrprobe/submit", strings.NewReader(`{"model":"hdr_model"}`))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trace-Id", "trace-123")
	req.Header.Set("Accept-Language", "zh-CN")
	req.Header.Set("Authorization", "Bearer sk-gateway-key")
	req.Header.Set("Cookie", "session=secret-session")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("响应不是 JSON: %s", raw)
	}
	msg := out.Error.Message
	if resp.StatusCode != http.StatusBadGateway || !strings.HasPrefix(msg, "jsplugin hdrprobe hook decodeRequest: Error: KEYS:") {
		t.Fatalf("探针应经钩子抛错回显 headers: status=%d body=%s", resp.StatusCode, raw)
	}
	for _, want := range []string{`"Accept-Language"`, `"Content-Type"`, `"X-Trace-Id"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("解码钩子应看到请求头 %s: %s", want, msg)
		}
	}
	for _, banned := range []string{"Authorization", "Cookie"} {
		if strings.Contains(msg, `"`+banned+`"`) {
			t.Errorf("凭据头 %s 不应进解码上下文: %s", banned, msg)
		}
	}
	if !strings.HasSuffix(msg, "TRACE:trace-123") {
		t.Errorf("请求头应取首值: %s", msg)
	}
}
