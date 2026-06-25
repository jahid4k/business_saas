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

const FONT_INTER = "var(--font-inter, Inter, sans-serif)";

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
          getMyMembership(), // uses new token's bid claim
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

  // Loading state (shown on page refresh or direct URL visit)
  if (!ready) {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ background: "#0a0a0a" }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <span
            className="animate-spin block"
            style={{
              width: 18,
              height: 18,
              borderRadius: "50%",
              border: "2px solid rgba(124,58,237,0.2)",
              borderTopColor: "#7c3aed",
            }}
          />
          <span
            style={{
              fontSize: "0.875rem",
              color: "#555",
              fontFamily: FONT_INTER,
            }}
          >
            Loading workspace…
          </span>
        </div>
      </div>
    );
  }

  // Dashboard shell
  return (
    <DrawerProvider>
      <div
        style={{
          display: "flex",
          height: "100vh",
          overflow: "hidden",
          background: "#0a0a0a",
        }}
      >
        <Sidebar orgId={orgId} />
        <div
          style={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            minWidth: 0,
            minHeight: 0,
          }}
        >
          <Topbar orgId={orgId} />
          <main style={{ flex: 1, overflowY: "auto", overflowX: "hidden" }}>
            {children}
          </main>
        </div>
      </div>
    </DrawerProvider>
  );
}
