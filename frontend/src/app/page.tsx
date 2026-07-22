// "use client";

// // ═══════════════════════════════════════════════════════════════════
// // app/page.tsx
// // BusinessSAAS — Public marketing homepage
// // Dark-first · Purple accent · Inter headings · GSAP entrance
// // ═══════════════════════════════════════════════════════════════════

// import React, { useEffect, useLayoutEffect, useRef } from "react";
// import Link from "next/link";
// import gsap from "gsap";
// import { ScrollTrigger } from "gsap/ScrollTrigger";

// gsap.registerPlugin(ScrollTrigger);

// const useIsomorphicLayoutEffect =
//   typeof window !== "undefined" ? useLayoutEffect : useEffect;
// import {
//   ArrowRight,
//   Shield,
//   GitBranch,
//   Activity,
//   Lock,
//   Server,
//   FileText,
//   CheckCircle,
//   Building2,
// } from "lucide-react";
// import { Inter } from "next/font/google";

// const inter = Inter({
//   subsets: ["latin"],
//   variable: "--font-inter",
//   weight: ["400", "500", "600", "700"],
//   display: "swap",
// });

// // ─── Data ────────────────────────────────────────────────────────────

// const MODULES = [
//   {
//     name: "CRM",
//     desc: "Full-cycle relationship management — leads, deals, contact timelines, and pipeline analytics across your entire organization.",
//     status: "live" as const,
//     meta: "73 API endpoints · Production-ready",
//   },
//   {
//     name: "HRM",
//     desc: "Departments, headcount, leave management, payroll processing, and organizational hierarchy in a single system of record.",
//     status: "soon" as const,
//     meta: "131 endpoints planned",
//   },
//   {
//     name: "Accounting",
//     desc: "Invoices, expense tracking, bank reconciliation, and executive-level financial reporting.",
//     status: "soon" as const,
//     meta: "Coming Q3 2026",
//   },
//   {
//     name: "Projects",
//     desc: "Task tracking, milestone management, resource allocation, and cross-team workload visibility.",
//     status: "soon" as const,
//     meta: "Coming Q4 2026",
//   },
//   {
//     name: "E-commerce",
//     desc: "Product catalog, order management, customer segmentation, and real-time inventory control.",
//     status: "soon" as const,
//     meta: "Coming 2027",
//   },
//   {
//     name: "Learning",
//     desc: "Internal training programs, course delivery, completion tracking, and employee skills assessment.",
//     status: "soon" as const,
//     meta: "Coming 2027",
//   },
// ];

// const STATS = [
//   { n: "73+", label: "API endpoints" },
//   { n: "6", label: "Platform modules" },
//   { n: "5", label: "Permission levels" },
//   { n: "17", label: "Schema migrations" },
// ];

// const FEATURES = [
//   {
//     Icon: Shield,
//     title: "Security without shortcuts",
//     desc: "JWT access tokens, httpOnly refresh cookies, bcrypt password hashing, audit logging, and session management — engineered into every layer, not retrofitted after the fact.",
//   },
//   {
//     Icon: GitBranch,
//     title: "True multi-tenancy",
//     desc: "Organization scope is enforced at the database level. Cross-tenant data access is architecturally impossible — not just prevented by convention.",
//   },
//   {
//     Icon: Activity,
//     title: "Unified activity timeline",
//     desc: "Every email, note, task, and deal surfaces in a single contact timeline — consistent and shared across CRM, HRM, and every module you deploy.",
//   },
// ];

// const SECURITY_ITEMS = [
//   {
//     Icon: Lock,
//     title: "JWT + Refresh Rotation",
//     desc: "Stateless access tokens paired with httpOnly secure refresh cookie rotation on every session renewal.",
//   },
//   {
//     Icon: Shield,
//     title: "Granular RBAC",
//     desc: "Five role tiers — Owner, Admin, Manager, Member, Viewer — with module-level permission enforcement on every request.",
//   },
//   {
//     Icon: FileText,
//     title: "Immutable Audit Logs",
//     desc: "Every create, update, delete, and administrative event is logged with actor identity, timestamp, and state diff.",
//   },
//   {
//     Icon: Server,
//     title: "Tenant Isolation",
//     desc: "Multi-tenancy enforced at the query level. No shared mutable state. No cross-org data exposure — by design.",
//   },
//   {
//     Icon: CheckCircle,
//     title: "Cryptographic Passwords",
//     desc: "Industry-standard bcrypt hashing with configurable cost factor. Breach-detection ready architecture from day one.",
//   },
//   {
//     Icon: Building2,
//     title: "SOC 2 Aligned",
//     desc: "Infrastructure, logging, and access control policies designed to satisfy SOC 2 Type II audit requirements.",
//   },
// ];

// const TRUST_ORGS = [
//   "Meridian Group",
//   "Novex Systems",
//   "Vantage Digital",
//   "Axiom Corp",
//   "Elevate Labs",
//   "Strata Partners",
// ];

// const FOOTER_COLS = [
//   {
//     heading: "Product",
//     links: [
//       "CRM",
//       "HRM",
//       "Accounting",
//       "Projects",
//       "API Reference",
//       "Changelog",
//     ],
//   },
//   {
//     heading: "Solutions",
//     links: ["For Agencies", "For Startups", "For Enterprise", "For SMBs"],
//   },
//   {
//     heading: "Company",
//     links: ["About", "Careers", "Blog", "Press Kit"],
//   },
//   {
//     heading: "Legal",
//     links: ["Privacy Policy", "Terms of Service", "Security", "Cookie Policy"],
//   },
// ];

// const MARQUEE_ITEMS = [
//   "CRM",
//   "HRM",
//   "Accounting",
//   "Projects",
//   "E-commerce",
//   "Learning",
// ];

// const ROLES = ["Owner", "Admin", "Manager", "Member", "Viewer"] as const;
// const MATRIX_COLS = ["CRM", "Tasks", "HRM", "Reports"] as const;
// const PERMISSION_MATRIX: number[][] = [
//   [2, 2, 2, 2],
//   [2, 2, 2, 2],
//   [2, 2, 1, 1],
//   [2, 2, 0, 0],
//   [1, 1, 0, 0],
// ];

// // ─── Permission Card (hero visual) ───────────────────────────────────

// function PermissionCard() {
//   const dot = (level: number) =>
//     ({
//       width: 10,
//       height: 10,
//       borderRadius: "50%",
//       flexShrink: 0,
//       backgroundColor:
//         level === 2
//           ? "#7c3aed"
//           : level === 1
//             ? "rgba(124,58,237,0.3)"
//             : "rgba(255,255,255,0.07)",
//       boxShadow: level === 2 ? "0 0 7px rgba(124,58,237,0.8)" : "none",
//     }) as React.CSSProperties;

//   return (
//     <div style={{ position: "relative", width: 356 }}>
//       {/* Ambient glow */}
//       <div
//         aria-hidden
//         style={{
//           position: "absolute",
//           inset: -24,
//           background:
//             "radial-gradient(ellipse at 50% 40%, rgba(124,58,237,0.22) 0%, transparent 68%)",
//           filter: "blur(32px)",
//           pointerEvents: "none",
//         }}
//       />

//       {/* Main card */}
//       <div
//         style={{
//           position: "relative",
//           borderRadius: 18,
//           border: "1px solid rgba(255,255,255,0.09)",
//           background: "#111111",
//           padding: "24px 24px 20px",
//           boxShadow:
//             "0 40px 80px rgba(0,0,0,0.65), 0 0 0 1px rgba(124,58,237,0.08)",
//           transform: "rotate(1.5deg)",
//         }}
//       >
//         {/* Header row */}
//         <div
//           style={{
//             display: "flex",
//             alignItems: "center",
//             gap: 8,
//             marginBottom: 22,
//           }}
//         >
//           <Shield
//             style={{ width: 13, height: 13, color: "#a855f7", flexShrink: 0 }}
//             strokeWidth={2.5}
//           />
//           <span
//             style={{
//               fontSize: 12,
//               fontWeight: 500,
//               color: "rgba(255,255,255,0.52)",
//             }}
//           >
//             Role-based access control
//           </span>
//           <div
//             style={{
//               marginLeft: "auto",
//               display: "flex",
//               alignItems: "center",
//               gap: 5,
//               borderRadius: 100,
//               background: "rgba(124,58,237,0.13)",
//               border: "1px solid rgba(124,58,237,0.24)",
//               padding: "3px 10px",
//             }}
//           >
//             <span
//               className="rbac-pulse"
//               style={{
//                 width: 5,
//                 height: 5,
//                 borderRadius: "50%",
//                 background: "#a855f7",
//                 display: "inline-block",
//               }}
//             />
//             <span
//               style={{
//                 fontSize: 9,
//                 fontWeight: 700,
//                 textTransform: "uppercase",
//                 letterSpacing: "0.1em",
//                 color: "#a855f7",
//               }}
//             >
//               Live
//             </span>
//           </div>
//         </div>

//         {/* Matrix grid */}
//         <div
//           style={{
//             display: "grid",
//             gridTemplateColumns: "76px repeat(4, 1fr)",
//             rowGap: 10,
//             columnGap: 4,
//             alignItems: "center",
//           }}
//         >
//           <div />
//           {MATRIX_COLS.map((col) => (
//             <div
//               key={col}
//               style={{
//                 fontSize: 9,
//                 fontWeight: 700,
//                 textTransform: "uppercase",
//                 letterSpacing: "0.12em",
//                 color: "rgba(255,255,255,0.22)",
//                 textAlign: "center",
//               }}
//             >
//               {col}
//             </div>
//           ))}
//           {ROLES.map((role, ri) => (
//             <React.Fragment key={role}>
//               <div
//                 style={{
//                   fontSize: 11,
//                   fontWeight: 500,
//                   color: "rgba(255,255,255,0.44)",
//                   paddingRight: 6,
//                 }}
//               >
//                 {role}
//               </div>
//               {PERMISSION_MATRIX[ri].map((level, ci) => (
//                 <div
//                   key={ci}
//                   style={{ display: "flex", justifyContent: "center" }}
//                 >
//                   <div style={dot(level)} />
//                 </div>
//               ))}
//             </React.Fragment>
//           ))}
//         </div>

