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
