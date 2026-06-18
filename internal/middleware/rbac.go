package middleware

import (
	"strings"

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

// RequireTenantRole returns middleware that checks if the tenant user has the required role.
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

// adminPermissionRules defines the mapping from route patterns to permission points.
// Only routes with matching permission groups are mapped; unmapped routes remain
// accessible to all authenticated admins.
var adminPermissionRules = []permissionRule{
	// ── tenant 租户管理 ──
	{method: "GET", path: "/api/admin/tenants", perm: "tenant:view"},
	{method: "GET", prefix: "/api/admin/tenants/", perm: "tenant:view"},
	{method: "POST", path: "/api/admin/tenants", perm: "tenant:create"},
	{method: "PUT", prefix: "/api/admin/tenants/", suffix: "/status", perm: "tenant:suspend"},
	{method: "PUT", prefix: "/api/admin/tenants/", suffix: "/channel-scope", perm: "tenant:edit"},
	{method: "PUT", prefix: "/api/admin/tenants/", perm: "tenant:edit"},
	{method: "GET", path: "/api/admin/tenants/export", perm: "tenant:view"},
	{method: "GET", path: "/api/admin/tenants/select", perm: "tenant:view"},
	{method: "GET", path: "/api/admin/tenants/members/export", perm: "member:view"},

	// ── user 用户管理（管理员） ──
	{method: "GET", path: "/api/admin/users", perm: "user:view"},
	{method: "GET", path: "/api/admin/users/export", perm: "user:view"},
	{method: "POST", path: "/api/admin/users", perm: "user:create"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/status", perm: "user:edit"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/reset-password", perm: "user:edit"},
	{method: "PUT", prefix: "/api/admin/users/", perm: "user:edit"},
	{method: "DELETE", prefix: "/api/admin/users/", perm: "user:delete"},

	// ── channel 渠道管理 ──
	{method: "GET", path: "/api/admin/channels", perm: "channel:view"},
	{method: "GET", path: "/api/admin/channels/export", perm: "channel:view"},
	{method: "GET", path: "/api/admin/channels/provider-default-urls", perm: "channel:view"},
	{method: "GET", prefix: "/api/admin/channels/", perm: "channel:view"},
	{method: "POST", path: "/api/admin/channels", perm: "channel:create"},
	{method: "POST", prefix: "/api/admin/channels/", suffix: "/test", perm: "channel:test"},
	{method: "POST", prefix: "/api/admin/channels/", suffix: "/clone", perm: "channel:create"},
	{method: "POST", prefix: "/api/admin/channels/", suffix: "/keys", perm: "channel:edit"},
	{method: "PUT", prefix: "/api/admin/channels/", perm: "channel:edit"},
	{method: "DELETE", prefix: "/api/admin/channels/", perm: "channel:delete"},
	// Channel OAuth helper endpoints
	{method: "POST", path: "/api/admin/channels/oauth/auth-url", perm: "channel:edit"},
	{method: "POST", path: "/api/admin/channels/oauth/exchange", perm: "channel:edit"},
	{method: "POST", path: "/api/admin/channels/oauth/refresh", perm: "channel:edit"},

	// ── model 模型管理 ──
	{method: "GET", path: "/api/admin/models", perm: "model:view"},
	{method: "GET", path: "/api/admin/models/options", perm: "model:view"},
	{method: "GET", path: "/api/admin/models/export", perm: "model:view"},
	{method: "GET", path: "/api/admin/models/official-info", perm: "model:view"},
	{method: "GET", prefix: "/api/admin/models/", perm: "model:view"},
	{method: "POST", path: "/api/admin/models", perm: "model:create"},
	{method: "POST", path: "/api/admin/models/export-json", perm: "model:view"},
	{method: "POST", path: "/api/admin/models/import-preview", perm: "model:create"},
	{method: "POST", path: "/api/admin/models/import", perm: "model:create"},
	{method: "PUT", prefix: "/api/admin/models/", perm: "model:edit"},
	{method: "DELETE", prefix: "/api/admin/models/", perm: "model:delete"},
	// Model groups
	{method: "GET", path: "/api/admin/model-groups", perm: "model:view"},
	{method: "GET", path: "/api/admin/model-groups/options", perm: "model:view"},
	{method: "GET", prefix: "/api/admin/model-groups/", perm: "model:view"},
	{method: "POST", path: "/api/admin/model-groups", perm: "model:create"},
	{method: "PUT", prefix: "/api/admin/model-groups/", perm: "model:edit"},
	{method: "DELETE", prefix: "/api/admin/model-groups/", perm: "model:delete"},
	// Tenant model assignments
	{method: "GET", prefix: "/api/admin/tenants/", suffix: "/models", perm: "model:view"},
	{method: "GET", prefix: "/api/admin/tenants/", suffix: "/available-models", perm: "model:view"},
	{method: "GET", prefix: "/api/admin/tenants/", suffix: "/groups", perm: "model:view"},
	{method: "POST", prefix: "/api/admin/tenants/", suffix: "/models", perm: "model:edit"},
	{method: "PUT", prefix: "/api/admin/tenants/", suffix: "/groups", perm: "model:edit"},
	{method: "PUT", prefix: "/api/admin/tenants/", suffix: "/models/", perm: "model:edit"},
	{method: "DELETE", prefix: "/api/admin/tenants/", suffix: "/models/", perm: "model:delete"},

	// ── billing 计费管理 ──
	{method: "GET", path: "/api/admin/billing-records", perm: "billing:view"},
	{method: "GET", path: "/api/admin/billing-records/export", perm: "billing:export"},
	{method: "GET", path: "/api/admin/usage-logs", perm: "billing:view"},
	{method: "GET", path: "/api/admin/usage-logs/export", perm: "billing:export"},
	{method: "GET", path: "/api/admin/transactions", perm: "billing:view"},
	{method: "GET", path: "/api/admin/wallets", perm: "billing:view"},
	{method: "GET", prefix: "/api/admin/wallets/", perm: "billing:view"},
	{method: "POST", prefix: "/api/admin/wallets/", suffix: "/adjust", perm: "billing:refund"},
	{method: "PUT", prefix: "/api/admin/wallets/", suffix: "/warning-threshold", perm: "billing:view"},

	// ── plan 套餐管理 ──
	{method: "GET", path: "/api/admin/plans", perm: "plan:view"},
	{method: "GET", path: "/api/admin/plans/export", perm: "plan:view"},
	{method: "GET", prefix: "/api/admin/plans/", perm: "plan:view"},
	{method: "POST", path: "/api/admin/plans", perm: "plan:create"},
	{method: "PUT", prefix: "/api/admin/plans/", perm: "plan:edit"},
	{method: "DELETE", prefix: "/api/admin/plans/", perm: "plan:delete"},

	// ── order 订单管理 ──
	{method: "GET", path: "/api/admin/orders", perm: "order:view"},
	{method: "GET", path: "/api/admin/orders/export", perm: "order:view"},
	{method: "GET", prefix: "/api/admin/orders/", perm: "order:view"},
	{method: "POST", prefix: "/api/admin/orders/", suffix: "/refund", perm: "order:refund"},
	{method: "POST", prefix: "/api/admin/orders/", suffix: "/complete", perm: "order:view"},

	// ── audit 审计日志 ──
	{method: "GET", path: "/api/admin/audit/config", perm: "audit:view"},
	{method: "PUT", path: "/api/admin/audit/config", perm: "audit:view"},
	{method: "GET", path: "/api/admin/audit/operation-logs", perm: "audit:view"},
	{method: "GET", path: "/api/admin/audit/operation-logs/export", perm: "audit:export"},
	{method: "GET", path: "/api/admin/audit/request-logs", perm: "audit:view"},
	{method: "GET", prefix: "/api/admin/audit/request-logs/", perm: "audit:view"},
	{method: "GET", path: "/api/admin/audit/sensitive-logs", perm: "audit:read_sensitive"},
	{method: "GET", path: "/api/admin/audit/content-filter-logs", perm: "audit:view"},

	// ── system 系统设置 ──
	{method: "GET", path: "/api/admin/settings/categories", perm: "system:view"},
	{method: "GET", prefix: "/api/admin/settings/", perm: "system:view"},
	{method: "PUT", prefix: "/api/admin/settings/", perm: "system:edit"},
	// Payment settings & channels (system scope)
	{method: "GET", path: "/api/admin/payment-channels", perm: "system:view"},
	{method: "PUT", prefix: "/api/admin/payment-channels/", perm: "system:edit"},
	{method: "GET", path: "/api/admin/payment-settings", perm: "system:view"},
	{method: "PUT", path: "/api/admin/payment-settings", perm: "system:edit"},

	// ── promo 优惠码管理 ──
	{method: "GET", path: "/api/admin/promo-codes", perm: "promo:view"},
	{method: "GET", path: "/api/admin/promo-codes/export", perm: "promo:view"},
	{method: "GET", prefix: "/api/admin/promo-codes/", perm: "promo:view"},
	{method: "POST", path: "/api/admin/promo-codes", perm: "promo:create"},
	{method: "PUT", prefix: "/api/admin/promo-codes/", perm: "promo:edit"},

	// ── member 成员管理（管理后台维度） ──
	{method: "GET", path: "/api/admin/members", perm: "member:view"},
	{method: "POST", path: "/api/admin/members", perm: "member:import"},
	{method: "PUT", prefix: "/api/admin/members/", suffix: "/disable", perm: "member:view"},
	{method: "PUT", prefix: "/api/admin/members/", suffix: "/enable", perm: "member:view"},
	{method: "PUT", prefix: "/api/admin/members/", suffix: "/reset-password", perm: "member:view"},

	// ── redemption 兑换码管理 ──
	{method: "GET", path: "/api/admin/redemptions", perm: "redemption:view"},
	{method: "GET", path: "/api/admin/redemptions/usages", perm: "redemption:view"},
	{method: "GET", path: "/api/admin/redemptions/export", perm: "redemption:view"},
	{method: "POST", path: "/api/admin/redemptions", perm: "redemption:create"},
	{method: "PUT", prefix: "/api/admin/redemptions/", perm: "redemption:view"},

	// ── permission 权限管理（仅 user 组） ──
	{method: "GET", path: "/api/admin/permissions", perm: "user:view"},
	{method: "GET", prefix: "/api/admin/users/", suffix: "/permissions", perm: "user:view"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/permissions", perm: "user:edit"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/data-scopes", perm: "user:edit"},
}

type permissionRule struct {
	method string // HTTP method
	path   string // exact path match (mutually exclusive with prefix)
	prefix string // path prefix match
	suffix string // path suffix match (combined with prefix)
	perm   string // required permission point
}

// AdminPermissionGuard enforces RBAC permission checks for admin routes.
// Routes without a matching rule remain accessible to all authenticated admins.
func AdminPermissionGuard(r *ghttp.Request) {
	role := GetUserRole(r.Context())
	if role == "super_admin" {
		r.Middleware.Next()
		return
	}

	perm := matchPermission(r.Method, r.URL.Path)
	if perm == "" {
		r.Middleware.Next()
		return
	}

	userID := GetUserID(r.Context())
	if !admin.HasPermission(r.Context(), userID, role, perm) {
		response.ErrorMsg(r, consts.CodeForbidden, "缺少权限："+perm)
		return
	}

	r.Middleware.Next()
}

// matchPermission finds the permission required for the given method and path.
// Matching priority: exact path > prefix+suffix > prefix-only (catch-all).
// This three-phase approach eliminates rule ordering dependencies.
func matchPermission(method, path string) string {
	// Phase 1: exact path match (highest priority)
	for _, rule := range adminPermissionRules {
		if rule.method != method || rule.path == "" {
			continue
		}
		if path == rule.path {
			return rule.perm
		}
	}

	// Phase 2: prefix+suffix match (more specific)
	for _, rule := range adminPermissionRules {
		if rule.method != method || rule.prefix == "" || rule.suffix == "" {
			continue
		}
		if strings.HasPrefix(path, rule.prefix) {
			remainder := path[len(rule.prefix):]
			if strings.HasSuffix(path, rule.suffix) || strings.Contains(remainder, strings.TrimPrefix(rule.suffix, "/")) {
				return rule.perm
			}
		}
	}

	// Phase 3: prefix-only match (catch-all, lowest priority)
	for _, rule := range adminPermissionRules {
		if rule.method != method || rule.prefix == "" || rule.suffix != "" {
			continue
		}
		if strings.HasPrefix(path, rule.prefix) {
			return rule.perm
		}
	}

	return ""
}
