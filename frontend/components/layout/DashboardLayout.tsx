"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/hooks/useAuth";
import { usePermission } from "@/hooks/usePermission";
import { Badge } from "@/components/ui/Badge";
import clsx from "clsx";

const NAV = [
  { href: "/dashboard", label: "Overview", icon: "⊞" },
  {
    href: "/dashboard/tasks",
    label: "Tasks",
    icon: "✓",
    perm: "tasks.read" as const,
  },
  { href: "/dashboard/members", label: "Members", icon: "⊙" },
  { href: "/dashboard/profile", label: "Profile", icon: "○" },
];

export function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { user, logout, role, isLoading } = useAuth();
  const { can } = usePermission();

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <svg
            className="animate-spin h-4 w-4 text-brand-600"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
          Loading…
        </div>
      </div>
    );
  }

  const visibleNav = NAV.filter((item) => !item.perm || can(item.perm));

  return (
    <div className="min-h-screen flex bg-gray-50">
      {/* Sidebar */}
      <aside className="w-60 flex-shrink-0 bg-white border-r border-gray-200 flex flex-col">
        {/* Logo */}
        <div className="px-5 h-14 flex items-center border-b border-gray-200">
          <span className="font-semibold text-gray-900 tracking-tight">
            BusinessSAAS
          </span>
          <span className="ml-2 text-2xs font-mono text-gray-400 bg-gray-100 px-1.5 py-0.5 rounded">
            dev
          </span>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-3 py-3 space-y-0.5">
          {visibleNav.map((item) => {
            const active =
              pathname === item.href ||
              (item.href !== "/dashboard" && pathname.startsWith(item.href));
            return (
              <Link
                key={item.href}
                href={item.href}
                className={clsx(
                  "flex items-center gap-2.5 px-3 py-2 rounded text-sm transition-colors",
                  active
                    ? "bg-brand-50 text-brand-700 font-medium"
                    : "text-gray-600 hover:text-gray-900 hover:bg-gray-50",
                )}
              >
                <span className="text-base leading-none w-4 text-center opacity-70">
                  {item.icon}
                </span>
                {item.label}
              </Link>
            );
          })}
        </nav>

        {/* User */}
        <div className="border-t border-gray-200 px-4 py-3">
          {user && (
            <div className="flex items-center gap-2.5 mb-2.5">
              <div className="w-7 h-7 rounded-full bg-brand-100 flex items-center justify-center text-xs font-semibold text-brand-700 flex-shrink-0">
                {user.first_name[0]}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-xs font-medium text-gray-900 truncate">
                  {user.first_name} {user.last_name}
                </p>
                {role && (
                  <Badge variant="neutral" className="mt-0.5 text-2xs">
                    {role}
                  </Badge>
                )}
              </div>
            </div>
          )}
          <button
            onClick={logout}
            className="text-xs text-gray-500 hover:text-red-600 transition-colors w-full text-left"
          >
            Sign out
          </button>
        </div>
      </aside>

      {/* Content */}
      <main className="flex-1 flex flex-col min-w-0">{children}</main>
    </div>
  );
}
