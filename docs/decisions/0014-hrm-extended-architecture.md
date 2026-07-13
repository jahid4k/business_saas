# ADR-0014: HRM Extended Architecture — config-first group dependency chain

**Date:** 2026-07-07
**Status:** Accepted
**Deciders:** Mridha

---

## Context

The original HRM Phase 1 backend covers departments, positions, employees, leave types, leave
requests, and reports — 31 routes across 5 sub-domains. This handles the basics.

The extended HRM scope adds payslips, attendance, warnings, complaints, transfers, promotions,
resignations, terminations, contracts, documents, acknowledgements, announcements, events, awards,
and employee milestones — roughly 22 additional migrations and 100+ routes.

These sub-modules are not independent. A warning record needs a warning *type* (config).
A payslip needs salary *components* (config) and a finalized attendance *period* (runtime).
A resignation needs a *contract* (data) to auto-compute the notice period.

The core design question was: do we build these sub-modules in any order, or impose a
dependency-driven sequence?

---

## Decision

Structure the extended HRM build into **five dependency-ordered groups (A through E)**,
where later groups may only be started after earlier groups are fully implemented and locked.

```
Group A — Setup / Config (migrations 00021–00027)
  A1. Salary component engine
  A2. Approval chain templates
  A3. Warning type configuration
  A4. Document templates
  A5. Work schedule and shift configuration
  A6. Holiday calendar management
  A7. Employee contracts

Group B — Core Employee Lifecycle (migrations 00028–00031)
  B1. Promotions          → uses A1 (salary record), A2 (approval), A4 (letter)
  B2. Transfers           → uses A2 (approval), A4 (letter)
  B3. Resignations        → uses A7 (notice period), A4 (acceptance letter)
  B4. Terminations        → uses A2 (approval), A4 (termination letter)

Group C — Disciplinary and Compliance (migrations 00032–00034)
  C1. Warning records     → uses A3 (type config), A2 (approval), A4 (letter)
  C2. Complaints          → uses A4 (resolution letter)
  C3. Employee documents  → uses A4 (template engine)
  C4. Acknowledgements    → cross-cutting: used by C1, C3, E2, E3

Group D — Time and Compensation (migrations 00035–00038)
  D1. Attendance          → uses A5 (shift), A6 (holidays), A2 (regularization approval)
  D2. Payslips            → uses A1 (formula engine), D1 (attendance period, must finalize first)

Group E — Recognition and Communication (migrations 00039–00042)
  E1. Awards              → uses A2 (approval), A4 (certificate), E2 (announcement)
  E2. Announcements       → uses C4 (acknowledgement)
  E3. HR Calendar         → uses C4 (RSVP as acknowledgement)
  E4. Employee Milestones → uses A7 (contract dates), E1 (auto-award), E2 (auto-announce)
```

No group's implementation begins until the previous group is deployed and tested.

---

## Reasoning

### Why group at all

If we built sub-modules in arbitrary order, we would encounter this situation repeatedly:
a service implementation references a table or type that does not exist yet in the database,
because the config layer has not been built. Grouping eliminates this class of error entirely.

### Why A before everything

Group A contains the configuration primitives that all other groups reference. Without salary
components (A1), there is nothing for the payslip engine (D2) to compute. Without approval
templates (A2), the resignation (B3), transfer (B2), and warning (C1) flows have no way to
route for approval. Building A first is not a technical constraint — it is a product constraint.
HR cannot configure the system without A, and the system cannot function without HR configuration.

### Why the internal order within A

Within Group A, the sub-modules have their own dependency order:
- A1, A2, A5, A6 are independent — they reference only core tables (organizations, users, employees)
- A4 must come after A2 — warning types (A3) reference both A2 (approval template) and A4
  (document template), so A4 must exist before A3's FK can be satisfied
- A7 (contracts) references A1 (salary structure) and A4 (document), so it is last

This is reflected in the migration numbering: 00021 (A1) → 00022 (A2) → 00024 (A4) → 00023 (A3)
— note A4 before A3 despite the label order, specifically to resolve the FK dependency.

### Why B before C, D, and E

Group B adds the core employee lifecycle events (promotion, transfer, resignation, termination).
These are the most commonly needed features after basic CRUD, and they are relatively simple —
they use A's config but do not depend on each other within B. Building B before the disciplinary
and compensation layers means the most important operational HR flows are live before the complex
formula-based payroll computation.

### Why D2 (payslip) is last within D

The payslip engine has the strictest dependency: it requires the attendance period for the
relevant month to be finalized (locked) before a payroll run can be created. This is enforced
at the service layer — `CreatePayrollRun` returns an error if the attendance period is still
open. D1 (attendance) must therefore be built, deployed, and producing finalized periods before
D2 is useful.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Build all sub-modules in a flat list, any order | FK constraint violations in migrations; circular service dependencies; runtime errors when config tables are missing |
| Build by feature area (all payroll-adjacent together) | Payslip depends on attendance which depends on shifts which depends on nothing — the "feature area" grouping would still need internal ordering, making it identical to this approach |
| Build everything in one giant migration | Unacceptable — a single 42-table migration is impossible to debug, impossible to roll back partially, and blocks other work during the build |
| Build config and runtime in parallel | Runtime code cannot be tested without config data; integration tests would be impossible before Group A is complete |

---

## Consequences

**Positive:**
- Each group can be deployed independently — Group A adds real value to HR admins (configuring
  salary structures, warning types, approval chains) before any Group B code exists
- Migrations can be run incrementally — the DB never enters an invalid state
- Service-level dependencies are explicit — `approvals.Service` is injected into `salary.Service`
  only in Group D2, not in A1, which prevents circular imports
- Testing is tractable — Group A unit tests use no external dependencies; Group B tests mock
  only the Group A service interfaces

**Negative:**
- A full HRM deployment takes multiple sprints; early sprints deliver config-only value with no
  runtime behaviour visible to employees
- The group ordering must be documented and maintained; if a developer adds a Group C feature that
  references a Group D table, the constraint is violated silently unless enforced by code review

---

## Related decisions

- [ADR-0015](0015-hrm-formula-engine.md) — Formula engine choice for salary components (A1)
- [ADR-0016](0016-hrm-approval-chains.md) — Approval chain design decisions (A2)
- [ADR-0017](0017-hrm-payslip-engine.md) — Payslip computation design (D2)
- [ADR-0018](0018-hrm-attendance-sources.md) — Attendance multi-source architecture (D1)
- [ADR-0019](0019-hrm-document-templates.md) — Document template engine decisions (A4)
