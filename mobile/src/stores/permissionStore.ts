import { create } from 'zustand';

interface PermissionStore {
  permissions: string[];
  setPermissions: (permissions: string[]) => void;
  can: (perm: string) => boolean;
  canAny: (perms: string[]) => boolean;
  reset: () => void;
}

export const usePermissionStore = create<PermissionStore>((set, get) => ({
  permissions: [],
  setPermissions: (permissions) => set({ permissions }),
  can: (perm) => get().permissions.includes(perm),
  canAny: (perms) => perms.some((p) => get().permissions.includes(p)),
  reset: () => set({ permissions: [] }),
}));
