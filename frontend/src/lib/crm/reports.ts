// src/lib/crm/reports.ts
import api from '../api'
import type { OverviewResponse, DealByStage, LeadBySource } from '@/types/crm'

const base = (orgId: string) => `/api/v1/organizations/${orgId}/crm/reports`

// GET /reports/overview → data: { summary, recent_deals }
export async function getOverview(orgId: string): Promise<OverviewResponse> {
  const res = await api.get<{ success: boolean; data: OverviewResponse }>(
    `${base(orgId)}/overview`
  )
  return res.data.data
}

// GET /reports/deals/by-stage → data: { deals_by_stage[] }
export async function getDealsByStage(orgId: string): Promise<DealByStage[]> {
  const res = await api.get<{ success: boolean; data: { deals_by_stage: DealByStage[] } }>(
    `${base(orgId)}/deals/by-stage`
  )
  return res.data.data.deals_by_stage ?? []
}

// GET /reports/leads/by-source → data: { leads_by_source[] }
export async function getLeadsBySource(orgId: string): Promise<LeadBySource[]> {
  const res = await api.get<{ success: boolean; data: { leads_by_source: LeadBySource[] } }>(
    `${base(orgId)}/leads/by-source`
  )
  return res.data.data.leads_by_source ?? []
}