// lib/api/authz.ts
import { apiGet, apiPost } from "@/lib/api";
import type { Role, Permission, RoleWithPermissions } from "@/types/domain";

const rolesBase = (orgId: string) => `api/v1/organizations/${orgId}/roles`;
const permsBase = (orgId: string) =>
  `api/v1/organizations/${orgId}/permissions`;

export const authzApi = {
  roles: {
    list: (orgId: string) => apiGet<{ roles: Role[] }>(rolesBase(orgId)),

    get: (orgId: string, roleId: string) =>
      apiGet<{ role: RoleWithPermissions }>(`${rolesBase(orgId)}/${roleId}`),

    assignPermissions: (
      orgId: string,
      roleId: string,
      permissionIds: string[],
    ) =>
      apiPost<{ role: RoleWithPermissions }>(
        `${rolesBase(orgId)}/${roleId}/permissions`,
        { permission_ids: permissionIds },
      ),
  },

  permissions: {
    list: (orgId: string) =>
      apiGet<{ permissions: Permission[] }>(permsBase(orgId)),
  },
};
