package task

import (
	"testing"
	"time"

	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/model/entity"
)

func TestAsyncTaskFromEntity(t *testing.T) {
	createdAt := time.Date(2026, 7, 26, 10, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	row := &entity.TskModelTasks{
		Id:              1,
		PublicTaskId:    "task_public",
		RequestId:       "req_public",
		Platform:        "sora",
		Action:          "generate",
		Status:          "IN_PROGRESS",
		Progress:        "50%",
		FailReason:      "temporary",
		TenantId:        2,
		UserId:          3,
		ApiKeyId:        4,
		ChannelId:       5,
		ModelName:       "requested-model",
		UpstreamModel:   "upstream-model",
		PreDeductAmount: decimal.RequireFromString("0.25"),
		ActualCost:      decimal.RequireFromString("0.10"),
		BillingSettled:  true,
		ResultUrl:       "https://example.com/result",
		Data:            `{"result":"ok"}`,
		PrivateData:     `{"upstream_task_id":"upstream-1"}`,
		SubmitTime:      gtime.NewFromTime(createdAt),
		StartTime:       gtime.NewFromTime(createdAt.Add(time.Second)),
		FinishTime:      gtime.NewFromTime(createdAt.Add(2 * time.Second)),
		CreatedAt:       gtime.NewFromTime(createdAt),
		UpdatedAt:       gtime.NewFromTime(updatedAt),
	}

	task := asyncTaskFromEntity(row)
	if task == nil {
		t.Fatal("asyncTaskFromEntity returned nil")
	}
	if task.ID != row.Id || task.PublicTaskID != row.PublicTaskId || task.RequestID != row.RequestId {
		t.Fatalf("identity fields not mapped: %+v", task)
	}
	if task.Action != row.Action || task.FailReason != row.FailReason || task.ResultURL != row.ResultUrl {
		t.Fatalf("task detail fields not mapped: %+v", task)
	}
	if !task.PreDeductAmount.Equal(row.PreDeductAmount) || !task.ActualCost.Equal(row.ActualCost) || !task.BillingSettled {
		t.Fatalf("billing fields not mapped: %+v", task)
	}
	if string(task.Data) != row.Data || string(task.PrivateData) != row.PrivateData {
		t.Fatalf("JSON fields not mapped: data=%s private_data=%s", task.Data, task.PrivateData)
	}
	if task.SubmitTime == nil || !task.SubmitTime.Equal(createdAt) || !task.CreatedAt.Equal(createdAt) || !task.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("time fields not mapped: %+v", task)
	}
}

func TestAsyncTaskFromEntityNil(t *testing.T) {
	if task := asyncTaskFromEntity(nil); task != nil {
		t.Fatalf("asyncTaskFromEntity(nil) = %+v, want nil", task)
	}
	if tasks := asyncTasksFromEntities([]*entity.TskModelTasks{nil}); len(tasks) != 0 {
		t.Fatalf("asyncTasksFromEntities retained nil rows: %+v", tasks)
	}
}
