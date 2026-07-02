// src/app/(dashboard)/error.tsx
// Error boundary for the dashboard layout group — catches errors that happen
// INSIDE the AuthProvider shell but outside the org-specific [orgId] pages.
// Renders WITHOUT the sidebar/topbar (because those are inside [orgId]/layout).
// In practice this fires rarely, but it's the safety net between auth and org context.
"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { AlertTriangle, RefreshCw, LogIn } from "lucide-react";

interface Props {
  error: Error & { digest?: string };
  reset: () => void;
}

export default function DashboardError({ error, reset }: Props) {
  const router = useRouter();

  useEffect(() => {
    console.error("[Dashboard Error Boundary]", error);
  }, [error]);

  return (
    <div
      style={{
        minHeight: "100vh",
        background: "#0a0a0a",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        padding: "24px",
        fontFamily: "var(--font-inter, Inter, sans-serif)",
      }}
    >
      {/* Texture */}
      <div
        aria-hidden
        style={{
          position: "fixed",
          inset: 0,
          backgroundImage:
            "radial-gradient(circle, rgba(255,255,255,0.015) 1px, transparent 1px)",
          backgroundSize: "32px 32px",
          pointerEvents: "none",
        }}
      />

      <div
        style={{
          position: "relative",
          textAlign: "center",
          maxWidth: 420,
        }}
      >
        {/* Icon */}
        <div
          style={{
            width: 52,
            height: 52,
            borderRadius: 13,
            background: "rgba(239,68,68,0.07)",
            border: "1px solid rgba(239,68,68,0.15)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            margin: "0 auto 24px",
          }}
        >
          <AlertTriangle
            size={20}
            style={{ color: "rgba(239,68,68,0.7)" }}
            strokeWidth={1.75}
          />
        </div>

        {/* Logo */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 8,
            marginBottom: 28,
          }}
        >
          <div
            style={{
              width: 22,
              height: 22,
              borderRadius: 6,
              background: "#7c3aed",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <span style={{ fontSize: 9, fontWeight: 800, color: "white" }}>
              B
            </span>
          </div>
          <span
            style={{
              fontSize: 13,
              fontWeight: 600,
              color: "rgba(255,255,255,0.4)",
              letterSpacing: "-0.2px",
            }}
          >
            BusinessSAAS
          </span>
        </div>

        <h1
          style={{
            fontSize: 22,
            fontWeight: 700,
            lineHeight: 1.3,
            color: "white",
            marginBottom: 10,
            letterSpacing: "-0.3px",
            fontFamily: "var(--font-syne, Syne, sans-serif)",
          }}
        >
          Session error
        </h1>

        <p
          style={{
            fontSize: 13.5,
            lineHeight: 1.72,
            color: "rgba(255,255,255,0.34)",
            marginBottom: 32,
          }}
        >
          Something went wrong loading your workspace. Try reloading or sign in
          again.
          {error.digest && (
            <>
              {" "}
              <span
                style={{
                  fontFamily: "monospace",
                  fontSize: 10,
                  color: "rgba(255,255,255,0.15)",
                }}
              >
                ({error.digest})
              </span>
            </>
          )}
        </p>

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
              gap: 8,
              background: "#7c3aed",
              color: "white",
              padding: "10px 20px",
              borderRadius: 8,
              fontSize: 13,
              fontWeight: 600,
              border: "none",
              cursor: "pointer",
              boxShadow: "0 0 20px rgba(124,58,237,0.28)",
              transition: "background 0.15s",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "#6d28d9")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "#7c3aed")}
          >
            <RefreshCw size={12} />
            Try again
          </button>

          <button
            onClick={() => router.replace("/login")}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              padding: "10px 20px",
              borderRadius: 8,
              fontSize: 13,
              fontWeight: 500,
              border: "1px solid rgba(255,255,255,0.09)",
              background: "transparent",
              color: "rgba(255,255,255,0.4)",
              cursor: "pointer",
              transition: "border-color 0.15s, color 0.15s",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.18)";
              e.currentTarget.style.color = "rgba(255,255,255,0.7)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.09)";
              e.currentTarget.style.color = "rgba(255,255,255,0.4)";
            }}
          >
            <LogIn size={12} />
            Sign in again
          </button>
        </div>
      </div>
    </div>
  );
}
