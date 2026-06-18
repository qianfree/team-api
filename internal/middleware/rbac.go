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
// Unmapped admin routes are denied by default in AdminPermissionGuard.
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
	{method: "DELETE", prefix: "/api/admin/channels/", suffix: "/keys/", perm: "channel:edit"},
	{method: "GET", prefix: "/api/admin/channels/", suffix: "/keys", perm: "channel:view"},
	{method: "GET", prefix: "/api/admin/channels/", suffix: "/abilities", perm: "channel:view"},
	{method: "PUT", prefix: "/api/admin/channels/", suffix: "/abilities", perm: "channel:edit"},
	{method: "GET", prefix: "/api/admin/channels/", suffix: "/health_trend", perm: "channel:view"},
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
	{method: "GET", prefix: "/api/admin/models/", suffix: "/pricing", perm: "model:view"},
	{method: "PUT", prefix: "/api/admin/models/", suffix: "/pricing", perm: "model:edit"},
	{method: "GET", prefix: "/api/admin/models/", suffix: "/official-pricing", perm: "model:view"},
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
	{method: "GET", prefix: "/api/admin/model-groups/", suffix: "/models", perm: "model:view"},
	{method: "PUT", prefix: "/api/admin/model-groups/", suffix: "/models", perm: "model:edit"},
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

	// ── operation 内容运营 ──
	{method: "GET", path: "/api/admin/changelogs", perm: "operation:view"},
	{method: "POST", path: "/api/admin/changelogs", perm: "operation:edit"},
	{method: "PUT", prefix: "/api/admin/changelogs/", perm: "operation:edit"},
	{method: "DELETE", prefix: "/api/admin/changelogs/", perm: "operation:edit"},
	{method: "POST", prefix: "/api/admin/changelogs/", suffix: "/publish", perm: "operation:edit"},
	{method: "GET", path: "/api/admin/announcements", perm: "operation:view"},
	{method: "POST", path: "/api/admin/announcements", perm: "operation:edit"},
	{method: "PUT", prefix: "/api/admin/announcements/", perm: "operation:edit"},
	{method: "GET", path: "/api/admin/notification/templates", perm: "operation:view"},
	{method: "GET", prefix: "/api/admin/notification/templates/", perm: "operation:view"},
	{method: "PUT", prefix: "/api/admin/notification/templates/", perm: "operation:edit"},
	{method: "POST", prefix: "/api/admin/notification/templates/", suffix: "/test", perm: "operation:edit"},
	{method: "GET", path: "/api/admin/notification/messages", perm: "operation:view"},
	{method: "POST", path: "/api/admin/notification/messages/send", perm: "operation:edit"},
	{method: "POST", path: "/api/admin/notification/messages/broadcast", perm: "operation:edit"},
	{method: "GET", path: "/api/admin/feedbacks", perm: "support:view"},
	{method: "GET", path: "/api/admin/feedbacks/stats", perm: "support:view"},
	{method: "POST", prefix: "/api/admin/feedbacks/", suffix: "/reply", perm: "support:reply"},
	{method: "PUT", prefix: "/api/admin/feedbacks/", suffix: "/status", perm: "support:edit"},
	{method: "GET", path: "/api/admin/tickets", perm: "support:view"},
	{method: "GET", prefix: "/api/admin/tickets/", perm: "support:view"},
	{method: "PUT", prefix: "/api/admin/tickets/", suffix: "/assign", perm: "support:edit"},
	{method: "POST", prefix: "/api/admin/tickets/", suffix: "/reply", perm: "support:reply"},
	{method: "PUT", prefix: "/api/admin/tickets/", suffix: "/status", perm: "support:edit"},

	// ── help center 帮助中心 ──
	{method: "GET", path: "/api/admin/help-categories", perm: "support:view"},
	{method: "POST", path: "/api/admin/help-categories", perm: "support:edit"},
	{method: "PUT", prefix: "/api/admin/help-categories/", perm: "support:edit"},
	{method: "DELETE", prefix: "/api/admin/help-categories/", perm: "support:edit"},
	{method: "GET", path: "/api/admin/help-articles", perm: "support:view"},
	{method: "GET", prefix: "/api/admin/help-articles/", perm: "support:view"},
	{method: "POST", path: "/api/admin/help-articles", perm: "support:edit"},
	{method: "PUT", prefix: "/api/admin/help-articles/", perm: "support:edit"},
	{method: "DELETE", prefix: "/api/admin/help-articles/", perm: "support:edit"},

	// ── audit 审计日志 ──
	{method: "GET", path: "/api/admin/audit/config", perm: "audit:view"},
	{method: "PUT", path: "/api/admin/audit/config", perm: "audit:view"},
	{method: "GET", path: "/api/admin/audit/operation-logs", perm: "audit:view"},
	{method: "GET", path: "/api/admin/audit/operation-logs/export", perm: "audit:export"},
	{method: "GET", path: "/api/admin/audit/request-logs", perm: "audit:view"},
	{method: "GET", prefix: "/api/admin/audit/request-logs/", perm: "audit:view"},
	{method: "GET", path: "/api/admin/audit/sensitive-logs", perm: "audit:read_sensitive"},
	{method: "GET", path: "/api/admin/audit/content-filter-logs", perm: "audit:view"},

	// ── monitor 监控告警 ──
	{method: "GET", prefix: "/api/admin/monitor/", perm: "monitor:view"},
	{method: "GET", path: "/api/admin/alert/rules", perm: "monitor:view"},
	{method: "GET", path: "/api/admin/alert/options", perm: "monitor:view"},
	{method: "POST", path: "/api/admin/alert/rules", perm: "monitor:edit"},
	{method: "PUT", prefix: "/api/admin/alert/rules/", perm: "monitor:edit"},
	{method: "DELETE", prefix: "/api/admin/alert/rules/", perm: "monitor:edit"},
	{method: "POST", prefix: "/api/admin/alert/rules/", suffix: "/test", perm: "monitor:edit"},
	{method: "GET", path: "/api/admin/alert/events", perm: "monitor:view"},
	{method: "PUT", prefix: "/api/admin/alert/events/", perm: "monitor:edit"},
	{method: "GET", path: "/api/admin/error-logs", perm: "monitor:view"},
	{method: "GET", prefix: "/api/admin/error-logs/", perm: "monitor:view"},
	{method: "PUT", prefix: "/api/admin/error-logs/", perm: "monitor:edit"},
	{method: "PUT", path: "/api/admin/error-logs/batch-resolve", perm: "monitor:edit"},
	{method: "GET", path: "/api/admin/error-logs/stats", perm: "monitor:view"},
	{method: "GET", path: "/api/admin/cron-jobs", perm: "system:view"},
	{method: "POST", prefix: "/api/admin/cron-jobs/", suffix: "/trigger", perm: "system:edit"},

	// ── system 系统设置 ──
	{method: "GET", path: "/api/admin/settings/categories", perm: "system:view"},
	{method: "GET", prefix: "/api/admin/settings/", perm: "system:view"},
	{method: "PUT", prefix: "/api/admin/settings/", perm: "system:edit"},
	// Payment settings & channels (system scope)
	{method: "GET", path: "/api/admin/payment-channels", perm: "system:view"},
	{method: "PUT", prefix: "/api/admin/payment-channels/", perm: "system:edit"},
	{method: "GET", path: "/api/admin/payment-settings", perm: "system:view"},
	{method: "PUT", path: "/api/admin/payment-settings", perm: "system:edit"},
	{method: "GET", prefix: "/api/admin/update/", perm: "system:update"},
	{method: "POST", prefix: "/api/admin/update/", perm: "system:update"},
	{method: "GET", path: "/api/admin/data-governance/settings", perm: "system:view"},
	{method: "PUT", path: "/api/admin/data-governance/settings", perm: "system:edit"},
	{method: "POST", prefix: "/api/admin/data-governance/", perm: "system:edit"},
	{method: "GET", prefix: "/api/admin/plugins", perm: "system:plugin"},
	{method: "POST", prefix: "/api/admin/plugins/", perm: "system:plugin"},
	{method: "PUT", prefix: "/api/admin/plugins/", perm: "system:plugin"},
	{method: "GET", prefix: "/api/admin/email/", perm: "system:view"},

	// ── tenant level config 租户等级配置 ──
	{method: "GET", path: "/api/admin/tenant-level-configs", perm: "tenant:view"},
	{method: "POST", path: "/api/admin/tenant-level-configs", perm: "tenant:create"},
	{method: "PUT", prefix: "/api/admin/tenant-level-configs/", perm: "tenant:edit"},
	{method: "DELETE", prefix: "/api/admin/tenant-level-configs/", perm: "tenant:delete"},

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
	{method: "PUT", prefix: "/api/admin/redemptions/", perm: "redemption:edit"},

	// ── permission 权限管理（仅 user 组） ──
	{method: "GET", path: "/api/admin/permissions", perm: "user:view"},
	{method: "GET", prefix: "/api/admin/users/", suffix: "/permissions", perm: "user:view"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/permissions", perm: "user:edit"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/data-scopes", perm: "user:edit"},

	// ── admin session and security ──
	{method: "POST", path: "/api/admin/auth/logout", perm: "self:access"},
	{method: "GET", path: "/api/admin/auth/sessions", perm: "self:access"},
	{method: "DELETE", prefix: "/api/admin/auth/sessions/user/", perm: "user:edit"},
	{method: "DELETE", prefix: "/api/admin/auth/sessions/", perm: "self:access"},
	{method: "PUT", path: "/api/admin/auth/change-password", perm: "self:access"},
	{method: "POST", prefix: "/api/admin/security/2fa/", perm: "self:access"},
	{method: "GET", path: "/api/admin/security/login-history", perm: "audit:view"},
	{method: "GET", path: "/api/admin/security/tenant-login-history", perm: "audit:view"},
	{method: "GET", prefix: "/api/admin/agreements/", suffix: "/acceptances", perm: "audit:view"},
	{method: "GET", path: "/api/admin/agreements/pending", perm: "self:access"},
	{method: "POST", path: "/api/admin/agreements/accept", perm: "self:access"},
	{method: "GET", path: "/api/admin/agreements", perm: "system:view"},
	{method: "GET", prefix: "/api/admin/agreements/", perm: "system:view"},
	{method: "POST", path: "/api/admin/agreements", perm: "system:edit"},
	{method: "PUT", prefix: "/api/admin/agreements/", perm: "system:edit"},
	{method: "DELETE", prefix: "/api/admin/agreements/", perm: "system:edit"},
	{method: "POST", prefix: "/api/admin/agreements/", suffix: "/publish", perm: "system:edit"},

	// ── dashboard and async task management ──
	{method: "GET", prefix: "/api/admin/dashboard", perm: "dashboard:view"},
	{method: "GET", path: "/api/admin/tasks", perm: "task:view"},
	{method: "GET", prefix: "/api/admin/tasks/", perm: "task:view"},
	{method: "POST", prefix: "/api/admin/tasks/", suffix: "/cancel", perm: "task:edit"},
	{method: "POST", path: "/api/admin/usage-logs/cleanup", perm: "system:edit"},
	{method: "GET", path: "/api/admin/usage-logs/cleanup/tasks", perm: "system:view"},
	{method: "POST", prefix: "/api/admin/usage-logs/cleanup/tasks/", suffix: "/cancel", perm: "system:edit"},
}

