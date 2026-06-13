"use client";

import { useState, useEffect } from "react";
import { api, extractApiError } from "@/lib/api";
import { usePermission } from "@/hooks/usePermission";
import { Badge } from "@/components/ui/Badge";
import type { Member } from "@/types/business";
import type { ApiSuccess } from "@/types/api";
import clsx from "clsx";

const ROLE_BADGE: Record<string, "blue" | "info" | "neutral" | "warning"> = {
  owner: "blue",
  admin: "info",
  member: "neutral",
  viewer: "warning",
};

const ASSIGNABLE_ROLES = ["admin", "member", "viewer"] as const;
type AssignableRole = (typeof ASSIGNABLE_ROLES)[number];

export function MemberList() {
  const { can } = usePermission();
  const [members, setMembers] = useState<Member[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [changingRole, setChangingRole] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadMembers() {
      try {
        const { data } = await api.get<ApiSuccess<Member[]>>("/members");

        if (!cancelled) {
          setMembers(data.data ?? []);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(extractApiError(err));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    void loadMembers();

    return () => {
      cancelled = true;
    };
  }, []);

  async function handleRoleChange(userId: string, newRole: AssignableRole) {
    setChangingRole(userId);
    setError(null);

    try {
      const { data } = await api.post<ApiSuccess<Member>>(
        `/members/${userId}/role`,
        { role: newRole },
      );

      setMembers((currentMembers) =>
        currentMembers.map((member) =>
          member.user_id === userId
            ? data.data
              ? data.data
              : { ...member, role_name: newRole }
            : member,
        ),
      );
    } catch (err) {
      setError(extractApiError(err));
    } finally {
      setChangingRole(null);
    }
  }

  if (isLoading) return <MemberSkeleton />;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-xs text-gray-500">
          {members.length} member{members.length !== 1 ? "s" : ""}
        </p>
      </div>

      {error && (
        <div className="px-3 py-2.5 bg-error-light border border-error-border rounded text-sm text-error">
          {error}
        </div>
      )}

      <div className="space-y-2">
        {members.map((member) => {
          const isOwner = member.role_name === "owner";
          const isChanging = changingRole === member.user_id;

          return (
            <div key={member.id} className="table-row">
              <div className="w-8 h-8 rounded-full bg-brand-100 flex items-center justify-center text-xs font-semibold text-brand-700 flex-shrink-0">
                {(
                  member.user_first_name?.[0] ??
                  member.user_email?.[0] ??
                  "?"
                ).toUpperCase()}
              </div>

              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-900 truncate">
                  {member.user_first_name} {member.user_last_name}
                </p>
                <p className="text-xs text-gray-500 truncate">
                  {member.user_email}
                </p>
              </div>

              {can("members.manage") && !isOwner ? (
                <div className="flex items-center gap-2">
                  <select
                    value={member.role_name ?? "member"}
                    onChange={(event) =>
                      handleRoleChange(
                        member.user_id,
                        event.target.value as AssignableRole,
                      )
                    }
                    disabled={isChanging}
                    className={clsx(
                      "text-xs border border-gray-300 rounded px-2 py-1.5 bg-white text-gray-700",
                      "focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500",
                      "disabled:opacity-50 transition-colors",
                    )}
                  >
                    {ASSIGNABLE_ROLES.map((role) => (
                      <option key={role} value={role}>
                        {role}
                      </option>
                    ))}
                  </select>

                  {isChanging && (
                    <svg
                      className="animate-spin h-3.5 w-3.5 text-brand-600 flex-shrink-0"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      />
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                      />
                    </svg>
                  )}
                </div>
              ) : (
                <Badge
                  variant={
                    ROLE_BADGE[member.role_name ?? "member"] ?? "neutral"
                  }
                >
                  {member.role_name}
                </Badge>
              )}
            </div>
          );
        })}
      </div>

      <div className="pt-2 border-t border-gray-100">
        <p className="text-xs font-medium text-gray-500 mb-2">
          Role permissions
        </p>
        <div className="grid grid-cols-2 gap-x-6 gap-y-1 text-2xs text-gray-500">
          {[
            { role: "owner", perms: "all permissions" },
            { role: "admin", perms: "tasks + manage members" },
            { role: "member", perms: "tasks read / create / update" },
            { role: "viewer", perms: "tasks read only" },
          ].map(({ role, perms }) => (
            <div key={role} className="flex items-center gap-1.5">
              <Badge
                variant={ROLE_BADGE[role] ?? "neutral"}
                className="text-2xs"
              >
                {role}
              </Badge>
              <span className="text-gray-400">{perms}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function MemberSkeleton() {
  return (
    <div className="space-y-2 animate-pulse">
      {[1, 2, 3, 4].map((item) => (
        <div
          key={item}
          className="h-14 bg-gray-100 rounded-lg border border-gray-200"
        />
      ))}
    </div>
  );
}
