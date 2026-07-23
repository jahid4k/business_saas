import { api } from './api';

export type LeadStatus = "new" | "contacted" | "qualified" | "converted" | "unqualified";

export interface Lead {
  id: string;
  firstName: string;
  lastName: string;
  companyName?: string;
  email?: string;
  phone?: string;
  status: LeadStatus;
  source?: string;
  estimatedValue?: number;
  assignedTo?: string;
  createdAt: string;
  updatedAt: string;
}

export interface LeadListResponse {
  leads: Lead[];
  total: number;
}

export const crmApi = {
  listLeads: async (orgId: string): Promise<LeadListResponse> => {
    const response = await api.get(`/organizations/${orgId}/crm/leads`);
    return response.data.data;
  },
};