type permissionRule struct {
	method string // HTTP method
	path   string // exact path match (mutually exclusive with prefix)
	prefix string // path prefix match
	suffix string // path suffix match (combined with prefix)
	perm   string // required permission point
}

// AdminPermissionGuard enforces RBAC permission checks for admin routes.
// Routes without a matching rule are denied by default.
func AdminPermissionGuard(r *ghttp.Request) {
	role := GetUserRole(r.Context())
	if role == "super_admin" {
		r.Middleware.Next()
		return
	}

	perm := matchPermission(r.Method, r.URL.Path)
	if perm == "" {
		response.ErrorMsg(r, consts.CodeForbidden, "未配置接口权限："+r.URL.Path)
		return
	}

	if perm == "self:access" {
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
	var bestPrefixSuffix *permissionRule
	for _, rule := range adminPermissionRules {
		if rule.method != method || rule.prefix == "" || rule.suffix == "" {
			continue
		}
		if strings.HasPrefix(path, rule.prefix) {
			remainder := path[len(rule.prefix):]
			if strings.HasSuffix(path, rule.suffix) || strings.Contains(remainder, strings.TrimPrefix(rule.suffix, "/")) {
				if bestPrefixSuffix == nil || len(rule.prefix)+len(rule.suffix) > len(bestPrefixSuffix.prefix)+len(bestPrefixSuffix.suffix) {
					r := rule
					bestPrefixSuffix = &r
				}
			}
		}
	}
	if bestPrefixSuffix != nil {
		return bestPrefixSuffix.perm
	}

	// Phase 3: prefix-only match (catch-all, lowest priority)
	var bestPrefixOnly *permissionRule
	for _, rule := range adminPermissionRules {
		if rule.method != method || rule.prefix == "" || rule.suffix != "" {
			continue
		}
		if strings.HasPrefix(path, rule.prefix) {
			if bestPrefixOnly == nil || len(rule.prefix) > len(bestPrefixOnly.prefix) {
				r := rule
				bestPrefixOnly = &r
			}
		}
	}
	if bestPrefixOnly != nil {
		return bestPrefixOnly.perm
	}

	return ""
}
