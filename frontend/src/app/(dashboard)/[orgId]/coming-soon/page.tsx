// src/app/(dashboard)/[orgId]/coming-soon/page.tsx
"use client";

import { Suspense } from "react";
import { useParams, useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  TrendingUp,
  UsersRound,
  ReceiptText,
  FolderKanban,
  ShoppingCart,
  CheckCircle2,
  Clock,
  ArrowLeft,
  Sparkles,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

// ── Module registry ────────────────────────────────────
interface ModuleInfo {
  label: string;
  icon: LucideIcon;
  status: "live" | "soon";
  eta?: string;
  desc: string;
  features: string[];
}

const MODULES: Record<string, ModuleInfo> = {
  crm: {
    label: "CRM",
    icon: TrendingUp,
    status: "live",
    desc: "Customer relationship management — live now.",
    features: [
      "Leads & pipeline",
      "Contacts & companies",
      "Deals & stages",
      "Activity timeline",
      "Reports",
    ],
  },
  hrm: {
    label: "HRM",
    icon: UsersRound,
    status: "soon",
    eta: "Q3 2026",
    desc: "Human resource management — departments, employees, leave, and payroll.",
    features: [
      "Departments & org chart",
      "Employee profiles & documents",
      "Leave & attendance tracking",
      "Payroll processing",
      "Performance reviews",
      "Onboarding & offboarding workflows",
    ],
  },
  accounting: {
    label: "Accounting",
    icon: ReceiptText,
    status: "soon",
    eta: "Q4 2026",
    desc: "Full accounting suite — invoices, expenses, and financial reporting.",
    features: [
      "Invoice creation & management",
      "Expense tracking & categories",
      "Bank reconciliation",
      "Profit & loss reports",
      "Tax compliance tools",
      "Multi-currency support",
    ],
  },
  projects: {
    label: "Projects",
    icon: FolderKanban,
    status: "soon",
    eta: "Q4 2026",
    desc: "Project management — tasks, milestones, and team workload visibility.",
    features: [
      "Project boards & milestones",
      "Task assignment & tracking",
      "Time logging",
      "Team workload overview",
      "Gantt chart view",
      "Client-facing project portals",
    ],
  },
  ecommerce: {
    label: "E-commerce",
    icon: ShoppingCart,
    status: "soon",
    eta: "2027",
    desc: "E-commerce admin — products, orders, customers, and inventory.",
    features: [
      "Product catalog management",
      "Order processing & fulfilment",
      "Customer segmentation",
      "Inventory tracking",
      "Discount & coupon engine",
      "Revenue analytics",
    ],
  },
};

// Ordered roadmap for the timeline
const ROADMAP_ORDER = ["crm", "hrm", "accounting", "projects", "ecommerce"];

// ── Page content ───────────────────────────────────────
function ComingSoonContent() {
  const params = useParams();
  const searchParams = useSearchParams();
  const orgId = params.orgId as string;
  const moduleId = searchParams.get("module") ?? "";
  const info = MODULES[moduleId];

  if (!info) {
    return (
      <div className="p-8 text-(--text-muted) text-sm">
        Unknown module.{" "}
        <Link href={`/${orgId}`} className="text-[#7c3aed] underline">
          Go back.
        </Link>
      </div>
    );
  }

  const Icon = info.icon;

  return (
    <div className="max-w-3xl px-8 py-10">
      {/* Back */}
      <Link
        href={`/${orgId}`}
        className="inline-flex items-center gap-2 text-sm text-(--text-muted) hover:text-(--text-secondary) transition-colors no-underline mb-10"
      >
        <ArrowLeft size={14} />
        Back to dashboard
      </Link>

      {/* Module hero */}
      <div className="flex items-start gap-5 mb-10">
        <div
          className="w-14 h-14 rounded-2xl shrink-0 flex items-center justify-center"
          style={{
            background: "rgba(124,58,237,0.12)",
            border: "1px solid rgba(124,58,237,0.2)",
          }}
        >
          <Icon size={26} style={{ color: "#7c3aed" }} />
        </div>
        <div>
          <div className="flex items-center gap-3 mb-1.5">
            <h1
              className="text-2xl font-bold text-(--text-primary)"
              style={{
                fontFamily: "var(--font-syne, Syne, sans-serif)",
                letterSpacing: "-0.02em",
              }}
            >
              {info.label}
            </h1>
            {info.status === "soon" ? (
              <span
                className="
                text-[0.65rem] font-semibold uppercase tracking-wider
                text-purple-400 bg-purple-500/10 border border-purple-500/20
                px-2 py-0.5 rounded-full
              "
              >
                Coming {info.eta}
              </span>
            ) : (
              <span
                className="
                text-[0.65rem] font-semibold uppercase tracking-wider
                text-emerald-400 bg-emerald-500/10 border border-emerald-500/20
                px-2 py-0.5 rounded-full
              "
              >
                Live
              </span>
            )}
          </div>
          <p className="text-sm text-(--text-muted)">{info.desc}</p>
        </div>
      </div>

      {/* Planned features */}
      {info.status === "soon" && (
        <div
          className="rounded-xl p-6 mb-10"
          style={{
            background: "var(--bg-surface)",
            border: "1px solid var(--border)",
          }}
        >
          <div className="flex items-center gap-2 mb-4">
            <Sparkles size={14} style={{ color: "#7c3aed" }} />
            <p
              className="text-xs font-semibold text-(--text-muted) uppercase tracking-wider"
              style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
            >
              Planned features
            </p>
          </div>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {info.features.map((f) => (
              <div key={f} className="flex items-center gap-2.5">
                <div
                  className="w-1.5 h-1.5 rounded-full shrink-0"
                  style={{ background: "#7c3aed" }}
                />
                <span className="text-sm text-(--text-secondary)">
                  {f}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Product roadmap */}
      <div>
        <h2
          className="text-base font-semibold text-(--text-primary) mb-5"
          style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
        >
          Product roadmap
        </h2>

        <div className="space-y-2">
          {ROADMAP_ORDER.map((id, idx) => {
            const mod = MODULES[id];
            const ModIcon = mod.icon;
            const isCurrent = id === moduleId;
            const isLive = mod.status === "live";

            return (
              <div
                key={id}
                className={`
                  flex items-center gap-4 px-4 py-3.5 rounded-xl
                  transition-colors
                  ${
                    isCurrent
                      ? "border border-[#7c3aed]/30 bg-[rgba(124,58,237,0.06)]"
                      : "border border-(--border) bg-(--bg-surface)"
                  }
                `}
              >
                {/* Timeline dot */}
                <div className="flex flex-col items-center self-stretch">
                  <div
                    className={`
                      w-7 h-7 rounded-full shrink-0 flex items-center justify-center
                      ${isLive ? "bg-emerald-500/15" : isCurrent ? "bg-purple-500/15" : "bg-(--bg-elevated)"}
                    `}
                  >
                    {isLive ? (
                      <CheckCircle2 size={14} className="text-emerald-400" />
                    ) : (
                      <Clock
                        size={13}
                        className={
                          isCurrent
                            ? "text-purple-400"
                            : "text-(--text-muted)"
                        }
                      />
                    )}
                  </div>
                  {idx < ROADMAP_ORDER.length - 1 && (
                    <div className="w-px flex-1 mt-1.5 bg-(--border)" />
                  )}
                </div>

                {/* Content */}
                <div className="flex items-center gap-3 flex-1 min-w-0 py-0.5">
                  <div
                    className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center"
                    style={{
                      background:
                        isLive || isCurrent
                          ? "rgba(124,58,237,0.1)"
                          : "var(--bg-elevated)",
                      border: `1px solid ${isLive || isCurrent ? "rgba(124,58,237,0.2)" : "var(--border)"}`,
                    }}
                  >
                    <ModIcon
                      size={15}
                      style={{
                        color:
                          isLive || isCurrent ? "#7c3aed" : "var(--text-muted)",
                      }}
                    />
                  </div>
                  <div className="min-w-0">
                    <p
                      className={`text-sm font-semibold ${isLive || isCurrent ? "text-(--text-primary)" : "text-(--text-muted)"}`}
                      style={{
                        fontFamily: "var(--font-inter, Inter, sans-serif)",
                      }}
                    >
                      {mod.label}
                      {isCurrent && (
                        <span className="ml-2 text-[0.65rem] font-normal text-purple-400">
                          ← you are here
                        </span>
                      )}
                    </p>
                    <p className="text-xs text-(--text-muted)">
                      {mod.desc}
                    </p>
                  </div>
                </div>

                {/* ETA badge */}
                <div className="shrink-0">
                  {isLive ? (
                    <span className="text-xs font-medium text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2.5 py-1 rounded-full">
                      Live
                    </span>
                  ) : (
                    <span className="text-xs font-medium text-(--text-muted) bg-(--bg-elevated) border border-(--border) px-2.5 py-1 rounded-full">
                      {mod.eta}
                    </span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ── Page export — useSearchParams requires Suspense ────
export default function ComingSoonPage() {
  return (
    <Suspense
      fallback={
        <div className="p-8 text-sm text-(--text-muted)">Loading…</div>
      }
    >
      <ComingSoonContent />
    </Suspense>
  );
}
