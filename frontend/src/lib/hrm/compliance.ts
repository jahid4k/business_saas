// src/lib/hrm/compliance.ts
import api from "../api";
import type {
  Complaint,
  ComplaintListResponse,
  CreateComplaintPayload,
  AssignComplaintPayload,
  ResolveComplaintPayload,
  DismissComplaintPayload,
  EmployeeDocument,
  EmployeeDocumentListResponse,
  CreateEmployeeDocumentPayload,
  Acknowledgement,
  AcknowledgementListResponse,
  CreateAcknowledgementPayload,
  RespondAcknowledgementPayload,
  DeclineAcknowledgementPayload,
} from "@/types/hrm";

// ── Complaints ────────────────────────────────────────────
const complaintsAllUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/complaints`;
const complaintsBase = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/complaints`;

export async function listAllComplaints(
  orgId: string,
): Promise<ComplaintListResponse> {
  const res = await api.get<{ success: boolean; data: ComplaintListResponse }>(
    complaintsAllUrl(orgId),
  );
  return res.data.data;
}

export async function createComplaint(
  orgId: string,
  employeeId: string,
  body: CreateComplaintPayload,
): Promise<Complaint> {
  const res = await api.post<{
    success: boolean;
    data: { complaint: Complaint };
  }>(complaintsBase(orgId, employeeId), body);
  return res.data.data.complaint;
}

export async function startReviewComplaint(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Complaint> {
  const res = await api.post<{
    success: boolean;
    data: { complaint: Complaint };
  }>(`${complaintsBase(orgId, employeeId)}/${id}/start-review`, {});
  return res.data.data.complaint;
}

export async function assignComplaint(
  orgId: string,
  employeeId: string,
  id: string,
  body: AssignComplaintPayload,
): Promise<Complaint> {
  const res = await api.post<{
    success: boolean;
    data: { complaint: Complaint };
  }>(`${complaintsBase(orgId, employeeId)}/${id}/assign`, body);
  return res.data.data.complaint;
}

export async function resolveComplaint(
  orgId: string,
  employeeId: string,
  id: string,
  body: ResolveComplaintPayload,
): Promise<Complaint> {
  const res = await api.post<{
    success: boolean;
    data: { complaint: Complaint };
  }>(`${complaintsBase(orgId, employeeId)}/${id}/resolve`, body);
  return res.data.data.complaint;
}

export async function dismissComplaint(
  orgId: string,
  employeeId: string,
  id: string,
  body: DismissComplaintPayload,
): Promise<Complaint> {
  const res = await api.post<{
    success: boolean;
    data: { complaint: Complaint };
  }>(`${complaintsBase(orgId, employeeId)}/${id}/dismiss`, body);
  return res.data.data.complaint;
}

export async function withdrawComplaint(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<Complaint> {
  const res = await api.post<{
    success: boolean;
    data: { complaint: Complaint };
  }>(`${complaintsBase(orgId, employeeId)}/${id}/withdraw`, {});
  return res.data.data.complaint;
}

// ── Employee Documents ────────────────────────────────────
const docsAllUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/documents`;
const docsBase = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/documents`;

export async function listAllEmployeeDocuments(
  orgId: string,
): Promise<EmployeeDocumentListResponse> {
  const res = await api.get<{
    success: boolean;
    data: EmployeeDocumentListResponse;
  }>(docsAllUrl(orgId));
  return res.data.data;
}

export async function createEmployeeDocument(
  orgId: string,
  employeeId: string,
  body: CreateEmployeeDocumentPayload,
): Promise<EmployeeDocument> {
  const res = await api.post<{
    success: boolean;
    data: { document: EmployeeDocument };
  }>(docsBase(orgId, employeeId), body);
  return res.data.data.document;
}

export async function sendEmployeeDocument(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<EmployeeDocument> {
  const res = await api.post<{
    success: boolean;
    data: { document: EmployeeDocument };
  }>(`${docsBase(orgId, employeeId)}/${id}/send`, {});
  return res.data.data.document;
}

export async function withdrawEmployeeDocument(
  orgId: string,
  employeeId: string,
  id: string,
): Promise<EmployeeDocument> {
  const res = await api.post<{
    success: boolean;
    data: { document: EmployeeDocument };
  }>(`${docsBase(orgId, employeeId)}/${id}/withdraw`, {});
  return res.data.data.document;
}

// ── Acknowledgements ──────────────────────────────────────
const ackAllUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/acknowledgements`;

export interface AcknowledgementFilter {
  employee_id?: string;
  acknowledgeable_type?: string;
  status?: string;
}

export async function listAcknowledgements(
  orgId: string,
  filter?: AcknowledgementFilter,
): Promise<AcknowledgementListResponse> {
  const res = await api.get<{
    success: boolean;
    data: AcknowledgementListResponse;
  }>(ackAllUrl(orgId), {
    params: filter,
  });
  return res.data.data;
}

export async function createAcknowledgement(
  orgId: string,
  body: CreateAcknowledgementPayload,
): Promise<Acknowledgement> {
  const res = await api.post<{
    success: boolean;
    data: { acknowledgement: Acknowledgement };
  }>(ackAllUrl(orgId), body);
  return res.data.data.acknowledgement;
}

export async function respondAcknowledgement(
  orgId: string,
  id: string,
  body: RespondAcknowledgementPayload,
): Promise<Acknowledgement> {
  const res = await api.post<{
    success: boolean;
    data: { acknowledgement: Acknowledgement };
  }>(`${ackAllUrl(orgId)}/${id}/acknowledge`, body);
  return res.data.data.acknowledgement;
}

export async function declineAcknowledgement(
  orgId: string,
  id: string,
  body: DeclineAcknowledgementPayload,
): Promise<Acknowledgement> {
  const res = await api.post<{
    success: boolean;
    data: { acknowledgement: Acknowledgement };
  }>(`${ackAllUrl(orgId)}/${id}/decline`, body);
  return res.data.data.acknowledgement;
}
