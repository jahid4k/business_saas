// src/app/(dashboard)/[orgId]/hrm/employees/page.tsx
"use client";

import { use, useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Pencil,
  Trash2,
  UserRound,
  Loader2,
  Search,
  UserX,
} from "lucide-react";
import gsap from "gsap";
import type {
  Employee,
  EmployeeStatus,
  Department,
  Position,
} from "@/types/hrm";
import {
  listEmployees,
  createEmployee,
  updateEmployee,
  deleteEmployee,
  terminateEmployee,
} from "@/lib/hrm/employees";
import { listDepartments } from "@/lib/hrm/departments";
import { listPositions } from "@/lib/hrm/positions";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import EmployeeForm from "@/components/hrm/employees/EmployeeForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type FilterKey = "all" | EmployeeStatus;

const STATUS_STYLE: Record<EmployeeStatus, { label: string; badge: string }> = {
  active: {
    label: "Active",
    badge: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  },
  inactive: {
    label: "Inactive",
    badge: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  },
  on_leave: {
    label: "On leave",
    badge: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  },
  terminated: {
    label: "Terminated",
    badge: "bg-red-500/10 text-red-400 border-red-500/20",
  },
};

const STATUS_TABS: FilterKey[] = [
  "all",
  "active",
  "on_leave",
  "inactive",
  "terminated",
];

