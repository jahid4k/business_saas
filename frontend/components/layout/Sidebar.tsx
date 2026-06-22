"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Users,
  CheckSquare,
  Settings,
  ChevronLeft,
  ChevronRight,
  Building2,
  BarChart3,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useUIStore } from "@/store/ui";
import { useOrg } from "@/hooks/useOrg";
import { Button } from "@/components/ui/button";

interface NavItem {
  label: string;
  href: string;
  icon: React.ComponentType<{ className?: string }>;
}

function useNavItems(orgSlug: string | null): NavItem[] {
  if (!orgSlug) return [];
  const base = `/app/${orgSlug}`;
  return [
    { label: "Dashboard", href: `${base}/dashboard`, icon: LayoutDashboard },
    { label: "Tasks", href: `${base}/tasks`, icon: CheckSquare },
    { label: "CRM", href: `${base}/crm`, icon: BarChart3 },
    { label: "Members", href: `${base}/settings/members`, icon: Users },
    { label: "Settings", href: `${base}/settings`, icon: Settings },
  ];
}

export function Sidebar() {
  const pathname = usePathname();
  const { sidebarCollapsed, toggleSidebar } = useUIStore();
  const { orgSlug, orgName } = useOrg();
  const navItems = useNavItems(orgSlug);

  return (
    <aside
      className={cn(
        "relative flex h-full flex-col border-r bg-card transition-all duration-300",
        sidebarCollapsed ? "w-16" : "w-60",
      )}
    >
      {/* Logo / Org name */}
      <div className="flex h-14 items-center border-b px-3">
        <div className="flex items-center gap-2 overflow-hidden">
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-primary">
            <Building2 className="h-4 w-4 text-primary-foreground" />
          </div>
          {!sidebarCollapsed && (
            <span className="truncate text-sm font-semibold">
              {orgName ?? "BusinessSAAS"}
            </span>
          )}
        </div>
      </div>

      {/* Nav items */}
      <nav className="flex flex-1 flex-col gap-1 p-2">
        {navItems.map((item) => {
          const isActive =
            pathname === item.href || pathname.startsWith(`${item.href}/`);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 rounded-md px-2 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-primary/10 text-primary"
                  : "text-muted-foreground hover:bg-accent hover:text-foreground",
              )}
            >
              <item.icon className="h-4 w-4 shrink-0" />
              {!sidebarCollapsed && (
                <span className="truncate">{item.label}</span>
              )}
            </Link>
          );
        })}
      </nav>

      {/* Collapse toggle */}
      <div className="border-t p-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={toggleSidebar}
          className="w-full justify-center"
          aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
        >
          {sidebarCollapsed ? (
            <ChevronRight className="h-4 w-4" />
          ) : (
            <>
              <ChevronLeft className="h-4 w-4" />
              <span className="ml-1 text-xs">Collapse</span>
            </>
          )}
        </Button>
      </div>
    </aside>
  );
}
