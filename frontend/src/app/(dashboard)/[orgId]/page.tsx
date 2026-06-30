// // src/app/(dashboard)/[orgId]/page.tsx
// "use client";

// import { use } from "react";
// import Link from "next/link";
// import { useAuthStore } from "@/stores/authStore";
// import { usePermissionStore } from "@/stores/permissionStore";
// import {
//   CheckSquare,
//   Users,
//   TrendingUp,
//   BarChart2,
//   ArrowRight,
// } from "lucide-react";

// const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
// const FONT_INTER = "var(--font-inter, Inter, sans-serif)";
// const PURPLE = "#7c3aed";

// const QUICK_LINKS = [
//   {
//     label: "Tasks",
//     desc: "Manage your to-dos",
//     icon: CheckSquare,
//     href: "tasks",
//     perm: "tasks.view",
//   },
//   {
//     label: "Leads",
//     desc: "Track new prospects",
//     icon: Users,
//     href: "crm/leads",
//     perm: "crm.leads.view",
//   },
//   {
//     label: "Pipeline",
//     desc: "Visualise your deals",
//     icon: TrendingUp,
//     href: "crm/pipeline",
//     perm: "crm.deals.view",
//   },
//   {
//     label: "Reports",
//     desc: "CRM analytics",
//     icon: BarChart2,
//     href: "crm/reports",
//     perm: "crm.reports.view",
//   },
// ];

// export default function OrgDashboardPage({
//   params,
// }: {
//   params: Promise<{ orgId: string }>;
// }) {
//   const { orgId } = use(params);
//   const { currentOrg, user } = useAuthStore();
//   const { hasPermission } = usePermissionStore();

//   const firstName = user?.firstName ?? user?.displayName ?? "there";
//   const orgName = currentOrg?.name ?? "your workspace";

//   return (
//     <div
//       style={{ padding: "36px 32px", maxWidth: 800, fontFamily: FONT_INTER }}
//     >
//       {/* Greeting */}
//       <h1
//         style={{
//           fontFamily: FONT_SYNE,
//           fontSize: "1.6rem",
//           fontWeight: 700,
//           color: "white",
//           letterSpacing: "-0.02em",
//           marginBottom: 6,
//         }}
//       >
//         Good {timeGreeting()}, {firstName} 👋
//       </h1>
//       <p style={{ fontSize: "0.875rem", color: "#666", marginBottom: 36 }}>
//         You&apos;re in <span style={{ color: "#aaa" }}>{orgName}</span>.
//         Here&apos;s a quick overview.
//       </p>

//       {/* Quick access cards */}
//       <div
//         style={{
//           display: "grid",
//           gridTemplateColumns: "repeat(2, 1fr)",
//           gap: 12,
//         }}
//       >
//         {QUICK_LINKS.filter((l) => hasPermission(l.perm)).map((link) => {
//           const Icon = link.icon;
//           return (
//             <Link
//               key={link.href}
//               href={`/${orgId}/${link.href}`}
//               style={{
//                 display: "flex",
//                 alignItems: "center",
//                 gap: 14,
//                 padding: "16px 18px",
//                 borderRadius: 10,
//                 border: "1px solid rgba(255,255,255,0.07)",
//                 background: "#0f0f0f",
//                 textDecoration: "none",
//                 transition: "border-color 150ms ease, background 150ms ease",
//                 cursor: "pointer",
//               }}
//               onMouseEnter={(e) => {
//                 e.currentTarget.style.borderColor = "rgba(124,58,237,0.35)";
//                 e.currentTarget.style.background = "rgba(124,58,237,0.06)";
//               }}
//               onMouseLeave={(e) => {
//                 e.currentTarget.style.borderColor = "rgba(255,255,255,0.07)";
//                 e.currentTarget.style.background = "#0f0f0f";
//               }}
//             >
//               <div
//                 style={{
//                   width: 38,
//                   height: 38,
//                   borderRadius: 9,
//                   background: "rgba(124,58,237,0.12)",
//                   display: "flex",
//                   alignItems: "center",
//                   justifyContent: "center",
//                   flexShrink: 0,
//                 }}
//               >
//                 <Icon size={17} style={{ color: PURPLE }} />
//               </div>
//               <div style={{ flex: 1, minWidth: 0 }}>
//                 <p
//                   style={{
//                     fontSize: "0.875rem",
//                     fontWeight: 500,
//                     color: "#e0e0e0",
//                     fontFamily: FONT_INTER,
//                     marginBottom: 2,
//                   }}
//                 >
//                   {link.label}
//                 </p>
//                 <p
//                   style={{
//                     fontSize: "0.75rem",
//                     color: "#555",
//                     fontFamily: FONT_INTER,
//                   }}
//                 >
//                   {link.desc}
//                 </p>
//               </div>
//               <ArrowRight size={14} style={{ color: "#333", flexShrink: 0 }} />
//             </Link>
//           );
//         })}
//       </div>

