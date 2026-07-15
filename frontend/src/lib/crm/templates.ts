// src/lib/crm/templates.ts
import api from "@/lib/api";

export type TemplateType = "email" | "note";

export interface TemplateModel {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  type: TemplateType;
  subject?: string;
  body: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export async function listTemplates(orgId: string): Promise<TemplateModel[]> {
  const res = await api.get(`/${orgId}/crm/templates`);
  return res.data.templates;
}

export async function createTemplate(
  orgId: string,
  payload: { name: string; type: string; subject?: string; body: string }
): Promise<TemplateModel> {
  const res = await api.post(`/${orgId}/crm/templates`, payload);
  return res.data.template;
}

export async function updateTemplate(
  orgId: string,
  templateId: string,
  payload: { name?: string; subject?: string; body?: string }
): Promise<TemplateModel> {
  const res = await api.patch(`/${orgId}/crm/templates/${templateId}`, payload);
  return res.data.template;
}

export async function deleteTemplate(orgId: string, templateId: string): Promise<void> {
  await api.delete(`/${orgId}/crm/templates/${templateId}`);
}
