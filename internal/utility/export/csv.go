package export

import (
	"encoding/csv"
	"strconv"

	"github.com/gogf/gf/v2/net/ghttp"
)

// StreamCSV writes a CSV file as a streaming HTTP response.
// queryFn receives a yield callback; call yield(row) for each data row.
// Data is fetched and written in batches to avoid loading everything into memory.
func StreamCSV(r *ghttp.Request, config Config, queryFn func(yield func(map[string]any) bool)) error {
	filename := config.Filename
	if filename == "" {
		filename = defaultFilename("export")
	}
	setDownloadHeaders(r, filename, "csv")

	w := r.Response.RawWriter()

	// UTF-8 BOM for Excel compatibility
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.UseCRLF = true

	// Write header row
	headers := make([]string, len(config.Columns))
	for i, col := range config.Columns {
		headers[i] = col.Header
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	// Write data rows via the query callback
	rowCount := 0
	queryFn(func(row map[string]any) bool {
		record := make([]string, len(config.Columns))
		for i, col := range config.Columns {
			record[i] = GetCellValue(row, col)
		}
		_ = writer.Write(record)
		rowCount++

		// Flush every 500 rows to keep memory low
		if rowCount%500 == 0 {
			writer.Flush()
		}
		return true
	})

	writer.Flush()
	return writer.Error()
}

// WriteCSVAll writes a CSV file from a complete data slice.
// For large datasets, prefer StreamCSV.
func WriteCSVAll(r *ghttp.Request, config Config, data []map[string]any) error {
	return StreamCSV(r, config, func(yield func(map[string]any) bool) {
		for _, row := range data {
			if !yield(row) {
				break
			}
		}
	})
}

// parseCSVInt safely parses an int from a CSV cell value.
func parseCSVInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
