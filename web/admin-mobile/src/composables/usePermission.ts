import { hasPermission as checkPermission, hasRole } from '@/utils/permission'

export function usePermission() {
  function check(permission: string): boolean {
    return checkPermission(permission)
  }

  function checkRole(role: string): boolean {
    return hasRole(role)
  }

  return { check, checkRole }
}
