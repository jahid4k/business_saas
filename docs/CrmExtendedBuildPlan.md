# CRM EXTENDED — BUILD PLAN

> Source-audited 2026-09-01 against the live repository and both databases.
> Companion to `docs/HrmExtendedBuildPlan.md`. **Neither module is the product on its own** —
> see `docs/Project_Instruction.md` § Mission.
> Status vocabulary: ⚪ NOT STARTED / 🔵 ACTIVE / ✅ DONE

---

## THE MISSION THIS PLAN SERVES

> **BusinessSAAS is two modules: CRM and HRM. Both complete → deploy → business.**

HRM's backend is complete (Phases 0–11, 322 integration tests). Its frontend is not. CRM is
production-_shaped_ but not production-_grade_. This plan brings CRM to the same standard HRM
reached, and connects the two — which is where the product's actual advantage lives.

---

## 0. AUDIT RESULTS — what the source actually says

Verified directly against the repo and `businesssaas_test`, not from documentation.

### Resolved TRUE (assumptions that held)

| Claim                                        | Verdict    | Evidence                                                                                                                                         |
| -------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| Deal pipelines are per-org configurable      | ✅ **YES** | `crm_pipeline_stages (org_id, pipeline_id, name, position, probability)`                                                                         |
| Money is NUMERIC in Postgres                 | ✅ **YES** | `crm_deals.value NUMERIC(15,2)`                                                                                                                  |
| Soft delete exists                           | ✅ **YES** | `deleted_at` on `crm_leads` and `crm_deals`                                                                                                      |
| Custom field storage exists                  | ✅ **YES** | `crm_leads.custom_fields jsonb`                                                                                                                  |
| Platform primitives are generic and reusable | ✅ **YES** | `platform_{activities,notes,tasks,tickets,forms,kb,checklists,sla_policies,notifications,scheduled_jobs}` — all built for HRM, none HRM-specific |
| Lead capture surface exists                  | ✅ **YES** | `internal/capture/{visitors,apikeys}`, `crm_leads.capture_source` + `capture_metadata`                                                           |
| CRM permissions are seeded                   | ✅ **YES** | **39 keys** across 10 resources                                                                                                                  |

**Consequence:** Extended CRM needs almost no new _engines_. Forms, checklists, approvals,
notifications, scheduler, doc templates and the ticket/SLA stack already exist and are generic.
Most phases below are **consumers of primitives that are already built**, which is why 5 phases is
realistic where HRM needed 11.

### Resolved FALSE / newly found

| Finding                                        | Severity    | Evidence                                                                                                                                                                                                                                                                                                 |
| ---------------------------------------------- | ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **CRM money is `float64` in Go**               | 🔴 Critical | `internal/crm/deals/model.go:32,51,63` — `Value float64` against `NUMERIC(15,2)`. Zero `shopspring/decimal` in the package. **This is the r37 payroll defect, living in CRM.**                                                                                                                           |
| **The money guard has a vocabulary hole**      | 🔴 Critical | `internal/tests/unit/architecture/hygiene_test.go:13` — `moneyTerms = {amount, pay, salary, price, cost, bonus, severance}`. **`"value"` is absent**, so the guard walks `internal/` (line 42), reads `crm/deals/model.go`, and passes. The guard built to prevent float money never saw the deal value. |
| **CRM has NO scope tiers at all**              | 🔴 Critical | No `scope.` or `ResolveScope` reference anywhere in `internal/crm`. Every user who can read deals reads **every** deal. HRM's `view_own/view_team/view_all` does not exist here.                                                                                                                         |
| **Leads are rigid while deals are flexible**   | 🟠 High     | `crm_leads_status_check` hardcodes `new, contacted, qualified, unqualified, converted`. A company cannot add a stage — although `crm_pipeline_stages` lets them do exactly that for deals. Inconsistent within one module.                                                                               |
| **No `crm_deal_stage_history`**                | 🟠 High     | Confirmed absent. `HrmExtendedBuildPlan.md` already calls this out: _"`crm_deals` skipped this and that is precisely why sales velocity is still blocked."_                                                                                                                                              |
| **Four permission resources have no consumer** | 🟠 High     | `crm.activities`, `crm.emails`, `crm.notes`, `crm.tasks` are seeded; nothing reads them. Sixth instance of the planted-ahead-of-consumer pattern after `is_taxable`, `is_rehire_eligible`, `encashment_rate_basis`, `form_type='exit_interview'`, `legal_entity_id`, `subscriptions`.                    |
| A deal has no line items                       | 🟡 Note     | `crm_deals.value` is a single column. No products, price book or line items.                                                                                                                                                                                                                             |
| `docs/modules/crm.md` is stale                 | 🟡 Note     | Documents `app/(app)/[orgSlug]/crm/deals/` pages that do not exist. Real tree: `app/(dashboard)/[orgId]/crm/{pipeline,leads,agenda,reports,visitors,setup/*}`.                                                                                                                                           |
| Scale gap                                      | 🟡 Note     | CRM: 6 packages · 6 tables · ~42 routes. HRM: 49 · ~100 · 805.                                                                                                                                                                                                                                           |

