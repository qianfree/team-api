package helper

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Claude 官方错误类型词表（docs/协议文档/Claude协议文档.md §10 错误格式）。
// 写给 Claude 客户端的 error.type 必须取自该词表，Anthropic SDK 按 type 分支；
// 网关内部口径（upstream_error / channel_error 等）不可直接外泄。
const (
	ClaudeErrorInvalidRequest  = "invalid_request_error"
	ClaudeErrorAuthentication  = "authentication_error"
	ClaudeErrorPermission      = "permission_error"
	ClaudeErrorNotFound        = "not_found_error"
	ClaudeErrorRequestTooLarge = "request_too_large"
	ClaudeErrorRateLimit       = "rate_limit_error"
	ClaudeErrorAPI             = "api_error"
	ClaudeErrorOverloaded      = "overloaded_error"
)

// claudeTypeByStatus HTTP 状态码 → Claude 错误类型（官方映射表）。
// 未列出的状态码按官方口径兜底：其余 4xx 归 invalid_request_error，5xx 归 api_error。
var claudeTypeByStatus = map[int]string{
	400: ClaudeErrorInvalidRequest,
	401: ClaudeErrorAuthentication,
	403: ClaudeErrorPermission,
	404: ClaudeErrorNotFound,
	413: ClaudeErrorRequestTooLarge,
	429: ClaudeErrorRateLimit,
	500: ClaudeErrorAPI,
	529: ClaudeErrorOverloaded,
}

// claudeTypeByInternal 网关内部错误类型（constant.RelayError.Type）→ Claude 类型。
// 未列出的内部类型（如 upstream_error，语义由状态码承载）走状态码兜底。
var claudeTypeByInternal = map[string]string{
	"auth_error":         ClaudeErrorAuthentication,
	"request_error":      ClaudeErrorInvalidRequest,
	"insufficient_quota": ClaudeErrorInvalidRequest, // 官方词表无计费类型，按未列出 4xx 口径归 invalid_request_error
	"rate_limit_error":   ClaudeErrorRateLimit,
	"model_gone":         ClaudeErrorNotFound,
	"channel_error":      ClaudeErrorAPI,
}

// ClaudeErrorType 计算写给 Claude 客户端的 error.type，三级优先：
//  1. 上游响应体为 Anthropic 错误信封时原样保留其 error.type——原生 Claude 渠道
//     错误语义最准（如 overloaded_error）；type 是受控小词表字段，无泄露风险
//  2. 网关自产错误按内部类型映射（auth_error → authentication_error 等）
//  3. 状态码按官方映射表兜底——跨协议上游（OpenAI/Gemini）错误体的 type 不取，
//     其词表与 Claude 不通用（如 OpenAI 的 insufficient_quota 不在 Claude 词表内）
func ClaudeErrorType(statusCode int, internalType, upstreamBody string) string {
	if t := extractAnthropicErrorType(upstreamBody); t != "" {
		return t
	}
	if t, ok := claudeTypeByInternal[internalType]; ok {
		return t
	}
	if t, ok := claudeTypeByStatus[statusCode]; ok {
		return t
	}
	if statusCode >= 500 {
		return ClaudeErrorAPI
	}
	return ClaudeErrorInvalidRequest
}

// extractAnthropicErrorType 从上游错误响应体原文中提取 Anthropic 信封的 error.type。
// 仅当响应体形如 {"type":"error","error":{"type":"..."}} 时提取——顶层 type=="error"
// 是 Anthropic 方言的强信号；OpenAI 信封（无顶层 type）与 Gemini 信封（code/status
// 结构）不满足该形状，自动落到状态码映射。非 JSON / 解析失败 / type 为空返回空串。
func extractAnthropicErrorType(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return ""
	}
	var envelope struct {
		Type  string `json:"type"`
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		return ""
	}
	if envelope.Type != "error" {
		return ""
	}
	return strings.TrimSpace(envelope.Error.Type)
}

// ShouldRetryStatus 判断状态码是否值得客户端重试（x-should-retry 头取值）。
// 429/5xx：退避后或换渠道可能成功；其余 4xx 为确定性失败，重试必然同结果。
// Claude Code 读取该头决定是否自动重试——余额不足（402）等场景直接报给用户，
// 避免无效重试拖长失败反馈。
func ShouldRetryStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}
