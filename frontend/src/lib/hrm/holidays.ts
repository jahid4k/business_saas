// src/lib/hrm/holidays.ts
import api from "../api";
import type {
  HolidayCalendar,
  CalendarListResponse,
  CreateCalendarPayload,
  Holiday,
  HolidayListResponse,
  CreateHolidayPayload,
  CalendarAssignment,
  AssignCalendarPayload,
} from "@/types/hrm";

const calendarsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/holiday-calendars`;

export async function listHolidayCalendars(
  orgId: string,
  opts?: { activeOnly?: boolean },
): Promise<CalendarListResponse> {
  const res = await api.get<{ success: boolean; data: CalendarListResponse }>(
    calendarsUrl(orgId),
    {
      params: opts?.activeOnly ? { active: "true" } : undefined,
    },
  );
  return res.data.data;
}

export async function getHolidayCalendar(
  orgId: string,
  calId: string,
): Promise<HolidayCalendar> {
  const res = await api.get<{
    success: boolean;
    data: { calendar: HolidayCalendar };
  }>(`${calendarsUrl(orgId)}/${calId}`);
  return res.data.data.calendar;
}

export async function createHolidayCalendar(
  orgId: string,
  body: CreateCalendarPayload,
): Promise<HolidayCalendar> {
  const res = await api.post<{
    success: boolean;
    data: { calendar: HolidayCalendar };
  }>(calendarsUrl(orgId), body);
  return res.data.data.calendar;
}

export async function deleteHolidayCalendar(
  orgId: string,
  calId: string,
): Promise<void> {
  await api.delete(`${calendarsUrl(orgId)}/${calId}`);
}

export async function listHolidays(
  orgId: string,
  calId: string,
): Promise<HolidayListResponse> {
  const res = await api.get<{ success: boolean; data: HolidayListResponse }>(
    `${calendarsUrl(orgId)}/${calId}/holidays`,
  );
  return res.data.data;
}

export async function createHoliday(
  orgId: string,
  calId: string,
  body: CreateHolidayPayload,
): Promise<Holiday> {
  const res = await api.post<{ success: boolean; data: { holiday: Holiday } }>(
    `${calendarsUrl(orgId)}/${calId}/holidays`,
    body,
  );
  return res.data.data.holiday;
}

export async function deleteHoliday(
  orgId: string,
  calId: string,
  holidayId: string,
): Promise<void> {
  await api.delete(`${calendarsUrl(orgId)}/${calId}/holidays/${holidayId}`);
}

export async function assignHolidayCalendar(
  orgId: string,
  body: AssignCalendarPayload,
): Promise<CalendarAssignment> {
  const res = await api.post<{
    success: boolean;
    data: { assignment: CalendarAssignment };
  }>(`${calendarsUrl(orgId)}/assignments`, body);
  return res.data.data.assignment;
}
