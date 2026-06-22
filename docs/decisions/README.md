# Architecture Decision Records

This directory contains all Architecture Decision Records (ADRs) for BusinessSAAS.

An ADR documents a significant architectural decision: what was decided, why, what alternatives
were considered, and what the consequences are. Once accepted, an ADR is never edited — if the
decision changes, a new ADR is written that supersedes it.

## How to read these

Read them in order when onboarding. Each ADR is self-contained but cross-references related ones.

## How to write a new ADR

Copy `0000-template.md`, increment the number, fill in every section. Status must be one of:
`Proposed` → `Accepted` → `Superseded by ADR-XXXX` or `Deprecated`.

---

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [0001](0001-backend-framework.md) | Backend framework: Go + Fiber v3 | Accepted | 2025-06-22 |
| [0002](0002-database-and-cache.md) | Database: PostgreSQL 16 + Redis 7 | Accepted | 2025-06-22 |
| [0003](0003-auth-token-strategy.md) | Auth: JWT access + opaque refresh tokens | Accepted | 2025-06-22 |
| [0004](0004-frontend-framework.md) | Frontend framework: Next.js 15 App Router | Accepted | 2025-06-22 |
| [0005](0005-ui-component-library.md) | UI library: shadcn/ui + Tailwind CSS v4 | Accepted | 2025-06-22 |
| [0006](0006-token-storage-strategy.md) | Token storage: memory-only access token | Accepted | 2025-06-22 |
| [0007](0007-state-management.md) | State management: TanStack Query + Zustand | Accepted | 2025-06-22 |
| [0008](0008-multi-tenancy-url-structure.md) | Multi-tenancy: path-based URL routing | Accepted | 2025-06-22 |
| [0009](0009-form-validation.md) | Forms: React Hook Form + Zod | Accepted | 2025-06-22 |
| [0010](0010-package-manager.md) | Package manager: Bun | Accepted | 2025-06-22 |
| [0011](0011-data-tables.md) | Data tables: TanStack Table v8 | Accepted | 2025-06-22 |
| [0012](0012-error-handling-strategy.md) | Error handling: three-layer strategy | Accepted | 2025-06-22 |
| [0013](0013-dark-mode.md) | Theme: dark/light mode with next-themes | Accepted | 2025-06-22 |
