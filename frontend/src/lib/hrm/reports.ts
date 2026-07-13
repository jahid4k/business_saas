// src/lib/hrm/reports.ts
import api from "../api";
import type {
  HRMSummary,
  HeadcountByDepartment,
  LeaveSummaryByType,
} from "@/types/hrm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/hrm/reports`;

export async function getHRMOverview(orgId: string): Promise<HRMSummary> {
  const res = await api.get<{
    success: boolean;
    data: { summary: HRMSummary };
  }>(`${base(orgId)}/overview`);
  return res.data.data.summary;
}

export async function getHeadcountByDepartment(
  orgId: string,
): Promise<HeadcountByDepartment[]> {
  const res = await api.get<{
    success: boolean;
    data: { headcount_by_department: HeadcountByDepartment[] };
  }>(`${base(orgId)}/headcount`);
  return res.data.data.headcount_by_department;
}

export async function getLeaveSummaryByType(
  orgId: string,
): Promise<LeaveSummaryByType[]> {
  const res = await api.get<{
    success: boolean;
    data: { leave_summary: LeaveSummaryByType[] };
  }>(`${base(orgId)}/leave-summary`);
  return res.data.data.leave_summary;
}
