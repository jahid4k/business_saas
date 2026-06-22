"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronRight } from "lucide-react";
import { cn, slugToTitle } from "@/lib/utils";

// Segments to skip from the breadcrumb trail
const SKIP_SEGMENTS = new Set(["app"]);

// Map raw URL segments to display labels
const SEGMENT_LABELS: Record<string, string> = {
  dashboard: "Dashboard",
  tasks: "Tasks",
  crm: "CRM",
  leads: "Leads",
  deals: "Deals",
  contacts: "Contacts",
  companies: "Companies",
  reports: "Reports",
  settings: "Settings",
  members: "Members",
  roles: "Roles",
  profile: "Profile",
  hrm: "HRM",
  "new-org": "New Organization",
};

function getLabel(segment: string): string {
  return SEGMENT_LABELS[segment] ?? slugToTitle(segment);
}

export function Breadcrumb({ className }: { className?: string }) {
  const pathname = usePathname();

  // Split path into segments, skipping empty strings and route groups
  const segments = pathname
    .split("/")
    .filter((s) => s && !SKIP_SEGMENTS.has(s));

  // Build cumulative paths for each breadcrumb
  const crumbs: { label: string; href: string }[] = [];
  let builtPath = "/app";

  for (const segment of segments) {
    builtPath += `/${segment}`;
    crumbs.push({ label: getLabel(segment), href: builtPath });
  }

  // Only show breadcrumb when there's more than one segment
  if (crumbs.length <= 1) return null;

  return (
    <nav
      aria-label="Breadcrumb"
      className={cn("flex items-center gap-1 text-sm", className)}
    >
      {crumbs.map((crumb, index) => {
        const isLast = index === crumbs.length - 1;
        return (
          <span key={crumb.href} className="flex items-center gap-1">
            {index > 0 && (
              <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/50" />
            )}
            {isLast ? (
              <span className="font-medium text-foreground">{crumb.label}</span>
            ) : (
              <Link
                href={crumb.href}
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                {crumb.label}
              </Link>
            )}
          </span>
        );
      })}
    </nav>
  );
}
