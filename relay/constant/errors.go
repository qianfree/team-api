package constant

import (
	"errors"

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
	return false
}
