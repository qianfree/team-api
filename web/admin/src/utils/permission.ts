export const ADMIN_ROLES = {
  SUPER_ADMIN: 'super_admin',
  ADMIN: 'admin',
} as const

const ADMIN_PERMISSIONS_KEY = 'admin_permissions'
const ADMIN_ROLE_KEY = 'admin_role'

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

export function hasPermission(permission: string): boolean {
  // Super admin bypasses all permission checks
  if (hasRole(ADMIN_ROLES.SUPER_ADMIN)) {
    return true
  }
  const permissions = getStoredJSON<string>(ADMIN_PERMISSIONS_KEY)
  return permissions.includes(permission)
}

/**
 * 是否超级管理员。
 *
 * 用于「不参与权限点授权、硬性限定超管」的功能（如角色权限管理）：
 * 这类功能是权限体系的元操作，不能通过授予权限点下放，否则被下放者
 * 能给自己挂上更高权限的角色。后端有对应的 superAdminOnlyRules 拦截，
 * 前端这份判断只用于隐藏入口，不构成安全边界。
 */
export function isSuperAdmin(): boolean {
  return hasRole(ADMIN_ROLES.SUPER_ADMIN)
}

export function hasRole(role: string): boolean {
  const currentRole = getStoredString(ADMIN_ROLE_KEY)
  return currentRole === role
}

export function setAdminSession(role: string, permissions: string[]): void {
  localStorage.setItem(ADMIN_ROLE_KEY, role)
  localStorage.setItem(ADMIN_PERMISSIONS_KEY, JSON.stringify(permissions))
}

export function clearAdminSession(): void {
  localStorage.removeItem(ADMIN_ROLE_KEY)
  localStorage.removeItem(ADMIN_PERMISSIONS_KEY)
}
