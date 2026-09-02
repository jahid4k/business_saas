// src/app/(dashboard)/[orgId]/hrm/setup/approvals/page.tsx
"use client";

import { use, useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Loader2, Workflow, Trash2 } from "lucide-react";
import type { Employee, ApprovalTemplate, ApprovalInstance } from "@/types/hrm";
import {
  listApprovalTemplates,
  createApprovalTemplate,
  deleteApprovalTemplate,
  listApprovalInstances,
} from "@/lib/hrm/approvals";
import { listEmployees } from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import ApprovalTemplateForm from "@/components/hrm/approvals/ApprovalTemplateForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

const APPROVER_LABEL: Record<string, string> = {
  reporting_manager: "Reporting manager",
  dept_head: "Department head",
  role: "Role",
  specific_user: "Specific person",
};

export default function ApprovalsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { openDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();
  const canManage = hasPermission("hrm.approvals.manage");

  const [employees, setEmployees] = useState<Employee[]>([]);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"templates" | "instances">(
    "templates",
  );

  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
  }, [orgId]);

  const listKey = queryKeys.hrm.approvalTemplates.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listApprovalTemplates(orgId).then((r) => r.templates),
  });
  const items = listQuery.data ?? [];

  const instancesKey = queryKeys.hrm.approvalInstances.list(orgId);
  const instancesQuery = useQuery({
    queryKey: instancesKey,
    queryFn: () => listApprovalInstances(orgId).then((r) => r.instances),
  });
  const instances = instancesQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New approval template",
      content: (
        <ApprovalTemplateForm
          employees={employees}
          onSave={async (payload) => {
            const created = await createApprovalTemplate(orgId, payload);
            queryClient.setQueryData<ApprovalTemplate[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Approval template created.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (templateId: string) => {
    try {
      await deleteApprovalTemplate(orgId, templateId);
      queryClient.setQueryData<ApprovalTemplate[]>(listKey, (old) =>
        (old ?? []).filter((t) => t.id !== templateId),
      );
      toast.success("Template deleted.");
    } catch {
      toast.error("Failed to delete template.");
    }
    setDeleteConfirm(null);
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
            Approval Chains
          </h1>
          <p className="text-sm text-(--text-muted)">
            Multi-level approval workflows for promotions, transfers,
            terminations, warnings, and more
          </p>
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New template
          </button>
        )}
      </div>

      <div className="flex items-center gap-6 border-b border-(--border) mb-6">
        <button
          onClick={() => setActiveTab("templates")}
          className={`py-3 text-sm font-medium border-b-2 transition-colors ${
            activeTab === "templates"
              ? "border-purple-500 text-purple-400"
              : "border-transparent text-(--text-muted) hover:text-(--text-secondary)"
          }`}
        >
          Templates
        </button>
        <button
          onClick={() => setActiveTab("instances")}
          className={`py-3 text-sm font-medium border-b-2 transition-colors ${
            activeTab === "instances"
              ? "border-purple-500 text-purple-400"
              : "border-transparent text-(--text-muted) hover:text-(--text-secondary)"
          }`}
        >
          Active Requests
        </button>
      </div>

      {activeTab === "templates" &&
        (listQuery.isPending ? (
          <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
            <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
            Loading…
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
              <Workflow size={20} className="text-(--text-muted)" />
            </div>
            <p className="text-sm font-medium text-(--text-secondary)">
              No approval templates yet
            </p>
            <p className="text-xs text-(--text-muted) mt-1">
              Without one, Promotions/Transfers/Terminations/Warnings/Awards
              keep auto-approving on submit.
            </p>
          </div>
        ) : (
          <div className="space-y-1.5">
            {items.map((t) => (
              <div
                key={t.id}
                className="flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border)"
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <Workflow size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-(--text-primary)">
                    {t.name}{" "}
                    {t.is_default && (
                      <span className="text-xs text-purple-400 ml-1">
                        (default)
                      </span>
                    )}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5">
                    {t.action_type.replace("_", " ")} ·{" "}
                    {(t.levels ?? [])
                      .map((l) => APPROVER_LABEL[l.approver_type])
                      .join(" → ") || "no levels"}
                  </p>
                </div>
                {canManage &&
                  (deleteConfirm === t.id ? (
                    <div className="flex items-center gap-2 shrink-0">
                      <span className="text-xs text-(--text-muted)">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(t.id)}
                        className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(null)}
                        className="px-2.5 py-1 rounded-md text-xs text-(--text-secondary) hover:bg-(--bg-elevated)"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setDeleteConfirm(t.id)}
                      className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10 shrink-0"
                    >
                      <Trash2 size={14} />
                    </button>
                  ))}
              </div>
            ))}
          </div>
        ))}

      {activeTab === "instances" &&
        (instancesQuery.isPending ? (
          <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
            <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
            Loading…
          </div>
        ) : instances.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <p className="text-sm font-medium text-(--text-secondary)">
              No active approval requests found.
            </p>
          </div>
        ) : (
          <div className="space-y-1.5">
            {instances.map((i) => (
              <div
                key={i.id}
                className="flex items-center justify-between px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border)"
              >
                <div>
                  <p className="text-sm font-medium text-(--text-primary)">
                    {i.entity_type}{" "}
                    <span className="text-xs text-(--text-muted)">
                      ({i.entity_id})
                    </span>
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5">
                    Status:{" "}
                    <span className="text-purple-400 font-medium capitalize">
                      {i.overall_status}
                    </span>{" "}
                    · Level {i.current_level}
                  </p>
                </div>
              </div>
            ))}
          </div>
        ))}
    </div>
  );
}
