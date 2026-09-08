package helper

import (
	"bytes"
	"encoding/json"
	"errors"
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

// ssePingFrame 保活注释帧。SSE 规范中以 `:` 开头的行是注释，客户端会忽略但会保持连接活跃。
const ssePingFrame = ": PING\n\n"

// WriteSSEPing 写入 SSE 保活注释（`: PING\n\n`）。
// w 为 SafeWriter 时走 WritePing：在同一把锁内检查「调用方是否正处于分多次写同一帧的中途」，
// 是则跳过本次 ping —— ping 自带的空行会把那一帧劈成两个事件（详见 SSEFrameWriter 说明）。
func WriteSSEPing(w http.ResponseWriter) error {
	if sw, ok := w.(*SafeWriter); ok {
		_, err := sw.WritePing()
		return err
	}
	_, err := w.Write([]byte(ssePingFrame))
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

const (
	// sseEventBufInitCap 池中缓冲的初始容量，覆盖绝大多数流式事件，避免首次写入扩容
	sseEventBufInitCap = 2 * 1024
	// sseEventBufKeepCap 归还池的缓冲容量上限，超过则丢弃，避免个别超大事件把容量永久钉在池里
	sseEventBufKeepCap = 64 * 1024
)

// ErrSSEPayloadMarshal payload 序列化失败，事件未写出（一个字节都没有出网）。
// 调用方据此区分两类失败：
//   - 序列化失败 → 事件被跳过，应记日志：写出 data 为空的畸形帧会让客户端 JSON.parse("")
//     抛错并中止整条请求，而网关侧只会看到 ctx 取消、记成 client_gone；
//   - 写客户端失败 → 客户端已不可达，由主循环的中断路径统一处理，不必逐事件刷日志。
var ErrSSEPayloadMarshal = errors.New("sse payload marshal failed")

// sseEventBuffer 复用的帧组装缓冲。encoder 在构造时绑定到 buf，二者一起池化，
// 使「序列化 + 拼帧」全程零分配。
//
// 复用 json.Encoder 的前提：Encoder 只在底层 writer 写失败时置位其内部粘滞错误，
// 而 bytes.Buffer 的 Write 永不返回错误，故池中的 encoder 不会被毒化。
type sseEventBuffer struct {
	buf *bytes.Buffer
	enc *json.Encoder
}

var sseEventBufPool = sync.Pool{
	New: func() any {
		buf := bytes.NewBuffer(make([]byte, 0, sseEventBufInitCap))
		return &sseEventBuffer{buf: buf, enc: json.NewEncoder(buf)}
	},
}

func getSSEEventBuffer() *sseEventBuffer {
	eb := sseEventBufPool.Get().(*sseEventBuffer)
	eb.buf.Reset()
	return eb
}

func putSSEEventBuffer(eb *sseEventBuffer) {
	if eb.buf.Cap() > sseEventBufKeepCap {
		return
	}
	sseEventBufPool.Put(eb)
}

// writeSSEFrame 把 eb 中已拼好的帧一次性写出并 Flush。
func writeSSEFrame(w http.ResponseWriter, eb *sseEventBuffer) error {
	if _, err := w.Write(eb.buf.Bytes()); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// WriteSSEEventJSON 序列化 payload 并作为一个完整 SSE 事件帧一次性写出（`event: X\ndata: {...}\n\n`）。
// 相比「json.Marshal → string() → Fprintf」少若干次分配，且整帧单次 Write —— 天然满足帧原子，
// 并发的保活 ping 无法把空行插进帧中间。
// 序列化失败时不写出任何字节，返回包装 ErrSSEPayloadMarshal 的错误。
func WriteSSEEventJSON(w http.ResponseWriter, event string, payload any) error {
	eb := getSSEEventBuffer()
	defer putSSEEventBuffer(eb)

	eb.buf.WriteString("event: ")
	eb.buf.WriteString(event)
	eb.buf.WriteString("\ndata: ")
	// Encode 在 JSON 之后自带换行，正好充当 data 行的行尾；序列化失败时不会向 buf 写入任何内容
	if err := eb.enc.Encode(payload); err != nil {
		return fmt.Errorf("%w: %v", ErrSSEPayloadMarshal, err)
	}
	eb.buf.WriteByte('\n') // 帧分隔空行

	return writeSSEFrame(w, eb)
}

// WriteSSEDataJSON 序列化 payload 并作为一个只有 data 行的完整 SSE 帧一次性写出
// （`data: {...}\n\n`）。语义与 WriteSSEEventJSON 相同，用于 OpenAI 风格的无 event 名流式块。
func WriteSSEDataJSON(w http.ResponseWriter, payload any) error {
	eb := getSSEEventBuffer()
	defer putSSEEventBuffer(eb)

	eb.buf.WriteString("data: ")
	if err := eb.enc.Encode(payload); err != nil {
		return fmt.Errorf("%w: %v", ErrSSEPayloadMarshal, err)
	}
	eb.buf.WriteByte('\n')

	return writeSSEFrame(w, eb)
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
	midFrame   bool // 调用方正在分多次写出同一帧（SSEFrameWriter 超大帧降级），期间抑制 ping
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

// SetMidFrame 标记/清除「当前有一帧正在分多次写出」。置位期间 WritePing 跳过写入，
// 避免 ping 自带的空行落在帧中间。仅 SSEFrameWriter 的超大帧降级路径会用到。
func (s *SafeWriter) SetMidFrame(v bool) {
	s.mu.Lock()
	s.midFrame = v
	s.mu.Unlock()
}

// WritePing 在持锁状态下写出保活注释帧，返回是否实际写出。
// 检查与写入必须在同一临界区内完成，否则「检查通过 → 调用方开始写半帧 → ping 写入」
// 的交错依然会劈开帧。
func (s *SafeWriter) WritePing() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.midFrame {
		return false, nil
	}
	if _, err := s.w.Write([]byte(ssePingFrame)); err != nil {
		return false, err
	}
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
	return true, nil
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
// 单帧超过 sseFrameMaxBuf 时降级为分次写出，此期间通过 SafeWriter.SetMidFrame 抑制 ping，
// 保证降级路径同样不会被空行劈开。
// 本类型非并发安全，只应由转发主循环单协程使用，底层 w 需为 SafeWriter。
type SSEFrameWriter struct {
	w   http.ResponseWriter
	buf []byte
}

// NewSSEFrameWriter 创建帧级原子写入器，w 应为并发安全的 writer（如 SafeWriter）。
func NewSSEFrameWriter(w http.ResponseWriter) *SSEFrameWriter {
	return &SSEFrameWriter{w: w}
}

// WriteLine 追加一行（含行尾换行符）。该行为帧分隔空行时整帧写出；
// 缓冲超过上限时降级为分次写出，并在帧写完前抑制保活 ping。
func (f *SSEFrameWriter) WriteLine(line string) error {
	f.buf = append(f.buf, line...)
	if IsSSEFrameBoundary(line) {
		return f.flush(true)
	}
	if len(f.buf) >= sseFrameMaxBuf {
		// 超大帧降级：写出已缓冲部分，但这一批字节不含帧尾，
		// 标记 midFrame 让 ping 让路，直到该帧的空行到达
		return f.flush(false)
	}
	return nil
}

// FlushPartial 冲刷未以空行结尾的残留内容（上游 EOF 时调用，避免丢掉最后一帧）。
func (f *SSEFrameWriter) FlushPartial() error {
	return f.flush(true)
}

// Discard 丢弃尚未写出的半帧内容。
func (f *SSEFrameWriter) Discard() {
	f.buf = f.buf[:0]
	f.setMidFrame(false)
}

// Buffered 返回当前缓冲的字节数（尚未写给客户端的半帧）。
func (f *SSEFrameWriter) Buffered() int {
	return len(f.buf)
}

// setMidFrame 把「帧写到一半」的状态透给底层 SafeWriter，由它在写 ping 时同锁判定。
func (f *SSEFrameWriter) setMidFrame(v bool) {
	if sw, ok := f.w.(*SafeWriter); ok {
		sw.SetMidFrame(v)
	}
}

// flush 写出缓冲内容。frameComplete 表示这批字节写完后帧是否完整
// （遇到分隔空行 / 上游 EOF 收尾为 true，超大帧降级为 false）。
func (f *SSEFrameWriter) flush(frameComplete bool) error {
	if len(f.buf) == 0 {
		if frameComplete {
			f.setMidFrame(false)
		}
		return nil
	}
	if !frameComplete {
		// 必须在 Write 之前置位：否则 ping 可能挤在「写出半帧」与「置位」之间
		f.setMidFrame(true)
	}
	_, err := f.w.Write(f.buf)
	if cap(f.buf) > sseFrameKeepBuf {
		f.buf = nil
	} else {
		f.buf = f.buf[:0]
	}
	if frameComplete {
		// 帧已完整出网，放行 ping（写失败也要清除，避免把标记永久留在 writer 上）
		f.setMidFrame(false)
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
