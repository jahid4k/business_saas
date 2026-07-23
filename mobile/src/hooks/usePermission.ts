import { usePermissionStore } from '@/stores/permissionStore';

export function usePermission() {
  const permissions = usePermissionStore(state => state.permissions);

  const hasAnyPermission = (requiredAny: string[]) => {
    if (!requiredAny || requiredAny.length === 0) return true;
    return requiredAny.some(perm => permissions.includes(perm));
  };

  const hasAllPermissions = (requiredAll: string[]) => {
    if (!requiredAll || requiredAll.length === 0) return true;
    return requiredAll.every(perm => permissions.includes(perm));
  };

  return {
    permissions,
    hasAnyPermission,
    hasAllPermissions,
  };
}
