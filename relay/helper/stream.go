package helper

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SetEventStreamHeaders 设置 SSE 必要的响应头
func SetEventStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲
	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// WriteSSEData 写入一行 SSE 数据
func WriteSSEData(w http.ResponseWriter, data string) error {
	_, err := fmt.Fprintf(w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// WriteSSEEvent 写入一个完整的 SSE 事件（包含 event 和 data）
func WriteSSEEvent(w http.ResponseWriter, event string, data string) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// WriteSSEPing 写入 SSE 保活注释（`: PING\n\n`）
// SSE 规范中以 `:` 开头的行是注释，客户端会忽略但会保持连接活跃
func WriteSSEPing(w http.ResponseWriter) error {
	_, err := fmt.Fprintf(w, ": PING\n\n")
	if err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// WriteJSON 写入 JSON 响应
func WriteJSON(w http.ResponseWriter, statusCode int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, err := fmt.Fprintf(w, "%s", data)
	return err
}

// ExtractSSEData 从 SSE 行中提取 data 字段内容
// 兼容 "data: content"（带空格）和 "data:content"（不带空格）两种格式
// 如果不是 data 行返回 ("", false)
func ExtractSSEData(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	data := line[5:] // 跳过 "data:" 前缀
	// 跳过可选的单个空格（SSE 规范：data: 后最多忽略一个空格）
	if len(data) > 0 && data[0] == ' ' {
		data = data[1:]
	}
	return strings.TrimSpace(data), true
}

// PingTicker 在后台定期发送 SSE 保活注释
// 返回一个 stop 函数用于停止 goroutine
func PingTicker(w http.ResponseWriter, interval time.Duration) (stop func()) {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := WriteSSEPing(w); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}
