import type { RouteRecordRaw } from 'vue-router'

// 'vue-router' 模块类型扩展，让 meta 字段有类型提示
declare module 'vue-router' {
	interface RouteMeta {
		title?: string
		requiresAuth?: boolean
		icon?: string
		sort?: number
		roles?: string[]
	}
}

const tenantRoutes: RouteRecordRaw[] = [
	{
		path: '/tenant/login',
		name: 'TenantLogin',
		component: () => import('@/views/auth/LoginPage.vue'),
		meta: { requiresAuth: false, title: '登录' },
	},
	{
		path: '/tenant/register',
		name: 'TenantRegister',
		component: () => import('@/views/auth/RegisterPage.vue'),
		meta: { requiresAuth: false, title: '注册' },
	},
	{
		path: '/tenant/forgot-password',
		name: 'TenantForgotPassword',
		component: () => import('@/views/auth/ForgotPasswordPage.vue'),
		meta: { requiresAuth: false, title: '找回密码' },
	},
	{
		path: '/tenant/join',
		name: 'TenantJoin',
		component: () => import('@/views/auth/JoinPage.vue'),
		meta: { requiresAuth: false, title: '加入组织' },
	},
	{
		path: '/tenant',
		component: () => import('@/views/Layout.vue'),
		meta: { requiresAuth: true },
		children: [
			// 概览 (10-19)
			{
				path: 'dashboard',
				name: 'TenantDashboard',
				component: () => import('@/views/dashboard/DashboardPage.vue'),
				meta: { title: '仪表盘', icon: 'grid', sort: 10, roles: ['owner', 'admin'] },
			},
			{
				path: '',
				redirect: '/tenant/dashboard',
			},
			{
				path: 'personal-dashboard',
				name: 'TenantPersonalDashboard',
				component: () => import('@/views/dashboard/PersonalDashboardPage.vue'),
				meta: { title: '个人看板', icon: 'user', sort: 15, roles: ['owner', 'admin', 'member'] },
			},
			// 团队 (20-29)
			{
				path: 'members',
				name: 'TenantMembers',
				component: () => import('@/views/team/MembersPage.vue'),
				meta: { title: '成员管理', icon: 'users', sort: 20, roles: ['owner', 'admin'] },
			},
			{
				path: 'members/invitations',
				name: 'TenantInvitations',
				component: () => import('@/views/team/InvitationsPage.vue'),
				meta: { title: '邀请记录', roles: ['owner', 'admin'] },
			},
			{
				path: 'members/:id',
				name: 'TenantMemberDetail',
				component: () => import('@/views/team/MemberDetailPage.vue'),
				meta: { title: '成员详情', roles: ['owner', 'admin'] },
			},
			{
				path: 'projects',
				name: 'TenantProjects',
				component: () => import('@/views/team/ProjectsPage.vue'),
				meta: { title: '项目管理', icon: 'project', sort: 21, roles: ['owner', 'admin'] },
			},
			{
				path: 'projects/:id',
				name: 'TenantProjectDetail',
				component: () => import('@/views/team/ProjectDetailPage.vue'),
				meta: { title: '项目详情', roles: ['owner', 'admin'] },
			},
			// AI 服务 (30-39)
			{
				path: 'models',
				name: 'TenantModels',
				component: () => import('@/views/service/ModelsPage.vue'),
				meta: { title: '可用模型', icon: 'cube', sort: 30, roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'model-comparison',
				name: 'TenantModelComparison',
				component: () => import('@/views/service/ModelComparisonPage.vue'),
				meta: { title: '模型对比', icon: 'chart', sort: 31, roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'api-keys',
				name: 'TenantApiKeys',
				component: () => import('@/views/service/ApiKeysPage.vue'),
				meta: { title: 'API 密钥', icon: 'key', sort: 32, roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'usage-logs',
				name: 'TenantUsageLogs',
				component: () => import('@/views/service/UsageLogsPage.vue'),
				meta: { title: '用量日志', icon: 'chart', sort: 33, roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'request-audit-logs',
				name: 'TenantRequestAuditLogs',
				component: () => import('@/views/service/RequestAuditLogsPage.vue'),
				meta: { title: '请求审计日志', icon: 'clipboard', sort: 34, roles: ['owner', 'admin'] },
			},
			// 财务 (40-49)
			{
				path: 'wallet',
				name: 'TenantWallet',
				component: () => import('@/views/finance/WalletPage.vue'),
				meta: { title: '钱包', icon: 'creditCard', sort: 40, roles: ['owner', 'admin'] },
			},
			{
				path: 'plans',
				name: 'TenantPlans',
				component: () => import('@/views/finance/PlansPage.vue'),
				meta: { title: '套餐方案', icon: 'gift', sort: 41, roles: ['owner', 'admin'] },
			},
			{
				path: 'orders',
				name: 'TenantOrders',
				component: () => import('@/views/finance/OrdersPage.vue'),
				meta: { title: '订单记录', icon: 'document', sort: 42, roles: ['owner', 'admin'] },
			},
			{
				path: 'redeem',
				name: 'TenantRedeem',
				component: () => import('@/views/finance/RedeemPage.vue'),
				meta: { title: '兑换码', icon: 'receipt', sort: 43, roles: ['owner', 'admin'] },
			},
			// 沟通 (50-59)
			{
				path: 'notifications',
				name: 'TenantNotifications',
				component: () => import('@/views/notification/NotificationsPage.vue'),
				meta: { title: '通知中心', roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'notification-preferences',
				name: 'TenantNotificationPreferences',
				component: () => import('@/views/notification/NotificationPreferencesPage.vue'),
				meta: { title: '通知偏好', roles: ['owner', 'admin'] },
			},
			{
				path: 'playground',
				name: 'TenantPlayground',
				component: () => import('@/views/service/PlaygroundPage.vue'),
				meta: { title: '在线体验', icon: 'terminal', sort: 50, roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'tickets',
				name: 'TenantTickets',
				component: () => import('@/views/support/TicketsPage.vue'),
				meta: { title: '工单中心', icon: 'ticket', sort: 51, roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'feedback',
				name: 'TenantFeedback',
				component: () => import('@/views/support/FeedbackPage.vue'),
				meta: { title: '意见反馈', icon: 'chat', sort: 52, roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'help',
				name: 'TenantHelpCenter',
				component: () => import('@/views/help/HelpCenterPage.vue'),
				meta: { title: '帮助中心', icon: 'bookOpen', sort: 53, roles: ['owner', 'admin', 'member'] },
			},
			// 设置 (60-69)
			{
				path: 'organization',
				name: 'TenantOrganization',
				component: () => import('@/views/settings/OrganizationPage.vue'),
				meta: { title: '组织设置', roles: ['owner'] },
			},
			{
				path: 'audit-config',
				name: 'TenantAuditConfig',
				component: () => import('@/views/settings/AuditConfigPage.vue'),
				meta: { title: '审计设置', roles: ['owner', 'admin'] },
			},
			{
				path: 'login-history',
				name: 'TenantLoginHistory',
				component: () => import('@/views/settings/LoginHistoryPage.vue'),
				meta: { title: '登录历史', roles: ['owner', 'admin', 'member'] },
			},
			{
				path: 'profile',
				name: 'TenantProfile',
				component: () => import('@/views/settings/ProfilePage.vue'),
				meta: { title: '个人设置', roles: ['owner', 'admin', 'member'] },
			},
			// 开放平台 (70-79)
			{
				path: 'open-apps',
				name: 'TenantOpenApps',
				component: () => import('@/views/open/OpenAppsPage.vue'),
				meta: { title: '开放平台', icon: 'cog', sort: 70, roles: ['owner', 'admin'] },
			},
			{
				path: 'webhooks',
				name: 'TenantWebhooks',
				component: () => import('@/views/open/WebhooksPage.vue'),
				meta: { title: 'Webhook', icon: 'link', sort: 71, roles: ['owner', 'admin'] },
			},
			// API 文档 (90+)
			{
				path: 'docs',
				name: 'TenantDocs',
				component: () => import('@/views/service/DocsPage.vue'),
				meta: { title: 'API 文档', icon: 'bookOpen', sort: 90, roles: ['owner', 'admin', 'member'] },
			},
		],
	},
]

export default tenantRoutes