//         {/* Legend */}
//         <div
//           style={{
//             display: "flex",
//             gap: 16,
//             marginTop: 18,
//             paddingTop: 14,
//             borderTop: "1px solid rgba(255,255,255,0.05)",
//           }}
//         >
//           {[
//             { bg: "#7c3aed", label: "Full" },
//             { bg: "rgba(124,58,237,0.3)", label: "Limited" },
//             { bg: "rgba(255,255,255,0.07)", label: "None" },
//           ].map(({ bg, label }) => (
//             <div
//               key={label}
//               style={{ display: "flex", alignItems: "center", gap: 5 }}
//             >
//               <div
//                 style={{
//                   width: 7,
//                   height: 7,
//                   borderRadius: "50%",
//                   background: bg,
//                   flexShrink: 0,
//                 }}
//               />
//               <span style={{ fontSize: 9, color: "rgba(255,255,255,0.22)" }}>
//                 {label}
//               </span>
//             </div>
//           ))}
//         </div>
//       </div>

//       {/* Floating stat chip */}
//       <div
//         style={{
//           position: "absolute",
//           bottom: -14,
//           left: -22,
//           borderRadius: 12,
//           border: "1px solid rgba(255,255,255,0.07)",
//           background: "#1a1a1a",
//           padding: "12px 18px",
//           boxShadow: "0 20px 40px rgba(0,0,0,0.55)",
//           transform: "rotate(-1.8deg)",
//           zIndex: 10,
//         }}
//       >
//         <div
//           style={{
//             fontSize: 9,
//             color: "rgba(255,255,255,0.24)",
//             marginBottom: 2,
//           }}
//         >
//           Permissions active
//         </div>
//         <div
//           style={{
//             fontSize: 30,
//             fontWeight: 700,
//             color: "white",
//             lineHeight: 1,
//           }}
//         >
//           47+
//         </div>
//         <div
//           style={{ fontSize: 9, color: "rgba(255,255,255,0.2)", marginTop: 2 }}
//         >
//           across 5 roles
//         </div>
//       </div>
//     </div>
//   );
// }

// // ─── Main page ────────────────────────────────────────────────────────

// export default function HomePage() {
//   const badgeRef = useRef<HTMLDivElement>(null);
//   const headlineRef = useRef<HTMLHeadingElement>(null);
//   const subRef = useRef<HTMLParagraphElement>(null);
//   const ctaRef = useRef<HTMLDivElement>(null);
//   const visualRef = useRef<HTMLDivElement>(null);

//   useIsomorphicLayoutEffect(() => {
//     let revert: (() => void) | undefined;

//     (() => {
//       const ctx = gsap.context(() => {
//         const ease = "power2.out";
//         // play on enter · reverse when scrolled back above trigger → re-plays on next scroll down
//         const toggleActions = "play none none reverse";

//         // ── Hero entrance (load, no scroll) ──────────────────────────
//         gsap
//           .timeline({ defaults: { ease: "power3.out" } })
//           .fromTo(
//             badgeRef.current,
//             { opacity: 0, y: 14 },
//             { opacity: 1, y: 0, duration: 0.5 },
//           )
//           .fromTo(
//             headlineRef.current,
//             { opacity: 0, y: 40 },
//             { opacity: 1, y: 0, duration: 0.8 },
//             "-=0.2",
//           )
//           .fromTo(
//             subRef.current,
//             { opacity: 0, y: 22 },
//             { opacity: 1, y: 0, duration: 0.55 },
//             "-=0.45",
//           )
//           .fromTo(
//             ctaRef.current,
//             { opacity: 0, y: 16 },
//             { opacity: 1, y: 0, duration: 0.5 },
//             "-=0.3",
//           )
//           .fromTo(
//             visualRef.current,
//             { opacity: 0, x: 36, scale: 0.96 },
//             { opacity: 1, x: 0, scale: 1, duration: 0.9, ease: "power2.out" },
//             "-=0.75",
//           );

//         // ── Section headers (data-reveal) ────────────────────────────
//         // Simple fade-up for each section label+heading block
//         document
//           .querySelectorAll<HTMLElement>("[data-reveal]")
//           .forEach((el) => {
//             gsap.fromTo(
//               el,
//               { opacity: 0, y: 28 },
//               {
//                 opacity: 1,
//                 y: 0,
//                 duration: 0.65,
//                 ease,
//                 scrollTrigger: { trigger: el, start: "top 88%", toggleActions },
//               },
//             );
//           });

//         // ── Trust strip — items stagger in ───────────────────────────
//         gsap.fromTo(
//           ".trust-items > *",
//           { opacity: 0, y: 10 },
//           {
//             opacity: 1,
//             y: 0,
//             duration: 0.4,
//             ease,
//             stagger: 0.06,
//             scrollTrigger: {
//               trigger: ".trust-items",
//               start: "top 92%",
//               toggleActions,
//             },
//           },
//         );

//         // ── Module cards — grid stagger ───────────────────────────────
//         gsap.fromTo(
//           ".module-card",
//           { opacity: 0, y: 40 },
//           {
//             opacity: 1,
//             y: 0,
//             duration: 0.55,
//             ease,
//             stagger: 0.07,
//             scrollTrigger: {
//               trigger: ".module-grid",
//               start: "top 82%",
//               toggleActions,
//             },
//           },
//         );

//         // ── Stats — staggered left to right ──────────────────────────
//         gsap.fromTo(
//           ".stats-grid > div",
//           { opacity: 0, y: 22 },
//           {
//             opacity: 1,
//             y: 0,
//             duration: 0.5,
//             ease,
//             stagger: 0.1,
//             scrollTrigger: {
//               trigger: ".stats-grid",
//               start: "top 84%",
//               toggleActions,
//             },
//           },
//         );

//         // ── Security cards — staggered ───────────────────────────────
//         gsap.fromTo(
//           ".security-grid > .feature-card",
//           { opacity: 0, y: 32 },
//           {
//             opacity: 1,
//             y: 0,
//             duration: 0.55,
//             ease,
//             stagger: 0.08,
//             scrollTrigger: {
//               trigger: ".security-grid",
//               start: "top 82%",
//               toggleActions,
//             },
//           },
//         );

//         // ── Feature cards — staggered ────────────────────────────────
//         gsap.fromTo(
//           ".feature-grid > .feature-card",
//           { opacity: 0, y: 32 },
//           {
//             opacity: 1,
//             y: 0,
//             duration: 0.6,
//             ease,
//             stagger: 0.12,
//             scrollTrigger: {
//               trigger: ".feature-grid",
//               start: "top 82%",
//               toggleActions,
//             },
//           },
//         );

//         // ── CTA block — scale + fade ──────────────────────────────────
//         const ctaBlock = document.querySelector<HTMLElement>(".cta-block");
//         if (ctaBlock) {
//           gsap.fromTo(
//             ctaBlock,
//             { opacity: 0, y: 32, scale: 0.98 },
//             {
//               opacity: 1,
//               y: 0,
//               scale: 1,
//               duration: 0.75,
//               ease,
//               scrollTrigger: {
//                 trigger: ctaBlock,
//                 start: "top 86%",
//                 toggleActions,
//               },
//             },
//           );
//         }

//         // ── Footer columns — staggered ───────────────────────────────
//         gsap.fromTo(
//           ".footer-grid > *",
//           { opacity: 0, y: 20 },
//           {
//             opacity: 1,
//             y: 0,
//             duration: 0.5,
//             ease,
//             stagger: 0.08,
//             scrollTrigger: {
//               trigger: ".footer-grid",
//               start: "top 92%",
//               toggleActions,
//             },
//           },
//         );
//       });

//       revert = () => ctx.revert();
//     })();

//     return () => revert?.();
//   }, []);

//   return (
//     <div
//       className={`${inter.variable}`}
//       style={{
//         minHeight: "100vh",
//         background: "#0a0a0a",
//         color: "white",
//         overflowX: "hidden",
//         fontFamily: "var(--font-inter, 'Inter', sans-serif)",
//       }}
//     >
//       {/* ─── Embedded styles ─── */}
//       <style>{`
//         @keyframes marquee {
//           from { transform: translateX(0); }
//           to   { transform: translateX(-50%); }
//         }
//         @keyframes soft-pulse {
//           0%, 100% { opacity: 1; }
//           50%       { opacity: 0.35; }
//         }
//         .marquee-track { animation: marquee 32s linear infinite; }
//         .marquee-track:hover { animation-play-state: paused; }
//         .rbac-pulse { animation: soft-pulse 2s ease-in-out infinite; }

//         /* Hero layout */
//         .hero-grid {
//           display: grid;
//           grid-template-columns: 1fr;
//           gap: 56px;
//           align-items: center;
//         }
//         .hero-visual { display: none; }
//         @media (min-width: 1024px) {
//           .hero-grid { grid-template-columns: 1fr 356px; gap: 64px; }
//           .hero-visual { display: block; }
//         }

//         /* Module grid */
//         .module-grid {
//           display: grid;
//           grid-template-columns: 1fr;
//           gap: 1px;
//           background: rgba(255,255,255,0.04);
//           border: 1px solid rgba(255,255,255,0.06);
//           border-radius: 16px;
//           overflow: hidden;
//         }
//         @media (min-width: 640px) {
//           .module-grid { grid-template-columns: repeat(2, 1fr); }
//         }
//         @media (min-width: 1024px) {
//           .module-grid { grid-template-columns: repeat(3, 1fr); }
//         }

//         /* Stats grid */
//         .stats-grid {
//           display: grid;
//           grid-template-columns: repeat(2, 1fr);
//           gap: 1px;
//           background: rgba(255,255,255,0.04);
//           border-left: 1px solid rgba(255,255,255,0.05);
//           border-right: 1px solid rgba(255,255,255,0.05);
//         }
//         @media (min-width: 768px) {
//           .stats-grid { grid-template-columns: repeat(4, 1fr); }
//         }

//         /* Feature grid */
//         .feature-grid {
//           display: grid;
//           grid-template-columns: 1fr;
//           gap: 16px;
//         }
//         @media (min-width: 768px) {
//           .feature-grid { grid-template-columns: repeat(3, 1fr); }
//         }

//         /* Security grid */
//         .security-grid {
//           display: grid;
//           grid-template-columns: 1fr;
//           gap: 16px;
//         }
//         @media (min-width: 640px) {
//           .security-grid { grid-template-columns: repeat(2, 1fr); }
//         }
//         @media (min-width: 1024px) {
//           .security-grid { grid-template-columns: repeat(3, 1fr); }
//         }

//         /* Footer grid */
//         .footer-grid {
//           display: grid;
//           grid-template-columns: 1fr;
//           gap: 40px;
//         }
//         @media (min-width: 768px) {
//           .footer-grid { grid-template-columns: 1.6fr repeat(4, 1fr); gap: 40px; }
//         }

//         /* Trust items */
//         .trust-items {
//           display: flex;
//           flex-wrap: wrap;
//           align-items: center;
//           justify-content: center;
//           gap: 8px 32px;
//         }

