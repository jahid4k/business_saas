// lib/api/members.ts
import { apiGet, apiPatch, apiDelete } from "@/lib/api";
import type { MemberWithUser, MyMembership } from "@/types/domain";

const base = (orgId: string) => `api/v1/organizations/${orgId}/members`;

export const membersApi = {
  list: (orgId: string) => apiGet<{ members: MemberWithUser[] }>(base(orgId)),

  me: (orgId: string) =>
    apiGet<{ membership: MyMembership }>(`${base(orgId)}/me`),

  updateRole: (orgId: string, memberId: string, roleId: string) =>
    apiPatch<{ membership: MemberWithUser }>(`${base(orgId)}/${memberId}`, {
      role_id: roleId,
    }),

  remove: (orgId: string, memberId: string) =>
    apiDelete(`${base(orgId)}/${memberId}`),
};
