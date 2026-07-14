// src/lib/hrm/employees.ts
import api from "../api";
import type {
  Employee,
  EmployeeListResponse,
  EmployeeListFilter,
  CreateEmployeePayload,
  UpdateEmployeePayload,
  TerminateEmployeePayload,
  EmployeeStatusModel,
} from "@/types/hrm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/hrm/employees`;

export async function listEmployees(
  orgId: string,
  filter?: EmployeeListFilter,
): Promise<EmployeeListResponse> {
  const res = await api.get<{ success: boolean; data: EmployeeListResponse }>(
    base(orgId),
    {
      params: filter,
    },
  );
  return res.data.data;
}

export async function getEmployee(
  orgId: string,
  empId: string,
): Promise<Employee> {
  const res = await api.get<{ success: boolean; data: { employee: Employee } }>(
    `${base(orgId)}/${empId}`,
  );
  return res.data.data.employee;
}

export async function createEmployee(
  orgId: string,
  body: CreateEmployeePayload,
): Promise<Employee> {
  const res = await api.post<{
    success: boolean;
    data: { employee: Employee };
  }>(base(orgId), body);
  return res.data.data.employee;
}

export async function updateEmployee(
  orgId: string,
  empId: string,
  body: UpdateEmployeePayload,
): Promise<Employee> {
  const res = await api.patch<{
    success: boolean;
    data: { employee: Employee };
  }>(`${base(orgId)}/${empId}`, body);
  return res.data.data.employee;
}

export async function deleteEmployee(
  orgId: string,
  empId: string,
): Promise<void> {
  await api.delete(`${base(orgId)}/${empId}`);
}

export async function terminateEmployee(
  orgId: string,
  empId: string,
  body: TerminateEmployeePayload,
): Promise<Employee> {
  const res = await api.post<{
    success: boolean;
    data: { employee: Employee };
  }>(`${base(orgId)}/${empId}/terminate`, body);
  return res.data.data.employee;
}

export async function listEmployeeStatuses(
  orgId: string,
): Promise<EmployeeStatusModel[]> {
  const res = await api.get<{
    success: boolean;
    data: { statuses: EmployeeStatusModel[] };
  }>(`/api/v1/organizations/${orgId}/hrm/employee-statuses`);
  return res.data.data.statuses;
}

export async function createEmployeeStatus(
  orgId: string,
  payload: { name: string; category: string; color: string },
): Promise<EmployeeStatusModel> {
  const res = await api.post<{
    success: boolean;
    data: { status: EmployeeStatusModel };
  }>(`/api/v1/organizations/${orgId}/hrm/employee-statuses`, payload);
  return res.data.data.status;
}

export async function updateEmployeeStatus(
  orgId: string,
  statusId: string,
  payload: { name?: string; category?: string; color?: string },
): Promise<EmployeeStatusModel> {
  const res = await api.patch<{
    success: boolean;
    data: { status: EmployeeStatusModel };
  }>(
    `/api/v1/organizations/${orgId}/hrm/employee-statuses/${statusId}`,
    payload,
  );
  return res.data.data.status;
}

export async function deleteEmployeeStatus(
  orgId: string,
  statusId: string,
): Promise<void> {
  await api.delete(
    `/api/v1/organizations/${orgId}/hrm/employee-statuses/${statusId}`,
  );
}
