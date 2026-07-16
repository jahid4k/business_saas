// src/components/layout/Topbar.tsx
"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import {
  Bell,
  Sun,
  Moon,
  LogOut,
  User,
  Building2,
  ChevronDown,
  Menu,
} from "lucide-react";
import { useTheme } from "next-themes";
import { useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";
import { useUiStore } from "@/stores/uiStore";
import { useCommandStore } from "@/stores/commandStore";
import { logout } from "@/lib/auth";
import { setToken } from "@/lib/token";
import CommandMenu from "@/components/ui/CommandMenu";

const FONT_INTER = "var(--font-inter, Inter, sans-serif)";
const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";

const PAGE_TITLES: Record<string, string> = {
  "": "Dashboard",
  tasks: "Tasks",
  crm: "CRM",
  leads: "Leads",
  contacts: "Contacts",
  companies: "Companies",
  pipeline: "Pipeline",
  deals: "Deals",
  reports: "Reports",
  settings: "Settings",
  members: "Members",
  roles: "Roles",
  security: "Security",
  sessions: "Security",
  profile: "Profile",
};

export default function Topbar({ orgId }: { orgId: string }) {
  const router = useRouter();
  const pathname = usePathname();

  // ★ FIX: Use resolvedTheme, not theme.
  // `theme` can be undefined during SSR / before hydration.
  // `resolvedTheme` is always the actual applied value after mount.
  const { resolvedTheme, setTheme } = useTheme();
  const { setUiTheme, toggleMobileMenu } = useUiStore();
  const { setOpen: setCommandOpen } = useCommandStore();
  const { user, reset: resetAuth } = useAuthStore();
  const { reset: resetPerms } = usePermissionStore();
  const queryClient = useQueryClient();

  const [menuOpen, setMenuOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // eslint-disable-next-line react-hooks/set-state-in-effect -- intentional: standard next-themes pattern to delay theme-dependent rendering until after hydration, avoiding a server/client mismatch on resolvedTheme
  useEffect(() => setMounted(true), []);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, []);

  // ★ FIX: Use resolvedTheme so toggle always knows the real current state
  const isDark = mounted ? resolvedTheme !== "light" : true;

  const toggleTheme = () => {
    // resolvedTheme is always 'dark' or 'light' (never undefined after mount)
    const next = resolvedTheme === "dark" ? "light" : "dark";
    setTheme(next);
    setUiTheme(next as "dark" | "light");
  };

  const handleSignOut = async () => {
    setMenuOpen(false);
    try {
      await logout();
    } catch {
      /* ignore */
    }
    setToken(null);
    resetAuth();
    resetPerms();
    queryClient.clear(); // wipe cached org/CRM data — prevents stale tenant data leaking into the next login in this tab
    router.replace("/login");
  };

  const afterOrg = pathname.replace(`/${orgId}`, "").replace(/^\//, "");
  const segments = afterOrg.split("/").filter(Boolean);
  const titleKey =
    segments.findLast((s) => PAGE_TITLES[s]) ?? segments[0] ?? "";
  const pageTitle =
    (PAGE_TITLES[titleKey] ??
      titleKey.charAt(0).toUpperCase() + titleKey.slice(1)) ||
    "Dashboard";
  const initial = (user?.firstName ??
    user?.displayName ??
    "?")[0].toUpperCase();

  // ★ FIX: Use CSS variables for backgrounds so light mode actually works.
  // Hardcoded '#0a0a0a' ignores the theme toggle — CSS vars respond to .dark class.
  return (
    <>
      <style>{`
        @keyframes topbarMenuIn {
          from { opacity: 0; transform: translateY(-6px) scale(0.98); }
          to   { opacity: 1; transform: translateY(0)   scale(1); }
        }
        .topbar-menu { animation: topbarMenuIn 150ms ease forwards; }
      `}</style>

      <header
        style={{
          height: 56,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "0 20px",
          // ★ CSS variable — responds to .dark class change
          background: "var(--bg-base)",
          borderBottom: "1px solid var(--border)",
          flexShrink: 0,
          gap: 16,
          zIndex: 10,
        }}
      >
        <div
          style={{ display: "flex", alignItems: "center", flex: 1, gap: 24 }}
        >
          {/* ── Hamburger (mobile only) + page title ─ */}
          <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
            <TopbarBtn
              className="lg:hidden"
              onClick={toggleMobileMenu}
              title="Open menu"
            >
              <Menu size={18} style={{ color: "var(--text-muted)" }} />
            </TopbarBtn>
            <span
              style={{
                fontFamily: FONT_INTER,
                fontSize: "0.875rem",
                fontWeight: 500,
                color: "var(--text-secondary)",
                whiteSpace: "nowrap",
              }}
            >
              {pageTitle}
            </span>
          </div>

          {/* ── Left-aligned Search & Quick Actions Trigger ─ */}
          <div className="hidden sm:flex items-center gap-2">
            <button
              onClick={() => setCommandOpen(true)}
              className="flex items-center gap-2 px-3 py-1.5 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-elevated)] transition-colors text-sm text-[var(--text-muted)] w-48 lg:w-64"
            >
              <span className="flex-1 text-left">Quick actions...</span>
              <kbd className="hidden sm:inline-flex h-5 items-center gap-1 rounded border border-[var(--border)] bg-[var(--bg-base)] px-1.5 font-mono text-[10px] font-medium text-[var(--text-muted)]">
                <span className="text-xs">⌘</span>K
              </kbd>
            </button>
            <button
              onClick={() => setCommandOpen(true)}
              className="flex h-8 w-8 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-muted)]"
              title="Search (/)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <circle cx="11" cy="11" r="8" />
                <path d="m21 21-4.3-4.3" />
              </svg>
            </button>
          </div>
        </div>

        {/* ── Right actions ───────────────── */}
        <div style={{ display: "flex", alignItems: "center", gap: 2 }}>
          {/* Notifications placeholder */}
          <TopbarBtn title="Notifications">
            <Bell size={15} style={{ color: "var(--text-muted)" }} />
          </TopbarBtn>

          {/* ★ Theme toggle — only shown after mount to avoid hydration mismatch */}
          {mounted && (
            <TopbarBtn
              onClick={toggleTheme}
              title={isDark ? "Switch to light mode" : "Switch to dark mode"}
            >
              {isDark ? (
                <Sun size={15} style={{ color: "var(--text-muted)" }} />
              ) : (
                <Moon size={15} style={{ color: "var(--text-muted)" }} />
              )}
            </TopbarBtn>
          )}

          {/* Divider */}
          <div
            style={{
              width: 1,
              height: 20,
              background: "var(--border)",
              margin: "0 6px",
            }}
          />

          {/* User menu */}
          <div ref={menuRef} style={{ position: "relative" }}>
            <button
              onClick={() => setMenuOpen((v) => !v)}
              style={{
                ...btnBase,
                display: "flex",
                alignItems: "center",
                gap: 7,
                padding: "4px 10px 4px 5px",
                borderRadius: 8,
                width: "auto",
              }}
              onMouseEnter={(e) =>
                (e.currentTarget.style.background = "var(--bg-elevated)")
              }
              onMouseLeave={(e) =>
                (e.currentTarget.style.background = "transparent")
              }
            >
              <div
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: "50%",
                  flexShrink: 0,
                  background: "linear-gradient(135deg, #7c3aed, #a855f7)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontFamily: FONT_SYNE,
                  fontSize: "0.72rem",
                  fontWeight: 700,
                  color: "white",
                }}
              >
                {initial}
              </div>
              <ChevronDown
                size={12}
                style={{
                  color: "var(--text-muted)",
                  transition: "transform 150ms ease",
                  transform: menuOpen ? "rotate(180deg)" : "rotate(0deg)",
                }}
              />
            </button>

            {/* Dropdown */}
            {menuOpen && (
              <div
                className="topbar-menu"
                style={{
                  position: "absolute",
                  top: "calc(100% + 8px)",
                  right: 0,
                  width: 224,
                  // ★ CSS variables so dropdown looks correct in both themes
                  background: "var(--bg-elevated)",
                  border: "1px solid var(--border)",
                  borderRadius: 10,
                  boxShadow: "0 20px 48px rgba(0,0,0,0.5)",
                  zIndex: 50,
                  overflow: "hidden",
                }}
              >
                {/* User info */}
                <div
                  style={{
                    padding: "12px 14px 10px",
                    borderBottom: "1px solid var(--border)",
                  }}
                >
                  <p
                    style={{
                      fontFamily: FONT_INTER,
                      fontSize: "0.8rem",
                      fontWeight: 500,
                      color: "var(--text-primary)",
                      marginBottom: 2,
                    }}
                  >
                    {user?.displayName ?? user?.firstName}
                  </p>
                  <p
                    style={{
                      fontFamily: FONT_INTER,
                      fontSize: "0.72rem",
                      color: "var(--text-muted)",
                    }}
                  >
                    {user?.email}
                  </p>
                </div>

                <div style={{ padding: "4px" }}>
                  <DropdownLink
                    href={`/${orgId}/settings/profile`}
                    icon={User}
                    onClick={() => setMenuOpen(false)}
                  >
                    Profile &amp; settings
                  </DropdownLink>
                  <DropdownLink
                    href="/select-organization"
                    icon={Building2}
                    onClick={() => setMenuOpen(false)}
                  >
                    Switch workspace
                  </DropdownLink>
                </div>

                <div
                  style={{
                    padding: "4px",
                    borderTop: "1px solid var(--border)",
                  }}
                >
                  <button
                    onClick={handleSignOut}
                    style={{
                      ...dropdownItemStyle,
                      width: "100%",
                      textAlign: "left",
                      background: "transparent",
                      border: "none",
                    }}
                    onMouseEnter={(e) =>
                      (e.currentTarget.style.background = "rgba(239,68,68,0.1)")
                    }
                    onMouseLeave={(e) =>
                      (e.currentTarget.style.background = "transparent")
                    }
                  >
                    <LogOut
                      size={13}
                      style={{ color: "var(--text-muted)", flexShrink: 0 }}
                    />
                    <span style={{ color: "var(--text-secondary)" }}>
                      Sign out
                    </span>
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Command Menu Modal */}
      <CommandMenu />
    </>
  );
}

// ── Sub-components ────────────────────────────────────

function TopbarBtn({
  children,
  onClick,
  title,
  className,
}: {
  children: React.ReactNode;
  onClick?: () => void;
  title?: string;
  className?: string;
}) {
  return (
    <button
      onClick={onClick}
      title={title}
      className={className}
      style={btnBase}
      onMouseEnter={(e) =>
        (e.currentTarget.style.background = "var(--bg-elevated)")
      }
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
    >
      {children}
    </button>
  );
}

function DropdownLink({
  href,
  icon: Icon,
  children,
  onClick,
}: {
  href: string;
  icon: typeof User;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Link
      href={href}
      onClick={onClick}
      style={{ ...dropdownItemStyle, textDecoration: "none" }}
      onMouseEnter={(e) =>
        (e.currentTarget.style.background = "var(--bg-surface)")
      }
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
    >
      <Icon size={13} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
      <span style={{ color: "var(--text-secondary)" }}>{children}</span>
    </Link>
  );
}

// ── Shared styles ─────────────────────────────────────

const btnBase: React.CSSProperties = {
  width: 34,
  height: 34,
  borderRadius: 8,
  background: "transparent",
  border: "none",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  transition: "background 120ms ease",
  flexShrink: 0,
};

const dropdownItemStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 9,
  padding: "8px 10px",
  borderRadius: 6,
  fontFamily: "var(--font-inter, Inter, sans-serif)",
  fontSize: "0.8rem",
  transition: "background 120ms ease",
  cursor: "pointer",
};
