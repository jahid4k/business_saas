// src/lib/hrm/doctemplates.ts
import api from "../api";
import type {
  DocumentTemplate,
  DocumentTemplateListResponse,
  CreateDocumentTemplatePayload,
  UpdateDocumentTemplatePayload,
  PreviewTemplateResult,
} from "@/types/hrm";

const templatesUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/document-templates`;

export async function listDocumentTemplates(
  orgId: string,
  opts?: { activeOnly?: boolean; documentType?: string },
): Promise<DocumentTemplateListResponse> {
  const res = await api.get<{
    success: boolean;
    data: DocumentTemplateListResponse;
  }>(templatesUrl(orgId), {
    params: {
      active: opts?.activeOnly ? "true" : undefined,
      document_type: opts?.documentType,
    },
  });
  return res.data.data;
}

export async function createDocumentTemplate(
  orgId: string,
  body: CreateDocumentTemplatePayload,
): Promise<DocumentTemplate> {
  const res = await api.post<{
    success: boolean;
    data: { template: DocumentTemplate };
  }>(templatesUrl(orgId), body);
  return res.data.data.template;
}

export async function updateDocumentTemplate(
  orgId: string,
  templateId: string,
  body: UpdateDocumentTemplatePayload,
): Promise<DocumentTemplate> {
  const res = await api.patch<{
    success: boolean;
    data: { template: DocumentTemplate };
  }>(`${templatesUrl(orgId)}/${templateId}`, body);
  return res.data.data.template;
}

export async function deleteDocumentTemplate(
  orgId: string,
  templateId: string,
): Promise<void> {
  await api.delete(`${templatesUrl(orgId)}/${templateId}`);
}

export async function previewDocumentTemplate(
  orgId: string,
  templateId: string,
  variables: Record<string, string>,
): Promise<PreviewTemplateResult> {
  const res = await api.post<{
    success: boolean;
    data: { preview: PreviewTemplateResult };
  }>(`${templatesUrl(orgId)}/${templateId}/preview`, { variables });
  return res.data.data.preview;
}
