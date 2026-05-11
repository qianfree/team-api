import { computed } from 'vue'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import type { TenantInfo, TenantUser, RegisterPayload } from '@/stores/tenant-auth'

export function useTenantAuth() {
  const store = useTenantAuthStore()

  const isLoggedIn = computed(() => store.isLoggedIn)

  const isOwner = computed(() => store.isOwner)

  const tenant = computed<TenantInfo | null>(() => store.tenant)

  const user = computed<TenantUser | null>(() => store.user)

  async function login(account: string, password: string, type: 'ram' | 'admin', captcha?: { captchaKey: string; captchaX: number }): Promise<void> {
    await store.login(account, password, type, captcha)
  }

  async function register(payload: RegisterPayload): Promise<void> {
    await store.register(payload)
  }

  async function logout(): Promise<void> {
    await store.logout()
    window.location.hash = '#/tenant/login'
  }

  function ensureLoaded(): void {
    if (!store.isLoggedIn && !store.token) {
      store.loadFromStorage()
    }
  }

  return {
    isLoggedIn,
    isOwner,
    tenant,
    user,
    login,
    register,
    logout,
    ensureLoaded,
  }
}
