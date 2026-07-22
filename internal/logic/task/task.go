package task

import (
	"context"
	"encoding/json"
	"fmt"
	do "github.com/qianfree/team-api/internal/model/do"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
)

// TaskStatus defines the status of an async task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusSucceeded TaskStatus = "succeeded"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

// Task defines an async task to be executed.
type Task struct {
	Name        string
	Handler     string // handler function identifier
	Payload     any
	MaxRetries  int
	ScheduledAt *time.Time
}

// TaskResult represents the result of a task execution.
type TaskResult struct {
	Success bool
	Data    any
	Error   string
}

// HandlerFunc is the function signature for task handlers.
type HandlerFunc func(ctx context.Context, payload json.RawMessage) (any, error)

var (
	handlers   = make(map[string]HandlerFunc)
	handlersMu sync.RWMutex
)

// RegisterHandler registers a task handler function.
func RegisterHandler(name string, handler HandlerFunc) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	handlers[name] = handler
}

// GetHandler retrieves a registered handler.
func GetHandler(name string) (HandlerFunc, bool) {
	handlersMu.RLock()
	defer handlersMu.RUnlock()
	h, ok := handlers[name]
	return h, ok
}

// CreateTask creates a new task record in the database.
func CreateTask(ctx context.Context, task *Task) (int64, error) {
	payload, err := json.Marshal(task.Payload)
	if err != nil {
		return 0, gerror.Wrapf(err, "marshal payload")
	}

	result, err := dao.TskTasks.Ctx(ctx).Data(do.TskTasks{
		Name:       task.Name,
		Handler:    task.Handler,
		Payload:    payload,
		Status:     StatusPending,
		MaxRetries: task.MaxRetries,
		ScheduledAt: func() *gtime.Time {
			if task.ScheduledAt != nil {
				return gtime.NewFromTime(*task.ScheduledAt)
			}
			return nil
		}(),
	}).Insert()
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// ExecuteTask executes a task by ID.
// It loads the task from DB, updates status, calls the handler, and records the result.
func ExecuteTask(ctx context.Context, taskID int64) error {
	// Load task
	var task *struct {
		ID         int64           `json:"id"`
		Name       string          `json:"name"`
		Handler    string          `json:"handler"`
		Payload    json.RawMessage `json:"payload"`
		MaxRetries int             `json:"max_retries"`
		RetryCount int             `json:"retry_count"`
	}

	err := dao.TskTasks.Ctx(ctx).
		Where("id", taskID).
		Where("status", StatusPending).
		Scan(&task)
	if err != nil {
		return gerror.Wrapf(err, "load task")
	}

	if task == nil {
		return gerror.Newf("task %d not found or not pending", taskID)
	}

	// Mark as running —— 原子领取：仅当仍为 pending 时才能翻成 running。
	// 用条件更新 + RowsAffected 做 CAS（对齐 async_provider.UpdateTaskCAS 的做法），
	// 避免「先 SELECT pending 再无条件 UPDATE」在多副本/并发调度下被两个 worker 同时领取、重复执行同一任务。
	res, err := dao.TskTasks.Ctx(ctx).
		Where("id", taskID).
		Where("status", StatusPending).
		Data(do.TskTasks{
			Status:    StatusRunning,
			StartedAt: gtime.Now(),
		}).Update()
	if err != nil {
		return gerror.Wrapf(err, "update task status to running")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return gerror.Wrapf(err, "confirm task claim result")
	}
	if affected == 0 {
		// 另一 worker 已抢先领取（status 已非 pending），本次放弃，避免重复执行
		return nil
	}

	// Get handler
	handler, ok := GetHandler(task.Handler)
	if !ok {
		return failTask(ctx, taskID, fmt.Sprintf("handler %q not registered", task.Handler))
	}

	// Execute handler
	result, execErr := handler(ctx, task.Payload)

	// Record result
	var resultJSON []byte
	if result != nil {
		resultJSON, _ = json.Marshal(result)
	}

	if execErr != nil {
		// Check if should retry
		if task.RetryCount < task.MaxRetries {
			_, _ = dao.TskTasks.Ctx(ctx).
				Where("id", taskID).
				Data(do.TskTasks{
					Status:       StatusPending,
					RetryCount:   task.RetryCount + 1,
					ErrorMessage: execErr.Error(),
					StartedAt:    nil,
				}).Update()
			return execErr
		}
		return failTask(ctx, taskID, execErr.Error())
	}

	// Success
	_, err = dao.TskTasks.Ctx(ctx).
		Where("id", taskID).
		Data(do.TskTasks{
			Status:     StatusSucceeded,
			Result:     resultJSON,
			FinishedAt: gtime.Now(),
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "update task %d to succeeded: %v", taskID, err)
	}

	return nil
}

// failTask marks a task as failed and logs the error.
func failTask(ctx context.Context, taskID int64, errMsg string) error {
	_, err := dao.TskTasks.Ctx(ctx).
		Where("id", taskID).
		Data(do.TskTasks{
			Status:       StatusFailed,
			ErrorMessage: errMsg,
			FinishedAt:   gtime.Now(),
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "update task %d to failed: %v", taskID, err)
	}

	// Log error
	logTask(ctx, taskID, "error", errMsg)
	return gerror.Newf("task failed: %s", errMsg)
}

// logTask adds a log entry for a task.
func logTask(ctx context.Context, taskID int64, level, message string) {
	_, err := dao.TskTaskLogs.Ctx(ctx).Data(do.TskTaskLogs{
		TaskId:  taskID,
		Level:   level,
		Message: message,
	}).Insert()
	if err != nil {
		g.Log().Errorf(ctx, "insert task log: %v", err)
	}
}

// LogInfo adds an info log for a task.
func LogInfo(ctx context.Context, taskID int64, message string) {
	logTask(ctx, taskID, "info", message)
}

// LogError adds an error log for a task.
func LogError(ctx context.Context, taskID int64, message string) {
	logTask(ctx, taskID, "error", message)
}

// CancelTask cancels a pending task.
func CancelTask(ctx context.Context, taskID int64) error {
	result, err := dao.TskTasks.Ctx(ctx).
		Where("id", taskID).
		WhereIn("status", []string{string(StatusPending), string(StatusRunning)}).
		Data(do.TskTasks{
			Status:     StatusCancelled,
			FinishedAt: gtime.Now(),
		}).Update()
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return gerror.Newf("task %d cannot be cancelled", taskID)
	}

	return nil
}

