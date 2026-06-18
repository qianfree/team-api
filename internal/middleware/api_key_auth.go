package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/relay"
)

// ApiKeyAuth 是 API Key 认证中间件，用于 AI 代理端点。
// 支持以下认证方式（按优先级）：
//   - x-goog-api-key: sk-xxx（Gemini 原生格式）
//   - Authorization: Bearer sk-xxx（OpenAI 兼容格式）
//   - x-api-key: sk-xxx（Anthropic Claude 原生格式）
//   - ?key=xxx query parameter（Gemini SDK 格式，仅 /v1beta 路径）
//
// 验证通过后注入 TenantID/UserID/ApiKeyID。
// 认证失败时使用供应商原生错误格式（/v1/messages 用 Claude 格式，其余用 OpenAI 格式）。
func ApiKeyAuth(r *ghttp.Request) {
	tokenStr := strings.TrimSpace(r.GetHeader("x-goog-api-key"))
	if tokenStr == "" {
		tokenStr = extractBearerToken(r)
	}
	if tokenStr == "" {
		tokenStr = strings.TrimSpace(r.GetHeader("x-api-key"))
	}
	// Gemini 原生 API 通过 query parameter 传递密钥：?key=xxx（仅 /v1beta 路径）
	if tokenStr == "" && strings.HasPrefix(r.URL.Path, "/v1beta") {
		tokenStr = strings.TrimSpace(r.Get("key").String())
	}
	if tokenStr == "" {
		writeRelayAuthError(r, http.StatusUnauthorized, "authentication_error", "missing API key")
		return
	}

	apiKeyInfo, err := relay.ValidateApiKey(r.Context(), tokenStr)
	if err != nil {
		code := http.StatusUnauthorized
		errType := "authentication_error"
		msg := "invalid API key"

		switch err {
		case consts.ErrKeyExpired:
			errType = "authentication_error"
			msg = "API key has expired"
		case consts.ErrKeyDisabled:
			code = http.StatusForbidden
			errType = "permission_error"
			msg = "API key is disabled"
		case consts.ErrTenantSuspended:
			code = http.StatusForbidden
			errType = "permission_error"
			msg = "tenant is suspended"
		case consts.ErrProjectNotActive:
			code = http.StatusForbidden
			errType = "permission_error"
			msg = "project is not active"
		}

		writeRelayAuthError(r, code, errType, msg)
		return
	}

	// Set auth context
	r.SetCtxVar(CtxKeyTenantID, apiKeyInfo.TenantID)
	r.SetCtxVar(CtxKeyUserID, apiKeyInfo.UserID)
	r.SetCtxVar(CtxKeyApiKeyID, apiKeyInfo.ID)
	r.SetCtxVar(CtxKeyProjectID, apiKeyInfo.ProjectID)
	r.SetCtxVar("ApiKeyScope", apiKeyInfo.Scope)

	r.Middleware.Next()
}

// writeRelayAuthError 写入供应商原生格式的认证错误响应
// /v1/messages 使用 Claude 格式，其余使用 OpenAI 格式
func writeRelayAuthError(r *ghttp.Request, statusCode int, errType, message string) {
	r.Response.Header().Set("Content-Type", "application/json")
	r.Response.WriteHeader(statusCode)

	path := r.URL.Path
	if path == "/v1/messages" {
		// Claude 格式: {"type": "error", "error": {"type": "...", "message": "..."}}
		body, _ := json.Marshal(map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    errType,
				"message": message,
			},
		})
		r.Response.Write(body)
	} else {
		// OpenAI 格式: {"error": {"type": "...", "message": "..."}}
		body, _ := json.Marshal(map[string]any{
			"error": map[string]string{
				"type":    errType,
				"message": message,
			},
		})
		r.Response.Write(body)
	}
}
