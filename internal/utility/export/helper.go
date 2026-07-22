package export

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
)

var (
	csvContentType  = "text/csv; charset=utf-8"
	xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// setDownloadHeaders sets HTTP headers for file download.
func setDownloadHeaders(r *ghttp.Request, filename, format string) {
	contentType := csvContentType
	ext := ".csv"
	if format == "xlsx" {
		contentType = xlsxContentType
		ext = ".xlsx"
	}

	encodedName := url.PathEscape(filename + ext)
	r.Response.Header().Set("Content-Type", contentType)
	r.Response.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename+ext, encodedName))
	r.Response.Header().Set("X-Content-Type-Options", "nosniff")
}

// defaultFilename generates a default filename with timestamp.
func defaultFilename(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, time.Now().Format("20060102_150405"))
}

// detectFormat validates and normalizes the format string.
func detectFormat(format string) string {
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "xlsx" || f == "excel" {
		return "xlsx"
	}
	return "csv"
}

// GetCellValue extracts a value from a row map, applying the column's Transform if set.
func GetCellValue(row map[string]any, col Column) string {
	val, ok := row[col.Field]
	if !ok {
		return ""
	}
	if col.Transform != nil {
		return col.Transform(val)
	}
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

// sanitizeCSVField 防御 CSV 公式注入（CWE-1236）。
// Excel/LibreOffice/Google Sheets 打开 CSV 时，会把以 = + - @ 及 TAB(0x09)/CR(0x0D) 开头的
// 单元格当作公式求值，用户可控字段（租户名/成员名/工单标题等）可借此注入 DDE/命令执行。
// 处理策略：对以危险字符开头且【整体不是合法数字】的值，前置单引号中和为纯文本；
// 合法数字（含负号/正号/科学计数法）保持原样，避免破坏数值列的可计算性。
// 仅用于 CSV 写入；XLSX 经 excelize 以字符串类型单元格存储，Excel 不会将其当公式求值，无需处理。
func sanitizeCSVField(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return s // 纯数字，Excel 不会当公式，安全
		}
		return "'" + s
	}
	return s
}
