// src/components/hrm/salary/StructureComponentsManager.tsx
"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2, Loader2 } from "lucide-react";
import { toast } from "sonner";
import {
  getSalaryStructure,
  addComponentToStructure,
  removeComponentFromStructure,
} from "@/lib/hrm/salary";
import type { SalaryComponent } from "@/types/hrm";
import { queryKeys } from "@/lib/queryKeys";

interface StructureComponentsManagerProps {
  orgId: string;
  structureId: string;
  allComponents: SalaryComponent[];
}

const inputCls = `
  px-3.5 py-2.5 rounded-lg text-sm
  bg-(--bg-elevated) border border-(--border)
  text-(--text-primary) placeholder:text-(--text-muted)
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function StructureComponentsManager({
  orgId,
  structureId,
  allComponents,
}: StructureComponentsManagerProps) {
  const queryClient = useQueryClient();
  const [pickId, setPickId] = useState("");
  const [overrideValue, setOverrideValue] = useState("");

  const detailKey = queryKeys.hrm.salaryStructures.detail(orgId, structureId);
  const detailQuery = useQuery({
    queryKey: detailKey,
    queryFn: () => getSalaryStructure(orgId, structureId),
  });
  const structure = detailQuery.data;

  const refresh = () => queryClient.invalidateQueries({ queryKey: detailKey });

  const handleAdd = async () => {
    if (!pickId) return;
    try {
      await addComponentToStructure(orgId, structureId, {
        component_id: pickId,
        override_value: overrideValue ? Number(overrideValue) : undefined,
      });
      toast.success("Component added.");
      setPickId("");
      setOverrideValue("");
      refresh();
    } catch {
      toast.error("Failed to add component.");
    }
  };

  const handleRemove = async (compId: string) => {
    try {
      await removeComponentFromStructure(orgId, structureId, compId);
      toast.success("Component removed.");
      refresh();
    } catch {
      toast.error("Failed to remove component.");
    }
  };

  const inStructureIds = new Set(
    (structure?.components ?? []).map((c) => c.component_id),
  );
  const available = allComponents.filter((c) => !inStructureIds.has(c.id));

  if (detailQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-10 text-sm text-(--text-muted) gap-3">
        <Loader2 size={16} className="animate-spin text-purple-500" /> Loading…
      </div>
    );
  }

  if (detailQuery.isError) {
    return (
      <div className="px-6 py-5">
        <p className="text-sm text-red-400">
          Failed to load structure components.
        </p>
      </div>
    );
  }

  return (
    <div className="px-6 py-5 space-y-4">
      <div className="space-y-2">
        {(structure?.components ?? []).length === 0 ? (
          <p className="text-sm text-(--text-muted)">
            No components in this structure yet.
          </p>
        ) : (
          (structure?.components ?? [])
            .slice()
            .sort((a, b) => a.display_order - b.display_order)
            .map((sc) => (
              <div
                key={sc.component_id}
                className="flex items-center justify-between px-3.5 py-2.5 rounded-lg bg-(--bg-elevated) border border-(--border)"
              >
                <div>
                  <p className="text-sm text-(--text-primary)">
                    {sc.component?.name ?? sc.component_id}
                  </p>
                  <p className="text-xs text-(--text-muted)">
                    {sc.component?.component_type} · {sc.component?.calc_method}
                    {sc.override_value !== undefined
                      ? ` · override: ${sc.override_value}`
                      : ""}
                  </p>
                </div>
                <button
                  onClick={() => handleRemove(sc.component_id)}
                  className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10 transition-colors"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ))
        )}
      </div>

      <div className="flex items-end gap-2 pt-2 border-t border-(--border)">
        <div className="flex-1 space-y-1.5">
          <label className="block text-xs font-medium text-(--text-secondary)">
            Add component
          </label>
          <select
            value={pickId}
            onChange={(e) => setPickId(e.target.value)}
            className={`${inputCls} w-full`}
          >
            <option value="">Select…</option>
            {available.map((c) => (
              <option
                key={c.id}
                value={c.id}
                style={{ background: "var(--bg-elevated)" }}
              >
                {c.name}
              </option>
            ))}
          </select>
        </div>
        <div className="w-28 space-y-1.5">
          <label className="block text-xs font-medium text-(--text-secondary)">
            Override
          </label>
          <input
            value={overrideValue}
            onChange={(e) => setOverrideValue(e.target.value)}
            type="number"
            step="0.01"
            placeholder="Optional"
            className={`${inputCls} w-full`}
          />
        </div>
        <button
          onClick={handleAdd}
          disabled={!pickId}
          className="flex items-center gap-1.5 px-3.5 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 transition-colors"
        >
          <Plus size={14} />
          Add
        </button>
      </div>
    </div>
  );
}
