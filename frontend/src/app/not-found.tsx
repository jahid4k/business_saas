// src/app/not-found.tsx
// Next.js App Router 404 — fires on unmatched routes and notFound() calls.
// Dark-first · matches marketing page aesthetic · purple accent
"use client";

import Link from "next/link";
import { useEffect, useRef } from "react";
import { ArrowLeft, ArrowRight } from "lucide-react";

export default function NotFound() {
  const numRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let revert: (() => void) | undefined;

    (async () => {
      const gsap = (await import("gsap")).default;

      const ctx = gsap.context(() => {
        gsap
          .timeline({ defaults: { ease: "power3.out" } })
          .fromTo(
            numRef.current,
            { opacity: 0, y: 40, scale: 0.92 },
            { opacity: 1, y: 0, scale: 1, duration: 0.8 },
          )
          .fromTo(
            contentRef.current,
            { opacity: 0, y: 24 },
            { opacity: 1, y: 0, duration: 0.6 },
            "-=0.4",
          );
      });

      revert = () => ctx.revert();
    })();

    return () => revert?.();
  }, []);

  return (
    <div
      style={{
        minHeight: "100vh",
        background: "#0a0a0a",
        color: "white",
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
      {/* Background noise texture */}
      <div
        aria-hidden
        style={{
          position: "absolute",
          inset: 0,
          backgroundImage:
            "radial-gradient(circle, rgba(255,255,255,0.018) 1px, transparent 1px)",
          backgroundSize: "32px 32px",
          pointerEvents: "none",
        }}
      />

      {/* Purple radial glow — top left */}
      <div
        aria-hidden
        style={{
          position: "absolute",
          top: "-10%",
          left: "5%",
          width: 560,
          height: 400,
          background:
            "radial-gradient(ellipse at center, rgba(124,58,237,0.11) 0%, transparent 68%)",
          filter: "blur(60px)",
          pointerEvents: "none",
        }}
      />

      {/* Nav bar — minimal */}
      <header
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          padding: "18px 28px",
          display: "flex",
          alignItems: "center",
          borderBottom: "1px solid rgba(255,255,255,0.05)",
        }}
      >
        <Link
          href="/"
          style={{
            display: "flex",
            alignItems: "center",
            gap: 9,
            textDecoration: "none",
          }}
        >
          <div
            style={{
              width: 26,
              height: 26,
              borderRadius: 7,
              background: "#7c3aed",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              boxShadow: "0 0 12px rgba(124,58,237,0.45)",
            }}
          >
            <span style={{ fontSize: 10, fontWeight: 800, color: "white" }}>
              B
            </span>
          </div>
          <span
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: "rgba(255,255,255,0.55)",
              letterSpacing: "-0.2px",
            }}
          >
            BusinessSAAS
          </span>
        </Link>
      </header>

      {/* Main content */}
      <div style={{ position: "relative", textAlign: "center", maxWidth: 520 }}>
        {/* Large 404 */}
        <div ref={numRef} style={{ position: "relative", marginBottom: 32 }}>
          {/* Glowing backdrop number */}
          <div
            aria-hidden
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: "clamp(140px, 18vw, 220px)",
              fontWeight: 900,
              letterSpacing: "-8px",
              color: "transparent",
              WebkitTextStroke: "1px rgba(255,255,255,0.03)",
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              userSelect: "none",
              pointerEvents: "none",
              filter: "blur(1px)",
              transform: "scale(1.04)",
            }}
          >
            404
          </div>

          {/* Foreground number */}
          <div
            style={{
              fontSize: "clamp(100px, 14vw, 160px)",
              fontWeight: 800,
              letterSpacing: "-4px",
              lineHeight: 1,
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              background:
                "linear-gradient(145deg, rgba(255,255,255,0.9) 0%, rgba(124,58,237,0.6) 55%, rgba(168,85,247,0.4) 100%)",
              WebkitBackgroundClip: "text",
              WebkitTextFillColor: "transparent",
              backgroundClip: "text",
              position: "relative",
            }}
          >
            404
          </div>
        </div>

        {/* Copy */}
        <div ref={contentRef}>
          {/* Label */}
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              borderRadius: 100,
              border: "1px solid rgba(124,58,237,0.22)",
              background: "rgba(124,58,237,0.08)",
              padding: "5px 14px",
              marginBottom: 24,
            }}
          >
            <span
              style={{
                width: 5,
                height: 5,
                borderRadius: "50%",
                background: "#a855f7",
                display: "inline-block",
                flexShrink: 0,
              }}
            />
            <span
              style={{
                fontSize: 11,
                fontWeight: 600,
                color: "#a855f7",
                letterSpacing: "0.04em",
              }}
            >
              Page not found
            </span>
          </div>

          <h1
            style={{
              fontSize: "clamp(22px, 3vw, 30px)",
              fontWeight: 700,
              lineHeight: 1.25,
              letterSpacing: "-0.3px",
              color: "white",
              marginBottom: 14,
              fontFamily: "var(--font-syne, Syne, sans-serif)",
            }}
          >
            This page doesn&apos;t exist.
          </h1>

          <p
            style={{
              fontSize: 14,
              lineHeight: 1.75,
              color: "rgba(255,255,255,0.36)",
              marginBottom: 36,
              maxWidth: 380,
              margin: "0 auto 36px",
            }}
          >
            The URL you followed may be incorrect, the page may have moved, or
            you might not have permission to view it.
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
            <Link
              href="/"
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
                textDecoration: "none",
                boxShadow: "0 0 24px rgba(124,58,237,0.38)",
                transition: "background 0.15s",
              }}
              onMouseEnter={(e) =>
                (e.currentTarget.style.background = "#6d28d9")
              }
              onMouseLeave={(e) =>
                (e.currentTarget.style.background = "#7c3aed")
              }
            >
              <ArrowLeft size={13} />
              Back to Home
            </Link>

            <Link
              href="/login"
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 8,
                padding: "11px 22px",
                borderRadius: 9,
                fontSize: 13,
                fontWeight: 500,
                textDecoration: "none",
                border: "1px solid rgba(255,255,255,0.09)",
                color: "rgba(255,255,255,0.45)",
                transition: "border-color 0.15s, color 0.15s",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = "rgba(255,255,255,0.16)";
                e.currentTarget.style.color = "rgba(255,255,255,0.7)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = "rgba(255,255,255,0.09)";
                e.currentTarget.style.color = "rgba(255,255,255,0.45)";
              }}
            >
              Sign in
              <ArrowRight size={13} />
            </Link>
          </div>

          {/* Footer hint */}
          <p
            style={{
              marginTop: 40,
              fontSize: 11,
              color: "rgba(255,255,255,0.12)",
              letterSpacing: "0.02em",
            }}
          >
            If this keeps happening, contact your workspace admin.
          </p>
        </div>
      </div>
    </div>
  );
}
