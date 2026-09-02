# HRM EXTENDED — BUILD PLAN

> ⚑ **ONE OF TWO MODULE PLANS.** The mission is **CRM + HRM → deploy → business**
> (`docs/Project_Instruction.md` § Mission). Its companion is `docs/CrmExtendedBuildPlan.md`.
>
> ⚠ **HRM backend complete ≠ product complete.** Phases 0–11 are done and tested, but the HRM
> *frontend* covers roughly Phases 0–3 only — ~37 backend packages have no dedicated UI — and CRM
> is not yet production-grade. Neither module ships alone.

> Source-audited 2026-07-29 against `business_saas-develop.zip`.
> This plan supersedes assumptions in `docs/Project_Instruction.md` r15 → Section 9 →
> HRM EXTENDED MODULES wherever the two disagree. Source wins.
> Status vocabulary: ⚪ NOT STARTED / 🔵 ACTIVE / ✅ DONE

---

## 0. AUDIT RESULTS — what the source actually says

Verified directly, not from documentation.

### Resolved TRUE (assumptions that held)

| Claim                             | Verdict    | Evidence                                                                                 |
| --------------------------------- | ---------- | ---------------------------------------------------------------------------------------- |
| Reporting manager chain exists    | ✅ **YES** | `hrm_employees.manager_id` UUID, self-FK, `idx_hrm_emp_manager_id` (00021)               |
| Salary records are temporal       | ✅ **YES** | `hrm_employee_salary_records.effective_date` + `-- No updated_at — append-only` (00023)  |
| Money is NUMERIC in Postgres      | ✅ **YES** | `NUMERIC(15,2)` / `(18,2)` / `(15,4)` throughout; zero `float`/`real`/`double precision` |
| Approval engine is reusable       | ✅ **YES** | `RegisterCallback(entityType, fn)`, `FindDefault`, `CreateInstance`, `Decide`            |
| Acknowledgements are polymorphic  | ✅ **YES** | `acknowledgeable_type` + `acknowledgeable_id` + `entity_title` snapshot (00038)          |
| Doc templates anticipate ATS/Exit | ✅ **YES** | CHECK already contains `offer_letter`, `experience_letter` (00026)                       |

**Consequence:** PREP MIGRATIONS #1 (`reporting_manager_id`) is **cancelled** — the column
exists under the name `manager_id`. Update `docs/Project_Instruction.md` accordingly.
Compensation's temporal foundation is also already correct; Batch 3's largest risk is void.

### Resolved FALSE / newly found

| Finding                                  | Severity    | Evidence                                                                                                                                                                                      |
| ---------------------------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Go money is `float64` everywhere**     | 🔴 Critical | `payslips/model.go:38-94`, `salary/model.go:86-235`; `expr.Compile(..., expr.AsFloat64())` in `salary/service.go:432,437` and `payslips/service.go:416`. No `shopspring/decimal` in `go.mod`. |
| **No scheduler / no cron exists at all** | 🔴 Critical | Only `go func()` in `main.go:587` is the Fiber listener. `milestones.GenerateUpcoming` is a manual endpoint. Doc's "Nightly crons for milestones/absences" is **false**.                      |
| **No notification system**               | 🔴 Critical | `internal/platform/` = `contacts` + `engagement` only                                                                                                                                         |
| **Leave has no balance engine**          | 🟠 High     | `hrm_leave_types` has only `max_days_per_year`. No accrual, carry-forward, encashment, or balance table. F&F leave encashment is not computable.                                              |
| Migration 00065 already consumed         | 🟡 Note     | `00065_update_tasks_context_and_permissions.sql`. Capture Fix Pass A's `created_by` migration is now **00066**.                                                                               |
| Structural guard tests absent            | 🟡 Note     | No `permissions_test.go` / `routing_test.go` / `hygiene_test.go` anywhere                                                                                                                     |
| Count drift                              | 🟡 Note     | 65 migrations · 74 tables · **233 HRM routes** (doc: 205) · **475 total routes** (doc: 391+)                                                                                                  |

### Not changed

Module path is still `github.com/mridha/businesssaas`. No Havelio rename in source.
Mobile scaffold exists (`mobile/src/{app,components,stores,theme,lib,hooks,types}`) — further
along than r15 claims. Frontend HRM pages all present (12 route groups).

---

## 1. PLAN SHAPE

Thirteen modules across five batches of design produced one structural fact: **the modules are
not the work.** Eight shared primitives are, and every module is a thin consumer on top.

So this plan is ordered by dependency, not by module appeal. Three rules govern it:

1. **Nothing speculative.** A primitive gets built when its first real consumer is queued next,
   never before. Checklist engine and form engine therefore appear late, not in Phase 0.
2. **One phase fully shipped before the next starts.** No parallel branches.
3. **Each phase ends with the doc updated** — Section 5 entry, Section 8 entry, migration count,
   route count. Drift has appeared in every audit so far; the fix is same-day updates.

**Total scope: 11 phases.** Phases 0–2 are repair and foundation, and none of them are HRM
features — they are what makes HRM features possible. Phases 3–10 are the modules.

---

## PHASE 0 — Foundation repairs

Nothing in HRM Extended can be built correctly on top of the current state. Three defects,
all pre-existing, all worsening with every feature added.

### 0.1 Money: `float64` → `decimal` 🔴

