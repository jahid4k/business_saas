// src/lib/hrm/warnings.ts
import api from "../api";
import type {
  EmployeeWarning,
  WarningListResponse,
  CreateWarningPayload,
  AppealWarningPayload,
  CloseWarningPayload,
} from "@/types/hrm";

const allUrl = (orgId: string) => `/api/v1/organizations/${orgId}/hrm/warnings`;
const base = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/warnings`;

export async function listAllWarnings(
  orgId: string,
): Promise<WarningListResponse> {
  const res = await api.get<{ success: boolean; data: WarningListResponse }>(
    allUrl(orgId),
  );
  return res.data.data;
}

export async function createWarning(
  orgId: string,
  employeeId: string,
  body: CreateWarningPayload,
): Promise<EmployeeWarning> {
  const res = await api.post<{
    success: boolean;
    data: { warning: EmployeeWarning };
  }>(base(orgId, employeeId), body);
  return res.data.data.warning;
}

export async function issueWarning(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<EmployeeWarning> {
  const res = await api.post<{
    success: boolean;
    data: { warning: EmployeeWarning };
  }>(`${base(orgId, employeeId)}/${id}/issue`, {});
  return res.data.data.warning;
}

export async function acknowledgeWarning(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<EmployeeWarning> {
  const res = await api.post<{
    success: boolean;
    data: { warning: EmployeeWarning };
  }>(`${base(orgId, employeeId)}/${id}/acknowledge`, {});
  return res.data.data.warning;
}

export async function appealWarning(
  orgId: string,
  employeeId: string,
  id: string,
  body: AppealWarningPayload,
): Promise<EmployeeWarning> {
  const res = await api.post<{
    success: boolean;
    data: { warning: EmployeeWarning };
  }>(`${base(orgId, employeeId)}/${id}/appeal`, body);
  return res.data.data.warning;
}

export async function closeWarning(
  orgId: string,
  employeeId: string,
  id: string,
  body?: CloseWarningPayload,
): Promise<EmployeeWarning> {
  const res = await api.post<{
    success: boolean;
    data: { warning: EmployeeWarning };
  }>(`${base(orgId, employeeId)}/${id}/close`, body ?? {});
  return res.data.data.warning;
}

export async function cancelWarning(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<EmployeeWarning> {
  const res = await api.post<{
    success: boolean;
    data: { warning: EmployeeWarning };
  }>(`${base(orgId, employeeId)}/${id}/cancel`, {});
  return res.data.data.warning;
}
