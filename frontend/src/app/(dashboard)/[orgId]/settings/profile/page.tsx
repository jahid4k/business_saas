// src/app/(dashboard)/[orgId]/settings/profile/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import { Camera, Loader2 } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import { getProfile, updateProfile, uploadAvatar } from "@/lib/profile";
import { queryKeys } from "@/lib/queryKeys";
import type { SafeUser } from "@/types/auth";
import Image from "next/image";

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
  bg-[var(--bg-elevated)] border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all disabled:opacity-50 disabled:cursor-not-allowed
`;

// Populates the form + avatar preview from a fetched/updated user record.
// Shared by the initial query-sync effect and the save/upload handlers so
// the "what fields does the form pull from a user object" logic lives in
// exactly one place.
function formValuesFrom(u: SafeUser): ProfileValues {
  return {
    displayName: u.displayName ?? "",
    firstName: u.firstName ?? "",
    lastName: u.lastName ?? "",
    timezone: u.timezone ?? "UTC",
  };
}

export default function ProfilePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  use(params); // orgId available but not needed for /me calls

  const { user, setUser } = useAuthStore();
  const queryClient = useQueryClient();

  const [avatarLoading, setAvatarLoading] = useState(false);
  // Holds ONLY a transient local blob URL while an avatar upload is in
  // flight — never the server's photoURL. Set on file pick, cleared once
  // the upload settles (success or failure), at which point the derived
  // `avatarUrl` below falls through to the query/store value instead.
  // Keeping this state's purpose this narrow means it never needs writing
  // from the query-sync effect, which is what let that effect drop the
  // one remaining useState call that ESLint's set-state-in-effect rule
  // flagged (setUser/reset are external-system updates and are exempt;
  // a local useState setter is not).
  const [localAvatarPreview, setLocalAvatarPreview] = useState<string | null>(
    null,
  );

  const fileInputRef = useRef<HTMLInputElement>(null);
  // Guards the sync effect below so it only ever populates the form once,
  // from whichever data arrives first (cache or network) — see note there.
  const syncedRef = useRef(false);

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

  // ── Query ─────────────────────────────────────────────────────────────────
  // GET /api/v1/me is intentionally fetched here even though authStore.user
  // may already be populated (from login) — this endpoint returns the
  // freshest profile, matching the original component's comment.
  //
  // refetchOnWindowFocus is turned off for this one query: the app-wide
  // default (see QueryProvider) refetches on tab focus, which is fine for
  // read-only dashboards but would be actively harmful on a page holding
  // an unsaved form — a background refetch mid-edit must never overwrite
  // what the person is typing.
  const profileQuery = useQuery({
    queryKey: queryKeys.profile.me(),
    queryFn: getProfile,
    refetchOnWindowFocus: false,
  });

  // Local override (mid-upload) > freshest server copy > last-known store
  // value. This is what the avatar circle actually renders.
  const avatarUrl =
    localAvatarPreview ?? profileQuery.data?.photoURL ?? user?.photoURL ?? null;

  // ── Sync query data → form + authStore (once) ──────────────────────────────
  // This mirrors the two blessed effect uses from the React docs: it's
  // reading from an external system (the query cache) and pushing that
  // into two other external systems (the RHF form instance and the
  // Zustand authStore) — not deriving local component state from props.
  // `syncedRef` keeps it a one-time hydration on first load rather than
  // something that re-fires (and wipes in-progress edits) on every
  // background refetch.
  useEffect(() => {
    if (syncedRef.current) return;

    if (profileQuery.data) {
      syncedRef.current = true;
      setUser(profileQuery.data);
      reset(formValuesFrom(profileQuery.data));
    } else if (profileQuery.isError && user) {
      // Network fetch failed — fall back to whatever profile data we
      // already have from a previous login/org-switch response, same as
      // the original component's .catch() branch.
      syncedRef.current = true;
      reset(formValuesFrom(user));
    }
  }, [profileQuery.data, profileQuery.isError, user, reset, setUser]);

  // ── Handlers ──────────────────────────────────────────────────────────────
  const onSubmit = async (values: ProfileValues) => {
    try {
      const updated = await updateProfile({
        displayName: values.displayName,
        firstName: values.firstName || undefined,
        lastName: values.lastName || undefined,
        timezone: values.timezone || undefined,
      });
      queryClient.setQueryData(queryKeys.profile.me(), updated);
      setUser(updated);
      reset(formValuesFrom(updated));
      toast.success("Profile updated.");
    } catch {
      toast.error("Failed to save profile. Please try again.");
    }
  };

  const handleAvatarClick = () => {
    fileInputRef.current?.click();
  };

  const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Preview immediately — a plain event handler, not an effect, so
    // setting local state here synchronously is the normal, expected thing.
    const blobUrl = URL.createObjectURL(file);
    setLocalAvatarPreview(blobUrl);
    setAvatarLoading(true);

    try {
      const updated = await uploadAvatar(file);
      queryClient.setQueryData(queryKeys.profile.me(), updated);
      setUser(updated);
      toast.success("Avatar updated.");
    } catch {
      toast.error("Failed to upload avatar.");
    } finally {
      URL.revokeObjectURL(blobUrl);
      setLocalAvatarPreview(null);
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
          className="text-2xl font-bold text-(--text-primary) mb-1"
          style={{
            fontFamily: "var(--font-syne, Syne, sans-serif)",
            letterSpacing: "-0.02em",
          }}
        >
          Profile
        </h1>
        <p className="text-sm text-(--text-muted)">
          Manage your personal information
        </p>
      </div>

      {profileQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          Could not refresh your profile from the server — showing your last
          known info.
        </div>
      )}

      {profileQuery.isPending ? (
        <div className="flex items-center gap-3 py-16 text-sm text-(--text-muted)">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading profile…
        </div>
      ) : (
        <div className="space-y-6">
          {/* ── Avatar section ────────────────────── */}
          <div className="flex items-start gap-6 p-6 rounded-xl border border-(--border) bg-(--bg-surface)">
            {/* Avatar */}
            <div className="relative shrink-0">
              <div
                onClick={handleAvatarClick}
                className="w-20 h-20 rounded-full cursor-pointer overflow-hidden relative group"
                style={{
                  background: avatarUrl
                    ? undefined
                    : "linear-gradient(135deg, #7c3aed, #a855f7)",
                }}
              >
                {avatarUrl ? (
                  // <img
                  //   src={avatarUrl}
                  //   alt="Avatar"
                  //   className="w-full h-full object-cover"
                  // />

                  <Image
                    src={avatarUrl}
                    alt="Avatar"
                    fill
                    className="object-cover"
                    sizes="80px"
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
                className="text-lg font-bold text-(--text-primary) mb-0.5"
                style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
              >
                {displayName}
              </p>
              <p className="text-sm text-(--text-muted) mb-3">{user?.email}</p>
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
          <div className="rounded-xl border border-(--border) bg-(--bg-surface) overflow-hidden">
            <div className="px-6 py-4 border-b border-(--border)">
              <p
                className="text-sm font-semibold text-(--text-primary)"
                style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
              >
                Personal information
              </p>
            </div>

            <form onSubmit={handleSubmit(onSubmit)} className="p-6 space-y-5">
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
                  <span className="ml-2 text-xs font-normal text-(--text-muted)">
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
              </div>
            </form>
          </div>

          {/* ── Account information (read only) ─────── */}
          <div className="rounded-xl border border-(--border) bg-(--bg-surface) overflow-hidden">
            <div className="px-6 py-4 border-b border-(--border)">
              <p
                className="text-sm font-semibold text-(--text-primary)"
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
                  <span className="text-sm text-(--text-muted)">
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
