// src/lib/hrm/departments.ts
import api from "../api";
import type {
  Department,
  DepartmentListResponse,
  CreateDepartmentPayload,
  UpdateDepartmentPayload,
} from "@/types/hrm";

const base = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/departments`;

export async function listDepartments(
  orgId: string,
): Promise<DepartmentListResponse> {
  const res = await api.get<{ success: boolean; data: DepartmentListResponse }>(
    base(orgId),
  );
  return res.data.data;
}

export async function getDepartment(
  orgId: string,
  deptId: string,
): Promise<Department> {
  const res = await api.get<{
    success: boolean;
    data: { department: Department };
  }>(`${base(orgId)}/${deptId}`);
  return res.data.data.department;
}

export async function createDepartment(
  orgId: string,
  body: CreateDepartmentPayload,
): Promise<Department> {
  const res = await api.post<{
    success: boolean;
    data: { department: Department };
  }>(base(orgId), body);
  return res.data.data.department;
}

export async function updateDepartment(
  orgId: string,
  deptId: string,
  body: UpdateDepartmentPayload,
): Promise<Department> {
  const res = await api.patch<{
    success: boolean;
    data: { department: Department };
  }>(`${base(orgId)}/${deptId}`, body);
  return res.data.data.department;
}

export async function deleteDepartment(
  orgId: string,
  deptId: string,
): Promise<void> {
  await api.delete(`${base(orgId)}/${deptId}`);
}
