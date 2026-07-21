// src/app/(dashboard)/[orgId]/hrm/payroll/page.tsx
"use client";

import { use, useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  Loader2,
  Receipt,
  FileSpreadsheet,
  ChevronDown,
  ChevronUp,
} from "lucide-react";
import type { Employee, PayslipRun, Payslip } from "@/types/hrm";
import {
  listPayrollRuns,
  createPayrollRun,
  computePayrollRun,
  approvePayrollRun,
  payPayrollRun,
  cancelPayrollRun,
  listPayslips,
} from "@/lib/hrm/payroll";
import { listAttendancePeriods } from "@/lib/hrm/attendance";
import { listEmployees } from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import PayrollRunForm from "@/components/hrm/payroll/PayrollRunForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type TabKey = "runs" | "payslips";

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const RUN_STATUS_TONE: Record<string, string> = {
  draft: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  computing: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  computed: "bg-blue-500/10 text-blue-400 border-blue-500/20",
  approved: "bg-purple-500/10 text-purple-400 border-purple-500/20",
  paid: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  cancelled: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
};

export default function PayrollPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const [tab, setTab] = useState<TabKey>("runs");
  const [employees, setEmployees] = useState<Employee[]>([]);

  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
  }, [orgId]);

  const empName = (id: string) => {
    const e = employees.find((x) => x.id === id);
    return e ? `${e.first_name} ${e.last_name ?? ""}`.trim() : "—";
  };

  return (
    <div className="p-6 md:p-8 max-w-5xl">
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1
            className="text-2xl font-bold text-(--text-primary) mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Payroll
          </h1>
          <p className="text-sm text-(--text-muted)">
            Monthly runs and employee payslips
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-(--bg-elevated) border border-(--border) w-fit">
        {(["runs", "payslips"] as TabKey[]).map((key) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === key
                ? "bg-purple-600 text-white"
                : "text-(--text-secondary) hover:text-(--text-primary)"
            }`}
          >
            {key === "runs" ? "Runs" : "Payslips"}
          </button>
        ))}
      </div>

      {tab === "runs" ? (
        <RunsView
          orgId={orgId}
          canManage={hasPermission("hrm.payroll.manage")}
          canCompute={hasPermission("hrm.payroll.compute")}
          canApprove={hasPermission("hrm.payroll.approve")}
          canPay={hasPermission("hrm.payroll.pay")}
        />
      ) : (
        <PayslipsView orgId={orgId} empName={empName} />
      )}
    </div>
  );
}

// ── Runs ──────────────────────────────────────────────────────────────────
function RunsView({
  orgId,
  canManage,
  canCompute,
  canApprove,
  canPay,
}: {
  orgId: string;
  canManage: boolean;
  canCompute: boolean;
  canApprove: boolean;
  canPay: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [busyId, setBusyId] = useState<string | null>(null);

  const listKey = queryKeys.hrm.payrollRuns.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listPayrollRuns(orgId).then((r) => r.runs),
  });
  const items = (listQuery.data ?? [])
    .slice()
    .sort(
      (a, b) =>
        b.period_year - a.period_year || b.period_month - a.period_month,
    );

  const periodsQuery = useQuery({
    queryKey: queryKeys.hrm.attendance.periods(orgId),
    queryFn: () => listAttendancePeriods(orgId).then((r) => r.periods),
  });

  const update = (updated: PayslipRun) =>
    queryClient.setQueryData<PayslipRun[]>(listKey, (old) =>
      (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
    );

  const openCreate = () => {
    openDrawer({
      title: "New payroll run",
      content: (
        <PayrollRunForm
          attendancePeriods={periodsQuery.data ?? []}
          onSave={async (payload) => {
            const created = await createPayrollRun(orgId, payload);
            queryClient.setQueryData<PayslipRun[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Payroll run created.");
          }}
        />
      ),
    });
  };

  const runAction = async (
    run: PayslipRun,
    action: "compute" | "approve" | "pay" | "cancel",
  ) => {
    setBusyId(run.id);
    const fn =
      action === "compute"
        ? computePayrollRun
        : action === "approve"
          ? approvePayrollRun
          : action === "pay"
            ? payPayrollRun
            : cancelPayrollRun;
    try {
      update(await fn(orgId, run.id));
      toast.success(
        action === "compute"
          ? "Payroll computed."
          : action === "approve"
            ? "Run approved."
            : action === "pay"
              ? "Run marked as paid."
              : "Run cancelled.",
      );
    } catch {
      toast.error(`Failed to ${action} run.`);
    }
    setBusyId(null);
  };

  return (
    <>
      <div className="flex items-center justify-end mb-5">
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New run
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <Receipt size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            No payroll runs yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((run) => {
            const busy = busyId === run.id;
            return (
              <div
                key={run.id}
                className="flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border)"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <Receipt size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-(--text-primary)">
                    {MONTHS[run.period_month - 1]} {run.period_year}
                    {run.description ? ` — ${run.description}` : ""}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5">
                    {run.total_employees} employees · Gross{" "}
                    {run.total_gross_pay} · Deductions {run.total_deductions} ·
                    Net {run.total_net_pay} {run.currency}
                  </p>
                  <span
                    className={`inline-block mt-2 text-xs px-2 py-0.5 rounded-full border font-medium ${RUN_STATUS_TONE[run.status]}`}
                  >
                    {run.status}
                  </span>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {canCompute && run.status === "draft" && (
                    <button
                      disabled={busy}
                      onClick={() => runAction(run, "compute")}
                      className="px-3 py-1.5 rounded-lg text-xs font-medium text-blue-400 border border-blue-500/20 hover:bg-blue-500/10 disabled:opacity-50"
                    >
                      Compute
                    </button>
                  )}
                  {canApprove && run.status === "computed" && (
                    <button
                      disabled={busy}
                      onClick={() => runAction(run, "approve")}
                      className="px-3 py-1.5 rounded-lg text-xs font-medium text-purple-400 border border-purple-500/20 hover:bg-purple-500/10 disabled:opacity-50"
                    >
                      Approve
                    </button>
                  )}
                  {canPay && run.status === "approved" && (
                    <button
                      disabled={busy}
                      onClick={() => runAction(run, "pay")}
                      className="px-3 py-1.5 rounded-lg text-xs font-medium text-emerald-400 border border-emerald-500/20 hover:bg-emerald-500/10 disabled:opacity-50"
                    >
                      Mark paid
                    </button>
                  )}
                  {canManage &&
                    run.status !== "paid" &&
                    run.status !== "cancelled" && (
                      <button
                        disabled={busy}
                        onClick={() => runAction(run, "cancel")}
                        className="px-3 py-1.5 rounded-lg text-xs font-medium text-red-400 border border-red-500/20 hover:bg-red-500/10 disabled:opacity-50"
                      >
                        Cancel
                      </button>
                    )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}

// ── Payslips ──────────────────────────────────────────────────────────────
function PayslipsView({
  orgId,
  empName,
}: {
  orgId: string;
  empName: (id: string) => string;
}) {
  const [runFilter, setRunFilter] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const runsQuery = useQuery({
    queryKey: queryKeys.hrm.payrollRuns.list(orgId),
    queryFn: () => listPayrollRuns(orgId).then((r) => r.runs),
  });
  const runs = (runsQuery.data ?? [])
    .slice()
    .sort(
      (a, b) =>
        b.period_year - a.period_year || b.period_month - a.period_month,
    );

  const listKey = queryKeys.hrm.payslips.list(orgId, runFilter || undefined);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () =>
      listPayslips(orgId, runFilter ? { run_id: runFilter } : undefined).then(
        (r) => r.payslips,
      ),
    enabled: runs.length > 0 || runFilter === "",
  });
  const items = listQuery.data ?? [];

  return (
    <>
      <div className="flex items-center gap-3 mb-5">
        <select
          value={runFilter}
          onChange={(e) => setRunFilter(e.target.value)}
          className="px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
        >
          <option value="">All runs</option>
          {runs.map((r) => (
            <option
              key={r.id}
              value={r.id}
              style={{ background: "var(--bg-elevated)" }}
            >
              {MONTHS[r.period_month - 1]} {r.period_year}
            </option>
          ))}
        </select>
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <FileSpreadsheet size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            {runFilter
              ? "No payslips for this run yet — compute it first."
              : "No payslips yet"}
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((p) => {
            const expanded = expandedId === p.id;
            return (
              <div
                key={p.id}
                className="rounded-xl bg-(--bg-surface) border border-(--border) overflow-hidden"
              >
                <button
                  onClick={() => setExpandedId(expanded ? null : p.id)}
                  className="w-full flex items-start gap-3.5 px-4 py-3.5 text-left hover:bg-(--bg-elevated)/40 transition-colors"
                >
                  <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                    <FileSpreadsheet size={15} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-(--text-primary)">
                      {empName(p.employee_id)}
                    </p>
                    <p className="text-xs text-(--text-muted) mt-0.5">
                      {MONTHS[p.period_month - 1]} {p.period_year}
                      {p.salary_structure_name
                        ? ` · ${p.salary_structure_name}`
                        : ""}{" "}
                      · {p.present_days}/{p.work_days} days
                    </p>
                    <p className="text-xs text-(--text-muted) mt-0.5">
                      Gross {p.gross_pay} · Deductions {p.total_deductions} ·{" "}
                      <span className="text-(--text-primary) font-medium">
                        Net {p.net_pay}
                      </span>{" "}
                      {p.currency}
                    </p>
                  </div>
                  {expanded ? (
                    <ChevronUp
                      size={15}
                      className="text-(--text-muted) shrink-0 mt-1"
                    />
                  ) : (
                    <ChevronDown
                      size={15}
                      className="text-(--text-muted) shrink-0 mt-1"
                    />
                  )}
                </button>
                {expanded && (
                  <div className="px-4 pb-4 pt-1 border-t border-(--border)">
                    {(p.lines ?? []).length === 0 ? (
                      <p className="text-xs text-(--text-muted) py-2">
                        No component lines — this employee had no salary
                        structure assigned for this run.
                      </p>
                    ) : (
                      <div className="space-y-1 mt-2">
                        {p
                          .lines!.slice()
                          .sort((a, b) => a.display_order - b.display_order)
                          .map((line) => (
                            <div
                              key={line.id}
                              className="flex items-center justify-between text-xs py-1"
                            >
                              <span className="text-(--text-secondary)">
                                {line.component_name}{" "}
                                <span className="text-(--text-muted)">
                                  ({line.component_type})
                                </span>
                              </span>
                              <span
                                className={
                                  line.component_type === "deduction"
                                    ? "text-red-400"
                                    : "text-(--text-primary)"
                                }
                              >
                                {line.component_type === "deduction" ? "-" : ""}
                                {line.computed_amount}
                              </span>
                            </div>
                          ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}
