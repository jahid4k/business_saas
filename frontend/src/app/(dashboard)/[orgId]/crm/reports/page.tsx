// src/app/(dashboard)/[orgId]/crm/reports/page.tsx
"use client";

import { use, useEffect, useState } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
} from "recharts";
import {
  Users,
  Building2,
  UserPlus,
  TrendingUp,
  DollarSign,
  Trophy,
  X as XIcon,
  Loader2,
} from "lucide-react";
import { usePermissionStore } from "@/stores/permissionStore";
import {
  getOverview,
  getDealsByStage,
  getLeadsBySource,
} from "@/lib/crm/reports";
import type { CRMSummary, DealByStage, LeadBySource, Deal } from "@/types/crm";

// ── Chart colors ───────────────────────────────────────
const PURPLE_PALETTE = ["#7c3aed", "#a855f7", "#c084fc", "#d8b4fe", "#ede9fe"];

const SOURCE_LABELS: Record<string, string> = {
  linkedin: "LinkedIn",
  website: "Website",
  referral: "Referral",
  cold_call: "Cold Call",
  email_campaign: "Email",
  trade_show: "Trade Show",
  other: "Other",
};

// ── Helpers ───────────────────────────────────────────
function formatCurrency(v: number) {
  if (v >= 1_000_000) return `$${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `$${(v / 1_000).toFixed(0)}K`;
  return `$${v}`;
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}

// ── Stat card ─────────────────────────────────────────
function StatCard({
  label,
  value,
  icon: Icon,
  sub,
  accent,
}: {
  label: string;
  value: string | number;
  icon: typeof Users;
  sub?: string;
  accent?: boolean;
}) {
  return (
    <div
      className={`
        rounded-xl p-5 border transition-all
        ${
          accent
            ? "bg-purple-600/10 border-purple-500/30"
            : "bg-[var(--bg-surface)] border-[var(--border)]"
        }
      `}
    >
      <div className="flex items-start justify-between mb-3">
        <div
          className={`w-9 h-9 rounded-lg flex items-center justify-center
            ${accent ? "bg-purple-500/20" : "bg-[var(--bg-elevated)]"}`}
        >
          <Icon
            size={16}
            className={accent ? "text-purple-400" : "text-[var(--text-muted)]"}
          />
        </div>
      </div>
      <p
        className={`text-2xl font-bold mb-0.5 ${accent ? "text-purple-300" : "text-[var(--text-primary)]"}`}
        style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
      >
        {value}
      </p>
      <p className="text-xs text-[var(--text-muted)]">{label}</p>
      {sub && <p className="text-xs text-purple-400 mt-0.5">{sub}</p>}
    </div>
  );
}

// ── Custom tooltip for bar chart ─────────────────────
function BarTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: { value: number; name: string }[];
  label?: string;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div
      className="rounded-lg px-3 py-2 text-xs border"
      style={{
        background: "#1a1a1a",
        borderColor: "rgba(255,255,255,0.1)",
        color: "#ccc",
      }}
    >
      <p className="font-semibold mb-1">{label}</p>
      {payload.map((p) => (
        <p key={p.name}>{formatCurrency(p.value)}</p>
      ))}
    </div>
  );
}

// ── Custom tooltip for pie chart ─────────────────────
function PieTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: { name: string; value: number }[];
}) {
  if (!active || !payload?.length) return null;
  return (
    <div
      className="rounded-lg px-3 py-2 text-xs border"
      style={{
        background: "#1a1a1a",
        borderColor: "rgba(255,255,255,0.1)",
        color: "#ccc",
      }}
    >
      <p className="font-semibold">
        {SOURCE_LABELS[payload[0].name] ?? payload[0].name}
      </p>
      <p>
        {payload[0].value} lead{payload[0].value !== 1 ? "s" : ""}
      </p>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────
export default function ReportsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();

  const [summary, setSummary] = useState<CRMSummary | null>(null);
  const [recentDeals, setRecentDeals] = useState<Deal[]>([]);
  const [dealsByStage, setDealsByStage] = useState<DealByStage[]>([]);
  const [leadsBySource, setLeadsBySource] = useState<LeadBySource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  if (!hasPermission("crm.reports.view")) {
    return (
      <div className="p-8 text-sm text-[var(--text-muted)]">
        You don't have permission to view reports.
      </div>
    );
  }

  useEffect(() => {
    Promise.all([
      getOverview(orgId),
      getDealsByStage(orgId),
      getLeadsBySource(orgId),
    ])
      .then(([overview, byStage, bySource]) => {
        setSummary(overview.summary);
        setRecentDeals(overview.recent_deals);
        setDealsByStage(byStage);
        setLeadsBySource(bySource);
      })
      .catch(() => {
        setError("Failed to load reports.");
      })
      .finally(() => setLoading(false));
  }, [orgId]);

  if (loading) {
    return (
      <div className="flex items-center gap-3 p-8 text-sm text-[var(--text-muted)]">
        <Loader2 size={15} className="animate-spin text-purple-500" />
        Loading reports…
      </div>
    );
  }

  if (error || !summary) {
    return (
      <div className="p-8 text-sm text-red-400">
        {error ?? "Failed to load."}
      </div>
    );
  }

  // Prepare chart data
  const stageChartData = dealsByStage.map((d) => ({
    name: d.stage_name,
    value: d.total_value,
    count: d.count,
  }));

  const sourceChartData = leadsBySource.map((d) => ({
    name: d.source,
    value: d.count,
  }));

  return (
    <div className="p-6 md:p-8 max-w-7xl">
      {/* Header */}
      <div className="mb-8">
        <h1
          className="text-2xl font-bold text-[var(--text-primary)] mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          CRM Reports
        </h1>
        <p className="text-sm text-[var(--text-muted)]">
          Overview of your sales pipeline
        </p>
      </div>

      {/* ── Row 1: Stat cards ─────────────────── */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
        <StatCard
          label="Total leads"
          value={summary.total_leads}
          icon={UserPlus}
        />
        <StatCard
          label="Contacts"
          value={summary.total_contacts}
          icon={Users}
        />
        <StatCard
          label="Companies"
          value={summary.total_companies}
          icon={Building2}
        />
        <StatCard
          label="Open deals"
          value={summary.open_deals}
          icon={TrendingUp}
          sub={`${summary.total_deals} total`}
        />
        <StatCard
          label="Pipeline value"
          value={formatCurrency(summary.total_deal_value)}
          icon={DollarSign}
          sub={
            summary.won_deal_value > 0
              ? `${formatCurrency(summary.won_deal_value)} won`
              : undefined
          }
          accent
        />
      </div>

      {/* Deal outcome row */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="rounded-xl p-4 border bg-[var(--bg-surface)] border-[var(--border)] flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-blue-500/10 flex items-center justify-center">
            <TrendingUp size={18} className="text-blue-400" />
          </div>
          <div>
            <p
              className="text-xl font-bold text-[var(--text-primary)]"
              style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
            >
              {summary.open_deals}
            </p>
            <p className="text-xs text-[var(--text-muted)]">Open</p>
          </div>
        </div>
        <div className="rounded-xl p-4 border bg-[var(--bg-surface)] border-[var(--border)] flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-emerald-500/10 flex items-center justify-center">
            <Trophy size={18} className="text-emerald-400" />
          </div>
          <div>
            <p
              className="text-xl font-bold text-emerald-400"
              style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
            >
              {summary.won_deals}
            </p>
            <p className="text-xs text-[var(--text-muted)]">Won</p>
          </div>
        </div>
        <div className="rounded-xl p-4 border bg-[var(--bg-surface)] border-[var(--border)] flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-red-500/10 flex items-center justify-center">
            <XIcon size={18} className="text-red-400" />
          </div>
          <div>
            <p
              className="text-xl font-bold text-red-400"
              style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
            >
              {summary.lost_deals}
            </p>
            <p className="text-xs text-[var(--text-muted)]">Lost</p>
          </div>
        </div>
      </div>

      {/* ── Row 2: Charts ─────────────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-6 mb-8">
        {/* Deals by stage — bar chart (3/5 width) */}
        <div className="lg:col-span-3 rounded-xl p-6 border bg-[var(--bg-surface)] border-[var(--border)]">
          <p
            className="text-sm font-semibold text-[var(--text-primary)] mb-1"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            Deals by stage
          </p>
          <p className="text-xs text-[var(--text-muted)] mb-5">
            Total value per pipeline stage
          </p>

          {stageChartData.length === 0 ? (
            <div className="flex items-center justify-center h-48 text-sm text-[var(--text-muted)]">
              No deals data yet
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={stageChartData} barSize={36}>
                <CartesianGrid
                  strokeDasharray="3 3"
                  stroke="rgba(255,255,255,0.04)"
                  vertical={false}
                />
                <XAxis
                  dataKey="name"
                  tick={{ fill: "#666", fontSize: 11 }}
                  axisLine={false}
                  tickLine={false}
                />
                <YAxis
                  tick={{ fill: "#666", fontSize: 11 }}
                  axisLine={false}
                  tickLine={false}
                  tickFormatter={(v) => formatCurrency(v)}
                />
                <Tooltip
                  content={<BarTooltip />}
                  cursor={{ fill: "rgba(255,255,255,0.03)" }}
                />
                <Bar dataKey="value" fill="#7c3aed" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Leads by source — pie chart (2/5 width) */}
        <div className="lg:col-span-2 rounded-xl p-6 border bg-[var(--bg-surface)] border-[var(--border)]">
          <p
            className="text-sm font-semibold text-[var(--text-primary)] mb-1"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            Leads by source
          </p>
          <p className="text-xs text-[var(--text-muted)] mb-5">
            Where your leads come from
          </p>

          {sourceChartData.length === 0 ? (
            <div className="flex items-center justify-center h-48 text-sm text-[var(--text-muted)]">
              No leads data yet
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie
                  data={sourceChartData}
                  cx="50%"
                  cy="45%"
                  innerRadius={55}
                  outerRadius={80}
                  dataKey="value"
                  nameKey="name"
                  paddingAngle={3}
                >
                  {sourceChartData.map((_, i) => (
                    <Cell
                      key={i}
                      fill={PURPLE_PALETTE[i % PURPLE_PALETTE.length]}
                    />
                  ))}
                </Pie>
                <Tooltip content={<PieTooltip />} />
                <Legend
                  formatter={(value: string) => (
                    <span style={{ color: "#888", fontSize: 11 }}>
                      {SOURCE_LABELS[value] ?? value}
                    </span>
                  )}
                  iconSize={8}
                  iconType="circle"
                />
              </PieChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* ── Row 3: Recent deals ─────────────── */}
      {recentDeals.length > 0 && (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] overflow-hidden">
          <div className="px-6 py-4 border-b border-[var(--border)]">
            <p
              className="text-sm font-semibold text-[var(--text-primary)]"
              style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
            >
              Recent deals
            </p>
          </div>
          <div className="divide-y divide-[var(--border)]">
            {recentDeals.slice(0, 8).map((deal) => (
              <div
                key={deal.id}
                className="flex items-center gap-4 px-6 py-3.5 hover:bg-[var(--bg-elevated)] transition-colors"
              >
                <div className="flex-1 min-w-0">
                  <p
                    className="text-sm font-medium text-[var(--text-primary)] truncate"
                    style={{
                      fontFamily: "var(--font-inter, Inter, sans-serif)",
                    }}
                  >
                    {deal.title}
                  </p>
                  <p className="text-xs text-[var(--text-muted)]">
                    {formatDate(deal.created_at)}
                  </p>
                </div>
                <p className="text-sm font-semibold text-[var(--text-primary)] flex-shrink-0">
                  {formatCurrency(deal.value)}
                </p>
                <span
                  className={`text-[0.65rem] font-semibold capitalize px-2 py-0.5 rounded-full border flex-shrink-0
                    ${
                      deal.status === "won"
                        ? "text-emerald-400 bg-emerald-500/10 border-emerald-500/20"
                        : deal.status === "lost"
                          ? "text-red-400 bg-red-500/10 border-red-500/20"
                          : "text-blue-400 bg-blue-500/10 border-blue-500/20"
                    }
                  `}
                >
                  {deal.status}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