export default function EmployeesPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { openDrawer } = useDrawer();
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();

  const [activeFilter, setActiveFilter] = useState<FilterKey>("all");
  const [deptFilter, setDeptFilter] = useState<string>("");
  const [search, setSearch] = useState("");
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [terminateConfirm, setTerminateConfirm] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);

  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const canCreate = hasPermission("hrm.employees.create");
  const canUpdate = hasPermission("hrm.employees.update");
  const canDelete = hasPermission("hrm.employees.delete");
  const canTerminate = hasPermission("hrm.employees.terminate");

  const empKey = queryKeys.hrm.employees.list(orgId);
  const empQuery = useQuery({
    queryKey: empKey,
    queryFn: () =>
      listEmployees(orgId, { limit: 200 }).then((r) => r.employees),
  });

  const employees = empQuery.data ?? [];

  useEffect(() => {
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
    listPositions(orgId)
      .then((r) => setPositions(r.positions))
      .catch(() => {});
  }, [orgId]);

  useEffect(() => {
    if (empQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".emp-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [empQuery.isPending, activeFilter, deptFilter, search]);

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

  const deptName = (id?: string) => departments.find((d) => d.id === id)?.name;
  const posTitle = (id?: string) => positions.find((p) => p.id === id)?.title;

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return employees.filter((e) => {
      if (activeFilter !== "all" && e.status !== activeFilter) return false;
      if (deptFilter && e.department_id !== deptFilter) return false;
      if (q) {
        const hay =
          `${e.first_name} ${e.last_name ?? ""} ${e.email ?? ""} ${e.employee_number ?? ""}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [employees, activeFilter, deptFilter, search]);

  const openCreate = () => {
    openDrawer({
      title: "New employee",
      width: "lg",
      content: (
        <EmployeeForm
          orgId={orgId}
          onSave={async (values) => {
            const created = await createEmployee(orgId, {
              first_name: values.first_name,
              last_name: values.last_name || undefined,
              email: values.email || undefined,
              work_email: values.work_email || undefined,
              phone: values.phone || undefined,
              work_phone: values.work_phone || undefined,
              employee_number: values.employee_number || undefined,
              date_of_birth: values.date_of_birth || undefined,
              gender: (values.gender || undefined) as Employee["gender"],
              hire_date: values.hire_date,
              employment_type: (values.employment_type ||
                undefined) as Employee["employment_type"],
              department_id: values.department_id || undefined,
              position_id: values.position_id || undefined,
              manager_id: values.manager_id || undefined,
              address: values.address || undefined,
              city: values.city || undefined,
              country: values.country || undefined,
              notes: values.notes || undefined,
            });
            queryClient.setQueryData<Employee[]>(empKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Employee created.");
          }}
        />
      ),
    });
  };

  const openEdit = (employee: Employee) => {
    setOpenMenuId(null);
    openDrawer({
      title: "Edit employee",
      width: "lg",
      content: (
        <EmployeeForm
          orgId={orgId}
          employee={employee}
          onSave={async (values) => {
            const updated = await updateEmployee(orgId, employee.id, {
              first_name: values.first_name,
              last_name: values.last_name || undefined,
              email: values.email || undefined,
              work_email: values.work_email || undefined,
              phone: values.phone || undefined,
              work_phone: values.work_phone || undefined,
              employee_number: values.employee_number || undefined,
              date_of_birth: values.date_of_birth || undefined,
              gender: (values.gender || undefined) as Employee["gender"],
              employment_type: (values.employment_type ||
                undefined) as Employee["employment_type"],
              status: (values.status || undefined) as Employee["status"],
              department_id: values.department_id || undefined,
              position_id: values.position_id || undefined,
              manager_id: values.manager_id || undefined,
              address: values.address || undefined,
              city: values.city || undefined,
              country: values.country || undefined,
              notes: values.notes || undefined,
            });
            queryClient.setQueryData<Employee[]>(empKey, (old) =>
              (old ?? []).map((e) => (e.id === updated.id ? updated : e)),
            );
            toast.success("Employee updated.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (empId: string) => {
    try {
      await deleteEmployee(orgId, empId);
      queryClient.setQueryData<Employee[]>(empKey, (old) =>
        (old ?? []).filter((e) => e.id !== empId),
      );
      toast.success("Employee deleted.");
    } catch {
      toast.error("Failed to delete employee.");
    }
    setDeleteConfirm(null);
    setOpenMenuId(null);
  };

  const handleTerminate = async (empId: string) => {
    try {
      const updated = await terminateEmployee(orgId, empId, {
        termination_date: new Date().toISOString().slice(0, 10),
      });
      queryClient.setQueryData<Employee[]>(empKey, (old) =>
        (old ?? []).map((e) => (e.id === updated.id ? updated : e)),
      );
      toast.success("Employee terminated.");
    } catch {
      toast.error("Failed to terminate employee.");
    }
    setTerminateConfirm(null);
    setOpenMenuId(null);
  };

  return (
    <>
      <div className="p-6 md:p-8 max-w-5xl">
        <div className="flex items-start justify-between mb-8">
          <div>
            <h1
              className="text-2xl font-bold text-[var(--text-primary)] mb-1"
              style={{
                fontFamily: "var(--font-syne, Syne, sans-serif)",
                letterSpacing: "-0.02em",
              }}
            >
              Employees
            </h1>
            <p className="text-sm text-[var(--text-muted)]">
              {employees.length}{" "}
              {employees.length === 1 ? "employee" : "employees"} total
            </p>
          </div>
          {canCreate && (
            <button
              onClick={openCreate}
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={15} />
              New employee
            </button>
          )}
        </div>

        {empQuery.isError && (
          <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
            Failed to load employees. Please refresh.
          </div>
        )}

        <div className="flex items-center gap-3 mb-5">
          <div className="relative flex-1 max-w-xs">
            <Search
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]"
            />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by name, email, ID…"
              className="w-full pl-9 pr-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
            />
          </div>
          <select
            value={deptFilter}
            onChange={(e) => setDeptFilter(e.target.value)}
            className="px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500 transition-all"
          >
            <option value="">All departments</option>
            {departments.map((d) => (
              <option
                key={d.id}
                value={d.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {d.name}
              </option>
            ))}
          </select>
        </div>

        <div className="flex items-center gap-0.5 mb-6 border-b border-[var(--border)]">
          {STATUS_TABS.map((key) => {
            const count =
              key === "all"
                ? employees.length
                : employees.filter((e) => e.status === key).length;
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

        {empQuery.isPending ? (
          <div className="flex items-center justify-center py-20">
            <div className="flex items-center gap-3 text-sm text-[var(--text-muted)]">
              <Loader2 size={16} className="animate-spin text-purple-500" />
              Loading employees…
            </div>
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
              <UserRound size={20} className="text-[var(--text-muted)]" />
            </div>
            <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
              {activeFilter === "all" && !search && !deptFilter
                ? "No employees yet"
                : "No matching employees"}
            </p>
            <p className="text-xs text-[var(--text-muted)] mb-4">
              {canCreate && activeFilter === "all" && !search && !deptFilter
                ? "Add your first employee to get started."
                : "Try adjusting filters or search."}
            </p>
            {canCreate && activeFilter === "all" && !search && !deptFilter && (
              <button
                onClick={openCreate}
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
              >
                <Plus size={14} />
                New employee
              </button>
            )}
          </div>
        ) : (
          <div ref={listRef} className="space-y-1.5">
            {filtered.map((emp) => {
              const confirmingDelete = deleteConfirm === emp.id;
              const confirmingTerminate = terminateConfirm === emp.id;
              const menuOpen = openMenuId === emp.id;
              const s = STATUS_STYLE[emp.status];
              const fullName =
                `${emp.first_name} ${emp.last_name ?? ""}`.trim();

              return (
                <div
                  key={emp.id}
                  className="emp-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150"
                >
                  <div className="w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-white text-xs font-bold font-syne bg-linear-to-br from-[#7c3aed] to-[#a855f7]">
                    {emp.first_name[0]?.toUpperCase()}
                  </div>

                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                      {fullName}
                    </p>
                    <p className="text-xs text-[var(--text-muted)] mt-0.5">
                      {[posTitle(emp.position_id), deptName(emp.department_id)]
                        .filter(Boolean)
                        .join(" · ") || "No position set"}
                    </p>
                    <div className="flex items-center gap-3 mt-2 flex-wrap">
                      <span
                        className={`text-xs px-2 py-0.5 rounded-full border font-medium ${s.badge}`}
                      >
                        {s.label}
                      </span>
                      {emp.employee_number && (
                        <span className="text-xs text-[var(--text-muted)]">
                          {emp.employee_number}
                        </span>
                      )}
                      {emp.email && (
                        <span className="text-xs text-[var(--text-muted)]">
                          {emp.email}
                        </span>
                      )}
                    </div>
                  </div>

                  {confirmingDelete ? (
                    <div className="flex items-center gap-2 flex-shrink-0 pt-0.5">
                      <span className="text-xs text-[var(--text-muted)]">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(emp.id)}
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
                  ) : confirmingTerminate ? (
                    <div className="flex items-center gap-2 flex-shrink-0 pt-0.5">
                      <span className="text-xs text-[var(--text-muted)]">
                        Terminate today?
                      </span>
                      <button
                        onClick={() => handleTerminate(emp.id)}
                        className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setTerminateConfirm(null)}
                        className="px-2.5 py-1 rounded-md text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    (canUpdate || canDelete || canTerminate) && (
                      <div
                        className="relative flex-shrink-0"
                        ref={(el) => {
                          if (el) menuRefs.current.set(emp.id, el);
                          else menuRefs.current.delete(emp.id);
                        }}
                      >
                        <button
                          onClick={() =>
                            setOpenMenuId(menuOpen ? null : emp.id)
                          }
                          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                        >
                          <MoreHorizontal size={15} />
                        </button>
                        {menuOpen && (
                          <div className="absolute right-0 top-full mt-1.5 w-44 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                            {canUpdate && (
                              <button
                                onClick={() => openEdit(emp)}
                                className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors text-left"
                              >
                                <Pencil size={13} />
                                Edit
                              </button>
                            )}
                            {canTerminate && emp.status !== "terminated" && (
                              <button
                                onClick={() => {
                                  setTerminateConfirm(emp.id);
                                  setOpenMenuId(null);
                                }}
                                className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-amber-400 hover:bg-amber-500/10 transition-colors text-left"
                              >
                                <UserX size={13} />
                                Terminate
                              </button>
                            )}
                            {canDelete && (
                              <button
                                onClick={() => {
                                  setDeleteConfirm(emp.id);
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

        {!empQuery.isPending && filtered.length > 0 && (
          <p className="mt-5 text-xs text-[var(--text-muted)]">
            Showing {filtered.length} of {employees.length}{" "}
            {employees.length === 1 ? "employee" : "employees"}
          </p>
        )}
      </div>
    </>
  );
}
