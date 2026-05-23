package constant

import (
	"errors"
	"strings"
)

// 错误分类常量
const (
	ErrorCategoryRateLimit     = "rate_limit"
	ErrorCategoryAuthError     = "auth_error"
	ErrorCategoryTimeout       = "timeout"
	ErrorCategoryUpstreamError = "upstream_error"
	ErrorCategoryServerError   = "server_error"
	ErrorCategoryNetworkError  = "network_error"
	ErrorCategoryUnknown       = "unknown"
)

// ClassifyError 将 RelayError 映射到标准错误分类
func ClassifyError(err error) string {
	if err == nil {
		return ErrorCategoryUnknown
	}

	var relayErr *RelayError
	if errors.As(err, &relayErr) {
		return classifyRelayError(relayErr)
	}

	return classifyRawError(err)
}

func classifyRelayError(err *RelayError) string {
	switch err.StatusCode {
	case 429:
		return ErrorCategoryRateLimit
	case 401, 403:
		return ErrorCategoryAuthError
	case 504:
		return ErrorCategoryTimeout
	case 500, 502, 503:
		return ErrorCategoryServerError
	case 400, 422:
		return ErrorCategoryUpstreamError
	default:
		return classifyByType(err.Type)
	}
}

func classifyByType(errType string) string {
	switch errType {
	case "rate_limit_error":
		return ErrorCategoryRateLimit
	case "auth_error":
		return ErrorCategoryAuthError
	case "insufficient_quota":
		return ErrorCategoryUpstreamError
	case "model_gone":
		return ErrorCategoryUpstreamError
	case "channel_error":
		return ErrorCategoryServerError
	case "upstream_error":
		return ErrorCategoryUpstreamError
	case "request_error":
		return ErrorCategoryUpstreamError
	default:
		return ErrorCategoryUnknown
	}
}

func classifyRawError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "deadline exceeded"), strings.Contains(msg, "context deadline"):
		return ErrorCategoryTimeout
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "DNS"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "TLS handshake"), strings.Contains(msg, "certificate"):
		return ErrorCategoryNetworkError
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "i/o timeout"):
		return ErrorCategoryTimeout
	default:
		return ErrorCategoryUnknown
	}
}
