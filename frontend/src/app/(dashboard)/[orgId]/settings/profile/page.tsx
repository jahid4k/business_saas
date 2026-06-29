// src/app/(dashboard)/[orgId]/settings/profile/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Camera, Check, Loader2 } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import { getProfile, updateProfile, uploadAvatar } from "@/lib/profile";

// ── Common timezones ──────────────────────────────────
const TIMEZONES = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Toronto",
  "America/Vancouver",
  "America/Sao_Paulo",
  "Europe/London",
  "Europe/Paris",
  "Europe/Berlin",
  "Europe/Madrid",
  "Europe/Amsterdam",
  "Europe/Rome",
  "Europe/Moscow",
  "Asia/Dubai",
  "Asia/Kolkata",
  "Asia/Dhaka",
  "Asia/Bangkok",
  "Asia/Singapore",
  "Asia/Shanghai",
  "Asia/Tokyo",
  "Asia/Seoul",
  "Australia/Sydney",
  "Australia/Melbourne",
  "Pacific/Auckland",
  "Pacific/Honolulu",
];

// ── Validation ────────────────────────────────────────
const schema = z.object({
  displayName: z.string().min(1, "Display name is required").max(100),
  firstName: z.string().max(50).optional(),
  lastName: z.string().max(50).optional(),
  timezone: z.string().optional(),
});
type ProfileValues = z.infer<typeof schema>;

