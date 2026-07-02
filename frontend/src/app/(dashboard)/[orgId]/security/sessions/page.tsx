// src/app/(dashboard)/[orgId]/security/sessions/page.tsx
"use client";

import { use, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Monitor,
  Smartphone,
  Globe,
  Clock,
  Shield,
  Trash2,
  Loader2,
  CheckCircle2,
  XCircle,
} from "lucide-react";
import { usePermissionStore } from "@/stores/permissionStore";
import { listSessions, revokeSession, listLoginEvents } from "@/lib/security";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";
import type { Session } from "@/lib/security";

// ── Helpers ───────────────────────────────────────────────────────────────
function parseBrowser(ua: string) {
  if (ua.includes("PostmanRuntime")) return "Postman";
  if (ua.includes("Edg/")) return "Edge";
  if (ua.includes("Chrome/")) return "Chrome";
  if (ua.includes("Firefox/")) return "Firefox";
  if (ua.includes("Safari/") && !ua.includes("Chrome")) return "Safari";
  return "Browser";
}

function parseOS(ua: string) {
  if (ua.includes("Windows")) return "Windows";
  if (ua.includes("Macintosh")) return "macOS";
  if (ua.includes("iPhone") || ua.includes("iPad")) return "iOS";
  if (ua.includes("Android")) return "Android";
  if (ua.includes("Linux")) return "Linux";
  return "Unknown OS";
}

function isMobileUA(ua: string) {
  return ua.includes("iPhone") || ua.includes("Android") || ua.includes("iPad");
}

