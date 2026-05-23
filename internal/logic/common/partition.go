package common

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// partitionedTable 定义需要自动管理分区的表
type partitionedTable struct {
	table           string
	partitionColumn string                          // 分区依据的时间列
	preCheck        func(ctx context.Context) error // 写入前的幂等修复（如补列）
}

// partitionedTables 所有需要分区管理的表
var partitionedTables = []partitionedTable{
	{
		table:           "bil_usage_logs",
		partitionColumn: "created_at",
	},
	{
		table:           "ops_system_metrics",
		partitionColumn: "collected_at",
	},
	{
		table:           "chn_error_events",
		partitionColumn: "created_at",
	},
}

// EnsurePartitions 检查并补齐所有分区表的当前月+未来 3 个月分区
func EnsurePartitions(ctx context.Context) error {
	now := time.Now()
	// 确保当前月 + 未来 3 个月
	months := make([]time.Time, 0, 4)
	for i := range 4 {
		months = append(months, time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC))
	}

	var firstErr error
	for _, pt := range partitionedTables {
		// 写入前的幂等修复（如补列）
		if pt.preCheck != nil {
			if err := pt.preCheck(ctx); err != nil {
				g.Log().Warningf(ctx, "partition: pre-check for %s failed: %v", pt.table, err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}

		for _, m := range months {
			partitionName := fmt.Sprintf("%s_%d_%02d", pt.table, m.Year(), m.Month())
			nextMonth := time.Date(m.Year(), m.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			from := m.Format("2006-01-02")
			to := nextMonth.Format("2006-01-02")

			sql := fmt.Sprintf(
				`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
				partitionName, pt.table, from, to,
			)
			_, err := g.DB().Exec(ctx, sql)
			if err != nil {
				g.Log().Warningf(ctx, "partition: create %s failed: %v", partitionName, err)
				if firstErr == nil {
					firstErr = gerror.Wrapf(err, "create %s", partitionName)
				}
			}
		}
	}

	if firstErr != nil {
		return gerror.Newf("partition check had errors (first: %v)", firstErr)
	}
	return nil
}
