package helper

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/qianfree/team-api/relay/dto"
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

// BuildOpenAIStreamChunk 构建 OpenAI 格式的流式响应块
func BuildOpenAIStreamChunk(id string, created int64, model string, content string, finishReason *string) dto.ChatCompletionStreamResponse {
	delta := dto.Message{}
	if content != "" {
		delta.Content = content
	}
	if finishReason == nil {
		delta.Role = "assistant"
	}

	return dto.ChatCompletionStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []dto.StreamChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	}
}

// EstimateTokens 粗略估算 token 数（每 4 个字符约 1 个 token）
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

// SafeWriter 包装 http.ResponseWriter，用互斥锁串行化 Write/WriteHeader/Flush，
// 使保活 ping goroutine 与主循环可以安全地并发写同一个 ResponseWriter。
//
// 注意：fmt.Fprintf 对单个 SSE 帧只产生一次 Write 调用，因此每个 SSE 帧在锁内是原子写入的；
// Flush 单独加锁，与其他写入交错也不会破坏帧的字节完整性。
type SafeWriter struct {
	w  http.ResponseWriter
	mu sync.Mutex
}

// NewSafeWriter 创建一个并发安全的 ResponseWriter 包装器。
func NewSafeWriter(w http.ResponseWriter) *SafeWriter {
	return &SafeWriter{w: w}
}

func (s *SafeWriter) Header() http.Header {
	return s.w.Header()
}

func (s *SafeWriter) WriteHeader(statusCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.w.WriteHeader(statusCode)
}

func (s *SafeWriter) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(b)
}

// Flush 实现 http.Flusher，刷新底层 writer（若支持）。
func (s *SafeWriter) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

var (
	_ http.ResponseWriter = (*SafeWriter)(nil)
	_ http.Flusher        = (*SafeWriter)(nil)
)

// PingTicker 在后台定期发送 SSE 保活注释
// 调用方必须传入并发安全的 writer（如 SafeWriter），以避免与主循环并发写 ResponseWriter
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
