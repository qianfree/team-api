import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import adminRoutes from './admin'
import { useAuthStore } from '@/stores/auth'
import { hasPermission, isSuperAdmin } from '@/utils/permission'
import request, { shouldRefresh, getRefreshToken } from '@/utils/request'
import { useSiteName } from '@/composables/useSiteName'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { useTopProgress } from '@/composables/useTopProgress'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/admin/login',
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/admin/login',
  },
  ...adminRoutes,
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

let setupChecked = false
let systemInitialized: boolean | null = null

export function markSystemInitialized(): void {
  setupChecked = true
  systemInitialized = true
}

async function checkSetupStatus(): Promise<boolean> {
  if (setupChecked) return systemInitialized === true
  try {
    const res = await request.get('/setup/status', { timeout: 5000 })
    systemInitialized = res.data?.data?.initialized === true
  } catch {
    // Allow the current navigation, but retry the setup check next time.
    return true
  }
  setupChecked = true
  return systemInitialized === true
}

// 每次页面加载只向服务端同步一次权限：守卫在每次路由跳转都会执行，
// 不加这道闸门会让每次点菜单都多打一个请求。
let permissionsSynced = false

const { siteName, fetchSiteName } = useSiteName()
const { fetchSettings: fetchPublicSettings } = usePublicSettings()
const { start, done } = useTopProgress()

router.beforeEach(async (to) => {
  start()

  const authStore = useAuthStore()
  authStore.loadFromStorage()

  if (to.meta.title) {
    // 站点名与公共配置（本位币/汇率，供全站货币显示）并行拉取，30s TTL
    await Promise.all([fetchSiteName(), fetchPublicSettings()])
    const name = siteName.value || 'Team-API'
    document.title = `${to.meta.title as string} — ${name}`
  }

  if (to.name === 'AdminSetup') {
    return true
  }

  const initialized = await checkSetupStatus()
  if (!initialized) {
    return { name: 'AdminSetup' }
  }

  const requiresAuth = to.meta.requiresAuth !== false
  if (!requiresAuth) {
    return true
  }

  if (!authStore.isLoggedIn) {
    return { name: 'AdminLogin', query: { redirect: to.fullPath } }
  }

  // Token expired and no refresh token available — force login
  if (shouldRefresh() && !getRefreshToken()) {
    // 同步清空登录数据后再跳转，避免 async 登出未完成时登录页把用户又跳回系统
    authStore.logoutLocal()
    return { name: 'AdminLogin', query: { redirect: to.fullPath } }
  }

  // 本次会话首次进入受保护页面时，从服务端刷新一次有效权限。
  // 权限只在登录响应里下发过一次，之后由 localStorage 恢复：管理员改了某人的权限，
  // 对方不重新登录就会一直按旧权限渲染菜单。这里补上刷新。
  // 注意 await：权限到位后再做下面的 meta.perm 判断，否则首屏可能被误判为越权。
  if (!permissionsSynced) {
    permissionsSynced = true
    await authStore.fetchMe()
  }

  // 页面级权限校验。菜单已按权限过滤，这里拦的是直接输 URL 的情况。
  // 接口层本就会 403，但没有这道守卫，用户会先看到一个空白页再看到一片报错。
  if (to.meta.superOnly && !isSuperAdmin()) {
    return { name: 'AdminForbidden' }
  }
  const requiredPerm = to.meta.perm as string | undefined
  if (requiredPerm && !hasPermission(requiredPerm)) {
    return { name: 'AdminForbidden' }
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
