// src/lib/org.ts
import api from "./api";
import type {
  Business,
  MembershipWithRole,
  MyMembershipResponse,
  SwitchOrgResponse,
  CreateOrgRequest,
} from "@/types/org";

// GET /api/v1/organizations → data.organizations: MembershipWithRole[]
export async function listOrganizations(): Promise<MembershipWithRole[]> {
  const res = await api.get<{
    success: boolean;
    data: { organizations: MembershipWithRole[] };
  }>("/api/v1/organizations");
  return res.data.data.organizations ?? [];
}

// POST /api/v1/organizations → data.organization: Business
export async function createOrganization(
  body: CreateOrgRequest,
): Promise<Business> {
  const res = await api.post<{
    success: boolean;
    data: { organization: Business };
  }>("/api/v1/organizations", body);
  return res.data.data.organization;
}

// GET /api/v1/organizations/:id → data.organization: Business
export async function getOrganization(id: string): Promise<Business> {
  const res = await api.get<{
    success: boolean;
    data: { organization: Business };
  }>(`/api/v1/organizations/${id}`);
  return res.data.data.organization;
}

// POST /api/v1/organizations/:id/switch → data: { access_token, role, organization_id }
export async function switchOrganization(
  id: string,
): Promise<SwitchOrgResponse> {
  const res = await api.post<{
    success: boolean;
    data: SwitchOrgResponse;
  }>(`/api/v1/organizations/${id}/switch`);
  return res.data.data;
}

// GET /api/v1/members/me → data.membership (needs business_id in JWT)
// Call this AFTER switchOrganization sets the new token
export async function getMyMembership(): Promise<MyMembershipResponse> {
  const res = await api.get<{
    success: boolean;
    data: { membership: MyMembershipResponse };
  }>("/api/v1/members/me");
  return res.data.data.membership;
}
