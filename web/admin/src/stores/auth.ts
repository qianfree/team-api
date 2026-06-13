import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import request, { setTokens, clearTokens, onTokenRefreshed } from '@/utils/request'
import { setAdminSession, clearAdminSession, ADMIN_ROLES } from '@/utils/permission'

export interface AdminUser {
  id: number
  username: string
  display_name: string
  role: string
}

export interface PendingAgreement {
  id: number
  code: string
  title: string
  version: string
}

interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_at: number
  user: AdminUser
  permissions: string[]
  pending_agreements?: PendingAgreement[]
}

const STORE_KEY = 'admin_auth'

export const useAuthStore = defineStore('admin-auth', () => {
  const token = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)
  const expiresAt = ref<number | null>(null)
  const user = ref<AdminUser | null>(null)
  const permissions = ref<string[]>([])
  const pendingAgreements = ref<PendingAgreement[]>([])

  const isLoggedIn = computed(() => !!token.value)

  const isSuperAdmin = computed(() => user.value?.role === ADMIN_ROLES.SUPER_ADMIN)

  function persist(): void {
    const data = {
      token: token.value,
      refreshToken: refreshToken.value,
      expiresAt: expiresAt.value,
      user: user.value,
      permissions: permissions.value,
    }
    localStorage.setItem(STORE_KEY, JSON.stringify(data))
  }

  function hydrate(): void {
    try {
      const raw = localStorage.getItem(STORE_KEY)
      if (!raw) return
      const data = JSON.parse(raw) as {
        token: string | null
        refreshToken: string | null
        expiresAt: number | null
        user: AdminUser | null
        permissions: string[]
      }
      token.value = data.token
      refreshToken.value = data.refreshToken
      expiresAt.value = data.expiresAt
      user.value = data.user
      permissions.value = data.permissions ?? []
    } catch {
      // corrupted data — ignore
    }
  }

  function applySession(loginRes: LoginResponse): void {
    token.value = loginRes.access_token
    refreshToken.value = loginRes.refresh_token
    expiresAt.value = loginRes.expires_at
    user.value = loginRes.user
    permissions.value = loginRes.permissions ?? []

    setTokens({
      accessToken: loginRes.access_token,
      refreshToken: loginRes.refresh_token,
      expiresAt: loginRes.expires_at,
    })
    setAdminSession(loginRes.user.role, loginRes.permissions ?? [])
    persist()
  }

  async function login(username: string, password: string, captcha?: { captchaKey: string; captchaX: number }): Promise<any> {
    const { data } = await request.post('/admin/auth/login', {
      username,
      password,
      captcha_key: captcha?.captchaKey,
      captcha_x: captcha?.captchaX,
    })
    if (data.data?.totp_required) {
      return data.data
    }
    pendingAgreements.value = data.data?.pending_agreements || []
    applySession(data.data)
    return data.data
  }

  function applyTokensFrom2FA(loginData: any): void {
    pendingAgreements.value = loginData?.pending_agreements || []
    applySession(loginData)
  }

  async function logout(): Promise<void> {
    try {
      await request.post('/admin/auth/logout')
    } catch {
      // best-effort
    }
    token.value = null
    refreshToken.value = null
    expiresAt.value = null
    user.value = null
    permissions.value = []

    clearTokens()
    clearAdminSession()
    localStorage.removeItem(STORE_KEY)
  }

  async function refreshTokens(): Promise<void> {
    const { data } = await request.post<{ data: LoginResponse }>('/admin/auth/refresh', {
      refresh_token: refreshToken.value,
    })
    applySession(data.data)
  }

  function loadFromStorage(): void {
    hydrate()
  }

  // Sync Pinia store when Axios interceptor refreshes tokens
  onTokenRefreshed((tokens) => {
    token.value = tokens.accessToken
    refreshToken.value = tokens.refreshToken
    expiresAt.value = tokens.expiresAt
    persist()
  })

  function clearPendingAgreements(): void {
    pendingAgreements.value = []
  }

  return {
    token,
    refreshToken,
    expiresAt,
    user,
    permissions,
    pendingAgreements,
    isLoggedIn,
    isSuperAdmin,
    login,
    applyTokensFrom2FA,
    logout,
    refreshTokens,
    loadFromStorage,
    clearPendingAgreements,
  }
})
