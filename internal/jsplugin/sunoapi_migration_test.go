package jsplugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// ── 迁移验证：new-api sunoapi 插件（原版 + 兼容垫片 + 钩子适配）跑通宿主全链 ──

func sunoapiSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("builtin/sunoapi/plugin.js")
	if err != nil {
		t.Fatalf("读取 sunoapi 插件失败: %v", err)
	}
	return string(data)
}

// TestMigratedSunoapiManifest meta 兼容垫片验证：本地化 description / map 形态
// usageSchema / 对象 author / 保留的 channelTypes（宽容忽略）/ dynamic 路由。
func TestMigratedSunoapiManifest(t *testing.T) {
	compiled := mustCompile(t, sunoapiSource(t))
	m := compiled.Manifest

	// 本地化 description → zh 优先
	if m.Description.String() != "SunoAPI 项目 音乐与歌词生成" {
		t.Fatalf("本地化 description 解析异常: %q", m.Description.String())
	}
	// map 形态 usageSchema → 数组（clips=number/count；action 的 enum 数组 → type enum）
	clips := m.UsageFieldByKey("clips")
	if clips == nil || clips.Type != "number" || clips.Unit != "count" {
		t.Fatalf("clips 解析异常: %+v", clips)
	}
	action := m.UsageFieldByKey("action")
	if action == nil || action.Type != "enum" {
		t.Fatalf("action（enum 数组形态）解析异常: %+v", action)
	}
	// 字符串 author（qianfree）
	if m.Author.Name != "qianfree" {
		t.Fatalf("author 解析异常: %+v", m.Author)
	}
	// dynamic 路由 + 别名
	if len(m.Routes) != 3 || m.Routes[1].Type != contract.RouteTypeDynamic ||
		m.Routes[1].Decode != "decodeBatch" || m.Routes[0].Decode != "decodeSubmit" {
		t.Fatalf("路由/别名解析异常: %+v", m.Routes)
	}
	if m.EffectiveFetchMode() != "batch" {
		t.Fatalf("fetchMode 应为 batch: %s", m.EffectiveFetchMode())
	}
}

