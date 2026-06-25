// src/lib/crm/leads.ts
import api from "../api";
import type {
  Lead,
  LeadListResponse,
  CreateLeadPayload,
  UpdateLeadPayload,
  ConvertLeadPayload,
  ConvertLeadResponse,
} from "@/types/crm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/crm/leads`;

// GET → data: { leads[], total }
export async function listLeads(orgId: string): Promise<LeadListResponse> {
  const res = await api.get<{ success: boolean; data: LeadListResponse }>(
    base(orgId),
  );
  return res.data.data;
}

// GET → data: { lead }
export async function getLead(orgId: string, leadId: string): Promise<Lead> {
  const res = await api.get<{ success: boolean; data: { lead: Lead } }>(
    `${base(orgId)}/${leadId}`,
  );
  return res.data.data.lead;
}

// POST → data: { lead }
export async function createLead(
  orgId: string,
  body: CreateLeadPayload,
): Promise<Lead> {
  const res = await api.post<{ success: boolean; data: { lead: Lead } }>(
    base(orgId),
    body,
  );
  return res.data.data.lead;
}

// PATCH → data: { lead }
export async function updateLead(
  orgId: string,
  leadId: string,
  body: UpdateLeadPayload,
): Promise<Lead> {
  const res = await api.patch<{ success: boolean; data: { lead: Lead } }>(
    `${base(orgId)}/${leadId}`,
    body,
  );
  return res.data.data.lead;
}

// DELETE
export async function deleteLead(orgId: string, leadId: string): Promise<void> {
  await api.delete(`${base(orgId)}/${leadId}`);
}

// POST → data: { lead, contact_id?, deal_id? }
export async function convertLead(
  orgId: string,
  leadId: string,
  body: ConvertLeadPayload,
): Promise<ConvertLeadResponse> {
  const res = await api.post<{ success: boolean; data: ConvertLeadResponse }>(
    `${base(orgId)}/${leadId}/convert`,
    body,
  );
  return res.data.data;
}
