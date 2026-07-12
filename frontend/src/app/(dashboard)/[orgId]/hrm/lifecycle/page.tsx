// src/app/(dashboard)/[orgId]/hrm/lifecycle/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Loader2,
  TrendingUp,
  ArrowRightLeft,
  LogOut,
  UserX,
} from "lucide-react";
import gsap from "gsap";
import type {
  Employee,
  Department,
  Position,
  Promotion,
  Transfer,
  Resignation,
  Termination,
} from "@/types/hrm";
import {
  listAllPromotions,
  createPromotion,
  submitPromotion,
  cancelPromotion,
  applyPromotion,
  listAllTransfers,
  createTransfer,
  submitTransfer,
  cancelTransfer,
  applyTransfer,
  listAllResignations,
  submitResignation,
  withdrawResignation,
  acceptResignation,
  rejectResignation,
  listAllTerminations,
  createTermination,
  submitTermination,
  cancelTermination,
  applyTermination,
} from "@/lib/hrm/lifecycle";
import { listEmployees } from "@/lib/hrm/employees";
import { listDepartments } from "@/lib/hrm/departments";
import { listPositions } from "@/lib/hrm/positions";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import PromotionForm from "@/components/hrm/lifecycle/PromotionForm";
import TransferForm from "@/components/hrm/lifecycle/TransferForm";
import ResignationForm from "@/components/hrm/lifecycle/ResignationForm";
import TerminationForm from "@/components/hrm/lifecycle/TerminationForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type TabKey = "promotions" | "transfers" | "resignations" | "terminations";

const STATUS_META: Record<string, { label: string; badge: string }> = {
  draft: {
    label: "Draft",
    badge: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  },
  pending_approval: {
    label: "Pending approval",
    badge: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  },
  approved: {
    label: "Approved",
    badge: "bg-blue-500/10 text-blue-400 border-blue-500/20",
  },
  rejected: {
    label: "Rejected",
    badge: "bg-red-500/10 text-red-400 border-red-500/20",
  },
  cancelled: {
    label: "Cancelled",
    badge: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  },
  applied: {
    label: "Applied",
    badge: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  },
  submitted: {
    label: "Submitted",
    badge: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  },
  accepted: {
    label: "Accepted",
    badge: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  },
  withdrawn: {
    label: "Withdrawn",
    badge: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  },
};

function StatusBadge({ status }: { status: string }) {
  const meta = STATUS_META[status] ?? {
    label: status,
    badge: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  };
  return (
    <span
      className={`text-xs px-2 py-0.5 rounded-full border font-medium ${meta.badge}`}
    >
      {meta.label}
    </span>
  );
}

function fmtDate(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function useOutsideMenuClose(
  menuRefs: React.MutableRefObject<Map<string, HTMLDivElement>>,
  onClose: () => void,
) {
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      let inside = false;
      menuRefs.current.forEach((el) => {
        if (el.contains(e.target as Node)) inside = true;
      });
      if (!inside) onClose();
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [menuRefs, onClose]);
}

// ── Page ────────────────────────────────────────────────────────────────────
export default function LifecyclePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();

  const ALL_TABS: { key: TabKey; label: string; visible: boolean }[] = [
    {
      key: "promotions",
      label: "Promotions",
      visible: hasPermission("hrm.promotions.view"),
    },
    {
      key: "transfers",
      label: "Transfers",
      visible: hasPermission("hrm.transfers.view"),
    },
    {
      key: "resignations",
      label: "Resignations",
      visible: hasPermission("hrm.resignations.view"),
    },
    {
      key: "terminations",
      label: "Terminations",
      visible: hasPermission("hrm.terminations.view"),
    },
  ];
  const TABS = ALL_TABS.filter((t) => t.visible);

  const [tab, setTab] = useState<TabKey>(TABS[0]?.key ?? "promotions");

  const [employees, setEmployees] = useState<Employee[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);

  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
    listPositions(orgId)
      .then((r) => setPositions(r.positions))
      .catch(() => {});
  }, [orgId]);

  const empName = (id: string) => {
    const e = employees.find((x) => x.id === id);
    return e ? `${e.first_name} ${e.last_name ?? ""}`.trim() : "—";
  };

  if (TABS.length === 0) {
    return (
      <div className="p-6 md:p-8 max-w-5xl">
        <p className="text-sm text-[var(--text-muted)]">
          You don&apos;t have access to any lifecycle records.
        </p>
      </div>
    );
  }

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
            Lifecycle
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Promotions, transfers, resignations, terminations
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] w-fit flex-wrap">
        {TABS.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === t.key
                ? "bg-purple-600 text-white"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "promotions" && (
        <PromotionsView
          orgId={orgId}
          employees={employees}
          positions={positions}
          departments={departments}
          empName={empName}
          canManage={hasPermission("hrm.promotions.manage")}
          canApply={hasPermission("hrm.promotions.apply")}
        />
      )}
      {tab === "transfers" && (
        <TransfersView
          orgId={orgId}
          employees={employees}
          departments={departments}
          empName={empName}
          canManage={hasPermission("hrm.transfers.manage")}
          canApply={hasPermission("hrm.transfers.apply")}
        />
      )}
      {tab === "resignations" && (
        <ResignationsView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.resignations.manage")}
          canProcess={hasPermission("hrm.resignations.process")}
        />
      )}
      {tab === "terminations" && (
        <TerminationsView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.terminations.manage")}
          canApply={hasPermission("hrm.terminations.apply")}
        />
      )}
    </div>
  );
}

