package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
)

// 查询端点 usage 摘要：SUCCESS 任务经 TaskUsageRecoverer 恢复后附加 usage 对象；
// 无恢复能力/无用量/非终态任务不出现该字段。

type stubUsageProvider struct {
	common.TaskDataProvider
	task       *common.AsyncTask
	usage      *common.TaskUsageSummary
	calledFlag bool
}

func (s *stubUsageProvider) GetTaskByPublicIDAndUser(_ context.Context, _ string, _ int64, _ int64) (*common.AsyncTask, error) {
	return s.task, nil
}

func (s *stubUsageProvider) RecoverTaskUsage(_ context.Context, _ *common.AsyncTask) *common.TaskUsageSummary {
	s.calledFlag = true
	return s.usage
}

// stubNoUsageProvider 未实现 TaskUsageRecoverer（等价于历史测试桩）
type stubNoUsageProvider struct {
	common.TaskDataProvider
	task *common.AsyncTask
}

func (s *stubNoUsageProvider) GetTaskByPublicIDAndUser(_ context.Context, _ string, _ int64, _ int64) (*common.AsyncTask, error) {
	return s.task, nil
}

func successUsageTask() *common.AsyncTask {
	now := time.Now()
	return &common.AsyncTask{
		PublicTaskID: "task_usage_1", Status: "SUCCESS", ModelName: "wan3.0-video",
		ResultURL: "https://example.com/v.mp4", FinishTime: &now, CreatedAt: now,
	}
}

func TestHandleTaskFetch_UsageAttached(t *testing.T) {
	task := successUsageTask()
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, RequestID: "req_u1", Writer: rec}
	p := &stubUsageProvider{task: task, usage: &common.TaskUsageSummary{
		OutputVideoSeconds: common.UsageF(5),
		Resolution:         "720P",
		CompletionTokens:   common.UsageI(38000),
		TotalTokens:        common.UsageI(38000),
		PromptTokens:       common.UsageI(0),
	}}

	HandleTaskFetch(context.Background(), task.PublicTaskID, rc, p)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Usage *common.TaskUsageSummary `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Usage == nil {
		t.Fatalf("usage 应出现在响应: %s", rec.Body.String())
	}
	if resp.Usage.OutputVideoSeconds == nil || *resp.Usage.OutputVideoSeconds != 5 || resp.Usage.Resolution != "720P" {
		t.Fatalf("usage 内容: %+v", resp.Usage)
	}
	// 火山口径 prompt_tokens=0 透出；未上报的素材字段省略
	if resp.Usage.PromptTokens == nil || *resp.Usage.PromptTokens != 0 {
		t.Fatalf("prompt_tokens=0 应透出: %+v", resp.Usage)
	}
	if resp.Usage.InputVideoSeconds != nil || resp.Usage.InputImageCount != nil {
		t.Fatalf("未上报字段应省略: %+v", resp.Usage)
	}
}

func TestHandleTaskFetch_UsageOmitted(t *testing.T) {
	// 恢复出 nil（渠道无用量）
	task := successUsageTask()
	rec := httptest.NewRecorder()
	HandleTaskFetch(context.Background(), task.PublicTaskID,
		&TaskRelayContext{TenantID: 1, UserID: 2, RequestID: "r", Writer: rec},
		&stubUsageProvider{task: task})
	var resp map[string]json.RawMessage
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, ok := resp["usage"]; ok {
		t.Fatalf("无用量不应有 usage 字段: %s", rec.Body.String())
	}

	// 非终态任务不触发恢复
	pending := successUsageTask()
	pending.Status = "IN_PROGRESS"
	rec2 := httptest.NewRecorder()
	p := &stubUsageProvider{task: pending}
	HandleTaskFetch(context.Background(), pending.PublicTaskID,
		&TaskRelayContext{TenantID: 1, UserID: 2, RequestID: "r", Writer: rec2}, p)
	if p.calledFlag {
		t.Fatal("非终态任务不应调用用量恢复")
	}

	// provider 未实现恢复能力（测试桩）：静默省略不 panic
	rec3 := httptest.NewRecorder()
	HandleTaskFetch(context.Background(), task.PublicTaskID,
		&TaskRelayContext{TenantID: 1, UserID: 2, RequestID: "r", Writer: rec3},
		&stubNoUsageProvider{task: task})
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec3.Code, rec3.Body.String())
	}
	var resp3 map[string]json.RawMessage
	_ = json.Unmarshal(rec3.Body.Bytes(), &resp3)
	if _, ok := resp3["usage"]; ok {
		t.Fatalf("未实现恢复能力不应有 usage 字段: %s", rec3.Body.String())
	}
}
