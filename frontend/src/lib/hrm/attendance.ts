// src/lib/hrm/attendance.ts
import api from "../api";
import type {
  AttendanceRecord,
  AttendanceRecordListResponse,
  CreateAttendanceRecordPayload,
  RegularizeAttendancePayload,
  AttendancePeriod,
  AttendancePeriodListResponse,
} from "@/types/hrm";

const recordsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/attendance`;
const periodsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/attendance/periods`;

export interface AttendanceRecordFilter {
  employee_id?: string;
  status?: string;
  year?: number;
  month?: number;
}

export async function listAttendanceRecords(
  orgId: string,
  filter?: AttendanceRecordFilter,
): Promise<AttendanceRecordListResponse> {
  const res = await api.get<{
    success: boolean;
    data: AttendanceRecordListResponse;
  }>(recordsUrl(orgId), {
    params: filter,
  });
  return res.data.data;
}

export async function createAttendanceRecord(
  orgId: string,
  body: CreateAttendanceRecordPayload,
): Promise<AttendanceRecord> {
  const res = await api.post<{
    success: boolean;
    data: { record: AttendanceRecord };
  }>(recordsUrl(orgId), body);
  return res.data.data.record;
}

export async function approveAttendanceRecord(
  orgId: string,
  recordId: string,
): Promise<AttendanceRecord> {
  const res = await api.post<{
    success: boolean;
    data: { record: AttendanceRecord };
  }>(`${recordsUrl(orgId)}/${recordId}/approve`, {});
  return res.data.data.record;
}

export async function rejectAttendanceRecord(
  orgId: string,
  recordId: string,
): Promise<AttendanceRecord> {
  const res = await api.post<{
    success: boolean;
    data: { record: AttendanceRecord };
  }>(`${recordsUrl(orgId)}/${recordId}/reject`, {});
  return res.data.data.record;
}

export async function regularizeAttendanceRecord(
  orgId: string,
  recordId: string,
  body: RegularizeAttendancePayload,
): Promise<AttendanceRecord> {
  const res = await api.post<{
    success: boolean;
    data: { record: AttendanceRecord };
  }>(`${recordsUrl(orgId)}/${recordId}/regularize`, body);
  return res.data.data.record;
}

export async function listAttendancePeriods(
  orgId: string,
): Promise<AttendancePeriodListResponse> {
  const res = await api.get<{
    success: boolean;
    data: AttendancePeriodListResponse;
  }>(periodsUrl(orgId));
  return res.data.data;
}

export async function getOrCreateAttendancePeriod(
  orgId: string,
  year: number,
  month: number,
): Promise<AttendancePeriod> {
  const res = await api.post<{
    success: boolean;
    data: { period: AttendancePeriod };
  }>(periodsUrl(orgId), {
    year,
    month,
  });
  return res.data.data.period;
}

export async function finalizeAttendancePeriod(
  orgId: string,
  year: number,
  month: number,
): Promise<AttendancePeriod> {
  const res = await api.post<{
    success: boolean;
    data: { period: AttendancePeriod };
  }>(`${periodsUrl(orgId)}/${year}/${month}/finalize`, {});
  return res.data.data.period;
}

export async function lockAttendancePeriod(
  orgId: string,
  year: number,
  month: number,
): Promise<AttendancePeriod> {
  const res = await api.post<{
    success: boolean;
    data: { period: AttendancePeriod };
  }>(`${periodsUrl(orgId)}/${year}/${month}/lock`, {});
  return res.data.data.period;
}
