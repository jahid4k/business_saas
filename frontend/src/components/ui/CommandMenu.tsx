// src/components/ui/CommandMenu.tsx
"use client";

import { useEffect, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import { Command } from "cmdk";
import {
  Search,
  FileText,
  Settings,
  Building,
  Sun,
  Moon,
  LayoutDashboard,
} from "lucide-react";
import { useCommandStore } from "@/stores/commandStore";
import { useTheme } from "next-themes";
import { useUiStore } from "@/stores/uiStore";

export default function CommandMenu() {
  const router = useRouter();
  const params = useParams();
  const orgId = params.orgId as string | undefined;
  const { isOpen, setOpen } = useCommandStore();
  const { resolvedTheme, setTheme } = useTheme();
  const { setUiTheme } = useUiStore();

  const [mounted, setMounted] = useState(false);
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setMounted(true), []);

  // Toggle the menu when ⌘K or / is pressed
  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen(true);
      }

      if (e.key === "/" && !isOpen) {
        // Only prevent default if we're not already focused on an input
        if (
          document.activeElement?.tagName !== "INPUT" &&
          document.activeElement?.tagName !== "TEXTAREA"
        ) {
          e.preventDefault();
          setOpen(true);
        }
      }

      if (e.key === "Escape" && isOpen) {
        e.preventDefault();
        setOpen(false);
      }
    };

    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, [setOpen, isOpen]);

  const runCommand = (command: () => void) => {
    setOpen(false);
    command();
  };

  const toggleTheme = () => {
    const next = resolvedTheme === "dark" ? "light" : "dark";
    setTheme(next);
    setUiTheme(next as "dark" | "light");
  };

  if (!mounted || !isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh] sm:pt-[20vh] pointer-events-auto">
      {/* Overlay */}
      <div
        className="fixed inset-0 bg-black/50 backdrop-blur-sm transition-opacity"
        onClick={() => setOpen(false)}
      />

      {/* Modal */}
      <Command
        className="relative z-50 flex w-full max-w-160 flex-col overflow-hidden rounded-xl border border-(--border) bg-(--bg-elevated) shadow-2xl mx-4 sm:mx-0 animate-in fade-in zoom-in-95 duration-200"
        loop
      >
        <div className="flex items-center border-b border-(--border) px-4">
          <Search className="mr-3 h-5 w-5 text-(--text-muted) shrink-0" />
          <Command.Input
            autoFocus
            className="flex h-14 w-full rounded-md bg-transparent py-3 text-sm outline-none placeholder:text-(--text-muted) text-(--text-primary) disabled:cursor-not-allowed disabled:opacity-50"
            placeholder="Type a command or search..."
          />
        </div>

        <Command.List className="max-h-75 overflow-y-auto overflow-x-hidden p-2">
          <Command.Empty className="py-6 text-center text-sm text-(--text-muted)">
            No results found.
          </Command.Empty>

          <Command.Group
            heading="Suggestions"
            className="px-2 text-xs font-medium text-(--text-muted) **:[[cmdk-group-heading]]:px-2 **:[[cmdk-group-heading]]:py-1.5 **:[[cmdk-group-heading]]:text-(--text-muted) **:[[cmdk-group-heading]]:font-semibold"
          >
            {orgId && (
              <Command.Item
                onSelect={() => runCommand(() => router.push(`/${orgId}`))}
                className="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-2.5 text-sm outline-none aria-selected:bg-(--bg-surface) aria-selected:text-(--text-primary) data-disabled:pointer-events-none data-disabled:opacity-50 text-(--text-secondary) transition-colors"
              >
                <LayoutDashboard className="mr-2 h-4 w-4" />
                Go to Dashboard
              </Command.Item>
            )}
            {orgId && (
              <Command.Item
                onSelect={() =>
                  runCommand(() => router.push(`/${orgId}/crm/leads`))
                }
                className="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-2.5 text-sm outline-none aria-selected:bg-(--bg-surface) aria-selected:text-(--text-primary) data-disabled:pointer-events-none data-disabled:opacity-50 text-(--text-secondary) transition-colors"
              >
                <FileText className="mr-2 h-4 w-4" />
                Go to Leads
              </Command.Item>
            )}
            <Command.Item
              onSelect={() => runCommand(toggleTheme)}
              className="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-2.5 text-sm outline-none c data-disabled:pointer-events-none data-disabled:opacity-50 text-(--text-secondary) transition-colors"
            >
              {resolvedTheme === "dark" ? (
                <Sun className="mr-2 h-4 w-4" />
              ) : (
                <Moon className="mr-2 h-4 w-4" />
              )}
              Toggle Theme
            </Command.Item>
          </Command.Group>

          <Command.Separator className="-mx-2 h-px bg-(--border) my-1" />

          <Command.Group
            heading="Quick Actions"
            className="px-2 text-xs font-medium text-(--text-muted) **:[[cmdk-group-heading]]:px-2 **:[[cmdk-group-heading]]:py-1.5 **:[[cmdk-group-heading]]:text-(--text-muted) **:[[cmdk-group-heading]]:font-semibold"
          >
            {orgId && (
              <Command.Item
                onSelect={() =>
                  runCommand(() => router.push(`/${orgId}/settings`))
                }
                className="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-2.5 text-sm outline-none aria-selected:bg-(--bg-surface) aria-selected:text-(--text-primary) data-disabled:pointer-events-none data-disabled:opacity-50 text-(--text-secondary) transition-colors"
              >
                <Settings className="mr-2 h-4 w-4" />
                Settings
              </Command.Item>
            )}
            <Command.Item
              onSelect={() =>
                runCommand(() => router.push("/select-organization"))
              }
              className="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-2.5 text-sm outline-none aria-selected:bg-(--bg-surface) aria-selected:text-(--text-primary) data-disabled:pointer-events-none data-disabled:opacity-50 text-(--text-secondary) transition-colors"
            >
              <Building className="mr-2 h-4 w-4" />
              Switch Workspace
            </Command.Item>
          </Command.Group>
        </Command.List>
      </Command>

      {/* Global styles for cmdk that are hard to target with tailwind utility classes alone */}
      <style
        dangerouslySetInnerHTML={{
          __html: `
        [cmdk-group-heading] {
          padding-left: 0.5rem;
          padding-right: 0.5rem;
          padding-top: 0.375rem;
          padding-bottom: 0.375rem;
          font-size: 0.75rem;
          line-height: 1rem;
          font-weight: 500;
          color: var(--text-muted);
        }
      `,
        }}
      />
    </div>
  );
}
