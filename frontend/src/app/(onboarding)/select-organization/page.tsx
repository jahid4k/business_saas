// src/app/(onboarding)/select-organization/page.tsx
"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Building2, ChevronRight, Plus, LogOut } from "lucide-react";
import gsap from "gsap";

import {
  listOrganizations,
  switchOrganization,
  getMyMembership,
} from "@/lib/org";
import { setToken } from "@/lib/token";
import { logout } from "@/lib/auth";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";
import type { MembershipWithRole } from "@/types/org";

const PURPLE = "#7c3aed";
const PURPLE_HOVER = "#a855f7";
const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
const FONT_INTER = "var(--font-inter, Inter, sans-serif)";

function roleBadge(role: string) {
  const map: Record<string, { bg: string; color: string }> = {
    owner: { bg: "rgba(124,58,237,0.15)", color: "#a855f7" },
    admin: { bg: "rgba(59,130,246,0.12)", color: "#60a5fa" },
    manager: { bg: "rgba(20,184,166,0.12)", color: "#2dd4bf" },
    member: { bg: "rgba(34,197,94,0.12)", color: "#4ade80" },
    viewer: { bg: "rgba(156,163,175,0.1)", color: "#9ca3af" },
  };
  return map[role] ?? { bg: "rgba(156,163,175,0.1)", color: "#9ca3af" };
}