//         /* Nav center links */
//         .nav-center { display: none; }
//         @media (min-width: 900px) {
//           .nav-center { display: flex; align-items: center; gap: 2px; }
//         }

//         /* Hover helpers */
//         .module-card { transition: background 0.2s; }
//         .module-card:hover { background: #131313 !important; }
//         .feature-card { transition: background 0.2s, border-color 0.2s; }
//         .feature-card:hover {
//           background: #131313 !important;
//           border-color: rgba(255,255,255,0.1) !important;
//         }
//         .nav-link { transition: color 0.15s, background 0.15s; }
//         .nav-link:hover {
//           color: rgba(255,255,255,0.82) !important;
//           background: rgba(255,255,255,0.04) !important;
//         }
//         .footer-link { transition: color 0.15s; }
//         .footer-link:hover { color: rgba(255,255,255,0.68) !important; }
//         .cta-primary:hover { background: #6d28d9 !important; }
//         .cta-secondary:hover {
//           border-color: rgba(255,255,255,0.14) !important;
//           color: rgba(255,255,255,0.72) !important;
//         }
//         .nav-cta:hover { background: #6d28d9 !important; }
//         .talk-sales:hover { color: rgba(255,255,255,0.65) !important; }
//       `}</style>

//       {/* ══════════════════════════════════════════════
//           NAVIGATION
//       ══════════════════════════════════════════════ */}
//       <header
//         style={{
//           position: "sticky",
//           top: 0,
//           zIndex: 50,
//           borderBottom: "1px solid rgba(255,255,255,0.06)",
//           background: "rgba(10,10,10,0.88)",
//           backdropFilter: "blur(20px)",
//           WebkitBackdropFilter: "blur(20px)",
//         }}
//       >
//         <nav
//           style={{
//             maxWidth: 1200,
//             margin: "0 auto",
//             padding: "13px 24px",
//             display: "flex",
//             alignItems: "center",
//             justifyContent: "space-between",
//             gap: 16,
//           }}
//         >
//           {/* Logo */}
//           <div
//             style={{
//               display: "flex",
//               alignItems: "center",
//               gap: 10,
//               flexShrink: 0,
//             }}
//           >
//             <div
//               style={{
//                 width: 28,
//                 height: 28,
//                 borderRadius: 8,
//                 background: "#7c3aed",
//                 display: "flex",
//                 alignItems: "center",
//                 justifyContent: "center",
//                 boxShadow: "0 0 14px rgba(124,58,237,0.5)",
//                 flexShrink: 0,
//               }}
//             >
//               <span
//                 style={{
//                   fontSize: 11,
//                   fontWeight: 800,
//                   color: "white",
//                 }}
//               >
//                 B
//               </span>
//             </div>
//             <span
//               style={{
//                 fontSize: 15,
//                 fontWeight: 600,
//                 letterSpacing: "-0.2px",
//               }}
//             >
//               BusinessSAAS
//             </span>
//           </div>

//           {/* Center links */}
//           <div className="nav-center">
//             {[
//               { label: "Product", href: "#modules" },
//               { label: "Solutions", href: "#features" },
//               { label: "Enterprise", href: "#security" },
//               { label: "Pricing", href: "#pricing" },
//             ].map(({ label, href }) => (
//               <a
//                 key={label}
//                 href={href}
//                 className="nav-link"
//                 style={{
//                   fontSize: 13,
//                   fontWeight: 500,
//                   color: "rgba(255,255,255,0.38)",
//                   padding: "6px 12px",
//                   borderRadius: 6,
//                   textDecoration: "none",
//                   background: "transparent",
//                 }}
//               >
//                 {label}
//               </a>
//             ))}
//           </div>

//           {/* Right actions */}
//           <div
//             style={{
//               display: "flex",
//               alignItems: "center",
//               gap: 8,
//               flexShrink: 0,
//             }}
//           >
//             <Link
//               href="/login"
//               className="nav-link"
//               style={{
//                 fontSize: 13,
//                 color: "rgba(255,255,255,0.38)",
//                 padding: "6px 12px",
//                 borderRadius: 6,
//                 textDecoration: "none",
//                 background: "transparent",
//               }}
//             >
//               Sign in
//             </Link>
//             <Link
//               href="/signup"
//               className="nav-cta"
//               style={{
//                 display: "flex",
//                 alignItems: "center",
//                 gap: 6,
//                 background: "#7c3aed",
//                 color: "white",
//                 padding: "8px 16px",
//                 borderRadius: 8,
//                 fontSize: 13,
//                 fontWeight: 600,
//                 textDecoration: "none",
//                 boxShadow: "0 0 20px rgba(124,58,237,0.35)",
//                 transition: "background 0.15s",
//               }}
//             >
//               Book a Demo
//               <ArrowRight style={{ width: 13, height: 13 }} />
//             </Link>
//           </div>
//         </nav>
//       </header>

//       {/* ══════════════════════════════════════════════
//           HERO
//       ══════════════════════════════════════════════ */}
//       <section
//         style={{
//           position: "relative",
//           maxWidth: 1200,
//           margin: "0 auto",
//           padding: "104px 24px 96px",
//         }}
//       >
//         {/* Dot-grid background texture */}
//         <div
//           aria-hidden
//           style={{
//             position: "absolute",
//             inset: 0,
//             backgroundImage:
//               "radial-gradient(circle, rgba(255,255,255,0.022) 1px, transparent 1px)",
//             backgroundSize: "32px 32px",
//             pointerEvents: "none",
//           }}
//         />

//         {/* Purple radial glow */}
//         <div
//           aria-hidden
//           style={{
//             position: "absolute",
//             top: -40,
//             left: "18%",
//             width: 640,
//             height: 360,
//             background:
//               "radial-gradient(ellipse at center, rgba(124,58,237,0.13) 0%, transparent 70%)",
//             filter: "blur(48px)",
//             pointerEvents: "none",
//           }}
//         />

//         <div className="hero-grid" style={{ position: "relative" }}>
//           {/* ── Left: content ── */}
//           <div style={{ maxWidth: 560 }}>
//             {/* Badge */}
//             <div
//               ref={badgeRef}
//               style={{
//                 display: "inline-flex",
//                 alignItems: "center",
//                 gap: 8,
//                 borderRadius: 100,
//                 border: "1px solid rgba(124,58,237,0.28)",
//                 background: "rgba(124,58,237,0.09)",
//                 padding: "6px 14px",
//                 marginBottom: 28,
//               }}
//             >
//               <span
//                 className="rbac-pulse"
//                 style={{
//                   width: 6,
//                   height: 6,
//                   borderRadius: "50%",
//                   background: "#a855f7",
//                   display: "inline-block",
//                   flexShrink: 0,
//                 }}
//               />
//               <span
//                 style={{
//                   fontSize: 11,
//                   fontWeight: 600,
//                   color: "#a855f7",
//                   letterSpacing: "0.03em",
//                 }}
//               >
//                 SOC 2 aligned · CRM module now in production
//               </span>
//             </div>

//             {/* Headline */}
//             <h1
//               ref={headlineRef}
//               style={{
//                 fontSize: "clamp(38px, 4vw, 54px)",
//                 fontWeight: 700,
//                 lineHeight: 1.1,
//                 letterSpacing: "-0.5px",
//                 color: "white",
//                 marginBottom: 24,
//               }}
//             >
//               Run your entire
//               <br />
//               business from
//               <br />
//               <span
//                 style={{
//                   background:
//                     "linear-gradient(135deg, #7c3aed 0%, #a855f7 55%, #c084fc 100%)",
//                   WebkitBackgroundClip: "text",
//                   WebkitTextFillColor: "transparent",
//                   backgroundClip: "text",
//                 }}
//               >
//                 one platform.
//               </span>
//             </h1>

//             {/* Sub */}
//             <p
//               ref={subRef}
//               style={{
//                 fontSize: 17,
//                 lineHeight: 1.72,
//                 color: "rgba(255,255,255,0.42)",
//                 maxWidth: 440,
//                 marginBottom: 36,
//               }}
//             >
//               Consolidate CRM, HRM, Accounting, and Projects into a single
//               secure workspace — with enterprise-grade access control and
//               audit-ready infrastructure built in from day one.
//             </p>

//             {/* CTAs */}
//             <div
//               ref={ctaRef}
//               style={{
//                 display: "flex",
//                 flexWrap: "wrap",
//                 alignItems: "center",
//                 gap: 12,
//               }}
//             >
//               <Link
//                 href="/signup"
//                 className="cta-primary"
//                 style={{
//                   display: "flex",
//                   alignItems: "center",
//                   gap: 9,
//                   background: "#7c3aed",
//                   color: "white",
//                   padding: "14px 26px",
//                   borderRadius: 10,
//                   fontSize: 15,
//                   fontWeight: 600,
//                   textDecoration: "none",
//                   boxShadow: "0 0 28px rgba(124,58,237,0.42)",
//                   transition: "background 0.15s",
//                 }}
//               >
//                 Book a Demo
//                 <ArrowRight style={{ width: 15, height: 15 }} />
//               </Link>
//               <Link
//                 href="/signup"
//                 className="cta-secondary"
//                 style={{
//                   display: "flex",
//                   alignItems: "center",
//                   padding: "14px 26px",
//                   borderRadius: 10,
//                   fontSize: 15,
//                   fontWeight: 500,
//                   textDecoration: "none",
//                   border: "1px solid rgba(255,255,255,0.09)",
//                   color: "rgba(255,255,255,0.52)",
//                   transition: "border-color 0.15s, color 0.15s",
//                 }}
//               >
//                 Start Free Trial
//               </Link>
//             </div>

//             <div
//               style={{
//                 marginTop: 18,
//                 display: "flex",
//                 flexWrap: "wrap",
//                 alignItems: "center",
//                 gap: 16,
//               }}
//             >
//               <p
//                 style={{
//                   fontSize: 11,
//                   color: "rgba(255,255,255,0.18)",
//                   letterSpacing: "0.02em",
//                 }}
//               >
//                 No credit card required · Enterprise plans available
//               </p>
//               <a
//                 href="#pricing"
//                 className="talk-sales"
//                 style={{
//                   fontSize: 11,
//                   color: "rgba(255,255,255,0.28)",
//                   textDecoration: "none",
//                   display: "inline-flex",
//                   alignItems: "center",
//                   gap: 4,
//                   transition: "color 0.15s",
//                 }}
//               >
//                 Talk to Sales
//                 <ArrowRight style={{ width: 10, height: 10 }} />
//               </a>
//             </div>
//           </div>

//           {/* ── Right: RBAC visual ── */}
//           <div className="hero-visual" ref={visualRef}>
//             <PermissionCard />
//           </div>
//         </div>
//       </section>

