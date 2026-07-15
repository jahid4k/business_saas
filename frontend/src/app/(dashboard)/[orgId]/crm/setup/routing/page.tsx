"use client";

import { use, useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader2, Route, Save, Users, Plus, X } from "lucide-react";
import { getCRMSettings, updateCRMSettings } from "@/lib/crm/settings";
import { listMembers } from "@/lib/members";
import { usePermissionStore } from "@/stores/permissionStore";

export default function LeadRoutingPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const queryClient = useQueryClient();
  const { hasPermission } = usePermissionStore();
  const canUpdate = hasPermission("settings.update"); // Assuming settings.update for CRM settings

  const [enabled, setEnabled] = useState(false);
  const [assignees, setAssignees] = useState<string[]>([]);
  const [selectedUser, setSelectedUser] = useState("");

  const settingsQuery = useQuery({
    queryKey: ["crm", "settings", orgId],
    queryFn: () => getCRMSettings(orgId),
  });

  const membersQuery = useQuery({
    queryKey: ["members", orgId],
    queryFn: () => listMembers(orgId),
  });

  useEffect(() => {
    if (settingsQuery.data) {
      setEnabled(settingsQuery.data.lead_routing_enabled);
      setAssignees(settingsQuery.data.round_robin_assignees || []);
    }
  }, [settingsQuery.data]);

  const updateMutation = useMutation({
    mutationFn: () =>
      updateCRMSettings(orgId, {
        lead_routing_enabled: enabled,
        round_robin_assignees: assignees,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["crm", "settings", orgId] });
    },
  });

  const handleSave = () => {
    updateMutation.mutate();
  };

  const handleAddAssignee = () => {
    if (selectedUser && !assignees.includes(selectedUser)) {
      setAssignees([...assignees, selectedUser]);
      setSelectedUser("");
    }
  };

  const handleRemoveAssignee = (id: string) => {
    setAssignees(assignees.filter((a) => a !== id));
  };

  if (settingsQuery.isPending || membersQuery.isPending) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 size={32} className="animate-spin text-purple-600" />
      </div>
    );
  }

  const members = membersQuery.data || [];

  return (
    <div className="max-w-4xl space-y-8 pb-12 p-8">
      <div>
        <h1
          className="text-2xl font-bold text-[var(--text-primary)] mb-2"
          style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
        >
          Lead Routing
        </h1>
        <p className="text-sm text-[var(--text-secondary)]">
          Configure how new leads are automatically assigned to your team members.
        </p>
      </div>

      <div className="bg-[var(--bg-surface)] border border-[var(--border)] rounded-xl p-6">
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-purple-500/10 text-purple-400">
              <Route size={20} />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-[var(--text-primary)]">
                Round-Robin Assignment
              </h2>
              <p className="text-xs text-[var(--text-muted)]">
                Automatically distribute new leads equally among selected team members.
              </p>
            </div>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              className="sr-only peer"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              disabled={!canUpdate || updateMutation.isPending}
            />
            <div className="w-11 h-6 bg-[var(--bg-elevated)] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-purple-600"></div>
          </label>
        </div>

        {enabled && (
          <div className="mt-8 space-y-6 border-t border-[var(--border)] pt-6">
            <div>
              <h3 className="text-sm font-medium text-[var(--text-primary)] mb-2 flex items-center gap-2">
                <Users size={16} className="text-purple-500" />
                Eligible Assignees
              </h3>
              <p className="text-xs text-[var(--text-muted)] mb-4">
                These users will receive new leads in sequential order.
              </p>

              <div className="flex gap-2 mb-4">
                <select
                  className="flex-1 bg-[var(--bg-canvas)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] focus:outline-none focus:border-purple-500 transition-colors"
                  value={selectedUser}
                  onChange={(e) => setSelectedUser(e.target.value)}
                  disabled={!canUpdate}
                >
                  <option value="">Select a team member...</option>
                  {members
                    .filter((m) => !assignees.includes(m.userId))
                    .map((m) => (
                      <option key={m.userId} value={m.userId}>
                        {m.firstName} {m.lastName} ({m.email})
                      </option>
                    ))}
                </select>
                <button
                  onClick={handleAddAssignee}
                  disabled={!selectedUser || !canUpdate}
                  className="px-4 py-2 bg-[var(--bg-elevated)] border border-[var(--border)] rounded-lg text-sm font-medium text-[var(--text-primary)] hover:bg-[var(--bg-hover)] disabled:opacity-50 transition-colors flex items-center gap-2"
                >
                  <Plus size={16} /> Add
                </button>
              </div>

              {assignees.length > 0 ? (
                <div className="space-y-2">
                  {assignees.map((id, index) => {
                    const m = members.find((user) => user.userId === id);
                    return (
                      <div
                        key={id}
                        className="flex items-center justify-between p-3 rounded-lg border border-[var(--border)] bg-[var(--bg-canvas)]"
                      >
                        <div className="flex items-center gap-3">
                          <div className="w-6 h-6 rounded bg-purple-500/10 text-purple-400 flex items-center justify-center text-xs font-bold">
                            {index + 1}
                          </div>
                          <div>
                            <p className="text-sm font-medium text-[var(--text-primary)]">
                              {m ? `${m.firstName} ${m.lastName}` : "Unknown User"}
                            </p>
                            {m && <p className="text-xs text-[var(--text-muted)]">{m.email}</p>}
                          </div>
                        </div>
                        {canUpdate && (
                          <button
                            onClick={() => handleRemoveAssignee(id)}
                            className="p-1 text-[var(--text-muted)] hover:text-red-400 hover:bg-red-500/10 rounded transition-colors"
                          >
                            <X size={16} />
                          </button>
                        )}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="p-8 text-center border border-dashed border-[var(--border)] rounded-lg text-[var(--text-muted)] text-sm">
                  No assignees configured. Round-robin assignment will not run.
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {canUpdate && (
        <div className="flex justify-end border-t border-[var(--border)] pt-6 mt-8">
          <button
            onClick={handleSave}
            disabled={updateMutation.isPending}
            className="px-6 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-sm font-medium transition-colors flex items-center gap-2 disabled:opacity-50"
          >
            {updateMutation.isPending ? (
              <Loader2 size={16} className="animate-spin" />
            ) : (
              <Save size={16} />
            )}
            Save Configuration
          </button>
        </div>
      )}
    </div>
  );
}
