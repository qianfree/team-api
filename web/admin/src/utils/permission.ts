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
