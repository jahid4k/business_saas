// src/lib/hrm/lifecycle.ts
import api from "../api";
import type {
  Promotion,
  PromotionListResponse,
  CreatePromotionPayload,
  Transfer,
  TransferListResponse,
  CreateTransferPayload,
  Resignation,
  ResignationListResponse,
  SubmitResignationPayload,
  Termination,
  TerminationListResponse,
  CreateTerminationPayload,
} from "@/types/hrm";

interface LifecycleListFilter {
  employee_id?: string;
  status?: string;
}

// ── Promotions ────────────────────────────────────────────
const promoAllUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/promotions`;
const promoBase = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/promotions`;

export async function listAllPromotions(
  orgId: string,
  filter?: LifecycleListFilter,
): Promise<PromotionListResponse> {
  const res = await api.get<{ success: boolean; data: PromotionListResponse }>(
    promoAllUrl(orgId),
    {
      params: filter,
    },
  );
  return res.data.data;
}

export async function createPromotion(
  orgId: string,
  employeeId: string,
  body: CreatePromotionPayload,
): Promise<Promotion> {
  const res = await api.post<{
    success: boolean;
    data: { promotion: Promotion };
  }>(promoBase(orgId, employeeId), body);
  return res.data.data.promotion;
}

export async function submitPromotion(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Promotion> {
  const res = await api.post<{
    success: boolean;
    data: { promotion: Promotion };
  }>(`${promoBase(orgId, employeeId)}/${id}/submit`, {});
  return res.data.data.promotion;
}

export async function cancelPromotion(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Promotion> {
  const res = await api.post<{
    success: boolean;
    data: { promotion: Promotion };
  }>(`${promoBase(orgId, employeeId)}/${id}/cancel`, {});
  return res.data.data.promotion;
}

export async function applyPromotion(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Promotion> {
  const res = await api.post<{
    success: boolean;
    data: { promotion: Promotion };
  }>(`${promoBase(orgId, employeeId)}/${id}/apply`, {});
  return res.data.data.promotion;
}

// ── Transfers ─────────────────────────────────────────────
const transferAllUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/transfers`;
const transferBase = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/transfers`;

export async function listAllTransfers(
  orgId: string,
  filter?: LifecycleListFilter,
): Promise<TransferListResponse> {
  const res = await api.get<{ success: boolean; data: TransferListResponse }>(
    transferAllUrl(orgId),
    {
      params: filter,
    },
  );
  return res.data.data;
}

export async function createTransfer(
  orgId: string,
  employeeId: string,
  body: CreateTransferPayload,
): Promise<Transfer> {
  const res = await api.post<{
    success: boolean;
    data: { transfer: Transfer };
  }>(transferBase(orgId, employeeId), body);
  return res.data.data.transfer;
}

export async function submitTransfer(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Transfer> {
  const res = await api.post<{
    success: boolean;
    data: { transfer: Transfer };
  }>(`${transferBase(orgId, employeeId)}/${id}/submit`, {});
  return res.data.data.transfer;
}

export async function cancelTransfer(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Transfer> {
  const res = await api.post<{
    success: boolean;
    data: { transfer: Transfer };
  }>(`${transferBase(orgId, employeeId)}/${id}/cancel`, {});
  return res.data.data.transfer;
}

export async function applyTransfer(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Transfer> {
  const res = await api.post<{
    success: boolean;
    data: { transfer: Transfer };
  }>(`${transferBase(orgId, employeeId)}/${id}/apply`, {});
  return res.data.data.transfer;
}

// ── Resignations ──────────────────────────────────────────
const resignAllUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/resignations`;
const resignBase = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/resignations`;

export async function listAllResignations(
  orgId: string,
  filter?: LifecycleListFilter,
): Promise<ResignationListResponse> {
  const res = await api.get<{
    success: boolean;
    data: ResignationListResponse;
  }>(resignAllUrl(orgId), {
    params: filter,
  });
  return res.data.data;
}

export async function submitResignation(
  orgId: string,
  employeeId: string,
  body: SubmitResignationPayload,
): Promise<Resignation> {
  const res = await api.post<{
    success: boolean;
    data: { resignation: Resignation };
  }>(resignBase(orgId, employeeId), body);
  return res.data.data.resignation;
}

export async function withdrawResignation(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Resignation> {
  const res = await api.post<{
    success: boolean;
    data: { resignation: Resignation };
  }>(`${resignBase(orgId, employeeId)}/${id}/withdraw`, {});
  return res.data.data.resignation;
}

export async function acceptResignation(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Resignation> {
  const res = await api.post<{
    success: boolean;
    data: { resignation: Resignation };
  }>(`${resignBase(orgId, employeeId)}/${id}/accept`, {});
  return res.data.data.resignation;
}

export async function rejectResignation(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Resignation> {
  const res = await api.post<{
    success: boolean;
    data: { resignation: Resignation };
  }>(`${resignBase(orgId, employeeId)}/${id}/reject`, {});
  return res.data.data.resignation;
}

// ── Terminations ──────────────────────────────────────────
const termAllUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/terminations`;
const termBase = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/terminations`;

export async function listAllTerminations(
  orgId: string,
  filter?: LifecycleListFilter,
): Promise<TerminationListResponse> {
  const res = await api.get<{
    success: boolean;
    data: TerminationListResponse;
  }>(termAllUrl(orgId), {
    params: filter,
  });
  return res.data.data;
}

export async function createTermination(
  orgId: string,
  employeeId: string,
  body: CreateTerminationPayload,
): Promise<Termination> {
  const res = await api.post<{
    success: boolean;
    data: { termination: Termination };
  }>(termBase(orgId, employeeId), body);
  return res.data.data.termination;
}

export async function submitTermination(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Termination> {
  const res = await api.post<{
    success: boolean;
    data: { termination: Termination };
  }>(`${termBase(orgId, employeeId)}/${id}/submit`, {});
  return res.data.data.termination;
}

export async function cancelTermination(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Termination> {
  const res = await api.post<{
    success: boolean;
    data: { termination: Termination };
  }>(`${termBase(orgId, employeeId)}/${id}/cancel`, {});
  return res.data.data.termination;
}

export async function applyTermination(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Termination> {
  const res = await api.post<{
    success: boolean;
    data: { termination: Termination };
  }>(`${termBase(orgId, employeeId)}/${id}/apply`, {});
  return res.data.data.termination;
}
