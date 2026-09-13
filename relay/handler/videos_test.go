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

// stubVideosTaskProvider 查询/删除路径的 TaskDataProvider 桩：
// 嵌入接口零值，未覆盖的方法被调用时 panic（测试应只触达覆盖的方法）。
type stubVideosTaskProvider struct {
	common.TaskDataProvider

	task        *common.AsyncTask
	fetchErr    error
	deleteCalls int
	deleteErr   error
}

func (s *stubVideosTaskProvider) GetTaskByPublicIDAndUser(_ context.Context, publicTaskID string, _ int64, _ int64) (*common.AsyncTask, error) {
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	if s.task != nil && s.task.PublicTaskID == publicTaskID {
		return s.task, nil
	}
	return nil, nil
}

func (s *stubVideosTaskProvider) SoftDeleteTask(_ context.Context, task *common.AsyncTask) error {
	s.deleteCalls++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	task.Deleted = true
	return nil
}

func newVideosTestTask() *common.AsyncTask {
	finish := time.Unix(1760000432, 0)
	privateData, _ := json.Marshal(map[string]any{
		"upstream_task_id": "up-1",
		"task_type":        "kling",
		"request_echo":     map[string]any{"prompt": "a cat", "seconds": "8", "size": "1280x720"},
	})
	return &common.AsyncTask{
		ID:             1,
		PublicTaskID:   "video_abc",
		Status:         "SUCCESS",
		Progress:       "100%",
		ModelName:      "kling-v2-master",
		ResultURL:      "https://cdn.example.com/v.mp4",
		BillingSettled: true,
		FinishTime:     &finish,
		CreatedAt:      time.Unix(1760000000, 0),
		PrivateData:    privateData,
		TenantID:       7,
		UserID:         9,
	}
}

func videosTestRC(w http.ResponseWriter) *TaskRelayContext {
	return &TaskRelayContext{TenantID: 7, UserID: 9, Writer: w}
}

func decodeVideosJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode json: %v, body=%s", err, rec.Body.String())
	}
	return m
}

func TestVideosStatusFromTask(t *testing.T) {
	cases := map[string]string{
		"NOT_START":   "queued",
		"SUBMITTED":   "queued",
		"QUEUED":      "queued",
		"IN_PROGRESS": "in_progress",
		"SUCCESS":     "completed",
		"FAILURE":     "failed",
		"WEIRD":       "queued",
	}
	for in, want := range cases {
		if got := videosStatusFromTask(in); got != want {
			t.Errorf("videosStatusFromTask(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVideosProgressFromTask(t *testing.T) {
	makeTask := func(status, progress string) *common.AsyncTask {
		return &common.AsyncTask{Status: status, Progress: progress}
	}
	if got := videosProgressFromTask(makeTask("IN_PROGRESS", "42%")); got != 42 {
		t.Errorf("42%% → %d", got)
	}
	if got := videosProgressFromTask(makeTask("IN_PROGRESS", "150%")); got != 100 {
		t.Errorf("150%% should clamp to 100, got %d", got)
	}
	if got := videosProgressFromTask(makeTask("IN_PROGRESS", "")); got != 50 {
		t.Errorf("fallback in_progress → 50, got %d", got)
	}
	if got := videosProgressFromTask(makeTask("SUCCESS", "")); got != 100 {
		t.Errorf("fallback success → 100, got %d", got)
	}
}

func TestBuildVideoObject_SuccessWithEcho(t *testing.T) {
	obj := buildVideoObject(newVideosTestTask())
	if obj.ID != "video_abc" || obj.Object != "video" || obj.Status != "completed" || obj.Progress != 100 {
		t.Fatalf("unexpected object: %+v", obj)
	}
	if obj.Model != "kling-v2-master" || obj.Prompt == nil || *obj.Prompt != "a cat" {
		t.Fatalf("unexpected model/prompt: %+v", obj)
	}
	if obj.Seconds != "8" || obj.Size != "1280x720" {
		t.Fatalf("expected echo seconds/size, got %q/%q", obj.Seconds, obj.Size)
	}
	if obj.CompletedAt == nil || *obj.CompletedAt != 1760000432 {
		t.Fatalf("unexpected completed_at: %v", obj.CompletedAt)
	}
	if obj.Error != nil || obj.RemixedFromVideoID != nil || obj.ExpiresAt != nil {
		t.Fatalf("nullables must be nil: %+v", obj)
	}
}

func TestBuildVideoObject_Failure(t *testing.T) {
	task := newVideosTestTask()
	task.Status = "FAILURE"
	task.Progress = "30%"
	task.FailReason = "upstream exploded"
	obj := buildVideoObject(task)
	if obj.Status != "failed" || obj.Error == nil {
		t.Fatalf("expected failed with error: %+v", obj)
	}
	if obj.Error.Code != "video_generation_failed" || obj.Error.Message != "upstream exploded" {
		t.Fatalf("unexpected error object: %+v", obj.Error)
	}
}

func TestHandleVideosRetrieve_FoundAndPollHeader(t *testing.T) {
	task := newVideosTestTask()
	task.Status = "IN_PROGRESS"
	task.Progress = "42%"
	task.FinishTime = nil

	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: task}
	HandleVideosRetrieve(context.Background(), "video_abc", videosTestRC(rec), provider)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("openai-poll-after-ms"); got != videosPollAfterMs {
		t.Fatalf("expected poll header %q, got %q", videosPollAfterMs, got)
	}
	m := decodeVideosJSON(t, rec)
	if m["status"] != "in_progress" || m["progress"] != float64(42) || m["object"] != "video" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleVideosRetrieve_TerminalNoPollHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: newVideosTestTask()}
	HandleVideosRetrieve(context.Background(), "video_abc", videosTestRC(rec), provider)
	if got := rec.Header().Get("openai-poll-after-ms"); got != "" {
		t.Fatalf("terminal task must not carry poll header, got %q", got)
	}
}

func TestHandleVideosRetrieve_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{}
	HandleVideosRetrieve(context.Background(), "video_missing", videosTestRC(rec), provider)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleVideosDelete_HappyPath(t *testing.T) {
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: newVideosTestTask()}
	HandleVideosDelete(context.Background(), "video_abc", videosTestRC(rec), provider)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if provider.deleteCalls != 1 {
		t.Fatalf("expected 1 soft delete call, got %d", provider.deleteCalls)
	}
	m := decodeVideosJSON(t, rec)
	if m["object"] != "video.deleted" || m["deleted"] != true || m["id"] != "video_abc" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleVideosDelete_NonTerminalRejected(t *testing.T) {
	task := newVideosTestTask()
	task.Status = "IN_PROGRESS"
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: task}
	HandleVideosDelete(context.Background(), "video_abc", videosTestRC(rec), provider)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if provider.deleteCalls != 0 {
		t.Fatal("must not delete non-terminal task")
	}
	if !strings.Contains(rec.Body.String(), "video_not_deletable") {
		t.Fatalf("expected video_not_deletable code, body=%s", rec.Body.String())
	}
}

