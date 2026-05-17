// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

type (
	IAdmin interface {
		// ListUsers returns a paginated list of admin users.
		ListUsers(ctx context.Context, req *v1.AdminUserListReq) (*v1.AdminUserListRes, error)
		// CreateUser creates a new admin user.
		CreateUser(ctx context.Context, req *v1.AdminUserCreateReq) (*v1.AdminUserCreateRes, error)
		// UpdateUser updates an admin user.
		UpdateUser(ctx context.Context, req *v1.AdminUserUpdateReq) (*v1.AdminUserUpdateRes, error)
		// DeleteUser deletes an admin user.
		DeleteUser(ctx context.Context, req *v1.AdminUserDeleteReq) (*v1.AdminUserDeleteRes, error)
		// UpdateUserStatus enables or disables an admin user.
		UpdateUserStatus(ctx context.Context, req *v1.AdminUserUpdateStatusReq) (*v1.AdminUserUpdateStatusRes, error)
		// ResetUserPassword resets an admin user's password.
		ResetUserPassword(ctx context.Context, req *v1.AdminUserResetPasswordReq) (*v1.AdminUserResetPasswordRes, error)
		// ExportUsers exports admin users to CSV or Excel.
		ExportUsers(ctx context.Context, req *v1.AdminUserExportReq) (*v1.AdminUserExportRes, error)
		// GetAuditConfig retrieves the global audit level from sys_options.
		GetAuditConfig(ctx context.Context, _ *v1.AuditConfigGetReq) (*v1.AuditConfigGetRes, error)
		// UpdateAuditConfig updates the global audit level.
		UpdateAuditConfig(ctx context.Context, req *v1.AuditConfigUpdateReq) (*v1.AuditConfigUpdateRes, error)
		// ListOperationLogs retrieves a paginated list of operation logs with optional filters.
		ListOperationLogs(ctx context.Context, req *v1.OperationLogListReq) (*v1.OperationLogListRes, error)
		// ListSensitiveAccessLogs retrieves a paginated list of sensitive data access logs.
		ListSensitiveAccessLogs(ctx context.Context, req *v1.SensitiveLogListReq) (*v1.SensitiveLogListRes, error)
		// ListRequestAuditLogs 分页查询请求审计日志（不返回 request_body/response_body 以优化性能）
		ListRequestAuditLogs(ctx context.Context, req *v1.RequestAuditLogListReq) (*v1.RequestAuditLogListRes, error)
		// GetRequestAuditLogDetail 查询单条请求审计日志详情（含完整 request_body 和 response_body）
		GetRequestAuditLogDetail(ctx context.Context, req *v1.RequestAuditLogDetailReq) (*v1.RequestAuditLogDetailRes, error)
		// ExportOperationLogs exports operation logs to CSV or Excel.
		ExportOperationLogs(ctx context.Context, req *v1.OperationLogExportReq) (*v1.OperationLogExportRes, error)
		// ContentFilterLogList returns a paginated list of content filter interception logs.
		ContentFilterLogList(ctx context.Context, req *v1.ContentFilterLogListReq) (*v1.ContentFilterLogListRes, error)
		// Login handles admin login.
		Login(ctx context.Context, req *v1.AdminLoginReq) (*v1.AdminLoginRes, error)
		// Logout handles admin logout.
		Logout(ctx context.Context, _ *v1.AdminLogoutReq) (*v1.AdminLogoutRes, error)
		// Refresh handles token refresh.
		Refresh(ctx context.Context, req *v1.AdminRefreshReq) (*v1.AdminRefreshRes, error)
		// ListSessions returns active sessions for the current admin user.
		ListSessions(ctx context.Context, req *v1.AdminSessionListReq) (*v1.AdminSessionListRes, error)
		// RevokeSession revokes a specific session.
		RevokeSession(ctx context.Context, req *v1.AdminRevokeSessionReq) (*v1.AdminRevokeSessionRes, error)
		// ForceLogout revokes all sessions for a specific user.
		ForceLogout(ctx context.Context, req *v1.AdminForceLogoutReq) (*v1.AdminForceLogoutRes, error)
		// ChangePassword handles admin password change.
		ChangePassword(ctx context.Context, req *v1.AdminChangePasswordReq) (*v1.AdminChangePasswordRes, error)
		// CreateChangelog 创建更新日志
		CreateChangelog(ctx context.Context, req *v1.ChangelogCreateReq) (*v1.ChangelogCreateRes, error)
		// ListChangelogs 更新日志列表（管理后台，含草稿）
		ListChangelogs(ctx context.Context, req *v1.ChangelogListReq) (*v1.ChangelogListRes, error)
		// UpdateChangelog 更新更新日志
		UpdateChangelog(ctx context.Context, req *v1.ChangelogUpdateReq) (*v1.ChangelogUpdateRes, error)
		// DeleteChangelog 删除更新日志
		DeleteChangelog(ctx context.Context, req *v1.ChangelogDeleteReq) (*v1.ChangelogDeleteRes, error)
		// PublishChangelog 发布更新日志
		PublishChangelog(ctx context.Context, req *v1.ChangelogPublishReq) (*v1.ChangelogPublishRes, error)
		// ListChannels 获取渠道列表
		ListChannels(ctx context.Context, req *v1.ChannelListReq) (*v1.ChannelListRes, error)
		// CloneChannel 克隆渠道
		CloneChannel(ctx context.Context, req *v1.ChannelCloneReq) (*v1.ChannelCloneRes, error)
		// CreateChannel 创建渠道
		CreateChannel(ctx context.Context, req *v1.ChannelCreateReq) (*v1.ChannelCreateRes, error)
		// UpdateChannel 更新渠道
		UpdateChannel(ctx context.Context, req *v1.ChannelUpdateReq) (*v1.ChannelUpdateRes, error)
		// DeleteChannel 删除渠道
		DeleteChannel(ctx context.Context, req *v1.ChannelDeleteReq) (*v1.ChannelDeleteRes, error)
		// GetChannelDetail 获取渠道详情
		GetChannelDetail(ctx context.Context, req *v1.ChannelDetailReq) (*v1.ChannelDetailRes, error)
		// AddChannelKey 添加渠道 API Key（已废弃：每渠道仅支持一个 Key）
		AddChannelKey(ctx context.Context, req *v1.ChannelKeyCreateReq) (*v1.ChannelKeyCreateRes, error)
		// DeleteChannelKey 删除渠道 API Key
		DeleteChannelKey(ctx context.Context, req *v1.ChannelKeyDeleteReq) (*v1.ChannelKeyDeleteRes, error)
		// SetChannelAbilities 设置渠道模型能力
		SetChannelAbilities(ctx context.Context, req *v1.ChannelAbilityBatchReq) (*v1.ChannelAbilityBatchRes, error)
		// GetChannelKeys 获取渠道 Key 列表
		GetChannelKeys(ctx context.Context, req *v1.ChannelKeyListReq) (*v1.ChannelKeyListRes, error)
		// GetChannelAbilities 获取渠道模型能力列表
		GetChannelAbilities(ctx context.Context, req *v1.ChannelAbilitiesGetReq) (*v1.ChannelAbilitiesGetRes, error)
		// GetProviderDefaultURLs 获取供应商默认 API 地址
		GetProviderDefaultURLs(ctx context.Context, _ *v1.ProviderDefaultURLReq) (*v1.ProviderDefaultURLRes, error)
		// GetChannelHealthTrend 获取渠道健康趋势数据
		GetChannelHealthTrend(ctx context.Context, req *v1.ChannelHealthTrendReq) (*v1.ChannelHealthTrendRes, error)
		// ExportChannels exports channel list to CSV or Excel.
		ExportChannels(ctx context.Context, req *v1.ChannelExportReq) (*v1.ChannelExportRes, error)
		// ChannelOAuthAuthURL 生成 OAuth 授权链接
		ChannelOAuthAuthURL(ctx context.Context, req *v1.ChannelOAuthAuthURLReq) (*v1.ChannelOAuthAuthURLRes, error)
		// ChannelOAuthExchange OAuth 授权码换取令牌
		ChannelOAuthExchange(ctx context.Context, req *v1.ChannelOAuthExchangeReq) (*v1.ChannelOAuthExchangeRes, error)
		// ChannelOAuthRefresh 手动刷新 OAuth 令牌
		ChannelOAuthRefresh(ctx context.Context, req *v1.ChannelOAuthRefreshReq) (*v1.ChannelOAuthRefreshRes, error)
		// TestChannel 测试渠道可用性（发送最小请求验证）
		TestChannel(ctx context.Context, req *v1.ChannelTestReq) (*v1.ChannelTestRes, error)
		// CronJobList returns all registered cron jobs with their last execution status.
		CronJobList(ctx context.Context, _ *v1.CronJobListReq) (*v1.CronJobListRes, error)
		// CronJobExecutions returns paginated execution history for a specific job.
		CronJobExecutions(ctx context.Context, req *v1.CronJobExecutionsReq) (*v1.CronJobExecutionsRes, error)
		// CronJobTrigger manually triggers a cron job.
		CronJobTrigger(ctx context.Context, req *v1.CronJobTriggerReq) (*v1.CronJobTriggerRes, error)
		// GetDashboardStats 获取管理后台仪表盘统计
		GetDashboardStats(ctx context.Context, req *v1.AdminDashboardReq) (*v1.AdminDashboardRes, error)
		// GetDashboardTrends returns daily revenue and request trends for the past N days.
		GetDashboardTrends(ctx context.Context, req *v1.AdminDashboardTrendsReq) (*v1.AdminDashboardTrendsRes, error)
		// GetTopTenants returns the top 10 tenants by revenue.
		GetTopTenants(ctx context.Context, req *v1.AdminDashboardTopTenantsReq) (*v1.AdminDashboardTopTenantsRes, error)
		// GetModelDistribution returns the model usage distribution.
		GetModelDistribution(ctx context.Context, req *v1.AdminDashboardModelDistributionReq) (*v1.AdminDashboardModelDistributionRes, error)
		// GetAllUsageLogs 获取所有租户的用量日志（管理后台）
		GetAllUsageLogs(ctx context.Context, req *v1.AdminUsageLogListReq) (*v1.AdminUsageLogListRes, error)
		// GetAllBillingRecords 获取所有计费记录（管理后台）
		GetAllBillingRecords(ctx context.Context, req *v1.AdminBillingRecordListReq) (*v1.AdminBillingRecordListRes, error)
		// GetTenantWallets 获取所有租户钱包（管理后台）
		GetTenantWallets(ctx context.Context, req *v1.AdminWalletListReq) (*v1.AdminWalletListRes, error)
		// AdjustBalance 调整租户余额（管理后台）
		AdjustBalance(ctx context.Context, req *v1.AdminWalletAdjustReq) (*v1.AdminWalletAdjustRes, error)
		// GetWalletInfo 获取租户钱包信息（管理后台）
		GetWalletInfo(ctx context.Context, req *v1.AdminWalletInfoReq) (*v1.AdminWalletInfoRes, error)
		// GetWalletTransactions 获取租户钱包交易流水（管理后台）
		GetWalletTransactions(ctx context.Context, req *v1.AdminWalletTransactionListReq) (*v1.AdminWalletTransactionListRes, error)
		// SetWarningThreshold 设置租户钱包预警阈值（管理后台）
		SetWarningThreshold(ctx context.Context, req *v1.AdminWalletSetWarningThresholdReq) (*v1.AdminWalletSetWarningThresholdRes, error)
		// GetDashboardChannelHealth 获取渠道健康概览（最不健康的5个活跃渠道）
		GetDashboardChannelHealth(ctx context.Context, req *v1.AdminDashboardChannelHealthReq) (*v1.AdminDashboardChannelHealthRes, error)
		// GetDashboardRecentAlerts 获取最近5条告警
		GetDashboardRecentAlerts(ctx context.Context, req *v1.AdminDashboardRecentAlertsReq) (*v1.AdminDashboardRecentAlertsRes, error)
		// ExportUsageLogs exports usage logs to CSV or Excel.
		ExportUsageLogs(ctx context.Context, req *v1.AdminUsageLogExportReq) (*v1.AdminUsageLogExportRes, error)
		// ExportBillingRecords exports billing records to CSV or Excel.
		ExportBillingRecords(ctx context.Context, req *v1.AdminBillingRecordExportReq) (*v1.AdminBillingRecordExportRes, error)
		DataGovernanceSettingsGet(ctx context.Context, _ *v1.DataGovernanceSettingsGetReq) (*v1.DataGovernanceSettingsGetRes, error)
		// UpdateDataGovernanceSettings 更新数据治理设置
		DataGovernanceSettingsUpdate(ctx context.Context, req *v1.DataGovernanceSettingsUpdateReq) (*v1.DataGovernanceSettingsUpdateRes, error)
		// RequestDataExport 请求数据导出
		DataGovernanceExport(ctx context.Context, req *v1.DataGovernanceExportReq) (*v1.DataGovernanceExportRes, error)
		// RequestDataDeletion 请求数据删除
		DataGovernanceDeletion(ctx context.Context, req *v1.DataGovernanceDeletionReq) (*v1.DataGovernanceDeletionRes, error)
		// TriggerDataCleanup 手动触发数据清理
		DataGovernanceCleanup(ctx context.Context, _ *v1.DataGovernanceCleanupReq) (*v1.DataGovernanceCleanupRes, error)
		// ErrorLogList returns a paginated list of system error logs.
		ErrorLogList(ctx context.Context, req *v1.ErrorLogListReq) (*v1.ErrorLogListRes, error)
		// ErrorLogDetail returns the detail of a single error log.
		ErrorLogDetail(ctx context.Context, req *v1.ErrorLogDetailReq) (*v1.ErrorLogDetailRes, error)
		// ErrorLogResolve marks an error log as resolved.
		ErrorLogResolve(ctx context.Context, req *v1.ErrorLogResolveReq) (*v1.ErrorLogResolveRes, error)
		// ErrorLogBatchResolve marks multiple error logs as resolved.
		ErrorLogBatchResolve(ctx context.Context, req *v1.ErrorLogBatchResolveReq) (*v1.ErrorLogBatchResolveRes, error)
		// ErrorLogStats returns error log statistics.
		ErrorLogStats(ctx context.Context, _ *v1.ErrorLogStatsReq) (*v1.ErrorLogStatsRes, error)
		// ListAllFeedbacks 管理后台反馈列表
		ListAllFeedbacks(ctx context.Context, req *v1.FeedbackListAllReq) (*v1.FeedbackListAllRes, error)
		// ReplyToFeedback 管理员回复反馈
		ReplyToFeedback(ctx context.Context, req *v1.FeedbackReplyReq) (*v1.FeedbackReplyRes, error)
		// UpdateFeedbackStatus 更新反馈状态
		UpdateFeedbackStatus(ctx context.Context, req *v1.FeedbackUpdateStatusReq) (*v1.FeedbackUpdateStatusRes, error)
		// GetFeedbackStats 反馈统计
		GetFeedbackStats(ctx context.Context, req *v1.FeedbackStatsReq) (*v1.FeedbackStatsRes, error)
		// CreateHelpCategory 创建帮助分类
		CreateHelpCategory(ctx context.Context, req *v1.HelpCategoryCreateReq) (*v1.HelpCategoryCreateRes, error)
		// UpdateHelpCategory 更新帮助分类
		UpdateHelpCategory(ctx context.Context, req *v1.HelpCategoryUpdateReq) (*v1.HelpCategoryUpdateRes, error)
		// DeleteHelpCategory 删除帮助分类
		DeleteHelpCategory(ctx context.Context, req *v1.HelpCategoryDeleteReq) (*v1.HelpCategoryDeleteRes, error)
		// ListHelpCategories 帮助分类列表（管理后台）
		ListHelpCategories(ctx context.Context, req *v1.HelpCategoryListReq) (*v1.HelpCategoryListRes, error)
		// CreateHelpArticle 创建帮助文章
		CreateHelpArticle(ctx context.Context, req *v1.HelpArticleCreateReq) (*v1.HelpArticleCreateRes, error)
		// UpdateHelpArticle 更新帮助文章
		UpdateHelpArticle(ctx context.Context, req *v1.HelpArticleUpdateReq) (*v1.HelpArticleUpdateRes, error)
		// DeleteHelpArticle 删除帮助文章
		DeleteHelpArticle(ctx context.Context, req *v1.HelpArticleDeleteReq) (*v1.HelpArticleDeleteRes, error)
		// ListHelpArticles 帮助文章列表（管理后台）
		ListHelpArticles(ctx context.Context, req *v1.HelpArticleListReq) (*v1.HelpArticleListRes, error)
		// GetHelpArticle 帮助文章详情（管理后台）
		GetHelpArticle(ctx context.Context, req *v1.HelpArticleGetReq) (*v1.HelpArticleGetRes, error)
		// CreateMember adds a new member to a specified tenant.
		CreateMember(ctx context.Context, req *v1.AdminMemberCreateReq) (*v1.AdminMemberCreateRes, error)
		// ListAllMembers returns a paginated list of all tenant members across all tenants.
		ListAllMembers(ctx context.Context, req *v1.AdminMemberListReq) (*v1.AdminMemberListRes, error)
		// DisableMember disables a tenant member by admin.
		DisableMember(ctx context.Context, req *v1.AdminMemberDisableReq) (*v1.AdminMemberDisableRes, error)
		// EnableMember re-enables a tenant member by admin.
		EnableMember(ctx context.Context, req *v1.AdminMemberEnableReq) (*v1.AdminMemberEnableRes, error)
		// ResetMemberPassword resets a member's password by admin, returns the new random password.
		ResetMemberPassword(ctx context.Context, req *v1.AdminMemberResetPasswordReq) (*v1.AdminMemberResetPasswordRes, error)
		// ExportMembers exports member list to CSV or Excel.
		ExportMembers(ctx context.Context, req *v1.AdminMemberExportReq) (*v1.AdminMemberExportRes, error)
		// ListModels 获取模型列表
		ListModels(ctx context.Context, req *v1.ModelListReq) (*v1.ModelListRes, error)
		// CreateModel 创建模型（自动创建默认 token 定价记录）
		CreateModel(ctx context.Context, req *v1.ModelCreateReq) (*v1.ModelCreateRes, error)
		// UpdateModel 更新模型（含弃用状态管理）
		UpdateModel(ctx context.Context, req *v1.ModelUpdateReq) (*v1.ModelUpdateRes, error)
		// DeleteModel 删除模型（同时删除定价记录）
		DeleteModel(ctx context.Context, req *v1.ModelDeleteReq) (*v1.ModelDeleteRes, error)
		// ListModelPricing 模型定价列表（模型定价页面专用）
		ListModelPricing(ctx context.Context, req *v1.PricingListReq) (*v1.PricingListRes, error)
		// GetModelPricing 获取模型定价
		GetModelPricing(ctx context.Context, req *v1.PricingGetReq) (*v1.PricingGetRes, error)
		// SetModelPricing 设置模型定价（全量替换）
		SetModelPricing(ctx context.Context, req *v1.PricingSetReq) (*v1.PricingSetRes, error)
		// FetchOfficialPricing 拉取模型官方定价（来自 LiteLLM + models.dev 双数据源）
		FetchOfficialPricing(ctx context.Context, req *v1.PricingFetchOfficialReq) (*v1.PricingFetchOfficialRes, error)
		// FetchOfficialModelInfo 按模型名称拉取官方模型信息（上下文长度+能力特性）
		FetchOfficialModelInfo(ctx context.Context, req *v1.ModelFetchOfficialInfoReq) (*v1.ModelFetchOfficialInfoRes, error)
		// ExportModels exports model list to CSV or Excel.
		ExportModels(ctx context.Context, req *v1.ModelExportReq) (*v1.ModelExportRes, error)
		// ListTemplates 获取通知模板列表（分页）
		ListTemplates(ctx context.Context, req *v1.TemplateListReq) (*v1.TemplateListRes, error)
		// GetTemplate 获取单个通知模板
		GetTemplate(ctx context.Context, req *v1.TemplateGetReq) (*v1.TemplateGetRes, error)
		// UpdateTemplate 更新通知模板
		UpdateTemplate(ctx context.Context, req *v1.TemplateUpdateReq) (*v1.TemplateUpdateRes, error)
		// TestTemplate 用测试变量渲染模板，返回渲染结果（不发送）
		TestTemplate(ctx context.Context, req *v1.TemplateTestReq) (*v1.TemplateTestRes, error)
		// SendMessage 创建手动站内消息并推送 WebSocket 通知
		SendMessage(ctx context.Context, req *v1.MessageSendReq) (*v1.MessageSendRes, error)
		// SendBroadcast 创建广播消息并推送 WebSocket 通知
		SendBroadcast(ctx context.Context, req *v1.MessageBroadcastReq) (*v1.MessageBroadcastRes, error)
		// ListMessages 获取所有消息列表（管理后台，支持过滤）
		ListMessages(ctx context.Context, req *v1.MessageListReq) (*v1.MessageListRes, error)
		// CreateAnnouncement 创建公告
		CreateAnnouncement(ctx context.Context, req *v1.AnnouncementCreateReq) (*v1.AnnouncementCreateRes, error)
		// UpdateAnnouncement 更新公告
		UpdateAnnouncement(ctx context.Context, req *v1.AnnouncementUpdateReq) (*v1.AnnouncementUpdateRes, error)
		// ListAnnouncements 获取公告列表（分页）
		ListAnnouncements(ctx context.Context, req *v1.AnnouncementListReq) (*v1.AnnouncementListRes, error)
		// PublishAnnouncement 发布公告
		PublishAnnouncement(ctx context.Context, req *v1.AnnouncementPublishReq) (*v1.AnnouncementPublishRes, error)
		// ArchiveAnnouncement 归档公告
		ArchiveAnnouncement(ctx context.Context, req *v1.AnnouncementArchiveReq) (*v1.AnnouncementArchiveRes, error)
		// ListOrders 获取全部订单列表
		ListOrders(ctx context.Context, req *v1.OrderListReq) (*v1.OrderListRes, error)
		// GetOrder 获取订单详情
		GetOrder(ctx context.Context, req *v1.OrderDetailReq) (*v1.OrderDetailRes, error)
		// RefundOrder 发起退款
		RefundOrder(ctx context.Context, req *v1.OrderRefundReq) (*v1.OrderRefundRes, error)
		// OrderComplete 手动完成订单
		OrderComplete(ctx context.Context, req *v1.OrderCompleteReq) (*v1.OrderCompleteRes, error)
		// GetPaymentChannels 获取支付渠道配置
		GetPaymentChannels(ctx context.Context, _ *v1.PaymentChannelListReq) (*v1.PaymentChannelListRes, error)
		// CreatePaymentChannel 创建支付渠道（默认禁用）。
		CreatePaymentChannel(ctx context.Context, req *v1.PaymentChannelCreateReq) (*v1.PaymentChannelCreateRes, error)
		// GetPaymentChannel 获取单个支付渠道详情。
		GetPaymentChannel(ctx context.Context, req *v1.PaymentChannelDetailReq) (*v1.PaymentChannelDetailRes, error)
		// UpdatePaymentChannel 更新支付渠道配置
		UpdatePaymentChannel(ctx context.Context, req *v1.PaymentChannelUpdateReq) (*v1.PaymentChannelUpdateRes, error)
		// DeletePaymentChannel 删除支付渠道。
		DeletePaymentChannel(ctx context.Context, req *v1.PaymentChannelDeleteReq) (*v1.PaymentChannelDeleteRes, error)
		// TogglePaymentChannel 切换支付渠道启用/禁用状态。
		TogglePaymentChannel(ctx context.Context, req *v1.PaymentChannelToggleReq) (*v1.PaymentChannelToggleRes, error)
		// GetPaymentSettings 获取全局支付设置。
		GetPaymentSettings(ctx context.Context, _ *v1.PaymentSettingsGetReq) (*v1.PaymentSettingsGetRes, error)
		// UpdatePaymentSettings 更新全局支付设置。
		UpdatePaymentSettings(ctx context.Context, req *v1.PaymentSettingsUpdateReq) (*v1.PaymentSettingsUpdateRes, error)
		// ExportOrders exports order list to CSV or Excel.
		ExportOrders(ctx context.Context, req *v1.OrderExportReq) (*v1.OrderExportRes, error)
		// GetUserPermissions returns permission points and data scopes for an admin user.
		GetUserPermissions(ctx context.Context, req *v1.AdminPermissionListReq) (*v1.AdminPermissionListRes, error)
		// UpdateUserPermissions updates permission points for an admin user.
		UpdateUserPermissions(ctx context.Context, req *v1.AdminPermissionUpdateReq) (*v1.AdminPermissionUpdateRes, error)
		// UpdateUserDataScopes updates data scopes for an admin user.
		UpdateUserDataScopes(ctx context.Context, req *v1.AdminDataScopeUpdateReq) (*v1.AdminDataScopeUpdateRes, error)
		// GetAllPermissions returns all predefined permission groups.
		GetAllPermissions(ctx context.Context, _ *v1.AdminAllPermissionsReq) (*v1.AdminAllPermissionsRes, error)
		// ListPlans 获取套餐列表
		ListPlans(ctx context.Context, req *v1.PlanListReq) (*v1.PlanListRes, error)
		// GetPlan 获取套餐详情
		GetPlan(ctx context.Context, req *v1.PlanDetailReq) (*v1.PlanDetailRes, error)
		// CreatePlan 创建套餐
		CreatePlan(ctx context.Context, req *v1.PlanCreateReq) (*v1.PlanCreateRes, error)
		// UpdatePlan 更新套餐
		UpdatePlan(ctx context.Context, req *v1.PlanUpdateReq) (*v1.PlanUpdateRes, error)
		// ArchivePlan 下架套餐（软删除）
		ArchivePlan(ctx context.Context, req *v1.PlanArchiveReq) (*v1.PlanArchiveRes, error)
		// ToggleRecommend 切换推荐标记
		ToggleRecommend(ctx context.Context, req *v1.PlanToggleRecommendReq) (*v1.PlanToggleRecommendRes, error)
		// ExportPlans exports plan list to CSV or Excel.
		ExportPlans(ctx context.Context, req *v1.PlanExportReq) (*v1.PlanExportRes, error)
		PluginList(ctx context.Context, req *v1.PluginListReq) (*v1.PluginListRes, error)
		PluginDetail(ctx context.Context, req *v1.PluginDetailReq) (*v1.PluginDetailRes, error)
		PluginInstall(ctx context.Context, req *v1.PluginInstallReq) (*v1.PluginInstallRes, error)
		PluginEnable(ctx context.Context, req *v1.PluginEnableReq) (*v1.PluginEnableRes, error)
		PluginDisable(ctx context.Context, req *v1.PluginDisableReq) (*v1.PluginDisableRes, error)
		PluginUninstall(ctx context.Context, req *v1.PluginUninstallReq) (*v1.PluginUninstallRes, error)
		PluginUpgrade(ctx context.Context, req *v1.PluginUpgradeReq) (*v1.PluginUpgradeRes, error)
		PluginConfigUpdate(ctx context.Context, req *v1.PluginConfigUpdateReq) (*v1.PluginConfigUpdateRes, error)
		PluginConfigSchema(ctx context.Context, req *v1.PluginConfigSchemaReq) (*v1.PluginConfigSchemaRes, error)
		// ListPromoCodes 获取优惠码列表
		ListPromoCodes(ctx context.Context, req *v1.PromoCodeListReq) (*v1.PromoCodeListRes, error)
		// CreatePromoCode 创建优惠码
		CreatePromoCode(ctx context.Context, req *v1.PromoCodeCreateReq) (*v1.PromoCodeCreateRes, error)
		// UpdatePromoCode 更新优惠码
		UpdatePromoCode(ctx context.Context, req *v1.PromoCodeUpdateReq) (*v1.PromoCodeUpdateRes, error)
		// GetPromoCodeUsages 获取优惠码使用记录
		GetPromoCodeUsages(ctx context.Context, req *v1.PromoCodeUsagesReq) (*v1.PromoCodeUsagesRes, error)
		// ExportPromoCodes exports promo code list to CSV or Excel.
		ExportPromoCodes(ctx context.Context, req *v1.PromoCodeExportReq) (*v1.PromoCodeExportRes, error)
		// ListRedemptions 获取兑换码列表
		ListRedemptions(ctx context.Context, req *v1.RedemptionListReq) (*v1.RedemptionListRes, error)
		// BatchCreateRedemptions 批量生成兑换码
		BatchCreateRedemptions(ctx context.Context, req *v1.RedemptionCreateReq) (*v1.RedemptionCreateRes, error)
		// DisableRedemption 禁用兑换码
		DisableRedemption(ctx context.Context, req *v1.RedemptionDisableReq) (*v1.RedemptionDisableRes, error)
		// ListRedemptionUsages 获取兑换码使用记录
		ListRedemptionUsages(ctx context.Context, req *v1.RedemptionUsagesReq) (*v1.RedemptionUsagesRes, error)
		// ExportRedemptions exports redemption list to CSV or Excel.
		ExportRedemptions(ctx context.Context, req *v1.RedemptionExportReq) (*v1.RedemptionExportRes, error)
		// Verify2FA handles the 2FA verification step during admin login.
		Verify2FA(ctx context.Context, req *v1.Admin2FAVerifyReq) (*v1.Admin2FAVerifyRes, error)
		// Setup2FA starts the 2FA setup process for the current admin user.
		Setup2FA(ctx context.Context, _ *v1.Admin2FASetupReq) (*v1.Admin2FASetupRes, error)
		// Enable2FA confirms and enables 2FA after verifying the code.
		Enable2FA(ctx context.Context, req *v1.Admin2FAEnableReq) (*v1.Admin2FAEnableRes, error)
		// Disable2FA disables 2FA for the current admin user.
		Disable2FA(ctx context.Context, req *v1.Admin2FADisableReq) (*v1.Admin2FADisableRes, error)
		// RegenerateBackupCodes generates new backup codes.
		RegenerateBackupCodes(ctx context.Context, req *v1.Admin2FARegenerateBackupCodesReq) (*v1.Admin2FARegenerateBackupCodesRes, error)
		// ConfirmHighRisk generates a confirm token for high-risk operations.
		ConfirmHighRisk(ctx context.Context, req *v1.Admin2FAConfirmReq) (*v1.Admin2FAConfirmRes, error)
		// LoginHistory returns the login history for all admin users with search filters.
		LoginHistory(ctx context.Context, req *v1.AdminLoginHistoryReq) (*v1.AdminLoginHistoryRes, error)
		// TenantLoginHistory returns login history for tenant users (admin view).
		TenantLoginHistory(ctx context.Context, req *v1.AdminTenantLoginHistoryReq) (*v1.AdminTenantLoginHistoryRes, error)
		// GetSettingsCategories returns all available setting categories.
		GetSettingsCategories(ctx context.Context, _ *v1.AdminSettingsCategoriesReq) (*v1.AdminSettingsCategoriesRes, error)
		// GetSettings retrieves settings with schema for a given category.
		GetSettings(ctx context.Context, req *v1.AdminSettingsGetReq) (*v1.AdminSettingsGetRes, error)
		// UpdateSettings batch-updates settings for a given category.
		UpdateSettings(ctx context.Context, req *v1.AdminSettingsUpdateReq) (*v1.AdminSettingsUpdateRes, error)
		// TaskList 大模型异步任务列表
		TaskList(ctx context.Context, req *v1.TaskListReq) (*v1.TaskListRes, error)
		// TaskDetail 大模型异步任务详情
		TaskDetail(ctx context.Context, req *v1.TaskDetailReq) (*v1.TaskDetailRes, error)
		// TaskCancel 取消大模型异步任务
		TaskCancel(ctx context.Context, req *v1.TaskCancelReq) (*v1.TaskCancelRes, error)
		// TenantSelect returns a lightweight paginated tenant list for dropdown selectors.
		TenantSelect(ctx context.Context, req *v1.TenantSelectReq) (*v1.TenantSelectRes, error)
		// CreateTenant creates a new tenant with its owner user and wallet.
		CreateTenant(ctx context.Context, req *v1.TenantCreateReq) (*v1.TenantCreateRes, error)
		// ListTenants returns a paginated list of tenants.
		ListTenants(ctx context.Context, req *v1.TenantListReq) (*v1.TenantListRes, error)
		// GetTenant returns detail of a single tenant.
		GetTenant(ctx context.Context, req *v1.TenantGetReq) (*v1.TenantGetRes, error)
		// UpdateTenantStatus updates a tenant's status.
		UpdateTenantStatus(ctx context.Context, req *v1.TenantUpdateStatusReq) (*v1.TenantUpdateStatusRes, error)
		// UpdateTenant updates tenant information.
		UpdateTenant(ctx context.Context, req *v1.TenantUpdateReq) (*v1.TenantUpdateRes, error)
		// UpdateTenantChannelScope 更新租户默认渠道范围
		UpdateTenantChannelScope(ctx context.Context, req *v1.TenantChannelScopeUpdateReq) (*v1.TenantChannelScopeUpdateRes, error)
		// ExportTenants exports tenant list to CSV or Excel.
		ExportTenants(ctx context.Context, req *v1.TenantExportReq) (*v1.TenantExportRes, error)
		// ListTenantModels 列出租户已分配的模型
		ListTenantModels(ctx context.Context, req *v1.TenantModelListReq) (*v1.TenantModelListRes, error)
		// BatchAssignModels 批量分配模型给租户
		BatchAssignModels(ctx context.Context, req *v1.TenantModelBatchAssignReq) (*v1.TenantModelBatchAssignRes, error)
		// UpdateTenantModel 更新租户模型配置
		UpdateTenantModel(ctx context.Context, req *v1.TenantModelUpdateReq) (*v1.TenantModelUpdateRes, error)
		// DeleteTenantModel 移除租户模型分配
		DeleteTenantModel(ctx context.Context, req *v1.TenantModelDeleteReq) (*v1.TenantModelDeleteRes, error)
		// ListAllTickets 获取全部工单列表（管理后台）
		ListAllTickets(ctx context.Context, req *v1.TicketListReq) (*v1.TicketListRes, error)
		// GetTicketAdmin 获取工单详情（管理后台，含回复）
		GetTicketAdmin(ctx context.Context, req *v1.TicketGetReq) (*v1.TicketGetRes, error)
		// AssignTicket 分配工单给管理员
		AssignTicket(ctx context.Context, req *v1.TicketAssignReq) (*v1.TicketAssignRes, error)
		// ReplyToTicketAdmin 管理员回复工单
		ReplyToTicketAdmin(ctx context.Context, req *v1.TicketReplyReq) (*v1.TicketReplyRes, error)
		// UpdateTicketStatus 更新工单状态
		UpdateTicketStatus(ctx context.Context, req *v1.TicketStatusUpdateReq) (*v1.TicketStatusUpdateRes, error)
		UsageLogCleanupCreate(ctx context.Context, req *v1.UsageLogCleanupCreateReq) (*v1.UsageLogCleanupCreateRes, error)
		UsageLogCleanupList(ctx context.Context, req *v1.UsageLogCleanupListReq) (*v1.UsageLogCleanupListRes, error)
		UsageLogCleanupCancel(ctx context.Context, req *v1.UsageLogCleanupCancelReq) (*v1.UsageLogCleanupCancelRes, error)
	}
)

var (
	localAdmin IAdmin
)

func Admin() IAdmin {
	if localAdmin == nil {
		panic("implement not found for interface IAdmin, forgot register?")
	}
	return localAdmin
}

func RegisterAdmin(i IAdmin) {
	localAdmin = i
}
