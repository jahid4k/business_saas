import Link from "next/link";

// ============================================================
// BusinessSAAS — Root Landing Page
// app/page.tsx
// ============================================================
// Public marketing page. No auth check here — the user lands
// here first and chooses to sign in or sign up.
// Auth redirects live inside the protected (app) routes.
// ============================================================

const MODULES = [
  {
    icon: "🤝",
    name: "CRM",
    description:
      "Manage leads, deals, pipelines, and customer relationships in one place.",
    color: "bg-emerald-50",
    status: "live" as const,
  },
  {
    icon: "👥",
    name: "HRM",
    description:
      "Departments, employees, leave management, payroll, and org charts.",
    color: "bg-violet-50",
    status: "soon" as const,
  },
  {
    icon: "📊",
    name: "Accounting",
    description:
      "Invoices, expenses, bank reconciliation, and financial reporting.",
    color: "bg-amber-50",
    status: "soon" as const,
  },
  {
    icon: "✅",
    name: "Projects",
    description:
      "Track tasks, milestones, and team workload across every project.",
    color: "bg-blue-50",
    status: "soon" as const,
  },
  {
    icon: "🛒",
    name: "E-commerce",
    description:
      "Product catalog, orders, customers, and inventory from one dashboard.",
    color: "bg-orange-50",
    status: "soon" as const,
  },
  {
    icon: "🎓",
    name: "Learning",
    description:
      "Internal training, course management, and employee progress tracking.",
    color: "bg-green-50",
    status: "soon" as const,
  },
];

export default function HomePage() {
  return (
    <div className="min-h-screen bg-stone-50 text-stone-900">
      {/* ── Navigation ─────────────────────────────────────── */}
      <header className="sticky top-0 z-20 border-b border-stone-200 bg-stone-50/90 backdrop-blur-sm">
        <nav className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          {/* Logo */}
          <div className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full bg-emerald-500" />
            <span className="text-[17px] font-semibold tracking-tight">
              BusinessSAAS
            </span>
          </div>

          {/* Nav links */}
          <div className="flex items-center gap-1">
            <a
              href="#modules"
              className="rounded-md px-3 py-2 text-sm font-medium text-stone-500 transition-colors hover:bg-stone-100 hover:text-stone-900"
            >
              Features
            </a>
            <Link
              href="/login"
              className="rounded-md px-3 py-2 text-sm font-medium text-stone-500 transition-colors hover:bg-stone-100 hover:text-stone-900"
            >
              Sign in
            </Link>
            <Link
              href="/signup"
              className="ml-2 rounded-lg bg-stone-900 px-4 py-2 text-sm font-medium text-stone-50 transition-colors hover:bg-stone-700"
            >
              Get started →
            </Link>
          </div>
        </nav>
      </header>

      {/* ── Hero ───────────────────────────────────────────── */}
      <section className="mx-auto max-w-5xl px-6 pb-20 pt-24">
        {/* Eyebrow badge */}
        <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-semibold uppercase tracking-wider text-emerald-700">
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
          Now in beta
        </div>

        {/* Headline */}
        <h1 className="mb-5 max-w-2xl text-[56px] font-semibold leading-[1.08] tracking-[-1.5px] text-stone-900">
          Run your business
          <br />
          on one <span className="text-emerald-600">platform</span>
        </h1>

        {/* Subheading */}
        <p className="mb-9 max-w-lg text-lg leading-relaxed text-stone-500">
          CRM, HRM, Accounting, and Project Management — unified under one roof,
          with a permission model that keeps every team working in the right
          context.
        </p>

        {/* CTAs */}
        <div className="flex flex-wrap items-center gap-3">
          <Link
            href="/signup"
            className="rounded-lg bg-stone-900 px-7 py-3 text-[15px] font-medium text-stone-50 transition-colors hover:bg-stone-700"
          >
            Start for free
          </Link>
          <Link
            href="/login"
            className="rounded-lg border border-stone-200 bg-white px-7 py-3 text-[15px] font-medium text-stone-900 transition-colors hover:border-stone-300 hover:bg-stone-50"
          >
            Sign in to your workspace
          </Link>
        </div>
      </section>

      {/* ── Module Grid ────────────────────────────────────── */}
      <section
        id="modules"
        className="border-t border-stone-200 bg-white py-16"
      >
        <div className="mx-auto max-w-5xl px-6">
          <p className="mb-10 text-xs font-semibold uppercase tracking-widest text-stone-400">
            Modules
          </p>

          <div className="grid grid-cols-1 gap-px bg-stone-200 sm:grid-cols-2 lg:grid-cols-3 rounded-xl overflow-hidden border border-stone-200">
            {MODULES.map((mod) => (
              <div key={mod.name} className="bg-white p-7">
                {/* Icon */}
                <div
                  className={`mb-4 flex h-9 w-9 items-center justify-center rounded-lg text-lg ${mod.color}`}
                >
                  {mod.icon}
                </div>

                {/* Title */}
                <h3 className="mb-1.5 text-sm font-semibold text-stone-900">
                  {mod.name}
                </h3>

                {/* Description */}
                <p className="text-[13px] leading-relaxed text-stone-400">
                  {mod.description}
                </p>

                {/* Status pill */}
                {mod.status === "live" ? (
                  <span className="mt-3 inline-block rounded text-[10px] font-semibold uppercase tracking-wider bg-emerald-50 px-2 py-0.5 text-emerald-700">
                    Live
                  </span>
                ) : (
                  <span className="mt-3 inline-block rounded text-[10px] font-semibold uppercase tracking-wider bg-stone-100 px-2 py-0.5 text-stone-400">
                    Coming soon
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── CTA Strip ──────────────────────────────────────── */}
      <section className="bg-stone-900 py-20 text-center">
        <h2 className="mb-3 text-4xl font-semibold tracking-tight text-stone-50">
          One workspace. Every tool.
        </h2>
        <p className="mb-8 text-base text-stone-400">
          Set up your organization in minutes. Roles and permissions are baked
          in from day one.
        </p>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Link
            href="/signup"
            className="rounded-lg bg-stone-50 px-7 py-3 text-[15px] font-medium text-stone-900 transition-colors hover:bg-white"
          >
            Create free account
          </Link>
          <Link
            href="/login"
            className="rounded-lg border border-stone-700 px-7 py-3 text-[15px] font-medium text-stone-50 transition-colors hover:border-stone-500"
          >
            Sign in
          </Link>
        </div>
      </section>

      {/* ── Footer ─────────────────────────────────────────── */}
      <footer className="border-t border-stone-800 bg-stone-900 px-6 py-6">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <p className="text-sm text-stone-500">
            © {new Date().getFullYear()} BusinessSAAS. All rights reserved.
          </p>
          <p className="text-sm text-stone-600">Built with Go + Next.js</p>
        </div>
      </footer>
    </div>
  );
}
