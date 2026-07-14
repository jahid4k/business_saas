// src/app/(dashboard)/[orgId]/hrm/setup/warning-types/page.tsx
"use client";

import { useState, use } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Loader2,
  ShieldAlert,
  Bell,
  Pencil,
  Trash2,
} from "lucide-react";
import type { WarningType, WarningEscalationRule } from "@/types/hrm";
import {
  listWarningTypes,
  createWarningType,
  updateWarningType,
  deleteWarningType,
  listEscalationRules,
  createEscalationRule,
  deleteEscalationRule,
} from "@/lib/hrm/warningtypes";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import WarningTypeForm from "@/components/hrm/warningtypes/WarningTypeForm";
import EscalationRuleForm from "@/components/hrm/warningtypes/EscalationRuleForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type TabKey = "types" | "escalations";

export default function WarningTypesPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  return <WarningTypesContent orgId={orgId} />;
}

function WarningTypesContent({ orgId }: { orgId: string }) {
  const { hasPermission } = usePermissionStore();
  const [tab, setTab] = useState<TabKey>("types");
  const canManage = hasPermission("hrm.warning_types.manage");

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
            Warning Types
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Categories and escalation rules for disciplinary warnings
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] w-fit">
        {(["types", "escalations"] as TabKey[]).map((key) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === key
                ? "bg-purple-600 text-white"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            {key === "types" ? "Warning Types" : "Escalation Rules"}
          </button>
        ))}
      </div>

      {tab === "types" ? (
        <TypesView orgId={orgId} canManage={canManage} />
      ) : (
        <EscalationsView orgId={orgId} canManage={canManage} />
      )}
    </div>
  );
}

