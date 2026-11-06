package jsplugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// ── kling 迁移验证：JWT 签名鉴权 + 双 API（v2/v3 与 kling-4.0）+ units 计费 + fileRef ──

func klingSource(t *testing.T) string {
	t.Helper()
	return readBuiltin(t, "kling")
}

func TestMigratedKlingManifest(t *testing.T) {
	compiled := mustCompile(t, klingSource(t))
	m := compiled.Manifest
	if m.Key != "kling" || len(m.Models) != 4 || len(m.Routes) != 6 {
		t.Fatalf("manifest 异常: models=%v routes=%d", m.Models, len(m.Routes))
	}
	if m.UsageFieldByKey("units") == nil {
		t.Fatalf("usageSchema（units）缺失: %+v", m.UsageSchema)
	}
}

func TestMigratedKlingV2Flow(t *testing.T) {
	cleanup, err := PublishForTest(klingSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("kling")

	call := func(hook string, arg any) map[string]json.RawMessage {
		t.Helper()
		args, _ := json.Marshal(arg)
		out, err := plugin.Pool.Call(context.Background(), hook, args)
		if err != nil {
			t.Fatalf("钩子 %s 失败: %v", hook, err)
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			t.Fatalf("钩子 %s 返回非对象: %v", hook, err)
		}
		return raw
	}

	// v2/v3 解码（model_name 原生形态）
	decoded, err := contract.ValidateDecoded(call("decodeRequest", contract.DecodeContext{
		Path: "/videos/text2video", Method: "POST",
		Body: json.RawMessage(`{"model_name":"kling-v2-6","prompt":"a dog","duration":5,"mode":"std"}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "kling-v2-6" {
		t.Fatalf("v2 解码失败: %+v err=%v", decoded, err)
	}

	// 估算 units：0.3（std）× 5s = 1.5
	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["units"] != 1.5 {
		t.Fatalf("v2 units 估算异常: %v err=%v", facts, err)
	}

	// build：JWT 鉴权头（ak|sk → Authorization 覆盖）+ 相对 URL
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
		ChannelBaseURL: "https://api.kling.example.com",
		ApiKey:         "myAccessKey|mySecretKey",
	}))
	if err != nil || built.URL != "/v1/videos/text2video" {
		t.Fatalf("v2 build 异常: %+v err=%v", built, err)
	}
	auth := built.Headers["Authorization"]
	if !strings.HasPrefix(auth, "Bearer ey") {
		t.Fatalf("JWT 鉴权头缺失: %q", auth)
	}
	// JWT 三段且 payload 含 iss=myAccessKey（RawURL 编码解码校验）
	parts := strings.Split(auth[7:], ".")
	if len(parts) != 3 {
		t.Fatalf("JWT 应为三段: %v", parts)
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !strings.Contains(string(payloadJSON), "myAccessKey") {
		t.Fatalf("JWT payload 缺少 iss: %s err=%v", payloadJSON, err)
	}

	// 受理解析（v2：data.task_id + _action 标记）
	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Path: "/videos/text2video", Model: decoded.Model,
		ResponseBody: json.RawMessage(`{"code":0,"message":"success","data":{"task_id":"kl-42","task_status":"submitted"}}`),
	}))
	if err != nil || sub.TaskID != "kl-42" {
		t.Fatalf("v2 受理解析异常: %+v err=%v", sub, err)
	}
	var subData map[string]any
	_ = json.Unmarshal(sub.TaskData, &subData)
	if subData["_action"] != "text_to_video" {
		t.Fatalf("_action 标记缺失: %v", subData)
	}

	// 轮询构造（action 从 taskData 恢复）与终态解析（units 结算 + 产物 URL）
	taskRef := contract.QueryTaskRef{TaskID: "kl-42", TaskData: sub.TaskData}
	qBuilt, err := contract.ValidateBuiltRequest(call("buildQueryRequest", contract.BuildQueryContext{
		Tasks: []contract.QueryTaskRef{taskRef}, Model: decoded.Model,
	}))
	if err != nil || qBuilt.URL != "/v1/videos/text2video/kl-42" {
		t.Fatalf("v2 查询构造异常: %+v err=%v", qBuilt, err)
	}
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"code":0,"data":{"task_id":"kl-42","task_status":"succeed","final_unit_deduction":"2.5","task_result":{"videos":[{"url":"https://cdn.kling.example/v.mp4","duration":"5"}]}}}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("v2 轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || len(item.Artifacts) != 1 {
		t.Fatalf("v2 终态 item 异常: %+v", item)
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	if err != nil || termFacts["units"] != 2.5 {
		t.Fatalf("v2 终态 units 异常: %v err=%v", termFacts, err)
	}
}

func TestMigratedKling4Flow(t *testing.T) {
	cleanup, err := PublishForTest(klingSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("kling")

	call := func(hook string, arg any) map[string]json.RawMessage {
		t.Helper()
		args, _ := json.Marshal(arg)
		out, err := plugin.Pool.Call(context.Background(), hook, args)
		if err != nil {
			t.Fatalf("钩子 %s 失败: %v", hook, err)
		}
		raw, err := contract.ParseJSONObject(out)
		if err != nil {
			t.Fatalf("钩子 %s 返回非对象: %v", hook, err)
		}
		return raw
	}

	// kling-4.0 解码（vendorBody 透传，路由路径分流）
	decoded, err := contract.ValidateDecoded(call("decodeRequest", contract.DecodeContext{
		Path: "/text-to-video/kling-4.0", Method: "POST",
		Body: json.RawMessage(`{"prompt":"future city","settings":{"resolution":"1080p","duration":10}}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "kling-4.0" {
		t.Fatalf("k4 解码失败: %+v err=%v", decoded, err)
	}

	// 估算 units：1080p 1.5 × 10s = 15
	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["units"] != 15 {
		t.Fatalf("k4 units 估算异常: %v err=%v", facts, err)
	}

	// build：/text-to-video/kling-4.0
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
		ApiKey: "ak|sk",
	}))
	if err != nil || built.URL != "/text-to-video/kling-4.0" {
		t.Fatalf("k4 build 异常: %+v err=%v", built, err)
	}

	// 受理（data.id → _kling4 标记）
	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, Path: "/text-to-video/kling-4.0", Model: "kling-4.0",
		ResponseBody: json.RawMessage(`{"code":0,"data":{"id":"k4-abc","status":"submitted"}}`),
	}))
	if err != nil || sub.TaskID != "k4-abc" {
		t.Fatalf("k4 受理解析异常: %+v err=%v", sub, err)
	}

	// 轮询：/tasks?task_ids= + billing units 结算
	taskRef := contract.QueryTaskRef{TaskID: "k4-abc", TaskData: sub.TaskData}
	qBuilt, err := contract.ValidateBuiltRequest(call("buildQueryRequest", contract.BuildQueryContext{
		Tasks: []contract.QueryTaskRef{taskRef}, Model: "kling-4.0",
	}))
	if err != nil || qBuilt.URL != "/tasks?task_ids=k4-abc" {
		t.Fatalf("k4 查询构造异常: %+v err=%v", qBuilt, err)
	}
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"code":0,"data":[{"id":"k4-abc","status":"succeeded","billing":[{"charge_type":"unit","amount":14.2}],"outputs":[{"type":"video","url":"https://cdn.kling.example/k4.mp4","duration":"10"}]}]}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("k4 轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done {
		t.Fatal("k4 应终态")
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	if err != nil || termFacts["units"] != 14.2 {
		t.Fatalf("k4 billing units 异常: %v err=%v", termFacts, err)
	}
}
