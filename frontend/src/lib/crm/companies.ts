// src/lib/crm/companies.ts
import api from "../api";
import type {
  Company,
  CompanyListResponse,
  CreateCompanyPayload,
  UpdateCompanyPayload,
  ContactListResponse,
  EnrichedCompanyData,
} from "@/types/crm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/crm/companies`;

export async function listCompanies(
  orgId: string,
  search?: string,
): Promise<CompanyListResponse> {
  const url = new URL(base(orgId), window.location.origin);
  if (search) url.searchParams.set("search", search);

  const res = await api.get<{ success: boolean; data: CompanyListResponse }>(
    url.pathname + url.search,
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

export async function enrichCompany(
  orgId: string,
  domain: string,
): Promise<EnrichedCompanyData> {
  const res = await api.get<{ success: boolean; data: EnrichedCompanyData }>(
    `${base(orgId)}/enrich`,
    { params: { domain } },
  );
  return res.data.data;
}
