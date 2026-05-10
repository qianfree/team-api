package export

import (
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/xuri/excelize/v2"
)

// WriteExcel generates an Excel file and writes it as an HTTP response.
// The entire dataset must fit in memory; use CSV for very large exports.
func WriteExcel(r *ghttp.Request, config Config, data []map[string]any) error {
	if len(data) > config.GetMaxRows() {
		return gerror.Newf("数据量超过 %d 行，请改用 CSV 格式导出", config.GetMaxRows())
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"

	// Create header style
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#F3F4F6"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "D1D5DB", Style: 1},
			{Type: "top", Color: "D1D5DB", Style: 1},
			{Type: "bottom", Color: "D1D5DB", Style: 1},
			{Type: "right", Color: "D1D5DB", Style: 1},
		},
	})

	// Write headers
	for i, col := range config.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, col.Header)
		_ = f.SetCellStyle(sheet, cell, cell, style)
	}

	// Write data rows
	for rowIdx, row := range data {
		rowNum := rowIdx + 2 // 1-indexed, row 1 is header
		for colIdx, col := range config.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNum)
			val := GetCellValue(row, col)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}

	// Auto-fit column widths based on header length
	for i, col := range config.Columns {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		width := float64(len([]rune(col.Header)))*1.5 + 4
		if width < 12 {
			width = 12
		}
		_ = f.SetColWidth(sheet, colName, colName, width)
	}

	// Write to response
	filename := config.Filename
	if filename == "" {
		filename = defaultFilename("export")
	}
	setDownloadHeaders(r, filename, "xlsx")

	buf, err := f.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("生成 Excel 失败: %w", err)
	}

	_, err = r.Response.RawWriter().Write(buf.Bytes())
	return err
}
