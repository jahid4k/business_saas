// src/app/(dashboard)/[orgId]/page.tsx
"use client";

import { use } from "react";
import Link from "next/link";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";
import {
  CheckSquare,
  Users,
  TrendingUp,
  BarChart2,
  ArrowRight,
} from "lucide-react";

const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
const FONT_INTER = "var(--font-inter, Inter, sans-serif)";
const PURPLE = "#7c3aed";

const QUICK_LINKS = [
  {
    label: "Tasks",
    desc: "Manage your to-dos",
    icon: CheckSquare,
    href: "tasks",
    perm: "tasks.view",
  },
  {
    label: "Leads",
    desc: "Track new prospects",
    icon: Users,
    href: "crm/leads",
    perm: "crm.leads.view",
  },
  {
    label: "Pipeline",
    desc: "Visualise your deals",
    icon: TrendingUp,
    href: "crm/pipeline",
    perm: "crm.deals.view",
  },
  {
    label: "Reports",
    desc: "CRM analytics",
    icon: BarChart2,
    href: "crm/reports",
    perm: "crm.reports.view",
  },
];

export default function OrgDashboardPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { currentOrg, user } = useAuthStore();
  const { hasPermission } = usePermissionStore();

  const firstName = user?.firstName ?? user?.displayName ?? "there";
  const orgName = currentOrg?.name ?? "your workspace";

  return (
    <div
      style={{ padding: "36px 32px", maxWidth: 800, fontFamily: FONT_INTER }}
    >
      {/* Greeting */}
      <h1
        style={{
          fontFamily: FONT_SYNE,
          fontSize: "1.6rem",
          fontWeight: 700,
          color: "white",
          letterSpacing: "-0.02em",
          marginBottom: 6,
        }}
      >
        Good {timeGreeting()}, {firstName} 👋
      </h1>
      <p style={{ fontSize: "0.875rem", color: "#666", marginBottom: 36 }}>
        You're in <span style={{ color: "#aaa" }}>{orgName}</span>. Here's a
        quick overview.
      </p>

      {/* Quick access cards */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(2, 1fr)",
          gap: 12,
        }}
      >
        {QUICK_LINKS.filter((l) => hasPermission(l.perm)).map((link) => {
          const Icon = link.icon;
          return (
            <Link
              key={link.href}
              href={`/${orgId}/${link.href}`}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 14,
                padding: "16px 18px",
                borderRadius: 10,
                border: "1px solid rgba(255,255,255,0.07)",
                background: "#0f0f0f",
                textDecoration: "none",
                transition: "border-color 150ms ease, background 150ms ease",
                cursor: "pointer",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = "rgba(124,58,237,0.35)";
                e.currentTarget.style.background = "rgba(124,58,237,0.06)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = "rgba(255,255,255,0.07)";
                e.currentTarget.style.background = "#0f0f0f";
              }}
            >
              <div
                style={{
                  width: 38,
                  height: 38,
                  borderRadius: 9,
                  background: "rgba(124,58,237,0.12)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  flexShrink: 0,
                }}
              >
                <Icon size={17} style={{ color: PURPLE }} />
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <p
                  style={{
                    fontSize: "0.875rem",
                    fontWeight: 500,
                    color: "#e0e0e0",
                    fontFamily: FONT_INTER,
                    marginBottom: 2,
                  }}
                >
                  {link.label}
                </p>
                <p
                  style={{
                    fontSize: "0.75rem",
                    color: "#555",
                    fontFamily: FONT_INTER,
                  }}
                >
                  {link.desc}
                </p>
              </div>
              <ArrowRight size={14} style={{ color: "#333", flexShrink: 0 }} />
            </Link>
          );
        })}
      </div>

      {/* Next steps note */}
      <div
        style={{
          marginTop: 28,
          padding: "14px 16px",
          borderRadius: 8,
          border: "1px solid rgba(124,58,237,0.15)",
          background: "rgba(124,58,237,0.05)",
        }}
      >
        <p
          style={{
            fontSize: "0.8rem",
            color: "#7c3aed",
            fontWeight: 600,
            marginBottom: 3,
          }}
        >
          Step 5 complete ✓
        </p>
        <p style={{ fontSize: "0.8rem", color: "#555" }}>
          Dashboard shell is live. Next: build individual feature pages (Tasks,
          CRM, etc.)
        </p>
      </div>
    </div>
  );
}

function timeGreeting() {
  const h = new Date().getHours();
  if (h < 12) return "morning";
  if (h < 17) return "afternoon";
  return "evening";
}
