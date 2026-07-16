import api from "./api";

export interface OrgInboundEmail {
  id: string;
  org_id: string;
  address: string;
  is_active: boolean;
  created_at: string;
}

export interface SocialIntegration {
  id: string;
  org_id: string;
  platform: string;
  page_id: string;
  is_active: boolean;
  created_at: string;
}

export async function listOrgEmails(orgId: string): Promise<OrgInboundEmail[]> {
  const { data } = await api.get<{ data: OrgInboundEmail[] }>(`/api/v1/organizations/${orgId}/capture/email`);
  return data.data;
}

export async function createOrgEmail(orgId: string, address: string): Promise<OrgInboundEmail> {
  const { data } = await api.post<{ data: OrgInboundEmail }>(`/api/v1/organizations/${orgId}/capture/email`, { address });
  return data.data;
}

export async function deleteOrgEmail(orgId: string, id: string): Promise<void> {
  await api.delete(`/api/v1/organizations/${orgId}/capture/email/${id}`);
}

export async function listOrgSocials(orgId: string): Promise<SocialIntegration[]> {
  const { data } = await api.get<{ data: SocialIntegration[] }>(`/api/v1/organizations/${orgId}/capture/social`);
  return data.data;
}

export async function createOrgSocial(orgId: string, platform: string, pageId: string): Promise<SocialIntegration> {
  const { data } = await api.post<{ data: SocialIntegration }>(`/api/v1/organizations/${orgId}/capture/social`, { platform, page_id: pageId });
  return data.data;
}

export async function deleteOrgSocial(orgId: string, id: string): Promise<void> {
  await api.delete(`/api/v1/organizations/${orgId}/capture/social/${id}`);
}
