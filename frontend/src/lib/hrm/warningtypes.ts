// src/lib/hrm/warningtypes.ts
import api from "../api";
import type {
  WarningType,
  WarningTypeListResponse,
  CreateWarningTypePayload,
  UpdateWarningTypePayload,
  WarningEscalationRule,
  EscalationRuleListResponse,
  CreateEscalationRulePayload,
} from "@/types/hrm";

const typesUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/warning-types`;
const escalationsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/warning-types/escalations`;

export async function listWarningTypes(
  orgId: string,
  opts?: { activeOnly?: boolean },
): Promise<WarningTypeListResponse> {
  const res = await api.get<{
    success: boolean;
    data: WarningTypeListResponse;
  }>(typesUrl(orgId), {
    params: opts?.activeOnly ? { active: "true" } : undefined,
  });
  return res.data.data;
}

export async function createWarningType(
  orgId: string,
  body: CreateWarningTypePayload,
): Promise<WarningType> {
  const res = await api.post<{
    success: boolean;
    data: { warning_type: WarningType };
  }>(typesUrl(orgId), body);
  return res.data.data.warning_type;
}

export async function updateWarningType(
  orgId: string,
  typeId: string,
  body: UpdateWarningTypePayload,
): Promise<WarningType> {
  const res = await api.patch<{
    success: boolean;
    data: { warning_type: WarningType };
  }>(`${typesUrl(orgId)}/${typeId}`, body);
  return res.data.data.warning_type;
}

export async function deleteWarningType(
  orgId: string,
  typeId: string,
): Promise<void> {
  await api.delete(`${typesUrl(orgId)}/${typeId}`);
}

export async function listEscalationRules(
  orgId: string,
  warningTypeId?: string,
): Promise<EscalationRuleListResponse> {
  const res = await api.get<{
    success: boolean;
    data: EscalationRuleListResponse;
  }>(escalationsUrl(orgId), {
    params: warningTypeId ? { warning_type_id: warningTypeId } : undefined,
  });
  return res.data.data;
}

export async function createEscalationRule(
  orgId: string,
  body: CreateEscalationRulePayload,
): Promise<WarningEscalationRule> {
  const res = await api.post<{
    success: boolean;
    data: { rule: WarningEscalationRule };
  }>(escalationsUrl(orgId), body);
  return res.data.data.rule;
}

export async function deleteEscalationRule(
  orgId: string,
  ruleId: string,
): Promise<void> {
  await api.delete(`${escalationsUrl(orgId)}/${ruleId}`);
}
