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
	do "github.com/qianfree/team-api/internal/model/do"
)

// TaskList 任务列表
func (s *sAdmin) TaskList(ctx context.Context, req *v1.TaskListReq) (*v1.TaskListRes, error) {
	m := dao.TskTasks.Ctx(ctx).OrderDesc("id")
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}
	if req.Handler != "" {
		m = m.Where("handler", req.Handler)
	}

	var tasks []struct {
		Id           int64       `json:"id"`
		Name         string      `json:"name"`
		Handler      string      `json:"handler"`
		Status       string      `json:"status"`
		MaxRetries   int         `json:"max_retries"`
		RetryCount   int         `json:"retry_count"`
		ErrorMessage string      `json:"error_message"`
		StartedAt    *gtime.Time `json:"started_at"`
		FinishedAt   *gtime.Time `json:"finished_at"`
		ScheduledAt  *gtime.Time `json:"scheduled_at"`
		CreatedAt    *gtime.Time `json:"created_at"`
	}

	var total int
	err := m.Page(req.Page, req.PageSize).ScanAndCount(&tasks, &total, false)
	if err != nil {
		return nil, err
	}

	list := make([]v1.TaskItem, 0, len(tasks))
	for _, t := range tasks {
		item := v1.TaskItem{
			ID:           t.Id,
			Name:         t.Name,
			Handler:      t.Handler,
			Status:       t.Status,
			MaxRetries:   t.MaxRetries,
			RetryCount:   t.RetryCount,
			ErrorMessage: t.ErrorMessage,
			CreatedAt:    t.CreatedAt.Format("Y-m-d H:i:s"),
		}
		if t.StartedAt != nil {
			item.StartedAt = t.StartedAt.Format("Y-m-d H:i:s")
		}
		if t.FinishedAt != nil {
			item.FinishedAt = t.FinishedAt.Format("Y-m-d H:i:s")
		}
		if t.ScheduledAt != nil {
			item.ScheduledAt = t.ScheduledAt.Format("Y-m-d H:i:s")
		}
		list = append(list, item)
	}

	return &v1.TaskListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// TaskDetail 任务详情
func (s *sAdmin) TaskDetail(ctx context.Context, req *v1.TaskDetailReq) (*v1.TaskDetailRes, error) {
	var task struct {
		Id           int64       `json:"id"`
		Name         string      `json:"name"`
		Handler      string      `json:"handler"`
		Status       string      `json:"status"`
		MaxRetries   int         `json:"max_retries"`
		RetryCount   int         `json:"retry_count"`
		ErrorMessage string      `json:"error_message"`
		StartedAt    *gtime.Time `json:"started_at"`
		FinishedAt   *gtime.Time `json:"finished_at"`
		ScheduledAt  *gtime.Time `json:"scheduled_at"`
		CreatedAt    *gtime.Time `json:"created_at"`
	}
	err := dao.TskTasks.Ctx(ctx).Where("id", req.ID).Scan(&task)
	if err != nil {
		return nil, err
	}
	if task.Id == 0 {
		return nil, gerror.NewCode(gcode.New(consts.CodeNotFound, consts.MsgNotFound, nil), consts.MsgNotFound)
	}

	taskItem := v1.TaskItem{
		ID:           task.Id,
		Name:         task.Name,
		Handler:      task.Handler,
		Status:       task.Status,
		MaxRetries:   task.MaxRetries,
		RetryCount:   task.RetryCount,
		ErrorMessage: task.ErrorMessage,
		CreatedAt:    task.CreatedAt.Format("Y-m-d H:i:s"),
	}
	if task.StartedAt != nil {
		taskItem.StartedAt = task.StartedAt.Format("Y-m-d H:i:s")
	}
	if task.FinishedAt != nil {
		taskItem.FinishedAt = task.FinishedAt.Format("Y-m-d H:i:s")
	}
	if task.ScheduledAt != nil {
		taskItem.ScheduledAt = task.ScheduledAt.Format("Y-m-d H:i:s")
	}

	// 查询最近日志
	var logs []struct {
		Level     string      `json:"level"`
		Message   string      `json:"message"`
		CreatedAt *gtime.Time `json:"created_at"`
	}
	_ = dao.TskTaskLogs.Ctx(ctx).
		Where("task_id", req.ID).
		OrderDesc("id").
		Limit(50).
		Scan(&logs)

	logList := make([]v1.TaskLogItem, 0, len(logs))
	for _, l := range logs {
		logList = append(logList, v1.TaskLogItem{
			Level:     l.Level,
			Message:   l.Message,
			CreatedAt: l.CreatedAt.Format("Y-m-d H:i:s"),
		})
	}

	return &v1.TaskDetailRes{
		Task:       taskItem,
		RecentLogs: logList,
	}, nil
}

// TaskCancel 取消任务
func (s *sAdmin) TaskCancel(ctx context.Context, req *v1.TaskCancelReq) (*v1.TaskCancelRes, error) {
	result, err := dao.TskTasks.Ctx(ctx).
		Where("id", req.ID).
		WhereIn("status", []string{"pending", "running"}).
		Data(do.TskTasks{
			Status:     "cancelled",
			FinishedAt: gtime.Now(),
		}).Update()
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, gerror.New("任务无法取消（可能已完成或已取消）")
	}
	return &v1.TaskCancelRes{}, nil
}

// TaskRetry 重试失败任务
func (s *sAdmin) TaskRetry(ctx context.Context, req *v1.TaskRetryReq) (*v1.TaskRetryRes, error) {
	var task struct {
		Id         int64  `json:"id"`
		Status     string `json:"status"`
		RetryCount int    `json:"retry_count"`
		MaxRetries int    `json:"max_retries"`
	}
	err := dao.TskTasks.Ctx(ctx).Where("id", req.ID).Scan(&task)
	if err != nil {
		return nil, err
	}
	if task.Id == 0 {
		return nil, gerror.NewCode(gcode.New(consts.CodeNotFound, consts.MsgNotFound, nil), consts.MsgNotFound)
	}
	if task.Status != "failed" {
		return nil, gerror.NewCode(gcode.New(consts.CodeTaskNotRetryable, consts.MsgTaskNotRetryable, nil),
			consts.MsgTaskNotRetryable)
	}

	_, err = dao.TskTasks.Ctx(ctx).Where("id", req.ID).Data(do.TskTasks{
		Status:       "pending",
		RetryCount:   task.RetryCount + 1,
		ErrorMessage: "",
		StartedAt:    nil,
		FinishedAt:   nil,
	}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.TaskRetryRes{TaskID: task.Id}, nil
}

// MarkStuckTasksFailed 标记卡住的任务为失败（定时任务）
func MarkStuckTasksFailed(ctx context.Context) error {
	threshold := time.Now().Add(-30 * time.Minute)
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
