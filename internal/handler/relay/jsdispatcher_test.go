// jsdispatcher_test.go /pjs 分发器链路单测：起真实 gf 服务器（不带鉴权中间件，
// 鉴权链由集成测试覆盖），覆盖路由匹配、查询链渲染与跨插件任务隔离。
// 提交链的资金出口（预扣/退款）在 relay/handler/task_handler_jsplugin_test.go 覆盖。
package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/jsplugin"
	relay_common "github.com/qianfree/team-api/relay/common"
)

// stubJSTaskProvider 分发器查询链的 TaskDataProvider 桩：按公开 ID 返回预置任务。
type stubJSTaskProvider struct {
	relay_common.TaskDataProvider
	tasks map[string]*relay_common.AsyncTask
}

func (s *stubJSTaskProvider) GetTaskByPublicIDAndUser(_ context.Context, publicTaskID string, _ int64, _ int64) (*relay_common.AsyncTask, error) {
	return s.tasks[publicTaskID], nil
}

// plainQueryPluginSource 无渲染钩子的最小插件：query 路由走默认透传渲染，
// dynamic 路由（POST /q + ids 列表）走批量查询默认渲染。路由形状与 golden 错开。
const plainQueryPluginSource = `export const meta = {
  apiVersion: 1, key: "plainq", name: "Plain Query", version: "1.0.0",
  models: ["plain_model"], fetchMode: "per_task",
  routes: [
    { method: "GET", path: "/q/:task_id", type: "query" },
    { method: "POST", path: "/q", type: "dynamic" },
  ],
};
export function decodeRequest(ctx) {
  return { kind: "query", taskIds: ((ctx.body || {}).ids) || [] };
}`

func goldenPluginSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../../internal/jsplugin/builtin/golden/plugin.js")
	if err != nil {
		t.Fatalf("读取 golden 插件失败: %v", err)
	}
	return string(data)
}

// setupJSDispatcher 发布 golden + plainq 插件并替换任务提供者，返回测试服务器基址。
// 服务器进程内单例（分发器读 CurrentGeneration/taskProvider 均为每请求动态取）。
func setupJSDispatcher(t *testing.T, tasks map[string]*relay_common.AsyncTask) string {
	t.Helper()
	cleanup, err := jsplugin.PublishForTest(goldenPluginSource(t), plainQueryPluginSource)
	if err != nil {
		t.Fatalf("发布测试插件失败: %v", err)
	}
	t.Cleanup(cleanup)

	old := taskDataProvider
	taskDataProvider = &stubJSTaskProvider{tasks: tasks}
	t.Cleanup(func() { taskDataProvider = old })

	jsDispatcherTestOnce.Do(func() {
		s := g.Server("jsdispatcher-test")
		s.SetDumpRouterMap(false)
		s.Group("/pjs", func(group *ghttp.RouterGroup) {
			group.ALL("/*path", dispatchJSPlugin)
		})
		s.SetAddr("127.0.0.1:0")
		if err := s.Start(); err != nil {
			t.Fatalf("启动 /pjs 测试服务器失败: %v", err)
		}
		jsDispatcherTestURL = fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())
	})
	return jsDispatcherTestURL
}

var (
	jsDispatcherTestOnce sync.Once
	jsDispatcherTestURL  string
)

// jsHTTPDo 发请求并解析 JSON 响应。
func jsHTTPDo(t *testing.T, method, url string, body []byte) (int, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("响应不是 JSON: %s", raw)
	}
	return resp.StatusCode, out
}

func jsTaskFixture(publicID, platform, status, progress string, data json.RawMessage) *relay_common.AsyncTask {
	return &relay_common.AsyncTask{
		PublicTaskID: publicID,
		Platform:     platform,
		Status:       status,
		Progress:     progress,
		ModelName:    "golden_music",
		Data:         data,
	}
}

// TestJSDispatch_UnknownRoute 未声明的路由形状 → 404。
func TestJSDispatch_UnknownRoute(t *testing.T) {
	base := setupJSDispatcher(t, nil)
	status, out := jsHTTPDo(t, http.MethodGet, base+"/pjs/golden/nowhere", nil)
	if status != http.StatusNotFound {
		t.Fatalf("期望 404，得到 %d", status)
	}
	if !jsonFieldContains(t, out, "error", "message", "js plugin route not found") {
		t.Fatalf("错误信息异常: %v", out)
	}
}

// TestJSDispatch_KeyMismatch URL key 与路由命中插件不一致 → 404（不泄露路由存在性）。
func TestJSDispatch_KeyMismatch(t *testing.T) {
	base := setupJSDispatcher(t, nil)
	// /submit/:action 形状命中 golden，但 URL key 是 notgolden
	status, _ := jsHTTPDo(t, http.MethodPost, base+"/pjs/notgolden/submit/music", nil)
	if status != http.StatusNotFound {
		t.Fatalf("期望 404，得到 %d", status)
	}
}

