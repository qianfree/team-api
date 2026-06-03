package middleware

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/entity"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/response"
)

// AuthContextKey is the key used to store auth info in context.
const (
	CtxKeyUserID    = "userId"
	CtxKeyUserType  = "userType"
	CtxKeyRole      = "role"
	CtxKeyTenantID  = "tenantId"
	CtxKeySessionID = "sessionId"
	CtxKeyJti       = "jti"
	CtxKeyApiKeyID  = "apiKeyId"
	CtxKeyProjectID = "projectId"
)

// adminPublicPaths lists admin routes that skip JWT auth.
// Keep in sync with api/admin/v1/ structs tagged group:"public" middleware:"-".
var adminPublicPaths = map[string]bool{
	"/api/admin/auth/login":   true,
	"/api/admin/auth/refresh": true,
}

// AdminAuth is JWT authentication middleware for admin backend.
func AdminAuth(r *ghttp.Request) {
	// g.Meta middleware:"-" only skips service middleware, not group middleware.
	// Public endpoints must be checked explicitly here.
	if adminPublicPaths[r.URL.Path] {
		r.Middleware.Next()
		return
	}

	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		response.ErrorMsg(r, consts.CodeUnauthorized, consts.MsgUnauthorized)
		return
	}

	claims, err := common.ParseAccessToken(tokenStr)
	if err != nil {
		response.ErrorMsg(r, consts.CodeUnauthorized, consts.MsgUnauthorized)
		return
	}

	// Verify user type
	if claims.UserType != "admin" {
		response.ErrorMsg(r, consts.CodeForbidden, consts.MsgForbidden)
		return
	}

	// Check if session is revoked by jti (JWT ID).
	// Note: tokens without jti cannot be revoked — this is by design for backward compat.
	if claims.ID != "" && common.IsSessionRevoked(r.Context(), claims.ID) {
		response.ErrorWithCode(r, consts.CodeUnauthorized, consts.CodeTokenRevoked, consts.MsgTokenRevoked)
		return
	}

	// Verify user still exists and is active
	var user *entity.SysAdminUsers
	err = dao.SysAdminUsers.Ctx(r.Context()).
		Where("id", claims.UserID).
		Fields("status").
		Scan(&user)
	if err != nil {
		response.ErrorMsg(r, consts.CodeUnauthorized, consts.MsgUnauthorized)
		return
	}
	if user == nil || user.Status != "active" {
		response.ErrorMsg(r, consts.CodeUnauthorized, consts.MsgUnauthorized)
		return
	}

	// Set auth context
	r.SetCtxVar(CtxKeyUserID, claims.UserID)
	r.SetCtxVar(CtxKeyUserType, claims.UserType)
	r.SetCtxVar(CtxKeyRole, claims.Role)
	r.SetCtxVar(CtxKeySessionID, claims.SessionID)
	r.SetCtxVar(CtxKeyJti, claims.ID)

	r.Middleware.Next()
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) int64 {
	val := ctx.Value(CtxKeyUserID)
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

// GetUserRole extracts user role from context.
func GetUserRole(ctx context.Context) string {
	val := ctx.Value(CtxKeyRole)
	if val == nil {
		return ""
	}
	if role, ok := val.(string); ok {
		return role
	}
	return ""
}

// GetUserType extracts user type from context.
func GetUserType(ctx context.Context) string {
	val := ctx.Value(CtxKeyUserType)
	if val == nil {
		return ""
	}
	if t, ok := val.(string); ok {
		return t
	}
	return ""
}

// GetSessionID extracts session ID from context.
func GetSessionID(ctx context.Context) int64 {
	val := ctx.Value(CtxKeySessionID)
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

// GetJti extracts the JWT ID (jti) from context.
func GetJti(ctx context.Context) string {
	val := ctx.Value(CtxKeyJti)
	if val == nil {
		return ""
	}
	if jti, ok := val.(string); ok {
		return jti
	}
	return ""
}

// GetTenantID extracts tenant ID from context.
func GetTenantID(ctx context.Context) int64 {
	val := ctx.Value(CtxKeyTenantID)
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

// GetApiKeyID extracts API Key ID from context.
func GetApiKeyID(ctx context.Context) int64 {
	val := ctx.Value(CtxKeyApiKeyID)
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

// GetProjectID extracts Project ID from context.
func GetProjectID(ctx context.Context) int64 {
	val := ctx.Value(CtxKeyProjectID)
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

// extractBearerToken extracts the Bearer token from the Authorization header.
// Falls back to "token" query parameter for WebSocket connections.
func extractBearerToken(r *ghttp.Request) string {
	authHeader := r.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// Fallback: check query parameter for WebSocket (browsers can't set headers)
	if token := r.Get("token").String(); token != "" {
		return token
	}

	return ""
}
