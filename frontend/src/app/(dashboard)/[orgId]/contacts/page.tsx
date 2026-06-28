// src/app/(dashboard)/[orgId]/contacts/page.tsx
"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import {
  Plus,
  Users,
  Mail,
  Phone,
  Briefcase,
  Building2,
  ChevronRight,
  ChevronDown,
  Loader2,
  Search,
  Pencil,
  Trash2,
} from "lucide-react";
import gsap from "gsap";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import {
  listContacts,
  createContact,
  updateContact,
  deleteContact,
} from "@/lib/crm/contacts";
import { listCompanies } from "@/lib/crm/companies";
import ContactForm from "@/components/crm/contacts/ContactForm";
import type { Contact, Company } from "@/types/crm";

const SOURCE_LABELS: Record<string, string> = {
  linkedin: "LinkedIn",
  website: "Website",
  referral: "Referral",
  cold_call: "Cold Call",
  email_campaign: "Email",
  trade_show: "Trade Show",
  other: "Other",
};

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function ContactAvatar({ name }: { name: string }) {
  return (
    <div
      className="w-8 h-8 rounded-full flex-shrink-0 flex items-center justify-center text-xs font-bold text-white"
      style={{ background: "linear-gradient(135deg, #7c3aed, #a855f7)" }}
    >
      {name[0]?.toUpperCase() ?? "?"}
    </div>
  );
}

