// src/lib/hrm/leave.ts
import api from "../api";
import type {
  LeaveType,
  LeaveTypeListResponse,
  CreateLeaveTypePayload,
  UpdateLeaveTypePayload,
  LeaveRequest,
  LeaveRequestListResponse,
  LeaveRequestListFilter,
  CreateLeaveRequestPayload,
  ReviewLeaveRequestPayload,
} from "@/types/hrm";

const typesBase = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/leave/types`;
const requestsBase = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/leave/requests`;

// ── Leave Types ───────────────────────────────────────────
export async function listLeaveTypes(
  orgId: string,
  opts?: { activeOnly?: boolean },
): Promise<LeaveTypeListResponse> {
  const res = await api.get<{ success: boolean; data: LeaveTypeListResponse }>(
    typesBase(orgId),
    {
      params: opts?.activeOnly ? { active: "true" } : undefined,
    },
  );
  return res.data.data;
}

export async function getLeaveType(
  orgId: string,
  typeId: string,
): Promise<LeaveType> {
  const res = await api.get<{
    success: boolean;
    data: { leave_type: LeaveType };
  }>(`${typesBase(orgId)}/${typeId}`);
  return res.data.data.leave_type;
}

export async function createLeaveType(
  orgId: string,
  body: CreateLeaveTypePayload,
): Promise<LeaveType> {
  const res = await api.post<{
    success: boolean;
    data: { leave_type: LeaveType };
  }>(typesBase(orgId), body);
  return res.data.data.leave_type;
}

export async function updateLeaveType(
  orgId: string,
  typeId: string,
  body: UpdateLeaveTypePayload,
): Promise<LeaveType> {
  const res = await api.patch<{
    success: boolean;
    data: { leave_type: LeaveType };
  }>(`${typesBase(orgId)}/${typeId}`, body);
  return res.data.data.leave_type;
}

export async function deleteLeaveType(
  orgId: string,
  typeId: string,
): Promise<void> {
  await api.delete(`${typesBase(orgId)}/${typeId}`);
}

// ── Leave Requests ────────────────────────────────────────
export async function listLeaveRequests(
  orgId: string,
  filter?: LeaveRequestListFilter,
): Promise<LeaveRequestListResponse> {
  const res = await api.get<{
    success: boolean;
    data: LeaveRequestListResponse;
  }>(requestsBase(orgId), { params: filter });
  return res.data.data;
}

export async function getLeaveRequest(
  orgId: string,
  reqId: string,
): Promise<LeaveRequest> {
  const res = await api.get<{
    success: boolean;
    data: { leave_request: LeaveRequest };
  }>(`${requestsBase(orgId)}/${reqId}`);
  return res.data.data.leave_request;
}

export async function createLeaveRequest(
  orgId: string,
  body: CreateLeaveRequestPayload,
): Promise<LeaveRequest> {
  const res = await api.post<{
    success: boolean;
    data: { leave_request: LeaveRequest };
  }>(requestsBase(orgId), body);
  return res.data.data.leave_request;
}

export async function approveLeaveRequest(
  orgId: string,
  reqId: string,
  body?: ReviewLeaveRequestPayload,
): Promise<LeaveRequest> {
  const res = await api.post<{
    success: boolean;
    data: { leave_request: LeaveRequest };
  }>(`${requestsBase(orgId)}/${reqId}/approve`, body ?? {});
  return res.data.data.leave_request;
}

export async function rejectLeaveRequest(
  orgId: string,
  reqId: string,
  body?: ReviewLeaveRequestPayload,
): Promise<LeaveRequest> {
  const res = await api.post<{
    success: boolean;
    data: { leave_request: LeaveRequest };
  }>(`${requestsBase(orgId)}/${reqId}/reject`, body ?? {});
  return res.data.data.leave_request;
}

export async function cancelLeaveRequest(
  orgId: string,
  reqId: string,
): Promise<LeaveRequest> {
  const res = await api.post<{
    success: boolean;
    data: { leave_request: LeaveRequest };
  }>(`${requestsBase(orgId)}/${reqId}/cancel`, {});
  return res.data.data.leave_request;
}

export async function deleteLeaveRequest(
  orgId: string,
  reqId: string,
): Promise<void> {
  await api.delete(`${requestsBase(orgId)}/${reqId}`);
}
