// src/app/(dashboard)/[orgId]/settings/members/page.tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { UserPlus, MoreHorizontal, Mail, Clock, Loader2 } from "lucide-react";
import gsap from "gsap";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import {
  listMembers,
  inviteMember,
  updateMemberRole,
  updateMemberStatus,
  resendInvitation,
  cancelInvitation,
  resetMemberPassword,
  updateMemberPermissions,
} from "@/lib/members";
import { listRoles } from "@/lib/roles";
import { toast } from "sonner";
import { queryKeys } from "@/lib/queryKeys";
import InviteForm from "@/components/members/InviteForm";
import ResetPasswordForm from "@/components/members/ResetPasswordForm";
import MemberPermissionsForm from "@/components/members/MemberPermissionsForm";
import type { Member, MemberRole } from "@/types/rbac";

const ROLE_STYLE: Record<string, { label: string; cls: string }> = {
  owner: {
    label: "Owner",
    cls: "text-purple-400 bg-purple-500/10 border-purple-500/20",
  },
  admin: {
    label: "Admin",
    cls: "text-blue-400   bg-blue-500/10   border-blue-500/20",
  },
  manager: {
    label: "Manager",
    cls: "text-teal-400   bg-teal-500/10   border-teal-500/20",
  },
  member: {
    label: "Member",
    cls: "text-green-400  bg-green-500/10  border-green-500/20",
  },
  viewer: {
    label: "Viewer",
    cls: "text-zinc-400   bg-zinc-500/10   border-zinc-500/20",
  },
};

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function Avatar({ name, email }: { name: string; email: string }) {
  const initial = (name || email)[0]?.toUpperCase() ?? "?";
  const hue = email.split("").reduce((n, c) => n + c.charCodeAt(0), 0) % 360;
  return (
    <div
      className="w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-xs font-bold text-white"
      style={{ background: `hsl(${hue},55%,42%)` }}
    >
      {initial}
    </div>
  );
}

