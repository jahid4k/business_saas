// src/components/layout/Sidebar.tsx
"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  CheckSquare,
  TrendingUp,
  UsersRound,
  ReceiptText,
  Funnel,
  FolderKanban,
  ShoppingCart,
  Users,
  Building2,
  Kanban,
  BarChart2,
  Shield,
  Lock,
  ChevronLeft,
  ChevronRight,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import { useUiStore } from "@/stores/uiStore";
import { usePermissionStore } from "@/stores/permissionStore";

// ── Types ──────────────────────────────────────────────────────────────────────
interface NavItem {
  label: string;
  href: string;
  icon: LucideIcon;
  permission?: string;
}

interface Module {
  id: string;
  label: string;
  icon: LucideIcon;
  status: "live" | "soon";
  eta?: string;
  items?: NavItem[];
}

// ── Nav config ─────────────────────────────────────────────────────────────────
function buildModules(orgId: string): Module[] {
  return [
    {
      id: "crm",
      label: "CRM",
      icon: TrendingUp,
      status: "live",
      items: [
        {
          label: "Leads",
          href: `/${orgId}/crm/leads`,
          icon: Funnel,
          permission: "crm.leads.view",
        },
        {
          label: "Pipeline",
          href: `/${orgId}/crm/pipeline`,
          icon: Kanban,
          permission: "crm.deals.view",
        },
        {
          label: "Reports",
          href: `/${orgId}/crm/reports`,
          icon: BarChart2,
          permission: "crm.reports.view",
        },
      ],
    },
    {
      id: "hrm",
      label: "HRM",
      icon: UsersRound,
      status: "soon",
      eta: "Q3 2026",
    },
    {
      id: "accounting",
      label: "Accounting",
      icon: ReceiptText,
      status: "soon",
      eta: "Q4 2026",
    },
    {
      id: "projects",
      label: "Projects",
      icon: FolderKanban,
      status: "soon",
      eta: "Q4 2026",
    },
    {
      id: "ecommerce",
      label: "E-commerce",
      icon: ShoppingCart,
      status: "soon",
      eta: "2027",
    },
  ];
}

const PLATFORM_ITEMS: NavItem[] = [
  {
    label: "Contacts",
    href: "contacts",
    icon: Users,
    permission: "crm.contacts.view",
  },
  {
    label: "Companies",
    href: "companies",
    icon: Building2,
    permission: "crm.companies.view",
  },
];

const SETTINGS: NavItem[] = [
  {
    label: "Members",
    href: "settings/members",
    icon: Users,
    permission: "members.view",
  },
  {
    label: "Roles",
    href: "settings/roles",
    icon: Shield,
    permission: "roles.view",
  },
  {
    label: "Security",
    href: "security/sessions",
    icon: Lock,
    permission: "security.sessions.view",
  },
];

// ── LogoMark ───────────────────────────────────────────────────────────────────
function LogoMark() {
  return (
    <div className="w-7 h-7 rounded-lg shrink-0 flex items-center justify-center bg-linear-to-br from-[#7c3aed] to-[#a855f7]">
      <svg
        width="14"
        height="14"
        viewBox="0 0 18 18"
        fill="none"
        aria-hidden="true"
      >
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
        <rect x="10" y="10" width="6" height="6" rx="1.5" fill="white" />
      </svg>
    </div>
  );
}

