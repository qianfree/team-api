import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import adminRoutes from './admin'
import { useAuthStore } from '@/stores/auth'
import { shouldRefresh, getRefreshToken } from '@/utils/request'
import { useSiteName } from '@/composables/useSiteName'
import { useTopProgress } from '@/composables/useTopProgress'
import axios from 'axios'

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
    const res = await axios.get('/api/setup/status', { timeout: 5000 })
    systemInitialized = res.data?.data?.initialized === true
  } catch {
    systemInitialized = true
  }
  setupChecked = true
  return systemInitialized === true
}

const { siteName, fetchSiteName } = useSiteName()
const { start, done } = useTopProgress()

router.beforeEach(async (to) => {
  start()

  const authStore = useAuthStore()
  authStore.loadFromStorage()

  if (to.meta.title) {
    fetchSiteName()
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
    authStore.logout()
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
