"use client";

// ═══════════════════════════════════════════════════════════════════
// app/page.tsx
// BusinessSAAS — Public marketing homepage
// Dark-first · Purple accent · Inter headings · GSAP entrance
// ═══════════════════════════════════════════════════════════════════

import React, { useEffect, useRef } from "react";
import Link from "next/link";
import {
  ArrowRight,
  Shield,
  GitBranch,
  Activity,
  Lock,
  Server,
  FileText,
  CheckCircle,
  Building2,
} from "lucide-react";
import { Inter } from "next/font/google";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

// ─── Data ────────────────────────────────────────────────────────────

const MODULES = [
  {
    name: "CRM",
    desc: "Full-cycle relationship management — leads, deals, contact timelines, and pipeline analytics across your entire organization.",
    status: "live" as const,
    meta: "73 API endpoints · Production-ready",
  },
  {
    name: "HRM",
    desc: "Departments, headcount, leave management, payroll processing, and organizational hierarchy in a single system of record.",
    status: "soon" as const,
    meta: "131 endpoints planned",
  },
  {
    name: "Accounting",
    desc: "Invoices, expense tracking, bank reconciliation, and executive-level financial reporting.",
    status: "soon" as const,
    meta: "Coming Q3 2026",
  },
  {
    name: "Projects",
    desc: "Task tracking, milestone management, resource allocation, and cross-team workload visibility.",
    status: "soon" as const,
    meta: "Coming Q4 2026",
  },
  {
    name: "E-commerce",
    desc: "Product catalog, order management, customer segmentation, and real-time inventory control.",
    status: "soon" as const,
    meta: "Coming 2027",
  },
  {
    name: "Learning",
    desc: "Internal training programs, course delivery, completion tracking, and employee skills assessment.",
    status: "soon" as const,
    meta: "Coming 2027",
  },
];

const STATS = [
  { n: "73+", label: "API endpoints" },
  { n: "6", label: "Platform modules" },
  { n: "5", label: "Permission levels" },
  { n: "17", label: "Schema migrations" },
];

const FEATURES = [
  {
    Icon: Shield,
    title: "Security without shortcuts",
    desc: "JWT access tokens, httpOnly refresh cookies, bcrypt password hashing, audit logging, and session management — engineered into every layer, not retrofitted after the fact.",
  },
  {
    Icon: GitBranch,
    title: "True multi-tenancy",
    desc: "Organization scope is enforced at the database level. Cross-tenant data access is architecturally impossible — not just prevented by convention.",
  },
  {
    Icon: Activity,
    title: "Unified activity timeline",
    desc: "Every email, note, task, and deal surfaces in a single contact timeline — consistent and shared across CRM, HRM, and every module you deploy.",
  },
];

const SECURITY_ITEMS = [
  {
    Icon: Lock,
    title: "JWT + Refresh Rotation",
    desc: "Stateless access tokens paired with httpOnly secure refresh cookie rotation on every session renewal.",
  },
  {
    Icon: Shield,
    title: "Granular RBAC",
    desc: "Five role tiers — Owner, Admin, Manager, Member, Viewer — with module-level permission enforcement on every request.",
  },
  {
    Icon: FileText,
    title: "Immutable Audit Logs",
    desc: "Every create, update, delete, and administrative event is logged with actor identity, timestamp, and state diff.",
  },
  {
    Icon: Server,
    title: "Tenant Isolation",
    desc: "Multi-tenancy enforced at the query level. No shared mutable state. No cross-org data exposure — by design.",
  },
  {
    Icon: CheckCircle,
    title: "Cryptographic Passwords",
    desc: "Industry-standard bcrypt hashing with configurable cost factor. Breach-detection ready architecture from day one.",
  },
  {
    Icon: Building2,
    title: "SOC 2 Aligned",
    desc: "Infrastructure, logging, and access control policies designed to satisfy SOC 2 Type II audit requirements.",
  },
];

const TRUST_ORGS = [
  "Meridian Group",
  "Novex Systems",
  "Vantage Digital",
  "Axiom Corp",
  "Elevate Labs",
  "Strata Partners",
];

const FOOTER_COLS = [
  {
    heading: "Product",
    links: [
      "CRM",
      "HRM",
      "Accounting",
      "Projects",
      "API Reference",
      "Changelog",
    ],
  },
  {
    heading: "Solutions",
    links: ["For Agencies", "For Startups", "For Enterprise", "For SMBs"],
  },
  {
    heading: "Company",
    links: ["About", "Careers", "Blog", "Press Kit"],
  },
  {
    heading: "Legal",
    links: ["Privacy Policy", "Terms of Service", "Security", "Cookie Policy"],
  },
];

const MARQUEE_ITEMS = [
  "CRM",
  "HRM",
  "Accounting",
  "Projects",
  "E-commerce",
  "Learning",
];

const ROLES = ["Owner", "Admin", "Manager", "Member", "Viewer"] as const;
const MATRIX_COLS = ["CRM", "Tasks", "HRM", "Reports"] as const;
const PERMISSION_MATRIX: number[][] = [
  [2, 2, 2, 2],
  [2, 2, 2, 2],
  [2, 2, 1, 1],
  [2, 2, 0, 0],
  [1, 1, 0, 0],
];

// ─── Permission Card (hero visual) ───────────────────────────────────

