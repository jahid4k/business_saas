"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Mail, Trash2, Loader2, Share2 } from "lucide-react";
import { toast } from "sonner";

import { useDrawer } from "@/contexts/DrawerContext";
import { usePermissionStore } from "@/stores/permissionStore";
import {
  listOrgEmails,
  createOrgEmail,
  deleteOrgEmail,
  listOrgSocials,
  deleteOrgSocial,
  OrgInboundEmail,
  SocialIntegration,
} from "@/lib/integrations";

function CreateEmailForm({
  orgId,
  onSave,
}: {
  orgId: string;
  onSave: () => void;
}) {
  const { closeDrawer } = useDrawer();
  const [address, setAddress] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await createOrgEmail(orgId, address);
      toast.success("Email address configured.");
      onSave();
      closeDrawer();
    } catch {
      toast.error("Failed to configure email address");
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="p-6 space-y-6">
      <div className="mb-2">
        <p className="text-sm text-(--text-muted)">
          Any emails forwarded to this address will automatically create a Lead
          in your CRM.
        </p>
      </div>

      <div>
        <label className="block text-sm font-medium text-(--text-primary) mb-1.5">
          Inbound Email Address
        </label>
        <input
          required
          type="email"
          placeholder="e.g. leads@yourcompany.com"
          className="w-full px-3 py-2 bg-transparent border border-(--border) rounded-lg text-sm text-(--text-primary) placeholder-(--text-muted) focus:outline-none focus:border-purple-500"
          value={address}
          onChange={(e) => setAddress(e.target.value)}
        />
      </div>

      <div className="flex items-center justify-end gap-3 pt-4 border-t border-(--border)">
        <button
          type="button"
          onClick={() => closeDrawer()}
          className="px-4 py-2 rounded-lg text-sm font-medium text-(--text-secondary) hover:bg-(--bg-elevated) transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={loading || !address.trim()}
          className="px-4 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 transition-colors flex items-center gap-2"
        >
          {loading && <Loader2 size={16} className="animate-spin" />}
          Add Address
        </button>
      </div>
    </form>
  );
}

