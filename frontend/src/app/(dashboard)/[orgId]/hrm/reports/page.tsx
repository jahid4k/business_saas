// src/app/(dashboard)/[orgId]/hrm/reports/page.tsx
"use client";

import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import {
  Loader2,
  Users,
  UserCheck,
  CalendarClock,
  Hourglass,
} from "lucide-react";
import {
  getHRMOverview,
  getHeadcountByDepartment,
  getLeaveSummaryByType,
} from "@/lib/hrm/reports";
import { queryKeys } from "@/lib/queryKeys";

function KpiCard({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Users;
  label: string;
  value: number;
}) {
  return (
    <div className="p-4 rounded-xl bg-(--bg-surface) border border-(--border)">
      <div className="flex items-center gap-2.5 mb-2">
        <div className="w-7 h-7 rounded-lg flex items-center justify-center bg-purple-500/10 text-purple-400">
          <Icon size={14} />
        </div>
        <span className="text-xs text-(--text-muted)">{label}</span>
      </div>
      <p className="text-2xl font-bold text-(--text-primary) tabular-nums">
        {value.toLocaleString()}
      </p>
    </div>
  );
}

const CHART_TOOLTIP_STYLE = {
  background: "var(--bg-elevated)",
  border: "1px solid var(--border)",
  borderRadius: 8,
  fontSize: 12,
  color: "var(--text-primary)",
};

