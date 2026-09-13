package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
)

// MiniMax 官方协议查询/删除处理器单测（stub TaskDataProvider）。

// stubMiniMaxTaskProvider 返回预置任务并回查渠道
type stubMiniMaxTaskProvider struct {
	common.TaskDataProvider

	task    *common.AsyncTask
	channel *common.ChannelBasicInfo
}

func (s *stubMiniMaxTaskProvider) GetTaskByPublicIDAndUser(_ context.Context, publicTaskID string, _ int64, _ int64) (*common.AsyncTask, error) {
	if s.task != nil && s.task.PublicTaskID == publicTaskID {
		return s.task, nil
	}
	return nil, nil
}

func (s *stubMiniMaxTaskProvider) GetChannelByID(_ context.Context, _ int64) (*common.ChannelBasicInfo, error) {
	return s.channel, nil
}

// newMiniMaxTask 构造 minimax 平台任务记录（带上游任务 ID，供取消链路使用）
func newMiniMaxTask(status string) *common.AsyncTask {
	now := time.Now()
	return &common.AsyncTask{
		PublicTaskID: "task_mm_1",
		Platform:     "minimax",
		Status:       status,
		ModelName:    "MiniMax-H3",
		TenantID:     1,
		UserID:       2,
		ChannelID:    21,
		PrivateData:  json.RawMessage(`{"upstream_task_id":"424010985738629"}`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestMiniMaxStatusFromTask(t *testing.T) {
	cases := map[string]string{
		"NOT_START":   "queued",
		"SUBMITTED":   "queued",
		"QUEUED":      "queued",
		"IN_PROGRESS": "running",
		"SUCCESS":     "succeeded",
		"FAILURE":     "failed",
		"WHATEVER":    "queued",
	}
	for in, want := range cases {
		if got := minimaxStatusFromTask(in); got != want {
			t.Errorf("minimaxStatusFromTask(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandleMiniMaxVideoRetrieve_EnrichFromUpstreamData(t *testing.T) {
	task := newMiniMaxTask("SUCCESS")
	task.ResultURL = "https://video.cdn.example/out.mp4"
	task.Data = json.RawMessage(`{"task":{"id":"up-1","status":"succeeded","content":{"url":"https://video.cdn.example/out.mp4"},"usage":{"total_tokens":273890,"prompt_tokens":13500,"completion_tokens":260390},"resolution":"2K","duration":5,"ratio":"16:9"}}`)

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}
	HandleMiniMaxVideoRetrieve(context.Background(), "task_mm_1", rc, &stubMiniMaxTaskProvider{task: task})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	taskObj, _ := resp["task"].(map[string]any)
	if taskObj == nil {
		t.Fatalf("expected {\"task\": {...}} wrapper: %s", rec.Body.String())
	}
	// id 用网关公开 ID（不透出上游 ID）
	if taskObj["id"] != "task_mm_1" || taskObj["status"] != "succeeded" {
		t.Fatalf("task identity: %s", rec.Body.String())
	}
	content, _ := taskObj["content"].(map[string]any)
	if content["url"] != "https://video.cdn.example/out.mp4" {
		t.Fatalf("content.url: %v", taskObj["content"])
	}
	usage, _ := taskObj["usage"].(map[string]any)
	if usage["total_tokens"] != float64(273890) {
		t.Fatalf("usage replay: %v", taskObj["usage"])
	}
	if taskObj["resolution"] != "2K" || taskObj["duration"] != float64(5) || taskObj["ratio"] != "16:9" {
		t.Fatalf("enriched specs: %s", rec.Body.String())
	}
}

// TestHandleMiniMaxVideoRetrieve_SubmitShapedDataDegrade 提交后首拍轮询前 Data 是提交响应形态，
// 查询必须静默降级（缺失字段不输出，状态以任务记录为准）。
func TestHandleMiniMaxVideoRetrieve_SubmitShapedDataDegrade(t *testing.T) {
	task := newMiniMaxTask("SUBMITTED")
	task.Data = json.RawMessage(`{"task_id":"424010985738629"}`)

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}
	HandleMiniMaxVideoRetrieve(context.Background(), "task_mm_1", rc, &stubMiniMaxTaskProvider{task: task})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	taskObj, _ := resp["task"].(map[string]any)
	if taskObj["status"] != "queued" {
		t.Fatalf("status from record: %s", rec.Body.String())
	}
	if _, has := taskObj["usage"]; has {
		t.Fatalf("usage must be absent for submit-shaped data: %s", rec.Body.String())
	}
	if _, has := taskObj["resolution"]; has {
		t.Fatalf("resolution must be absent for submit-shaped data: %s", rec.Body.String())
	}
}

// TestHandleMiniMaxVideoRetrieve_PlatformGate 平台门禁：异构平台任务 ID 打 /v2 端点按 404 处理
func TestHandleMiniMaxVideoRetrieve_PlatformGate(t *testing.T) {
	task := newMiniMaxTask("SUCCESS")
	task.Platform = "kling"

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}
	HandleMiniMaxVideoRetrieve(context.Background(), "task_mm_1", rc, &stubMiniMaxTaskProvider{task: task})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-minimax platform, got %d", rec.Code)
	}
}

// TestHandleMiniMaxVideoRetrieve_FailedTask 失败任务回放官方 error 对象
func TestHandleMiniMaxVideoRetrieve_FailedTask(t *testing.T) {
	task := newMiniMaxTask("FAILURE")
	task.FailReason = "content policy violation"

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}
	HandleMiniMaxVideoRetrieve(context.Background(), "task_mm_1", rc, &stubMiniMaxTaskProvider{task: task})

	resp := decodeVideosJSON(t, rec)
	taskObj, _ := resp["task"].(map[string]any)
	if taskObj["status"] != "failed" {
		t.Fatalf("failed status: %s", rec.Body.String())
	}
	errObj, _ := taskObj["error"].(map[string]any)
	if errObj["message"] != "content policy violation" {
		t.Fatalf("error object: %s", rec.Body.String())
	}
}

// newMiniMaxCancelUpstream mock MiniMax v2 取消端点（DELETE /v2/video_generation/{id}）：
// 记录路径与方法，按脚本返回成功 / 上游拒绝 / 竞态删记录三种形态。
func newMiniMaxCancelUpstream(t *testing.T, script string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("cancel must use DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v2/video_generation/424010985738629" {
			t.Errorf("unexpected cancel path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch script {
		case "ok":
			_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "424010985738629", "action": "cancelled", "status": "cancelled"})
		case "running":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"type": "invalid_request_error", "message": "running task cannot be cancelled"}})
		case "deleted":
			_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "424010985738629", "action": "deleted", "status": "deleted"})
		}
	}))
}

