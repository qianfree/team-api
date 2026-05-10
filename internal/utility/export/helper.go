package export

import (
	"fmt"
	"net/url"
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
