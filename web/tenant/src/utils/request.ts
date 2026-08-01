import axios from 'axios'
import type { AxiosInstance, AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios'
import { toast } from '@/utils/toast'

interface TokenPair {
  accessToken: string
  refreshToken: string
  expiresAt: string | number
}

interface ApiError {
  code: number
  message: string
  request_id: string
}

/** Request config extension — set _suppressErrorMsg to skip auto error toast */
declare module 'axios' {
  interface AxiosRequestConfig {
    _suppressErrorMsg?: boolean
  }
}

const ACCESS_TOKEN_KEY = 'tenant_access_token'
const REFRESH_TOKEN_KEY = 'tenant_refresh_token'
const EXPIRES_AT_KEY = 'tenant_token_expires_at'
const REMEMBER_ME_KEY = 'tenant_remember_me'
const TOKEN_BUFFER_SECONDS = 60

// Dynamic storage selection based on "remember me" preference
function getStorage(): Storage {
  // Check user's "remember me" preference (defaults to true for backward compatibility)
  const rememberMe = localStorage.getItem(REMEMBER_ME_KEY)
  return rememberMe === 'false' ? sessionStorage : localStorage
}

export function setRememberMe(remember: boolean): void {
  // Store the preference itself in localStorage (so it persists across sessions)
  localStorage.setItem(REMEMBER_ME_KEY, String(remember))
}

export function getRememberMe(): boolean {
  const saved = localStorage.getItem(REMEMBER_ME_KEY)
  return saved === null ? true : saved === 'true'
}

export function setTokens(tokens: TokenPair): void {
  const storage = getStorage()
  storage.setItem(ACCESS_TOKEN_KEY, tokens.accessToken)
  storage.setItem(REFRESH_TOKEN_KEY, tokens.refreshToken)
  storage.setItem(EXPIRES_AT_KEY, String(tokens.expiresAt))
}

export function clearTokens(): void {
  // Clear from both storages to prevent leftover data
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
  localStorage.removeItem(EXPIRES_AT_KEY)
  sessionStorage.removeItem(ACCESS_TOKEN_KEY)
  sessionStorage.removeItem(REFRESH_TOKEN_KEY)
  sessionStorage.removeItem(EXPIRES_AT_KEY)
}

export function getAccessToken(): string | null {
  // Try localStorage first (persistent), then sessionStorage (temporary)
  return localStorage.getItem(ACCESS_TOKEN_KEY) || sessionStorage.getItem(ACCESS_TOKEN_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY) || sessionStorage.getItem(REFRESH_TOKEN_KEY)
}

export function getExpiresAt(): number | null {
  const raw = localStorage.getItem(EXPIRES_AT_KEY) || sessionStorage.getItem(EXPIRES_AT_KEY)
  return raw ? Number(raw) : null
}

export function shouldRefresh(): boolean {
  const expiresAt = getExpiresAt()
  if (expiresAt === null) return false  // No token → no need to refresh
  return Date.now() / 1000 + TOKEN_BUFFER_SECONDS >= expiresAt
}

let isRefreshing = false
interface PendingRequest {
  resolve: (token: string) => void
  reject: (error: unknown) => void
}

let pendingRequests: PendingRequest[] = []
let tokenRefreshedCallback: ((tokens: TokenPair) => void) | null = null

export function onTokenRefreshed(cb: (tokens: TokenPair) => void): void {
  tokenRefreshedCallback = cb
}

function replayPending(token: string): void {
  pendingRequests.forEach(({ resolve }) => resolve(token))
  pendingRequests = []
}

function rejectPending(error: unknown): void {
  pendingRequests.forEach(({ reject }) => reject(error))
  pendingRequests = []
}

function waitForRefresh(): Promise<string> {
  return new Promise((resolve, reject) => {
    pendingRequests.push({ resolve, reject })
  })
}

const request: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

async function doRefresh(): Promise<TokenPair> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    throw new Error('No refresh token available')
  }

  const { data } = await axios.post('/api/tenant/auth/refresh', {
    refresh_token: refreshToken,
  })

  const tokenData = data.data
  const newTokens: TokenPair = {
    accessToken: tokenData.access_token,
    refreshToken: tokenData.refresh_token,
    expiresAt: Number(tokenData.expires_at),
  }

  setTokens(newTokens)
  if (tokenRefreshedCallback) {
    tokenRefreshedCallback(newTokens)
  }
  return newTokens
}

const PUBLIC_PATHS = [
	'/settings/',
	'/captcha',
	'/captcha/',
	'/auth/login',
	'/auth/register',
	'/agreements/',
	// 找回密码流程无需登录态：发送验证码 / 重置密码
	'/email/send-code',
	'/email/reset-password',
]

function isPublicPath(url?: string): boolean {
  if (!url) return false
  return PUBLIC_PATHS.some(p => url.includes(p))
}

