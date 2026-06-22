// hooks/usePermission.ts
// Client-side permission check — UX only.
// Backend always re-enforces these independently.

"use client";

import { useOrgStore } from "@/store/org";

/** Returns true if the active org membership includes the given permission key. */
export function usePermission(key: string): boolean {
  const permissions = useOrgStore((s) => s.permissions);
  return permissions.includes(key);
}

/** Returns true if the user has ALL of the given permissions. */
export function usePermissions(...keys: string[]): boolean {
  const permissions = useOrgStore((s) => s.permissions);
  return keys.every((k) => permissions.includes(k));
}

/** Returns true if the user has ANY of the given permissions. */
export function useAnyPermission(...keys: string[]): boolean {
  const permissions = useOrgStore((s) => s.permissions);
  return keys.some((k) => permissions.includes(k));
}
