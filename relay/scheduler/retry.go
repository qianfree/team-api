package scheduler

import (
	"github.com/qianfree/team-api/relay/constant"
)

// MaxRetries 最大重试次数
const MaxRetries = 3

// RetryStrategy 重试策略
type RetryStrategy struct {
	MaxRetries     int
	ExcludeChannel int64
}

// ShouldRetry 判断是否应该重试
func ShouldRetry(err error, retryCount int, maxRetries int) bool {
	if retryCount >= maxRetries {
		return false
	}
	return constant.IsRetryable(err)
}

// DefaultRetryStrategy 默认重试策略
func DefaultRetryStrategy(excludeChannelID int64) *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:     MaxRetries,
		ExcludeChannel: excludeChannelID,
	}
}