⚠ **The first two findings compound.** Adding `"value"` to `moneyTerms` will fail the build
immediately — which is correct, and is how Phase 1 starts. Expect the widened guard to flag other
`*_value` fields (`fixed_value`, `override_value`, `point_value`); each needs judging on whether it
is money or a rating/multiplier, exactly as r37 judged `ComputeSlab`'s inputs.

---

## THE DIFFERENTIATION STRATEGY

**Do not compete with Salesforce or HubSpot on CRM features.** They have thousands of engineers and
a twenty-year head start. Feature-count competition is a losing game and there is no reason for a
customer to switch.

**The moat is structural, and no standalone CRM can copy it:**

> **This is not another CRM. It is the only system where your salespeople are also your employees.**

Because CRM and HRM share one organization, one RBAC layer, one scope engine and one set of platform
primitives:

- **Commission** — a won deal becomes a commission calculation, becomes an `hrm_bonuses` row,
  becomes a payslip line. _Salesforce cannot pay your rep. BambooHR does not know what a deal is._
  ⚠ **The rail already exists**: `crm_deals.owner_id → users(id)`,
  `hrm_employees.user_id → users(id)`, `hrm_bonuses` with `calculation_snapshot` /
  `approval_instance_id` / `payslip_run_id` / `payslip_line_id`, consumed by `payslips.BonusSource`.
- **Quota → appraisal** — HRM Phase 5's goals/OKR and appraisal cycles take quota attainment as an
  input, so a rep's number reaches their review instead of a spreadsheet.
- **Sales hierarchy = org chart** — HRM 10A's `hrm_employees.manager_id` already drives
  `scope.Predicate`'s `view_team` CTE. "A sales manager sees their team's deals" costs one filter,
  not a new permission model.
- **Hire → onboard → sell** — recruitment (Phase 4) → onboarding checklist (Phase 3) → CRM pipeline
  assignment, in one system.

**Second moat — configurable without a consultant.** Odoo and Salesforce need implementation
partners. This ships configurable stages, fields and forms out of the box (Phase 1).

**Third moat — local fit.** BDT, `hrm_country_configs`, and a statutory payroll engine already built
for Bangladesh. A combined CRM+HR that handles local labour law is a regional wedge global vendors
will not build.

---

## THE FLEXIBILITY ARCHITECTURE

**The pattern already exists in this codebase. Copy it; do not invent one.**

`hrm_employee_statuses` is `(org_id, name, category)` where `name` is org-customisable and
`category` is a fixed CHECK vocabulary that code relies on. The rule is stated in
`payslips/service.go`:

> _"Category, not status name: names are org-customisable, categories are the fixed
> CHECK-constrained vocabulary, so this cannot be broken by a rename."_

Applied to CRM:

- **`crm_lead_stages`** — `(org_id, name, category, position, is_active)` with
  `category ∈ (open, working, qualified, disqualified, converted)`. A company adds _"Awaiting Budget
  Approval"_ with category `working`; reports still group correctly and conversion logic still knows
  what `converted` means.
- **Typed custom-field definitions** — a definition table per entity (`lead`, `deal`, `contact`,
  `company`) giving each field a name, type and validation. ⚠ Typed fields are **filterable and
  reportable**; the raw `custom_fields` jsonb that exists today is neither.
- **Reuse the form engine** — `platform_form_templates/_sections/_questions/_responses` is already
  generic. CRM gets custom intake forms with **zero new engine**.