function ConnectSocialModal({ orgId }: { orgId: string }) {
  const { closeDrawer } = useDrawer();

  const handleConnect = (platform: string) => {
    window.location.href = `/api/v1/pub/social/auth/${platform}?orgId=${orgId}`;
  };

  return (
    <div className="p-6 space-y-6">
      <div className="mb-4">
        <p className="text-sm text-(--text-muted)">
          Select a platform to connect. You will be redirected to authorize
          BusinessSAAS to access your pages and leads.
        </p>
      </div>

      <div className="space-y-3">
        <button
          onClick={() => handleConnect("facebook")}
          className="w-full flex items-center justify-between p-4 rounded-xl border border-(--border) bg-(--bg-surface) hover:border-blue-500/50 transition-colors"
        >
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-blue-500/10 text-blue-500 flex items-center justify-center">
              <Share2 size={18} />
            </div>
            <div className="text-left">
              <p className="text-sm font-semibold text-(--text-primary)">
                Facebook Lead Ads
              </p>
              <p className="text-xs text-(--text-muted)">
                Connect Meta Business Pages
              </p>
            </div>
          </div>
          <Plus size={16} className="text-(--text-muted)" />
        </button>

        <button
          onClick={() => handleConnect("linkedin")}
          className="w-full flex items-center justify-between p-4 rounded-xl border border-(--border) bg-(--bg-surface) hover:border-blue-600/50 transition-colors"
        >
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-blue-600/10 text-blue-600 flex items-center justify-center">
              <Share2 size={18} />
            </div>
            <div className="text-left">
              <p className="text-sm font-semibold text-(--text-primary)">
                LinkedIn Lead Gen Forms
              </p>
              <p className="text-xs text-(--text-muted)">
                Connect LinkedIn Company Pages
              </p>
            </div>
          </div>
          <Plus size={16} className="text-(--text-muted)" />
        </button>
      </div>

      <div className="flex justify-end pt-4 border-t border-(--border) mt-6">
        <button
          type="button"
          onClick={() => closeDrawer()}
          className="px-4 py-2 rounded-lg text-sm font-medium text-(--text-secondary) hover:bg-(--bg-elevated) transition-colors"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

export default function IntegrationsPage() {
  const params = useParams();
  const orgId = params.orgId as string;
  const queryClient = useQueryClient();
  const { openDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();
  const canEdit = hasPermission("settings.view"); // Assuming general settings perm

  const emailsQuery = useQuery({
    queryKey: ["integrations", "email", orgId],
    queryFn: () => listOrgEmails(orgId),
  });

  const socialsQuery = useQuery({
    queryKey: ["integrations", "social", orgId],
    queryFn: () => listOrgSocials(orgId),
  });

  const emails = emailsQuery.data ?? [];
  const socials = socialsQuery.data ?? [];

  const handleAddEmail = () => {
    openDrawer({
      title: "Add Inbound Email",
      width: "md",
      content: (
        <CreateEmailForm
          orgId={orgId}
          onSave={() =>
            queryClient.invalidateQueries({
              queryKey: ["integrations", "email", orgId],
            })
          }
        />
      ),
    });
  };

  const handleDeleteEmail = async (id: string) => {
    try {
      await deleteOrgEmail(orgId, id);
      queryClient.setQueryData<OrgInboundEmail[]>(
        ["integrations", "email", orgId],
        (old) => (old ?? []).filter((e) => e.id !== id),
      );
      toast.success("Email address removed.");
    } catch {
      toast.error("Failed to remove email address.");
    }
  };

  const handleAddSocial = () => {
    openDrawer({
      title: "Connect Social Page",
      width: "md",
      content: <ConnectSocialModal orgId={orgId} />,
    });
  };

  const handleDeleteSocial = async (id: string) => {
    try {
      await deleteOrgSocial(orgId, id);
      queryClient.setQueryData<SocialIntegration[]>(
        ["integrations", "social", orgId],
        (old) => (old ?? []).filter((s) => s.id !== id),
      );
      toast.success("Social integration removed.");
    } catch {
      toast.error("Failed to remove social integration.");
    }
  };

  return (
    <div className="p-6 md:p-8 max-w-4xl space-y-12">
      <div>
        <h1
          className="text-2xl font-bold text-[(--text-primary)] mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          Integrations
        </h1>
        <p className="text-sm text-[(--text-muted)]">
          Manage omnichannel capture settings for email and social media
          platforms.
        </p>
      </div>

      {/* --- EMAIL SECTION --- */}
      <section>
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold text-[(--text-primary)] mb-1">
              Inbound Email Parsing
            </h2>
            <p className="text-sm text-[(--text-muted)]">
              Automatically create leads when emails are sent to these
              addresses.
            </p>
          </div>
          {canEdit && (
            <button
              onClick={handleAddEmail}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-semibold text-[(--text-primary)] bg-[(--bg-elevated)] border border-[(--border)] hover:border-[(--text-muted)] transition-colors"
            >
              <Plus size={15} />
              Add Address
            </button>
          )}
        </div>

        {emailsQuery.isPending ? (
          <div className="py-8 flex items-center justify-center text-sm text-[(--text-muted)]">
            <Loader2 size={15} className="animate-spin mr-2" /> Loading...
          </div>
        ) : emails.length === 0 ? (
          <div className="py-10 text-center border border-dashed border-[(--border)] rounded-xl bg-[(--bg-surface)]">
            <Mail
              size={30}
              className="mx-auto text-[(--text-muted)] mb-3 opacity-50"
            />
            <p className="text-sm font-semibold text-[(--text-primary)]">
              No emails configured
            </p>
            <p className="text-xs text-[(--text-muted)] mt-1">
              Add an email address to start capturing leads from your inbox.
            </p>
          </div>
        ) : (
          <div className="space-y-2.5">
            {emails.map((e) => (
              <div
                key={e.id}
                className="flex items-center justify-between p-4 rounded-xl border border-[(--border)] bg-[(--bg-surface)] hover:border-[(--text-muted)]/30 transition-colors"
              >
                <div className="flex items-center gap-4">
                  <div className="w-9 h-9 rounded-lg flex items-center justify-center bg-[(--bg-elevated)] border border-[(--border)]">
                    <Mail size={15} className="text-[(--text-muted)]" />
                  </div>
                  <div>
                    <p className="text-sm font-medium text-[(--text-primary)]">
                      {e.address}
                    </p>
                    <p className="text-xs text-[(--text-muted)]">
                      Active since {new Date(e.created_at).toLocaleDateString()}
                    </p>
                  </div>
                </div>
                {canEdit && (
                  <button
                    onClick={() => handleDeleteEmail(e.id)}
                    className="p-1.5 text-[(--text-muted)] hover:text-red-400 hover:bg-red-500/10 rounded-md transition-colors"
                  >
                    <Trash2 size={14} />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      {/* --- SOCIAL SECTION --- */}
      <section>
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold text-[(--text-primary)] mb-1">
              Social Lead Ads
            </h2>
            <p className="text-sm text-[(--text-muted)]">
              Map Facebook and LinkedIn lead ad campaigns to this CRM.
            </p>
          </div>
          {canEdit && (
            <button
              onClick={handleAddSocial}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-semibold text-[(--text-primary)] bg-[(--bg-elevated)] border border-[(--border)] hover:border-[(--text-muted)] transition-colors"
            >
              <Plus size={15} />
              Connect Page
            </button>
          )}
        </div>

        {socialsQuery.isPending ? (
          <div className="py-8 flex items-center justify-center text-sm text-[(--text-muted)]">
            <Loader2 size={15} className="animate-spin mr-2" /> Loading...
          </div>
        ) : socials.length === 0 ? (
          <div className="py-10 text-center border border-dashed border-[(--border)] rounded-xl bg-[(--bg-surface)]">
            <Share2
              size={30}
              className="mx-auto text-[(--text-muted)] mb-3 opacity-50"
            />
            <p className="text-sm font-semibold text-[(--text-primary)]">
              No social accounts connected
            </p>
            <p className="text-xs text-[(--text-muted)] mt-1">
              Connect a page to start syncing forms automatically.
            </p>
          </div>
        ) : (
          <div className="space-y-2.5">
            {socials.map((s) => (
              <div
                key={s.id}
                className="flex items-center justify-between p-4 rounded-xl border border-[(--border)] bg-[(--bg-surface)] hover:border-[(--text-muted)]/30 transition-colors"
              >
                <div className="flex items-center gap-4">
                  <div
                    className={`w-9 h-9 rounded-lg flex items-center justify-center border ${s.platform === "facebook" ? "bg-blue-500/10 border-blue-500/20 text-blue-500" : "bg-blue-600/10 border-blue-600/20 text-blue-600"}`}
                  >
                    <Share2 size={16} />
                  </div>
                  <div>
                    <p className="text-sm font-medium text-[(--text-primary)] capitalize">
                      {s.platform} Page
                    </p>
                    <div className="flex items-center gap-2">
                      <code className="text-xs text-[(--text-muted)]">
                        ID: {s.page_id}
                      </code>
                      <span className="text-[10px] uppercase tracking-wider font-semibold text-green-500 bg-green-500/10 px-1.5 py-0.5 rounded">
                        Connected
                      </span>
                    </div>
                  </div>
                </div>
                {canEdit && (
                  <button
                    onClick={() => handleDeleteSocial(s.id)}
                    className="p-1.5 text-[(--text-muted)] hover:text-red-400 hover:bg-red-500/10 rounded-md transition-colors"
                  >
                    <Trash2 size={14} />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
