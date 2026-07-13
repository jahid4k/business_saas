// src/app/(dashboard)/[orgId]/hrm/positions/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Pencil,
  Trash2,
  Briefcase,
  Loader2,
} from "lucide-react";
import gsap from "gsap";
import type { Position, Department } from "@/types/hrm";
import {
  listPositions,
  createPosition,
  updatePosition,
  deletePosition,
} from "@/lib/hrm/positions";
import { listDepartments } from "@/lib/hrm/departments";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import PositionForm from "@/components/hrm/positions/PositionForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type FilterKey = "all" | "active" | "inactive";

export default function PositionsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { openDrawer } = useDrawer();
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();

  const [activeFilter, setActiveFilter] = useState<FilterKey>("all");
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [departments, setDepartments] = useState<Department[]>([]);

  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const canCreate = hasPermission("hrm.positions.create");
  const canUpdate = hasPermission("hrm.positions.update");
  const canDelete = hasPermission("hrm.positions.delete");

  const posKey = queryKeys.hrm.positions.list(orgId);
  const posQuery = useQuery({
    queryKey: posKey,
    queryFn: () => listPositions(orgId).then((r) => r.positions),
  });

  const positions = posQuery.data ?? [];

  useEffect(() => {
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
  }, [orgId]);

  useEffect(() => {
    if (posQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".pos-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [posQuery.isPending, activeFilter]);

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
    activeFilter === "all"
      ? positions
      : positions.filter((p) =>
          activeFilter === "active" ? p.is_active : !p.is_active,
        );

  const deptName = (id?: string) => departments.find((d) => d.id === id)?.name;

  const openCreate = () => {
    openDrawer({
      title: "New position",
      content: (
        <PositionForm
          orgId={orgId}
          onSave={async (values) => {
            const created = await createPosition(orgId, {
              title: values.title,
              description: values.description || undefined,
              department_id: values.department_id || undefined,
            });
            queryClient.setQueryData<Position[]>(posKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Position created.");
          }}
        />
      ),
    });
  };

  const openEdit = (position: Position) => {
    setOpenMenuId(null);
    openDrawer({
      title: "Edit position",
      content: (
        <PositionForm
          orgId={orgId}
          position={position}
          onSave={async (values) => {
            const updated = await updatePosition(orgId, position.id, {
              title: values.title,
              description: values.description || undefined,
              department_id: values.department_id || undefined,
              is_active: values.is_active,
            });
            queryClient.setQueryData<Position[]>(posKey, (old) =>
              (old ?? []).map((p) => (p.id === updated.id ? updated : p)),
            );
            toast.success("Position updated.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (posId: string) => {
    try {
      await deletePosition(orgId, posId);
      queryClient.setQueryData<Position[]>(posKey, (old) =>
        (old ?? []).filter((p) => p.id !== posId),
      );
      toast.success("Position deleted.");
    } catch {
      toast.error("Failed to delete position.");
    }
    setDeleteConfirm(null);
    setOpenMenuId(null);
  };

  return (
    <>
      <div className="p-6 md:p-8 max-w-4xl">
        <div className="flex items-start justify-between mb-8">
          <div>
            <h1
              className="text-2xl font-bold text-[var(--text-primary)] mb-1"
              style={{
                fontFamily: "var(--font-syne, Syne, sans-serif)",
                letterSpacing: "-0.02em",
              }}
            >
              Positions
            </h1>
            <p className="text-sm text-[var(--text-muted)]">
              {positions.length}{" "}
              {positions.length === 1 ? "position" : "positions"} total
            </p>
          </div>
          {canCreate && (
            <button
              onClick={openCreate}
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={15} />
              New position
            </button>
          )}
        </div>

        {posQuery.isError && (
          <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
            Failed to load positions. Please refresh.
          </div>
        )}

        <div className="flex items-center gap-0.5 mb-6 border-b border-[var(--border)]">
          {(["all", "active", "inactive"] as FilterKey[]).map((key) => {
            const count =
              key === "all"
                ? positions.length
                : positions.filter((p) =>
                    key === "active" ? p.is_active : !p.is_active,
                  ).length;
            const active = activeFilter === key;
            return (
              <button
                key={key}
                onClick={() => setActiveFilter(key)}
                className={`flex items-center gap-2 px-3.5 py-2.5 text-sm font-medium -mb-px border-b-2 transition-colors ${
                  active
                    ? "text-purple-400 border-purple-500"
                    : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
                }`}
              >
                {key === "all"
                  ? "All"
                  : key === "active"
                    ? "Active"
                    : "Inactive"}
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

        {posQuery.isPending ? (
          <div className="flex items-center justify-center py-20">
            <div className="flex items-center gap-3 text-sm text-[var(--text-muted)]">
              <Loader2 size={16} className="animate-spin text-purple-500" />
              Loading positions…
            </div>
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
              <Briefcase size={20} className="text-[var(--text-muted)]" />
            </div>
            <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
              {activeFilter === "all"
                ? "No positions yet"
                : `No ${activeFilter} positions`}
            </p>
            <p className="text-xs text-[var(--text-muted)] mb-4">
              {canCreate && activeFilter === "all"
                ? "Create your first position to get started."
                : "Nothing here for this filter."}
            </p>
            {canCreate && activeFilter === "all" && (
              <button
                onClick={openCreate}
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
              >
                <Plus size={14} />
                New position
              </button>
            )}
          </div>
        ) : (
          <div ref={listRef} className="space-y-1.5">
            {filtered.map((pos) => {
              const confirming = deleteConfirm === pos.id;
              const menuOpen = openMenuId === pos.id;

              return (
                <div
                  key={pos.id}
                  className="pos-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
                >
                  <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                    <Briefcase size={15} />
                  </div>

                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                      {pos.title}
                    </p>
                    {pos.description && (
                      <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                        {pos.description}
                      </p>
                    )}
                    <div className="flex items-center gap-3 mt-2 flex-wrap">
                      <span
                        className={`text-xs px-2 py-0.5 rounded-full border font-medium ${
                          pos.is_active
                            ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                            : "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"
                        }`}
                      >
                        {pos.is_active ? "Active" : "Inactive"}
                      </span>
                      {deptName(pos.department_id) && (
                        <span className="text-xs text-[var(--text-muted)]">
                          {deptName(pos.department_id)}
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
                        onClick={() => handleDelete(pos.id)}
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
                          if (el) menuRefs.current.set(pos.id, el);
                          else menuRefs.current.delete(pos.id);
                        }}
                      >
                        <button
                          onClick={() =>
                            setOpenMenuId(menuOpen ? null : pos.id)
                          }
                          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                        >
                          <MoreHorizontal size={15} />
                        </button>
                        {menuOpen && (
                          <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                            {canUpdate && (
                              <button
                                onClick={() => openEdit(pos)}
                                className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors text-left"
                              >
                                <Pencil size={13} />
                                Edit
                              </button>
                            )}
                            {canDelete && (
                              <button
                                onClick={() => {
                                  setDeleteConfirm(pos.id);
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

        {!posQuery.isPending && filtered.length > 0 && (
          <p className="mt-5 text-xs text-[var(--text-muted)]">
            Showing {filtered.length} of {positions.length}{" "}
            {positions.length === 1 ? "position" : "positions"}
          </p>
        )}
      </div>
    </>
  );
}
