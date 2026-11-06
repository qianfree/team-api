package jsbridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/relay/common"
)

// batchPluginSource 批量轮询示例插件源码（fetchMode=batch，suno 式 fetch 批量端点）。
const batchPluginSource = `
export const meta = {
  apiVersion: 1,
  key: "batch-demo",
  name: "Batch Demo",
  version: "1.0.0",
  models: ["batch_music"],
  fetchMode: "batch",
  usageSchema: [{ key: "clips", type: "number", unit: "count" }],
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};

export function buildQueryRequest(ctx) {
  const ids = (ctx.tasks || []).map(function (t) { return t.taskId; });
  return {
    url: "/v1/batch/fetch",
    method: "POST",
    body: { ids: ids },
  };
}

export function parseQueryResponse(ctx) {
  const byId = {};
  const list = ((ctx.responseBody || {}).list) || [];
  for (const it of list) byId[it.task_id] = it;
  const items = [];
  for (const t of (ctx.tasks || [])) {
    const it = byId[t.taskId];
    if (!it) continue; // 上游未回报本任务：省略该项（轮询循环保持任务现状）
    if (it.status === "success") {
      items.push({
        taskId: t.taskId, done: true, taskData: it,
        usage: { clips: Number(it.clips || 1) },
        artifacts: [{ key: "audio", source: { type: "url", path: "url" } }],
      });
    } else if (it.status === "failed") {
      items.push({ taskId: t.taskId, done: true, error: { code: "upstream", message: it.error || "failed" } });
    } else {
      items.push({ taskId: t.taskId, done: false, taskData: it });
    }
  }
  return { items: items };
}
`

// TestBatchFetchMode 批量插件：FetchModeFor 反映 manifest 声明。
// 以 platform 判定（轮询循环拿到的是工厂新实例，pluginKey 未初始化——
// 旧按实例状态判定的实现恒得 per_task，批量通道从未激活过）。
func TestBatchFetchMode(t *testing.T) {
	cleanup, err := jsplugin.PublishForTest(batchPluginSource)
	if err != nil {
		t.Fatalf("发布批量插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	a := &Adaptor{} // 工厂新实例，不预置任何状态
	if got := a.FetchModeFor("js:batch-demo"); got != "batch" {
		t.Fatalf("FetchModeFor(js:batch-demo) 应为 batch: %s", got)
	}
	if got := a.FetchModeFor("js:not-published"); got != "per_task" {
		t.Fatalf("未激活插件应回落 per_task: %s", got)
	}
	if got := a.FetchModeFor("suno"); got != "per_task" {
		t.Fatalf("非 JS 平台应回落 per_task: %s", got)
	}
}

// TestBatchRoundTrip 整批一次往返：3 个任务一次请求带回，按任务 ID 映射终态/用量/产物。
func TestBatchRoundTrip(t *testing.T) {
	cleanup, err := jsplugin.PublishForTest(batchPluginSource)
	if err != nil {
		t.Fatalf("发布批量插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	var requestCount int32
	var lastBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/batch/fetch", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		_ = json.NewDecoder(r.Body).Decode(&lastBody)
		_, _ = w.Write([]byte(`{"list":[
			{"task_id":"t1","status":"success","clips":2,"url":"https://up.example.com/1.mp3"},
			{"task_id":"t2","status":"processing"},
			{"task_id":"t3","status":"failed","error":"boom"}
		]}`))
	})
	vendor := httptest.NewServer(mux)
	t.Cleanup(vendor.Close)

	a := &Adaptor{}
	refs := []common.BatchTaskRef{
		{Platform: "js:batch-demo", TaskID: "t1"},
		{Platform: "js:batch-demo", TaskID: "t2"},
		{Platform: "js:batch-demo", TaskID: "t3"},
	}
	resp, err := a.FetchBatchTasks(context.Background(), vendor.URL, "sk-test", refs)
	if err != nil {
		t.Fatalf("FetchBatchTasks 失败: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	infos, err := a.ParseBatchTasks(body)
	if err != nil {
		t.Fatalf("ParseBatchTasks 失败: %v", err)
	}

	// 上游一次往返承运 3 个任务
	if requestCount != 1 {
		t.Fatalf("批量应一次请求，实际 %d 次", requestCount)
	}
	if ids, _ := lastBody["ids"].([]any); len(ids) != 3 {
		t.Fatalf("请求应携带 3 个任务 ID: %v", lastBody)
	}

	// 按任务 ID 的映射与终态
	if info := infos["t1"]; info == nil || info.Status != common.TaskStatusSuccess ||
		info.MaterialUsage == nil || info.MaterialUsage.Facts["clips"] != 2 ||
		len(info.Artifacts) != 1 || info.Artifacts[0].SourcePath != "url" {
		t.Fatalf("t1 映射异常: %+v", info)
	}
	if info := infos["t2"]; info == nil || info.Status != common.TaskStatusInProgress {
		t.Fatalf("t2 应进行中: %+v", info)
	}
	if info := infos["t3"]; info == nil || info.Status != common.TaskStatusFailure || info.FailReason != "boom" {
		t.Fatalf("t3 应失败: %+v", info)
	}
}

// TestBatchUnreportedTask 上游部分返回：未回报的任务不出现在 map（轮询循环保持现状）。
func TestBatchUnreportedTask(t *testing.T) {
	cleanup, err := jsplugin.PublishForTest(batchPluginSource)
	if err != nil {
		t.Fatalf("发布批量插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/batch/fetch", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"list":[{"task_id":"t1","status":"processing"}]}`)) // 只回报 t1
	})
	vendor := httptest.NewServer(mux)
	t.Cleanup(vendor.Close)

	a := &Adaptor{}
	resp, err := a.FetchBatchTasks(context.Background(), vendor.URL, "sk-test", []common.BatchTaskRef{
		{Platform: "js:batch-demo", TaskID: "t1"},
		{Platform: "js:batch-demo", TaskID: "t9"},
	})
	if err != nil {
		t.Fatalf("FetchBatchTasks 失败: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	infos, err := a.ParseBatchTasks(body)
	if err != nil {
		t.Fatalf("ParseBatchTasks 失败: %v", err)
	}
	if _, ok := infos["t9"]; ok {
		t.Fatal("未回报的任务不应出现在结果 map")
	}
	if _, ok := infos["t1"]; !ok {
		t.Fatal("已回报任务缺失")
	}
}
