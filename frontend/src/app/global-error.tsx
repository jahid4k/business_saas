// src/app/global-error.tsx
// Catches errors thrown FROM the root layout.tsx itself (e.g. ThemeProvider crash).
// This is the last resort — it REPLACES the entire layout, so it MUST include
// its own <html> and <body> tags. Keep it minimal and dependency-free.
// Fired in production only — Next.js dev mode shows the overlay instead.
"use client";

import { useEffect } from "react";
import { RefreshCw } from "lucide-react";

interface Props {
  error: Error & { digest?: string };
  reset: () => void;
}

export default function GlobalError({ error, reset }: Props) {
  useEffect(() => {
    // Log to your error tracking service here (e.g. Sentry)
    console.error("[Global Error Boundary]", error);
  }, [error]);

  return (
    // Must include html + body — this replaces the entire root layout
    <html lang="en" className="dark">
      <body
        style={{
          margin: 0,
          minHeight: "100vh",
          background: "#0a0a0a",
          color: "white",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          padding: "24px",
          fontFamily: "Inter, system-ui, -apple-system, sans-serif",
          WebkitFontSmoothing: "antialiased",
        }}
      >
        {/* Minimal branded content — no external dependencies here,
            since the layout itself may have crashed */}
        <div style={{ textAlign: "center", maxWidth: 400 }}>
          {/* Logo mark */}
          <div
            style={{
              width: 44,
              height: 44,
              borderRadius: 11,
              background: "#7c3aed",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              margin: "0 auto 28px",
              boxShadow: "0 0 20px rgba(124,58,237,0.4)",
            }}
          >
            <span style={{ fontSize: 16, fontWeight: 800, color: "white" }}>
              B
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
            }}
          >
            BusinessSAAS is unavailable
          </h1>

          <p
            style={{
              fontSize: 13.5,
              lineHeight: 1.7,
              color: "rgba(255,255,255,0.35)",
              marginBottom: 32,
            }}
          >
            A critical error has occurred. This has been logged automatically.
            {error.digest && (
              <>
                {" "}
                Ref:{" "}
                <span
                  style={{
                    fontFamily: "monospace",
                    fontSize: 11,
                    color: "rgba(255,255,255,0.18)",
                  }}
                >
                  {error.digest}
                </span>
              </>
            )}
          </p>

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
              boxShadow: "0 0 20px rgba(124,58,237,0.35)",
            }}
          >
            <RefreshCw size={13} />
            Reload application
          </button>
        </div>
      </body>
    </html>
  );
}
