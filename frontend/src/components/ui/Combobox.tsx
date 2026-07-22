"use client";

import { useState, useRef, useEffect, useMemo } from "react";
import { ChevronDown, Search } from "lucide-react";
import type { Option } from "./Select";

interface ComboboxProps {
  value: string;
  onChange: (value: string) => void;
  options: Option[];
  disabled?: boolean;
  placeholder?: string;
  searchPlaceholder?: string;
}

export function Combobox({
  value,
  onChange,
  options,
  disabled,
  placeholder = "Select an option",
  searchPlaceholder = "Search...",
}: ComboboxProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };

    if (open) {
      document.addEventListener("mousedown", handleClickOutside);
      // Focus input when opened
      setTimeout(() => inputRef.current?.focus(), 0);
    } else {
      setSearch(""); // Reset search when closed
    }

    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const selectedOption = options.find((o) => o.value === value);

  const filteredOptions = useMemo(() => {
    if (!search) return options;
    const lower = search.toLowerCase();
    return options.filter((o) => o.label.toLowerCase().includes(lower));
  }, [options, search]);

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => !disabled && setOpen(!open)}
        className={`w-full flex items-center justify-between px-3.5 py-2.5 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary) outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
      >
        <span className={selectedOption ? "" : "text-(--text-muted)"}>
          {selectedOption ? selectedOption.label : placeholder}
        </span>
        <ChevronDown size={16} className="text-(--text-muted)" />
      </button>

      {open && (
        <div className="absolute top-full left-0 mt-1.5 w-full z-50 bg-(--bg-elevated) border border-(--border) rounded-lg shadow-lg overflow-hidden flex flex-col max-h-60">
          <div className="p-2 border-b border-(--border) sticky top-0 bg-(--bg-elevated) z-10 flex items-center gap-2">
            <Search size={14} className="text-(--text-muted) ml-1" />
            <input
              ref={inputRef}
              type="text"
              className="w-full bg-transparent text-sm outline-none text-(--text-primary) placeholder:text-(--text-muted)"
              placeholder={searchPlaceholder}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onClick={(e) => e.stopPropagation()}
            />
          </div>
          <div className="overflow-y-auto py-1">
            {filteredOptions.length === 0 ? (
              <div className="px-3.5 py-2 text-sm text-(--text-muted) text-center">
                No results found
              </div>
            ) : (
              filteredOptions.map((o) => (
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
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
