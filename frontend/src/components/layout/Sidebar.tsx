// // src/components/layout/Sidebar.tsx
// "use client";

// import Link from "next/link";
// import { usePathname } from "next/navigation";
// import {
//   CheckSquare,
//   UserPlus,
//   Users,
//   Building2,
//   Kanban,
//   BarChart2,
//   Shield,
//   Lock,
//   ChevronLeft,
//   ChevronRight,
// } from "lucide-react";
// import type { LucideIcon } from "lucide-react";
// import { useAuthStore } from "@/stores/authStore";
// import { useUiStore } from "@/stores/uiStore";
// import { usePermissionStore } from "@/stores/permissionStore";

// const PURPLE = "#7c3aed";
// const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
// const FONT_INTER = "var(--font-inter, Inter, sans-serif)";
// const W_OPEN = 240;
// const W_CLOSED = 64;
// const TRANSITION =
//   "width 220ms cubic-bezier(0.4,0,0.2,1), min-width 220ms cubic-bezier(0.4,0,0.2,1)";

// interface NavItem {
//   label: string;
//   href: string;
//   icon: LucideIcon;
//   permission?: string;
// }
// interface NavSection {
//   title: string;
//   items: NavItem[];
// }

// function buildNav(orgId: string): NavSection[] {
//   return [
//     {
//       title: "Main",
//       items: [
//         {
//           label: "Tasks",
//           href: `/${orgId}/tasks`,
//           icon: CheckSquare,
//           permission: "tasks.view",
//         },
//       ],
//     },
//     {
//       title: "CRM",
//       items: [
//         {
//           label: "Leads",
//           href: `/${orgId}/crm/leads`,
//           icon: UserPlus,
//           permission: "crm.leads.view",
//         },
//         {
//           label: "Contacts",
//           href: `/${orgId}/crm/contacts`,
//           icon: Users,
//           permission: "crm.contacts.view",
//         },
//         {
//           label: "Companies",
//           href: `/${orgId}/crm/companies`,
//           icon: Building2,
//           permission: "crm.companies.view",
//         },
//         {
//           label: "Pipeline",
//           href: `/${orgId}/crm/pipeline`,
//           icon: Kanban,
//           permission: "crm.deals.view",
//         },
//         {
//           label: "Reports",
//           href: `/${orgId}/crm/reports`,
//           icon: BarChart2,
//           permission: "crm.reports.view",
//         },
//       ],
//     },
//     {
//       title: "Settings",
//       items: [
//         {
//           label: "Members",
//           href: `/${orgId}/settings/members`,
//           icon: Users,
//           permission: "members.view",
//         },
//         {
//           label: "Roles",
//           href: `/${orgId}/settings/roles`,
//           icon: Shield,
//           permission: "roles.view",
//         },
//         {
//           label: "Security",
//           href: `/${orgId}/security/sessions`,
//           icon: Lock,
//           permission: "security.sessions.view",
//         },
//       ],
//     },
//   ];
// }

// export default function Sidebar({ orgId }: { orgId: string }) {
//   const pathname = usePathname();
//   const { sidebarCollapsed, toggleSidebar } = useUiStore();
//   const { hasPermission } = usePermissionStore();
//   const { currentOrg, user } = useAuthStore();

//   const closed = sidebarCollapsed;
//   const w = closed ? W_CLOSED : W_OPEN;
//   const nav = buildNav(orgId);

//   const isActive = (href: string) =>
//     pathname === href || pathname.startsWith(href + "/");

