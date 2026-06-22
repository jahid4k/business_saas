"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ChevronsUpDown, Plus, Check, Building2, Loader2 } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useSession } from "next-auth/react";

import { cn } from "@/lib/utils";
import { apiGet, apiPost, setAccessToken } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { useOrg } from "@/hooks/useOrg";
import { useOrgStore } from "@/store/org";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { MembershipWithOrg } from "@/types/domain";

export function OrgSwitcher() {
  const router = useRouter();
  const { update: updateSession } = useSession();
  const queryClient = useQueryClient();
  const { orgSlug, orgName, orgId } = useOrg();
  const { setActiveOrg } = useOrgStore();
  const [open, setOpen] = useState(false);

  const { data: orgs = [], isLoading } = useQuery({
    queryKey: queryKeys.orgs.list(),
    queryFn: () =>
      apiGet<{ memberships: MembershipWithOrg[] }>("api/v1/organizations").then(
        (d) => d.memberships ?? [],
      ),
  });

  const switchMutation = useMutation({
    mutationFn: async (targetOrgId: string) => {
      const result = await apiPost<{
        access_token: string;
        role: string;
        business_id: string;
      }>(`api/v1/organizations/${targetOrgId}/switch`);

      // 1. Update in-memory token
      setAccessToken(result.access_token);

      // 2. Fetch membership/permissions for this org
      const membership = await apiGet<{ membership: { permissions: string[]; organization: { slug: string; name: string; id: string }; role: { key: string } } }>(
        `api/v1/organizations/${targetOrgId}/members/me`,
      );
      const m = membership.membership;

      // 3. Update Zustand store
      setActiveOrg({
        id: m.organization.id,
        slug: m.organization.slug,
        name: m.organization.name,
        role: m.role?.key ?? result.role,
        permissions: m.permissions ?? [],
      });

      // 4. Sync new token into next-auth session
      await updateSession({
        accessToken: result.access_token,
        activeOrgId: m.organization.id,
        activeOrgSlug: m.organization.slug,
        activeRole: m.role?.key ?? result.role,
      });

      return m.organization.slug;
    },
    onSuccess: (newOrgSlug) => {
      queryClient.invalidateQueries();
      router.push(`/app/${newOrgSlug}/dashboard`);
      setOpen(false);
    },
    onError: () => {
      toast.error("Failed to switch organization");
    },
  });

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-8 gap-1.5 px-2 text-sm font-medium"
          aria-label="Switch organization"
        >
          <Building2 className="h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="max-w-[120px] truncate">{orgName ?? "Select org"}</span>
          <ChevronsUpDown className="h-3 w-3 shrink-0 text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="start" className="w-56">
        <DropdownMenuLabel className="text-xs text-muted-foreground">
          Organizations
        </DropdownMenuLabel>
        <DropdownMenuSeparator />

        {isLoading ? (
          <div className="flex items-center justify-center py-3">
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
          </div>
        ) : orgs.length === 0 ? (
          <div className="px-2 py-3 text-center text-xs text-muted-foreground">
            No organizations
          </div>
        ) : (
          orgs.map((m) => (
            <DropdownMenuItem
              key={m.org_id}
              onClick={() => {
                if (m.org_id !== orgId) {
                  switchMutation.mutate(m.org_id);
                }
              }}
              className={cn(
                "cursor-pointer",
                switchMutation.isPending && "opacity-50 pointer-events-none",
              )}
            >
              <Building2 className="mr-2 h-4 w-4 text-muted-foreground" />
              <span className="flex-1 truncate">{m.organization?.name ?? m.org_id}</span>
              {m.org_id === orgId && (
                <Check className="ml-2 h-3.5 w-3.5 text-primary" />
              )}
            </DropdownMenuItem>
          ))
        )}

        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="cursor-pointer text-muted-foreground"
          onClick={() => router.push("/app/new-org")}
        >
          <Plus className="mr-2 h-4 w-4" />
          Create organization
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