export default function MembersPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { user } = useAuthStore();
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();

  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<string | null>(null);
  const [roleLoading, setRoleLoading] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);
  const menuRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const canInvite = hasPermission("members.invite");
  const canUpdate = hasPermission("members.update");
  const canRemove = hasPermission("members.remove");
  const canResetPassword = hasPermission("members.password_reset");
  const canViewMemberPerms = hasPermission("members.permissions.view");
  const canEditMemberPerms = hasPermission("members.permissions.update");

  // ── Query ─────────────────────────────────────────────────────────────────
  const membersKey = queryKeys.members.list(orgId);
  const membersQuery = useQuery({
    queryKey: membersKey,
    queryFn: () => listMembers(orgId),
  });
  const members = membersQuery.data ?? [];
  const active = members.filter((m) => m.status === "active");
  const pending = members.filter((m) => m.status === "pending");

  const rolesKey = queryKeys.roles.list(orgId);
  const rolesQuery = useQuery({
    queryKey: rolesKey,
    queryFn: () => listRoles(orgId),
  });
  const allRoles = rolesQuery.data ?? [];
  const assignableRoles = allRoles.filter((r) => r.role.name !== "owner");

  // ── GSAP ──────────────────────────────────────────────────────────────────
  useEffect(() => {
    if (membersQuery.isPending || !listRef.current) return;
    const rows = listRef.current.querySelectorAll(".member-row");
    if (rows.length) {
      gsap.fromTo(
        rows,
        { opacity: 0, y: 6 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.035, ease: "power2.out" },
      );
    }
  }, [membersQuery.isPending]);

  // ── Close menu on outside click ───────────────────────────────────────────
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      let inside = false;
      menuRefs.current.forEach((el) => {
        if (el?.contains(e.target as Node)) inside = true;
      });
      if (!inside) setOpenMenu(null);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  // ── Handlers ──────────────────────────────────────────────────────────────
  const handleInvite = () => {
    openDrawer({
      title: "Invite member",
      width: "md",
      content: (
        <InviteForm
          orgRoles={allRoles.map((r) => r.role)}
          onSave={async (email, role) => {
            await inviteMember(orgId, { email, role });
            // inviteMember returns void; invalidate to get the new pending member
            await queryClient.invalidateQueries({ queryKey: membersKey });
            toast.success("Invitation sent.");
          }}
        />
      ),
    });
  };

  const handleResetPassword = (m: Member) => {
    setOpenMenu(null);
    openDrawer({
      title: `Reset password — ${m.displayName}`,
      width: "md",
      content: (
        <ResetPasswordForm
          memberName={m.displayName}
          onSave={async (newPassword) => {
            await resetMemberPassword(orgId, m.membershipId, newPassword);
            toast.success(`Password reset for ${m.displayName}.`);
          }}
        />
      ),
    });
  };

  const handleManagePermissions = (m: Member) => {
    setOpenMenu(null);
    openDrawer({
      title: `Permissions — ${m.displayName}`,
      width: "lg",
      content: (
        <MemberPermissionsForm
          orgId={orgId}
          membershipId={m.membershipId}
          memberName={m.displayName}
          onSave={async (customPermissions, deniedPermissions) => {
            await updateMemberPermissions(orgId, m.membershipId, {
              customPermissions,
              deniedPermissions,
            });
            await queryClient.invalidateQueries({
              queryKey: queryKeys.members.permissions(orgId, m.membershipId),
            });
            toast.success(`Permissions updated for ${m.displayName}.`);
          }}
        />
      ),
    });
  };

  const handleRoleChange = async (m: Member, role: MemberRole) => {
    setRoleLoading(m.membershipId);
    toast.error(null);
    try {
      await updateMemberRole(orgId, m.membershipId, role);
      queryClient.setQueryData<Member[]>(membersKey, (old) =>
        (old ?? []).map((x) =>
          x.membershipId === m.membershipId ? { ...x, role } : x,
        ),
      );
      toast.success("Role updated.");
    } catch {
      toast.error("Failed to update role.");
    } finally {
      setRoleLoading(null);
    }
  };

  const handleRemove = async (membershipId: string) => {
    toast.error(null);
    try {
      await updateMemberStatus(orgId, membershipId, "inactive");
      queryClient.setQueryData<Member[]>(membersKey, (old) =>
        (old ?? []).filter((m) => m.membershipId !== membershipId),
      );
    } catch {
      toast.error("Failed to remove member.");
      toast.success("Member removed.");
    }
    setConfirm(null);
    setOpenMenu(null);
  };

  const handleResend = async (membershipId: string) => {
    toast.error(null);
    try {
      await resendInvitation(orgId, membershipId);
      toast.success("Invitation resent.");
    } catch {
      toast.error("Failed to resend invitation.");
    }
  };

  const handleCancelInvite = async (membershipId: string) => {
    toast.error(null);
    try {
      await cancelInvitation(orgId, membershipId);
      queryClient.setQueryData<Member[]>(membersKey, (old) =>
        (old ?? []).filter((m) => m.membershipId !== membershipId),
      );
    } catch {
      toast.error("Failed to cancel invitation.");
      toast.success("Invitation cancelled.");
    }
  };

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div className="p-6 md:p-8 max-w-4xl">
      <div className="flex items-start justify-between mb-8">
        <div>
          <h1
            className="text-2xl font-bold text-(--text-primary) mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Members
          </h1>
          <p className="text-sm text-(--text-muted)">
            {active.length} active · {pending.length} pending
          </p>
        </div>
        {canInvite && (
          <button
            onClick={handleInvite}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <UserPlus size={15} />
            Invite member
          </button>
        )}
      </div>

      {membersQuery.isError && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          Failed to load members. Please refresh.
        </div>
      )}

      {membersQuery.isPending ? (
        <div className="flex items-center gap-3 py-16 text-sm text-(--text-muted)">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading members…
        </div>
      ) : (
        <div ref={listRef} className="space-y-6">
          {/* Active members */}
          <section>
            <p
              className="text-[0.65rem] font-semibold text-(--text-muted) uppercase tracking-widest mb-3"
              style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
            >
              Active members ({active.length}) fasdfasd
            </p>
            <div className="rounded-xl border border-(--border) divide-y divide-(--border)">
              {active.length === 0 ? (
                <p className="px-5 py-8 text-sm text-center text-(--text-muted)">
                  No active members yet.
                </p>
              ) : (
                active.map((m) => {
                  const isMe = m.userId === user?.id;
                  const isOwner = m.role === "owner";
                  const menuOpen = openMenu === m.membershipId;
                  const confirming = confirm === m.membershipId;
                  const roleStyle = ROLE_STYLE[m.role] || {
                    label: m.role.charAt(0).toUpperCase() + m.role.slice(1),
                    cls: "text-zinc-400 bg-zinc-500/10 border-zinc-500/20",
                  };

                  return (
                    <div
                      key={m.membershipId}
                      className={`member-row group flex items-center gap-4 px-5 py-3.5 bg-(--bg-surface) hover:bg-(--bg-elevated) transition-colors ${menuOpen ? "relative z-50" : "relative z-0"
                        }`}
                    >
                      <Avatar name={m.displayName} email={m.email} />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium text-(--text-primary) truncate">
                            {m.displayName}
                          </p>
                          {isMe && (
                            <span className="text-[0.6rem] font-semibold text-purple-400 bg-purple-500/10 border border-purple-500/20 px-1.5 py-0.5 rounded-full">
                              You
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-(--text-muted) truncate">
                          {m.email}
                        </p>
                      </div>

                      {canUpdate && !isMe && !isOwner ? (
                        <div className="flex items-center gap-1.5">
                          {roleLoading === m.membershipId && (
                            <Loader2
                              size={12}
                              className="animate-spin text-purple-400"
                            />
                          )}
                          <select
                            value={m.role}
                            onChange={(e) =>
                              handleRoleChange(m, e.target.value as MemberRole)
                            }
                            disabled={roleLoading === m.membershipId}
                            className="text-xs px-2.5 py-1.5 rounded-md bg-(--bg-elevated) border border-(--border) text-(--text-secondary) outline-none focus:border-purple-500 transition-colors cursor-pointer disabled:opacity-50"
                          >
                            {assignableRoles.map((r) => (
                              <option
                                key={r.role.id}
                                value={r.role.name}
                                style={{ background: "var(--bg-elevated)" }}
                              >
                                {r.role.name.charAt(0).toUpperCase() +
                                  r.role.name.slice(1)}
                              </option>
                            ))}
                          </select>
                        </div>
                      ) : (
                        <span
                          className={`text-[0.65rem] font-semibold border px-2 py-0.5 rounded-full ${roleStyle.cls}`}
                        >
                          {roleStyle.label}
                        </span>
                      )}

                      <span className="text-xs text-(--text-muted) shrink-0 hidden sm:block">
                        {formatDate(m.joinedAt)}
                      </span>

                      {confirming ? (
                        <div className="flex items-center gap-2 shrink-0">
                          <span className="text-xs text-(--text-muted)">
                            Remove?
                          </span>
                          <button
                            onClick={() => handleRemove(m.membershipId)}
                            className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                          >
                            Yes
                          </button>
                          <button
                            onClick={() => setConfirm(null)}
                            className="px-2.5 py-1 rounded-md text-xs text-(--text-secondary) hover:bg-(--bg-elevated) transition-colors"
                          >
                            No
                          </button>
                        </div>
                      ) : (canRemove ||
                        canResetPassword ||
                        canViewMemberPerms ||
                        canEditMemberPerms) &&
                        !isMe &&
                        !isOwner ? (
                        <div
                          className="relative shrink-0"
                          ref={(el) => {
                            if (el) menuRefs.current.set(m.membershipId, el);
                            else menuRefs.current.delete(m.membershipId);
                          }}
                        >
                          <button
                            onClick={() =>
                              setOpenMenu(menuOpen ? null : m.membershipId)
                            }
                            className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 text-(--text-muted) hover:bg-(--bg-elevated) transition-all"
                          >
                            <MoreHorizontal size={15} />
                          </button>
                          {menuOpen && (
                            <div className="absolute right-0 top-full mt-1.5 w-44 rounded-xl bg-(--bg-elevated) border border-(--border) shadow-xl z-20">
                              {canResetPassword && (
                                <button
                                  onClick={() => handleResetPassword(m)}
                                  className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) transition-colors text-left"
                                >
                                  Reset password
                                </button>
                              )}
                              {(canViewMemberPerms || canEditMemberPerms) && (
                                <button
                                  onClick={() => handleManagePermissions(m)}
                                  className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-(--text-secondary) hover:bg-(--bg-surface) transition-colors text-left"
                                >
                                  {canEditMemberPerms
                                    ? "Manage permissions"
                                    : "View permissions"}
                                </button>
                              )}
                              {canRemove && (
                                <button
                                  onClick={() => {
                                    setConfirm(m.membershipId);
                                    setOpenMenu(null);
                                  }}
                                  className="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-sm text-red-400 hover:bg-red-500/10 transition-colors text-left"
                                >
                                  Remove member
                                </button>
                              )}
                            </div>
                          )}
                        </div>
                      ) : (
                        <div className="w-[30px] shrink-0" />
                      )}
                    </div>
                  );
                })
              )}
            </div>
          </section>

          {/* Pending invitations */}
          {pending.length > 0 && (
            <section>
              <p
                className="text-[0.65rem] font-semibold text-(--text-muted) uppercase tracking-widest mb-3"
                style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
              >
                Pending invitations ({pending.length})
              </p>
              <div className="rounded-xl border border-(--border) divide-y divide-(--border)">
                {pending.map((m) => {
                  const roleStyle = ROLE_STYLE[m.role] || {
                    label: m.role.charAt(0).toUpperCase() + m.role.slice(1),
                    cls: "text-zinc-400 bg-zinc-500/10 border-zinc-500/20",
                  };
                  return (
                    <div
                      key={m.membershipId}
                      className="member-row flex items-center gap-4 px-5 py-3.5 bg-(--bg-surface)"
                    >
                      <div className="w-8 h-8 rounded-full shrink-0 flex items-center justify-center bg-(--bg-elevated) border border-(--border)">
                        <Mail size={13} className="text-(--text-muted)" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-(--text-secondary) truncate">
                          {m.email}
                        </p>
                        <div className="flex items-center gap-1.5 mt-0.5">
                          <Clock size={10} className="text-(--text-muted)" />
                          <p className="text-xs text-(--text-muted)">
                            Invited {formatDate(m.joinedAt)}
                          </p>
                        </div>
                      </div>
                      <span
                        className={`text-[0.65rem] font-semibold border px-2 py-0.5 rounded-full shrink-0 ${roleStyle.cls}`}
                      >
                        {roleStyle.label}
                      </span>
                      <div className="flex items-center gap-2 shrink-0">
                        {canInvite && (
                          <button
                            onClick={() => handleResend(m.membershipId)}
                            className="text-xs text-purple-400 hover:text-purple-300 transition-colors"
                          >
                            Resend
                          </button>
                        )}
                        {canInvite && (
                          <button
                            onClick={() => handleCancelInvite(m.membershipId)}
                            className="text-xs text-(--text-muted) hover:text-red-400 transition-colors"
                          >
                            Cancel
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </section>
          )}
        </div>
      )}
    </div>
  );
}
