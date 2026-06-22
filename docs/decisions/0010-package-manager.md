# ADR-0010: Package manager — Bun

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

The frontend needs a package manager for installing dependencies, running scripts, and managing
lockfiles. The choice affects developer experience (install speed), CI time, and the lockfile
format committed to the repository.

---

## Decision

Use **Bun** as the package manager for the frontend.

All package installation commands use `bun add` / `bun remove`. Scripts run with `bun run`.
The lockfile is `bun.lock` (committed to the repository).

Node.js 20+ is still required for Next.js itself — Bun is used as the package manager and
script runner, not as the Node.js replacement runtime for production.

---

## Reasoning

### Speed

Bun installs packages 10–25x faster than npm and 3–5x faster than pnpm. On a cold install
(no cache), a typical Next.js SaaS project with 80+ dependencies installs in under 10 seconds
with Bun vs 60–90 seconds with npm.

In CI, this directly reduces pipeline time and cost.

### Unified toolchain

Bun includes a test runner, bundler, and TypeScript runner natively. Even though this project
uses Next.js's built-in bundler and next's test setup, having a fast TypeScript runner available
(`bun run script.ts`) without additional setup is occasionally useful for one-off tooling scripts.

### Full npm compatibility

Bun is fully compatible with the npm registry and `package.json`. All npm packages install and
work correctly. There is no ecosystem fragmentation concern.

### Developer experience

`bun add shadcn` is equivalent to `npm install shadcn` with identical semantics. Any developer
familiar with npm can use Bun without a learning curve — the commands are the same.

---

## Lockfile policy

`bun.lock` is committed to the repository. This ensures:
- Deterministic installs across all machines and CI environments
- The exact dependency versions are reviewable in code review
- No version drift between developer machines

Do not commit both `bun.lock` and `package-lock.json`. Use one or the other. If a contributor
accidentally runs `npm install` and creates a `package-lock.json`, delete it and run `bun install`.

---

## shadcn/ui integration

shadcn/ui's CLI is invoked via `bunx`:

```bash
bunx shadcn@latest init
bunx shadcn@latest add button
bunx shadcn@latest add dialog
bunx shadcn@latest add data-table
```

`bunx` is equivalent to `npx` — it runs a package binary without installing it globally.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| npm | Slowest; `package-lock.json` is large and noisy in diffs |
| yarn (v1 classic) | Maintenance mode; `yarn.lock` format is outdated |
| yarn (v3/4 PnP) | PnP is incompatible with some tools; complex setup |
| pnpm | Faster than npm, excellent workspace support; Bun is faster still; either would be fine |

---

## CI configuration

GitHub Actions CI uses Bun:

```yaml
- name: Install Bun
  uses: oven-sh/setup-bun@v1
  with:
    bun-version: latest

- name: Install dependencies
  run: bun install --frozen-lockfile
  working-directory: frontend
```

`--frozen-lockfile` ensures CI fails if `bun.lock` is out of sync with `package.json`, catching
cases where a developer forgot to commit the updated lockfile.

---

## Consequences

**Positive:**
- Dramatically faster `bun install` vs npm — saves time every day
- CI pipeline faster — direct cost reduction
- Drop-in replacement; no learning curve for npm users

**Negative:**
- Some edge-case package incompatibilities with Bun's Node.js implementation (rare; can be worked around)
- `bun.lock` format differs from `package-lock.json` — contributors must use Bun, not npm
- Bun is not yet at 1.0 stability for all features (the package manager is stable; the runtime less so)

---

## Related decisions

- [ADR-0004](0004-frontend-framework.md) — Next.js 15; Bun manages its dependencies
- [ADR-0005](0005-ui-component-library.md) — shadcn/ui installed via `bunx shadcn@latest add`
