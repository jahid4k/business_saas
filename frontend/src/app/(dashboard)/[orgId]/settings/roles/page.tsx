// src/app/(dashboard)/[orgId]/settings/roles/page.tsx
"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import { Shield, Copy, Trash2, Loader2, ChevronRight } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import gsap from "gsap";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import {
  listRoles,
  listPermissions,
  cloneRole,
  deleteRole,
  updateRolePermissions,
} from "@/lib/roles";
import PermissionForm from "@/components/roles/PermissionForm";
import type { Role, RoleWithMeta, Permission } from "@/types/rbac";

// ── Role display colours ──────────────────────────────
const ROLE_COLOR: Record<string, string> = {
  owner: "text-purple-400 bg-purple-500/10 border-purple-500/20",
  admin: "text-blue-400   bg-blue-500/10   border-blue-500/20",
  manager: "text-teal-400   bg-teal-500/10   border-teal-500/20",
  member: "text-green-400  bg-green-500/10  border-green-500/20",
  viewer: "text-zinc-400   bg-zinc-500/10   border-zinc-500/20",
};

// ── Clone form (inline component) ─────────────────────
const cloneSchema = z.object({
  name: z.string().min(2, "At least 2 characters").max(64),
});
type CloneValues = z.infer<typeof cloneSchema>;

function CloneForm({
  sourceRole,
  onSave,
}: {
  sourceRole: Role;
  onSave: (name: string) => Promise<void>;
}) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<CloneValues>({
    resolver: zodResolver(cloneSchema),
    defaultValues: { name: `${sourceRole.name} (copy)` },
  });

  const onSubmit = async ({ name }: CloneValues) => {
    setError(null);
    try {
      await onSave(name);
      closeDrawer();
    } catch {
      setError("Failed to clone role. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="clone-form"
        onSubmit={handleSubmit(onSubmit)}
        className="flex-1 px-6 py-5 space-y-5"
      >
        <div className="flex items-center gap-3 p-4 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)]">
          <Shield size={16} className="text-purple-400 flex-shrink-0" />
          <div>
            <p className="text-sm font-medium text-[var(--text-primary)] capitalize">
              {sourceRole.name}
            </p>
            <p className="text-xs text-[var(--text-muted)]">
              {sourceRole.permissionKeys.length} permissions will be copied
            </p>
          </div>
        </div>

        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            New role name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("name")}
            autoFocus
            className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-primary)] outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>
      </form>

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
          form="clone-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? "Cloning…" : "Clone role"}
        </button>
      </div>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────
