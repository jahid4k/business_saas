"use client";

import { use, type ElementType } from "react";
import Link from "next/link";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";
import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "@/lib/queryKeys";
import { getDashboardMetrics } from "@/lib/dashboard";
import {
  // CRM
  Users,
  TrendingUp,
  BarChart2,
  // HRM
  UserCircle,
  CalendarDays,
  Banknote,
  // General / KPIs
  CheckSquare,
  ArrowRight,
  Bell,
  Clock,
  Target,
  DollarSign,
  Loader2,
  Building2,
} from "lucide-react";

// --- Data Structures ---

const QUICK_LINKS_CRM = [
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

const QUICK_LINKS_HRM = [
  {
    label: "Employees",
    desc: "Team directory",
    icon: UserCircle,
    href: "hrm/employees",
    perm: "hrm.employees.view",
  },
  {
    label: "Leave",
    desc: "Manage time off",
    icon: CalendarDays,
    href: "hrm/leave",
    perm: "hrm.leave.view",
  },
  {
    label: "Payroll",
    desc: "Salary & compensation",
    icon: Banknote,
    href: "hrm/payroll",
    perm: "hrm.payroll.view",
  },
  {
    label: "Attendance",
    desc: "Track work hours",
    icon: Clock,
    href: "hrm/attendance",
    perm: "hrm.attendance.view",
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

  const { data: dashboard, isLoading } = useQuery({
    queryKey: queryKeys.dashboard.metrics(orgId),
    queryFn: () => getDashboardMetrics(orgId),
  });

  const firstName = user?.firstName ?? user?.displayName ?? "there";
  const orgName = currentOrg?.name ?? "your workspace";

  return (
    <div className="p-8 max-w-6xl mx-auto bg-base min-h-screen">
      {/* ── Greeting ── */}
      <div className="mb-10 flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <h1 className="font-syne text-3xl font-bold tracking-tight text-gray-900 dark:text-white mb-2">
            Good {timeGreeting()}, {firstName} 👋
          </h1>
          <p className="text-gray-500 dark:text-[#888] text-sm max-w-xl">
            You&apos;re viewing{" "}
            <span className="text-gray-800 dark:text-[#ddd] font-medium">
              {orgName}
            </span>
            . Here&apos;s your pulse check on sales and team operations today.
          </p>
        </div>
      </div>

      {/* ── KPI Pulse Check ── */}
      {isLoading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="w-8 h-8 text-gray-400 animate-spin" />
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-5 mb-10">
          <KPICard
            title="Active Pipeline"
            value={new Intl.NumberFormat("en-US", {
              style: "currency",
              currency: "USD",
              maximumFractionDigits: 0,
            }).format(dashboard?.kpis.active_pipeline_value || 0)}
            trend="Current Open Deals"
            icon={DollarSign}
            color="blue"
          />
          <KPICard
            title="Total Headcount"
            value={(dashboard?.kpis.total_headcount || 0).toString()}
            trend="Active Employees"
            icon={Users}
            color="purple"
          />
          <KPICard
            title="Pending Approvals"
            value={(dashboard?.kpis.pending_approvals || 0).toString()}
            trend="Requires Action"
            icon={Bell}
            color="orange"
            alert={(dashboard?.kpis.pending_approvals || 0) > 0}
          />
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* ── Main Content Area (Quick Links) ── */}
        <div className="lg:col-span-2 space-y-8">
          {/* CRM Links */}
          <section>
            <div className="flex items-center gap-2 mb-4">
              <Target size={18} className="text-blue-500" />
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                Sales & CRM
              </h2>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {QUICK_LINKS_CRM.filter((l) => hasPermission(l.perm)).map(
                (link) => (
                  <QuickLinkCard
                    key={link.href}
                    link={link}
                    orgId={orgId}
                    color="blue"
                  />
                ),
              )}
            </div>
          </section>

          {/* HRM Links */}
          <section>
            <div className="flex items-center gap-2 mb-4">
              <UserCircle size={18} className="text-purple-500" />
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                Team & HR
              </h2>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {QUICK_LINKS_HRM.filter((l) => hasPermission(l.perm)).map(
                (link) => (
                  <QuickLinkCard
                    key={link.href}
                    link={link}
                    orgId={orgId}
                    color="purple"
                  />
                ),
              )}
            </div>
          </section>
        </div>

        {/* ── Sidebar (Action Center & Workspace Info) ── */}
        <div className="space-y-6">
          {/* Workspace Info Card */}
          <div className="rounded-2xl border border-gray-200 dark:border-white/8 bg-white dark:bg-[#0f0f0f] p-5 shadow-sm">
            <h3 className="text-base font-semibold text-gray-900 dark:text-white flex items-center gap-2 mb-4">
              <Building2 size={16} className="text-indigo-500" />
              Workspace Info
            </h3>

            <div className="space-y-3">
              <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500 dark:text-[#888]">
                  Legal Name
                </span>
                <span className="text-gray-900 dark:text-[#eee] font-medium">
                  {currentOrg?.legalName || "—"}
                </span>
              </div>
              <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500 dark:text-[#888]">Industry</span>
                <span className="text-gray-900 dark:text-[#eee] font-medium">
                  {currentOrg?.industry || "—"}
                </span>
              </div>
              <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500 dark:text-[#888]">Type</span>
                <span className="text-gray-900 dark:text-[#eee] font-medium">
                  {currentOrg?.type || "—"}
                </span>
              </div>
              <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500 dark:text-[#888]">Timezone</span>
                <span className="text-gray-900 dark:text-[#eee] font-medium">
                  {currentOrg?.timezone || "UTC"}
                </span>
              </div>
            </div>

            <Link
              href={`/${orgId}/settings/workspace`}
              className="w-full flex justify-center items-center mt-5 py-2.5 rounded-lg text-sm font-medium text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-500/10 hover:bg-indigo-100 dark:hover:bg-indigo-500/20 transition-colors no-underline"
            >
              Manage Workspace
            </Link>
          </div>

          <div className="rounded-2xl border border-gray-200 dark:border-white/8 bg-white dark:bg-[#0f0f0f] p-5 shadow-sm">
            <h3 className="text-base font-semibold text-gray-900 dark:text-white flex items-center gap-2 mb-4">
              <CheckSquare size={16} className="text-green-500" />
              Needs Attention
            </h3>

            <div className="space-y-3">
              {isLoading ? (
                <div className="flex justify-center py-4">
                  <Loader2 className="w-5 h-5 text-gray-400 animate-spin" />
                </div>
              ) : dashboard?.action_items &&
                dashboard.action_items.length > 0 ? (
                dashboard.action_items.map((item) => (
                  <ActionItem
                    key={item.id}
                    title={item.title}
                    desc={item.description}
                    time={formatTimeAgo(item.timestamp)}
                    actionUrl={`/${orgId}/${item.action_url}`}
                  />
                ))
              ) : (
                <p className="text-sm text-gray-500 text-center py-4">
                  No pending action items.
                </p>
              )}
            </div>

            <button className="w-full mt-5 py-2.5 rounded-lg text-sm font-medium text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/4 hover:bg-gray-100 dark:hover:bg-white/8 transition-colors border border-gray-200 dark:border-white/5">
              View All Tasks
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// --- Subcomponents ---

interface KPICardProps {
  title: string;
  value: string;
  trend: string;
  icon: ElementType;
  color: "blue" | "purple" | "orange";
  alert?: boolean;
}

function KPICard({
  title,
  value,
  trend,
  icon: Icon,
  color,
  alert,
}: KPICardProps) {
  const colorStyles: Record<string, string> = {
    blue: "from-blue-500/10 to-transparent border-blue-500/20 text-blue-600 dark:text-blue-400",
    purple:
      "from-purple-500/10 to-transparent border-purple-500/20 text-purple-600 dark:text-purple-400",
    orange:
      "from-orange-500/10 to-transparent border-orange-500/20 text-orange-600 dark:text-orange-400",
  };
  const activeStyle = colorStyles[color] || "";

  return (
    <div
      className={`relative overflow-hidden rounded-2xl border border-gray-200 dark:border-white/8 bg-white dark:bg-[#0f0f0f] p-5 shadow-sm transition-all hover:shadow-md`}
    >
      <div
        className={`absolute top-0 right-0 w-32 h-32 bg-linear-to-bl ${activeStyle.split(" ").slice(0, 2).join(" ")} rounded-full blur-3xl -mr-10 -mt-10 opacity-50 pointer-events-none`}
      />

      <div className="flex items-start justify-between mb-4">
        <p className="text-sm font-medium text-gray-500 dark:text-[#888]">
          {title}
        </p>
        <div
          className={`p-2 rounded-lg bg-gray-50 dark:bg-white/4 ${alert ? "animate-pulse" : ""}`}
        >
          <Icon
            size={18}
            className={activeStyle
              .split(" ")
              .find((c: string) => c.startsWith("text-"))}
          />
        </div>
      </div>

      <div>
        <h3 className="text-3xl font-bold text-gray-900 dark:text-white mb-1 tracking-tight">
          {value}
        </h3>
        <p className="text-xs font-medium text-gray-400 dark:text-[#666]">
          {trend}
        </p>
      </div>
    </div>
  );
}

interface QuickLinkCardProps {
  link: {
    href: string;
    icon: ElementType;
    label: string;
    desc: string;
    perm: string;
  };
  orgId: string;
  color: "blue" | "purple";
}

function QuickLinkCard({ link, orgId, color }: QuickLinkCardProps) {
  const Icon = link.icon;

  const hoverColors: Record<string, string> = {
    blue: "hover:border-blue-400/40 dark:hover:border-blue-500/30 hover:bg-blue-50/60 dark:hover:bg-blue-500/6 text-blue-600 dark:text-blue-400 group-hover:text-blue-500",
    purple:
      "hover:border-purple-400/40 dark:hover:border-purple-500/30 hover:bg-purple-50/60 dark:hover:bg-purple-500/6 text-purple-600 dark:text-purple-400 group-hover:text-purple-500",
  };
  const activeHover = hoverColors[color] || "";

  const iconBg =
    color === "blue"
      ? "bg-blue-100 dark:bg-blue-500/12"
      : "bg-purple-100 dark:bg-purple-500/12";

  return (
    <Link
      href={`/${orgId}/${link.href}`}
      className={`
        group flex items-center gap-3.5 px-4 py-4 rounded-xl
        border border-gray-200 dark:border-white/[0.07]
        bg-white dark:bg-[#0f0f0f] no-underline
        transition-all duration-200 ease-out shadow-sm
        ${activeHover}
      `}
    >
      <div
        className={`w-10 h-10 shrink-0 rounded-lg ${iconBg} flex items-center justify-center`}
      >
        <Icon
          size={18}
          className={activeHover
            .split(" ")
            .find((c: string) => c.startsWith("text-"))}
        />
      </div>

      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-gray-800 dark:text-[#e0e0e0] mb-0.5">
          {link.label}
        </p>
        <p className="text-xs text-gray-400 dark:text-[#666] truncate">
          {link.desc}
        </p>
      </div>

      <ArrowRight
        size={14}
        className={`shrink-0 text-gray-300 dark:text-[#444] transition-transform duration-200 group-hover:translate-x-1 ${activeHover.split(" ").find((c: string) => c.startsWith("group-hover:text-"))}`}
      />
    </Link>
  );
}

interface ActionItemProps {
  title: string;
  desc: string;
  time: string;
  actionUrl?: string;
}

function ActionItem({ title, desc, time, actionUrl }: ActionItemProps) {
  const content = (
    <>
      <div className="w-2 h-2 mt-2 rounded-full bg-orange-500 shrink-0" />
      <div>
        <p className="text-sm font-medium text-gray-800 dark:text-[#ddd] mb-0.5">
          {title}
        </p>
        <p className="text-xs text-gray-500 dark:text-[#777] mb-1">{desc}</p>
        <p className="text-[10px] uppercase font-bold tracking-wider text-gray-400 dark:text-[#555]">
          {time}
        </p>
      </div>
    </>
  );

  const cls =
    "group flex gap-3 p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-white/3 transition-colors cursor-pointer border border-transparent hover:border-gray-100 dark:hover:border-white/5 no-underline";

  if (actionUrl) {
    return (
      <Link href={actionUrl} className={cls}>
        {content}
      </Link>
    );
  }

  return <div className={cls}>{content}</div>;
}

function timeGreeting() {
  const h = new Date().getHours();
  if (h < 12) return "morning";
  if (h < 17) return "afternoon";
  return "evening";
}

function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (seconds < 60) return "Just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;

  return date.toLocaleDateString();
}
