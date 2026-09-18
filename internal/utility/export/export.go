package export

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// Column defines one column in the exported file.
type Column struct {
	Field     string           // Key in the data map
	Header    string           // Display header (Chinese)
	Transform func(any) string // Optional value transformer
}

// Config holds the full export configuration.
type Config struct {
	Format   string   // "csv" or "xlsx"
	Filename string   // Without extension
	Columns  []Column // Ordered column definitions
	MaxRows  int      // Safety cap for Excel (default 100000)
}

// GetMaxRows returns the configured max rows or the default.
func (c Config) GetMaxRows() int {
	if c.MaxRows <= 0 {
		return 100000
	}
	return c.MaxRows
}

// GenericExport handles format normalization and xlsx/csv branching.
// queryFn yields data rows — same callback as StreamCSV's queryFn.
// For xlsx: collects rows from queryFn (capped), then calls WriteExcel.
// For csv: passes queryFn to StreamCSV for streaming (capped).
//
// 行数上限在这里统一强制执行（对拉取过程截停，而不是拉完再检查）：
// 导出的数据源是「无总量保证」的分批循环查询，上限是防止一次导出请求
// 拖着数据库扫全量大表的最后一道护栏。
//   - xlsx：达到上限直接返回错误（此时还未写出任何响应，能干净地报错），
//     并且不再继续拉取 —— 旧实现是全部拉进内存后才检查，护栏形同虚设；
//   - csv：边查边流式写出，表头和数据已经发给客户端，超限时无法再改报错误，
//     只能停止拉取并在文件末尾追加一行可见的截断标记，避免使用者
//     误把部分数据当作全量。
func GenericExport(ctx context.Context, config Config, queryFn func(yield func(map[string]any) bool)) error {
	r := g.RequestFromCtx(ctx)
	config.Format = detectFormat(config.Format)
	maxRows := config.GetMaxRows()

	if config.Format == "xlsx" {
		var (
			data     []map[string]any
			overflow bool
		)
		queryFn(func(row map[string]any) bool {
			if len(data) >= maxRows {
				overflow = true
				return false
			}
			data = append(data, row)
			return true
		})
		if overflow {
			return gerror.Newf("数据量超过 %d 行，请缩小筛选范围后重试，或改用 CSV 格式导出", maxRows)
		}
		return WriteExcel(r, config, data)
	}

	truncated := false
	capped := func(yield func(map[string]any) bool) {
		rowCount := 0
		queryFn(func(row map[string]any) bool {
			if rowCount >= maxRows {
				truncated = true
				return false
			}
			rowCount++
			return yield(row)
		})
		if truncated && len(config.Columns) > 0 {
			yield(map[string]any{
				config.Columns[0].Field: fmt.Sprintf("注意：已达到导出上限 %d 行，结果被截断，请缩小时间范围或筛选条件后分批导出", maxRows),
			})
		}
	}
	return StreamCSV(r, config, capped)
}
