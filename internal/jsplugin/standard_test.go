// standard_test.go 标准端点解码计划合成的单元测试：声明判定（nil,nil 语义）、
// 钩子映射成功、钩子执行失败、kind=query 拒绝、模型未解析。
package jsplugin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// stdNoDeclareSource 未声明 standardVideos 的最小插件（不支持标准端点）
const stdNoDeclareSource = `export const meta = {
  apiVersion: 1, key: "stdplain", name: "StdPlain", version: "1.0.0",
  models: ["plain_model"], fetchMode: "per_task",
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeRequest(ctx) {
  return { kind: "submit", model: "plain_model", requestBody: ctx.body || {} };
}
export function buildSubmitRequest() { return { url: "/s", method: "POST", body: {} }; }
export function parseSubmitResponse() { return { taskId: "t" }; }
export function buildQueryRequest() { return { url: "/q", method: "GET" }; }
export function parseQueryResponse(ctx) {
  return { items: [{ taskId: (ctx.tasks || [])[0].taskId, done: false }] };
}`

// stdDeclareSource 声明 standardVideos 的插件：通用任务体 → 原生 requestBody 映射
const stdDeclareSource = `export const meta = {
  apiVersion: 1, key: "stdvideo", name: "StdVideo", version: "1.0.0",
  models: ["std_model"], fetchMode: "per_task", standardVideos: "decodeStd",
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeStd(ctx) {
  const raw = ctx.body || {};
  const metadata = raw.metadata || {};
  if (!String(raw.prompt || "").trim()) throw new Error("prompt is required");
  return { kind: "submit", model: "std_model",
    requestBody: { prompt: raw.prompt, duration: metadata.duration || raw.seconds || 0 } };
}
export function decodeRequest(ctx) {
  return { kind: "submit", model: "std_model", requestBody: ctx.body || {} };
}
export function buildSubmitRequest() { return { url: "/s", method: "POST", body: {} }; }
export function parseSubmitResponse() { return { taskId: "t" }; }
export function buildQueryRequest() { return { url: "/q", method: "GET" }; }
export function parseQueryResponse(ctx) {
  return { items: [{ taskId: (ctx.tasks || [])[0].taskId, done: false }] };
}`

// stdQueryKindSource 解码钩子返回 kind=query（标准端点无查询语义，应拒绝）
const stdQueryKindSource = `export const meta = {
  apiVersion: 1, key: "stdquery", name: "StdQuery", version: "1.0.0",
  models: ["q_model"], fetchMode: "per_task", standardVideos: "decodeStd",
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeStd() { return { kind: "query", taskIds: ["t1"] }; }
export function decodeRequest(ctx) {
  return { kind: "submit", model: "q_model", requestBody: ctx.body || {} };
}
export function buildSubmitRequest() { return { url: "/s", method: "POST", body: {} }; }
export function parseSubmitResponse() { return { taskId: "t" }; }
export function buildQueryRequest() { return { url: "/q", method: "GET" }; }
export function parseQueryResponse(ctx) {
  return { items: [{ taskId: (ctx.tasks || [])[0].taskId, done: false }] };
}`

