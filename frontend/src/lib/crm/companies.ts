// src/lib/crm/companies.ts
import api from "../api";
import type {
  Company,
  CompanyListResponse,
  CreateCompanyPayload,
  UpdateCompanyPayload,
  ContactListResponse,
} from "@/types/crm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/crm/companies`;

export async function listCompanies(
  orgId: string,
): Promise<CompanyListResponse> {
  const res = await api.get<{ success: boolean; data: CompanyListResponse }>(
    base(orgId),
  );
  return res.data.data;
}

export async function getCompany(
  orgId: string,
  companyId: string,
): Promise<Company> {
  const res = await api.get<{ success: boolean; data: { company: Company } }>(
    `${base(orgId)}/${companyId}`,
  );
  return res.data.data.company;
}

export async function createCompany(
  orgId: string,
  body: CreateCompanyPayload,
): Promise<Company> {
  const res = await api.post<{ success: boolean; data: { company: Company } }>(
    base(orgId),
    body,
  );
  return res.data.data.company;
}

export async function updateCompany(
  orgId: string,
  companyId: string,
  body: UpdateCompanyPayload,
): Promise<Company> {
  const res = await api.patch<{ success: boolean; data: { company: Company } }>(
    `${base(orgId)}/${companyId}`,
    body,
  );
  return res.data.data.company;
}

export async function deleteCompany(
  orgId: string,
  companyId: string,
): Promise<void> {
  await api.delete(`${base(orgId)}/${companyId}`);
}

// GET /crm/companies/:id/contacts → data.contacts[]
export async function getCompanyContacts(
  orgId: string,
  companyId: string,
): Promise<ContactListResponse> {
  const res = await api.get<{ success: boolean; data: ContactListResponse }>(
    `${base(orgId)}/${companyId}/contacts`,
  );
  return res.data.data;
}
