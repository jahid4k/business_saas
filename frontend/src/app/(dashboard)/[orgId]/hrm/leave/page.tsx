// src/app/(dashboard)/[orgId]/hrm/leave/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Pencil,
  Trash2,
  CalendarClock,
  Loader2,
  Check,
  X,
  Ban,
} from "lucide-react";
import gsap from "gsap";
import type {
  LeaveRequest,
  LeaveRequestStatus,
  LeaveType,
  Employee,
} from "@/types/hrm";
import {
  listLeaveRequests,
  createLeaveRequest,
  approveLeaveRequest,
  rejectLeaveRequest,
  cancelLeaveRequest,
  deleteLeaveRequest,
  listLeaveTypes,
  createLeaveType,
  updateLeaveType,
  deleteLeaveType,
} from "@/lib/hrm/leave";
import { listEmployees } from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import LeaveRequestForm from "@/components/hrm/leave/LeaveRequestForm";
import LeaveTypeForm from "@/components/hrm/leave/LeaveTypeForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type TabKey = "requests" | "types";
type StatusFilterKey = "all" | LeaveRequestStatus;

const STATUS_STYLE: Record<
  LeaveRequestStatus,
  { label: string; badge: string }
> = {
  pending: {
    label: "Pending",
    badge: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  },
  approved: {
    label: "Approved",
    badge: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  },
  rejected: {
    label: "Rejected",
    badge: "bg-red-500/10 text-red-400 border-red-500/20",
  },
  cancelled: {
    label: "Cancelled",
    badge: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  },
};

function formatDate(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

// ── Page ────────────────────────────────────────────────────────────────────
export default function LeavePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();

  const [tab, setTab] = useState<TabKey>("requests");

  const canRequest = hasPermission("hrm.leave.request");
  const canApprove = hasPermission("hrm.leave.approve");
  const canDeleteRequest = hasPermission("hrm.leave.delete");
  const canCreateType = hasPermission("hrm.leave.create");
  const canUpdateType = hasPermission("hrm.leave.update");
  const canDeleteType = hasPermission("hrm.leave.delete");

  const [employees, setEmployees] = useState<Employee[]>([]);
  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
  }, [orgId]);

  const typesKey = queryKeys.hrm.leaveTypes.list(orgId);
  const typesQuery = useQuery({
    queryKey: typesKey,
    queryFn: () => listLeaveTypes(orgId).then((r) => r.leave_types),
  });
  const leaveTypes = typesQuery.data ?? [];

  const empName = (id: string) => {
    const e = employees.find((x) => x.id === id);
    return e ? `${e.first_name} ${e.last_name ?? ""}`.trim() : "—";
  };
  const typeName = (id: string) =>
    leaveTypes.find((t) => t.id === id)?.name ?? "—";

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
            Leave
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Requests and leave type configuration
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] w-fit">
        {(["requests", "types"] as TabKey[]).map((key) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === key
                ? "bg-purple-600 text-white"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            {key === "requests" ? "Requests" : "Leave types"}
          </button>
        ))}
      </div>

      {tab === "requests" ? (
        <RequestsView
          orgId={orgId}
          employees={employees}
          leaveTypes={leaveTypes}
          empName={empName}
          typeName={typeName}
          canRequest={canRequest}
          canApprove={canApprove}
          canDelete={canDeleteRequest}
        />
      ) : (
        <TypesView
          orgId={orgId}
          leaveTypes={leaveTypes}
          typesKey={typesKey}
          canCreate={canCreateType}
          canUpdate={canUpdateType}
          canDelete={canDeleteType}
        />
      )}
    </div>
  );
}

