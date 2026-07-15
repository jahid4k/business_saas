import api from "@/lib/api";

function base(orgId: string) {
  return `/organizations/${orgId}/crm/settings`;
}

export interface CRMSettings {
  org_id: string;
  lead_routing_enabled: boolean;
  round_robin_assignees: string[];
  created_at: string;
  updated_at: string;
}

export interface UpdateCRMSettingsPayload {
  lead_routing_enabled?: boolean;
  round_robin_assignees?: string[];
}

export async function getCRMSettings(orgId: string): Promise<CRMSettings> {
  const res = await api.get<{ success: boolean; data: CRMSettings }>(base(orgId));
  return res.data.data;
}

export async function updateCRMSettings(orgId: string, payload: UpdateCRMSettingsPayload): Promise<CRMSettings> {
  const res = await api.patch<{ success: boolean; data: CRMSettings }>(base(orgId), payload);
  return res.data.data;
}
