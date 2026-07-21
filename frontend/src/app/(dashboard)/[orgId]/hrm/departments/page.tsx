// src/app/(dashboard)/[orgId]/hrm/departments/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Pencil,
  Trash2,
  Building2,
  Loader2,
} from "lucide-react";
import gsap from "gsap";
import type { Department } from "@/types/hrm";
import {
  listDepartments,
  createDepartment,
  updateDepartment,
  deleteDepartment,
} from "@/lib/hrm/departments";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import DepartmentForm from "@/components/hrm/departments/DepartmentForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type FilterKey = "all" | "active" | "inactive";

export default function DepartmentsPage({
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

  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const canCreate = hasPermission("hrm.departments.create");
  const canUpdate = hasPermission("hrm.departments.update");
  const canDelete = hasPermission("hrm.departments.delete");

  const deptKey = queryKeys.hrm.departments.list(orgId);
  const deptQuery = useQuery({
    queryKey: deptKey,
    queryFn: () => listDepartments(orgId).then((r) => r.departments),
  });

  const departments = deptQuery.data ?? [];

  useEffect(() => {
    if (deptQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".dept-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [deptQuery.isPending, activeFilter]);

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
      ? departments
      : departments.filter((d) =>
          activeFilter === "active" ? d.is_active : !d.is_active,
        );

  const nameById = (id?: string) => departments.find((d) => d.id === id)?.name;

  const openCreate = () => {
    openDrawer({
      title: "New department",
      content: (
        <DepartmentForm
          orgId={orgId}
          onSave={async (values) => {
            const created = await createDepartment(orgId, {
              name: values.name,
              description: values.description || undefined,
              parent_department_id: values.parent_department_id || undefined,
              head_employee_id: values.head_employee_id || undefined,
            });
            queryClient.setQueryData<Department[]>(deptKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Department created.");
          }}
        />
      ),
    });
  };

  const openEdit = (dept: Department) => {
    setOpenMenuId(null);
    openDrawer({
      title: "Edit department",
      content: (
        <DepartmentForm
          orgId={orgId}
          department={dept}
          onSave={async (values) => {
            const updated = await updateDepartment(orgId, dept.id, {
              name: values.name,
              description: values.description || undefined,
              parent_department_id: values.parent_department_id || undefined,
              head_employee_id: values.head_employee_id || undefined,
              is_active: values.is_active,
            });
            queryClient.setQueryData<Department[]>(deptKey, (old) =>
              (old ?? []).map((d) => (d.id === updated.id ? updated : d)),
            );
            toast.success("Department updated.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (deptId: string) => {
    try {
      await deleteDepartment(orgId, deptId);
      queryClient.setQueryData<Department[]>(deptKey, (old) =>
        (old ?? []).filter((d) => d.id !== deptId),
      );
      toast.success("Department deleted.");
    } catch {
      toast.error("Failed to delete department.");
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
              className="text-2xl font-bold text-(--text-primary) mb-1"
              style={{
                fontFamily: "var(--font-syne, Syne, sans-serif)",
                letterSpacing: "-0.02em",
              }}
            >
              Departments
            </h1>
            <p className="text-sm text-(--text-muted)">
              {departments.length}{" "}
              {departments.length === 1 ? "department" : "departments"} total
            </p>
          </div>
          {canCreate && (
            <button
              onClick={openCreate}
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={15} />
              New department
            </button>
          )}
        </div>

        {deptQuery.isError && (
          <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
            Failed to load departments. Please refresh.
          </div>
        )}

        <div className="flex items-center gap-0.5 mb-6 border-b border-(--border)">
          {(["all", "active", "inactive"] as FilterKey[]).map((key) => {
            const count =
              key === "all"
                ? departments.length
                : departments.filter((d) =>
                    key === "active" ? d.is_active : !d.is_active,
                  ).length;
            const active = activeFilter === key;
            return (
              <button
                key={key}
                onClick={() => setActiveFilter(key)}
                className={`flex items-center gap-2 px-3.5 py-2.5 text-sm font-medium -mb-px border-b-2 transition-colors ${
                  active
                    ? "text-purple-400 border-purple-500"
                    : "text-(--text-muted) border-transparent hover:text-(--text-secondary)"
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
                        : "bg-(--bg-elevated) text-(--text-muted)"
                    }`}
                  >
                    {count}
                  </span>
                )}
              </button>
            );
          })}
        </div>

        {deptQuery.isPending ? (
          <div className="flex items-center justify-center py-20">
            <div className="flex items-center gap-3 text-sm text-(--text-muted)">
              <Loader2 size={16} className="animate-spin text-purple-500" />
              Loading departments…
            </div>
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
              <Building2 size={20} className="text-(--text-muted)" />
            </div>
            <p className="text-sm font-medium text-(--text-secondary) mb-1">
              {activeFilter === "all"
                ? "No departments yet"
                : `No ${activeFilter} departments`}
            </p>
            <p className="text-xs text-(--text-muted) mb-4">
              {canCreate && activeFilter === "all"
                ? "Create your first department to get started."
                : "Nothing here for this filter."}
            </p>
            {canCreate && activeFilter === "all" && (
              <button
                onClick={openCreate}
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
              >
                <Plus size={14} />
                New department
              </button>
            )}
          </div>
        ) : (
          <div ref={listRef} className="space-y-1.5">
            {filtered.map((dept) => {
              const confirming = deleteConfirm === dept.id;
              const menuOpen = openMenuId === dept.id;
              const parentName = nameById(dept.parent_department_id);

              return (
                <div
                  key={dept.id}
                  className={`dept-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border) hover:border-(--text-muted)/25 transition-all duration-150 ${menuOpen ? "z-30 border-(--text-muted)/30" : "z-10"}`}
                >
                  <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                    <Building2 size={15} />
                  </div>

                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium leading-snug text-(--text-primary)">
                      {dept.name}
                    </p>
                    {dept.description && (
                      <p className="text-xs text-(--text-muted) mt-0.5 line-clamp-1">
                        {dept.description}
                      </p>
                    )}
                    <div className="flex items-center gap-3 mt-2 flex-wrap">
                      <span
                        className={`text-xs px-2 py-0.5 rounded-full border font-medium ${
                          dept.is_active
                            ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                            : "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"
                        }`}
                      >
                        {dept.is_active ? "Active" : "Inactive"}
                      </span>
                      {parentName && (
                        <span className="text-xs text-(--text-muted)">
                          Under {parentName}
                        </span>
                      )}
                    </div>
                  </div>

                  {confirming ? (
                    <div className="flex items-center gap-2 shrink-0 pt-0.5">
                      <span className="text-xs text-(--text-muted)">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(dept.id)}
                        className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(null)}
                        className="px-2.5 py-1 rounded-md text-xs font-medium text-(--text-secondary) hover:bg-(--bg-elevated) transition-colors"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    (canUpdate || canDelete) && (
                      <div
                        className="relative shrink-0"
                        ref={(el) => {
                          if (el) menuRefs.current.set(dept.id, el);
                          else menuRefs.current.delete(dept.id);
                        }}
                      >
                        <button
                          onClick={() =>
                            setOpenMenuId(menuOpen ? null : dept.id)
                          }
                          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-all"
                        >
                          <MoreHorizontal size={15} />
                        </button>
                        {menuOpen && (
                          <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-(--bg-elevated) border border-(--border) shadow-xl z-20">
                            {canUpdate && (
                              <button
                                onClick={() => openEdit(dept)}
                                className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) hover:text-(--text-primary) transition-colors text-left"
                              >
                                <Pencil size={13} />
                                Edit
                              </button>
                            )}
                            {canDelete && (
                              <button
                                onClick={() => {
                                  setDeleteConfirm(dept.id);
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

        {!deptQuery.isPending && filtered.length > 0 && (
          <p className="mt-5 text-xs text-(--text-muted)">
            Showing {filtered.length} of {departments.length}{" "}
            {departments.length === 1 ? "department" : "departments"}
          </p>
        )}
      </div>
    </>
  );
}
