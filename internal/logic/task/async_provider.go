package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/relay/common"
)

// AsyncProvider 异步任务数据持久化实现
type AsyncProvider struct{}

var DefaultAsyncProvider = &AsyncProvider{}

// CreateTask 创建异步任务记录
func (p *AsyncProvider) CreateTask(ctx context.Context, task *common.AsyncTask) error {
	_, err := dao.TskModelTasks.Ctx(ctx).Insert(map[string]any{
		"public_task_id":    task.PublicTaskID,
		"request_id":        task.RequestID,
		"platform":          task.Platform,
		"action":            task.Action,
		"status":            task.Status,
		"progress":          task.Progress,
		"fail_reason":       task.FailReason,
		"tenant_id":         task.TenantID,
		"user_id":           task.UserID,
		"api_key_id":        task.ApiKeyID,
		"channel_id":        task.ChannelID,
		"model_name":        task.ModelName,
		"upstream_model":    task.UpstreamModel,
		"pre_deduct_amount": task.PreDeductAmount,
		"actual_cost":       task.ActualCost,
		"billing_settled":   task.BillingSettled,
		"result_url":        task.ResultURL,
		"data":              task.Data,
		"private_data":      task.PrivateData,
		"submit_time":       task.SubmitTime,
	})
	if err != nil {
		return gerror.Wrapf(err, "create async task failed: public_id=%s", task.PublicTaskID)
	}
	IncrActiveTask()
	return nil
}

// UpdateTask 更新任务记录
func (p *AsyncProvider) UpdateTask(ctx context.Context, task *common.AsyncTask) error {
	_, err := dao.TskModelTasks.Ctx(ctx).
		Where("id", task.ID).
		Update(map[string]any{
			"status":          task.Status,
			"progress":        task.Progress,
			"fail_reason":     task.FailReason,
			"actual_cost":     task.ActualCost,
			"billing_settled": task.BillingSettled,
			"result_url":      task.ResultURL,
			"data":            task.Data,
			"start_time":      task.StartTime,
			"finish_time":     task.FinishTime,
			"updated_at":      time.Now(),
		})
	if err != nil {
		return gerror.Wrapf(err, "update async task failed: id=%d", task.ID)
	}
	return nil
}

// UpdateTaskCAS CAS 状态更新
func (p *AsyncProvider) UpdateTaskCAS(ctx context.Context, task *common.AsyncTask, oldStatus string) error {
	result, err := dao.TskModelTasks.Ctx(ctx).
		Where("id", task.ID).
		Where("status", oldStatus).
		Update(map[string]any{
			"status":          task.Status,
			"progress":        task.Progress,
			"fail_reason":     task.FailReason,
			"actual_cost":     task.ActualCost,
			"billing_settled": task.BillingSettled,
			"result_url":      task.ResultURL,
			"data":            task.Data,
			"start_time":      task.StartTime,
			"finish_time":     task.FinishTime,
			"updated_at":      time.Now(),
		})
	if err != nil {
		return gerror.Wrapf(err, "CAS update async task failed: id=%d", task.ID)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("CAS conflict: task %d status changed from %s by another process", task.ID, oldStatus)
	}
	return nil
}

