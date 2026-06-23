package middleware

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/entity"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/response"
)

// tenantPublicPaths lists tenant routes that skip JWT auth.
// Keep in sync with api/tenant/v1/ structs tagged group:"public" middleware:"-".
var tenantPublicPaths = map[string]bool{
	"/api/tenant/auth/register":        true,
	"/api/tenant/auth/login":           true,
	"/api/tenant/auth/refresh":         true,
	"/api/tenant/auth/2fa/verify":      true,
	"/api/tenant/email/send-code":      true,
	"/api/tenant/email/reset-password": true,
	"/api/tenant/members/join":         true,
	"/api/tenant/members/invite-info":  true,
	"/api/tenant/agreements/current":   true,
	"/api/tenant/oauth/authorize":      true,
	"/api/tenant/help/categories":      true,
	"/api/tenant/help/search":          true,
}

// tenantPublicPrefixes lists path prefixes that skip JWT auth (for dynamic routes like /current/{code}).
var tenantPublicPrefixes = []string{
	"/api/tenant/agreements/current/",
	"/api/tenant/help/categories/",
	"/api/tenant/help/articles/",
}

// TenantAuth is JWT authentication middleware for tenant console.
func TenantAuth(r *ghttp.Request) {
	// g.Meta middleware:"-" only skips service middleware, not group middleware.
	// Public endpoints must be checked explicitly here.
	if tenantPublicPaths[r.URL.Path] {
		r.Middleware.Next()
		return
	}
	for _, prefix := range tenantPublicPrefixes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			r.Middleware.Next()
			return
		}
	}
	// OAuth callback: /api/tenant/oauth/{provider}/callback
	if strings.HasPrefix(r.URL.Path, "/api/tenant/oauth/") && strings.HasSuffix(r.URL.Path, "/callback") {
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
	if claims.UserType != "tenant" {
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
	var user *entity.TntUsers
	err = dao.TntUsers.Ctx(r.Context()).
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
	r.SetCtxVar(CtxKeyTenantID, claims.TenantID)
	r.SetCtxVar(CtxKeySessionID, claims.SessionID)
	r.SetCtxVar(CtxKeyJti, claims.ID)

	r.Middleware.Next()
}
