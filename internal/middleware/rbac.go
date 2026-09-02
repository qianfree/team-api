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
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/unlock", perm: "user:edit"},
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
	{method: "POST", prefix: "/api/admin/channels/", suffix: "/reset-health", perm: "channel:edit"},
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
	{method: "GET", path: "/api/admin/usage-logs/summary", perm: "billing:view"},
	{method: "GET", path: "/api/admin/usage-logs/export", perm: "billing:export"},
	{method: "GET", path: "/api/admin/transactions", perm: "billing:view"},
	{method: "GET", path: "/api/admin/wallets", perm: "billing:view"},
	{method: "GET", prefix: "/api/admin/wallets/", perm: "billing:view"},
	{method: "POST", prefix: "/api/admin/wallets/", suffix: "/adjust", perm: "billing:refund"},
	{method: "POST", prefix: "/api/admin/wallets/", suffix: "/offline-recharge", perm: "billing:refund"},
	{method: "POST", prefix: "/api/admin/wallets/", suffix: "/frozen-items/release", perm: "billing:refund"},
	{method: "POST", prefix: "/api/admin/wallets/", suffix: "/frozen-items/release-all", perm: "billing:refund"},
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
	{method: "GET", prefix: "/api/admin/audit/forwarding-trace/", perm: "audit:view"},
	{method: "GET", path: "/api/admin/audit/sensitive-logs", perm: "audit:read_sensitive"},
	{method: "GET", path: "/api/admin/audit/content-filter-logs", perm: "audit:view"},
	{method: "DELETE", path: "/api/admin/audit/content-filter-logs/clear", perm: "audit:clear"},

	// ── file 文件管理 ──
	{method: "GET", path: "/api/admin/files", perm: "file:view"},
	{method: "GET", path: "/api/admin/files/stats", perm: "file:view"},
	{method: "POST", path: "/api/admin/files/cleanup", perm: "file:cleanup"},
	{method: "GET", prefix: "/api/admin/files/", suffix: "/download", perm: "file:view"},
	{method: "GET", prefix: "/api/admin/files/", suffix: "/serve", perm: "file:view"},
	{method: "DELETE", prefix: "/api/admin/files/", perm: "file:delete"},

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
	{method: "DELETE", path: "/api/admin/alert/events/clear", perm: "monitor:edit"},
	{method: "GET", path: "/api/admin/error-logs", perm: "monitor:view"},
	{method: "GET", prefix: "/api/admin/error-logs/", perm: "monitor:view"},
	{method: "PUT", prefix: "/api/admin/error-logs/", perm: "monitor:edit"},
	{method: "PUT", path: "/api/admin/error-logs/batch-resolve", perm: "monitor:edit"},
	{method: "DELETE", path: "/api/admin/error-logs/clear", perm: "monitor:edit"},
	{method: "DELETE", path: "/api/admin/monitor/channel-errors/clear", perm: "monitor:edit"},
	{method: "GET", path: "/api/admin/error-logs/stats", perm: "monitor:view"},
	{method: "GET", path: "/api/admin/cron-jobs", perm: "system:view"},
	{method: "POST", prefix: "/api/admin/cron-jobs/", suffix: "/trigger", perm: "system:edit"},

	// ── system 系统设置 ──
	{method: "GET", path: "/api/admin/settings/categories", perm: "system:view"},
	{method: "GET", prefix: "/api/admin/settings/", perm: "system:view"},
	{method: "PUT", prefix: "/api/admin/settings/", perm: "system:edit"},
	// 配置连通性测试（对象存储 / 邮件）：会真实调用外部服务并可能发信，按写权限管控
	{method: "POST", prefix: "/api/admin/settings/", perm: "system:edit"},
	// Payment settings & channels (system scope)
	{method: "GET", path: "/api/admin/payment-channels", perm: "system:view"},
	{method: "PUT", prefix: "/api/admin/payment-channels/", perm: "system:edit"},
	{method: "GET", path: "/api/admin/payment-settings", perm: "system:view"},
	{method: "PUT", path: "/api/admin/payment-settings", perm: "system:edit"},
	{method: "GET", path: "/api/admin/data-governance/settings", perm: "system:view"},
	{method: "PUT", path: "/api/admin/data-governance/settings", perm: "system:edit"},
	{method: "POST", prefix: "/api/admin/data-governance/", perm: "system:edit"},
	{method: "GET", prefix: "/api/admin/plugins", perm: "system:plugin"},
	{method: "POST", prefix: "/api/admin/plugins/", perm: "system:plugin"},
	{method: "PUT", prefix: "/api/admin/plugins/", perm: "system:plugin"},
	// 邮件发送记录是运营查看通知触达效果的手段，与通知模板同属内容运营，不归系统设置
	{method: "GET", prefix: "/api/admin/email/", perm: "operation:view"},

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
	{method: "PUT", prefix: "/api/admin/members/", suffix: "/unlock", perm: "member:view"},

	// ── redemption 兑换码管理 ──
	{method: "GET", path: "/api/admin/redemptions", perm: "redemption:view"},
	{method: "GET", path: "/api/admin/redemptions/usages", perm: "redemption:view"},
	{method: "GET", path: "/api/admin/redemptions/export", perm: "redemption:view"},
	{method: "POST", path: "/api/admin/redemptions", perm: "redemption:create"},
	{method: "PUT", prefix: "/api/admin/redemptions/", perm: "redemption:edit"},

	// ── role 角色管理（复用 user 组权限点） ──
	// 账号管理与角色管理是同一件事的两面（谁能进后台、能干什么），拆成两组权限没有实际场景：
	// 「能建角色但不能分配给人」毫无意义，「能分配角色但不能建角色」用禁用角色即可表达。
	{method: "GET", path: "/api/admin/roles", perm: "user:view"},
	{method: "POST", path: "/api/admin/roles", perm: "user:create"},
	{method: "POST", prefix: "/api/admin/roles/", suffix: "/reset", perm: "user:edit"},
	{method: "GET", prefix: "/api/admin/roles/", perm: "user:view"},
	{method: "PUT", prefix: "/api/admin/roles/", suffix: "/status", perm: "user:edit"},
	{method: "PUT", prefix: "/api/admin/roles/", perm: "user:edit"},
	{method: "DELETE", prefix: "/api/admin/roles/", perm: "user:delete"},
	{method: "GET", prefix: "/api/admin/users/", suffix: "/roles", perm: "user:view"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/roles", perm: "user:edit"},

	// ── permission 权限管理（仅 user 组） ──
	{method: "GET", path: "/api/admin/permissions", perm: "user:view"},
	{method: "GET", prefix: "/api/admin/users/", suffix: "/permissions", perm: "user:view"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/permissions", perm: "user:edit"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/data-scopes", perm: "user:edit"},

	// ── admin session and security ──
	{method: "POST", path: "/api/admin/auth/logout", perm: "self:access"},
	// 当前用户信息与有效权限：登录即可访问自己的资料，不需要任何业务权限点
	{method: "GET", path: "/api/admin/auth/me", perm: "self:access"},
	// 会话管理列出并处置的是【全部管理员】的登录态，属于账号管理（user 域）的操作能力：
	// 按用户管理模块的操作档位（user:edit）授权，只读档位不可见。
	// 超管会话的额外保护在 logic 层（assertCanOperateSessions）：仅账号本人可操作。
	{method: "GET", path: "/api/admin/auth/sessions", perm: "user:edit"},
	{method: "DELETE", prefix: "/api/admin/auth/sessions/", perm: "user:edit"},
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
	// 工作台读接口只要求 dashboard:view：跨域条目由 logic 层按 channel:view / billing:view /
	// support:view 等逐条过滤，进得来不等于看得到，权限收敛在数据侧而非路由侧。
	// 工作台没有写接口 —— 待办由源表实时派生，唯一的处置方式是去源头解决。
	{method: "GET", path: "/api/admin/workbench/summary", perm: "dashboard:view"},
	{method: "GET", path: "/api/admin/workbench/badges", perm: "dashboard:view"},
	{method: "GET", path: "/api/admin/tasks", perm: "task:view"},
	{method: "GET", prefix: "/api/admin/tasks/", perm: "task:view"},
	{method: "POST", prefix: "/api/admin/tasks/", suffix: "/cancel", perm: "task:edit"},
	{method: "POST", path: "/api/admin/usage-logs/cleanup", perm: "system:edit"},
	{method: "GET", path: "/api/admin/usage-logs/cleanup/tasks", perm: "system:view"},
	{method: "POST", prefix: "/api/admin/usage-logs/cleanup/tasks/", suffix: "/cancel", perm: "system:edit"},
}