**Why first:** every subsequent compensation feature multiplies the blast radius. Salary
revisions, bonus, loans, statutory, F&F, expense claims — all money. Fixing after those exist
means touching six modules instead of two.

- Add `github.com/shopspring/decimal` to `go.mod`
- `payslips/model.go` + `salary/model.go`: `float64` → `decimal.Decimal` on every money field
  (keep `float64` for `overtime_hours`, `work_hours_per_week`, `total_days` — those are genuinely
  fractional quantities, not money)
- pgx v5 scans `NUMERIC` → `decimal.Decimal` natively; verify each `rows.Scan` site
- `expr` evaluation: `expr.AsFloat64()` cannot produce decimals. **Decision: keep `expr` on
  float64 for formula evaluation, convert at the boundary** — parse inputs as float, evaluate,
  then `decimal.NewFromFloat(result).Round(orgRoundingScale)` immediately on exit. A formula
  engine that is decimal end-to-end means writing a custom evaluator; not worth it. What matters
  is that _storage and accumulation_ are decimal, and that rounding happens once at a defined
  point rather than drifting across additions.
- Add `org_settings.money_rounding_scale` + `money_rounding_mode` (org-level, per Section 10)
- Widen the two `NUMERIC(15,2)` payslip totals to `NUMERIC(18,4)` for consistency with
  `fixed_value`/`override_value` which are already `(15,4)`

**Deliverable:** migration 00066, `go.mod` + `go.sum`, two model files, two service files,
updated `payslips` unit tests (they exist — `internal/tests/unit/payslips`).

### 0.2 Scheduler registry (`internal/platform/scheduler/`) 🔴

**Why:** eleven of thirteen extended modules need it. Two _already-shipped_ functions are
waiting for it (`milestones.GenerateUpcoming`, the absence sweep) and currently never run.

- `platform_scheduled_jobs` — job_name (unique), cron_expr, is_enabled, last_run_at,
  next_run_at, last_status, consecutive_failures
- `platform_job_runs` — append-only: job_name, started_at, finished_at, status, error, items_processed
- Redis distributed lock (`SET NX PX`) keyed on job name — multi-instance safe, matches the
  existing Redis usage in rate limiting
- In-process ticker in `main.go` (not a separate binary — Docker Compose runs one backend
  container; revisit if that changes)
- Go API: `scheduler.Register(name, cronExpr, fn)` called during DI wiring, mirroring how
  `approvals.RegisterCallback` already works
- Admin routes: list jobs, run-now, view run history — `platform.scheduler.view` / `.manage`
- **Migrate the two orphaned HRM crons onto it immediately** — that is the acceptance test

### 0.3 Notification system (`internal/platform/notifications/`) 🔴

- `platform_notifications` — org_id, user_id, event_type, title, body, related_type,
  related_id, read_at, delivered_channels[]
- `platform_notification_preferences` — user_id × event_type × channel, opt-out matrix
- Channel abstraction: `Channel` interface with `InApp` implemented now, `Email` stubbed
- Digest batching deferred until a consumer asks for it
- Routes: list, mark-read, mark-all-read, preferences get/patch
- Frontend: bell icon in Topbar + notification drawer (dashboard shell already has
  `DrawerContext`)

**⚠️ EMAIL SENDING is a hard dependency for the email channel.** In-app-only ships now; the
email channel stays stubbed until the transactional provider lands (Section 9). Recruitment
(Phase 4) is the first module that genuinely cannot ship without it — decide the provider
before Phase 4 starts, not during.

### 0.4 Prep migrations (cheap now)

- `legal_entity_id` on org-scoped HRM tables, auto-populated with one default entity per org,
  **zero logic written**
- `currency` CHAR(3) alongside every money column, defaulted from an org setting
- ~~`reporting_manager_id`~~ — **cancelled, `manager_id` exists**

### 0.5 Structural guard tests

Memory said these were written; they are not in the repo. Build them now — Phase 1's permission
work is exactly the bug class they catch.

- `permissions_test.go` — every permission string referenced in any `RequirePermission(...)`
  call must exist in a seed migration, and vice versa. (`capture.visitors.view` /
  `crm.view` mismatch is the canonical failure.)
- `routing_test.go` — no collection route registered as `.Get("/")` under `StrictRouting: true`
- `hygiene_test.go` — no AI-conversation comment artifacts in source

---

## PHASE 1 — Resource-level permissions

Already ⚪ in Section 9; promoted here because Performance, Compensation, Helpdesk, and
Succession all leak sensitive data without it. Highest severity-of-failure primitive in the
system.

- Three-tier convention applied consistently: `<module>.<resource>.view_own` /
  `.view_team` / `.view_all`
- `view_team` resolves through `hrm_employees.manager_id` — **available today**, recursive CTE
  for multi-level, direct-report-only as the default
- Scope resolution happens in the **repository layer** as a query predicate, never as a
  post-filter in the service — post-filtering leaks through counts and pagination
- Pattern: `scopeUserID *string` parameter on list/get repository functions, same shape as the
  task visibility scoping already queued
- Field-level filtering (needed by Succession in Phase 10) designed now, implemented then
- **Needs its own ADR** — this touches every repository in the codebase

---

## PHASE 2 — Leave engine upgrade

