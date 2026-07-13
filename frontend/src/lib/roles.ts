// src/lib/roles.ts
import api from "./api";
import type { Role, RoleWithMeta, Permission } from "@/types/rbac";

// GET /organizations/:orgId/rbac/roles → data.roles: RoleWithMeta[]
export async function listRoles(orgId: string): Promise<RoleWithMeta[]> {
  const res = await api.get<{
    success: boolean;
    data: { roles: RoleWithMeta[] };
  }>(`/api/v1/organizations/${orgId}/rbac/roles`);
  return res.data.data.roles ?? [];
}

// GET /organizations/:orgId/rbac/permissions → data.permissions: Permission[]
export async function listPermissions(orgId: string): Promise<Permission[]> {
  const res = await api.get<{
    success: boolean;
    data: { permissions: Permission[] };
  }>(`/api/v1/organizations/${orgId}/rbac/permissions`);
  return res.data.data.permissions ?? [];
}

// PATCH /organizations/:orgId/rbac/roles/:roleId/permissions
// Sends full replacement list of permissionKeys
export async function updateRolePermissions(
  orgId: string,
  roleId: string,
  permissionKeys: string[],
): Promise<Role> {
  const res = await api.patch<{ success: boolean; data: { role: Role } }>(
    `/api/v1/organizations/${orgId}/rbac/roles/${roleId}/permissions`,
    { permissionKeys },
  );
  return res.data.data.role;
}

// POST /organizations/:orgId/rbac/roles/:roleId/clone
export async function cloneRole(
  orgId: string,
  roleId: string,
  name: string,
): Promise<Role> {
  const res = await api.post<{ success: boolean; data: { role: Role } }>(
    `/api/v1/organizations/${orgId}/rbac/roles/${roleId}/clone`,
    { name },
  );
  return res.data.data.role;
}

// DELETE /organizations/:orgId/rbac/roles/:roleId
export async function deleteRole(orgId: string, roleId: string): Promise<void> {
  await api.delete(`/api/v1/organizations/${orgId}/rbac/roles/${roleId}`);
}

// POST /organizations/:orgId/rbac/roles
export async function createRole(
  orgId: string,
  body: { name: string; description: string; permissionKeys: string[] },
): Promise<Role> {
  const res = await api.post<{ success: boolean; data: { role: Role } }>(
    `/api/v1/organizations/${orgId}/rbac/roles`,
    body,
  );
  return res.data.data.role;
}
