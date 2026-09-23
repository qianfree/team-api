import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import tenantRoutes from './tenant'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { shouldRefresh, getRefreshToken } from '@/utils/request'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { useTopProgress } from '@/composables/useTopProgress'

const routes: RouteRecordRaw[] = [
	{
		path: '/',
		name: 'TenantHome',
		component: () => import('@/views/landing/LandingPage.vue'),
		meta: {
				requiresAuth: false,
				title: 'Team-API — 团队统一充值、按人分额、用量实时入账 | 企业级多租户 AI 网关',
				description: '开源自托管的企业级多租户 AI 网关：团队统一充值、按成员与项目分配额度、用量实时入账，预算触线自动熔断。完全兼容 OpenAI SDK，一个 Key 接入所有大模型。',
				keywords: 'Team-API, 大模型网关, API Gateway, 多租户, 额度管控, 预算熔断, 团队管理, 用量计费, RBAC, 开源, OpenAI 兼容',
			},
	},
	...tenantRoutes,
	{
		path: '/:pathMatch(.*)*',
		redirect: '/',
	},
]

const router = createRouter({
	history: createWebHashHistory(),
	routes,
})

const { start, done } = useTopProgress()
const { settings: publicSettings, fetchSettings } = usePublicSettings()

router.beforeEach(async (to) => {
	start()

	const tenantAuthStore = useTenantAuthStore()
	tenantAuthStore.loadFromStorage()

	if (to.meta.title) {
		await fetchSettings()
		const siteName = publicSettings.value.site_name || 'Team-API'
		document.title = to.name === 'TenantHome' ? `${siteName} — 团队统一充值、按人分额、用量实时入账 | 企业级多租户 AI 网关` : `${to.meta.title} — ${siteName}`
	}

	// Auth pages — always allow
	if (to.name === 'TenantHome' || to.name === 'TenantLogin' || to.name === 'TenantRegister' || to.name === 'TenantForgotPassword' || to.name === 'TenantJoin') {
		return true
	}

	const requiresAuth = to.meta.requiresAuth !== false

	if (!requiresAuth) {
		return true
	}

	if (!tenantAuthStore.isLoggedIn) {
		return { name: 'TenantLogin', query: { redirect: to.fullPath } }
	}

	// Token expired and no refresh token available — force login
	if (shouldRefresh() && !getRefreshToken()) {
		// 同步清空登录数据后再跳转，避免 async 登出未完成时登录页把用户又跳回系统
		tenantAuthStore.logoutLocal()
		return { name: 'TenantLogin', query: { redirect: to.fullPath } }
	}

	// Role-based access control: if route defines roles, check membership
	const role = tenantAuthStore.user?.role
	const allowedRoles = to.meta.roles

	if (allowedRoles && role && !allowedRoles.includes(role)) {
		// 无权访问时回落到该角色自己的主页：member 回个人看板，
		// 管理层回仪表盘。原先一律弹到「可用模型」，等于让成员永远错过个人看板。
		const isManager = role === 'owner' || role === 'admin'
		return { name: isManager ? 'TenantDashboard' : 'TenantPersonalDashboard' }
	}

	return true
})

router.afterEach(() => {
	done()
})

router.onError(() => {
	done()
})

export default router
