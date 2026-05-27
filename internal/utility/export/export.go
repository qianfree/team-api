package export

import (
	"context"

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
// For xlsx: collects all rows from queryFn, then calls WriteExcel.
// For csv: passes queryFn directly to StreamCSV for streaming.
func GenericExport(ctx context.Context, config Config, queryFn func(yield func(map[string]any) bool)) error {
	r := g.RequestFromCtx(ctx)
	config.Format = detectFormat(config.Format)

	if config.Format == "xlsx" {
		var data []map[string]any
		queryFn(func(row map[string]any) bool {
			data = append(data, row)
			return true
		})
		return WriteExcel(r, config, data)
	}

	return StreamCSV(r, config, queryFn)
}
