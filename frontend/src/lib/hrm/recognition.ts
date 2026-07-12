// src/lib/hrm/recognition.ts
import api from "../api";
import type {
  Award,
  AwardListResponse,
  CreateAwardPayload,
  IssueAwardPayload,
  Announcement,
  AnnouncementListResponse,
  CreateAnnouncementPayload,
  CalendarEvent,
  CalendarEventListResponse,
  CreateCalendarEventPayload,
  Milestone,
  MilestoneListResponse,
  CreateMilestonePayload,
  GenerateMilestonesPayload,
  GenerateMilestonesResult,
} from "@/types/hrm";

// ── Awards ────────────────────────────────────────────────
const awardsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/awards`;

export async function listAwards(
  orgId: string,
  filter?: { employee_id?: string; status?: string },
): Promise<AwardListResponse> {
  const res = await api.get<{ success: boolean; data: AwardListResponse }>(
    awardsUrl(orgId),
    { params: filter },
  );
  return res.data.data;
}

export async function createAward(
  orgId: string,
  body: CreateAwardPayload,
): Promise<Award> {
  const res = await api.post<{ success: boolean; data: { award: Award } }>(
    awardsUrl(orgId),
    body,
  );
  return res.data.data.award;
}

export async function submitAward(orgId: string, id: string): Promise<Award> {
  const res = await api.post<{ success: boolean; data: { award: Award } }>(
    `${awardsUrl(orgId)}/${id}/submit`,
    {},
  );
  return res.data.data.award;
}

export async function issueAward(
  orgId: string,
  id: string,
  body?: IssueAwardPayload,
): Promise<Award> {
  const res = await api.post<{ success: boolean; data: { award: Award } }>(
    `${awardsUrl(orgId)}/${id}/issue`,
    body ?? {},
  );
  return res.data.data.award;
}

export async function cancelAward(orgId: string, id: string): Promise<Award> {
  const res = await api.post<{ success: boolean; data: { award: Award } }>(
    `${awardsUrl(orgId)}/${id}/cancel`,
    {},
  );
  return res.data.data.award;
}

// ── Announcements ─────────────────────────────────────────
const announcementsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/announcements`;

export async function listAnnouncements(
  orgId: string,
  filter?: { category?: string; status?: string },
): Promise<AnnouncementListResponse> {
  const res = await api.get<{
    success: boolean;
    data: AnnouncementListResponse;
  }>(announcementsUrl(orgId), {
    params: filter,
  });
  return res.data.data;
}

export async function createAnnouncement(
  orgId: string,
  body: CreateAnnouncementPayload,
): Promise<Announcement> {
  const res = await api.post<{
    success: boolean;
    data: { announcement: Announcement };
  }>(announcementsUrl(orgId), body);
  return res.data.data.announcement;
}

export async function publishAnnouncement(
  orgId: string,
  id: string,
): Promise<Announcement> {
  const res = await api.post<{
    success: boolean;
    data: { announcement: Announcement };
  }>(`${announcementsUrl(orgId)}/${id}/publish`, {});
  return res.data.data.announcement;
}

export async function scheduleAnnouncement(
  orgId: string,
  id: string,
): Promise<Announcement> {
  const res = await api.post<{
    success: boolean;
    data: { announcement: Announcement };
  }>(`${announcementsUrl(orgId)}/${id}/schedule`, {});
  return res.data.data.announcement;
}

export async function archiveAnnouncement(
  orgId: string,
  id: string,
): Promise<Announcement> {
  const res = await api.post<{
    success: boolean;
    data: { announcement: Announcement };
  }>(`${announcementsUrl(orgId)}/${id}/archive`, {});
  return res.data.data.announcement;
}

// ── Calendar ──────────────────────────────────────────────
const calendarUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/calendar`;

export async function listCalendarEvents(
  orgId: string,
  filter?: {
    event_type?: string;
    status?: string;
    from_date?: string;
    to_date?: string;
  },
): Promise<CalendarEventListResponse> {
  const res = await api.get<{
    success: boolean;
    data: CalendarEventListResponse;
  }>(calendarUrl(orgId), {
    params: filter,
  });
  return res.data.data;
}

export async function createCalendarEvent(
  orgId: string,
  body: CreateCalendarEventPayload,
): Promise<CalendarEvent> {
  const res = await api.post<{
    success: boolean;
    data: { event: CalendarEvent };
  }>(calendarUrl(orgId), body);
  return res.data.data.event;
}

export async function cancelCalendarEvent(
  orgId: string,
  id: string,
): Promise<CalendarEvent> {
  const res = await api.post<{
    success: boolean;
    data: { event: CalendarEvent };
  }>(`${calendarUrl(orgId)}/${id}/cancel`, {});
  return res.data.data.event;
}

export async function requestCalendarRsvp(
  orgId: string,
  id: string,
): Promise<CalendarEvent> {
  const res = await api.post<{
    success: boolean;
    data: { event: CalendarEvent };
  }>(`${calendarUrl(orgId)}/${id}/rsvp`, {});
  return res.data.data.event;
}

// ── Milestones ────────────────────────────────────────────
const milestonesUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/milestones`;

export async function listMilestones(
  orgId: string,
  filter?: {
    employee_id?: string;
    milestone_type?: string;
    upcoming?: boolean;
  },
): Promise<MilestoneListResponse> {
  const res = await api.get<{ success: boolean; data: MilestoneListResponse }>(
    milestonesUrl(orgId),
    {
      params: filter,
    },
  );
  return res.data.data;
}

export async function createMilestone(
  orgId: string,
  body: CreateMilestonePayload,
): Promise<Milestone> {
  const res = await api.post<{
    success: boolean;
    data: { milestone: Milestone };
  }>(milestonesUrl(orgId), body);
  return res.data.data.milestone;
}

export async function acknowledgeMilestone(
  orgId: string,
  id: string,
): Promise<Milestone> {
  const res = await api.post<{
    success: boolean;
    data: { milestone: Milestone };
  }>(`${milestonesUrl(orgId)}/${id}/acknowledge`, {});
  return res.data.data.milestone;
}

export async function generateMilestones(
  orgId: string,
  body: GenerateMilestonesPayload,
): Promise<GenerateMilestonesResult> {
  const res = await api.post<{
    success: boolean;
    data: GenerateMilestonesResult;
  }>(`${milestonesUrl(orgId)}/generate`, body);
  return res.data.data;
}
