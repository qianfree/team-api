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

export function setTenantSession(role: string, permissions: string[]): void {
  localStorage.setItem(TENANT_ROLE_KEY, role)
  localStorage.setItem(TENANT_PERMISSIONS_KEY, JSON.stringify(permissions))
}

export function clearTenantSession(): void {
  localStorage.removeItem(TENANT_ROLE_KEY)
  localStorage.removeItem(TENANT_PERMISSIONS_KEY)
}
