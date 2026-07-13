// src/app/(dashboard)/[orgId]/hrm/setup/salary/page.tsx
"use client";

import { use, useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  Loader2,
  Layers,
  Building2,
  Wallet,
  Pencil,
  Trash2,
} from "lucide-react";
import type {
  Employee,
  SalaryComponent,
  SalaryStructure,
  EmployeeSalaryRecord,
} from "@/types/hrm";
import {
  listSalaryComponents,
  createSalaryComponent,
  updateSalaryComponent,
  deleteSalaryComponent,
  listSalaryStructures,
  createSalaryStructure,
  deleteSalaryStructure,
  getEmployeeSalaryHistory,
  assignEmployeeSalary,
  activeSalaryRecord,
} from "@/lib/hrm/salary";
import { listEmployees } from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import SalaryComponentForm from "@/components/hrm/salary/SalaryComponentForm";
import SalaryStructureForm from "@/components/hrm/salary/SalaryStructureForm";
import StructureComponentsManager from "@/components/hrm/salary/StructureComponentsManager";
import EmployeeSalaryForm from "@/components/hrm/salary/EmployeeSalaryForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type TabKey = "components" | "structures" | "assignments";

export default function SalarySetupPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const [tab, setTab] = useState<TabKey>("components");
  const canManage = hasPermission("hrm.salary.manage");
  const canManageEmp = hasPermission("hrm.salary.employee.manage");

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
            Salary Setup
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Components, structures, and employee assignments
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] w-fit">
        {(["components", "structures", "assignments"] as TabKey[]).map(
          (key) => (
            <button
              key={key}
              onClick={() => setTab(key)}
              className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
                tab === key
                  ? "bg-purple-600 text-white"
                  : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              }`}
            >
              {key === "components"
                ? "Components"
                : key === "structures"
                  ? "Structures"
                  : "Employee Assignments"}
            </button>
          ),
        )}
      </div>

      {tab === "components" && (
        <ComponentsView orgId={orgId} canManage={canManage} />
      )}
      {tab === "structures" && (
        <StructuresView orgId={orgId} canManage={canManage} />
      )}
      {tab === "assignments" && (
        <AssignmentsView orgId={orgId} canManage={canManageEmp} />
      )}
    </div>
  );
}

// ── Components ────────────────────────────────────────────────────────────
function ComponentsView({
  orgId,
  canManage,
}: {
  orgId: string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const listKey = queryKeys.hrm.salaryComponents.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listSalaryComponents(orgId).then((r) => r.components),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New salary component",
      content: (
        <SalaryComponentForm
          orgId={orgId}
          onSave={async (payload) => {
            const created = await createSalaryComponent(orgId, payload);
            queryClient.setQueryData<SalaryComponent[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Component created.");
          }}
        />
      ),
    });
  };

  const openEdit = (comp: SalaryComponent) => {
    openDrawer({
      title: "Edit salary component",
      content: (
        <SalaryComponentForm
          orgId={orgId}
          component={comp}
          onSave={async (payload) => {
            const updated = await updateSalaryComponent(
              orgId,
              comp.id,
              payload,
            );
            queryClient.setQueryData<SalaryComponent[]>(listKey, (old) =>
              (old ?? []).map((c) => (c.id === updated.id ? updated : c)),
            );
            toast.success("Component updated.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (compId: string) => {
    try {
      await deleteSalaryComponent(orgId, compId);
      queryClient.setQueryData<SalaryComponent[]>(listKey, (old) =>
        (old ?? []).filter((c) => c.id !== compId),
      );
      toast.success("Component deleted.");
    } catch {
      toast.error("Failed to delete component.");
    }
    setDeleteConfirm(null);
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
            New component
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
            <Wallet size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No salary components yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((c) => (
            <div
              key={c.id}
              className="group flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <Wallet size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">
                  {c.name}
                </p>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {c.component_type.replace("_", " ")} ·{" "}
                  {c.calc_method.replace("_", " ")}
                  {c.calc_method === "fixed" ? ` · ${c.fixed_value}` : ""}
                  {c.calc_method.startsWith("pct_")
                    ? ` · ${c.fixed_value}%`
                    : ""}
                  {!c.is_taxable ? " · Non-taxable" : ""}
                </p>
              </div>
              {canManage &&
                (deleteConfirm === c.id ? (
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <span className="text-xs text-[var(--text-muted)]">
                      Delete?
                    </span>
                    <button
                      onClick={() => handleDelete(c.id)}
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
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0">
                    <button
                      onClick={() => openEdit(c)}
                      className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
                    >
                      <Pencil size={14} />
                    </button>
                    <button
                      onClick={() => setDeleteConfirm(c.id)}
                      className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
            </div>
          ))}
        </div>
      )}
    </>
  );
}

// ── Structures ────────────────────────────────────────────────────────────
function StructuresView({
  orgId,
  canManage,
}: {
  orgId: string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const listKey = queryKeys.hrm.salaryStructures.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listSalaryStructures(orgId).then((r) => r.structures),
  });
  const items = listQuery.data ?? [];

  const compKey = queryKeys.hrm.salaryComponents.list(orgId);
  const compQuery = useQuery({
    queryKey: compKey,
    queryFn: () => listSalaryComponents(orgId).then((r) => r.components),
  });
  const allComponents = compQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New salary structure",
      content: (
        <SalaryStructureForm
          onSave={async (payload) => {
            const created = await createSalaryStructure(orgId, payload);
            queryClient.setQueryData<SalaryStructure[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Structure created.");
          }}
        />
      ),
    });
  };

  const openManageComponents = (structId: string, name: string) => {
    openDrawer({
      title: `${name} — components`,
      content: (
        <StructureComponentsManager
          orgId={orgId}
          structureId={structId}
          allComponents={allComponents}
        />
      ),
    });
  };

  const handleDelete = async (structId: string) => {
    try {
      await deleteSalaryStructure(orgId, structId);
      queryClient.setQueryData<SalaryStructure[]>(listKey, (old) =>
        (old ?? []).filter((s) => s.id !== structId),
      );
      toast.success("Structure deleted.");
    } catch {
      toast.error("Failed to delete structure.");
    }
    setDeleteConfirm(null);
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
            New structure
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
            <Layers size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No salary structures yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((s) => (
            <div
              key={s.id}
              className="group flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <Layers size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">
                  {s.name}
                </p>
                {s.grade_label && (
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {s.grade_label}
                  </p>
                )}
              </div>
              <div className="flex items-center gap-2 flex-shrink-0">
                {canManage && (
                  <button
                    onClick={() => openManageComponents(s.id, s.name)}
                    className="px-3 py-1.5 rounded-lg text-xs font-medium text-purple-400 border border-purple-500/30 hover:bg-purple-500/10 transition-colors"
                  >
                    Manage components
                  </button>
                )}
                {canManage &&
                  (deleteConfirm === s.id ? (
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-[var(--text-muted)]">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(s.id)}
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
                      onClick={() => setDeleteConfirm(s.id)}
                      className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10 opacity-0 group-hover:opacity-100 transition-opacity"
                    >
                      <Trash2 size={14} />
                    </button>
                  ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </>
  );
}

// ── Employee Assignments ──────────────────────────────────────────────────
function AssignmentsView({
  orgId,
  canManage,
}: {
  orgId: string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();

  const structQuery = useQuery({
    queryKey: queryKeys.hrm.salaryStructures.list(orgId),
    queryFn: () => listSalaryStructures(orgId).then((r) => r.structures),
  });
  const structures = structQuery.data ?? [];

  const assignmentsKey = ["hrm", orgId, "salary-assignments"] as const;
  const assignmentsQuery = useQuery({
    queryKey: assignmentsKey,
    queryFn: async () => {
      const empRes = await listEmployees(orgId, { limit: 200 });
      const entries = await Promise.all(
        empRes.employees.map(async (e) => {
          const hist = await getEmployeeSalaryHistory(orgId, e.id).catch(
            () => ({ records: [] as EmployeeSalaryRecord[] }),
          );
          return [e.id, hist.records] as const;
        }),
      );
      return {
        employees: empRes.employees,
        records: Object.fromEntries(entries) as Record<
          string,
          EmployeeSalaryRecord[]
        >,
      };
    },
  });

  const employees = assignmentsQuery.data?.employees ?? [];
  const records = assignmentsQuery.data?.records ?? {};

  const openAssign = (emp: Employee) => {
    openDrawer({
      title: "Assign salary",
      content: (
        <EmployeeSalaryForm
          employeeName={`${emp.first_name} ${emp.last_name ?? ""}`.trim()}
          structures={structures}
          onSave={async (payload) => {
            await assignEmployeeSalary(orgId, emp.id, payload);
            queryClient.invalidateQueries({ queryKey: assignmentsKey });
            toast.success("Salary assigned.");
          }}
        />
      ),
    });
  };

  if (assignmentsQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
        <Loader2 size={16} className="animate-spin text-purple-500" /> Loading…
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      {employees.map((e) => {
        const active = activeSalaryRecord(records[e.id] ?? []);
        const structureName = structures.find(
          (s) => s.id === active?.structure_id,
        )?.name;
        return (
          <div
            key={e.id}
            className="flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
          >
            <div className="w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-white text-xs font-bold bg-linear-to-br from-[#7c3aed] to-[#a855f7]">
              {e.first_name[0]?.toUpperCase()}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-[var(--text-primary)]">
                {e.first_name} {e.last_name ?? ""}
              </p>
              <p className="text-xs text-[var(--text-muted)] mt-0.5">
                {active ? (
                  <>
                    {active.basic_pay} basic pay
                    {structureName ? ` · ${structureName}` : ""} · since{" "}
                    {new Date(active.effective_date).toLocaleDateString(
                      "en-US",
                      { month: "short", day: "numeric", year: "numeric" },
                    )}
                  </>
                ) : (
                  "No salary assigned"
                )}
              </p>
            </div>
            {canManage && (
              <button
                onClick={() => openAssign(e)}
                className="px-3 py-1.5 rounded-lg text-xs font-medium text-purple-400 border border-purple-500/30 hover:bg-purple-500/10 transition-colors flex-shrink-0"
              >
                {active ? "Update" : "Assign"}
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