// TestJSDispatch_Query_RenderTask golden 导出 renderTask → 钩子输出替换默认视图。
func TestJSDispatch_Query_RenderTask(t *testing.T) {
	tasks := map[string]*relay_common.AsyncTask{
		"task_golden": jsTaskFixture("task_golden", "js:golden", "IN_PROGRESS", "50%",
			json.RawMessage(`{"data":{"audio_url":"https://oss.example.com/x.mp3"}}`)),
	}
	base := setupJSDispatcher(t, tasks)
	status, out := jsHTTPDo(t, http.MethodGet, base+"/pjs/golden/fetch/task_golden", nil)
	if status != http.StatusOK {
		t.Fatalf("期望 200，得到 %d body=%v", status, out)
	}
	if out["task_id"] != "task_golden" || out["audio_url"] != "https://oss.example.com/x.mp3" {
		t.Fatalf("renderTask 输出异常: %v", out)
	}
}

// TestJSDispatch_Query_DefaultView 插件未导出渲染钩子 → 默认透传任务关键信息。
func TestJSDispatch_Query_DefaultView(t *testing.T) {
	tasks := map[string]*relay_common.AsyncTask{
		"task_own": jsTaskFixture("task_own", "js:plainq", "IN_PROGRESS", "50%",
			json.RawMessage(`{"foo":1}`)),
	}
	base := setupJSDispatcher(t, tasks)
	status, out := jsHTTPDo(t, http.MethodGet, base+"/pjs/plainq/q/task_own", nil)
	if status != http.StatusOK {
		t.Fatalf("期望 200，得到 %d body=%v", status, out)
	}
	if out["task_id"] != "task_own" || out["status"] != "IN_PROGRESS" || out["progress"] != "50%" {
		t.Fatalf("默认视图异常: %v", out)
	}
	data, _ := out["data"].(map[string]any)
	if data == nil || data["foo"] != float64(1) {
		t.Fatalf("默认视图 data 异常: %v", out["data"])
	}
}

// TestJSDispatch_Query_CrossPluginIsolation 任务归属其他插件 → 按 404 处理，
// 不泄露任务存在性（跨插件任务互相不可见）。
func TestJSDispatch_Query_CrossPluginIsolation(t *testing.T) {
	tasks := map[string]*relay_common.AsyncTask{
		"task_foreign": jsTaskFixture("task_foreign", "js:plainq", "SUCCESS", "100%", nil),
	}
	base := setupJSDispatcher(t, tasks)
	// 任务属于 plainq，经 golden 的查询路由访问
	status, out := jsHTTPDo(t, http.MethodGet, base+"/pjs/golden/fetch/task_foreign", nil)
	if status != http.StatusNotFound {
		t.Fatalf("期望 404，得到 %d body=%v", status, out)
	}
	if !jsonFieldContains(t, out, "error", "message", "task not found") {
		t.Fatalf("错误信息异常: %v", out)
	}
}

// TestJSDispatch_Query_TaskNotFound 任务不存在 → 404。
func TestJSDispatch_Query_TaskNotFound(t *testing.T) {
	base := setupJSDispatcher(t, map[string]*relay_common.AsyncTask{})
	status, _ := jsHTTPDo(t, http.MethodGet, base+"/pjs/golden/fetch/task_missing", nil)
	if status != http.StatusNotFound {
		t.Fatalf("期望 404，得到 %d", status)
	}
}

// TestJSDispatch_DynamicQuery_FiltersForeignTasks dynamic 批量查询：ids 中的
// 他插件任务被过滤，仅本插件任务进入渲染列表（部分命中不整体失败）。
func TestJSDispatch_DynamicQuery_FiltersForeignTasks(t *testing.T) {
	tasks := map[string]*relay_common.AsyncTask{
		"task_own": jsTaskFixture("task_own", "js:plainq", "IN_PROGRESS", "50%",
			json.RawMessage(`{"foo":1}`)),
		"task_golden": jsTaskFixture("task_golden", "js:golden", "SUCCESS", "100%", nil),
	}
	base := setupJSDispatcher(t, tasks)
	body, _ := json.Marshal(map[string]any{"ids": []string{"task_own", "task_golden", "task_missing"}})
	status, out := jsHTTPDo(t, http.MethodPost, base+"/pjs/plainq/q", body)
	if status != http.StatusOK {
		t.Fatalf("期望 200，得到 %d body=%v", status, out)
	}
	list, _ := out["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("应只保留本插件任务（1 条），得到 %d: %v", len(list), out)
	}
	item, _ := list[0].(map[string]any)
	if item["task_id"] != "task_own" {
		t.Fatalf("列表条目异常: %v", list)
	}
}

// jsonFieldContains 断言 out[field][subField] 字符串包含 sub。
func jsonFieldContains(t *testing.T, out map[string]any, field, subField, sub string) bool {
	t.Helper()
	f, _ := out[field].(map[string]any)
	if f == nil {
		return false
	}
	msg, _ := f[subField].(string)
	return bytesContains(msg, sub)
}

func bytesContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOfStr(s, sub) >= 0)
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
