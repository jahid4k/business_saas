// src/components/hrm/setup/StatusForm.tsx
"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
import type { EmployeeStatusModel, EmployeeStatusCategory } from "@/types/hrm";

interface StatusFormProps {
  initialData?: EmployeeStatusModel;
  onSave: (payload: { name: string; category: EmployeeStatusCategory; color: string }) => Promise<void>;
  onCancel: () => void;
}

const CATEGORIES: { value: EmployeeStatusCategory; label: string; desc: string }[] = [
  { value: "active", label: "Active", desc: "Currently employed and working" },
  { value: "inactive", label: "Inactive", desc: "Not currently working (e.g. suspended, on garden leave)" },
  { value: "on_leave", label: "On Leave", desc: "Taking extended time off" },
  { value: "terminated", label: "Terminated", desc: "No longer employed at the company" },
];

const PRESET_COLORS = [
  "#22c55e", // Green
  "#3b82f6", // Blue
  "#eab308", // Yellow
  "#f97316", // Orange
  "#ef4444", // Red
  "#8b5cf6", // Purple
  "#ec4899", // Pink
  "#64748b", // Slate
];

export default function StatusForm({ initialData, onSave, onCancel }: StatusFormProps) {
  const [loading, setLoading] = useState(false);
  
  // Prevent modifying the category or name of default statuses
  const isDefault = initialData && ["Active", "Inactive", "On leave", "Terminated", "Resigned"].includes(initialData.name);

  const [name, setName] = useState(initialData?.name || "");
  const [category, setCategory] = useState<EmployeeStatusCategory>(initialData?.category || "active");
  const [color, setColor] = useState(initialData?.color || "#22c55e");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setLoading(true);
    try {
      await onSave({
        name: name.trim(),
        category,
        color,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        <div>
          <label className="block text-sm font-medium text-[var(--text-secondary)] mb-1.5">
            Status Name
          </label>
          <input
            type="text"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={isDefault}
            placeholder="e.g. Sabbatical"
            className="w-full px-3.5 py-2.5 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-[var(--text-primary)] focus:outline-none focus:border-purple-500 transition-colors disabled:opacity-50"
          />
          {isDefault && (
            <p className="text-xs text-[var(--text-muted)] mt-1.5">
              The name of this system-default status cannot be changed.
            </p>
          )}
        </div>

        <div>
          <label className="block text-sm font-medium text-[var(--text-secondary)] mb-1.5">
            System Category
          </label>
          <div className="space-y-2">
            {CATEGORIES.map((cat) => (
              <label
                key={cat.value}
                className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                  category === cat.value
                    ? "bg-purple-500/5 border-purple-500/30"
                    : "bg-[var(--bg-surface)] border-[var(--border)] hover:border-[var(--text-muted)]"
                } ${isDefault && category !== cat.value ? "opacity-50 cursor-not-allowed" : ""}`}
              >
                <div className="flex items-center h-5">
                  <input
                    type="radio"
                    name="category"
                    value={cat.value}
                    checked={category === cat.value}
                    onChange={() => !isDefault && setCategory(cat.value)}
                    disabled={isDefault && category !== cat.value}
                    className="w-4 h-4 text-purple-500 bg-transparent border-[var(--border)] focus:ring-purple-500 focus:ring-offset-[var(--bg-base)]"
                  />
                </div>
                <div>
                  <div className="text-sm font-medium text-[var(--text-primary)]">
                    {cat.label}
                  </div>
                  <div className="text-xs text-[var(--text-muted)] mt-0.5">
                    {cat.desc}
                  </div>
                </div>
              </label>
            ))}
          </div>
          {isDefault && (
            <p className="text-xs text-[var(--text-muted)] mt-1.5">
              The system category of this default status cannot be changed.
            </p>
          )}
        </div>

        <div>
          <label className="block text-sm font-medium text-[var(--text-secondary)] mb-2">
            Badge Color
          </label>
          <div className="flex flex-wrap gap-3 mb-4">
            {PRESET_COLORS.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => setColor(c)}
                className={`w-8 h-8 rounded-full flex items-center justify-center transition-transform ${
                  color === c ? "ring-2 ring-offset-2 ring-offset-[var(--bg-base)] ring-[currentcolor] scale-110" : "hover:scale-110"
                }`}
                style={{ backgroundColor: c, color: c }}
              />
            ))}
          </div>
          <div className="flex items-center gap-3">
            <input
              type="color"
              value={color}
              onChange={(e) => setColor(e.target.value)}
              className="h-9 w-16 p-1 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg cursor-pointer"
            />
            <input
              type="text"
              value={color}
              onChange={(e) => setColor(e.target.value)}
              pattern="^#[0-9a-fA-F]{6}$"
              className="flex-1 px-3.5 py-2 bg-[var(--bg-surface)] border border-[var(--border)] rounded-lg text-[var(--text-primary)] font-mono focus:outline-none focus:border-purple-500 transition-colors uppercase"
            />
          </div>
          
          <div className="mt-6 p-4 rounded-xl border border-[var(--border)] bg-[var(--bg-surface)]">
            <div className="text-xs font-medium text-[var(--text-muted)] mb-3">PREVIEW</div>
            <div className="flex gap-2 items-center">
              <span
                className="inline-flex px-2.5 py-1 rounded-md text-xs font-semibold"
                style={{
                  backgroundColor: `color-mix(in srgb, ${color} 15%, transparent)`,
                  color: color,
                }}
              >
                {name || "Status Name"}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div className="p-6 border-t border-[var(--border)] bg-[var(--bg-elevated)] flex gap-3">
        <button
          type="button"
          onClick={onCancel}
          disabled={loading}
          className="flex-1 px-4 py-2.5 rounded-lg text-sm font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] transition-colors disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={loading || !name.trim()}
          className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium text-white bg-purple-600 hover:bg-purple-500 transition-colors disabled:opacity-50"
        >
          {loading ? <Loader2 size={16} className="animate-spin" /> : "Save status"}
        </button>
      </div>
    </form>
  );
}
