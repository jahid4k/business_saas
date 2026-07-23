export type ThemeTokens = {
  bgCanvas: string;
  bgSurface: string;
  border: string;
  accent: string;
  accentHover: string;
  success: string;
  warning: string;
  destructive: string;
  textPrimary: string;
  textMuted: string;
};

export const tokens: Record<'light' | 'dark', ThemeTokens> = {
  light: {
    bgCanvas: "#f8fafc",
    bgSurface: "#ffffff",
    border: "#e2e8f0",
    accent: "#4f46e5",
    accentHover: "#4338ca",
    success: "#10b981",
    warning: "#f59e0b",
    destructive: "#ef4444",
    textPrimary: "#0f172a",
    textMuted: "#64748b"
  },
  dark: {
    bgCanvas: "#020617",
    bgSurface: "#0f172a",
    border: "#1e293b",
    accent: "#4f46e5",
    accentHover: "#4338ca",
    success: "#10b981",
    warning: "#f59e0b",
    destructive: "#ef4444",
    textPrimary: "#f8fafc",
    textMuted: "#94a3b8"
  },
};
