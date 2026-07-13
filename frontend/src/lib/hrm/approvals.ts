// src/lib/hrm/approvals.ts
import api from "../api";
import type {
  ApprovalTemplate,
  TemplateListResponse,
  CreateTemplatePayload,
  UpdateTemplatePayload,
  ApprovalInstance,
  DecisionPayload,
} from "@/types/hrm";

const templatesUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/approvals`;
const instancesUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/approvals/instances`;

export async function listApprovalTemplates(
  orgId: string,
  actionType?: string,
): Promise<TemplateListResponse> {
  const res = await api.get<{ success: boolean; data: TemplateListResponse }>(
    templatesUrl(orgId),
    {
      params: actionType ? { action_type: actionType } : undefined,
    },
  );
  return res.data.data;
}

export async function createApprovalTemplate(
  orgId: string,
  body: CreateTemplatePayload,
): Promise<ApprovalTemplate> {
  const res = await api.post<{
    success: boolean;
    data: { template: ApprovalTemplate };
  }>(templatesUrl(orgId), body);
  return res.data.data.template;
}

export async function updateApprovalTemplate(
  orgId: string,
  templateId: string,
  body: UpdateTemplatePayload,
): Promise<ApprovalTemplate> {
  const res = await api.patch<{
    success: boolean;
    data: { template: ApprovalTemplate };
  }>(`${templatesUrl(orgId)}/${templateId}`, body);
  return res.data.data.template;
}

export async function deleteApprovalTemplate(
  orgId: string,
  templateId: string,
): Promise<void> {
  await api.delete(`${templatesUrl(orgId)}/${templateId}`);
}

export async function getApprovalInstance(
  orgId: string,
  instanceId: string,
): Promise<ApprovalInstance> {
  const res = await api.get<{
    success: boolean;
    data: { instance: ApprovalInstance };
  }>(`${instancesUrl(orgId)}/${instanceId}`);
  return res.data.data.instance;
}

export async function approveInstance(
  orgId: string,
  instanceId: string,
  body?: DecisionPayload,
): Promise<ApprovalInstance> {
  const res = await api.post<{
    success: boolean;
    data: { instance: ApprovalInstance };
  }>(`${instancesUrl(orgId)}/${instanceId}/approve`, body ?? {});
  return res.data.data.instance;
}

export async function rejectInstance(
  orgId: string,
  instanceId: string,
  body?: DecisionPayload,
): Promise<ApprovalInstance> {
  const res = await api.post<{
    success: boolean;
    data: { instance: ApprovalInstance };
  }>(`${instancesUrl(orgId)}/${instanceId}/reject`, body ?? {});
  return res.data.data.instance;
}
