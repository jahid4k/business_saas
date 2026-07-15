// src/app/(dashboard)/[orgId]/crm/setup/templates/page.tsx
"use client";

import { use, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Loader2, FileText, Trash2, Pencil } from "lucide-react";
import type { TemplateModel } from "@/lib/crm/templates";
import {
  listTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
} from "@/lib/crm/templates";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import TemplateForm from "@/components/crm/setup/TemplateForm";
import { toast } from "sonner";

export default function TemplatesPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { openDrawer, closeDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();
  const canManage = hasPermission("crm.templates.update");

  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const listKey = ["crm", "templates", orgId];
  
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listTemplates(orgId),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New Template",
      content: (
        <TemplateForm
          onSave={async (payload) => {
            try {
              const res = await createTemplate(orgId, payload);
              queryClient.setQueryData<TemplateModel[]>(listKey, (old) => [...(old ?? []), res]);
              toast.success("Template created.");
              closeDrawer();
            } catch (err: any) {
              toast.error(err.response?.data?.message || "Failed to create template.");
              throw err;
            }
          }}
          onCancel={closeDrawer}
        />
      ),
    });
  };

  const openEdit = (template: TemplateModel) => {
    openDrawer({
      title: "Edit Template",
      content: (
        <TemplateForm
          initialData={template}
          onSave={async (payload) => {
            try {
              const res = await updateTemplate(orgId, template.id, payload);
              queryClient.setQueryData<TemplateModel[]>(listKey, (old) =>
                (old ?? []).map((t) => (t.id === template.id ? res : t))
              );
              toast.success("Template updated.");
              closeDrawer();
            } catch (err: any) {
              toast.error(err.response?.data?.message || "Failed to update template.");
              throw err;
            }
          }}
          onCancel={closeDrawer}
        />
      ),
    });
  };

  const handleDelete = async (template: TemplateModel) => {
    try {
      await deleteTemplate(orgId, template.id);
      queryClient.setQueryData<TemplateModel[]>(listKey, (old) =>
        (old ?? []).filter((t) => t.id !== template.id)
      );
      toast.success("Template deleted.");
    } catch (err: any) {
      toast.error(err.response?.data?.message || "Failed to delete template.");
    }
    setDeleteConfirm(null);
  };

  return (
    <div className="flex flex-col h-full bg-[var(--bg-canvas)]">
      {/* ── Header ── */}
      <div className="shrink-0 border-b border-[var(--border)] bg-[var(--bg-surface)] px-8 py-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-xl font-semibold text-[var(--text-primary)]">Templates</h1>
            <p className="mt-1.5 text-sm text-[var(--text-secondary)] max-w-2xl">
              Standardize outreach with reusable email and note templates.
            </p>
          </div>
          {canManage && (
            <button
              onClick={openCreate}
              className="inline-flex items-center gap-2 rounded-lg bg-purple-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-purple-700 transition-colors"
            >
              <Plus size={16} />
              New Template
            </button>
          )}
        </div>
      </div>

      {/* ── Content ── */}
      <div className="flex-1 overflow-auto p-8">
        <div className="mx-auto max-w-5xl">
          {listQuery.isLoading ? (
            <div className="flex justify-center p-12">
              <Loader2 className="animate-spin text-purple-600" size={32} />
            </div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-16 text-center bg-[var(--bg-surface)] rounded-xl border border-[var(--border)] border-dashed">
              <div className="h-12 w-12 rounded-full bg-purple-50 dark:bg-purple-500/10 flex items-center justify-center mb-4">
                <FileText className="text-purple-600 dark:text-purple-400" size={24} />
              </div>
              <h3 className="text-base font-medium text-[var(--text-primary)] mb-1">
                No templates configured
              </h3>
              <p className="text-sm text-[var(--text-secondary)] max-w-sm mb-6">
                Create templates to help your sales reps quickly insert standard text for emails and notes.
              </p>
              {canManage && (
                <button
                  onClick={openCreate}
                  className="inline-flex items-center gap-2 rounded-lg bg-[var(--bg-surface)] border border-[var(--border)] px-4 py-2 text-sm font-medium text-[var(--text-primary)] shadow-sm hover:bg-gray-50 dark:hover:bg-white/5 transition-colors"
                >
                  <Plus size={16} />
                  Add First Template
                </button>
              )}
            </div>
          ) : (
            <div className="bg-[var(--bg-surface)] border border-[var(--border)] rounded-xl overflow-hidden shadow-sm">
              <div className="grid grid-cols-12 gap-4 border-b border-[var(--border)] bg-gray-50/50 dark:bg-white/[0.02] p-4 text-xs font-semibold text-[var(--text-secondary)] uppercase tracking-wider">
                <div className="col-span-4">Name</div>
                <div className="col-span-3">Type</div>
                <div className="col-span-4">Subject (Email)</div>
                <div className="col-span-1 text-right">Actions</div>
              </div>

              <div className="divide-y divide-[var(--border)]">
                {items.map((template) => (
                  <div key={template.id} className="grid grid-cols-12 gap-4 items-center p-4 hover:bg-gray-50/50 dark:hover:bg-white/[0.02] transition-colors">
                    <div className="col-span-4">
                      <div className="font-medium text-[var(--text-primary)]">{template.name}</div>
                    </div>
                    
                    <div className="col-span-3">
                      <span className={`inline-flex items-center rounded-md px-2 py-1 text-xs font-medium border ${
                        template.type === 'email' 
                          ? 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-500/10 dark:text-blue-400 dark:border-blue-500/20' 
                          : 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/20'
                      }`}>
                        {template.type === 'email' ? 'Email' : 'Note'}
                      </span>
                    </div>

                    <div className="col-span-4">
                      <div className="text-sm text-[var(--text-secondary)] truncate">
                        {template.type === 'email' ? template.subject : "—"}
                      </div>
                    </div>

                    <div className="col-span-1 flex justify-end">
                      {canManage ? (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => openEdit(template)}
                            className="p-1.5 text-[var(--text-muted)] hover:text-purple-600 hover:bg-purple-50 dark:hover:bg-purple-500/10 rounded transition-colors"
                            title="Edit"
                          >
                            <Pencil size={16} />
                          </button>
                          
                          {deleteConfirm === template.id ? (
                            <div className="flex items-center gap-1 animate-in fade-in zoom-in duration-200">
                              <button
                                onClick={() => handleDelete(template)}
                                className="px-2 py-1 bg-red-600 hover:bg-red-700 text-white text-xs font-medium rounded transition-colors"
                              >
                                Confirm
                              </button>
                              <button
                                onClick={() => setDeleteConfirm(null)}
                                className="px-2 py-1 bg-[var(--bg-surface)] hover:bg-gray-100 dark:hover:bg-white/5 border border-[var(--border)] text-[var(--text-secondary)] text-xs font-medium rounded transition-colors"
                              >
                                Cancel
                              </button>
                            </div>
                          ) : (
                            <button
                              onClick={() => setDeleteConfirm(template.id)}
                              className="p-1.5 text-[var(--text-muted)] hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-500/10 rounded transition-colors"
                              title="Delete"
                            >
                              <Trash2 size={16} />
                            </button>
                          )}
                        </div>
                      ) : (
                        <span className="text-xs text-[var(--text-muted)]">Read only</span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