// ── Requests view ─────────────────────────────────────────────────────────
function RequestsView({
  orgId,
  employees,
  leaveTypes,
  empName,
  typeName,
  canRequest,
  canApprove,
  canDelete,
}: {
  orgId: string;
  employees: Employee[];
  leaveTypes: LeaveType[];
  empName: (id: string) => string;
  typeName: (id: string) => string;
  canRequest: boolean;
  canApprove: boolean;
  canDelete: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();

  const [statusFilter, setStatusFilter] = useState<StatusFilterKey>("all");
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [rejectConfirm, setRejectConfirm] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const reqKey = queryKeys.hrm.leaveRequests.list(orgId);
  const reqQuery = useQuery({
    queryKey: reqKey,
    queryFn: () =>
      listLeaveRequests(orgId, { limit: 200 }).then((r) => r.requests),
  });
  const requests = reqQuery.data ?? [];

  useEffect(() => {
    if (reqQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".req-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [reqQuery.isPending, statusFilter]);

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

  const filtered =
    statusFilter === "all"
      ? requests
      : requests.filter((r) => r.status === statusFilter);

  const openCreate = () => {
    openDrawer({
      title: "New leave request",
      content: (
        <LeaveRequestForm
          employees={employees}
          leaveTypes={leaveTypes}
          onSave={async (values) => {
            const created = await createLeaveRequest(orgId, values);
            queryClient.setQueryData<LeaveRequest[]>(reqKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Leave request submitted.");
          }}
        />
      ),
    });
  };

  const handleApprove = async (reqId: string) => {
    try {
      const updated = await approveLeaveRequest(orgId, reqId);
      queryClient.setQueryData<LeaveRequest[]>(reqKey, (old) =>
        (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success("Leave request approved.");
    } catch {
      toast.error("Failed to approve request.");
    }
    setOpenMenuId(null);
  };

  const handleReject = async (reqId: string) => {
    try {
      const updated = await rejectLeaveRequest(orgId, reqId);
      queryClient.setQueryData<LeaveRequest[]>(reqKey, (old) =>
        (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success("Leave request rejected.");
    } catch {
      toast.error("Failed to reject request.");
    }
    setRejectConfirm(null);
    setOpenMenuId(null);
  };

  const handleCancel = async (reqId: string) => {
    try {
      const updated = await cancelLeaveRequest(orgId, reqId);
      queryClient.setQueryData<LeaveRequest[]>(reqKey, (old) =>
        (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success("Leave request cancelled.");
    } catch {
      toast.error("Failed to cancel request.");
    }
    setOpenMenuId(null);
  };

  const handleDelete = async (reqId: string) => {
    try {
      await deleteLeaveRequest(orgId, reqId);
      queryClient.setQueryData<LeaveRequest[]>(reqKey, (old) =>
        (old ?? []).filter((r) => r.id !== reqId),
      );
      toast.success("Leave request deleted.");
    } catch {
      toast.error("Failed to delete request.");
    }
    setDeleteConfirm(null);
    setOpenMenuId(null);
  };

  const STATUS_TABS: StatusFilterKey[] = [
    "all",
    "pending",
    "approved",
    "rejected",
    "cancelled",
  ];

  return (
    <>
      <div className="flex items-start justify-end mb-5">
        {canRequest && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New request
          </button>
        )}
      </div>

      {reqQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          Failed to load leave requests. Please refresh.
        </div>
      )}

      <div className="flex items-center gap-0.5 mb-6 border-b border-[var(--border)]">
        {STATUS_TABS.map((key) => {
          const count =
            key === "all"
              ? requests.length
              : requests.filter((r) => r.status === key).length;
          const active = statusFilter === key;
          return (
            <button
              key={key}
              onClick={() => setStatusFilter(key)}
              className={`flex items-center gap-2 px-3.5 py-2.5 text-sm font-medium -mb-px border-b-2 transition-colors ${
                active
                  ? "text-purple-400 border-purple-500"
                  : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
              }`}
            >
              {key === "all" ? "All" : STATUS_STYLE[key].label}
              {count > 0 && (
                <span
                  className={`text-xs px-1.5 py-0.5 rounded-full min-w-[20px] text-center ${
                    active
                      ? "bg-purple-500/15 text-purple-400"
                      : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"
                  }`}
                >
                  {count}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {reqQuery.isPending ? (
        <div className="flex items-center justify-center py-20">
          <div className="flex items-center gap-3 text-sm text-[var(--text-muted)]">
            <Loader2 size={16} className="animate-spin text-purple-500" />
            Loading requests…
          </div>
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <CalendarClock size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
            {statusFilter === "all"
              ? "No leave requests yet"
              : `No ${statusFilter} requests`}
          </p>
          <p className="text-xs text-[var(--text-muted)] mb-4">
            {canRequest && statusFilter === "all"
              ? "Submit the first leave request."
              : "Nothing here for this filter."}
          </p>
          {canRequest && statusFilter === "all" && (
            <button
              onClick={openCreate}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={14} />
              New request
            </button>
          )}
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((req) => {
            const confirmingDelete = deleteConfirm === req.id;
            const confirmingReject = rejectConfirm === req.id;
            const menuOpen = openMenuId === req.id;
            const s = STATUS_STYLE[req.status];
            const showMenu =
              ((req.status === "pending" || req.status === "approved") &&
                canRequest) ||
              canDelete;

            return (
              <div
                key={req.id}
                className="req-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <CalendarClock size={15} />
                </div>

                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {empName(req.employee_id)} · {typeName(req.leave_type_id)}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {formatDate(req.start_date)} – {formatDate(req.end_date)} ·{" "}
                    {req.total_days} {req.total_days === 1 ? "day" : "days"}
                  </p>
                  {req.reason && (
                    <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                      {req.reason}
                    </p>
                  )}
                  <div className="flex items-center gap-3 mt-2 flex-wrap">
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full border font-medium ${s.badge}`}
                    >
                      {s.label}
                    </span>
                  </div>
                </div>

                {confirmingDelete ? (
                  <div className="flex items-center gap-2 flex-shrink-0 pt-0.5">
                    <span className="text-xs text-[var(--text-muted)]">
                      Delete?
                    </span>
                    <button
                      onClick={() => handleDelete(req.id)}
                      className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                    >
                      Yes
                    </button>
                    <button
                      onClick={() => setDeleteConfirm(null)}
                      className="px-2.5 py-1 rounded-md text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                    >
                      No
                    </button>
                  </div>
                ) : confirmingReject ? (
                  <div className="flex items-center gap-2 flex-shrink-0 pt-0.5">
                    <span className="text-xs text-[var(--text-muted)]">
                      Reject?
                    </span>
                    <button
                      onClick={() => handleReject(req.id)}
                      className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                    >
                      Yes
                    </button>
                    <button
                      onClick={() => setRejectConfirm(null)}
                      className="px-2.5 py-1 rounded-md text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                    >
                      No
                    </button>
                  </div>
                ) : (
                  <div className="flex items-center gap-1.5 flex-shrink-0">
                    {req.status === "pending" && canApprove && (
                      <>
                        <button
                          onClick={() => handleApprove(req.id)}
                          title="Approve"
                          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-emerald-400 hover:bg-emerald-500/10 transition-all"
                        >
                          <Check size={15} />
                        </button>
                        <button
                          onClick={() => setRejectConfirm(req.id)}
                          title="Reject"
                          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-red-400 hover:bg-red-500/10 transition-all"
                        >
                          <X size={15} />
                        </button>
                      </>
                    )}

                    {showMenu && (
                      <div
                        className="relative"
                        ref={(el) => {
                          if (el) menuRefs.current.set(req.id, el);
                          else menuRefs.current.delete(req.id);
                        }}
                      >
                        <button
                          onClick={() =>
                            setOpenMenuId(menuOpen ? null : req.id)
                          }
                          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                        >
                          <MoreHorizontal size={15} />
                        </button>
                        {menuOpen && (
                          <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                            {(req.status === "pending" ||
                              req.status === "approved") &&
                              canRequest && (
                                <button
                                  onClick={() => handleCancel(req.id)}
                                  className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-amber-400 hover:bg-amber-500/10 transition-colors text-left"
                                >
                                  <Ban size={13} />
                                  Cancel
                                </button>
                              )}
                            {canDelete && (
                              <button
                                onClick={() => {
                                  setDeleteConfirm(req.id);
                                  setOpenMenuId(null);
                                }}
                                className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 transition-colors text-left"
                              >
                                <Trash2 size={13} />
                                Delete
                              </button>
                            )}
                          </div>
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

      {!reqQuery.isPending && filtered.length > 0 && (
        <p className="mt-5 text-xs text-[var(--text-muted)]">
          Showing {filtered.length} of {requests.length}{" "}
          {requests.length === 1 ? "request" : "requests"}
        </p>
      )}
    </>
  );
}

// ── Leave types view ──────────────────────────────────────────────────────
function TypesView({
  orgId,
  leaveTypes,
  typesKey,
  canCreate,
  canUpdate,
  canDelete,
}: {
  orgId: string;
  leaveTypes: LeaveType[];
  typesKey: QueryKey;
  canCreate: boolean;
  canUpdate: boolean;
  canDelete: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();

  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  useEffect(() => {
    if (!listRef.current) return;
    const rows = listRef.current.querySelectorAll(".type-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [leaveTypes.length]);

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
      title: "New leave type",
      content: (
        <LeaveTypeForm
          onSave={async (values) => {
            const created = await createLeaveType(orgId, values);
            queryClient.setQueryData<LeaveType[]>(typesKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Leave type created.");
          }}
        />
      ),
    });
  };

  const openEdit = (lt: LeaveType) => {
    setOpenMenuId(null);
    openDrawer({
      title: "Edit leave type",
      content: (
        <LeaveTypeForm
          leaveType={lt}
          onSave={async (values) => {
            const updated = await updateLeaveType(orgId, lt.id, values);
            queryClient.setQueryData<LeaveType[]>(typesKey, (old) =>
              (old ?? []).map((t) => (t.id === updated.id ? updated : t)),
            );
            toast.success("Leave type updated.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (typeId: string) => {
    try {
      await deleteLeaveType(orgId, typeId);
      queryClient.setQueryData<LeaveType[]>(typesKey, (old) =>
        (old ?? []).filter((t) => t.id !== typeId),
      );
      toast.success("Leave type deleted.");
    } catch {
      toast.error("Failed to delete leave type.");
    }
    setDeleteConfirm(null);
    setOpenMenuId(null);
  };

  return (
    <>
      <div className="flex items-start justify-between mb-5">
        <p className="text-sm text-[var(--text-muted)]">
          {leaveTypes.length}{" "}
          {leaveTypes.length === 1 ? "leave type" : "leave types"} configured
        </p>
        {canCreate && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New leave type
          </button>
        )}
      </div>

      {leaveTypes.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <CalendarClock size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
            No leave types yet
          </p>
          <p className="text-xs text-[var(--text-muted)] mb-4">
            {canCreate
              ? "Create your first leave type — e.g. Annual, Sick, Unpaid."
              : "Nothing configured yet."}
          </p>
          {canCreate && (
            <button
              onClick={openCreate}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={14} />
              New leave type
            </button>
          )}
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {leaveTypes.map((lt) => {
            const confirming = deleteConfirm === lt.id;
            const menuOpen = openMenuId === lt.id;

            return (
              <div
                key={lt.id}
                className="type-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <CalendarClock size={15} />
                </div>

                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {lt.name}
                  </p>
                  {lt.description && (
                    <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                      {lt.description}
                    </p>
                  )}
                  <div className="flex items-center gap-3 mt-2 flex-wrap">
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full border font-medium ${
                        lt.is_active
                          ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                          : "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"
                      }`}
                    >
                      {lt.is_active ? "Active" : "Inactive"}
                    </span>
                    <span className="text-xs text-[var(--text-muted)]">
                      {lt.max_days_per_year > 0
                        ? `${lt.max_days_per_year} days/yr`
                        : "Unlimited"}
                    </span>
                    <span className="text-xs text-[var(--text-muted)]">
                      {lt.is_paid ? "Paid" : "Unpaid"}
                    </span>
                    {!lt.requires_approval && (
                      <span className="text-xs text-[var(--text-muted)]">
                        No approval needed
                      </span>
                    )}
                  </div>
                </div>

                {confirming ? (
                  <div className="flex items-center gap-2 flex-shrink-0 pt-0.5">
                    <span className="text-xs text-[var(--text-muted)]">
                      Delete?
                    </span>
                    <button
                      onClick={() => handleDelete(lt.id)}
                      className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                    >
                      Yes
                    </button>
                    <button
                      onClick={() => setDeleteConfirm(null)}
                      className="px-2.5 py-1 rounded-md text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                    >
                      No
                    </button>
                  </div>
                ) : (
                  (canUpdate || canDelete) && (
                    <div
                      className="relative flex-shrink-0"
                      ref={(el) => {
                        if (el) menuRefs.current.set(lt.id, el);
                        else menuRefs.current.delete(lt.id);
                      }}
                    >
                      <button
                        onClick={() => setOpenMenuId(menuOpen ? null : lt.id)}
                        className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                      >
                        <MoreHorizontal size={15} />
                      </button>
                      {menuOpen && (
                        <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                          {canUpdate && (
                            <button
                              onClick={() => openEdit(lt)}
                              className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors text-left"
                            >
                              <Pencil size={13} />
                              Edit
                            </button>
                          )}
                          {canDelete && (
                            <button
                              onClick={() => {
                                setDeleteConfirm(lt.id);
                                setOpenMenuId(null);
                              }}
                              className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 transition-colors text-left"
                            >
                              <Trash2 size={13} />
                              Delete
                            </button>
                          )}
                        </div>
                      )}
                    </div>
                  )
                )}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}
