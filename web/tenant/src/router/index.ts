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
				title: 'Team-API — 企业级多租户大模型 API 网关平台 | 开源自托管',
				description: '开源自托管的企业级多租户大模型 API 网关平台，聚合 40+ 供应商，内置计费引擎、团队管理与渠道调度。',
				keywords: 'Team-API, 大模型网关, API Gateway, 多租户, OpenAI, Claude, 开源',
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
		document.title = to.name === 'TenantHome' ? `${siteName} — 企业级多租户大模型 API 网关平台 | 开源自托管` : `${to.meta.title} — ${siteName}`
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
