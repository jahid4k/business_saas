// src/app/(dashboard)/[orgId]/hrm/warnings/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, MoreHorizontal, Loader2, ShieldAlert } from "lucide-react";
import gsap from "gsap";
import type {
  Employee,
  WarningType,
  EmployeeWarning,
  WarningStatus,
} from "@/types/hrm";
import {
  listAllWarnings,
  createWarning,
  issueWarning,
  acknowledgeWarning,
  appealWarning,
  closeWarning,
  cancelWarning,
} from "@/lib/hrm/warnings";
import { listWarningTypes } from "@/lib/hrm/warningtypes";
import { listEmployees } from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import WarningForm from "@/components/hrm/warnings/WarningForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";
import ApprovalInstanceView from "@/components/hrm/approvals/ApprovalInstanceView";

const STATUS_TONE: Record<WarningStatus, string> = {
  draft: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  pending_approval: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  issued: "bg-blue-500/10 text-blue-400 border-blue-500/20",
  acknowledged: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  appealed: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  closed: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  cancelled: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
};

function fmtDate(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export default function WarningsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { openDrawer } = useDrawer();
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();

  const [statusFilter, setStatusFilter] = useState<"all" | WarningStatus>(
    "all",
  );
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [appealId, setAppealId] = useState<string | null>(null);
  const [appealReason, setAppealReason] = useState("");
  const [closeId, setCloseId] = useState<string | null>(null);
  const [closeResolution, setCloseResolution] = useState("");
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const canManage = hasPermission("hrm.warnings.manage");
  const canIssue = hasPermission("hrm.warnings.issue");
  const canAcknowledge = hasPermission("hrm.warnings.acknowledge");
  const canClose = hasPermission("hrm.warnings.close");

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

  const typesQuery = useQuery({
    queryKey: queryKeys.hrm.warningTypes.list(orgId),
    queryFn: () => listWarningTypes(orgId).then((r) => r.warning_types),
  });
  const warningTypes = typesQuery.data ?? [];

  const listKey = queryKeys.hrm.warnings.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAllWarnings(orgId).then((r) => r.warnings),
  });
  const items = listQuery.data ?? [];
  const filtered =
    statusFilter === "all"
      ? items
      : items.filter((w) => w.status === statusFilter);

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".wn-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, statusFilter]);

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

  const update = (updated: EmployeeWarning) =>
    queryClient.setQueryData<EmployeeWarning[]>(listKey, (old) =>
      (old ?? []).map((w) => (w.id === updated.id ? updated : w)),
    );

  const openCreate = () => {
    openDrawer({
      title: "New warning",
      content: (
        <WarningForm
          employees={employees}
          warningTypes={warningTypes}
          onSave={async (employeeId, payload) => {
            const created = await createWarning(orgId, employeeId, payload);
            queryClient.setQueryData<EmployeeWarning[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Warning created as draft.");
          }}
        />
      ),
    });
  };

  const handleIssue = async (w: EmployeeWarning) => {
    try {
      update(await issueWarning(orgId, w.employee_id, w.id));
      toast.success("Warning issued.");
    } catch {
      toast.error("Failed to issue warning.");
    }
    setOpenMenuId(null);
  };

  const handleAcknowledge = async (w: EmployeeWarning) => {
    try {
      update(await acknowledgeWarning(orgId, w.employee_id, w.id));
      toast.success("Warning acknowledged.");
    } catch {
      toast.error("Failed to acknowledge.");
    }
    setOpenMenuId(null);
  };

  const handleAppeal = async () => {
    if (!appealId || !appealReason.trim()) return;
    const w = items.find((x) => x.id === appealId);
    if (!w) return;
    try {
      update(
        await appealWarning(orgId, w.employee_id, w.id, {
          reason: appealReason.trim(),
        }),
      );
      toast.success("Appeal submitted.");
    } catch {
      toast.error("Failed to submit appeal.");
    }
    setAppealId(null);
    setAppealReason("");
  };

  const handleClose = async () => {
    if (!closeId) return;
    const w = items.find((x) => x.id === closeId);
    if (!w) return;
    try {
      update(
        await closeWarning(
          orgId,
          w.employee_id,
          w.id,
          w.status === "appealed"
            ? { appeal_resolution: closeResolution.trim() || undefined }
            : undefined,
        ),
      );
      toast.success("Warning closed.");
    } catch {
      toast.error("Failed to close warning.");
    }
    setCloseId(null);
    setCloseResolution("");
  };

  const handleCancel = async (w: EmployeeWarning) => {
    try {
      update(await cancelWarning(orgId, w.employee_id, w.id));
      toast.success("Warning cancelled.");
    } catch {
      toast.error("Failed to cancel warning.");
    }
    setOpenMenuId(null);
  };

  const STATUS_TABS: ("all" | WarningStatus)[] = [
    "all",
    "draft",
    "issued",
    "acknowledged",
    "appealed",
    "closed",
  ];

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
            Warnings
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            {items.length} {items.length === 1 ? "warning" : "warnings"} total
          </p>
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New warning
          </button>
        )}
      </div>

      <div className="flex items-center gap-0.5 mb-6 border-b border-[var(--border)] flex-wrap">
        {STATUS_TABS.map((key) => {
          const count =
            key === "all"
              ? items.length
              : items.filter((w) => w.status === key).length;
          const active = statusFilter === key;
          return (
            <button
              key={key}
              onClick={() => setStatusFilter(key)}
              className={`px-3 py-2 text-sm font-medium -mb-px border-b-2 transition-colors ${
                active
                  ? "text-purple-400 border-purple-500"
                  : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
              }`}
            >
              {key === "all" ? "All" : key}
              {count > 0 && <span className="ml-1.5 text-xs">{count}</span>}
            </button>
          );
        })}
      </div>

      {appealId && (
        <div className="mb-4 p-4 rounded-xl bg-[var(--bg-surface)] border border-purple-500/30 space-y-2">
          <textarea
            value={appealReason}
            onChange={(e) => setAppealReason(e.target.value)}
            rows={2}
            placeholder="Reason for appeal"
            className="w-full px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)]"
          />
          <div className="flex gap-2">
            <button
              onClick={handleAppeal}
              className="px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500"
            >
              Submit appeal
            </button>
            <button
              onClick={() => setAppealId(null)}
              className="px-3.5 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {closeId && (
        <div className="mb-4 p-4 rounded-xl bg-[var(--bg-surface)] border border-purple-500/30 space-y-2">
          <textarea
            value={closeResolution}
            onChange={(e) => setCloseResolution(e.target.value)}
            rows={2}
            placeholder={
              items.find((w) => w.id === closeId)?.status === "appealed"
                ? "Appeal resolution (required)"
                : "Closing note (optional)"
            }
            className="w-full px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)]"
          />
          <div className="flex gap-2">
            <button
              onClick={handleClose}
              className="px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500"
            >
              Close warning
            </button>
            <button
              onClick={() => setCloseId(null)}
              className="px-3.5 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <ShieldAlert size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            {statusFilter === "all"
              ? "No warnings yet"
              : `No ${statusFilter} warnings`}
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((w) => {
            const menuOpen = openMenuId === w.id;
            const showMenu =
              (canIssue &&
                (w.status === "draft" || w.status === "pending_approval")) ||
              (canAcknowledge && w.status === "issued") ||
              (canClose &&
                (w.status === "issued" || w.status === "appealed")) ||
              (canManage && (w.status === "draft" || w.status === "issued")) ||
              !!w.approval_instance_id;

            return (
              <div
                key={w.id}
                className="wn-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <ShieldAlert size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {w.title}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {empName(w.employee_id)} · {w.warning_type_name} (severity{" "}
                    {w.severity_level}) · Incident {fmtDate(w.incident_date)}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                    {w.description}
                  </p>
                  {w.response_deadline && w.status === "issued" && (
                    <p className="text-xs text-amber-400 mt-0.5">
                      Respond by {fmtDate(w.response_deadline)}
                    </p>
                  )}
                  <div className="mt-2">
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full border font-medium ${STATUS_TONE[w.status]}`}
                    >
                      {w.status.replace("_", " ")}
                    </span>
                  </div>
                </div>

                {showMenu && (
                  <div
                    className="relative flex-shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(w.id, el);
                      else menuRefs.current.delete(w.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : w.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-44 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                        {w.approval_instance_id && (
                          <button
                            onClick={() => {
                              openDrawer({
                                title: "Approval status",
                                content: (
                                  <ApprovalInstanceView
                                    orgId={orgId}
                                    instanceId={w.approval_instance_id!}
                                  />
                                ),
                              });
                              setOpenMenuId(null);
                            }}
                            className="w-full flex items-center px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] text-left"
                          >
                            View approval
                          </button>
                        )}
                        {canIssue &&
                          (w.status === "draft" ||
                            w.status === "pending_approval") && (
                            <button
                              onClick={() => handleIssue(w)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-blue-400 hover:bg-blue-500/10 text-left"
                            >
                              Issue
                            </button>
                          )}
                        {canAcknowledge && w.status === "issued" && (
                          <button
                            onClick={() => handleAcknowledge(w)}
                            className="w-full flex items-center px-3.5 py-2.5 text-sm text-emerald-400 hover:bg-emerald-500/10 text-left"
                          >
                            Acknowledge
                          </button>
                        )}
                        {canAcknowledge &&
                          w.status === "issued" &&
                          w.can_employee_respond && (
                            <button
                              onClick={() => {
                                setAppealId(w.id);
                                setOpenMenuId(null);
                              }}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-amber-400 hover:bg-amber-500/10 text-left"
                            >
                              Appeal
                            </button>
                          )}
                        {canClose &&
                          (w.status === "issued" ||
                            w.status === "appealed") && (
                            <button
                              onClick={() => {
                                setCloseId(w.id);
                                setOpenMenuId(null);
                              }}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] text-left"
                            >
                              Close
                            </button>
                          )}
                        {canManage &&
                          (w.status === "draft" || w.status === "issued") && (
                            <button
                              onClick={() => handleCancel(w)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                            >
                              Cancel
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
    </div>
  );
}
