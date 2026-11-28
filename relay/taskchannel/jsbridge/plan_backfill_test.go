package jsbridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// planBackfillSource validateRequest 修正请求体、parseSubmitResponse 回填 state 与
// immediate 的插件（技能文案推荐的三种做法同时使用）。
const planBackfillSource = `
export const meta = {
  apiVersion: 1, key: "backfill-demo", name: "Backfill Demo", version: "1.0.0",
  models: ["backfill_model"],
  routes: [{ method: "POST", path: "/submit", type: "submit" }, { method: "GET", path: "/q/:task_id", type: "query" }],
};
export function decodeRequest(ctx) { return { kind: "submit", model: ctx.body.model, requestBody: ctx.body }; }
export function validateRequest(ctx) { return { requestBody: Object.assign({}, ctx.body, { duration: 5 }) }; }
export function estimateUsage(ctx) { return { output_seconds: ctx.body.duration }; }
export function buildSubmitRequest(ctx) { return { url: "/submit", body: ctx.body }; }
export function parseSubmitResponse(ctx) {
  return {
    taskId: "t1",
    taskData: ctx.responseBody,
    state: { kind: "video" },
    immediate: { status: "SUCCESS", usage: { output_seconds: 5 } },
  };
}
export function buildQueryRequest(ctx) { return { url: "/q" }; }
export function parseQueryResponse(ctx) { return { items: [{ taskId: ctx.tasks[0].taskId, done: false }] }; }
`

// TestValidateRequestFixKeepsBackfillOnPlan validateRequest 修正请求体后，
// parseSubmitResponse 回填的 state / immediate 必须落在分发器与任务管线读取的那份
// 计划上（ctx 里的原计划）。此前修正时复制计划，回填落在副本上被静默丢弃：
// 轮询时插件拿到的 state 为空、受理即终态被忽略。
func TestValidateRequestFixKeepsBackfillOnPlan(t *testing.T) {
	cleanup, err := jsplugin.PublishForTest(planBackfillSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := jsplugin.CurrentGeneration().Plugin("backfill-demo")

	mux := http.NewServeMux()
	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["duration"] != float64(5) {
			// 下游必须看到修正值（就地替换仍然生效）
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"id":"t1"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	plan := &jsplugin.SubmitPlan{
		Plugin:  plugin,
		Decoded: &contract.DecodeResult{Kind: "submit", Model: "platform-alias", RequestBody: json.RawMessage(`{"model":"platform-alias","duration":12}`)},
	}
	ctx := jsplugin.WithSubmitPlan(context.Background(), plan)
	info := relayInfo(srv.URL)
	info.OriginModelName = "platform-alias"
	info.ChannelMeta.UpstreamModelName = "backfill_model"
	info.ChannelMeta.Settings.PluginKey = plugin.Key
	adaptor := &Adaptor{}
	adaptor.Init(info)

	if taskErr := adaptor.ValidateRequest(ctx, info, nil); taskErr != nil {
		t.Fatalf("ValidateRequest 失败: %s", taskErr.Message)
	}
	body, err := adaptor.BuildRequestBody(ctx, info, nil)
	if err != nil {
		t.Fatalf("BuildRequestBody 失败: %v", err)
	}
	resp, err := adaptor.DoRequest(ctx, info, body)
	if err != nil {
		t.Fatalf("DoRequest 失败: %v", err)
	}
	taskID, _, taskErr := adaptor.DoResponse(ctx, resp, info)
	if taskErr != nil {
		t.Fatalf("DoResponse 失败: %s", taskErr.Message)
	}
	if taskID != "t1" {
		t.Fatalf("上游任务 ID 异常: %s", taskID)
	}

	seen := jsplugin.SubmitPlanFromContext(ctx)
	if seen != adaptor.plan {
		t.Fatal("桥与分发器/任务管线必须持有同一份解码计划（修正请求体不得复制计划）")
	}
	var state map[string]any
	if len(seen.State) == 0 || json.Unmarshal(seen.State, &state) != nil || state["kind"] != "video" {
		t.Fatalf("任务管线读取的计划应带回 state: %q", string(seen.State))
	}
	if seen.Immediate == nil || seen.Immediate.Status != "SUCCESS" {
		t.Fatalf("分发器读取的计划应带回 immediate: %+v", seen.Immediate)
	}
	u, err := jsplugin.ParseHostUsage(seen.Immediate.Usage)
	if err != nil || u == nil {
		t.Fatalf("immediate.usage 非法: %s err=%v", seen.Immediate.Usage, err)
	}
	if secs, _ := u.Number(contract.UsageOutputSeconds); secs != 5 {
		t.Fatalf("immediate.usage 时长异常: %+v", u)
	}
}