function formatIP(ip: string) {
  return ip.replace(/\/\d+$/, "");
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function timeAgo(iso: string) {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  const hours = Math.floor(diff / 3_600_000);
  const days = Math.floor(diff / 86_400_000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  if (hours < 24) return `${hours}h ago`;
  return `${days}d ago`;
}

function formatProvider(p: string) {
  return (
    { credentials: "Email / Password", google: "Google", github: "GitHub" }[
      p
    ] ?? p
  );
}

// ── Session row ───────────────────────────────────────────────────────────
function SessionRow({
  session,
  onRevoke,
  revoking,
  canRevoke,
}: {
  session: Session;
  onRevoke: () => void;
  revoking: boolean;
  canRevoke: boolean;
}) {
  const browser = parseBrowser(session.userAgent);
  const os = parseOS(session.userAgent);
  const mobile = isMobileUA(session.userAgent);
  const Icon = mobile ? Smartphone : Monitor;

  return (
    <div
      className={`flex items-center gap-4 px-5 py-4 transition-colors ${session.isActive ? "hover:bg-[var(--bg-elevated)]" : "opacity-50"}`}
    >
      <div className="w-9 h-9 rounded-lg flex-shrink-0 flex items-center justify-center bg-[var(--bg-elevated)] border border-[var(--border)]">
        <Icon size={16} className="text-[var(--text-muted)]" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-0.5">
          <p
            className="text-sm font-medium text-[var(--text-primary)]"
            style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
          >
            {browser} on {os}
          </p>
          {session.isActive ? (
            <span className="text-[0.6rem] font-semibold text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-1.5 py-0.5 rounded-full">
              Active
            </span>
          ) : (
            <span className="text-[0.6rem] font-semibold text-zinc-500 bg-zinc-500/10 border border-zinc-500/20 px-1.5 py-0.5 rounded-full">
              Revoked
            </span>
          )}
        </div>
        <p className="text-xs text-[var(--text-muted)]">
          {formatIP(session.ipAddress)}
          <span className="mx-1.5 text-[var(--border)]">·</span>
          Last active {timeAgo(session.lastActivityAt)}
          <span className="mx-1.5 text-[var(--border)]">·</span>
          {session.isActive
            ? `Expires ${formatDate(session.expiresAt)}`
            : `Revoked ${formatDate(session.revokedAt!)}`}
        </p>
      </div>
      {session.isActive && canRevoke && (
        <button
          onClick={onRevoke}
          disabled={revoking}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium text-red-400 border border-red-500/25 hover:bg-red-500/10 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex-shrink-0"
        >
          {revoking ? (
            <Loader2 size={11} className="animate-spin" />
          ) : (
            <Trash2 size={11} />
          )}
          Revoke
        </button>
      )}
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────
export default function SecurityPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();

  type Tab = "sessions" | "events";
  const [activeTab, setActiveTab] = useState<Tab>("sessions");
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const canView = hasPermission("security.sessions.view");
  const canRevoke = hasPermission("security.sessions.revoke");

  // ── Queries ───────────────────────────────────────────────────────────────
  // enabled: canView gates both queries — avoids a 403 round-trip for users
  // without permission (the original code always fetched regardless of permission)
  const sessionsKey = queryKeys.security.sessions(orgId);
  const eventsKey = queryKeys.security.loginEvents(orgId);

  const sessionsQuery = useQuery({
    queryKey: sessionsKey,
    queryFn: () => listSessions(orgId),
    enabled: canView,
  });
  const eventsQuery = useQuery({
    queryKey: eventsKey,
    queryFn: () => listLoginEvents(orgId),
    enabled: canView,
  });

  const sessions = sessionsQuery.data ?? [];
  const events = eventsQuery.data ?? [];
  const activeSessions = sessions.filter((s) => s.isActive);
  const revokedSessions = sessions.filter((s) => !s.isActive);

  // ── Handlers ──────────────────────────────────────────────────────────────
  const handleRevoke = async (sessionId: string) => {
    setRevokingId(sessionId);
    toast.error(null);
    try {
      await revokeSession(orgId, sessionId);
      queryClient.setQueryData<Session[]>(sessionsKey, (old) =>
        (old ?? []).map((s) =>
          s.id === sessionId
            ? { ...s, isActive: false, revokedAt: new Date().toISOString() }
            : s,
        ),
      );
      toast.success("Session revoked.");
    } catch {
      toast.error("Failed to revoke session.");
    } finally {
      setRevokingId(null);
    }
  };

  const isLoading = sessionsQuery.isPending || eventsQuery.isPending;

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div className="p-6 md:p-8 max-w-4xl">
      <div className="mb-8">
        <h1
          className="text-2xl font-bold text-[var(--text-primary)] mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          Security
        </h1>
        <p className="text-sm text-[var(--text-muted)]">
          Manage active sessions and review login history
        </p>
      </div>

      {!canView ? (
        <div className="flex items-center gap-3 p-4 rounded-xl bg-[var(--bg-elevated)] border border-[var(--border)]">
          <Shield size={16} className="text-[var(--text-muted)]" />
          <p className="text-sm text-[var(--text-muted)]">
            You do not have permission to view security settings.
          </p>
        </div>
      ) : (
        <>
          {(sessionsQuery.isError || eventsQuery.isError) && (
            <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
              Failed to load security data.
            </div>
          )}

          {/* Tabs */}
          <div className="flex items-center gap-0.5 border-b border-[var(--border)] mb-6">
            {[
              {
                key: "sessions" as Tab,
                label: "Sessions",
                count: activeSessions.length,
              },
              {
                key: "events" as Tab,
                label: "Login history",
                count: events.length,
              },
            ].map((tab) => {
              const active = activeTab === tab.key;
              return (
                <button
                  key={tab.key}
                  onClick={() => setActiveTab(tab.key)}
                  className={`flex items-center gap-2 px-3.5 py-2.5 text-sm font-medium -mb-px border-b-2 transition-colors ${
                    active
                      ? "text-purple-400 border-purple-500"
                      : "text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]"
                  }`}
                >
                  {tab.label}
                  {tab.count > 0 && !isLoading && (
                    <span
                      className={`text-xs px-1.5 py-0.5 rounded-full ${active ? "bg-purple-500/15 text-purple-400" : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"}`}
                    >
                      {tab.count}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          {isLoading ? (
            <div className="flex items-center gap-3 py-16 text-sm text-[var(--text-muted)]">
              <Loader2 size={15} className="animate-spin text-purple-500" />
              Loading…
            </div>
          ) : activeTab === "sessions" ? (
            <div className="space-y-6">
              <div>
                <p
                  className="text-[0.65rem] font-semibold text-[var(--text-muted)] uppercase tracking-widest mb-3"
                  style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
                >
                  Active sessions ({activeSessions.length})
                </p>
                <div className="rounded-xl border border-[var(--border)] overflow-hidden divide-y divide-[var(--border)] bg-[var(--bg-surface)]">
                  {activeSessions.length === 0 ? (
                    <p className="px-5 py-8 text-sm text-center text-[var(--text-muted)]">
                      No active sessions
                    </p>
                  ) : (
                    activeSessions
                      .sort(
                        (a, b) =>
                          new Date(b.lastActivityAt).getTime() -
                          new Date(a.lastActivityAt).getTime(),
                      )
                      .map((s) => (
                        <SessionRow
                          key={s.id}
                          session={s}
                          onRevoke={() => handleRevoke(s.id)}
                          revoking={revokingId === s.id}
                          canRevoke={canRevoke}
                        />
                      ))
                  )}
                </div>
              </div>

              {revokedSessions.length > 0 && (
                <div>
                  <p
                    className="text-[0.65rem] font-semibold text-[var(--text-muted)] uppercase tracking-widest mb-3"
                    style={{
                      fontFamily: "var(--font-inter, Inter, sans-serif)",
                    }}
                  >
                    Revoked sessions ({revokedSessions.length})
                  </p>
                  <div className="rounded-xl border border-[var(--border)] overflow-hidden divide-y divide-[var(--border)] bg-[var(--bg-surface)]">
                    {revokedSessions.map((s) => (
                      <SessionRow
                        key={s.id}
                        session={s}
                        onRevoke={() => {}}
                        revoking={false}
                        canRevoke={false}
                      />
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div>
              <div className="rounded-xl border border-[var(--border)] overflow-hidden bg-[var(--bg-surface)]">
                <div className="grid grid-cols-[1fr_auto_auto_auto] gap-4 px-5 py-3 border-b border-[var(--border)] bg-[var(--bg-elevated)]">
                  {["Time", "IP address", "Method", "Status"].map((h) => (
                    <span
                      key={h}
                      className="text-[0.65rem] font-semibold text-[var(--text-muted)] uppercase tracking-wider"
                      style={{
                        fontFamily: "var(--font-inter, Inter, sans-serif)",
                      }}
                    >
                      {h}
                    </span>
                  ))}
                </div>
                <div className="divide-y divide-[var(--border)]">
                  {events.length === 0 ? (
                    <p className="px-5 py-8 text-sm text-center text-[var(--text-muted)]">
                      No login events
                    </p>
                  ) : (
                    events.map((ev) => (
                      <div
                        key={ev.id}
                        className="grid grid-cols-[1fr_auto_auto_auto] gap-4 items-center px-5 py-3.5 hover:bg-[var(--bg-elevated)] transition-colors"
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          <Clock
                            size={13}
                            className="text-[var(--text-muted)] flex-shrink-0"
                          />
                          <span className="text-sm text-[var(--text-secondary)]">
                            {formatDate(ev.createdAt)}
                          </span>
                        </div>
                        <div className="flex items-center gap-2">
                          <Globe
                            size={12}
                            className="text-[var(--text-muted)] flex-shrink-0"
                          />
                          <span className="text-xs text-[var(--text-muted)] font-mono">
                            {formatIP(ev.ipAddress)}
                          </span>
                        </div>
                        <span className="text-xs text-[var(--text-secondary)] whitespace-nowrap">
                          {formatProvider(ev.provider)}
                        </span>
                        <div className="flex items-center justify-end">
                          {ev.status === "success" ? (
                            <span className="flex items-center gap-1 text-[0.65rem] font-semibold text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 rounded-full">
                              <CheckCircle2 size={9} />
                              Success
                            </span>
                          ) : (
                            <span className="flex items-center gap-1 text-[0.65rem] font-semibold text-red-400 bg-red-500/10 border border-red-500/20 px-2 py-0.5 rounded-full">
                              <XCircle size={9} />
                              Failed
                            </span>
                          )}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
