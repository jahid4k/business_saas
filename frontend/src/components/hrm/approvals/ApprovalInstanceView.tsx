// src/components/hrm/approvals/ApprovalInstanceView.tsx
"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Check, X, Clock } from "lucide-react";
import { toast } from "sonner";
import {
  getApprovalInstance,
  approveInstance,
  rejectInstance,
} from "@/lib/hrm/approvals";
import { usePermissionStore } from "@/stores/permissionStore";

interface ApprovalInstanceViewProps {
  orgId: string;
  instanceId: string;
}

const STATUS_TONE: Record<string, string> = {
  pending: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  approved: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  rejected: "bg-red-500/10 text-red-400 border-red-500/20",
  cancelled: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
};

const APPROVER_LABEL: Record<string, string> = {
  reporting_manager: "Reporting manager",
  dept_head: "Department head",
  role: "Role",
  specific_user: "Specific person",
};

export default function ApprovalInstanceView({
  orgId,
  instanceId,
}: ApprovalInstanceViewProps) {
  const { hasPermission } = usePermissionStore();
  const queryClient = useQueryClient();
  const [note, setNote] = useState("");
  const canAct = hasPermission("hrm.approvals.action");

  const instKey = ["hrm", orgId, "approval-instance", instanceId] as const;
  const instQuery = useQuery({
    queryKey: instKey,
    queryFn: () => getApprovalInstance(orgId, instanceId),
  });
  const inst = instQuery.data;

  const handleApprove = async () => {
    try {
      const updated = await approveInstance(orgId, instanceId, {
        note: note || undefined,
      });
      queryClient.setQueryData(instKey, updated);
      toast.success("Approved.");
      setNote("");
    } catch {
      toast.error("Failed to approve.");
    }
  };

  const handleReject = async () => {
    try {
      const updated = await rejectInstance(orgId, instanceId, {
        note: note || undefined,
      });
      queryClient.setQueryData(instKey, updated);
      toast.success("Rejected.");
      setNote("");
    } catch {
      toast.error("Failed to reject.");
    }
  };

  if (instQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-10 text-sm text-(--text-muted) gap-3">
        <Loader2 size={16} className="animate-spin text-purple-500" /> Loading…
      </div>
    );
  }

  if (!inst) {
    return (
      <div className="px-6 py-5 text-sm text-red-400">
        Failed to load approval instance.
      </div>
    );
  }

  const levels = (inst.snapshot ?? [])
    .slice()
    .sort((a, b) => a.level - b.level);
  const decisionForLevel = (level: number) =>
    (inst.decisions ?? []).find((d) => d.level === level);

  return (
    <div className="px-6 py-5 space-y-4">
      <div className="flex items-center gap-2">
        <span
          className={`text-xs px-2 py-0.5 rounded-full border font-medium ${STATUS_TONE[inst.overall_status]}`}
        >
          {inst.overall_status}
        </span>
        <span className="text-xs text-(--text-muted)">
          Level {inst.current_level} of {levels.length}
        </span>
      </div>

      <div className="space-y-2">
        {levels.map((lvl) => {
          const decision = decisionForLevel(lvl.level);
          const isCurrent =
            lvl.level === inst.current_level &&
            inst.overall_status === "pending";
          return (
            <div
              key={lvl.level}
              className={`p-3 rounded-lg border ${
                isCurrent
                  ? "border-purple-500/40 bg-purple-500/5"
                  : "border-(--border) bg-(--bg-elevated)"
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-(--text-primary)">
                  Level {lvl.level} —{" "}
                  {APPROVER_LABEL[lvl.approver_type] ?? lvl.approver_type}
                </span>
                {decision ? (
                  <span
                    className={`text-xs px-2 py-0.5 rounded-full border font-medium ${
                      decision.action === "approved"
                        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                        : "bg-red-500/10 text-red-400 border-red-500/20"
                    }`}
                  >
                    {decision.action}
                  </span>
                ) : isCurrent ? (
                  <span className="flex items-center gap-1 text-xs text-amber-400">
                    <Clock size={12} /> Waiting
                  </span>
                ) : (
                  <span className="text-xs text-(--text-muted)">—</span>
                )}
              </div>
              {decision?.note && (
                <p className="text-xs text-(--text-muted) mt-1.5">
                  &quot;{decision.note}&quot;
                </p>
              )}
            </div>
          );
        })}
      </div>

      {inst.overall_status === "pending" && canAct && (
        <div className="pt-3 border-t border-(--border) space-y-2">
          <textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={2}
            placeholder="Optional note"
            className="w-full px-3 py-2 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary)"
          />
          <div className="flex gap-2">
            <button
              onClick={handleApprove}
              className="flex-1 flex items-center justify-center gap-1.5 py-2.5 rounded-lg text-sm font-semibold text-white bg-emerald-600 hover:bg-emerald-500 transition-colors"
            >
              <Check size={14} />
              Approve
            </button>
            <button
              onClick={handleReject}
              className="flex-1 flex items-center justify-center gap-1.5 py-2.5 rounded-lg text-sm font-semibold text-white bg-red-500 hover:bg-red-400 transition-colors"
            >
              <X size={14} />
              Reject
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
