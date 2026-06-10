package tenant

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/utility/export"
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

// ExportTasks 导出异步任务日志
// owner/admin 可导出租户所有任务，member 只能导出自己的任务
func (s *sTenant) ExportTasks(ctx context.Context, req *v1.TenantTaskExportReq) (*v1.TenantTaskExportRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	statusLabel := map[string]string{
		"NOT_START":   "未开始",
		"SUBMITTED":   "已提交",
		"IN_PROGRESS": "进行中",
		"SUCCESS":     "成功",
		"FAILURE":     "失败",
		"TIMEOUT":     "超时",
	}

	platformLabel := map[string]string{
		"sora":       "Sora",
		"kling":      "Kling",
		"midjourney": "Midjourney",
		"suno":       "Suno",
	}

	columns := []export.Column{
		{Field: "public_task_id", Header: "任务ID"},
		{Field: "platform_name", Header: "平台"},
		{Field: "status_name", Header: "状态"},
		{Field: "action", Header: "动作"},
		{Field: "model_name", Header: "模型"},
		{Field: "progress", Header: "进度"},
		{Field: "username", Header: "用户"},
		{Field: "pre_deduct_amount", Header: "预扣金额(USD)"},
		{Field: "actual_cost", Header: "实际费用(USD)"},
		{Field: "billing_settled", Header: "是否已结算"},
		{Field: "fail_reason", Header: "失败原因"},
		{Field: "submit_time", Header: "提交时间"},
		{Field: "finish_time", Header: "完成时间"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "任务日志_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "t.tenant_id = ?")
	args = append(args, tenantID)

	// member 只能导出自己的任务
	if role == "member" {
		conditions = append(conditions, "t.user_id = ?")
		args = append(args, userID)
	}
	if req.Status != "" {
		conditions = append(conditions, "t.status = ?")
		args = append(args, req.Status)
	}
	if req.Platform != "" {
		conditions = append(conditions, "t.platform = ?")
		args = append(args, req.Platform)
	}
	if req.PublicTaskID != "" {
		conditions = append(conditions, "t.public_task_id = ?")
		args = append(args, req.PublicTaskID)
	}

	where := ""
	for i, c := range conditions {
		if i > 0 {
			where += " AND "
		}
		where += c
	}

	fromClause := "tsk_model_tasks t LEFT JOIN tnt_users u ON t.user_id = u.id AND t.tenant_id = u.tenant_id"

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			dataSQL := fmt.Sprintf(
				`SELECT t.public_task_id, t.platform, t.status, t.action, t.model_name, t.progress,
				        COALESCE(u.username, '') AS username,
				        t.pre_deduct_amount, t.actual_cost, t.billing_settled,
				        COALESCE(t.fail_reason, '') AS fail_reason,
				        t.submit_time, t.finish_time, t.created_at
				 FROM %s WHERE %s ORDER BY t.id DESC LIMIT 1000 OFFSET ?`,
				fromClause, where,
			)
			exportArgs := make([]any, len(args)+1)
			copy(exportArgs, args)
			exportArgs[len(args)] = offset
			result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, exportArgs...)
			if err != nil {
				return
			}
			for _, row := range result {
				m := make(map[string]any, len(row))
				for k, v := range row {
					switch raw := v.Val().(type) {
					case []byte:
						s := string(raw)
						if f, err := strconv.ParseFloat(s, 64); err == nil {
							m[k] = f
						} else {
							m[k] = s
						}
					default:
						m[k] = raw
					}
				}
				// 翻译平台和状态为中文
				if platform, ok := m["platform"].(string); ok {
					if label, exists := platformLabel[platform]; exists {
						m["platform_name"] = label
					} else {
						m["platform_name"] = platform
					}
				}
				if status, ok := m["status"].(string); ok {
					if label, exists := statusLabel[status]; exists {
						m["status_name"] = label
					} else {
						m["status_name"] = status
					}
				}
				if !yield(m) {
					return
				}
			}
			if len(result) < 1000 {
				break
			}
			offset += 1000
		}
	})
}