export default function HRMReportsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);

  const summaryQuery = useQuery({
    queryKey: ["hrm", orgId, "reports", "overview"],
    queryFn: () => getHRMOverview(orgId),
  });

  const headcountQuery = useQuery({
    queryKey: ["hrm", orgId, "reports", "headcount"],
    queryFn: () => getHeadcountByDepartment(orgId),
  });

  const leaveQuery = useQuery({
    queryKey: ["hrm", orgId, "reports", "leave-summary"],
    queryFn: () => getLeaveSummaryByType(orgId),
  });

  const summary = summaryQuery.data;
  const headcount = headcountQuery.data ?? [];
  const leaveSummary = leaveQuery.data ?? [];

  const isLoading =
    summaryQuery.isPending || headcountQuery.isPending || leaveQuery.isPending;

  return (
    <div className="p-6 md:p-8 max-w-6xl">
      <div className="mb-6">
        <h1
          className="text-2xl font-bold text-(--text-primary) mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          HRM Reports
        </h1>
        <p className="text-sm text-(--text-muted)">
          Headcount, leave, and workforce overview
        </p>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center py-24 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : (
        <>
          {/* KPI strip */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
            <KpiCard
              icon={Users}
              label="Total employees"
              value={summary?.total_employees ?? 0}
            />
            <KpiCard
              icon={UserCheck}
              label="Active"
              value={summary?.active_employees ?? 0}
            />
            <KpiCard
              icon={CalendarClock}
              label="On leave"
              value={summary?.on_leave_employees ?? 0}
            />
            <KpiCard
              icon={Hourglass}
              label="Pending leave requests"
              value={summary?.pending_leave_requests ?? 0}
            />
          </div>

          {/* Secondary stats */}
          <div className="flex items-center gap-5 mb-6 text-xs text-(--text-muted) flex-wrap">
            <span>{summary?.total_departments ?? 0} departments</span>
            <span>{summary?.total_positions ?? 0} positions</span>
            <span>{summary?.terminated_employees ?? 0} terminated</span>
            <span>
              {summary?.approved_leave_today ?? 0} on approved leave today
            </span>
          </div>

          {/* Charts — split pane */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
            <div className="p-4 rounded-xl bg-(--bg-surface) border border-(--border)">
              <p className="text-sm font-semibold text-(--text-primary) mb-4">
                Headcount by department
              </p>
              {headcount.length === 0 ? (
                <p className="text-xs text-(--text-muted) py-10 text-center">
                  No department data yet.
                </p>
              ) : (
                <ResponsiveContainer
                  width="100%"
                  height={Math.max(200, headcount.length * 36)}
                >
                  <BarChart
                    data={headcount}
                    layout="vertical"
                    margin={{ left: 8, right: 16 }}
                  >
                    <CartesianGrid
                      strokeDasharray="3 3"
                      stroke="var(--border)"
                      horizontal={false}
                    />
                    <XAxis
                      type="number"
                      allowDecimals={false}
                      tick={{ fill: "var(--text-muted)", fontSize: 11 }}
                    />
                    <YAxis
                      type="category"
                      dataKey="department_name"
                      width={110}
                      tick={{ fill: "var(--text-muted)", fontSize: 11 }}
                    />
                    <Tooltip
                      contentStyle={CHART_TOOLTIP_STYLE}
                      cursor={{ fill: "var(--bg-elevated)" }}
                    />
                    <Bar
                      dataKey="headcount"
                      fill="#7c3aed"
                      radius={[0, 4, 4, 0]}
                    />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="p-4 rounded-xl bg-(--bg-surface) border border-(--border)">
              <p className="text-sm font-semibold text-(--text-primary) mb-4">
                Leave requests by type
              </p>
              {leaveSummary.length === 0 ? (
                <p className="text-xs text-(--text-muted) py-10 text-center">
                  No leave data yet.
                </p>
              ) : (
                <ResponsiveContainer
                  width="100%"
                  height={Math.max(200, leaveSummary.length * 50)}
                >
                  <BarChart
                    data={leaveSummary}
                    layout="vertical"
                    margin={{ left: 8, right: 16 }}
                  >
                    <CartesianGrid
                      strokeDasharray="3 3"
                      stroke="var(--border)"
                      horizontal={false}
                    />
                    <XAxis
                      type="number"
                      allowDecimals={false}
                      tick={{ fill: "var(--text-muted)", fontSize: 11 }}
                    />
                    <YAxis
                      type="category"
                      dataKey="leave_type_name"
                      width={100}
                      tick={{ fill: "var(--text-muted)", fontSize: 11 }}
                    />
                    <Tooltip
                      contentStyle={CHART_TOOLTIP_STYLE}
                      cursor={{ fill: "var(--bg-elevated)" }}
                    />
                    <Legend wrapperStyle={{ fontSize: 11 }} />
                    <Bar
                      dataKey="approved"
                      stackId="a"
                      fill="#10b981"
                      name="Approved"
                    />
                    <Bar
                      dataKey="pending"
                      stackId="a"
                      fill="#f59e0b"
                      name="Pending"
                    />
                    <Bar
                      dataKey="rejected"
                      stackId="a"
                      fill="#ef4444"
                      name="Rejected"
                      radius={[0, 4, 4, 0]}
                    />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>
          </div>

          {/* Table */}
          <div className="rounded-xl bg-(--bg-surface) border border-(--border) overflow-hidden">
            <p className="text-sm font-semibold text-(--text-primary) px-4 pt-4 pb-3">
              Leave summary detail
            </p>
            {leaveSummary.length === 0 ? (
              <p className="text-xs text-(--text-muted) px-4 pb-4">
                No leave requests recorded yet.
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-t border-(--border) text-xs text-(--text-muted)">
                      <th className="text-left font-medium px-4 py-2.5">
                        Leave type
                      </th>
                      <th className="text-right font-medium px-4 py-2.5">
                        Total
                      </th>
                      <th className="text-right font-medium px-4 py-2.5">
                        Approved
                      </th>
                      <th className="text-right font-medium px-4 py-2.5">
                        Pending
                      </th>
                      <th className="text-right font-medium px-4 py-2.5">
                        Rejected
                      </th>
                      <th className="text-right font-medium px-4 py-2.5">
                        Total days
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {leaveSummary.map((row) => (
                      <tr
                        key={row.leave_type_id}
                        className="border-t border-(--border)"
                      >
                        <td className="px-4 py-2.5 text-(--text-primary)">
                          {row.leave_type_name}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums text-(--text-secondary)">
                          {row.total_requests}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums text-emerald-400">
                          {row.approved}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums text-amber-400">
                          {row.pending}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums text-red-400">
                          {row.rejected}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums text-(--text-secondary)">
                          {row.total_days}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
