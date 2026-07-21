// src/components/members/InviteForm.tsx
"use client";

import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";
import type { Role } from "@/types/rbac";

const schema = z.object({
  email: z.string().min(1, "Email is required").email("Enter a valid email"),
  role: z.string().min(1, "Role is required"),
});
type InviteValues = z.infer<typeof schema>;

interface InviteFormProps {
  orgRoles: Role[];
  onSave: (email: string, role: string) => Promise<void>;
}

export default function InviteForm({ orgRoles, onSave }: InviteFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const assignableRoles = orgRoles.filter((r) => r.name !== "owner");

  const {
    register,
    handleSubmit,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<InviteValues>({
    resolver: zodResolver(schema),
    defaultValues: { role: "member" },
  });

  useEffect(() => {
    if (
      assignableRoles.length > 0 &&
      !assignableRoles.some((r) => r.name === "member")
    ) {
      setValue("role", assignableRoles[0].name);
    }
  }, [assignableRoles, setValue]);

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
          <label className="block text-sm font-medium text-(--text-secondary)">
            Email address <span className="text-red-400">*</span>
          </label>
          <input
            {...register("email")}
            type="email"
            placeholder="colleague@company.com"
            autoFocus
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary) placeholder:text-(--text-muted) outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
          />
          {errors.email && (
            <p className="text-xs text-red-400">{errors.email.message}</p>
          )}
        </div>

        {/* Role */}
        <div className="space-y-2">
          <label className="block text-sm font-medium text-(--text-secondary)">
            Role <span className="text-red-400">*</span>
          </label>
          <div className="space-y-2">
            {assignableRoles.map((r) => (
              <label
                key={r.id}
                className="flex items-start gap-3 p-3 rounded-lg border border-(--border) cursor-pointer hover:border-purple-500/40 hover:bg-purple-500/5 transition-all has-checked:border-purple-500 has-checked:bg-purple-500/8"
              >
                <input
                  {...register("role")}
                  type="radio"
                  value={r.name}
                  className="mt-0.5 accent-purple-500 shrink-0"
                />
                <div>
                  <p className="text-sm font-medium text-(--text-primary) capitalize">
                    {r.name}
                  </p>
                  <p className="text-xs text-(--text-muted)">
                    {r.description}
                  </p>
                </div>
              </label>
            ))}
          </div>
        </div>
      </form>

      {/* Footer */}
      <div className="flex items-center gap-3 px-6 py-4 border-t border-(--border) shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-(--text-secondary) border border-(--border) hover:bg-(--bg-elevated) transition-colors"
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
