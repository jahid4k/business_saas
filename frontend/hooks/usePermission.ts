"use client";

import { useState, useMemo, useEffect } from "react";
import { can, canAny, canAll } from "@/lib/permissions";
import { getCurrentRole } from "@/lib/auth";
import type { PermissionKey } from "@/types/permission";

export function usePermission() {
  // Lazy initialiser handles the SSR/client split cleanly:
  // - Server: typeof window === "undefined" → returns null (no token)
  // - Client: reads the in-memory token immediately on first render
  // No useEffect, no setState cascade, no hydration mismatch.
  const [role, setRole] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    return getCurrentRole();
  });

  // The token is set asynchronously by silentRefresh in useAuth.
  // We need to react when it becomes available. The correct pattern
  // here is a subscription — we listen for the custom event that
  // useAuth dispatches after setting the token, rather than polling.
  useEffect(() => {
    function handleTokenSet() {
      setRole(getCurrentRole());
    }

    // Listen for the custom event dispatched by useAuth after login/refresh
    window.addEventListener("bsaas:token-set", handleTokenSet);
    return () => window.removeEventListener("bsaas:token-set", handleTokenSet);
  }, []);

  return useMemo(
    () => ({
      role,
      can: (p: PermissionKey) => can(p),
      canAny: (...ps: PermissionKey[]) => canAny(...ps),
      canAll: (...ps: PermissionKey[]) => canAll(...ps),
    }),
    [role],
  );
}
