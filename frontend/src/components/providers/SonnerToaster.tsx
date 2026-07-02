// src/components/providers/SonnerToaster.tsx
"use client";

import { Toaster } from "sonner";
import { useTheme } from "next-themes";

/**
 * Renders Sonner's toast container with theme kept in sync with next-themes.
 * Must be rendered inside <ThemeProvider> so useTheme() resolves correctly.
 * The `?? "dark"` fallback covers the brief SSR window where resolvedTheme
 * is undefined — matching the app's default dark theme.
 */
export function SonnerToaster() {
  const { resolvedTheme } = useTheme();
  return (
    <Toaster
      theme={(resolvedTheme as "dark" | "light") ?? "dark"}
      position="bottom-right"
      richColors
      closeButton
      duration={3500}
    />
  );
}
