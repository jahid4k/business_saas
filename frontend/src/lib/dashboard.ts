import api from "./api";

export interface DashboardKPIs {
  active_pipeline_value: number;
  total_headcount: number;
  pending_approvals: number;
}

export interface DashboardActionItem {
  id: string;
  title: string;
  description: string;
  timestamp: string;
  type: string;
  action_url: string;
}

export interface DashboardResponse {
  kpis: DashboardKPIs;
  action_items: DashboardActionItem[];
}

export async function getDashboardMetrics(
  orgId: string,
): Promise<DashboardResponse> {
  const { data } = await api.get(`/orgs/${orgId}/dashboard`);
  return data.data; // The envelope wraps actual data in "data"
}
