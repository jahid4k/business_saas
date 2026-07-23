import { api } from './api';

export interface KPIs {
  active_pipeline_value: number;
  total_headcount: number;
  pending_approvals: number;
}

export interface ActionItem {
  id: string;
  title: string;
  description: string;
  timestamp: string;
  type: string;
  action_url: string;
}

export interface DashboardResponse {
  kpis: KPIs;
  action_items: ActionItem[];
}

export const dashboardApi = {
  getMetrics: async (orgId: string): Promise<DashboardResponse> => {
    const response = await api.get(`/orgs/${orgId}/dashboard`);
    return response.data.data;
  },
};
