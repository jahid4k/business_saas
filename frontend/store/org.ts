// store/org.ts
// Active organization context.
// Set after the user switches into an org (POST /organizations/:id/switch).
// The slug comes from the URL; the id + name are resolved via API.

import { create } from "zustand";

interface OrgStore {
  activeOrgId: string | null;
  activeOrgSlug: string | null;
  activeOrgName: string | null;
  activeRole: string | null;
  permissions: string[]; // flat list: ['tasks.view', 'tasks.create', ...]

  setActiveOrg: (org: {
    id: string;
    slug: string;
    name: string;
    role: string;
    permissions: string[];
  }) => void;
  clearActiveOrg: () => void;
  setPermissions: (permissions: string[]) => void;
}

export const useOrgStore = create<OrgStore>((set) => ({
  activeOrgId: null,
  activeOrgSlug: null,
  activeOrgName: null,
  activeRole: null,
  permissions: [],

  setActiveOrg: ({ id, slug, name, role, permissions }) =>
    set({
      activeOrgId: id,
      activeOrgSlug: slug,
      activeOrgName: name,
      activeRole: role,
      permissions,
    }),

  clearActiveOrg: () =>
    set({
      activeOrgId: null,
      activeOrgSlug: null,
      activeOrgName: null,
      activeRole: null,
      permissions: [],
    }),

  setPermissions: (permissions) => set({ permissions }),
}));
