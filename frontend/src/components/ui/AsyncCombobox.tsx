"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { ChevronDown, Search, Loader2 } from "lucide-react";

export interface AsyncOption {
  value: string;
  label: string;
}

interface AsyncComboboxProps {
  value: string;
  onChange: (value: string, label: string) => void;
  fetchOptions: (query: string) => Promise<AsyncOption[]>;
  defaultLabel?: string;
  disabled?: boolean;
  placeholder?: string;
  searchPlaceholder?: string;
  debounceMs?: number;
}

export function AsyncCombobox({
  value,
  onChange,
  fetchOptions,
  defaultLabel,
  disabled,
  placeholder = "Select an option",
  searchPlaceholder = "Search...",
  debounceMs = 300,
}: AsyncComboboxProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [options, setOptions] = useState<AsyncOption[]>([]);
  const [loading, setLoading] = useState(false);

  // We need to keep track of the selected label so it renders even when options aren't loaded
  const [selectedLabel, setSelectedLabel] = useState<string | undefined>(
    defaultLabel,
  );

  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceTimer = useRef<NodeJS.Timeout | null>(null);

  // Sync defaultLabel if it changes externally
  useEffect(() => {
    if (defaultLabel) setSelectedLabel(defaultLabel);
  }, [defaultLabel]);

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

      // Load initial options if empty
      if (options.length === 0) {
        loadOptions("");
      }
    } else {
      setSearch(""); // Reset search when closed
    }

    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const loadOptions = useCallback(
    async (query: string) => {
      setLoading(true);
      try {
        const results = await fetchOptions(query);
        setOptions(results);
      } catch (err) {
        console.error("AsyncCombobox fetch error:", err);
      } finally {
        setLoading(false);
      }
    },
    [fetchOptions],
  );

  useEffect(() => {
    if (!open) return;

    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = setTimeout(() => {
      loadOptions(search);
    }, debounceMs);

    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current);
    };
  }, [search, open, loadOptions, debounceMs]);

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => !disabled && setOpen(!open)}
        className={`w-full flex items-center justify-between px-3.5 py-2.5 rounded-lg text-sm bg-(--bg-elevated) border border-(--border) text-(--text-primary) outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15 transition-all ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
      >
        <span className={selectedLabel ? "" : "text-(--text-muted)"}>
          {selectedLabel || placeholder}
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
            {loading ? (
              <div className="px-3.5 py-4 text-sm text-(--text-muted) flex items-center justify-center gap-2">
                <Loader2 size={16} className="animate-spin" />
                Loading...
              </div>
            ) : options.length === 0 ? (
              <div className="px-3.5 py-2 text-sm text-(--text-muted) text-center">
                No results found
              </div>
            ) : (
              options.map((o) => (
                <button
                  key={o.value}
                  type="button"
                  onClick={() => {
                    setSelectedLabel(o.label);
                    onChange(o.value, o.label);
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