//   return (
//     <aside
//       style={{
//         width: w,
//         minWidth: w,
//         height: "100vh",
//         display: "flex",
//         flexDirection: "column",
//         flexShrink: 0,
//         background: "#080808",
//         borderRight: "1px solid rgba(255,255,255,0.055)",
//         transition: TRANSITION,
//         overflow: "hidden",
//       }}
//     >
//       {/* ── Header — toggle button ALWAYS here ────── */}
//       {/* Both collapse and expand buttons live in this same spot.
//           The button just flips its icon. No more hunting for it. */}
//       <div
//         style={{
//           height: 56,
//           display: "flex",
//           alignItems: "center",
//           // When expanded: logo on left, button on right
//           // When collapsed: button centered (logo hidden)
//           justifyContent: closed ? "center" : "space-between",
//           padding: closed ? "0 12px" : "0 12px 0 18px",
//           borderBottom: "1px solid rgba(255,255,255,0.05)",
//           flexShrink: 0,
//           gap: 8,
//         }}
//       >
//         {/* Logo — hidden when collapsed to give room to the button */}
//         {!closed && (
//           <Link
//             href={`/${orgId}`}
//             style={{
//               display: "flex",
//               alignItems: "center",
//               gap: 9,
//               textDecoration: "none",
//               minWidth: 0,
//             }}
//           >
//             <LogoMark />
//             <span
//               style={{
//                 fontFamily: FONT_SYNE,
//                 fontSize: "0.875rem",
//                 fontWeight: 700,
//                 color: "white",
//                 letterSpacing: "-0.01em",
//                 whiteSpace: "nowrap",
//               }}
//             >
//               BusinessSAAS
//             </span>
//           </Link>
//         )}

//         {/* ★ THE FIX: Single toggle button — always at the top, always same location */}
//         <button
//           onClick={toggleSidebar}
//           style={toggleBtnStyle}
//           title={closed ? "Expand sidebar" : "Collapse sidebar"}
//           onMouseEnter={(e) => {
//             e.currentTarget.style.background = "rgba(255,255,255,0.08)";
//             e.currentTarget.style.color = "#bbb";
//           }}
//           onMouseLeave={(e) => {
//             e.currentTarget.style.background = "transparent";
//             e.currentTarget.style.color = "#444";
//           }}
//         >
//           {closed ? <ChevronRight size={13} /> : <ChevronLeft size={13} />}
//         </button>
//       </div>

//       {/* ── Org switcher ──────────────────────────── */}
//       <Link
//         href="/select-organization"
//         style={{
//           display: "flex",
//           alignItems: "center",
//           gap: 10,
//           padding: closed ? "10px" : "10px 14px",
//           borderBottom: "1px solid rgba(255,255,255,0.05)",
//           textDecoration: "none",
//           flexShrink: 0,
//           justifyContent: closed ? "center" : "flex-start",
//         }}
//         title={closed ? (currentOrg?.name ?? "Switch workspace") : undefined}
//         onMouseEnter={(e) =>
//           (e.currentTarget.style.background = "rgba(255,255,255,0.04)")
//         }
//         onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
//       >
//         <div
//           style={{
//             width: 30,
//             height: 30,
//             borderRadius: 8,
//             flexShrink: 0,
//             background: "linear-gradient(135deg, #7c3aed, #a855f7)",
//             display: "flex",
//             alignItems: "center",
//             justifyContent: "center",
//             fontFamily: FONT_SYNE,
//             fontSize: "0.8rem",
//             fontWeight: 700,
//             color: "white",
//           }}
//         >
//           {(currentOrg?.name ?? "W")[0].toUpperCase()}
//         </div>
//         {!closed && (
//           <div style={{ minWidth: 0 }}>
//             <p
//               style={{
//                 fontFamily: FONT_INTER,
//                 fontSize: "0.8rem",
//                 fontWeight: 600,
//                 color: "#e0e0e0",
//                 whiteSpace: "nowrap",
//                 overflow: "hidden",
//                 textOverflow: "ellipsis",
//                 lineHeight: 1.35,
//               }}
//             >
//               {currentOrg?.name ?? "Workspace"}
//             </p>
//             <p
//               style={{
//                 fontFamily: FONT_INTER,
//                 fontSize: "0.65rem",
//                 color: "#444",
//                 lineHeight: 1,
//               }}
//             >
//               Switch workspace
//             </p>
//           </div>
//         )}
//       </Link>

//       {/* ── Navigation ──────────────────────────────── */}
//       <nav
//         style={{
//           flex: 1,
//           overflowY: "auto",
//           overflowX: "hidden",
//           padding: "6px",
//           scrollbarWidth: "none",
//         }}
//       >
//         {nav.map((section) => {
//           const visible = section.items.filter(
//             (i) => !i.permission || hasPermission(i.permission),
//           );
//           if (!visible.length) return null;
//           return (
//             <div key={section.title} style={{ marginBottom: 2 }}>
//               {!closed ? (
//                 <p
//                   style={{
//                     fontFamily: FONT_INTER,
//                     fontSize: "0.6rem",
//                     fontWeight: 600,
//                     color: "#2e2e2e",
//                     letterSpacing: "0.1em",
//                     textTransform: "uppercase",
//                     padding: "10px 8px 4px",
//                   }}
//                 >
//                   {section.title}
//                 </p>
//               ) : (
//                 <div style={{ height: 10 }} />
//               )}

