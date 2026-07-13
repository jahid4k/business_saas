// src/app/(dashboard)/[orgId]/hrm/setup/shifts/page.tsx
"use client";

import { use, useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Loader2, Clock3, Users, Trash2, Pencil } from "lucide-react";
import type {
  Employee,
  Department,
  Shift,
  WorkScheduleAssignment,
} from "@/types/hrm";
import {
  listShifts,
  createShift,
  updateShift,
  deleteShift,
  listShiftAssignments,
  assignShift,
  removeShiftAssignment,
} from "@/lib/hrm/shifts";
import { listEmployees } from "@/lib/hrm/employees";
import { listDepartments } from "@/lib/hrm/departments";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import ShiftForm from "@/components/hrm/shifts/ShiftForm";
import ShiftAssignmentForm from "@/components/hrm/shifts/ShiftAssignmentForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type TabKey = "shifts" | "assignments";

export default function ShiftsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const [tab, setTab] = useState<TabKey>("shifts");
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const canManage = hasPermission("hrm.shifts.manage");

  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
  }, [orgId]);

  const empName = (id: string) => {
    const e = employees.find((x) => x.id === id);
    return e ? `${e.first_name} ${e.last_name ?? ""}`.trim() : id;
  };
  const deptName = (id: string) =>
    departments.find((d) => d.id === id)?.name ?? id;

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
            Shifts
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Work schedules and assignments
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] w-fit">
        {(["shifts", "assignments"] as TabKey[]).map((key) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === key
                ? "bg-purple-600 text-white"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            {key === "shifts" ? "Shifts" : "Assignments"}
          </button>
        ))}
      </div>

      {tab === "shifts" ? (
        <ShiftsView orgId={orgId} canManage={canManage} />
      ) : (
        <AssignmentsView
          orgId={orgId}
          employees={employees}
          departments={departments}
          empName={empName}
          deptName={deptName}
          canManage={canManage}
        />
      )}
    </div>
  );
}

function ShiftsView({
  orgId,
  canManage,
}: {
  orgId: string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const listKey = queryKeys.hrm.shifts.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listShifts(orgId).then((r) => r.shifts),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New shift",
      content: (
        <ShiftForm
          onSave={async (payload) => {
            const created = await createShift(orgId, payload);
            queryClient.setQueryData<Shift[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Shift created.");
          }}
        />
      ),
    });
  };

  const openEdit = (s: Shift) => {
    openDrawer({
      title: "Edit shift",
      content: (
        <ShiftForm
          shift={s}
          onSave={async (payload) => {
            const updated = await updateShift(orgId, s.id, payload);
            queryClient.setQueryData<Shift[]>(listKey, (old) =>
              (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
            );
            toast.success("Shift updated.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (shiftId: string) => {
    try {
      await deleteShift(orgId, shiftId);
      queryClient.setQueryData<Shift[]>(listKey, (old) =>
        (old ?? []).filter((s) => s.id !== shiftId),
      );
      toast.success("Shift deleted.");
    } catch {
      toast.error("Failed to delete shift.");
    }
    setDeleteConfirm(null);
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
            New shift
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
            <Clock3 size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No shifts yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((s) => (
            <div
              key={s.id}
              className="group flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <Clock3 size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">
                  {s.name}{" "}
                  {s.is_default && (
                    <span className="text-xs text-purple-400 ml-1">
                      (default)
                    </span>
                  )}
                </p>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {s.shift_type === "fixed"
                    ? `${s.start_time} – ${s.end_time}`
                    : `${s.weekly_hours_target}h/week flexible`}
                  {" · "}
                  {s.working_days.join(", ")}
                </p>
              </div>
              {canManage &&
                (deleteConfirm === s.id ? (
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <span className="text-xs text-[var(--text-muted)]">
                      Delete?
                    </span>
                    <button
                      onClick={() => handleDelete(s.id)}
                      className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400"
                    >
                      Yes
                    </button>
                    <button
                      onClick={() => setDeleteConfirm(null)}
                      className="px-2.5 py-1 rounded-md text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
                    >
                      No
                    </button>
                  </div>
                ) : (
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0">
                    <button
                      onClick={() => openEdit(s)}
                      className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
                    >
                      <Pencil size={14} />
                    </button>
                    <button
                      onClick={() => setDeleteConfirm(s.id)}
                      className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
            </div>
          ))}
        </div>
      )}
    </>
  );
}

function AssignmentsView({
  orgId,
  employees,
  departments,
  empName,
  deptName,
  canManage,
}: {
  orgId: string;
  employees: Employee[];
  departments: Department[];
  empName: (id: string) => string;
  deptName: (id: string) => string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();

  const shiftsQuery = useQuery({
    queryKey: queryKeys.hrm.shifts.list(orgId),
    queryFn: () => listShifts(orgId).then((r) => r.shifts),
  });
  const shifts = shiftsQuery.data ?? [];
  const shiftName = (id: string) => shifts.find((s) => s.id === id)?.name ?? id;

  const listKey = queryKeys.hrm.shiftAssignments.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listShiftAssignments(orgId).then((r) => r.assignments),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "Assign a shift",
      content: (
        <ShiftAssignmentForm
          shifts={shifts}
          employees={employees}
          departments={departments}
          onSave={async (payload) => {
            const created = await assignShift(orgId, payload);
            queryClient.setQueryData<WorkScheduleAssignment[]>(
              listKey,
              (old) => [created, ...(old ?? [])],
            );
            toast.success("Shift assigned.");
          }}
        />
      ),
    });
  };

  const handleRemove = async (id: string) => {
    try {
      await removeShiftAssignment(orgId, id);
      queryClient.setQueryData<WorkScheduleAssignment[]>(listKey, (old) =>
        (old ?? []).filter((a) => a.id !== id),
      );
      toast.success("Assignment removed.");
    } catch {
      toast.error("Failed to remove assignment.");
    }
  };

  const label = (a: WorkScheduleAssignment) => {
    if (a.assignee_type === "organization") return "Whole organization";
    if (a.assignee_type === "department") return deptName(a.assignee_id);
    return empName(a.assignee_id);
  };

  return (
    <>
      <div className="flex items-center justify-end mb-5">
        {canManage && (
          <button
            onClick={openCreate}
            disabled={shifts.length === 0}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 transition-colors"
          >
            <Plus size={15} />
            New assignment
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
            <Users size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No shift assignments yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((a) => (
            <div
              key={a.id}
              className="flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <Users size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">
                  {shiftName(a.shift_id)} → {label(a)}
                </p>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  From{" "}
                  {new Date(a.effective_date).toLocaleDateString("en-US", {
                    month: "short",
                    day: "numeric",
                    year: "numeric",
                  })}
                  {a.end_date
                    ? ` to ${new Date(a.end_date).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}`
                    : " (ongoing)"}
                </p>
              </div>
              {canManage && (
                <button
                  onClick={() => handleRemove(a.id)}
                  className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10 flex-shrink-0"
                >
                  <Trash2 size={14} />
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </>
  );
}