function PermissionCard() {
  const dot = (level: number) =>
    ({
      width: 10,
      height: 10,
      borderRadius: "50%",
      flexShrink: 0,
      backgroundColor:
        level === 2
          ? "#7c3aed"
          : level === 1
            ? "rgba(124,58,237,0.3)"
            : "rgba(255,255,255,0.07)",
      boxShadow: level === 2 ? "0 0 7px rgba(124,58,237,0.8)" : "none",
    }) as React.CSSProperties;

  return (
    <div style={{ position: "relative", width: 356 }}>
      {/* Ambient glow */}
      <div
        aria-hidden
        style={{
          position: "absolute",
          inset: -24,
          background:
            "radial-gradient(ellipse at 50% 40%, rgba(124,58,237,0.22) 0%, transparent 68%)",
          filter: "blur(32px)",
          pointerEvents: "none",
        }}
      />

      {/* Main card */}
      <div
        style={{
          position: "relative",
          borderRadius: 18,
          border: "1px solid rgba(255,255,255,0.09)",
          background: "#111111",
          padding: "24px 24px 20px",
          boxShadow:
            "0 40px 80px rgba(0,0,0,0.65), 0 0 0 1px rgba(124,58,237,0.08)",
          transform: "rotate(1.5deg)",
        }}
      >
        {/* Header row */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            marginBottom: 22,
          }}
        >
          <Shield
            style={{ width: 13, height: 13, color: "#a855f7", flexShrink: 0 }}
            strokeWidth={2.5}
          />
          <span
            style={{
              fontSize: 12,
              fontWeight: 500,
              color: "rgba(255,255,255,0.52)",
            }}
          >
            Role-based access control
          </span>
          <div
            style={{
              marginLeft: "auto",
              display: "flex",
              alignItems: "center",
              gap: 5,
              borderRadius: 100,
              background: "rgba(124,58,237,0.13)",
              border: "1px solid rgba(124,58,237,0.24)",
              padding: "3px 10px",
            }}
          >
            <span
              className="rbac-pulse"
              style={{
                width: 5,
                height: 5,
                borderRadius: "50%",
                background: "#a855f7",
                display: "inline-block",
              }}
            />
            <span
              style={{
                fontSize: 9,
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.1em",
                color: "#a855f7",
              }}
            >
              Live
            </span>
          </div>
        </div>

        {/* Matrix grid */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "76px repeat(4, 1fr)",
            rowGap: 10,
            columnGap: 4,
            alignItems: "center",
          }}
        >
          <div />
          {MATRIX_COLS.map((col) => (
            <div
              key={col}
              style={{
                fontSize: 9,
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.12em",
                color: "rgba(255,255,255,0.22)",
                textAlign: "center",
              }}
            >
              {col}
            </div>
          ))}
          {ROLES.map((role, ri) => (
            <React.Fragment key={role}>
              <div
                style={{
                  fontSize: 11,
                  fontWeight: 500,
                  color: "rgba(255,255,255,0.44)",
                  paddingRight: 6,
                }}
              >
                {role}
              </div>
              {PERMISSION_MATRIX[ri].map((level, ci) => (
                <div
                  key={ci}
                  style={{ display: "flex", justifyContent: "center" }}
                >
                  <div style={dot(level)} />
                </div>
              ))}
            </React.Fragment>
          ))}
        </div>

        {/* Legend */}
        <div
          style={{
            display: "flex",
            gap: 16,
            marginTop: 18,
            paddingTop: 14,
            borderTop: "1px solid rgba(255,255,255,0.05)",
          }}
        >
          {[
            { bg: "#7c3aed", label: "Full" },
            { bg: "rgba(124,58,237,0.3)", label: "Limited" },
            { bg: "rgba(255,255,255,0.07)", label: "None" },
          ].map(({ bg, label }) => (
            <div
              key={label}
              style={{ display: "flex", alignItems: "center", gap: 5 }}
            >
              <div
                style={{
                  width: 7,
                  height: 7,
                  borderRadius: "50%",
                  background: bg,
                  flexShrink: 0,
                }}
              />
              <span style={{ fontSize: 9, color: "rgba(255,255,255,0.22)" }}>
                {label}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Floating stat chip */}
      <div
        style={{
          position: "absolute",
          bottom: -14,
          left: -22,
          borderRadius: 12,
          border: "1px solid rgba(255,255,255,0.07)",
          background: "#1a1a1a",
          padding: "12px 18px",
          boxShadow: "0 20px 40px rgba(0,0,0,0.55)",
          transform: "rotate(-1.8deg)",
          zIndex: 10,
        }}
      >
        <div
          style={{
            fontSize: 9,
            color: "rgba(255,255,255,0.24)",
            marginBottom: 2,
          }}
        >
          Permissions active
        </div>
        <div
          style={{
            fontSize: 30,
            fontWeight: 700,
            color: "white",
            lineHeight: 1,
          }}
        >
          47+
        </div>
        <div
          style={{ fontSize: 9, color: "rgba(255,255,255,0.2)", marginTop: 2 }}
        >
          across 5 roles
        </div>
      </div>
    </div>
  );
}

// ─── Main page ────────────────────────────────────────────────────────

