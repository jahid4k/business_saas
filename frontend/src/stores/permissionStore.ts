// src/stores/permissionStore.ts
import { create } from "zustand";

interface PermissionState {
  permissions: string[];
  setPermissions: (permissions: string[]) => void;
  hasPermission: (permission: string) => boolean;
  reset: () => void;
}

export const usePermissionStore = create<PermissionState>((set, get) => ({
  permissions: [],

  /**
   * Hydrates the store with the freshly fetched list of permission strings
   * right after switching to an organization context.
   */
  setPermissions: (permissions) => set({ permissions }),

  /**
   * Helper function to easily gate UI elements or check if a user is allowed
   * to perform actions within a view.
   * * @example const canEdit = hasPermission('tasks:write')
   */
  hasPermission: (permission) => {
    return get().permissions.includes(permission);
  },

  /**
   * Wipes permissions completely upon logout or organization un-docking.
   */
  reset: () => set({ permissions: [] }),
}));
