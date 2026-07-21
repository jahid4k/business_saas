// src/app/(dashboard)/[orgId]/crm/reports/page.tsx
"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
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
  Phone,
  Video,
} from "lucide-react";
import { usePermissionStore } from "@/stores/permissionStore";
import {
  getOverview,
  getDealsByStage,
  getLeadsBySource,
  getRepPerformance,
  getForecast,
} from "@/lib/crm/reports";
import { queryKeys } from "@/lib/queryKeys";
import type { DealByStage, LeadBySource } from "@/types/crm";

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
function formatCurrency(value: number, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    maximumFractionDigits: 0,
  }).format(value);
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
            : "bg-(--bg-surface) border-(--border)"
        }
      `}
    >
      <div className="flex items-start justify-between mb-3">
        <div
          className={`w-9 h-9 rounded-lg flex items-center justify-center
            ${accent ? "bg-purple-500/20" : "bg-(--bg-elevated)"}`}
        >
          <Icon
            size={16}
            className={accent ? "text-purple-400" : "text-(--text-muted)"}
          />
        </div>
      </div>
      <p
        className={`text-2xl font-bold mb-0.5 ${accent ? "text-purple-300" : "text-(--text-primary)"}`}
        style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
      >
        {value}
      </p>
      <p className="text-xs text-(--text-muted)">{label}</p>
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

// ── Inline state for a single chart card (loading / error / empty) ──
function ChartCardState({
  loading,
  error,
  emptyLabel,
}: {
  loading: boolean;
  error: boolean;
  emptyLabel: string;
}) {
  if (loading) {
    return (
      <div className="flex items-center justify-center h-48 gap-2 text-sm text-(--text-muted)">
        <Loader2 size={14} className="animate-spin text-purple-500" />
        Loading…
      </div>
    );
  }
  if (error) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-red-400">
        Failed to load this chart.
      </div>
    );
  }
  return (
    <div className="flex items-center justify-center h-48 text-sm text-(--text-muted)">
      {emptyLabel}
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
  const canView = hasPermission("crm.reports.view");

  const [activeTab, setActiveTab] = useState<"overview" | "performance" | "forecast">("overview");

  const overviewQuery = useQuery({
    queryKey: queryKeys.crm.reports.overview(orgId),
    queryFn: () => getOverview(orgId),
    enabled: canView && (activeTab === "overview" || activeTab === "forecast"),
  });

  const dealsByStageQuery = useQuery({
    queryKey: queryKeys.crm.reports.dealsByStage(orgId),
    queryFn: () => getDealsByStage(orgId),
    enabled: canView && activeTab === "overview",
  });

  const leadsBySourceQuery = useQuery({
    queryKey: queryKeys.crm.reports.leadsBySource(orgId),
    queryFn: () => getLeadsBySource(orgId),
    enabled: canView && activeTab === "overview",
  });

  const repPerformanceQuery = useQuery({
    queryKey: ["crm", "reports", "repPerformance", orgId],
    queryFn: () => getRepPerformance(orgId),
    enabled: canView && activeTab === "performance",
  });

  const forecastQuery = useQuery({
    queryKey: ["crm", "reports", "forecast", orgId],
    queryFn: () => getForecast(orgId),
    enabled: canView && activeTab === "forecast",
  });

  if (!canView) {
    return (
      <div className="p-8 text-sm text-(--text-muted)">
        You do not have permission to view reports.
      </div>
    );
  }

  // Stat cards, the won/lost row, and the recent-deals list all come from
  // `overview` — that one query is the only thing the page genuinely can't
  // render without, so it's the only one still gating the whole page.
  if (activeTab === "overview" && overviewQuery.isPending) {
    return (
      <div className="flex items-center gap-3 p-8 text-sm text-(--text-muted)">
        <Loader2 size={15} className="animate-spin text-purple-500" />
        Loading reports…
      </div>
    );
  }

  if (activeTab === "overview" && (overviewQuery.isError || !overviewQuery.data)) {
    return (
      <div className="p-8 text-sm text-red-400">Failed to load reports.</div>
    );
  }

  const { summary, recent_deals: recentDeals } = overviewQuery.data ?? { summary: {} as any, recent_deals: [] };

  const stageChartData = (dealsByStageQuery.data ?? []).map(
    (d: DealByStage) => ({
      name: d.stage_name,
      value: d.total_value,
      count: d.count,
    }),
  );

  const sourceChartData = (leadsBySourceQuery.data ?? []).map(
    (d: LeadBySource) => ({
      name: d.source,
      value: d.count,
    }),
  );

  return (
    <div className="p-6 md:p-8 max-w-7xl">
      {/* Header */}
      <div className="mb-8">
        <h1
          className="text-2xl font-bold text-(--text-primary) mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          CRM Reports
        </h1>
        <p className="text-sm text-(--text-muted)">
          Overview of your sales pipeline
        </p>
      </div>

      <div className="flex border-b border-(--border) mb-8">
        <button
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === "overview"
              ? "border-purple-500 text-purple-600 dark:text-purple-400"
              : "border-transparent text-(--text-secondary) hover:text-(--text-primary)"
          }`}
          onClick={() => setActiveTab("overview")}
        >
          Overview
        </button>
        <button
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === "performance"
              ? "border-purple-500 text-purple-600 dark:text-purple-400"
              : "border-transparent text-(--text-secondary) hover:text-(--text-primary)"
          }`}
          onClick={() => setActiveTab("performance")}
        >
          Rep Performance
        </button>
        <button
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === "forecast"
              ? "border-purple-500 text-purple-600 dark:text-purple-400"
              : "border-transparent text-(--text-secondary) hover:text-(--text-primary)"
          }`}
          onClick={() => setActiveTab("forecast")}
        >
          Forecast
        </button>
      </div>

      {activeTab === "overview" ? (
        <>
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
        <div className="rounded-xl p-4 border bg-(--bg-surface) border-(--border) flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-blue-500/10 flex items-center justify-center">
            <TrendingUp size={18} className="text-blue-400" />
          </div>
          <div>
            <p
              className="text-xl font-bold text-(--text-primary)"
              style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
            >
              {summary.open_deals}
            </p>
            <p className="text-xs text-(--text-muted)">Open</p>
          </div>
        </div>
        <div className="rounded-xl p-4 border bg-(--bg-surface) border-(--border) flex items-center gap-4">
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
            <p className="text-xs text-(--text-muted)">Won</p>
          </div>
        </div>
        <div className="rounded-xl p-4 border bg-(--bg-surface) border-(--border) flex items-center gap-4">
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
            <p className="text-xs text-(--text-muted)">Lost</p>
          </div>
        </div>
      </div>

      {/* ── Row 2: Charts ─────────────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-6 mb-8">
        {/* Deals by stage — bar chart (3/5 width) */}
        <div className="lg:col-span-3 rounded-xl p-6 border bg-(--bg-surface) border-(--border)">
          <p
            className="text-sm font-semibold text-(--text-primary) mb-1"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            Deals by stage
          </p>
          <p className="text-xs text-(--text-muted) mb-5">
            Total value per pipeline stage
          </p>

          {dealsByStageQuery.isPending ||
          dealsByStageQuery.isError ||
          stageChartData.length === 0 ? (
            <ChartCardState
              loading={dealsByStageQuery.isPending}
              error={dealsByStageQuery.isError}
              emptyLabel="No deals data yet"
            />
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
        <div className="lg:col-span-2 rounded-xl p-6 border bg-(--bg-surface) border-(--border)">
          <p
            className="text-sm font-semibold text-(--text-primary) mb-1"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            Leads by source
          </p>
          <p className="text-xs text-(--text-muted) mb-5">
            Where your leads come from
          </p>

          {leadsBySourceQuery.isPending ||
          leadsBySourceQuery.isError ||
          sourceChartData.length === 0 ? (
            <ChartCardState
              loading={leadsBySourceQuery.isPending}
              error={leadsBySourceQuery.isError}
              emptyLabel="No leads data yet"
            />
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
        <div className="rounded-xl border border-(--border) bg-(--bg-surface) overflow-hidden">
          <div className="px-6 py-4 border-b border-(--border)">
            <p
              className="text-sm font-semibold text-(--text-primary)"
              style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
            >
              Recent deals
            </p>
          </div>
          <div className="divide-y divide-(--border)">
            {recentDeals.slice(0, 8).map((deal) => (
              <div
                key={deal.id}
                className="flex items-center gap-4 px-6 py-3.5 hover:bg-(--bg-elevated) transition-colors"
              >
                <div className="flex-1 min-w-0">
                  <p
                    className="text-sm font-medium text-(--text-primary) truncate"
                    style={{
                      fontFamily: "var(--font-inter, Inter, sans-serif)",
                    }}
                  >
                    {deal.title}
                  </p>
                  <p className="text-xs text-(--text-muted)">
                    {formatDate(deal.created_at)}
                  </p>
                </div>
                <p className="text-sm font-semibold text-(--text-primary) shrink-0">
                  {formatCurrency(deal.value)}
                </p>
                <span
                  className={`text-[0.65rem] font-semibold capitalize px-2 py-0.5 rounded-full border shrink-0
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
        </>
      ) : activeTab === "performance" ? (
        <div className="bg-(--bg-surface) border border-(--border) rounded-xl overflow-hidden shadow-sm">
          <div className="px-6 py-4 border-b border-(--border)">
            <h2 className="text-lg font-semibold text-(--text-primary)">Rep Performance</h2>
            <p className="text-sm text-(--text-secondary)">Activity and deal metrics by team member</p>
          </div>
          
          {repPerformanceQuery.isPending ? (
            <div className="flex justify-center p-12">
              <Loader2 className="animate-spin text-purple-600" size={32} />
            </div>
          ) : repPerformanceQuery.isError ? (
            <div className="p-8 text-center text-red-500">Failed to load performance data</div>
          ) : repPerformanceQuery.data?.length === 0 ? (
            <div className="p-12 text-center text-(--text-muted)">No data available</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="border-b border-(--border) bg-(--bg-canvas)">
                    <th className="px-6 py-4 text-xs font-semibold text-(--text-muted) uppercase tracking-wider">Rep Name</th>
                    <th className="px-6 py-4 text-xs font-semibold text-(--text-muted) uppercase tracking-wider">Calls</th>
                    <th className="px-6 py-4 text-xs font-semibold text-(--text-muted) uppercase tracking-wider">Meetings</th>
                    <th className="px-6 py-4 text-xs font-semibold text-(--text-muted) uppercase tracking-wider">Deals Closed</th>
                    <th className="px-6 py-4 text-xs font-semibold text-(--text-muted) uppercase tracking-wider">Revenue Won</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-(--border)">
                  {repPerformanceQuery.data?.map((rep) => (
                    <tr key={rep.rep_id} className="hover:bg-gray-50/50 dark:hover:bg-white/2 transition-colors">
                      <td className="px-6 py-4 text-sm font-medium text-(--text-primary)">{rep.rep_name}</td>
                      <td className="px-6 py-4 text-sm text-(--text-secondary) flex items-center gap-2">
                        <Phone size={14} className="text-blue-500" />
                        {rep.calls}
                      </td>
                      <td className="px-6 py-4 text-sm text-(--text-secondary)">
                        <div className="flex items-center gap-2">
                          <Video size={14} className="text-purple-500" />
                          {rep.meetings}
                        </div>
                      </td>
                      <td className="px-6 py-4 text-sm text-(--text-secondary)">{rep.deals_closed}</td>
                      <td className="px-6 py-4 text-sm font-semibold text-emerald-600 dark:text-emerald-400">
                        {formatCurrency(rep.revenue_won)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : activeTab === "forecast" ? (
        <div className="bg-(--bg-surface) border border-(--border) rounded-xl overflow-hidden shadow-sm p-8">
          <div className="mb-8 border-b border-(--border) pb-6">
            <h2 className="text-xl font-bold text-(--text-primary) mb-2">Revenue Forecast</h2>
            <p className="text-sm text-(--text-secondary)">
              Expected revenue calculated by multiplying open deal values by their pipeline stage probability.
            </p>
          </div>

          {forecastQuery.isPending ? (
            <div className="flex justify-center p-12">
              <Loader2 className="animate-spin text-purple-600" size={32} />
            </div>
          ) : forecastQuery.isError ? (
            <div className="p-8 text-center text-red-500">Failed to load forecast data</div>
          ) : !forecastQuery.data ? (
            <div className="p-12 text-center text-(--text-muted)">No data available</div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
              <div className="p-6 rounded-xl border border-(--border) bg-(--bg-elevated)">
                <p className="text-sm font-medium text-(--text-secondary) mb-2">Total Pipeline Value</p>
                <p className="text-3xl font-bold text-(--text-primary)" style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}>
                  {formatCurrency(forecastQuery.data.total_pipeline_value)}
                </p>
                <p className="text-xs text-(--text-muted) mt-2">Sum of all open deals</p>
              </div>
              
              <div className="p-6 rounded-xl border border-purple-500/30 bg-purple-500/5 relative overflow-hidden group">
                <div className="absolute top-0 right-0 p-16 bg-purple-500/20 blur-[50px] rounded-full" />
                <p className="text-sm font-medium text-purple-400 mb-2">Weighted Forecast</p>
                <p className="text-4xl font-bold text-purple-300 relative z-10" style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}>
                  {formatCurrency(forecastQuery.data.weighted_forecast)}
                </p>
                <p className="text-xs text-purple-500/70 mt-2 relative z-10">Adjusted by win probability</p>
              </div>
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}
