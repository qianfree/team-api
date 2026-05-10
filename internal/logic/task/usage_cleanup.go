package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	handlerUsageLogCleanup = "usage_log_cleanup"
	maxCleanupRangeDays    = 366
	defaultCleanupBatch    = 5000
)

type UsageCleanupPayload struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	TenantID  *int64    `json:"tenant_id,omitempty"`
	ModelName string    `json:"model_name,omitempty"`
	Status    string    `json:"status,omitempty"`
	BatchSize int       `json:"batch_size,omitempty"`
	DryRun    bool      `json:"dry_run,omitempty"`
}

type cleanupResult struct {
	Mode             string   `json:"mode"`
	DroppedParts     []string `json:"dropped_partitions,omitempty"`
	DeletedRows      int64    `json:"deleted_rows"`
	DryRun           bool     `json:"dry_run"`
	PartitionActions []string `json:"partition_actions,omitempty"`
}

func init() {
	RegisterHandler(handlerUsageLogCleanup, handleUsageLogCleanup)
}

func handleUsageLogCleanup(ctx context.Context, payload json.RawMessage) (any, error) {
	var p UsageCleanupPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("unmarshal cleanup payload: %w", err)
	}

	if p.EndTime.Before(p.StartTime) || p.EndTime.Equal(p.StartTime) {
		return nil, fmt.Errorf("end_time must be after start_time")
	}
	if p.EndTime.Sub(p.StartTime).Hours()/24 > maxCleanupRangeDays {
		return nil, fmt.Errorf("time range exceeds %d days", maxCleanupRangeDays)
	}
	if p.BatchSize <= 0 {
		p.BatchSize = defaultCleanupBatch
	}

	hasFilters := p.TenantID != nil || p.ModelName != "" || p.Status != ""

	result := &cleanupResult{DryRun: p.DryRun}

	if !hasFilters {
		if err := cleanupWithPartitions(ctx, &p, result); err != nil {
			return result, err
		}
	} else {
		result.Mode = "row_delete"
		if err := cleanupWithBatchDelete(ctx, &p, result); err != nil {
			return result, err
		}
	}

	return result, nil
}

// cleanupWithPartitions 分区级清理 + 头尾月份行级删除
func cleanupWithPartitions(ctx context.Context, p *UsageCleanupPayload, result *cleanupResult) error {
	result.Mode = "partition"

	startMonth := time.Date(p.StartTime.Year(), p.StartTime.Month(), 1, 0, 0, 0, 0, time.UTC)
	endMonth := time.Date(p.EndTime.Year(), p.EndTime.Month(), 1, 0, 0, 0, 0, time.UTC)

	// 判断 start 是否在月初
	startIsMonthStart := p.StartTime.Equal(startMonth)
	// 判断 end 是否在月初（即覆盖到上个月末）
	endIsMonthStart := p.EndTime.Equal(endMonth)

	// 可以整月 DROP 的范围
	dropFrom := startMonth
	if !startIsMonthStart {
		dropFrom = startMonth.AddDate(0, 1, 0) // 头部不完整，从下个月开始 DROP
	}
	dropTo := endMonth // DROP 到 endMonth 之前（不含 endMonth 所在月）
	if endIsMonthStart {
		dropTo = endMonth
	}

	// DROP 完整月份分区
	cur := dropFrom
	for cur.Before(dropTo) {
		partName := fmt.Sprintf("bil_usage_logs_%d_%02d", cur.Year(), cur.Month())

		if p.DryRun {
			result.PartitionActions = append(result.PartitionActions, fmt.Sprintf("WOULD DROP %s", partName))
		} else {
			exists, err := partitionExists(ctx, partName)
			if err != nil {
				g.Log().Warningf(ctx, "usage_cleanup: check partition %s: %v", partName, err)
			}
			if exists {
				_, err := g.DB().Exec(ctx, fmt.Sprintf("DROP TABLE %s", partName))
				if err != nil {
					g.Log().Errorf(ctx, "usage_cleanup: drop partition %s failed: %v", partName, err)
					return fmt.Errorf("drop partition %s: %w", partName, err)
				}
				result.DroppedParts = append(result.DroppedParts, partName)
			}
		}
		cur = cur.AddDate(0, 1, 0)
	}

	// 头部不完整月份：行级删除
	if !startIsMonthStart {
		headEnd := dropFrom
		if headEnd.After(p.EndTime) {
			headEnd = p.EndTime
		}
		headPayload := &UsageCleanupPayload{
			StartTime: p.StartTime,
			EndTime:   headEnd,
			BatchSize: p.BatchSize,
			DryRun:    p.DryRun,
		}
		if err := cleanupWithBatchDelete(ctx, headPayload, result); err != nil {
			return err
		}
	}

	// 尾部不完整月份：行级删除
	if !endIsMonthStart && dropTo.Before(p.EndTime) {
		tailPayload := &UsageCleanupPayload{
			StartTime: dropTo,
			EndTime:   p.EndTime,
			BatchSize: p.BatchSize,
			DryRun:    p.DryRun,
		}
		if err := cleanupWithBatchDelete(ctx, tailPayload, result); err != nil {
			return err
		}
	}

	return nil
}

