// src/components/ui/Drawer.tsx
"use client";

import { useEffect, useRef } from "react";
import { X } from "lucide-react";
import gsap from "gsap";

const WIDTH: Record<string, string> = {
  sm: "max-w-[360px]",
  md: "max-w-[440px]",
  lg: "max-w-[560px]",
};

interface DrawerProps {
  open: boolean;
  title: string;
  width?: "sm" | "md" | "lg";
  onClose: () => void;
  children: React.ReactNode;
}

export default function Drawer({
  open,
  title,
  width = "md",
  onClose,
  children,
}: DrawerProps) {
  const overlayRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const initRef = useRef(false);

  // Set off-screen initial state once — no animation on mount
  useEffect(() => {
    if (!panelRef.current || !overlayRef.current) return;
    gsap.set(panelRef.current, { x: "100%" });
    gsap.set(overlayRef.current, { opacity: 0, pointerEvents: "none" });
    initRef.current = true;
  }, []);

  // Slide in / out when open changes
  useEffect(() => {
    if (!initRef.current) return;
    if (open) {
      gsap.to(overlayRef.current, {
        opacity: 1,
        pointerEvents: "auto",
        duration: 0.2,
      });
      gsap.to(panelRef.current, { x: "0%", duration: 0.3, ease: "power3.out" });
    } else {
      gsap.to(panelRef.current, {
        x: "100%",
        duration: 0.25,
        ease: "power2.in",
      });
      gsap.to(overlayRef.current, {
        opacity: 0,
        pointerEvents: "none",
        duration: 0.2,
      });
    }
  }, [open]);

  // Escape key
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    if (open) document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onClose]);

  return (
    <div className="fixed inset-0 z-50 pointer-events-none">
      {/* Overlay */}
      <div
        ref={overlayRef}
        className="absolute inset-0 bg-black/60"
        onClick={onClose}
      />

      {/* Panel */}
      <div
        ref={panelRef}
        className={`
          absolute right-0 top-0 h-full w-full pointer-events-auto
          flex flex-col
          bg-[var(--bg-surface)] border-l border-[var(--border)] shadow-2xl
          ${WIDTH[width]}
        `}
      >
        {/* Header — title + close button. Always the same. */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)] flex-shrink-0">
          <h2
            className="text-base font-semibold text-[var(--text-primary)]"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            {title}
          </h2>
          <button
            onClick={onClose}
            className="p-1.5 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors"
          >
            <X size={16} />
          </button>
        </div>

        {/* Content — injected by DrawerContext */}
        <div className="flex-1 min-h-0 flex flex-col">{children}</div>
      </div>
    </div>
  );
}