//       {/* ══════════════════════════════════════════════
//           TRUST STRIP
//       ══════════════════════════════════════════════ */}
//       <div
//         style={{
//           borderTop: "1px solid rgba(255,255,255,0.04)",
//           borderBottom: "1px solid rgba(255,255,255,0.04)",
//           padding: "36px 24px",
//         }}
//       >
//         <div style={{ maxWidth: 1200, margin: "0 auto", textAlign: "center" }}>
//           <p
//             style={{
//               fontSize: 9.5,
//               fontWeight: 700,
//               textTransform: "uppercase",
//               letterSpacing: "0.12em",
//               color: "rgba(255,255,255,0.14)",
//               marginBottom: 20,
//             }}
//           >
//             Trusted by forward-thinking teams
//           </p>
//           <div className="trust-items">
//             {TRUST_ORGS.map((org, i) => (
//               <React.Fragment key={org}>
//                 <span
//                   style={{
//                     fontSize: 13,
//                     fontWeight: 600,
//                     color: "rgba(255,255,255,0.18)",
//                     letterSpacing: "0",
//                     whiteSpace: "nowrap",
//                   }}
//                 >
//                   {org}
//                 </span>
//                 {i < TRUST_ORGS.length - 1 && (
//                   <span
//                     style={{
//                       width: 3,
//                       height: 3,
//                       borderRadius: "50%",
//                       background: "rgba(255,255,255,0.08)",
//                       display: "inline-block",
//                       flexShrink: 0,
//                     }}
//                   />
//                 )}
//               </React.Fragment>
//             ))}
//           </div>
//         </div>
//       </div>

//       {/* ══════════════════════════════════════════════
//           MARQUEE
//       ══════════════════════════════════════════════ */}
//       <div
//         style={{
//           overflow: "hidden",
//           borderBottom: "1px solid rgba(255,255,255,0.05)",
//           padding: "16px 0",
//         }}
//       >
//         <div
//           className="marquee-track"
//           style={{ display: "flex", width: "max-content" }}
//         >
//           {[0, 1].map((gi) =>
//             MARQUEE_ITEMS.map((m) => (
//               <span
//                 key={`${gi}-${m}`}
//                 style={{
//                   display: "inline-flex",
//                   alignItems: "center",
//                   padding: "0 32px",
//                   fontSize: 11,
//                   fontWeight: 700,
//                   textTransform: "uppercase",
//                   letterSpacing: "0.1em",
//                   color: "rgba(255,255,255,0.13)",
//                   whiteSpace: "nowrap",
//                 }}
//               >
//                 {m}
//                 <span
//                   style={{
//                     display: "inline-block",
//                     marginLeft: 32,
//                     width: 4,
//                     height: 4,
//                     borderRadius: "50%",
//                     background: "rgba(255,255,255,0.09)",
//                     flexShrink: 0,
//                   }}
//                 />
//               </span>
//             )),
//           )}
//         </div>
//       </div>

//       {/* ══════════════════════════════════════════════
//           MODULES
//       ══════════════════════════════════════════════ */}
//       <section
//         id="modules"
//         style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}
//       >
//         <div data-reveal style={{ marginBottom: 48 }}>
//           <p
//             style={{
//               fontSize: 11,
//               fontWeight: 700,
//               textTransform: "uppercase",
//               letterSpacing: "0.1em",
//               color: "rgba(255,255,255,0.22)",
//               marginBottom: 12,
//             }}
//           >
//             Platform Suite
//           </p>
//           <h2
//             style={{
//               fontSize: "clamp(26px, 2.8vw, 36px)",
//               fontWeight: 700,
//               lineHeight: 1.2,
//               letterSpacing: "-0.3px",
//               color: "white",
//             }}
//           >
//             Every business function, unified in one platform.
//           </h2>
//         </div>

//         <div className="module-grid">
//           {MODULES.map((mod) => (
//             <div
//               key={mod.name}
//               className="module-card"
//               style={{
//                 background: "#0e0e0e",
//                 padding: "28px 28px 24px",
//                 position: "relative",
//               }}
//             >
//               {/* Status badge */}
//               {mod.status === "live" ? (
//                 <div
//                   style={{
//                     position: "absolute",
//                     top: 16,
//                     right: 16,
//                     display: "flex",
//                     alignItems: "center",
//                     gap: 5,
//                     borderRadius: 100,
//                     background: "rgba(124,58,237,0.13)",
//                     border: "1px solid rgba(124,58,237,0.24)",
//                     padding: "3px 10px",
//                   }}
//                 >
//                   <span
//                     className="rbac-pulse"
//                     style={{
//                       width: 5,
//                       height: 5,
//                       borderRadius: "50%",
//                       background: "#a855f7",
//                       display: "inline-block",
//                     }}
//                   />
//                   <span
//                     style={{
//                       fontSize: 8.5,
//                       fontWeight: 700,
//                       textTransform: "uppercase",
//                       letterSpacing: "0.1em",
//                       color: "#a855f7",
//                     }}
//                   >
//                     Live
//                   </span>
//                 </div>
//               ) : (
//                 <div
//                   style={{
//                     position: "absolute",
//                     top: 16,
//                     right: 16,
//                     borderRadius: 100,
//                     border: "1px solid rgba(255,255,255,0.07)",
//                     padding: "3px 10px",
//                   }}
//                 >
//                   <span
//                     style={{
//                       fontSize: 8.5,
//                       fontWeight: 600,
//                       textTransform: "uppercase",
//                       letterSpacing: "0.1em",
//                       color: "rgba(255,255,255,0.18)",
//                     }}
//                   >
//                     Roadmap
//                   </span>
//                 </div>
//               )}

//               <h3
//                 style={{
//                   fontSize: 20,
//                   fontWeight: 700,
//                   letterSpacing: "0",
//                   lineHeight: 1.1,
//                   color:
//                     mod.status === "live"
//                       ? "rgba(255,255,255,0.9)"
//                       : "rgba(255,255,255,0.42)",
//                   marginBottom: 9,
//                 }}
//               >
//                 {mod.name}
//               </h3>

//               <p
//                 style={{
//                   fontSize: 13,
//                   lineHeight: 1.65,
//                   color: "rgba(255,255,255,0.3)",
//                   marginBottom: 20,
//                   minHeight: 52,
//                 }}
//               >
//                 {mod.desc}
//               </p>

//               <div
//                 style={{
//                   fontSize: 10.5,
//                   fontWeight: 500,
//                   color: "rgba(255,255,255,0.14)",
//                   paddingTop: 14,
//                   borderTop: "1px solid rgba(255,255,255,0.05)",
//                   letterSpacing: "0.02em",
//                 }}
//               >
//                 {mod.meta}
//               </div>

//               {mod.status === "live" && (
//                 <div
//                   aria-hidden
//                   style={{
//                     position: "absolute",
//                     bottom: 0,
//                     left: 0,
//                     right: 0,
//                     height: 2,
//                     background:
//                       "linear-gradient(90deg, transparent 0%, #7c3aed 40%, #a855f7 60%, transparent 100%)",
//                     opacity: 0.65,
//                   }}
//                 />
//               )}
//             </div>
//           ))}
//         </div>
//       </section>

//       {/* ══════════════════════════════════════════════
//           STATS STRIP
//       ══════════════════════════════════════════════ */}
//       <div
//         style={{
//           borderTop: "1px solid rgba(255,255,255,0.05)",
//           borderBottom: "1px solid rgba(255,255,255,0.05)",
//           background: "#0c0c0c",
//         }}
//       >
//         <div style={{ maxWidth: 1200, margin: "0 auto", padding: "0 24px" }}>
//           <div className="stats-grid">
//             {STATS.map((stat) => (
//               <div
//                 key={stat.n}
//                 style={{
//                   background: "#0c0c0c",
//                   padding: "44px 32px",
//                   textAlign: "center",
//                 }}
//               >
//                 <div
//                   style={{
//                     fontSize: "clamp(30px, 3vw, 42px)",
//                     fontWeight: 700,
//                     lineHeight: 1,
//                     letterSpacing: "-0.5px",
//                     color: "white",
//                     marginBottom: 7,
//                   }}
//                 >
//                   {stat.n}
//                 </div>
//                 <div
//                   style={{
//                     fontSize: 10.5,
//                     fontWeight: 600,
//                     textTransform: "uppercase",
//                     letterSpacing: "0.1em",
//                     color: "rgba(255,255,255,0.22)",
//                   }}
//                 >
//                   {stat.label}
//                 </div>
//               </div>
//             ))}
//           </div>
//         </div>
//       </div>

//       {/* ══════════════════════════════════════════════
//           SECURITY
//       ══════════════════════════════════════════════ */}
//       <section
//         id="security"
//         style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}
//       >
//         <div data-reveal style={{ marginBottom: 56 }}>
//           <p
//             style={{
//               fontSize: 11,
//               fontWeight: 700,
//               textTransform: "uppercase",
//               letterSpacing: "0.1em",
//               color: "rgba(255,255,255,0.22)",
//               marginBottom: 12,
//             }}
//           >
//             Security & Compliance
//           </p>
//           <h2
//             style={{
//               fontSize: "clamp(26px, 2.8vw, 36px)",
//               fontWeight: 700,
//               lineHeight: 1.2,
//               letterSpacing: "-0.3px",
//               color: "white",
//             }}
//           >
//             Enterprise-grade security. Built in, not bolted on.
//           </h2>
//         </div>

//         <div className="security-grid">
//           {SECURITY_ITEMS.map(({ Icon, title, desc }) => (
//             <div
//               key={title}
//               className="feature-card"
//               style={{
//                 borderRadius: 14,
//                 border: "1px solid rgba(255,255,255,0.06)",
//                 background: "#0e0e0e",
//                 padding: "28px",
//               }}
//             >
//               <div
//                 style={{
//                   width: 40,
//                   height: 40,
//                   borderRadius: 10,
//                   background: "rgba(124,58,237,0.1)",
//                   border: "1px solid rgba(124,58,237,0.2)",
//                   display: "flex",
//                   alignItems: "center",
//                   justifyContent: "center",
//                   marginBottom: 20,
//                   flexShrink: 0,
//                 }}
//               >
//                 <Icon
//                   style={{ width: 18, height: 18, color: "#7c3aed" }}
//                   strokeWidth={1.75}
//                 />
//               </div>
//               <h3
//                 style={{
//                   fontSize: 15,
//                   fontWeight: 600,
//                   letterSpacing: "0",
//                   lineHeight: 1.4,
//                   color: "white",
//                   marginBottom: 10,
//                 }}
//               >
//                 {title}
//               </h3>
//               <p
//                 style={{
//                   fontSize: 13,
//                   lineHeight: 1.72,
//                   color: "rgba(255,255,255,0.34)",
//                 }}
//               >
//                 {desc}
//               </p>
//             </div>
//           ))}
//         </div>
//       </section>