//               {visible.map((item) => {
//                 const active = isActive(item.href);
//                 const Icon = item.icon;
//                 return (
//                   <Link
//                     key={item.href}
//                     href={item.href}
//                     title={closed ? item.label : undefined}
//                     style={{
//                       display: "flex",
//                       alignItems: "center",
//                       gap: 9,
//                       padding: closed ? "9px 0" : "8px 8px",
//                       borderRadius: 6,
//                       textDecoration: "none",
//                       marginBottom: 1,
//                       justifyContent: closed ? "center" : "flex-start",
//                       background: active
//                         ? "rgba(124,58,237,0.1)"
//                         : "transparent",
//                       borderLeft: `2px solid ${active ? PURPLE : "transparent"}`,
//                       transition:
//                         "background 100ms ease, border-color 100ms ease",
//                     }}
//                     onMouseEnter={(e) => {
//                       if (!active)
//                         e.currentTarget.style.background =
//                           "rgba(255,255,255,0.04)";
//                     }}
//                     onMouseLeave={(e) => {
//                       if (!active)
//                         e.currentTarget.style.background = "transparent";
//                     }}
//                   >
//                     <Icon
//                       size={15}
//                       style={{
//                         color: active ? PURPLE : "#555",
//                         flexShrink: 0,
//                         transition: "color 100ms ease",
//                       }}
//                     />
//                     {!closed && (
//                       <span
//                         style={{
//                           fontFamily: FONT_INTER,
//                           fontSize: "0.8rem",
//                           fontWeight: active ? 500 : 400,
//                           color: active ? "#f0f0f0" : "#888",
//                           whiteSpace: "nowrap",
//                         }}
//                       >
//                         {item.label}
//                       </span>
//                     )}
//                   </Link>
//                 );
//               })}
//             </div>
//           );
//         })}
//       </nav>

//       {/* ── Footer — user profile only (no expand button anymore) ── */}
//       <div
//         style={{
//           borderTop: "1px solid rgba(255,255,255,0.05)",
//           padding: closed ? "8px 6px" : "8px",
//           flexShrink: 0,
//         }}
//       >
//         <Link
//           href="/settings/profile"
//           style={{
//             display: "flex",
//             alignItems: "center",
//             gap: 9,
//             padding: "6px 6px",
//             borderRadius: 6,
//             textDecoration: "none",
//             justifyContent: closed ? "center" : "flex-start",
//           }}
//           title={closed ? (user?.displayName ?? "Profile") : undefined}
//           onMouseEnter={(e) =>
//             (e.currentTarget.style.background = "rgba(255,255,255,0.05)")
//           }
//           onMouseLeave={(e) =>
//             (e.currentTarget.style.background = "transparent")
//           }
//         >
//           <div
//             style={{
//               width: 28,
//               height: 28,
//               borderRadius: "50%",
//               flexShrink: 0,
//               background: "linear-gradient(135deg, #7c3aed, #a855f7)",
//               display: "flex",
//               alignItems: "center",
//               justifyContent: "center",
//               fontFamily: FONT_SYNE,
//               fontSize: "0.72rem",
//               fontWeight: 700,
//               color: "white",
//             }}
//           >
//             {(user?.firstName ?? user?.displayName ?? "?")[0].toUpperCase()}
//           </div>
//           {!closed && (
//             <div style={{ minWidth: 0 }}>
//               <p
//                 style={{
//                   fontFamily: FONT_INTER,
//                   fontSize: "0.78rem",
//                   fontWeight: 500,
//                   color: "#d0d0d0",
//                   whiteSpace: "nowrap",
//                   overflow: "hidden",
//                   textOverflow: "ellipsis",
//                   lineHeight: 1.3,
//                 }}
//               >
//                 {user?.displayName ?? user?.firstName ?? "User"}
//               </p>
//               <p
//                 style={{
//                   fontFamily: FONT_INTER,
//                   fontSize: "0.65rem",
//                   color: "#444",
//                   whiteSpace: "nowrap",
//                   overflow: "hidden",
//                   textOverflow: "ellipsis",
//                 }}
//               >
//                 {user?.email ?? ""}
//               </p>
//             </div>
//           )}
//         </Link>
//       </div>
//     </aside>
//   );
// }

