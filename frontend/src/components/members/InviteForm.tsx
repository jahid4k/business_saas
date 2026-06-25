// src/components/members/InviteForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { MemberRole } from "@/types/rbac";

const ROLES: { value: MemberRole; label: string; desc: string }[] = [
  { value: "admin", label: "Admin", desc: "Broad management access" },
  { value: "manager", label: "Manager", desc: "Project and member visibility" },
  { value: "member", label: "Member", desc: "Standard access" },
  { value: "viewer", label: "Viewer", desc: "Read-only access" },
];

const schema = z.object({
  email: z.string().min(1, "Email is required").email("Enter a valid email"),
  role: z.enum(["admin", "manager", "member", "viewer"]),
});
type InviteValues = z.infer<typeof schema>;

interface InviteFormProps {
  onSave: (email: string, role: MemberRole) => Promise<void>;
}

export default function InviteForm({ onSave }: InviteFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<InviteValues>({
    resolver: zodResolver(schema),
    defaultValues: { role: "member" },
  });

  const onSubmit = async (values: InviteValues) => {
    setError(null);
    try {
      await onSave(values.email, values.role);
      closeDrawer();
    } catch {
      setError("Failed to send invitation. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="invite-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-5"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {/* Email */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Email address <span className="text-red-400">*</span>
          </label>
          <input
            {...register("email")}
            type="email"
            placeholder="colleague@company.com"
            autoFocus
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
          />
          {errors.email && (
            <p className="text-xs text-red-400">{errors.email.message}</p>
          )}
        </div>

        {/* Role */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Role <span className="text-red-400">*</span>
          </label>
          <div className="space-y-2">
            {ROLES.map((r) => (
              <label
                key={r.value}
                className="flex items-start gap-3 p-3 rounded-lg border border-[var(--border)] cursor-pointer hover:border-purple-500/40 hover:bg-purple-500/5 transition-all has-[:checked]:border-purple-500 has-[:checked]:bg-purple-500/8"
              >
                <input
                  {...register("role")}
                  type="radio"
                  value={r.value}
                  className="mt-0.5 accent-purple-500 flex-shrink-0"
                />
                <div>
                  <p className="text-sm font-medium text-[var(--text-primary)]">
                    {r.label}
                  </p>
                  <p className="text-xs text-[var(--text-muted)]">{r.desc}</p>
                </div>
              </label>
            ))}
          </div>
        </div>
      </form>

      {/* Footer */}
      <div className="flex items-center gap-3 px-6 py-4 border-t border-[var(--border)] flex-shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-elevated)] transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          form="invite-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Sending…" : "Send invitation"}
        </button>
      </div>
    </div>
  );
}