⚠ **The lead-stage migration is the riskiest data change in this plan.** It replaces a CHECK
constraint with a foreign key, and must seed one stage row per existing status per org and map every
existing lead onto it — the `seedEmployeeStatusesTx` precedent from r37, which had to backfill
orgs created before the seeding existed. An org that has never touched stages must behave exactly as
it does today.

---

## PHASE 0 — Foundations audit ✅ DONE (this document)

Findings above. Two deliverables carried into Phase 1: widen the money guard, and correct
`docs/modules/crm.md`.

## PHASE 1 — Configurability core + money correctness ⚪

Migrations `00134`/`00135`.

- **Fix CRM money first.** `crm/deals` to `shopspring/decimal`; add `"value"` to `moneyTerms` so the
  guard can never miss it again. ⚠ Do this **before** anything computes with a deal value — Phase 2
  computes velocity and Phase 5 computes commission from it.
- **`crm_lead_stages`** — org-configurable, category-backed, replacing `crm_leads_status_check`.
- **Custom field definitions** for lead/deal/contact/company, typed and validated.
- **Form-engine reuse** for custom intake.

**Prove:** an org can add a stage and reports still group by category; an org with no custom stages
behaves exactly as before; every existing lead maps to a seeded stage; deal arithmetic is exact.

## PHASE 2 — Sales execution & velocity ⚪

Migrations `00136`/`00137`.

- **`crm_deal_stage_history`** — ⚠ **in the first migration, not later.** The HRM plan says exactly
  why: _"`crm_deals` skipped this and that is precisely why sales velocity is still blocked."_
- Stage duration, time-in-stage, win/loss analysis, conversion rates by stage.
- Basic forecasting from pipeline value × stage probability (`crm_pipeline_stages.probability`
  already exists).

**Prove:** every transition is recorded, including the first; velocity is computed from history and
not from `updated_at`; a deal that skips stages is still measurable.

## PHASE 3 — Activities, tasks & timeline ⚪

Migrations `00138`/`00139`.

- Wire `platform_activities`, `platform_notes`, `platform_tasks` into a unified timeline per
  contact / deal / company — **giving `crm.activities`, `crm.notes` and `crm.tasks` their first
  consumers**.
- ⚠ `crm.emails` stays unconsumed until email is genuinely wired; do not seed a reader for a feature
  that does not exist (the speculative-primitive rule).
- **Add CRM scope tiers** — `view_own/view_team/view_all` on deals and leads, reusing
  `scope.Predicate`. ⚠ `TestPermissions_ScopeTiersSeeded` is **all-or-nothing per resource**: seed
  every tier for a resource or none.

**Prove:** a rep sees only their own deals; a manager sees their team's via the existing
`manager_id` CTE with no new tier; the timeline orders correctly across three source tables.

## PHASE 4 — Products, quotes & capture ⚪

Migrations `00140`/`00141`.

- Products, price book, **deal line items** (a deal has one `value` column today), quotes via the
  existing doc-template engine that already produces offer letters.
- ⚠ **Never store a computed total.** Deal value becomes derived from line items — the 00076
  computed-not-stored rule, and the same reason `hrm_expense_claims` has no total columns.