// function LogoMark() {
//   return (
//     <div
//       style={{
//         width: 28,
//         height: 28,
//         borderRadius: 7,
//         flexShrink: 0,
//         background: "linear-gradient(135deg, #7c3aed, #a855f7)",
//         display: "flex",
//         alignItems: "center",
//         justifyContent: "center",
//       }}
//     >
//       <svg
//         width="14"
//         height="14"
//         viewBox="0 0 18 18"
//         fill="none"
//         aria-hidden="true"
//       >
//         <rect x="2" y="2" width="6" height="6" rx="1.5" fill="white" />
//         <rect
//           x="10"
//           y="2"
//           width="6"
//           height="6"
//           rx="1.5"
//           fill="white"
//           fillOpacity="0.5"
//         />
//         <rect
//           x="2"
//           y="10"
//           width="6"
//           height="6"
//           rx="1.5"
//           fill="white"
//           fillOpacity="0.5"
//         />
//         <rect x="10" y="10" width="6" height="6" rx="1.5" fill="white" />
//       </svg>
//     </div>
//   );
// }

// const toggleBtnStyle: React.CSSProperties = {
//   width: 26,
//   height: 26,
//   borderRadius: 6,
//   flexShrink: 0,
//   background: "transparent",
//   border: "1px solid rgba(255,255,255,0.08)",
//   display: "flex",
//   alignItems: "center",
//   justifyContent: "center",
//   color: "#444",
//   transition: "background 120ms ease, color 120ms ease",
// };

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
  FolderKanban,
  ShoppingCart,
  UserPlus,
  Users,
  Building2,
  Kanban,
  BarChart2,
  Shield,
  Lock,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import { useUiStore } from "@/stores/uiStore";
import { usePermissionStore } from "@/stores/permissionStore";

// ── Types ─────────────────────────────────────────────
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
  eta?: string; // shown in tooltip for 'soon' modules
  items?: NavItem[]; // only for 'live' modules
}

