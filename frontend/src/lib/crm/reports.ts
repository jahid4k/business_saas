// src/lib/crm/reports.ts
import api from "../api";
import type { OverviewResponse, DealByStage, LeadBySource } from "@/types/crm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/crm/reports`;

// GET /reports/overview → data: { summary, recent_deals }
export async function getOverview(orgId: string): Promise<OverviewResponse> {
  const res = await api.get<{ success: boolean; data: OverviewResponse }>(
    `${base(orgId)}/overview`,
  );
  return res.data.data;
}

// GET /reports/deals/by-stage → data: { deals_by_stage[] }
export async function getDealsByStage(orgId: string): Promise<DealByStage[]> {
  const res = await api.get<{
    success: boolean;
    data: { deals_by_stage: DealByStage[] };
  }>(`${base(orgId)}/deals/by-stage`);
  return res.data.data.deals_by_stage ?? [];
}

// GET /reports/leads/by-source → data: { leads_by_source[] }
export async function getLeadsBySource(orgId: string): Promise<LeadBySource[]> {
  const res = await api.get<{
    success: boolean;
    data: { leads_by_source: LeadBySource[] };
  }>(`${base(orgId)}/leads/by-source`);
  return res.data.data.leads_by_source ?? [];
}

export interface AgendaItem {
  id: string;
  type: string;
  title: string;
  description: string;
  status: string;
  due_date?: string;
  occurred_at?: string;
  related_type?: string;
  related_id?: string;
  updated_at?: string;
}

export async function getAgenda(orgId: string): Promise<AgendaItem[]> {
  const res = await api.get<{
    success: boolean;
    data: { items: AgendaItem[] };
  }>(`${base(orgId)}/agenda`);
  return res.data.data.items ?? [];
}

export interface RepPerformance {
  rep_id: string;
  rep_name: string;
  calls: number;
  meetings: number;
  deals_closed: number;
  revenue_won: number;
}

export async function getRepPerformance(
  orgId: string,
): Promise<RepPerformance[]> {
  const res = await api.get<{
    success: boolean;
    data: { rep_performance: RepPerformance[] };
  }>(`${base(orgId)}/rep-performance`);
  return res.data.data.rep_performance ?? [];
}

export interface Forecast {
  total_pipeline_value: number;
  weighted_forecast: number;
}

export async function getForecast(orgId: string): Promise<Forecast> {
  const res = await api.get<{ success: boolean; data: Forecast }>(
    `${base(orgId)}/forecast`,
  );
  return res.data.data;
}
