package jsbridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/relay/common"
)

// fetchAuthSource 钩子内经 utils.fetch 相对路径调渠道接口的插件：提交链
// （parseSubmitResponse）与轮询链（parseQueryResponse，逐任务/批量共用）都探测
// 渠道 /me 看到的 Authorization，并经 state / taskData 带回。
const fetchAuthSource = `
export const meta = {
  apiVersion: 1, key: "fetchauth", name: "Fetch Auth", version: "1.0.0",
  models: ["fetchauth_model"],
  routes: [{ method: "POST", path: "/submit", type: "submit" }, { method: "GET", path: "/q/:task_id", type: "query" }],
};
export function decodeRequest(ctx) { return { kind: "submit", model: ctx.body.model, requestBody: ctx.body }; }
export function validateRequest(ctx) { return { requestBody: Object.assign({}, ctx.body, { fixed: true }) }; }
export function estimateUsage() { return { output_seconds: 1 }; }
export function buildSubmitRequest(ctx) { return { url: "/submit", body: ctx.body }; }
function probeAuth() {
  const response = utils.fetch({ url: "/me", headers: { Authorization: "Bearer plugin-supplied" } });
  return response.body.auth;
}
export function parseSubmitResponse(ctx) {
  return { taskId: "t1", taskData: ctx.responseBody, state: { auth: probeAuth() } };
}
export function buildQueryRequest(ctx) {
  return { url: "/q", method: "POST", body: { ids: ctx.tasks.map(function (t) { return t.taskId; }) } };
}
export function parseQueryResponse(ctx) {
  const auth = probeAuth();
  return { items: ctx.tasks.map(function (t) { return { taskId: t.taskId, done: true, taskData: { auth: auth } }; }) };
}
`