// superAdminOnlyRules 列出仅超级管理员可访问的路由。
//
// 角色管理不走「权限点授权」而是硬性限定超管，理由是它是权限体系的元操作：
// 谁能改角色，谁就能决定所有人的权限。一旦把它下放出去（哪怕只给 user:edit），
// 被下放者就能给自己挂上更高权限的角色——权限体系当场失效。
//
// 这条限制让提权路径根本不存在，而不是靠「只能授予自己已有的权限」这类兜底去堵。
// 普通账号管理（创建/禁用/改密/删除）不受影响，仍按 user:* 授权，可以正常下放。
var superAdminOnlyRules = []permissionRule{
	// 角色的增删改查与启停
	{method: "GET", path: "/api/admin/roles"},
	{method: "POST", path: "/api/admin/roles"},
	{method: "GET", prefix: "/api/admin/roles/"},
	{method: "POST", prefix: "/api/admin/roles/"},
	{method: "PUT", prefix: "/api/admin/roles/"},
	{method: "DELETE", prefix: "/api/admin/roles/"},
	// 给账号分配角色 —— 这是最直接的提权入口
	{method: "GET", prefix: "/api/admin/users/", suffix: "/roles"},
	{method: "PUT", prefix: "/api/admin/users/", suffix: "/roles"},

	// 在线自更新（版本状态 / 检查 / 执行 / 回滚）：替换平台二进制并重启服务，
	// 等同于在服务器上执行代码，是比角色管理更根本的平台级元操作，不下放。
	// 版本检查（GET）看似只读，但前端每次刷新都会请求，若走权限点授权，
	// 未授权角色会持续收到 403 —— 硬性限定超管后非超管前端不再发起请求。
	{method: "GET", prefix: "/api/admin/update/"},
	{method: "POST", prefix: "/api/admin/update/"},
}

