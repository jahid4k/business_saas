// src/components/members/MemberPermissionsForm.tsx
"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Search } from "lucide-react";
import { useDrawer } from "@/contexts/DrawerContext";
import { usePermissionStore } from "@/stores/permissionStore";
import { getMemberPermissions } from "@/lib/members";
import { listPermissions } from "@/lib/roles";
import { queryKeys } from "@/lib/queryKeys";
import { GROUPS, RESOURCE_LABEL } from "@/lib/permissionGroups";
import type { MemberPermissions, Permission } from "@/types/rbac";

interface MemberPermissionsFormProps {
  orgId: string;
  membershipId: string;
  memberName: string;
  /** Called with the full replacement custom-grant and denied lists. */
  onSave: (
    customPermissions: string[],
    deniedPermissions: string[],
  ) => Promise<void>;
}

// Fetches the member's current permissions and the full permission catalog,
// then hands off to Checklist once both are loaded. Kept separate from
// Checklist so Checklist's local `effective` state can be lazily initialized
// straight from `data` at mount time instead of synced in via an effect —
// the data is guaranteed present by the time Checklist exists at all.
export default function MemberPermissionsForm({
  orgId,
  membershipId,
  memberName,
  onSave,
}: MemberPermissionsFormProps) {
  const permsQuery = useQuery({
    queryKey: queryKeys.roles.permissions(orgId),
    queryFn: () => listPermissions(orgId),
  });
  const memberPermsQuery = useQuery({
    queryKey: queryKeys.members.permissions(orgId, membershipId),
    queryFn: () => getMemberPermissions(orgId, membershipId),
  });

  if (permsQuery.isPending || memberPermsQuery.isPending) {
    return (
      <div className="flex items-center justify-center h-full py-20">
        <Loader2 size={18} className="animate-spin text-purple-500" />
      </div>
    );
  }

  if (
    permsQuery.isError ||
    memberPermsQuery.isError ||
    !memberPermsQuery.data
  ) {
    return (
      <div className="p-6">
        <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
          Failed to load permissions for {memberName}.
        </div>
      </div>
    );
  }

  return (
    <Checklist
      data={memberPermsQuery.data}
      allPerms={permsQuery.data ?? []}
      memberName={memberName}
      onSave={onSave}
    />
  );
}

