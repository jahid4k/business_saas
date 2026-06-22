"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useSession } from "next-auth/react";
import { Building2, Plus, ArrowRight, Loader2 } from "lucide-react";

import { apiGet, apiPost, setAccessToken } from "@/lib/api";
import { useOrgStore } from "@/store/org";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { CreateOrgDialog } from "./CreateOrgDialog";
import type { MembershipWithOrg } from "@/types/domain";

export function OrgSelectionView() {
  const router = useRouter();
  const { update: updateSession } = useSession();
  const { setActiveOrg } = useOrgStore();
  const [createOpen, setCreateOpen] = useState(false);

  const { data: orgs = [], isLoading } = useQuery({
    queryKey: ["orgs-list"],
    queryFn: () =>
      apiGet<{ memberships: MembershipWithOrg[] }>("api/v1/organizations").then(
        (d) => d.memberships ?? [],
      ),
  });

  const switchMutation = useMutation({
    mutationFn: async (orgId: string) => {
      const result = await apiPost<{
        access_token: string;
        role: string;
      }>(`api/v1/organizations/${orgId}/switch`);

      setAccessToken(result.access_token);

      const membershipData = await apiGet<{
        membership: {
          permissions: string[];
          organization: { slug: string; name: string; id: string };
          role: { key: string };
        };
      }>(`api/v1/organizations/${orgId}/members/me`);

      const m = membershipData.membership;

      setActiveOrg({
        id: m.organization.id,
        slug: m.organization.slug,
        name: m.organization.name,
        role: m.role?.key ?? result.role,
        permissions: m.permissions ?? [],
      });

      await updateSession({
        accessToken: result.access_token,
        activeOrgId: m.organization.id,
        activeOrgSlug: m.organization.slug,
        activeRole: m.role?.key ?? result.role,
      });

      return m.organization.slug;
    },
    onSuccess: (slug) => {
      router.push(`/app/${slug}/dashboard`);
    },
  });

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-muted/30 px-4 py-12">
      <div className="mb-8 text-center">
        <div className="mb-4 flex justify-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
            <Building2 className="h-6 w-6 text-primary-foreground" />
          </div>
        </div>
        <h1 className="text-2xl font-semibold">Select an organization</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Choose which workspace you want to work in
        </p>
      </div>

      <div className="w-full max-w-md space-y-3">
        {isLoading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : orgs.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">No organizations yet</CardTitle>
              <CardDescription>
                Create your first organization to get started
              </CardDescription>
            </CardHeader>
          </Card>
        ) : (
          orgs.map((m) => (
            <button
              key={m.org_id}
              onClick={() => switchMutation.mutate(m.org_id)}
              disabled={switchMutation.isPending}
              className="group w-full rounded-xl border bg-card p-4 text-left shadow-sm transition-all hover:border-primary/50 hover:shadow-md disabled:opacity-50"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
                    <Building2 className="h-4 w-4 text-primary" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">
                      {m.organization?.name ?? m.org_id}
                    </p>
                    <p className="text-xs capitalize text-muted-foreground">
                      {m.role_key}
                    </p>
                  </div>
                </div>
                {switchMutation.isPending &&
                switchMutation.variables === m.org_id ? (
                  <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                ) : (
                  <ArrowRight className="h-4 w-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
                )}
              </div>
            </button>
          ))
        )}

        <Button
          variant="outline"
          className="w-full"
          onClick={() => setCreateOpen(true)}
        >
          <Plus className="mr-2 h-4 w-4" />
          Create new organization
        </Button>
      </div>

      <CreateOrgDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  );
}
