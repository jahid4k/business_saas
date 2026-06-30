// src/app/(dashboard)/[orgId]/error.tsx
// Error boundary for org-specific pages — this is the MOST USEFUL one.
// Because [orgId]/layout.tsx (OrgProvider) wraps this file, the error renders
// INSIDE the sidebar + topbar shell. Users stay oriented in the dashboard.
// Fires when any page under /[orgId]/ throws an unhandled exception.
"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { AlertTriangle, RefreshCw, Home } from "lucide-react";

interface Props {
  error: Error & { digest?: string };
  reset: () => void;
}

export default function OrgPageError({ error, reset }: Props) {
  const router = useRouter();

  useEffect(() => {
    console.error("[Org Page Error Boundary]", error);
  }, [error]);

  return (
    <div
      style={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        padding: "40px 24px",
        fontFamily: "var(--font-inter, Inter, sans-serif)",
        // No background override — inherits from the dashboard shell
      }}
    >
      <div style={{ textAlign: "center", maxWidth: 400 }}>
        {/* Icon */}
        <div
          style={{
            width: 48,
            height: 48,
            borderRadius: 12,
            background: "rgba(239,68,68,0.07)",
            border: "1px solid rgba(239,68,68,0.15)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            margin: "0 auto 22px",
          }}
        >
          <AlertTriangle
            size={20}
            style={{ color: "rgba(239,68,68,0.7)" }}
            strokeWidth={1.75}
          />
        </div>

        <h2
          style={{
            fontSize: 20,
            fontWeight: 700,
            lineHeight: 1.3,
            color: "white",
            marginBottom: 8,
            letterSpacing: "-0.3px",
            fontFamily: "var(--font-syne, Syne, sans-serif)",
          }}
        >
          This page failed to load
        </h2>

        <p
          style={{
            fontSize: 13,
            lineHeight: 1.7,
            color: "rgba(255,255,255,0.35)",
            marginBottom: 28,
          }}
        >
          An unexpected error occurred. You can retry or go back to the
          dashboard.
          {error.digest && (
            <span
              style={{
                display: "block",
                marginTop: 8,
                fontFamily: "monospace",
                fontSize: 10,
                color: "rgba(255,255,255,0.14)",
              }}
            >
              Error ref: {error.digest}
            </span>
          )}
        </p>

        {/* Dev-only: show the message in development */}
        {process.env.NODE_ENV === "development" && error.message && (
          <div
            style={{
              marginBottom: 24,
              padding: "10px 14px",
              borderRadius: 8,
              background: "rgba(239,68,68,0.05)",
              border: "1px solid rgba(239,68,68,0.12)",
              textAlign: "left",
            }}
          >
            <p
              style={{
                fontSize: 10,
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.08em",
                color: "rgba(239,68,68,0.6)",
                marginBottom: 6,
              }}
            >
              Dev — error message
            </p>
            <code
              style={{
                display: "block",
                fontSize: 11,
                color: "rgba(255,255,255,0.28)",
                fontFamily: "monospace",
                wordBreak: "break-word",
                lineHeight: 1.6,
              }}
            >
              {error.message}
            </code>
          </div>
        )}

        {/* Actions */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 10,
          }}
        >
          <button
            onClick={reset}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              background: "#7c3aed",
              color: "white",
              padding: "9px 18px",
              borderRadius: 8,
              fontSize: 12.5,
              fontWeight: 600,
              border: "none",
              cursor: "pointer",
              boxShadow: "0 0 18px rgba(124,58,237,0.28)",
              transition: "background 0.15s",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "#6d28d9")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "#7c3aed")}
          >
            <RefreshCw size={11} />
            Retry
          </button>

          <button
            onClick={() => router.back()}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              padding: "9px 18px",
              borderRadius: 8,
              fontSize: 12.5,
              fontWeight: 500,
              border: "1px solid rgba(255,255,255,0.09)",
              background: "transparent",
              color: "rgba(255,255,255,0.38)",
              cursor: "pointer",
              transition: "border-color 0.15s, color 0.15s",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.18)";
              e.currentTarget.style.color = "rgba(255,255,255,0.65)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.09)";
              e.currentTarget.style.color = "rgba(255,255,255,0.38)";
            }}
          >
            <Home size={11} />
            Dashboard
          </button>
        </div>
      </div>
    </div>
  );
}
