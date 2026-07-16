"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  Key,
  Copy,
  Trash2,
  Check,
  Loader2,
  AlertCircle,
  Eye,
  EyeOff,
} from "lucide-react";
import { toast } from "sonner";

import { useDrawer } from "@/contexts/DrawerContext";
import { usePermissionStore } from "@/stores/permissionStore";
import {
  listAPIKeys,
  createAPIKey,
  revokeAPIKey,
  OrgAPIKey,
} from "@/lib/apikeys";

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      onClick={() => {
        navigator.clipboard.writeText(text);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }}
      className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors"
      title="Copy to clipboard"
    >
      {copied ? (
        <Check size={14} className="text-green-500" />
      ) : (
        <Copy size={14} />
      )}
    </button>
  );
}

function CreateKeyForm({
  orgId,
  onSave,
}: {
  orgId: string;
  onSave: () => void;
}) {
  const { closeDrawer } = useDrawer();
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState<string[]>(["capture:leads"]);
  const [allowedOriginsString, setAllowedOriginsString] = useState("");
  const [loading, setLoading] = useState(false);
  const [newKey, setNewKey] = useState<string | null>(null);
  const [showKey, setShowKey] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const parsedOrigins = allowedOriginsString
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);

      const res = await createAPIKey(orgId, {
        name,
        scopes,
        allowed_origins: parsedOrigins,
      });
      setNewKey(res.raw_key);
      toast.success("API Key created successfully.");
      onSave(); // Refetches list behind modal
    } catch (err) {
      toast.error("Failed to create API key");
    } finally {
      setLoading(false);
    }
  };

  if (newKey) {
    return (
      <div className="p-6">
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="w-12 h-12 rounded-full bg-green-500/10 text-green-500 flex items-center justify-center mb-4">
            <Check size={24} />
          </div>
          <h3 className="text-lg font-bold text-[var(--text-primary)] mb-2">
            Your new API key
          </h3>
          <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-left w-full mt-2">
            <p className="text-sm font-semibold text-red-400 mb-1 flex items-center gap-1.5">
              <AlertCircle size={15} />
              Important Security Notice
            </p>
            <p className="text-xs text-red-400/90 leading-relaxed">
              Please copy this full API key now and store it securely.
              <strong>
                {" "}
                You will not be able to see or copy the full key again once you
                close this window.
              </strong>
              For security, the API key list will only display a short prefix.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 p-3 bg-[var(--bg-elevated)] border border-[var(--border)] rounded-lg mb-6">
          <code className="flex-1 text-sm font-mono text-[var(--text-primary)] break-all">
            {showKey ? newKey : "•".repeat(32)}
          </code>
          <button
            type="button"
            onClick={() => setShowKey(!showKey)}
            className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors"
            title={showKey ? "Hide API key" : "Show API key"}
          >
            {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
          </button>
          <CopyButton text={newKey} />
        </div>

        <div className="flex justify-end">
          <button
            onClick={() => closeDrawer()}
            className="px-4 py-2 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            Done
          </button>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="p-6 space-y-6">
      <div>
        <label className="block text-sm font-medium text-[var(--text-primary)] mb-1.5">
          Key Name
        </label>
        <input
          required
          type="text"
          placeholder="e.g. Production Web Forms"
          className="w-full px-3 py-2 bg-transparent border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-purple-500"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-[var(--text-primary)] mb-1.5">
          Capabilities (Scopes)
        </label>
        <div className="space-y-2">
          <label className="flex items-center gap-3 p-3 border border-[var(--border)] rounded-lg cursor-pointer hover:bg-[var(--bg-elevated)] transition-colors">
            <input
              type="checkbox"
              checked={scopes.includes("capture:leads")}
              onChange={(e) => {
                if (e.target.checked) setScopes((s) => [...s, "capture:leads"]);
                else setScopes((s) => s.filter((x) => x !== "capture:leads"));
              }}
              className="w-4 h-4 text-purple-600 border-gray-300 rounded focus:ring-purple-500"
            />
            <div>
              <p className="text-sm font-medium text-[var(--text-primary)]">
                Lead Capture
              </p>
              <p className="text-xs text-[var(--text-muted)]">
                Allow creating leads via the public API
              </p>
            </div>
          </label>
          <label className="flex items-center gap-3 p-3 border border-[var(--border)] rounded-lg cursor-pointer hover:bg-[var(--bg-elevated)] transition-colors">
            <input
              type="checkbox"
              checked={scopes.includes("capture:visitors")}
              onChange={(e) => {
                if (e.target.checked)
                  setScopes((s) => [...s, "capture:visitors"]);
                else
                  setScopes((s) => s.filter((x) => x !== "capture:visitors"));
              }}
              className="w-4 h-4 text-purple-600 border-gray-300 rounded focus:ring-purple-500"
            />
            <div>
              <p className="text-sm font-medium text-[var(--text-primary)]">
                Website Visitors
              </p>
              <p className="text-xs text-[var(--text-muted)]">
                Allow tracking and identifying website visitors
              </p>
            </div>
          </label>
        </div>
      </div>

      <div>
        <label className="flex items-center justify-between text-sm font-medium text-[var(--text-primary)] mb-1.5">
          Allowed Domains
          <span className="text-xs text-[var(--text-muted)] font-normal">
            Optional
          </span>
        </label>
        <p className="text-xs text-[var(--text-muted)] mb-2">
          Restrict this API key to specific websites (e.g.{" "}
          <code>https://mywebsite.com</code>). Separate multiple domains with
          commas. Leave empty to allow any origin.
        </p>
        <input
          type="text"
          placeholder="https://example.com, https://test.com"
          className="w-full px-3 py-2 bg-transparent border border-[var(--border)] rounded-lg text-sm text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-purple-500"
          value={allowedOriginsString}
          onChange={(e) => setAllowedOriginsString(e.target.value)}
        />
      </div>

      <div className="flex items-center justify-end gap-3 pt-4 border-t border-[var(--border)]">
        <button
          type="button"
          onClick={() => closeDrawer()}
          className="px-4 py-2 rounded-lg text-sm font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={loading || !name.trim() || scopes.length === 0}
          className="px-4 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 transition-colors flex items-center gap-2"
        >
          {loading ? <Loader2 size={16} className="animate-spin" /> : null}
          Generate Key
        </button>
      </div>
    </form>
  );
}

function KeyPrefixDisplay({ prefix }: { prefix: string }) {
  const [show, setShow] = useState(false);
  const displayKey = `${prefix}...`;

  return (
    <div className="flex items-center gap-0.5">
      <code className="text-[0.7rem] px-2 py-0.5 rounded bg-[var(--bg-elevated)] text-[var(--text-secondary)] font-mono border border-[var(--border)] min-w-[140px] text-center mr-1">
        {show ? displayKey : "••••••••••••••••••••"}
      </code>
      <button
        type="button"
        onClick={() => setShow(!show)}
        className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors flex-shrink-0"
        title={show ? "Hide key prefix" : "Show key prefix"}
      >
        {show ? <EyeOff size={14} /> : <Eye size={14} />}
      </button>
      <CopyButton text={prefix} />
    </div>
  );
}

export default function APIKeysPage() {
  const params = useParams();
  const orgId = params.orgId as string;
  const queryClient = useQueryClient();
  const { openDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();

  const [delConfirm, setDelConfirm] = useState<string | null>(null);

  // Assuming you need settings.view or settings.edit, we'll just check if they can view for now.
  const canEdit = hasPermission("settings.view"); // Assuming general settings perm

  const keysQuery = useQuery({
    queryKey: ["apikeys", orgId],
    queryFn: () => listAPIKeys(orgId),
  });

  const keys = keysQuery.data ?? [];

  const handleCreate = () => {
    openDrawer({
      title: "Create API Key",
      width: "md",
      content: (
        <CreateKeyForm
          orgId={orgId}
          onSave={() =>
            queryClient.invalidateQueries({ queryKey: ["apikeys", orgId] })
          }
        />
      ),
    });
  };

  const handleRevoke = async (keyId: string) => {
    toast.dismiss();
    try {
      await revokeAPIKey(orgId, keyId);
      queryClient.setQueryData<OrgAPIKey[]>(["apikeys", orgId], (old) =>
        (old ?? []).map((k) =>
          k.id === keyId ? { ...k, is_active: false } : k,
        ),
      );
      toast.success("API Key revoked.");
    } catch {
      toast.error("Failed to revoke API key.");
    }
    setDelConfirm(null);
  };

  return (
    <div className="p-6 md:p-8 max-w-4xl">
      <div className="flex items-start justify-between mb-8">
        <div>
          <h1
            className="text-2xl font-bold text-[var(--text-primary)] mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            API Keys
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Manage keys used to integrate external forms and applications.
          </p>
        </div>
        {canEdit && (
          <button
            onClick={handleCreate}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            Create key
          </button>
        )}
      </div>

      {keysQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          Failed to load API keys. Please refresh.
        </div>
      )}

      {keysQuery.isPending ? (
        <div className="flex items-center gap-3 py-16 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading API keys…
        </div>
      ) : keys.length === 0 ? (
        <div className="py-16 text-center border border-dashed border-[var(--border)] rounded-xl bg-[var(--bg-surface)]">
          <Key
            size={32}
            className="mx-auto text-[var(--text-muted)] mb-3 opacity-50"
          />
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-1">
            No API keys
          </h3>
          <p className="text-xs text-[var(--text-muted)] max-w-[250px] mx-auto mb-4">
            Create an API key to allow external forms and services to capture
            leads in your CRM.
          </p>
          {canEdit && (
            <button
              onClick={handleCreate}
              className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold text-[var(--text-primary)] bg-[var(--bg-elevated)] border border-[var(--border)] hover:border-[var(--text-muted)] transition-colors"
            >
              <Plus size={13} />
              Create key
            </button>
          )}
        </div>
      ) : (
        <div className="space-y-2.5">
          {keys.map((k) => {
            const confirming = delConfirm === k.id;
            return (
              <div
                key={k.id}
                className={`rounded-xl border bg-[var(--bg-surface)] transition-all duration-150 ${
                  k.is_active
                    ? "border-[var(--border)] hover:border-[var(--text-muted)]/30"
                    : "border-red-500/10 opacity-70"
                }`}
              >
                <div className="flex items-center gap-4 px-5 py-4">
                  <div
                    className={`w-9 h-9 rounded-lg flex-shrink-0 flex items-center justify-center border ${
                      k.is_active
                        ? "bg-[var(--bg-elevated)] border-[var(--border)]"
                        : "bg-red-500/5 border-red-500/10"
                    }`}
                  >
                    {k.is_active ? (
                      <Key size={15} className="text-[var(--text-muted)]" />
                    ) : (
                      <AlertCircle size={15} className="text-red-400" />
                    )}
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2.5 mb-1">
                      <span
                        className="text-sm font-semibold text-[var(--text-primary)]"
                        style={{
                          fontFamily: "var(--font-inter, Inter, sans-serif)",
                        }}
                      >
                        {k.name}
                      </span>
                      {!k.is_active && (
                        <span className="text-[0.6rem] font-semibold border border-red-500/20 text-red-500 bg-red-500/10 px-1.5 py-0.5 rounded-full">
                          Revoked
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-3">
                      <KeyPrefixDisplay prefix={k.key_prefix} />
                      <span className="text-xs text-[var(--text-muted)] truncate hidden sm:block">
                        Scopes: {k.scopes.join(", ")}
                      </span>
                    </div>
                  </div>

                  <div className="text-right hidden md:block mr-4">
                    <p className="text-[0.65rem] text-[var(--text-muted)] uppercase tracking-wider mb-0.5">
                      Created
                    </p>
                    <p className="text-xs text-[var(--text-secondary)]">
                      {new Date(k.created_at).toLocaleDateString()}
                    </p>
                  </div>
                  <div className="text-right hidden md:block mr-4">
                    <p className="text-[0.65rem] text-[var(--text-muted)] uppercase tracking-wider mb-0.5">
                      Last Used
                    </p>
                    <p className="text-xs text-[var(--text-secondary)]">
                      {k.last_used_at
                        ? new Date(k.last_used_at).toLocaleDateString()
                        : "Never"}
                    </p>
                  </div>

                  {k.is_active && canEdit && (
                    <div className="flex items-center flex-shrink-0 border-l border-[var(--border)] pl-4 ml-2">
                      {confirming ? (
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-[var(--text-muted)] mr-1">
                            Revoke?
                          </span>
                          <button
                            onClick={() => handleRevoke(k.id)}
                            className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                          >
                            Yes
                          </button>
                          <button
                            onClick={() => setDelConfirm(null)}
                            className="px-2.5 py-1 rounded-md text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                          >
                            No
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setDelConfirm(k.id)}
                          title="Revoke key"
                          className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-red-400 hover:bg-red-500/10 transition-colors"
                        >
                          <Trash2 size={14} />
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
