// src/components/hrm/holidays/HolidayManager.tsx
"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { listHolidays, createHoliday, deleteHoliday } from "@/lib/hrm/holidays";
import type { CreateHolidayPayload } from "@/types/hrm";
import { queryKeys } from "@/lib/queryKeys";

const HOLIDAY_TYPES = [
  { value: "public", label: "Public" },
  { value: "company", label: "Company" },
  { value: "optional", label: "Optional" },
];

const inputCls = `
  px-3 py-2 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function HolidayManager({
  orgId,
  calendarId,
}: {
  orgId: string;
  calendarId: string;
}) {
  const queryClient = useQueryClient();
  const listKey = ["hrm", orgId, "holidays", calendarId] as const;
  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: () => listHolidays(orgId, calendarId).then((r) => r.holidays),
  });
  const items = (listQuery.data ?? [])
    .slice()
    .sort((a, b) => a.date.localeCompare(b.date));

  const [name, setName] = useState("");
  const [date, setDate] = useState("");
  const [type, setType] =
    useState<CreateHolidayPayload["holiday_type"]>("public");
  const [repeatYearly, setRepeatYearly] = useState(true);
  const [isPaid, setIsPaid] = useState(true);

  const refresh = () => queryClient.invalidateQueries({ queryKey: listKey });

  const handleAdd = async () => {
    if (!name.trim() || !date) return;
    try {
      await createHoliday(orgId, calendarId, {
        name: name.trim(),
        date,
        holiday_type: type,
        is_paid: isPaid,
        repeat_yearly: repeatYearly,
      });
      toast.success("Holiday added.");
      setName("");
      setDate("");
      refresh();
    } catch {
      toast.error(
        "Failed to add holiday — a holiday may already exist on this date.",
      );
    }
  };

  const handleRemove = async (holidayId: string) => {
    try {
      await deleteHoliday(orgId, calendarId, holidayId);
      toast.success("Holiday removed.");
      refresh();
    } catch {
      toast.error("Failed to remove holiday.");
    }
  };

  return (
    <div className="px-6 py-5 space-y-4">
      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-10 text-sm text-[var(--text-muted)] gap-3">
          <Loader2 size={16} className="animate-spin text-purple-500" />{" "}
          Loading…
        </div>
      ) : items.length === 0 ? (
        <p className="text-sm text-[var(--text-muted)]">
          No holidays in this calendar yet.
        </p>
      ) : (
        <div className="space-y-2">
          {items.map((h) => (
            <div
              key={h.id}
              className="flex items-center justify-between px-3.5 py-2.5 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)]"
            >
              <div>
                <p className="text-sm text-[var(--text-primary)]">{h.name}</p>
                <p className="text-xs text-[var(--text-muted)]">
                  {new Date(h.date).toLocaleDateString("en-US", {
                    month: "short",
                    day: "numeric",
                    year: "numeric",
                  })}{" "}
                  · {h.holiday_type} {h.repeat_yearly ? "· repeats yearly" : ""}{" "}
                  {!h.is_paid ? "· unpaid" : ""}
                </p>
              </div>
              <button
                onClick={() => handleRemove(h.id)}
                className="p-1.5 rounded-md text-red-400 hover:bg-red-500/10 transition-colors"
              >
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="pt-3 border-t border-[var(--border)] space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
          Add holiday
        </p>
        <div className="grid grid-cols-2 gap-2">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Name"
            className={inputCls}
          />
          <input
            value={date}
            onChange={(e) => setDate(e.target.value)}
            type="date"
            className={inputCls}
          />
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <select
            value={type}
            onChange={(e) =>
              setType(e.target.value as CreateHolidayPayload["holiday_type"])
            }
            className={inputCls}
          >
            {HOLIDAY_TYPES.map((t) => (
              <option
                key={t.value}
                value={t.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {t.label}
              </option>
            ))}
          </select>
          <label className="flex items-center gap-1.5 text-xs text-[var(--text-secondary)]">
            <input
              type="checkbox"
              checked={isPaid}
              onChange={(e) => setIsPaid(e.target.checked)}
              className="w-3.5 h-3.5 accent-purple-600"
            />
            Paid
          </label>
          <label className="flex items-center gap-1.5 text-xs text-[var(--text-secondary)]">
            <input
              type="checkbox"
              checked={repeatYearly}
              onChange={(e) => setRepeatYearly(e.target.checked)}
              className="w-3.5 h-3.5 accent-purple-600"
            />
            Repeats yearly
          </label>
          <button
            onClick={handleAdd}
            disabled={!name.trim() || !date}
            className="ml-auto flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-50 transition-colors"
          >
            <Plus size={14} />
            Add
          </button>
        </div>
      </div>
    </div>
  );
}