// isSuperAdminOnly 判断路由是否仅限超级管理员。
// 任一规则命中即为真，不需要优先级判定。
func isSuperAdminOnly(method, path string) bool {
	for _, rule := range superAdminOnlyRules {
		if rule.method != method {
			continue
		}
		if rule.path != "" && path == rule.path {
			return true
		}
		if rule.prefix != "" && strings.HasPrefix(path, rule.prefix) {
			if rule.suffix == "" || suffixMatches(path, rule.suffix) {
				return true
			}
		}
	}
	return false
}

type permissionRule struct {
	method string // HTTP method
	path   string // exact path match (mutually exclusive with prefix)
	prefix string // path prefix match
	suffix string // path suffix match (combined with prefix)
	perm   string // required permission point
}

// AdminPermissionGuard enforces RBAC permission checks for admin routes.
//
// 策略：默认拒绝 —— 未匹配到权限规则的接口一律 403。
//
// 早期为降低上线回归风险曾采用「未匹配即放行」，那在只有 super_admin / admin 两种可信
// 身份时无害；引入运营、技术支持等低权限角色后，放行等同于越权：任何漏配规则的接口对
// 所有角色都是敞开的。改为默认拒绝后，遗漏会立刻表现为 403 而不是静默的权限缺口。
//
// 兜底保证：super_admin 在规则匹配之前短路放行，因此即便规则有遗漏也不会把系统锁死，
// 超管始终可以进入并修复。规则的完整性由 rbac_coverage_test.go 在 CI 阶段保证。
func AdminPermissionGuard(r *ghttp.Request) {
	if isAdminPublicPath(r.URL.Path) {
		r.Middleware.Next()
		return
	}

	role := GetUserRole(r.Context())
	if role == "super_admin" {
		r.Middleware.Next()
		return
	}

	// 角色管理与在线自更新是平台级元操作，硬性限定超管，不参与权限点授权
	if isSuperAdminOnly(r.Method, r.URL.Path) {
		response.ErrorMsg(r, consts.CodeForbidden, "该操作仅超级管理员可用")
		return
	}

	perm := matchPermission(r.Method, r.URL.Path)
	if perm == "" {
		// 默认拒绝：未配置权限规则的接口不放行（super_admin 已在上方短路）
		response.ErrorMsg(r, consts.CodeForbidden, "接口未配置权限规则，请联系管理员")
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
		if strings.HasPrefix(path, rule.prefix) && suffixMatches(path, rule.suffix) {
			if bestPrefixSuffix == nil || len(rule.prefix)+len(rule.suffix) > len(bestPrefixSuffix.prefix)+len(bestPrefixSuffix.suffix) {
				r := rule
				bestPrefixSuffix = &r
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

// suffixMatches 判断 path 是否按【路径分段边界】匹配规则后缀 suffix，避免子串误匹配。
// 旧实现用 strings.Contains(remainder, "test") 会把 /.../latest 误判为命中 /test 规则
// （"latest" 里含 "test"），从而套用错误的权限点。
//   - suffix 以 "/" 结尾（如 "/keys/"，用于匹配 /.../keys/{id} 这类带尾段的路由）：
//     要求 path 中出现完整的 "/keys/" 片段。
//   - suffix 不以 "/" 结尾（如 "/status"、"/test"）：要求 path 以该后缀结尾，
//     或该后缀后紧跟一个 "/"（即作为完整分段出现，如 /.../test/xxx）。
func suffixMatches(path, suffix string) bool {
	if strings.HasSuffix(suffix, "/") {
		return strings.Contains(path, suffix)
	}
	return strings.HasSuffix(path, suffix) || strings.Contains(path, suffix+"/")
}
