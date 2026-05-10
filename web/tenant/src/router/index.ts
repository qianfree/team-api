import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import tenantRoutes from './tenant'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { shouldRefresh, getRefreshToken } from '@/utils/request'

const routes: RouteRecordRaw[] = [
	{
		path: '/',
		name: 'TenantHome',
		component: () => import('@/views/landing/LandingPage.vue'),
		meta: { requiresAuth: false, title: 'Team-API — 统一 AI API 网关' },
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

router.beforeEach((to) => {
	const tenantAuthStore = useTenantAuthStore()
	tenantAuthStore.loadFromStorage()

	if (to.meta.title) {
		document.title = to.name === 'TenantHome' ? to.meta.title : `${to.meta.title} — Team-API`
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
		tenantAuthStore.logout()
		return { name: 'TenantLogin', query: { redirect: to.fullPath } }
	}

	// Role-based access control: if route defines roles, check membership
	const role = tenantAuthStore.user?.role
	const allowedRoles = to.meta.roles

	if (allowedRoles && role && !allowedRoles.includes(role)) {
		// Redirect member to their first accessible page (models)
		return { name: 'TenantModels' }
	}

	return true
})

export default router
