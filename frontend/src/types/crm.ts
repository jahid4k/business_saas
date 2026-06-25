// src/types/crm.ts
// All fields mirror backend JSON tags exactly (snake_case)

// ── Leads ─────────────────────────────────────────────
export type LeadStatus =
  | "new"
  | "contacted"
  | "qualified"
  | "unqualified"
  | "converted";

export interface Lead {
  id: string;
  public_id: string;
  org_id: string;
  first_name: string;
  last_name?: string;
  email?: string;
  phone?: string;
  company_name?: string;
  title?: string; // job title
  source?: string;
  status: LeadStatus;
  converted_at?: string;
  converted_contact_id?: string;
  converted_deal_id?: string;
  owner_id?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface LeadListResponse {
  leads: Lead[];
  total: number;
}

export interface CreateLeadPayload {
  first_name: string;
  last_name?: string;
  email?: string;
  phone?: string;
  company_name?: string;
  title?: string;
  source?: string;
  owner_id?: string;
}

export interface UpdateLeadPayload {
  first_name?: string;
  last_name?: string;
  email?: string;
  phone?: string;
  company_name?: string;
  title?: string;
  source?: string;
  status?: string;
  owner_id?: string;
}

export interface ConvertLeadPayload {
  create_contact: boolean;
  create_deal: boolean;
  deal_title?: string;
  pipeline_id?: string;
  stage_id?: string;
  deal_value?: number;
}

export interface ConvertLeadResponse {
  lead: Lead;
  contact_id?: string;
  deal_id?: string;
}

// ── Pipelines & Stages ────────────────────────────────
export interface Pipeline {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  is_default: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
  stages?: Stage[];
}

export interface Stage {
  id: string;
  public_id: string;
  org_id: string;
  pipeline_id: string;
  name: string;
  position: number;
  probability: number;
  created_at: string;
  updated_at: string;
}

export interface PipelineListResponse {
  pipelines: Pipeline[];
  total: number;
}

export interface StageListResponse {
  stages: Stage[];
  total: number;
}
