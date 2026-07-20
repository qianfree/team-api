package admin

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// TaskList 大模型异步任务列表
func (s *sAdmin) TaskList(ctx context.Context, req *v1.TaskListReq) (*v1.TaskListRes, error) {
	m := dao.TskModelTasks.Ctx(ctx).OrderDesc("id")
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}
	if req.Platform != "" {
		m = m.Where("platform", req.Platform)
	}
	if req.PublicTaskID != "" {
		m = m.Where("public_task_id", req.PublicTaskID)
	}

	var tasks []struct {
		Id              int64       `json:"id"`
		PublicTaskId    string      `json:"public_task_id"`
		Platform        string      `json:"platform"`
		Action          string      `json:"action"`
		Status          string      `json:"status"`
		Progress        string      `json:"progress"`
		ModelName       string      `json:"model_name"`
		FailReason      string      `json:"fail_reason"`
		PreDeductAmount float64     `json:"pre_deduct_amount"`
		ActualCost      float64     `json:"actual_cost"`
		BillingSettled  bool        `json:"billing_settled"`
		ResultUrl       string      `json:"result_url"`
		TenantId        int64       `json:"tenant_id"`
		UserId          int64       `json:"user_id"`
		SubmitTime      *gtime.Time `json:"submit_time"`
		FinishTime      *gtime.Time `json:"finish_time"`
		CreatedAt       *gtime.Time `json:"created_at"`
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var total int
	err := m.Page(page, pageSize).ScanAndCount(&tasks, &total, false)
	if err != nil {
		return nil, err
	}

	list := make([]v1.ModelTaskItem, 0, len(tasks))
	for _, t := range tasks {
		item := v1.ModelTaskItem{
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
			TenantID:        t.TenantId,
			UserID:          t.UserId,
			CreatedAt:       t.CreatedAt.Format("Y-m-d H:i:s"),
		}
		if t.SubmitTime != nil {
			item.SubmitTime = t.SubmitTime.Format("Y-m-d H:i:s")
		}
		if t.FinishTime != nil {
			item.FinishTime = t.FinishTime.Format("Y-m-d H:i:s")
		}
		list = append(list, item)
	}

	return &v1.TaskListRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// TaskDetail 大模型异步任务详情
func (s *sAdmin) TaskDetail(ctx context.Context, req *v1.TaskDetailReq) (*v1.TaskDetailRes, error) {
	var task *struct {
		Id              int64       `json:"id"`
		PublicTaskId    string      `json:"public_task_id"`
		Platform        string      `json:"platform"`
		Action          string      `json:"action"`
		Status          string      `json:"status"`
		Progress        string      `json:"progress"`
		ModelName       string      `json:"model_name"`
		UpstreamModel   string      `json:"upstream_model"`
		FailReason      string      `json:"fail_reason"`
		PreDeductAmount float64     `json:"pre_deduct_amount"`
		ActualCost      float64     `json:"actual_cost"`
		BillingSettled  bool        `json:"billing_settled"`
		ResultUrl       string      `json:"result_url"`
		TenantId        int64       `json:"tenant_id"`
		UserId          int64       `json:"user_id"`
		SubmitTime      *gtime.Time `json:"submit_time"`
		StartTime       *gtime.Time `json:"start_time"`
		FinishTime      *gtime.Time `json:"finish_time"`
		CreatedAt       *gtime.Time `json:"created_at"`
	}
	err := dao.TskModelTasks.Ctx(ctx).Where("id", req.ID).Scan(&task)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if task == nil {
		return nil, gerror.NewCode(gcode.New(consts.CodeNotFound, consts.MsgNotFound, nil), consts.MsgNotFound)
	}

	item := v1.ModelTaskItem{
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
		TenantID:        task.TenantId,
		UserID:          task.UserId,
		CreatedAt:       task.CreatedAt.Format("Y-m-d H:i:s"),
	}
	if task.SubmitTime != nil {
		item.SubmitTime = task.SubmitTime.Format("Y-m-d H:i:s")
	}
	if task.FinishTime != nil {
		item.FinishTime = task.FinishTime.Format("Y-m-d H:i:s")
	}

	// 对 re-host 到对象存储的结果图，生成新鲜的缩略图 URL 供详情弹窗内联预览，并刷新原图 URL
	// （规避 result_url 24h 预签名过期）。非图片/上游直链任务查不到文件记录，保持原 result_url。
	if thumb, orig := common.TaskResultImageURLs(ctx, task.TenantId, task.PublicTaskId, 600); thumb != "" {
		item.ResultThumbURL = thumb
		if orig != "" {
			item.ResultURL = orig
		}
	}

	return &v1.TaskDetailRes{
		Task: item,
	}, nil
}

// TaskCancel 取消大模型异步任务
func (s *sAdmin) TaskCancel(ctx context.Context, req *v1.TaskCancelReq) (*v1.TaskCancelRes, error) {
	result, err := dao.TskModelTasks.Ctx(ctx).
		Where("id", req.ID).
		WhereIn("status", []string{"NOT_START", "SUBMITTED", "IN_PROGRESS"}).
		Data(do.TskModelTasks{
			Status:     "FAILURE",
			FailReason: "管理员手动取消",
			FinishTime: gtime.Now(),
		}).Update()
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, common.NewBadRequestError("任务无法取消（可能已完成或已失败）")
	}
	return &v1.TaskCancelRes{}, nil
}

// MarkStuckTasksFailed 标记卡住的系统任务为失败（定时任务，操作 tsk_tasks 表）
func MarkStuckTasksFailed(ctx context.Context) error {
	threshold := gtime.Now().Add(-30 * time.Minute)
	_, err := dao.TskTasks.Ctx(ctx).
		Where("status", "running").
		Where("started_at < ?", threshold).
		Data(do.TskTasks{
			Status:       "failed",
			ErrorMessage: "任务执行超时（30分钟无响应）",
			FinishedAt:   gtime.Now(),
		}).Update()
	return err
}