Not on the original thirteen — the audit found it. Shipped HRM has leave _requests_ but no
leave _balance_. That is a hole in the core product, and F&F (Phase 9) cannot compute
encashment without it.

- `hrm_leave_policies` — per leave type: accrual method (monthly/annual/on_joining),
  accrual rate, carry-forward cap, encashable flag, encashment rate basis
- `hrm_leave_balances` — effective-dated per employee × leave type × period:
  opening, accrued, taken, encashed, carried_forward, closing
- `hrm_leave_transactions` — append-only ledger; balance is derived, never stored as a
  mutable current value
- Accrual job → Phase 0.2 scheduler
- Year-end carry-forward job → scheduler
- Backfill: existing `hrm_leave_requests` replayed into the transaction ledger

---

## PHASE 3 — Checklist engine + Onboarding

First real consumer arrives, so the primitive gets built now.

**`internal/platform/checklists/`**

- `platform_checklist_templates` — `checklist_type` discriminator
  (`onboarding` / `offboarding` / `probation_confirmation` / `transfer_handover`)
- `platform_checklist_template_items` — title, `owner_type`, `due_offset_days`, `is_blocking`,
  `requires_attachment`, optional `blocking_amount_allowed`
- `platform_checklist_instances` / `_instance_items` — assignee resolved **at instantiation**,
  never a template-stored user_id
- `owner_type` resolution: `subject` / `manager` (via `manager_id`) / `hr` (role holders) /
  `it` / `finance` / `custom_role`

**HRM Onboarding** — thin consumer: instantiate on employee create with joining_date as the
offset anchor, completion %, reminders via Phase 0.3 notification.

---

## PHASE 4 — Recruitment / ATS ✅ DONE (internal-only scope; see note below)

> **Actually shipped, 2026-08-05:** split into two sub-phases during planning (this doc's
> undivided description below is the original plan, kept for reference). **Phase 4A** (r20):
> `hrm_recruitment_pipelines`/`_stages`, `hrm_job_requisitions`, `hrm_job_postings`,
> `hrm_candidates`, `hrm_applications`, `hrm_application_stage_history`. **Phase 4B** (r21):
> `hrm_interviews`/`_panelists`, `hrm_interview_scorecards`, `hrm_offers`, `hrm_referrals`,
> hire→employee conversion. Scorecard visibility and hire→employee conversion match this
> section's design intent exactly (see below). **Not shipped, and NOT a soft dependency**: the
> public `/pub/careers/*` surface and candidate email, both still blocked on EMAIL SENDING and
> Capture Fix Pass B — verified unresolved at Phase 4A planning time and re-verified still
> unresolved at Phase 4B completion (`RESEND_API_KEY` absent from every env file;
> `NewPublicCaptureRateLimit` does not exist). Building an unauthenticated public apply endpoint
> ahead of its own security prerequisites was rejected both times. See `docs/Project_Instruction.md`
> r20/r21 changelog entries for the full audit and implementation detail.

**Blocked until EMAIL SENDING ships** *(for the public surface only — see the shipped-scope note
above; the internal-only ATS itself is not blocked on this)*. An ATS without candidate email is
half a product; this is not a soft dependency.

- `hrm_job_requisitions` (approval-gated, reuses approval engine)
- `hrm_job_postings` — `public_slug`
- `hrm_candidates` ≠ `hrm_applications` — **stage lives on the application**
- `hrm_recruitment_pipelines` / `_stages` — mirrors `crm_pipelines` shape
- `hrm_application_stage_history` — **in the first migration, not later.** `crm_deals` skipped
  this and that is precisely why sales velocity is still blocked.
- `hrm_interviews` + `_panelists` + `hrm_interview_scorecards`
- `hrm_offers` (approval-gated) → `hrm_document_templates.offer_letter` **already exists**
- `hrm_referrals`

**Public surface — the only anonymous entry point in all of HRM:**

```
GET  /api/v1/pub/careers/:orgSlug
GET  /api/v1/pub/careers/:orgSlug/:postingSlug
POST /api/v1/pub/careers/:postingId/apply    ← multipart, rate-limited
```

Reuses everything Capture Fix Pass B produces: Redis rate limiting on `/pub/*`, `LOWER(email)`
dedup, file type/size validation. **Do Capture Fix Pass B first** — not because of priority
ordering, but because building this endpoint twice is waste.

Hire → Employee conversion mirrors `crm.leads.convert`: one transaction, `candidate_id`
retained on the employee for provenance, onboarding instance (Phase 3) instantiated.