// newFetchAuthVendor 模拟渠道：/me 回显请求看到的 Authorization。
func newFetchAuthVendor(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/me":
			_ = json.NewEncoder(w).Encode(map[string]string{"auth": r.Header.Get("Authorization")})
		case "/submit":
			_, _ = io.WriteString(w, `{"id":"t1"}`)
		case "/q":
			_, _ = io.WriteString(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func publishFetchAuthPlugin(t *testing.T) *jsplugin.ActivePlugin {
	t.Helper()
	cleanup, err := jsplugin.PublishForTest(fetchAuthSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := jsplugin.CurrentGeneration().Plugin("fetchauth")
	return plugin
}

// TestFetchCarriesChannelAuth 提交链与轮询链（逐任务 / 批量）钩子内的 utils.fetch
// 同源请求由桥接层注入渠道密钥：此前 fetch 从不带鉴权，插件按文案写的"二跳取直链 /
// 查详情"上线即 401（hailuo v1 的 /v1/files/retrieve 即此形态）。
func TestFetchCarriesChannelAuth(t *testing.T) {
	plugin := publishFetchAuthPlugin(t)
	vendor := newFetchAuthVendor(t)
	const want = "Bearer sk-test" // relayInfo 的渠道密钥

	// 提交链：validateRequest 修正请求体 + parseSubmitResponse 回填 state（顺带覆盖
	// 修正后 state 不丢），state 里的 auth 取自钩子内 fetch
	plan := &jsplugin.SubmitPlan{
		Plugin:  plugin,
		Decoded: &contract.DecodeResult{Kind: "submit", Model: "platform-alias", RequestBody: json.RawMessage(`{"model":"platform-alias"}`)},
	}
	ctx := jsplugin.WithSubmitPlan(context.Background(), plan)
	info := relayInfo(vendor.URL)
	info.OriginModelName = "platform-alias"
	info.ChannelMeta.UpstreamModelName = "fetchauth_model"
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
	if _, _, taskErr := adaptor.DoResponse(ctx, resp, info); taskErr != nil {
		t.Fatalf("DoResponse 失败: %s", taskErr.Message)
	}
	var state struct {
		Auth string `json:"auth"`
	}
	if seen := jsplugin.SubmitPlanFromContext(ctx); len(seen.State) == 0 || json.Unmarshal(seen.State, &state) != nil {
		t.Fatalf("提交链应回填 state: %q", string(seen.State))
	}
	if state.Auth != want {
		t.Fatalf("提交链钩子内 fetch 的 Authorization = %q, want %q", state.Auth, want)
	}

	// 轮询链（逐任务）
	taskData, _ := json.Marshal(map[string]any{"task_id": "t1", "platform": "js:fetchauth", "model": "fetchauth_model"})
	poller := &Adaptor{}
	presp, err := poller.FetchTask(context.Background(), vendor.URL, "sk-test", taskData)
	if err != nil {
		t.Fatalf("FetchTask 失败: %v", err)
	}
	pb, _ := io.ReadAll(presp.Body)
	presp.Body.Close()
	result, err := poller.ParseTaskResult(pb)
	if err != nil {
		t.Fatalf("ParseTaskResult 失败: %v", err)
	}
	if !strings.Contains(string(result.Data), `"auth":"`+want+`"`) {
		t.Fatalf("逐任务轮询钩子内 fetch 的鉴权异常: %s", result.Data)
	}

	// 轮询链（批量）
	batcher := &Adaptor{}
	bresp, err := batcher.FetchBatchTasks(context.Background(), vendor.URL, "sk-test", []common.BatchTaskRef{
		{Platform: "js:fetchauth", TaskID: "t1", TaskData: json.RawMessage(`{}`), State: nil},
	})
	if err != nil {
		t.Fatalf("FetchBatchTasks 失败: %v", err)
	}
	bb, _ := io.ReadAll(bresp.Body)
	bresp.Body.Close()
	infos, err := batcher.ParseBatchTasks(bb)
	if err != nil {
		t.Fatalf("ParseBatchTasks 失败: %v", err)
	}
	if infos["t1"] == nil || !strings.Contains(string(infos["t1"].Data), `"auth":"`+want+`"`) {
		t.Fatalf("批量轮询钩子内 fetch 的鉴权异常: %+v", infos["t1"])
	}
}

// TestHailuoV1FileRetrieveAuthenticated 内置 hailuo 插件 v1 成功分支回归：utils.fetch
// 相对路径调 /v1/files/retrieve 换签名 download_url。上游要求 Bearer 时——fetch 注入
// 渠道密钥之前该请求不带鉴权（401 被插件 try/catch 吞掉），任务判成功却拿不到视频直链。
func TestHailuoV1FileRetrieveAuthenticated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 真实厂商口径：JSON 响应带 JSON Content-Type（utils.fetch 仅在响应头含 json 时
		// 把 body 解析为对象）
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/files/retrieve" || r.URL.Query().Get("file_id") != "file-9" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"base_resp":{"status_code":1004,"status_msg":"auth invalid"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"base_resp":{"status_code":0},"file":{"download_url":"https://vid.example/signed.mp4"}}`)
	}))
	t.Cleanup(srv.Close)

	src, err := os.ReadFile("../../../internal/jsplugin/builtin/hailuo/plugin.js")
	if err != nil {
		t.Fatalf("读取 hailuo 源码失败: %v", err)
	}
	cleanup, err := jsplugin.PublishForTest(string(src))
	if err != nil {
		t.Fatalf("发布 hailuo 失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, ok := jsplugin.CurrentGeneration().Plugin("hailuo")
	if !ok {
		t.Fatal("hailuo 插件未进 generation")
	}

	taskRef := contract.QueryTaskRef{TaskID: "hl-42", TaskData: json.RawMessage(`{"task_id":"hl-42","_isH3":false}`)}
	args, _ := json.Marshal(contract.ParseQueryContext{
		Status:       200,
		Tasks:        []contract.QueryTaskRef{taskRef},
		ResponseBody: json.RawMessage(`{"base_resp":{"status_code":0},"task_id":"hl-42","status":"Success","file_id":"file-9"}`),
	})
	// 轮询链同款上下文：渠道 baseUrl + 渠道密钥 + fetch 白名单
	ctx := channelHookCtx(context.Background(), srv.URL, "sk-test", []string{hostOf(srv.URL)})
	out, err := plugin.Pool.Call(ctx, "parseQueryResponse", args)
	if err != nil {
		t.Fatalf("parseQueryResponse 失败: %v", err)
	}
	raw, err := contract.ParseJSONObject(out)
	if err != nil {
		t.Fatalf("返回非对象: %v", err)
	}
	queried, err := contract.ValidateQueried(raw, []contract.QueryTaskRef{taskRef})
	if err != nil {
		t.Fatalf("未过契约: %v", err)
	}
	item := queried.Items[0]
	if !item.Done || item.Error != nil || len(item.Artifacts) != 1 || item.Artifacts[0].Source.Path != "_video_url" {
		t.Fatalf("上游要求 Bearer 时 v1 成功任务应拿到视频直链: done=%v error=%+v artifacts=%+v", item.Done, item.Error, item.Artifacts)
	}
	var td map[string]any
	if err := json.Unmarshal(item.TaskData, &td); err != nil || td["_video_url"] != "https://vid.example/signed.mp4" {
		t.Fatalf("taskData._video_url 异常: %v", err)
	}
}
