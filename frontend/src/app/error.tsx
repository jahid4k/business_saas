// src/app/error.tsx
// Root-level error boundary — catches unhandled errors in root-level pages
// (e.g. the marketing homepage). Does NOT catch errors inside layouts.
// Must be a Client Component and receive `error` + `reset` props.
"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle, RefreshCw } from "lucide-react";

interface Props {
  error: Error & { digest?: string };
  reset: () => void;
}

export default function RootError({ error, reset }: Props) {
  useEffect(() => {
    // Log to your error tracking service here (e.g. Sentry)
    console.error("[Root Error Boundary]", error);
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
        position: "relative",
        overflow: "hidden",
      }}
    >
      {/* Background texture */}
      <div
        aria-hidden
        style={{
          position: "absolute",
          inset: 0,
          backgroundImage:
            "radial-gradient(circle, rgba(255,255,255,0.015) 1px, transparent 1px)",
          backgroundSize: "32px 32px",
          pointerEvents: "none",
        }}
      />

      {/* Red-tinted glow for error state */}
      <div
        aria-hidden
        style={{
          position: "absolute",
          top: "20%",
          left: "30%",
          width: 500,
          height: 300,
          background:
            "radial-gradient(ellipse at center, rgba(239,68,68,0.06) 0%, transparent 68%)",
          filter: "blur(60px)",
          pointerEvents: "none",
        }}
      />

      <div
        style={{
          position: "relative",
          textAlign: "center",
          maxWidth: 460,
        }}
      >
        {/* Icon */}
        <div
          style={{
            width: 56,
            height: 56,
            borderRadius: 14,
            background: "rgba(239,68,68,0.08)",
            border: "1px solid rgba(239,68,68,0.18)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            margin: "0 auto 28px",
          }}
        >
          <AlertTriangle
            size={22}
            style={{ color: "rgba(239,68,68,0.75)" }}
            strokeWidth={1.75}
          />
        </div>

        {/* Badge */}
        <div
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            borderRadius: 100,
            border: "1px solid rgba(239,68,68,0.2)",
            background: "rgba(239,68,68,0.07)",
            padding: "5px 14px",
            marginBottom: 22,
          }}
        >
          <span
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: "rgba(239,68,68,0.8)",
              letterSpacing: "0.04em",
            }}
          >
            Something went wrong
          </span>
        </div>

        <h1
          style={{
            fontSize: "clamp(20px, 2.5vw, 26px)",
            fontWeight: 700,
            lineHeight: 1.3,
            letterSpacing: "-0.3px",
            color: "white",
            marginBottom: 12,
            fontFamily: "var(--font-syne, Syne, sans-serif)",
          }}
        >
          An unexpected error occurred.
        </h1>

        <p
          style={{
            fontSize: 13.5,
            lineHeight: 1.72,
            color: "rgba(255,255,255,0.34)",
            marginBottom: 32,
          }}
        >
          The application encountered an error it couldn&apos;t recover from
          automatically.
          {error.digest && (
            <>
              {" "}
              Error reference:{" "}
              <code
                style={{
                  fontFamily: "monospace",
                  fontSize: 11,
                  color: "rgba(255,255,255,0.2)",
                  background: "rgba(255,255,255,0.04)",
                  padding: "1px 5px",
                  borderRadius: 4,
                }}
              >
                {error.digest}
              </code>
            </>
          )}
        </p>

        {/* Actions */}
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
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
              padding: "11px 22px",
              borderRadius: 9,
              fontSize: 13,
              fontWeight: 600,
              border: "none",
              cursor: "pointer",
              boxShadow: "0 0 20px rgba(124,58,237,0.32)",
              transition: "background 0.15s",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "#6d28d9")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "#7c3aed")}
          >
            <RefreshCw size={13} />
            Try again
          </button>

          <Link
            href="/"
            style={{
              display: "inline-flex",
              alignItems: "center",
              padding: "11px 22px",
              borderRadius: 9,
              fontSize: 13,
              fontWeight: 500,
              textDecoration: "none",
              border: "1px solid rgba(255,255,255,0.09)",
              color: "rgba(255,255,255,0.42)",
              transition: "border-color 0.15s, color 0.15s",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.16)";
              e.currentTarget.style.color = "rgba(255,255,255,0.7)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = "rgba(255,255,255,0.09)";
              e.currentTarget.style.color = "rgba(255,255,255,0.42)";
            }}
          >
            Go to homepage
          </Link>
        </div>
      </div>
    </div>
  );
}