// mockSunoAPI 模拟 Suno-API 上游：submit 受理 / fetch 批量查询（含终态歌曲数据）。
func mockSunoAPI(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/suno/submit/MUSIC", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "unauthorized"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "success", "message": "提交成功", "data": "suno-task-1"})
	})
	mux.HandleFunc("/suno/fetch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs []string `json:"ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := make([]map[string]any, 0, len(body.IDs))
		for _, id := range body.IDs {
			if id == "suno-task-1" {
				data = append(data, map[string]any{
					"task_id": id, "status": "success",
					"data": []map[string]any{{
						"id": "song-a", "audio_url": "https://cdn.suno.example/a.mp3",
						"image_url": "https://cdn.suno.example/a.jpg", "title": "Dawn",
					}},
				})
			} else {
				data = append(data, map[string]any{"task_id": id, "status": "processing"})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "success", "data": data})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestMigratedSunoapiFullFlow 引擎级全链：别名解码（含 dynamic 查询意图）→ 提交构造
// → 受理解析 → 批量查询构造/解析（usage/artifacts 终态映射）→ 渲染。
func TestMigratedSunoapiFullFlow(t *testing.T) {
	cleanup, err := PublishForTest(sunoapiSource(t))
	if err != nil {
		t.Fatalf("发布迁移插件失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, ok := CurrentGeneration().Plugin("sunoapi")
	if !ok {
		t.Fatal("sunoapi 未进 generation")
	}
	vendor := mockSunoAPI(t)

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

	submitRoute := plugin.Manifest.Routes[0]

	// 1) 别名解码：routes[0].decode=decodeSubmit（音乐提交）
	decoded, err := contract.ValidateDecoded(
		call(submitRoute.ResolveDecode(), contract.DecodeContext{
			Path: "/submit/music", Method: "POST", Params: map[string]string{"action": "music"},
			Body: json.RawMessage(`{"prompt":"a song about dawn"}`),
		}), plugin.Manifest, false)
	if err != nil || decoded.Model != "suno_music" {
		t.Fatalf("别名解码失败: %+v err=%v", decoded, err)
	}

	// 2) dynamic 别名解码：decodeBatch → kind=query（ids 批量查询）
	dynRoute := plugin.Manifest.Routes[1]
	qDecoded, err := contract.ValidateDecoded(
		call(dynRoute.ResolveDecode(), contract.DecodeContext{
			Path: "/fetch", Method: "POST", Body: json.RawMessage(`{"ids":["task_pub_1","task_pub_2"]}`),
		}), plugin.Manifest, true)
	if err != nil || qDecoded.Kind != "query" || len(qDecoded.TaskIDs) != 2 {
		t.Fatalf("dynamic 查询意图解码失败: %+v err=%v", qDecoded, err)
	}

	// 3) 估算（map 形态 usageSchema 的事实校验）
	estFacts, err := contract.ValidateUsageFacts(
		call("estimateUsage", contract.EstimateUsageContext{Body: decoded.RequestBody, Model: decoded.Model}),
		plugin.Manifest)
	if err != nil || estFacts["clips"] != 2 {
		t.Fatalf("estimateUsage 事实异常: %v err=%v", estFacts, err)
	}

	// 4) 提交构造 + 受理解析（相对 URL + 宿主注入鉴权）
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Params: map[string]string{"action": "music"},
		Model: decoded.Model, ChannelBaseURL: vendor.URL,
	}))
	if err != nil || built.URL != "/suno/submit/MUSIC" {
		t.Fatalf("buildSubmitRequest 异常: %+v err=%v", built, err)
	}
	sub, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status: 200, ResponseBody: json.RawMessage(`{"code":"success","message":"提交成功","data":"suno-task-1"}`),
	}))
	if err != nil || sub.TaskID != "suno-task-1" {
		t.Fatalf("parseSubmitResponse 异常: %+v err=%v", sub, err)
	}

	// 5) 批量查询构造 + 解析（fetchMode=batch 契约：ctx.tasks 整批）
	tasks := []contract.QueryTaskRef{{TaskID: "suno-task-1"}, {TaskID: "suno-task-2"}}
	qBuilt, err := contract.ValidateBuiltRequest(call("buildQueryRequest", contract.BuildQueryContext{Tasks: tasks}))
	if err != nil || qBuilt.URL != "/suno/fetch" {
		t.Fatalf("buildQueryRequest 异常: %+v err=%v", qBuilt, err)
	}
	var qBody map[string]any
	_ = json.Unmarshal(qBuilt.Body, &qBody)
	if ids, _ := qBody["ids"].([]any); len(ids) != 2 {
		t.Fatalf("批量 ids 异常: %v", qBody)
	}
	queried, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Status: 200, Tasks: tasks,
		ResponseBody: json.RawMessage(`{"code":"success","data":[
			{"task_id":"suno-task-1","status":"success","data":[{"id":"song-a","audio_url":"https://cdn.suno.example/a.mp3","image_url":"https://cdn.suno.example/a.jpg"}]},
			{"task_id":"suno-task-2","status":"processing"}]}`),
	}), tasks)
	if err != nil {
		t.Fatalf("parseQueryResponse 校验失败: %v", err)
	}
	if !queried.Items[0].Done || queried.Items[1].Done {
		t.Fatal("终态映射异常")
	}
	if len(queried.Items[0].Artifacts) != 2 { // audio + cover
		t.Fatalf("artifacts 声明异常: %+v", queried.Items[0].Artifacts)
	}
	facts, err := contract.ValidateUsageFacts(mustRawMap(t, queried.Items[0].Usage), plugin.Manifest)
	if err != nil || facts["clips"] != 1 {
		t.Fatalf("终态 usage 异常: %v err=%v", facts, err)
	}

	// 6) 渲染三钩子（new-api native 形态包装为 {status, body}）
	rend, err := contract.ValidateRendered(call("renderSubmit", contract.RenderTaskContext{
		Task: contract.RenderTask{PublicTaskID: "task_pub_1", TaskID: "suno-task-1", Status: "SUBMITTED", Data: sub.TaskData},
	}))
	if err != nil || rend.Status != 200 {
		t.Fatalf("renderSubmit 异常: %+v err=%v", rend, err)
	}
	rendTask, err := contract.ValidateRendered(call(submitRoute2(plugin).ResolveRender(contract.HookNameRenderTask),
		contract.RenderTaskContext{Task: contract.RenderTask{
			PublicTaskID: "task_pub_1", TaskID: "suno-task-1", Status: "SUCCESS", Data: queried.Items[0].TaskData,
		}}))
	if err != nil {
		t.Fatalf("renderTask 异常: %v", err)
	}
	var taskBody map[string]any
	_ = json.Unmarshal(rendTask.Body, &taskBody)
	if taskBody["code"] != "success" {
		t.Fatalf("renderTask body 形态异常: %v", taskBody)
	}
	rendTasks, err := contract.ValidateRendered(call("renderTasks", contract.RenderTasksContext{
		Tasks: []contract.RenderTask{{PublicTaskID: "task_pub_1", Status: "SUCCESS"}},
	}))
	if err != nil {
		t.Fatalf("renderTasks 异常: %v", err)
	}
	var tasksBody struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal(rendTasks.Body, &tasksBody)
	if len(tasksBody.Data) != 1 {
		t.Fatalf("renderTasks data 应为单元素数组: %s", rendTasks.Body)
	}
	_ = io.Discard
	_ = http.StatusOK
	_ = os.ReadFile
}

func submitRoute2(plugin *ActivePlugin) contract.Route {
	for _, r := range plugin.Manifest.Routes {
		if r.Type == contract.RouteTypeQuery {
			return r
		}
	}
	return contract.Route{}
}