export default function ContactsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();

  const [contacts, setContacts] = useState<Contact[]>([]);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [loading, setLoading] = useState(true);
  const [pageErr, setPageErr] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);

  const canCreate = hasPermission("crm.contacts.create");
  const canUpdate = hasPermission("crm.contacts.update");
  const canDelete = hasPermission("crm.contacts.delete");

  const fetch = useCallback(async () => {
    setLoading(true);
    setPageErr(null);
    try {
      const [contactsData, companiesData] = await Promise.all([
        listContacts(orgId),
        listCompanies(orgId),
      ]);
      setContacts(contactsData.contacts);
      setCompanies(companiesData.companies);
    } catch {
      setPageErr("Failed to load contacts. Please refresh.");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    fetch();
  }, [fetch]);

  useEffect(() => {
    if (loading || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".contact-row");
    if (rows.length) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 6 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [loading]);

  // Map company_id → company name for display
  const companyMap = new Map(companies.map((c) => [c.id, c]));

  const filtered = contacts.filter((c) => {
    if (!search) return true;
    const q = search.toLowerCase();
    const name = `${c.first_name} ${c.last_name ?? ""}`.toLowerCase();
    return (
      name.includes(q) ||
      (c.email ?? "").toLowerCase().includes(q) ||
      (companyMap.get(c.company_id ?? "")?.name ?? "").toLowerCase().includes(q)
    );
  });

  const openCreate = () => {
    openDrawer({
      title: "New contact",
      width: "md",
      content: (
        <ContactForm
          orgId={orgId}
          onSave={async (values) => {
            const created = await createContact(orgId, {
              first_name: values.first_name,
              last_name: values.last_name || undefined,
              email: values.email || undefined,
              phone: values.phone || undefined,
              title: values.title || undefined,
              company_id: values.company_id || undefined,
              source: values.source || undefined,
            });
            setContacts((prev) => [created, ...prev]);
          }}
        />
      ),
    });
  };

  const openEdit = (contact: Contact) => {
    openDrawer({
      title: "Edit contact",
      width: "md",
      content: (
        <ContactForm
          contact={contact}
          orgId={orgId}
          onSave={async (values) => {
            const updated = await updateContact(orgId, contact.id, {
              first_name: values.first_name || undefined,
              last_name: values.last_name || undefined,
              email: values.email || undefined,
              phone: values.phone || undefined,
              title: values.title || undefined,
              company_id: values.company_id || undefined,
              source: values.source || undefined,
            });
            setContacts((prev) =>
              prev.map((c) => (c.id === updated.id ? updated : c)),
            );
          }}
        />
      ),
    });
  };

  const handleDelete = async (contactId: string) => {
    try {
      await deleteContact(orgId, contactId);
      setContacts((prev) => prev.filter((c) => c.id !== contactId));
      if (expandedId === contactId) setExpandedId(null);
    } catch {
      setPageErr("Failed to delete contact.");
    }
    setDeleteId(null);
  };

  return (
    <div className="p-6 md:p-8 max-w-5xl">
      {/* Header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1
            className="text-2xl font-bold text-[var(--text-primary)] mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Contacts
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            {contacts.length} total contacts
          </p>
        </div>
        {canCreate && (
          <button
            onClick={openCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New contact
          </button>
        )}
      </div>

      {pageErr && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          {pageErr}
        </div>
      )}

      {/* Search */}
      <div className="relative mb-5 max-w-xs">
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

      {loading ? (
        <div className="flex items-center gap-3 py-20 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading contacts…
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-12 h-12 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)] flex items-center justify-center mb-4">
            <Users size={20} className="text-[var(--text-muted)]" />
          </div>
          <p className="text-sm font-medium text-[var(--text-secondary)] mb-1">
            {search ? "No contacts match your search" : "No contacts yet"}
          </p>
          {canCreate && !search && (
            <button
              onClick={openCreate}
              className="mt-3 flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={14} />
              Add first contact
            </button>
          )}
        </div>
      ) : (
        <div>
          {/* Table header */}
          <div className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 px-4 py-2 mb-1">
            {["Contact", "Company", "Source", "Created", ""].map((h) => (
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
            {filtered.map((contact) => {
              const expanded = expandedId === contact.id;
              const confirming = deleteId === contact.id;
              const fullName = [contact.first_name, contact.last_name]
                .filter(Boolean)
                .join(" ");
              const company = companyMap.get(contact.company_id ?? "");

              return (
                <div key={contact.id} className="contact-row">
                  {/* Main row */}
                  <div
                    className={`
                      grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 items-center
                      px-4 py-3.5 rounded-xl border cursor-pointer transition-all duration-150
                      ${
                        expanded
                          ? "bg-[var(--bg-elevated)] border-purple-500/25 rounded-b-none border-b-0"
                          : "bg-[var(--bg-surface)] border-[var(--border)] hover:border-[var(--text-muted)]/20"
                      }
                    `}
                    onClick={() => setExpandedId(expanded ? null : contact.id)}
                  >
                    {/* Name */}
                    <div className="flex items-center gap-3 min-w-0">
                      <ContactAvatar name={contact.first_name} />
                      <div className="min-w-0">
                        <p
                          className="text-sm font-medium text-[var(--text-primary)] truncate"
                          style={{
                            fontFamily: "var(--font-inter, Inter, sans-serif)",
                          }}
                        >
                          {fullName}
                        </p>
                        {contact.title && (
                          <p className="text-xs text-[var(--text-muted)] truncate">
                            {contact.title}
                          </p>
                        )}
                      </div>
                    </div>

                    {/* Company */}
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {company?.name ?? "—"}
                    </span>

                    {/* Source */}
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {contact.source
                        ? (SOURCE_LABELS[contact.source] ?? contact.source)
                        : "—"}
                    </span>

                    {/* Created */}
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {formatDate(contact.created_at)}
                    </span>

                    {/* Chevron */}
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

                  {/* Expanded panel */}
                  {expanded && (
                    <div
                      className="px-5 py-4 bg-[var(--bg-elevated)] border border-purple-500/25 border-t-0 rounded-b-xl"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <div className="flex items-start justify-between gap-6">
                        {/* Details */}
                        <div className="grid grid-cols-3 gap-x-8 gap-y-3 flex-1">
                          {contact.email && (
                            <div className="flex items-center gap-2">
                              <Mail
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {contact.email}
                              </span>
                            </div>
                          )}
                          {contact.phone && (
                            <div className="flex items-center gap-2">
                              <Phone
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {contact.phone}
                              </span>
                            </div>
                          )}
                          {company && (
                            <div className="flex items-center gap-2">
                              <Building2
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {company.name}
                              </span>
                            </div>
                          )}
                          {contact.title && (
                            <div className="flex items-center gap-2">
                              <Briefcase
                                size={12}
                                className="text-[var(--text-muted)] flex-shrink-0"
                              />
                              <span className="text-xs text-[var(--text-secondary)]">
                                {contact.title}
                              </span>
                            </div>
                          )}
                        </div>

                        {/* Actions */}
                        {confirming ? (
                          <div className="flex items-center gap-2 flex-shrink-0">
                            <span className="text-xs text-[var(--text-muted)]">
                              Delete?
                            </span>
                            <button
                              onClick={() => handleDelete(contact.id)}
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
                            {canUpdate && (
                              <button
                                onClick={() => openEdit(contact)}
                                className="px-3 py-1.5 rounded-md text-xs font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors"
                              >
                                Edit
                              </button>
                            )}
                            {canDelete && (
                              <button
                                onClick={() => setDeleteId(contact.id)}
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
            Showing {filtered.length} of {contacts.length} contacts
          </p>
        </div>
      )}
    </div>
  );
}
