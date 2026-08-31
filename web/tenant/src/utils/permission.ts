export const TENANT_ROLES = {
  OWNER: 'owner',
  ADMIN: 'admin',
  MEMBER: 'member',
} as const

const TENANT_PERMISSIONS_KEY = 'tenant_permissions'
const TENANT_ROLE_KEY = 'tenant_role'

function getStoredJSON<T>(key: string): T[] {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T[]) : []
  } catch {
    return []
  }
}

function getStoredString(key: string): string | null {
  return localStorage.getItem(key)
}

export function hasTenantPermission(permission: string): boolean {
  const permissions = getStoredJSON<string>(TENANT_PERMISSIONS_KEY)
  return permissions.includes(permission)
}

export function hasTenantRole(role: string): boolean {
  const currentRole = getStoredString(TENANT_ROLE_KEY)
  return currentRole === role
}

/**
 * 是否为管理层（owner / admin）。
 * 读 localStorage 而非 Pinia：路由重定向在组件 setup 之外同步执行，
 * 此时 store 可能尚未 hydrate，localStorage 是唯一可靠的同步来源。
 * 组件内做条件渲染请用 store 的 isManager（响应式）。
 */
export function isTenantManager(): boolean {
  const currentRole = getStoredString(TENANT_ROLE_KEY)
  return currentRole === TENANT_ROLES.OWNER || currentRole === TENANT_ROLES.ADMIN
}

export function setTenantSession(role: string, permissions: string[]): void {
  localStorage.setItem(TENANT_ROLE_KEY, role)
  localStorage.setItem(TENANT_PERMISSIONS_KEY, JSON.stringify(permissions))
}

export function clearTenantSession(): void {
  localStorage.removeItem(TENANT_ROLE_KEY)
  localStorage.removeItem(TENANT_PERMISSIONS_KEY)
}
