// src/app/(dashboard)/[orgId]/hrm/setup/statuses/page.tsx
"use client";

import { use, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Loader2, ListTree, Trash2, Pencil } from "lucide-react";
import type { EmployeeStatusModel } from "@/types/hrm";
import {
  listEmployeeStatuses,
  createEmployeeStatus,
  updateEmployeeStatus,
  deleteEmployeeStatus,
} from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import StatusForm from "@/components/hrm/setup/StatusForm";
import { toast } from "sonner";

export default function StatusesPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { openDrawer, closeDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();
  const canManage = hasPermission("hrm.employees.update"); // using update permission as per our route setup

  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  // Instead of using a dedicated queryKeys for statuses, we invalidate the employees key or create one
  // let's define a unique key for statuses
  const listKey = ["hrm", "employee_statuses", orgId];

  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listEmployeeStatuses(orgId),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New Custom Status",
      content: (
        <StatusForm
          onSave={async (payload) => {
            try {
              const res = await createEmployeeStatus(orgId, payload);
              queryClient.setQueryData<EmployeeStatusModel[]>(
                listKey,
                (old) => [...(old ?? []), res],
              );
              toast.success("Status created.");
              closeDrawer();
            } catch (err: any) {
              toast.error(
                err.response?.data?.message || "Failed to create status.",
              );
              throw err;
            }
          }}
          onCancel={closeDrawer}
        />
      ),
    });
  };

  const openEdit = (status: EmployeeStatusModel) => {
    openDrawer({
      title: "Edit Status",
      content: (
        <StatusForm
          initialData={status}
          onSave={async (payload) => {
            try {
              const res = await updateEmployeeStatus(orgId, status.id, payload);
              queryClient.setQueryData<EmployeeStatusModel[]>(listKey, (old) =>
                (old ?? []).map((s) => (s.id === status.id ? res : s)),
              );
              toast.success("Status updated.");
              closeDrawer();
            } catch (err: any) {
              toast.error(
                err.response?.data?.message || "Failed to update status.",
              );
              throw err;
            }
          }}
          onCancel={closeDrawer}
        />
      ),
    });
  };

  const handleDelete = async (status: EmployeeStatusModel) => {
    try {
      await deleteEmployeeStatus(orgId, status.id);
      queryClient.setQueryData<EmployeeStatusModel[]>(listKey, (old) =>
        (old ?? []).filter((s) => s.id !== status.id),
      );
      toast.success("Status deleted.");
    } catch (err: any) {
      toast.error(
        err.response?.data?.message ||
          "Failed to delete status. Ensure no employees are assigned to it.",
      );
    }
    setDeleteConfirm(null);
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
            Employee Statuses
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Manage dynamic status tabs and badges for your employees.
          </p>
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New status
          </button>
        )}
      </div>

      {listQuery.isLoading ? (
        <div className="flex justify-center p-12">
          <Loader2 className="animate-spin text-purple-500" size={32} />
        </div>
      ) : items.length === 0 ? (
        <div className="text-center p-12 border border-dashed border-[var(--border)] rounded-2xl bg-[var(--bg-surface)]">
          <div className="inline-flex w-12 h-12 bg-purple-500/10 rounded-full items-center justify-center text-purple-500 mb-4">
            <ListTree size={24} />
          </div>
          <h3 className="text-[var(--text-primary)] font-medium mb-1">
            No statuses found
          </h3>
          <p className="text-sm text-[var(--text-muted)] mb-6 max-w-sm mx-auto">
            You don&apos;t have any employee statuses yet.
          </p>
          {canManage && (
            <button
              onClick={openCreate}
              className="inline-flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--text-muted)] transition-all"
            >
              <Plus size={15} />
              Create your first status
            </button>
          )}
        </div>
      ) : (
        <div className="bg-[var(--bg-surface)] border border-[var(--border)] rounded-2xl overflow-hidden">
          <div className="divide-y divide-[var(--border)]">
            {items.map((status) => {
              const isDefault = [
                "Active",
                "Inactive",
                "On leave",
                "Terminated",
                "Resigned",
              ].includes(status.name);
              const isConfirming = deleteConfirm === status.id;

              return (
                <div
                  key={status.id}
                  className="p-5 flex items-start gap-4 hover:bg-[var(--bg-elevated)] transition-colors group"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3 mb-1">
                      <span
                        className="inline-flex px-2.5 py-1 rounded-md text-xs font-semibold"
                        style={{
                          backgroundColor: `color-mix(in srgb, ${status.color} 15%, transparent)`,
                          color: status.color,
                        }}
                      >
                        {status.name}
                      </span>
                      {isDefault && (
                        <span className="text-[10px] uppercase font-bold tracking-wider text-[var(--text-muted)] bg-[var(--bg-base)] px-2 py-0.5 rounded border border-[var(--border)]">
                          System Default
                        </span>
                      )}
                    </div>
                    <div className="text-sm text-[var(--text-muted)] flex items-center gap-2">
                      <span className="capitalize">
                        {status.category.replace("_", " ")}
                      </span>
                    </div>
                  </div>

                  {canManage && (
                    <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                      {isConfirming ? (
                        <div className="flex items-center gap-2 bg-red-500/10 text-red-500 rounded-lg p-1 pr-3">
                          <button
                            onClick={() => handleDelete(status)}
                            className="px-3 py-1.5 rounded bg-red-500 text-white text-xs font-medium hover:bg-red-600 transition-colors"
                          >
                            Confirm Delete
                          </button>
                          <button
                            onClick={() => setDeleteConfirm(null)}
                            className="px-2 py-1.5 text-xs font-medium hover:text-red-400 transition-colors"
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <>
                          <button
                            onClick={() => openEdit(status)}
                            className="p-2 rounded-lg text-[var(--text-muted)] hover:text-purple-400 hover:bg-purple-500/10 transition-colors"
                          >
                            <Pencil size={16} />
                          </button>
                          {!isDefault && (
                            <button
                              onClick={() => setDeleteConfirm(status.id)}
                              className="p-2 rounded-lg text-[var(--text-muted)] hover:text-red-400 hover:bg-red-500/10 transition-colors"
                            >
                              <Trash2 size={16} />
                            </button>
                          )}
                        </>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
