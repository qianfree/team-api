import axios from 'axios'
import type { AxiosInstance, InternalAxiosRequestConfig } from 'axios'
import { Message } from '@arco-design/web-vue'

interface TokenPair {
  accessToken: string
  refreshToken: string
  expiresAt: number
}

interface ApiError {
  code: number
  message: string
  request_id: string
}

/** Request config extension — set _suppressErrorMsg to skip auto error toast */
declare module 'axios' {
  interface InternalAxiosRequestConfig {
    _suppressErrorMsg?: boolean
  }
}

const ACCESS_TOKEN_KEY = 'admin_access_token'
const REFRESH_TOKEN_KEY = 'admin_refresh_token'
const EXPIRES_AT_KEY = 'admin_token_expires_at'
const REMEMBER_ME_KEY = 'admin_remember_me'
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
let pendingRequests: Array<(token: string) => void> = []
let tokenRefreshedCallback: ((tokens: TokenPair) => void) | null = null

export function onTokenRefreshed(cb: (tokens: TokenPair) => void): void {
  tokenRefreshedCallback = cb
}

function replayPending(token: string): void {
  pendingRequests.forEach((cb) => cb(token))
  pendingRequests = []
}

function clearPending(): void {
  pendingRequests = []
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

  const { data } = await axios.post('/api/admin/auth/refresh', {
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

request.interceptors.request.use(
  async (config: InternalAxiosRequestConfig) => {
    if (shouldRefresh() && !config.url?.includes('/auth/refresh')) {
      if (isRefreshing) {
        await new Promise<void>((resolve) => {
          pendingRequests.push(() => resolve())
        })
      } else {
        isRefreshing = true
        try {
          const newTokens = await doRefresh()
          replayPending(newTokens.accessToken)
        } catch {
          clearPending()
          clearTokens()
          window.location.hash = '#/admin/login'
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
function showErrorToast(message: string, config?: InternalAxiosRequestConfig) {
  if (config?._suppressErrorMsg) return
  Message.error(message)
}

request.interceptors.response.use(
  (response) => {
    const data = response.data
    // Unified response: code === 0 means success
    if (data && typeof data === 'object' && data.code !== undefined && data.code !== 0) {
      const msg = data.message || '请求失败'
      if (data.code === DEMO_MODE_CODE) {
        Message.warning({ content: msg, duration: 3 })
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
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean }

    if (error.response?.status !== 401 || originalRequest._retry) {
      // Extract unified error from response body
      if (error.response?.data && typeof error.response.data === 'object' && 'code' in error.response.data) {
        const msg = error.response.data.message || '请求失败'
        if (error.response.data.code === DEMO_MODE_CODE) {
          Message.warning({ content: msg, duration: 3 })
        } else {
          showErrorToast(msg, originalRequest)
        }
        const err = new Error(msg)
        ;(err as any).apiError = error.response.data as ApiError
        ;(err as any).isDemoModeError = error.response.data.code === DEMO_MODE_CODE
        return Promise.reject(err)
      }
      return Promise.reject(error)
    }

    if (isRefreshing) {
      return new Promise((resolve) => {
        pendingRequests.push((newToken: string) => {
          originalRequest.headers = originalRequest.headers ?? {}
          originalRequest.headers.Authorization = `Bearer ${newToken}`
          originalRequest._retry = true
          resolve(request(originalRequest))
        })
      })
    }

    originalRequest._retry = true
    isRefreshing = true

    try {
      const newTokens = await doRefresh()
      replayPending(newTokens.accessToken)

      originalRequest.headers = originalRequest.headers ?? {}
      originalRequest.headers.Authorization = `Bearer ${newTokens.accessToken}`
      return request(originalRequest)
    } catch {
      clearPending()
      clearTokens()
      window.location.hash = '#/admin/login'
      return Promise.reject(error)
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
