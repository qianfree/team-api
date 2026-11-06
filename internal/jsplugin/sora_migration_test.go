package jsplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// ── sora 迁移验证：文生视频全链 + remix + 鉴权产物端点声明 ──

func soraSource(t *testing.T) string {
	t.Helper()
	return readBuiltin(t, "sora")
}

func readBuiltin(t *testing.T, key string) string {
	t.Helper()
	data, err := os.ReadFile("builtin/" + key + "/plugin.js")
	if err != nil {
		t.Fatalf("读取内置插件 %s 失败: %v", key, err)
	}
	return string(data)
}

func mockSoraVendor(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/videos", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"id":"video_abc","object":"video","status":"queued"}`))
	})
	mux.HandleFunc("/v1/videos/video_abc", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"video_abc","object":"video","status":"completed","seconds":8,"size":"1280x720"}`))
	})
	mux.HandleFunc("/v1/videos/video_abc/content", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("FAKESORAVIDEO"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestMigratedSoraManifest(t *testing.T) {
	compiled := mustCompile(t, soraSource(t))
	m := compiled.Manifest
	if m.Key != "sora" || len(m.Models) != 2 {
		t.Fatalf("manifest 异常: %+v", m)
	}
	if m.UsageFieldByKey("seconds") == nil || m.UsageFieldByKey("size") == nil {
		t.Fatalf("usageSchema（map 形态）解析异常: %+v", m.UsageSchema)
	}
	if len(m.Routes) != 3 {
		t.Fatalf("路由数异常: %+v", m.Routes)
	}
}

func TestMigratedSoraFullFlow(t *testing.T) {
	cleanup, err := PublishForTest(soraSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := CurrentGeneration().Plugin("sora")
	_ = mockSoraVendor(t)

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

	// decode：文生视频
	decoded, err := contract.ValidateDecoded(call("decodeRequest", contract.DecodeContext{
		Path: "/videos", Method: "POST",
		Body: json.RawMessage(`{"model":"sora-2","prompt":"a cat","seconds":8,"size":"1280x720"}`),
	}), plugin.Manifest, false)
	if err != nil || decoded.Model != "sora-2" {
		t.Fatalf("解码失败: %+v err=%v", decoded, err)
	}

	// estimateUsage（seconds→second unit）
	facts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || facts["seconds"] != 8 {
		t.Fatalf("估算事实异常: %v err=%v", facts, err)
	}

	// build：普通提交 / remix 提交（remix_of 恢复路径）
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model,
	}))
	if err != nil || built.URL != "/v1/videos" {
		t.Fatalf("build 异常: %+v err=%v", built, err)
	}
	remixBody, _ := json.Marshal(map[string]any{"prompt": "remix it", "remix_of": "video_abc"})
	rb, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: remixBody, Model: "sora-2",
	}))
	if err != nil || rb.URL != "/v1/videos/video_abc/remix" {
		t.Fatalf("remix build 异常: %+v err=%v", rb, err)
	}

	// parseSubmitResponse
	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, ResponseBody: json.RawMessage(`{"id":"video_abc","status":"queued"}`),
	}))
	if err != nil || sub.TaskID != "video_abc" {
		t.Fatalf("受理解析异常: %+v err=%v", sub, err)
	}

	// 轮询：completed → usage（seconds/size）+ 鉴权产物（相对端点）
	taskRef := contract.QueryTaskRef{TaskID: "video_abc", TaskData: sub.TaskData}
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: []contract.QueryTaskRef{taskRef}, ChannelBaseURL: "https://up.example.com",
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"completed","seconds":8,"size":"1280x720"}`),
	}), []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("轮询解析失败: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || len(item.Artifacts) != 1 || item.Artifacts[0].Source.Auth != "channel-bearer" {
		t.Fatalf("终态 item 异常: %+v", item)
	}
	// taskData 内含相对端点（流水线按渠道 base 解析 + 注入鉴权）
	var td map[string]any
	_ = json.Unmarshal(item.TaskData, &td)
	if td["_content_url"] != "/v1/videos/video_abc/content" {
		t.Fatalf("产物相对端点缺失: %v", td)
	}
	termFacts, err := contract.ValidateUsageFacts(mustRawMap(t, item.Usage), plugin.Manifest)
	if err != nil || termFacts["seconds"] != 8 {
		t.Fatalf("终态用量异常: %v err=%v", termFacts, err)
	}

	// 渲染
	rend, err := contract.ValidateRendered(call("renderTask", contract.RenderTaskContext{
		Task: contract.RenderTask{PublicTaskID: "task_pub", Model: "sora-2", Status: "SUCCESS", Data: item.TaskData},
	}))
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	var body map[string]any
	_ = json.Unmarshal(rend.Body, &body)
	if body["status"] != "completed" || body["object"] != "video" {
		t.Fatalf("渲染形态异常: %v", body)
	}
}
