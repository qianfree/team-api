package jsbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// filerefTestPlugin 文件引用测试插件：buildSubmitRequest 把上传文件以占位符
// 放进 body 任意深度（base64 与 dataUrl 两种编码、含 maxBytes 上限声明）。
const filerefTestPlugin = `
export const meta = {
  apiVersion: 1, key: "fileref-demo", name: "F", version: "1.0.0",
  models: ["img_model"],
  routes: [{ method: "POST", path: "/edit", type: "submit" }],
};
export function decodeRequest(ctx) {
  return { kind: "submit", model: "img_model", requestBody: ctx.body || {} };
}
export function buildSubmitRequest(ctx) {
  const req = { prompt: "edit this" };
  if (ctx.body.wantDataUrl) {
    req.image = { __fileRef: "request_file:image", encoding: "dataUrl", maxBytes: 1000000 };
  } else {
    req.image = { __fileRef: "request_file:image", encoding: "base64" };
    req.meta = { nested: { frames: [{ __fileRef: "request_file:image#1", encoding: "base64" }] } };
  }
  return { url: "/v1/edit", method: "POST", body: req };
}
export function parseSubmitResponse(ctx) {
  const r = ctx.responseBody || {};
  if (!r.id) throw new Error("no id");
  return { taskId: String(r.id) };
}
`

func publishFileRefPlugin(t *testing.T) *jsplugin.ActivePlugin {
	t.Helper()
	cleanup, err := jsplugin.PublishForTest(filerefTestPlugin)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, ok := jsplugin.CurrentGeneration().Plugin("fileref-demo")
	if !ok {
		t.Fatal("fileref-demo 未进 generation")
	}
	return plugin
}

func fileRefPlan(plugin *jsplugin.ActivePlugin, files map[string]jsplugin.FilePayload, body string) context.Context {
	decoded := &contract.DecodeResult{
		Kind:        "submit",
		Model:       "img_model",
		RequestBody: json.RawMessage(body),
	}
	return jsplugin.WithSubmitPlan(context.Background(), &jsplugin.SubmitPlan{
		Plugin:  plugin,
		Decoded: decoded,
		Files:   files,
	})
}

// TestInlineFileRefs 纯函数：占位符替换（base64/dataUrl/深度嵌套/多文件 ref/超限/未知字段/缺引用）。
func TestInlineFileRefs(t *testing.T) {
	files := map[string]jsplugin.FilePayload{
		"request_file:image":   {Filename: "a.png", MimeType: "image/png", Data: []byte("PNGDATA")},
		"request_file:image#1": {Filename: "b.png", MimeType: "image/png", Data: []byte("PNGDATA2")},
	}

	// 深度嵌套 + 双编码
	body := []byte(`{"image":{"__fileRef":"request_file:image","encoding":"base64"},"meta":{"nested":{"frames":[{"__fileRef":"request_file:image#1","encoding":"base64"}]}}}`)
	out, err := inlineFileRefs(body, files)
	if err != nil {
		t.Fatalf("内联失败: %v", err)
	}
	var parsed map[string]any
	_ = json.Unmarshal(out, &parsed)
	if parsed["image"] != base64.StdEncoding.EncodeToString([]byte("PNGDATA")) {
		t.Fatalf("base64 内联异常: %v", parsed["image"])
	}
	meta := parsed["meta"].(map[string]any)["nested"].(map[string]any)
	if meta["frames"].([]any)[0] != base64.StdEncoding.EncodeToString([]byte("PNGDATA2")) {
		t.Fatalf("嵌套内联异常: %v", meta["frames"])
	}

	// dataUrl
	out, err = inlineFileRefs([]byte(`{"image":{"__fileRef":"request_file:image","encoding":"dataUrl","mimeType":"image/jpeg"}}`), files)
	if err != nil {
		t.Fatalf("dataUrl 内联失败: %v", err)
	}
	_ = json.Unmarshal(out, &parsed)
	want := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("PNGDATA"))
	if parsed["image"] != want {
		t.Fatalf("dataUrl/mimeType 覆盖异常: %v", parsed["image"])
	}

	// maxBytes 超限
	_, err = inlineFileRefs([]byte(`{"image":{"__fileRef":"request_file:image","encoding":"base64","maxBytes":3}}`), files)
	if err == nil {
		t.Fatal("超限应拒绝")
	}

	// 未知字段
	_, err = inlineFileRefs([]byte(`{"image":{"__fileRef":"request_file:image","evil":1}}`), files)
	if err == nil {
		t.Fatal("未知字段应拒绝")
	}

	// 缺失引用
	_, err = inlineFileRefs([]byte(`{"image":{"__fileRef":"request_file:not_uploaded","encoding":"base64"}}`), files)
	if err == nil {
		t.Fatal("缺失引用应拒绝")
	}

	// 无占位符原样返回
	plain := []byte(`{"a":1}`)
	out, err = inlineFileRefs(plain, files)
	if err != nil || string(out) != string(plain) {
		t.Fatalf("无占位符应原样返回: %s %v", out, err)
	}
}

// TestSubmitFlowWithFileRefs 端到端：multipart 上传的文件经 plan → 桥 DoRequest
// 内联发送，mock 上游回显收到的内联字段。
func TestSubmitFlowWithFileRefs(t *testing.T) {
	plugin := publishFileRefPlugin(t)

	var received map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/edit", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"id":"edit-42"}`))
	})
	vendor := httptest.NewServer(mux)
	t.Cleanup(vendor.Close)

	files := map[string]jsplugin.FilePayload{
		"request_file:image":   {Filename: "cat.png", MimeType: "image/png", Data: []byte("CATPIC")},
		"request_file:image#1": {Filename: "cat2.png", MimeType: "image/png", Data: []byte("CATPIC2")},
	}
	ctx := fileRefPlan(plugin, files, `{"prompt":"edit"}`)

	adaptor := &Adaptor{}
	info := relayInfo(vendor.URL)
	info.ChannelMeta.Settings.PluginKey = "fileref-demo"
	adaptor.Init(info)
	if taskErr := adaptor.ValidateRequest(ctx, info, nil); taskErr != nil {
		t.Fatalf("ValidateRequest 失败: %s", taskErr.Message)
	}
	resp, err := adaptor.DoRequest(ctx, info, nil)
	if err != nil {
		t.Fatalf("DoRequest 失败: %v", err)
	}
	taskID, _, taskErr := adaptor.DoResponse(ctx, resp, info)
	if taskErr != nil || taskID != "edit-42" {
		t.Fatalf("DoResponse 异常: %s %v", taskID, taskErr)
	}

	// 上游收到的 image 应为 base64(占位符已替换，JS 从未见过字节)
	if received["image"] != base64.StdEncoding.EncodeToString([]byte("CATPIC")) {
		t.Fatalf("上游未收到内联字节: %v", received["image"])
	}
	meta := received["meta"].(map[string]any)["nested"].(map[string]any)
	if meta["frames"].([]any)[0] != base64.StdEncoding.EncodeToString([]byte("CATPIC2")) {
		t.Fatalf("嵌套占位符未内联: %v", meta["frames"])
	}
}