//       {/* Next steps note */}
//       <div
//         style={{
//           marginTop: 28,
//           padding: "14px 16px",
//           borderRadius: 8,
//           border: "1px solid rgba(124,58,237,0.15)",
//           background: "rgba(124,58,237,0.05)",
//         }}
//       >
//         <p
//           style={{
//             fontSize: "0.8rem",
//             color: "#7c3aed",
//             fontWeight: 600,
//             marginBottom: 3,
//           }}
//         >
//           Step 5 complete ✓
//         </p>
//         <p style={{ fontSize: "0.8rem", color: "#555" }}>
//           Dashboard shell is live. Next: build individual feature pages (Tasks,
//           CRM, etc.)
//         </p>
//       </div>
//     </div>
//   );
// }

// function timeGreeting() {
//   const h = new Date().getHours();
//   if (h < 12) return "morning";
//   if (h < 17) return "afternoon";
//   return "evening";
// }

// src/app/(dashboard)/[orgId]/page.tsx
"use client";

import { use } from "react";
import Link from "next/link";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";
import {
  CheckSquare,
  Users,
  TrendingUp,
  BarChart2,
  ArrowRight,
} from "lucide-react";

const QUICK_LINKS = [
  {
    label: "Tasks",
    desc: "Manage your to-dos",
    icon: CheckSquare,
    href: "tasks",
    perm: "tasks.view",
  },
  {
    label: "Leads",
    desc: "Track new prospects",
    icon: Users,
    href: "crm/leads",
    perm: "crm.leads.view",
  },
  {
    label: "Pipeline",
    desc: "Visualise your deals",
    icon: TrendingUp,
    href: "crm/pipeline",
    perm: "crm.deals.view",
  },
  {
    label: "Reports",
    desc: "CRM analytics",
    icon: BarChart2,
    href: "crm/reports",
    perm: "crm.reports.view",
  },
];

export default function OrgDashboardPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { currentOrg, user } = useAuthStore();
  const { hasPermission } = usePermissionStore();

  const firstName = user?.firstName ?? user?.displayName ?? "there";
  const orgName = currentOrg?.name ?? "your workspace";

  return (
    <div className="p-8 max-w-2xl bg-base">
      {/* ── Greeting ── */}
      <h1 className="font-syne text-[1.6rem] font-bold tracking-tight text-gray-900 dark:text-white mb-1.5">
        Good {timeGreeting()}, {firstName} 👋
      </h1>
      <p className="text-sm text-gray-500 dark:text-[#666] mb-9">
        You&apos;re in{" "}
        <span className="text-gray-700 dark:text-[#aaa]">{orgName}</span>.
        Here&apos;s a quick overview.
      </p>

      {/* ── Quick access cards ── */}
      <div className="grid grid-cols-2 gap-3">
        {QUICK_LINKS.filter((l) => hasPermission(l.perm)).map((link) => {
          const Icon = link.icon;
          return (
            <Link
              key={link.href}
              href={`/${orgId}/${link.href}`}
              className="
                group
                flex items-center gap-3.5
                px-4.5 py-4
                rounded-[10px]
                border border-gray-200 dark:border-white/[0.07]
                bg-white dark:bg-[#0f0f0f]
                no-underline
                transition-all duration-150 ease-out
                hover:border-purple-400/40 dark:hover:border-purple-600/35
                hover:bg-purple-50/60 dark:hover:bg-purple-600/[0.06]
              "
            >
              {/* Icon container */}
              <div
                className="
                  w-[38px] h-[38px] shrink-0
                  rounded-[9px]
                  bg-purple-100 dark:bg-purple-600/[0.12]
                  flex items-center justify-center
                "
              >
                <Icon
                  size={17}
                  className="text-purple-600 dark:text-[#7c3aed]"
                />
              </div>

              {/* Labels */}
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-800 dark:text-[#e0e0e0] mb-0.5">
                  {link.label}
                </p>
                <p className="text-xs text-gray-400 dark:text-[#555]">
                  {link.desc}
                </p>
              </div>

              {/* Arrow */}
              <ArrowRight
                size={14}
                className="
                  shrink-0
                  text-gray-300 dark:text-[#333]
                  transition-transform duration-150
                  group-hover:translate-x-0.5
                  group-hover:text-purple-400 dark:group-hover:text-purple-600
                "
              />
            </Link>
          );
        })}
      </div>

      {/* ── Next steps note ── */}
      <div
        className="
          mt-7 px-4 py-3.5
          rounded-lg
          border border-purple-300/30 dark:border-purple-600/[0.15]
          bg-purple-50/50 dark:bg-purple-600/[0.05]
        "
      >
        <p className="text-[0.8rem] font-semibold text-purple-600 dark:text-[#7c3aed] mb-0.5">
          Step 5 complete ✓
        </p>
        <p className="text-[0.8rem] text-gray-400 dark:text-[#555]">
          Dashboard shell is live. Next: build individual feature pages (Tasks,
          CRM, etc.)
        </p>
      </div>
    </div>
  );
}

function timeGreeting() {
  const h = new Date().getHours();
  if (h < 12) return "morning";
  if (h < 17) return "afternoon";
  return "evening";
}