// GetTaskByPublicID 根据公开任务 ID 查询
func (p *AsyncProvider) GetTaskByPublicID(ctx context.Context, publicTaskID string) (*common.AsyncTask, error) {
	var row struct {
		ID              int64           `json:"id"`
		PublicTaskID    string          `json:"public_task_id"`
		RequestId       string          `json:"request_id"`
		Platform        string          `json:"platform"`
		Action          string          `json:"action"`
		Status          string          `json:"status"`
		Progress        string          `json:"progress"`
		FailReason      string          `json:"fail_reason"`
		TenantID        int64           `json:"tenant_id"`
		UserID          int64           `json:"user_id"`
		ApiKeyID        int64           `json:"api_key_id"`
		ChannelID       int64           `json:"channel_id"`
		ModelName       string          `json:"model_name"`
		UpstreamModel   string          `json:"upstream_model"`
		PreDeductAmount float64         `json:"pre_deduct_amount"`
		ActualCost      float64         `json:"actual_cost"`
		BillingSettled  bool            `json:"billing_settled"`
		ResultURL       string          `json:"result_url"`
		Data            json.RawMessage `json:"data"`
		PrivateData     json.RawMessage `json:"private_data"`
		SubmitTime      *time.Time      `json:"submit_time"`
		StartTime       *time.Time      `json:"start_time"`
		FinishTime      *time.Time      `json:"finish_time"`
		CreatedAt       time.Time       `json:"created_at"`
		UpdatedAt       time.Time       `json:"updated_at"`
	}
	err := dao.TskModelTasks.Ctx(ctx).
		Where("public_task_id", publicTaskID).
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrapf(err, "query async task failed: public_id=%s", publicTaskID)
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &common.AsyncTask{
		ID:              row.ID,
		PublicTaskID:    row.PublicTaskID,
		RequestID:       row.RequestId,
		Platform:        row.Platform,
		Action:          row.Action,
		Status:          row.Status,
		Progress:        row.Progress,
		FailReason:      row.FailReason,
		TenantID:        row.TenantID,
		UserID:          row.UserID,
		ApiKeyID:        row.ApiKeyID,
		ChannelID:       row.ChannelID,
		ModelName:       row.ModelName,
		UpstreamModel:   row.UpstreamModel,
		PreDeductAmount: row.PreDeductAmount,
		ActualCost:      row.ActualCost,
		BillingSettled:  row.BillingSettled,
		ResultURL:       row.ResultURL,
		Data:            row.Data,
		PrivateData:     row.PrivateData,
		SubmitTime:      row.SubmitTime,
		StartTime:       row.StartTime,
		FinishTime:      row.FinishTime,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

// GetTaskByPublicIDAndUser 根据公开任务 ID + 用户 ID 查询
func (p *AsyncProvider) GetTaskByPublicIDAndUser(ctx context.Context, publicTaskID string, userID int64) (*common.AsyncTask, error) {
	var row struct {
		ID              int64           `json:"id"`
		PublicTaskID    string          `json:"public_task_id"`
		RequestId       string          `json:"request_id"`
		Platform        string          `json:"platform"`
		Action          string          `json:"action"`
		Status          string          `json:"status"`
		Progress        string          `json:"progress"`
		FailReason      string          `json:"fail_reason"`
		TenantID        int64           `json:"tenant_id"`
		UserID          int64           `json:"user_id"`
		ApiKeyID        int64           `json:"api_key_id"`
		ChannelID       int64           `json:"channel_id"`
		ModelName       string          `json:"model_name"`
		UpstreamModel   string          `json:"upstream_model"`
		PreDeductAmount float64         `json:"pre_deduct_amount"`
		ActualCost      float64         `json:"actual_cost"`
		BillingSettled  bool            `json:"billing_settled"`
		ResultURL       string          `json:"result_url"`
		Data            json.RawMessage `json:"data"`
		SubmitTime      *time.Time      `json:"submit_time"`
		StartTime       *time.Time      `json:"start_time"`
		FinishTime      *time.Time      `json:"finish_time"`
		CreatedAt       time.Time       `json:"created_at"`
		UpdatedAt       time.Time       `json:"updated_at"`
	}
	err := dao.TskModelTasks.Ctx(ctx).
		Where("public_task_id", publicTaskID).
		Where("user_id", userID).
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrapf(err, "query async task failed: public_id=%s user_id=%d", publicTaskID, userID)
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &common.AsyncTask{
		ID:              row.ID,
		PublicTaskID:    row.PublicTaskID,
		RequestID:       row.RequestId,
		Platform:        row.Platform,
		Action:          row.Action,
		Status:          row.Status,
		Progress:        row.Progress,
		FailReason:      row.FailReason,
		TenantID:        row.TenantID,
		UserID:          row.UserID,
		ApiKeyID:        row.ApiKeyID,
		ChannelID:       row.ChannelID,
		ModelName:       row.ModelName,
		UpstreamModel:   row.UpstreamModel,
		PreDeductAmount: row.PreDeductAmount,
		ActualCost:      row.ActualCost,
		BillingSettled:  row.BillingSettled,
		ResultURL:       row.ResultURL,
		Data:            row.Data,
		SubmitTime:      row.SubmitTime,
		StartTime:       row.StartTime,
		FinishTime:      row.FinishTime,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

// GetNonTerminalTasks 获取所有非终态任务
func (p *AsyncProvider) GetNonTerminalTasks(ctx context.Context, limit int) ([]*common.AsyncTask, error) {
	var rows []struct {
		ID              int64           `json:"id"`
		PublicTaskID    string          `json:"public_task_id"`
		RequestId       string          `json:"request_id"`
		Platform        string          `json:"platform"`
		Action          string          `json:"action"`
		Status          string          `json:"status"`
		Progress        string          `json:"progress"`
		TenantID        int64           `json:"tenant_id"`
		UserID          int64           `json:"user_id"`
		ApiKeyID        int64           `json:"api_key_id"`
		ChannelID       int64           `json:"channel_id"`
		ModelName       string          `json:"model_name"`
		UpstreamModel   string          `json:"upstream_model"`
		PreDeductAmount float64         `json:"pre_deduct_amount"`
		ActualCost      float64         `json:"actual_cost"`
		BillingSettled  bool            `json:"billing_settled"`
		Data            json.RawMessage `json:"data"`
		PrivateData     json.RawMessage `json:"private_data"`
		SubmitTime      *time.Time      `json:"submit_time"`
		CreatedAt       time.Time       `json:"created_at"`
	}
	err := dao.TskModelTasks.Ctx(ctx).
		Where("status NOT IN (?, ?)", "SUCCESS", "FAILURE").
		Order("submit_time ASC").
		Limit(limit).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrapf(err, "query non-terminal tasks failed")
	}

	tasks := make([]*common.AsyncTask, 0, len(rows))
	for _, r := range rows {
		tasks = append(tasks, &common.AsyncTask{
			ID:              r.ID,
			PublicTaskID:    r.PublicTaskID,
			RequestID:       r.RequestId,
			Platform:        r.Platform,
			Action:          r.Action,
			Status:          r.Status,
			Progress:        r.Progress,
			TenantID:        r.TenantID,
			UserID:          r.UserID,
			ApiKeyID:        r.ApiKeyID,
			ChannelID:       r.ChannelID,
			ModelName:       r.ModelName,
			UpstreamModel:   r.UpstreamModel,
			PreDeductAmount: r.PreDeductAmount,
			ActualCost:      r.ActualCost,
			BillingSettled:  r.BillingSettled,
			Data:            r.Data,
			PrivateData:     r.PrivateData,
			SubmitTime:      r.SubmitTime,
			CreatedAt:       r.CreatedAt,
		})
	}
	return tasks, nil
}

// GetTimedOutTasks 获取超时未完成任务
func (p *AsyncProvider) GetTimedOutTasks(ctx context.Context, cutoffUnix int64, limit int) ([]*common.AsyncTask, error) {
	cutoffTime := time.Unix(cutoffUnix, 0)
	var rows []struct {
		ID              int64           `json:"id"`
		PublicTaskID    string          `json:"public_task_id"`
		RequestId       string          `json:"request_id"`
		Platform        string          `json:"platform"`
		Status          string          `json:"status"`
		TenantID        int64           `json:"tenant_id"`
		UserID          int64           `json:"user_id"`
		ApiKeyID        int64           `json:"api_key_id"`
		ChannelID       int64           `json:"channel_id"`
		ModelName       string          `json:"model_name"`
		PreDeductAmount float64         `json:"pre_deduct_amount"`
		PrivateData     json.RawMessage `json:"private_data"`
		SubmitTime      *time.Time      `json:"submit_time"`
	}
	err := dao.TskModelTasks.Ctx(ctx).
		Where("status NOT IN (?, ?)", "SUCCESS", "FAILURE").
		Where("submit_time < ?", cutoffTime).
		Order("submit_time ASC").
		Limit(limit).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrapf(err, "query timed-out tasks failed")
	}

	tasks := make([]*common.AsyncTask, 0, len(rows))
	for _, r := range rows {
		tasks = append(tasks, &common.AsyncTask{
			ID:              r.ID,
			PublicTaskID:    r.PublicTaskID,
			RequestID:       r.RequestId,
			Platform:        r.Platform,
			Status:          r.Status,
			TenantID:        r.TenantID,
			UserID:          r.UserID,
			ApiKeyID:        r.ApiKeyID,
			ChannelID:       r.ChannelID,
			ModelName:       r.ModelName,
			PreDeductAmount: r.PreDeductAmount,
			PrivateData:     r.PrivateData,
			SubmitTime:      r.SubmitTime,
		})
	}
	return tasks, nil
}

// GetChannelByID 获取渠道基本信息（含从 chn_channel_keys 解密的 API Key）
func (p *AsyncProvider) GetChannelByID(ctx context.Context, channelID int64) (*common.ChannelBasicInfo, error) {
	var row struct {
		ID       int64           `json:"id"`
		Type     int             `json:"type"`
		Name     string          `json:"name"`
		BaseURL  string          `json:"base_url"`
		Settings json.RawMessage `json:"settings"`
	}
	err := dao.ChnChannels.Ctx(ctx).
		Where("id", channelID).
		Fields("id, type, name, base_url, settings").
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrapf(err, "query channel failed: id=%d", channelID)
	}
	if row.ID == 0 {
		return nil, nil
	}

	// 从 chn_channel_keys 获取解密后的 API Key
	apiKey, keyErr := getChannelApiKey(ctx, channelID)
	if keyErr != nil {
		return nil, gerror.Wrapf(keyErr, "get channel key failed: channelID=%d", channelID)
	}

	return &common.ChannelBasicInfo{
		ID:       row.ID,
		Type:     row.Type,
		Name:     row.Name,
		BaseURL:  row.BaseURL,
		ApiKey:   apiKey,
		Settings: row.Settings,
	}, nil
}

// getChannelApiKey 从 chn_channel_keys 获取并解密渠道 API Key
func getChannelApiKey(ctx context.Context, channelID int64) (string, error) {
	type keyRow struct {
		ID           int64  `json:"id"`
		EncryptedKey string `json:"encrypted_key"`
	}

	var key *keyRow
	err := dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", channelID).
		Where("status", "active").
		Fields("id, encrypted_key").
		Scan(&key)
	if err != nil || key == nil {
		return "", fmt.Errorf("no active key found for channel %d", channelID)
	}

	// 更新最后使用时间
	dao.ChnChannelKeys.Ctx(ctx).
		Where("id", key.ID).
		Data(do.ChnChannelKeys{LastUsedAt: gtime.Now()}).
		Update()

	encKey := relay.GetEncryptionKey()
	decrypted, err := crypto.DecryptString(encKey, key.EncryptedKey)
	if err != nil {
		return "", fmt.Errorf("decrypt key failed: %w", err)
	}
	return decrypted, nil
}
