// src/app/(dashboard)/[orgId]/hrm/recognition/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Loader2,
  Award as AwardIcon,
  Megaphone,
  CalendarHeart,
  PartyPopper,
  Sparkles,
} from "lucide-react";
import gsap from "gsap";
import type {
  Employee,
  Department,
  Award,
  Announcement,
  CalendarEvent,
  Milestone,
} from "@/types/hrm";
import {
  listAwards,
  createAward,
  submitAward,
  issueAward,
  cancelAward,
  listAnnouncements,
  createAnnouncement,
  publishAnnouncement,
  scheduleAnnouncement,
  archiveAnnouncement,
  listCalendarEvents,
  createCalendarEvent,
  cancelCalendarEvent,
  requestCalendarRsvp,
  listMilestones,
  createMilestone,
  acknowledgeMilestone,
  generateMilestones,
} from "@/lib/hrm/recognition";
import { listEmployees } from "@/lib/hrm/employees";
import { listDepartments } from "@/lib/hrm/departments";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import AwardForm from "@/components/hrm/recognition/AwardForm";
import AnnouncementForm from "@/components/hrm/recognition/AnnouncementForm";
import CalendarEventForm from "@/components/hrm/recognition/CalendarEventForm";
import MilestoneForm from "@/components/hrm/recognition/MilestoneForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";
import ApprovalInstanceView from "@/components/hrm/approvals/ApprovalInstanceView";

type TabKey = "awards" | "announcements" | "calendar" | "milestones";

