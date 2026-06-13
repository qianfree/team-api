import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import request, { setTokens, clearTokens, onTokenRefreshed } from '@/utils/request'
import { setTenantSession, clearTenantSession, TENANT_ROLES } from '@/utils/permission'

export interface TenantInfo {
  id: number
  name: string
  code: string
}

export interface TenantUser {
  id: number
  username: string
  role: string
}

export interface PendingAgreement {
  id: number
  code: string
  title: string
  version: string
}

interface AuthResponse {
  access_token: string
  refresh_token: string
  expires_at: string
  tenant: TenantInfo
  user: TenantUser
  permissions?: string[]
  pending_agreements?: PendingAgreement[]
}

export interface RegisterPayload {
  tenant_name: string
  tenant_code: string
  username: string
  password: string
  email: string
  code?: string
  captcha_key?: string
  captcha_x?: number
}

const STORE_KEY = 'tenant_auth'

export const useTenantAuthStore = defineStore('tenant-auth', () => {
  const token = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)
  const expiresAt = ref<string | null>(null)
  const tenant = ref<TenantInfo | null>(null)
  const user = ref<TenantUser | null>(null)
  const permissions = ref<string[]>([])
  const pendingAgreements = ref<PendingAgreement[]>([])

  const isLoggedIn = computed(() => !!token.value)

  const isOwner = computed(() => user.value?.role === TENANT_ROLES.OWNER)

  function persist(): void {
    try {
      const data = {
        token: token.value,
        refreshToken: refreshToken.value,
        expiresAt: expiresAt.value,
        tenant: tenant.value,
        user: user.value,
        permissions: permissions.value,
      }
      localStorage.setItem(STORE_KEY, JSON.stringify(data))
    } catch {
      // ignore localStorage errors
    }
  }

  function hydrate(): void {
    try {
      const raw = localStorage.getItem(STORE_KEY)
      if (!raw) return
      const data = JSON.parse(raw) as {
        token: string | null
        refreshToken: string | null
        expiresAt: string | null
        tenant: TenantInfo | null
        user: TenantUser | null
        permissions: string[]
      }
      token.value = data.token ?? null
      refreshToken.value = data.refreshToken ?? null
      expiresAt.value = data.expiresAt ?? null
      tenant.value = data.tenant ?? null
      user.value = data.user ?? null
      permissions.value = data.permissions ?? []
    } catch {
      // corrupted data — ignore
    }
  }

  function applySession(res: AuthResponse): void {
    token.value = res.access_token
    refreshToken.value = res.refresh_token
    expiresAt.value = res.expires_at
    tenant.value = res.tenant
    user.value = res.user
    permissions.value = res.permissions ?? []

    setTokens({
      accessToken: res.access_token,
      refreshToken: res.refresh_token,
      expiresAt: res.expires_at,
    })
    setTenantSession(res.user.role, res.permissions ?? [])
    persist()
  }

  async function login(account: string, password: string, type: 'ram' | 'admin', captcha?: { captchaKey: string; captchaX: number; turnstileToken?: string }): Promise<any> {
    const { data } = await request.post('/tenant/auth/login', {
      account,
      password,
      type,
      captcha_key: captcha?.captchaKey,
      captcha_x: captcha?.captchaX,
      turnstile_token: captcha?.turnstileToken,
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

  async function register(payload: RegisterPayload): Promise<any> {
    const { data } = await request.post<{ data: AuthResponse }>('/tenant/auth/register', payload)
    pendingAgreements.value = data.data?.pending_agreements || []
    applySession(data.data)
    return data.data
  }

  async function logout(): Promise<void> {
    try {
      await request.post('/tenant/auth/logout')
    } catch {
      // best-effort
    }
    token.value = null
    refreshToken.value = null
    expiresAt.value = null
    tenant.value = null
    user.value = null
    permissions.value = []

    clearTokens()
    clearTenantSession()
    localStorage.removeItem(STORE_KEY)
  }

  async function refreshTokens(): Promise<void> {
    const { data } = await request.post<{ data: AuthResponse }>('/tenant/auth/refresh', {
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
    expiresAt.value = String(tokens.expiresAt)
    persist()
  })

  function clearPendingAgreements(): void {
    pendingAgreements.value = []
  }

  return {
    token,
    refreshToken,
    expiresAt,
    tenant,
    user,
    permissions,
    pendingAgreements,
    isLoggedIn,
    isOwner,
    login,
    applyTokensFrom2FA,
    register,
    logout,
    refreshTokens,
    loadFromStorage,
    clearPendingAgreements,
  }
})