// ── Nav config ─────────────────────────────────────────
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
          icon: UserPlus,
          permission: "crm.leads.view",
        },
        {
          label: "Contacts",
          href: `/${orgId}/crm/contacts`,
          icon: Users,
          permission: "crm.contacts.view",
        },
        {
          label: "Companies",
          href: `/${orgId}/crm/companies`,
          icon: Building2,
          permission: "crm.companies.view",
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

const SETTINGS: Omit<NavItem, "permission"> & { permission?: string }[] = [
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

// ── Helpers ────────────────────────────────────────────
function LogoMark() {
  return (
    <div
      className="w-7 h-7 rounded-lg flex-shrink-0 flex items-center justify-center"
      style={{ background: "linear-gradient(135deg, #7c3aed, #a855f7)" }}
    >
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

// Tooltip that appears to the right in collapsed mode
function CollapseTooltip({ label, extra }: { label: string; extra?: string }) {
  return (
    <span
      className="
      pointer-events-none absolute left-full top-1/2 -translate-y-1/2 ml-3
      px-2.5 py-1.5 rounded-md text-xs font-medium
      bg-[#1a1a1a] text-white border border-white/10
      whitespace-nowrap shadow-lg z-50
      opacity-0 group-hover:opacity-100 transition-opacity duration-150
    "
    >
      {label}
      {extra && <span className="ml-2 text-purple-400">{extra}</span>}
    </span>
  );
}

// Section label shown when sidebar is expanded
function SectionLabel({ children }: { children: string }) {
  return (
    <p
      className="
      text-[0.6rem] font-semibold text-[#2e2e2e]
      tracking-[0.1em] uppercase px-2 pt-3 pb-1
    "
      style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
    >
      {children}
    </p>
  );
}

// ── NavLink ─────────────────────────────────────────────
// Used for Tasks (flat) and Settings items
interface NavLinkProps {
  href: string;
  icon: LucideIcon;
  label: string;
  active: boolean;
  closed: boolean;
  compact?: boolean; // slightly smaller text/padding for sub-items
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
            ? `justify-center py-2.5 px-0 w-full ${active ? "bg-purple-700/10" : "hover:bg-white/[0.04]"}`
            : `gap-2.5 ${compact ? "py-[7px] px-2" : "py-2 px-2"}
             border-l-2 ${active ? "bg-purple-700/10 border-[#7c3aed]" : "border-transparent hover:bg-white/[0.04]"}`
        }
      `}
    >
      <Icon
        size={compact ? 13 : 15}
        className={`flex-shrink-0 ${active ? "text-[#7c3aed]" : "text-[#555]"}`}
      />
      {!closed && (
        <span
          className={`whitespace-nowrap ${active ? "text-white font-medium" : "text-[#888] font-normal"}`}
          style={{
            fontFamily: "var(--font-inter, Inter, sans-serif)",
            fontSize: compact ? "0.78rem" : "0.8rem",
          }}
        >
          {label}
        </span>
      )}
      {closed && <CollapseTooltip label={label} />}
    </Link>
  );
}

// ── ModuleRow ───────────────────────────────────────────
// The clickable module header (CRM, HRM, etc.)
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
              ? "opacity-40 cursor-not-allowed"
              : isActive
                ? "bg-purple-700/10 cursor-pointer"
                : "hover:bg-white/[0.04] cursor-pointer"
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
          className={`flex-shrink-0 ${isActive && !soon ? "text-[#7c3aed]" : "text-[#555]"}`}
        />
        {!closed && (
          <>
            <span
              className={`flex-1 text-left ${isActive && !soon ? "text-white font-medium" : soon ? "text-[#666]" : "text-[#888] font-normal"}`}
              style={{
                fontFamily: "var(--font-inter, Inter, sans-serif)",
                fontSize: "0.8rem",
              }}
            >
              {module.label}
            </span>
            {soon ? (
              <span
                className="
                text-[0.6rem] font-semibold text-purple-400 uppercase tracking-wide
                bg-purple-500/10 border border-purple-500/20 px-1.5 py-0.5 rounded-full
              "
              >
                Soon
              </span>
            ) : (
              <ChevronRight
                size={12}
                className={`text-[#444] transition-transform duration-200 flex-shrink-0 ${isOpen ? "rotate-90" : ""}`}
              />
            )}
          </>
        )}
      </button>

      {/* Tooltip — collapsed mode only */}
      {closed && (
        <CollapseTooltip
          label={module.label}
          extra={soon ? `Coming ${module.eta}` : undefined}
        />
      )}
    </div>
  );
}

// ── Sidebar ─────────────────────────────────────────────
export default function Sidebar({ orgId }: { orgId: string }) {
  const pathname = usePathname();
  const router = useRouter();
  const { sidebarCollapsed, toggleSidebar, setSidebarCollapsed } = useUiStore();
  const { hasPermission } = usePermissionStore();
  const { currentOrg, user } = useAuthStore();

  // Which modules are currently expanded (multiple allowed)
  const [openModules, setOpenModules] = useState<Set<string>>(new Set());

  const closed = sidebarCollapsed;
  const modules = buildModules(orgId);

  // Auto-expand the module that owns the current route
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
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  const handleModuleClick = (m: Module) => {
    if (m.status === "soon") {
      // Disabled → go to roadmap page
      router.push(`/${orgId}/coming-soon?module=${m.id}`);
      return;
    }
    if (closed) {
      // Collapsed → expand sidebar and open this module
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
    <aside
      className="flex flex-col flex-shrink-0 h-screen overflow-hidden"
      style={{
        width: closed ? 64 : 240,
        minWidth: closed ? 64 : 240,
        background: "#080808",
        borderRight: "1px solid rgba(255,255,255,0.055)",
        transition:
          "width 220ms cubic-bezier(0.4,0,0.2,1), min-width 220ms cubic-bezier(0.4,0,0.2,1)",
      }}
    >
      {/* ── Header ─────────────────────────────── */}
      <div
        className={`
          flex items-center flex-shrink-0 border-b border-white/5 gap-2
          ${closed ? "justify-center px-3" : "justify-between pl-[18px] pr-3"}
        `}
        style={{ height: 56 }}
      >
        {!closed && (
          <Link
            href={`/${orgId}`}
            className="flex items-center gap-2.5 no-underline min-w-0"
          >
            <LogoMark />
            <span
              className="text-sm font-bold text-white whitespace-nowrap"
              style={{
                fontFamily: "var(--font-syne, Syne, sans-serif)",
                letterSpacing: "-0.01em",
              }}
            >
              BusinessSAAS
            </span>
          </Link>
        )}
        <button
          onClick={toggleSidebar}
          className="w-6 h-6 flex-shrink-0 flex items-center justify-center rounded-md border border-white/10 text-[#444] hover:bg-white/[0.08] hover:text-[#bbb] transition-colors"
          title={closed ? "Expand sidebar" : "Collapse sidebar"}
        >
          {closed ? <ChevronRight size={13} /> : <ChevronLeft size={13} />}
        </button>
      </div>

      {/* ── Org switcher ───────────────────────── */}
      <Link
        href="/select-organization"
        className={`
          flex items-center flex-shrink-0 no-underline
          border-b border-white/5 hover:bg-white/[0.04] transition-colors
          ${closed ? "justify-center px-2 py-2.5" : "gap-2.5 px-[14px] py-2.5"}
        `}
        title={closed ? (currentOrg?.name ?? "Switch workspace") : undefined}
      >
        <div
          className="w-[30px] h-[30px] rounded-lg flex-shrink-0 flex items-center justify-center text-white text-xs font-bold"
          style={{
            background: "linear-gradient(135deg, #7c3aed, #a855f7)",
            fontFamily: "var(--font-syne, Syne, sans-serif)",
          }}
        >
          {(currentOrg?.name ?? "W")[0].toUpperCase()}
        </div>
        {!closed && (
          <div className="min-w-0">
            <p
              className="text-[0.8rem] font-semibold text-[#e0e0e0] truncate leading-snug"
              style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
            >
              {currentOrg?.name ?? "Workspace"}
            </p>
            <p
              className="text-[0.65rem] text-[#444]"
              style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
            >
              Switch workspace
            </p>
          </div>
        )}
      </Link>

      {/* ── Scrollable nav ─────────────────────── */}
      <nav
        className="flex-1 overflow-y-auto overflow-x-hidden px-1.5 py-2"
        style={{ scrollbarWidth: "none" }}
      >
        {/* COMMON — Tasks */}
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

        {/* MODULES — CRM, HRM, etc. */}
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

                {/* Accordion sub-items — only when live and expanded */}
                {m.status === "live" && !closed && (
                  <div
                    className="overflow-hidden"
                    style={{
                      maxHeight: isOpen
                        ? `${visible.length * itemH + 8}px`
                        : "0px",
                      transition: "max-height 200ms ease-in-out",
                    }}
                  >
                    <div className="ml-3 pl-3 border-l border-white/[0.06] py-0.5 space-y-0.5">
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

        {/* SETTINGS — Members, Roles, Security */}
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

      {/* ── Footer — user profile ───────────────── */}
      <div
        className={`border-t border-white/5 flex-shrink-0 ${closed ? "p-1.5" : "p-2"}`}
      >
        <Link
          href="/settings/profile"
          className={`
            flex items-center no-underline rounded-md hover:bg-white/[0.05] transition-colors
            ${closed ? "justify-center p-2" : "gap-2.5 px-2 py-1.5"}
          `}
          title={closed ? (user?.displayName ?? "Profile") : undefined}
        >
          <div
            className="w-7 h-7 rounded-full flex-shrink-0 flex items-center justify-center text-white text-xs font-bold"
            style={{
              background: "linear-gradient(135deg, #7c3aed, #a855f7)",
              fontFamily: "var(--font-syne, Syne, sans-serif)",
            }}
          >
            {(user?.firstName ?? user?.displayName ?? "?")[0].toUpperCase()}
          </div>
          {!closed && (
            <div className="min-w-0">
              <p
                className="text-[0.78rem] font-medium text-[#d0d0d0] truncate leading-snug"
                style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
              >
                {user?.displayName ?? user?.firstName ?? "User"}
              </p>
              <p
                className="text-[0.65rem] text-[#444] truncate"
                style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
              >
                {user?.email ?? ""}
              </p>
            </div>
          )}
        </Link>
      </div>
    </aside>
  );
}
