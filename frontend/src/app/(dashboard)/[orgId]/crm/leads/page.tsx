// src/app/(dashboard)/[orgId]/crm/leads/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  Mail,
  Phone,
  Briefcase,
  Loader2,
  Search,
  ArrowRightLeft,
  CalendarDays,
  MoreVertical,
  Trash2,
  Pencil,
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
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";
import LeadForm from "@/components/crm/leads/LeadForm";
import ConvertForm from "@/components/crm/leads/ConvertForm";
import type { Lead, LeadStatus } from "@/types/crm";

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
  const initial = name ? name.charAt(0).toUpperCase() : "?";
  return (
    <div className="w-8 h-8 rounded-full bg-purple-500/10 text-purple-400 border border-purple-500/20 flex items-center justify-center text-xs font-bold shrink-0">
      {initial}
    </div>
  );
}

interface LeadCardProps {
  lead: Lead;
  canUpdate: boolean;
  canConvert: boolean;
  canDelete: boolean;
  deleteId: string | null;
  setDeleteId: (id: string | null) => void;
  deletingId: string | null;
  handleDelete: (id: string) => void;
  openEdit: (lead: Lead) => void;
  openConvert: (lead: Lead) => void;
}

function LeadCard({
  lead,
  canUpdate,
  canConvert,
  canDelete,
  deleteId,
  setDeleteId,
  deletingId,
  handleDelete,
  openEdit,
  openConvert,
}: LeadCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    if (menuOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [menuOpen]);

  const confirming = deleteId === lead.id;
  const deleting = deletingId === lead.id;
  const status = STATUS_STYLE[lead.status as LeadStatus];
  const fullName = [lead.first_name, lead.last_name].filter(Boolean).join(" ");
  const isConverted = lead.status === "converted";

  return (
    <div className="bg-(--bg-surface) border border-(--border) rounded-xl p-5 hover:border-(--text-muted)/30 transition-colors relative flex flex-col h-full shadow-sm">
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3">
          <LeadAvatar name={lead.first_name} />
          <div>
            <h3
              className="text-sm font-semibold text-(--text-primary)"
              style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
            >
              {fullName}
            </h3>
            <p className="text-xs text-(--text-muted)">
              {lead.company_name ?? "No company"}
            </p>
          </div>
        </div>

        <div className="relative" ref={menuRef}>
          <button
            onClick={(e) => {
              e.stopPropagation();
              setMenuOpen(!menuOpen);
            }}
            className="p-1.5 rounded-md text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-colors"
          >
            <MoreVertical size={16} />
          </button>

          {menuOpen && (
            <div className="absolute right-0 mt-1 w-36 bg-(--bg-surface) border border-(--border) rounded-lg shadow-xl z-20 py-1 overflow-hidden">
              {canUpdate && !isConverted && (
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    openEdit(lead);
                  }}
                  className="w-full text-left px-3 py-2 text-xs font-medium text-(--text-secondary) hover:bg-(--bg-elevated) hover:text-(--text-primary) transition-colors flex items-center gap-2"
                >
                  <Pencil size={13} />
                  Edit
                </button>
              )}
              {canConvert && !isConverted && (
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    openConvert(lead);
                  }}
                  className="w-full text-left px-3 py-2 text-xs font-medium text-purple-400 hover:bg-purple-500/10 transition-colors flex items-center gap-2"
                >
                  <ArrowRightLeft size={13} />
                  Convert
                </button>
              )}
              {canDelete && (
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    setDeleteId(lead.id);
                  }}
                  className="w-full text-left px-3 py-2 text-xs font-medium text-red-400 hover:bg-red-500/10 transition-colors flex items-center gap-2"
                >
                  <Trash2 size={13} />
                  Delete
                </button>
              )}
              {!canUpdate && !canConvert && !canDelete && (
                <div className="px-3 py-2 text-xs text-(--text-muted) text-center">
                  No actions
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="flex items-center gap-2 mb-4 flex-wrap">
        <span
          className={`text-[0.65rem] font-semibold border px-2 py-0.5 rounded-full ${status.cls}`}
        >
          {status.label}
        </span>
        <span className="text-[0.65rem] font-medium border border-(--border) bg-(--bg-elevated) text-(--text-muted) px-2 py-0.5 rounded-full">
          {lead.source ? (SOURCE_LABELS[lead.source] ?? lead.source) : "Direct"}
        </span>
      </div>

      <div className="space-y-3 flex-1 mb-5">
        {lead.email && (
          <div className="flex items-center gap-2.5">
            <Mail
              size={14}
              className="text-(--text-muted) shrink-0"
            />
            <span className="text-xs text-(--text-secondary) truncate">
              {lead.email}
            </span>
          </div>
        )}
        {lead.phone && (
          <div className="flex items-center gap-2.5">
            <Phone
              size={14}
              className="text-(--text-muted) shrink-0"
            />
            <span className="text-xs text-(--text-secondary) truncate">
              {lead.phone}
            </span>
          </div>
        )}
        {lead.title && (
          <div className="flex items-center gap-2.5">
            <Briefcase
              size={14}
              className="text-(--text-muted) shrink-0"
            />
            <span className="text-xs text-(--text-secondary) truncate">
              {lead.title}
            </span>
          </div>
        )}
      </div>

      <div className="pt-3 border-t border-(--border)">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5 text-xs text-(--text-muted)">
            <CalendarDays size={12} />
            <span>Added {new Date(lead.created_at).toLocaleDateString()}</span>
          </div>
          {isConverted && lead.converted_at && (
            <div className="flex items-center gap-1.5 text-xs text-purple-400">
              <ArrowRightLeft size={12} />
              <span>{new Date(lead.converted_at).toLocaleDateString()}</span>
            </div>
          )}
        </div>
      </div>

      {confirming && (
        <div className="absolute inset-0 bg-(--bg-surface)/90 backdrop-blur-sm rounded-xl border border-red-500/30 flex flex-col items-center justify-center p-4 text-center z-10 animate-in fade-in">
          <p className="text-sm font-semibold text-(--text-primary) mb-1">
            Delete Lead?
          </p>
          <p className="text-xs text-(--text-muted) mb-4">
            This action cannot be undone.
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setDeleteId(null)}
              className="px-3 py-1.5 rounded-lg text-xs font-medium text-(--text-secondary) bg-(--bg-elevated) hover:bg-(--border) transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={() => handleDelete(lead.id)}
              disabled={deleting}
              className="px-3 py-1.5 rounded-lg text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors disabled:opacity-50"
            >
              {deleting ? "Deleting..." : "Delete"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export default function LeadsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();

  const [activeStatus, setActiveStatus] = useState<FilterStatus>("all");
  const [sourceFilter, setSourceFilter] = useState("");
  const [search, setSearch] = useState("");
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);

  const canCreate = hasPermission("crm.leads.create");
  const canUpdate = hasPermission("crm.leads.update");
  const canDelete = hasPermission("crm.leads.delete");
  const canConvert = hasPermission("crm.leads.convert");

  const leadsKey = queryKeys.crm.leads.list(orgId);
  const leadsQuery = useQuery({
    queryKey: leadsKey,
    queryFn: () => listLeads(orgId).then((r) => r.leads),
  });
  const leads = leadsQuery.data ?? [];

  useEffect(() => {
    if (leadsQuery.isPending || !listRef.current) return;
    const cards = listRef.current.children;
    if (cards.length) {
      gsap.fromTo(
        cards,
        { opacity: 0, y: 10 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.03, ease: "power2.out" },
      );
    }
  }, [leadsQuery.isPending, leads.length]);

  const filtered = leads.filter((l) => {
    if (activeStatus !== "all" && l.status !== activeStatus) return false;
    if (sourceFilter && l.source !== sourceFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      const name = `${l.first_name} ${l.last_name ?? ""}`.toLowerCase();
      if (
        !name.includes(q) &&
        !(l.email ?? "").toLowerCase().includes(q) &&
        !(l.company_name ?? "").toLowerCase().includes(q)
      )
        return false;
    }
    return true;
  });

  const sources = getSources(leads);

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
            queryClient.setQueryData<Lead[]>(leadsKey, (old) => [
              created,
              ...(old ?? []),
            ]);
            toast.success("Lead created.");
          }}
        />
      ),
    });
  };

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
            queryClient.setQueryData<Lead[]>(leadsKey, (old) =>
              (old ?? []).map((l) => (l.id === updated.id ? updated : l)),
            );
            toast.success("Lead updated.");
          }}
        />
      ),
    });
  };

  const openConvert = (lead: Lead) => {
    openDrawer({
      title: "Convert lead",
      width: "md",
      content: (
        <ConvertForm
          lead={lead}
          orgId={orgId}
          onSave={async (payload) => {
            const result = await convertLead(orgId, lead.id, payload);
            queryClient.setQueryData<Lead[]>(leadsKey, (old) =>
              (old ?? []).map((l) =>
                l.id === result.lead.id ? result.lead : l,
              ),
            );
            toast.success("Lead converted.");
          }}
        />
      ),
    });
  };

  const handleDelete = async (leadId: string) => {
    setDeletingId(leadId);
    toast.error(null);
    try {
      await deleteLead(orgId, leadId);
      queryClient.setQueryData<Lead[]>(leadsKey, (old) =>
        (old ?? []).filter((l) => l.id !== leadId),
      );
      toast.success("Lead deleted.");
    } catch {
      toast.error("Failed to delete lead.");
    } finally {
      setDeletingId(null);
      setDeleteId(null);
    }
  };

  return (
    <div className="p-6 md:p-8 max-w-[1600px] mx-auto">
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1
            className="text-2xl font-bold text-(--text-primary) mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Leads
          </h1>
          <p className="text-sm text-(--text-muted)">
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

      {leadsQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          Failed to load leads. Please refresh.
        </div>
      )}

      <div className="space-y-3 mb-5">
        <div className="flex items-center gap-3">
          <div className="relative flex-1 max-w-xs">
            <Search
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-(--text-muted)"
            />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search name, email, company…"
              className="w-full pl-9 pr-3.5 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary) placeholder:text-(--text-muted) outline-none focus:border-purple-500 transition-all"
            />
          </div>
          <select
            value={sourceFilter}
            onChange={(e) => setSourceFilter(e.target.value)}
            className="px-3.5 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-secondary) outline-none focus:border-purple-500 transition-all"
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

        <div className="flex items-center gap-0.5 border-b border-(--border)">
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
                className={`flex items-center gap-2 px-3.5 py-2.5 text-sm font-medium -mb-px border-b-2 transition-colors ${
                  active
                    ? "text-purple-400 border-purple-500"
                    : "text-(--text-muted) border-transparent hover:text-(--text-secondary)"
                }`}
              >
                {tab.label}
                {count > 0 && (
                  <span
                    className={`text-xs px-1.5 py-0.5 rounded-full min-w-[20px] text-center ${
                      active
                        ? "bg-purple-500/15 text-purple-400"
                        : "bg-(--bg-elevated) text-(--text-muted)"
                    }`}
                  >
                    {count}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      </div>

      {leadsQuery.isPending ? (
        <div className="flex items-center gap-3 py-20 text-sm text-(--text-muted)">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading leads…
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-(--bg-elevated) border border-(--border) flex items-center justify-center mb-4">
            <ArrowRightLeft size={18} className="text-(--text-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-secondary) mb-1">
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
          <div
            ref={listRef}
            className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 mt-4"
          >
            {filtered.map((lead) => (
              <LeadCard
                key={lead.id}
                lead={lead}
                canUpdate={canUpdate}
                canConvert={canConvert}
                canDelete={canDelete}
                deleteId={deleteId}
                setDeleteId={setDeleteId}
                deletingId={deletingId}
                handleDelete={handleDelete}
                openEdit={openEdit}
                openConvert={openConvert}
              />
            ))}
          </div>
          <p className="mt-4 text-xs text-(--text-muted)">
            Showing {filtered.length} of {leads.length} leads
          </p>
        </>
      )}
    </div>
  );
}
