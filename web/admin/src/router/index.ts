import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import adminRoutes from './admin'
import { useAuthStore } from '@/stores/auth'
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

  return true
})

router.afterEach(() => {
  done()
})

router.onError(() => {
  done()
})

export default router
