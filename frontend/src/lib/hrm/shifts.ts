// src/lib/hrm/shifts.ts
import api from "../api";
import type {
  Shift,
  ShiftListResponse,
  CreateShiftPayload,
  UpdateShiftPayload,
  WorkScheduleAssignment,
  AssignmentListResponse,
  AssignShiftPayload,
} from "@/types/hrm";

const shiftsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/shifts`;
const assignmentsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/shifts/assignments`;

export async function listShifts(
  orgId: string,
  opts?: { activeOnly?: boolean },
): Promise<ShiftListResponse> {
  const res = await api.get<{ success: boolean; data: ShiftListResponse }>(
    shiftsUrl(orgId),
    {
      params: opts?.activeOnly ? { active: "true" } : undefined,
    },
  );
  return res.data.data;
}

export async function createShift(
  orgId: string,
  body: CreateShiftPayload,
): Promise<Shift> {
  const res = await api.post<{ success: boolean; data: { shift: Shift } }>(
    shiftsUrl(orgId),
    body,
  );
  return res.data.data.shift;
}

export async function updateShift(
  orgId: string,
  shiftId: string,
  body: UpdateShiftPayload,
): Promise<Shift> {
  const res = await api.patch<{ success: boolean; data: { shift: Shift } }>(
    `${shiftsUrl(orgId)}/${shiftId}`,
    body,
  );
  return res.data.data.shift;
}

export async function deleteShift(
  orgId: string,
  shiftId: string,
): Promise<void> {
  await api.delete(`${shiftsUrl(orgId)}/${shiftId}`);
}

export async function listShiftAssignments(
  orgId: string,
): Promise<AssignmentListResponse> {
  const res = await api.get<{ success: boolean; data: AssignmentListResponse }>(
    assignmentsUrl(orgId),
  );
  return res.data.data;
}

export async function assignShift(
  orgId: string,
  body: AssignShiftPayload,
): Promise<WorkScheduleAssignment> {
  const res = await api.post<{
    success: boolean;
    data: { assignment: WorkScheduleAssignment };
  }>(assignmentsUrl(orgId), body);
  return res.data.data.assignment;
}

export async function removeShiftAssignment(
  orgId: string,
  assignmentId: string,
): Promise<void> {
  await api.delete(`${assignmentsUrl(orgId)}/${assignmentId}`);
}