//       {/* ══════════════════════════════════════════════
//           FEATURES
//       ══════════════════════════════════════════════ */}
//       <section
//         id="features"
//         style={{
//           background: "#0c0c0c",
//           borderTop: "1px solid rgba(255,255,255,0.04)",
//           borderBottom: "1px solid rgba(255,255,255,0.04)",
//         }}
//       >
//         <div style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}>
//           <div data-reveal style={{ marginBottom: 56 }}>
//             <p
//               style={{
//                 fontSize: 11,
//                 fontWeight: 700,
//                 textTransform: "uppercase",
//                 letterSpacing: "0.1em",
//                 color: "rgba(255,255,255,0.22)",
//                 marginBottom: 12,
//               }}
//             >
//               Foundation
//             </p>
//             <h2
//               style={{
//                 fontSize: "clamp(26px, 2.8vw, 36px)",
//                 fontWeight: 700,
//                 lineHeight: 1.2,
//                 letterSpacing: "-0.3px",
//                 color: "white",
//               }}
//             >
//               Not just features. Architecture that holds.
//             </h2>
//           </div>

//           <div className="feature-grid">
//             {FEATURES.map(({ Icon, title, desc }) => (
//               <div
//                 key={title}
//                 className="feature-card"
//                 style={{
//                   borderRadius: 14,
//                   border: "1px solid rgba(255,255,255,0.06)",
//                   background: "#0e0e0e",
//                   padding: "28px",
//                 }}
//               >
//                 <div
//                   style={{
//                     width: 40,
//                     height: 40,
//                     borderRadius: 10,
//                     background: "rgba(124,58,237,0.1)",
//                     border: "1px solid rgba(124,58,237,0.2)",
//                     display: "flex",
//                     alignItems: "center",
//                     justifyContent: "center",
//                     marginBottom: 20,
//                     flexShrink: 0,
//                   }}
//                 >
//                   <Icon
//                     style={{ width: 18, height: 18, color: "#7c3aed" }}
//                     strokeWidth={1.75}
//                   />
//                 </div>
//                 <h3
//                   style={{
//                     fontSize: 15,
//                     fontWeight: 600,
//                     letterSpacing: "0",
//                     lineHeight: 1.4,
//                     color: "white",
//                     marginBottom: 10,
//                   }}
//                 >
//                   {title}
//                 </h3>
//                 <p
//                   style={{
//                     fontSize: 13,
//                     lineHeight: 1.72,
//                     color: "rgba(255,255,255,0.34)",
//                   }}
//                 >
//                   {desc}
//                 </p>
//               </div>
//             ))}
//           </div>
//         </div>
//       </section>

//       {/* ══════════════════════════════════════════════
//           CTA
//       ══════════════════════════════════════════════ */}
//       <section
//         id="pricing"
//         style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}
//       >
//         <div
//           className="cta-block"
//           style={{
//             position: "relative",
//             overflow: "hidden",
//             borderRadius: 20,
//             border: "1px solid rgba(124,58,237,0.18)",
//             background:
//               "linear-gradient(145deg, #0e0e0e 0%, #0f0f1b 50%, #0e0e0e 100%)",
//             padding: "80px 40px",
//             textAlign: "center",
//           }}
//         >
//           {/* Glow */}
//           <div
//             aria-hidden
//             style={{
//               position: "absolute",
//               inset: 0,
//               background:
//                 "radial-gradient(ellipse at 50% 60%, rgba(124,58,237,0.15) 0%, transparent 65%)",
//               filter: "blur(32px)",
//               pointerEvents: "none",
//             }}
//           />

//           {/* Top shimmer */}
//           <div
//             aria-hidden
//             style={{
//               position: "absolute",
//               top: 0,
//               left: "15%",
//               right: "15%",
//               height: 1,
//               background:
//                 "linear-gradient(90deg, transparent 0%, rgba(124,58,237,0.7) 50%, transparent 100%)",
//             }}
//           />

//           <div style={{ position: "relative" }}>
//             <p
//               style={{
//                 fontSize: 11,
//                 fontWeight: 700,
//                 textTransform: "uppercase",
//                 letterSpacing: "0.1em",
//                 color: "rgba(168,85,247,0.55)",
//                 marginBottom: 16,
//               }}
//             >
//               Schedule a demo
//             </p>

//             <h2
//               style={{
//                 fontSize: "clamp(30px, 3.2vw, 44px)",
//                 fontWeight: 700,
//                 lineHeight: 1.15,
//                 letterSpacing: "-0.5px",
//                 color: "white",
//                 marginBottom: 20,
//               }}
//             >
//               One platform.
//               <br />
//               Every business function.
//             </h2>

//             <p
//               style={{
//                 fontSize: 15,
//                 lineHeight: 1.65,
//                 color: "rgba(255,255,255,0.36)",
//                 maxWidth: 420,
//                 margin: "0 auto 36px",
//               }}
//             >
//               See how BusinessSAAS consolidates your operations, secures your
//               data, and scales with your organization — in a personalized
//               walkthrough.
//             </p>

//             <div
//               style={{
//                 display: "flex",
//                 flexWrap: "wrap",
//                 alignItems: "center",
//                 justifyContent: "center",
//                 gap: 12,
//                 marginBottom: 20,
//               }}
//             >
//               <Link
//                 href="/signup"
//                 className="cta-primary"
//                 style={{
//                   display: "flex",
//                   alignItems: "center",
//                   gap: 9,
//                   background: "#7c3aed",
//                   color: "white",
//                   padding: "14px 28px",
//                   borderRadius: 10,
//                   fontSize: 15,
//                   fontWeight: 600,
//                   textDecoration: "none",
//                   boxShadow: "0 0 32px rgba(124,58,237,0.45)",
//                   transition: "background 0.15s",
//                 }}
//               >
//                 Book a Demo
//                 <ArrowRight style={{ width: 15, height: 15 }} />
//               </Link>
//               <Link
//                 href="/signup"
//                 className="cta-secondary"
//                 style={{
//                   display: "flex",
//                   alignItems: "center",
//                   padding: "14px 28px",
//                   borderRadius: 10,
//                   fontSize: 15,
//                   fontWeight: 500,
//                   textDecoration: "none",
//                   border: "1px solid rgba(255,255,255,0.09)",
//                   color: "rgba(255,255,255,0.48)",
//                   transition: "border-color 0.15s, color 0.15s",
//                 }}
//               >
//                 Start Free Trial
//               </Link>
//             </div>

//             <p
//               style={{
//                 fontSize: 11,
//                 color: "rgba(255,255,255,0.16)",
//                 letterSpacing: "0.02em",
//               }}
//             >
//               No credit card required · Enterprise contracts available · Custom
//               pricing for 50+ seats
//             </p>
//           </div>
//         </div>
//       </section>

//       {/* ══════════════════════════════════════════════
//           FOOTER
//       ══════════════════════════════════════════════ */}
//       <footer
//         style={{
//           borderTop: "1px solid rgba(255,255,255,0.05)",
//           background: "#0a0a0a",
//         }}
//       >
//         <div
//           style={{
//             maxWidth: 1200,
//             margin: "0 auto",
//             padding: "64px 24px 40px",
//           }}
//         >
//           {/* Main grid */}
//           <div className="footer-grid">
//             {/* Company column */}
//             <div>
//               <div
//                 style={{
//                   display: "flex",
//                   alignItems: "center",
//                   gap: 8,
//                   marginBottom: 16,
//                 }}
//               >
//                 <div
//                   style={{
//                     width: 24,
//                     height: 24,
//                     borderRadius: 7,
//                     background: "rgba(124,58,237,0.75)",
//                     display: "flex",
//                     alignItems: "center",
//                     justifyContent: "center",
//                     flexShrink: 0,
//                   }}
//                 >
//                   <span
//                     style={{
//                       fontSize: 9,
//                       fontWeight: 800,
//                       color: "white",
//                     }}
//                   >
//                     B
//                   </span>
//                 </div>
//                 <span
//                   style={{
//                     fontSize: 14,
//                     fontWeight: 600,
//                     letterSpacing: "-0.2px",
//                     color: "rgba(255,255,255,0.82)",
//                   }}
//                 >
//                   BusinessSAAS
//                 </span>
//               </div>
//               <p
//                 style={{
//                   fontSize: 12.5,
//                   lineHeight: 1.72,
//                   color: "rgba(255,255,255,0.26)",
//                   maxWidth: 220,
//                   marginBottom: 24,
//                 }}
//               >
//                 A unified business operating platform built for organizations
//                 that demand security, auditability, and scale.
//               </p>
//               {/* SOC 2 badge */}
//               <div
//                 style={{
//                   display: "inline-flex",
//                   alignItems: "center",
//                   gap: 6,
//                   border: "1px solid rgba(255,255,255,0.07)",
//                   borderRadius: 8,
//                   padding: "8px 12px",
//                 }}
//               >
//                 <Shield
//                   style={{
//                     width: 11,
//                     height: 11,
//                     color: "rgba(255,255,255,0.28)",
//                     flexShrink: 0,
//                   }}
//                   strokeWidth={2}
//                 />
//                 <span
//                   style={{
//                     fontSize: 9.5,
//                     fontWeight: 700,
//                     textTransform: "uppercase",
//                     letterSpacing: "0.1em",
//                     color: "rgba(255,255,255,0.24)",
//                   }}
//                 >
//                   SOC 2 Aligned
//                 </span>
//               </div>
//             </div>

//             {/* Link columns */}
//             {FOOTER_COLS.map((col) => (
//               <div key={col.heading}>
//                 <p
//                   style={{
//                     fontSize: 10.5,
//                     fontWeight: 700,
//                     textTransform: "uppercase",
//                     letterSpacing: "0.08em",
//                     color: "rgba(255,255,255,0.2)",
//                     marginBottom: 18,
//                   }}
//                 >
//                   {col.heading}
//                 </p>
//                 <ul
//                   style={{
//                     listStyle: "none",
//                     padding: 0,
//                     margin: 0,
//                     display: "flex",
//                     flexDirection: "column",
//                     gap: 11,
//                   }}
//                 >
//                   {col.links.map((link) => (
//                     <li key={link}>
//                       <a
//                         href="#"
//                         className="footer-link"
//                         style={{
//                           fontSize: 13,
//                           color: "rgba(255,255,255,0.34)",
//                           textDecoration: "none",
//                         }}
//                       >
//                         {link}
//                       </a>
//                     </li>
//                   ))}
//                 </ul>
//               </div>
//             ))}
//           </div>