// TestSynthesizeStandardPlan_NoDeclaration 未声明 standardVideos → (nil, nil)，
// 调用方据此走「不支持标准端点」的明确拒绝分支。
func TestSynthesizeStandardPlan_NoDeclaration(t *testing.T) {
	cleanup, err := PublishForTest(stdNoDeclareSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	plan, err := SynthesizeStandardPlan(context.Background(), "stdplain",
		contract.SurfaceVideoGenerations, "/v1/video/generations",
		[]byte(`{"model":"plain_model","prompt":"hi"}`), "plain_model")
	if err != nil {
		t.Fatalf("未声明不应报错: %v", err)
	}
	if plan != nil {
		t.Fatalf("未声明应返回 nil 计划，得到 %+v", plan)
	}
}

// TestSynthesizeStandardPlan_Success 声明并映射成功：计划携带插件、端点路径与
// 映射后的 requestBody（metadata.duration → duration）。
func TestSynthesizeStandardPlan_Success(t *testing.T) {
	cleanup, err := PublishForTest(stdDeclareSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	plan, err := SynthesizeStandardPlan(context.Background(), "stdvideo",
		contract.SurfaceOpenAIVideos, "/v1/videos",
		[]byte(`{"model":"std_model","prompt":"a cat","seconds":"8","metadata":{"duration":8,"size":"1280x720"}}`),
		"std_model")
	if err != nil {
		t.Fatalf("合成失败: %v", err)
	}
	if plan == nil || plan.Plugin == nil || plan.Plugin.Key != "stdvideo" {
		t.Fatalf("计划插件异常: %+v", plan)
	}
	if plan.NativePath != "/v1/videos" {
		t.Fatalf("NativePath 应为标准端点路径，得到 %q", plan.NativePath)
	}
	if plan.Decoded == nil || plan.Decoded.Model != "std_model" {
		t.Fatalf("解码结果异常: %+v", plan.Decoded)
	}
	var body map[string]any
	if err := json.Unmarshal(plan.Decoded.RequestBody, &body); err != nil {
		t.Fatalf("requestBody 非法 JSON: %v", err)
	}
	if body["prompt"] != "a cat" || body["duration"] != float64(8) {
		t.Fatalf("映射结果异常（metadata.duration 应映射为 duration）: %v", body)
	}
}

// TestSynthesizeStandardPlan_HookThrows 解码钩子抛错 → 错误透出（调用方 400）。
func TestSynthesizeStandardPlan_HookThrows(t *testing.T) {
	cleanup, err := PublishForTest(stdDeclareSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	_, err = SynthesizeStandardPlan(context.Background(), "stdvideo",
		contract.SurfaceOpenAIVideos, "/v1/videos", []byte(`{"model":"std_model"}`), "std_model")
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("应透出钩子错误，得到 %v", err)
	}
}

// TestSynthesizeStandardPlan_QueryKindRejected kind=query 无 dynamic 语境 → 拒绝。
func TestSynthesizeStandardPlan_QueryKindRejected(t *testing.T) {
	cleanup, err := PublishForTest(stdQueryKindSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	_, err = SynthesizeStandardPlan(context.Background(), "stdquery",
		contract.SurfaceVideoGenerations, "/v1/video/generations",
		[]byte(`{"model":"q_model","prompt":"hi"}`), "q_model")
	if err == nil || !strings.Contains(err.Error(), "dynamic") {
		t.Fatalf("kind=query 应被拒绝，得到 %v", err)
	}
}

// TestSynthesizeStandardPlan_PluginNotActive 插件不在当前 generation（禁用/不存在）→ 错误。
func TestSynthesizeStandardPlan_PluginNotActive(t *testing.T) {
	cleanup, err := PublishForTest(stdDeclareSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	_, err = SynthesizeStandardPlan(context.Background(), "nope",
		contract.SurfaceOpenAIVideos, "/v1/videos", []byte(`{"model":"std_model"}`), "std_model")
	if err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("插件未激活应报错，得到 %v", err)
	}
}

// TestCheckReferenceRoles 标准端点参考素材的插件能力拦截：未声明 referenceRoles 仅放行首帧，
// 声明后按声明放行；无 references / 非 JSON 不拦截。
func TestCheckReferenceRoles(t *testing.T) {
	legacy := &contract.Manifest{Key: "legacy"}
	full := &contract.Manifest{Key: "full", ReferenceRoles: []string{"first_frame", "reference_image", "reference_video"}}
	cases := []struct {
		name    string
		m       *contract.Manifest
		body    string
		wantErr bool
	}{
		{"无 references", legacy, `{"prompt":"p"}`, false},
		{"非 JSON", legacy, `not json`, false},
		{"未声明仅首帧", legacy, `{"references":[{"role":"first_frame","url":"https://a"}]}`, false},
		{"未声明拒参考图", legacy, `{"references":[{"role":"reference_image","url":"https://a"}]}`, true},
		{"声明放行", full, `{"references":[{"role":"reference_image","url":"https://a"},{"role":"reference_video","url":"https://v"}]}`, false},
		{"声明外拒绝", full, `{"references":[{"role":"reference_audio","url":"https://a"}]}`, true},
	}
	for _, c := range cases {
		err := checkReferenceRoles(c.m, c.m.Key, []byte(c.body))
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
		}
		var roleErr *ReferenceRoleError
		if c.wantErr && !errors.As(err, &roleErr) {
			t.Errorf("%s: 应为 ReferenceRoleError，got %T", c.name, err)
		}
	}
}
