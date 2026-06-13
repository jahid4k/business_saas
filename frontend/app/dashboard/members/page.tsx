"use client";
// frontend/app/dashboard/members/page.tsx
// List workspace members, assign roles. Requires members.manage permission.

import { useState, useEffect, FormEvent } from "react";
import { useAuth } from "@/hooks/useAuth";
import * as api from "@/lib/api";
import type { MemberWithUser } from "@/types";

const ASSIGNABLE_ROLES = ["admin", "member", "viewer"];

const roleBadgeClass: Record<string, string> = {
  owner: "bg-yellow-900 text-yellow-300",
  admin: "bg-blue-900 text-blue-300",
  member: "bg-green-900 text-green-300",
  viewer: "bg-gray-800 text-gray-400",
};

export default function MembersPage() {
  const { currentBusiness, hasPermission } = useAuth();

  const businessId = currentBusiness?.id ?? "";

  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [loadedBusinessId, setLoadedBusinessId] = useState("");
  const [error, setError] = useState("");

  // Add member by user ID
  const [addUserId, setAddUserId] = useState("");
  const [addRole, setAddRole] = useState("member");
  const [addLoading, setAddLoading] = useState(false);
  const [addError, setAddError] = useState("");
  const [addSuccess, setAddSuccess] = useState("");

  // Role change
  const [changingRole, setChangingRole] = useState<string | null>(null);

  const canManage = hasPermission("members.manage");

  const loading = Boolean(businessId && loadedBusinessId !== businessId);

  useEffect(() => {
    if (!businessId) return;

    let cancelled = false;

    async function loadInitialMembers() {
      try {
        const res = await api.listMembers();

        if (cancelled) return;

        if (res.success && res.data) {
          setMembers(res.data.members ?? []);
          setError("");
        } else {
          setMembers([]);
          setError(res.error?.message || "Failed to load members");
        }
      } catch (err) {
        if (cancelled) return;

        setMembers([]);
        setError(err instanceof Error ? err.message : "Failed to load members");
      } finally {
        if (!cancelled) {
          setLoadedBusinessId(businessId);
        }
      }
    }

    void loadInitialMembers();

    return () => {
      cancelled = true;
    };
  }, [businessId]);

  async function reloadMembers() {
    if (!businessId) return;

    setLoadedBusinessId("");
    setError("");

    try {
      const res = await api.listMembers();

      if (res.success && res.data) {
        setMembers(res.data.members ?? []);
        setError("");
      } else {
        setMembers([]);
        setError(res.error?.message || "Failed to load members");
      }
    } catch (err) {
      setMembers([]);
      setError(err instanceof Error ? err.message : "Failed to load members");
    } finally {
      setLoadedBusinessId(businessId);
    }
  }

  async function handleAddMember(e: FormEvent) {
    e.preventDefault();

    setAddError("");
    setAddSuccess("");
    setAddLoading(true);

    try {
      const res = await api.assignRole(addUserId.trim(), addRole);

      if (!res.success) {
        setAddError(res.error?.message || "Failed to add member");
        return;
      }

      setAddSuccess(`User added as ${addRole}`);
      setAddUserId("");

      await reloadMembers();
    } catch (err) {
      setAddError(err instanceof Error ? err.message : "Failed to add member");
    } finally {
      setAddLoading(false);
    }
  }

  async function handleRoleChange(userId: string, newRole: string) {
    setChangingRole(userId);
    setError("");

    try {
      const res = await api.assignRole(userId, newRole);

      if (!res.success) {
        setError(res.error?.message || "Failed to change role");
        return;
      }

      await reloadMembers();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to change role");
    } finally {
      setChangingRole(null);
    }
  }

  if (!currentBusiness) {
    return (
      <div className="p-8 max-w-4xl">
        <NoBusiness />
      </div>
    );
  }

  return (
    <div className="p-8 max-w-4xl">
      <h2 className="text-xl font-semibold text-white mb-1">Members</h2>

      <p className="text-gray-400 text-sm mb-8">
        Workspace: <span className="text-gray-300">{currentBusiness.name}</span>
        {!canManage && (
          <span className="text-yellow-600 ml-3">
            · View only (no members.manage permission)
          </span>
        )}
      </p>

      {canManage && (
        <section className="mb-8">
          <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
            Add Member by User ID
          </h3>

          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-4">
              Users must sign up first. Paste their User ID from the Overview
              page or backend.
            </p>

            <form onSubmit={handleAddMember} className="space-y-4">
              {addError && (
                <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-lg px-4 py-3">
                  {addError}
                </div>
              )}

              {addSuccess && (
                <div className="bg-green-950 border border-green-800 text-green-300 text-sm rounded-lg px-4 py-3">
                  {addSuccess}
                </div>
              )}

              <div className="grid grid-cols-3 gap-3">
                <div className="col-span-2">
                  <label className="block text-sm text-gray-400 mb-1.5">
                    User ID (UUID)
                  </label>

                  <input
                    type="text"
                    value={addUserId}
                    onChange={(e) => setAddUserId(e.target.value)}
                    required
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-xs font-mono placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                    placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                  />
                </div>

                <div>
                  <label className="block text-sm text-gray-400 mb-1.5">
                    Role
                  </label>

                  <select
                    value={addRole}
                    onChange={(e) => setAddRole(e.target.value)}
                    className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm focus:outline-none focus:border-indigo-500"
                  >
                    {ASSIGNABLE_ROLES.map((r) => (
                      <option key={r} value={r} className="capitalize">
                        {r}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <button
                type="submit"
                disabled={addLoading}
                className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
              >
                {addLoading ? "Adding..." : "Add member"}
              </button>
            </form>
          </div>
        </section>
      )}

      <section>
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Members ({members.length})
        </h3>

        {loading ? (
          <p className="text-gray-500 text-sm">Loading...</p>
        ) : error ? (
          <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-xl p-5">
            {error}
          </div>
        ) : members.length === 0 ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-400 text-sm">No members found.</p>
          </div>
        ) : (
          <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-800">
                  <th className="text-left px-5 py-3 text-xs text-gray-500 font-medium">
                    Name
                  </th>
                  <th className="text-left px-5 py-3 text-xs text-gray-500 font-medium">
                    Email
                  </th>
                  <th className="text-left px-5 py-3 text-xs text-gray-500 font-medium">
                    Role
                  </th>
                  <th className="text-left px-5 py-3 text-xs text-gray-500 font-medium">
                    Joined
                  </th>

                  {canManage && (
                    <th className="px-5 py-3 text-xs text-gray-500 font-medium">
                      Change Role
                    </th>
                  )}
                </tr>
              </thead>

              <tbody>
                {members.map((m, i) => (
                  <tr
                    key={m.membership_id}
                    className={
                      i < members.length - 1 ? "border-b border-gray-800" : ""
                    }
                  >
                    <td className="px-5 py-3.5 text-sm text-white">
                      {m.first_name} {m.last_name}
                    </td>

                    <td className="px-5 py-3.5 text-sm text-gray-400">
                      {m.email}
                    </td>

                    <td className="px-5 py-3.5">
                      <span
                        className={`text-xs font-medium px-2.5 py-1 rounded-md capitalize ${
                          roleBadgeClass[m.role] ?? "bg-gray-800 text-gray-400"
                        }`}
                      >
                        {m.role}
                      </span>
                    </td>

                    <td className="px-5 py-3.5 text-sm text-gray-500">
                      {new Date(m.joined_at).toLocaleDateString()}
                    </td>

                    {canManage && (
                      <td className="px-5 py-3.5">
                        {m.role === "owner" ? (
                          <span className="text-xs text-gray-600">
                            Owner (unchangeable)
                          </span>
                        ) : (
                          <select
                            value={m.role}
                            onChange={(e) =>
                              handleRoleChange(m.user_id, e.target.value)
                            }
                            disabled={changingRole === m.user_id}
                            className="bg-gray-800 border border-gray-700 rounded-lg px-2 py-1 text-white text-xs focus:outline-none focus:border-indigo-500"
                          >
                            {ASSIGNABLE_ROLES.map((r) => (
                              <option key={r} value={r} className="capitalize">
                                {r}
                              </option>
                            ))}
                          </select>
                        )}
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}

function NoBusiness() {
  return (
    <div>
      <h2 className="text-xl font-semibold text-white mb-1">Members</h2>

      <div className="mt-6 bg-gray-900 border border-gray-800 rounded-xl p-6">
        <p className="text-gray-400 text-sm">No workspace selected.</p>
        <p className="text-gray-500 text-xs mt-1">
          Go to Businesses and switch into a workspace first.
        </p>
      </div>
    </div>
  );
}