// ── Promotions ────────────────────────────────────────────────────────────
function PromotionsView({
  orgId,
  employees,
  positions,
  departments,
  empName,
  canManage,
  canApply,
}: {
  orgId: string;
  employees: Employee[];
  positions: Position[];
  departments: Department[];
  empName: (id: string) => string;
  canManage: boolean;
  canApply: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("all");
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.promotions.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAllPromotions(orgId).then((r) => r.promotions),
  });
  const items = listQuery.data ?? [];

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".lc-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, statusFilter]);

  useOutsideMenuClose(menuRefs, () => setOpenMenuId(null));

  const filtered =
    statusFilter === "all"
      ? items
      : items.filter((p) => p.status === statusFilter);
  const posTitle = (id?: string) =>
    positions.find((p) => p.id === id)?.title ?? "—";

  const openCreate = () => {
    openDrawer({
      title: "New promotion",
      content: (
        <PromotionForm
          employees={employees}
          positions={positions}
          departments={departments}
          onSave={async (employeeId, payload) => {
            const created = await createPromotion(orgId, employeeId, payload);
            queryClient.setQueryData<Promotion[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Promotion record created.");
          }}
        />
      ),
    });
  };

  const runAction = async (
    item: Promotion,
    action: "submit" | "cancel" | "apply",
  ) => {
    const fn =
      action === "submit"
        ? submitPromotion
        : action === "cancel"
          ? cancelPromotion
          : applyPromotion;
    try {
      const updated = await fn(orgId, item.employee_id, item.id);
      queryClient.setQueryData<Promotion[]>(listKey, (old) =>
        (old ?? []).map((p) => (p.id === updated.id ? updated : p)),
      );
      toast.success(
        action === "submit"
          ? "Promotion submitted."
          : action === "cancel"
            ? "Promotion cancelled."
            : "Promotion applied — employee updated.",
      );
    } catch {
      toast.error(`Failed to ${action} promotion.`);
    }
    setOpenMenuId(null);
  };

  const STATUS_TABS = ["all", "draft", "approved", "applied", "cancelled"];

  return (
    <>
      <div className="flex items-start justify-between mb-5">
        <div className="flex items-center gap-0.5 border-b border-[var(--border)]">
          {STATUS_TABS.map((key) => {
            const count =
              key === "all"
                ? items.length
                : items.filter((p) => p.status === key).length;
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
                {key === "all" ? "All" : (STATUS_META[key]?.label ?? key)}
                {count > 0 && <span className="ml-1.5 text-xs">{count}</span>}
              </button>
            );
          })}
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New promotion
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState icon={TrendingUp} label="No promotion records" />
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((item) => {
            const menuOpen = openMenuId === item.id;
            const showMenu =
              (canManage &&
                ["draft", "pending_approval", "approved"].includes(
                  item.status,
                )) ||
              (canApply && item.status === "approved");

            return (
              <div
                key={item.id}
                className="lc-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <TrendingUp size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {empName(item.employee_id)} →{" "}
                    {posTitle(item.to_position_id)}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    Effective {fmtDate(item.effective_date)}
                    {item.new_basic_pay
                      ? ` · New pay ${item.new_basic_pay}`
                      : ""}
                  </p>
                  {item.reason && (
                    <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                      {item.reason}
                    </p>
                  )}
                  <div className="mt-2">
                    <StatusBadge status={item.status} />
                  </div>
                </div>

                {showMenu && (
                  <div
                    className="relative flex-shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(item.id, el);
                      else menuRefs.current.delete(item.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : item.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                        {canManage && item.status === "draft" && (
                          <MenuItem onClick={() => runAction(item, "submit")}>
                            Submit
                          </MenuItem>
                        )}
                        {canApply && item.status === "approved" && (
                          <MenuItem
                            onClick={() => runAction(item, "apply")}
                            tone="emerald"
                          >
                            Apply
                          </MenuItem>
                        )}
                        {canManage &&
                          ["draft", "pending_approval", "approved"].includes(
                            item.status,
                          ) && (
                            <MenuItem
                              onClick={() => runAction(item, "cancel")}
                              tone="red"
                            >
                              Cancel
                            </MenuItem>
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

// ── Transfers ─────────────────────────────────────────────────────────────
function TransfersView({
  orgId,
  employees,
  departments,
  empName,
  canManage,
  canApply,
}: {
  orgId: string;
  employees: Employee[];
  departments: Department[];
  empName: (id: string) => string;
  canManage: boolean;
  canApply: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("all");
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.transfers.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAllTransfers(orgId).then((r) => r.transfers),
  });
  const items = listQuery.data ?? [];

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".lc-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, statusFilter]);

  useOutsideMenuClose(menuRefs, () => setOpenMenuId(null));

  const filtered =
    statusFilter === "all"
      ? items
      : items.filter((t) => t.status === statusFilter);
  const deptName = (id?: string) => departments.find((d) => d.id === id)?.name;

  const openCreate = () => {
    openDrawer({
      title: "New transfer",
      content: (
        <TransferForm
          employees={employees}
          departments={departments}
          onSave={async (employeeId, payload) => {
            const created = await createTransfer(orgId, employeeId, payload);
            queryClient.setQueryData<Transfer[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Transfer record created.");
          }}
        />
      ),
    });
  };

  const runAction = async (
    item: Transfer,
    action: "submit" | "cancel" | "apply",
  ) => {
    const fn =
      action === "submit"
        ? submitTransfer
        : action === "cancel"
          ? cancelTransfer
          : applyTransfer;
    try {
      const updated = await fn(orgId, item.employee_id, item.id);
      queryClient.setQueryData<Transfer[]>(listKey, (old) =>
        (old ?? []).map((t) => (t.id === updated.id ? updated : t)),
      );
      toast.success(
        action === "submit"
          ? "Transfer submitted."
          : action === "cancel"
            ? "Transfer cancelled."
            : "Transfer applied — employee updated.",
      );
    } catch {
      toast.error(`Failed to ${action} transfer.`);
    }
    setOpenMenuId(null);
  };

  const STATUS_TABS = ["all", "draft", "approved", "applied", "cancelled"];

  return (
    <>
      <div className="flex items-start justify-between mb-5">
        <div className="flex items-center gap-0.5 border-b border-[var(--border)]">
          {STATUS_TABS.map((key) => {
            const count =
              key === "all"
                ? items.length
                : items.filter((t) => t.status === key).length;
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
                {key === "all" ? "All" : (STATUS_META[key]?.label ?? key)}
                {count > 0 && <span className="ml-1.5 text-xs">{count}</span>}
              </button>
            );
          })}
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New transfer
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState icon={ArrowRightLeft} label="No transfer records" />
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((item) => {
            const menuOpen = openMenuId === item.id;
            const showMenu =
              (canManage &&
                ["draft", "pending_approval", "approved"].includes(
                  item.status,
                )) ||
              (canApply && item.status === "approved");
            const toLabel = [deptName(item.to_department_id), item.to_location]
              .filter(Boolean)
              .join(" · ");

            return (
              <div
                key={item.id}
                className="lc-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <ArrowRightLeft size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {empName(item.employee_id)} {toLabel ? `→ ${toLabel}` : ""}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {item.transfer_type} · Effective{" "}
                    {fmtDate(item.effective_date)}
                  </p>
                  {item.reason && (
                    <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                      {item.reason}
                    </p>
                  )}
                  <div className="mt-2">
                    <StatusBadge status={item.status} />
                  </div>
                </div>

                {showMenu && (
                  <div
                    className="relative flex-shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(item.id, el);
                      else menuRefs.current.delete(item.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : item.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                        {canManage && item.status === "draft" && (
                          <MenuItem onClick={() => runAction(item, "submit")}>
                            Submit
                          </MenuItem>
                        )}
                        {canApply && item.status === "approved" && (
                          <MenuItem
                            onClick={() => runAction(item, "apply")}
                            tone="emerald"
                          >
                            Apply
                          </MenuItem>
                        )}
                        {canManage &&
                          ["draft", "pending_approval", "approved"].includes(
                            item.status,
                          ) && (
                            <MenuItem
                              onClick={() => runAction(item, "cancel")}
                              tone="red"
                            >
                              Cancel
                            </MenuItem>
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

// ── Resignations ──────────────────────────────────────────────────────────
function ResignationsView({
  orgId,
  employees,
  empName,
  canManage,
  canProcess,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
  canProcess: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("all");
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.resignations.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAllResignations(orgId).then((r) => r.resignations),
  });
  const items = listQuery.data ?? [];

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".lc-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, statusFilter]);

  useOutsideMenuClose(menuRefs, () => setOpenMenuId(null));

  const filtered =
    statusFilter === "all"
      ? items
      : items.filter((r) => r.status === statusFilter);

  const openCreate = () => {
    openDrawer({
      title: "New resignation",
      content: (
        <ResignationForm
          employees={employees}
          onSave={async (employeeId, payload) => {
            const created = await submitResignation(orgId, employeeId, payload);
            queryClient.setQueryData<Resignation[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Resignation submitted.");
          }}
        />
      ),
    });
  };

  const runAction = async (
    item: Resignation,
    action: "withdraw" | "accept" | "reject",
  ) => {
    const fn =
      action === "withdraw"
        ? withdrawResignation
        : action === "accept"
          ? acceptResignation
          : rejectResignation;
    try {
      const updated = await fn(orgId, item.employee_id, item.id);
      queryClient.setQueryData<Resignation[]>(listKey, (old) =>
        (old ?? []).map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success(
        action === "withdraw"
          ? "Resignation withdrawn."
          : action === "accept"
            ? "Resignation accepted."
            : "Resignation rejected.",
      );
    } catch {
      toast.error(`Failed to ${action} resignation.`);
    }
    setOpenMenuId(null);
  };

  const STATUS_TABS = ["all", "submitted", "accepted", "rejected", "withdrawn"];

  return (
    <>
      <div className="flex items-start justify-between mb-5">
        <div className="flex items-center gap-0.5 border-b border-[var(--border)]">
          {STATUS_TABS.map((key) => {
            const count =
              key === "all"
                ? items.length
                : items.filter((r) => r.status === key).length;
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
                {key === "all" ? "All" : (STATUS_META[key]?.label ?? key)}
                {count > 0 && <span className="ml-1.5 text-xs">{count}</span>}
              </button>
            );
          })}
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New resignation
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState icon={LogOut} label="No resignation records" />
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((item) => {
            const menuOpen = openMenuId === item.id;
            const showMenu =
              item.status === "submitted" && (canManage || canProcess);

            return (
              <div
                key={item.id}
                className="lc-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <LogOut size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {empName(item.employee_id)}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    Resigned {fmtDate(item.resignation_date)} · Last day{" "}
                    {fmtDate(item.last_working_date)}
                    {item.is_notice_waived ? " · Notice waived" : ""}
                  </p>
                  <div className="flex items-center gap-3 mt-2 flex-wrap">
                    <StatusBadge status={item.status} />
                    {item.exit_interview_completed && (
                      <span className="text-xs text-[var(--text-muted)]">
                        Exit interview done
                      </span>
                    )}
                    {item.exit_clearance_completed && (
                      <span className="text-xs text-[var(--text-muted)]">
                        Clearance done
                      </span>
                    )}
                  </div>
                </div>

                {showMenu && (
                  <div
                    className="relative flex-shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(item.id, el);
                      else menuRefs.current.delete(item.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : item.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                        {canProcess && (
                          <MenuItem
                            onClick={() => runAction(item, "accept")}
                            tone="emerald"
                          >
                            Accept
                          </MenuItem>
                        )}
                        {canProcess && (
                          <MenuItem
                            onClick={() => runAction(item, "reject")}
                            tone="red"
                          >
                            Reject
                          </MenuItem>
                        )}
                        {canManage && (
                          <MenuItem onClick={() => runAction(item, "withdraw")}>
                            Withdraw
                          </MenuItem>
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

// ── Terminations ──────────────────────────────────────────────────────────
function TerminationsView({
  orgId,
  employees,
  empName,
  canManage,
  canApply,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
  canApply: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("all");
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.terminations.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAllTerminations(orgId).then((r) => r.terminations),
  });
  const items = listQuery.data ?? [];

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".lc-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, statusFilter]);

  useOutsideMenuClose(menuRefs, () => setOpenMenuId(null));

  const filtered =
    statusFilter === "all"
      ? items
      : items.filter((t) => t.status === statusFilter);

  const openCreate = () => {
    openDrawer({
      title: "New termination",
      content: (
        <TerminationForm
          employees={employees}
          onSave={async (employeeId, payload) => {
            const created = await createTermination(orgId, employeeId, payload);
            queryClient.setQueryData<Termination[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Termination record created.");
          }}
        />
      ),
    });
  };

  const runAction = async (
    item: Termination,
    action: "submit" | "cancel" | "apply",
  ) => {
    const fn =
      action === "submit"
        ? submitTermination
        : action === "cancel"
          ? cancelTermination
          : applyTermination;
    try {
      const updated = await fn(orgId, item.employee_id, item.id);
      queryClient.setQueryData<Termination[]>(listKey, (old) =>
        (old ?? []).map((t) => (t.id === updated.id ? updated : t)),
      );
      toast.success(
        action === "submit"
          ? "Termination submitted."
          : action === "cancel"
            ? "Termination cancelled."
            : "Termination applied — employee is now terminated.",
      );
    } catch {
      toast.error(`Failed to ${action} termination.`);
    }
    setOpenMenuId(null);
  };

  const STATUS_TABS = ["all", "draft", "approved", "applied", "cancelled"];

  return (
    <>
      <div className="flex items-start justify-between mb-5">
        <div className="flex items-center gap-0.5 border-b border-[var(--border)]">
          {STATUS_TABS.map((key) => {
            const count =
              key === "all"
                ? items.length
                : items.filter((t) => t.status === key).length;
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
                {key === "all" ? "All" : (STATUS_META[key]?.label ?? key)}
                {count > 0 && <span className="ml-1.5 text-xs">{count}</span>}
              </button>
            );
          })}
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New termination
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState icon={UserX} label="No termination records" />
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((item) => {
            const menuOpen = openMenuId === item.id;
            const showMenu =
              (canManage &&
                ["draft", "pending_approval", "approved"].includes(
                  item.status,
                )) ||
              (canApply && item.status === "approved");

            return (
              <div
                key={item.id}
                className="lc-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <UserX size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {empName(item.employee_id)}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {item.termination_type} · Last day{" "}
                    {fmtDate(item.last_working_date)}
                    {item.severance_amount
                      ? ` · Severance ${item.severance_amount} ${item.severance_currency}`
                      : ""}
                  </p>
                  {item.reason && (
                    <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                      {item.reason}
                    </p>
                  )}
                  <div className="mt-2">
                    <StatusBadge status={item.status} />
                  </div>
                </div>

                {showMenu && (
                  <div
                    className="relative flex-shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(item.id, el);
                      else menuRefs.current.delete(item.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : item.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                        {canManage && item.status === "draft" && (
                          <MenuItem onClick={() => runAction(item, "submit")}>
                            Submit
                          </MenuItem>
                        )}
                        {canApply && item.status === "approved" && (
                          <MenuItem
                            onClick={() => runAction(item, "apply")}
                            tone="emerald"
                          >
                            Apply
                          </MenuItem>
                        )}
                        {canManage &&
                          ["draft", "pending_approval", "approved"].includes(
                            item.status,
                          ) && (
                            <MenuItem
                              onClick={() => runAction(item, "cancel")}
                              tone="red"
                            >
                              Cancel
                            </MenuItem>
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

// ── Shared bits ───────────────────────────────────────────────────────────
function EmptyState({
  icon: Icon,
  label,
}: {
  icon: typeof TrendingUp;
  label: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
        <Icon size={20} className="text-[var(--text-muted)]" />
      </div>
      <p className="text-sm font-medium text-[var(--text-secondary)]">
        {label}
      </p>
    </div>
  );
}

function MenuItem({
  children,
  onClick,
  tone = "default",
}: {
  children: React.ReactNode;
  onClick: () => void;
  tone?: "default" | "emerald" | "red";
}) {
  const toneCls =
    tone === "emerald"
      ? "text-emerald-400 hover:bg-emerald-500/10"
      : tone === "red"
        ? "text-red-400 hover:bg-red-500/10"
        : "text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]";
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm transition-colors text-left ${toneCls}`}
    >
      {children}
    </button>
  );
}
