// src/components/members/ResetPasswordForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useDrawer } from "@/contexts/DrawerContext";

const schema = z.object({
  newPassword: z.string().min(8, "Must be at least 8 characters"),
});
type ResetPasswordValues = z.infer<typeof schema>;

interface ResetPasswordFormProps {
  memberName: string;
  onSave: (newPassword: string) => Promise<void>;
}

export default function ResetPasswordForm({
  memberName,
  onSave,
}: ResetPasswordFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ResetPasswordValues>({
    resolver: zodResolver(schema),
  });

  const onSubmit = async (values: ResetPasswordValues) => {
    setError(null);
    try {
      await onSave(values.newPassword);
      closeDrawer();
    } catch {
      setError("Failed to reset password. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="reset-password-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-5"
      >
        <div className="px-4 py-3 rounded-lg text-sm text-amber-400 bg-amber-500/8 border border-amber-500/20">
          {memberName} will be signed out of every active session and must log
          in again with this password.
        </div>

        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {/* New password */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            New password <span className="text-red-400">*</span>
          </label>
          <input
            {...register("newPassword")}
            type="password"
            placeholder="At least 8 characters"
            autoFocus
            autoComplete="new-password"
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
          />
          {errors.newPassword && (
            <p className="text-xs text-red-400">{errors.newPassword.message}</p>
          )}
          <p className="text-xs text-[var(--text-muted)]">
            Share this with {memberName} yourself — there&apos;s no email step
            yet, so nothing gets sent automatically.
          </p>
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
          form="reset-password-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Resetting…" : "Reset password"}
        </button>
      </div>
    </div>
  );
}