func TestHandleMiniMaxVideoCancel(t *testing.T) {
	// 终态任务：不可取消、不可删除（不做软删），且不触达上游
	upstream := newMiniMaxCancelUpstream(t, "ok")
	defer upstream.Close()
	provider := &stubMiniMaxTaskProvider{
		task:    newMiniMaxTask("SUCCESS"),
		channel: &common.ChannelBasicInfo{ID: 21, BaseURL: upstream.URL, ApiKey: "sk-test"},
	}
	rec := httptest.NewRecorder()
	HandleMiniMaxVideoCancel(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, provider)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("terminal task must not be cancellable/deletable: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 运行中任务：前置拦截，不可取消
	provider.task = newMiniMaxTask("IN_PROGRESS")
	rec = httptest.NewRecorder()
	HandleMiniMaxVideoCancel(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, provider)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("running task must not be cancellable: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 排队中任务：上游确认取消 → 官方 cancelled 响应
	provider.task = newMiniMaxTask("QUEUED")
	rec = httptest.NewRecorder()
	HandleMiniMaxVideoCancel(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, provider)
	if rec.Code != http.StatusOK {
		t.Fatalf("queued cancel should succeed: code=%d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	if resp["task_id"] != "task_mm_1" || resp["action"] != "cancelled" || resp["status"] != "cancelled" {
		t.Fatalf("cancel response shape: %s", rec.Body.String())
	}
	// 本地状态不由取消端点改写（轮询循环对账收敛）
	if provider.task.Status != "QUEUED" {
		t.Fatalf("cancel endpoint must not mutate local status directly: %s", provider.task.Status)
	}
}

// TestHandleMiniMaxVideoCancel_UpstreamRejected 远程取消失败：上游判定运行中，
// 错误原样透传（本地状态滞后时由上游裁决兜底）。
func TestHandleMiniMaxVideoCancel_UpstreamRejected(t *testing.T) {
	upstream := newMiniMaxCancelUpstream(t, "running")
	defer upstream.Close()

	provider := &stubMiniMaxTaskProvider{
		task:    newMiniMaxTask("QUEUED"), // 本地状态滞后：本地仍是排队中
		channel: &common.ChannelBasicInfo{ID: 21, BaseURL: upstream.URL, ApiKey: "sk-test"},
	}
	rec := httptest.NewRecorder()
	HandleMiniMaxVideoCancel(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, provider)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upstream rejection must relay: code=%d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	errObj, _ := resp["error"].(map[string]any)
	if errObj["message"] != "running task cannot be cancelled" {
		t.Fatalf("upstream message must be relayed: %s", rec.Body.String())
	}
}

// TestHandleMiniMaxVideoCancel_UpstreamDeletedRace 竞态：本地检查后任务已在上游完成，
// 上游执行的是删记录（action=deleted）而非取消——不能向客户端谎报取消成功。
func TestHandleMiniMaxVideoCancel_UpstreamDeletedRace(t *testing.T) {
	upstream := newMiniMaxCancelUpstream(t, "deleted")
	defer upstream.Close()

	provider := &stubMiniMaxTaskProvider{
		task:    newMiniMaxTask("QUEUED"),
		channel: &common.ChannelBasicInfo{ID: 21, BaseURL: upstream.URL, ApiKey: "sk-test"},
	}
	rec := httptest.NewRecorder()
	HandleMiniMaxVideoCancel(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, provider)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("deleted action must not be reported as cancelled: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleMiniMaxVideoCancel_V1Rejected v1（Hailuo 系列）任务：上游无取消端点，直接拒绝
func TestHandleMiniMaxVideoCancel_V1Rejected(t *testing.T) {
	upstream := newMiniMaxCancelUpstream(t, "ok") // 上游即使可达也不应被调用
	defer upstream.Close()

	task := newMiniMaxTask("QUEUED")
	task.ModelName = "MiniMax-Hailuo-2.3" // UpstreamModel 为空时回落 ModelName 分派
	provider := &stubMiniMaxTaskProvider{
		task:    task,
		channel: &common.ChannelBasicInfo{ID: 21, BaseURL: upstream.URL, ApiKey: "sk-test"},
	}
	rec := httptest.NewRecorder()
	HandleMiniMaxVideoCancel(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, provider)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("v1 cancel must be rejected: code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "v1 API has no cancel endpoint") {
		t.Fatalf("reject message: %s", rec.Body.String())
	}
}

// ==================== v1（Hailuo 系列）官方协议 ====================

func TestMiniMaxV1StatusFromTask(t *testing.T) {
	cases := map[string]string{
		"NOT_START":   "Preparing",
		"SUBMITTED":   "Preparing",
		"QUEUED":      "Queueing",
		"IN_PROGRESS": "Processing",
		"SUCCESS":     "Success",
		"FAILURE":     "Fail",
	}
	for in, want := range cases {
		if got := minimaxV1StatusFromTask(in); got != want {
			t.Errorf("minimaxV1StatusFromTask(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAndValidateMiniMaxVideoV1CreateRequest(t *testing.T) {
	// 图生视频形态（prompt 可选）
	req, taskErr := ParseMiniMaxVideoV1CreateRequest([]byte(`{
		"model": "MiniMax-Hailuo-2.3",
		"first_frame_image": "https://cdn.example/f.png",
		"prompt": "让画面动起来",
		"duration": "10",
		"resolution": "768P",
		"prompt_optimizer": false,
		"callback_url": "https://example.com/cb"
	}`))
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	if req.Model != "MiniMax-Hailuo-2.3" || req.FirstFrameImage != "https://cdn.example/f.png" || req.Duration != 10 || req.Resolution != "768P" {
		t.Fatalf("parsed fields: %+v", req)
	}
	if req.PromptOptimizer == nil || *req.PromptOptimizer {
		t.Fatalf("prompt_optimizer: %v", req.PromptOptimizer)
	}
	if taskErr := ValidateMiniMaxVideoV1CreateRequest(req); taskErr != nil {
		t.Fatalf("i2v form should pass: %+v", taskErr)
	}

	// 必填校验：model 必填；prompt/first_frame_image/subject_reference 至少其一
	noModel, _ := ParseMiniMaxVideoV1CreateRequest([]byte(`{"prompt":"hi"}`))
	if taskErr := ValidateMiniMaxVideoV1CreateRequest(noModel); taskErr == nil {
		t.Fatal("expected model required")
	}
	empty, _ := ParseMiniMaxVideoV1CreateRequest([]byte(`{"model":"MiniMax-Hailuo-2.3","resolution":"768P"}`))
	if taskErr := ValidateMiniMaxVideoV1CreateRequest(empty); taskErr == nil {
		t.Fatal("expected one-of-prompt/image/subject required")
	}
}

func TestBuildMiniMaxVideoV1TaskBody(t *testing.T) {
	req := &minimaxVideoV1CreateRequest{
		Model:           "MiniMax-Hailuo-2.3",
		Prompt:          "海浪",
		FirstFrameImage: "https://cdn.example/f.png",
		Duration:        6,
		Resolution:      "768P",
	}
	body, err := BuildMiniMaxVideoV1TaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// v1 扁平字段保留
	if m["model"] != "MiniMax-Hailuo-2.3" || m["prompt"] != "海浪" || m["first_frame_image"] != "https://cdn.example/f.png" {
		t.Fatalf("flat fields: %v", m)
	}
	if m["duration"] != float64(6) || m["resolution"] != "768P" {
		t.Fatalf("duration/resolution: %v/%v", m["duration"], m["resolution"])
	}
	if _, has := m["callback_url"]; has {
		t.Fatal("callback_url must never enter the task body")
	}
	// 通用词汇双写
	if m["seconds"] != "6" {
		t.Fatalf("seconds dual-write: %v", m["seconds"])
	}
	images, _ := m["images"].([]any)
	if len(images) != 1 || images[0] != "https://cdn.example/f.png" {
		t.Fatalf("images dual-write: %v", m["images"])
	}
	meta, _ := m["metadata"].(map[string]any)
	if meta["resolution"] != "768P" || meta["duration"] != float64(6) || meta["image"] != "https://cdn.example/f.png" {
		t.Fatalf("metadata dual-write: %v", m["metadata"])
	}
}

func TestHandleMiniMaxVideoV1Retrieve(t *testing.T) {
	// 成功任务：官方 QueryVideoGenerationTaskResp 形态 + Data 富集 + ResultURL 直链
	task := newMiniMaxTask("SUCCESS")
	task.ModelName = "MiniMax-Hailuo-2.3"
	task.ResultURL = "https://cdn.example/out.mp4"
	task.Data = json.RawMessage(`{"task_id":"v1-up-1","status":"Success","file_id":"9001","video_width":1280,"video_height":720,"download_url":"https://cdn.example/out.mp4","base_resp":{"status_code":0}}`)

	rec := httptest.NewRecorder()
	HandleMiniMaxVideoV1Retrieve(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, &stubMiniMaxTaskProvider{task: task})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	if resp["task_id"] != "task_mm_1" || resp["status"] != "Success" {
		t.Fatalf("official shape: %s", rec.Body.String())
	}
	if resp["file_id"] != "9001" || resp["video_width"] != float64(1280) || resp["video_height"] != float64(720) {
		t.Fatalf("enriched fields: %s", rec.Body.String())
	}
	if resp["download_url"] != "https://cdn.example/out.mp4" {
		t.Fatalf("download_url: %s", rec.Body.String())
	}
	baseResp, _ := resp["base_resp"].(map[string]any)
	if baseResp["status_code"] != float64(0) {
		t.Fatalf("base_resp success envelope: %s", rec.Body.String())
	}

	// 失败任务：状态 Fail + 增补 error 字段；v2 形态 Data 不富集
	failTask := newMiniMaxTask("FAILURE")
	failTask.ModelName = "MiniMax-Hailuo-2.3"
	failTask.FailReason = "content violates policy"
	failTask.Data = json.RawMessage(`{"task":{"id":"up-1","status":"failed"}}`) // v2 形态：跳过富集
	rec = httptest.NewRecorder()
	HandleMiniMaxVideoV1Retrieve(context.Background(), "task_mm_1", &TaskRelayContext{TenantID: 1, UserID: 2, Writer: rec}, &stubMiniMaxTaskProvider{task: failTask})

	resp = decodeVideosJSON(t, rec)
	if resp["status"] != "Fail" || resp["error"] != "content violates policy" {
		t.Fatalf("failed replay: %s", rec.Body.String())
	}
	if _, has := resp["file_id"]; has {
		t.Fatalf("v2-shaped data must not enrich v1 response: %s", rec.Body.String())
	}
}
