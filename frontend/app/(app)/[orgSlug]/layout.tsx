"use client";

// [orgSlug]/layout.tsx
// Runs on every org-scoped page.
// Resolves the URL slug → org ID and loads the user's permissions.
// Stores both in Zustand so all child components can read them.

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { apiGet } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { useOrgStore } from "@/store/org";
import { Skeleton } from "@/components/ui/skeleton";
import type { MyMembership } from "@/types/domain";

export default function OrgLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const params = useParams<{ orgSlug: string }>();
  const router = useRouter();
  const { setActiveOrg, activeOrgSlug } = useOrgStore();

  // Fetch my membership for this org (includes org details + permissions)
  // We use the slug to find the org — query the orgs list first, then membership
  const { data, isLoading, isError } = useQuery({
    queryKey: [...queryKeys.members.me("by-slug"), params.orgSlug],
    queryFn: async () => {
      // First get orgs list to find the ID from slug
      const orgsData = await apiGet<{ memberships: Array<{ org_id: string; organization: { slug: string; id: string } }> }>(
        "api/v1/organizations",
      );

      const matched = orgsData.memberships?.find(
        (m) => m.organization?.slug === params.orgSlug,
      );

      if (!matched) throw new Error("Organization not found");

      // Now fetch full membership with permissions
      const membershipData = await apiGet<{ membership: MyMembership }>(
        `api/v1/organizations/${matched.org_id}/members/me`,
      );

      return membershipData.membership;
    },
    enabled: Boolean(params.orgSlug),
    staleTime: 60 * 1000, // 1 minute
    retry: 1,
  });

  // Load org context into Zustand whenever the slug changes or data loads
  useEffect(() => {
    if (data && params.orgSlug) {
      setActiveOrg({
        id: data.organization.id,
        slug: data.organization.slug,
        name: data.organization.name,
        role: data.role?.key ?? data.role_key ?? "member",
        permissions: data.permissions ?? [],
      });
    }
  }, [data, params.orgSlug, setActiveOrg]);

  // Redirect if org not found or user is not a member
  useEffect(() => {
    if (isError) {
      router.push("/app");
    }
  }, [isError, router]);

  if (isLoading && activeOrgSlug !== params.orgSlug) {
    return (
      <div className="p-6 space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-72" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return <>{children}</>;
}