const cls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all disabled:opacity-50 disabled:cursor-not-allowed
`;

export default function ProfilePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  use(params); // orgId available but not needed for /me calls

  const { user, setUser } = useAuthStore();

  const [pageLoading, setPageLoading] = useState(true);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [avatarLoading, setAvatarLoading] = useState(false);
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null);

  const fileInputRef = useRef<HTMLInputElement>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting, isDirty },
  } = useForm<ProfileValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      displayName: "",
      firstName: "",
      lastName: "",
      timezone: "UTC",
    },
  });

  // Fetch fresh profile data on mount
  useEffect(() => {
    getProfile()
      .then((profile) => {
        setUser(profile);
        reset({
          displayName: profile.displayName ?? "",
          firstName: profile.firstName ?? "",
          lastName: profile.lastName ?? "",
          timezone: profile.timezone ?? "UTC",
        });
        if (profile.photoURL) setAvatarPreview(profile.photoURL);
      })
      .catch(() => {
        // Fall back to authStore data
        if (user) {
          reset({
            displayName: user.displayName ?? "",
            firstName: user.firstName ?? "",
            lastName: user.lastName ?? "",
            timezone: user.timezone ?? "UTC",
          });
          if (user.photoURL) setAvatarPreview(user.photoURL);
        }
      })
      .finally(() => setPageLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const onSubmit = async (values: ProfileValues) => {
    setSaveError(null);
    setSaveSuccess(false);
    try {
      const updated = await updateProfile({
        displayName: values.displayName,
        firstName: values.firstName || undefined,
        lastName: values.lastName || undefined,
        timezone: values.timezone || undefined,
      });
      setUser(updated);
      reset({
        displayName: updated.displayName ?? "",
        firstName: updated.firstName ?? "",
        lastName: updated.lastName ?? "",
        timezone: updated.timezone ?? "UTC",
      });
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 3000);
    } catch {
      setSaveError("Failed to save profile. Please try again.");
    }
  };

  // Avatar click → open file picker
  const handleAvatarClick = () => {
    fileInputRef.current?.click();
  };

  // File selected → upload
  const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Preview immediately
    const url = URL.createObjectURL(file);
    setAvatarPreview(url);
    setAvatarLoading(true);

    try {
      const updated = await uploadAvatar(file);
      setUser(updated);
      if (updated.photoURL) setAvatarPreview(updated.photoURL);
    } catch {
      // Revert preview on error
      setAvatarPreview(user?.photoURL ?? null);
      setSaveError("Failed to upload avatar.");
    } finally {
      setAvatarLoading(false);
      // Reset file input so the same file can be re-selected
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const displayName = user?.displayName ?? user?.firstName ?? "User";
  const initial = displayName[0]?.toUpperCase() ?? "?";

  const formatDate = (iso?: string) =>
    iso
      ? new Date(iso).toLocaleDateString("en-US", {
          month: "long",
          day: "numeric",
          year: "numeric",
        })
      : "—";

  return (
    <div className="p-6 md:p-8 max-w-2xl">
      {/* Header */}
      <div className="mb-8">
        <h1
          className="text-2xl font-bold text-[var(--text-primary)] mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          Profile
        </h1>
        <p className="text-sm text-[var(--text-muted)]">
          Manage your personal information
        </p>
      </div>

      {pageLoading ? (
        <div className="flex items-center gap-3 py-16 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading profile…
        </div>
      ) : (
        <div className="space-y-6">
          {/* ── Avatar section ────────────────────── */}
          <div className="flex items-start gap-6 p-6 rounded-xl border border-[var(--border)] bg-[var(--bg-surface)]">
            {/* Avatar */}
            <div className="relative flex-shrink-0">
              <div
                onClick={handleAvatarClick}
                className="w-20 h-20 rounded-full cursor-pointer overflow-hidden relative group"
                style={{
                  background: avatarPreview
                    ? undefined
                    : "linear-gradient(135deg, #7c3aed, #a855f7)",
                }}
              >
                {avatarPreview ? (
                  <img
                    src={avatarPreview}
                    alt="Avatar"
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <span
                    className="w-full h-full flex items-center justify-center text-2xl font-bold text-white"
                    style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
                  >
                    {initial}
                  </span>
                )}

                {/* Hover overlay */}
                <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
                  {avatarLoading ? (
                    <Loader2 size={18} className="text-white animate-spin" />
                  ) : (
                    <Camera size={18} className="text-white" />
                  )}
                </div>
              </div>

              {/* Hidden file input */}
              <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                className="hidden"
                onChange={handleAvatarChange}
              />
            </div>

            {/* User summary */}
            <div>
              <p
                className="text-lg font-bold text-[var(--text-primary)] mb-0.5"
                style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
              >
                {displayName}
              </p>
              <p className="text-sm text-[var(--text-muted)] mb-3">
                {user?.email}
              </p>
              <button
                onClick={handleAvatarClick}
                disabled={avatarLoading}
                className="text-xs font-medium text-purple-400 hover:text-purple-300 transition-colors disabled:opacity-50"
              >
                {avatarLoading ? "Uploading…" : "Change photo"}
              </button>
            </div>
          </div>

          {/* ── Personal information form ──────────── */}
          <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] overflow-hidden">
            <div className="px-6 py-4 border-b border-[var(--border)]">
              <p
                className="text-sm font-semibold text-[var(--text-primary)]"
                style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
              >
                Personal information
              </p>
            </div>

            <form onSubmit={handleSubmit(onSubmit)} className="p-6 space-y-5">
              {saveError && (
                <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
                  {saveError}
                </div>
              )}

              {/* Display name */}
              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-[var(--text-secondary)]">
                  Display name <span className="text-red-400">*</span>
                </label>
                <input
                  {...register("displayName")}
                  placeholder="How you appear to others"
                  className={cls}
                />
                {errors.displayName && (
                  <p className="text-xs text-red-400">
                    {errors.displayName.message}
                  </p>
                )}
              </div>

              {/* First + Last name */}
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-[var(--text-secondary)]">
                    First name
                  </label>
                  <input
                    {...register("firstName")}
                    placeholder="RBAC"
                    className={cls}
                  />
                </div>
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-[var(--text-secondary)]">
                    Last name
                  </label>
                  <input
                    {...register("lastName")}
                    placeholder="Tester"
                    className={cls}
                  />
                </div>
              </div>

              {/* Email — read only */}
              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-[var(--text-secondary)]">
                  Email
                  <span className="ml-2 text-xs font-normal text-[var(--text-muted)]">
                    read only
                  </span>
                </label>
                <input
                  value={user?.email ?? ""}
                  readOnly
                  className={`${cls} opacity-50 cursor-not-allowed`}
                />
              </div>

              {/* Timezone */}
              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-[var(--text-secondary)]">
                  Timezone
                </label>
                <select {...register("timezone")} className={cls}>
                  {TIMEZONES.map((tz) => (
                    <option
                      key={tz}
                      value={tz}
                      style={{ background: "var(--bg-elevated)" }}
                    >
                      {tz}
                    </option>
                  ))}
                </select>
              </div>

              {/* Submit */}
              <div className="flex items-center gap-3 pt-2">
                <button
                  type="submit"
                  disabled={isSubmitting || !isDirty}
                  className="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 size={14} className="animate-spin" />
                      Saving…
                    </>
                  ) : (
                    "Save changes"
                  )}
                </button>

                {saveSuccess && (
                  <div className="flex items-center gap-1.5 text-sm text-emerald-400">
                    <Check size={14} />
                    Saved!
                  </div>
                )}
              </div>
            </form>
          </div>

          {/* ── Account information (read only) ─────── */}
          <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] overflow-hidden">
            <div className="px-6 py-4 border-b border-[var(--border)]">
              <p
                className="text-sm font-semibold text-[var(--text-primary)]"
                style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
              >
                Account information
              </p>
            </div>

            <div className="divide-y divide-[var(--border)]">
              {[
                { label: "Account type", value: user?.accountType ?? "—" },
                { label: "Status", value: user?.status ?? "—" },
                { label: "Member since", value: formatDate(user?.createdAt) },
                { label: "Last login", value: formatDate(user?.lastLoginAt) },
              ].map((row) => (
                <div
                  key={row.label}
                  className="flex items-center justify-between px-6 py-3.5"
                >
                  <span className="text-sm text-[var(--text-muted)]">
                    {row.label}
                  </span>
                  <span
                    className="text-sm text-[var(--text-secondary)] capitalize"
                    style={{
                      fontFamily: "var(--font-inter, Inter, sans-serif)",
                    }}
                  >
                    {row.value}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
