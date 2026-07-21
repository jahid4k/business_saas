// src/app/(dashboard)/[orgId]/companies/[companyId]/page.tsx
"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Globe,
  Phone,
  Building2,
  Plus,
  Mail,
  Briefcase,
  Loader2,
  Pencil,
  Trash2,
  ExternalLink,
  Sparkles,
} from "lucide-react";
import gsap from "gsap";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import {
  getCompany,
  enrichCompany,
  updateCompany,
  deleteCompany,
  getCompanyContacts,
} from "@/lib/crm/companies";
import {
  createContact,
  updateContact,
  deleteContact,
} from "@/lib/crm/contacts";
import CompanyForm from "@/components/crm/companies/CompanyForm";
import ContactForm from "@/components/crm/contacts/ContactForm";
import type { Company, Contact } from "@/types/crm";

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
      className="w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-xs font-bold text-white"
      style={{ background: "linear-gradient(135deg, #5b21b6, #7c3aed)" }}
    >
      {name[0]?.toUpperCase() ?? "?"}
    </div>
  );
}

export default function CompanyDetailPage({
  params,
}: {
  params: Promise<{ orgId: string; companyId: string }>;
}) {
  const { orgId, companyId } = use(params);
  const router = useRouter();
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();

  const [company, setCompany] = useState<Company | null>(null);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [pageErr, setPageErr] = useState<string | null>(null);
  const [deleteContactId, setDeleteContactId] = useState<string | null>(null);
  const [enriching, setEnriching] = useState(false);

  const contactListRef = useRef<HTMLDivElement>(null);

  const canUpdateCompany = hasPermission("crm.companies.update");
  const canDeleteCompany = hasPermission("crm.companies.delete");
  const canCreateContact = hasPermission("crm.contacts.create");
  const canUpdateContact = hasPermission("crm.contacts.update");
  const canDeleteContact = hasPermission("crm.contacts.delete");

  const handleEnrich = async () => {
    if (!company?.domain) {
      setPageErr("Company domain is required for enrichment.");
      return;
    }
    setEnriching(true);
    setPageErr(null);
    try {
      const data = await enrichCompany(orgId, company.domain);
      // Auto-update the company with the enriched data
      const updated = await updateCompany(orgId, companyId, {
        name: data.name,
        industry: data.industry,
        // The mock returns more fields, but for now we only update what our model has
      });
      setCompany(updated);
      alert("Company enriched successfully!");
    } catch {
      setPageErr("Failed to enrich company data.");
    } finally {
      setEnriching(false);
    }
  };

  const fetch = useCallback(async () => {
    setLoading(true);
    setPageErr(null);
    try {
      const [comp, contactsData] = await Promise.all([
        getCompany(orgId, companyId),
        getCompanyContacts(orgId, companyId),
      ]);
      setCompany(comp);
      setContacts(contactsData.contacts);
    } catch {
      setPageErr("Failed to load company details.");
    } finally {
      setLoading(false);
    }
  }, [orgId, companyId]);

  useEffect(() => {
    fetch();
  }, [fetch]);

  // GSAP for contact rows
  useEffect(() => {
    if (loading || !contactListRef.current) return;
    const rows = contactListRef.current.querySelectorAll(".contact-row");
    if (rows.length) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 5 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.04, ease: "power2.out" },
      );
    }
  }, [loading]);

  const openEditCompany = () => {
    if (!company) return;
    openDrawer({
      title: "Edit company",
      width: "md",
      content: (
        <CompanyForm
          company={company}
          onSave={async (values) => {
            const updated = await updateCompany(orgId, companyId, values);
            setCompany(updated);
          }}
        />
      ),
    });
  };

  const handleDeleteCompany = async () => {
    try {
      await deleteCompany(orgId, companyId);
      router.push(`/${orgId}/companies`);
    } catch {
      setPageErr("Failed to delete company.");
    }
  };

  const openAddContact = () => {
    openDrawer({
      title: "Add contact",
      width: "md",
      content: (
        <ContactForm
          orgId={orgId}
          preselectedCompany={companyId}
          onSave={async (values) => {
            const created = await createContact(orgId, {
              ...values,
              company_id: companyId,
            });
            setContacts((prev) => [created, ...prev]);
          }}
        />
      ),
    });
  };

  const openEditContact = (contact: Contact) => {
    openDrawer({
      title: "Edit contact",
      width: "md",
      content: (
        <ContactForm
          contact={contact}
          orgId={orgId}
          onSave={async (values) => {
            const updated = await updateContact(orgId, contact.id, values);
            setContacts((prev) =>
              prev.map((c) => (c.id === updated.id ? updated : c)),
            );
          }}
        />
      ),
    });
  };

  const handleDeleteContact = async (contactId: string) => {
    try {
      await deleteContact(orgId, contactId);
      setContacts((prev) => prev.filter((c) => c.id !== contactId));
    } catch {
      setPageErr("Failed to delete contact.");
    }
    setDeleteContactId(null);
  };

  if (loading) {
    return (
      <div className="flex items-center gap-3 p-8 text-sm text-(--text-muted)">
        <Loader2 size={15} className="animate-spin text-purple-500" />
        Loading company…
      </div>
    );
  }

  if (!company) {
    return (
      <div className="p-8">
        <p className="text-sm text-red-400">
          {pageErr ?? "Company not found."}
        </p>
        <button
          onClick={() => router.push(`/${orgId}/companies`)}
          className="mt-3 text-sm text-purple-400 hover:underline"
        >
          ← Back to companies
        </button>
      </div>
    );
  }

  return (
    <div className="p-6 md:p-8 max-w-4xl">
      {/* Back */}
      <button
        onClick={() => router.push(`/${orgId}/companies`)}
        className="flex items-center gap-2 text-sm text-(--text-muted) hover:text-(--text-secondary) transition-colors mb-8"
      >
        <ArrowLeft size={14} />
        Back to companies
      </button>

      {pageErr && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          {pageErr}
        </div>
      )}

      {/* ── Company header ─────────────────────── */}
      <div className="rounded-xl p-6 mb-6 border border-(--border) bg-(--bg-surface)">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-4">
            {/* Avatar */}
            <div
              className="w-14 h-14 rounded-2xl shrink-0 flex items-center justify-center text-xl font-bold text-white"
              style={{
                background: "linear-gradient(135deg, #7c3aed, #a855f7)",
              }}
            >
              {company.name[0].toUpperCase()}
            </div>

            {/* Info */}
            <div>
              <h1
                className="text-xl font-bold text-(--text-primary) mb-0.5"
                style={{
                  fontFamily: "var(--font-syne, Syne, sans-serif)",
                  letterSpacing: "-0.02em",
                }}
              >
                {company.name}
              </h1>
              {company.industry && (
                <p className="text-sm text-(--text-muted) mb-3">
                  {company.industry}
                </p>
              )}

              {/* Meta details */}
              <div className="flex flex-wrap gap-4">
                {company.website && (
                  <a
                    href={company.website}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-1.5 text-xs text-purple-400 hover:underline"
                  >
                    <Globe size={12} />
                    {company.website.replace(/^https?:\/\//, "")}
                    <ExternalLink size={9} />
                  </a>
                )}
                {company.phone && (
                  <span className="flex items-center gap-1.5 text-xs text-(--text-muted)">
                    <Phone size={12} />
                    {company.phone}
                  </span>
                )}
                {company.domain && (
                  <span className="flex items-center gap-1.5 text-xs text-(--text-muted)">
                    <Building2 size={12} />
                    {company.domain}
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Edit / Delete / Enrich */}
          <div className="flex items-center gap-2 shrink-0">
            {canUpdateCompany && (
              <>
                <button
                  onClick={handleEnrich}
                  disabled={enriching || !company.domain}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-white bg-purple-600 hover:bg-purple-700 disabled:opacity-50 transition-colors"
                >
                  <Sparkles size={12} />
                  {enriching ? "Enriching..." : "Enrich"}
                </button>
                <button
                  onClick={openEditCompany}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-(--text-secondary) border border-(--border) hover:bg-(--bg-elevated) transition-colors"
                >
                  <Pencil size={12} />
                  Edit
                </button>
              </>
            )}
            {canDeleteCompany && (
              <button
                onClick={handleDeleteCompany}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-red-400 border border-red-500/30 hover:bg-red-500/10 transition-colors"
              >
                <Trash2 size={12} />
                Delete
              </button>
            )}
          </div>
        </div>
      </div>

      {/* ── Contacts section ───────────────────── */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2
            className="text-base font-semibold text-(--text-primary)"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            Contacts
            <span className="ml-2 text-sm font-normal text-(--text-muted)">
              ({contacts.length})
            </span>
          </h2>
          {canCreateContact && (
            <button
              onClick={openAddContact}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
            >
              <Plus size={13} />
              Add contact
            </button>
          )}
        </div>

        {contacts.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 rounded-xl border border-dashed border-(--border) text-center">
            <p className="text-sm text-(--text-muted) mb-3">
              No contacts linked to this company yet
            </p>
            {canCreateContact && (
              <button
                onClick={openAddContact}
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
              >
                <Plus size={14} />
                Add first contact
              </button>
            )}
          </div>
        ) : (
          <div
            ref={contactListRef}
            className="rounded-xl border border-(--border) overflow-hidden divide-y divide-(--border)"
          >
            {contacts.map((contact) => {
              const fullName = [contact.first_name, contact.last_name]
                .filter(Boolean)
                .join(" ");
              const confirming = deleteContactId === contact.id;

              return (
                <div
                  key={contact.id}
                  className="contact-row flex items-center gap-4 px-4 py-3.5 bg-(--bg-surface) hover:bg-(--bg-elevated) transition-colors group"
                >
                  <ContactAvatar name={contact.first_name} />

                  <div className="flex-1 min-w-0">
                    <p
                      className="text-sm font-medium text-(--text-primary) truncate"
                      style={{
                        fontFamily: "var(--font-inter, Inter, sans-serif)",
                      }}
                    >
                      {fullName}
                    </p>
                    <p className="text-xs text-(--text-muted) truncate">
                      {contact.title ?? contact.email ?? ""}
                    </p>
                  </div>

                  {contact.email && (
                    <div className="flex items-center gap-1.5 text-xs text-(--text-muted) hidden sm:flex">
                      <Mail size={11} />
                      {contact.email}
                    </div>
                  )}

                  {contact.title && (
                    <div className="flex items-center gap-1.5 text-xs text-(--text-muted) hidden md:flex">
                      <Briefcase size={11} />
                      {contact.title}
                    </div>
                  )}

                  {confirming ? (
                    <div className="flex items-center gap-2 shrink-0">
                      <span className="text-xs text-(--text-muted)">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDeleteContact(contact.id)}
                        className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setDeleteContactId(null)}
                        className="px-2.5 py-1 rounded-md text-xs text-(--text-secondary) hover:bg-(--bg-elevated) transition-colors"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                      {canUpdateContact && (
                        <button
                          onClick={() => openEditContact(contact)}
                          className="p-1.5 rounded-md text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-colors"
                          title="Edit contact"
                        >
                          <Pencil size={13} />
                        </button>
                      )}
                      {canDeleteContact && (
                        <button
                          onClick={() => setDeleteContactId(contact.id)}
                          className="p-1.5 rounded-md text-(--text-muted) hover:text-red-400 hover:bg-red-500/10 transition-colors"
                          title="Delete contact"
                        >
                          <Trash2 size={13} />
                        </button>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
