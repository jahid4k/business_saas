// src/lib/crm/contacts.ts
import api from "../api";
import type {
  Contact,
  ContactListResponse,
  CreateContactPayload,
  UpdateContactPayload,
} from "@/types/crm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/crm/contacts`;

export async function listContacts(
  orgId: string,
): Promise<ContactListResponse> {
  const res = await api.get<{ success: boolean; data: ContactListResponse }>(
    base(orgId),
  );
  return res.data.data;
}

export async function getContact(
  orgId: string,
  contactId: string,
): Promise<Contact> {
  const res = await api.get<{ success: boolean; data: { contact: Contact } }>(
    `${base(orgId)}/${contactId}`,
  );
  return res.data.data.contact;
}

export async function createContact(
  orgId: string,
  body: CreateContactPayload,
): Promise<Contact> {
  const res = await api.post<{ success: boolean; data: { contact: Contact } }>(
    base(orgId),
    body,
  );
  return res.data.data.contact;
}

export async function updateContact(
  orgId: string,
  contactId: string,
  body: UpdateContactPayload,
): Promise<Contact> {
  const res = await api.patch<{ success: boolean; data: { contact: Contact } }>(
    `${base(orgId)}/${contactId}`,
    body,
  );
  return res.data.data.contact;
}

export async function deleteContact(
  orgId: string,
  contactId: string,
): Promise<void> {
  await api.delete(`${base(orgId)}/${contactId}`);
}
