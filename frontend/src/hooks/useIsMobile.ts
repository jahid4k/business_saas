// src/hooks/useIsMobile.ts
"use client";

import { useSyncExternalStore } from "react";

// Matches Tailwind's `lg` breakpoint — same threshold used in Sidebar and Topbar.
const QUERY = "(max-width: 1023px)";

// Both functions are defined at module level so they're stable references —
// useSyncExternalStore requires subscribe to not change identity on every render.
function subscribe(callback: () => void) {
  if (typeof window === "undefined") return () => {};
  const mq = window.matchMedia(QUERY);
  mq.addEventListener("change", callback);
  return () => mq.removeEventListener("change", callback);
}

function getSnapshot() {
  if (typeof window === "undefined") return false;
  return window.matchMedia(QUERY).matches;
}

/**
 * Returns true when the viewport is below Tailwind's `lg` breakpoint (1024px).
 *
 * Uses useSyncExternalStore — React's recommended pattern for subscribing to
 * browser APIs. Avoids the setState-in-effect anti-pattern and handles SSR
 * by returning false from getServerSnapshot (same safe default as before).
 *
 * Pipeline uses this to switch between the multi-column DnD kanban (desktop)
 * and the single-stage view with explicit stage-move controls (mobile).
 */
export function useIsMobile(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => false);
}
