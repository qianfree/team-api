import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import request, { setTokens, clearTokens, onTokenRefreshed, onAuthExpired, setRememberMe, getRememberMe } from '@/utils/request'
import { setTenantSession, clearTenantSession, TENANT_ROLES } from '@/utils/permission'

export interface TenantInfo {
  id: number
  name: string
  code: string
  team_enabled: boolean
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
  username: string
  password: string
  email: string
  code?: string
  captcha_key?: string
  captcha_x?: number
  // 组织信息可选（留空由后端自动生成），保留以兼容显式传入场景
  tenant_name?: string
  tenant_code?: string
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
  const rememberMe = ref<boolean>(getRememberMe())

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

  async function login(account: string, password: string, type: 'ram' | 'admin', captcha?: { captchaKey: string; captchaX: number; turnstileToken?: string }, remember: boolean = true): Promise<any> {
    // Save user's "remember me" preference
    rememberMe.value = remember
    setRememberMe(remember)

    const { data } = await request.post('/tenant/auth/login', {
      account,
      password,
      type,
      captcha_key: captcha?.captchaKey,
      captcha_x: captcha?.captchaX,
      turnstile_token: captcha?.turnstileToken,
    }, { _suppressErrorMsg: true } as any)
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

  // 拉取最新组织信息，更新 store.tenant（激活团队功能后调用，驱动菜单/页面响应）
  async function refreshOrgInfo(): Promise<void> {
    const { data } = await request.get<{ data: { id: number; name: string; code: string; team_enabled: boolean } }>('/tenant/organization')
    if (tenant.value && data.data) {
      tenant.value = {
        id: data.data.id,
        name: data.data.name,
        code: data.data.code,
        team_enabled: data.data.team_enabled,
      }
      persist()
    }
  }

  /** 同步清空全部登录数据（响应式状态 + token storage + session 持久化），不调用后端 */
  function clearAuthState(): void {
    token.value = null
    refreshToken.value = null
    expiresAt.value = null
    tenant.value = null
    user.value = null
    permissions.value = []
    pendingAgreements.value = []

    clearTokens()
    clearTenantSession()
    localStorage.removeItem(STORE_KEY)
  }

  /** 同步本地登出（仅清前端，不通知后端），用于路由守卫等需要立即生效的场景 */
  function logoutLocal(): void {
    clearAuthState()
  }

  async function logout(): Promise<void> {
    try {
      await request.post('/tenant/auth/logout')
    } catch {
      // best-effort
    }
    clearAuthState()
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

  // Axios 拦截器在 401 认证彻底失效时回调，由 store 清空全部登录数据
  // （必须先清后跳，否则路由守卫会从持久化数据恢复登录态，造成登录页↔系统死循环）
  onAuthExpired(() => {
    clearAuthState()
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
    rememberMe,
    isLoggedIn,
    isOwner,
    login,
    applyTokensFrom2FA,
    register,
    refreshOrgInfo,
    logout,
    logoutLocal,
    refreshTokens,
    loadFromStorage,
    clearPendingAgreements,
  }
})
