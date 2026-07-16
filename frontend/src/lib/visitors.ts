import api from "./api";

export interface WebsiteVisitor {
  id: string;
  org_id: string;
  session_id: string;
  ip_address?: string;
  user_agent?: string;
  company_name?: string;
  company_domain?: string;
  enrichment_data?: any;
  linked_lead_id?: string;
  created_at: string;
  updated_at: string;
}

export async function listVisitors(orgId: string): Promise<WebsiteVisitor[]> {
  const { data } = await api.get<{ data: WebsiteVisitor[] }>(`/api/v1/organizations/${orgId}/capture/visitors`);
  return data.data;
}
