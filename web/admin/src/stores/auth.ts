import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import request, { setTokens, clearTokens, onTokenRefreshed, onAuthExpired, setRememberMe, getRememberMe } from '@/utils/request'
import { setAdminSession, clearAdminSession, ADMIN_ROLES } from '@/utils/permission'

export interface AdminUser {
  id: number
  username: string
  display_name: string
  role: string
}

export interface AdminRoleBrief {
  id: number
  code: string
  name: string
  is_enabled: boolean
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
  roles?: AdminRoleBrief[]
  pending_agreements?: PendingAgreement[]
}

const STORE_KEY = 'admin_auth'

export const useAuthStore = defineStore('admin-auth', () => {
  const token = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)
  const expiresAt = ref<number | null>(null)
  const user = ref<AdminUser | null>(null)
  const permissions = ref<string[]>([])
  const roles = ref<AdminRoleBrief[]>([])
  const pendingAgreements = ref<PendingAgreement[]>([])
  const rememberMe = ref<boolean>(getRememberMe())

  const isLoggedIn = computed(() => !!token.value)

  const isSuperAdmin = computed(() => user.value?.role === ADMIN_ROLES.SUPER_ADMIN)

  function persist(): void {
    const data = {
      token: token.value,
      refreshToken: refreshToken.value,
      expiresAt: expiresAt.value,
      user: user.value,
      permissions: permissions.value,
      roles: roles.value,
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
        roles?: AdminRoleBrief[]
      }
      token.value = data.token
      refreshToken.value = data.refreshToken
      expiresAt.value = data.expiresAt
      user.value = data.user
      permissions.value = data.permissions ?? []
      roles.value = data.roles ?? []
      // 恢复 permission.ts 读取的镜像副本：hasPermission() 走 localStorage，
      // 只恢复 Pinia 而不同步这份镜像，会让刷新后的按钮级权限全部失效
      setAdminSession(data.user?.role ?? '', permissions.value)
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
    roles.value = loginRes.roles ?? []

    setTokens({
      accessToken: loginRes.access_token,
      refreshToken: loginRes.refresh_token,
      expiresAt: loginRes.expires_at,
    })
    setAdminSession(loginRes.user.role, loginRes.permissions ?? [])
    persist()
  }

  async function login(username: string, password: string, captcha?: { captchaKey: string; captchaX: number }, remember: boolean = true): Promise<any> {
    // Save user's "remember me" preference
    rememberMe.value = remember
    setRememberMe(remember)

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

  /** 同步清空全部登录数据（响应式状态 + token storage + session 持久化），不调用后端 */
  function clearAuthState(): void {
    token.value = null
    refreshToken.value = null
    expiresAt.value = null
    user.value = null
    permissions.value = []
    roles.value = []
    pendingAgreements.value = []

    clearTokens()
    clearAdminSession()
    localStorage.removeItem(STORE_KEY)
  }

  /** 同步本地登出（仅清前端，不通知后端），用于路由守卫等需要立即生效的场景 */
  function logoutLocal(): void {
    clearAuthState()
  }

  async function logout(): Promise<void> {
    try {
      await request.post('/admin/auth/logout')
    } catch {
      // best-effort
    }
    clearAuthState()
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

  /**
   * 从服务端拉取当前用户的最新权限。
   *
   * 权限此前只在登录响应里下发一次，之后由 localStorage 恢复：管理员改了某人的权限，
   * 对方不重新登录就一直按旧权限渲染菜单。在应用启动与 token 刷新后调用本方法即可跟上。
   *
   * 失败时保持现有权限不变 —— 后端鉴权以服务端为准，前端权限只影响展示，
   * 一次网络抖动不该把用户的菜单清空。
   */
  async function fetchMe(): Promise<void> {
    if (!token.value) return
    try {
      const { data } = await request.get('/admin/auth/me')
      const me = data?.data
      if (!me) return
      user.value = {
        id: me.id,
        username: me.username,
        display_name: me.display_name,
        role: me.role,
      }
      permissions.value = me.permissions ?? []
      roles.value = me.roles ?? []
      setAdminSession(me.role, permissions.value)
      persist()
    } catch {
      // 保持现有权限
    }
  }

  // Sync Pinia store when Axios interceptor refreshes tokens
  onTokenRefreshed((tokens) => {
    token.value = tokens.accessToken
    refreshToken.value = tokens.refreshToken
    expiresAt.value = tokens.expiresAt
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
    user,
    permissions,
    roles,
    pendingAgreements,
    rememberMe,
    isLoggedIn,
    isSuperAdmin,
    login,
    applyTokensFrom2FA,
    logout,
    logoutLocal,
    refreshTokens,
    loadFromStorage,
    fetchMe,
    clearPendingAgreements,
  }
})