function fmtDate(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function StatusChip({ status, tone }: { status: string; tone: string }) {
  return (
    <span
      className={`text-xs px-2 py-0.5 rounded-full border font-medium ${tone}`}
    >
      {status.replace("_", " ")}
    </span>
  );
}

// ── Page ────────────────────────────────────────────────────────────────────
export default function RecognitionPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const [tab, setTab] = useState<TabKey>("awards");
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);

  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
      .catch(() => {});
    listDepartments(orgId)
      .then((r) => setDepartments(r.departments))
      .catch(() => {});
  }, [orgId]);

  const empName = (id: string) => {
    const e = employees.find((x) => x.id === id);
    return e ? `${e.first_name} ${e.last_name ?? ""}`.trim() : "—";
  };

  return (
    <div className="p-6 md:p-8 max-w-5xl">
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1
            className="text-2xl font-bold text-(--text-primary) mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Recognition
          </h1>
          <p className="text-sm text-(--text-muted)">
            Awards, announcements, calendar, and milestones
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-(--bg-elevated) border border-(--border) w-fit flex-wrap">
        {(
          ["awards", "announcements", "calendar", "milestones"] as TabKey[]
        ).map((key) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === key
                ? "bg-purple-600 text-white"
                : "text-(--text-secondary) hover:text-(--text-primary)"
            }`}
          >
            {key === "awards"
              ? "Awards"
              : key === "announcements"
                ? "Announcements"
                : key === "calendar"
                  ? "Calendar"
                  : "Milestones"}
          </button>
        ))}
      </div>

      {tab === "awards" && (
        <AwardsView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.awards.manage")}
          canApprove={hasPermission("hrm.awards.approve")}
          canIssue={hasPermission("hrm.awards.issue")}
        />
      )}
      {tab === "announcements" && (
        <AnnouncementsView
          orgId={orgId}
          employees={employees}
          departments={departments}
          canManage={hasPermission("hrm.announcements.manage")}
          canPublish={hasPermission("hrm.announcements.publish")}
        />
      )}
      {tab === "calendar" && (
        <CalendarView
          orgId={orgId}
          employees={employees}
          departments={departments}
          canManage={hasPermission("hrm.calendar.manage")}
        />
      )}
      {tab === "milestones" && (
        <MilestonesView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.milestones.manage")}
          canGenerate={hasPermission("hrm.milestones.generate")}
        />
      )}
    </div>
  );
}

// ── Awards ────────────────────────────────────────────────────────────────
function AwardsView({
  orgId,
  employees,
  empName,
  canManage,
  canApprove,
  canIssue,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
  canApprove: boolean;
  canIssue: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.awards.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAwards(orgId).then((r) => r.awards),
  });
  const items = listQuery.data ?? [];

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".aw-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      let inside = false;
      menuRefs.current.forEach((el) => {
        if (el.contains(e.target as Node)) inside = true;
      });
      if (!inside) setOpenMenuId(null);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const update = (updated: Award) =>
    queryClient.setQueryData<Award[]>(listKey, (old) =>
      (old ?? []).map((a) => (a.id === updated.id ? updated : a)),
    );

  const openCreate = () => {
    openDrawer({
      title: "New award",
      content: (
        <AwardForm
          employees={employees}
          onSave={async (payload) => {
            const created = await createAward(orgId, payload);
            queryClient.setQueryData<Award[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Award created.");
          }}
        />
      ),
    });
  };

  const handleSubmit = async (a: Award) => {
    try {
      update(await submitAward(orgId, a.id));
      toast.success("Award submitted for approval.");
    } catch {
      toast.error("Failed to submit award.");
    }
    setOpenMenuId(null);
  };

  const handleIssue = async (a: Award) => {
    try {
      update(await issueAward(orgId, a.id, { create_announcement: true }));
      toast.success("Award issued — announcement created.");
    } catch {
      toast.error("Failed to issue award.");
    }
    setOpenMenuId(null);
  };

  const handleCancel = async (a: Award) => {
    try {
      update(await cancelAward(orgId, a.id));
      toast.success("Award cancelled.");
    } catch {
      toast.error("Failed to cancel award.");
    }
    setOpenMenuId(null);
  };

  const STATUS_TONE: Record<string, string> = {
    draft: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    pending_approval: "bg-amber-500/10 text-amber-400 border-amber-500/20",
    approved: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    issued: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
    cancelled: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  };

  return (
    <>
      <div className="flex items-center justify-end mb-5">
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New award
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <AwardIcon size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            No awards yet
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {items.map((a) => {
            const menuOpen = openMenuId === a.id;
            const showMenu =
              (canManage && a.status === "draft") ||
              (canApprove && a.status === "draft") ||
              (canIssue && (a.status === "draft" || a.status === "approved")) ||
              !!a.approval_instance_id;

            return (
              <div
                key={a.id}
                className={`aw-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border) hover:border-(--text-muted)/25 transition-all duration-150 ${menuOpen ? "z-30 border-(--text-muted)/30" : "z-10"}`}
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <AwardIcon size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-(--text-primary)">
                    {a.title}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5">
                    {empName(a.employee_id)} · {a.award_type.replace("_", " ")}
                    {a.points > 0 ? ` · ${a.points} pts` : ""}
                    {a.monetary_value
                      ? ` · ${a.monetary_value} ${a.currency}`
                      : ""}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5 line-clamp-1">
                    {a.description}
                  </p>
                  <div className="mt-2">
                    <StatusChip
                      status={a.status}
                      tone={STATUS_TONE[a.status]}
                    />
                  </div>
                </div>

                {showMenu && (
                  <div
                    className="relative shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(a.id, el);
                      else menuRefs.current.delete(a.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : a.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-(--bg-elevated) border border-(--border) shadow-xl z-20">
                        {a.approval_instance_id && (
                          <button
                            onClick={() => {
                              openDrawer({
                                title: "Approval status",
                                content: (
                                  <ApprovalInstanceView
                                    orgId={orgId}
                                    instanceId={a.approval_instance_id!}
                                  />
                                ),
                              });
                              setOpenMenuId(null);
                            }}
                            className="w-full flex items-center px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) hover:text-(--text-primary) text-left"
                          >
                            View approval
                          </button>
                        )}
                        {canApprove && a.status === "draft" && (
                          <button
                            onClick={() => handleSubmit(a)}
                            className="w-full flex items-center px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) hover:text-(--text-primary) text-left"
                          >
                            Submit
                          </button>
                        )}
                        {canIssue &&
                          (a.status === "draft" || a.status === "approved") && (
                            <button
                              onClick={() => handleIssue(a)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-emerald-400 hover:bg-emerald-500/10 text-left"
                            >
                              Issue
                            </button>
                          )}
                        {canManage && a.status === "draft" && (
                          <button
                            onClick={() => handleCancel(a)}
                            className="w-full flex items-center px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                          >
                            Cancel
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}

// ── Announcements ─────────────────────────────────────────────────────────
function AnnouncementsView({
  orgId,
  employees,
  departments,
  canManage,
  canPublish,
}: {
  orgId: string;
  employees: Employee[];
  departments: Department[];
  canManage: boolean;
  canPublish: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.announcements.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAnnouncements(orgId).then((r) => r.announcements),
  });
  const items = listQuery.data ?? [];

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".an-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      let inside = false;
      menuRefs.current.forEach((el) => {
        if (el.contains(e.target as Node)) inside = true;
      });
      if (!inside) setOpenMenuId(null);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const update = (updated: Announcement) =>
    queryClient.setQueryData<Announcement[]>(listKey, (old) =>
      (old ?? []).map((a) => (a.id === updated.id ? updated : a)),
    );

  const openCreate = () => {
    openDrawer({
      title: "New announcement",
      content: (
        <AnnouncementForm
          employees={employees}
          departments={departments}
          onSave={async (payload) => {
            const created = await createAnnouncement(orgId, payload);
            queryClient.setQueryData<Announcement[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Announcement created.");
          }}
        />
      ),
    });
  };

  const handlePublish = async (a: Announcement) => {
    try {
      update(await publishAnnouncement(orgId, a.id));
      toast.success("Announcement published.");
    } catch {
      toast.error("Failed to publish.");
    }
    setOpenMenuId(null);
  };

  const handleSchedule = async (a: Announcement) => {
    try {
      update(await scheduleAnnouncement(orgId, a.id));
      toast.success("Announcement scheduled.");
    } catch {
      toast.error("Failed to schedule.");
    }
    setOpenMenuId(null);
  };

  const handleArchive = async (a: Announcement) => {
    try {
      update(await archiveAnnouncement(orgId, a.id));
      toast.success("Announcement archived.");
    } catch {
      toast.error("Failed to archive.");
    }
    setOpenMenuId(null);
  };

  const STATUS_TONE: Record<string, string> = {
    draft: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    scheduled: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    published: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
    expired: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    archived: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  };

  return (
    <>
      <div className="flex items-center justify-end mb-5">
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New announcement
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <Megaphone size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            No announcements yet
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {items.map((a) => {
            const menuOpen = openMenuId === a.id;
            return (
              <div
                key={a.id}
                className={`an-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border) hover:border-(--text-muted)/25 transition-all duration-150 ${menuOpen ? "z-30 border-(--text-muted)/30" : "z-10"}`}
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <Megaphone size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-(--text-primary)">
                    {a.is_pinned && "📌 "}
                    {a.title}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5">
                    {a.category.replace("_", " ")} · {a.scope_type}
                    {a.requires_acknowledgement ? " · Requires ack" : ""}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5 line-clamp-1">
                    {a.content}
                  </p>
                  <div className="mt-2">
                    <StatusChip
                      status={a.status}
                      tone={STATUS_TONE[a.status]}
                    />
                  </div>
                </div>

                {canPublish &&
                  (a.status === "draft" ||
                    a.status === "scheduled" ||
                    a.status === "published") && (
                    <div
                      className="relative shrink-0"
                      ref={(el) => {
                        if (el) menuRefs.current.set(a.id, el);
                        else menuRefs.current.delete(a.id);
                      }}
                    >
                      <button
                        onClick={() => setOpenMenuId(menuOpen ? null : a.id)}
                        className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-all"
                      >
                        <MoreHorizontal size={15} />
                      </button>
                      {menuOpen && (
                        <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-(--bg-elevated) border border-(--border) shadow-xl z-20">
                          {a.status === "draft" && (
                            <button
                              onClick={() => handlePublish(a)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-emerald-400 hover:bg-emerald-500/10 text-left"
                            >
                              Publish now
                            </button>
                          )}
                          {a.status === "draft" && (
                            <button
                              onClick={() => handleSchedule(a)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) hover:text-(--text-primary) text-left"
                            >
                              Schedule
                            </button>
                          )}
                          {(a.status === "published" ||
                            a.status === "scheduled") && (
                            <button
                              onClick={() => handleArchive(a)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                            >
                              Archive
                            </button>
                          )}
                        </div>
                      )}
                    </div>
                  )}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}

// ── Calendar ──────────────────────────────────────────────────────────────
function CalendarView({
  orgId,
  employees,
  departments,
  canManage,
}: {
  orgId: string;
  employees: Employee[];
  departments: Department[];
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.calendar.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listCalendarEvents(orgId).then((r) => r.events),
  });
  const items = (listQuery.data ?? [])
    .slice()
    .sort((a, b) => a.start_date.localeCompare(b.start_date));

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".cal-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      let inside = false;
      menuRefs.current.forEach((el) => {
        if (el.contains(e.target as Node)) inside = true;
      });
      if (!inside) setOpenMenuId(null);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const openCreate = () => {
    openDrawer({
      title: "New calendar event",
      content: (
        <CalendarEventForm
          employees={employees}
          departments={departments}
          onSave={async (payload) => {
            const created = await createCalendarEvent(orgId, payload);
            queryClient.setQueryData<CalendarEvent[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Event created.");
          }}
        />
      ),
    });
  };

  const handleCancel = async (e: CalendarEvent) => {
    try {
      const updated = await cancelCalendarEvent(orgId, e.id);
      queryClient.setQueryData<CalendarEvent[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Event cancelled.");
    } catch {
      toast.error("Failed to cancel event.");
    }
    setOpenMenuId(null);
  };

  const handleRsvp = async (e: CalendarEvent) => {
    try {
      const updated = await requestCalendarRsvp(orgId, e.id);
      queryClient.setQueryData<CalendarEvent[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("RSVP requests sent.");
    } catch {
      toast.error("Failed to send RSVP requests.");
    }
    setOpenMenuId(null);
  };

  const STATUS_TONE: Record<string, string> = {
    upcoming: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    ongoing: "bg-amber-500/10 text-amber-400 border-amber-500/20",
    completed: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
    cancelled: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  };

  return (
    <>
      <div className="flex items-center justify-end mb-5">
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New event
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <CalendarHeart size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            No calendar events
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {items.map((e) => {
            const menuOpen = openMenuId === e.id;
            return (
              <div
                key={e.id}
                className={`cal-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border) hover:border-(--text-muted)/25 transition-all duration-150 ${menuOpen ? "z-30 border-(--text-muted)/30" : "z-10"}`}
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <CalendarHeart size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-(--text-primary)">
                    {e.title}
                  </p>
                  <p className="text-xs text-(--text-muted) mt-0.5">
                    {fmtDate(e.start_date)}
                    {e.end_date !== e.start_date
                      ? ` – ${fmtDate(e.end_date)}`
                      : ""}
                    {!e.is_all_day && e.start_time ? ` · ${e.start_time}` : ""}
                    {e.location ? ` · ${e.location}` : ""}
                    {e.is_auto_generated ? " · Auto-generated" : ""}
                  </p>
                  <div className="mt-2">
                    <StatusChip
                      status={e.status}
                      tone={STATUS_TONE[e.status]}
                    />
                  </div>
                </div>

                {canManage && e.status === "upcoming" && (
                  <div
                    className="relative shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(e.id, el);
                      else menuRefs.current.delete(e.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : e.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-44 rounded-xl overflow-hidden bg-(--bg-elevated) border border-(--border) shadow-xl z-20">
                        {e.requires_rsvp && (
                          <button
                            onClick={() => handleRsvp(e)}
                            className="w-full flex items-center px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) hover:text-(--text-primary) text-left"
                          >
                            Send RSVP requests
                          </button>
                        )}
                        <button
                          onClick={() => handleCancel(e)}
                          className="w-full flex items-center px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                        >
                          Cancel event
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}

// ── Milestones ────────────────────────────────────────────────────────────
function MilestonesView({
  orgId,
  employees,
  empName,
  canManage,
  canGenerate,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
  canGenerate: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const now = new Date();
  const [genYear, setGenYear] = useState(now.getFullYear());
  const [genMonth, setGenMonth] = useState(now.getMonth() + 1);
  const listRef = useRef<HTMLDivElement>(null);

  const listKey = queryKeys.hrm.milestones.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listMilestones(orgId).then((r) => r.milestones),
  });
  const items = (listQuery.data ?? [])
    .slice()
    .sort((a, b) => b.milestone_date.localeCompare(a.milestone_date));

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".ms-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending]);

  const openCreate = () => {
    openDrawer({
      title: "New milestone",
      content: (
        <MilestoneForm
          employees={employees}
          onSave={async (payload) => {
            const created = await createMilestone(orgId, payload);
            queryClient.setQueryData<Milestone[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Milestone created.");
          }}
        />
      ),
    });
  };

  const handleAcknowledge = async (m: Milestone) => {
    try {
      const updated = await acknowledgeMilestone(orgId, m.id);
      queryClient.setQueryData<Milestone[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Milestone acknowledged.");
    } catch {
      toast.error("Failed to acknowledge.");
    }
  };

  const handleGenerate = async () => {
    try {
      const result = await generateMilestones(orgId, {
        year: genYear,
        month: genMonth,
        include_anniversaries: true,
        include_birthdays: true,
        include_probation: true,
        include_contract_renewals: true,
      });
      queryClient.invalidateQueries({ queryKey: listKey });
      toast.success(
        `Generated ${result.generated} milestones (${result.skipped} already existed).`,
      );
    } catch {
      toast.error("Failed to generate milestones.");
    }
  };

  const MONTHS = [
    "January",
    "February",
    "March",
    "April",
    "May",
    "June",
    "July",
    "August",
    "September",
    "October",
    "November",
    "December",
  ];

  return (
    <>
      <div className="flex items-center gap-3 mb-5 flex-wrap">
        {canGenerate && (
          <>
            <select
              value={genMonth}
              onChange={(e) => setGenMonth(Number(e.target.value))}
              className="px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
            >
              {MONTHS.map((m, i) => (
                <option
                  key={m}
                  value={i + 1}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {m}
                </option>
              ))}
            </select>
            <select
              value={genYear}
              onChange={(e) => setGenYear(Number(e.target.value))}
              className="px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
            >
              {[genYear - 1, genYear, genYear + 1].map((y) => (
                <option
                  key={y}
                  value={y}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {y}
                </option>
              ))}
            </select>
            <button
              onClick={handleGenerate}
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-purple-400 border border-purple-500/30 hover:bg-purple-500/10 transition-colors"
            >
              <Sparkles size={15} />
              Generate for this month
            </button>
          </>
        )}
        <div className="flex-1" />
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New milestone
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-(--text-muted) gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <PartyPopper size={20} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary)">
            No milestones yet
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {items.map((m) => (
            <div
              key={m.id}
              className="ms-row flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-(--bg-surface) border border-(--border)"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <PartyPopper size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium leading-snug text-(--text-primary)">
                  {m.title}
                </p>
                <p className="text-xs text-(--text-muted) mt-0.5">
                  {empName(m.employee_id)} ·{" "}
                  {m.milestone_type.replace("_", " ")} ·{" "}
                  {fmtDate(m.milestone_date)}
                  {m.years_count ? ` · ${m.years_count} yrs` : ""}
                  {m.is_auto_generated ? " · Auto-generated" : ""}
                </p>
                {m.is_acknowledged && (
                  <span className="inline-block mt-2 text-xs px-2 py-0.5 rounded-full border font-medium bg-emerald-500/10 text-emerald-400 border-emerald-500/20">
                    Acknowledged
                  </span>
                )}
              </div>
              {!m.is_acknowledged && canManage && (
                <button
                  onClick={() => handleAcknowledge(m)}
                  className="px-3 py-1.5 rounded-lg text-xs font-medium text-emerald-400 border border-emerald-500/20 hover:bg-emerald-500/10 shrink-0"
                >
                  Acknowledge
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </>
  );
}
