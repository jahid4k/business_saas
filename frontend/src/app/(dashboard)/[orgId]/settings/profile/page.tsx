// src/app/(dashboard)/[orgId]/settings/profile/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import { Camera, Loader2, Plus, X } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import {
  getProfile,
  updateProfile,
  uploadAvatar,
  listAvatars,
  activateAvatar,
  deleteAvatar,
  apiErrorCode,
} from "@/lib/profile";
import { queryKeys } from "@/lib/queryKeys";
import { resolveAssetUrl } from "@/lib/constants";
import AvatarCropModal from "@/components/settings/AvatarCropModal";
import type { SafeUser, UserAvatar } from "@/types/auth";
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
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all disabled:opacity-50 disabled:cursor-not-allowed
`;

function formValuesFrom(u: SafeUser): ProfileValues {
  return {
    displayName: u.displayName ?? "",
    firstName: u.firstName ?? "",
    lastName: u.lastName ?? "",
    timezone: u.timezone ?? "UTC",
  };
}

// ── One thumbnail in the avatar gallery ────────────────
// Inline, page-specific component — same convention as ContactAvatar /
// CompanyAvatar elsewhere in the app (small presentational pieces live
// alongside the one page that uses them, not in their own file).
function AvatarGalleryItem({
  avatar,
  onActivate,
  onDelete,
  activating,
  deleting,
}: {
  avatar: UserAvatar;
  onActivate: () => void;
  onDelete: () => void;
  activating: boolean;
  deleting: boolean;
}) {
  console.log(avatar);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const url = resolveAssetUrl(avatar.url);
  const busy = activating || deleting;

  return (
    <div className="relative group pt-2">
      <button
        onClick={avatar.isActive ? undefined : onActivate}
        disabled={avatar.isActive || busy}
        className={`relative w-16 h-16 rounded-full overflow-hidden border-2 transition-colors ${
          avatar.isActive
            ? "border-purple-500"
            : "border-transparent hover:border-purple-500/40"
        } ${avatar.isActive || busy ? "cursor-default" : "cursor-pointer"}`}
        title={avatar.isActive ? "Currently active" : "Set as active"}
      >
        {url && (
          <Image
            src={url}
            alt="Stored avatar"
            className="w-full h-full object-cover"
            width={64}
            height={64}
            unoptimized
          />
        )}
        {activating && (
          <div className="absolute inset-0 bg-black/50 flex items-center justify-center">
            <Loader2 size={16} className="text-white animate-spin" />
          </div>
        )}
      </button>

      {avatar.isActive && (
        <span className="absolute -bottom-1 left-1/2 -translate-x-1/2 text-[0.55rem] font-semibold text-purple-400 bg-(--bg-surface) px-1.5 py-px rounded-full border border-purple-500/30 whitespace-nowrap">
          Active
        </span>
      )}

      {!confirmDelete ? (
        <button
          onClick={() => setConfirmDelete(true)}
          disabled={deleting}
          className="absolute top-0 right-0 w-5 h-5 rounded-full bg-red-500 text-white opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity disabled:opacity-50"
          title="Delete this avatar"
        >
          <X size={11} />
        </button>
      ) : (
        <div className="absolute -top-2 left-1/2 -translate-x-1/2 flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-(--bg-elevated) border border-(--border) shadow-lg whitespace-nowrap z-10">
          <button
            onClick={onDelete}
            disabled={deleting}
            className="text-xs font-semibold text-red-400 disabled:opacity-50"
          >
            {deleting ? "…" : "Delete?"}
          </button>
          <button
            onClick={() => setConfirmDelete(false)}
            className="text-xs text-(--text-muted)"
          >
            No
          </button>
        </div>
      )}
    </div>
  );
}

export default function ProfilePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  use(params); // orgId available but not needed for /me calls

  const { user, setUser } = useAuthStore();
  const queryClient = useQueryClient();

  const fileInputRef = useRef<HTMLInputElement>(null);
  const syncedRef = useRef(false);

  // Object URL of a just-picked file, shown in the crop modal. null = no
  // crop flow in progress. Created in handleFileSelected, revoked whenever
  // the flow ends (confirm or cancel) in closeCropModal.
  const [pendingImageSrc, setPendingImageSrc] = useState<string | null>(null);
  const [activatingId, setActivatingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

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

  // ── Queries ───────────────────────────────────────────────────────────────
  const profileQuery = useQuery({
    queryKey: queryKeys.profile.me(),
    queryFn: getProfile,
    refetchOnWindowFocus: false, // never clobber an in-progress edit on tab refocus
  });

  const avatarsQuery = useQuery({
    queryKey: queryKeys.profile.avatars(),
    queryFn: listAvatars,
  });

  const avatarUrl = resolveAssetUrl(user?.photoURL) ?? null;
  const avatarCount = avatarsQuery.data?.avatars.length ?? 0;
  const avatarMax = avatarsQuery.data?.max ?? 3;
  const canAddAvatar = avatarCount < avatarMax;

  // ── Sync query data → form + authStore (once) ──────────────────────────────
  useEffect(() => {
    if (syncedRef.current) return;

    if (profileQuery.data) {
      syncedRef.current = true;
      setUser(profileQuery.data);
      reset(formValuesFrom(profileQuery.data));
    } else if (profileQuery.isError && user) {
      syncedRef.current = true;
      reset(formValuesFrom(user));
    }
  }, [profileQuery.data, profileQuery.isError, user, reset, setUser]);

  // ── Mutations ─────────────────────────────────────────────────────────────
  // All three share the same shape: the backend always returns the updated
  // SafeUser, so profile.me() and authStore get patched directly; the small
  // avatars list (max 3 items, infrequent user-initiated action — not a hot
  // path) is simply invalidated rather than hand-patched, which is far less
  // error-prone than replicating the backend's dedup/promotion logic here.
  const uploadMutation = useMutation({
    mutationFn: (blob: Blob) => uploadAvatar(blob),
    onSuccess: ({ user: updated }) => {
      queryClient.setQueryData(queryKeys.profile.me(), updated);
      setUser(updated);
      queryClient.invalidateQueries({ queryKey: queryKeys.profile.avatars() });
      toast.success("Avatar updated.");
    },
    onError: (err) => {
      const code = apiErrorCode(err);
      if (code === "AVATAR_LIMIT_REACHED") {
        toast.error(
          `You can store up to ${avatarMax} avatars — delete one first.`,
        );
      } else if (code === "INVALID_AVATAR_TYPE") {
        toast.error("That file isn't a supported image type.");
      } else {
        toast.error("Failed to upload avatar.");
      }
    },
  });

  const activateMutation = useMutation({
    mutationFn: (avatarId: string) => activateAvatar(avatarId),
    onMutate: (avatarId) => setActivatingId(avatarId),
    onSuccess: (updated) => {
      queryClient.setQueryData(queryKeys.profile.me(), updated);
      setUser(updated);
      queryClient.invalidateQueries({ queryKey: queryKeys.profile.avatars() });
      toast.success("Avatar switched.");
    },
    onError: () => toast.error("Failed to switch avatar."),
    onSettled: () => setActivatingId(null),
  });

  const deleteMutation = useMutation({
    mutationFn: (avatarId: string) => deleteAvatar(avatarId),
    onMutate: (avatarId) => setDeletingId(avatarId),
    onSuccess: (updated) => {
      queryClient.setQueryData(queryKeys.profile.me(), updated);
      setUser(updated);
      queryClient.invalidateQueries({ queryKey: queryKeys.profile.avatars() });
      toast.success("Avatar deleted.");
    },
    onError: () => toast.error("Failed to delete avatar."),
    onSettled: () => setDeletingId(null),
  });

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
    if (!canAddAvatar) {
      toast.error(
        `You're at your limit of ${avatarMax} avatars — delete one to add another.`,
      );
      return;
    }
    fileInputRef.current?.click();
  };

  const handleFileSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    // Reset the input immediately so picking the exact same file again
    // still fires this handler next time.
    if (fileInputRef.current) fileInputRef.current.value = "";
    if (!file) return;
    setPendingImageSrc(URL.createObjectURL(file));
  };

  const closeCropModal = () => {
    if (pendingImageSrc) URL.revokeObjectURL(pendingImageSrc);
    setPendingImageSrc(null);
  };

  const handleCropConfirm = async (blob: Blob) => {
    try {
      await uploadMutation.mutateAsync(blob);
    } finally {
      closeCropModal();
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
          <div className="p-6 rounded-xl border border-(--border) bg-(--bg-surface)">
            <div className="flex items-start gap-6 mb-6">
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
                    <img
                      src={avatarUrl}
                      alt="Avatar"
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <span
                      className="w-full h-full flex items-center justify-center text-2xl font-bold text-white"
                      style={{
                        fontFamily: "var(--font-syne, Syne, sans-serif)",
                      }}
                    >
                      {initial}
                    </span>
                  )}

                  <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
                    {uploadMutation.isPending ? (
                      <Loader2 size={18} className="text-white animate-spin" />
                    ) : (
                      <Camera size={18} className="text-white" />
                    )}
                  </div>
                </div>

                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp,image/gif"
                  className="hidden"
                  onChange={handleFileSelected}
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
                <p className="text-sm text-(--text-muted) mb-3">
                  {user?.email}
                </p>
                <button
                  onClick={handleAvatarClick}
                  disabled={uploadMutation.isPending}
                  className="text-xs font-medium text-purple-400 hover:text-purple-300 transition-colors disabled:opacity-50"
                >
                  {uploadMutation.isPending ? "Uploading…" : "Change photo"}
                </button>
              </div>
            </div>

            {/* ── Avatar gallery / management ──────── */}
            <div className="pt-5 border-t border-(--border)">
              <div className="flex items-center justify-between mb-3">
                <p className="text-xs font-semibold text-(--text-muted) uppercase tracking-wider">
                  Your avatars
                </p>
                <span className="text-xs text-(--text-muted)">
                  {avatarCount} of {avatarMax}
                </span>
              </div>

              {avatarsQuery.isPending ? (
                <div className="flex items-center gap-2 text-xs text-(--text-muted)">
                  <Loader2 size={12} className="animate-spin" />
                  Loading…
                </div>
              ) : (
                <div className="flex items-center gap-4 flex-wrap">
                  {(avatarsQuery.data?.avatars ?? []).map((a) => (
                    <AvatarGalleryItem
                      key={a.id}
                      avatar={a}
                      onActivate={() => activateMutation.mutate(a.id)}
                      onDelete={() => deleteMutation.mutate(a.id)}
                      activating={activatingId === a.id}
                      deleting={deletingId === a.id}
                    />
                  ))}

                  {canAddAvatar ? (
                    <button
                      onClick={handleAvatarClick}
                      className="w-16 h-16 mt-2 rounded-full border-2 border-dashed border-(--border) hover:border-purple-500/40 flex items-center justify-center text-(--text-muted) hover:text-purple-400 transition-colors"
                      title="Add a new avatar"
                    >
                      <Plus size={18} />
                    </button>
                  ) : (
                    avatarCount > 0 && (
                      <p className="text-xs text-(--text-muted) max-w-[160px] leading-snug">
                        Delete one above to add a new photo.
                      </p>
                    )
                  )}
                </div>
              )}
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
              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-(--text-secondary)">
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

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    First name
                  </label>
                  <input
                    {...register("firstName")}
                    placeholder="RBAC"
                    className={cls}
                  />
                </div>
                <div className="space-y-1.5">
                  <label className="block text-sm font-medium text-(--text-secondary)">
                    Last name
                  </label>
                  <input
                    {...register("lastName")}
                    placeholder="Tester"
                    className={cls}
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-(--text-secondary)">
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

              <div className="space-y-1.5">
                <label className="block text-sm font-medium text-(--text-secondary)">
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

            <div className="divide-y divide-(--border)">
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
                    className="text-sm text-(--text-secondary) capitalize"
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

      {pendingImageSrc && (
        <AvatarCropModal
          imageSrc={pendingImageSrc}
          onConfirm={handleCropConfirm}
          onCancel={closeCropModal}
        />
      )}
    </div>
  );
}
