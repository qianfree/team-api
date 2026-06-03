package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// TenantTaskList 租户异步任务列表
// owner/admin 可查看租户所有任务，member 只能查看自己的任务
func (s *sTenant) TenantTaskList(ctx context.Context, req *v1.TenantTaskListReq) (*v1.TenantTaskListRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	role := middleware.GetUserRole(ctx)
	userID := middleware.GetUserID(ctx)

	m := dao.TskModelTasks.Ctx(ctx).
		LeftJoin("tnt_users u", "u.id = tsk_model_tasks.user_id AND u.tenant_id = tsk_model_tasks.tenant_id").
		Fields("tsk_model_tasks.*, COALESCE(u.username, '') as username").
		Where("tsk_model_tasks.tenant_id", tenantID).
		OrderDesc("tsk_model_tasks.id")

	// member 只能看自己的任务
	if role == "member" {
		m = m.Where("tsk_model_tasks.user_id", userID)
	}

	if req.Status != "" {
		m = m.Where("tsk_model_tasks.status", req.Status)
	}
	if req.Platform != "" {
		m = m.Where("tsk_model_tasks.platform", req.Platform)
	}
	if req.PublicTaskID != "" {
		m = m.Where("tsk_model_tasks.public_task_id", req.PublicTaskID)
	}

	var tasks []struct {
		Id              int64   `json:"id"`
		PublicTaskId    string  `json:"public_task_id"`
		Platform        string  `json:"platform"`
		Action          string  `json:"action"`
		Status          string  `json:"status"`
		Progress        string  `json:"progress"`
		ModelName       string  `json:"model_name"`
		FailReason      string  `json:"fail_reason"`
		PreDeductAmount float64 `json:"pre_deduct_amount"`
		ActualCost      float64 `json:"actual_cost"`
		BillingSettled  bool    `json:"billing_settled"`
		ResultUrl       string  `json:"result_url"`
		Username        string  `json:"username"`
		SubmitTime      *string `json:"submit_time"`
		FinishTime      *string `json:"finish_time"`
		CreatedAt       *string `json:"created_at"`
	}

	var total int
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)
	err := m.Page(page, pageSize).ScanAndCount(&tasks, &total, false)
	if err != nil {
		return nil, err
	}

	list := make([]v1.TenantTaskItem, 0, len(tasks))
	for _, t := range tasks {
		item := v1.TenantTaskItem{
			ID:              t.Id,
			PublicTaskID:    t.PublicTaskId,
			Platform:        t.Platform,
			Action:          t.Action,
			Status:          t.Status,
			Progress:        t.Progress,
			ModelName:       t.ModelName,
			FailReason:      t.FailReason,
			PreDeductAmount: t.PreDeductAmount,
			ActualCost:      t.ActualCost,
			BillingSettled:  t.BillingSettled,
			ResultURL:       t.ResultUrl,
			Username:        t.Username,
		}
		if t.SubmitTime != nil {
			item.SubmitTime = *t.SubmitTime
		}
		if t.FinishTime != nil {
			item.FinishTime = *t.FinishTime
		}
		if t.CreatedAt != nil {
			item.CreatedAt = *t.CreatedAt
		}
		list = append(list, item)
	}

	return &v1.TenantTaskListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// TenantTaskDetail 租户异步任务详情
// owner/admin 可查看租户所有任务，member 只能查看自己的任务
func (s *sTenant) TenantTaskDetail(ctx context.Context, req *v1.TenantTaskDetailReq) (*v1.TenantTaskDetailRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	role := middleware.GetUserRole(ctx)
	userID := middleware.GetUserID(ctx)

	var task *struct {
		Id              int64   `json:"id"`
		PublicTaskId    string  `json:"public_task_id"`
		Platform        string  `json:"platform"`
		Action          string  `json:"action"`
		Status          string  `json:"status"`
		Progress        string  `json:"progress"`
		ModelName       string  `json:"model_name"`
		FailReason      string  `json:"fail_reason"`
		PreDeductAmount float64 `json:"pre_deduct_amount"`
		ActualCost      float64 `json:"actual_cost"`
		BillingSettled  bool    `json:"billing_settled"`
		ResultUrl       string  `json:"result_url"`
		UserId          int64   `json:"user_id"`
		SubmitTime      *string `json:"submit_time"`
		FinishTime      *string `json:"finish_time"`
		CreatedAt       *string `json:"created_at"`
	}

	m := dao.TskModelTasks.Ctx(ctx).Where("id", req.ID).Where("tenant_id", tenantID)
	// member 只能看自己的任务
	if role == "member" {
		m = m.Where("user_id", userID)
	}

	err := m.Scan(&task)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if task == nil {
		return nil, common.NewNotFoundError("任务")
	}

	item := v1.TenantTaskItem{
		ID:              task.Id,
		PublicTaskID:    task.PublicTaskId,
		Platform:        task.Platform,
		Action:          task.Action,
		Status:          task.Status,
		Progress:        task.Progress,
		ModelName:       task.ModelName,
		FailReason:      task.FailReason,
		PreDeductAmount: task.PreDeductAmount,
		ActualCost:      task.ActualCost,
		BillingSettled:  task.BillingSettled,
		ResultURL:       task.ResultUrl,
	}
	if task.SubmitTime != nil {
		item.SubmitTime = *task.SubmitTime
	}
	if task.FinishTime != nil {
		item.FinishTime = *task.FinishTime
	}
	if task.CreatedAt != nil {
		item.CreatedAt = *task.CreatedAt
	}

	return &v1.TenantTaskDetailRes{
		Task: item,
	}, nil
}
