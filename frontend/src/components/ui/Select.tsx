"use client";

import { useState, useRef, useEffect } from "react";
import { ChevronDown } from "lucide-react";

export interface Option {
  value: string;
  label: string;
}

interface SelectProps {
  value: string;
  onChange: (value: string) => void;
  options: Option[];
  disabled?: boolean;
}

export function Select({ value, onChange, options, disabled }: SelectProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };

    if (open) {
      document.addEventListener("mousedown", handleClickOutside);
    }

    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const selectedOption = options.find((o) => o.value === value) || options[0];

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => !disabled && setOpen(!open)}
        className={`w-full flex items-center justify-between px-3.5 py-2.5 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary) outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
      >
        <span>{selectedOption?.label}</span>
        <ChevronDown size={16} className="text-(--text-muted)" />
      </button>

      {open && (
        <div className="absolute top-full left-0 mt-1.5 w-full z-50 bg-(--bg-elevated) border border-(--border) rounded-lg shadow-lg overflow-hidden py-1">
          {options.map((o) => (
            <button
              key={o.value}
              type="button"
              onClick={() => {
                onChange(o.value);
                setOpen(false);
              }}
              className={`w-full text-left px-3.5 py-2 text-sm transition-colors ${
                o.value === value
                  ? "bg-purple-500/10 text-purple-400 font-medium"
                  : "text-(--text-primary) hover:bg-(--bg-surface)"
              }`}
            >
              {o.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
