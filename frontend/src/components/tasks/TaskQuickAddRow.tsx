// src/components/tasks/TaskQuickAddRow.tsx
"use client";

import { useEffect, useRef, useState } from "react";
import { Plus } from "lucide-react";

interface TaskQuickAddRowProps {
  isAdding: boolean;
  onStartAdding: () => void;
  onCancel: () => void;
  onCreate: (title: string) => Promise<void>;
  placeholder?: string;
}

export default function TaskQuickAddRow({
  isAdding,
  onStartAdding,
  onCancel,
  onCreate,
  placeholder = "New task",
}: TaskQuickAddRowProps) {
  const [value, setValue] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isAdding) inputRef.current?.focus();
  }, [isAdding]);

  const submit = async (keepOpen: boolean) => {
    const title = value.trim();
    if (!title) {
      onCancel();
      return;
    }
    setValue("");
    try {
      await onCreate(title);
      if (keepOpen) inputRef.current?.focus();
      else onCancel();
    } catch {
      // Restore the typed title so nothing is lost on a failed request.
      setValue(title);
    }
  };

  if (!isAdding) {
    return (
      <button
        onClick={onStartAdding}
        className="flex items-center gap-2 w-full pl-1.5 pr-1.5 py-1.25 rounded-md text-[13px] text-(--text-muted) hover:text-(--text-primary) hover:bg-(--bg-elevated) transition-colors text-left"
      >
        <Plus size={13} />
        {placeholder}
      </button>
    );
  }

  return (
    <div className="flex items-center gap-2.5 pl-1.5 pr-1.5 py-1.25">
      <div className="w-3.75 h-3.75 rounded-sm border border-(--border) shrink-0" />
      <input
        ref={inputRef}
        type="text"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            submit(true);
          }
          if (e.key === "Escape") {
            setValue("");
            onCancel();
          }
        }}
        onBlur={() => submit(false)}
        placeholder="Task name"
        className="flex-1 bg-transparent outline-none text-[13px] text-(--text-primary) placeholder:text-(--text-muted)"
      />
    </div>
  );
}
