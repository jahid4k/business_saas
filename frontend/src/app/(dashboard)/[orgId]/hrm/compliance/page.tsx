// src/app/(dashboard)/[orgId]/hrm/compliance/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  MoreHorizontal,
  Loader2,
  AlertTriangle,
  FileText,
  ClipboardCheck,
} from "lucide-react";
import gsap from "gsap";
import type {
  Employee,
  Complaint,
  EmployeeDocument,
  Acknowledgement,
} from "@/types/hrm";
import {
  listAllComplaints,
  createComplaint,
  startReviewComplaint,
  assignComplaint,
  resolveComplaint,
  dismissComplaint,
  withdrawComplaint,
  listAllEmployeeDocuments,
  createEmployeeDocument,
  sendEmployeeDocument,
  withdrawEmployeeDocument,
  listAcknowledgements,
  createAcknowledgement,
  respondAcknowledgement,
  declineAcknowledgement,
} from "@/lib/hrm/compliance";
import { listEmployees } from "@/lib/hrm/employees";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import ComplaintForm from "@/components/hrm/compliance/ComplaintForm";
import EmployeeDocumentForm from "@/components/hrm/compliance/EmployeeDocumentForm";
import AcknowledgementForm from "@/components/hrm/compliance/AcknowledgementForm";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";

type TabKey = "complaints" | "documents" | "acknowledgements";

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
      {status}
    </span>
  );
}

// ── Page ────────────────────────────────────────────────────────────────────
export default function CompliancePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const [tab, setTab] = useState<TabKey>("complaints");
  const [employees, setEmployees] = useState<Employee[]>([]);

  useEffect(() => {
    listEmployees(orgId, { limit: 200 })
      .then((r) => setEmployees(r.employees))
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
            className="text-2xl font-bold text-[var(--text-primary)] mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Compliance
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Complaints, documents, and acknowledgements
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1 mb-6 p-1 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] w-fit flex-wrap">
        {(["complaints", "documents", "acknowledgements"] as TabKey[]).map(
          (key) => (
            <button
              key={key}
              onClick={() => setTab(key)}
              className={`px-3.5 py-1.5 rounded-md text-sm font-medium transition-colors ${
                tab === key
                  ? "bg-purple-600 text-white"
                  : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              }`}
            >
              {key === "complaints"
                ? "Complaints"
                : key === "documents"
                  ? "Documents"
                  : "Acknowledgements"}
            </button>
          ),
        )}
      </div>

      {tab === "complaints" && (
        <ComplaintsView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.complaints.manage")}
          canProcess={hasPermission("hrm.complaints.process")}
        />
      )}
      {tab === "documents" && (
        <DocumentsView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.documents.manage")}
        />
      )}
      {tab === "acknowledgements" && (
        <AcknowledgementsView
          orgId={orgId}
          employees={employees}
          empName={empName}
          canManage={hasPermission("hrm.acknowledgements.manage")}
          canRespond={hasPermission("hrm.acknowledgements.respond")}
        />
      )}
    </div>
  );
}

