"use client";
// frontend/app/dashboard/roles/page.tsx
// View all system roles with their permissions. Read-only in Phase 1.

import { useState, useEffect } from "react";
import * as api from "@/lib/api";
import type { RoleWithPermissions, Permission } from "@/types";

export default function RolesPage() {
  const [roles, setRoles] = useState<RoleWithPermissions[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      setLoading(true);
      const [rolesRes, permsRes] = await Promise.all([
        api.listRoles(),
        api.listPermissions(),
      ]);
      setLoading(false);
      if (rolesRes.success && rolesRes.data) {
        setRoles(rolesRes.data.roles ?? []);
      }
      if (permsRes.success && permsRes.data) {
        setPermissions(permsRes.data.permissions ?? []);
      }
      if (!rolesRes.success) {
        setError(rolesRes.error?.message || "Failed to load roles");
      }
    }
    load();
  }, []);

  // Group permissions by resource
  const grouped = permissions.reduce<Record<string, Permission[]>>((acc, p) => {
    if (!acc[p.resource]) acc[p.resource] = [];
    acc[p.resource].push(p);
    return acc;
  }, {});

  return (
    <div className="p-8 max-w-4xl">
      <h2 className="text-xl font-semibold text-white mb-1">
        Roles & Permissions
      </h2>
      <p className="text-gray-400 text-sm mb-8">
        System-defined roles and their permission sets
      </p>

      {loading ? (
        <p className="text-gray-500 text-sm">Loading...</p>
      ) : error ? (
        <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-xl p-5">
          {error}
        </div>
      ) : (
        <>
          {/* Roles */}
          <section className="mb-10">
            <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-4">
              System Roles
            </h3>
            <div className="grid grid-cols-2 gap-4">
              {roles.map(({ role, permissions: rolePerms }) => (
                <div
                  key={role.id}
                  className="bg-gray-900 border border-gray-800 rounded-xl p-5"
                >
                  <div className="flex items-center gap-2 mb-3">
                    <span className="text-white text-sm font-medium capitalize">
                      {role.name}
                    </span>
                    {role.is_system && (
                      <span className="text-xs bg-gray-800 text-gray-500 px-2 py-0.5 rounded">
                        System
                      </span>
                    )}
                  </div>
                  {role.description && (
                    <p className="text-gray-500 text-xs mb-3">
                      {role.description}
                    </p>
                  )}
                  <div className="flex flex-wrap gap-1.5">
                    {rolePerms.length === 0 ? (
                      <span className="text-gray-600 text-xs">
                        No permissions
                      </span>
                    ) : (
                      rolePerms.map((p) => (
                        <span
                          key={p.id}
                          className="bg-gray-800 text-gray-400 text-xs font-mono px-2 py-0.5 rounded"
                        >
                          {p.resource}.{p.action}
                        </span>
                      ))
                    )}
                  </div>
                </div>
              ))}
            </div>
          </section>

          {/* All permissions grouped by resource */}
          <section>
            <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-4">
              All Permissions ({permissions.length})
            </h3>
            <div className="space-y-4">
              {Object.entries(grouped).map(([resource, perms]) => (
                <div
                  key={resource}
                  className="bg-gray-900 border border-gray-800 rounded-xl p-5"
                >
                  <h4 className="text-white text-sm font-medium capitalize mb-3">
                    {resource}
                  </h4>
                  <div className="space-y-2">
                    {perms.map((p) => (
                      <div key={p.id} className="flex items-center gap-3">
                        <span className="text-indigo-400 text-xs font-mono w-40 shrink-0">
                          {p.resource}.{p.action}
                        </span>
                        <span className="text-gray-500 text-xs">
                          {p.description}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </section>
        </>
      )}
    </div>
  );
}
