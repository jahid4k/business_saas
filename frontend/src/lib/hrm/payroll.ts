// src/lib/hrm/payroll.ts
import api from "../api";
import type {
  PayslipRun,
  PayslipRunListResponse,
  CreatePayslipRunPayload,
  Payslip,
  PayslipListResponse,
} from "@/types/hrm";

const runsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/payroll/runs`;
const payslipsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/payroll/payslips`;

export async function listPayrollRuns(
  orgId: string,
): Promise<PayslipRunListResponse> {
  const res = await api.get<{ success: boolean; data: PayslipRunListResponse }>(
    runsUrl(orgId),
  );
  return res.data.data;
}

export async function createPayrollRun(
  orgId: string,
  body: CreatePayslipRunPayload,
): Promise<PayslipRun> {
  const res = await api.post<{ success: boolean; data: { run: PayslipRun } }>(
    runsUrl(orgId),
    body,
  );
  return res.data.data.run;
}

export async function computePayrollRun(
  orgId: string,
  runId: string,
): Promise<PayslipRun> {
  const res = await api.post<{ success: boolean; data: { run: PayslipRun } }>(
    `${runsUrl(orgId)}/${runId}/compute`,
    {},
  );
  return res.data.data.run;
}

export async function approvePayrollRun(
  orgId: string,
  runId: string,
): Promise<PayslipRun> {
  const res = await api.post<{ success: boolean; data: { run: PayslipRun } }>(
    `${runsUrl(orgId)}/${runId}/approve`,
    {},
  );
  return res.data.data.run;
}

export async function payPayrollRun(
  orgId: string,
  runId: string,
): Promise<PayslipRun> {
  const res = await api.post<{ success: boolean; data: { run: PayslipRun } }>(
    `${runsUrl(orgId)}/${runId}/pay`,
    {},
  );
  return res.data.data.run;
}

export async function cancelPayrollRun(
  orgId: string,
  runId: string,
): Promise<PayslipRun> {
  const res = await api.post<{ success: boolean; data: { run: PayslipRun } }>(
    `${runsUrl(orgId)}/${runId}/cancel`,
    {},
  );
  return res.data.data.run;
}

export async function listPayslips(
  orgId: string,
  filter?: { run_id?: string; employee_id?: string },
): Promise<PayslipListResponse> {
  const res = await api.get<{ success: boolean; data: PayslipListResponse }>(
    payslipsUrl(orgId),
    {
      params: filter,
    },
  );
  return res.data.data;
}

export async function getPayslip(
  orgId: string,
  payslipId: string,
): Promise<Payslip> {
  const res = await api.get<{ success: boolean; data: { payslip: Payslip } }>(
    `${payslipsUrl(orgId)}/${payslipId}`,
  );
  return res.data.data.payslip;
}