//           {/* Bottom bar */}
//           <div
//             style={{
//               marginTop: 52,
//               paddingTop: 24,
//               borderTop: "1px solid rgba(255,255,255,0.05)",
//               display: "flex",
//               alignItems: "center",
//               justifyContent: "space-between",
//               flexWrap: "wrap",
//               gap: 12,
//             }}
//           >
//             <span style={{ fontSize: 12, color: "rgba(255,255,255,0.18)" }}>
//               © {new Date().getFullYear()} BusinessSAAS. All rights reserved.
//             </span>
//             <span style={{ fontSize: 12, color: "rgba(255,255,255,0.1)" }}>
//               Built with Go + Next.js
//             </span>
//           </div>
//         </div>
//       </footer>
//     </div>
//   );
// }

"use client";

// ═══════════════════════════════════════════════════════════════════
// app/page.tsx
// BusinessSAAS — Public marketing homepage
// Dark · Purple accent (#7c3aed / #a855f7) · Syne headings + Inter body · GSAP
//
// Redo notes (2026-07-20):
// - Design tokens now mirror the REAL app (globals.css / layout.tsx / login
//   page), which is dark-default + purple + Syne/Inter — not the indigo/
//   light system still documented in Project_Instruction.md Section 3/7.
//   Worth reconciling that doc separately; this file matches what's actually
//   shipped.
// - MODULES/STATS corrected: HRM is live (was shown as "soon"), fabricated
//   trust logos removed, hard "Coming Qx 2026" dates removed.
// - Added: Product Preview section, FAQ section.
// - "Book a Demo" / "Join Early Access" now open a real capture form that
//   posts to /api/lead-capture → backend POST /pub/leads (the capture
//   endpoint your own audit already confirmed works). See that route file
//   for the one field-name assumption that still needs a source check.
// ═══════════════════════════════════════════════════════════════════

import React, { useEffect, useLayoutEffect, useRef, useState } from "react";
import Link from "next/link";
import gsap from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import { toast } from "sonner";
import {
  ArrowRight,
  Shield,
  GitBranch,
  Lock,
  Server,
  FileText,
  CheckCircle,
  Building2,
  X,
  ChevronDown,
  TrendingUp,
  Boxes,
  Sparkles,
} from "lucide-react";
import { Syne, Inter } from "next/font/google";

gsap.registerPlugin(ScrollTrigger);

const useIsomorphicLayoutEffect =
  typeof window !== "undefined" ? useLayoutEffect : useEffect;

const syne = Syne({
  subsets: ["latin"],
  variable: "--font-syne",
  weight: ["600", "700", "800"],
  display: "swap",
});

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
const FONT_INTER = "var(--font-inter, Inter, sans-serif)";
const PURPLE = "#7c3aed";
const PURPLE_HOVER = "#a855f7";

// ─── Data ────────────────────────────────────────────────────────────

const NAV_LINKS = [
  { label: "Product", href: "#modules" },
  { label: "Security", href: "#security" },
  { label: "Pricing", href: "#pricing" },
];

const MODULES = [
  {
    name: "CRM",
    desc: "Full-cycle relationship management — leads, deals, contact timelines, and pipeline analytics across your entire organization.",
    status: "live" as const,
    meta: "Live · production-ready",
  },
  {
    name: "HRM",
    desc: "Departments, headcount, leave, attendance, payroll, and the full employee lifecycle in one system of record.",
    status: "live" as const,
    meta: "Live · production-ready",
  },
  {
    name: "Accounting",
    desc: "Invoices, expense tracking, bank reconciliation, and executive-level financial reporting.",
    status: "soon" as const,
    meta: "On the roadmap",
  },
  {
    name: "Projects",
    desc: "Task tracking, milestone management, resource allocation, and cross-team workload visibility.",
    status: "soon" as const,
    meta: "On the roadmap",
  },
  {
    name: "E-commerce",
    desc: "Product catalog, order management, customer segmentation, and real-time inventory control.",
    status: "soon" as const,
    meta: "On the roadmap",
  },
  {
    name: "Learning",
    desc: "Internal training programs, course delivery, completion tracking, and employee skills assessment.",
    status: "soon" as const,
    meta: "On the roadmap",
  },
];

const STATS = [
  { n: "2", label: "Modules live now" },
  { n: "5", label: "Role tiers" },
  { n: "64", label: "Schema migrations" },
  { n: "365+", label: "API endpoints" },
];

const FEATURES = [
  {
    Icon: Boxes,
    title: "One system of record",
    desc: "Every note, task, and email lands on the same contact timeline. CRM and HRM read from the same shared engagement layer — not two products stapled together.",
  },
  {
    Icon: Shield,
    title: "Permissions built to be trusted",
    desc: "Five role tiers plus per-member overrides, enforced on every single request at the middleware layer — not just a hidden button in the UI.",
  },
  {
    Icon: GitBranch,
    title: "Grows without duct tape",
    desc: "New modules plug into the same multi-tenant core, the same roles, the same timeline. Nothing bolted on after the fact.",
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
    title: "SOC 2 Principles",
    desc: "Infrastructure, logging, and access-control policies follow SOC 2 Type II principles from day one — not certified yet, built to survive the audit when we get there.",
  },
];

const TRUST_BADGES = [
  "Multi-tenant from day one",
  "RBAC enforced on every request",
  "Immutable audit logs",
  "Zero plaintext secrets",
];

const FOOTER_COLS = [
  {
    heading: "Product",
    links: [
      { label: "CRM", href: "#modules" },
      { label: "HRM", href: "#modules" },
      { label: "Roadmap", href: "#modules" },
      { label: "Security", href: "#security" },
    ],
  },
  {
    heading: "Solutions",
    links: [
      { label: "For Agencies", href: "#" },
      { label: "For Startups", href: "#" },
      { label: "For Enterprise", href: "#" },
      { label: "For SMBs", href: "#" },
    ],
  },
  {
    heading: "Company",
    links: [
      { label: "About", href: "#" },
      { label: "Blog", href: "#" },
    ],
  },
  {
    heading: "Legal",
    links: [
      { label: "Privacy Policy", href: "#" },
      { label: "Terms of Service", href: "#" },
    ],
  },
];