// ── Types ─────────────────────────────────────────────────────────────────
function TypesView({
  orgId,
  canManage,
}: {
  orgId: string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  const listKey = queryKeys.hrm.warningTypes.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listWarningTypes(orgId).then((r) => r.warning_types),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New warning type",
      content: (
        <WarningTypeForm
          onSave={async (payload) => {
            const created = await createWarningType(orgId, payload);
            queryClient.setQueryData<WarningType[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Warning type created.");
          }}
        />
      ),
    });
  };

  const openEdit = (wt: WarningType) => {
    setOpenMenuId(null);
    openDrawer({
      title: "Edit warning type",
      content: (
        <WarningTypeForm
          warningType={wt}
          onSave={async (payload) => {
            const updated = await updateWarningType(orgId, wt.id, payload);
            queryClient.setQueryData<WarningType[]>(listKey, (old) =>
              (old ?? []).map((w) => (w.id === updated.id ? updated : w)),
            );
            toast.success("Warning type updated.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (typeId: string) => {
    try {
      await deleteWarningType(orgId, typeId);
      queryClient.setQueryData<WarningType[]>(listKey, (old) =>
        (old ?? []).filter((w) => w.id !== typeId),
      );
      toast.success("Warning type deleted.");
    } catch {
      toast.error("Failed to delete warning type.");
    }
    setDeleteConfirm(null);
    setOpenMenuId(null);
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
            New warning type
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
            <ShieldAlert size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No warning types yet
          </p>
          <p className="text-xs text-[var(--text-muted)] mt-1">
            Once created, Warnings can be issued referencing these types.
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((wt) => {
            const menuOpen = openMenuId === wt.id;
            return (
              <div
                key={wt.id}
                className={`group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] ${menuOpen ? "z-30 border-[var(--text-muted)]/30" : "z-10"}`}
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <ShieldAlert size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-[var(--text-primary)]">
                    {wt.name}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    Severity {wt.severity_level} ·{" "}
                    {wt.can_be_issued_by.join(", ") || "any role"}
                    {wt.requires_hr_approval ? " · HR approval required" : ""}
                    {wt.valid_duration_days > 0
                      ? ` · Valid ${wt.valid_duration_days} days`
                      : " · Permanent"}
                  </p>
                  <span
                    className={`inline-block mt-2 text-xs px-2 py-0.5 rounded-full border font-medium ${
                      wt.is_active
                        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                        : "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"
                    }`}
                  >
                    {wt.is_active ? "Active" : "Inactive"}
                  </span>
                </div>
                {canManage &&
                  (deleteConfirm === wt.id ? (
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <span className="text-xs text-[var(--text-muted)]">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(wt.id)}
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
                    <div className="relative flex-shrink-0">
                      <button
                        onClick={() => setOpenMenuId(menuOpen ? null : wt.id)}
                        className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                      >
                        <MoreHorizontal size={15} />
                      </button>
                      {menuOpen && (
                        <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                          <button
                            onClick={() => openEdit(wt)}
                            className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] text-left"
                          >
                            <Pencil size={13} />
                            Edit
                          </button>
                          <button
                            onClick={() => setDeleteConfirm(wt.id)}
                            className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                          >
                            <Trash2 size={13} />
                            Delete
                          </button>
                        </div>
                      )}
                    </div>
                  ))}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}

// ── Escalations ───────────────────────────────────────────────────────────
function EscalationsView({
  orgId,
  canManage,
}: {
  orgId: string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const typesQuery = useQuery({
    queryKey: queryKeys.hrm.warningTypes.list(orgId),
    queryFn: () => listWarningTypes(orgId).then((r) => r.warning_types),
  });
  const warningTypes = typesQuery.data ?? [];
  const typeName = (id: string) =>
    warningTypes.find((t) => t.id === id)?.name ?? "—";

  const listKey = queryKeys.hrm.escalationRules.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listEscalationRules(orgId).then((r) => r.rules),
  });
  const items = listQuery.data ?? [];

  const ACTION_LABEL: Record<string, string> = {
    notify_hr: "Notify HR",
    notify_management: "Notify management",
    flag_termination_review: "Flag for termination review",
  };

  const openCreate = () => {
    openDrawer({
      title: "New escalation rule",
      content: (
        <EscalationRuleForm
          warningTypes={warningTypes}
          onSave={async (payload) => {
            const created = await createEscalationRule(orgId, payload);
            queryClient.setQueryData<WarningEscalationRule[]>(
              listKey,
              (old) => [created, ...(old ?? [])],
            );
            toast.success("Escalation rule created.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (ruleId: string) => {
    try {
      await deleteEscalationRule(orgId, ruleId);
      queryClient.setQueryData<WarningEscalationRule[]>(listKey, (old) =>
        (old ?? []).filter((r) => r.id !== ruleId),
      );
      toast.success("Escalation rule deleted.");
    } catch {
      toast.error("Failed to delete rule.");
    }
    setDeleteConfirm(null);
  };

  return (
    <>
      <div className="flex items-center justify-end mb-5">
        {canManage && (
          <button
            onClick={openCreate}
            disabled={warningTypes.length === 0}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 transition-colors"
          >
            <Plus size={15} />
            New rule
          </button>
        )}
      </div>

      {warningTypes.length === 0 && !typesQuery.isPending && (
        <p className="text-sm text-[var(--text-muted)] mb-5">
          Create a warning type first — escalation rules need one to trigger on.
        </p>
      )}

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <Bell size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No escalation rules yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((r) => (
            <div
              key={r.id}
              className="flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <Bell size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">
                  {r.trigger_count}× {typeName(r.trigger_warning_type_id)}
                  {r.within_days > 0
                    ? ` within ${r.within_days} days`
                    : " (all-time)"}
                </p>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {ACTION_LABEL[r.action]} · notifies{" "}
                  {r.notification_roles.join(", ") || "no one configured"}
                </p>
              </div>
              {canManage &&
                (deleteConfirm === r.id ? (
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <span className="text-xs text-[var(--text-muted)]">
                      Delete?
                    </span>
                    <button
                      onClick={() => handleDelete(r.id)}
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
                  <button
                    onClick={() => setDeleteConfirm(r.id)}
                    className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10 flex-shrink-0"
                  >
                    <Trash2 size={14} />
                  </button>
                ))}
            </div>
          ))}
        </div>
      )}
    </>
  );
}
