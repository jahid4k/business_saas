// hooks/useOrg.ts
// Reads active org from Zustand (populated by [orgSlug]/layout.tsx).
// Components never touch the store directly — they use this hook.

"use client";

import { useOrgStore } from "@/store/org";

export function useOrg() {
  const { activeOrgId, activeOrgSlug, activeOrgName, activeRole, permissions } =
    useOrgStore();

  return {
    orgId: activeOrgId,
    orgSlug: activeOrgSlug,
    orgName: activeOrgName,
    role: activeRole,
    permissions,
    hasOrg: Boolean(activeOrgId),
  };
}