const FAQS = [
  {
    q: "Which modules are actually live today?",
    a: "CRM and HRM are fully built and in active use — everything listed for them ships, nothing is a placeholder. Accounting, Projects, E-commerce, and Learning are on the roadmap. We'd rather tell you what's real than pad the list.",
  },
  {
    q: "How does data isolation work between organizations?",
    a: "Every request is scoped to your organization at the middleware layer, not filtered in a query somewhere downstream. Cross-tenant access is architecturally blocked — it isn't a setting anyone could accidentally leave off.",
  },
  {
    q: "What does pricing look like?",
    a: "We're finalizing plans as we bring on early customers, so we won't guess a number here. Book a demo or join early access and we'll walk you through it directly.",
  },
  {
    q: "Can I bring over data from what I use today?",
    a: "Yes. Get on a call with us and we'll scope the import together — spreadsheets, exports from your current CRM or HR tool, whatever you're sitting on.",
  },
  {
    q: "Why not just connect separate CRM, HR, and accounting tools?",
    a: "You can, and plenty of teams do. The trade-off is a contact record that lives in one tool, a note about them in another, and a task in a third. BusinessSAAS keeps all of it on one timeline, under one set of permissions, from day one.",
  },
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

const PIPELINE_STAGES = [
  {
    name: "Qualified",
    dot: "#7c3aed",
    deals: [
      { company: "Nexus Solutions", initial: "N", value: "$24,000" },
      { company: "Vantage Digital", initial: "V", value: "$8,400" },
    ],
  },
  {
    name: "Proposal",
    dot: "#a855f7",
    deals: [{ company: "Axiom Corp", initial: "A", value: "$41,200" }],
  },
  {
    name: "Won",
    dot: "#34d399",
    deals: [{ company: "Strata Partners", initial: "S", value: "$18,600" }],
  },
];

const modalInputStyle: React.CSSProperties = {
  width: "100%",
  background: "#161616",
  border: "1px solid rgba(255,255,255,0.08)",
  borderRadius: 8,
  padding: "10px 12px",
  fontSize: 13.5,
  color: "white",
  outline: "none",
  fontFamily: FONT_INTER,
  transition: "border-color 150ms ease, box-shadow 150ms ease",
};

function focusInput(e: React.FocusEvent<HTMLInputElement>) {
  e.currentTarget.style.borderColor = PURPLE;
  e.currentTarget.style.boxShadow = "0 0 0 3px rgba(124,58,237,0.14)";
}
function blurInput(e: React.FocusEvent<HTMLInputElement>) {
  e.currentTarget.style.borderColor = "rgba(255,255,255,0.08)";
  e.currentTarget.style.boxShadow = "none";
}

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

// ─── Pipeline Preview (product-preview visual) ────────────────────────

function PipelinePreviewCard() {
  return (
    <div style={{ position: "relative", width: "100%", maxWidth: 460 }}>
      <div
        aria-hidden
        style={{
          position: "absolute",
          inset: -24,
          background:
            "radial-gradient(ellipse at 50% 40%, rgba(124,58,237,0.18) 0%, transparent 68%)",
          filter: "blur(32px)",
          pointerEvents: "none",
        }}
      />
      <div
        style={{
          position: "relative",
          borderRadius: 18,
          border: "1px solid rgba(255,255,255,0.09)",
          background: "#111111",
          padding: "22px 20px 20px",
          boxShadow:
            "0 40px 80px rgba(0,0,0,0.65), 0 0 0 1px rgba(124,58,237,0.08)",
          transform: "rotate(-1.2deg)",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            marginBottom: 18,
          }}
        >
          <TrendingUp
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
            CRM · Pipeline board
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

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(3, 1fr)",
            gap: 10,
          }}
        >
          {PIPELINE_STAGES.map((stage) => (
            <div key={stage.name}>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 5,
                  marginBottom: 8,
                }}
              >
                <span
                  style={{
                    width: 6,
                    height: 6,
                    borderRadius: "50%",
                    background: stage.dot,
                    flexShrink: 0,
                  }}
                />
                <span
                  style={{
                    fontSize: 8.5,
                    fontWeight: 700,
                    textTransform: "uppercase",
                    letterSpacing: "0.07em",
                    color: "rgba(255,255,255,0.3)",
                  }}
                >
                  {stage.name}
                </span>
              </div>
              <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                {stage.deals.map((deal) => (
                  <div
                    key={deal.company}
                    style={{
                      borderRadius: 8,
                      background: "rgba(255,255,255,0.03)",
                      border: "1px solid rgba(255,255,255,0.06)",
                      padding: "8px",
                    }}
                  >
                    <div
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: 6,
                        marginBottom: 5,
                      }}
                    >
                      <div
                        style={{
                          width: 16,
                          height: 16,
                          borderRadius: 5,
                          background:
                            "linear-gradient(135deg, #7c3aed, #a855f7)",
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          fontSize: 8,
                          fontWeight: 700,
                          color: "white",
                          flexShrink: 0,
                        }}
                      >
                        {deal.initial}
                      </div>
                      <span
                        style={{
                          fontSize: 9,
                          color: "rgba(255,255,255,0.58)",
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {deal.company}
                      </span>
                    </div>
                    <div
                      style={{
                        fontSize: 10.5,
                        fontWeight: 700,
                        color: "white",
                      }}
                    >
                      {deal.value}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div
        style={{
          position: "absolute",
          bottom: -14,
          right: -14,
          borderRadius: 12,
          border: "1px solid rgba(255,255,255,0.07)",
          background: "#1a1a1a",
          padding: "12px 18px",
          boxShadow: "0 20px 40px rgba(0,0,0,0.55)",
          transform: "rotate(1.6deg)",
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
          Open pipeline
        </div>
        <div
          style={{
            fontSize: 26,
            fontWeight: 700,
            color: "white",
            lineHeight: 1,
          }}
        >
          $92K
        </div>
      </div>
    </div>
  );
}

// ─── Lead capture modal (Book a Demo / Join Early Access) ─────────────

function LeadCaptureModal({
  open,
  intent,
  onClose,
}: {
  open: boolean;
  intent: "demo" | "waitlist";
  onClose: () => void;
}) {
  const [selectedIntent, setSelectedIntent] = useState<"demo" | "waitlist">(
    intent,
  );
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [company, setCompany] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);
  const firstFieldRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSelectedIntent(intent);
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setDone(false);
      const t = setTimeout(() => firstFieldRef.current?.focus(), 50);
      return () => clearTimeout(t);
    }
  }, [open, intent]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !email.trim()) return;
    setSubmitting(true);
    try {
      const res = await fetch("/api/lead-capture", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          email: email.trim(),
          company: company.trim(),
          intent: selectedIntent,
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok || !data.success) {
        throw new Error(data.error || "Could not submit right now.");
      }
      setDone(true);
      toast.success(
        selectedIntent === "demo"
          ? "Got it — we'll reach out to schedule."
          : "You're on the list.",
      );
      setTimeout(() => {
        onClose();
        setName("");
        setEmail("");
        setCompany("");
      }, 1800);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Get started with BusinessSAAS"
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 100,
        background: "rgba(0,0,0,0.65)",
        backdropFilter: "blur(4px)",
        WebkitBackdropFilter: "blur(4px)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          width: "100%",
          maxWidth: 420,
          borderRadius: 16,
          border: "1px solid rgba(255,255,255,0.09)",
          background: "#111111",
          padding: 28,
          position: "relative",
          boxShadow: "0 40px 100px rgba(0,0,0,0.7)",
        }}
      >
        <button
          onClick={onClose}
          aria-label="Close"
          type="button"
          style={{
            position: "absolute",
            top: 16,
            right: 16,
            background: "transparent",
            border: "none",
            color: "rgba(255,255,255,0.4)",
            cursor: "pointer",
            padding: 4,
          }}
        >
          <X style={{ width: 18, height: 18 }} />
        </button>

        {done ? (
          <div style={{ textAlign: "center", padding: "24px 0" }}>
            <CheckCircle
              style={{
                width: 32,
                height: 32,
                color: "#a855f7",
                margin: "0 auto 14px",
              }}
            />
            <h3
              style={{
                fontFamily: FONT_SYNE,
                fontSize: 18,
                fontWeight: 700,
                color: "white",
                marginBottom: 6,
              }}
            >
              {selectedIntent === "demo"
                ? "Demo request sent"
                : "You're on the list"}
            </h3>
            <p style={{ fontSize: 13, color: "rgba(255,255,255,0.4)" }}>
              We&apos;ll be in touch at {email}.
            </p>
          </div>
        ) : (
          <>
            <h3
              style={{
                fontFamily: FONT_SYNE,
                fontSize: 19,
                fontWeight: 700,
                color: "white",
                marginBottom: 6,
              }}
            >
              Get started
            </h3>
            <p
              style={{
                fontSize: 13,
                color: "rgba(255,255,255,0.42)",
                marginBottom: 20,
                lineHeight: 1.5,
              }}
            >
              Tell us a little about you — we&apos;ll follow up directly, not
              through a drip campaign.
            </p>

            <div style={{ display: "flex", gap: 8, marginBottom: 18 }}>
              {(["demo", "waitlist"] as const).map((val) => (
                <button
                  key={val}
                  type="button"
                  onClick={() => setSelectedIntent(val)}
                  style={{
                    flex: 1,
                    padding: "9px 10px",
                    borderRadius: 8,
                    fontSize: 12.5,
                    fontWeight: 600,
                    cursor: "pointer",
                    fontFamily: FONT_INTER,
                    border:
                      selectedIntent === val
                        ? "1px solid rgba(124,58,237,0.5)"
                        : "1px solid rgba(255,255,255,0.08)",
                    background:
                      selectedIntent === val
                        ? "rgba(124,58,237,0.14)"
                        : "transparent",
                    color:
                      selectedIntent === val
                        ? "#a855f7"
                        : "rgba(255,255,255,0.42)",
                    transition: "all 0.15s",
                  }}
                >
                  {val === "demo" ? "Book a demo call" : "Just keep me updated"}
                </button>
              ))}
            </div>

            <form
              onSubmit={handleSubmit}
              style={{ display: "flex", flexDirection: "column", gap: 12 }}
            >
              <input
                ref={firstFieldRef}
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                onFocus={focusInput}
                onBlur={blurInput}
                placeholder="Full name"
                style={modalInputStyle}
              />
              <input
                required
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onFocus={focusInput}
                onBlur={blurInput}
                placeholder="Work email"
                style={modalInputStyle}
              />
              <input
                value={company}
                onChange={(e) => setCompany(e.target.value)}
                onFocus={focusInput}
                onBlur={blurInput}
                placeholder="Company (optional)"
                style={modalInputStyle}
              />
              <button
                type="submit"
                disabled={submitting}
                style={{
                  marginTop: 6,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 8,
                  background: PURPLE,
                  color: "white",
                  padding: "12px 20px",
                  borderRadius: 9,
                  fontSize: 14,
                  fontWeight: 600,
                  fontFamily: FONT_INTER,
                  border: "none",
                  cursor: submitting ? "wait" : "pointer",
                  opacity: submitting ? 0.7 : 1,
                }}
              >
                {submitting
                  ? "Sending…"
                  : selectedIntent === "demo"
                    ? "Request demo"
                    : "Join early access"}
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  );
}

// ─── FAQ item ───────────────────────────────────────────────────────

function FAQItem({
  q,
  a,
  isOpen,
  onToggle,
}: {
  q: string;
  a: string;
  isOpen: boolean;
  onToggle: () => void;
}) {
  return (
    <div style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
      <button
        onClick={onToggle}
        type="button"
        style={{
          width: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 16,
          padding: "20px 4px",
          background: "transparent",
          border: "none",
          cursor: "pointer",
          textAlign: "left",
        }}
      >
        <span
          style={{
            fontSize: 15,
            fontWeight: 600,
            color: "white",
            fontFamily: FONT_INTER,
          }}
        >
          {q}
        </span>
        <ChevronDown
          style={{
            width: 16,
            height: 16,
            color: "rgba(255,255,255,0.4)",
            flexShrink: 0,
            transform: isOpen ? "rotate(180deg)" : "rotate(0deg)",
            transition: "transform 0.2s ease",
          }}
        />
      </button>
      <div
        style={{
          maxHeight: isOpen ? 200 : 0,
          overflow: "hidden",
          transition: "max-height 0.3s ease",
        }}
      >
        <p
          style={{
            fontSize: 13.5,
            lineHeight: 1.7,
            color: "rgba(255,255,255,0.42)",
            paddingBottom: 20,
            paddingRight: 32,
          }}
        >
          {a}
        </p>
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

  const [modalOpen, setModalOpen] = useState(false);
  const [modalIntent, setModalIntent] = useState<"demo" | "waitlist">("demo");
  const [openFaq, setOpenFaq] = useState<number | null>(0);

  const openModal = (intent: "demo" | "waitlist") => {
    setModalIntent(intent);
    setModalOpen(true);
  };

  useIsomorphicLayoutEffect(() => {
    let revert: (() => void) | undefined;

    (() => {
      const ctx = gsap.context(() => {
        const ease = "power2.out";
        const toggleActions = "play none none reverse";

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

        const previewVisual =
          document.querySelector<HTMLElement>(".preview-visual");
        if (previewVisual) {
          gsap.fromTo(
            previewVisual,
            { opacity: 0, y: 32, scale: 0.97 },
            {
              opacity: 1,
              y: 0,
              scale: 1,
              duration: 0.7,
              ease,
              scrollTrigger: {
                trigger: previewVisual,
                start: "top 85%",
                toggleActions,
              },
            },
          );
        }

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

        gsap.fromTo(
          ".faq-list > div",
          { opacity: 0, y: 16 },
          {
            opacity: 1,
            y: 0,
            duration: 0.4,
            ease,
            stagger: 0.06,
            scrollTrigger: {
              trigger: ".faq-list",
              start: "top 85%",
              toggleActions,
            },
          },
        );

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
      className={`${syne.variable} ${inter.variable}`}
      style={{
        minHeight: "100vh",
        background: "#0a0a0a",
        color: "white",
        overflowX: "hidden",
        fontFamily: FONT_INTER,
      }}
    >
      <style>{`
        @keyframes soft-pulse {
          0%, 100% { opacity: 1; }
          50%       { opacity: 0.35; }
        }
        .rbac-pulse { animation: soft-pulse 2s ease-in-out infinite; }

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

        .preview-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 40px;
          align-items: center;
        }
        .preview-visual { display: flex; justify-content: center; }
        @media (min-width: 1024px) {
          .preview-grid { grid-template-columns: 1fr 460px; gap: 56px; }
        }

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

        .feature-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 16px;
        }
        @media (min-width: 768px) {
          .feature-grid { grid-template-columns: repeat(3, 1fr); }
        }

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

        .footer-grid {
          display: grid;
          grid-template-columns: 1fr;
          gap: 40px;
        }
        @media (min-width: 768px) {
          .footer-grid { grid-template-columns: 1.6fr repeat(4, 1fr); gap: 40px; }
        }

        .trust-items {
          display: flex;
          flex-wrap: wrap;
          align-items: center;
          justify-content: center;
          gap: 8px 32px;
        }

        .nav-center { display: none; }
        @media (min-width: 900px) {
          .nav-center { display: flex; align-items: center; gap: 2px; }
        }

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

        button:focus-visible, a:focus-visible {
          outline: 2px solid ${PURPLE_HOVER};
          outline-offset: 2px;
        }
        @media (prefers-reduced-motion: reduce) {
          .rbac-pulse { animation: none; }
        }
      `}</style>

      {/* ══════ NAVIGATION ══════ */}
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
                background: PURPLE,
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
                  fontFamily: FONT_SYNE,
                }}
              >
                B
              </span>
            </div>
            <span
              style={{
                fontSize: 15,
                fontWeight: 700,
                letterSpacing: "-0.2px",
                fontFamily: FONT_SYNE,
              }}
            >
              BusinessSAAS
            </span>
          </div>

          <div className="nav-center">
            {NAV_LINKS.map(({ label, href }) => (
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
            <button
              type="button"
              onClick={() => openModal("demo")}
              className="nav-cta"
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                background: PURPLE,
                color: "white",
                padding: "8px 16px",
                borderRadius: 8,
                fontSize: 13,
                fontWeight: 600,
                fontFamily: FONT_INTER,
                border: "none",
                cursor: "pointer",
                boxShadow: "0 0 20px rgba(124,58,237,0.35)",
                transition: "background 0.15s",
              }}
            >
              Book a Demo
              <ArrowRight style={{ width: 13, height: 13 }} />
            </button>
          </div>
        </nav>
      </header>

      {/* ══════ HERO ══════ */}
      <section
        style={{
          position: "relative",
          maxWidth: 1200,
          margin: "0 auto",
          padding: "104px 24px 96px",
        }}
      >
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
          <div style={{ maxWidth: 560 }}>
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
                Two modules live now — CRM &amp; HRM
              </span>
            </div>

            <h1
              ref={headlineRef}
              style={{
                fontFamily: FONT_SYNE,
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
              Consolidate CRM, HRM, and the modules coming next into a single
              secure workspace — with enterprise-grade access control and
              audit-ready infrastructure built in from day one.
            </p>

            <div
              ref={ctaRef}
              style={{
                display: "flex",
                flexWrap: "wrap",
                alignItems: "center",
                gap: 12,
              }}
            >
              <button
                type="button"
                onClick={() => openModal("demo")}
                className="cta-primary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 9,
                  background: PURPLE,
                  color: "white",
                  padding: "14px 26px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 600,
                  fontFamily: FONT_INTER,
                  border: "none",
                  cursor: "pointer",
                  boxShadow: "0 0 28px rgba(124,58,237,0.42)",
                  transition: "background 0.15s",
                }}
              >
                Book a Demo
                <ArrowRight style={{ width: 15, height: 15 }} />
              </button>
              <button
                type="button"
                onClick={() => openModal("waitlist")}
                className="cta-secondary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  padding: "14px 26px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 500,
                  fontFamily: FONT_INTER,
                  border: "1px solid rgba(255,255,255,0.09)",
                  background: "transparent",
                  color: "rgba(255,255,255,0.52)",
                  cursor: "pointer",
                  transition: "border-color 0.15s, color 0.15s",
                }}
              >
                Join Early Access
              </button>
            </div>

            <p
              style={{
                marginTop: 18,
                fontSize: 11,
                color: "rgba(255,255,255,0.18)",
                letterSpacing: "0.02em",
              }}
            >
              No credit card required · We reply personally, not through a bot
            </p>
          </div>

          <div className="hero-visual" ref={visualRef}>
            <PermissionCard />
          </div>
        </div>
      </section>

      {/* ══════ TRUST / CREDIBILITY STRIP ══════ */}
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
            Engineered for trust, not bolted on after
          </p>
          <div className="trust-items">
            {TRUST_BADGES.map((label, i) => (
              <React.Fragment key={label}>
                <span
                  style={{
                    fontSize: 13,
                    fontWeight: 600,
                    color: "rgba(255,255,255,0.24)",
                    whiteSpace: "nowrap",
                  }}
                >
                  {label}
                </span>
                {i < TRUST_BADGES.length - 1 && (
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

      {/* ══════ MODULES ══════ */}
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
              fontFamily: FONT_SYNE,
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
                  fontFamily: FONT_SYNE,
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

      {/* ══════ PRODUCT PREVIEW ══════ */}
      <section
        style={{
          background: "#0c0c0c",
          borderTop: "1px solid rgba(255,255,255,0.04)",
          borderBottom: "1px solid rgba(255,255,255,0.04)",
        }}
      >
        <div style={{ maxWidth: 1200, margin: "0 auto", padding: "96px 24px" }}>
          <div className="preview-grid">
            <div data-reveal>
              <p
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 6,
                  fontSize: 11,
                  fontWeight: 700,
                  textTransform: "uppercase",
                  letterSpacing: "0.1em",
                  color: "rgba(255,255,255,0.22)",
                  marginBottom: 12,
                }}
              >
                <Sparkles style={{ width: 12, height: 12 }} />
                See it in action
              </p>
              <h2
                style={{
                  fontFamily: FONT_SYNE,
                  fontSize: "clamp(26px, 2.8vw, 36px)",
                  fontWeight: 700,
                  lineHeight: 1.2,
                  letterSpacing: "-0.3px",
                  color: "white",
                  marginBottom: 20,
                }}
              >
                A real pipeline, not a screenshot of a promise.
              </h2>
              <p
                style={{
                  fontSize: 15,
                  lineHeight: 1.72,
                  color: "rgba(255,255,255,0.4)",
                  maxWidth: 440,
                }}
              >
                Drag deals across stages, see contact history inline, and let
                round-robin routing assign new leads automatically. This is the
                actual pipeline board — not a mockup for the slide deck.
              </p>
            </div>
            <div className="preview-visual">
              <PipelinePreviewCard />
            </div>
          </div>
        </div>
      </section>

      {/* ══════ STATS ══════ */}
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
                key={stat.label}
                style={{
                  background: "#0c0c0c",
                  padding: "44px 32px",
                  textAlign: "center",
                }}
              >
                <div
                  style={{
                    fontFamily: FONT_SYNE,
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

      {/* ══════ SECURITY ══════ */}
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
            Security &amp; Compliance
          </p>
          <h2
            style={{
              fontFamily: FONT_SYNE,
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
                  fontFamily: FONT_SYNE,
                  fontSize: 15,
                  fontWeight: 700,
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

      {/* ══════ FEATURES ══════ */}
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
              Why BusinessSAAS
            </p>
            <h2
              style={{
                fontFamily: FONT_SYNE,
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
                    fontFamily: FONT_SYNE,
                    fontSize: 15,
                    fontWeight: 700,
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

      {/* ══════ FAQ ══════ */}
      <section
        style={{ maxWidth: 720, margin: "0 auto", padding: "96px 24px" }}
      >
        <div data-reveal style={{ marginBottom: 40, textAlign: "center" }}>
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
            Questions
          </p>
          <h2
            style={{
              fontFamily: FONT_SYNE,
              fontSize: "clamp(24px, 2.6vw, 32px)",
              fontWeight: 700,
              lineHeight: 1.2,
              letterSpacing: "-0.3px",
              color: "white",
            }}
          >
            Before you book a call
          </h2>
        </div>

        <div className="faq-list">
          {FAQS.map((item, i) => (
            <FAQItem
              key={item.q}
              q={item.q}
              a={item.a}
              isOpen={openFaq === i}
              onToggle={() => setOpenFaq(openFaq === i ? null : i)}
            />
          ))}
        </div>
      </section>

      {/* ══════ CTA ══════ */}
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
              Pricing, straight from us
            </p>

            <h2
              style={{
                fontFamily: FONT_SYNE,
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
              We&apos;re still finalizing plans as we onboard early customers,
              so book a demo and we&apos;ll talk pricing directly — or join
              early access if you&apos;d rather just hear when we&apos;re ready
              for you.
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
              <button
                type="button"
                onClick={() => openModal("demo")}
                className="cta-primary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 9,
                  background: PURPLE,
                  color: "white",
                  padding: "14px 28px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 600,
                  fontFamily: FONT_INTER,
                  border: "none",
                  cursor: "pointer",
                  boxShadow: "0 0 32px rgba(124,58,237,0.45)",
                  transition: "background 0.15s",
                }}
              >
                Book a Demo
                <ArrowRight style={{ width: 15, height: 15 }} />
              </button>
              <button
                type="button"
                onClick={() => openModal("waitlist")}
                className="cta-secondary"
                style={{
                  display: "flex",
                  alignItems: "center",
                  padding: "14px 28px",
                  borderRadius: 10,
                  fontSize: 15,
                  fontWeight: 500,
                  fontFamily: FONT_INTER,
                  border: "1px solid rgba(255,255,255,0.09)",
                  background: "transparent",
                  color: "rgba(255,255,255,0.48)",
                  cursor: "pointer",
                  transition: "border-color 0.15s, color 0.15s",
                }}
              >
                Join Early Access
              </button>
            </div>

            <p
              style={{
                fontSize: 11,
                color: "rgba(255,255,255,0.16)",
                letterSpacing: "0.02em",
              }}
            >
              No credit card required · We reply personally, not through a bot
            </p>
          </div>
        </div>
      </section>

      {/* ══════ FOOTER ══════ */}
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
          <div className="footer-grid">
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
                      fontFamily: FONT_SYNE,
                    }}
                  >
                    B
                  </span>
                </div>
                <span
                  style={{
                    fontSize: 14,
                    fontWeight: 700,
                    letterSpacing: "-0.2px",
                    color: "rgba(255,255,255,0.82)",
                    fontFamily: FONT_SYNE,
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
                  maxWidth: 240,
                  marginBottom: 24,
                }}
              >
                Built end-to-end by a single engineer who believes a business
                platform should be trustworthy before it&apos;s flashy — CRM and
                HRM are live, more modules are shipping in the open.
              </p>
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
                    <li key={link.label}>
                      <a
                        href={link.href}
                        className="footer-link"
                        style={{
                          fontSize: 13,
                          color: "rgba(255,255,255,0.34)",
                          textDecoration: "none",
                        }}
                      >
                        {link.label}
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>

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

      <LeadCaptureModal
        open={modalOpen}
        intent={modalIntent}
        onClose={() => setModalOpen(false)}
      />
    </div>
  );
}
