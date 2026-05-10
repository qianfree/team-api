import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { AdminUser } from '@/stores/auth'

export function useAuth() {
  const store = useAuthStore()

  const isLoggedIn = computed(() => store.isLoggedIn)

  const isSuperAdmin = computed(() => store.isSuperAdmin)

  const user = computed<AdminUser | null>(() => store.user)

  async function login(username: string, password: string): Promise<void> {
    await store.login(username, password)
  }

  async function logout(): Promise<void> {
    await store.logout()
    window.location.hash = '#/admin/login'
  }

  function ensureLoaded(): void {
    if (!store.isLoggedIn && !store.token) {
      store.loadFromStorage()
    }
  }

  return {
    isLoggedIn,
    isSuperAdmin,
    user,
    login,
    logout,
    ensureLoaded,
  }
}
