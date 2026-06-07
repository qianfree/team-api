import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import mobileRoutes from './mobile'
import { useAuthStore } from '@/stores/auth'
import { shouldRefresh, getRefreshToken } from '@/utils/request'
import axios from 'axios'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/m/' },
  { path: '/:pathMatch(.*)*', redirect: '/m/' },
  ...mobileRoutes,
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

router.beforeEach(async (to) => {
  const authStore = useAuthStore()
  authStore.loadFromStorage()

  if (to.meta.title) {
    document.title = `${to.meta.title as string} — Team-API`
  }

  if (to.name === 'MobileSetup') return true

  const initialized = await checkSetupStatus()
  if (!initialized) return { name: 'MobileSetup' }

  const requiresAuth = to.meta.requiresAuth !== false
  if (!requiresAuth) return true

  if (!authStore.isLoggedIn) {
    return { name: 'MobileLogin', query: { redirect: to.fullPath } }
  }

  if (shouldRefresh() && !getRefreshToken()) {
    authStore.logout()
    return { name: 'MobileLogin', query: { redirect: to.fullPath } }
  }

  return true
})

export default router
