package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

func (s *sAdmin) UsageLogCleanupCreate(ctx context.Context, req *v1.UsageLogCleanupCreateReq) (*v1.UsageLogCleanupCreateRes, error) {
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return nil, err
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return nil, err
	}
	if startTime.After(endTime) {
		return nil, common.NewBadRequestError("起始时间不能晚于截止时间")
	}

	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 5000
	}

	payload := map[string]any{
		"start_time": startTime,
		"end_time":   endTime,
		"batch_size": batchSize,
		"dry_run":    req.DryRun,
	}
	if req.TenantID != nil {
		payload["tenant_id"] = *req.TenantID
	}
	if req.ModelName != "" {
		payload["model_name"] = req.ModelName
	}
	if req.Status != "" {
		payload["status"] = req.Status
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	now := gtime.Now()
	result, err := dao.TskTasks.Ctx(ctx).Data(do.TskTasks{
		Name:        "usage_log_cleanup",
		Handler:     "usage_log_cleanup",
		Payload:     payloadJSON,
		Status:      "pending",
		MaxRetries:  0,
		ScheduledAt: now,
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.UsageLogCleanupCreateRes{TaskID: id}, nil
}

func (s *sAdmin) UsageLogCleanupList(ctx context.Context, req *v1.UsageLogCleanupListReq) (*v1.UsageLogCleanupListRes, error) {
	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.TskTasks.Ctx(ctx).
		Where("handler", "usage_log_cleanup").
		OrderDesc("id")

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	var tasks []struct {
		Id           int64      `json:"id"`
		Name         string     `json:"name"`
		Status       string     `json:"status"`
		ErrorMessage string     `json:"error_message"`
		Result       string     `json:"result"`
		StartedAt    *time.Time `json:"started_at"`
		FinishedAt   *time.Time `json:"finished_at"`
		CreatedAt    *time.Time `json:"created_at"`
	}
	err = m.Page(page, pageSize).Scan(&tasks)
	if err != nil {
		return nil, err
	}

	items := make([]v1.UsageLogCleanupTaskItem, len(tasks))
	for i, t := range tasks {
		item := v1.UsageLogCleanupTaskItem{
			ID:           t.Id,
			Name:         t.Name,
			Status:       t.Status,
			ErrorMessage: t.ErrorMessage,
			Result:       t.Result,
		}
		if t.StartedAt != nil {
			item.StartedAt = t.StartedAt.Format(time.RFC3339)
		}
		if t.FinishedAt != nil {
			item.FinishedAt = t.FinishedAt.Format(time.RFC3339)
		}
		if t.CreatedAt != nil {
			item.CreatedAt = t.CreatedAt.Format(time.RFC3339)
		}
		items[i] = item
	}

	return &v1.UsageLogCleanupListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *sAdmin) UsageLogCleanupCancel(ctx context.Context, req *v1.UsageLogCleanupCancelReq) (*v1.UsageLogCleanupCancelRes, error) {
	count, err := dao.TskTasks.Ctx(ctx).Where("id", req.ID).Where("handler", "usage_log_cleanup").Count()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, common.NewNotFoundError("清理任务")
	}
	_, err = dao.TskTasks.Ctx(ctx).
		Where("id", req.ID).
		Where("handler", "usage_log_cleanup").
		Where("status", "pending").
		Data(do.TskTasks{Status: "cancelled"}).
		Update()
	if err != nil {
		return nil, err
	}
	return &v1.UsageLogCleanupCancelRes{}, nil
}
