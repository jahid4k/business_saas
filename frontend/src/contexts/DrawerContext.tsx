// src/contexts/DrawerContext.tsx
"use client";

import { createContext, useCallback, useContext, useState } from "react";
import Drawer from "@/components/ui/Drawer";

// ── Types ─────────────────────────────────────────────
interface DrawerConfig {
  title: string;
  content: React.ReactNode;
  width?: "sm" | "md" | "lg";
}

interface DrawerContextValue {
  openDrawer: (config: DrawerConfig) => void;
  closeDrawer: () => void;
}

// ── Context ───────────────────────────────────────────
const DrawerContext = createContext<DrawerContextValue | null>(null);

// ── Provider ──────────────────────────────────────────
export function DrawerProvider({ children }: { children: React.ReactNode }) {
  const [config, setConfig] = useState<DrawerConfig | null>(null);
  const [open, setOpen] = useState(false);

  const openDrawer = useCallback((cfg: DrawerConfig) => {
    setConfig(cfg);
    setOpen(true);
  }, []);

  const closeDrawer = useCallback(() => {
    setOpen(false);
    // Wait for the slide-out animation to finish before clearing content
    // so the panel doesn't flash empty while closing
    setTimeout(() => setConfig(null), 300);
  }, []);

  return (
    <DrawerContext.Provider value={{ openDrawer, closeDrawer }}>
      {children}
      {/* Single Drawer shell — always in DOM, GSAP controls visibility */}
      <Drawer
        open={open}
        title={config?.title ?? ""}
        width={config?.width}
        onClose={closeDrawer}
      >
        {config?.content}
      </Drawer>
    </DrawerContext.Provider>
  );
}

// ── Hook ──────────────────────────────────────────────
export function useDrawer(): DrawerContextValue {
  const ctx = useContext(DrawerContext);
  if (!ctx) throw new Error("useDrawer() must be used inside <DrawerProvider>");
  return ctx;
}
