// src/components/providers/OrgProvider.tsx
// Handles org context hydration + renders the full dashboard shell.
//
// Responsibilities:
//   1. On mount: check if currentOrg in store matches the URL orgId
//   2. If not: fetch org + membership (e.g. after browser refresh or direct URL visit)
//   3. When ready: render Sidebar + Topbar + main > children
"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getToken, setToken } from "@/lib/token";
import { decodeToken } from "@/lib/jwt";
import {
  getOrganization,
  switchOrganization,
  getMyMembership,
} from "@/lib/org";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";
import Sidebar from "@/components/layout/Sidebar";
import Topbar from "@/components/layout/Topbar";
import { DrawerProvider } from "@/contexts/DrawerContext";

interface Props {
  orgId: string;
  children: React.ReactNode;
}

export default function OrgProvider({ orgId, children }: Props) {
  const [ready, setReady] = useState(false);
  const router = useRouter();
  const { currentOrg, setOrg } = useAuthStore();
  const { setPermissions } = usePermissionStore();

  useEffect(() => {
    let cancelled = false;

    const init = async () => {
      // Case 1: correct org already in memory (same-session navigation) → instant
      if (currentOrg?.id === orgId) {
        if (!cancelled) setReady(true);
        return;
      }

      // Case 2: need to hydrate org context (page refresh / direct URL)
      const token = getToken();
      if (!token) {
        router.replace("/login");
        return;
      }

      const claims = decodeToken(token);

      // If token has a different business context, switch first
      if (claims.bid !== orgId) {
        try {
          const switchData = await switchOrganization(orgId);
          setToken(switchData.access_token);
        } catch {
          if (!cancelled) router.replace("/select-organization");
          return;
        }
      }

      // Fetch org + membership in parallel
      try {
        const [org, membership] = await Promise.all([
          getOrganization(orgId),
          getMyMembership(),
        ]);
        if (!cancelled) {
          setOrg(org, membership);
          setPermissions(membership.permissions);
          setReady(true);
        }
      } catch {
        if (!cancelled) router.replace("/select-organization");
      }
    };

    setReady(false);
    init();

    return () => {
      cancelled = true;
    };
  }, [orgId]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Loading state ────────────────────────────────────────────────────────────
  if (!ready) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-white dark:bg-[#0a0a0a]">
        <div className="flex items-center gap-3">
          <span
            className="
              animate-spin block
              w-[18px] h-[18px] rounded-full
              border-2 border-purple-600/20 border-t-purple-600
            "
          />
          <span className="text-sm text-gray-400 dark:text-[#555]">
            Loading workspace…
          </span>
        </div>
      </div>
    );
  }

  // ── Dashboard shell ──────────────────────────────────────────────────────────
  return (
    <DrawerProvider>
      <div className="flex h-screen overflow-hidden bg-white dark:bg-[#0a0a0a]">
        <Sidebar orgId={orgId} />

        <div className="flex flex-1 flex-col min-w-0 min-h-0">
          <Topbar orgId={orgId} />
          <main className="flex-1 overflow-y-auto overflow-x-hidden">
            {children}
          </main>
        </div>
      </div>
    </DrawerProvider>
  );
}

// to make the content center
// <main className="flex-1 overflow-y-auto overflow-x-hidden">
//   <div className="w-full max-w-[1200px] mx-auto">{children}</div>
// </main>