function initials(name: string) {
  return name
    .split(" ")
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

function timeGreeting() {
  const h = new Date().getHours();
  if (h < 12) return "morning";
  if (h < 17) return "afternoon";
  return "evening";
}

export default function SelectOrganizationPage() {
  const router = useRouter();
  const { user, setOrg, reset: resetAuth } = useAuthStore();
  const { setPermissions, reset: resetPerms } = usePermissionStore();

  const [orgs, setOrgs] = useState<MembershipWithRole[]>([]);
  const [fetching, setFetching] = useState(true);
  const [switching, setSwitching] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const wrapRef = useRef<HTMLDivElement>(null);
  const topRef = useRef<HTMLDivElement>(null);
  const headRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const footRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listOrganizations()
      .then(setOrgs)
      .catch(() => setError("Failed to load workspaces. Please refresh."))
      .finally(() => setFetching(false));
  }, []);

  // GSAP stagger entrance after data loads
  useEffect(() => {
    if (fetching) return;
    const targets = [
      topRef.current,
      headRef.current,
      listRef.current,
      footRef.current,
    ];
    const ctx = gsap.context(() => {
      gsap.set(targets, { opacity: 0, y: 18 });
      gsap.to(targets, {
        opacity: 1,
        y: 0,
        duration: 0.55,
        stagger: 0.07,
        ease: "power3.out",
      });
    }, wrapRef);
    return () => ctx.revert();
  }, [fetching]);

  const handleSelect = async (membership: MembershipWithRole) => {
    const org = membership.organization;
    setSwitching(org.id);
    setError(null);
    try {
      // 1. Switch org — get new JWT with business_id
      const switchData = await switchOrganization(org.id);
      setToken(switchData.access_token);

      // 2. Fetch permissions using the new token (which now has business_id)
      const myMembership = await getMyMembership();

      // 3. Hydrate stores
      setOrg(org, myMembership);
      setPermissions(myMembership.permissions);

      // 4. Enter dashboard
      router.push(`/${org.id}`);
    } catch {
      setError("Failed to switch workspace. Please try again.");
      setSwitching(null);
    }
  };

  const handleSignOut = async () => {
    try {
      await logout();
    } catch {
      /* ignore */
    }
    setToken(null);
    resetAuth();
    resetPerms();
    router.replace("/login");
  };

  const firstName = user?.firstName ?? user?.displayName ?? "there";

  if (fetching) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <span
          className="w-5 h-5 rounded-full animate-spin block"
          style={{
            border: "2px solid rgba(124,58,237,0.2)",
            borderTopColor: PURPLE,
          }}
        />
      </div>
    );
  }

  return (
    <div ref={wrapRef} className="min-h-screen py-14 px-4">
      <div className="max-w-[520px] mx-auto">
        {/* ── Top bar ─────────────────────────── */}
        <div ref={topRef} className="flex items-center justify-between mb-12">
          <div className="flex items-center gap-2">
            <div
              className="w-7 h-7 rounded-md flex items-center justify-center"
              style={{
                background: "linear-gradient(135deg, #7c3aed, #a855f7)",
              }}
            >
              <svg width="14" height="14" viewBox="0 0 18 18" fill="none">
                <rect x="2" y="2" width="6" height="6" rx="1.5" fill="white" />
                <rect
                  x="10"
                  y="2"
                  width="6"
                  height="6"
                  rx="1.5"
                  fill="white"
                  fillOpacity="0.5"
                />
                <rect
                  x="2"
                  y="10"
                  width="6"
                  height="6"
                  rx="1.5"
                  fill="white"
                  fillOpacity="0.5"
                />
                <rect
                  x="10"
                  y="10"
                  width="6"
                  height="6"
                  rx="1.5"
                  fill="white"
                />
              </svg>
            </div>
            <span
              className="text-sm font-semibold text-white"
              style={{ fontFamily: FONT_SYNE, letterSpacing: "-0.01em" }}
            >
              BusinessSAAS
            </span>
          </div>

          <button
            onClick={handleSignOut}
            className="flex items-center gap-1.5 text-xs rounded-md px-3 py-1.5 transition-all"
            style={{
              color: "#555",
              border: "1px solid rgba(255,255,255,0.07)",
              fontFamily: FONT_INTER,
              cursor: "pointer",
              background: "transparent",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = "#aaa";
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.14)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = "#555";
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.07)";
            }}
          >
            <LogOut size={12} />
            Sign out
          </button>
        </div>

        {/* ── Heading ─────────────────────────── */}
        <div ref={headRef} className="mb-8">
          <h1
            className="font-bold text-white mb-1"
            style={{
              fontFamily: FONT_SYNE,
              fontSize: "1.875rem",
              letterSpacing: "-0.025em",
              lineHeight: 1.15,
            }}
          >
            Good {timeGreeting()}, {firstName}
          </h1>
          <p
            className="text-sm"
            style={{ color: "#666", fontFamily: FONT_INTER }}
          >
            {orgs.length === 0
              ? "Create your first workspace to get started"
              : "Choose a workspace to continue"}
          </p>
        </div>

        {/* ── Error ───────────────────────────── */}
        {error && (
          <div
            className="rounded-lg px-4 py-3 text-sm mb-4"
            style={{
              background: "rgba(239,68,68,0.07)",
              border: "1px solid rgba(239,68,68,0.2)",
              color: "#f87171",
              fontFamily: FONT_INTER,
            }}
          >
            {error}
          </div>
        )}

        {/* ── Org list ────────────────────────── */}
        <div ref={listRef} className="space-y-2">
          {orgs.length === 0 ? (
            <div
              className="rounded-xl py-14 flex flex-col items-center gap-4"
              style={{ border: "1px dashed rgba(255,255,255,0.1)" }}
            >
              <div
                className="w-12 h-12 rounded-xl flex items-center justify-center"
                style={{
                  background: "rgba(124,58,237,0.1)",
                  border: "1px solid rgba(124,58,237,0.2)",
                }}
              >
                <Building2 size={22} style={{ color: PURPLE }} />
              </div>
              <div className="text-center">
                <p
                  className="text-sm font-medium text-white mb-1"
                  style={{ fontFamily: FONT_INTER }}
                >
                  No workspaces yet
                </p>
                <p
                  className="text-xs"
                  style={{ color: "#555", fontFamily: FONT_INTER }}
                >
                  Create your first workspace to start
                </p>
              </div>
              <Link
                href="/create-organization"
                className="flex items-center gap-2 rounded-lg px-5 py-2.5 text-sm font-semibold text-white transition-colors"
                style={{ background: PURPLE }}
                onMouseEnter={(e) =>
                  (e.currentTarget.style.background = PURPLE_HOVER)
                }
                onMouseLeave={(e) =>
                  (e.currentTarget.style.background = PURPLE)
                }
              >
                <Plus size={14} />
                Create workspace
              </Link>
            </div>
          ) : (
            orgs.map((m) => {
              const org = m.organization;
              const badge = roleBadge(m.role);
              const isThisLoading = switching === org.id;
              const anyLoading = switching !== null;

              return (
                <button
                  key={org.id}
                  onClick={() => !anyLoading && handleSelect(m)}
                  disabled={anyLoading}
                  className="w-full rounded-xl px-5 py-4 flex items-center gap-4 text-left transition-all duration-150"
                  style={{
                    background: "#0f0f0f",
                    border: `1px solid ${isThisLoading ? "rgba(124,58,237,0.45)" : "rgba(255,255,255,0.07)"}`,
                    cursor: anyLoading ? "not-allowed" : "pointer",
                    opacity: anyLoading && !isThisLoading ? 0.4 : 1,
                    boxShadow: isThisLoading
                      ? "0 0 0 1px rgba(124,58,237,0.15)"
                      : "none",
                  }}
                  onMouseEnter={(e) => {
                    if (!anyLoading) {
                      e.currentTarget.style.borderColor =
                        "rgba(124,58,237,0.4)";
                      e.currentTarget.style.boxShadow =
                        "0 0 0 1px rgba(124,58,237,0.1)";
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (!anyLoading) {
                      e.currentTarget.style.borderColor =
                        "rgba(255,255,255,0.07)";
                      e.currentTarget.style.boxShadow = "none";
                    }
                  }}
                >
                  {/* Initials avatar */}
                  <div
                    className="flex-shrink-0 rounded-lg flex items-center justify-center text-white font-bold"
                    style={{
                      width: 44,
                      height: 44,
                      background: "linear-gradient(135deg, #7c3aed, #a855f7)",
                      fontFamily: FONT_SYNE,
                      fontSize: "0.9rem",
                      letterSpacing: "-0.01em",
                    }}
                  >
                    {initials(org.name)}
                  </div>

                  {/* Org details */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span
                        className="text-sm font-semibold text-white truncate"
                        style={{ fontFamily: FONT_INTER }}
                      >
                        {org.name}
                      </span>
                      <span
                        className="flex-shrink-0 text-xs px-2 py-0.5 rounded-full font-medium capitalize"
                        style={{
                          background: badge.bg,
                          color: badge.color,
                          fontFamily: FONT_INTER,
                        }}
                      >
                        {m.role}
                      </span>
                    </div>
                    <span
                      className="text-xs"
                      style={{ color: "#555", fontFamily: FONT_INTER }}
                    >
                      {org.slug}
                    </span>
                  </div>

                  {/* Arrow / spinner */}
                  <div className="flex-shrink-0" style={{ color: "#444" }}>
                    {isThisLoading ? (
                      <span
                        className="w-4 h-4 rounded-full animate-spin block"
                        style={{
                          border: "2px solid rgba(124,58,237,0.25)",
                          borderTopColor: PURPLE,
                        }}
                      />
                    ) : (
                      <ChevronRight size={16} />
                    )}
                  </div>
                </button>
              );
            })
          )}
        </div>

        {/* ── Create new (when orgs exist) ────── */}
        {orgs.length > 0 && (
          <div ref={footRef} className="mt-3">
            <Link
              href="/create-organization"
              className="w-full rounded-xl px-5 py-3.5 flex items-center justify-center gap-2 text-sm font-medium transition-all duration-150"
              style={{
                border: "1px dashed rgba(255,255,255,0.09)",
                color: "#4a4a4a",
                fontFamily: FONT_INTER,
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = "rgba(124,58,237,0.3)";
                e.currentTarget.style.color = "#a855f7";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = "rgba(255,255,255,0.09)";
                e.currentTarget.style.color = "#4a4a4a";
              }}
            >
              <Plus size={14} />
              Create new workspace
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
