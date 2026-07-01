// src/app/(dashboard)/[orgId]/companies/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import {
  Plus,
  Building2,
  Globe,
  Phone,
  ChevronRight,
  ChevronDown,
  Loader2,
  Search,
  Pencil,
  Trash2,
  ExternalLink,
  Users,
} from "lucide-react";
import gsap from "gsap";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import {
  listCompanies,
  createCompany,
  updateCompany,
  deleteCompany,
} from "@/lib/crm/companies";
import { queryKeys } from "@/lib/queryKeys";
import CompanyForm from "@/components/crm/companies/CompanyForm";
import type { Company } from "@/types/crm";

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function CompanyAvatar({ name }: { name: string }) {
  return (
    <div
      className="w-8 h-8 rounded-lg flex-shrink-0 flex items-center justify-center text-xs font-bold text-white"
      style={{ background: "linear-gradient(135deg, #7c3aed, #a855f7)" }}
    >
      {name[0]?.toUpperCase() ?? "?"}
    </div>
  );
}

export default function CompaniesPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const router = useRouter();
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [mutationErr, setMutationErr] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);

  const canCreate = hasPermission("crm.companies.create");
  const canUpdate = hasPermission("crm.companies.update");
  const canDelete = hasPermission("crm.companies.delete");

  // ── Query ─────────────────────────────────────────────────────────────────
  const companiesKey = queryKeys.crm.companies.list(orgId);
  const companiesQuery = useQuery({
    queryKey: companiesKey,
    queryFn: () => listCompanies(orgId).then((r) => r.companies),
  });
  const companies = companiesQuery.data ?? [];

  // ── GSAP ──────────────────────────────────────────────────────────────────
  useEffect(() => {
    if (companiesQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".company-row");
    if (rows.length) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 6 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [companiesQuery.isPending]);

  const filtered = companies.filter((c) => {
    if (!search) return true;
    const q = search.toLowerCase();
    return (
      c.name.toLowerCase().includes(q) ||
      (c.industry ?? "").toLowerCase().includes(q) ||
      (c.domain ?? "").toLowerCase().includes(q)
    );
  });

  // ── Handlers ──────────────────────────────────────────────────────────────
  const openCreate = () => {
    openDrawer({
      title: "New company",
      width: "md",
      content: (
        <CompanyForm
          onSave={async (values) => {
            const created = await createCompany(orgId, {
              name: values.name,
              domain: values.domain || undefined,
              industry: values.industry || undefined,
              website: values.website || undefined,
              phone: values.phone || undefined,
              address: values.address || undefined,
              country: values.country || undefined,
            });
            queryClient.setQueryData<Company[]>(companiesKey, (old) => [
              created,
              ...(old ?? []),
            ]);
          }}
        />
      ),
    });
  };

  const openEdit = (company: Company) => {
    openDrawer({
      title: "Edit company",
      width: "md",
      content: (
        <CompanyForm
          company={company}
          onSave={async (values) => {
            const updated = await updateCompany(orgId, company.id, {
              name: values.name || undefined,
              domain: values.domain || undefined,
              industry: values.industry || undefined,
              website: values.website || undefined,
              phone: values.phone || undefined,
              address: values.address || undefined,
              country: values.country || undefined,
            });
            queryClient.setQueryData<Company[]>(companiesKey, (old) =>
              (old ?? []).map((c) => (c.id === updated.id ? updated : c)),
            );
          }}
        />
      ),
    });
  };

  const handleDelete = async (companyId: string) => {
    setMutationErr(null);
    try {
      await deleteCompany(orgId, companyId);
      queryClient.setQueryData<Company[]>(companiesKey, (old) =>
        (old ?? []).filter((c) => c.id !== companyId),
      );
      if (expandedId === companyId) setExpandedId(null);
    } catch {
      setMutationErr("Failed to delete company.");
    }
    setDeleteId(null);
  };

  // ── Render ────────────────────────────────────────────────────────────────
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
            Companies
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            {companies.length} total companies
          </p>
        </div>
        {canCreate && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New company
          </button>
        )}
      </div>

      {mutationErr && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          {mutationErr}
        </div>
      )}
      {companiesQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          Failed to load companies. Please refresh.
        </div>
      )}

      <div className="relative mb-5 max-w-xs">
        <Search
          size={14}
          className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]"
        />
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search name, industry, domain…"
          className="w-full pl-9 pr-3.5 py-2 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] outline-none focus:border-purple-500 transition-all"
        />
      </div>

      {companiesQuery.isPending ? (
        <div className="flex items-center gap-3 py-20 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading companies…
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <Building2 size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
            {search ? "No companies match your search" : "No companies yet"}
          </p>
          {canCreate && !search && (
            <button
              onClick={openCreate}
              className="mt-3 flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={14} />
              Add first company
            </button>
          )}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 px-4 py-2 mb-1">
            {["Company", "Industry", "Domain", "Created", ""].map((h) => (
              <span
                key={h}
                className="text-[0.65rem] font-semibold text-[var(--text-muted)] uppercase tracking-wider"
                style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
              >
                {h}
              </span>
            ))}
          </div>

          <div ref={listRef} className="space-y-1">
            {filtered.map((company) => {
              const expanded = expandedId === company.id;
              const confirming = deleteId === company.id;

              return (
                <div key={company.id} className="company-row">
                  <div
                    className={`grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 items-center px-4 py-3.5 rounded-xl border cursor-pointer transition-all duration-150 ${
                      expanded
                        ? "bg-[var(--bg-elevated)] border-purple-500/25 rounded-b-none border-b-0"
                        : "bg-[var(--bg-surface)] border-[var(--border)] hover:border-[var(--text-muted)]/20"
                    }`}
                    onClick={() => setExpandedId(expanded ? null : company.id)}
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <CompanyAvatar name={company.name} />
                      <span
                        className="text-sm font-medium text-[var(--text-primary)] truncate"
                        style={{
                          fontFamily: "var(--font-inter, Inter, sans-serif)",
                        }}
                      >
                        {company.name}
                      </span>
                    </div>
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {company.industry ?? "—"}
                    </span>
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {company.domain ?? "—"}
                    </span>
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {formatDate(company.created_at)}
                    </span>
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

                  {expanded && (
                    <div
                      className="px-5 py-4 bg-[var(--bg-elevated)] border border-purple-500/25 border-t-0 rounded-b-xl"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <div className="flex items-start justify-between gap-6">
                        <div className="grid grid-cols-3 gap-x-8 gap-y-3 flex-1">
                          {company.website && (
                            <div className="flex items-center gap-2">
                              <Globe
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <a
                                href={company.website}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-xs text-purple-400 hover:underline flex items-center gap-1"
                                onClick={(e) => e.stopPropagation()}
                              >
                                {company.website.replace(/^https?:\/\//, "")}
                                <ExternalLink size={9} />
                              </a>
                            </div>
                          )}
                          {company.phone && (
                            <div className="flex items-center gap-2">
                              <Phone
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {company.phone}
                              </span>
                            </div>
                          )}
                          {company.address && (
                            <div className="flex items-center gap-2 col-span-2">
                              <span className="text-xs text-[var(--text-muted)] flex-shrink-0">
                                📍
                              </span>
                              <span className="text-xs text-[var(--text-secondary)]">
                                {company.address}
                              </span>
                            </div>
                          )}
                        </div>
                        {confirming ? (
                          <div className="flex items-center gap-2 flex-shrink-0">
                            <span className="text-xs text-[var(--text-muted)]">
                              Delete?
                            </span>
                            <button
                              onClick={() => handleDelete(company.id)}
                              className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                            >
                              Yes
                            </button>
                            <button
                              onClick={() => setDeleteId(null)}
                              className="px-2.5 py-1 rounded-md text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] transition-colors"
                            >
                              No
                            </button>
                          </div>
                        ) : (
                          <div className="flex items-center gap-2 flex-shrink-0">
                            <button
                              onClick={() =>
                                router.push(`/${orgId}/companies/${company.id}`)
                              }
                              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-surface)] transition-colors"
                            >
                              <Users size={12} />
                              Contacts
                            </button>
                            {canUpdate && (
                              <button
                                onClick={() => openEdit(company)}
                                className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-surface)] transition-colors"
                                title="Edit"
                              >
                                <Pencil size={13} />
                              </button>
                            )}
                            {canDelete && (
                              <button
                                onClick={() => setDeleteId(company.id)}
                                className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-red-400 hover:bg-red-500/10 transition-colors"
                                title="Delete"
                              >
                                <Trash2 size={13} />
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
            Showing {filtered.length} of {companies.length} companies
          </p>
        </>
      )}
    </div>
  );
}
