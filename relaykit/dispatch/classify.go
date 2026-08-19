package dispatch

import (
	"context"
	"errors"

	"github.com/qianfree/team-api/relaykit/types"
)

// ErrorClass 错误分类。
type ErrorClass int

const (
	ErrClassNone         ErrorClass = iota // 无错误 / 未知
	ErrClassClient                         // 客户端错误：换渠道结果相同，不重试
	ErrClassTransient                      // 瞬时错误：同渠道退避后原地重试
	ErrClassRateLimit                      // 限流：Retry-After 短则原地等待，否则 failover
	ErrClassCredential                     // 凭证错误，先冷却当前 Key 并轮换，耗尽后升级 CHANNEL_FATAL
	ErrClassChannelFatal                   // 渠道致命错误：零原地重试，立即 failover + 熔断计数直达
	ErrClassTimeout                        // 超时（含 504）：不原地重试，按可重放性决定 failover
	ErrClassModelFatal                     // 模型级致命（404/模型不存在/模型映射失败）：立即 failover + 模型级熔断直达，不归因渠道整体
)

// String 实现 fmt.Stringer，用于日志与指标标签。
func (c ErrorClass) String() string {
	switch c {
	case ErrClassClient:
		return "client"
	case ErrClassTransient:
		return "transient"
	case ErrClassRateLimit:
		return "rate_limit"
	case ErrClassCredential:
		return "credential"
	case ErrClassChannelFatal:
		return "channel_fatal"
	case ErrClassModelFatal:
		return "model_fatal"
	case ErrClassTimeout:
		return "timeout"
	default:
		return "none"
	}
}

// Classify 错误分类器（纯函数）。
//
// statusCode 为上游 HTTP 状态码（网络错误时为 0）；err 允许为 relaykit/types.NewAPIError
// （解包并尊重其 SkipRetry / 错误码语义）；delivery 用于区分网络错误发生阶段。
func Classify(statusCode int, err error, delivery DeliveryState) ErrorClass {
	if statusCode == 0 && err == nil {
		return ErrClassNone
	}

	// 优先按 relaykit 错误体系归因
	var apiErr *types.NewAPIError
	if errors.As(err, &apiErr) && apiErr != nil {
		if cls, ok := classifyNewAPIError(apiErr); ok {
			return cls
		}
		// 错误码不能定类时，回落到其携带的状态码
		if statusCode == 0 {
			statusCode = apiErr.StatusCode
		}
	}

	// 超时错误（连接超时 / 首 token 超时 / context deadline）
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
		if errors.Is(err, context.Canceled) {
			// 客户端取消：不重试也不计渠道健康
			return ErrClassClient
		}
		return ErrClassTimeout
	}

	if statusCode > 0 {
		return classifyStatusCode(statusCode)
	}

	// 纯网络错误（无状态码）：按送达阶段分类。
	// NotSent（连接拒绝/DNS/TLS 建连失败）与 MaybeSent（写后 RST/EOF）均归 TRANSIENT，
	// MaybeSent 的「禁止原地重试 + 可重放性限制」由重试 FSM 的 delivery 硬规则处理。
	_ = delivery
	return ErrClassTransient
}

// classifyNewAPIError 按 relaykit 错误码归因。返回 ok=false 表示交由状态码兜底。
func classifyNewAPIError(e *types.NewAPIError) (ErrorClass, bool) {
	// SkipRetry 语义：调用方已声明重试无意义
	if types.IsSkipRetryError(e) {
		return ErrClassClient, true
	}

	switch e.GetErrorCode() {
	// 凭证类
	case types.ErrorCodeChannelInvalidKey:
		return ErrClassCredential, true

	// 渠道级致命
	case types.ErrorCodeChannelNoAvailableKey,
		types.ErrorCodeChannelParamOverrideInvalid,
		types.ErrorCodeChannelHeaderOverrideInvalid,
		types.ErrorCodeChannelAwsClientError:
		return ErrClassChannelFatal, true

	// 模型级致命：只影响渠道下的单个模型，仅喂渠道×模型级熔断（不归因渠道整体）
	case types.ErrorCodeChannelModelMappedError,
		types.ErrorCodeModelNotFound:
		return ErrClassModelFatal, true

	// 超时
	case types.ErrorCodeChannelResponseTimeExceeded:
		return ErrClassTimeout, true

	// 客户端 / 请求级：换渠道结果相同
	case types.ErrorCodeInvalidRequest,
		types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodePromptBlocked,
		types.ErrorCodeBadRequestBody,
		types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeConvertRequestFailed,
		types.ErrorCodeAccessDenied,
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed:
		return ErrClassClient, true
	}

	// 其余 channel: 前缀错误一律渠道级致命
	if types.IsChannelError(e) {
		return ErrClassChannelFatal, true
	}
	return ErrClassNone, false
}

// classifyStatusCode 按 HTTP 状态码分类。
func classifyStatusCode(code int) ErrorClass {
	switch code {
	case 401, 403:
		// 先归因凭证，轮换耗尽后由 FSM 升级为渠道级
		return ErrClassCredential
	case 402:
		// 上游余额耗尽
		return ErrClassChannelFatal
	case 404:
		// 模型在该渠道不存在：换渠道可能有效（喂给渠道×模型级熔断，不伤渠道其它模型）
		return ErrClassModelFatal
	case 408:
		return ErrClassTimeout
	case 429:
		return ErrClassRateLimit
	case 504:
		// 网关超时大概率已送达且上游处理慢，禁止原地重试
		return ErrClassTimeout
	case 500, 502, 503:
		return ErrClassTransient
	}
	// Cloudflare 系瞬时错误
	if code >= 520 && code <= 527 {
		return ErrClassTransient
	}
	if code >= 500 {
		return ErrClassTransient
	}
	if code >= 400 {
		return ErrClassClient
	}
	return ErrClassNone
}
