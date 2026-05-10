package middleware

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/admin"
	"github.com/qianfree/team-api/internal/response"
)

// RequirePermission returns middleware that checks if the admin user has the required permission.
func RequirePermission(permission string) func(r *ghttp.Request) {
	return func(r *ghttp.Request) {
		role := GetUserRole(r.Context())
		userID := GetUserID(r.Context())

		if role == "super_admin" {
			r.Middleware.Next()
			return
		}

		if !admin.HasPermission(r.Context(), userID, role, permission) {
			response.ErrorMsg(r, consts.CodeForbidden, "缺少权限："+permission)
			return
		}

		r.Middleware.Next()
	}
}

// RequireRole returns middleware that checks if the tenant user has the required role.
func RequireTenantRole(roles ...string) func(r *ghttp.Request) {
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(r *ghttp.Request) {
		role := GetUserRole(r.Context())
		if !roleSet[role] {
			response.ErrorMsg(r, consts.CodeForbidden, consts.MsgForbidden)
			return
		}
		r.Middleware.Next()
	}
}
