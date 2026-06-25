// src/app/(dashboard)/[orgId]/crm/leads/page.tsx
"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import {
  Plus,
  ChevronRight,
  ChevronDown,
  Mail,
  Phone,
  Briefcase,
  Loader2,
  Search,
  ArrowRightLeft,
} from "lucide-react";
import gsap from "gsap";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import {
  listLeads,
  createLead,
  updateLead,
  deleteLead,
  convertLead,
} from "@/lib/crm/leads";
import LeadForm from "@/components/crm/leads/LeadForm";
import ConvertForm from "@/components/crm/leads/ConvertForm";
import type { Lead, LeadStatus } from "@/types/crm";

// ── Status config ─────────────────────────────────────
type FilterStatus = "all" | LeadStatus;

const STATUS_TABS: { key: FilterStatus; label: string }[] = [
  { key: "all", label: "All" },
  { key: "new", label: "New" },
  { key: "contacted", label: "Contacted" },
  { key: "qualified", label: "Qualified" },
  { key: "unqualified", label: "Unqualified" },
  { key: "converted", label: "Converted" },
];

const STATUS_STYLE: Record<LeadStatus, { label: string; cls: string }> = {
  new: {
    label: "New",
    cls: "text-zinc-400   bg-zinc-500/10   border-zinc-500/20",
  },
  contacted: {
    label: "Contacted",
    cls: "text-blue-400   bg-blue-500/10   border-blue-500/20",
  },
  qualified: {
    label: "Qualified",
    cls: "text-emerald-400 bg-emerald-500/10 border-emerald-500/20",
  },
  unqualified: {
    label: "Unqualified",
    cls: "text-rose-400   bg-rose-500/10   border-rose-500/20",
  },
  converted: {
    label: "Converted",
    cls: "text-purple-400 bg-purple-500/10  border-purple-500/20",
  },
};

const SOURCE_LABELS: Record<string, string> = {
  linkedin: "LinkedIn",
  website: "Website",
  referral: "Referral",
  cold_call: "Cold Call",
  email_campaign: "Email",
  trade_show: "Trade Show",
  other: "Other",
};