function Checklist({
  data,
  allPerms,
  memberName,
  onSave,
}: {
  data: MemberPermissions;
  allPerms: Permission[];
  memberName: string;
  onSave: (
    customPermissions: string[],
    deniedPermissions: string[],
  ) => Promise<void>;
}) {
  const { closeDrawer } = useDrawer();
  const { hasPermission } = usePermissionStore();
  const canEdit = hasPermission("members.permissions.update");

  // Immutable baseline — what the member's role grants, on its own.
  const rolePerms = useMemo(
    () => new Set(data.rolePermissionKeys),
    [data.rolePermissionKeys],
  );

  // Editable — what the member currently, effectively has. Diffed against
  // rolePerms at save time to work out custom grants vs. explicit denials,
  // so toggling never has to decide that up front.
  const [effective, setEffective] = useState(
    () => new Set(data.effectivePermissions),
  );
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const byResource = useMemo(() => {
    const map = new Map<string, Permission[]>();
    const q = searchQuery.toLowerCase();
    for (const p of allPerms) {
      const resourceLabel = RESOURCE_LABEL[p.resource] ?? p.resource;
      const matchesSearch =
        !q ||
        p.action.replace(/_/g, " ").toLowerCase().includes(q) ||
        p.description.toLowerCase().includes(q) ||
        resourceLabel.toLowerCase().includes(q);

      if (!matchesSearch) continue;

      const arr = map.get(p.resource) ?? [];
      arr.push(p);
      map.set(p.resource, arr);
    }
    return map;
  }, [allPerms, searchQuery]);

  const toggle = (key: string) => {
    if (!canEdit) return;
    setEffective((prev) => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });
  };

  const toggleResource = (resource: string) => {
    if (!canEdit) return;
    const perms = byResource.get(resource) ?? [];
    const keys = perms.map((p) => p.key);
    const allOn = keys.every((k) => effective.has(k));
    setEffective((prev) => {
      const next = new Set(prev);
      allOn
        ? keys.forEach((k) => next.delete(k))
        : keys.forEach((k) => next.add(k));
      return next;
    });
  };

  const handleSave = async () => {
    setError(null);
    setSaving(true);
    try {
      const customPermissions = Array.from(effective).filter(
        (k) => !rolePerms.has(k),
      );
      const deniedPermissions = Array.from(rolePerms).filter(
        (k) => !effective.has(k),
      );
      await onSave(customPermissions, deniedPermissions);
      closeDrawer();
    } catch {
      setError("Failed to save permissions. Please try again.");
    } finally {
      setSaving(false);
    }
  };

  const customCount = Array.from(effective).filter(
    (k) => !rolePerms.has(k),
  ).length;
  const deniedCount = Array.from(rolePerms).filter(
    (k) => !effective.has(k),
  ).length;

  return (
    <div className="flex flex-col h-full">
      {/* Body */}
      <div className="flex-1 overflow-y-auto px-6 py-4 space-y-6">
        <div className="relative">
          <Search
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-(--text-muted)"
          />
          <input
            type="text"
            placeholder="Search permissions..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-9 pr-4 py-2 rounded-lg text-sm bg-(--bg-surface) border border-(--border) text-(--text-primary) placeholder:text-(--text-muted) outline-none focus:border-purple-500 transition-colors"
          />
        </div>

        {!canEdit && (
          <div className="px-4 py-3 rounded-lg text-sm text-purple-300 bg-purple-500/8 border border-purple-500/20">
            You can view {memberName}&apos;s permissions, but you don&apos;t
            have permission to change them.
          </div>
        )}

        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {(customCount > 0 || deniedCount > 0) && (
          <div className="px-4 py-3 rounded-lg text-xs text-(--text-muted) bg-(--bg-elevated) border border-(--border)">
            {customCount > 0 && <span>{customCount} granted beyond role</span>}
            {customCount > 0 && deniedCount > 0 && <span> · </span>}
            {deniedCount > 0 && <span>{deniedCount} denied despite role</span>}
          </div>
        )}

        {GROUPS.map((group) => {
          const groupPerms = group.resources.flatMap(
            (r) => byResource.get(r) ?? [],
          );
          if (groupPerms.length === 0) return null;

          return (
            <div key={group.label}>
              <p
                className="text-[0.65rem] font-semibold text-(--text-muted) uppercase tracking-widest mb-3"
                style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
              >
                {group.label}
              </p>

              <div className="space-y-3">
                {group.resources.map((resource) => {
                  const perms = byResource.get(resource);
                  if (!perms || perms.length === 0) return null;

                  const allOn = perms.every((p) => effective.has(p.key));
                  const someOn =
                    !allOn && perms.some((p) => effective.has(p.key));

                  return (
                    <div
                      key={resource}
                      className="rounded-lg border border-(--border) overflow-hidden"
                    >
                      <div
                        className="flex items-center gap-3 px-3.5 py-2.5 bg-(--bg-elevated) cursor-pointer"
                        onClick={() => toggleResource(resource)}
                      >
                        <input
                          type="checkbox"
                          checked={allOn}
                          ref={(el) => {
                            if (el) el.indeterminate = someOn;
                          }}
                          onChange={() => toggleResource(resource)}
                          disabled={!canEdit}
                          className="accent-purple-500 shrink-0"
                          onClick={(e) => e.stopPropagation()}
                        />
                        <span
                          className="text-xs font-semibold text-(--text-secondary)"
                          style={{
                            fontFamily: "var(--font-inter, Inter, sans-serif)",
                          }}
                        >
                          {RESOURCE_LABEL[resource] ?? resource}
                        </span>
                        <span className="ml-auto text-[0.65rem] text-(--text-muted)">
                          {perms.filter((p) => effective.has(p.key)).length}/
                          {perms.length}
                        </span>
                      </div>

                      <div className="divide-y divide-(--border)">
                        {perms.map((p) => {
                          const fromRole = rolePerms.has(p.key);
                          return (
                            <label
                              key={p.key}
                              className={`
                                flex items-center gap-3 px-3.5 py-2
                                ${canEdit ? "cursor-pointer hover:bg-(--bg-elevated)/50" : "cursor-default"}
                                transition-colors
                              `}
                            >
                              <input
                                type="checkbox"
                                checked={effective.has(p.key)}
                                onChange={() => toggle(p.key)}
                                disabled={!canEdit}
                                className="accent-purple-500 shrink-0"
                              />
                              <div className="min-w-0 flex-1">
                                <div className="flex items-center gap-1.5">
                                  <span
                                    className="text-xs font-medium text-(--text-primary) capitalize"
                                    style={{
                                      fontFamily:
                                        "var(--font-inter, Inter, sans-serif)",
                                    }}
                                  >
                                    {p.action.replace(/_/g, " ")}
                                  </span>
                                  {fromRole && (
                                    <span className="text-[0.55rem] font-semibold text-(--text-muted) bg-(--bg-surface) border border-(--border) px-1.5 py-0.5 rounded-full shrink-0">
                                      from role
                                    </span>
                                  )}
                                </div>
                                <p className="text-[0.65rem] text-(--text-muted) leading-none mt-0.5">
                                  {p.description}
                                </p>
                              </div>
                            </label>
                          );
                        })}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>

      {/* Footer */}
      <div className="flex items-center gap-3 px-6 py-4 border-t border-(--border) shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-(--text-secondary) border border-(--border) hover:bg-(--bg-elevated) transition-colors"
        >
          {canEdit ? "Cancel" : "Close"}
        </button>
        {canEdit && (
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
          >
            {saving ? "Saving…" : "Save permissions"}
          </button>
        )}
      </div>
    </div>
  );
}