export default function RolesPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();

  const [roles, setRoles] = useState<RoleWithMeta[]>([]);
  const [allPerms, setAllPerms] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(true);
  const [pageErr, setPageErr] = useState<string | null>(null);
  const [delConfirm, setDelConfirm] = useState<string | null>(null);

  const listRef = useRef<HTMLDivElement>(null);

  const canViewPerms = hasPermission("roles.view");
  const canClone = hasPermission("roles.clone");
  const canDelete = hasPermission("roles.delete");
  const canEditPerms = hasPermission("roles.permissions.update");

  // Fetch
  const fetch = useCallback(async () => {
    setLoading(true);
    setPageErr(null);
    try {
      const [r, p] = await Promise.all([
        listRoles(orgId),
        listPermissions(orgId),
      ]);
      setRoles(r);
      setAllPerms(p);
    } catch {
      setPageErr("Failed to load roles. Please refresh.");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    fetch();
  }, [fetch]);

  // GSAP
  useEffect(() => {
    if (loading || !listRef.current) return;
    const cards = listRef.current.querySelectorAll(".role-card");
    if (cards.length) {
      gsap.fromTo(
        cards,
        { opacity: 0, y: 8 },
        { opacity: 1, y: 0, duration: 0.3, stagger: 0.05, ease: "power2.out" },
      );
    }
  }, [loading]);

  // Open permission drawer
  const openPermissions = (rwm: RoleWithMeta) => {
    openDrawer({
      title: `${rwm.role.name.charAt(0).toUpperCase() + rwm.role.name.slice(1)} permissions`,
      width: "lg",
      content: (
        <PermissionForm
          role={rwm.role}
          allPerms={allPerms}
          onSave={async (keys) => {
            const updated = await updateRolePermissions(
              orgId,
              rwm.role.id,
              keys,
            );
            setRoles((prev) =>
              prev.map((r) =>
                r.role.id === updated.id ? { ...r, role: updated } : r,
              ),
            );
          }}
        />
      ),
    });
  };

  // Open clone drawer
  const openClone = (rwm: RoleWithMeta) => {
    openDrawer({
      title: `Clone ${rwm.role.name}`,
      width: "md",
      content: (
        <CloneForm
          sourceRole={rwm.role}
          onSave={async (name) => {
            await cloneRole(orgId, rwm.role.id, name);
            await fetch();
          }}
        />
      ),
    });
  };

  // Delete
  const handleDelete = async (roleId: string) => {
    try {
      await deleteRole(orgId, roleId);
      setRoles((prev) => prev.filter((r) => r.role.id !== roleId));
    } catch {
      setPageErr("Failed to delete role.");
    }
    setDelConfirm(null);
  };

  return (
    <div className="p-6 md:p-8 max-w-4xl">
      {/* Header */}
      <div className="flex items-start justify-between mb-8">
        <div>
          <h1
            className="text-2xl font-bold text-[var(--text-primary)] mb-1"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Roles
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            {roles.length} roles · Click a role to view its permissions
          </p>
        </div>
      </div>

      {pageErr && (
        <div className="mb-5 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20">
          {pageErr}
        </div>
      )}

      {loading ? (
        <div className="flex items-center gap-3 py-16 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading roles…
        </div>
      ) : (
        <div ref={listRef} className="space-y-2.5">
          {roles.map((rwm) => {
            const { role } = rwm;
            const colorCls = ROLE_COLOR[role.name] ?? ROLE_COLOR.viewer;
            const confirming = delConfirm === role.id;

            return (
              <div
                key={role.id}
                className="role-card rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] hover:border-[var(--text-muted)]/20 transition-all duration-150"
              >
                <div className="flex items-center gap-4 px-5 py-4">
                  {/* Icon */}
                  <div className="w-9 h-9 rounded-lg flex-shrink-0 flex items-center justify-center bg-[var(--bg-elevated)] border border-[var(--border)]">
                    <Shield size={15} className="text-[var(--text-muted)]" />
                  </div>

                  {/* Info */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2.5 mb-0.5">
                      <span
                        className="text-sm font-semibold text-[var(--text-primary)] capitalize"
                        style={{
                          fontFamily: "var(--font-inter, Inter, sans-serif)",
                        }}
                      >
                        {role.name}
                      </span>
                      <span
                        className={`text-[0.6rem] font-semibold border px-1.5 py-0.5 rounded-full ${colorCls}`}
                      >
                        {role.isSystem ? "System" : "Custom"}
                      </span>
                    </div>
                    <p className="text-xs text-[var(--text-muted)] truncate">
                      {role.description}
                    </p>
                  </div>

                  {/* Permission count */}
                  <span className="text-xs text-[var(--text-muted)] flex-shrink-0 hidden sm:block">
                    {role.permissionKeys.length} permissions
                  </span>

                  {/* Actions */}
                  {confirming ? (
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <span className="text-xs text-[var(--text-muted)]">
                        Delete?
                      </span>
                      <button
                        onClick={() => handleDelete(role.id)}
                        className="px-2.5 py-1 rounded-md text-xs font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
                      >
                        Yes
                      </button>
                      <button
                        onClick={() => setDelConfirm(null)}
                        className="px-2.5 py-1 rounded-md text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                      >
                        No
                      </button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-1 flex-shrink-0">
                      {/* View / Edit permissions */}
                      {(canViewPerms || canEditPerms) && (
                        <button
                          onClick={() => openPermissions(rwm)}
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors"
                        >
                          {canEditPerms && !role.isSystem ? "Edit" : "View"}{" "}
                          permissions
                          <ChevronRight size={11} />
                        </button>
                      )}

                      {/* Clone */}
                      {canClone && (
                        <button
                          onClick={() => openClone(rwm)}
                          className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors"
                          title="Clone role"
                        >
                          <Copy size={13} />
                        </button>
                      )}

                      {/* Delete — custom roles only */}
                      {canDelete && !role.isSystem && (
                        <button
                          onClick={() => setDelConfirm(role.id)}
                          className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-red-400 hover:bg-red-500/10 transition-colors"
                          title="Delete role"
                        >
                          <Trash2 size={13} />
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
