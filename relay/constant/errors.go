package constant

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Relay 层哨兵错误
var ErrAllChannelsFailed = gerror.New("all channels failed")

// RelayError 包装 relay 层错误，携带上游返回的状态码和信息
type RelayError struct {
	StatusCode int
	Message    string
	Type       string // upstream_error / channel_error / auth_error / request_error
	Cause      error
	// ResponseWritten 表示 adaptor 已直接向客户端写入响应体，
	// 上层错误写入器（WriteRelayError / WriteClaudeRelayError / WriteGeminiRelayError）应跳过二次写入。
	// 字段对重试 / 健康度 / 计费 / 状态码重映射等逻辑透明（仍按 StatusCode 判断）。
	ResponseWritten bool
}

func (e *RelayError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *RelayError) Unwrap() error {
	return e.Cause
}

// NewUpstreamError 创建上游错误
func NewUpstreamError(statusCode int, message string, cause error) *RelayError {
	return &RelayError{
		StatusCode: statusCode,
		Message:    message,
		Type:       "upstream_error",
		Cause:      cause,
	}
}

// NewChannelError 创建渠道错误
func NewChannelError(message string, cause error) *RelayError {
	return &RelayError{
		StatusCode: 503,
		Message:    message,
		Type:       "channel_error",
		Cause:      cause,
	}
}

// NewAuthError 创建认证错误
func NewAuthError(message string) *RelayError {
	return &RelayError{
		StatusCode: 401,
		Message:    message,
		Type:       "auth_error",
	}
}

// NewRequestError 创建请求错误
func NewRequestError(message string, cause error) *RelayError {
	return &RelayError{
		StatusCode: 400,
		Message:    message,
		Type:       "request_error",
		Cause:      cause,
	}
}

// NewQuotaError 创建额度/余额不足错误
func NewQuotaError(message string, cause error) *RelayError {
	return &RelayError{
		StatusCode: 402,
		Message:    message,
		Type:       "insufficient_quota",
		Cause:      cause,
	}
}

// NewRateLimitError 创建频率限制错误
func NewRateLimitError(message string) *RelayError {
	return &RelayError{
		StatusCode: 429,
		Message:    message,
		Type:       "rate_limit_error",
	}
}

// NewModelGoneError 创建模型已下线错误（410 Gone）
func NewModelGoneError(modelName, sunsetDate string) *RelayError {
	return &RelayError{
		StatusCode: 410,
		Message:    "model '" + modelName + "' has been sunset since " + sunsetDate,
		Type:       "model_gone",
	}
}

// IsResponseWritten 判断错误对应的响应体是否已由 adaptor 直接写入客户端。
// 一旦上游的原生错误体已透传给客户端，重试后再写成功体会造成「错误体+成功体」拼接污染，
// 因此重试循环命中此情况必须终止重试（即使 StatusCode 本身可重试）。
func IsResponseWritten(err error) bool {
	if err == nil {
		return false
	}
	var relayErr *RelayError
	if errors.As(err, &relayErr) {
		return relayErr.ResponseWritten
	}
	return false
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var relayErr *RelayError
	if errors.As(err, &relayErr) {
		switch relayErr.StatusCode {
		case 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}
	// 非 RelayError：检查是否为连接层 / 网络层瞬时错误（请求未获得任何 HTTP 响应）
	return isTransientNetworkError(err)
}

// isTransientNetworkError 判断是否为可重试的连接层 / 网络层瞬时错误。
// 这类错误发生时请求未获得任何 HTTP 响应（连接被对端提前关闭、重置、超时或拒绝），
// 重试通常能命中健康渠道或新建连接成功，是中转网关最应重试 / 故障转移的场景。
// 适配器（如 Gemini DoRequest）把 client.Do 的这类错误包成普通 error（非 *RelayError），
// 因此需要在这里显式识别，否则会被 IsRetryable 判为不可重试而直接失败。
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// 客户端主动断开（context 已取消）：响应无处可回，重试无意义且浪费上游调用
	if errors.Is(err, context.Canceled) {
		return false
	}

	// 连接被对端提前关闭（GFW 干扰 / 上游瞬断的典型表现）
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// 连接被重置 / 拒绝
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	// 请求 / 握手超时（i/o timeout、TLS handshake timeout、Client.Timeout 等）
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// http.Client 超时也会以 context.DeadlineExceeded 形式冒泡
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// isAmbiguousDelivery 判断连接层错误是否属于「请求可能已送达上游」的模糊情形。
// 这类错误（io.EOF/ErrUnexpectedEOF/ECONNRESET/超时）发生时，上游有可能已收到并开始
// 处理请求，只是响应在回传途中中断——对非幂等且高成本的生成（图片/视频）重试会造成
// 上游重复生成 + 重复计费。
// 反之不属于模糊情形（重试安全）：① 连接被拒绝 / DNS 解析失败——请求确定未送达；
// ② 状态码类 RelayError——已拿到上游 HTTP 响应，说明上游已明确返回错误、未成功生成。
func isAmbiguousDelivery(err error) bool {
	if err == nil {
		return false
	}
	// 拿到了 HTTP 响应（哪怕是错误状态码）→ 非模糊。
	var relayErr *RelayError
	if errors.As(err, &relayErr) {
		return false
	}
	// 连接被拒绝：上游从未接受连接，请求确定未送达 → 非模糊。
	if errors.Is(err, syscall.ECONNREFUSED) {
		return false
	}
	// DNS 解析失败：请求确定未送达 → 非模糊。
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return false
	}
	// 其余瞬时网络错误（EOF/RST/i/o timeout/DeadlineExceeded）：可能已送达 → 模糊。
	return isTransientNetworkError(err)
}

// IsRetryableForRequest 在 DoRequest（发送请求）阶段判断错误是否可重试。
// 相比 IsRetryable 多一个 nonIdempotentExpensive 参数：对非幂等且高成本的生成端点
// （图片/视频），「可能已送达上游」的模糊连接层错误不重试，避免上游重复生成 + 重复计费；
// 「确定未送达」的错误（连接被拒/DNS 失败）与状态码类错误仍照常重试。其它端点（chat 等）
// 行为与 IsRetryable 完全一致。
func IsRetryableForRequest(err error, nonIdempotentExpensive bool) bool {
	if !IsRetryable(err) {
		return false
	}
	if nonIdempotentExpensive && isAmbiguousDelivery(err) {
		return false
	}
	return true
}