// cleanupWithBatchDelete 行级批量删除
func cleanupWithBatchDelete(ctx context.Context, p *UsageCleanupPayload, result *cleanupResult) error {
	if p.DryRun {
		count, err := countMatchingRows(ctx, p)
		if err != nil {
			return err
		}
		result.DeletedRows += count
		return nil
	}

	for {
		deleted, err := deleteBatch(ctx, p)
		if err != nil {
			return fmt.Errorf("batch delete: %w", err)
		}
		result.DeletedRows += deleted
		if deleted < int64(p.BatchSize) {
			break
		}
	}
	return nil
}

// deleteBatch 执行一批删除，返回实际删除行数
func deleteBatch(ctx context.Context, p *UsageCleanupPayload) (int64, error) {
	sql := `DELETE FROM bil_usage_logs WHERE id IN (
		SELECT id FROM bil_usage_logs WHERE created_at >= $1 AND created_at < $2`
	args := []any{p.StartTime, p.EndTime}
	argIdx := 3

	if p.TenantID != nil {
		sql += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, *p.TenantID)
		argIdx++
	}
	if p.ModelName != "" {
		sql += fmt.Sprintf(" AND model_name = $%d", argIdx)
		args = append(args, p.ModelName)
		argIdx++
	}
	if p.Status != "" {
		sql += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, p.Status)
		argIdx++
	}

	sql += fmt.Sprintf(" LIMIT $%d)", argIdx)
	args = append(args, p.BatchSize)

	res, err := g.DB().Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// countMatchingRows DryRun 模式下统计匹配行数
func countMatchingRows(ctx context.Context, p *UsageCleanupPayload) (int64, error) {
	sql := "SELECT COUNT(*) FROM bil_usage_logs WHERE created_at >= $1 AND created_at < $2"
	args := []any{p.StartTime, p.EndTime}
	argIdx := 3

	if p.TenantID != nil {
		sql += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, *p.TenantID)
		argIdx++
	}
	if p.ModelName != "" {
		sql += fmt.Sprintf(" AND model_name = $%d", argIdx)
		args = append(args, p.ModelName)
		argIdx++
	}
	if p.Status != "" {
		sql += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, p.Status)
		argIdx++
	}

	val, err := g.DB().GetValue(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return val.Int64(), nil
}

// partitionExists 检查分区表是否存在
func partitionExists(ctx context.Context, partName string) (bool, error) {
	val, err := g.DB().GetValue(ctx,
		"SELECT COUNT(*) FROM pg_class WHERE relname = $1 AND relkind = 'r'", partName)
	if err != nil {
		return false, err
	}
	return val.Int64() > 0, nil
}

// CreateUsageCleanupTask 创建清理任务（供 admin logic 调用）
func CreateUsageCleanupTask(ctx context.Context, p *UsageCleanupPayload) (int64, error) {
	now := time.Now()
	return CreateTask(ctx, &Task{
		Name:        "usage_log_cleanup",
		Handler:     handlerUsageLogCleanup,
		Payload:     p,
		MaxRetries:  0,
		ScheduledAt: &now,
	})
}

// ScheduleAutoCleanup 自动清理入口（由 cron 调用）
func ScheduleAutoCleanup(ctx context.Context, retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	p := &UsageCleanupPayload{
		StartTime: cutoff.AddDate(0, 0, -7), // 兼容 cron 连续数天未执行的情况
		EndTime:   cutoff,
		BatchSize: defaultCleanupBatch,
	}
	_, err := CreateUsageCleanupTask(ctx, p)
	return err
}