// ScheduleTask creates a task scheduled for a future time.
func ScheduleTask(ctx context.Context, task *Task, runAt time.Time) (int64, error) {
	task.ScheduledAt = &runAt
	return CreateTask(ctx, task)
}

// RunPendingTasks executes all pending tasks that are due.
func RunPendingTasks(ctx context.Context) {
	var tasks []struct {
		ID int64 `json:"id"`
	}
	err := dao.TskTasks.Ctx(ctx).
		Where("status", StatusPending).
		Where("scheduled_at <= ? OR scheduled_at IS NULL", time.Now()).
		Fields("id").
		Scan(&tasks)
	if err != nil {
		g.Log().Errorf(ctx, "query pending tasks: %v", err)
		return
	}

	for _, task := range tasks {
		if err := ExecuteTask(ctx, task.ID); err != nil {
			g.Log().Errorf(ctx, "execute task %d: %v", task.ID, err)
		}
	}
}

// RunTaskAsync executes a task in a goroutine.
func RunTaskAsync(task *Task) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				glogError("panic in async task: %v", r)
			}
		}()

		ctx := gctx.New()
		id, err := CreateTask(ctx, task)
		if err != nil {
			glogError("create async task: %v", err)
			return
		}
		if err := ExecuteTask(ctx, id); err != nil {
			glogError("execute async task %d: %v", id, err)
		}
	}()
}

func glogError(format string, args ...any) {
	g.Log().Errorf(gctx.New(), format, args...)
}
