// src/app/(dashboard)/[orgId]/hrm/attendance/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Loader2,
  Clock,
  Lock,
  CheckCircle2,
} from "lucide-react";
import gsap from "gsap";
import type { Employee, AttendanceRecord, AttendancePeriod } from "@/types/hrm";
import {
  listAttendanceRecords,
  createAttendanceRecord,
  approveAttendanceRecord,
  rejectAttendanceRecord,
  regularizeAttendanceRecord,
  listAttendancePeriods,
  getOrCreateAttendancePeriod,
  finalizeAttendancePeriod,
  lockAttendancePeriod,
} from "@/lib/hrm/attendance";
import { listEmployees } from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import AttendanceRecordForm from "@/components/hrm/attendance/AttendanceRecordForm";
import RegularizeForm from "@/components/hrm/attendance/RegularizeForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

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

const STATUS_BADGE: Record<string, string> = {
  approved: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  pending: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  rejected: "bg-red-500/10 text-red-400 border-red-500/20",
};

const DAY_TYPE_LABEL: Record<string, string> = {
  present: "Present",
  absent: "Absent",
  half_day: "Half day",
  late: "Late",
  on_leave: "On leave",
  holiday: "Holiday",
  weekend: "Weekend",
  work_from_home: "WFH",
};

export default function AttendancePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const [tab, setTab] = useState<"records" | "periods">("records");
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
            className="text-2xl font-bold text-[var(--text-primary)] mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Attendance
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Daily records and monthly periods
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] w-fit">
        {(["records", "periods"] as const).map((key) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === key
                ? "bg-purple-600 text-white"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            {key === "records" ? "Records" : "Periods"}
          </button>
        ))}
      </div>

      {tab === "records" ? (
        <RecordsView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.attendance.manage")}
          canApprove={hasPermission("hrm.attendance.approve")}
        />
      ) : (
        <PeriodsView
          orgId={orgId}
          canManage={hasPermission("hrm.attendance.manage")}
          canFinalize={hasPermission("hrm.attendance.finalize")}
        />
      )}
    </div>
  );
}

