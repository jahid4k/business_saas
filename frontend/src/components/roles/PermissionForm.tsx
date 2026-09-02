// src/components/roles/PermissionForm.tsx
"use client";

import { useState, useMemo } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Search } from "lucide-react";
import { useDrawer } from "@/contexts/DrawerContext";
import { GROUPS, RESOURCE_LABEL } from "@/lib/permissionGroups";
import type { Role, Permission } from "@/types/rbac";

// Mirrors the backend's roleNamePattern + reserved-name check in
// internal/authz/service.go (validateRoleName) — same rule, checked
// client-side first so this fails instantly instead of after a round trip.
const RESERVED_NAMES = new Set([
  "owner",
  "admin",
  "manager",
  "member",
  "viewer",
]);

const createSchema = z.object({
  name: z
    .string()
    .trim()
    .min(2, "At least 2 characters")
    .max(50, "At most 50 characters")
    .regex(
      /^[A-Za-z0-9][A-Za-z0-9 _-]*$/,
      "Letters, numbers, spaces, - and _ only",
    )
    .refine(
      (v) => !RESERVED_NAMES.has(v.toLowerCase()),
      "That name is reserved for a built-in role",
    ),
  description: z.string().trim().max(200, "At most 200 characters").optional(),
});
type CreateValues = z.infer<typeof createSchema>;

interface PermissionFormProps {
  /** Omit to create a brand-new role instead of editing an existing one. */
  role?: Role;
  allPerms: Permission[];
  /** Edit mode — called with the full replacement permission list. */
  onSave?: (permissionKeys: string[]) => Promise<void>;
  /** Create mode — called with the new role's name, description, and permissions. */
  onCreate?: (values: {
    name: string;
    description: string;
    permissionKeys: string[];
  }) => Promise<void>;
}

export default function PermissionForm({
  role,
  allPerms,
  onSave,
  onCreate,
}: PermissionFormProps) {
  const { closeDrawer } = useDrawer();
  const isCreate = !role;
  const readonly = role?.isSystem ?? false; // System roles are view-only

  // Selected keys — starts from the role's current permissions, or empty when creating
  const [selected, setSelected] = useState<Set<string>>(
    new Set(role?.permissionKeys ?? []),
  );
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const {
    register,
    trigger,
    getValues,
    formState: { errors },
  } = useForm<CreateValues>({
    resolver: zodResolver(createSchema),
    defaultValues: { name: "", description: "" },
  });

  // Group all permissions by resource
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
    if (readonly) return;
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });
  };

  const toggleResource = (resource: string) => {
    if (readonly) return;
    const perms = byResource.get(resource) ?? [];
    const keys = perms.map((p) => p.key);
    const allOn = keys.every((k) => selected.has(k));
    setSelected((prev) => {
      const next = new Set(prev);
      allOn
        ? keys.forEach((k) => next.delete(k))
        : keys.forEach((k) => next.add(k));
      return next;
    });
  };

  const handleSave = async () => {
    setError(null);

    if (isCreate) {
      const valid = await trigger();
      if (!valid) return;
      const { name, description } = getValues();
      setSaving(true);
      try {
        await onCreate?.({
          name: name.trim(),
          description: (description ?? "").trim(),
          permissionKeys: Array.from(selected),
        });
        closeDrawer();
      } catch {
        setError("Failed to create role. Please try again.");
      } finally {
        setSaving(false);
      }
      return;
    }

    setSaving(true);
    try {
      await onSave?.(Array.from(selected));
      closeDrawer();
    } catch {
      setError("Failed to save permissions. Please try again.");
    } finally {
      setSaving(false);
    }
  };

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

        {/* System role notice */}
        {readonly && (
          <div className="px-4 py-3 rounded-lg text-sm text-purple-300 bg-purple-500/8 border border-purple-500/20">
            System roles are read-only. Clone this role to create an editable
            version.
          </div>
        )}

        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {/* Name + description — create mode only */}
        {isCreate && (
          <div className="space-y-4">
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-(--text-secondary)">
                Role name <span className="text-red-400">*</span>
              </label>
              <input
                {...register("name")}
                autoFocus
                placeholder="e.g. HRM Head"
                className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary) placeholder:text-(--text-muted) outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
              />
              {errors.name && (
                <p className="text-xs text-red-400">{errors.name.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <label className="block text-sm font-medium text-(--text-secondary)">
                Description
              </label>
              <input
                {...register("description")}
                placeholder="Optional — shown in the roles list"
                className="w-full px-3.5 py-2.5 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary) placeholder:text-(--text-muted) outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all"
              />
              {errors.description && (
                <p className="text-xs text-red-400">
                  {errors.description.message}
                </p>
              )}
            </div>
            <p className="text-[0.65rem] font-semibold text-(--text-muted) uppercase tracking-widest pt-2">
              Permissions
            </p>
          </div>
        )}

        {GROUPS.map((group) => {
          // Collect all permissions for this display group
          const groupPerms = group.resources.flatMap(
            (r) => byResource.get(r) ?? [],
          );
          if (groupPerms.length === 0) return null;

          return (
            <div key={group.label}>
              {/* Group header */}
              <p
                className="text-[0.65rem] font-semibold text-(--text-muted) uppercase tracking-widest mb-3"
                style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
              >
                {group.label}
              </p>

              {/* Resources inside the group */}
              <div className="space-y-3">
                {group.resources.map((resource) => {
                  const perms = byResource.get(resource);
                  if (!perms || perms.length === 0) return null;

                  const allOn = perms.every((p) => selected.has(p.key));
                  const someOn =
                    !allOn && perms.some((p) => selected.has(p.key));

                  return (
                    <div
                      key={resource}
                      className="rounded-lg border border-(--border) overflow-hidden"
                    >
                      {/* Resource header row */}
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
                          disabled={readonly}
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
                          {perms.filter((p) => selected.has(p.key)).length}/
                          {perms.length}
                        </span>
                      </div>

                      {/* Permission rows */}
                      <div className="divide-y divide-(--border)">
                        {perms.map((p) => (
                          <label
                            key={p.key}
                            className={`
                              flex items-center gap-3 px-3.5 py-2
                              ${readonly ? "cursor-default" : "cursor-pointer hover:bg-(--bg-elevated)/50"}
                              transition-colors
                            `}
                          >
                            <input
                              type="checkbox"
                              checked={selected.has(p.key)}
                              onChange={() => toggle(p.key)}
                              disabled={readonly}
                              className="accent-purple-500 shrink-0"
                            />
                            <div className="min-w-0">
                              <span
                                className="text-xs font-medium text-(--text-primary) capitalize"
                                style={{
                                  fontFamily:
                                    "var(--font-inter, Inter, sans-serif)",
                                }}
                              >
                                {p.action.replace(/_/g, " ")}
                              </span>
                              <p className="text-[0.65rem] text-(--text-muted) leading-none mt-0.5">
                                {p.description}
                              </p>
                            </div>
                          </label>
                        ))}
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
          {readonly ? "Close" : "Cancel"}
        </button>
        {!readonly && (
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
          >
            {saving
              ? isCreate
                ? "Creating…"
                : "Saving…"
              : isCreate
                ? "Create role"
                : "Save permissions"}
          </button>
        )}
      </div>
    </div>
  );
}
