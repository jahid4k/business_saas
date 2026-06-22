"use client";

import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { useSession } from "next-auth/react";
import { toast } from "sonner";

import { createOrgSchema, deriveSlug, type CreateOrgInput } from "@/lib/validations/orgs";
import { apiPost, setAccessToken } from "@/lib/api";
import { useOrgStore } from "@/store/org";
import { ApiError } from "@/types/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";

interface CreateOrgDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateOrgDialog({ open, onOpenChange }: CreateOrgDialogProps) {
  const router = useRouter();
  const { update: updateSession } = useSession();
  const { setActiveOrg } = useOrgStore();

  const form = useForm<CreateOrgInput>({
    resolver: zodResolver(createOrgSchema),
    defaultValues: { name: "", slug: "" },
  });

  const createMutation = useMutation({
    mutationFn: async (data: CreateOrgInput) => {
      // 1. Create org
      const orgData = await apiPost<{
        organization: { id: string; slug: string; name: string };
      }>("api/v1/organizations", data);

      const orgId = orgData.organization.id;

      // 2. Switch into the new org
      const switchData = await apiPost<{
        access_token: string;
        role: string;
      }>(`api/v1/organizations/${orgId}/switch`);

      setAccessToken(switchData.access_token);

      // 3. Get permissions
      const membershipData = await apiPost<{
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
        role: m.role?.key ?? switchData.role,
        permissions: m.permissions ?? [],
      });

      await updateSession({
        accessToken: switchData.access_token,
        activeOrgId: m.organization.id,
        activeOrgSlug: m.organization.slug,
        activeRole: m.role?.key ?? switchData.role,
      });

      return m.organization.slug;
    },
    onSuccess: (slug) => {
      toast.success("Organization created");
      onOpenChange(false);
      router.push(`/app/${slug}/dashboard`);
    },
    onError: (error) => {
      if (error instanceof ApiError && error.hasFieldErrors) {
        Object.entries(error.fields!).forEach(([field, message]) => {
          form.setError(field as keyof CreateOrgInput, { message });
        });
      } else {
        toast.error("Failed to create organization");
      }
    },
  });

  function onNameChange(name: string) {
    form.setValue("name", name);
    // Auto-derive slug only if user hasn't manually edited it
    if (!form.formState.dirtyFields.slug) {
      form.setValue("slug", deriveSlug(name));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create organization</DialogTitle>
          <DialogDescription>
            Create a new workspace for your team
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((d) => createMutation.mutate(d))}
            className="space-y-4"
          >
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Organization name</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="Acme Corp"
                      {...field}
                      onChange={(e) => onNameChange(e.target.value)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="slug"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>URL slug</FormLabel>
                  <FormControl>
                    <Input placeholder="acme-corp" {...field} />
                  </FormControl>
                  <FormDescription>
                    Used in your workspace URL: /app/
                    <strong>{field.value || "acme-corp"}</strong>
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
              >
                Cancel
              </Button>
              <Button type="submit" loading={createMutation.isPending}>
                Create organization
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
