// src/lib/crm/deals.ts
import api from "../api";
import type {
  Deal,
  DealListResponse,
  CreateDealPayload,
  UpdateDealPayload,
} from "@/types/crm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/crm/deals`;

// GET → data: { deals[], total }
export async function listDeals(orgId: string): Promise<DealListResponse> {
  const res = await api.get<{ success: boolean; data: DealListResponse }>(
    base(orgId),
  );
  return res.data.data;
}

// POST → data: { deal }
export async function createDeal(
  orgId: string,
  body: CreateDealPayload,
): Promise<Deal> {
  const res = await api.post<{ success: boolean; data: { deal: Deal } }>(
    base(orgId),
    body,
  );
  return res.data.data.deal;
}

// PATCH → data: { deal }
export async function updateDeal(
  orgId: string,
  dealId: string,
  body: UpdateDealPayload,
): Promise<Deal> {
  const res = await api.patch<{ success: boolean; data: { deal: Deal } }>(
    `${base(orgId)}/${dealId}`,
    body,
  );
  return res.data.data.deal;
}

// DELETE
export async function deleteDeal(orgId: string, dealId: string): Promise<void> {
  await api.delete(`${base(orgId)}/${dealId}`);
}

// POST /deals/:id/move → data: { deal }
export async function moveDeal(
  orgId: string,
  dealId: string,
  stageId: string,
): Promise<Deal> {
  const res = await api.post<{ success: boolean; data: { deal: Deal } }>(
    `${base(orgId)}/${dealId}/move`,
    { stage_id: stageId },
  );
  return res.data.data.deal;
}

// POST /deals/:id/won → data: { deal }
export async function markDealWon(
  orgId: string,
  dealId: string,
): Promise<Deal> {
  const res = await api.post<{ success: boolean; data: { deal: Deal } }>(
    `${base(orgId)}/${dealId}/won`,
  );
  return res.data.data.deal;
}

// POST /deals/:id/lost → data: { deal }
export async function markDealLost(
  orgId: string,
  dealId: string,
  lostReason?: string,
): Promise<Deal> {
  const res = await api.post<{ success: boolean; data: { deal: Deal } }>(
    `${base(orgId)}/${dealId}/lost`,
    lostReason ? { lost_reason: lostReason } : {},
  );
  return res.data.data.deal;
}