// ── Complaints ────────────────────────────────────────────────────────────
function ComplaintsView({
  orgId,
  employees,
  empName,
  canManage,
  canProcess,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
  canProcess: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("all");
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [assignId, setAssignId] = useState<string | null>(null);
  const [investigatorPick, setInvestigatorPick] = useState("");
  const [resolveId, setResolveId] = useState<string | null>(null);
  const [resolveText, setResolveText] = useState("");
  const [dismissId, setDismissId] = useState<string | null>(null);
  const [dismissText, setDismissText] = useState("");
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.complaints.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAllComplaints(orgId).then((r) => r.complaints),
  });
  const items = listQuery.data ?? [];
  const filtered =
    statusFilter === "all"
      ? items
      : items.filter((c) => c.status === statusFilter);

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".cp-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, statusFilter]);

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
      title: "New complaint",
      content: (
        <ComplaintForm
          employees={employees}
          onSave={async (employeeId, payload) => {
            const created = await createComplaint(orgId, employeeId, payload);
            queryClient.setQueryData<Complaint[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Complaint submitted.");
          }}
        />
      ),
    });
  };

  const update = (updated: Complaint) =>
    queryClient.setQueryData<Complaint[]>(listKey, (old) =>
      (old ?? []).map((c) => (c.id === updated.id ? updated : c)),
    );

  const handleStartReview = async (c: Complaint) => {
    try {
      update(await startReviewComplaint(orgId, c.employee_id, c.id));
      toast.success("Review started.");
    } catch {
      toast.error("Failed to start review.");
    }
    setOpenMenuId(null);
  };

  const handleAssign = async () => {
    if (!assignId || !investigatorPick) return;
    const c = items.find((x) => x.id === assignId);
    if (!c) return;
    try {
      update(
        await assignComplaint(orgId, c.employee_id, c.id, {
          investigator_id: investigatorPick,
        }),
      );
      toast.success("Investigator assigned.");
    } catch {
      toast.error("Failed to assign investigator.");
    }
    setAssignId(null);
    setInvestigatorPick("");
  };

  const handleResolve = async () => {
    if (!resolveId || !resolveText.trim()) return;
    const c = items.find((x) => x.id === resolveId);
    if (!c) return;
    try {
      update(
        await resolveComplaint(orgId, c.employee_id, c.id, {
          resolution: resolveText.trim(),
        }),
      );
      toast.success("Complaint resolved.");
    } catch {
      toast.error("Failed to resolve complaint.");
    }
    setResolveId(null);
    setResolveText("");
  };

  const handleDismiss = async () => {
    if (!dismissId || !dismissText.trim()) return;
    const c = items.find((x) => x.id === dismissId);
    if (!c) return;
    try {
      update(
        await dismissComplaint(orgId, c.employee_id, c.id, {
          resolution: dismissText.trim(),
        }),
      );
      toast.success("Complaint dismissed.");
    } catch {
      toast.error("Failed to dismiss complaint.");
    }
    setDismissId(null);
    setDismissText("");
  };

  const handleWithdraw = async (c: Complaint) => {
    try {
      update(await withdrawComplaint(orgId, c.employee_id, c.id));
      toast.success("Complaint withdrawn.");
    } catch {
      toast.error("Failed to withdraw complaint.");
    }
    setOpenMenuId(null);
  };

  const STATUS_TONE: Record<string, string> = {
    submitted: "bg-amber-500/10 text-amber-400 border-amber-500/20",
    under_review: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    investigating: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    resolved: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
    dismissed: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    withdrawn: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  };
  const STATUS_TABS = [
    "all",
    "submitted",
    "under_review",
    "investigating",
    "resolved",
    "dismissed",
  ];

  return (
    <>
      <div className="flex items-start justify-between mb-5 flex-wrap gap-3">
        <div className="flex items-center gap-0.5 border-b border-[var(--border)] flex-wrap">
          {STATUS_TABS.map((key) => {
            const count =
              key === "all"
                ? items.length
                : items.filter((c) => c.status === key).length;
            const active = statusFilter === key;
            return (
              <button
                key={key}
                onClick={() => setStatusFilter(key)}
                className={`px-3 py-2 text-sm font-medium -mb-px border-b-2 transition-colors ${
                  active
                    ? "text-purple-400 border-purple-500"
                    : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
                }`}
              >
                {key === "all" ? "All" : key.replace("_", " ")}
                {count > 0 && <span className="ml-1.5 text-xs">{count}</span>}
              </button>
            );
          })}
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New complaint
          </button>
        )}
      </div>

      {/* Inline mini-forms for assign/resolve/dismiss */}
      {assignId && (
        <div className="mb-4 p-4 rounded-xl bg-[var(--bg-surface)] border border-purple-500/30 flex items-center gap-3">
          <select
            value={investigatorPick}
            onChange={(e) => setInvestigatorPick(e.target.value)}
            className="flex-1 px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)]"
          >
            <option value="">Select investigator</option>
            {employees.map((e) => (
              <option
                key={e.id}
                value={e.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {e.first_name} {e.last_name ?? ""}
              </option>
            ))}
          </select>
          <button
            onClick={handleAssign}
            className="px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500"
          >
            Assign
          </button>
          <button
            onClick={() => setAssignId(null)}
            className="px-3.5 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
          >
            Cancel
          </button>
        </div>
      )}
      {resolveId && (
        <div className="mb-4 p-4 rounded-xl bg-[var(--bg-surface)] border border-purple-500/30 space-y-2">
          <textarea
            value={resolveText}
            onChange={(e) => setResolveText(e.target.value)}
            rows={2}
            placeholder="Resolution notes"
            className="w-full px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)]"
          />
          <div className="flex gap-2">
            <button
              onClick={handleResolve}
              className="px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500"
            >
              Resolve
            </button>
            <button
              onClick={() => setResolveId(null)}
              className="px-3.5 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
      {dismissId && (
        <div className="mb-4 p-4 rounded-xl bg-[var(--bg-surface)] border border-purple-500/30 space-y-2">
          <textarea
            value={dismissText}
            onChange={(e) => setDismissText(e.target.value)}
            rows={2}
            placeholder="Reason for dismissal"
            className="w-full px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)]"
          />
          <div className="flex gap-2">
            <button
              onClick={handleDismiss}
              className="px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-red-500 hover:bg-red-400"
            >
              Dismiss
            </button>
            <button
              onClick={() => setDismissId(null)}
              className="px-3.5 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <AlertTriangle size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No complaints
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((c) => {
            const menuOpen = openMenuId === c.id;
            return (
              <div
                key={c.id}
                className={`cp-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150 ${menuOpen ? "z-30 border-[var(--text-muted)]/30" : "z-10"}`}
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <AlertTriangle size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {c.title}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {c.complaint_type.replace("_", " ")} ·{" "}
                    {c.is_anonymous
                      ? "Filed anonymously"
                      : `By ${empName(c.employee_id)}`}
                    {c.incident_date ? ` · ${fmtDate(c.incident_date)}` : ""}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-1">
                    {c.description}
                  </p>
                  <div className="mt-2">
                    <StatusChip
                      status={c.status.replace("_", " ")}
                      tone={STATUS_TONE[c.status]}
                    />
                  </div>
                </div>

                {(canProcess || canManage) &&
                  c.status !== "resolved" &&
                  c.status !== "dismissed" &&
                  c.status !== "withdrawn" && (
                    <div
                      className="relative flex-shrink-0"
                      ref={(el) => {
                        if (el) menuRefs.current.set(c.id, el);
                        else menuRefs.current.delete(c.id);
                      }}
                    >
                      <button
                        onClick={() => setOpenMenuId(menuOpen ? null : c.id)}
                        className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                      >
                        <MoreHorizontal size={15} />
                      </button>
                      {menuOpen && (
                        <div className="absolute right-0 top-full mt-1.5 w-44 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                          {canProcess && c.status === "submitted" && (
                            <button
                              onClick={() => handleStartReview(c)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] text-left"
                            >
                              Start review
                            </button>
                          )}
                          {canProcess &&
                            (c.status === "under_review" ||
                              c.status === "investigating") && (
                              <button
                                onClick={() => {
                                  setAssignId(c.id);
                                  setOpenMenuId(null);
                                }}
                                className="w-full flex items-center px-3.5 py-2.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] text-left"
                              >
                                Assign investigator
                              </button>
                            )}
                          {canProcess &&
                            (c.status === "under_review" ||
                              c.status === "investigating") && (
                              <button
                                onClick={() => {
                                  setResolveId(c.id);
                                  setOpenMenuId(null);
                                }}
                                className="w-full flex items-center px-3.5 py-2.5 text-sm text-emerald-400 hover:bg-emerald-500/10 text-left"
                              >
                                Resolve
                              </button>
                            )}
                          {canProcess &&
                            (c.status === "under_review" ||
                              c.status === "investigating" ||
                              c.status === "submitted") && (
                              <button
                                onClick={() => {
                                  setDismissId(c.id);
                                  setOpenMenuId(null);
                                }}
                                className="w-full flex items-center px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                              >
                                Dismiss
                              </button>
                            )}
                          {canManage && c.status === "submitted" && (
                            <button
                              onClick={() => handleWithdraw(c)}
                              className="w-full flex items-center px-3.5 py-2.5 text-sm text-amber-400 hover:bg-amber-500/10 text-left"
                            >
                              Withdraw
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

// ── Employee Documents ────────────────────────────────────────────────────
function DocumentsView({
  orgId,
  employees,
  empName,
  canManage,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const listKey = queryKeys.hrm.documents.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAllEmployeeDocuments(orgId).then((r) => r.documents),
  });
  const items = listQuery.data ?? [];

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".doc-row");
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
      title: "New employee document",
      content: (
        <EmployeeDocumentForm
          employees={employees}
          onSave={async (employeeId, payload) => {
            const created = await createEmployeeDocument(
              orgId,
              employeeId,
              payload,
            );
            queryClient.setQueryData<EmployeeDocument[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Document created.");
          }}
        />
      ),
    });
  };

  const handleSend = async (d: EmployeeDocument) => {
    try {
      const updated = await sendEmployeeDocument(orgId, d.employee_id, d.id);
      queryClient.setQueryData<EmployeeDocument[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Document sent to employee.");
    } catch {
      toast.error("Failed to send document.");
    }
    setOpenMenuId(null);
  };

  const handleWithdraw = async (d: EmployeeDocument) => {
    try {
      const updated = await withdrawEmployeeDocument(
        orgId,
        d.employee_id,
        d.id,
      );
      queryClient.setQueryData<EmployeeDocument[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Document withdrawn.");
    } catch {
      toast.error("Failed to withdraw document.");
    }
    setOpenMenuId(null);
  };

  const STATUS_TONE: Record<string, string> = {
    draft: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    sent: "bg-amber-500/10 text-amber-400 border-amber-500/20",
    acknowledged: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
    declined: "bg-red-500/10 text-red-400 border-red-500/20",
    expired: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    withdrawn: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    superseded: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
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
            New document
          </button>
        )}
      </div>

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <FileText size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No documents
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {items.map((d) => {
            const menuOpen = openMenuId === d.id;
            return (
              <div
                key={d.id}
                className={`doc-row group relative flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)] hover:border-[var(--text-muted)]/25 transition-all duration-150 ${menuOpen ? "z-30 border-[var(--text-muted)]/30" : "z-10"}`}
              >
                <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                  <FileText size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                    {d.title}
                  </p>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">
                    {empName(d.employee_id)} · {d.document_type}
                    {d.expiry_date
                      ? ` · Expires ${fmtDate(d.expiry_date)}`
                      : ""}
                  </p>
                  <div className="mt-2">
                    <StatusChip
                      status={d.status}
                      tone={STATUS_TONE[d.status]}
                    />
                  </div>
                </div>

                {canManage && (d.status === "draft" || d.status === "sent") && (
                  <div
                    className="relative flex-shrink-0"
                    ref={(el) => {
                      if (el) menuRefs.current.set(d.id, el);
                      else menuRefs.current.delete(d.id);
                    }}
                  >
                    <button
                      onClick={() => setOpenMenuId(menuOpen ? null : d.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-all"
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menuOpen && (
                      <div className="absolute right-0 top-full mt-1.5 w-40 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-xl z-20">
                        {d.status === "draft" && (
                          <button
                            onClick={() => handleSend(d)}
                            className="w-full flex items-center px-3.5 py-2.5 text-sm text-emerald-400 hover:bg-emerald-500/10 text-left"
                          >
                            Send to employee
                          </button>
                        )}
                        <button
                          onClick={() => handleWithdraw(d)}
                          className="w-full flex items-center px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 text-left"
                        >
                          Withdraw
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

// ── Acknowledgements ──────────────────────────────────────────────────────
function AcknowledgementsView({
  orgId,
  employees,
  empName,
  canManage,
  canRespond,
}: {
  orgId: string;
  employees: Employee[];
  empName: (id: string) => string;
  canManage: boolean;
  canRespond: boolean;
}) {
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("all");
  const [declineId, setDeclineId] = useState<string | null>(null);
  const [declineReason, setDeclineReason] = useState("");
  const listRef = useRef<HTMLDivElement>(null);

  const listKey = queryKeys.hrm.acknowledgements.list(orgId);
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listAcknowledgements(orgId).then((r) => r.acknowledgements),
  });
  const items = listQuery.data ?? [];
  const filtered =
    statusFilter === "all"
      ? items
      : items.filter((a) => a.status === statusFilter);

  useEffect(() => {
    if (listQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".ack-row");
    if (rows.length > 0) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.25, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [listQuery.isPending, statusFilter]);

  const openCreate = () => {
    openDrawer({
      title: "New acknowledgement request",
      content: (
        <AcknowledgementForm
          employees={employees}
          onSave={async (payload) => {
            const created = await createAcknowledgement(orgId, payload);
            queryClient.setQueryData<Acknowledgement[]>(listKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Acknowledgement request sent.");
          }}
        />
      ),
    });
  };

  const handleAcknowledge = async (a: Acknowledgement) => {
    try {
      const updated = await respondAcknowledgement(orgId, a.id, {});
      queryClient.setQueryData<Acknowledgement[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Acknowledged.");
    } catch {
      toast.error("Failed to acknowledge.");
    }
  };

  const handleDecline = async () => {
    if (!declineId || !declineReason.trim()) return;
    try {
      const updated = await declineAcknowledgement(orgId, declineId, {
        reason: declineReason.trim(),
      });
      queryClient.setQueryData<Acknowledgement[]>(listKey, (old) =>
        (old ?? []).map((x) => (x.id === updated.id ? updated : x)),
      );
      toast.success("Declined.");
    } catch {
      toast.error("Failed to decline.");
    }
    setDeclineId(null);
    setDeclineReason("");
  };

  const STATUS_TONE: Record<string, string> = {
    pending: "bg-amber-500/10 text-amber-400 border-amber-500/20",
    acknowledged: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
    declined: "bg-red-500/10 text-red-400 border-red-500/20",
    expired: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  };
  const STATUS_TABS = ["all", "pending", "acknowledged", "declined", "expired"];

  return (
    <>
      <div className="flex items-start justify-between mb-5 flex-wrap gap-3">
        <div className="flex items-center gap-0.5 border-b border-[var(--border)]">
          {STATUS_TABS.map((key) => {
            const count =
              key === "all"
                ? items.length
                : items.filter((a) => a.status === key).length;
            const active = statusFilter === key;
            return (
              <button
                key={key}
                onClick={() => setStatusFilter(key)}
                className={`px-3 py-2 text-sm font-medium -mb-px border-b-2 transition-colors ${
                  active
                    ? "text-purple-400 border-purple-500"
                    : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
                }`}
              >
                {key === "all" ? "All" : key}
                {count > 0 && <span className="ml-1.5 text-xs">{count}</span>}
              </button>
            );
          })}
        </div>
        {canManage && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New request
          </button>
        )}
      </div>

      {declineId && (
        <div className="mb-4 p-4 rounded-xl bg-[var(--bg-surface)] border border-purple-500/30 space-y-2">
          <textarea
            value={declineReason}
            onChange={(e) => setDeclineReason(e.target.value)}
            rows={2}
            placeholder="Reason for declining"
            className="w-full px-3 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)]"
          />
          <div className="flex gap-2">
            <button
              onClick={handleDecline}
              className="px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-red-500 hover:bg-red-400"
            >
              Decline
            </button>
            <button
              onClick={() => setDeclineId(null)}
              className="px-3.5 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <ClipboardCheck size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)]">
            No acknowledgement requests
          </p>
        </div>
      ) : (
        <div ref={listRef} className="space-y-1.5">
          {filtered.map((a) => (
            <div
              key={a.id}
              className="ack-row flex items-start gap-3.5 px-4 py-3.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border)]"
            >
              <div className="w-8 h-8 rounded-lg shrink-0 flex items-center justify-center bg-purple-500/10 text-purple-400">
                <ClipboardCheck size={15} />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium leading-snug text-[var(--text-primary)]">
                  {a.entity_title}
                </p>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {empName(a.employee_id)} ·{" "}
                  {a.acknowledgeable_type.replace("_", " ")}
                  {a.signature_required ? " · Signature required" : ""}
                </p>
                <div className="mt-2">
                  <StatusChip status={a.status} tone={STATUS_TONE[a.status]} />
                </div>
              </div>
              {a.status === "pending" && canRespond && (
                <div className="flex items-center gap-2 flex-shrink-0 pt-0.5">
                  <button
                    onClick={() => handleAcknowledge(a)}
                    className="px-3 py-1.5 rounded-lg text-xs font-medium text-emerald-400 border border-emerald-500/20 hover:bg-emerald-500/10"
                  >
                    Acknowledge
                  </button>
                  <button
                    onClick={() => setDeclineId(a.id)}
                    className="px-3 py-1.5 rounded-lg text-xs font-medium text-red-400 border border-red-500/20 hover:bg-red-500/10"
                  >
                    Decline
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </>
  );
}
