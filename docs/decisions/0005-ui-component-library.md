# ADR-0005: UI library — shadcn/ui + Tailwind CSS v4

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS needs a UI component system that:

- Provides accessible, production-quality components out of the box
- Can be fully customised to match a unique brand identity
- Supports dark and light mode without a heavy theming system
- Has no vendor lock-in — the company must own the components
- Works with TypeScript and React 19
- Is maintainable by a single developer without design expertise

The original plan was to use Fuse React (Envato premium template) as the admin dashboard
foundation. After evaluation, this was reconsidered.

---

## Decision

Use **shadcn/ui** as the component system with **Tailwind CSS v4** as the styling layer.

**shadcn/ui is not a component library in the traditional sense.** Running
`bunx shadcn@latest add button` copies the `Button` component source code directly into
`components/ui/button.tsx`. The component is then owned by this project — not by a package.
There is no `node_modules/shadcn-ui` to update; the component is just TypeScript + Tailwind.

---

## Reasoning

### Why not Fuse React

Fuse React (Envato) is a premium MUI-based template. The fundamental problem is ownership:

- The design system is controlled by a third party
- Updates require purchasing a new license or waiting for the vendor
- Onboarding a new developer requires them to buy access
- MUI's CSS-in-JS approach conflicts with Server Components — MUI v7 partially addresses this
  but the integration is still complex
- Customising away from MUI's design language requires fighting the library
- The template ships with hundreds of demo pages — dead code in production

At the scale BusinessSAAS aims for, the frontend design system must be owned entirely.

### Why shadcn/ui

Components are copied into the codebase. Ownership is total. The source is readable TypeScript
that any React developer can understand and modify.

Built on **Radix UI primitives** — headless, fully accessible components (WCAG 2.1 AA) with
correct ARIA roles, keyboard navigation, and focus management. Accessibility is not bolted on;
it is the foundation.

Used in production by: Vercel, Linear, Resend, Clerk, Raycast, and hundreds of Series A+ SaaS
products. The pattern is proven at scale.

`bunx shadcn@latest add <component>` installs only the specific component needed. The bundle
grows only with what is used.

### Why Tailwind CSS v4

Tailwind v4 rewrites the engine in Rust (via Lightning CSS) and introduces:

- **CSS-first configuration:** Design tokens are defined in `@theme` inside a `.css` file,
  not in `tailwind.config.js`. This makes tokens available as native CSS variables everywhere.
- **Native cascade layers:** `@layer base`, `@layer components`, `@layer utilities` are now
  proper CSS cascade layers, not Tailwind-invented concepts.
- **Better dark mode:** `dark:` variant works with CSS `@media (prefers-color-scheme)` and
  the `class` strategy simultaneously, enabling the user-controlled theme toggle.
- **Smaller output:** The Rust engine produces smaller CSS with better purging.

Design tokens defined in Tailwind v4 are automatically available as CSS variables, so
shadcn/ui's component defaults (`bg-background`, `text-foreground`, etc.) work without
any extra configuration.

---

## Component installation pattern

```bash
# Add a component — this copies source into components/ui/
bunx shadcn@latest add dialog
bunx shadcn@latest add data-table
bunx shadcn@latest add command

# Components live at:
components/ui/dialog.tsx
components/ui/data-table.tsx
components/ui/command.tsx
```

Components are never imported from a package — always from `@/components/ui/*`.

---

## Design token convention

All design tokens are defined in `app/globals.css` using Tailwind v4's `@theme` block:

```css
@theme {
  --color-background: #ffffff;
  --color-foreground: #0a0a0a;
  --color-primary: #4f46e5;
  --color-primary-foreground: #ffffff;
  --color-muted: #f4f4f5;
  --color-muted-foreground: #71717a;
  /* ... */
}

.dark {
  --color-background: #0a0a0a;
  --color-foreground: #fafafa;
  /* ... */
}
```

This means dark mode is pure CSS — no JavaScript required to switch themes correctly.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Fuse React (Envato) | Vendor lock-in, MUI overhead, license per developer, fights Server Components |
| Material UI v7 | Opinionated Google aesthetic, hard to brand away from, CSS-in-JS complexity |
| Ant Design | Heavy bundle (~1MB), inconsistent with modern SaaS aesthetics |
| Chakra UI | Good DX but slower than Tailwind, more opinions about layout |
| Mantine | Excellent but adds its own style system that conflicts with Tailwind |
| Build from scratch | Months of work, no accessibility guarantees without expert knowledge |
| Headless UI only | Too minimal — shadcn/ui is already built on Headless UI (Radix) with better DX |

---

## Consequences

**Positive:**
- Zero vendor dependency — components are owned by this project
- Radix primitives provide accessibility guarantees
- Dark mode is CSS-native, no flash, no JavaScript required
- Adding a component is one command; only used components are in the bundle
- TypeScript-first — all component props are fully typed

**Negative:**
- Components must be individually chosen and added — no complete kit on day one
- Customising a component means editing the copied source — can diverge from upstream
- Tailwind v4 is newer; some tutorials still reference v3 syntax (`tailwind.config.js`)
- Developers unfamiliar with Tailwind have a learning curve (typically 1–2 days)

---

## Related decisions

- [ADR-0004](0004-frontend-framework.md) — Next.js App Router; components work with RSC
- [ADR-0013](0013-dark-mode.md) — dark/light theme toggle uses next-themes + Tailwind v4 CSS variables
