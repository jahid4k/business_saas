// src/app/(dashboard)/[orgId]/hrm/setup/holidays/page.tsx
"use client";

import { use, useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Loader2, CalendarRange, Trash2 } from "lucide-react";
import type { Employee, Department, HolidayCalendar } from "@/types/hrm";
import {
  listHolidayCalendars,
  createHolidayCalendar,
  deleteHolidayCalendar,
  assignHolidayCalendar,
} from "@/lib/hrm/holidays";
import { listEmployees } from "@/lib/hrm/employees";
import { listDepartments } from "@/lib/hrm/departments";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import HolidayCalendarForm from "@/components/hrm/holidays/HolidayCalendarForm";
import HolidayManager from "@/components/hrm/holidays/HolidayManager";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

export default function HolidaysPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { openDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();
  const canManage = hasPermission("hrm.holidays.manage");

  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [assignId, setAssignId] = useState<string | null>(null);
  const [assigneeType, setAssigneeType] = useState<
    "organization" | "department" | "employee"
  >("organization");
  const [assigneeId, setAssigneeId] = useState("");
  const [effectiveDate, setEffectiveDate] = useState("");
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);

  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
  }, [orgId]);

  const listKey = queryKeys.hrm.holidayCalendars.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listHolidayCalendars(orgId).then((r) => r.calendars),
  });
  const items = listQuery.data ?? [];

  const openCreate = () => {
    openDrawer({
      title: "New holiday calendar",
      content: (
        <HolidayCalendarForm
          onSave={async (payload) => {
            const created = await createHolidayCalendar(orgId, payload);
            queryClient.setQueryData<HolidayCalendar[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Holiday calendar created.");
          }}
        />
      ),
    });
  };

  const openManageHolidays = (cal: HolidayCalendar) => {
    openDrawer({
      title: `${cal.name} — holidays`,
      content: <HolidayManager orgId={orgId} calendarId={cal.id} />,
    });
  };

  const handleDelete = async (calId: string) => {
    try {
      await deleteHolidayCalendar(orgId, calId);
      queryClient.setQueryData<HolidayCalendar[]>(listKey, (old) =>
        (old ?? []).filter((c) => c.id !== calId),
      );
      toast.success("Calendar deleted.");
    } catch {
      toast.error("Failed to delete calendar.");
    }
    setDeleteConfirm(null);
  };

  const handleAssign = async () => {
    if (!assignId || !effectiveDate) return;
    if (assigneeType !== "organization" && !assigneeId) return;
    try {
      await assignHolidayCalendar(orgId, {
        calendar_id: assignId,
        assignee_type: assigneeType,
        assignee_id: assigneeType === "organization" ? "org" : assigneeId,
        effective_date: effectiveDate,
      });
      toast.success("Calendar assigned.");
      setAssignId(null);
      setAssigneeId("");
      setEffectiveDate("");
    } catch {
      toast.error("Failed to assign calendar.");
    }
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
            Holiday Calendars
          </h1>
          <p className="text-sm text-(--text-muted)">
            Public and company holidays by year
          </p>
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New calendar
          </button>
        )}
      </div>

      {assignId && (
        <div className="mb-4 p-4 rounded-xl bg-(--bg-surface) border border-purple-500/30 space-y-2">
          <div className="flex items-center gap-2 flex-wrap">
            <select
              value={assigneeType}
              onChange={(e) =>
                setAssigneeType(e.target.value as typeof assigneeType)
              }
              className="px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
            >
              <option
                value="organization"
                style={{ background: "var(--bg-elevated)" }}
              >
                Whole organization
              </option>
              <option
                value="department"
                style={{ background: "var(--bg-elevated)" }}
              >
                A department
              </option>
              <option
                value="employee"
                style={{ background: "var(--bg-elevated)" }}
              >
                An employee
              </option>
            </select>
            {assigneeType === "department" && (
              <select
                value={assigneeId}
                onChange={(e) => setAssigneeId(e.target.value)}
                className="px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
              >
                <option value="">Select department</option>
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
            )}
            {assigneeType === "employee" && (
              <select
                value={assigneeId}
                onChange={(e) => setAssigneeId(e.target.value)}
                className="px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
              >
                <option value="">Select employee</option>
                {employees.map((e) => (
                  <option
                    key={e.id}
                    value={e.id}
                    style={{ background: "var(--bg-elevated)" }}
                  >
                    {e.first_name} {e.last_name ?? ""}
                  </option>
                ))}
              </select>
            )}
            <input
              value={effectiveDate}
              onChange={(e) => setEffectiveDate(e.target.value)}
              type="date"
              className="px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
            />
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleAssign}
              className="px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500"
            >
              Assign
            </button>
            <button
              onClick={() => setAssignId(null)}
              className="px-3.5 py-2 rounded-lg text-sm text-(--text-secondary) hover:bg-(--bg-elevated)"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <CalendarRange size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            No holiday calendars yet
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {items.map((cal) => (
            <div
              key={cal.id}
              className="group flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border)"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <CalendarRange size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-(--text-primary)">
                  {cal.name}
                </p>
                <p className="text-xs text-(--text-muted) mt-0.5">
                  {cal.year}
                  {cal.country_code ? ` · ${cal.country_code}` : ""}
                </p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {canManage && (
                  <>
                    <button
                      onClick={() => openManageHolidays(cal)}
                      className="px-3 py-1.5 rounded-lg text-xs font-medium text-purple-400 border border-purple-500/30 hover:bg-purple-500/10 transition-colors"
                    >
                      Manage holidays
                    </button>
                    <button
                      onClick={() => setAssignId(cal.id)}
                      className="px-3 py-1.5 rounded-lg text-xs font-medium text-(--text-secondary) border border-(--border) hover:bg-(--bg-elevated) transition-colors"
                    >
                      Assign
                    </button>
                  </>
                )}
                {canManage &&
                  (deleteConfirm === cal.id ? (
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-(--text-muted)">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(cal.id)}
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
                      onClick={() => setDeleteConfirm(cal.id)}
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
    </div>
  );
}
