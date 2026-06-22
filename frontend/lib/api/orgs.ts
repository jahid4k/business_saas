// lib/api/orgs.ts
import { apiGet, apiPost, apiPatch, apiDelete } from "@/lib/api";
import type { Organization, MembershipWithOrg } from "@/types/domain";

export interface OrgsListResponse {
  memberships: MembershipWithOrg[];
}

export interface CreateOrgInput {
  name: string;
  slug?: string;
}

export interface SwitchOrgResponse {
  access_token: string;
  role: string;
}

export const orgsApi = {
  list: () => apiGet<OrgsListResponse>("api/v1/organizations"),

  create: (body: CreateOrgInput) =>
    apiPost<{ organization: Organization }>("api/v1/organizations", body),

  switch: (orgId: string) =>
    apiPost<SwitchOrgResponse>(`api/v1/organizations/${orgId}/switch`),

  leave: (orgId: string) => apiDelete(`api/v1/organizations/${orgId}/leave`),

  update: (orgId: string, body: Partial<CreateOrgInput>) =>
    apiPatch<{ organization: Organization }>(
      `api/v1/organizations/${orgId}`,
      body,
    ),
};