// ── CollapseTooltip ────────────────────────────────────────────────────────────
// Kept dark in both modes — tooltip pattern typically stays inverted
function CollapseTooltip({ label, extra }: { label: string; extra?: string }) {
  return (
    <span
      className="
        pointer-events-none absolute left-full top-1/2 -translate-y-1/2 ml-3
        px-2.5 py-1.5 rounded-md text-xs font-medium
        bg-gray-800 dark:bg-[#1a1a1a] text-white
        border border-white/10
        whitespace-nowrap shadow-lg z-50
        opacity-0 group-hover:opacity-100 transition-opacity duration-150
      "
    >
      {label}
      {extra && <span className="ml-2 text-purple-400">{extra}</span>}
    </span>
  );
}

// ── SectionLabel ───────────────────────────────────────────────────────────────
function SectionLabel({ children }: { children: string }) {
  return (
    <p className="text-[0.6rem] font-semibold tracking-widest uppercase px-2 pt-3 pb-1 text-gray-400 dark:text-[#3a3a3a]">
      {children}
    </p>
  );
}

// ── NavLink ────────────────────────────────────────────────────────────────────
interface NavLinkProps {
  href: string;
  icon: LucideIcon;
  label: string;
  active: boolean;
  closed: boolean;
  compact?: boolean;
}

function NavLink({
  href,
  icon: Icon,
  label,
  active,
  closed,
  compact,
}: NavLinkProps) {
  return (
    <Link
      href={href}
      title={closed ? label : undefined}
      className={`
        group relative flex items-center rounded-md no-underline
        transition-colors duration-100 mb-0.5
        ${
          closed
            ? `justify-center py-2.5 px-0 w-full ${
                active
                  ? "bg-purple-50 dark:bg-purple-700/10"
                  : "hover:bg-gray-100 dark:hover:bg-white/4"
              }`
            : `gap-2.5 border-l-2 ${compact ? "py-1.75 px-2" : "py-2 px-2"} ${
                active
                  ? "bg-purple-50 dark:bg-purple-700/10 border-purple-600 dark:border-[#7c3aed]"
                  : "border-transparent hover:bg-gray-100 dark:hover:bg-white/4"
              }`
        }
      `}
    >
      <Icon
        size={compact ? 13 : 15}
        className={`shrink-0 ${
          active
            ? "text-purple-600 dark:text-[#7c3aed]"
            : "text-gray-700 dark:text-[#cfcfcf]"
        }`}
      />
      {!closed && (
        <span
          className={`
            whitespace-nowrap
            ${compact ? "text-[0.78rem]" : "text-[0.8rem]"}
            ${
              active
                ? "text-gray-900 dark:text-white font-medium"
                : "text-gray-700 dark:text-[#cfcfcf] font-normal"
            }
          `}
        >
          {label}
        </span>
      )}
      {closed && <CollapseTooltip label={label} />}
    </Link>
  );
}

// ── ModuleRow ──────────────────────────────────────────────────────────────────
interface ModuleRowProps {
  module: Module;
  isOpen: boolean;
  isActive: boolean;
  closed: boolean;
  onClick: () => void;
}

function ModuleRow({
  module,
  isOpen,
  isActive,
  closed,
  onClick,
}: ModuleRowProps) {
  const Icon = module.icon;
  const soon = module.status === "soon";

  return (
    <div className="group relative">
      <button
        onClick={onClick}
        className={`
          w-full flex items-center rounded-md transition-colors duration-100
          ${closed ? "justify-center py-2.5 px-0" : "gap-2.5 px-2 py-2"}
          ${
            soon
              ? "cursor-not-allowed"
              : isActive
                ? "bg-purple-50 dark:bg-purple-700/10 cursor-pointer"
                : "hover:bg-gray-100 dark:hover:bg-white/4 cursor-pointer"
          }
        `}
        title={
          closed
            ? `${module.label}${soon ? ` — Coming ${module.eta}` : ""}`
            : undefined
        }
      >
        <Icon
          size={15}
          className={`shrink-0 ${
            isActive && !soon
              ? "text-purple-600 dark:text-[#7c3aed]"
              : "text-gray-400 dark:text-[#555]"
          }`}
        />
        {!closed && (
          <>
            <span
              className={`
                flex-1 text-left text-[0.8rem]
                ${
                  isActive && !soon
                    ? "text-gray-900 dark:text-white font-medium"
                    : soon
                      ? "text-gray-400 dark:text-[#666]"
                      : "text-gray-500 dark:text-[#ddd] font-normal"
                }
              `}
            >
              {module.label}
            </span>
            {soon ? (
              <span className="text-[0.6rem] font-semibold text-purple-400 uppercase tracking-wide bg-purple-500/10 border border-purple-500/20 px-1.5 py-0.5 rounded-full">
                Soon
              </span>
            ) : (
              <ChevronRight
                size={12}
                className={`shrink-0 transition-transform duration-200 text-gray-300 dark:text-[#444] ${
                  isOpen ? "rotate-90" : ""
                }`}
              />
            )}
          </>
        )}
      </button>

      {closed && (
        <CollapseTooltip
          label={module.label}
          extra={soon ? `Coming ${module.eta}` : undefined}
        />
      )}
    </div>
  );
}

// ── Sidebar ────────────────────────────────────────────────────────────────────
export default function Sidebar({ orgId }: { orgId: string }) {
  const pathname = usePathname();
  const router = useRouter();
  const {
    sidebarCollapsed,
    toggleSidebar,
    setSidebarCollapsed,
    mobileMenuOpen,
    setMobileMenuOpen,
  } = useUiStore();
  const { hasPermission } = usePermissionStore();
  const { currentOrg, user } = useAuthStore();

  const [openModules, setOpenModules] = useState<Set<string>>(new Set());

  // Icon-only collapse is a desktop concept. On mobile the sidebar is an
  // off-canvas drawer (open/closed driven by mobileMenuOpen) and should
  // always show full labels when it's open — collapsing it to icons would
  // defeat the point of a drawer meant for one-handed tap navigation.
  const closed = sidebarCollapsed && !mobileMenuOpen;
  const modules = buildModules(orgId);

  // Auto-expand module that owns the current route
  useEffect(() => {
    modules.forEach((m) => {
      if (m.status === "live" && pathname.startsWith(`/${orgId}/${m.id}`)) {
        setOpenModules((prev) => new Set([...prev, m.id]));
      }
    });
  }, [pathname, orgId]); // eslint-disable-line react-hooks/exhaustive-deps

  const toggleModule = (id: string) =>
    setOpenModules((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });

  const handleModuleClick = (m: Module) => {
    if (m.status === "soon") {
      router.push(`/${orgId}/coming-soon?module=${m.id}`);
      return;
    }
    if (closed) {
      setSidebarCollapsed(false);
      setOpenModules((prev) => new Set([...prev, m.id]));
      return;
    }
    toggleModule(m.id);
  };

  const isPathActive = (href: string) =>
    pathname === href || pathname.startsWith(href + "/");
  const isModActive = (m: Module) => pathname.startsWith(`/${orgId}/${m.id}`);

  return (
    <>
      {/* Mobile backdrop — tapping it closes the drawer */}
      {mobileMenuOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/50 lg:hidden"
          onClick={() => setMobileMenuOpen(false)}
          aria-hidden="true"
        />
      )}

      <aside
        onClick={() => setMobileMenuOpen(false)}
        className={`
          fixed inset-y-0 left-0 z-40 lg:relative lg:z-auto lg:inset-auto
          flex flex-col shrink-0 h-screen overflow-hidden
          bg-gray-50 dark:bg-[#080808]
          border-r border-gray-200 dark:border-white/5.5
          transition-transform lg:transition-[width,min-width] duration-220 ease-in-out
          w-72 ${mobileMenuOpen ? "translate-x-0" : "-translate-x-full"} lg:translate-x-0
          ${closed ? "lg:w-16 lg:min-w-16" : "lg:w-60 lg:min-w-60"}
        `}
      >
        {/* ── Header ──────────────────────────────── */}
        <div
          className={`
          h-14 flex items-center shrink-0 gap-2
          border-b border-gray-100 dark:border-white/5
          ${closed ? "justify-center px-3" : "justify-between pl-4.5 pr-3"}
        `}
        >
          {!closed && (
            <Link
              href={`/${orgId}`}
              className="flex items-center gap-2.5 no-underline min-w-0"
            >
              <LogoMark />
              <span className="font-syne text-sm font-bold tracking-[-0.01em] whitespace-nowrap text-gray-900 dark:text-white">
                BusinessSAAS
              </span>
            </Link>
          )}
          <button
            onClick={toggleSidebar}
            className="
            hidden lg:flex w-6 h-6 shrink-0 items-center justify-center rounded-md
            border border-gray-200 dark:border-white/10
            text-gray-400 dark:text-[#444]
            hover:bg-gray-100 dark:hover:bg-white/8
            hover:text-gray-700 dark:hover:text-[#bbb]
            transition-colors
          "
            title={closed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {closed ? <ChevronRight size={13} /> : <ChevronLeft size={13} />}
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation();
              setMobileMenuOpen(false);
            }}
            className="
            lg:hidden w-6 h-6 shrink-0 flex items-center justify-center rounded-md
            border border-gray-200 dark:border-white/10
            text-gray-400 dark:text-[#444]
            hover:bg-gray-100 dark:hover:bg-white/8
            hover:text-gray-700 dark:hover:text-[#bbb]
            transition-colors
          "
            title="Close menu"
          >
            <X size={14} />
          </button>
        </div>

        {/* ── Org switcher ────────────────────────── */}
        <Link
          href="/select-organization"
          className={`
          flex items-center shrink-0 no-underline
          border-b border-gray-100 dark:border-white/5
          hover:bg-gray-100 dark:hover:bg-white/4 transition-colors
          ${closed ? "justify-center px-2 py-2.5" : "gap-2.5 px-3.5 py-2.5"}
        `}
          title={closed ? (currentOrg?.name ?? "Switch workspace") : undefined}
        >
          <div className="w-7.5 h-7.5 rounded-lg shrink-0 flex items-center justify-center text-white text-xs font-bold font-syne bg-linear-to-br from-[#7c3aed] to-[#a855f7]">
            {(currentOrg?.name ?? "W")[0].toUpperCase()}
          </div>
          {!closed && (
            <div className="min-w-0">
              <p className="text-[0.8rem] font-semibold truncate leading-snug text-gray-800 dark:text-[#e0e0e0]">
                {currentOrg?.name ?? "Workspace"}
              </p>
              <p className="text-[0.65rem] text-gray-400 dark:text-[#444]">
                Switch workspace
              </p>
            </div>
          )}
        </Link>

        {/* ── Scrollable nav ──────────────────────── */}
        <nav className="flex-1 overflow-y-auto overflow-x-hidden px-1.5 py-2 scrollbar-none">
          {/* COMMON */}
          <div className="mb-2">
            {!closed && <SectionLabel>Common</SectionLabel>}
            {closed && <div className="h-2" />}
            {hasPermission("tasks.view") && (
              <NavLink
                href={`/${orgId}/tasks`}
                icon={CheckSquare}
                label="Tasks"
                active={isPathActive(`/${orgId}/tasks`)}
                closed={closed}
              />
            )}
          </div>

          {/* PLATFORM */}
          <div className="mb-2">
            {!closed && <SectionLabel>Platform</SectionLabel>}
            {closed && <div className="h-2" />}
            {PLATFORM_ITEMS.filter(
              (s) => !s.permission || hasPermission(s.permission),
            ).map((s) => (
              <NavLink
                key={s.href}
                href={`/${orgId}/${s.href}`}
                icon={s.icon}
                label={s.label}
                active={isPathActive(`/${orgId}/${s.href}`)}
                closed={closed}
              />
            ))}
          </div>

          {/* MODULES */}
          <div className="mb-2">
            {!closed && <SectionLabel>Modules</SectionLabel>}
            {closed && <div className="h-2" />}
            {modules.map((m) => {
              const isOpen = openModules.has(m.id);
              const isActive = isModActive(m);
              const visible = (m.items ?? []).filter(
                (i) => !i.permission || hasPermission(i.permission),
              );
              const itemH = 34; // px per sub-item

              return (
                <div key={m.id}>
                  <ModuleRow
                    module={m}
                    isOpen={isOpen}
                    isActive={isActive}
                    closed={closed}
                    onClick={() => handleModuleClick(m)}
                  />

                  {/* Accordion — maxHeight must stay inline (computed value) */}
                  {m.status === "live" && !closed && (
                    <div
                      className="overflow-hidden transition-[max-height] duration-200 ease-in-out"
                      style={{
                        maxHeight: isOpen
                          ? `${visible.length * itemH + 8}px`
                          : "0px",
                      }}
                    >
                      <div className="ml-3 pl-3 border-l border-gray-100 dark:border-white/6 py-0.5 space-y-0.5">
                        {visible.map((item) => (
                          <NavLink
                            key={item.href}
                            href={item.href}
                            icon={item.icon}
                            label={item.label}
                            active={isPathActive(item.href)}
                            closed={false}
                            compact
                          />
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/* SETTINGS */}
          <div>
            {!closed && <SectionLabel>Settings</SectionLabel>}
            {closed && <div className="h-2" />}
            {SETTINGS.filter(
              (s) => !s.permission || hasPermission(s.permission),
            ).map((s) => (
              <NavLink
                key={s.href}
                href={`/${orgId}/${s.href}`}
                icon={s.icon}
                label={s.label}
                active={isPathActive(`/${orgId}/${s.href}`)}
                closed={closed}
              />
            ))}
          </div>
        </nav>

        {/* ── Footer — user profile ────────────────── */}
        <div
          className={`border-t border-gray-100 dark:border-white/5 shrink-0 ${
            closed ? "p-1.5" : "p-2"
          }`}
        >
          <Link
            href={`/${orgId}/settings/profile`}
            className={`
            flex items-center no-underline rounded-md
            hover:bg-gray-100 dark:hover:bg-white/5 transition-colors
            ${closed ? "justify-center p-2" : "gap-2.5 px-2 py-1.5"}
          `}
            title={closed ? (user?.displayName ?? "Profile") : undefined}
          >
            <div className="w-7 h-7 rounded-full shrink-0 flex items-center justify-center text-white text-xs font-bold font-syne bg-linear-to-br from-[#7c3aed] to-[#a855f7]">
              {(user?.firstName ?? user?.displayName ?? "?")[0].toUpperCase()}
            </div>
            {!closed && (
              <div className="min-w-0">
                <p className="text-[0.78rem] font-medium truncate leading-snug text-gray-700 dark:text-[#d0d0d0]">
                  {user?.displayName ?? user?.firstName ?? "User"}
                </p>
                <p className="text-[0.65rem] truncate text-gray-400 dark:text-[#444]">
                  {user?.email ?? ""}
                </p>
              </div>
            )}
          </Link>
        </div>
      </aside>
    </>
  );
}