func TestHandleVideosDelete_UnsettledRejected(t *testing.T) {
	task := newVideosTestTask()
	task.BillingSettled = false
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: task}
	HandleVideosDelete(context.Background(), "video_abc", videosTestRC(rec), provider)

	if rec.Code != http.StatusBadRequest || provider.deleteCalls != 0 {
		t.Fatalf("expected 400 without delete, got %d calls=%d", rec.Code, provider.deleteCalls)
	}
	if !strings.Contains(rec.Body.String(), "billing_pending") {
		t.Fatalf("expected billing_pending code, body=%s", rec.Body.String())
	}
}

func TestHandleVideosDelete_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{}
	HandleVideosDelete(context.Background(), "video_gone", videosTestRC(rec), provider)
	if rec.Code != http.StatusNotFound || provider.deleteCalls != 0 {
		t.Fatalf("expected 404 without delete, got %d", rec.Code)
	}
}

func TestHandleVideosContent_StreamsBytes(t *testing.T) {
	content := []byte("FAKE-MP4-BYTES")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Write(content)
	}))
	defer upstream.Close()

	task := newVideosTestTask()
	task.ResultURL = upstream.URL

	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: task}
	HandleVideosContent(context.Background(), "video_abc", "video", videosTestRC(rec), provider)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Fatalf("expected video/mp4 passthrough, got %q", ct)
	}
	if rec.Body.String() != string(content) {
		t.Fatalf("body mismatch: %q", rec.Body.String())
	}
}

func TestHandleVideosContent_VariantUnsupported(t *testing.T) {
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: newVideosTestTask()}
	HandleVideosContent(context.Background(), "video_abc", "thumbnail", videosTestRC(rec), provider)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleVideosContent_NotCompleted(t *testing.T) {
	task := newVideosTestTask()
	task.Status = "IN_PROGRESS"
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: task}
	HandleVideosContent(context.Background(), "video_abc", "", videosTestRC(rec), provider)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "video_not_ready") {
		t.Fatalf("expected 400 video_not_ready, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleVideosContent_NoResultURL(t *testing.T) {
	task := newVideosTestTask()
	task.ResultURL = ""
	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: task}
	HandleVideosContent(context.Background(), "video_abc", "", videosTestRC(rec), provider)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "content_not_available") {
		t.Fatalf("expected 400 content_not_available, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleVideosContent_UpstreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	task := newVideosTestTask()
	task.ResultURL = upstream.URL

	rec := httptest.NewRecorder()
	provider := &stubVideosTaskProvider{task: task}
	HandleVideosContent(context.Background(), "video_abc", "", videosTestRC(rec), provider)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestWriteVideosSubmitResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	writeVideosSubmitResponse(rec, "video_new", "kling-v2-master", time.Unix(1760000000, 0), videosRequestEcho{Prompt: "p", Seconds: "8", Size: "720x1280"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	m := decodeVideosJSON(t, rec)
	if m["id"] != "video_new" || m["object"] != "video" || m["status"] != "queued" || m["progress"] != float64(0) {
		t.Fatalf("unexpected submit response: %s", rec.Body.String())
	}
	if m["seconds"] != "8" || m["size"] != "720x1280" {
		t.Fatalf("expected echo fields: %s", rec.Body.String())
	}
}