- Web-to-lead forms, assignment/routing rules, and the `/pub/*` rate limiter
  (**Capture Fix Pass B — which also unblocks HRM's public careers page**).

**Prove:** a deal's value equals the sum of its lines and cannot drift; an anonymous form submission
is rate-limited and cannot be used to enumerate orgs.

## PHASE 5 — The CRM↔HRM bridge ⚪ **(the moat)**

Migrations `00142`/`00143`.

- **Commission**: won deal → commission rule → `hrm_bonuses` → payslip line, through
  `payslips.BonusSource`. Consumer-owned narrow interface, satisfied structurally, no adapter.
- **Quota → appraisal**: quota attainment as an input to HRM Phase 5 goals.
- **Sales hierarchy from the org chart**: `hrm_employees.manager_id` as the team definition.

⚠ **This is the differentiator and must not be deferred.** It is also the shortest phase, because
every rail already exists. If time runs short, cut Phase 4's quotes — never this.

**Prove:** a won deal produces exactly **one** commission bonus, never two on recompute (the
9B-2 double-charge lesson); a deal owner with no employee record degrades visibly rather than
failing the run; reversing a won deal does not silently claw back a paid bonus.

---

## Constraints that will fail the build if missed

The six guards in `backend/internal/tests/unit/architecture/`:

1. **`TestPermissions_ScopeTiersSeeded`** — all-or-nothing per resource, fires only on
   `ResolveScope`. Relevant from Phase 3.
2. **`TestPermissions_AllRoutesProtected`** — `permFn("...")` with an **inline string literal**.
3. **`TestPermissions_UsedStringsExistInMigrations`** — first element of an INSERT tuple.
4. **`TestRouting_NoDuplicates`** — own group variable per sub-feature; literal segments registered
   before `:param` routes.
5. **`TestHygiene_NoFloatMoneyFields`** — ⚠ **widen `moneyTerms` in Phase 1**; it currently cannot
   see `value`.
6. **`TestHygiene_NoAIConversationArtifacts`** — declarative comments only.

**`authz.Can` builds its key as `resource + "." + action`** — pass the FULL dotted prefix; a bare
name denies everything silently.

**Migrations** start at **`00134`**, schema at N and permissions at N+1. Prove reversibility by
running `down` and checking the exact prior shape — tables that are ALTERed rather than created must
come back to their exact prior column list and indexes.

⚠ **Tooling lessons already paid for:** `touch` does not trigger an air rebuild (r36); a FAILED
build leaves the previous binary running (r33); **a SUCCESSFUL build also leaves it running if the
new process cannot bind the port** (r43) — check for `address already in use` after the last
`running...`. Backup filenames must be package-qualified and every restore verified (r43). Never
`git checkout` to undo an injection (9C).

---

## Sequencing (per phase)

1. Schema migration → prove reversibility (`up`, `down`, exact prior shape, re-up) → apply to both DBs.
2. Permission migration → verify per-role grants by querying `roles`.
3. Architecture suite **before** writing Go.
4. Models → **arithmetic with tests written first** → repositories → services → unit tests →
   handlers → **routes last**.
5. `main.go` + `setup_test.go` wiring.
6. Integration tests → **red/green injection proofs** → full suite → live smoke run → docs revision
   the same day.

⚠ **Every injection must fail for the RIGHT reason.** Three were invalid during Phase 11 — two did
not compile and one hit a SQL type error. A test that goes red because the code no longer builds
proves nothing. Run `go build` on the injected package before trusting the test result.

---

## Verification

```bash
cd backend && go build -o /dev/null ./... && go vet ./...
goose -dir ./internal/migrations postgres "$TEST_DSN" up      # then down, then up
go test ./internal/... ./pkg/...                              # includes the 6 guards
INTEGRATION=1 DATABASE_URL=... REDIS_URL=... go test ./internal/tests/integration/
```

**322 existing integration tests must stay green throughout** — CRM work must not regress HRM.

**What must be proved, not assumed:**

- A company can add a lead stage, and reports still group it correctly by `category`.
- An org with no custom stages behaves **exactly** as before the migration.
- Deal money is exact — the r37 lesson, now applied to CRM.
- `crm_deal_stage_history` records every transition, and velocity comes from it.
- A rep sees only their own deals; a manager sees their team's through the existing `view_team`.
- A deal's value equals the sum of its line items and cannot drift.
- **A won deal produces exactly one commission bonus, never two on recompute.**

Finish each phase with a live HTTP smoke run against real seeded data, cleaned up afterwards, then
`docs/Project_Instruction.md` revision and this file's status the same day.

---

## Known-open, carried

- **HRM frontend covers roughly Phases 0–3 only** — ~37 backend packages have no dedicated UI. This
  does not shrink while CRM grows, and it sits between here and a sellable product. A good moment to
  start it is after Phase 2, when CRM's data model has stopped moving.
- **Nothing is deployed** — `.github/workflows/deploy.yml` is an explicit placeholder.
- **Nothing is chargeable** — `subscriptions` (00009) and `organization_usage` (00010) have full
  schemas and zero Go code.
- **No backup story.** ⚠ Required before any real customer touches payroll data.
- `evalFormula` on `float64`; `MarkConverted` with no HRM-side caller; `EncashmentBasisFixed` has no
  column for its rate; scheduled report delivery unbuilt.
