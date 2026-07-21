// src/app/(dashboard)/[orgId]/hrm/setup/document-templates/page.tsx
"use client";

import { use, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Loader2,
  FileText,
  Pencil,
  Trash2,
  Eye,
} from "lucide-react";
import type { DocumentTemplate } from "@/types/hrm";
import {
  listDocumentTemplates,
  createDocumentTemplate,
  updateDocumentTemplate,
  deleteDocumentTemplate,
} from "@/lib/hrm/doctemplates";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import DocumentTemplateForm from "@/components/hrm/doctemplates/DocumentTemplateForm";
import TemplatePreview from "@/components/hrm/doctemplates/TemplatePreview";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

export default function DocumentTemplatesPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { openDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();
  const canManage = hasPermission("hrm.doc_templates.manage");

  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  const listKey = queryKeys.hrm.documentTemplates.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listDocumentTemplates(orgId).then((r) => r.templates),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New document template",
      content: (
        <DocumentTemplateForm
          onSave={async (payload) => {
            const created = await createDocumentTemplate(orgId, payload);
            queryClient.setQueryData<DocumentTemplate[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Template created.");
          }}
        />
      ),
    });
  };

  const openEdit = (t: DocumentTemplate) => {
    setOpenMenuId(null);
    openDrawer({
      title: "Edit document template",
      content: (
        <DocumentTemplateForm
          template={t}
          onSave={async (payload) => {
            const updated = await updateDocumentTemplate(orgId, t.id, payload);
            queryClient.setQueryData<DocumentTemplate[]>(listKey, (old) =>
              (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
            );
            toast.success("Template updated.");
          }}
        />
      ),
    });
  };

  const openPreview = (t: DocumentTemplate) => {
    setOpenMenuId(null);
    openDrawer({
      title: `${t.name} — preview`,
      content: <TemplatePreview orgId={orgId} template={t} />,
    });
  };

  const handleDelete = async (templateId: string) => {
    try {
      await deleteDocumentTemplate(orgId, templateId);
      queryClient.setQueryData<DocumentTemplate[]>(listKey, (old) =>
        (old ?? []).filter((t) => t.id !== templateId),
      );
      toast.success("Template deleted.");
    } catch {
      toast.error("Failed to delete template.");
    }
    setDeleteConfirm(null);
    setOpenMenuId(null);
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
            Document Templates
          </h1>
          <p className="text-sm text-(--text-muted)">
            {items.length} {items.length === 1 ? "template" : "templates"}
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

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <FileText size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            No document templates yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((t) => {
            const menuOpen = openMenuId === t.id;
            return (
              <div
                key={t.id}
                className={`group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border) ${menuOpen ? "z-30 border-(--text-muted)/30" : "z-10"}`}
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <FileText size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-(--text-primary)">
                    {t.name}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5">
                    {t.document_type.replace("_", " ")}
                    {t.available_variables.length > 0
                      ? ` · ${t.available_variables.length} variables`
                      : ""}
                    {t.requires_acknowledgement ? " · Requires ack" : ""}
                  </p>
                  <span
                    className={`inline-block mt-2 text-xs px-2 py-0.5 rounded-full border font-medium ${
                      t.is_active
                        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                        : "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"
                    }`}
                  >
                    {t.is_active ? "Active" : "Inactive"}
                  </span>
                </div>
                {deleteConfirm === t.id ? (
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
                  <div className="relative shrink-0">
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : t.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-(--bg-elevated) border border-(--border) shadow-xl z-20">
                        <button
                          onClick={() => openPreview(t)}
                          className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) hover:text-(--text-primary) text-left"
                        >
                          <Eye size={13} />
                          Preview
                        </button>
                        {canManage && (
                          <>
                            <button
                              onClick={() => openEdit(t)}
                              className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) hover:text-(--text-primary) text-left"
                            >
                              <Pencil size={13} />
                              Edit
                            </button>
                            <button
                              onClick={() => setDeleteConfirm(t.id)}
                              className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                            >
                              <Trash2 size={13} />
                              Delete
                            </button>
                          </>
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