Scorecard visibility uses Phase 1 (interviewer cannot see others' scores before submitting).
Candidate purge job uses Phase 0.2.

---

## PHASE 5 — Form engine + Performance Management ✅ DONE (2026-08-08)

> **Sliced three ways, 2026-08-06. All three shipped by 2026-08-08.** At ~18-19 tables this phase
> is roughly 2.5× Phase 4A and could not ship as one.
> **5A Goals/OKR ✅ DONE** (migrations `00082`/`00083`, `internal/hrm/performance/`, 19 routes).
> **5B form engine + appraisal cycles ✅ DONE** (migrations `00084`–`00087`,
> `internal/platform/forms/` 17 routes + 21 more routes in `internal/hrm/performance/`).
> **5C 360 feedback + PIP ✅ DONE** (migrations `00088`–`00091`, `internal/hrm/feedback/` 12 routes,
> `internal/hrm/pip/` 9 routes).
>
> **Three clauses below were revised or declined during 5B/5C implementation** — recorded here so
> they read as decisions rather than drift, per this plan's rule 3. Full detail in
> `docs/Project_Instruction.md` r23:
>
>   • *"anonymity stripped at the repository layer"* was implemented as **two separate repository
>     methods returning two separate types that share no field** (coordination: identity, no
>     answers; content: relationship + form instance, no identity), plus the policy being DERIVED
>     from `Relationship.IsAnonymous()` rather than stored. A stored flag is what
>     `hrm_complaints.is_anonymous` already is, and nothing in the codebase branches on it. The
>     third leg — never handing a subject a form instance id — is the leak that lives outside the
>     module, since `platform_form_instances` stores `respondent_user_id`.
>
>   • *"minimum-response threshold"* is enforced **per relationship group, not cycle-wide**. Five
>     responses of which exactly one is a direct report still identify that direct report the
>     moment their breakdown renders, so a cycle-wide total would satisfy the threshold while
>     leaking the individual. `self` and `manager` are exempt and attributed by nature: there is
>     exactly one manager, and suppressing them below a threshold they can never reach adds no
>     privacy while making the cycle's most actionable feedback unreadable.
>
>   • **Interview scorecards were NOT migrated onto the form engine.** r21 shipped
>     `hrm_interview_scorecards` fixed-shape and called it the engine's "consumer #1"; the r22 plan
>     deferred the migration decision to "when the engine's real shape is known". It is now known,
>     and the migration is declined: scorecards carry a bespoke reveal-after-own-submit rule the
>     generic engine has no concept of, and the fixed shape is what makes that rule cheap. Revisit
>     only if a second interview-form shape is genuinely needed.
>
> **One scope item deliberately deferred:** *continuous (non-cycle-bound) 360 feedback*. 5C shipped
> the formal cycle-bound half only. Continuous feedback has no cycle to hang a suppression
> threshold on — which is the entire anonymity mechanism — so it needs its own design (rolling
> windows, or per-subject rather than per-cycle thresholds), not a nullable `cycle_id`.
>
> Goals went first even though the prose below lists the form engine first, because Goals/OKR is
> the only sub-system with **no form-engine dependency** — goals are structured numeric data, not
> questionnaires. Building the engine first would have shipped a primitive with zero consumers,
> contradicting this plan's own rule 1 ("nothing speculative — a primitive gets built when its
> first real consumer is queued next"). The engine lands in 5B with appraisals, its first real
> consumer, mirroring Phase 3's checklist-engine-plus-onboarding shape.
>
> Two clauses below were revised during 5A implementation, both recorded in
> `docs/Project_Instruction.md` r22:
>   • *"`parent_goal_id` self-FK cascade"* was read as the OKR domain term (cascading alignment),
>     NOT `ON DELETE CASCADE` — the same sentence demands check-in history "from day one", and
>     CASCADE destroys the most of it. Shipped as `ON DELETE SET NULL`.
>   • *"weight-sum validation in service layer"* is not achievable in the service alone: a
>     read-then-write loses to concurrent requests, and `FOR UPDATE` on sibling goals does not
>     close the window. The rule stays in the service; enforcement moved into the repository
>     transaction, locking the employee row.

Second and third consumers of the form pattern arrive together, so the primitive gets built now.

**`internal/platform/forms/`** — sections → typed questions → typed responses → scoring →
aggregate. **Definition snapshotted onto each instance** so historical records render as
authored. Responses are rows, never a JSONB blob (aggregate queries need them).

**Performance Management** — four sub-systems, built in this order:

1. **Goals/OKR** — `parent_goal_id` self-FK cascade, `hrm_goal_checkins` history from day one,
   `measurement_type` strategy per type, weight-sum validation in service layer
2. **Appraisal cycles** — configurable `hrm_rating_scales`, template + scale **snapshotted**,
   phase state machine (`draft → self_review → manager_review → calibration → published →
acknowledged`) with per-transition guards, `manager_id_snapshot` frozen at instantiation,
   publish-immutable (payslip pattern), calibration adjustments with mandatory audit trail
3. **360 feedback** — anonymity stripped at the **repository** layer, minimum-response threshold
   before any aggregate renders
4. **PIP** — `failed` outcome hands off to existing `hrm_terminations`, no new path

`final_rating` must be a structured queryable FK — Phase 7 (merit matrix) and Phase 10 (9-box)
both read it.

---

## PHASE 6 — Learning & Development ✅ DONE (2026-08-10)

> **Sliced two ways, both shipped.** **6A LMS core ✅ DONE** (migrations `00092`/`00093`,
> `internal/hrm/learning/`, 29 routes, 8 tables). **6B certifications + skills + expiry sweep
> ✅ DONE** (migrations `00094`/`00095`, `internal/hrm/certifications/` 9 routes and
> `internal/hrm/skills/` 9 routes, 5 tables).
>
> **Three clauses below were corrected or declined during implementation** — recorded here as
> decisions rather than drift, per rule 3. Full detail in `docs/Project_Instruction.md` r24:
>
>   • *"Assessments reuse Phase 5's form engine. Separate DTOs: `QuestionForAttempt` never carries
>     the correct answer."* The DTO shipped as specified, but the premise did not hold:
>     `platform_form_questions` has **no correct-answer column and no pass mark**, and
>     `computeScore` produces a weighted RATING, not a mark. The key therefore lives in a Phase
>     6-owned `hrm_quiz_answer_keys`, which keeps the platform engine free of assessment semantics
>     AND makes the no-leak rule structural — the attempt read path never joins the key table.
>     A consequence the plan could not have anticipated: because
>     `platform_form_responses.question_id` is `ON DELETE SET NULL`, grading must happen ONCE at
>     submit and be stored, never re-derived.
>
>   • *"`hrm_skills` / `hrm_position_skills` / `hrm_employee_skills` — shared taxonomy, consumed by
>     Phases 4, 5 and 10."* **`hrm_position_skills` was NOT built.** Recruitment and performance
>     were grepped and contain zero skills fields, so there is nothing to retrofit; its only reader
>     is Phase 10, four phases out. Building it now is the speculative primitive rule 1 forbids.
>     The other two shipped, justified by a real in-phase consumer (issuing a certification that
>     carries a skill records it). The "not an LMS-internal table" instruction is honoured at the
>     package level: `internal/hrm/skills` is standalone and Phase 10 imports it directly.
>
>   • **Instructor-led sessions and training requests + budgets** are not built. They appear in
>     `Project_Instruction.md`'s Section 9 scoping paragraph but never in this plan's Phase 6 line
>     items, and neither has a consumer today. A training budget belongs next to Phase 7
>     compensation, where it would actually be spent.
>
> **Two pre-existing defects surfaced while verifying this phase, neither caused by it:**
> `scope.Predicate`'s `ScopeOwn` uses `= (SELECT …)` against a NON-unique `idx_hrm_emp_user_id`, so
> an org with one user on two employee rows makes every `view_own` list in all six scope-tiered
> modules fail SQLSTATE 21000; and the scheduler's manual-trigger route has no `:orgId` while its
> permission gate requires one, returning 400 for every job including pre-existing ones.

- `hrm_courses` + **`hrm_course_versions`** — enrollment pins `version_id`, not just `course_id`
- `hrm_course_modules` / `_lessons` — external link + PDF + mark-complete + quiz.
  **No SCORM player, no video hosting.**
- `hrm_enrollments` + `hrm_lesson_progress`
- Assessments reuse Phase 5's form engine. **Separate DTOs: `QuestionForAttempt` never carries
  the correct answer.**
- `hrm_certifications` + `hrm_employee_certifications` with `expires_at` —
  **expiry sweep is the highest-value feature here and is pure Phase 0.2**
- `hrm_skills` / `hrm_position_skills` / `hrm_employee_skills` — **shared taxonomy**, consumed
  by Phases 4, 5 and 10. Not an LMS-internal table.
- Compliance evidence reuses `hrm_acknowledgements` (needs `acknowledgeable_type` CHECK widened
  to include `course_completion`)
- Auto-assignment: simple rule rows (`department_id` / `position_id` / `on_hire`), **not a rule
  engine**

---

## PHASE 7 — Compensation depth + Benefits

**Design and build as one cluster.** Internally coupled: loans → statutory (perquisite),
benefits → statutory, revisions → payroll (arrears), bonus → performance. Piecemeal means rework.

**Payroll engine additions (first, everything else depends on them):**

- `hrm_payslip_runs.run_type` — `regular` / `off_cycle` / `bonus` / `arrears` / `fnf`
- `hrm_payslip_lines.line_type` — `earning` / `deduction` / `arrear` / `reimbursement` /
  `loan_recovery` / `statutory`
- `hrm_payslip_lines.is_employer_contribution` + nullable `source_period_id`
- Deterministic calculation order: earnings → gross → statutory base → statutory → other
  deductions → loan recovery → net
- Negative-net guard; **mandatory dry-run preview before finalize**

**Then:** salary revision cycles (batch-approved, merit matrix from `final_rating` ×
compa-ratio, `hrm_compensation_bands`) · bonus engine (shared `CompensationContext` builder
feeding both salary and bonus formulas; `calculation_snapshot` JSONB mandatory) · loans
(amortization generated once at approval, read not recomputed by payroll; foreclosure,
zero-net-pay, resignation edge cases) · reimbursement payout · statutory (country-pluggable
schema + per-country Go implementation behind an interface — **no universal formula engine**;
slabs move from code into effective-dated rows).

**Benefits** — plans, tiers, cost splits, enrollment windows (scheduler), dependents with
manual verification, enrollment → payroll deduction line. **Claims tracking out of scope.**

---

## PHASE 8 — Operations: Assets, Travel & Expense, Helpdesk

> **Status: ✅ COMPLETE — all 4 slices shipped, all uncommitted.**
> 8A Assets ✅ (r29, `internal/hrm/assets`, migrations `00106`/`00107`) ·
> 8B Travel & Expense ✅ (r30, `internal/hrm/expenses`, `00108`/`00109`) ·
> 8C Helpdesk core ✅ (r31, `internal/platform/tickets`, `00110`/`00111`) ·
> 8D Knowledge base + email-to-ticket ✅ (r32, `internal/platform/kb`, `00112`/`00113`).
>
> ⚠ 8D found and fixed a live production defect BEFORE writing any code: every system-originated
> lead capture (email, social, visitors) had been failing, because `crm_leads.created_by` was
> `NOT NULL` and all three pass an empty userID. It surfaced only because the "lead capture still
> works" regression test this phase mandates was written first and could not be made to pass. See
> r32.
>
> The architectural fork below is **decided: platform.** See r31 and Section 5 → PLATFORM —
> TICKETS in `Project_Instruction.md`. The `ticket_audience` discriminator suggested here was
> not built as such — `requester_type` (FK-free, `CHECK IN ('employee')`) plays that role, matching
> the `platform_checklist_instances.subject_type` / `platform_form_instances.subject_type`
> precedent already twice established in this codebase.

**Asset Management** — categories with `requires_return`, instances, **assignment history
where current holder is a derived query, never a stored column**, maintenance log, requests,
software license seats as a separate shape. Depreciation is a book-value stub; real fixed-asset
accounting belongs to the Accounting module. Handover sign-off reuses `hrm_acknowledgements`.

**Travel & Expense** — **line-level approval** (`amount` vs `approved_amount` per line, not
claim-level), policy violations recorded as warnings not hard blocks, per-diem and
effective-dated mileage rates, advances settled against claims with all three outcomes handled.
Multi-currency columns are mandatory here regardless of Phase 11. OCR: nullable column, manual
entry, vendor later.

**HR Helpdesk** — ✅ core shipped r31 in `internal/platform/tickets/`, the recommended location.
The pausable SLA clock is an append-only `platform_ticket_sla_events` ledger, not a counter, so a
ticket paused and resumed several times can be audited rather than merely totalled. `is_internal`
comments are filtered at the repository layer via two separate read methods, so the requester's
path never has an internal comment in scope. The `hrm_complaints` convert path is one-way and
initiated from the HRM side (`hrm → platform` is the allowed direction); `MarkConverted` is built
and tested but has **no HRM-side caller yet**. Email-to-ticket remains open — Slice 8D — and is
the riskiest change in this phase, since `internal/capture/email` hardcodes lead creation.

---

## PHASE 9 — Exit Management

> **Status: ✅ PHASE 9 COMPLETE — all 3 slices shipped. 8A–9B committed (`59d1e3e`); 9C uncommitted.**
> 9A Exit umbrella + clearance + rehire ✅ (r33, `internal/hrm/exits`, migrations `00114`/`00115`) ·
> 9B F&F settlement + gratuity ✅ (r34 core, r35 the three cross-module sources, `00116`/`00117`) ·
> 9C Exit interviews + documents + access revocation ✅ (r36, `00118`/`00119`).
>
> ⚠ **F&F turned out to be the ADDS-ON integration shape, NOT the REPLACES shape** the plan
> assumed. Bonus replaces the salary computation because a bonus run must not pay salary; an F&F
> run MUST pay prorated final salary. See r34.
>
> ✅ **All F&F sources are wired** (r35): leave encashment (priced from `encashment_rate_basis`,
> a Phase 2 column that finally has its reader), loan foreclosure (full outstanding, with the
> ordinary per-installment recovery SKIPPED for `fnf` runs so the due installment is not charged
> twice) and travel-advance recovery (same-currency only — no FX table exists, so a foreign advance
> is reported rather than converted at a guessed rate).
>
> Three decisions taken during planning, all confirmed with the user:
> **(1)** `payslips.ApproveRun`'s negative-net guard becomes RUN-TYPE-AWARE in 9B — negative net is
> valid for `run_type='fnf'` only, and the guard stays exactly as-is for every other run type.
> **(2)** Gratuity is built in 9B; the **training bond is deliberately NOT built** — it has no
> Phase 6 substrate (no course cost, no agreement record), so building it means inventing a
> Learning feature inside an Exit phase.
> **(3)** Access revocation is a scheduler sweep that suspends `organization_members.status` AND
> revokes sessions on last working date.
>
> ⚠ `hrm_notice_periods` below was NOT built as its own table — `hrm_resignations` already carries
> `notice_period_days`, `is_notice_waived` and `last_working_date`, and a second table holding the
> same facts is a second source of truth. Only the SHORTFALL was new. See r33.

Pulls from seven modules — this is the architectural stress test for everything above.

- `hrm_exits` umbrella over existing resignations/terminations decision records
- `hrm_notice_periods` with shortfall recovery
- Exit interviews — Phase 5 form engine, confidential, aggregate-only, **sent post-departure
  via scheduler**
- Clearance — Phase 3 checklist engine, `checklist_type = 'offboarding'`, plus `blocking_amount`
  which onboarding does not have
- **F&F = off-cycle payroll run (`run_type = 'fnf'`), not a separate calculator.** Same line
  types, same statutory engine, same immutability. Credits pull from Phase 2 (leave encashment),
  Phase 7 (prorated salary, gratuity, bonus). Debits from Phase 7 (loans), Phase 8 (advances,
  unreturned assets), Phase 6 (training bond).
- **Negative net is a valid outcome** — "recoverable from employee" state, not a crash
- Clearance blocking items gate finalization (`ErrClearancePending`)
- Relieving letter blocked until clearance + F&F complete; `experience_letter` template exists
- System access revocation on last working date — scheduler-driven, reuses existing
  `organization_members.status` + session revocation
- `hrm_rehire_eligibility` → checked by Phase 4 on candidate create

---

## PHASE 10 — Org Chart, Succession, Analytics ✅ DONE (2026-08-31)

> **Status: ✅ PHASE 10 COMPLETE — all 3 slices shipped, all uncommitted.**
> 10A Org chart ✅ (r38, `internal/hrm/orgchart`, migrations `00121`/`00122`) ·
> 10B Succession ✅ (r39, `internal/hrm/succession`, `00123`/`00124`) ·
> 10C People analytics ✅ (r40, `internal/hrm/analytics`, `00125`/`00126`)
>
> **10C resolved:** the read path touches ONLY `hrm_headcount_snapshots` and
> `hrm_attrition_facts`; the `analytics.nightly_snapshot` job is the sole OLTP reader and builds
> facts BEFORE snapshots. `hrm_metric_definitions` names a Go computation from a CHECKed
> vocabulary and never parses its `formula_statement` — deliberately not a second formula engine
> beside `evalFormula`. DEI suppression is four rules (primary, secondary, total withheld,
> last-group-standing), binds `view_all` holders, and no permission lifts it. Small-group pay is
> withheld at WRITE time so it is never stored. `is_regretted` is nullable and unknown stays
> unknown — Phase 9's rehire flag has its second reader. The export carries no demographic column.
>
> ⚠ **Five permission keys, not the four named below.** `hrm.analytics.manage` was added because a
> metric definition nobody may write is a constant with extra steps, and overloading `export` to
> mean "may redefine attrition" would have been worse.
>
> ⚠ **Scheduled report DELIVERY is still not built.** Resend is wired so it is unblocked, but a
> delivery mechanism with nothing anybody has asked to receive is speculative.
>
> ⚠ **The confidentiality enabler this plan names below does not exist.** "Phase 1's field-level
> filtering" was never built — the same situation as Phase 9's training bond. 10B used **two read
> paths returning two types** instead, the shape already used three times (5C 360 anonymity, 8C
> internal ticket comments, 9C exit interviews) and stronger than filtering, because the subject's
> query never selects the confidential columns at all.
>
> **10B resolved:** potential is its own column with a NOT NULL rationale and no derivation path
> from performance; the 9-box number is computed, never stored; the FK runs
> candidate → development_plan and never back, and plans have no `plan_type`, because either
> direction would leak a nomination to its subject; `view_confidential` is NOT granted to manager;
> flight risk is four derived, explained signals with no score and no table.
>
> ⚠ **Five tables, not the three named below.** The 9-box is per-employee and independent of any
> nomination — you assess the population, then nominate — so folding it into
> `hrm_succession_candidates` would force a nomination to exist before anyone could be assessed.
> Plan items are their own table; flight risk needed no table at all.
>
> **10A resolved:** `manager_id` is written back inside the relationship transaction and cleared
> when a solid line ends; only `solid` grants data access; cycle detection refuses indirect loops
> (`MaxChainDepth = 64`); the chart is **not** scope-tiered and must never call `ResolveScope`;
> `GetChart` reads `manager_id` deliberately so drift is visible rather than hidden.

**Org Chart** — `hrm_reporting_relationships` as an effective-dated table with
`relationship_type` (`solid` / `dotted` / `functional` / `project`). Does **not** delete
`hrm_employees.manager_id` — that stays as the denormalized current solid-line pointer that
Phase 1's `view_team` already depends on; the new table becomes the temporal + matrix source of
truth and keeps the column in sync. Cycle detection required. Recursive CTE first.
`hrm_position_seats` nullable so vacant-seat → requisition retrofits later. Frontend is
d3-hierarchy / react-flow with lazy expand — **Collection View Pattern does not apply**.

**Succession** — 9-box with **potential assessed separately, never derived from performance**.
Critical positions, readiness levels, signal-based flight risk (explainable indicators, no ML
score), development plans. ⚠️ Field-level confidentiality within a single record: 9-box position,
flight risk, and successor nomination invisible to the subject; development plan visible.
Phase 1's field-level filtering is the enabler.

**People Analytics** — nightly snapshot + fact tables in the same Postgres, **never live
aggregation over OLTP**. `hrm_metric_definitions` is non-optional. Attrition needs
voluntary/involuntary, regretted/non-regretted (Phase 9 rehire flag), cohort retention,
first-year attrition. Compensation analytics, DEI (aggregate-only, threshold-gated), and export
are separately permissioned. **Predictive scoring deliberately excluded.** Scheduled report
delivery needs EMAIL SENDING.

---

## PHASE 11 — Multi-country / Multi-currency ✅ DONE (2026-09-01)

> **Status: ✅ PHASE 11 COMPLETE — and with it the ENTIRE HRM EXTENDED BUILD PLAN.**
> 11A Legal entities ✅ (r41, `internal/hrm/entities`, `00127`/`00128`) ·
> 11B-1 FX rates ✅ (r42, `internal/hrm/fx`, `00129`/`00130`) ·
> 11B-2 Entity re-scoping ✅ (r43, `00131`/`00132`/`00133`)
>
> **11B-2 resolved:** statutory rules now narrow to the employee's country — fixing a live defect
> where a multi-country org applied EVERY country's deductions to everyone.
> `hrm_payslip_runs.legal_entity_id` has its first reader. Analytics populates the `legal_entity`
> dimension. All 6 hardcoded `currency := "BDT"` sites are gone.
>
> ⚠ **Everything fails OPEN.** Narrowing wrongly means withholding nothing or paying nobody — both
> worse than the defect. Rules narrow only when a LEGAL ENTITY declared a country (never
> `organizations.country`, a profile field); a country with no rules falls back to the full set; a
> run with no entity covers the whole organization.
>
> ⚠ **`00133` was discovered by a test:** `uq_hrm_pr_org_month_regular` prevented a multi-entity org
> from running its German and British payrolls in the same month.
>
> ⚠ **`hrm.entities.view_all` is an ordinary permission, not a fourth scope tier** (r38), so
> `TestPermissions_ScopeTiersSeeded` is untouched.
>
> ⚠ **11B was sliced in two** (decided with you): 11B-1 is additive — the FX table and the two
> carried gap closures; 11B-2 is surgery on payroll and statutory resolution, the code that has
> produced five money defects. 11B-1 is a clean stopping point.
>
> **11B-1 resolved:** `hrm_exchange_rates` effective-dated (`MAX(rate_date) <= asOf`), rate stored
> `NUMERIC(18,8)` because a rate is not money. Never store converted-only — all five audit fields,
> enforced all-or-nothing by `chk_hrm_esl_conversion_complete`. **With no rate nothing converts,
> and never at parity**: 9B's refusal survives intact. A caller-supplied rate still wins; the
> lookup is the fallback. Same-currency records no conversion. Inverted rates report
> `direction: inverted`.
>
> ⚠ **Still open for 11B-2:** `ListActiveRules` ignores `country_code` entirely, so a multi-country
> org applies every country's statutory rules to everyone; `hrm_payslip_runs.legal_entity_id` has
> no reader; `currency := "BDT"` is hardcoded in 6 places.
>
> **11A resolved:** `hrm_legal_entities` extended in place (never dropped — it holds 0.4 rows and
> 39 FKs point at it); `hrm_country_configs` and `hrm_locations` created; `hrm_employees.location_id`
> added, nullable. Resolution is a fallback chain **entity → org default → organization**, applied
> **field by field** so a half-populated entity cannot be silently relocated, with every resolved
> value carrying its `source`. The first entity becomes the default automatically and unsetting the
> default is refused. Country configs are DEFAULTS with every column nullable, attached to the
> RESOLVED country.
>
> ⚠ **All 39 `legal_entity_id` columns stay nullable and un-backfilled**, and an
> `information_schema` test asserts it. An org with no entities — every org in this database — is
> completely unaffected, which is the regression guard for the whole phase.

Not a module — a legal-entity layer between organization and employee. Phase 0.4 already
planted `legal_entity_id` and `currency`, so this phase writes logic rather than schema surgery.

`hrm_legal_entities`, `hrm_country_configs`, `hrm_exchange_rates`, `hrm_locations`.
Payroll runs, statutory resolution, and analytics views all re-scope to entity.
Currency rule: **never store converted-only** — `original_amount` + `original_currency` +
`rate` + `rate_date` + `converted_amount`.
⚠ `view_all_entities` is an **ordinary permission, not a fourth `authz.Scope` tier** (decided
r38): entity membership is orthogonal to reporting hierarchy, and a fourth tier would force every
scope-tiered resource to seed a new key or trip `TestPermissions_ScopeTiersSeeded`. Entity scoping
is a `LegalEntityFilter` applied alongside the existing own/team/all tier.
**Data residency is explicitly out of scope** and conflicts with the single-Postgres deployment
model — that, not schema, is the real wall.

---

## 2. IMMEDIATE DEPENDENCIES OUTSIDE THIS PLAN

Three items in Section 9 are load-bearing here and are not HRM work:

1. **Capture Fix Pass A/B** — Fix Pass B produces the `/pub/*` rate limiter that Phase 4's
   career page reuses. Also the only thing blocking deployment. Migration slot is now **00066**,
   not 00065.
2. **EMAIL SENDING** — hard blocker for Phase 4, and for the email channel in Phase 0.3.
   Decide the provider before Phase 0.3 ships so the channel interface is written against a real
   API rather than a guess.
3. **Doc reconciliation** — `Project_Instruction.md` is wrong about cron existence, route counts,
   migration 00065, and the `reporting_manager_id` prep migration. Fix on sight.

---

## 3. WHAT TO DECIDE BEFORE PHASE 0 STARTS

Four decisions, none of which should be made mid-build:

1. **Email provider** — Postmark serves both capture-inbound and transactional-outbound on one
   account. If that is the choice, it collapses two Section 9 items into one.
2. **Helpdesk placement** — `internal/platform/tickets/` with an audience discriminator, or
   inside HRM. Recommendation: platform. Decide now; retrofitting is a table rename plus every
   query.
3. **`expr` boundary** — confirm the float-in/decimal-out approach in 0.1 is acceptable, or
   commit to replacing the formula engine. Not a decision to defer into Phase 7.
4. **Rounding policy** — scale and mode, as an org setting. Needed by 0.1.

---

## 4. SUGGESTED STARTING POINT

**Phase 0.1 (money) → 0.2 (scheduler) → 0.5 (guard tests) → 0.3 (notification).**

0.1 first because it is pure repair with a bounded blast radius today and an unbounded one
later. 0.2 second because it has two shipped-but-dead consumers waiting, which makes it
self-validating. 0.5 before 0.3 because notification introduces a new permission group and the
permission-mismatch bug class has now appeared four times.
