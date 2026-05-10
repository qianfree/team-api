import { hasPermission } from '@/utils/permission'

export function usePermission() {
  function can(permission: string): boolean {
    return hasPermission(permission)
  }

  function cannot(permission: string): boolean {
    return !hasPermission(permission)
  }

  return {
    can,
    cannot,
  }
}
