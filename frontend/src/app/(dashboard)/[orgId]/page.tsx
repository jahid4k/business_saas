// src/app/(dashboard)/[orgId]/page.tsx
"use client";

import { use } from "react";
import Link from "next/link";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";

const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
const FONT_INTER = "var(--font-inter, Inter, sans-serif)";

export default function OrgRootPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { currentOrg, user } = useAuthStore();
  const { permissions } = usePermissionStore();

  const orgName = currentOrg?.name ?? "Your Workspace";
  const initial = orgName[0].toUpperCase();

  return (
    <div
      className="min-h-screen flex items-center justify-center px-4"
      style={{ background: "#0a0a0a" }}
    >
      <div className="text-center max-w-sm">
        {/* Org avatar */}
        <div
          className="w-16 h-16 rounded-2xl mx-auto mb-6 flex items-center justify-center text-xl font-bold text-white"
          style={{
            background: "linear-gradient(135deg, #7c3aed, #a855f7)",
            fontFamily: FONT_SYNE,
          }}
        >
          {initial}
        </div>

        <h1
          className="text-2xl font-bold text-white mb-2"
          style={{ fontFamily: FONT_SYNE, letterSpacing: "-0.02em" }}
        >
          {orgName}
        </h1>

        <p
          className="text-sm mb-2"
          style={{ color: "#555", fontFamily: FONT_INTER }}
        >
          Signed in as <span style={{ color: "#888" }}>{user?.email}</span>
        </p>

        <p
          className="text-xs mb-8"
          style={{ color: "#333", fontFamily: FONT_INTER }}
        >
          {permissions.length} permission{permissions.length !== 1 ? "s" : ""}{" "}
          loaded · org: {orgId.slice(0, 8)}…
        </p>

        {/* Step 5 coming */}
        <div
          className="rounded-xl p-6 mb-6 text-left"
          style={{
            background: "#0f0f0f",
            border: "1px solid rgba(255,255,255,0.06)",
          }}
        >
          <p
            className="text-xs font-medium mb-3"
            style={{
              color: "#7c3aed",
              fontFamily: FONT_INTER,
              letterSpacing: "0.08em",
              textTransform: "uppercase",
            }}
          >
            Next up
          </p>
          <p
            className="text-sm text-white mb-1"
            style={{ fontFamily: FONT_SYNE }}
          >
            Step 5 — Dashboard Shell
          </p>
          <p
            className="text-xs"
            style={{ color: "#555", fontFamily: FONT_INTER }}
          >
            Sidebar, Topbar, and navigation — coming next
          </p>
        </div>

        <Link
          href="/select-organization"
          className="text-sm transition-colors"
          style={{ color: "#444", fontFamily: FONT_INTER }}
          onMouseEnter={(e) => (e.currentTarget.style.color = "#888")}
          onMouseLeave={(e) => (e.currentTarget.style.color = "#444")}
        >
          ← Switch workspace
        </Link>
      </div>
    </div>
  );
}
