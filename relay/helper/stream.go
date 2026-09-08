package helper

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// SetEventStreamHeaders 设置 SSE 必要的响应头。
// 使用 SafeWriter 的 context 字段实现幂等性保护，防止重复设置导致的 panic。
func SetEventStreamHeaders(w http.ResponseWriter) {
	// 幂等性检查：如果是 SafeWriter 且已设置过，直接返回
	if sw, ok := w.(*SafeWriter); ok {
		sw.mu.Lock()
		if sw.headersSet {
			sw.mu.Unlock()
			return
		}
		sw.headersSet = true
		sw.mu.Unlock()
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked") // 显式声明分块传输
	w.Header().Set("X-Accel-Buffering", "no")      // 禁用 Nginx 缓冲
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

// EstimateStreamOutputTokens 流式输出兜底估算（上游未返回 usage 时按累计文本长度粗估）。
// 正常结束按每 4 字符 1 token；流中断（客户端断开/空闲超时等部分传输，IsPartialStreamEnd）
// 按每 2 字符 1 token —— 中断时拿不到上游最终 usage，按保守口径计费。
func EstimateStreamOutputTokens(info *common.RelayInfo, textLen int) int {
	if textLen <= 0 {
		return 0
	}
	if info != nil && info.StreamStatus != nil && info.StreamStatus.IsPartialStreamEnd() {
		return (textLen + 1) / 2
	}
	return textLen / 4
}

// ApplyInterruptedUsageFallback 流中断（IsPartialStreamEnd）时的计费兜底修正，正常结束为 no-op：
//  1. 输出：上游未返回输出 token（多数供应商 usage 在最后一个 chunk，中断时拿不到）时，
//     按已转发文本长度以每 2 字符 1 token 估算；
//  2. 输入：上游未返回输入 token 时，用请求侧估算值（与预扣同源）补齐，保证输入正常计费。
//
// 已有真实 usage（如 Claude message_delta 的累计值）时不覆盖，仅重算 TotalTokens。
func ApplyInterruptedUsageFallback(info *common.RelayInfo, usage *common.Usage, transferredTextLen int) {
	if info == nil || usage == nil || info.StreamStatus == nil || !info.StreamStatus.IsPartialStreamEnd() {
		return
	}
	if usage.CompletionTokens == 0 && transferredTextLen > 0 {
		usage.CompletionTokens = (transferredTextLen + 1) / 2
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
}

// SafeWriter 包装 http.ResponseWriter，用互斥锁串行化 Write/WriteHeader/Flush，
// 使保活 ping goroutine 与主循环可以安全地并发写同一个 ResponseWriter。
//
// 注意：本类型只保证**单次 Write 调用**原子，不保证 SSE 帧原子。
// 调用方一次 Write 写完整帧时（如 WriteSSEData/WriteSSEEvent 的单次 Fprintf），帧即原子；
// 而按行转发上游 SSE 的调用方必须改用 SSEFrameWriter 攒齐整帧再写，否则 ping 的空行
// 会插进帧中间，让客户端派发出 data 为空的事件（详见 SSEFrameWriter 的说明）。
type SafeWriter struct {
	w          http.ResponseWriter
	mu         sync.Mutex
	headersSet bool // SSE 头幂等性标志（防止 SetEventStreamHeaders 重复调用导致 panic）
}

// NewSafeWriter 创建一个并发安全的 ResponseWriter 包装器。
func NewSafeWriter(w http.ResponseWriter) *SafeWriter {
	return &SafeWriter{
		w:          w,
		headersSet: false,
	}
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

const (
	// sseFrameMaxBuf 单帧缓冲上限：超过则先把已缓冲内容写出（降级为逐行写），
	// 防止上游异常（始终不发空行）把整条流攒进内存。正常 SSE 帧远小于此。
	sseFrameMaxBuf = 1 << 20 // 1 MiB
	// sseFrameKeepBuf 帧写出后保留复用的缓冲上限，超过则释放，避免个别大帧长期占用内存
	sseFrameKeepBuf = 64 * 1024
)

// IsSSEFrameBoundary 判断一行是否为 SSE 帧分隔空行（兼容 LF / CRLF 两种行尾）。
func IsSSEFrameBoundary(line string) bool {
	return line == "\n" || line == "\r\n"
}

// SSEFrameWriter 为「逐行转发上游 SSE」的场景提供帧级原子写入。
//
// SSE 以空行分帧，客户端读到空行即派发事件。若转发方按行写，而保活 ping goroutine
// 又并发写同一个 writer，ping 自带的空行就可能插进一帧中间，把只写了 `event: X` 的
// 半帧提前派发出去 —— 客户端拿到 data 为空的事件，对其做 JSON.parse 直接抛错
// （如 Claude Code 的 "JSON Parse error: Unexpected EOF"）并中止请求，网关侧随后
// 只观察到 ctx 取消，被记成 client_gone，真实成因被掩盖。
// SafeWriter 只保证单次 Write 原子，保证不了整帧原子，故需本类型按帧攒齐再写。
//
// 用法：逐行 WriteLine（遇空行自动整帧写出并 Flush）；上游 EOF 时 FlushPartial
// 冲刷无空行结尾的残留；中断或上游读错时 Discard 丢弃半帧（半帧尚未出网，丢弃即可）。
// 本类型非并发安全，只应由转发主循环单协程使用，底层 w 需为 SafeWriter。
type SSEFrameWriter struct {
	w   http.ResponseWriter
	buf []byte
}

// NewSSEFrameWriter 创建帧级原子写入器，w 应为并发安全的 writer（如 SafeWriter）。
func NewSSEFrameWriter(w http.ResponseWriter) *SSEFrameWriter {
	return &SSEFrameWriter{w: w}
}

// WriteLine 追加一行（含行尾换行符）。该行为帧分隔空行、或缓冲超过上限时整帧写出。
func (f *SSEFrameWriter) WriteLine(line string) error {
	f.buf = append(f.buf, line...)
	if IsSSEFrameBoundary(line) || len(f.buf) >= sseFrameMaxBuf {
		return f.flush()
	}
	return nil
}

// FlushPartial 冲刷未以空行结尾的残留内容（上游 EOF 时调用，避免丢掉最后一帧）。
func (f *SSEFrameWriter) FlushPartial() error {
	return f.flush()
}

// Discard 丢弃尚未写出的半帧内容。
func (f *SSEFrameWriter) Discard() {
	f.buf = f.buf[:0]
}

// Buffered 返回当前缓冲的字节数（尚未写给客户端的半帧）。
func (f *SSEFrameWriter) Buffered() int {
	return len(f.buf)
}

func (f *SSEFrameWriter) flush() error {
	if len(f.buf) == 0 {
		return nil
	}
	_, err := f.w.Write(f.buf)
	if cap(f.buf) > sseFrameKeepBuf {
		f.buf = nil
	} else {
		f.buf = f.buf[:0]
	}
	if err != nil {
		return err
	}
	if fl, ok := f.w.(http.Flusher); ok {
		fl.Flush()
	}
	return nil
}

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