export default function HomePage() {
  const badgeRef = useRef<HTMLDivElement>(null);
  const headlineRef = useRef<HTMLHeadingElement>(null);
  const subRef = useRef<HTMLParagraphElement>(null);
  const ctaRef = useRef<HTMLDivElement>(null);
  const visualRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let revert: (() => void) | undefined;

    (async () => {
      const gsap = (await import("gsap")).default;
      const { ScrollTrigger } = await import("gsap/ScrollTrigger");
      gsap.registerPlugin(ScrollTrigger);

      const ctx = gsap.context(() => {
        const ease = "power2.out";
        // play on enter · reverse when scrolled back above trigger → re-plays on next scroll down
        const toggleActions = "play none none reverse";

        // ── Hero entrance (load, no scroll) ──────────────────────────
        gsap
          .timeline({ defaults: { ease: "power3.out" } })
          .fromTo(
            badgeRef.current,
            { opacity: 0, y: 14 },
            { opacity: 1, y: 0, duration: 0.5 },
          )
          .fromTo(
            headlineRef.current,
            { opacity: 0, y: 40 },
            { opacity: 1, y: 0, duration: 0.8 },
            "-=0.2",
          )
          .fromTo(
            subRef.current,
            { opacity: 0, y: 22 },
            { opacity: 1, y: 0, duration: 0.55 },
            "-=0.45",
          )
          .fromTo(
            ctaRef.current,
            { opacity: 0, y: 16 },
            { opacity: 1, y: 0, duration: 0.5 },
            "-=0.3",
          )
          .fromTo(
            visualRef.current,
            { opacity: 0, x: 36, scale: 0.96 },
            { opacity: 1, x: 0, scale: 1, duration: 0.9, ease: "power2.out" },
            "-=0.75",
          );

        // ── Section headers (data-reveal) ────────────────────────────
        // Simple fade-up for each section label+heading block
        document
          .querySelectorAll<HTMLElement>("[data-reveal]")
          .forEach((el) => {
            gsap.fromTo(
              el,
              { opacity: 0, y: 28 },
              {
                opacity: 1,
                y: 0,
                duration: 0.65,
                ease,
                scrollTrigger: { trigger: el, start: "top 88%", toggleActions },
              },
            );
          });

        // ── Trust strip — items stagger in ───────────────────────────
        gsap.fromTo(
          ".trust-items > *",
          { opacity: 0, y: 10 },
          {
            opacity: 1,
            y: 0,
            duration: 0.4,
            ease,
            stagger: 0.06,
            scrollTrigger: {
              trigger: ".trust-items",
              start: "top 92%",
              toggleActions,
            },
          },
        );

        // ── Module cards — grid stagger ───────────────────────────────
        gsap.fromTo(
          ".module-card",
          { opacity: 0, y: 40 },
          {
            opacity: 1,
            y: 0,
            duration: 0.55,
            ease,
            stagger: 0.07,
            scrollTrigger: {
              trigger: ".module-grid",
              start: "top 82%",
              toggleActions,
            },
          },
        );

        // ── Stats — staggered left to right ──────────────────────────
        gsap.fromTo(
          ".stats-grid > div",
          { opacity: 0, y: 22 },
          {
            opacity: 1,
            y: 0,
            duration: 0.5,
            ease,
            stagger: 0.1,
            scrollTrigger: {
              trigger: ".stats-grid",
              start: "top 84%",
              toggleActions,
            },
          },
        );

        // ── Security cards — staggered ───────────────────────────────
        gsap.fromTo(
          ".security-grid > .feature-card",
          { opacity: 0, y: 32 },
          {
            opacity: 1,
            y: 0,
            duration: 0.55,
            ease,
            stagger: 0.08,
            scrollTrigger: {
              trigger: ".security-grid",
              start: "top 82%",
              toggleActions,
            },
          },
        );

        // ── Feature cards — staggered ────────────────────────────────
        gsap.fromTo(
          ".feature-grid > .feature-card",
          { opacity: 0, y: 32 },
          {
            opacity: 1,
            y: 0,
            duration: 0.6,
            ease,
            stagger: 0.12,
            scrollTrigger: {
              trigger: ".feature-grid",
              start: "top 82%",
              toggleActions,
            },
          },
        );

        // ── CTA block — scale + fade ──────────────────────────────────
        const ctaBlock = document.querySelector<HTMLElement>(".cta-block");
        if (ctaBlock) {
          gsap.fromTo(
            ctaBlock,
            { opacity: 0, y: 32, scale: 0.98 },
            {
              opacity: 1,
              y: 0,
              scale: 1,
              duration: 0.75,
              ease,
              scrollTrigger: {
                trigger: ctaBlock,
                start: "top 86%",
                toggleActions,
              },
            },
          );
        }

        // ── Footer columns — staggered ───────────────────────────────
        gsap.fromTo(
          ".footer-grid > *",
          { opacity: 0, y: 20 },
          {
            opacity: 1,
            y: 0,
            duration: 0.5,
            ease,
            stagger: 0.08,
            scrollTrigger: {
              trigger: ".footer-grid",
              start: "top 92%",
              toggleActions,
            },
          },
        );
      });

      revert = () => ctx.revert();
    })();

    return () => revert?.();
  }, []);

  return (
    <div
      className={`${inter.variable}`}
      style={{
        minHeight: "100vh",
        background: "#0a0a0a",
        color: "white",
        overflowX: "hidden",
        fontFamily: "var(--font-inter, 'Inter', sans-serif)",
      }}
    >
      {/* ─── Embedded styles ─── */}
      <style>{`
        @keyframes marquee {
          from { transform: translateX(0); }
          to   { transform: translateX(-50%); }
        }
        @keyframes soft-pulse {
          0%, 100% { opacity: 1; }
          50%       { opacity: 0.35; }
        }
        .marquee-track { animation: marquee 32s linear infinite; }
        .marquee-track:hover { animation-play-state: paused; }
        .rbac-pulse { animation: soft-pulse 2s ease-in-out infinite; }

        /* Hero layout */
        .hero-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 56px;
          align-items: center;
        }
        .hero-visual { display: none; }
        @media (min-width: 1024px) {
          .hero-grid { grid-template-columns: 1fr 356px; gap: 64px; }
          .hero-visual { display: block; }
        }

        /* Module grid */
        .module-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 1px;
          background: rgba(255,255,255,0.04);
          border: 1px solid rgba(255,255,255,0.06);
          border-radius: 16px;
          overflow: hidden;
        }
        @media (min-width: 640px) {
          .module-grid { grid-template-columns: repeat(2, 1fr); }
        }
        @media (min-width: 1024px) {
          .module-grid { grid-template-columns: repeat(3, 1fr); }
        }

        /* Stats grid */
        .stats-grid {
          display: grid;
          grid-template-columns: repeat(2, 1fr);
          gap: 1px;
          background: rgba(255,255,255,0.04);
          border-left: 1px solid rgba(255,255,255,0.05);
          border-right: 1px solid rgba(255,255,255,0.05);
        }
        @media (min-width: 768px) {
          .stats-grid { grid-template-columns: repeat(4, 1fr); }
        }

        /* Feature grid */
        .feature-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 16px;
        }
        @media (min-width: 768px) {
          .feature-grid { grid-template-columns: repeat(3, 1fr); }
        }

        /* Security grid */
        .security-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 16px;
        }
        @media (min-width: 640px) {
          .security-grid { grid-template-columns: repeat(2, 1fr); }
        }
        @media (min-width: 1024px) {
          .security-grid { grid-template-columns: repeat(3, 1fr); }
        }

        /* Footer grid */
        .footer-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 40px;
        }
        @media (min-width: 768px) {
          .footer-grid { grid-template-columns: 1.6fr repeat(4, 1fr); gap: 40px; }
        }

        /* Trust items */
        .trust-items {
          display: flex;
          flex-wrap: wrap;
          align-items: center;
          justify-content: center;
          gap: 8px 32px;
        }

        /* Nav center links */
        .nav-center { display: none; }
        @media (min-width: 900px) {
          .nav-center { display: flex; align-items: center; gap: 2px; }
        }

        /* Hover helpers */
        .module-card { transition: background 0.2s; }
        .module-card:hover { background: #131313 !important; }
        .feature-card { transition: background 0.2s, border-color 0.2s; }
        .feature-card:hover {
          background: #131313 !important;
          border-color: rgba(255,255,255,0.1) !important;
        }
        .nav-link { transition: color 0.15s, background 0.15s; }
        .nav-link:hover {
          color: rgba(255,255,255,0.82) !important;
          background: rgba(255,255,255,0.04) !important;
        }
        .footer-link { transition: color 0.15s; }
        .footer-link:hover { color: rgba(255,255,255,0.68) !important; }
        .cta-primary:hover { background: #6d28d9 !important; }
        .cta-secondary:hover {
          border-color: rgba(255,255,255,0.14) !important;
          color: rgba(255,255,255,0.72) !important;
        }
        .nav-cta:hover { background: #6d28d9 !important; }
        .talk-sales:hover { color: rgba(255,255,255,0.65) !important; }
      `}</style>

      {/* ══════════════════════════════════════════════
          NAVIGATION
      ══════════════════════════════════════════════ */}
      <header
        style={{
          position: "sticky",
          top: 0,
          zIndex: 50,
          borderBottom: "1px solid rgba(255,255,255,0.06)",
          background: "rgba(10,10,10,0.88)",
          backdropFilter: "blur(20px)",
          WebkitBackdropFilter: "blur(20px)",
        }}
      >
        <nav
          style={{
            maxWidth: 1200,
            margin: "0 auto",
            padding: "13px 24px",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 16,
          }}
        >
          {/* Logo */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 10,
              flexShrink: 0,
            }}
          >
            <div
              style={{
                width: 28,
                height: 28,
                borderRadius: 8,
                background: "#7c3aed",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                boxShadow: "0 0 14px rgba(124,58,237,0.5)",
                flexShrink: 0,
              }}
            >
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 800,
                  color: "white",
                }}
              >
                B
              </span>
            </div>
            <span
              style={{
                fontSize: 15,
                fontWeight: 600,
                letterSpacing: "-0.2px",
              }}
            >
              BusinessSAAS
            </span>
          </div>

          {/* Center links */}
          <div className="nav-center">
            {[
              { label: "Product", href: "#modules" },
              { label: "Solutions", href: "#features" },
              { label: "Enterprise", href: "#security" },
              { label: "Pricing", href: "#pricing" },
            ].map(({ label, href }) => (
              <a
                key={label}
                href={href}
                className="nav-link"
                style={{
                  fontSize: 13,
                  fontWeight: 500,
                  color: "rgba(255,255,255,0.38)",
                  padding: "6px 12px",
                  borderRadius: 6,
                  textDecoration: "none",
                  background: "transparent",
                }}
              >
                {label}
              </a>
            ))}
          </div>

          {/* Right actions */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              flexShrink: 0,
            }}
          >
            <Link
              href="/login"
              className="nav-link"
              style={{
                fontSize: 13,
                color: "rgba(255,255,255,0.38)",
                padding: "6px 12px",
                borderRadius: 6,
                textDecoration: "none",
                background: "transparent",
              }}
            >
              Sign in
            </Link>
            <Link
              href="/signup"
              className="nav-cta"
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                background: "#7c3aed",
                color: "white",
                padding: "8px 16px",
                borderRadius: 8,
                fontSize: 13,
                fontWeight: 600,
                textDecoration: "none",
                boxShadow: "0 0 20px rgba(124,58,237,0.35)",
                transition: "background 0.15s",
              }}
            >
              Book a Demo
              <ArrowRight style={{ width: 13, height: 13 }} />
            </Link>
          </div>
        </nav>
      </header>

      {/* ══════════════════════════════════════════════
          HERO
      ══════════════════════════════════════════════ */}
      <section
        style={{
          position: "relative",
          maxWidth: 1200,
          margin: "0 auto",
          padding: "104px 24px 96px",
        }}
      >
        {/* Dot-grid background texture */}
        <div
          aria-hidden
          style={{
            position: "absolute",
            inset: 0,
            backgroundImage:
              "radial-gradient(circle, rgba(255,255,255,0.022) 1px, transparent 1px)",
            backgroundSize: "32px 32px",
            pointerEvents: "none",
          }}
        />

        {/* Purple radial glow */}
        <div
          aria-hidden
          style={{
            position: "absolute",
            top: -40,
            left: "18%",
            width: 640,
            height: 360,
            background:
              "radial-gradient(ellipse at center, rgba(124,58,237,0.13) 0%, transparent 70%)",
            filter: "blur(48px)",
            pointerEvents: "none",
          }}
        />

        <div className="hero-grid" style={{ position: "relative" }}>
          {/* ── Left: content ── */}
          <div style={{ maxWidth: 560 }}>
            {/* Badge */}
            <div
              ref={badgeRef}
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 8,
                borderRadius: 100,
                border: "1px solid rgba(124,58,237,0.28)",
                background: "rgba(124,58,237,0.09)",
                padding: "6px 14px",
                marginBottom: 28,
              }}
            >
              <span
                className="rbac-pulse"
                style={{
                  width: 6,
                  height: 6,
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
                  letterSpacing: "0.03em",
                }}
              >
                SOC 2 aligned · CRM module now in production
              </span>
            </div>

            {/* Headline */}
            <h1
              ref={headlineRef}
              style={{
                fontSize: "clamp(38px, 4vw, 54px)",
                fontWeight: 700,
                lineHeight: 1.1,
                letterSpacing: "-0.5px",
                color: "white",
                marginBottom: 24,
              }}
            >
              Run your entire
              <br />
              business from
              <br />
              <span
                style={{
                  background:
                    "linear-gradient(135deg, #7c3aed 0%, #a855f7 55%, #c084fc 100%)",
                  WebkitBackgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                  backgroundClip: "text",
                }}
              >
                one platform.
              </span>
            </h1>

            {/* Sub */}
            <p
              ref={subRef}
              style={{
                fontSize: 17,
                lineHeight: 1.72,
                color: "rgba(255,255,255,0.42)",
                maxWidth: 440,
                marginBottom: 36,
              }}
            >
              Consolidate CRM, HRM, Accounting, and Projects into a single
              secure workspace — with enterprise-grade access control and
              audit-ready infrastructure built in from day one.
            </p>

            {/* CTAs */}
            <div
              ref={ctaRef}
              style={{
                display: "flex",
                flexWrap: "wrap",
                alignItems: "center",
                gap: 12,
              }}
            >
              <Link
                href="/signup"
                className="cta-primary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 9,
                  background: "#7c3aed",
                  color: "white",
                  padding: "14px 26px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 600,
                  textDecoration: "none",
                  boxShadow: "0 0 28px rgba(124,58,237,0.42)",
                  transition: "background 0.15s",
                }}
              >
                Book a Demo
                <ArrowRight style={{ width: 15, height: 15 }} />
              </Link>
              <Link
                href="/signup"
                className="cta-secondary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  padding: "14px 26px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 500,
                  textDecoration: "none",
                  border: "1px solid rgba(255,255,255,0.09)",
                  color: "rgba(255,255,255,0.52)",
                  transition: "border-color 0.15s, color 0.15s",
                }}
              >
                Start Free Trial
              </Link>
            </div>

            <div
              style={{
                marginTop: 18,
                display: "flex",
                flexWrap: "wrap",
                alignItems: "center",
                gap: 16,
              }}
            >
              <p
                style={{
                  fontSize: 11,
                  color: "rgba(255,255,255,0.18)",
                  letterSpacing: "0.02em",
                }}
              >
                No credit card required · Enterprise plans available
              </p>
              <a
                href="#pricing"
                className="talk-sales"
                style={{
                  fontSize: 11,
                  color: "rgba(255,255,255,0.28)",
                  textDecoration: "none",
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 4,
                  transition: "color 0.15s",
                }}
              >
                Talk to Sales
                <ArrowRight style={{ width: 10, height: 10 }} />
              </a>
            </div>
          </div>

          {/* ── Right: RBAC visual ── */}
          <div className="hero-visual" ref={visualRef}>
            <PermissionCard />
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════════════════
          TRUST STRIP
      ══════════════════════════════════════════════ */}
      <div
        style={{
          borderTop: "1px solid rgba(255,255,255,0.04)",
          borderBottom: "1px solid rgba(255,255,255,0.04)",
          padding: "36px 24px",
        }}
      >
        <div style={{ maxWidth: 1200, margin: "0 auto", textAlign: "center" }}>
          <p
            style={{
              fontSize: 9.5,
              fontWeight: 700,
              textTransform: "uppercase",
              letterSpacing: "0.12em",
              color: "rgba(255,255,255,0.14)",
              marginBottom: 20,
            }}
          >
            Trusted by forward-thinking teams
          </p>
          <div className="trust-items">
            {TRUST_ORGS.map((org, i) => (
              <React.Fragment key={org}>
                <span
                  style={{
                    fontSize: 13,
                    fontWeight: 600,
                    color: "rgba(255,255,255,0.18)",
                    letterSpacing: "0",
                    whiteSpace: "nowrap",
                  }}
                >
                  {org}
                </span>
                {i < TRUST_ORGS.length - 1 && (
                  <span
                    style={{
                      width: 3,
                      height: 3,
                      borderRadius: "50%",
                      background: "rgba(255,255,255,0.08)",
                      display: "inline-block",
                      flexShrink: 0,
                    }}
                  />
                )}
              </React.Fragment>
            ))}
          </div>
        </div>
      </div>

      {/* ══════════════════════════════════════════════
          MARQUEE
      ══════════════════════════════════════════════ */}
      <div
        style={{
          overflow: "hidden",
          borderBottom: "1px solid rgba(255,255,255,0.05)",
          padding: "16px 0",
        }}
      >
        <div
          className="marquee-track"
          style={{ display: "flex", width: "max-content" }}
        >
          {[0, 1].map((gi) =>
            MARQUEE_ITEMS.map((m) => (
              <span
                key={`${gi}-${m}`}
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  padding: "0 32px",
                  fontSize: 11,
                  fontWeight: 700,
                  textTransform: "uppercase",
                  letterSpacing: "0.1em",
                  color: "rgba(255,255,255,0.13)",
                  whiteSpace: "nowrap",
                }}
              >
                {m}
                <span
                  style={{
                    display: "inline-block",
                    marginLeft: 32,
                    width: 4,
                    height: 4,
                    borderRadius: "50%",
                    background: "rgba(255,255,255,0.09)",
                    flexShrink: 0,
                  }}
                />
              </span>
            )),
          )}
        </div>
      </div>

      {/* ══════════════════════════════════════════════
          MODULES
      ══════════════════════════════════════════════ */}
      <section
        id="modules"
        style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}
      >
        <div data-reveal style={{ marginBottom: 48 }}>
          <p
            style={{
              fontSize: 11,
              fontWeight: 700,
              textTransform: "uppercase",
              letterSpacing: "0.1em",
              color: "rgba(255,255,255,0.22)",
              marginBottom: 12,
            }}
          >
            Platform Suite
          </p>
          <h2
            style={{
              fontSize: "clamp(26px, 2.8vw, 36px)",
              fontWeight: 700,
              lineHeight: 1.2,
              letterSpacing: "-0.3px",
              color: "white",
            }}
          >
            Every business function, unified in one platform.
          </h2>
        </div>

        <div className="module-grid">
          {MODULES.map((mod) => (
            <div
              key={mod.name}
              className="module-card"
              style={{
                background: "#0e0e0e",
                padding: "28px 28px 24px",
                position: "relative",
              }}
            >
              {/* Status badge */}
              {mod.status === "live" ? (
                <div
                  style={{
                    position: "absolute",
                    top: 16,
                    right: 16,
                    display: "flex",
                    alignItems: "center",
                    gap: 5,
                    borderRadius: 100,
                    background: "rgba(124,58,237,0.13)",
                    border: "1px solid rgba(124,58,237,0.24)",
                    padding: "3px 10px",
                  }}
                >
                  <span
                    className="rbac-pulse"
                    style={{
                      width: 5,
                      height: 5,
                      borderRadius: "50%",
                      background: "#a855f7",
                      display: "inline-block",
                    }}
                  />
                  <span
                    style={{
                      fontSize: 8.5,
                      fontWeight: 700,
                      textTransform: "uppercase",
                      letterSpacing: "0.1em",
                      color: "#a855f7",
                    }}
                  >
                    Live
                  </span>
                </div>
              ) : (
                <div
                  style={{
                    position: "absolute",
                    top: 16,
                    right: 16,
                    borderRadius: 100,
                    border: "1px solid rgba(255,255,255,0.07)",
                    padding: "3px 10px",
                  }}
                >
                  <span
                    style={{
                      fontSize: 8.5,
                      fontWeight: 600,
                      textTransform: "uppercase",
                      letterSpacing: "0.1em",
                      color: "rgba(255,255,255,0.18)",
                    }}
                  >
                    Roadmap
                  </span>
                </div>
              )}

              <h3
                style={{
                  fontSize: 20,
                  fontWeight: 700,
                  letterSpacing: "0",
                  lineHeight: 1.1,
                  color:
                    mod.status === "live"
                      ? "rgba(255,255,255,0.9)"
                      : "rgba(255,255,255,0.42)",
                  marginBottom: 9,
                }}
              >
                {mod.name}
              </h3>

              <p
                style={{
                  fontSize: 13,
                  lineHeight: 1.65,
                  color: "rgba(255,255,255,0.3)",
                  marginBottom: 20,
                  minHeight: 52,
                }}
              >
                {mod.desc}
              </p>

              <div
                style={{
                  fontSize: 10.5,
                  fontWeight: 500,
                  color: "rgba(255,255,255,0.14)",
                  paddingTop: 14,
                  borderTop: "1px solid rgba(255,255,255,0.05)",
                  letterSpacing: "0.02em",
                }}
              >
                {mod.meta}
              </div>

              {mod.status === "live" && (
                <div
                  aria-hidden
                  style={{
                    position: "absolute",
                    bottom: 0,
                    left: 0,
                    right: 0,
                    height: 2,
                    background:
                      "linear-gradient(90deg, transparent 0%, #7c3aed 40%, #a855f7 60%, transparent 100%)",
                    opacity: 0.65,
                  }}
                />
              )}
            </div>
          ))}
        </div>
      </section>

      {/* ══════════════════════════════════════════════
          STATS STRIP
      ══════════════════════════════════════════════ */}
      <div
        style={{
          borderTop: "1px solid rgba(255,255,255,0.05)",
          borderBottom: "1px solid rgba(255,255,255,0.05)",
          background: "#0c0c0c",
        }}
      >
        <div style={{ maxWidth: 1200, margin: "0 auto", padding: "0 24px" }}>
          <div className="stats-grid">
            {STATS.map((stat) => (
              <div
                key={stat.n}
                style={{
                  background: "#0c0c0c",
                  padding: "44px 32px",
                  textAlign: "center",
                }}
              >
                <div
                  style={{
                    fontSize: "clamp(30px, 3vw, 42px)",
                    fontWeight: 700,
                    lineHeight: 1,
                    letterSpacing: "-0.5px",
                    color: "white",
                    marginBottom: 7,
                  }}
                >
                  {stat.n}
                </div>
                <div
                  style={{
                    fontSize: 10.5,
                    fontWeight: 600,
                    textTransform: "uppercase",
                    letterSpacing: "0.1em",
                    color: "rgba(255,255,255,0.22)",
                  }}
                >
                  {stat.label}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* ══════════════════════════════════════════════
          SECURITY
      ══════════════════════════════════════════════ */}
      <section
        id="security"
        style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}
      >
        <div data-reveal style={{ marginBottom: 56 }}>
          <p
            style={{
              fontSize: 11,
              fontWeight: 700,
              textTransform: "uppercase",
              letterSpacing: "0.1em",
              color: "rgba(255,255,255,0.22)",
              marginBottom: 12,
            }}
          >
            Security & Compliance
          </p>
          <h2
            style={{
              fontSize: "clamp(26px, 2.8vw, 36px)",
              fontWeight: 700,
              lineHeight: 1.2,
              letterSpacing: "-0.3px",
              color: "white",
            }}
          >
            Enterprise-grade security. Built in, not bolted on.
          </h2>
        </div>

        <div className="security-grid">
          {SECURITY_ITEMS.map(({ Icon, title, desc }) => (
            <div
              key={title}
              className="feature-card"
              style={{
                borderRadius: 14,
                border: "1px solid rgba(255,255,255,0.06)",
                background: "#0e0e0e",
                padding: "28px",
              }}
            >
              <div
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 10,
                  background: "rgba(124,58,237,0.1)",
                  border: "1px solid rgba(124,58,237,0.2)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  marginBottom: 20,
                  flexShrink: 0,
                }}
              >
                <Icon
                  style={{ width: 18, height: 18, color: "#7c3aed" }}
                  strokeWidth={1.75}
                />
              </div>
              <h3
                style={{
                  fontSize: 15,
                  fontWeight: 600,
                  letterSpacing: "0",
                  lineHeight: 1.4,
                  color: "white",
                  marginBottom: 10,
                }}
              >
                {title}
              </h3>
              <p
                style={{
                  fontSize: 13,
                  lineHeight: 1.72,
                  color: "rgba(255,255,255,0.34)",
                }}
              >
                {desc}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* ══════════════════════════════════════════════
          FEATURES
      ══════════════════════════════════════════════ */}
      <section
        id="features"
        style={{
          background: "#0c0c0c",
          borderTop: "1px solid rgba(255,255,255,0.04)",
          borderBottom: "1px solid rgba(255,255,255,0.04)",
        }}
      >
        <div style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}>
          <div data-reveal style={{ marginBottom: 56 }}>
            <p
              style={{
                fontSize: 11,
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.1em",
                color: "rgba(255,255,255,0.22)",
                marginBottom: 12,
              }}
            >
              Foundation
            </p>
            <h2
              style={{
                fontSize: "clamp(26px, 2.8vw, 36px)",
                fontWeight: 700,
                lineHeight: 1.2,
                letterSpacing: "-0.3px",
                color: "white",
              }}
            >
              Not just features. Architecture that holds.
            </h2>
          </div>

          <div className="feature-grid">
            {FEATURES.map(({ Icon, title, desc }) => (
              <div
                key={title}
                className="feature-card"
                style={{
                  borderRadius: 14,
                  border: "1px solid rgba(255,255,255,0.06)",
                  background: "#0e0e0e",
                  padding: "28px",
                }}
              >
                <div
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 10,
                    background: "rgba(124,58,237,0.1)",
                    border: "1px solid rgba(124,58,237,0.2)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    marginBottom: 20,
                    flexShrink: 0,
                  }}
                >
                  <Icon
                    style={{ width: 18, height: 18, color: "#7c3aed" }}
                    strokeWidth={1.75}
                  />
                </div>
                <h3
                  style={{
                    fontSize: 15,
                    fontWeight: 600,
                    letterSpacing: "0",
                    lineHeight: 1.4,
                    color: "white",
                    marginBottom: 10,
                  }}
                >
                  {title}
                </h3>
                <p
                  style={{
                    fontSize: 13,
                    lineHeight: 1.72,
                    color: "rgba(255,255,255,0.34)",
                  }}
                >
                  {desc}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════════════════
          CTA
      ══════════════════════════════════════════════ */}
      <section
        id="pricing"
        style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}
      >
        <div
          className="cta-block"
          style={{
            position: "relative",
            overflow: "hidden",
            borderRadius: 20,
            border: "1px solid rgba(124,58,237,0.18)",
            background:
              "linear-gradient(145deg, #0e0e0e 0%, #0f0f1b 50%, #0e0e0e 100%)",
            padding: "80px 40px",
            textAlign: "center",
          }}
        >
          {/* Glow */}
          <div
            aria-hidden
            style={{
              position: "absolute",
              inset: 0,
              background:
                "radial-gradient(ellipse at 50% 60%, rgba(124,58,237,0.15) 0%, transparent 65%)",
              filter: "blur(32px)",
              pointerEvents: "none",
            }}
          />

          {/* Top shimmer */}
          <div
            aria-hidden
            style={{
              position: "absolute",
              top: 0,
              left: "15%",
              right: "15%",
              height: 1,
              background:
                "linear-gradient(90deg, transparent 0%, rgba(124,58,237,0.7) 50%, transparent 100%)",
            }}
          />

          <div style={{ position: "relative" }}>
            <p
              style={{
                fontSize: 11,
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.1em",
                color: "rgba(168,85,247,0.55)",
                marginBottom: 16,
              }}
            >
              Schedule a demo
            </p>

            <h2
              style={{
                fontSize: "clamp(30px, 3.2vw, 44px)",
                fontWeight: 700,
                lineHeight: 1.15,
                letterSpacing: "-0.5px",
                color: "white",
                marginBottom: 20,
              }}
            >
              One platform.
              <br />
              Every business function.
            </h2>

            <p
              style={{
                fontSize: 15,
                lineHeight: 1.65,
                color: "rgba(255,255,255,0.36)",
                maxWidth: 420,
                margin: "0 auto 36px",
              }}
            >
              See how BusinessSAAS consolidates your operations, secures your
              data, and scales with your organization — in a personalized
              walkthrough.
            </p>

            <div
              style={{
                display: "flex",
                flexWrap: "wrap",
                alignItems: "center",
                justifyContent: "center",
                gap: 12,
                marginBottom: 20,
              }}
            >
              <Link
                href="/signup"
                className="cta-primary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 9,
                  background: "#7c3aed",
                  color: "white",
                  padding: "14px 28px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 600,
                  textDecoration: "none",
                  boxShadow: "0 0 32px rgba(124,58,237,0.45)",
                  transition: "background 0.15s",
                }}
              >
                Book a Demo
                <ArrowRight style={{ width: 15, height: 15 }} />
              </Link>
              <Link
                href="/signup"
                className="cta-secondary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  padding: "14px 28px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 500,
                  textDecoration: "none",
                  border: "1px solid rgba(255,255,255,0.09)",
                  color: "rgba(255,255,255,0.48)",
                  transition: "border-color 0.15s, color 0.15s",
                }}
              >
                Start Free Trial
              </Link>
            </div>

            <p
              style={{
                fontSize: 11,
                color: "rgba(255,255,255,0.16)",
                letterSpacing: "0.02em",
              }}
            >
              No credit card required · Enterprise contracts available · Custom
              pricing for 50+ seats
            </p>
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════════════════
          FOOTER
      ══════════════════════════════════════════════ */}
      <footer
        style={{
          borderTop: "1px solid rgba(255,255,255,0.05)",
          background: "#0a0a0a",
        }}
      >
        <div
          style={{
            maxWidth: 1200,
            margin: "0 auto",
            padding: "64px 24px 40px",
          }}
        >
          {/* Main grid */}
          <div className="footer-grid">
            {/* Company column */}
            <div>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  marginBottom: 16,
                }}
              >
                <div
                  style={{
                    width: 24,
                    height: 24,
                    borderRadius: 7,
                    background: "rgba(124,58,237,0.75)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    flexShrink: 0,
                  }}
                >
                  <span
                    style={{
                      fontSize: 9,
                      fontWeight: 800,
                      color: "white",
                    }}
                  >
                    B
                  </span>
                </div>
                <span
                  style={{
                    fontSize: 14,
                    fontWeight: 600,
                    letterSpacing: "-0.2px",
                    color: "rgba(255,255,255,0.82)",
                  }}
                >
                  BusinessSAAS
                </span>
              </div>
              <p
                style={{
                  fontSize: 12.5,
                  lineHeight: 1.72,
                  color: "rgba(255,255,255,0.26)",
                  maxWidth: 220,
                  marginBottom: 24,
                }}
              >
                A unified business operating platform built for organizations
                that demand security, auditability, and scale.
              </p>
              {/* SOC 2 badge */}
              <div
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 6,
                  border: "1px solid rgba(255,255,255,0.07)",
                  borderRadius: 8,
                  padding: "8px 12px",
                }}
              >
                <Shield
                  style={{
                    width: 11,
                    height: 11,
                    color: "rgba(255,255,255,0.28)",
                    flexShrink: 0,
                  }}
                  strokeWidth={2}
                />
                <span
                  style={{
                    fontSize: 9.5,
                    fontWeight: 700,
                    textTransform: "uppercase",
                    letterSpacing: "0.1em",
                    color: "rgba(255,255,255,0.24)",
                  }}
                >
                  SOC 2 Aligned
                </span>
              </div>
            </div>

            {/* Link columns */}
            {FOOTER_COLS.map((col) => (
              <div key={col.heading}>
                <p
                  style={{
                    fontSize: 10.5,
                    fontWeight: 700,
                    textTransform: "uppercase",
                    letterSpacing: "0.08em",
                    color: "rgba(255,255,255,0.2)",
                    marginBottom: 18,
                  }}
                >
                  {col.heading}
                </p>
                <ul
                  style={{
                    listStyle: "none",
                    padding: 0,
                    margin: 0,
                    display: "flex",
                    flexDirection: "column",
                    gap: 11,
                  }}
                >
                  {col.links.map((link) => (
                    <li key={link}>
                      <a
                        href="#"
                        className="footer-link"
                        style={{
                          fontSize: 13,
                          color: "rgba(255,255,255,0.34)",
                          textDecoration: "none",
                        }}
                      >
                        {link}
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>

          {/* Bottom bar */}
          <div
            style={{
              marginTop: 52,
              paddingTop: 24,
              borderTop: "1px solid rgba(255,255,255,0.05)",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              flexWrap: "wrap",
              gap: 12,
            }}
          >
            <span style={{ fontSize: 12, color: "rgba(255,255,255,0.18)" }}>
              © {new Date().getFullYear()} BusinessSAAS. All rights reserved.
            </span>
            <span style={{ fontSize: 12, color: "rgba(255,255,255,0.1)" }}>
              Built with Go + Next.js
            </span>
          </div>
        </div>
      </footer>
    </div>
  );
}