// Distinct sources from the lead list
function getSources(leads: Lead[]) {
  return [...new Set(leads.map((l) => l.source).filter(Boolean) as string[])];
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function LeadAvatar({ name }: { name: string }) {
  return (
    <div
      className="w-8 h-8 rounded-full flex-shrink-0 flex items-center justify-center text-xs font-bold text-white"
      style={{ background: "linear-gradient(135deg, #7c3aed, #a855f7)" }}
    >
      {name[0]?.toUpperCase() ?? "?"}
    </div>
  );
}

// ── Page ──────────────────────────────────────────────
export default function LeadsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();

  const [leads, setLeads] = useState<Lead[]>([]);
  const [loading, setLoading] = useState(true);
  const [pageErr, setPageErr] = useState<string | null>(null);
  const [activeStatus, setActiveStatus] = useState<FilterStatus>("all");
  const [sourceFilter, setSourceFilter] = useState("");
  const [search, setSearch] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);

  const canCreate = hasPermission("crm.leads.create");
  const canUpdate = hasPermission("crm.leads.update");
  const canDelete = hasPermission("crm.leads.delete");
  const canConvert = hasPermission("crm.leads.convert");

  // Fetch
  const fetch = useCallback(async () => {
    setLoading(true);
    setPageErr(null);
    try {
      const data = await listLeads(orgId);
      setLeads(data.leads);
    } catch {
      setPageErr("Failed to load leads. Please refresh.");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    fetch();
  }, [fetch]);

  // GSAP
  useEffect(() => {
    if (loading || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".lead-row");
    if (rows.length) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 6 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.035, ease: "power2.out" },
      );
    }
  }, [loading]);

  // Client-side filtering
  const filtered = leads.filter((l) => {
    if (activeStatus !== "all" && l.status !== activeStatus) return false;
    if (sourceFilter && l.source !== sourceFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      const name = `${l.first_name} ${l.last_name ?? ""}`.toLowerCase();
      const email = (l.email ?? "").toLowerCase();
      const company = (l.company_name ?? "").toLowerCase();
      if (!name.includes(q) && !email.includes(q) && !company.includes(q))
        return false;
    }
    return true;
  });

  const sources = getSources(leads);

  // Open create drawer
  const openCreate = () => {
    openDrawer({
      title: "New lead",
      width: "md",
      content: (
        <LeadForm
          onSave={async (values) => {
            const created = await createLead(orgId, {
              first_name: values.first_name,
              last_name: values.last_name || undefined,
              email: values.email || undefined,
              phone: values.phone || undefined,
              company_name: values.company_name || undefined,
              title: values.title || undefined,
              source: values.source || undefined,
            });
            setLeads((prev) => [created, ...prev]);
          }}
        />
      ),
    });
  };

  // Open edit drawer
  const openEdit = (lead: Lead) => {
    openDrawer({
      title: "Edit lead",
      width: "md",
      content: (
        <LeadForm
          lead={lead}
          onSave={async (values) => {
            const updated = await updateLead(orgId, lead.id, {
              first_name: values.first_name || undefined,
              last_name: values.last_name || undefined,
              email: values.email || undefined,
              phone: values.phone || undefined,
              company_name: values.company_name || undefined,
              title: values.title || undefined,
              source: values.source || undefined,
              status: values.status || undefined,
            });
            setLeads((prev) =>
              prev.map((l) => (l.id === updated.id ? updated : l)),
            );
          }}
        />
      ),
    });
  };

  // Open convert drawer
  const openConvert = (lead: Lead) => {
    openDrawer({
      title: `Convert lead`,
      width: "md",
      content: (
        <ConvertForm
          lead={lead}
          orgId={orgId}
          onSave={async (payload) => {
            const result = await convertLead(orgId, lead.id, payload);
            // Update lead status to converted
            setLeads((prev) =>
              prev.map((l) => (l.id === result.lead.id ? result.lead : l)),
            );
          }}
        />
      ),
    });
  };

  // Delete
  const handleDelete = async (leadId: string) => {
    setDeletingId(leadId);
    try {
      await deleteLead(orgId, leadId);
      setLeads((prev) => prev.filter((l) => l.id !== leadId));
      if (expandedId === leadId) setExpandedId(null);
    } catch {
      setPageErr("Failed to delete lead.");
    } finally {
      setDeletingId(null);
      setDeleteId(null);
    }
  };

  return (
    <div className="p-6 md:p-8 max-w-6xl">
      {/* Page header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1
            className="text-2xl font-bold text-[var(--text-primary)] mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Leads
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            {leads.length} total leads
          </p>
        </div>
        {canCreate && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New lead
          </button>
        )}
      </div>

      {pageErr && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          {pageErr}
        </div>
      )}

      {/* ── Filter bar ─────────────────────────────── */}
      <div className="space-y-3 mb-5">
        {/* Search + Source row */}
        <div className="flex items-center gap-3">
          {/* Search */}
          <div className="relative flex-1 max-w-xs">
            <Search
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]"
            />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search name, email, company…"
              className="
                w-full pl-9 pr-3.5 py-2 rounded-lg text-sm
                bg-[var(--bg-elevated)] border border-[var(--border)]
                text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
                outline-none focus:border-purple-500 transition-all
              "
            />
          </div>

          {/* Source */}
          <select
            value={sourceFilter}
            onChange={(e) => setSourceFilter(e.target.value)}
            className="
              px-3.5 py-2 rounded-lg text-sm
              bg-[var(--bg-elevated)] border border-[var(--border)]
              text-[var(--text-secondary)] outline-none
              focus:border-purple-500 transition-all
            "
          >
            <option value="">All sources</option>
            {sources.map((s) => (
              <option
                key={s}
                value={s}
                style={{ background: "var(--bg-elevated)" }}
              >
                {SOURCE_LABELS[s] ?? s}
              </option>
            ))}
          </select>
        </div>

        {/* Status tabs */}
        <div className="flex items-center gap-0.5 border-b border-[var(--border)]">
          {STATUS_TABS.map((tab) => {
            const count =
              tab.key === "all"
                ? leads.length
                : leads.filter((l) => l.status === tab.key).length;
            const active = activeStatus === tab.key;

            return (
              <button
                key={tab.key}
                onClick={() => setActiveStatus(tab.key)}
                className={`
                  flex items-center gap-2 px-3.5 py-2.5 text-sm font-medium -mb-px border-b-2 transition-colors
                  ${
                    active
                      ? "text-purple-400 border-purple-500"
                      : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
                  }
                `}
              >
                {tab.label}
                {count > 0 && (
                  <span
                    className={`
                    text-xs px-1.5 py-0.5 rounded-full min-w-[20px] text-center
                    ${
                      active
                        ? "bg-purple-500/15 text-purple-400"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"
                    }
                  `}
                  >
                    {count}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      </div>

      {/* ── Table ────────────────────────────────────── */}
      {loading ? (
        <div className="flex items-center gap-3 py-20 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading leads…
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <ArrowRightLeft size={18} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
            {search || sourceFilter || activeStatus !== "all"
              ? "No leads match your filters"
              : "No leads yet"}
          </p>
          {canCreate && !search && !sourceFilter && activeStatus === "all" && (
            <button
              onClick={openCreate}
              className="mt-3 flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={14} />
              Add first lead
            </button>
          )}
        </div>
      ) : (
        <>
          {/* Table header */}
          <div className="grid grid-cols-[1fr_1fr_auto_auto_auto_auto] gap-4 px-4 py-2 mb-1">
            {["Name", "Company", "Status", "Source", "Created", ""].map((h) => (
              <span
                key={h}
                className="text-[0.65rem] font-semibold text-[var(--text-muted)] uppercase tracking-wider"
                style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
              >
                {h}
              </span>
            ))}
          </div>

          {/* Rows */}
          <div ref={listRef} className="space-y-1">
            {filtered.map((lead) => {
              const expanded = expandedId === lead.id;
              const confirming = deleteId === lead.id;
              const deleting = deletingId === lead.id;
              const status = STATUS_STYLE[lead.status];
              const fullName = [lead.first_name, lead.last_name]
                .filter(Boolean)
                .join(" ");
              const isConverted = lead.status === "converted";

              return (
                <div key={lead.id} className="lead-row">
                  {/* Main row */}
                  <div
                    className={`
                      grid grid-cols-[1fr_1fr_auto_auto_auto_auto] gap-4 items-center
                      px-4 py-3 rounded-xl border transition-all duration-150 cursor-pointer
                      ${
                        expanded
                          ? "bg-[var(--bg-elevated)] border-purple-500/25 rounded-b-none border-b-0"
                          : "bg-[var(--bg-surface)] border-[var(--border)] hover:border-[var(--text-muted)]/20"
                      }
                    `}
                    onClick={() => setExpandedId(expanded ? null : lead.id)}
                  >
                    {/* Name */}
                    <div className="flex items-center gap-3 min-w-0">
                      <LeadAvatar name={lead.first_name} />
                      <span
                        className="text-sm font-medium text-[var(--text-primary)] truncate"
                        style={{
                          fontFamily: "var(--font-inter, Inter, sans-serif)",
                        }}
                      >
                        {fullName}
                      </span>
                    </div>

                    {/* Company */}
                    <span className="text-sm text-[var(--text-muted)] truncate">
                      {lead.company_name ?? "—"}
                    </span>

                    {/* Status */}
                    <span
                      className={`text-[0.65rem] font-semibold border px-2 py-0.5 rounded-full whitespace-nowrap ${status.cls}`}
                    >
                      {status.label}
                    </span>

                    {/* Source */}
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {lead.source
                        ? (SOURCE_LABELS[lead.source] ?? lead.source)
                        : "—"}
                    </span>

                    {/* Created */}
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {formatDate(lead.created_at)}
                    </span>

                    {/* Expand chevron */}
                    <div className="flex items-center justify-end">
                      {expanded ? (
                        <ChevronDown
                          size={14}
                          className="text-[var(--text-muted)]"
                        />
                      ) : (
                        <ChevronRight
                          size={14}
                          className="text-[var(--text-muted)]"
                        />
                      )}
                    </div>
                  </div>

                  {/* Expanded detail panel */}
                  {expanded && (
                    <div
                      className="px-5 py-4 bg-[var(--bg-elevated)] border border-purple-500/25 border-t-0 rounded-b-xl"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <div className="flex items-start justify-between gap-6">
                        {/* Detail fields */}
                        <div className="grid grid-cols-3 gap-x-8 gap-y-3 flex-1">
                          {lead.email && (
                            <div className="flex items-center gap-2">
                              <Mail
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {lead.email}
                              </span>
                            </div>
                          )}
                          {lead.phone && (
                            <div className="flex items-center gap-2">
                              <Phone
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {lead.phone}
                              </span>
                            </div>
                          )}
                          {lead.title && (
                            <div className="flex items-center gap-2">
                              <Briefcase
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {lead.title}
                              </span>
                            </div>
                          )}
                          {isConverted && lead.converted_at && (
                            <div className="flex items-center gap-2 col-span-3">
                              <ArrowRightLeft
                                size={12}
                                className="text-purple-400 flex-shrink-0"
                              />
                              <span className="text-xs text-purple-400">
                                Converted on {formatDate(lead.converted_at)}
                              </span>
                            </div>
                          )}
                        </div>

                        {/* Action buttons */}
                        {confirming ? (
                          <div className="flex items-center gap-2 flex-shrink-0">
                            <span className="text-xs text-[var(--text-muted)]">
                              Delete?
                            </span>
                            <button
                              onClick={() => handleDelete(lead.id)}
                              disabled={deleting}
                              className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors disabled:opacity-50"
                            >
                              {deleting ? "…" : "Yes"}
                            </button>
                            <button
                              onClick={() => setDeleteId(null)}
                              className="px-2.5 py-1 rounded-md text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                            >
                              No
                            </button>
                          </div>
                        ) : (
                          <div className="flex items-center gap-2 flex-shrink-0">
                            {canUpdate && !isConverted && (
                              <button
                                onClick={() => openEdit(lead)}
                                className="px-3 py-1.5 rounded-md text-xs font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors"
                              >
                                Edit
                              </button>
                            )}
                            {canConvert && !isConverted && (
                              <button
                                onClick={() => openConvert(lead)}
                                className="px-3 py-1.5 rounded-md text-xs font-medium text-purple-400 border border-purple-500/30 hover:bg-purple-500/10 transition-colors"
                              >
                                Convert
                              </button>
                            )}
                            {canDelete && (
                              <button
                                onClick={() => setDeleteId(lead.id)}
                                className="px-3 py-1.5 rounded-md text-xs font-medium text-[var(--text-muted)] border border-[var(--border)] hover:text-red-400 hover:border-red-500/30 hover:bg-red-500/5 transition-colors"
                              >
                                Delete
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          <p className="mt-4 text-xs text-[var(--text-muted)]">
            Showing {filtered.length} of {leads.length} leads
          </p>
        </>
      )}
    </div>
  );
}
