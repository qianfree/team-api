// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

type (
	ITenant interface {
		// ApiKeyList 列出 API Keys，支持按类型过滤
		ApiKeyList(ctx context.Context, req *v1.TenantApiKeyListReq) (*v1.TenantApiKeyListRes, error)
		// ApiKeyCreate 创建新的 API Key
		ApiKeyCreate(ctx context.Context, req *v1.TenantApiKeyCreateReq) (*v1.TenantApiKeyCreateRes, error)
		// ApiKeyDelete 禁用 API Key
		ApiKeyDelete(ctx context.Context, req *v1.TenantApiKeyDeleteReq) (*v1.TenantApiKeyDeleteRes, error)
		// ApiKeyUpdate 更新 API Key 的可编辑字段
		ApiKeyUpdate(ctx context.Context, req *v1.TenantApiKeyUpdateReq) (*v1.TenantApiKeyUpdateRes, error)
		// ApiKeyUpdateScopes 更新 API Key 的模型 scope
		ApiKeyUpdateScopes(ctx context.Context, req *v1.TenantApiKeyUpdateScopesReq) (*v1.TenantApiKeyUpdateScopesRes, error)
		// ApiKeyModelScopes 查询 API Key 的模型范围
		ApiKeyModelScopes(ctx context.Context, req *v1.TenantApiKeyModelScopesReq) (*v1.TenantApiKeyModelScopesRes, error)
		// ExportApiKeys exports the tenant API key list as CSV or Excel.
		ExportApiKeys(ctx context.Context, req *v1.TenantApiKeyExportReq) (*v1.TenantApiKeyExportRes, error)
		// AuditConfigGet returns the tenant's own audit level.
		// 租户审计级别与全局级别完全独立，未设置时默认 masked。
		AuditConfigGet(ctx context.Context, req *v1.TenantAuditConfigGetReq) (*v1.TenantAuditConfigGetRes, error)
		// AuditConfigUpdate sets the audit level for a specific tenant.
		// 租户可独立设置自己的审计级别，不受全局级别约束（双级别存储，各管各的）。
		AuditConfigUpdate(ctx context.Context, req *v1.TenantAuditConfigUpdateReq) (*v1.TenantAuditConfigUpdateRes, error)
		// AuditLogs returns a paginated list of audit logs for the tenant.
		AuditLogs(ctx context.Context, req *v1.TenantAuditLogsReq) (*v1.TenantAuditLogsRes, error)
		// TenantRequestAuditLogs 分页查询租户的请求审计日志（不含 body，性能优先）
		TenantRequestAuditLogs(ctx context.Context, req *v1.TenantRequestAuditLogsReq) (*v1.TenantRequestAuditLogsRes, error)
		// TenantRequestAuditLogDetail 查询单条请求审计日志详情（含 request_body 和 response_body）
		TenantRequestAuditLogDetail(ctx context.Context, req *v1.TenantRequestAuditLogDetailReq) (*v1.TenantRequestAuditLogDetailRes, error)
		// Register handles tenant registration.
		Register(ctx context.Context, req *v1.TenantRegisterReq) (*v1.TenantRegisterRes, error)
		// Login handles tenant user login.
		Login(ctx context.Context, req *v1.TenantLoginReq) (*v1.TenantLoginRes, error)
		// Logout handles tenant user logout.
		Logout(ctx context.Context, req *v1.TenantLogoutReq) (*v1.TenantLogoutRes, error)
		// Refresh handles token refresh for tenant users.
		Refresh(ctx context.Context, req *v1.TenantRefreshReq) (*v1.TenantRefreshRes, error)
		// ChangePassword handles tenant user password change.
		ChangePassword(ctx context.Context, req *v1.TenantChangePasswordReq) (*v1.TenantChangePasswordRes, error)
		// ListSessions returns active sessions for the current tenant user.
		ListSessions(ctx context.Context, req *v1.TenantSessionListReq) (*v1.TenantSessionListRes, error)
		// RevokeSession revokes a specific session.
		RevokeSession(ctx context.Context, req *v1.TenantRevokeSessionReq) (*v1.TenantRevokeSessionRes, error)
		// Wallet 获取租户钱包余额
		Wallet(ctx context.Context, req *v1.TenantWalletReq) (*v1.TenantWalletRes, error)
		// WalletTransactions 获取租户钱包流水
		WalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsReq) (*v1.TenantWalletTransactionsRes, error)
		// BillingRecords 获取租户计费记录
		BillingRecords(ctx context.Context, req *v1.TenantBillingRecordsReq) (*v1.TenantBillingRecordsRes, error)
		// UsageLogs 获取租户用量日志
		UsageLogs(ctx context.Context, req *v1.TenantUsageLogsReq) (*v1.TenantUsageLogsRes, error)
		// ExportUsageLogs exports the tenant usage logs as CSV or Excel.
		ExportUsageLogs(ctx context.Context, req *v1.TenantUsageLogsExportReq) (*v1.TenantUsageLogsExportRes, error)
		// ExportWalletTransactions exports the tenant wallet transactions as CSV or Excel.
		ExportWalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsExportReq) (*v1.TenantWalletTransactionsExportRes, error)
		// Dashboard returns the tenant dashboard statistics.
		Dashboard(ctx context.Context, req *v1.TenantDashboardReq) (*v1.TenantDashboardRes, error)
		// TokenTrends returns daily token usage for the past N days.
		TokenTrends(ctx context.Context, req *v1.TenantTokenTrendsReq) (*v1.TenantTokenTrendsRes, error)
		// ModelDistribution returns the distribution of model usage.
		ModelDistribution(ctx context.Context, req *v1.TenantModelDistributionReq) (*v1.TenantModelDistributionRes, error)
		// BalancePrediction predicts when the balance will be exhausted.
		BalancePrediction(ctx context.Context, req *v1.TenantBalancePredictionReq) (*v1.TenantBalancePredictionRes, error)
		// BudgetAlerts checks member and project budget usage and returns those above 80%.
		BudgetAlerts(ctx context.Context, req *v1.TenantBudgetAlertsReq) (*v1.TenantBudgetAlertsRes, error)
		// GetMemberUsageRanking returns top members by usage cost in a given date range.
		GetMemberUsageRanking(ctx context.Context, req *v1.TenantMemberUsageRankingReq) (*v1.TenantMemberUsageRankingRes, error)
		// SendCode sends a verification code email.
		SendCode(ctx context.Context, req *v1.TenantSendCodeReq) (*v1.TenantSendCodeRes, error)
		// ResetPassword handles password reset.
		ResetPassword(ctx context.Context, req *v1.TenantResetPasswordReq) (*v1.TenantResetPasswordRes, error)
		// ChangeEmail handles email change for a tenant user.
		ChangeEmail(ctx context.Context, req *v1.TenantChangeEmailReq) (*v1.TenantChangeEmailRes, error)
		// CreateFeedback 提交反馈
		CreateFeedback(ctx context.Context, req *v1.FeedbackCreateReq) (*v1.FeedbackCreateRes, error)
		// ListFeedbacks 我的反馈列表
		ListFeedbacks(ctx context.Context, req *v1.FeedbackListReq) (*v1.FeedbackListRes, error)
		// GetFeedback 反馈详情
		GetFeedback(ctx context.Context, req *v1.FeedbackGetReq) (*v1.FeedbackGetRes, error)
		// ListHelpPublicCategories 公开帮助分类列表（树结构）
		ListHelpPublicCategories(ctx context.Context, _ *v1.HelpPublicCategoryListReq) (*v1.HelpPublicCategoryListRes, error)
		// ListHelpPublicArticles 分类下的文章列表
		ListHelpPublicArticles(ctx context.Context, req *v1.HelpPublicArticleListReq) (*v1.HelpPublicArticleListRes, error)
		// GetHelpPublicArticle 文章详情
		GetHelpPublicArticle(ctx context.Context, req *v1.HelpPublicArticleGetReq) (*v1.HelpPublicArticleGetRes, error)
		// SearchHelpPublicArticles 搜索文章
		SearchHelpPublicArticles(ctx context.Context, req *v1.HelpPublicSearchReq) (*v1.HelpPublicSearchRes, error)
		// InvitationList returns a paginated list of invitation records for the tenant.
		InvitationList(ctx context.Context, req *v1.TenantInvitationListReq) (*v1.TenantInvitationListRes, error)
		// RevokeInvitation revokes a pending invitation by setting used_by_user_id = -1.
		RevokeInvitation(ctx context.Context, req *v1.TenantInvitationRevokeReq) (*v1.TenantInvitationRevokeRes, error)
		// InviteInfo returns public information about an invitation (no auth required).
		InviteInfo(ctx context.Context, req *v1.TenantInviteInfoReq) (*v1.TenantInviteInfoRes, error)
		// RequestClosure 申请关户
		RequestClosure(ctx context.Context, req *v1.TenantRequestClosureReq) (*v1.TenantRequestClosureRes, error)
		// CancelClosure 取消关户
		CancelClosure(ctx context.Context, req *v1.TenantCancelClosureReq) (*v1.TenantCancelClosureRes, error)
		// ListMembers returns a paginated list of tenant members.
		ListMembers(ctx context.Context, req *v1.TenantMemberListReq) (*v1.TenantMemberListRes, error)
		// InviteMember generates an invitation link.
		InviteMember(ctx context.Context, req *v1.TenantMemberInviteReq) (*v1.TenantMemberInviteRes, error)
		// JoinByInvite handles a user joining a tenant via invitation link.
		JoinByInvite(ctx context.Context, req *v1.TenantMemberJoinReq) (*v1.TenantMemberJoinRes, error)
		// CreateMember directly creates a member account within the tenant.
		CreateMember(ctx context.Context, req *v1.TenantMemberCreateReq) (*v1.TenantMemberCreateRes, error)
		// RemoveMember removes a member from the tenant.
		// Revokes all API keys, anonymizes personal data, releases member model scopes.
		RemoveMember(ctx context.Context, req *v1.TenantMemberRemoveReq) (*v1.TenantMemberRemoveRes, error)
		// DisableMember disables a member and revokes all their API keys.
		DisableMember(ctx context.Context, tenantID int64, userID int64) error
		// EnableMember re-enables a member and restores their API keys.
		EnableMember(ctx context.Context, tenantID int64, userID int64) error
		// UpdateMemberRole updates a member's role.
		UpdateMemberRole(ctx context.Context, req *v1.TenantMemberUpdateRoleReq) (*v1.TenantMemberUpdateRoleRes, error)
		// ResetMemberPassword resets a member's password. Only admins can reset other members' passwords.
		ResetMemberPassword(ctx context.Context, req *v1.TenantMemberResetPasswordReq) (*v1.TenantMemberResetPasswordRes, error)
		// GetMember returns a single member's detail.
		GetMember(ctx context.Context, req *v1.TenantMemberGetReq) (*v1.TenantMemberGetRes, error)
		// GetMemberUsage returns usage statistics for a single member.
		GetMemberUsage(ctx context.Context, req *v1.TenantMemberUsageReq) (*v1.TenantMemberUsageRes, error)
		// ListMemberApiKeys returns a paginated list of API keys belonging to a specific member.
		ListMemberApiKeys(ctx context.Context, req *v1.TenantMemberApiKeysReq) (*v1.TenantMemberApiKeysRes, error)
		// ExportMembers exports the tenant member list as CSV or Excel.
		ExportMembers(ctx context.Context, req *v1.TenantMemberExportReq) (*v1.TenantMemberExportRes, error)
		// MemberImport parses CSV content, validates, creates an import record.
		MemberImport(ctx context.Context, req *v1.TenantMemberImportReq) (*v1.TenantMemberImportRes, error)
		// ImportRecords returns a paginated list of import records.
		ImportRecords(ctx context.Context, req *v1.TenantImportRecordsReq) (*v1.TenantImportRecordsRes, error)
		// ImportRecordGet returns the status of a member import.
		ImportRecordGet(ctx context.Context, req *v1.TenantImportRecordGetReq) (*v1.TenantImportRecordGetRes, error)
		// MemberModelScopes returns the model IDs available for a member.
		MemberModelScopes(ctx context.Context, req *v1.TenantMemberModelScopesReq) (*v1.TenantMemberModelScopesRes, error)
		// MemberModelScopesSet sets the available models for a member (full replace).
		MemberModelScopesSet(ctx context.Context, req *v1.TenantMemberModelScopesSetReq) (*v1.TenantMemberModelScopesSetRes, error)
		MemberQuota(ctx context.Context, req *v1.TenantMemberQuotaReq) (*v1.TenantMemberQuotaRes, error)
		MemberQuotaSet(ctx context.Context, req *v1.TenantMemberQuotaSetReq) (*v1.TenantMemberQuotaSetRes, error)
		// ListAvailableModels 获取租户可用的模型列表
		ListAvailableModels(ctx context.Context, req *v1.TenantAvailableModelsReq) (*v1.TenantAvailableModelsRes, error)
		ModelComparison(ctx context.Context, req *v1.ModelComparisonReq) (*v1.ModelComparisonRes, error)
		// Notifications 获取用户的通知列表（个人消息 + 广播消息）
		Notifications(ctx context.Context, req *v1.TenantNotificationsReq) (*v1.TenantNotificationsRes, error)
		// UnreadCount 获取未读消息数量
		UnreadCount(ctx context.Context, req *v1.TenantUnreadCountReq) (*v1.TenantUnreadCountRes, error)
		// MarkRead 标记消息为已读
		MarkRead(ctx context.Context, req *v1.TenantMarkReadReq) (*v1.TenantMarkReadRes, error)
		// MarkAllRead 标记所有未读消息为已读
		MarkAllRead(ctx context.Context, req *v1.TenantMarkAllReadReq) (*v1.TenantMarkAllReadRes, error)
		// DeleteNotification 删除已读的个人消息（广播消息不允许用户删除）
		DeleteNotification(ctx context.Context, req *v1.TenantNotificationDeleteReq) (*v1.TenantNotificationDeleteRes, error)
		// NotificationPreferencesGet 获取合并后的通知偏好（组织 + 用户）
		NotificationPreferencesGet(ctx context.Context, req *v1.TenantNotificationPreferencesGetReq) (*v1.TenantNotificationPreferencesGetRes, error)
		// NotificationPreferencesUpdate 更新通知偏好
		NotificationPreferencesUpdate(ctx context.Context, req *v1.TenantNotificationPreferencesUpdateReq) (*v1.TenantNotificationPreferencesUpdateRes, error)
		// Announcements 获取已发布的公告（未过期）
		Announcements(ctx context.Context, req *v1.TenantAnnouncementsReq) (*v1.TenantAnnouncementsRes, error)
		// ExportNotifications exports the tenant notification list as CSV or Excel.
		ExportNotifications(ctx context.Context, req *v1.TenantNotificationsExportReq) (*v1.TenantNotificationsExportRes, error)
		// GetOAuthAuthorizeURL 获取 OAuth 授权跳转 URL
		GetOAuthAuthorizeURL(ctx context.Context, req *v1.OAuthAuthorizeReq) (*v1.OAuthAuthorizeRes, error)
		// OAuthCallback 处理 OAuth 回调
		OAuthCallback(ctx context.Context, req *v1.OAuthCallbackReq) (*v1.OAuthCallbackRes, error)
		// LinkOAuth 绑定 OAuth 账号
		LinkOAuth(ctx context.Context, req *v1.OAuthLinkReq) (*v1.OAuthLinkRes, error)
		// UnlinkOAuth 解绑 OAuth 账号
		UnlinkOAuth(ctx context.Context, req *v1.OAuthUnlinkReq) (*v1.OAuthUnlinkRes, error)
		// ListOAuthProviders 获取已绑定的 OAuth 供应商列表
		ListOAuthProviders(ctx context.Context, req *v1.OAuthListProvidersReq) (*v1.OAuthListProvidersRes, error)
		OpenAppList(ctx context.Context, req *v1.OpenAppListReq) (*v1.OpenAppListRes, error)
		OpenAppCreate(ctx context.Context, req *v1.OpenAppCreateReq) (*v1.OpenAppCreateRes, error)
		OpenAppUpdate(ctx context.Context, req *v1.OpenAppUpdateReq) (*v1.OpenAppUpdateRes, error)
		OpenAppDelete(ctx context.Context, req *v1.OpenAppDeleteReq) (*v1.OpenAppDeleteRes, error)
		OpenAppResetSecret(ctx context.Context, req *v1.OpenAppResetSecretReq) (*v1.OpenAppResetSecretRes, error)
		OpenAppToggleStatus(ctx context.Context, req *v1.OpenAppToggleStatusReq) (*v1.OpenAppToggleStatusRes, error)
		WebhookConfigList(ctx context.Context, _ *v1.WebhookConfigListReq) (*v1.WebhookConfigListRes, error)
		WebhookConfigCreate(ctx context.Context, req *v1.WebhookConfigCreateReq) (*v1.WebhookConfigCreateRes, error)
		WebhookConfigUpdate(ctx context.Context, req *v1.WebhookConfigUpdateReq) (*v1.WebhookConfigUpdateRes, error)
		WebhookConfigDelete(ctx context.Context, req *v1.WebhookConfigDeleteReq) (*v1.WebhookConfigDeleteRes, error)
		WebhookDeliveryLogs(ctx context.Context, req *v1.WebhookDeliveryLogsReq) (*v1.WebhookDeliveryLogsRes, error)
		WebhookRetry(ctx context.Context, req *v1.WebhookRetryReq) (*v1.WebhookRetryRes, error)
		// OrderList 获取租户订单列表
		OrderList(ctx context.Context, req *v1.TenantOrderListReq) (*v1.TenantOrderListRes, error)
		// OrderDetail 获取订单详情
		OrderDetail(ctx context.Context, req *v1.TenantOrderDetailReq) (*v1.TenantOrderDetailRes, error)
		// OrderCreate 创建订单
		OrderCreate(ctx context.Context, req *v1.TenantOrderCreateReq) (*v1.TenantOrderCreateRes, error)
		// OrderCancel 取消订单
		OrderCancel(ctx context.Context, req *v1.TenantOrderCancelReq) (*v1.TenantOrderCancelRes, error)
		// OrderPay 支付订单
		OrderPay(ctx context.Context, req *v1.TenantOrderPayReq) (*v1.TenantOrderPayRes, error)
		// PaymentInfo 获取租户可用的支付信息（渠道列表、金额选项、折扣）
		PaymentInfo(ctx context.Context, req *v1.TenantPaymentInfoReq) (*v1.TenantPaymentInfoRes, error)
		// ExportOrders exports the tenant order list as CSV or Excel.
		ExportOrders(ctx context.Context, req *v1.TenantOrderExportReq) (*v1.TenantOrderExportRes, error)
		// GetOrgInfo returns tenant organization info.
		GetOrgInfo(ctx context.Context, req *v1.TenantOrgInfoReq) (*v1.TenantOrgInfoRes, error)
		// UpdateOrgInfo updates tenant organization info.
		UpdateOrgInfo(ctx context.Context, req *v1.TenantOrgUpdateReq) (*v1.TenantOrgUpdateRes, error)
		// TransferOwnership transfers tenant ownership to another member.
		TransferOwnership(ctx context.Context, req *v1.TenantOrgTransferReq) (*v1.TenantOrgTransferRes, error)
		// GetProfile returns current user's profile.
		GetProfile(ctx context.Context, req *v1.TenantProfileReq) (*v1.TenantProfileRes, error)
		// UpdateProfile updates current user's profile.
		UpdateProfile(ctx context.Context, req *v1.TenantProfileUpdateReq) (*v1.TenantProfileUpdateRes, error)
		// PersonalDashboard returns the personal dashboard overview for the current user.
		PersonalDashboard(ctx context.Context, req *v1.PersonalDashboardReq) (*v1.PersonalDashboardRes, error)
		// PersonalTokenTrends returns daily token usage trends for the current user.
		PersonalTokenTrends(ctx context.Context, req *v1.PersonalTokenTrendsReq) (*v1.PersonalTokenTrendsRes, error)
		// PersonalModelDistribution returns model usage distribution for the current user.
		PersonalModelDistribution(ctx context.Context, req *v1.PersonalModelDistReq) (*v1.PersonalModelDistRes, error)
		// PersonalApiKeyUsage returns per-API-key usage breakdown for the current user.
		PersonalApiKeyUsage(ctx context.Context, req *v1.PersonalApiKeyUsageReq) (*v1.PersonalApiKeyUsageRes, error)
		// PlanList 获取可购买的套餐列表（仅 active）
		PlanList(ctx context.Context, req *v1.TenantPlanListReq) (*v1.TenantPlanListRes, error)
		// PlanCurrent 获取租户当前套餐
		PlanCurrent(ctx context.Context, req *v1.TenantPlanCurrentReq) (*v1.TenantPlanCurrentRes, error)
		// PlanCancelAutoRenew 取消自动续费
		PlanCancelAutoRenew(ctx context.Context, req *v1.TenantPlanCancelAutoRenewReq) (*v1.TenantPlanCancelAutoRenewRes, error)
		PlaygroundChat(ctx context.Context, req *v1.PlaygroundChatReq) (*v1.PlaygroundChatRes, error)
		PlaygroundImage(ctx context.Context, req *v1.PlaygroundImageReq) (*v1.PlaygroundImageRes, error)
		PlaygroundAudioTTS(ctx context.Context, req *v1.PlaygroundAudioTTSReq) (*v1.PlaygroundAudioTTSRes, error)
		PlaygroundEmbedding(ctx context.Context, req *v1.PlaygroundEmbeddingReq) (*v1.PlaygroundEmbeddingRes, error)
		PlaygroundRerank(ctx context.Context, req *v1.PlaygroundRerankReq) (*v1.PlaygroundRerankRes, error)
		SandboxChat(ctx context.Context, req *v1.SandboxChatReq) (*v1.SandboxChatRes, error)
		SandboxQuota(ctx context.Context, req *v1.SandboxQuotaReq) (*v1.SandboxQuotaRes, error)
		TenantPluginList(ctx context.Context, req *v1.TenantPluginListReq) (*v1.TenantPluginListRes, error)
		TenantPluginDetail(ctx context.Context, req *v1.TenantPluginDetailReq) (*v1.TenantPluginDetailRes, error)
		TenantPluginConfigUpdate(ctx context.Context, req *v1.TenantPluginConfigUpdateReq) (*v1.TenantPluginConfigUpdateRes, error)
		TenantPluginEnable(ctx context.Context, req *v1.TenantPluginEnableReq) (*v1.TenantPluginEnableRes, error)
		TenantPluginDisable(ctx context.Context, req *v1.TenantPluginDisableReq) (*v1.TenantPluginDisableRes, error)
		// ProjectList returns a paginated list of projects for a tenant.
		ProjectList(ctx context.Context, req *v1.TenantProjectListReq) (*v1.TenantProjectListRes, error)
		// ProjectCreate creates a new project for a tenant.
		ProjectCreate(ctx context.Context, req *v1.TenantProjectCreateReq) (*v1.TenantProjectCreateRes, error)
		// ProjectUpdate updates a project.
		ProjectUpdate(ctx context.Context, req *v1.TenantProjectUpdateReq) (*v1.TenantProjectUpdateRes, error)
		// ProjectArchive archives a project and revokes all its keys.
		ProjectArchive(ctx context.Context, req *v1.TenantProjectArchiveReq) (*v1.TenantProjectArchiveRes, error)
		// ProjectUnarchive restores an archived project. Keys are NOT auto-restored.
		ProjectUnarchive(ctx context.Context, req *v1.TenantProjectUnarchiveReq) (*v1.TenantProjectUnarchiveRes, error)
		// ProjectGet 根据 ID 获取单个项目详情（含统计摘要）
		ProjectGet(ctx context.Context, req *v1.TenantProjectGetReq) (*v1.TenantProjectGetRes, error)
		// ProjectApiKeyList 获取项目密钥列表（owner/admin 权限）
		ProjectApiKeyList(ctx context.Context, req *v1.TenantProjectApiKeyListReq) (*v1.TenantProjectApiKeyListRes, error)
		// ProjectApiKeyCreate 创建项目密钥（owner/admin 权限）
		ProjectApiKeyCreate(ctx context.Context, req *v1.TenantProjectApiKeyCreateReq) (*v1.TenantProjectApiKeyCreateRes, error)
		// ProjectApiKeyDelete 删除项目密钥（owner/admin 权限）
		ProjectApiKeyDelete(ctx context.Context, req *v1.TenantProjectApiKeyDeleteReq) (*v1.TenantProjectApiKeyDeleteRes, error)
		// ProjectUsageStats 获取项目用量统计（按日汇总，近30天）（owner/admin 权限）
		ProjectUsageStats(ctx context.Context, req *v1.TenantProjectUsageStatsReq) (*v1.TenantProjectUsageStatsRes, error)
		// ProjectUsageLogs 获取项目用量日志（分页）（owner/admin 权限）
		ProjectUsageLogs(ctx context.Context, req *v1.TenantProjectUsageLogsReq) (*v1.TenantProjectUsageLogsRes, error)
		// ValidatePromoCode 校验优惠码并返回折扣金额
		ValidatePromoCode(ctx context.Context, req *v1.TenantValidatePromoCodeReq) (*v1.TenantValidatePromoCodeRes, error)
		// RedeemCode 租户兑换码
		RedeemCode(ctx context.Context, req *v1.TenantRedeemCodeReq) (*v1.TenantRedeemCodeRes, error)
		// ListRedemptionUsages 获取当前租户的兑换历史
		ListRedemptionUsages(ctx context.Context, req *v1.TenantRedemptionUsagesReq) (*v1.TenantRedemptionUsagesRes, error)
		// Verify2FA handles the 2FA verification step during tenant login.
		Verify2FA(ctx context.Context, req *v1.Tenant2FAVerifyReq) (*v1.Tenant2FAVerifyRes, error)
		// Setup2FA starts the 2FA setup process for the current tenant user.
		Setup2FA(ctx context.Context, _ *v1.Tenant2FASetupReq) (*v1.Tenant2FASetupRes, error)
		// Enable2FA confirms and enables 2FA.
		Enable2FA(ctx context.Context, req *v1.Tenant2FAEnableReq) (*v1.Tenant2FAEnableRes, error)
		// Disable2FA disables 2FA for the current tenant user.
		Disable2FA(ctx context.Context, req *v1.Tenant2FADisableReq) (*v1.Tenant2FADisableRes, error)
		// RegenerateBackupCodes generates new backup codes.
		RegenerateBackupCodes(ctx context.Context, req *v1.Tenant2FARegenerateBackupCodesReq) (*v1.Tenant2FARegenerateBackupCodesRes, error)
		// ConfirmHighRisk generates a confirm token for high-risk operations.
		ConfirmHighRisk(ctx context.Context, req *v1.Tenant2FAConfirmReq) (*v1.Tenant2FAConfirmRes, error)
		// LoginHistory returns the login history for tenant users.
		// owner/admin see all members in the tenant; member sees only own records.
		LoginHistory(ctx context.Context, req *v1.TenantLoginHistoryReq) (*v1.TenantLoginHistoryRes, error)
		// GetIPWhitelist returns the tenant's IP whitelist configuration.
		GetIPWhitelist(ctx context.Context, _ *v1.TenantIPWhitelistGetReq) (*v1.TenantIPWhitelistGetRes, error)
		// UpdateIPWhitelist updates the tenant's IP whitelist configuration.
		UpdateIPWhitelist(ctx context.Context, req *v1.TenantIPWhitelistUpdateReq) (*v1.TenantIPWhitelistUpdateRes, error)
		// TicketCreate 创建工单
		TicketCreate(ctx context.Context, req *v1.TenantTicketCreateReq) (*v1.TenantTicketCreateRes, error)
		// TicketList 获取租户工单列表
		TicketList(ctx context.Context, req *v1.TenantTicketListReq) (*v1.TenantTicketListRes, error)
		// TicketGet 获取工单详情（含回复）
		TicketGet(ctx context.Context, req *v1.TenantTicketGetReq) (*v1.TenantTicketGetRes, error)
		// TicketReply 租户用户回复工单
		TicketReply(ctx context.Context, req *v1.TenantTicketReplyReq) (*v1.TenantTicketReplyRes, error)
		// TicketClose 租户用户关闭工单
		TicketClose(ctx context.Context, req *v1.TenantTicketCloseReq) (*v1.TenantTicketCloseRes, error)
		// TicketReopen 租户用户重新打开工单
		TicketReopen(ctx context.Context, req *v1.TenantTicketReopenReq) (*v1.TenantTicketReopenRes, error)
		// ExportTickets exports the tenant ticket list as CSV or Excel.
		ExportTickets(ctx context.Context, req *v1.TenantTicketExportReq) (*v1.TenantTicketExportRes, error)
	}
)

var (
	localTenant ITenant
)

func Tenant() ITenant {
	if localTenant == nil {
		panic("implement not found for interface ITenant, forgot register?")
	}
	return localTenant
}

func RegisterTenant(i ITenant) {
	localTenant = i
}