// ── Records ───────────────────────────────────────────────────────────────
function RecordsView({
  orgId,
  employees,
  empName,
  canManage,
  canApprove,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
  canApprove: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const now = new Date();
  const [year, setYear] = useState(now.getFullYear());
  const [month, setMonth] = useState(now.getMonth() + 1);
  const [empFilter, setEmpFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.attendance.records(orgId, year, month);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () =>
      listAttendanceRecords(orgId, { year, month }).then((r) => r.records),
  });
  const allItems = listQuery.data ?? [];
  const items = allItems.filter((r) => {
    if (empFilter && r.employee_id !== empFilter) return false;
    if (statusFilter !== "all" && r.status !== statusFilter) return false;
    return true;
  });

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".att-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, empFilter, statusFilter]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      let inside = false;
      menuRefs.current.forEach((el) => {
        if (el.contains(e.target as Node)) inside = true;
      });
      if (!inside) setOpenMenuId(null);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const openCreate = () => {
    openDrawer({
      title: "New attendance record",
      content: (
        <AttendanceRecordForm
          employees={employees}
          onSave={async (payload) => {
            const created = await createAttendanceRecord(orgId, payload);
            queryClient.setQueryData<AttendanceRecord[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Attendance record saved.");
          }}
        />
      ),
    });
  };

  const openRegularize = (rec: AttendanceRecord) => {
    setOpenMenuId(null);
    openDrawer({
      title: "Request correction",
      content: (
        <RegularizeForm
          record={rec}
          onSave={async (payload) => {
            const updated = await regularizeAttendanceRecord(
              orgId,
              rec.id,
              payload,
            );
            queryClient.setQueryData<AttendanceRecord[]>(listKey, (old) =>
              (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
            );
            toast.success("Correction submitted for approval.");
          }}
        />
      ),
    });
  };

  const handleApprove = async (rec: AttendanceRecord) => {
    try {
      const updated = await approveAttendanceRecord(orgId, rec.id);
      queryClient.setQueryData<AttendanceRecord[]>(listKey, (old) =>
        (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success("Correction approved.");
    } catch {
      toast.error("Failed to approve.");
    }
    setOpenMenuId(null);
  };

  const handleReject = async (rec: AttendanceRecord) => {
    try {
      const updated = await rejectAttendanceRecord(orgId, rec.id);
      queryClient.setQueryData<AttendanceRecord[]>(listKey, (old) =>
        (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success("Correction rejected.");
    } catch {
      toast.error("Failed to reject.");
    }
    setOpenMenuId(null);
  };

  return (
    <>
      <div className="flex items-center gap-3 mb-5 flex-wrap">
        <select
          value={month}
          onChange={(e) => setMonth(Number(e.target.value))}
          className="px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500"
        >
          {MONTHS.map((m, i) => (
            <option
              key={m}
              value={i + 1}
              style={{ background: "var(--bg-elevated)" }}
            >
              {m}
            </option>
          ))}
        </select>
        <select
          value={year}
          onChange={(e) => setYear(Number(e.target.value))}
          className="px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500"
        >
          {[year - 1, year, year + 1].map((y) => (
            <option
              key={y}
              value={y}
              style={{ background: "var(--bg-elevated)" }}
            >
              {y}
            </option>
          ))}
        </select>
        <select
          value={empFilter}
          onChange={(e) => setEmpFilter(e.target.value)}
          className="px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500"
        >
          <option value="">All employees</option>
          {employees.map((e) => (
            <option
              key={e.id}
              value={e.id}
              style={{ background: "var(--bg-elevated)" }}
            >
              {e.first_name} {e.last_name ?? ""}
            </option>
          ))}
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500"
        >
          <option value="all">All statuses</option>
          <option value="approved">Approved</option>
          <option value="pending">Pending</option>
          <option value="rejected">Rejected</option>
        </select>
        <div className="flex-1" />
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New record
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <Clock size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No records for this period
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {items.map((rec) => {
            const menuOpen = openMenuId === rec.id;
            const showMenu =
              (rec.status === "pending" && canApprove) ||
              (rec.status === "approved" && canManage);

            return (
              <div
                key={rec.id}
                className="att-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <Clock size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {empName(rec.employee_id)} ·{" "}
                    {new Date(rec.attendance_date).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                    })}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {DAY_TYPE_LABEL[rec.day_type] ?? rec.day_type}
                    {rec.check_in_time ? ` · In ${rec.check_in_time}` : ""}
                    {rec.check_out_time ? ` · Out ${rec.check_out_time}` : ""}
                    {rec.overtime_hours > 0
                      ? ` · OT ${rec.overtime_hours}h`
                      : ""}
                  </p>
                  {rec.regularization_reason && (
                    <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                      Correction: {rec.regularization_reason}
                    </p>
                  )}
                  <div className="mt-2">
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full border font-medium ${STATUS_BADGE[rec.status]}`}
                    >
                      {rec.status}
                    </span>
                  </div>
                </div>

                {showMenu && (
                  <div
                    className="relative flex-shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(rec.id, el);
                      else menuRefs.current.delete(rec.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : rec.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                        {rec.status === "pending" && canApprove && (
                          <>
                            <button
                              onClick={() => handleApprove(rec)}
                              className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-emerald-400 hover:bg-emerald-500/10 transition-colors text-left"
                            >
                              Approve
                            </button>
                            <button
                              onClick={() => handleReject(rec)}
                              className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 transition-colors text-left"
                            >
                              Reject
                            </button>
                          </>
                        )}
                        {rec.status === "approved" && canManage && (
                          <button
                            onClick={() => openRegularize(rec)}
                            className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors text-left"
                          >
                            Request correction
                          </button>
                        )}
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

// ── Periods ───────────────────────────────────────────────────────────────
function PeriodsView({
  orgId,
  canManage,
  canFinalize,
}: {
  orgId: string;
  canManage: boolean;
  canFinalize: boolean;
}) {
  const queryClient = useQueryClient();
  const now = new Date();
  const [newYear, setNewYear] = useState(now.getFullYear());
  const [newMonth, setNewMonth] = useState(now.getMonth() + 1);

  const listKey = queryKeys.hrm.attendance.periods(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAttendancePeriods(orgId).then((r) => r.periods),
  });
  const periods = (listQuery.data ?? []).sort((a, b) =>
    a.period_year !== b.period_year
      ? b.period_year - a.period_year
      : b.period_month - a.period_month,
  );

  const handleOpenPeriod = async () => {
    try {
      const period = await getOrCreateAttendancePeriod(
        orgId,
        newYear,
        newMonth,
      );
      queryClient.setQueryData<AttendancePeriod[]>(listKey, (old) => {
        const exists = (old ?? []).some((p) => p.id === period.id);
        return exists
          ? (old ?? []).map((p) => (p.id === period.id ? period : p))
          : [period, ...(old ?? [])];
      });
      toast.success(`${MONTHS[newMonth - 1]} ${newYear} period ready.`);
    } catch {
      toast.error("Failed to open period.");
    }
  };

  const handleFinalize = async (p: AttendancePeriod) => {
    try {
      const updated = await finalizeAttendancePeriod(
        orgId,
        p.period_year,
        p.period_month,
      );
      queryClient.setQueryData<AttendancePeriod[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Period finalized.");
    } catch {
      toast.error("Failed to finalize period.");
    }
  };

  const handleLock = async (p: AttendancePeriod) => {
    try {
      const updated = await lockAttendancePeriod(
        orgId,
        p.period_year,
        p.period_month,
      );
      queryClient.setQueryData<AttendancePeriod[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Period locked.");
    } catch {
      toast.error("Failed to lock period.");
    }
  };

  return (
    <>
      {canManage && (
        <div className="flex items-center gap-3 mb-5">
          <select
            value={newMonth}
            onChange={(e) => setNewMonth(Number(e.target.value))}
            className="px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500"
          >
            {MONTHS.map((m, i) => (
              <option
                key={m}
                value={i + 1}
                style={{ background: "var(--bg-elevated)" }}
              >
                {m}
              </option>
            ))}
          </select>
          <select
            value={newYear}
            onChange={(e) => setNewYear(Number(e.target.value))}
            className="px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500"
          >
            {[newYear - 1, newYear, newYear + 1].map((y) => (
              <option
                key={y}
                value={y}
                style={{ background: "var(--bg-elevated)" }}
              >
                {y}
              </option>
            ))}
          </select>
          <button
            onClick={handleOpenPeriod}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            Open period
          </button>
        </div>
      )}

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : periods.length === 0 ? (
        <p className="text-sm text-[var(--text-muted)] py-10 text-center">
          No periods opened yet.
        </p>
      ) : (
        <div className="space-y-1.5">
          {periods.map((p) => (
            <div
              key={p.id}
              className="flex items-start justify-between gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
            >
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">
                  {MONTHS[p.period_month - 1]} {p.period_year}
                </p>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {p.total_employees} employees · {p.total_present} present ·{" "}
                  {p.total_absent} absent · {p.total_leaves} on leave ·{" "}
                  {p.total_holidays} holidays · {p.total_overtime_hours}h OT
                </p>
                <span
                  className={`inline-block mt-2 text-xs px-2 py-0.5 rounded-full border font-medium ${
                    p.status === "locked"
                      ? "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"
                      : p.status === "finalized"
                        ? "bg-blue-500/10 text-blue-400 border-blue-500/20"
                        : "bg-amber-500/10 text-amber-400 border-amber-500/20"
                  }`}
                >
                  {p.status}
                </span>
              </div>
              {canFinalize && p.status === "open" && (
                <button
                  onClick={() => handleFinalize(p)}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-blue-400 border border-blue-500/20 hover:bg-blue-500/10 transition-colors flex-shrink-0"
                >
                  <CheckCircle2 size={13} />
                  Finalize
                </button>
              )}
              {canFinalize && p.status === "finalized" && (
                <button
                  onClick={() => handleLock(p)}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-elevated)] transition-colors flex-shrink-0"
                >
                  <Lock size={13} />
                  Lock
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </>
  );
}
