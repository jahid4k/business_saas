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

// Add these to the existing src/types/crm.ts file

// ── Companies ──────────────────────────────────────────
export interface Company {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  domain?: string;
  industry?: string;
  website?: string;
  phone?: string;
  address?: string;
  country?: string;
  status: string;
  owner_id?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CompanyListResponse {
  companies: Company[];
  total: number;
}

export interface CreateCompanyPayload {
  name: string;
  domain?: string;
  industry?: string;
  website?: string;
  phone?: string;
  address?: string;
  country?: string;
  owner_id?: string;
}

export interface UpdateCompanyPayload {
  name?: string;
  domain?: string;
  industry?: string;
  website?: string;
  phone?: string;
  address?: string;
  country?: string;
  owner_id?: string;
}

// ── Contacts ───────────────────────────────────────────
export interface Contact {
  id: string;
  public_id: string;
  org_id: string;
  first_name: string;
  last_name?: string;
  email?: string;
  phone?: string;
  title?: string;
  company_id?: string;
  source?: string;
  status: string;
  owner_id?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ContactListResponse {
  contacts: Contact[];
  total: number;
}

export interface CreateContactPayload {
  first_name: string;
  last_name?: string;
  email?: string;
  phone?: string;
  title?: string;
  company_id?: string;
  source?: string;
  owner_id?: string;
}

export interface UpdateContactPayload {
  first_name?: string;
  last_name?: string;
  email?: string;
  phone?: string;
  title?: string;
  company_id?: string;
  source?: string;
  owner_id?: string;
}

// ── Deals ──────────────────────────────────────────────
export type DealStatus = "open" | "won" | "lost";

export interface Deal {
  id: string;
  public_id: string;
  org_id: string;
  title: string;
  value: number;
  currency: string;
  pipeline_id: string;
  stage_id: string;
  contact_id?: string;
  company_id?: string;
  status: DealStatus;
  close_date?: string;
  lost_reason?: string;
  owner_id?: string;
  won_at?: string;
  lost_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface DealListResponse {
  deals: Deal[];
  total: number;
}

export interface CreateDealPayload {
  title: string;
  value: number;
  currency?: string;
  pipeline_id: string;
  stage_id: string;
  contact_id?: string;
  company_id?: string;
  close_date?: string;
  owner_id?: string;
}

export interface UpdateDealPayload {
  title?: string;
  value?: number;
  currency?: string;
  contact_id?: string;
  company_id?: string;
  close_date?: string;
  owner_id?: string;
}

// ── Reports ────────────────────────────────────────────
export interface CRMSummary {
  total_contacts: number;
  total_companies: number;
  total_leads: number;
  total_deals: number;
  open_deals: number;
  won_deals: number;
  lost_deals: number;
  total_deal_value: number;
  won_deal_value: number;
}

export interface DealByStage {
  stage_id: string;
  stage_name: string;
  count: number;
  total_value: number;
}

export interface LeadBySource {
  source: string;
  count: number;
}

export interface OverviewResponse {
  summary: CRMSummary;
  recent_deals: Deal[];
}