request.interceptors.request.use(
  async (config: InternalAxiosRequestConfig) => {
    if (!isPublicPath(config.url) && shouldRefresh() && !config.url?.includes('/auth/refresh')) {
      if (isRefreshing) {
        await waitForRefresh()
      } else {
        isRefreshing = true
        try {
          const newTokens = await doRefresh()
          replayPending(newTokens.accessToken)
        } catch (error) {
          rejectPending(error)
          clearTokens()
          showRequestError(error, config)
          window.location.hash = '#/tenant/login'
          return Promise.reject(error)
        } finally {
          isRefreshing = false
        }
      }
    }

    const token = getAccessToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error),
)

/** Demo mode error code */
const DEMO_MODE_CODE = 10403

/** Show error toast unless suppressed by request config */
function showErrorToast(message: string, config?: InternalAxiosRequestConfig | AxiosRequestConfig) {
  if (config && '_suppressErrorMsg' in config && (config as any)._suppressErrorMsg) return
  toast.error(message)
}

function showRequestError(error: unknown, config?: InternalAxiosRequestConfig | AxiosRequestConfig) {
  if (config && '_suppressErrorMsg' in config && (config as any)._suppressErrorMsg) return
  if (!axios.isAxiosError(error)) return
  if ((error as any)._errorToastShown) return
  ;(error as any)._errorToastShown = true
  const responseData = error.response?.data
  if (responseData && typeof responseData === 'object' && 'message' in responseData) {
    toast.error(String(responseData.message || '请求失败'))
    return
  }
  if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
    toast.error('请求超时，请稍后重试')
    return
  }
  if (!error.response) {
    toast.error('网络连接异常，请检查后端服务是否正常')
    return
  }
  toast.error(`服务器请求失败（${error.response.status}）`)
}

request.interceptors.response.use(
  (response) => {
    const data = response.data
    // Unified response: code === 0 means success
    if (data && typeof data === 'object' && data.code !== undefined && data.code !== 0) {
      const msg = data.message || '请求失败'
      if (data.code === DEMO_MODE_CODE) {
        toast.warning(msg, 3000)
      } else {
        showErrorToast(msg, response.config)
      }
      const err = new Error(msg)
      ;(err as any).apiError = data as ApiError
      ;(err as any).isBusinessError = true
      ;(err as any).isDemoModeError = data.code === DEMO_MODE_CODE
      return Promise.reject(err)
    }
    return response
  },
  async (error) => {
    const originalRequest = error.config as AxiosRequestConfig & { _retry?: boolean }

    if (error.response?.status !== 401 || originalRequest._retry) {
      // Extract unified error from response body
      if (error.response?.data && typeof error.response.data === 'object' && 'code' in error.response.data) {
        const msg = error.response.data.message || '请求失败'
        if (error.response.data.code === DEMO_MODE_CODE) {
          toast.warning(msg, 3000)
        } else {
          showErrorToast(msg, originalRequest)
        }
        const err = new Error(msg)
        ;(err as any).apiError = error.response.data as ApiError
        ;(err as any).isBusinessError = true
        ;(err as any).isDemoModeError = error.response.data.code === DEMO_MODE_CODE
        return Promise.reject(err)
      }
      showRequestError(error, originalRequest)
      return Promise.reject(error)
    }

    if (isRefreshing) {
      try {
        const newToken = await waitForRefresh()
        originalRequest.headers = originalRequest.headers ?? {}
        originalRequest.headers.Authorization = `Bearer ${newToken}`
        originalRequest._retry = true
        return request(originalRequest)
      } catch (refreshError) {
        return Promise.reject(refreshError)
      }
    }

    originalRequest._retry = true
    isRefreshing = true

    try {
      const newTokens = await doRefresh()
      replayPending(newTokens.accessToken)

      originalRequest.headers = originalRequest.headers ?? {}
      originalRequest.headers.Authorization = `Bearer ${newTokens.accessToken}`
      return request(originalRequest)
    } catch (refreshError) {
      rejectPending(refreshError)
      clearTokens()
      showRequestError(refreshError, originalRequest)
      window.location.hash = '#/tenant/login'
      return Promise.reject(refreshError)
    } finally {
      isRefreshing = false
    }
  },
)

export function extractApiError(error: unknown): ApiError | null {
  // Business errors from response interceptor (code !== 0)
  if (error && typeof error === 'object' && (error as any).isBusinessError && (error as any).apiError) {
    return (error as any).apiError as ApiError
  }
  // Axios HTTP errors with unified response body
  if (
    axios.isAxiosError(error) &&
    error.response?.data &&
    typeof error.response.data === 'object' &&
    'code' in error.response.data
  ) {
    return error.response.data as ApiError
  }
  return null
}

export default request
