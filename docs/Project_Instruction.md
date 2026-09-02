# BUSINESSSAAS — PROJECT MASTER INSTRUCTION

## ⚑ MISSION

> **BusinessSAAS is TWO modules: CRM and HRM. Both complete → deploy → business.**

Neither module is the product on its own. The sequence is deliberate and in this order:

1. **CRM + HRM both production-grade** — HRM's backend is complete (Phases 0–11); its frontend is
   not. CRM is production-*shaped* but not production-*grade*
   (see `docs/CrmExtendedBuildPlan.md`).
2. **Deploy** — ⚠ `.github/workflows/deploy.yml` is still an explicit placeholder; nothing is live.
   ⚠ Automated, restore-tested Postgres backups are required before any real customer touches
   payroll data.
3. **Business** — ⚠ `subscriptions` (00009) and `organization_usage` (00010) have complete schemas
   and **zero Go code**; nobody can be charged today.

**The product's actual advantage is that these two modules share one system.** A won deal can become
a commission on a payslip; a sales quota can feed a performance review; the sales hierarchy *is* the
org chart. A standalone CRM cannot pay your rep, and a standalone HR system does not know what a
deal is. That connection — not CRM feature count — is what makes this worth building.

**The two module build plans:**
- `docs/HrmExtendedBuildPlan.md` — Phases 0–11 ✅ complete (backend)
- `docs/CrmExtendedBuildPlan.md` — Phases 0–5, Phase 0 ✅ audit complete

---


> Last updated: 2026-09-01 (r43 — **HRM Extended Phase 11B-2: entity re-scoping. PHASE 11 IS
> COMPLETE, AND WITH IT THE ENTIRE HRM EXTENDED BUILD PLAN.** Migrations `00131`/`00132`/`00133`,
> no new tables, 1 new permission, 322 integration tests passing.
>
> **⚠ A LIVE DEFECT IS FIXED: statutory rules ignored `country_code` entirely.**
> `ListActiveRules` returned EVERY active rule for an organization, so a company operating in
> Germany and Britain applied **both** countries' deductions to everyone. Proved live: a German
> employee now gets German-Tax 100 and a British employee British-Tax 200 — and critically, they do
> so **even inside a single org-wide run**, because the narrowing is per-EMPLOYEE, not per-run.
>
> **⚠ THE FIX DELIBERATELY FAILS OPEN, AND THE TEST SUITE COULD NOT HAVE CAUGHT IT OTHERWISE.**
> Narrowing to the wrong country means withholding NOTHING, and under-withholding statutory tax is a
> liability the employee discovers at year end — worse than the over-application being fixed. So
> rules are narrowed ONLY when a LEGAL ENTITY declared a country, never when it came from
> `organizations.country`, which is a profile field somebody filled in at signup. Test
> organizations set no country at all, so a strict filter would have left every existing test green
> while silently zeroing deductions for real organizations. `entities.CountryForEmployee` therefore
> returns the country **and** a `fromLegalEntity` flag, and that flag is the contract.
>
> **⚠ AN ENTITY DECLARING A COUNTRY THE ORG HAS NO RULES FOR ALSO FAILS OPEN.** A company that
> opens a German subsidiary before writing its German rules keeps withholding what it was
> withholding, visibly, rather than silently stopping.
>
> **⚠ `hrm_payslip_runs.legal_entity_id` GETS ITS FIRST READER** (planted in Phase 0.4). A run
> scoped to an entity narrows its employee set; a run with NULL covers the whole organization —
> which is every run that already exists. The predicate is
> `($4::uuid IS NULL OR e.legal_entity_id = $4::uuid)`; written as a plain equality it produced
> **0 payslips** for an org with no entities, which is every organization in this database. Nobody
> gets paid is the worst possible failure of this slice, and the injection proved it in one line.
>
> **⚠ `00133` WAS AN UNPLANNED DISCOVERY MADE BY A TEST.** `uq_hrm_pr_org_month_regular` was keyed
> on `(org_id, period_year, period_month)`, so a multi-entity organization **could not run its
> German and British payrolls in the same month** — the entire point of scoping a run to an entity.
> The index now includes `COALESCE(legal_entity_id, sentinel)`; COALESCE rather than a plain
> four-column index, because NULL never equals NULL and two org-wide runs would otherwise become
> possible, reintroducing the double-payment the index exists to prevent.
>
> **⚠ THE BDT FIX CHANGED SHAPE MID-SLICE ON A FACT I FOUND LATE.** When this was approved I said it
> would affect "orgs whose `organizations.currency` is not BDT". That column is
> **NOT NULL DEFAULT 'USD'** — every organization carries USD whether or not a human chose it, and
> nothing distinguishes a deliberate USD from an untouched default. Reading it would have silently
> relabelled every existing organization's payslips. The decision (yours, once I surfaced it) is
> that only a LEGAL ENTITY'S DECLARATION counts: `entities.DeclaredCurrency` returns a currency only
> when the entity or the org's default entity declared one, and `BDT` survives as the last resort so
> an unconfigured org gets exactly what it got before. `BaseCurrency` (the full chain) remains for
> **expenses**, where a conversion only happens once the org has deliberately recorded a rate and is
> stored with its full audit set. All 6 hardcoded `currency := "BDT"` sites are gone.
>
> **Verification:** all six architecture guards green, 9 new integration tests (**322 total**, up
> from 313), migrations reversibility proved, per-role grants queried
> (`hrm.entities.view_all`: owner/admin only), **five claims proved non-vacuous by injection**, the
> whole payroll/expenses/F&F family green, and a live smoke run.
>
> ⚠ **A THIRD, PREVIOUSLY UNKNOWN STALE-BINARY FAILURE MODE — AND MY REBUILD CHECK DID NOT CATCH
> IT.** The smoke run reported `currency=BDT` and `legal_entity_id=NULL` from correct source. The
> build had SUCCEEDED and air logged `running...`, but the new process then died instantly with
> `failed to listen: listen tcp4 :8080: bind: address already in use` — an orphaned older process
> still held the port, and **that stale binary kept serving every request**. The r40 check (count of
> `running...` increased, read after the last restore) passes anyway, because the count increments
> before the new process exits.
>
> **The rule now has three parts, and the third is new:** `touch` does not rebuild (r36); a FAILED
> build leaves the previous binary running (r33); and **a SUCCESSFUL build can leave the previous
> binary running too, if the new process cannot bind the port.** The reliable check is to look for
> `address already in use` AFTER the last `running...` line — zero means the new binary is actually
> serving — or simply `docker compose restart backend`. Everything the smoke reported as a defect
> was correct code all along; one restart produced `currency=EUR` and the entity stored properly.
>
> ⚠ **An injection compiled, ran, and still PASSED — a finding about the TEST.**
> `StatutoryFailsOpenWithoutALegalEntity` used a profile country (`BD`) that matched none of the
> rule countries, so narrowing on the profile found no rules and fell back — masking the injection
> entirely. Changed to `GB`, which matches one rule, the same injection now fails loudly with
> `1 statutory lines [British Income Tax], want 2`. A fail-open test must use inputs where failing
> open and failing closed produce DIFFERENT answers.
>
> ⚠ **I damaged a file and needed help recovering it.** Backing up for injections, I copied three
> files all named `service.go` into one flat directory; they overwrote each other, and the restore
> wrote `entities/service.go` over `statutory/service.go`. `git checkout` and `git show` were both
> blocked for me, so the user ran the one-line recovery. Backup filenames are now package-qualified
> and every restore is verified with `sed -n '2p'` on the destination before anything else runs.
>
> **Still open:** the frontend covers roughly Phases 0–3 only — no UI for recruitment, performance,
> learning, compensation/benefits, assets/expenses/helpdesk, exits, org chart, succession, analytics
> or entities. **That is now the largest and only major open item.** Also `evalFormula` on float64,
> `MarkConverted` with no HRM-side caller, `EncashmentBasisFixed` with no column for its rate, and
> scheduled report delivery (Resend is wired, nothing has asked to receive a report).
>
> ---
>
> r42 — **HRM Extended Phase 11B-1: exchange rates, and the two carried
> currency gaps closed.** Migrations `00129`/`00130`, 1 table created, 2 extended, 3 new routes,
> 1 new permission resource.
>
> **⚠ THIS SLICE EXISTS BECAUSE TWO SHIPPED SLICES DELIBERATELY REFUSED TO WORK WITHOUT IT.**
> `exits/settlement.go` reported a foreign-currency travel advance at **zero** with "NOT
> RECOVERED", because *"converting a foreign-currency advance would mean inventing a rate and
> mis-charging a departing person real money"*. `expenses/money.go` froze whatever rate the
> claimant typed. Both were correct refusals; neither could be fixed before an FX table existed,
> which is why the sequencing question had only one answer.
>
> **⚠ NEVER STORE CONVERTED-ONLY.** Every converted figure now keeps all five of
> **original_amount + original_currency + rate + rate_date + converted_amount**.
> `hrm_expense_lines` already had four — 8B's instinct was right — and was one column short, so it
> gained `exchange_rate_date`. `hrm_exit_settlement_lines` had none and gained the set, with
> `chk_hrm_esl_conversion_complete` making it **all-or-nothing in the database**: a half-recorded
> conversion is the converted-only case wearing four columns. An integration test inserts a partial
> set directly through SQL and asserts Postgres refuses it.
>
> **⚠ THE RATE IS `NUMERIC(18,8)`, NOT `(15,2)`. A RATE IS NOT MONEY.** Only the RESULT of a
> conversion rounds to 2 places. The injection proving this is the sharpest in the slice: rounding
> the rate first turns 1,000,000,000 × 0.00000123 = **1230** into **0**. Verified live end to end,
> through the database and back.
>
> **⚠ WITH NO RATE, NOTHING CONVERTS — AND CERTAINLY NOT AT PARITY.** This is the rule 9B's refusal
> was protecting, and giving settlement a rate source must not weaken it. With no recorded rate the
> advance is still reported at zero with its reason. The injection is the whole argument: a parity
> fallback charged a departing person **500** at an invented 1:1 rate *and marked the advance
> settled*, so it would never be recovered properly. `ConvertAsOf` returns `available=false` with
> no conversion attached, so a caller cannot accidentally use a figure that was never computed.
>
> **⚠ A CALLER-SUPPLIED RATE STILL WINS.** An organization with a contractual or corporate-card
> rate must be able to state it; the table lookup is the FALLBACK. Before 11B-1 the caller's number
> was the only path and its absence meant 1 — that is the actual change.
>
> **A same-currency line records NO conversion.** Writing a rate of 1 with today's date would put a
> fabricated lookup into an audit trail whose entire purpose is to say where a number came from.
>
> **An inverted rate says so.** `RateAsOf` tries the direct pair then the inverse, and reports
> `direction: inverted` plus the row it was derived from — a derived number must not pass as a
> recorded one. `Invert` divides at rate scale, never money scale; rounding 1/109 to 0.01 is a 9%
> error on everything it touches, and a round-trip test pins it.
>
> **The lookup is effective-dated** — `MAX(rate_date) <= asOf`, the `SlabsAsOf` shape. A rate
> recorded after somebody left must not reprice their settlement. Verified live across four dates
> including one recorded in the future, which is correctly ignored.
>
> **Both attachments are OPTIONAL and both degrade to the pre-11B behaviour.** `expenses` and
> `exits` each declare their OWN one-method `RateSource` interface satisfied structurally by
> `RateAsOfPrimitive` — consumer-owned narrow interfaces, twelfth and thirteenth instances, no
> adapter and no import in the provider's direction. `entities.BaseCurrency` was added returning a
> **primitive** for the same reason: handing back `*EntityContext` would have forced expenses to
> import entities and inverted the dependency.
>
> **Verification:** all six architecture guards green, 14 new integration tests (**313 total**, up
> from 299), 7 new unit tests on the rate arithmetic, migrations `00129`/`00130` reversibility
> proved **against real conversion data in both altered tables** — seeded, `down` twice, exact
> prior column lists and all 7 indexes restored, rows intact, then re-up with the new columns back
> and NULL rather than re-invented — per-role grants queried, **six claims proved non-vacuous by
> injection**, the full payroll/expenses/F&F family (66 tests) green, and a live smoke run over all
> 3 routes.
>
> ⚠ **THREE INJECTIONS WERE INVALID ON THE FIRST ATTEMPT AND WERE REDONE.** One removed the last
> use of a bound parameter and failed on a SQL type error rather than a wrong rate; two others left
> a variable unused and did not compile at all. **A test that goes red because the code no longer
> builds proves nothing about the claim.** Rewritten as `rate_date <= $4 + INTERVAL '100 years'`,
> `_ = rateDate`, and `|| true`, all three produced the real failures. Second occurrence of this
> after r40's threshold-guard injection; it is now a standing check.
>
> ⚠ **One full-suite run showed a failure that was transient**, not a regression:
> `TestIntegration_FnF_PaysProratedSalaryPlusSettlement` failed with a bare
> `context canceled` database dial error and no assertion message. It passed 3/3 in isolation and
> the full suite was re-run clean at 313/0. A failure with no assertion output is a connection
> problem, not a logic one — but it was re-run rather than assumed.
>
> **The stale comment on the existing 9B test was updated, not left.**
> `TestIntegration_FnFSources_ForeignCurrencyAdvanceIsReportedNotGuessed` said "no FX rate table
> exists anywhere". It now explains that the table exists and this org records no rate, which makes
> the test **more** important rather than obsolete: it is the half of the pair proving the refusal
> survived.
>
> **Still open:** 11B-2 is the last slice — entity re-scoping of payroll and statutory resolution,
> plus the hardcoded `currency := "BDT"` in 6 places. ⚠ `ListActiveRules` still returns **every**
> active statutory rule for an org regardless of `country_code`, so a multi-country org applies
> German and British deductions to everyone; and `hrm_payslip_runs.legal_entity_id` still has no
> reader. Also unchanged: the frontend covers roughly Phases 0–3 only, `evalFormula` on float64,
> `MarkConverted` with no HRM-side caller, `EncashmentBasisFixed` with no column for its rate.
>
> ---
>
> r41 — **HRM Extended Phase 11A: legal entities, country configs and
> locations.** Migrations `00127`/`00128`, 1 table extended, 2 created, 1 column added to
> `hrm_employees`, 11 new routes, 2 new permission resources.
>
> **⚠ THE CENTRAL CLAIM IS A NEGATIVE ONE: AN ORGANIZATION WITH NO LEGAL ENTITIES — WHICH IS EVERY
> ORGANIZATION IN THIS DATABASE — IS COMPLETELY UNAFFECTED.** `legal_entity_id` is a nullable FK on
> **39 tables** (38 before this slice), all un-backfilled, and 11A makes not one of them required.
> Resolution is a fallback chain rather than a stored value: **entity-specific → the org's default
> entity → the organization itself**, the same shape as `hrm_per_diem_rates`' country-specific →
> org-wide lookup. A NULL `legal_entity_id` is not missing data; it means "whatever the
> organization's default is", and that answer stays correct when the default changes — which is
> precisely why nothing backfills them. An injection making resolution require an entity failed
> immediately, and an `information_schema` test asserts all 39 columns stay nullable.
>
> **⚠ THE CHAIN RESOLVES FIELD BY FIELD, NOT AS A UNIT.** This is the part that is easy to get
> wrong and expensive when wrong. An entity recording a country but no base currency must get the
> next link's CURRENCY while keeping its own COUNTRY; falling through as a unit would take that
> link's country too and **silently relocate the subsidiary**. A half-populated record is the normal
> state of one somebody is part-way through setting up. Proved live: promoting a GB-only entity to
> default left currency falling all the way to the organization's USD and timezone to
> America/New_York, while country stayed GB.
>
> **⚠ EVERY RESOLVED FIELD CARRIES ITS SOURCE**, and that is not decoration. "GBP" alone is
> ambiguous — a reader cannot tell whether the entity declares GBP or whether the entity declares
> nothing and the organization happens to be British. Those become different facts the day somebody
> adds a second entity, and a value with no provenance hides the difference. `Resolved` is
> `{value, source}` throughout, with `SourceNone` for genuinely unset.
>
> ⚠ **AN UNSET FIELD RESOLVES TO EMPTY, NEVER TO A GUESS.** Defaulting a currency to USD because it
> is common would produce payslips in the wrong money.
>
> **⚠ THE FIRST ENTITY AN ORGANIZATION CREATES BECOMES ITS DEFAULT AUTOMATICALLY, AND UNSETTING THE
> DEFAULT IS REFUSED.** An org with entities and no default has no step two in its chain — every
> lookup would skip straight to the organization and silently ignore the entities somebody just set
> up. A default is replaced by promoting another, never by clearing this one (`ErrCannotUndefault`,
> 409). `SetDefault` demotes before promoting inside one transaction, because
> `idx_hrm_legal_entities_org_default` is a partial unique index and the other order violates it
> mid-transaction.
>
> **The one-default constraint ALREADY EXISTED and was verified, not assumed.** Phase 0.4 planted
> `idx_hrm_legal_entities_org_default` as a partial unique index on `(org_id) WHERE is_default`. It
> is what makes step two single-valued, so it is load-bearing for this phase; it was checked before
> being relied on rather than recreated.
>
> **⚠ COUNTRY CONFIGS ARE DEFAULTS, NOT ENFORCEMENT, AND EVERY COLUMN IS NULLABLE.** Nothing in
> payroll reads `hrm_country_configs` and refuses to run; it supplies a value where a caller has not
> given one. A config that hard-required a statutory notice period would break the first
> organization whose contracts are more generous, which is most of them. The config attaches to the
> **resolved** country, not the one written on the entity row, so a branch inheriting its country
> still gets that country's rules — an injection restricting it to `SourceEntity` went red.
>
> **⚠ `hrm_legal_entities` IS EXTENDED AND RESTORED, NOT DROPPED AND RECREATED.** It pre-dates this
> migration with real rows in it and 39 foreign keys pointing at it. Reversibility was therefore
> proved against REAL DATA: a seeded entity row, `down` twice, and the table comes back to its exact
> six-column prior shape (`id, org_id, name, is_default, created_at, updated_at`) with all three
> original indexes and the row still present, `legal_entity_id` still on 38 tables all nullable, and
> `hrm_employees.location_id` gone. Two genuine 0.4-era rows in the dev database (Nexus Solutions,
> Vertex Logistics, created 2026-08-03) took distinct backfilled `public_id`s, which is the
> real-data proof the backfill worked.
>
> ⚠ **`country_code`, `base_currency` and `created_by` are all NULLABLE on the extended table.**
> Rows predating 11A have none, and there is no honest value to write: guessing a country for
> somebody's subsidiary is worse than resolving the organization's at read time because a guess is
> indistinguishable from a fact once stored, and attributing an old row to whoever ran the migration
> would be a lie recorded in an audit field.
>
> **`hrm.locations` is a SEPARATE resource from `hrm.entities`.** A legal entity carries a
> registration number and a tax identifier and is edited by finance or legal; a work site is a
> building somebody adds when the company opens an office. Folding them together would mean nobody
> can add an office without also being able to change the company's tax registration.
>
> **Timezones are trimmed but NOT uppercased**, unlike the ISO codes beside them — IANA zone names
> are case-sensitive, and "EUROPE/LONDON" is not a zone. An injection folding them into the same
> normaliser went red.
>
> **Verification:** all six architecture guards green, 12 new integration tests (**299 total**, up
> from 287), 7 new unit tests on the resolution chain, migrations `00127`/`00128` reversibility
> proved against real data as above, per-role grants queried, **seven claims proved non-vacuous by
> injection**, the entire pre-existing payroll/compensation/statutory suite (34 tests) still green,
> and a live smoke run over all 11 routes.
>
> ✅ **The r40 stale-binary lesson was applied and worked.** The rebuild count was read AFTER the
> last injection restore, a real content change forced a rebuild, and the increase was confirmed
> before the smoke run — which was clean first time.
>
> **Still open:** unchanged — the frontend covers roughly Phases 0–3 only, `evalFormula` on float64,
> `MarkConverted` with no HRM-side caller, `EncashmentBasisFixed` with no column to store its rate,
> and no FX table. **11B is the last slice**: `hrm_exchange_rates`, entity re-scoping of payroll and
> statutory resolution, and the two carried currency gaps (9B's un-recovered foreign advances, 8B's
> caller-supplied exchange rate).
>
> ---
>
> r40 — **HRM Extended Phase 10C: people analytics. PHASE 10 IS
> COMPLETE.** Migrations `00125`/`00126`, 3 new tables, 10 new routes, 1 new permission resource
> (5 keys), 1 new scheduler job.
>
> **⚠ NIGHTLY SNAPSHOTS AND FACT TABLES, NEVER LIVE AGGREGATION OVER OLTP.** Every analytics
> endpoint reads `hrm_headcount_snapshots` and `hrm_attrition_facts` and nothing else; the nightly
> job is the only code in the package that touches `hrm_employees`, `hrm_exits`,
> `hrm_terminations` or `hrm_rehire_eligibility`. This is not a performance preference — a live
> aggregate changes under a reader between two refreshes of the same page, and correcting an old
> record silently rewrites last March. Proved by mutating OLTP (a real completed exit) and asserting
> the metric does **not** move until the job runs, then that it does. `internal/hrm/reports`' three
> `COUNT(*)` endpoints are deliberately left alone: "how many employees are there right now" is an
> OLTP question and they answer it correctly.
>
> ⚠ **Facts are built BEFORE snapshots inside `RunSnapshot`**, because the snapshot's leaver counts
> read from the facts. Reversing the order would report a month with no leavers and then never
> correct it — the row for that date already exists and the next run writes a different date.
>
> **⚠ `hrm_metric_definitions` IS NOT A FORMULA ENGINE, AND THAT IS THE WHOLE DESIGN.** The plan
> calls the table non-optional so two consumers cannot compute "attrition" two ways. But this
> codebase already carries one interpreted-expression defect — learning's `evalFormula` on
> `float64` — and a second interpreter would trade a definitional problem for a worse one. So
> `computation` NAMES a Go implementation from a CHECKed vocabulary, every PARAMETER of that
> computation is data (which termination types count, whether probation exits are included, the
> suppression threshold), and `formula_statement` is a NOT NULL human-readable statement that is
> **never parsed**. `predictive_attrition` is not in the vocabulary and cannot be added by naming
> it — verified live, 400.
>
> **⚠ THE SUPPRESSED DEI GROUP CANNOT BE RECOVERED BY SUBTRACTION, AND THAT TOOK THREE RULES.**
> Primary suppression alone is almost useless: with a published total, one hidden group is one
> subtraction. So `Suppress` also applies **secondary suppression** (if exactly one group would be
> hidden, the smallest disclosed group is hidden too) and **withholds the total entirely** whenever
> anything is suppressed. The third rule is stronger than standard cell suppression and is what the
> plan literally asks for; it also closes the narrow case rule 2 leaves open (two hidden groups
> summing to 2 can only be 1 and 1). A fourth rule suppresses everything when only one group would
> be left standing, because one disclosed group with a withheld total still says "everyone here is
> X". Proved live: 12 male / 8 female / 1 other returns **all three suppressed and no total**.
>
> ⚠ **SUPPRESSION IS NOT A PERMISSION AND NOTHING LIFTS IT.** `Suppress` takes no caller and has no
> bypass argument; `view_dei` decides whether somebody sees the breakdown at all, never whether a
> cell opens. The owner in the smoke run holds all five analytics keys and sees the same suppressed
> distribution a manager would. Same rule as 5C.
>
> **⚠ SMALL-GROUP PAY IS WITHHELD AT WRITE TIME, NOT READ TIME.** The nightly job writes
> `comp_p25/median/p75` as NULL for any group below the threshold, so a small team's pay
> distribution is never recorded — no later query, export or database backup can expose what was
> never stored. That is a different mechanism from the permission strip (a caller without
> `view_compensation` gets the columns blanked on read), and **neither substitutes for the other**;
> both have their own test, and disabling either one is red.
>
> **⚠ AN UNKNOWN REGRETTED STATUS STAYS UNKNOWN.** `is_regretted` is nullable and
> `RegrettedFromRehireStatus` returns nil for `conditional` and for no decision at all. Folding
> those into non-regretted would report every un-reviewed departure as a good riddance and flatter
> the exact number the metric exists to expose, so `AttritionSummary` reports
> `regretted / non_regretted / regretted_unknown` as three figures. This gave Phase 9's
> `hrm_rehire_eligibility.status` its second reader.
>
> **⚠ THE EXPORT CARRIES NO DEMOGRAPHIC COLUMN.** Export is row-level by nature, and a row-level
> extract with gender on it is precisely what "DEI is aggregate-only" forbids — a spreadsheet the
> suppression rule cannot reach. An un-reviewed exit exports `is_regretted=unknown` rather than a
> blank a spreadsheet would read as false.
>
> **Two arithmetic judgements, both stated rather than buried.** The attrition denominator is the
> **average** of opening and closing headcount, not closing — dividing by the smaller number
> inflates the rate in exactly the months somebody is looking at the chart. And a period with no
> denominator returns **no rate at all** rather than 0%: an organization with no snapshot did not
> achieve perfect retention.
>
> **Verification:** all six architecture guards green, 11 new integration tests (**287 total**, up
> from 276), 14 new unit tests across suppression and the metric arithmetic, migrations
> `00125`/`00126` reversibility proved down×2 with 10B untouched, per-role grants queried (manager
> holds `view` alone), **nine claims proved non-vacuous by injection**, and a live smoke run over
> all 10 routes showing 200/200/403/403/403/403 across the manager boundary.
>
> ⚠ **THE SMOKE RUN CAUGHT A STALE BINARY, AND MY REBUILD CHECK WAS THE THING AT FAULT.** The first
> run reported the group of one as disclosed with the total published — which is injection F's
> behaviour, not the source's. The container was still running a binary built during the red/green
> injections. Checking that the `running...` count was non-zero and that the new job had registered
> proved only that *a* rebuild had happened, not that one had happened **after the last restore**.
> The rule from r36 needs this addition: after any injection cycle, force a fresh rebuild and
> confirm the count **increased from a reading taken after the restore** before trusting a smoke
> run.
>
> ⚠ **One injection failed for the wrong reason and was redone.** Deleting the threshold guard from
> the snapshot SQL removed the last use of `$3` and the query failed on a parameter-count mismatch —
> red, but not evidence that pay would be disclosed. Rewritten as `headcount >= $3 - 1000` so the
> parameter stays bound and the guard is genuinely disabled, it produced the real failure: a stored
> median of 60000 for a group of 3.
>
> **Still open:** unchanged — the frontend covers roughly Phases 0–3 only (now with a ninth
> uncovered area), `evalFormula` on float64, `MarkConverted` with no HRM-side caller,
> `EncashmentBasisFixed` with no column to store its rate, and no FX table (Phase 11B). Scheduled
> report **delivery** remains out of scope: Resend is wired, but a delivery mechanism is worth
> building once there is a report somebody has asked to receive.
>
> ---
>
> r39 — **HRM Extended Phase 10B: succession planning.** Migrations
> `00123`/`00124`, 5 new tables, 16 new routes, 2 new permission resources (5 keys).
>
> **⚠ POTENTIAL IS ASSESSED SEPARATELY AND IS NEVER DERIVED FROM PERFORMANCE.** This is the rule
> the whole slice exists to protect. If potential were computed from the appraisal rating, every
> employee would land on the grid's diagonal and the 9-box would carry exactly as much information
> as the rating already did — an expensive way to draw one number twice. The schema enforces it
> three ways: `performance_band` and `potential_band` are independent columns with independent
> CHECKs and no default derived from the other; `potential_rationale` is **NOT NULL**, because a
> potential band nobody can explain is the unexplainable score this phase bans; and
> `performance_band` carries provenance (the appraisal id plus a value snapshot) while
> `potential_band` deliberately carries none, since there is no upstream record it could come from.
> Proved live: two employees with the **same** published rating of 4.5/5 and different assessed
> potential land in box 3 (Solid Performer) and box 9 (Star).
>
> ⚠ **Omitting the potential band is a REFUSAL, not a fallback.** Performance may be derived from
> the most recent published appraisal — the appraisal *is* the performance record — but there is no
> equivalent path for potential anywhere in the package. An injection that made potential default
> to the performance band failed two tests immediately.
>
> **⚠ THERE IS NO `box_number` COLUMN AND NO `Box` FIELD.** The 9-box position is
> `f(performance_band, potential_band)` computed in Go (the 00076 computed-not-stored rule) — a
> stored box would be a third value free to disagree with the two it claims to summarise.
>
> **⚠ CONFIDENTIALITY IS TWO READ PATHS, NOT FIELD FILTERING — because the enabler the build plan
> named does not exist.** The plan cited "Phase 1's field-level filtering"; it was never built, the
> same situation as Phase 9's training bond. Instead the confidential material
> (`hrm_talent_assessments`, `hrm_succession_candidates`) and the subject-visible material
> (`hrm_development_plans` + items) live in separate TABLES reached by separate REPOSITORY METHODS
> returning separate TYPES. `SubjectPlans`' SQL never names a confidential table, so there is
> nothing in memory for a handler to forget to strip. Fourth instance of this shape after 5C's 360
> anonymity, 8C's internal ticket comments and 9C's exit interviews — and stronger than the
> filtering the plan assumed.
>
> **The FK direction is part of the guarantee.** `hrm_succession_candidates` points AT a development
> plan; a plan never points at a nomination. A `development_plan_id` on the wrong side would tell
> the subject — who may read their own plan — that they are a named successor for a specific role.
> For the same reason `hrm_development_plans` has **no `plan_type` column**: a value like
> `succession_readiness` leaks the identical fact through a field the subject is entitled to read.
>
> **The confidentiality test is STRUCTURAL, not behavioural.** It walks `SubjectView`'s type graph
> reflectively and fails on any field whose type or name belongs to the reviewer's half. A
> behavioural test only shows that today's code does not populate those fields; this shows there is
> nowhere to populate. Adding an `Assessment *TalentAssessment` field failed it on both the type
> and the field name.
>
> **⚠ `hrm.succession.view_confidential` IS DELIBERATELY NOT GRANTED TO MANAGER**, following 9C's
> `hrm.exits.interview_view` precedent. A manager may read which roles are critical — org design
> they need — but not the 9-box, the flight-risk signals or who has been nominated, because the
> subject's own manager is usually both the source of the assessment and the reader it most needs
> protecting from. Verified live over HTTP: a manager gets **200** on critical-positions and **403**
> on the grid, the bench and the employee review.
>
> **⚠ FLIGHT RISK IS COMPUTED, NOT STORED — there is no table for it, and no score anywhere.** All
> four indicators are derivable from data this database already holds: time since last promotion
> (`hrm_promotions`), pay below band (salary record → `hrm_salary_structures.grade_label` →
> `hrm_compensation_bands`), manager churn (10A's `hrm_reporting_relationships`) and appraisal
> decline (`hrm_appraisals.final_rating_value`). Stored signals go stale the moment any of those
> change with nothing to detect it; deriving them guarantees each can state the query that produced
> it. Every signal carries a mandatory `Detail` naming the actual figures, and `ReviewerView` has
> nowhere to put a score, level or probability.
>
> **Two signal thresholds are judgements, and both are stated rather than tuned.** Below-band tests
> the band **minimum**, not the midpoint — half of any healthy band sits under its midpoint by
> construction, so a midpoint test would flag half the company and mean nothing. Appraisal decline
> fires on **any** drop, because how big a drop matters is exactly the judgement that belongs to a
> human rather than to a hidden constant. Manager churn counts **solid lines only**: counting matrix
> lines would fire on ordinary project rotation, and an injection removing that filter made four
> dotted-line changes trip the signal.
>
> **Verification:** all six architecture guards green, 12 new integration tests (**276 total**, up
> from 264), 12 new unit tests across the 9-box and the four evaluators, migrations `00123`/`00124`
> reversibility proved down×2 with zero tables/permissions/grants and 10A untouched, per-role grants
> queried and confirmed (manager has succession.view and development_plan.{view,manage} and nothing
> else), **eight claims proved non-vacuous by injection**, and a live smoke run covering all 16
> routes.
>
> ⚠ **AN INJECTION THAT STAYED GREEN WAS A FINDING ABOUT THE TEST.** Deleting the day-of-month
> correction from `monthsBetween` left the whole no-promotion table passing — none of its cases
> straddled a partial month, so they proved nothing about it. A dedicated pair differing by one day
> across the 36-month threshold now pins it, and the same injection is red. Same lesson as 9B's
> tautological settlement proof.
>
> **Deviation from the plan, stated: five tables rather than three.** The 9-box assessment is
> per-employee and independent of any nomination — you assess the population, *then* nominate — so
> folding it into `hrm_succession_candidates` would have forced a nomination to exist before anyone
> could be assessed. Development plan items are their own table because a plan without actions is a
> title. Flight risk needed no table at all.
>
> **Still open:** unchanged — the frontend covers roughly Phases 0–3 only (now with an eighth
> uncovered area), `evalFormula` on float64, `MarkConverted` with no HRM-side caller,
> `EncashmentBasisFixed` with no column to store its rate, and no FX table (Phase 11B).
>
> ---
>
> r38 — **HRM Extended Phase 10A: the org chart.** Migrations
> `00121`/`00122`, 2 new tables, 8 new routes, 1 new permission resource. First slice of Phase 10.
>
> **⚠ `hrm_employees.manager_id` STAYS, AND KEEPING IT IN SYNC IS THE ENTIRE DIFFICULTY OF THIS
> SLICE.** That column is not a convenience field — `scope.Predicate`'s `view_team` tier is a
> recursive CTE joining `he.manager_id = s.id`, so **every scope-tiered permission in this product
> resolves through it**. The new `hrm_reporting_relationships` table is the temporal and matrix
> source of truth and **writes the column back inside the same transaction**; the column is never
> where a change originates. If the two ever disagree, half the product's authorization silently
> follows the stale one, and nothing would report it. The drift test was mandatory, not optional,
> and both halves were proved by injection: suppressing the sync on create leaves `manager_id` NULL
> after a solid line, and suppressing the clear on end leaves a departed manager still reading their
> former report's payroll.
>
> **Only `solid` lines confer data access — `dotted`, `functional` and `project` deliberately do
> not.** Matrix reporting is real and the chart must draw it, but a project lead who could read
> their contributors' compensation because of a project line is a quiet privilege escalation.
> `RelationshipType.GrantsDataAccess()` is the single place that decision lives, and both the
> repository sync and the chart's `MatrixLines` split read it. Making it return `true` for every
> type fails the matrix test immediately.
>
> **Cycle detection is an AUTHORIZATION safety check, not data tidiness.** A loop in the hierarchy
> makes that same recursive CTE non-terminating, so a cycle does not corrupt an org chart — it hangs
> every permission check that walks the tree. The service refuses before inserting. The unit tests
> were proved non-vacuous by injecting a parent-only check, which passes the direct A→B/B→A case and
> **fails all three deeper ones**; the integration test closes an indirect A→B→C→A and asserts
> nothing was written, neither the row nor the column. `MaxChainDepth = 64` bounds the walk so
> corrupt pre-existing data cannot hang the checker itself.
>
> **The chart is NOT scope-tiered, and that is a deliberate exception to the HRM norm.** A chart
> whose shape depends on who is looking is a subtree, not a chart, and 10B succession and 10C
> analytics both need the whole graph to compute anything. What is sensitive is the salary and
> appraisal data hanging off each node, and that stays behind its own already-tiered resources.
> `hrm.org_chart.{view,manage}` therefore never calls `ResolveScope`, which is also what keeps
> `TestPermissions_ScopeTiersSeeded` (all-or-nothing per resource) satisfied. `.manage` is separated
> from `.view` because re-parenting somebody rewrites `manager_id` and thereby changes who can read
> whose payroll — an HR-administrative act, not an editing convenience.
>
> **`GetChart` reads `manager_id`, not the relationships table, on purpose.** Rendering from the
> prettier source would hide exactly the drift this design exists to prevent. The chart shows what
> authorization actually believes.
>
> **A vacated seat is not a deleted seat.** `hrm_position_seats.employee_id` is nullable and
> clearing it keeps the seat as headcount — which is precisely what a future requisition is raised
> against. There is no `is_occupied` column (the 00076 rule); occupancy is `EmployeeID != nil`.
>
> **Verification:** all six architecture guards green, 7 new integration tests (**264 total**, up
> from 257), 12 cycle-detection unit subtests, migrations `00121`/`00122` reversibility proved
> down×2 with zero tables/permissions/grants left and `manager_id` untouched, **four claims proved
> non-vacuous by injection** (create sync, end clear, matrix isolation, cycle refusal), and a live
> smoke run covering all 8 routes: solid line syncs the column, duplicate solid → 409, indirect
> cycle → 409, dotted line leaves the column alone and appears in `matrix_lines` rather than as a
> child edge, seat assigned then vacated with the vacancy list flipping 0→1, and ending the line
> clearing `manager_id` to NULL while both history rows survive.
>
> **The smoke run found only a documentation gap:** the list filters are `?vacant=true` and
> `?active=false`, not the `_only` names the plan's prose implied. Both are now written into
> `routes.go` beside the routes they belong to.
>
> **Still open:** unchanged from r37 — the frontend covers roughly Phases 0–3 only (now with a
> seventh uncovered area), `evalFormula` on float64, `MarkConverted` with no HRM-side caller,
> `EncashmentBasisFixed` with no column to store its rate, and no FX table (Phase 11B).
>
> ---
>
> r37 — **Hardening pass: clearing the known-open ledger.** Migration
> `00120`, no new permissions. Four long-carried defects fixed, one "defect" found not to exist, and
> two additional bugs discovered along the way.
>
> **⚠ PAYROLL MONEY WAS COMPUTED IN `float64`, AND IT WAS PRODUCING WRONG FIGURES.**
> `ComputeSlab` did progressive bracket arithmetic in floating point while its inputs and outputs
> were already `decimal` — `statutory/provider.go` literally called `base.InexactFloat64()` on its
> way in. Scanning 28,572 ordinary salary bases against a three-bracket table, **42 came out one
> paisa wrong after rounding** (base `1030.10` gave `51.50` where the exact answer is `51.51`). This
> is the statutory deduction on every payslip, so that is a wrong number on somebody's pay, not a
> rounding curiosity. The bracket walk is now decimal end to end, the slab base is resolved from the
> decimal source rather than the float formula `env`, and four of those exact bases are pinned as a
> regression test. Roughly 1 in 700 bases was affected.
>
> ⚠ **`evalFormula` deliberately stays on `float64`** — it evaluates user-authored expressions
> through `expr-lang`, and making formulas exact means replacing the evaluator. The single
> conversion point is now named in a comment, and this remains open rather than being implied fixed.
>
> **⚠ THE SCHEDULER API WAS NEVER BROKEN — the known-open entry was wrong.** It claimed
> `POST /platform/scheduler/jobs/:name/run` "returns 400 NO_BUSINESS_CONTEXT for every job — its
> route has no `:orgId` but its permission gate requires one." The gate reads `business_id` from the
> **JWT**, not the path. The real fact is narrower and different: **`Login` always issues a token
> with no org** (`auth/service.go` passes `nil`), so any route relying on JWT org context needs
> `POST /organizations/:id/switch` first. Verified live: with a switched token the endpoint returns
> all **8** registered jobs with their run history. No code changed; the entry has been corrected
> instead of repeated. (An earlier note in this session said 10 jobs — that was wrong, 8 is the
> count.)
>
> **A brand-new organization could not hire its first employee.** Migration `00053` seeded
> `hrm_employee_statuses` per-org, but `organizations.Create` never did — so every org created
> through the API had none and `POST /hrm/employees` failed on a NOT NULL `status_id`. Carried since
> r18 and worked around by hand in nearly every smoke run this project has done. Now seeded inside
> `Create`'s existing transaction (an org either exists fully usable or not at all), with `00120`
> backfilling the orgs already created. The five rows mirror `00053` exactly — including `Resigned`
> and `Terminated` **sharing** the `terminated` category, because HRM filters on category, never on
> name.
>
> **⚠ TWO STACKED DEFECTS meant every automatic note this system writes has ALWAYS failed
> silently.** r32 diagnosed one — `platform_notes.created_by` was NOT NULL while capture paths
> create notes with no acting user. The real path had a second, earlier failure: `crm/leads` and
> `crm/deals` passed the bare strings `"lead"` and `"deal"` as `related_type`, but the vocabulary is
> `"<module>.<entity>"` (`crm.lead` / `crm.deal`). `CreateNote` rejected them with "invalid
> related_type value" **before** the NOT NULL constraint was ever reached, and both callers discard
> the error with `_, _ =`. Both are fixed, both use the exported constants now, and each was proved
> independently necessary by injection.
>
> ⚠ **A unit test was asserting the bug.** `TestCreateLead_DuplicateEmail_AppendsNote` checked for
> `related_type == "lead"` and passed, because its engagement stub recorded whatever it was handed
> without validating. **A stub that accepts what the real implementation refuses is worse than no
> stub** — it makes the suite agree with the defect. The stub now validates `related_type` the way
> the service does.
>
> **Test fixtures had to change, and that is the fix working.** Three integration fixtures inserted
> statuses by hand and now collide with the seeded ones; they ask for the status they need via a new
> `statusIDFor` helper instead. Two "fails closed when the org has no terminated status" tests could
> no longer reach their condition naturally and now construct it explicitly — the behaviour still
> matters, because an org can delete its own statuses.
>
> **Verification:** all six architecture guards green, 4 new integration tests (**257 total**, up
> from 253), 2 new slab unit tests pinning exact paisa results, migration `00120` reversibility
> proved down/up, three claims proved non-vacuous by injection, and a live smoke run showing a fresh
> org hiring with no manual status insert, the scheduler listing 8 jobs, and a duplicate inbound
> email recording its note with `created_by IS NULL`.
>
> ⚠ **`00120`'s status backfill is deliberately NOT reverted by its down block.** There is no way to
> tell a backfilled row from one an org has since renamed or come to depend on, and deleting a
> status would orphan `hrm_employees.status_id`, which is NOT NULL. Reverting would risk destroying
> live data to undo a repair.
>
> **Still open:** the frontend covers roughly Phases 0–3 only — no UI exists for recruitment,
> performance, learning, compensation/benefits, assets/expenses/helpdesk or exits (six phases,
> ~60 tables), which sits against §1's "Frontend drives feature prioritization". Also `evalFormula`
> on float64, `MarkConverted` with no HRM-side caller, `EncashmentBasisFixed` with no column to
> store its rate, and no FX table (Phase 11).
>
> ---
>
> r36 — **HRM Extended Phase 9C: exit interviews, documents, access
> revocation.** Migrations `00118`/`00119`, 1 new table, 6 new routes, 2 scheduler jobs.
> **PHASE 9 IS COMPLETE, and so is the HRM Extended build plan through Phase 9.**
>
> **⚠ THE MOST IMPORTANT FINDING IS ABOUT TOOLING, NOT THIS FEATURE: `touch` DOES NOT TRIGGER AN
> `air` REBUILD.** Air compares file CONTENT and logs `skipping <file> because contents unchanged`.
> Every "force a rebuild before the smoke run" I have done since r32 by touching a file has been a
> **no-op** — it only ever appeared to work because a real edit usually happened just before. This
> slice's smoke run reported the relieving letter as issuable with clearance outstanding; the source
> was correct and the integration test passed, and the cause was a binary still carrying a
> red/green injection. **The rule is now: make a real content change, and confirm the count of
> `running...` lines actually INCREASED before trusting a smoke run.** This also retroactively
> explains r33's stale-binary incident more precisely than "a failed build leaves the previous
> binary running" did.
>
> **The exit interview is confidential, and confidentiality is a SEPARATE PERMISSION from
> administering it.** `hrm.exits.interview` schedules and sends; `hrm.exits.interview_view` reads
> what was actually said. A manager holds `view_team` over exits and can therefore see that an
> interview happened — they must not see its contents, and `interview_view` is granted to
> owner/admin and deliberately **not** to manager. Splitting read from schedule at the permission
> layer is what makes that structural rather than a matter of who remembers to check — the 5C
> 360-feedback precedent, where protection is which query the caller can reach. The service
> re-checks the same key the route gates on, so no future non-HTTP caller can bypass it.
>
> **`platform_form_instances.form_type` already permitted `'exit_interview'`** in both the Go enum
> and the `00084` CHECK — built ahead of this consumer and, unlike `AckType` in r29, verified NOT
> to have drifted apart. Fourth instance of something written ahead of its consumer, after
> `is_taxable` (7D), `is_rehire_eligible` (9A) and `encashment_rate_basis` (r35).
>
> **The interview is sent POST-DEPARTURE, and the timing is the mechanism, not a convenience.**
> `scheduled_for` defaults to the day AFTER the last working date; sending earlier is refused with
> `ErrInterviewNotDue` on both the manual path and the sweep. An interview answered while still on
> the payroll gets a different answer, and the honest one is the entire point of asking.
> `hrm_exit_interviews` stores **no responses** — the form engine owns them, and a second copy would
> disagree with the first the moment one was edited.
>
> **`relieving_letter` required BOTH `document_type` CHECKs widened — the fourth occurrence of that
> trap.** `hrm_document_templates` (11 values) and `hrm_employee_documents` (14, including
> `passport`/`visa`/`certificate`/`id_proof`) are separate constraints with deliberately different
> vocabularies; widening one leaves a template creatable but never issuable. The Go `DocumentType`
> enum **and** its `IsValid()` switch were widened alongside, because r29 found `AckType` had gone
> two phases with DB-legal values unreachable through the only typed write path. Reversibility was
> proved by restoring both lists to their exact differing prior states — over-reverting into one
> merged list would have been as much a defect as under-reverting.
>
> **The relieving letter waits for clearance AND settlement; the experience letter does not.** That
> is the one place two independently-tracked states must agree, because the relieving letter is the
> document saying the organization considers the person fully departed and owed nothing. The
> experience letter states employment dates, true regardless of what is owed — withholding it would
> punish somebody for a money dispute by making them unemployable while it is resolved.
>
> **Access revocation is idempotent by construction and REVERSIBLE.** The sweep filters on
> `access_revoked_at IS NULL` and stamps only after success, so a revoked exit leaves the set
> permanently instead of being re-revoked nightly; both underlying operations (suspending an
> already-suspended member, logging out an already-logged-out user) are no-ops anyway. The
> membership is **suspended, not deleted** — an admin can re-activate it, the user account survives,
> and no HR record is touched. `auth.Service.LogoutAll` already had exactly the shape needed, so no
> provider method was invented; `authz.SuspendMembership` is new and deliberately narrow (no caller
> id — a scheduler has none). An employee with **no platform account is still stamped**: there was
> nothing to revoke, and leaving them unstamped would make the sweep retry them every night forever.
>
> ⚠ **The stamp is written LAST and only on success.** Writing it first would mean a failure halfway
> leaves an exit marked revoked with access still live — the sweep would then skip it forever, the
> worst possible outcome for a feature whose whole job is closing off access.
>
> **Verification:** all six architecture guards green, 10 new integration tests (**253 total**, up
> from 243), migration reversibility proved down×2/re-up with both `document_type` CHECKs restored
> to their exact differing prior states and 9A/9B permissions intact, three claims proved
> non-vacuous by injection, both scheduler jobs confirmed registered, and a live HTTP smoke run
> covering the document gate, the confidentiality refusal, and immediate revocation with cleanup.
>
> ⚠ **One test was strengthened after its own proof exposed it as weak.** The confidentiality test
> originally scheduled an interview but never SENT it, so the read failed with "not found" and would
> have passed with the permission check removed. It now creates a real form template, sends the
> interview, asserts a caller WITH the key can read it, and only then asserts the caller without it
> is refused — the third time in this phase that an injection proof has been a finding about the
> test rather than the code.
>
> **Phase 9 is COMPLETE.** 8A–9B are COMMITTED (`59d1e3e`); 9C is uncommitted.
>
> ---
>
> r35 — **HRM Extended Phase 9B-2: the three cross-module settlement
> sources.** No migration — all three read tables that already existed. **Phase 9B is now complete.**
>
> **Three consumer-owned narrow interfaces, one per owning module**, each naming the provider's own
> types so the provider satisfies it structurally with no adapter — the corrected
> `certifications.SkillGranter` precedent, now applied for the eighth time. All three are **nil-safe**:
> a deployment without leave, loans or expenses wired produces no line from that source rather than
> failing the settlement.
>
> **Leave encashment: `hrm/leave` owns HOW MANY days, F&F owns what a day is WORTH.** That split was
> designed in Phase 2 — `PostEncashment` records days and never money, and `encashment_rate_basis`
> has sat there since with the comment "stored config a future F&F phase reads; this phase does not
> evaluate it". This is that phase. Third instance of a column written ahead of its consumer, after
> `is_taxable` (7D) and `is_rehire_eligible` (9A). **F&F prices what HR already recorded; it does not
> decide how much to encash** — auto-encashing the whole balance was rejected because no cap exists
> in `hrm_leave_policies` (only `carry_forward_cap`), so it would pay out unlimited accrued leave
> with no way to limit it, overriding HR's own judgement.
>
> ⚠ **`EncashmentBasisFixed` cannot be honoured, and says so.** `hrm_leave_policies` stores the
> BASIS but has no column for the fixed amount, so there is nothing to pay. The settlement produces a
> **zero-amount line reading "NOT PAID: the policy uses a fixed encashment rate, which
> hrm_leave_policies has no column to store"** rather than guessing a figure or silently pricing it
> at zero. A real gap in the Phase 2 schema, recorded as such.
>
> ⚠ **Encashment rows are stored with NEGATIVE days** (they reduce a balance), so the sum is negated
> to get the positive count F&F pays for. Reading that sign wrong turns a credit into a debit on
> somebody's final settlement — proved by dropping the negation and watching the line vanish.
>
> **Loan foreclosure: the FULL outstanding, and the ordinary per-installment recovery is SKIPPED for
> `fnf` runs.** 7C deliberately left auto-foreclosure-on-exit unbuilt because it "would require
> redesigning the negative-net guard around the still-unbuilt F&F module" — 9B did both, so it is
> unblocked. **Skipping the ordinary recovery is not optional:** it is not gated by run type, so
> leaving it in charges the installment due this period TWICE (once by recovery, once inside the
> foreclosed balance) and caps the two portions by different rules — the headroom cap exists to stop
> recovery driving net negative, which is precisely what an F&F run is allowed to do. Proved by
> removing the gate and watching the loan total go from 12,000 to **13,000**.
>
> Outstanding is `SUM(total_amount - recovered_amount)` over live schedule rows, **not
> `principal_amount`** — principal ignores everything already repaid and would charge a leaver for
> money they handed back month after month. `ForecloseForSettlement` is separate from the existing
> `ForecloseLoan` because the caller differs in kind: a human settling early supplies the amount, but
> a settlement must never trust a supplied figure and computes it itself.
>
> **Travel advances: same-currency recovered, foreign-currency reported.** No FX rate table exists
> anywhere (Phase 11 scope, r30), so converting would mean inventing a rate and mis-charging a
> departing person real money. A mismatched advance produces a zero-amount line reading **"NOT
> RECOVERED: no exchange rate to BDT, settle manually"** and is left unsettled. Proved live: a USD
> advance against a BDT run was reported, not charged, and `settled_amount` stayed 0.
>
> **`MarkSettled` closes out the SOURCE, not just the trail.** Linking an audit line only records
> that money was charged; without consuming the source the loan stays `active` and the advance stays
> `disbursed`, and the next process to look charges them again. Reached only after every payslip and
> line is persisted — never from `abortCompute` — so a loan can never be closed against a settlement
> that was rolled back, and failures are RETURNED so `ComputeRun` aborts rather than committing
> payslips with a loan still open. Proved by suppressing it: the loan stayed active with 12 schedule
> rows open.
>
> **Verification:** all six architecture guards green, 7 new integration tests (**243 total**, up
> from 236), four claims proved non-vacuous by injection (the `fnf` recovery gate, the encashment
> sign, the currency guard, and source consumption), the entire pre-existing suite green throughout,
> and a live smoke run settling 16 years: 30,000 salary + 480,000 gratuity + 5,000 encashment −
> 12,000 loan − 15,000 clearance = **488,000 net**, with the loan foreclosed and its 12 schedule rows
> closed. The advance recovery path is covered by integration test; the smoke run happened to
> exercise the foreign-currency skip instead, because payroll runs default to BDT while the seeded
> org's currency is USD.
>
> **⚠ Noted, not chased:** one full-suite run reported a single failure that did not reproduce on
> two subsequent runs and could not be identified from the output. Recorded rather than ignored.
>
> **Phase 9B is complete.** Remaining in Phase 9: **9C** — exit interviews, the relieving letter, and
> the access-revocation sweep. Phase 7 is COMMITTED (`f0eaf64`); 8A–8D, 9A and 9B remain uncommitted.
>
> ---
>
> r34 — **HRM Extended Phase 9B: full & final settlement.**
> Migrations `00116`/`00117`, 2 new tables, 4 new routes, and the first change to already-committed
> payroll code since r25.
>
> **⚠ CORRECTION TO THE APPROVED PLAN: F&F is the ADDS-ON shape, not REPLACES.** The plan said it
> would follow `computeBonusPayslips`, which REPLACES the salary computation. That is wrong.
> Bonus replaces because a bonus run must *not* pay regular salary; **an F&F run MUST pay prorated
> final salary** — it is the largest credit in most settlements. Building it as REPLACES would have
> duplicated the payroll engine. It is instead the additive shape loans, reimbursements, statutory
> and benefits already use, with the **employee set narrowed to the leaver**. That difference
> matters concretely: the per-employee body is a 367-line loop, and reusing it cost ~15 lines
> instead of a large refactor of shipped code.
>
> **`payslips.FnFSource` is the sixth consumer-owned narrow interface**, after `BonusSource` /
> `LoanSource` / `ReimbursementSource` / `StatutorySource` / `BenefitsSource`. Declared in
> `payslips`, satisfied by `hrm/exits`, nil-safe like all five before it. `SettlementForRun` also
> answers WHICH employee a run settles, which is how employee scoping works without adding an
> `employee_id` column to `hrm_payslip_runs` that would be meaningless for every other run type.
>
> **An F&F run must bypass the org-wide eligibility filter.** That filter pays a terminated
> employee only when their `termination_date` falls on or after the period start — precisely the
> person a settlement run exists to pay, months after they left. `loadFnFEmployee` loads them by id
> with the same SELECT list, so the computation downstream cannot tell which path produced its
> input. Proved by settling an employee whose termination date is the year 2000.
>
> **The negative-net guard is now run-type-aware, and the exception is to the CONCLUSION, not the
> reasoning.** For ordinary payroll a negative net means the inputs are wrong. For a settlement it
> means the leaver owes more than they are due — a receivable, and refusing to approve would strand
> the whole settlement. Every other run type keeps r25's guard exactly as written. Both directions
> are proved by injection: dropping the exception makes the F&F test fail, and inverting it makes
> BOTH the r25 guard test and the new 9B guard test fail — they cover the same line from opposite
> sides and fail for different reasons.
>
> **Clearance gates the money leaving, not the arithmetic.** `ErrClearancePending` is checked at
> APPROVAL, never at computation, so HR can compute a draft and answer "what will I actually
> receive" while clearance is still open. 8B's `ApproveLine` lesson: locking too early leaves the
> user with no remedy.
>
> **`hrm_exit_settlement_lines` is an audit trail, not the money.** The payslip remains the single
> source of truth for what was paid; this records WHERE each figure came from, because six months
> later "recovered 15,000" is unanswerable without it. One line per claim, never a lumped total — a
> departing employee has to be able to dispute a specific department's specific claim. **Amounts
> are ALWAYS positive**; direction lives in `is_credit`, because a sign convention every reader has
> to know is one reader away from a settlement that adds up backwards.
>
> **Gratuity is effective-dated** (`SlabsAsOf` shape) and its arithmetic is pure, tested before any
> caller. Partial years are NOT paid pro rata — the common statutory treatment, and paying a
> fraction would need a rounding rule nobody has specified. Below the minimum is zero **with a
> reason**, never an error: failing to qualify is an ordinary outcome, and a disputed zero has to be
> explainable. Misconduct forfeits only when the rule opts in, because forfeiture is legally loaded.
> The monthly-to-daily divisor is stored **per rule** (30 statutory vs 26 excluding weekly offs) —
> a policy choice, not an obvious fact, and the same rate is reused by the notice-shortfall line so
> one settlement cannot contain two different daily rates for the same person.
>
> **⚠ Two defects found, one by the smoke run and one by strengthening a weak test.**
> (1) Clearance-blocked approval returned **500** rather than 409 — `ErrClearancePending` was
> unmapped in the payslips handler. The service-level test passed because it asserts the sentinel;
> only the HTTP path showed it. Third consecutive slice where the smoke run found what the tests
> could not. (2) A test that only *logged* its result was rewritten to assert, and immediately
> caught the settlement audit trail **doubling** on re-assembly: the delete spared rows that already
> carried a `payslip_line_id`, so the same claim appeared twice and every figure read double.
>
> **⚠ And one of my own tests was tautological.** The effective-dating test asserted
> `amount == amount * 2`, which can only hold at zero — it could never fire. It passed the
> injection proof for the wrong reason. Rewritten to assert the exact expected figure derived from
> the same dates the service used; it now correctly fails with 960,000 against an expected 480,000
> when the date bound is removed. **An injection proof that does not go red is a finding about the
> test, not a confirmation of the code.**
>
> **Verification:** all six architecture guards green, 32 unit subtests in `hrm/exits` (gratuity and
> notice arithmetic, both written before any caller), 9 new integration tests (**236 total**, up
> from 227), migration reversibility proved down×2/re-up with 9A's tables and permissions intact,
> four claims proved non-vacuous by injection, the entire pre-existing payroll suite green
> throughout, and a live HTTP smoke run settling a 16-year employee (30,000 salary + 480,000
> gratuity − 15,000 clearance debt = 495,000 net) with cleanup.
>
> **⚠ Still open within 9B — three cross-module debit sources are NOT yet wired:** leave encashment
> (needs the money computed from `leave.PostEncashment`'s days), loan foreclosure (needs
> `loans.ForecloseLoan` triggered on exit) and travel-advance recovery. Each needs a read method on
> its owning module that does not exist today. The settlement engine, the audit trail, gratuity, the
> notice shortfall and clearance dues are complete and tested; those three are additional
> `SettlementLine` sources plugged into the same seam. Phase 7 is COMMITTED (`f0eaf64`); 8A–8D, 9A
> and 9B remain uncommitted.
>
> ---
>
> r33 — **HRM Extended Phase 9A: exit umbrella, clearance, rehire.**
> New package `internal/hrm/exits`, migrations `00114`/`00115`, 3 new tables, 11 new routes. First
> slice of Phase 9 (Exit Management), which the build plan calls "the architectural stress test for
> everything above" — it is the first phase that mostly CONSUMES rather than creates.
>
> **`hrm_exits` is an umbrella over the existing decision records, not a replacement.**
> `hrm_resignations` (`00033`) and `hrm_terminations` (`00034`) already own the DECISION to end
> employment, with their own approval chains, dates and letters. The exit record owns the PROCESS
> that follows: clearance, settlement, documents, access. Folding those two in would have thrown
> away two shipped approval flows to gain nothing. `source_type`/`source_id` are polymorphic and
> **FK-free**, the fourth instance of the pattern behind
> `platform_checklist_instances.subject_type`, `platform_form_instances.subject_type` and
> `platform_tickets.requester_type` — so `'abandonment'` or `'end_of_contract'` is a CHECK widening
> later. The trade is that referential integrity is the service's job, and it does it: an exit
> pointing at another employee's resignation is refused with `ErrSourceMismatch`, proved by
> reverting the check.
>
> **There is deliberately NO `hrm_notice_periods` table, though the build plan named one.**
> `hrm_resignations` already carries `notice_period_days`, `is_notice_waived` and
> `last_working_date`, snapshotted from the active contract at submission. A second table holding
> the same three facts is a second source of truth, and the first divergence between them is a
> payroll dispute nobody can adjudicate. What genuinely did not exist is the SHORTFALL, so that is
> the one new column — and it stores **DAYS, not money**: converting to an amount needs a daily
> rate that belongs to the salary structure, and freezing one here would silently disagree with the
> payslip. Phase 9B applies the rate.
>
> **A waived notice period is never a shortfall**, and that is the case worth naming because
> getting it wrong bills a departing person for time the employer agreed to forgo.
> `NoticeShortfallDays` is pure and was tested before any caller (the `Amortize` / `BookValue` /
> `SettleAgainstAdvance` / `EvaluateSLA` precedent), with every degenerate case given a defined
> answer: a waiver is zero, leaving LATER is zero rather than a negative that would quietly cancel
> another debt, and only the calendar date counts so a wall-clock time cannot produce an
> off-by-one.
>
> **Clearance completion is DERIVED, never stored.** It is "every blocking clearance item is
> resolved", computed from the items on read — the `00076` rule. `hrm_terminations.exit_clearance_completed`
> (`00034`) is exactly the denormalized boolean this avoids; it stays as a legacy column and Phase 9
> neither reads nor writes it. An integration test introspects `information_schema` to prove eight
> such columns stay absent from `hrm_exits`. Only a NON-ZERO amount blocks: an outstanding "return
> your badge" step is incomplete but owes nothing.
>
> **`hrm_exit_clearance_items` is the seam between clearance and F&F.** The checklist engine already
> owns who owns a step, whether it blocks and whether it is done (`00076`: `owner_type`,
> `is_blocking`, `status`); what it has no concept of is MONEY, and clearance is where IT says the
> laptop was never returned. Onboarding has no equivalent, which is why this is an overlay rather
> than an engine change. **Resolving does NOT zero `blocking_amount`** — a forgiven debt must still
> show what was forgiven, and rewriting it destroys the only record there was anything to forgive.
>
> **`hrm_terminations.is_rehire_eligible` finally has a reader.** It has existed since migration
> `00034`, commented "blocks future rehire in HR tools", and **nothing had ever read it** — the same
> shape as `hrm_salary_components.is_taxable`, which sat unread from `00023` until 7D needed it. A
> boolean alone was never enough (a resignation can carry the same decision, and it cannot record
> WHY or WHO decided, which is what a recruiter actually needs), so `hrm_rehire_eligibility` holds
> the readable decision and the exit seeds it from the old column. `recruitment.RehireChecker` is
> the consumer-owned narrow interface — declared in recruitment, naming exits' own types so
> `exits.Service` satisfies it structurally with no adapter (the corrected
> `certifications.SkillGranter` precedent, as used for `assets.HandoverAcknowledger`,
> `expenses.ReimbursementCreator` and `email.TicketRaiser`), and **nil-safe** so an install without
> exit management still recruits.
>
> **A flagged candidate is WARNED, never blocked**, and the flag is derived on read rather than
> stored on the candidate row: HR can revise a rehire decision, and a stored copy would keep
> warning about somebody already cleared. Only a NEGATIVE decision surfaces — flagging every former
> employee as "eligible" is noise a recruiter learns to ignore, and the one that matters would go
> unread with it. The check runs AFTER the insert, the same ordering 8B uses for expense-policy
> violations.
>
> **⚠ A defect the smoke run caught, and a sharper version of 8D's stale-binary lesson.** The
> rehire flag came back `null` over HTTP while the integration test for the same behaviour passed.
> The source was correct; `air` was serving an older binary. r32 already said "force a rebuild
> before a smoke run that follows red/green work" — that was not enough. The red/green injection
> for this slice left `main.go` momentarily inconsistent, so two builds FAILED, and a failed build
> leaves the PREVIOUS binary running. **Confirm the rebuild SUCCEEDED after the last restore**, not
> merely that one was attempted. A temporary probe log settled it in one cycle after speculation
> had not.
>
> **Also fixed while writing it:** `FindRehireEligibilityByEmail` derived its aliased column list
> with `strings.ReplaceAll(rehireSel, "id,", "rh.id,")`, which mangles `public_id`, `org_id`,
> `employee_id` and `exit_id` too. It failed silently because `attachRehireFlag` logs and skips on
> error by design. The test now calls the checker directly in its failure message to say WHICH it
> is — the 8D lesson about tests that swallow the reason they failed.
>
> **Verification:** all six architecture guards green, 11 unit subtests for the notice arithmetic
> (written before any caller), 13 new integration tests (**227 total**, up from 214), migration
> reversibility proved down×2/re-up with `hrm_terminations`' columns intact and the 8C/8D grants
> untouched, four claims proved non-vacuous by injection (waived-notice handling, source ownership,
> the scope tier, and the rehire flag), a new nil-safety unit test for the recruitment seam, and a
> live HTTP smoke run with all test data cleaned up.
>
> **Still open:** 9B (F&F settlement, gratuity, the run-type-aware negative-net guard) and 9C
> (exit interviews, relieving letter, access-revocation sweep). Phase 7 is COMMITTED (`f0eaf64`);
> 8A–8D and 9A all remain uncommitted.
>
> ---
>
> r32 — **HRM Extended Phase 8D: knowledge base + email-to-ticket.**
> New package `internal/platform/kb`, migrations `00112`/`00113`, 2 new tables, 9 new routes, plus
> a generalized inbound-email pipeline. **Phase 8 (Operations) is now ✅ COMPLETE across all four
> slices.**
>
> **⚠ A LIVE PRODUCTION DEFECT FOUND BEFORE ANY CODE WAS WRITTEN, AND FIXED HERE.**
> `internal/capture/{email,social,visitors}` all call `leads.CreateLead(ctx, orgID, "", req)` —
> an empty userID, because a system capture has no acting user — and `crm_leads.created_by` was
> `uuid NOT NULL`. Every one of those inserts had been dying with
> `invalid input syntax for type uuid: "" (22P02)`, the error swallowed into
> `inbound_email_logs.error_message` while the webhook still returned 200. **Email, social and
> visitor lead capture had never worked.** It surfaced because this slice's plan requires writing
> the "lead capture still works" regression test FIRST and watching it pass against existing code
> — it could not be made to pass, which is precisely what writing it first is for. `NOT NULL` was
> the assertion that lied: a system-captured lead genuinely has no human creator, and
> `capture_source` already records the origin. `created_by` is now nullable (`00112`) and
> `Lead.CreatedBy` is `*string`; attributing the row to the org owner instead would put a person's
> name on an action they did not take, in an audit column. One fix repairs all three capture paths,
> since they share `CreateLead`.
>
> **The email pipeline is now a router, and `destination` defaults to `'lead'` at three
> independent layers.** `ProcessInboundWebhook` hardcoded lead creation — a single-consumer
> pipeline, not a router. `org_inbound_emails.destination` (`'lead' | 'ticket'`) branches it, and
> the column default, the service default, and the router's fallback for an unrecognised value all
> agree on `'lead'`, so migration `00112` alone changes no behaviour and every pre-8D address keeps
> working. A test pins all three together, because if any one drifts every existing address
> silently changes what it does. `GetOrgByAddress` became `GetRouteByAddress` — the org-only lookup
> was only ever sufficient because there was one destination.
>
> **The ticket branch refuses rather than improvises.** `platform_tickets.requester_id` and
> `.requester_user_id` are both NOT NULL, so an email from somebody who is not an employee of that
> org produces no ticket — logged with a message naming the sender, not silently dropped.
> Attaching a stranger's email to a fallback employee would put words in that person's mouth, and
> in an HR helpdesk that may carry a grievance that is the worst available failure. The ticket is
> created AS the sender, so it appears in their own list and they can comment on it. Sender →
> employee resolution lives on **`capture/email`'s own repository**, not in `platform/tickets`,
> which must never reference `hrm_*` — the 7D `benefits.FindEmployeeIDByUserID` precedent.
> `email.TicketRaiser` is consumer-owned and names tickets' own types, so `tickets.Service`
> satisfies it structurally with no adapter, and it is nil-safe: an install with no ticket engine
> logs the address as unroutable rather than panicking inside a webhook handler.
>
> **The knowledge base is Platform Primitive #7**, `internal/platform/kb`. Two permissions only —
> `platform.kb.view` reads published articles, `.manage` reads and writes everything including
> drafts. The draft/published split is what makes two keys enough: a KB is org-wide reading
> material with no "mine" to narrow to, unlike tickets. A third `.view_unpublished` key would imply
> a contributor role this product does not have, and an unused key is one nobody notices is granted
> wrongly.
>
> **Unpublished articles are excluded in SQL, and the SAFE state is the DEFAULT.**
> `ArticleFilter.IncludeUnpublished` *adds* a `WHERE status = 'published'` clause when false rather
> than removing one, so a zero-valued filter — the shape of every "forgot to configure it" bug —
> returns published articles only. The failure mode is "too little", never "too much". The service
> sets the flag from the caller's own `.manage` grant and never reads it off the request. A draft
> reports NOT-FOUND rather than denied so the single-row read agrees with the list that hides it.
>
> **Search is a GIN expression index, with no `search_vector` column.** `to_tsvector('english',
> title || ' ' || body)` is indexed as an expression, so Postgres recomputes it on write — the one
> place a derived value genuinely cannot drift, unlike a stored vector somebody has to remember to
> update. `plainto_tsquery` means a user typing `&` or `!` gets a search rather than a syntax
> error. Proved live: editing a published article made the new wording searchable immediately.
> There is deliberately **no `view_count`** either: a counter nothing can recompute is unauditable
> the moment it drifts, and deriving one would need a view-event table no consumer has asked for.
> The `00076` rule cuts both ways — don't store what can be derived, and don't invent state nothing
> needs.
>
> **`published_at` records first publication, not last edit.** Editing a published article leaves
> it published (correcting a live policy must not silently unpublish it and leave employees reading
> nothing), and re-publishing an archived one restores it without pretending it is newly written —
> otherwise it jumps to the top of a list ordered by `COALESCE(published_at, created_at)`. Archive
> rather than delete, because superseded guidance still explains why somebody acted as they did.
>
> **Verification:** all six architecture guards green, 14 new integration tests (**214 total**, up
> from 200), migration reversibility proved down×2/re-up including `created_by` restored to its
> exact `NOT NULL` state and the 8C/checklists/forms grants confirmed untouched, four claims proved
> non-vacuous by injection (removing the draft SQL restriction, neutralising the tsquery match,
> forcing every address to `'lead'`, and letting the ticket branch fall back to an empty
> requester), a new unit test covering the system-capture creator case, and a live HTTP smoke run
> through both branches with all test data cleaned up.
>
> **⚠ Found but NOT fixed, deliberately:** `platform_notes.created_by` and
> `platform_activities.created_by` are `NOT NULL` and carry the identical defect — `leads.CreateLead`'s
> duplicate-capture path calls `engagementSvc.CreateNote(ctx, orgID, userID, …)` with the same empty
> userID, discarding the error with `_, _ =`. So a repeat email from a known sender still fails to
> record its duplicate-capture note, silently. Not fixed here because the engagement module has four
> entity types with this shape across 8 repository sites; fixing `crm_leads` was required to verify
> this slice, fixing engagement is its own piece of work.
>
> **Still open, unchanged:** `POST /platform/scheduler/jobs/:name/run` returns 400
> `NO_BUSINESS_CONTEXT` for every job; a fresh API-created org has no `hrm_employee_statuses`;
> `ComputeSlab`/`evalFormula` still operate on `float64`. Phase 7 is COMMITTED (`f0eaf64`); 8A, 8B,
> 8C and 8D all remain uncommitted.
>
> ---
>
> r31 — **HRM Extended Phase 8C: helpdesk core.** New package
> `internal/platform/tickets`, migrations `00110`/`00111`, 5 new tables, 17 new routes.
>
> **The architectural fork is resolved toward PLATFORM, and the codebase had already answered
> it twice.** The build plan flagged HR Helpdesk vs the CRM list's customer-facing Ticketing as
> "an architectural fork to decide before starting". `platform_checklist_instances.subject_type
> CHECK IN ('employee')` and `platform_form_instances.subject_type CHECK IN
> ('employee','candidate')` are both deliberately narrow polymorphic discriminators, widened
> later. `platform_tickets.requester_type` is the third instance of that pattern, not a new one —
> and because `requester_id` carries no FK, widening to `'contact'` when customer-facing ticketing
> lands is a CHECK change with no `hrm_*` reference to untangle first.
>
> **This package never imports `internal/hrm/scope`, so it has no scope tiers at all.**
> `scope.Predicate` hard-codes `FROM hrm_employees`; a platform primitive cannot use it, and
> `TestPermissions_ScopeTiersSeeded` never fires because nothing calls `ResolveScope`. "See only
> my own" is expressed instead as `ListFilter.CanViewAll`, resolved once in the service from
> `platform.tickets.view_all` and applied in SQL — the `platform.checklists.complete` precedent,
> where the route gate cannot express ownership so the service does. The single-row read restates
> the same rule: a filtered list is worthless if fetching the hidden ticket by id returns it
> anyway, and an invisible read reports NOT-FOUND rather than denied, because "you may not see
> ticket X" still confirms ticket X exists — in a helpdesk carrying harassment reports that is
> itself a disclosure.
>
> **`is_internal` comments are filtered at the REPOSITORY layer, in two separate read methods.**
> `FindPublicComments` and `FindAllComments` differ by one `WHERE` clause, and that difference is
> the whole of internal-comment protection. A single `FindComments` plus a caller-side filter is
> the version that eventually leaks, because the filter is one forgotten branch away from being
> skipped; the requester's path must never have an internal comment in memory at all. Structural,
> not disciplinary — the exact shape 5C used for 360 anonymity and 6A for quiz answer keys. Two
> tests prove it, one through the service as the requester and one calling the repository method
> directly, and reverting the `WHERE` clause turns both red.
>
> **The SLA clock is pausable via an append-only ledger, never a counter.**
> `platform_ticket_sla_events` holds pause/resume rows and elapsed time is computed from them on
> every read. One ticket is routinely paused and resumed several times ("waiting on the
> requester", twice), and a mutable `paused_minutes` shows the number but never how it got there —
> the same reasoning as 7C's `hrm_loan_recovery_events`. There is deliberately no
> `elapsed_minutes`, `paused_minutes` or `sla_breached` column, and an integration test
> introspects `information_schema` to prove nine such columns absent. Each clock stops at the
> event that satisfies it, not at `now`, so a ticket answered inside its window cannot drift into
> breach because nobody looked at it since.
>
> **The pause arithmetic is pure and was written before anything called it** (the `Amortize` /
> `BookValue` / `SettleAgainstAdvance` precedent), with every degenerate ledger given a defined
> answer: consecutive pauses (first wins), a resume with no pause (ignored), a trailing pause
> (runs to `now`), and an inverted interval contributing ZERO — never negative, which would
> SUBTRACT from paused time, inflate elapsed time and mask a breach. A zero target is never a
> breach: reporting every ticket in an org with no SLA policy as breached would make the signal
> useless.
>
> **The governing policy is pinned at creation.** A later policy edit tightening the target must
> not retroactively breach tickets raised under the old one — 7B's `calculation_snapshot` and 7D's
> `employee_cost_snapshot` discipline, applied to a target rather than a price. Resolution order
> is category-specific first, org-wide default (`NULL category_id`) as fallback.
>
> **Sensitive categories restrict the assignee pool**, resolved through
> `AccessDirectory.UserRoleName` rather than a local query, so this package holds exactly one
> notion of "what role does this user have" and it is authz's. The `is_sensitive` ↔
> `restricted_role` pairing is enforced in Go, not as a CHECK — a CHECK would be the `00076` trap
> the moment either column gained an `ON DELETE SET NULL` FK, and a sensitive category with no
> restricted role restricts nothing while looking like it does.
>
> **Conversion to an HR complaint is one-way and has no route.** `MarkConverted` is called from
> the HRM side, which reads the ticket, creates the complaint, then calls back — `hrm → platform`
> is the allowed direction, and this package must never import hrm to close the loop itself. An
> HTTP endpoint would have to trust a client-supplied `converted_to_id` (the same reasoning that
> gives checklists no generic instantiate route). A second conversion is REFUSED rather than
> silently accepted: two complaints believing they own the same ticket, with only the later
> recorded, is worse than an error.
>
> **⚠ Two defects the live smoke run caught, not the tests.** (1) `SLAStatus` had no json tags, so
> every duration went on the wire as a raw nanosecond count — `14400000000000` for four hours,
> which every client would have to know to divide. Now serialised as minutes, the unit the policy
> is configured in, with the minute fields DERIVED from the durations so the pair cannot disagree.
> (2) `ErrAlreadyPaused` was unreachable: a paused ticket sits in status `paused`, and the status
> guard ran ahead of the ledger check, so a second pause answered `WRONG_STATUS` — telling the
> caller the ticket was in the wrong state rather than the one thing they needed to know. The
> ledger is now consulted first, and the test asserts the specific sentinel rather than merely
> that it errored. This is the second consecutive slice where the smoke run found what the tests
> did not (r30's `ApproveLine`); the pattern is real and the smoke run is not optional.
>
> **A permission-prefix trap worth recording:** `authz.Can` builds its key as
> `resource + "." + action`, so the resource argument must carry the FULL dotted prefix
> (`"platform.tickets"`, not `"tickets"`). Passing the bare name denies everything silently and
> uniformly — which nearly produced a suite of vacuously-passing negative tests, since a test
> asserting "a member CANNOT do X" passes just as well when nobody can do anything.
>
> **Verification:** all six architecture guards green, 29 unit subtests for the pause arithmetic
> (written before any caller), 20 new integration tests (**200 total**, up from 180), migration
> reversibility proved down×2/re-up (zero tables, zero permission rows, zero role grants, with the
> pre-existing `platform.checklists.*`/`platform.forms.*` grants confirmed untouched), all three
> load-bearing claims proved non-vacuous by injection — dropping the `is_internal` filter, zeroing
> `PausedDuration`, and forcing `CanViewAll` — and a live HTTP smoke run against real seeded data
> with all test data cleaned up afterward.
>
> **Still open, unchanged:** `POST /platform/scheduler/jobs/:name/run` returns 400
> `NO_BUSINESS_CONTEXT` for every job — an SLA-breach sweep job would hit this again, and none is
> built; a fresh API-created org has no `hrm_employee_statuses`; `ComputeSlab`/`evalFormula` still
> operate on `float64`. Phase 7 is COMMITTED (`f0eaf64`); 8A, 8B and 8C remain uncommitted.
>
> ---
>
> r30 — **HRM Extended Phase 8B: travel & expense.** New package
> `internal/hrm/expenses`, migrations `00108`/`00109`, 9 new tables, 22 new routes.
>
> **Approval is per LINE, and the claim carries no total of its own.** The build plan is explicit
> — "`amount` vs `approved_amount` per line, not claim-level" — so `hrm_expense_lines` holds both
> and `hrm_expense_claims` holds NEITHER total. Both are `SUM(lines)` at read time (the `00076`
> rule). A stored claim total would disagree with its own lines the moment one was trimmed, which
> is precisely the failure r25 found in `hrm_payslip_runs`, where `TotalEmployees` counted rows
> the money totals did not. An integration test introspects `information_schema` to assert four
> forbidden total columns are absent.
>
> **`approved_amount` is nullable, and that is load-bearing.** `NULL` means "not yet reviewed";
> `0` means "reviewed, nothing payable". Two genuinely different states — defaulting to `0` would
> make an unreviewed line indistinguishable from a rejected one, and `SettleClaim` refuses while
> any line is still undecided rather than silently paying it as zero.
>
> **Multi-currency without an FX subsystem.** The build plan requires currency here "regardless of
> Phase 11", and no exchange-rate table exists in this codebase — building one is Phase 11 scope.
> Each line snapshots the `exchange_rate` used and the resulting `base_amount` at claim time, so a
> later rate change cannot rewrite a settled claim. Identical discipline to 7B's
> `calculation_snapshot` and 7D's `employee_cost_snapshot`. A rate of 1 (the default) makes
> `base_amount == amount`, so single-currency orgs never encounter it. Proved live: 500 EUR at
> 1.08 became exactly 540 base, with the original 500 EUR preserved alongside.
>
> **Policy violations are recorded warnings, never blocks** — again the build plan's own words. A
> breach writes a row to `hrm_expense_policy_violations` (with the cap snapshotted, so the warning
> still reads correctly after the policy is re-priced), and the claim submits successfully. A
> boolean on the line could say neither WHICH policy nor by how much; a hard block would make an
> over-cap taxi fare unclaimable rather than reviewable. The check runs AFTER the line is
> persisted, so a breach can never cost the employee their line.
>
> **All three advance-settlement outcomes, none of them an error state**: advance > claim
> (employee returns the balance), advance < claim (org pays the difference), advance == claim
> (clean). `SettleAgainstAdvance` is pure and takes the advance's OUTSTANDING balance, so a second
> claim against a partly-used advance sees only what is left. Only a positive payable becomes a
> reimbursement — creating a zero one would surface in a payroll run as a line nobody can explain.
>
> **The 7C boundary is honoured exactly: `internal/hrm/payslips` is untouched by 8B.** An approved
> claim's shortfall calls `hrm/reimbursements.Service.Create` through `expenses.ReimbursementCreator`
> — a consumer-owned narrow interface naming reimbursements' own types, so the service satisfies
> it structurally with no adapter (the corrected `certifications.SkillGranter` precedent). 7C's
> existing `payslips.ReimbursementSource` then pays it out. "Claim lifecycle here, payout via
> payroll in compensation", with no new payroll coupling. Proved live end to end: 800 claimed →
> 700 approved (one line trimmed 260→160) → 500 advance consumed → 200 reimbursement created.
>
> **Per-diem, mileage and policy caps are all effective-dated**, read with 7D's `SlabsAsOf` shape
> (`MAX(effective_date) <= asOf`). A cap raised next month must not retroactively excuse this
> month's breach, and a mileage line uses the rate in force on its EXPENSE date, not today's.
> Proved non-vacuous by neutralising the date filter on both lookups and watching a future rate
> leak into the past — a 2020 breach stopped being flagged and a 100km trip repriced from 45 to 90.
>
> **⚠ A usability defect the live smoke run caught, not the tests.** Deciding the LAST line flips
> a claim to `approved`, and `ApproveLine` originally refused anything but `pending_approval` /
> `partially_approved` — so an approver who mistyped that final line had no remedy at all. Now
> revisable until `paid`, because only settlement actually hands money to payroll. Covered by a
> test that corrects an approved claim and then confirms revision IS refused once settled.
>
> **Verification:** all six architecture guards green, 23 new unit subtests (FX conversion, claim
> folding, and all three settlement outcomes — pure, tested before anything calls them), 17 new
> integration tests (**179 total**, up from 163), migration reversibility proved down×2/re-up with
> both widened CHECKs restored to their exact r29 state without over-reverting (`asset_request`
> and `reimbursement` confirmed still present), and a live HTTP smoke run against real seeded data
> with all test data cleaned up afterward.
>
> **Still open, unchanged:** `POST /platform/scheduler/jobs/:name/run` returns 400
> `NO_BUSINESS_CONTEXT` for every job; a fresh API-created org has no `hrm_employee_statuses`;
> `ComputeSlab`/`evalFormula` still operate on `float64`. Phase 7 is now COMMITTED (`f0eaf64`);
> 8A and 8B remain uncommitted.
>
> ---
>
> r29 — **HRM Extended Phase 8A: asset management.** New package
> `internal/hrm/assets`, migrations `00106`/`00107`, 7 new tables, 21 new routes. First slice of
> Phase 8 (Operations), which is greenfield — exploration found no existing asset, ticketing or
> helpdesk code anywhere in the repo.
>
> **The current holder is a derived query and there is no column for it.** The build plan is
> emphatic ("assignment history where current holder is a derived query, never a stored column")
> and this is the `00076` computed-not-stored rule: a denormalized holder is a second source of
> truth that drifts the first time a return is recorded without updating it. The holder is the
> `hrm_asset_assignments` row with `returned_at IS NULL`, and the partial unique index
> `uq_hrm_asgn_active` is what makes that single-valued rather than a guess — assigning an
> already-held asset returns `ALREADY_ASSIGNED`, proved live. Identically, a licence's
> `seats_used` is `COUNT(*)` over unreleased seats, never a counter, and book value is computed
> per read. An integration test introspects `information_schema` to assert **eight** forbidden
> columns are absent (`current_holder_id`, `assigned_to`, `book_value`, `seats_used`, …) — the
> only way to prove a column does NOT exist, the 6A completion-percentage precedent.
>
> **Software licences are a separate table, not an asset category.** A physical asset has one
> holder and a serial number; a licence has N seats, a renewal date and a per-seat cost. Merging
> them means every hardware query carries a `seats_total` it never reads. The build plan names
> this distinction directly.
>
> **Depreciation is a book-value stub and says so.** Straight-line from `purchase_cost` over the
> CATEGORY's `useful_life_months`, floored at zero — an asset past its life is worth nothing, not
> a negative number. (Contrast r25's negative net pay, where a negative IS a real outcome that
> must survive; here it is arithmetic running off the end of a schedule, so clamping is correct
> rather than concealing.) Real fixed-asset accounting belongs to the Accounting module. 13 unit
> tests cover it before anything calls it — the `ComputeSlab` / `ApplyIncrease` / `Amortize`
> precedent — including day-of-month boundaries and a 120-month sweep asserting the value never
> goes negative nor exceeds cost.
>
> **⚠ A real pre-existing defect found and fixed: `AckType` had drifted from its own DB CHECK for
> two phases.** Migration `00086` (5B) added `'appraisal'` and `00094` (6B) added
> `'course_completion'` to `hrm_acknowledgements_acknowledgeable_type_check` — but neither updated
> the `AckType` enum in `internal/hrm/acknowledgements/model.go`, and `Create()` gates on
> `AckType.IsValid()`. Both values were therefore **unreachable through the only typed write
> path**: the DB permitted seven types, the Go service permitted five, and two migrations had
> added values no code could ever produce. Found while adding `'asset_handover'` as the third such
> widening — which would have been a third dead value. Fixed by widening the enum to match the
> CHECK exactly, with an integration test that walks every permitted type through
> `acknowledgements.Create` so the two can never silently diverge again. Proved non-vacuous by
> reverting the enum: the red run names all three values explicitly. This is the same class as
> r25's dropped `hrm_employees.status` and r28's unread `is_taxable` — schema and code moving
> apart with nothing to catch it.
>
> **Handover sign-off reuses `hrm_acknowledgements`** via `assets.HandoverAcknowledger`, which
> names acknowledgements' own types so `acknowledgements.Service` satisfies it structurally with
> no adapter — the *corrected* `certifications.SkillGranter` precedent (r24 rewrote that
> interface after an initial version hid real types behind aliases and `any`). A sign-off failure
> does not unwind the assignment: the asset really is in the employee's hands, and losing that
> record to keep a signature request atomic would be the worse outcome.
>
> **`.assign` is a separate permission from `.manage`**, and `.request` is self-service through
> `member` — the route cannot express "for yourself only", so `Service.RequestAsset` resolves the
> caller's own `employeeID` (the `hrm.goals.set_own` / `benefits.EnrollSelf` precedent, using this
> package's own `FindEmployeeIDByUserID` rather than a shared service method). Fulfilment is a
> distinct call from approval — `promotions.Apply` / `compensation.ApplyCycle` /
> `loans.DisburseLoan` again: a decision and the thing it authorizes are never the same call.
>
> **⚠ Scope-tier asymmetry, same shape as `hrm.payroll`'s (00097):** `hrm.assets`' tiers govern
> ASSIGNMENTS and REQUESTS, which carry `employee_id`; `hrm_assets` itself does not. An
> *unassigned* asset is org inventory visible to any `hrm.assets.view` holder — only an assigned
> one narrows to its holder's scope. `hrm.asset_config` (categories + licences) is catalog data
> with no `employee_id` and is deliberately NOT tiered.
>
> **Verification:** all six architecture guards green, 13 new unit tests, 12 new integration tests
> (**163 total**, up from 151), migration reversibility proved down×2/re-up with zero tables /
> permission rows / role grants **and all three widened CHECKs restored to their exact r28 state
> without over-reverting** (`reimbursement` and `course_completion` confirmed still present), and
> a live HTTP smoke run against real seeded data — book value exactly 1200 on a 12-month-old 2400
> asset with a 24-month life, holder derived correctly across assign→return, `ALREADY_ASSIGNED`
> and `NO_SEATS_LEFT` both refused, damaged return routed to `in_maintenance`, handover
> acknowledgement landed — all test data cleaned up afterward.
>
> **Still open, unchanged:** Phase 7 remains entirely uncommitted, and 8A now sits on top of it in
> the same working tree. `POST /platform/scheduler/jobs/:name/run` still returns 400
> `NO_BUSINESS_CONTEXT` for every job. A fresh API-created org still has no
> `hrm_employee_statuses`.
>
> ---
>
> r28 — **HRM Extended Phase 7D: statutory compliance + benefits
> administration — the final slice of Phase 7. Phase 7 (Compensation depth + Benefits) is now
> ✅ COMPLETE.** Two new packages, `internal/hrm/statutory` (country-pluggable `Provider` +
> `Registry`, one shipped data-driven `SlabProvider`) and `internal/hrm/benefits` (plans, tiers,
> enrollments, dependents), migrations `00102`–`00105`, 6 new tables, 18 new routes (7 statutory + 11
> benefits), one new scheduler consumer (`benefits.activate_pending_enrollments`).
>
> **The build plan's "no country implementation yet" scope note is NOT a bare interface.** Shipping
> a `Provider` interface with zero implementations would be exactly the speculative primitive rule
> 1 forbids — nothing would ever call it. Resolved with `SlabProvider`, ONE real, DATA-DRIVEN
> provider that reads `hrm_statutory_slabs` (effective-dated bracket rows) and evaluates them via
> `payslips.ComputeSlab` — the SAME function `hrm_salary_components`' own slab calc_method already
> uses (00023), reused rather than reimplemented. `Registry` routes each RULE (not each org) to a
> provider by `country_code`, falling back to `SlabProvider` when no country-specific override is
> registered — so a future India-specific provider (proration, eligibility thresholds a slab table
> cannot express, per the build plan's own words) can override just one rule type while every
> other rule in the same org still falls through to the generic evaluator, with zero schema change.
>
> **`TAXABLE_GROSS` did not exist in the payroll engine before this slice, and adding it required
> touching the already-carefully-tuned `computePayslips` loop a fifth time this phase.** `is_taxable`
> has existed on `hrm_salary_components` since `00023`, but nothing ever read it — Stage 1/2 now
> accumulate a `taxableGross` alongside `gross`, summing only earning components flagged taxable.
> `GrossPay` still means gross salary; `TAXABLE_GROSS` is what statutory rules actually read, kept
> deliberately separate so a non-taxable allowance cannot inflate what gets withheld. Proved live
> against real seeded payroll data with a genuine mix of taxable/non-taxable components: a 15%
> bracket over a 5000 threshold, against a 7500 taxable base (not the employee's full 8100 gross),
> produced exactly 375 — confirmed by hand against the actual `hrm_payslip_lines` rows.
>
> **Statutory and benefits both integrate into the EXISTING per-employee loop, not a dedicated
> branch** — the 7C loan/reimbursement shape, not 7B's bonus shape. Order, deliberately NOT the
> build plan's literal prose ("earnings → gross → statutory base → statutory → other deductions →
> loan recovery → net"): statutory runs AFTER the salary-structure deduction stage here, not before
> it, because statutory reads a wholly separate table and feeds nothing that stage needs — reordering
> the salary-structure stage itself (already covered by r25's reordering-safety tests) was not
> worth the risk for a placement that produces an identical total either way. It still lands before
> reimbursements/loan recovery, honoring "statutory sits between other deductions and loan_recovery"
> from r27. An employer-contribution statutory line is recorded but affects neither gross nor the
> employee's deductions — proved live and by test.
>
> **Benefit tier costs are mutable catalog data; what an employee actually pays is frozen at
> enrollment** — `employee_cost_snapshot`/`employer_cost_snapshot` — the `hrm_compensation_bands`
> (00098) pattern applied to benefits. Proved by test: repricing a tier directly at the DB layer
> after enrollment left the already-enrolled employee's payslip deduction unchanged. The employer's
> share is tracked but produces NO payslip line — no consumer reads one today, and adding one ahead
> of a real reader would be the speculative-primitive trap again; the column exists for the
> employer-cost reporting surface Section 9 already names as a future consumer.
>
> **`EnrollSelf` resolves the caller's own `employeeID` from `userID` inside the SERVICE, not the
> handler** — `hrm.benefit_enrollments.enroll_self` cannot express "for yourself only" (the
> `hrm.goals.set_own` precedent), so `benefits.Repository` grew its own `FindEmployeeIDByUserID`,
> mirroring `compensation.Repository`'s method of the same name and purpose rather than inventing a
> cross-package resolver interface — an earlier draft of this slice tried exactly that
> (`EmployeeIDResolver` satisfied by `hrm/employees.Service`) before discovering `employees.Service`
> has no such method; every prior "resolve my own employee id" need in this codebase solves it
> on the CALLER's own repository, not a shared service method, and this slice now follows that
> precedent instead of being the first exception.
>
> **`benefits.activate_pending_enrollments` is the phase's scheduler consumer** — instance-wide,
> the `attendance.absence_sweep`/`certifications.expiry_sweep` shape (r24): flips every `pending`
> enrollment whose `effective_date` has arrived to `active`. "Enrollment windows" per the build
> plan means the three named TYPES an enrollment is made under (`open`/`new_hire`/
> `qualifying_event`), modelled as a column on the enrollment itself — no separate
> org-configurable window-definition table was built, since the build plan asks for none and one
> would be exactly the speculative primitive this phase has repeatedly declined to build.
>
> **Verification:** every architecture guard green, 4 new unit tests (`SlabProvider.Compute` +
> `Registry` fallback routing — pure, no DB, the `ComputeSlab`/`ApplyIncrease`/`Amortize`
> precedent), 8 new integration tests (151 total, up from 143) covering progressive tax, the
> MANDATORY effective-dating guarantee ("a rule change dated next month must not alter this
> month's computed run" — proved non-vacuous by breaking the date-grouping query and watching a
> future 50% rate leak into the current month before restoring it), taxable-vs-non-taxable
> earnings, employer-contribution non-effect, benefits payroll deduction, tier-repricing
> immutability, the scheduler sweep (a due enrollment activates, a far-future one does not), and
> dependent verification. Migration reversibility proved down×4 (one per migration)/re-up with
> zero tables/permissions/role-grants. A live HTTP smoke run against dev with real, pre-existing
> seeded payroll data — not synthetic — confirmed the exact expected figures by hand, all created
> records cleaned up afterward.
>
> **Phase 7 is now fully shipped, end to end, across all four slices (7A payroll engine / 7B
> compensation / 7C loans+reimbursements / 7D statutory+benefits) — still entirely UNCOMMITTED,
> per standing instruction that the user commits manually.**
>
> ---
>
> r27 — **HRM Extended Phase 7C: employee loans with amortization +
> reimbursement payout.** Two new packages, `internal/hrm/loans` (amortization + schedule +
> recovery ledger) and `internal/hrm/reimbursements` (payout only), migrations `00100`–`00101`, 4
> new tables — `hrm_loans`, `hrm_loan_schedules`, `hrm_loan_recovery_events`,
> `hrm_reimbursements` — 12 new routes (7 loans + 5 reimbursements).
>
> **The amortization schedule is generated ONCE, at disbursement, and never recomputed by
> payroll** — the same immutability reasoning 5B's published appraisals and 6A's frozen quiz
> grades established. `Amortize` (pure, no DB) computes a reducing-balance schedule via decimal
> arithmetic throughout — `(1+r)^n` by a multiplication loop rather than any float pass-through —
> and the LAST installment absorbs whatever principal/rounding remainder is left, the
> `payslips.ComputeSlab` boundary-handling shape applied to loans. Tested BEFORE anything that
> calls it: 6 unit tests prove the schedule always sums to exactly the principal, interest strictly
> decreases installment to installment (reducing-balance, not flat-rate), and a pathological
> rate/tenure combination never drives the balance negative.
>
> **A single installment can be recovered across MULTIPLE payroll runs, and that forced a real
> schema decision.** The build plan's zero-net-pay rule — recovery must not push net negative —
> means a due installment may only be PARTIALLY recovered in any one run. `hrm_loan_schedules`
> tracks `recovered_amount` separately from the frozen `total_amount`
> (`pending`/`partially_recovered`/`recovered`/`foreclosed`), and a single FK from a schedule row to
> "the line that recovered it" cannot represent a three-run partial history — so
> `hrm_loan_recovery_events` exists as an append-only ledger, one row per actual recovery event,
> the `hrm_application_stage_history` (00078) shape.
>
> **The zero-net-pay cap, precisely:** inside `computePayslips`' existing per-employee loop (NOT
> a dedicated run_type branch — loan recovery and reimbursements integrate into whatever a
> regular/off_cycle/arrears/fnf run already computes for that employee, unlike 7B's bonus branch
> which replaces computation entirely), `headroom := gross - deductions + reimbursements`; each
> pending installment (oldest due first, catching up any backlog) takes `min(amount owed,
> headroom)`. A shortfall is not written off — the schedule row stays `partially_recovered` and is
> picked up again next run. Proved live and by test: a 1000 installment against 300 of headroom
> recovers exactly 300, net lands at exactly 0.00 (never negative), and the remaining 700 is
> caught up in the following run.
>
> **Reimbursements are additive but deliberately NOT folded into `GrossPay`.** A reimbursement
> repays money the employee already spent — not earned income — so it must not inflate the figure
> a future statutory (7D) engine would treat as taxable. It lands directly in `NetPay` instead:
> `net = gross - deductions + reimbursements - loanRecovery`. `TotalDeductions` DOES include loan
> recovery, though (unlike reimbursements' gross exclusion) — an employee reading their payslip
> needs to see it reflected as a real deduction. Proved live: gross stayed exactly 8100 across a
> run that paid out a 250 reimbursement; net absorbed the 250 directly.
>
> **`hrm_reimbursements` has no `calculation_snapshot`**, unlike `hrm_salary_revisions`/
> `hrm_bonuses` (00098). That column is mandatory specifically because it audits a
> CompensationContext-driven FORMULA. "Payout only" (the build plan's own words) means a flat
> HR-entered amount with no formula to snapshot — manufacturing one would be a JSONB blob nobody
> reads. Claims tracking (receipts, itemization) stays explicitly out of scope.
>
> **Loan recovery lines correlate back to their source WITHOUT the index-array mechanism 7B's
> bonus lines use.** Bonus payslips contain ONLY bonus lines (one dedicated run_type branch), so a
> parallel `SourceBonusIDs` slice matched by position was simplest there. Here a loan/reimbursement
> line sits among ordinary salary-structure lines in the SAME slice, so `PayslipLine` grew two
> unexported, unpersisted fields (`sourceLoanScheduleID`, `sourceReimbursementID`) — set at
> compute time, read directly off the same pointer once `CreatePayslipLines` assigns the real
> database ID. No index bookkeeping, no ordering assumption. Package-private by construction: only
> `computePayslips` and `ComputeRun`, in the same package, ever touch them.
>
> **The "resignation" edge case the build plan names, resolved narrowly and stated honestly.**
> Auto-foreclosing a loan at termination — attempting full recovery even at the cost of negative
> net — would require redesigning `ApproveRun`'s negative-net guard to know THAT negative net is
> legitimate, and effectively building part of the still-unbuilt F&F consolidation module (Section
> 9). Out of proportion for one loans edge case. What's built and tested instead: an employee who
> has fully left the org (their termination_date predates the run's period) is simply not in that
> period's eligible-employee set — no payslip, so no recovery attempt — and their remaining
> schedule rows stay `pending`, a receivable the org must settle out of band (via `ForecloseLoan`),
> never silently recovered NOR silently written off. Proved live and by test with a three-month
> loan: installment 1 (the exit month, a partial period) recovers normally; installment 3 (after
> the employee is fully gone) shows `run.TotalEmployees == 0` and the schedule row untouched.
>
> **Foreclosure never deletes schedule rows** — it flips the loan and every remaining
> `pending`/`partially_recovered` row to `foreclosed`, preserving the amortization as history
> (migration 00100's header). A subsequent payroll run finds nothing to recover: proved live.
>
> **Verification:** every architecture guard green, 6 new unit tests (the amortization math), 6
> new integration tests (143 total, up from 137) covering the full loan lifecycle, the zero-net-pay
> cap AND its carry-forward, foreclosure, the resignation edge case, reimbursement payout, and both
> coexisting in one run — the zero-net-pay cap proved non-vacuous by removing it and watching net
> go negative before restoring it. Migration reversibility proved down×2/re-up with zero tables /
> permission rows / role grants, correctly restoring to the exact r26 CHECK-constraint state (not
> further). Two live HTTP smoke runs against dev with real seeded payroll data (not synthetic —
> the recovery math had to coexist correctly with the employee's actual pre-existing salary
> components), all test data cleaned up afterward.
>
> **Not built, staying open for 7D:** statutory withholding on any payout, and benefits
> enrollment deductions — 7A's `line_type` ordering (`statutory` sits between `other deductions`
> and `loan_recovery`) exists specifically so 7D slots in without touching this run's arithmetic
> again.
>
> ---
>
> r26 — **HRM Extended Phase 7B: salary revision cycles + the bonus
> engine.** New package `internal/hrm/compensation` (15 files), migrations `00098`–`00099`, no
> new tables in payslips — 5 new tables here: `hrm_compensation_bands`, `hrm_merit_matrix_cells`,
> `hrm_salary_revision_cycles`, `hrm_salary_revisions`, `hrm_bonuses`. 20 new routes.
>
> **Bands are mutable catalog data, deliberately not append-only.** Unlike
> `hrm_employee_salary_records` (00023), editing a band tomorrow must not rewrite a decision made
> against today's numbers — so the compa-ratio a cycle computed is frozen into
> `calculation_snapshot` (mandatory, NOT NULL, no empty default) rather than re-derived from a
> band that can drift. Same reasoning gave `hrm_merit_matrix_cells` real, queryable rows instead
> of JSONB — computing a cycle means MATCHING a rating level and a compa-ratio against ranges,
> which is exactly the "slabs move from code into effective-dated rows" instruction Phase 7D's
> statutory engine will follow, applied here first.
>
> **A cycle carries ONE approval instance for the whole batch**, matching the build plan's
> "batch-approved" — an approver decides every proposed revision in one action, not a chain per
> employee. `HandleApprovalDecision` only flips `approved`/`rejected`; a separate `ApplyCycle`
> writes the `hrm_employee_salary_records` rows. That split is the
> `promotions.Apply`/`HandleApprovalDecision` precedent verbatim: a decision and the money
> movement it authorizes are never the same call.
>
> **Two CHECK constraints needed widening for `salary_revision`/`bonus`, not one, and they use
> different vocabularies.** `hrm_approval_templates.action_type` uses the short form
> (`'leave'`, `'promotion'`) while `hrm_approval_instances.entity_type` uses the long form
> (`'leave_request'`). Missing the template-side widening was caught immediately — reversibility
> testing rebuilt the migration before either DB kept the half-widened state.
>
> **`hrm_bonuses` gets a real consumer inside this same slice, not a future one.** The build plan
> names only compensation bands/cycles/bonuses for 7B, and payroll wiring only explicitly for
> loans in 7C — but an approved bonus nothing ever pays is the same defect shape as the dropped
> `hrm_employees.status` column (r25): a written record nothing reads, and this codebase has
> already found that class of bug three times. Resolved with `payslips.BonusSource` — a
> consumer-owned narrow interface (the `recruitment.EmployeeCreator` / `pip.TerminationCreator`
> shape: payslips is the CONSUMER and declares it; `hrm/compensation` imports `hrm/payslips`, and
> not the reverse, to satisfy it) — plus a new branch in `computePayslips`:
> `run_type='bonus'` skips the normal salary-structure computation entirely (which would
> double-pay basic salary alongside the bonus) and instead builds one payslip per employee holding
> approved-but-unpaid bonuses, one earning line each. `ComputeRun` marks the underlying bonuses
> `paid` — atomically, in one transaction across the whole batch — only after every payslip and
> line in the run has actually persisted, and a failure there aborts the whole run exactly like a
> failed payslip write (r25's `abortCompute`), rather than leaving payslips committed with their
> bonuses still `approved` and payable a second time by the next run. **Deduction-free by design,
> not omission**: statutory withholding on a bonus payout is Phase 7D scope, which does not exist
> yet — paying a bonus net-of-nothing today is an honest description of what this run type does
> pending that engine, the same distinction r25's negative-net fix drew between a real zero and a
> masked one.
>
> **A real bug found by the integration tests, not by inspection**: `SubmitCycle` and `ApplyCycle`
> each built a bare `ListFilter{Limit: N}` to check/enumerate a cycle's own revisions. `ListFilter`
> zero-values `Scope` to `authz.ScopeNone` (documented on the type: "means no rows"), so both
> internal existence/aggregate checks silently saw ZERO revisions regardless of how many existed —
> `SubmitCycle` refused every real cycle with `ErrCycleHasNoRevisions`, and had it not, `ApplyCycle`
> would have looped over nothing and written no salary records at all, succeeding while doing
> nothing. Fixed by setting `Scope: authz.ScopeAll` explicitly on both — an internal aggregate
> operation is not a caller-scoped read. Caught because the integration tests exercise the full
> `CreateCycle → ComputeCycle → SubmitCycle → ApplyCycle` path end-to-end against real data, not
> because anyone spotted the zero-value trap by reading the code.
>
> **Verification:** every architecture guard green, 5 new unit tests (pure arithmetic —
> `ApplyIncrease`, `ComputeCompaRatio`, `Revision.PctIncrease` — the `payslips.ComputeSlab` /
> `ReferencesGross` precedent: money arithmetic gets tested before anything that calls it, since
> `ComputeCycle` itself reaches `*pgxpool.Pool` directly and is only integration-testable), 10 new
> integration tests (137 total, up from 127), migration reversibility proved down×2/re-up with
> zero tables / zero permission rows / zero role grants, and a live HTTP smoke run against a real
> seeded employee: banded + rated → cycle computed a 15000→15900 increase (6% matrix cell) →
> submitted (no template configured, auto-approved fallback) → applied → confirmed as a real
> `hrm_employee_salary_records` row → a bonus computed against the NEW basic pay (the snapshot
> correctly picked up 15900) → submitted → paid out through a `run_type='bonus'` payroll run →
> confirmed the bonus flipped to `paid` with real `payslip_run_id`/`payslip_line_id`. Test data
> cleaned up afterward, including reverting the applied salary record.
>
> **Not built in this slice, staying open for 7C/7D as scoped:** `OverrideRevisionRequest` has no
> public `is_excluded` toggle yet — the exclusion path exists and is tested, but only reachable by
> a direct DB write today; a dedicated exclude/include endpoint is a 7C-or-later follow-up, not a
> defect. Loan recovery and reimbursement lines, statutory withholding on bonus payouts, and
> benefits enrollment deductions are unbuilt, as scoped — 7A's `line_type` ordering exists
> specifically so they slot in without re-deriving anyone's pay.
>
> ---
>
> r25 — **HRM Extended Phase 7A: the payroll engine — plus four money
> defects found in the existing engine and fixed before any feature work landed on it.**
> Migrations `00096`–`00097`, no new packages, 1 new route (389 HRM routes total).
>
> **Phase 7 began by finding that payroll did not work at all.** Exploration of
> `internal/hrm/payslips` — a module marked ✅ DONE since r19 — turned up four separate defects,
> every one of them in money arithmetic, and every one invisible to the existing tests for the
> same structural reason: `ComputeRun` had **no integration test whatsoever** and reached
> `*pgxpool.Pool` directly, so the stub-repo unit tests could not see it. This is the fourth
> appearance of that pattern (the terminations `status_id` bug, r23; the `ScopeOwn` bug, r24).
>
> **Defect 1 — payroll selected employees on a column that no longer exists.** The employee
> query filtered `WHERE e.status IN ('active','on_leave','resigned')`. `hrm_employees.status` was
> replaced by `status_id` → `hrm_employee_statuses` in migration `00053`; the column is gone, so
> the query errored and **no payroll run could compute for any org**. `'resigned'` was never a
> valid value even on the original `00021` CHECK (`active|inactive|on_leave|terminated`), so the
> filter was wrong twice over. The same dropped column was found in four further raw-SQL sites —
> `calendar` and `announcements` (`GetTargetEmployeeIDs`) and three headcount sub-selects in
> `reports`. Fixing it required deciding **who gets paid**, not merely translating the old
> predicate: active + on-leave + employees terminated mid-period (`termination_date >=` the period
> start), so a leaver is paid for the days they worked.
>
> **Defect 2 — component order silently changed everyone's pay.** `pct_of_gross`, `formula` and
> `slab` components all read a `gross` variable that the single compute loop was still
> accumulating, so a gross-dependent component evaluated third saw only the first two components'
> gross. Reordering rows in the salary-structure UI changed real net pay. Proved by
> re-introducing it: **15000 vs 13500, an 11% swing from a `display_order` edit.** Replaced with
> an explicit three-stage computation — earnings independent of gross define gross; earnings
> expressed as a share of gross evaluate against that; everything else evaluates against the
> final gross — with lines still emitted in display order. `ReferencesGross` is an exported,
> separately-tested predicate (14 cases) rather than an inline condition, because "does this
> component depend on gross" is now load-bearing for three calculation methods.
>
> **Defect 3 — negative net pay was silently zeroed.** `if netPay.IsNegative() { netPay =
> decimal.Zero }`, with the same clamp on individual line amounts. Deductions exceeding gross
> made the shortfall vanish with no error and no record, leaving a payslip whose lines total more
> than its own gross. The build plan asks for a "negative-net guard"; what existed was a
> silencer. Now the true figure is stored — it is a legitimate outcome, and F&F runs depend on it
> (Section 9) — and `ApproveRun` refuses the run while any payslip in it is negative
> (`ErrNegativeNetPay` → 409 `NEGATIVE_NET_PAY`). The count query lives on `Repository`, not on
> `s.db`, specifically so a unit test can reach it.
>
> **Defect 4 — a partially-written run reported success.** A failed `CreatePayslip` was answered
> with `continue` and a failed `CreatePayslipLines` with `_ =`, so an employee could end up
> unpaid, or paid with a gross and a net but no lines explaining either — while the run was
> marked `computed`. The run's own header concealed it: `TotalEmployees` counted every employee
> while the money totals counted only the ones that saved, so the run looked complete and merely
> disagreed with itself. A payroll run is now all-or-nothing: any persistence failure aborts,
> **deletes the payslips already written** (without which the retry would insert a second set
> alongside the first), returns the run to `draft`, and surfaces the error. Cleanup failures are
> `errors.Join`-ed onto the cause rather than replacing it.
>
> **7A itself — the payroll engine.** `hrm_payslip_runs.run_type`
> (`regular`/`off_cycle`/`bonus`/`arrears`/`fnf`) and `hrm_payslip_lines.line_type` +
> `is_employer_contribution` + `source_period_id`.
>
> **`uq_hrm_pr_org_month` had to be replaced or `run_type` was stillborn.** It is
> `UNIQUE (org_id, period_year, period_month)` — one run per org per month *of any type* — so the
> column could never have held a second value in a period. It is now a PARTIAL unique index over
> regular runs only: exactly one regular run per org per month still holds, while off-cycle,
> bonus, arrears and F&F runs are legitimately repeatable, because a leaver's final settlement
> does not wait for next month. The Down block restores the original constraint and will **fail
> loudly** if the org has since created two runs in a month, rather than silently discarding one.
>
> **`line_type` overlaps `component_type` and both are kept, deliberately.** `component_type`
> snapshots what the COMPONENT was; `line_type` records what the LINE DOES in the calculation.
> They diverge the moment a line has no component behind it — a loan recovery (7C), a statutory
> deduction from a rule (7D), an arrear from an earlier period. Same reasoning for
> `is_employer_contribution`, which is only *partly* derivable from
> `component_type='employer_contribution'`: 7D's statutory employer contributions are generated
> from rules and carry no component row at all. Both migration headers say so, because the
> obvious next move is to "deduplicate" them.
>
> **The mandatory dry-run preview.** `POST /runs/:runId/preview`, gated on a new
> `hrm.payroll.preview`. It shares `computePayslips` with `ComputeRun` rather than having a
> "preview mode", so a preview cannot disagree with what approval later commits — proved live:
> preview and compute returned identical figures (18 employees / 71870 gross / 57466 net) with
> zero rows written between them. `.preview` is its own permission key rather than a reuse of
> `.compute` because preview is read-shaped, so it is safe to grant to `manager`, who already
> holds `hrm.payroll.view`; reusing `.compute` would have meant anyone allowed to check the
> numbers was also allowed to commit them, inverting the point of a dry run.
>
> **⚠ Note the scope asymmetry on `hrm.payroll`,** since it reads wrong at a glance: the resource
> IS scope-tiered (`view_own`/`view_team`/`view_all`, seeded `00072`), and those tiers govern
> which PAYSLIPS a caller sees, because payslips carry an `employee_id`. RUNS are org-level
> objects with no `employee_id`, so `.preview` is an untiered org-wide capability — which is
> exactly why it is granted narrowly instead of to `member`.
>
> **r24's open defect #1 is now CLOSED.** `scope.Predicate`'s `ScopeOwn` emitted
> `employee_id = (SELECT …)` against a non-unique index, so one user with two employee rows in an
> org 500s every `view_own` list in all six scope-tiered modules. Fixed to `IN`, with an
> integration test that creates the duplicate state and asserts the query returns rows instead of
> SQLSTATE 21000.
>
> **Verification:** every one of the four defects was proved by reverting it and watching the
> test go red with the literal failure — the discipline that validated the r23 terminations fix.
> The replaced unique index was proved the same way, by re-imposing the original constraint (it
> could not even be created against the test's data, which is itself the proof). 14 new
> integration tests, 14 new unit tests, all six architecture guards green, migration
> reversibility proved down×2 → columns gone / original constraint restored / zero permission
> rows / zero role grants → re-up, and a live HTTP smoke run with test data cleaned up
> afterwards. Full integration suite: **127 tests**.
>
> **Still open, unchanged:** the scheduler's manual-trigger endpoint returns 400
> `NO_BUSINESS_CONTEXT` for every job (its route has no `:orgId` but its permission gate requires
> one); a fresh API-created org has no `hrm_employee_statuses`, so `POST /hrm/employees` 500s
> until one is created by hand. Also unchanged and **not** fixed by this phase: `ComputeSlab` and
> `evalFormula` still operate on `float64`. The `TestHygiene_NoFloatMoneyFields` guard only
> inspects `*model.go`, so they pass it — new money paths must not widen that hole.
>
> ---
>
> r24 — **HRM Extended Phase 6 COMPLETE: Learning & Development.**
> Sliced two ways and both shipped: 6A the LMS core, 6B certifications + the shared skills
> taxonomy + the expiry sweep. Migrations `00092`–`00095`, three new packages
> (`internal/hrm/learning`, `internal/hrm/skills`, `internal/hrm/certifications`), 47 new routes.
>
> **The build plan was wrong on one load-bearing point, and it reshaped the phase.** It says
> *"assessments reuse Phase 5's form engine — separate DTOs: `QuestionForAttempt` never carries the
> correct answer"*, but `platform_form_questions` has **no correct-answer column and no pass mark**.
> The engine's `computeScore` normalises each answer 0-1 against its own scale and weights it: a
> RATING score ("how highly did you rate this"), not an ASSESSMENT score ("did you get it right").
> The correct answer had nowhere to live.
>
> Resolved with a Phase 6-owned `hrm_quiz_answer_keys` keyed on `platform_form_questions.id`.
> `internal/platform/forms` keeps zero assessment semantics, so appraisals and 360 feedback never
> carry a `correct_answer` column they do not read — and the "never leak the answer" rule becomes
> **structural rather than disciplinary**: the attempt read path does not join the key table, so
> there is no field to forget to strip. Exactly the shape 5C used for 360 anonymity.
>
> **Consequence: grade once at submit, store the result, never re-derive.**
> `platform_form_responses.question_id` is `ON DELETE SET NULL` (documented in `00084` as
> "provenance only, never joined for display" — the question snapshot lives on the response row),
> and `hrm_quiz_answer_keys.question_id` is `ON DELETE CASCADE`. So deleting a question destroys
> the key AND severs the response's link to it; a re-grade would silently score zero. An
> integration test deletes a question after grading and asserts the stored score does not move.
>
> **6A — the LMS core.** `internal/hrm/learning/` (29 routes), eight tables. Content hangs off
> `hrm_course_versions`, not the course, and `hrm_enrollments` **pins `version_id`** with a
> RESTRICT FK: publishing a new version leaves an existing learner on the content they actually
> took, and the pinned version cannot be deleted from under them. Only a DRAFT version is
> editable — every content write passes one `assertEditableVersion` gate, because the one place
> that check gets forgotten is the one that corrupts a published version. Completion percentage is
> COMPUTED from `hrm_lesson_progress` on every read (the `00076` rule); an integration test
> introspects `information_schema` to assert no percentage column exists to drift. A quiz lesson is
> completed by PASSING an attempt, never by asserting completion — otherwise the assessment is
> optional, which is the same as absent. Grading has its own arithmetic (32 unit tests written
> before any dependent layer): multi-select partial credit is `(hits − misses) / expected` floored
> at zero, so selecting every option scores zero rather than full marks.
>
> **6B — certifications, skills, and the sweep.** `internal/hrm/certifications/` (9 routes) and
> `internal/hrm/skills/` (9 routes), five tables. Separate packages because `learning`'s composite
> Repository hit 53 methods in 6A against the ~60 split threshold Phase 5A recorded, and because
> skills is explicitly a SHARED taxonomy — Phase 10 succession imports it directly rather than
> reaching through an LMS dependency.
>
> The **expiry sweep** is what the build plan calls the highest-value feature in the phase. It is a
> `scheduler.Register("certifications.expiry_sweep", "0 4 * * *", …)` job, instance-wide like the
> leave and absence sweeps. Two passes in a fixed order — mark `expiring` within 30 days, THEN mark
> `expired` — because reversing them would flag something that has already lapsed as a warning. The
> boundary is strict: `expires_at < CURRENT_DATE`, so a credential expiring TODAY is still valid
> today; `<=` would cut somebody off a day early, which for a safety certification is a real
> operational error. `expiry_notified_at` stops the job re-flagging the same credential nightly.
> All three properties are pinned by integration tests.
>
> **`hrm_position_skills` was deliberately NOT built**, against the build plan's own list. Skills a
> POSITION requires has no reader until Phase 10, and recruitment and performance were both grepped
> and contain zero skills fields — there is nothing to retrofit into. Building it now is precisely
> the speculative primitive rule 1 exists to prevent. `hrm_skills` and `hrm_employee_skills` DO
> earn their place, via a real in-phase consumer: issuing a certification that carries a skill
> records that skill, with `source='certification'` and a pointer back to the credential.
>
> `hrm_acknowledgements.acknowledgeable_type` gained `'course_completion'` — the same sanctioned
> widening 5B made for `'appraisal'`, and one the build plan scheduled by name.
>
> **Verification:** 55 new unit tests (32 grading + 23 service), 19 new integration tests, all six
> architecture guards green, migration reversibility proved down→zero tables / zero permission rows
> / **zero role-array grants** / ack CHECK reverted→re-up, and two live HTTP smoke runs (22-step
> 6A, 13-step 6B) with test data cleaned up afterwards. Full integration suite: **112 tests**.
>
> **⚠ Two defects found during this phase, NEITHER caused by it, both still open:**
>
> 1. **`scope.Predicate`'s `ScopeOwn` breaks if one user has two employee records in an org.** It
>    emits `employee_id = (SELECT id FROM hrm_employees WHERE org_id=$1 AND user_id=$2)`, and
>    `idx_hrm_emp_user_id` is **not unique** — two rows make the subquery fail with SQLSTATE 21000
>    ("more than one row returned by a subquery used as an expression"). Every `view_own` list in
>    all SIX scope-tiered modules (goals, appraisals, feedback, PIPs, enrollments, certifications,
>    skills) 500s for that org. Found by accidentally creating the state in a test. The fix is
>    one character — `=` → `IN` — but it touches a file five prior phases depend on, so it is
>    reported rather than changed here.
> 2. **The scheduler's manual-trigger endpoint is unusable for every job.**
>    `POST /platform/scheduler/jobs/:name/run` carries no `:orgId`, but its `permFn` gate requires
>    one, so it returns 400 `NO_BUSINESS_CONTEXT`. Verified against a PRE-EXISTING job
>    (`leave.accrue_and_snapshot`), so this is not new. `internal/platform/checklists/routes.go`
>    already flags the no-`:orgId` scheduler shape as a pattern not to copy.
>
> Also still open from r23: a fresh API-created org has no `hrm_employee_statuses`, so
> `POST /hrm/employees` 500s until one is created by hand.
>
> ---
>
> r23 — **HRM Extended Phase 5 COMPLETE.** Phases 5B and 5C shipped,
> closing the three-way slice recorded in r22: 5A Goals/OKR (r22), 5B form engine + appraisal
> cycles, 5C 360 feedback + PIP. Migrations `00084`–`00091`, three new packages
> (`internal/platform/forms`, `internal/hrm/feedback`, `internal/hrm/pip`), 78 new routes.
>
> **5B — the form engine plus its first consumer.** `internal/platform/forms/` (17 routes) follows
> the `internal/platform/checklists/` template exactly: polymorphic subject with no FK so the
> platform never references `hrm_*`, definition snapshotted onto each instance as REAL COLUMNS not
> JSONB (migration `00076`'s rule: rows that get aggregated are rows), a response row per question
> created at instantiation so answering is an UPDATE, and **no generic instantiate route** — a
> generic route would have to trust a client-supplied subject id, which is an impersonation vector.
> Appraisals (`00086`/`00087`, 21 routes inside `internal/hrm/performance/`) are its first real
> consumer, shipped in the same phase so the primitive is never speculative.
>
> The appraisal phase machine (`draft → self_review → manager_review → calibration → published →
> acknowledged`) is **the first explicit transition map in this codebase**, and that was a decision,
> not drift: every other state machine here is inline `if x.Status != Expected` guards, but 6 phases
> with per-transition guards would have accumulated ~15 of them. `allowedPhaseTransitions` is the
> single source of truth and `IsTerminal()` is derived from it, so the two can never disagree. Two
> transitions deliberately run BACKWARD (`manager_review → self_review`, `calibration →
> manager_review`) to make a rejected review recoverable without cancelling it — and the
> send-back must NOT re-demand the self form, which is why the precondition checks
> `a.Phase == PhaseSelfReview` rather than only the target.
>
> `final_rating` is a structured queryable FK (`final_rating_level_id → hrm_rating_scale_levels`)
> **and** a `label`+`value` snapshot. Not redundancy: the FK gives Phase 7's merit matrix and Phase
> 10's 9-box something to query, the snapshot survives a level being renamed or deleted (`ON DELETE
> SET NULL`, proved by integration test). Publish **snapshots** the self/manager scores and goal
> attainment rather than recomputing them — an immutable record whose numbers derive from mutable
> sources is not immutable, and Phase 5A goals stay editable forever.
>
> **5C — 360 feedback + PIP, in their own packages.** The 5A package doc set a split threshold of
> ~60 methods on `performance`'s composite Repository; it reached 58 after 5B, so 5C went to
> `internal/hrm/feedback/` (12 routes) and `internal/hrm/pip/` (9 routes) rather than becoming a
> sixth and seventh quartet.
>
> **The 360 anonymity contract is the part worth reading.** This codebase already contains one
> cautionary example of anonymity documented but never implemented: `hrm_complaints.is_anonymous`
> is a stored boolean carrying a `COMMENT` that promises identity hiding, which nothing in the
> codebase branches on — grep it. Three structural decisions keep 5C from becoming the second:
> (a) anonymity is **derived**, not stored — `Relationship.IsAnonymous()` is the single source of
> truth, so there is no column to set to a value that lies; (b) identity and content are returned
> by **different repository methods with different types sharing no field** — the coordination query
> selects no answer column, the content query selects no identity column, and neither type is the
> other with fields blanked; (c) a form instance id is **never** handed to a subject, because
> `platform_form_instances` stores `respondent_user_id` and an id plus `GET /forms/instances/:id`
> defeats everything else. That third one is the leak that lives outside the module.
>
> Two corollaries that look like oversights and are not. **`self` and `manager` feedback is
> attributed by nature** — a subject knows what they wrote and knows who their manager is; there is
> exactly one manager, so "anonymous manager feedback" identifies them with certainty while
> pretending otherwise, and suppressing it below a threshold it can never reach would only make the
> most actionable feedback in the cycle unreadable. **Suppression is per relationship group, not
> cycle-wide** — five responses of which one is a direct report still identify that direct report
> the moment their breakdown renders. A suppressed group reports only that it is suppressed, not its
> count, and `TotalResponses` excludes it so the hidden size cannot be recovered by subtraction.
> Suppression applies to **every** tier including `view_all`: a promise of anonymity that is false
> for one role is false.
>
> **PIP hands off to terminations and stops.** A `failed` outcome creates a **DRAFT** termination
> and nothing more — `hrm_terminations` already owns a `draft → pending_approval → approved →
> applied` lifecycle with an approval chain, and a PIP that advanced past draft would route around
> the control that exists specifically to gate dismissals. Proved live: the integration test asserts
> the row is at status `draft` AND that zero approval instances exist. The seam is
> `pip.TerminationCreator`, declared in `internal/hrm/pip` with `terminations` importing `pip` — the
> consumer-owned narrow interface direction, matching `recruitment.EmployeeCreator` exactly. The
> close is deliberately NOT atomic with the handoff: the PIP commits first, and a handoff failure
> returns the closed plan alongside `ErrTerminationHandoff` rather than a bare error, because the
> opposite ordering risks a dismissal document with no process behind it.
>
> `hrm.pips.close` is a separate permission from `hrm.pips.manage` and `manager` holds only the
> latter — closing as `failed` is the moment the instrument stops being developmental, the same
> reasoning that keeps `hrm.appraisals.publish` away from `manager`. Same shape in feedback:
> `.coordinate` (who was asked) and `.view` (what was said) are separate keys and **no role holds a
> key yielding both**.
>
> **Verification:** 45 new unit tests (34 appraisal, 19 feedback, 21 pip — counted across both 5B
> and 5C), 24 new integration tests against real Postgres, all six architecture guards green,
> migration reversibility proved down×4 → zero tables / zero permission rows / **zero role-array
> grants** (the residue easiest to miss) → re-up, and two live HTTP smoke runs (22-step 5B, 18-step
> 5C) with test data cleaned up afterwards.
>
> **Two pre-existing defects surfaced by the smoke runs, neither caused by Phase 5, both still
> open:** (1) a fresh API-created org has NO `hrm_employee_statuses` rows, so `POST /hrm/employees`
> returns a bare 500 (`fetch default status: no rows in result set`) until a status is created by
> hand — the same seeding gap the r22 exit-path fix had to work around; (2) `authz
> .GetUserPermissions` surfaces a malformed org id as a 500 (`invalid input syntax for type uuid`)
> rather than a 400.
>
> ---
>
> r22 — HRM Extended Phase 5A shipped: Performance Management / Goals &
> OKR, plus a standalone fix for two broken employee-exit paths. **Phase 5 as written in the build
> plan bundles a form engine plus four sub-systems (~18-19 tables, ~2.5× Phase 4A) and was sliced
> three ways: 5A Goals/OKR, 5B form engine + appraisal cycles, 5C 360 feedback + PIP.** Goals went
> first specifically because it is the ONLY sub-system with no form-engine dependency — goals are
> structured numeric data, not questionnaires — so the primitive is not built speculatively. It
> lands in 5B alongside appraisals, its first real consumer, exactly as Phase 3's checklist engine
> shipped with onboarding rather than ahead of it. This honours the build plan's own governing rule
> ("nothing speculative — a primitive gets built when its first real consumer is queued next")
> against its own prose ordering, which lists the engine first.
>
> **Bug fix first (independent of Phase 5, found while checking a 5C dependency):** `terminations
> .Apply` AND `resignations.Accept` both wrote `hrm_employees.status`, a column migration `00053`
> dropped when it replaced the text status with a `status_id` FK. Both failed outright with
> SQLSTATE 42703 and rolled back, so applying a termination or accepting a resignation silently did
> nothing — in two modules marked ✅ DONE. No test caught it because both use raw SQL through
> `s.db.Begin(ctx)`, bypassing the `Repository` interface the stub-repo unit tests mock — precisely
> the gap `leave/balances_service_test.go` already documents. The naive fix would have been silently
> wrong: `GetDefaultStatusID` resolves by category with `ORDER BY created_at ASC`, and `00053` seeds
> TWO rows in the `terminated` category ('Resigned' before 'Terminated'), so a termination would
> have labelled the employee resigned. Both paths now resolve by name-within-category with a
> fallback, inline in each service's own transaction, raising `ErrNoTerminatedStatus` /
> `ErrNoResignedStatus` **inside** the transaction so a missing status rolls the whole operation
> back. 4 integration tests (`hrm_employee_exit_test.go`), verified in both directions — stashing
> only the two `service.go` files confirmed all four fail against the original code with the exact
> 42703.
>
> **Phase 5A itself:** migration `00082` adds `hrm_goal_cycles`, `hrm_goals`, `hrm_goal_checkins`;
> `00083` seeds 8 permission keys. New package `internal/hrm/performance/` (19 routes,
> recruitment-shaped quartets so 5B's appraisals land as more files, not a new package). Six
> decisions worth recording:
> **(1)** `parent_goal_id` is `ON DELETE SET NULL`, not `CASCADE` — the build plan says "self-FK
> cascade", but that is the OKR domain term (cascading alignment), not DDL: the same sentence
> demands check-in history "from day one", and CASCADE is the one choice that destroys the most of
> it. `hrm_departments.parent_department_id` (00021) is the existing SET NULL precedent.
> **(2)** `parent_goal_id` means alignment ONLY — no progress roll-up into a parent's stored value.
> Beyond being the denormalized drifting counter `00076` forbids, roll-up would break 5B: appraisals
> are publish-immutable, so a subordinate back-dating a check-in would mutate the inputs of an
> already-published appraisal.
> **(3)** The weight rule is `≤ target` at write time (requiring `== target` would make creating the
> first goal impossible); `== target` is enforced only at cycle lock. **Enforcement lives in the
> repository, not the service**, correcting the build plan's "weight-sum validation in service
> layer" as literally written — a service read-then-write loses to two concurrent requests, and
> `FOR UPDATE` on sibling goals does NOT help because the competing transaction INSERTs a row that
> was in neither locked set. `CreateGoalGuarded` locks the **employee row** instead, then issues the
> SUM as a separate statement so it takes a fresh READ COMMITTED snapshot. (`SELECT SUM(...) FOR
> UPDATE` is not even legal Postgres.) Proved with 8 concurrent goroutines in integration.
> **(4)** `hrm_goal_cycles` is its own table and is NOT merged with 5B's appraisal cycles — the
> lifecycles differ, cardinality is not 1:1, and 5B snapshots a rating scale a goal cycle has no use
> for. `hrm_attendance_periods` is the precedent for a period with a lock lifecycle.
> **(5)** An out-of-scope alignment parent returns a title-only `GoalRef` — **a distinct type, never
> a trimmed `*Goal`**, so no column added in 5B or 5C can leak a parent owner's performance data
> through a reference. Hydrated only on the detail endpoint; a unit test asserts the type's shape.
> **(6)** Objective vs key result is discriminated by `goal_level` + `weight IS NULL`, **not** by
> `parent_goal_id IS NULL` — real OKR trees run company → department → individual → key results, so
> a null-parent test misclassifies every intermediate level.
> Progress is computed, never stored on `hrm_goals` (it IS stored on check-ins, which is different:
> an immutable historical value, the `hrm_leave_balances` snapshot distinction). One formula covers
> every measurement type because `start_value` makes it direction-agnostic — `direction` validates
> and drives UI but is deliberately NOT an input to the arithmetic, which is how the obvious
> implementation gets a decrease goal's sign wrong. `RawProgressPercent` is unclamped (overshoot is
> real); `ProgressPercent` clamps to [0,100] so one 130% goal cannot push 5B's weighted attainment
> off the top of the rating scale.
> Goals carry `employee_id`, so unlike recruitment they DO use the Phase 1 scope tiers — which makes
> the three tier keys mandatory, since `TestPermissions_ScopeTiersSeeded` is all-or-nothing.
> `hrm.goals.manage` never appears in a `permFn(...)` call: the route gate cannot express "is this
> your own goal", so writes are gated on `set_own` and the service narrows — own goal needs nothing
> more, another's needs manage AND passing `AuthorizeRecordAccess` (the easily-omitted half that
> stops a `view_team` manager editing outside their reporting line).
> 22 unit tests + 9 integration tests + a 10-step live HTTP smoke test. Migration count 81 → 83,
> table count 98 → 101, HRM MODULE route count 277 → 296.)

> Last updated: 2026-08-05 (r21 — HRM Extended Phase 4B shipped: Recruitment / ATS, selection and
> close half — completes the build plan's Phase 4 minus the public `/pub/careers/*` surface, which
> stays deferred (EMAIL SENDING and Capture Fix Pass B were re-verified still not done; see r20's
> entry). Five tables in migration `00080`: `hrm_interviews` (`outcome` is a recommendation signal
> only — never auto-moves the application's pipeline stage, the recruiter still calls the existing
> `MoveApplication` explicitly), `hrm_interview_panelists`, `hrm_interview_scorecards` (fixed-shape,
> deliberately not a form engine — Phase 5 builds the real primitive and names scorecards as
> consumer #1; `submitted_at` is the only status field, NULL = draft, set = immutable), `hrm_offers`
> (approval-gated exactly like requisitions — added `'offer'` to both `hrm_approval_templates
> .action_type` and `hrm_approval_instances.entity_type` CHECK constraints, the same two-constraint
> pattern `00049` and `00078` used, plus `hrm_employee_documents.related_type`), and `hrm_referrals`
> (the formal bonus-program lifecycle, distinct from `hrm_candidates.referred_by_employee_id`'s
> lightweight provenance). Plus `hrm_employees.source_candidate_id` (`ON DELETE SET NULL`) for hire
> provenance. Migration `00081` seeds 7 permissions; `hrm.interviews.scorecard` is granted through
> `member` (unlike every other key here) then narrowed by the service to actual assigned panelists —
> the `platform.checklists.complete` precedent, since the route gate cannot express "is this your
> panel assignment." Scorecard visibility ("interviewer cannot see others' scores before submitting
> their own," named directly in the build plan) is a bespoke service-layer rule, confirmed again
> unexpressible with Phase 1 scope tiers (`internal/hrm/scope`'s `Predicate`/`AuthorizeRecordAccess`
> hard-code `FROM hrm_employees`, state-independent) — a panelist who hasn't submitted their own
> scorecard sees only their own (possibly empty) draft; everyone else, including a non-panelist
> admin auditing after the fact, sees every *submitted* scorecard, never a draft. Hire→employee
> conversion (`recruitment.HireApplication`) mirrors `crm/leads.ConvertLead` exactly: `employees
> .Repository.CreateTx`/`Service.CreateEmployeeTx` mirror `contacts.CreateContactTx`'s tx-scoped
> shape, and `recruitment.EmployeeCreator` is a consumer-owned narrow interface — `recruitment`
> declares the interface and its own `HireEmployeeRequest` type, `employees` imports `recruitment`
> to implement it, never the reverse (`ContactCreator`/`DealCreator`'s exact shape). One design flaw
> caught during implementation, not planning: the plan's sketch had `CreateEmployeeTx` fire the
> onboarding checklist hook itself — wrong, because that method runs *inside* the caller's
> transaction, before it's known whether the transaction will actually commit. Split into
> `CreateEmployeeTx` (pure insert, zero side effects) and `AfterHireCommit` (audit log + checklist
> hook), with the orchestrator calling the latter only after its own `tx.Commit()` succeeds. Atomic
> for the employee insert + application's `converted_employee_id` + requisition's `filled_count`
> (one transaction via a new `Repository.BeginTx`, the `crm/leads` precedent); onboarding
> instantiation stays a post-commit best-effort hook, identical to plain `employees.Create`'s
> existing behaviour — forcing it into the same transaction would require `Tx` variants throughout
> `internal/platform/checklists` too, out of scope here. A `SELECT ... FOR UPDATE` row lock (mirrors
> `MoveApplicationStage`) makes concurrent hire calls on the same application safe — proved live by
> firing 5 concurrent `HireApplication` calls at one application in an integration test: exactly one
> succeeds, exactly one employee row exists. Hire conversion reuses the existing `hrm.candidates
> .manage` permission (an application-lifecycle action, consistent with move/reject/withdraw) —
> no new key, to avoid permission sprawl. 16 new unit tests (34 total for the package), 5 new
> integration tests (13 total) — including the concurrency proof above, a forced employee-insert
> failure (bogus `department_id`, violates a real FK) proving nothing partial gets written, the
> `source_candidate_id` FK round-tripping through `ON DELETE SET NULL` when the candidate is later
> deleted, the scorecard panelist unique constraint firing on a raw duplicate insert, and a real
> offer-approval decision flipping status through the actual `RegisterCallback("offer", ...)` wiring.
> Live HTTP smoke test through the dev server exercised the full path end-to-end, including the
> scorecard visibility rule proven over real requests (owner sees an empty list while the only
> scorecard is a draft; sees it once the panelist submits). 25 new routes, all under existing
> `hrm.interviews.*` / `hrm.offers.*` / `hrm.referrals.*` / `hrm.candidates.manage` gates. Migration
> count 79 → 81, table count 93 → 98, HRM MODULE route count 252 → 277.)

> Last updated: 2026-08-05 (r20 — HRM Extended Phase 4A shipped: Recruitment / ATS, intake and
> pipeline half only. The build plan's Phase 4 names 12 tables plus a public `/pub/careers/*`
> surface, gated on two hard prerequisites — EMAIL SENDING and Capture Fix Pass B. Both were
> verified NOT actually done despite doc drift claiming otherwise: `RESEND_API_KEY` is absent from
> every env file and `notifications/service.go` silently no-ops (records "sent") when the key is
> empty; no `/pub` route has rate limiting and `NewPublicCaptureRateLimit` does not exist. Rather
> than build an unauthenticated public apply endpoint ahead of its own security prerequisites,
> this phase ships 7 tables, internal-only, no `/pub` routes, no candidate email — the public
> surface and email become an explicit follow-on (Phase 4B, unbuilt, sketched in the plan file for
> continuity). New package `internal/hrm/recruitment/` (17 files, `leave`-style filename-prefixed
> sub-features rather than a subpackage split): `hrm_recruitment_pipelines`/`_stages` mirror
> `crm_pipelines`/`crm_pipeline_stages` but fix a real defect — CRM's `is_default` has no partial
> unique index, so nothing stops an org ending up with two defaults or zero; this module adds
> `uq_hrm_rpipe_default` and an atomic clear-then-set service path. `hrm_job_requisitions`
> (approval-gated — required altering both `hrm_approval_templates.action_type` and
> `hrm_approval_instances.entity_type` CHECK constraints to add `'job_requisition'`, the same
> two-constraint pattern migration `00049` used for `'award'`) and `hrm_job_postings` (carries
> `public_slug` now, unread until the public phase — writing it later would be a pointless ALTER).
> `hrm_candidates` fixes another real, currently-shipping defect: `crm_leads` has no index on
> `email` at all, dedup is app-level, case-sensitive, and racy; this table gets a real
> `UNIQUE (org_id, LOWER(email))` partial index. `hrm_applications` — stage lives on the
> application, never the candidate — has `created_by` nullable from day one specifically because
> `crm_leads.created_by NOT NULL` is Capture Fix Pass A item 1's root cause (system-generated rows
> passing `""` and failing on an invalid-UUID error); this phase does not repeat that mistake and
> waits for a real public actor. `hrm_application_stage_history` ships in this first migration,
> append-only, with `seconds_in_previous_stage` computed inside the same locked transaction as
> each move — the table `crm_deals` never got, which is exactly why deal-velocity reporting is
> impossible today (`deals/service.go` overwrites `stage_id` in place and the prior value is gone).
> Resumes are stored in `backend/storage/resumes/` (new, gitignored alongside `backend/uploads/`),
> **not** `./uploads`, which `main.go` serves fully unauthenticated via `static.New` — content-
> addressed by SHA-256 (the avatar upload precedent), PDF-only (`http.DetectContentType`
> content-sniffed, never the file extension — DOCX sniffs as `application/zip` and accepting it
> would mean trusting the extension, the exact bug the avatar module's own comment says it fixed),
> behind a new `hrm.candidates.download_resume` permission separate from `.view` (the
> `hrm.leave.adjust_balance` precedent — the sharpest data gets its own gate). Deliberately **no**
> `authz.Service.ResolveScope` calls anywhere in this module: `internal/hrm/scope`'s `Predicate`
> and `AuthorizeRecordAccess` both hard-code `FROM hrm_employees`, and candidates/applications are
> not employees — the tiers are structurally inexpressible for them, so this module is flat RBAC
> only (`hrm.recruitment.*`, `hrm.candidates.*`) and every `hrm.candidates.view` holder sees every
> candidate in the org. One real bug caught only by a unit test — not integration, this one — a
> stub-repo test asserting `ErrStageNotInPipeline` (moving an application to a stage from a
> different pipeline) actually got `ErrStageNotFound` instead, because `FindStageByRef`'s query is
> pipeline-scoped, so a cross-pipeline stage lookup silently collapses into "not found" before the
> service's own pipeline-mismatch check can ever run. Fixed with a second, deliberately-unscoped
> `FindStageByRefAnyPipeline` used only by the stage-move path. 18 unit tests, 8 integration tests
> (including a live end-to-end approval decision through the real `RegisterCallback` wiring, and a
> resume tenant-isolation proof that org B cannot reach org A's candidate file). Migration count
> 77 → 79, table count 86 → 93.)

> Last updated: 2026-08-05 (r19 — HRM Extended Phase 3 shipped: the checklist engine (Section 9 →
> PLATFORM PRIMITIVES #4) plus its first consumer, HRM Onboarding. New platform package
> `internal/platform/checklists/` — 4 tables (migration `00076`): `platform_checklist_templates`
> (org-scoped, not global like `roles` — a global template can't express `owner_type='specific_user'`
> or an org-created role), `platform_checklist_template_items` (`owner_type` mirrors
> `hrm_approval_template_levels.approver_type`'s shape — `subject`/`manager`/`role`/`specific_user` —
> rather than literal hr/it/finance roles, which are not seeded system roles),
> `platform_checklist_instances` (polymorphic `subject_type`/`subject_id`, no FK — the
> `hrm_acknowledgements` precedent — so this package never references `hrm_employees` and stays a
> true platform primitive), `platform_checklist_instance_items` (column-level snapshot, not JSONB,
> since items are individually completed/skipped rows aggregated with `COUNT(*) FILTER`). Owner
> resolution happens once at instantiation via a caller-supplied `SubjectContext` value — this is
> also the security property behind a deliberate design choice: there is **no generic
> `POST .../checklists/instances` route**. Instantiation is reachable only through module-owned
> endpoints (here, `internal/hrm/onboarding/`) that resolve the subject server-side; a generic
> route would have to trust a client-supplied `subject_user_id`/`manager_user_id`, an impersonation
> vector. Two new `authz` methods (`RoleExists`, `UserRoleName`) reproduce the OR-join from
> `GetUserPermissions` (`internal/authz/repository.go`) verbatim — needed for role-owned "group
> claim" checklist items (nullable `assignee_user_id`, any holder of the named role may complete
> it; no role→users query ever runs). `internal/hrm/employees` gains a `ChecklistHook` interface
> and a breaking `NewService` signature (third param, nil-safe) — `Create` calls the hook
> synchronously after its existing audit log, with panic recovery inside the hook implementation so
> a bug in the checklist path can never fail employee creation, only log it. Migration `00077`
> seeds `platform.checklists.view/.complete/.manage`; `.complete` is granted broadly then narrowed
> per-item by the service (assignee, or matching role holder, or `.manage`) since the route gate
> cannot express "is this your own item." One real bug caught only by integration testing (not the
> 16 stub-backed unit tests, which share the same blind spot as the code under test): `FindTemplateItems`
> and `FindTemplateItemByID` joined `platform_checklist_template_items` to `platform_checklist_templates`
> with an unqualified column list — both tables have `id`/`public_id`/`created_at`/`updated_at`,
> so Postgres rejected the query as ambiguous (`42702`) despite every stub-backed unit test passing.
> Migration count 75 → 77, table count 82 → 86.)

> Last updated: 2026-08-04 (r18 — two HRM Extended phases shipped, neither previously folded into
> this document. **Phase 1 — Resource-Level Permissions**: `authz.Scope` type + `ResolveScope`
> method (`internal/authz`), plus a new `internal/hrm/scope` package (`Predicate()` for list-query
> WHERE-fragments, `Resolver.AuthorizeRecordAccess()` for GET-by-ID checks) — `view_own`/
> `view_team`/`view_all` layered on top of existing flat RBAC. `view_team` is a depth-parameterized
> recursive CTE over `hrm_employees.manager_id` (default depth 1, direct reports only) with an
> explicit path guard, since `manager_id` is a self-FK with no DB-level cycle prevention beyond
> blocking direct self-reference — proven safe against a real 3-node cycle via integration test.
> Migrations `00072`/`00073` seed the three tiers per resource per role and backfill custom
> roles/member overrides that held bare `.view` with no tier (a silent-regression risk otherwise:
> pass the route gate, then get zero rows with no error). Rolled out across all 12 employee-
> record-returning HRM modules plus salary's per-employee endpoints — every GET-by-ID now returns
> 403 `RECORD_ACCESS_DENIED` on an out-of-scope record instead of a silently wrong result. Also
> fixed while touching this area: `hrm.salary.employee` was missing from
> `frontend/src/lib/permissionGroups.ts`, and `PermissionForm.tsx` had its own hardcoded duplicate
> copy of the categorization instead of importing the shared one — same class of bug as the r10
> capture-permissions fix.
> **Phase 2 — Leave Engine Upgrade**: found during the Extended-plan audit, not on the original
> build list — shipped HRM had leave *requests* but no leave *balance*, and Phase 9 (Full & Final
> settlement) cannot compute encashment without one. Three new tables (migration `00074`):
> `hrm_leave_policies` (per-leave-type accrual method/rate, carry-forward cap, encashable flag —
> opt-in; a leave type with no policy behaves exactly as it did before this phase),
> `hrm_leave_transactions` (append-only signed ledger, the source of truth), `hrm_leave_balances`
> (immutable monthly snapshots, modeled on `hrm_employee_salary_records`'s single-`effective_date`
> pattern — the one actually-working "effective-dated" convention in this codebase, not the
> `effective_from`/`effective_to` one Section 4 describes). Two new scheduler jobs:
> `leave.accrue_and_snapshot` (daily) and `leave.year_end_carry_forward` (annual). Two new
> permissions, owner/admin only: `hrm.leave.adjust_balance`, `hrm.leave.encash` — balance/ledger
> reads reuse the existing `hrm.leave.view_own/team/all` tiers from Phase 1 with zero new seeding.
> `CreateRequest`/`ApproveRequest`/`CancelRequest`/`DeleteRequest` now post/reverse ledger
> transactions when a policy exists, via a real `pgx.Tx` bypassing `Repository` (mirroring
> `promotions.Apply`'s existing pattern) — approving past zero always succeeds, there is no
> balance-sufficiency gate anywhere in the write path, by design. One real bug caught by
> integration testing before it shipped: a snapshot boundary comparison used the wrong operator,
> silently dropping any transaction dated exactly on a snapshot's boundary date — fixed, and the
> test that caught it (comparing the checkpoint+delta read against a brute-force full-ledger sum)
> is now a permanent regression guard. Migration count 71 → 75, table count 79 → 82.)

> Last updated: 2026-08-03 (r17 — full-document drift audit, prompted by r16 having only touched
> Section 5. Read the whole doc against the whole source tree (three parallel audits: frontend,
> backend/database, mobile) rather than trusting any section's own status flags. Found drift in
> nearly every section:
> **Section 3** — frontend default theme is actually dark, not light (`app/layout.tsx`); GSAP is
> used across ~21 files, not "sparingly"; a 4th Zustand store (`commandStore`) was unlisted.
> **Section 4** — backend folder tree was missing `internal/hrm/` (the single largest module),
> `internal/platform/scheduler/` and `.../notifications/` (both shipped in r16), and all of
> `internal/dashboard/` — a real, wired, previously undocumented module.
> **Section 5** — added a DASHBOARD entry (flagged its `/orgs/:orgId/` route prefix breaking the
> app-wide `/organizations/:orgId/` convention); AUTH's route table was missing its alias routes
> and wrongly claimed mobile handlers weren't written (they are — see below); PLATFORM — CONTACTS
> was missing a `companies/enrich` route; CRM — DEALS documented the wrong param name for its board
> route (code and the code's own comment disagreed with each other); CRM — REPORTS was missing two
> routes (`rep-performance`, `forecast`).
> **Section 7** — folder tree was missing every file added in r16 (notifications) plus several
> that just predated this audit (`apikeys.ts`, `integrations.ts`, `visitors.ts`, `dashboard.ts`,
> a `crm/visitors` route, a `coming-soon` route). The Zustand store definitions had drifted hard:
> `permissionStore`'s real method is `hasPermission`, not the documented `can`/`canAny`; `authStore`
> has no `status` field and calls its membership field `currentMembership`, not `membership`; the
> "Permission Pattern" code sample called a `usePermission()` hook that doesn't exist anywhere in
> the codebase.
> **Section 8** — no entry existed for notifications (added one); CRM — CAPTURE said "zero
> frontend" but three of four planned pieces are actually built (`settings/apikeys`,
> `settings/integrations`, `crm/visitors`) — just under different paths than planned. The
> `capture.*` permission-group gap in `permissionGroups.ts` is real and still the one thing
> actually missing there.
> **Sections 9/14/15 — the big one.** MOBILE APP said "zero code written" / "nothing built yet."
> False: `mobile/` was committed 2026-07-23 (`a350092`, 93 files, +5799/-144) — one day after r14's
> own dated entry, never folded back in since. Real, working Expo app: auth screens, onboarding,
> dashboard shell, tasks, a single tabbed CRM screen (not split into per-entity routes as planned),
> flat settings. Real gaps that *are* still open: `forgotPassword`/`resetPassword` in `useAuth.ts`
> are stubs despite the pages and backend routes both being real; `components/{crm,tasks,rbac}/`
> are empty; no `security/` route; CRM — CONTACTS never got folded into the tabbed screen and is
> genuinely still unbuilt. Rewrote all three sections against source instead of the old plan.
> **Section 13** — `shopspring/decimal`, `robfig/cron/v3`, `resend/resend-go/v2` (all added in r16)
> were missing entirely; Expo/RN/expo-secure-store rows were still guesses (`57.0.7`, "latest
> compatible") — replaced with the confirmed versions from `mobile/package.json` now that it's
> known to exist.
> Nothing in Section 6, Section 10, or the CAPTURE/HRM entries already fixed in r16 needed further
> changes — re-verified, not re-assumed.)

> Last updated: 2026-08-03 (r16 — HRM Extended Phase 0 completed. Phase 0 had been reported
> substantially done, but a source audit found the branch didn't compile (three packages —
> `hrm/payslips`, `hrm/salary`, `platform/scheduler`, `platform/notifications` — imported
> `shopspring/decimal`, `robfig/cron/v3`, `resend-go/v2` that were never added to `go.mod`), and
> several stale test doubles no longer matched the interfaces they were meant to satisfy. Fixed
> both, then closed every remaining gap: **PLATFORM — SCHEDULER** and **PLATFORM —
> NOTIFICATIONS** promoted from scoping entries to real Section 5 modules (new migrations
> `00067`/`00068`); the two HRM crons already wired to the scheduler were both silently broken
> (milestones queried a column dropped in `00053`; attendance's `resolveShift` queried columns
> that don't exist, `wsa.scope`/`wsa.entity_id` instead of `assignee_type`/`assignee_id`) — both
> fixed, plus a real `attendance.absence_sweep` written to replace the previous stub; sentinel
> system user added (`00069`) so scheduler-triggered writes have a valid actor; PREP MIGRATIONS
> closed out (`00070` legal entity scaffold, `00071` currency columns) confirming
> `hrm_employees.manager_id` already existed under a different name than the doc assumed;
> notifications given a real API surface (list/mark-read/mark-all-read/preferences) and a
> frontend bell + drawer, replacing the placeholder icon. Guard tests extended: hygiene test now
> also catches AI-conversation comment artifacts (found and removed two real instances); the
> permissions test's bidirectional check — used-string-exists-in-a-seed-migration — is now
> codebase-wide instead of HRM-only, and immediately caught a real production bug on the first
> run: `capture/visitors` required `crm.view`, a permission that was never seeded, so the
> route 403'd for every user including owners. Migration count 64 → 71, table count 74 → 79.
> One thing checked and explicitly *not* changed: the original HRM Extended plan called for a
> `routing_test.go` check on `.Get("/")` under `StrictRouting: true`, reasoning it could 404
> requests without a trailing slash. Reading Fiber v3's router source suggested that risk was
> real; a throwaway reproduction against the pinned `v3.2.0` showed both `/x` and `/x/` match
> regardless of `StrictRouting` — not an actual bug in this version, so no test was added for it.)

> Last updated: 2026-07-29 (r15 — HRM extension surface scoped and recorded, plus two shared-
> infrastructure entries that came out of that scoping. New in Section 9: **PLATFORM PRIMITIVES**
> (five buildable shared pieces — notification, scheduler, resource-level permissions, checklist
> engine, form/question engine), **PREP MIGRATIONS** (three cheap-now/expensive-later schema
> hooks), and **HRM EXTENDED MODULES** (the full enterprise-HRM surface: recruitment, onboarding,
> performance, learning, compensation depth, benefits, assets, travel & expense, helpdesk, exit
> management, org chart, succession, analytics, multi-country). All three are scoping only — no
> build decision, no priority ordering, no effort estimates. Section 4 gained an **effective-dated
> (temporal) records** key pattern; Section 10 gained a **decimal money** rule. Section 5 → HRM
> gained a one-line pointer to the extension entry. Section 9 → CRM ADVANCED's
> notification/scheduler bullet was reduced to a pointer — those two now belong to PLATFORM
> PRIMITIVES (one fact, one owner).
> ⚠️ Merge note: the copy this revision was applied to predated the 2026-07-22 mobile r14 entry.
> That content has been restored — Section 5 → AUTH → MOBILE, Section 9 → MOBILE APP promoted to
> 🔵 ACTIVE, Section 13's Expo/RN/Expo Router/expo-secure-store rows, and Sections 14–15 (Mobile
> Architecture, Mobile Module Registry). If mobile was deliberately rolled back, delete those four
> places; nothing else in r15 depends on them.
> ⚠️ Open, unresolved: memory of recent sessions refers to this project as **Havelio**
> (domain `havelio.app`) rather than BusinessSAAS, and describes a mobile Expo scaffold with
> working auth/token handling — both go further than anything in this document. Neither was
> applied here, because a rename touches every section plus `go.mod`'s module path, and because
> this doc's own rule is that pasted docs and conversation memory are not authoritative against
> source. Verify against the repo, then either apply the rename in one deliberate pass or delete
> this note.

> Last updated: 2026-07-22 (r14 — Mobile App promoted ⚪ NOT STARTED → 🔵 ACTIVE. Section 9's
> entry slimmed to a pointer, same pattern as CAPTURE. Section 5 → AUTH gained a real MOBILE
> subsection with the actual route list, and resolved the r10 draft's one open question: checked
> against `frontend/src/app/(auth)/signup/page.tsx`, `Signup` does NOT auto-authenticate — the web
> client calls `login()` separately right after. `MobileSignup` mirrors that instead of inventing
> an auto-login path mobile alone would exercise. Mobile Architecture and Mobile Module Registry
> restored from the r10 archive as new Section 14 and Section 15 — appended at the end rather than
> reinstated at their old Section 9/10 slot, to avoid renumbering every cross-reference in Sections
> 1–13. Section 13's version table gained Expo SDK 57 (57.0.7)/React Native 0.86/Expo Router/
> expo-secure-store rows, plus a scaffolding gotcha: `create-expo-app` without
> `--template default@sdk-57` currently still lands on SDK 54 during the transition window.
> Zero mobile code written — this revision is the doc-level start only, implementation is next.)

> Last updated: 2026-07-21 (r14 — introduced the Collection View Pattern (Section 7): a
> Notion-style grouped/collapsible list with borderless rows, full inline editing, and inline
> quick-add, replacing the bordered-table pattern for single-entity lists. Tasks is the first
> page rebuilt on it (Section 8). Design tokens unchanged — this is a layout/interaction
> decision, not a token change. Candidate for Leads/Members when next revisited; not a mandate
> to rewrite them now.)
> Last updated: 2026-07-20 (r13 — Section 13's Tailwind v4 reminder expanded from a vague "don't use v3 patterns" into six concrete, verified syntax rules: `@import "tailwindcss"` entry point, `@theme` CSS config over `tailwind.config.js`, `bg-linear-*` gradient naming, parens vs brackets for arbitrary CSS-variable references, the gray-200 default border color, and `@utility` for custom utilities.)
> Last updated: 2026-07-20 (r12 — reconciled against the real r11 after a conversation had drifted onto a stale, incorrect picture of HRM's status; see note at bottom of this entry. Structural changes: removed "Current Focus" from Section 2 — its content either lived in Section 5 CAPTURE already or was pure priority-ranking, so it's gone rather than moved. Folded the Phase-2 backend-modification carve-out into a new general principle in Section 1: the system is interconnected, touching a connected module to finish what you're building is normal, not an exception requiring special permission. Section 9 renamed from "Build Queue" to "Unbuilt Module Registry," `⚪ QUEUED` → `⚪ NOT STARTED` everywhere, "order is decided when the current focus ships" and the CRM→HRM priority ranking removed — nothing in it carries priority ordering. Real technical dependency notes were kept as-is throughout (Capture Fix Pass B blocking deployment, ERP needing HRM/Accounting/Projects scoped first, sales velocity needing the stage-history table) — those are facts about how the system works, not schedule decisions, and stay regardless of the no-priority-ordering rule. — Also: a prior chat session spent several turns planning HRM frontend architecture, an RBAC override UI, and a full Mobile Architecture rebuild, all based on a wrong claim that HRM's frontend hadn't been built and Contacts had an unresolved integration gap. Neither was true against this document. None of that work was carried in here.)
> Last updated: 2026-07-15 (r11 — CRM Auto Lead-Capture backend built end-to-end and audited: new `internal/capture/` tree (apikeys, public, email, social, visitors), `RequireAPIKey` middleware, migrations 00057–00064. **Audit verdict: architecture sound, not yet functional** — 3 of 5 capture sources cannot create leads (`created_by` empty-UUID bug), social connect hits a column-name mismatch, visitors dashboard 403s on a nonexistent permission, and both webhook endpoints ship with zero signature verification. Full list in Section 5 → CAPTURE → Known open items; Fix Pass A/B is the current work item. Capture frontend not started. r11 also folds in four items shipped after r10 that never made it into the doc: CRM Templates (00054–55), CRM Settings with lead round-robin routing (00056, wired into lead creation via `GetLastAssignedLeadOwner`), HRM dynamic employee statuses (00053, `status_id` FK on `hrm_employees`), and the CRM Agenda page. Migration count 52 → 64, table count → 74. Structural change per Mridha's decision: **the "deferred" concept is removed from this document** — no more "deliberately last", "paused", or "confirmed still deferred"; everything not shipped is simply ⚪ QUEUED in one flat build queue. Mobile Architecture and Mobile Module Registry sections (old 9 & 10) deleted — zero code exists; the decided architecture is preserved in git history (r10) and gets restored when mobile work actually starts. Sections renumbered accordingly.)
> This document is both a Claude system instruction and a personal project reference.
> Update the STATUS blocks and MODULE REGISTRY whenever the project state changes.

---

## 1. PROJECT OVERVIEW

**BusinessSAAS** is a modular, multi-tenant SaaS platform built from scratch. It starts with a complete auth + RBAC foundation and a CRM module, and will expand into HRM, project management, e-commerce admin, and more business modules over time.

**Core principles:**

- Quality over scope. Build fewer things, but build them properly.
- Security is non-negotiable. Never cut security corners for speed.
- Design matters. Enterprise Minimalist — clean, trustworthy, information-dense done well. Not flashy, not decorative. Every screen earns its place through clarity and scannability, not visual flourish.
- Multi-tenancy is the foundation. Every feature lives inside an organization context.
- Backend stays stable. Frontend drives feature prioritization, not the other way around.
- The system is interconnected — CRM, HRM, RBAC, and the platform layer share models, permissions, and the engagement layer. Don't let scope discipline block necessary work: if finishing what you're building requires touching a connected module, touch it. Correct and complete beats narrowly scoped.

---

## 2. CURRENT STATUS

### Phase 1 — Backend Foundation: ✅ COMPLETE

Everything in this phase is done and should not be rebuilt or refactored unless there is a specific bug or a frontend integration demand.

Done:

- Docker Compose setup (backend, frontend placeholder, PostgreSQL, Redis)
- Go backend with clean layered architecture (handler → service → repository)
- Auth: signup, login, logout, logout-all, refresh token, password reset, OAuth sync
- JWT access token (short TTL) + opaque refresh token stored in httpOnly cookie (cookie name: `bsaas_refresh`, path: `/api/v1/auth`)
- RBAC: roles (owner, admin, manager, member, viewer), permissions, membership, custom/denied permission overrides
- Organization (workspace) model with multi-tenancy and context switching
- Security: session management, login event logging
- Task module (CRUD, permission-gated — used for RBAC testing)
- CRM module: leads, contacts, companies, pipeline, stages, deals, notes, activities, email logs, timeline, reports
- Platform engagement layer (notes, tasks, activities, emails)
- Audit logging (append-only)
- Rate limiting (Redis-backed, on auth endpoints)
- Tests: unit (auth, authz, user, orgs, CRM, HRM, pkg) + integration (auth flows, tenant isolation)
- CI workflow (GitHub Actions)

### Phase 2 — Frontend + CRM + HRM buildout: 🔵 ACTIVE

Building the full admin dashboard frontend plus the CRM Advanced Functionality Pass. This is not a test interface — it is the real product UI, Enterprise Minimalist quality.

Shipped and verified (see Section 7 for the full frontend registry):

- Auth pages, onboarding, dashboard shell, org switching
- RBAC management (roles, permissions, members, invitations, per-member overrides, admin password reset)
- Task module UI
- CRM UI: leads, contacts, companies, pipeline board, deals, reports, agenda, setup (routing, templates)
- HRM UI: all 8 phases complete (verified r9), plus dynamic employee statuses setup page (post-r10)
- Profile and settings pages, security pages

Scope beyond what's shipped lives in Section 5 (backend), Section 8 (frontend), and Section 9 — status per item lives in those registries, not here, so there's one place to check, not two that can quietly disagree. The Capture module (Section 5 → CAPTURE) has the most detailed known-defect write-up in the whole doc right now — read it there rather than a restated summary here.

### Unbuilt Modules

Full list: **Section 9 — Unbuilt Module Registry**. Anything in it can be picked up any time.

---

## 3. TECH STACK

### Backend

| Concern        | Choice                                                                                                       |
| -------------- | ------------------------------------------------------------------------------------------------------------ |
| Language       | Go 1.25+                                                                                                     |
| HTTP Framework | Fiber v3 (`github.com/gofiber/fiber/v3`)                                                                     |
| Database       | PostgreSQL 16+                                                                                               |
| DB Driver      | pgx v5 (`github.com/jackc/pgx/v5`)                                                                           |
| Cache / Rate   | Redis 7+ (`github.com/redis/go-redis/v9`)                                                                    |
| Migrations     | Goose (SQL migration files in `internal/migrations/`)                                                        |
| JWT            | `github.com/golang-jwt/jwt/v5`                                                                               |
| Password hash  | bcrypt via `golang.org/x/crypto`                                                                             |
| API key hash   | SHA-256 (`crypto/sha256`) — high-entropy keys, indexed exact-match lookup; bcrypt deliberately NOT used here |
| UUID           | `github.com/google/uuid`                                                                                     |
| Logger         | `log/slog` + `github.com/lmittmann/tint`                                                                     |
| Config         | `github.com/joho/godotenv`                                                                                   |
| Module path    | `github.com/mridha/businesssaas`                                                                             |

### Frontend

| Concern       | Choice                                                                                                 |
| ------------- | ------------------------------------------------------------------------------------------------------ |
| Framework     | Next.js 16.2.9 (latest stable, App Router)                                                             |
| Language      | TypeScript (strict mode)                                                                               |
| CSS + styling | Tailwind CSS v4                                                                                        |
| HTTP client   | Axios (single API client with interceptors)                                                            |
| State         | Zustand (four stores — see Section 7: `authStore`, `permissionStore`, `uiStore`, `commandStore`)       |
| Theme         | `next-themes` (light/dark, default **dark** — `app/layout.tsx` sets `defaultTheme="dark"`)             |
| Forms         | React Hook Form + Zod validation                                                                       |
| Animation     | GSAP — used across most major pages (auth, onboarding, CRM, HRM, settings, drawers), not just the sparse skeleton/count-up usage this row previously described |
| Icons         | Lucide React                                                                                           |

### Design System

**Enterprise Minimalist.** Clean, scannable, information-dense done well — the opposite of decorative. If a choice doesn't help someone read data faster, it doesn't earn a place.

| Concern         | Spec                                                                                                                                                                                           |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Theme mode      | Light + Dark both. Default: **light**. Token variables structured so dark is a straight swap, not an afterthought.                                                                             |
| Light bg        | `#ffffff` surface · `#f8fafc` canvas (page background, one step off-white)                                                                                                                     |
| Dark bg         | `#0f172a` surface · `#020617` canvas (slate-900/950 — cool dark, not pure black)                                                                                                               |
| Primary/Action  | Indigo/slate-blue — `#4f46e5` primary · `#4338ca` hover/active · same hue in both modes                                                                                                        |
| Semantic states | Success `#10b981` (emerald) · Warning `#f59e0b` (amber) · Destructive `#ef4444` (crimson) — used sparingly, only for real signal                                                               |
| Borders         | Thin, low-contrast — `#e2e8f0` light · `#1e293b` dark. Dividers, not boxes.                                                                                                                    |
| Typography      | One family only — Inter or Geist Sans, no separate display font. Strict scale: large tabular numbers for metrics, bold labels for card headers, muted small text for metadata.                 |
| Quality target  | Enterprise Minimalist — every screen reads clearly at a glance, nothing competes with the data                                                                                                 |
| Layout patterns | Two canonical page anatomies — Dashboard Page Pattern (overview/report pages) and Collection View Pattern (entity lists, Notion-style, borderless + inline-editable) — see Section 7 for both. |
| Border radius   | Subtle (4–8px), never rounded-full on blocks                                                                                                                                                   |
| Motion          | Restrained and functional — skeleton loading, hover/focus states, smooth number transitions. No entrance choreography, nothing decorative.                                                     |
| Density         | Medium-high — dashboards reward information density over whitespace, but never cramped                                                                                                         |
| Implementation  | CSS variables per theme, consumed via Tailwind `dark:` classes                                                                                                                                 |

### Infrastructure

| Concern    | Choice                                               |
| ---------- | ---------------------------------------------------- |
| Container  | Docker + Docker Compose                              |
| CI         | GitHub Actions                                       |
| Deployment | VPS via Docker Compose (not started — see Section 9) |
| Secrets    | GitHub Secrets + `.env` files (never committed)      |

---

## 4. BACKEND ARCHITECTURE

### Folder Structure

```
backend/
  cmd/server/main.go          ← app entry point, DI wiring
  internal/
    auth/                     ← auth domain (signup, login, tokens)
    user/                     ← user profile
    organizations/            ← org/workspace CRUD and context switching
    authz/                    ← RBAC (roles, permissions, memberships, invitations)
    security/                 ← session and login event management
    task/                     ← task CRUD (permission-gated test module)
    dashboard/                ← org-home metrics widget (undocumented until 2026-08-03 audit — see
                                 Section 5 → DASHBOARD; route is /api/v1/orgs/:orgId/dashboard,
                                 breaking the app-wide /organizations/:orgId/ convention)
    capture/                  ← lead auto-capture (r11)
      apikeys/                ← org API keys (generate, validate, revoke)
      public/                 ← /pub/leads public web-form capture endpoint
      email/                  ← inbound email webhook → lead + per-org addresses
      social/                 ← social lead-ad webhooks + integrations
      visitors/               ← website visitor identify + pageview log
    platform/
      contacts/               ← shared contacts + companies (used by CRM and future modules)
      engagement/             ← shared notes, tasks, activities, emails, timeline
      scheduler/              ← named-job registry, Redis-locked, run history (built 2026-08-03)
      notifications/          ← unified in-app/email dispatch (built 2026-08-03)
    crm/
      leads/                  ← CRM lead management (now with capture fields + dedup + round-robin)
      pipeline/               ← pipeline and stages
      deals/                  ← deal CRUD + board view
      reports/                ← CRM analytics endpoints (incl. agenda)
      templates/              ← email/note snippet templates (post-r10)
      settings/               ← per-org CRM settings: lead routing round-robin (post-r10)
    hrm/                      ← 26 sub-packages (departments, positions, employees, salary,
                                 attendance, payslips, leave, approvals, ...) — see Section 5 →
                                 HRM MODULE for the full breakdown; omitted here to keep this tree
                                 scannable, not because it's small
    middleware/               ← auth, business context, logger, rate limit, permission, recover, apikey
    database/                 ← postgres pool + redis client
    config/                   ← env loading and validation
    audit/                    ← append-only audit log
    migrations/               ← Goose SQL migration files
    tests/
      unit/                   ← service-level unit tests (incl. architecture/ guard tests — see
                                 Section 5 note under HRM Extended Phase 0)
      integration/            ← API + DB integration tests
  pkg/
    jwt/                      ← JWT manager
    token/                    ← opaque token generation
    password/                 ← bcrypt helpers
    response/                 ← standard JSON response helpers
    logger/                   ← slog helpers
    pagination/               ← pagination helpers
```

### Layer Rules

- **Handler**: HTTP only. Reads request, calls service, writes response. No SQL, no business logic.
- **Service**: Business logic only. No HTTP types, no SQL queries. Takes context.
- **Repository**: SQL only. No business logic. Uses parameterized queries always.
- **Middleware**: Request-level cross-cutting concerns (auth check, rate limit, permission check, API key check).
- **Pkg**: Stateless utilities with zero domain knowledge (jwt, password, token, response).

Never put business logic in handlers. Never put SQL in services. Never put HTTP types in services.

### Middleware Chains

Org-scoped JWT routes (unchanged):

```
RequireAuth → RequireOrganizationParam(:orgId) → RequirePermission(perm)
```

Public capture routes (r11):

```
RequireAPIKey(apiKeySvc, scope) → handler
```

`RequireAPIKey` reads `X-API-Key`, SHA-256 hashes it, looks up `org_api_keys`, checks `is_active` + scope + (post-Fix-Pass-A) expiry + optional per-key origin whitelist, then sets `org_id` and `user_id` (= key creator) in `c.Locals`. It is the API-key parallel of `RequireAuth`.

Webhook routes (`/pub/email/webhook`, `/pub/social/:platform/webhook`) currently run with **no auth middleware** — provider signature verification is Fix Pass B. Until that lands these endpoints must not be exposed to the public internet.

### Key Patterns

**PermissionFunc pattern** — avoids import cycles between route files and middleware:

```go
permFn := func(perm string) fiber.Handler {
    return middleware.RequirePermission(authzSvc, perm)
}
```

**Opaque refresh tokens** — raw token is returned once to handler (for httpOnly cookie), only the hash is stored in DB. Token is never logged or included in JSON body.

**Raw API keys** — same show-once discipline: `GenerateKey` returns the raw `bs_live_…` value exactly once in `CreateKeyResponse`; only the SHA-256 hash and a 16-char display prefix are persisted. `KeyHash` carries `json:"-"`.

**Webhook processing pattern (r11)** — inbound webhooks log every payload to a `*_logs` table (`raw_payload` JSONB, `processed` flag, `error_message`), and return 200 even on business failures so providers don't retry-storm. Failures are diagnosed from the log table, not from webhook response codes.

**Effective-dated (temporal) records (r15)** — for anything where "what was true on date X" is a real question, store `effective_from` / `effective_to` rows rather than overwriting a current value. Applies to: salary records, statutory slabs, compensation bands, reporting relationships, asset assignments, benefit enrollments, and rate tables. Overwriting makes historical recompute, retro/arrears, and any point-in-time analytics impossible after the fact. Corollary: transfers and reassignments are two rows (close one, open one), never an edit.

**JWT claims** include: `user_id`, `business_id` (org context), `email`, `role`. Business context is set when user selects/switches org.

**Tenant isolation** — `RequireOrganizationParam` compares `:orgId` in URL against `business_id` in JWT. Cross-org access is blocked at middleware, not just at repository level. Capture endpoints resolve org from the API key / inbound address / page_id instead, and every capture-side query is still `org_id`-scoped.

### Error Handling

All errors use a consistent model:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "Invalid email or password"
  },
  "request_id": "..."
}
```

Internal errors (SQL errors, stack traces, token internals) are never exposed to clients. They are logged server-side with `slog`.

### API Response Format

```json
{
  "success": true,
  "data": {},
  "message": "OK",
  "request_id": "..."
}
```

Use `pkg/response` helpers for all responses. Never write raw JSON responses in handlers.

---

## 5. BACKEND MODULE REGISTRY

Each module entry format: `MODULE [Status] — route prefix — key permissions`

---

### SYSTEM [✅ DONE]

Routes: `GET /api/v1/health` · `GET /api/v1/hello` · `GET /api/v1/routes` (dev only)
Notes: Health checks PostgreSQL + Redis. `/routes` lists all registered routes in dev.

---

### AUTH [✅ DONE]

Routes:

```
POST /api/v1/auth/signup           (alias: /sign-up)
POST /api/v1/auth/login            (alias: /sign-in)
POST /api/v1/auth/logout           (alias: /sign-out)
POST /api/v1/auth/logout-all       (alias: /sign-out-all)
POST /api/v1/auth/refresh          (alias: /refresh-token)
POST /api/v1/auth/password-reset/request
POST /api/v1/auth/password-reset/confirm
POST /api/v1/auth/oauth/sync
GET  /api/v1/auth/me
```

Every public endpoint above is rate-limited (`internal/auth/routes.go`'s
`RegisterRoutesWithRateLimit`, the variant actually wired in `main.go`); `logout`/`logout-all` are
not, matching `/mobile/logout` below — a holder of an expired access token must still be able to
revoke it.

Token contract:

- Access token → `Authorization: Bearer <token>` header (short TTL, default 15m)
- Refresh token → httpOnly cookie `bsaas_refresh`, path `/api/v1/auth` (long TTL, default 7d)
- Frontend never touches the refresh token directly
- `/refresh` sends cookie, receives new access token in body

**MOBILE [✅ DONE — confirmed built 2026-08-03, doc previously said "not yet written"]:**

Live in `backend/internal/auth/routes.go`, wired into `RegisterRoutesWithRateLimit`:

```
POST /api/v1/auth/mobile/signup   (rate-limited)
POST /api/v1/auth/mobile/login    (rate-limited)
POST /api/v1/auth/mobile/logout   (not rate-limited, matching web /logout)
POST /api/v1/auth/mobile/refresh  (rate-limited)
```

Same `Service`/`Repository` as web auth, zero new business logic — handler methods
`MobileSignup`/`MobileLogin`/`MobileLogout`/`MobileRefresh` (`internal/auth/handler.go`) return the
refresh token in the JSON body instead of setting the `bsaas_refresh` httpOnly cookie, and read it
back from the request body on `mobile/refresh` / `mobile/logout` instead of the cookie jar.
`logout-all`, `password-reset/*`, and `me` stay shared as-is — none of them touch tokens, no mobile
variant needed. This was never actually a gap — see Section 9 → MOBILE APP for the fuller
correction: the `mobile/` client itself is also far more built than this doc previously claimed.

Resolved (was an open assumption in the r10 draft): `Signup` does not auto-authenticate.
`frontend/src/app/(auth)/signup/page.tsx` creates the user then calls `login()` separately to
bootstrap the session. `MobileSignup` should do the same — create the user, return it with no
tokens, let the client call `mobile/login` right after. Keeps both clients on identical semantics
instead of giving mobile a second, divergent signup path.

---

### USER [✅ DONE]

Routes:

```
GET    /api/v1/me
PATCH  /api/v1/me
PATCH  /api/v1/me/settings
PATCH  /api/v1/me/preferences
POST   /api/v1/me/avatar
GET    /api/v1/me/avatars
POST   /api/v1/me/avatars/:avatarId/activate
DELETE /api/v1/me/avatars/:avatarId
```

`/users/me` (GET/PATCH) is a backward-compatible alias, same handlers as `/me`.

Key type: `SafeUser` (never expose `User` directly — it contains `password_hash`)

---

### ORGANIZATIONS [✅ DONE]

Routes:

```
POST /api/v1/organizations
GET  /api/v1/organizations
GET  /api/v1/organizations/:id
POST /api/v1/organizations/:id/switch   ← issues new JWT with org context
```

Notes: `/businesses` is a backward-compatible alias for all routes. Prefer `/organizations`.
`switch` returns a new access token with `business_id` set. Frontend must store new token.

---

### AUTHZ / RBAC [✅ DONE]

Routes:

```
GET   /api/v1/roles
GET   /api/v1/permissions
GET   /api/v1/members/me
GET   /api/v1/members
POST  /api/v1/members/:userId/role

GET   /api/v1/organizations/:orgId/members
POST  /api/v1/organizations/:orgId/members/invite
GET   /api/v1/organizations/:orgId/members/:memberId
PATCH /api/v1/organizations/:orgId/members/:memberId
PATCH /api/v1/organizations/:orgId/members/:memberId/role
PATCH /api/v1/organizations/:orgId/members/:memberId/status
POST  /api/v1/organizations/:orgId/members/:memberId/reset-password

POST   /api/v1/organizations/:orgId/invitations/:invitationId/resend
DELETE /api/v1/organizations/:orgId/invitations/:invitationId
POST   /api/v1/organizations/:orgId/invitations/:token/accept

GET    /api/v1/organizations/:orgId/rbac/permissions
GET    /api/v1/organizations/:orgId/rbac/permissions/grouped
GET    /api/v1/organizations/:orgId/rbac/permissions/matrix
POST   /api/v1/organizations/:orgId/rbac/check
POST   /api/v1/organizations/:orgId/rbac/check-member   ← alias of /check, same handler

GET    /api/v1/organizations/:orgId/rbac/roles
POST   /api/v1/organizations/:orgId/rbac/roles
GET    /api/v1/organizations/:orgId/rbac/roles/:roleId
PATCH  /api/v1/organizations/:orgId/rbac/roles/:roleId
DELETE /api/v1/organizations/:orgId/rbac/roles/:roleId
PATCH  /api/v1/organizations/:orgId/rbac/roles/:roleId/permissions
POST   /api/v1/organizations/:orgId/rbac/roles/:roleId/clone

GET   /api/v1/organizations/:orgId/rbac/members/:memberId/permissions
PATCH /api/v1/organizations/:orgId/rbac/members/:memberId/permissions
```

Roles: `owner` · `admin` · `manager` · `member` · `viewer`
Key permissions: `members.view` · `members.update` · `members.invite` · `members.remove` · `members.password_reset` · `roles.view` · `roles.create` · `roles.update` · `roles.delete` · `roles.clone` · `roles.permissions.update` · `members.permissions.view` · `members.permissions.update`

Guards: `UpdateMemberPermissions` rejects self-targeting (`ErrCannotChangeOwnPermissions`) and owner-targeting (`ErrCannotModifyOwner`). `members.password_reset` (owner/admin) revokes the target's sessions and is audit-logged. `organizations.max_seats` (nullable, migration 00052) enforced in `InviteMember` → 409 `ErrSeatLimitReached`; no admin UI, direct DB write only.

---

### SECURITY [✅ DONE]

Routes:

```
GET    /api/v1/organizations/:orgId/security/sessions
DELETE /api/v1/organizations/:orgId/security/sessions/:sessionId
GET    /api/v1/organizations/:orgId/security/login-events
```

Permissions: `security.sessions.view` · `security.sessions.revoke` · `security.login_events.view`

---

### TASK [✅ DONE]

Routes:

```
GET    /api/v1/organizations/:orgId/tasks
POST   /api/v1/organizations/:orgId/tasks
GET    /api/v1/organizations/:orgId/tasks/:taskId
PATCH  /api/v1/organizations/:orgId/tasks/:taskId
DELETE /api/v1/organizations/:orgId/tasks/:taskId
```

Permissions: `tasks.view` · `tasks.create` · `tasks.update` · `tasks.delete` · `tasks.view_all`
Statuses: `todo` · `in_progress` · `done` · `cancelled`

`tasks.view_all` (migration 00065) isn't route-gated via `permFn` — `handler.List` checks it
inline (`authzSvc.Can(ctx, userID, orgID, "tasks", "view_all")`) to decide between "see everyone's
tasks" and "see only your own." This is the one working precedent for the `view_own`/`view_team`/
`view_all` scoping pattern Phase 1 (resource-level permissions) is about to formalize across HRM —
worth reading before designing that ADR.

---

### DASHBOARD [✅ DONE — found undocumented in the 2026-08-03 audit]

Org-home metrics widget. One route, no permission gate beyond org membership:

```
GET /api/v1/orgs/:orgId/dashboard   ← requireAuth + requireOrgMatch only, no permFn
```

⚠️ Route prefix is `/orgs/:orgId/` — every other module in this registry uses
`/organizations/:orgId/`. Not fixed as part of this audit pass (would be a breaking API change for
whatever frontend already calls it — `frontend/src/lib/dashboard.ts` — needs to move in the same
commit as the backend route if this is ever corrected). Flagging so it doesn't get silently copied
as the pattern for the next new module.

---

### PLATFORM — CONTACTS [✅ DONE]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/contacts
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/contacts/:contactId

GET/POST         /api/v1/organizations/:orgId/crm/companies
GET              /api/v1/organizations/:orgId/crm/companies/enrich       ← crm.companies.view (registered before :companyId)
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/companies/:companyId
GET              /api/v1/organizations/:orgId/crm/companies/:companyId/contacts
```

Permissions: `crm.contacts.*` · `crm.companies.*`

---

### PLATFORM — ENGAGEMENT [✅ DONE]

Routes (all under `/organizations/:orgId/crm/`):

```
Notes:      GET/POST /notes  · GET/PATCH/DELETE /notes/:noteId
Tasks:      GET/POST /crm/tasks · GET/PATCH/DELETE/POST(complete/reopen/assign) /crm/tasks/:taskId
Activities: GET/POST /activities · GET/PATCH/DELETE /activities/:activityId
Emails:     GET/POST /emails · GET/DELETE /emails/:emailId
Timeline:   GET /timeline?related_type=&related_id=
```

Permissions: `crm.notes.*` · `crm.tasks.*` · `crm.activities.*` · `crm.emails.*`
Notes are tagged with module `"crm"` — every writer must use that exact tag or the record is invisible to the timeline (the capture dedup path currently violates this; Fix Pass A).

---

### PLATFORM — SCHEDULER [✅ DONE — built 2026-08-03]

Generic named-job registry in `internal/platform/scheduler/`: in-process ticker (30s), Redis
`SETNX` distributed lock per job (multi-instance safe), run history, manual trigger. Migration
`00067`. Supersedes the PLATFORM PRIMITIVES §2 scoping entry in Section 9 — built, not just scoped.

Routes:

```
GET  /api/v1/platform/scheduler/jobs              ← platform.scheduler.view
GET  /api/v1/platform/scheduler/jobs/:name/runs   ← platform.scheduler.view
POST /api/v1/platform/scheduler/jobs/:name/run    ← platform.scheduler.manage
```

⚠ **These three routes take their org context from the JWT, not the path** — there is no `:orgId`
segment, because scheduled jobs are instance-wide rather than org-scoped. `middleware.RequirePermission`
reads `business_id` from the token, and **`Login` issues a token with no org**, so a plain login
token gets `400 NO_BUSINESS_CONTEXT` here. Call `POST /organizations/:id/switch` first and use the
token it returns.

This was recorded as a bug ("the route has no `:orgId` but its permission gate requires one") from
r24 through r36 and repeated in every "still open" list. It is not a bug — verified in r37 against a
switched token, which returns all 8 registered jobs with their run history.

Permissions: `platform.scheduler.view` / `.manage` — granted to owner/admin.

Two jobs registered in `main.go`. Both were already wired to the scheduler before this revision,
but neither actually worked — one was a stub, the other silently errored on every run:

- `milestones.generate_upcoming` (daily 01:00) — was querying `hrm_employees.status`, a column
  migration `00053` dropped in favor of `status_id`; the query errored every time, the error was
  swallowed, and the cron always no-op'd. Fixed to join `hrm_employee_statuses` on
  `category='active'`.
- `attendance.absence_sweep` (daily 02:00) — was a literal stub (`slog.Info(...stub executed...)`
  and nothing else). Now marks employees absent for the prior day when no attendance record
  exists, it's a working day per their resolved shift, it isn't a holiday (employee → department
  → org calendar cascade), and they have no approved leave. Fixing this required also fixing a
  pre-existing bug in `attendance.resolveShift`: it queried columns `wsa.scope`/`wsa.entity_id`,
  but `hrm_work_schedule_assignments` (migration `00027`) actually names them
  `assignee_type`/`assignee_id` — shift resolution always silently failed and every attendance
  record fell back to a flat 8h default instead of the employee's real shift.

Both jobs need a valid `created_by`/actor UUID for system-generated rows; migration `00069` seeds
a sentinel system user (`scheduler.SystemUserID`, never a real login) for exactly this.

---

### PLATFORM — NOTIFICATIONS [🔵 ACTIVE — infra + API built 2026-08-03, digest/push not built]

Unified dispatch in `internal/platform/notifications/`: per-user, per-event-type, per-channel
preference matrix; in-app (DB row) and email (Resend — real, not a stub) channels. Migration
`00068`. Supersedes the PLATFORM PRIMITIVES §1 scoping entry in Section 9.

Routes (self-scoped to the requesting user, like `/me` — `requireAuth` only, no RBAC permission
check, since a user always owns their own notifications):

```
GET   /api/v1/notifications                ← list (paginated) + unread_count
POST  /api/v1/notifications/:id/read
POST  /api/v1/notifications/read-all
GET   /api/v1/notifications/preferences
PATCH /api/v1/notifications/preferences
```

Frontend: Topbar bell wired to a drawer (`NotificationDrawer`) with unread badge, mark-read,
mark-all-read (previously a decorative placeholder icon with no handler). Only consumer today is
`auth` (password reset / invite emails) — no HRM module calls `Dispatch()` yet, correctly, since
none is queued (Section 9's build-order note applies: build the primitive when the first real
consumer arrives). Digest batching and push channel are not built.

---

### PLATFORM — CHECKLISTS [✅ DONE — built 2026-08-05]

Generic checklist engine in `internal/platform/checklists/`: templates → typed items (`owner_type`
∈ `subject`/`manager`/`role`/`specific_user`) → instances → per-item runtime state, with
`checklist_type` as the discriminator (`onboarding`/`offboarding`/`probation_confirmation`/
`transfer_handover` — all four seeded now, only `onboarding` has a consumer). Migration `00076`.
Supersedes the PLATFORM PRIMITIVES §4 scoping entry in Section 9 — built, not just scoped.

Never imports `internal/hrm/employees` or any other `hrm_*` table — owner resolution takes a
caller-supplied `SubjectContext` value (subject/manager user IDs, anchor date) instead of querying
HRM directly. This is also a security boundary, not just an architectural one: there is
deliberately **no generic `POST .../instances` route**. A generic route would have to trust a
client-supplied `subject_user_id`/`manager_user_id`, letting a caller point every checklist item at
themselves or manufacture items assigned to a victim. Instantiation is service-level only, reached
through module-owned endpoints that resolve the subject server-side — today, only
`internal/hrm/onboarding/` (Section 5 → HRM MODULE → Group B).

Role-owned items (`owner_type='role'`) never resolve to a specific `assignee_user_id` at
instantiation — no role→users query runs. Instead the instance item carries `owner_type`+`owner_role`
forward as a "group claim": any org member currently holding that role may complete it, checked at
completion time via two new `authz` methods (`RoleExists`, `UserRoleName`) that reproduce the
OR-join from `GetUserPermissions` (`internal/authz/repository.go`) — a membership's `role_id` can be
NULL while `role_key` still names a live global role, and missing that OR silently drops those
members. `RoleHolders` (the reverse role→users direction) was deliberately not built — it has zero
callers; `/items/mine` and completion auth both only need the cheap user→role direction.

Routes (all under `/organizations/:orgId/checklists/`):

```
Templates       GET/POST         /templates                              ← platform.checklists.view / .manage
                GET/PATCH/DELETE /templates/:templateId                  ← .view / .manage / .manage
Template items  GET/POST         /templates/:templateId/items            ← .view / .manage
                PATCH/DELETE     /templates/:templateId/items/:itemId    ← .manage
Instances       GET              /instances                              ← .view
                GET              /instances/:instanceId                  ← .view
                POST             /instances/:instanceId/cancel           ← .manage
Items           GET              /items/mine                             ← .view  (registered before /items/:itemId)
                POST             /items/:itemId/complete                 ← .complete
                POST             /items/:itemId/reopen                   ← .complete
                POST             /items/:itemId/skip                     ← .manage
```

Permissions (migration `00077`): `platform.checklists.view` (owner/admin/manager/member/viewer),
`.complete` (owner/admin/manager/member — narrowed per-item by the service: assignee, or matching
role holder, or `.manage`, since the route gate cannot express "is this your own item"), `.manage`
(owner/admin only). Org-created custom roles get none of these until an admin grants them
explicitly — a deliberate choice, not an oversight (these are new capabilities, unlike Phase 1's
`view_own` backfill which restored prior behaviour).

Completion is always computed from instance items (`COUNT(*) FILTER`), never stored — an instance
auto-transitions to `completed` when every item is terminal (completed or skipped) and flips back
to `in_progress` if the last terminal item is reopened. Due dates are frozen at instantiation
(`anchor_date + due_offset_days`, `AddDate` not raw duration arithmetic — DST-safe); negative
offsets are valid (pre-boarding items) and not clamped.

Known limitation, stated not hidden: `owner_role TEXT` (not `owner_role_id UUID`) means renaming a
role via `authz.UpdateRole` silently breaks any template item still referencing the old name —
`RoleExists` validates on write and template-item reads expose `owner_role_exists` so the UI can
flag a broken reference, but the underlying fragility is accepted, not fixed, in this phase.

---

### PLATFORM — FORMS [✅ DONE — built 2026-08-07]

Generic form/question engine in `internal/platform/forms/`: templates → sections → typed questions
→ instances → one typed response row per question → scoring → aggregate. Migrations `00084`
(schema) / `00085` (permissions). Supersedes the PLATFORM PRIMITIVES §5 scoping entry in Section 9
— built, not just scoped, and built **with** its first real consumer (appraisals) rather than ahead
of one, the same shape as the checklist engine shipping with onboarding.

Follows the `internal/platform/checklists/` template exactly, including both structural rules:

- **Polymorphic subject with no FK** (`subject_type` CHECK + `subject_id`), so `platform/forms`
  never references `hrm_*`. Zero `internal/*` imports except `internal/middleware`.
- **No generic instantiate route.** A generic endpoint would have to trust a client-supplied
  `subject_id` and `respondent_user_id` — an impersonation vector, and a form response is
  attributable evidence about a person. Consumers instantiate service-side from their own
  endpoints having resolved the subject from their own domain. Today: appraisals
  (`internal/hrm/performance`) and 360 feedback (`internal/hrm/feedback`).

**Definition is snapshotted onto each instance as REAL COLUMNS, never JSONB.** Migration `00076`
states the rule directly — JSONB is for opaque ordered config read as a whole, real columns are
required whenever rows are individually mutated or aggregated. Form responses are aggregated, so
they are rows: `answer_text` / `answer_number` / `answer_boolean` / `answer_date` /
`answer_options`, one populated per response selected by question type. A response row per question
is created at instantiation, so answering is an UPDATE.

`SubjectContext` keeps **`SubjectID` (who it is about) separate from `RespondentUserID` (who fills
it in)**. This is the split appraisals depend on for self-vs-manager forms and 360 depends on for
every respondent; collapsing them files every response under the wrong person. A nil respondent is
legitimate — an employee with no manager, or a respondent with no platform account — and means the
form exists and is completable by a `platform.forms.manage` holder, which is different from the
form not existing.

`computeScore` normalises each answer to 0-1 against **its own scale** before weighting, so a 1-5
scale question and a 1-10 one can share a template without the second silently dominating.

Routes (17, all under `/organizations/:orgId/forms/`): templates + nested section create, sections
+ nested question create, questions, and instances (`/instances/mine` registered **before**
`/instances/:instanceId` — a literal segment loses to a param when registered after it).
Permissions (`00085`): `platform.forms.view`, `.respond` (reaching `member`, narrowed by the
service to the instance's own respondent, since the route gate cannot express "is this YOUR form"),
`.manage` (owner/admin).

**`hrm_interview_scorecards` is deliberately NOT on this engine** — see Section 9 → PLATFORM
PRIMITIVES §5 for why the r21 "consumer #1" framing was superseded.

---

### PLATFORM — TICKETS [✅ DONE — built 2026-08-27]

Ticket/helpdesk engine in `internal/platform/tickets/`: categories → SLA policies → tickets →
comments → an append-only SLA pause ledger. Migrations `00110` (schema) / `00111` (permissions),
five tables, 17 routes. Platform Primitive #6.

**Why platform and not `internal/hrm/helpdesk`.** The build plan called this "an architectural
fork to decide before starting" — HR Helpdesk in the HRM list, customer-facing Ticketing in the
CRM list. Building it as a platform primitive means the same engine serves both, and the codebase
had already made this exact decision twice: `platform_checklist_instances.subject_type CHECK IN
('employee')` and `platform_form_instances.subject_type CHECK IN ('employee','candidate')`.
`platform_tickets.requester_type CHECK IN ('employee')` is the third instance of that pattern.
`requester_id` carries **no FK**, so adding `'contact'` later is a CHECK widening with no `hrm_*`
reference to untangle — an integration test asserts no `platform_ticket*` table has an FK into
`hrm_*`.

**No scope tiers, because `internal/hrm/scope` cannot be imported here.** `scope.Predicate`
hard-codes `FROM hrm_employees`; a platform primitive must not reach into HRM tables, and
`TestPermissions_ScopeTiersSeeded` never fires because nothing calls `ResolveScope`. Visibility is
`ListFilter.CanViewAll` instead — resolved once in the service from `platform.tickets.view_all`
and applied in SQL as `requester_user_id = caller OR assignee_user_id = caller`. The
`platform.checklists.complete` precedent: the route gate cannot express ownership, so the service
does. `FindTickets` and `CountTickets` share one `ticketWhere` builder so a list and its own total
cannot drift. The single-row read restates the same rule and reports **NOT-FOUND** rather than
denied for an invisible ticket — "you may not see ticket X" still confirms ticket X exists, which
in a helpdesk carrying harassment reports is itself a disclosure.

**`is_internal` comments are filtered at the REPOSITORY layer, in two read methods.**
`FindPublicComments` (requester) and `FindAllComments` (agent) differ by one `WHERE` clause, and
that difference is the whole of internal-comment protection — a single method plus a caller-side
filter is one forgotten branch away from leaking, and the requester's path must never hold an
internal comment in memory at all. Structural, not disciplinary: the 5C 360-anonymity and 6A
answer-key shape. `idx_ptcmt_public` is a partial index on `is_internal = FALSE`, so the common
path never scans internal rows. Writing an internal note needs `platform.tickets.comment_internal`
separately from `.comment`, so a requester cannot author something they could not read back.

**The SLA clock is pausable, and pauses live in an append-only ledger.**
`platform_ticket_sla_events` holds pause/resume rows; there is deliberately no `elapsed_minutes`,
`paused_minutes` or `sla_breached` column (an integration test introspects `information_schema` to
prove nine such columns absent, plus no `updated_at` on the ledger itself). One ticket is routinely
paused and resumed several times, and a mutable counter shows the total but never how it was
reached — the `hrm_loan_recovery_events` reasoning. `sla.go` is pure and was written with its tests
first: consecutive pauses (first wins), a resume with no pause (ignored), a trailing pause (runs to
`now`), an inverted interval contributing zero rather than negative (a negative would subtract from
paused time and mask a breach), and a zero target never counting as a breach. Each clock stops at
the event that satisfies it — `first_response_at` or `resolved_at` — not at `now`, so a ticket
answered inside its window cannot drift into breach unattended. Only a public reply from somebody
other than the requester stamps `first_response_at`: the SLA measures what the requester actually
received, so neither their own follow-up nor an internal note counts.

**The governing policy is pinned at creation** (`sla_policy_id`), so tightening a policy later
cannot retroactively breach older tickets — 7B's `calculation_snapshot` / 7D's
`employee_cost_snapshot` discipline applied to a target rather than a price. `ResolvePolicy`
prefers a category-specific row and falls back to the org-wide default (`NULL category_id`).

**Sensitive categories restrict the assignee pool.** `restricted_role` is a role NAME, not an FK
(roles are org-scoped and partly system-seeded — the `platform/checklists` `owner_type='role'`
precedent), checked through `AccessDirectory.UserRoleName` so this package holds exactly one notion
of "what role does this user have" and it is authz's. The `is_sensitive` ↔ `restricted_role`
pairing is enforced in Go rather than as a CHECK, which would be the `00076` `ON DELETE SET NULL`
trap.

**Conversion to an HR complaint is one-way and has no route.** `MarkConverted` is called from the
HRM side, which reads the ticket, creates the complaint, then calls back — `hrm → platform` is the
allowed direction. A generic HTTP endpoint would have to trust a client-supplied
`converted_to_id`, the same reasoning that gives checklists no generic instantiate route. A second
conversion is refused rather than silently accepted.

Permissions (`00111`): `platform.ticket_config.{view,manage}` and
`platform.tickets.{view,create,comment,comment_internal,assign,resolve,pause,view_all}`. Owner and
admin hold all ten; manager holds the agent keys but **not** `.view_all`; member holds
view/create/comment; viewer holds none. `.view_all` and `.comment_internal` have no route of their
own by design — both are read inside the service to widen what an existing route returns.

⚠ **`authz.Can` builds its permission key as `resource + "." + action`**, so the resource argument
must carry the full dotted prefix (`"platform.tickets"`, not `"tickets"`). Passing the bare name
denies everything silently and uniformly, which nearly produced a suite of vacuously-passing
negative tests.

---

### PLATFORM — KNOWLEDGE BASE [✅ DONE — built 2026-08-27]

Article/documentation engine in `internal/platform/kb/`: categories → articles → publish/archive,
with full-text search. Migrations `00112` (schema) / `00113` (permissions), two tables, 9 routes.
Platform Primitive #7. Built alongside PLATFORM — TICKETS, which is what a knowledge base is for —
deflecting the tickets nobody needed to raise.

**Two permissions only.** `platform.kb.view` reads published articles; `platform.kb.manage` reads
and writes everything including drafts. The draft/published split is what makes two keys enough: a
KB is org-wide reading material with no "mine" to narrow to, unlike tickets. A third
`.view_unpublished` would imply a contributor role this product does not have, and an unused key is
one nobody notices is granted wrongly. `viewer` gets `.view` here (unlike tickets, where a
read-only role has no business reading other people's helpdesk threads — published documentation is
exactly what it should see); `manager` gets `.manage`, because helpdesk agents are who write and
correct KB articles.

**Unpublished articles are excluded in SQL, and the SAFE state is the DEFAULT.**
`ArticleFilter.IncludeUnpublished` *adds* `WHERE status = 'published'` when false rather than
removing it when true, so a zero-valued filter — the shape of every "forgot to configure it" bug —
returns published articles only. The failure mode is "too little", never "too much". The service
sets the flag from the caller's own `.manage` grant and never reads it off the request, and a draft
reports NOT-FOUND rather than denied so the single-row read agrees with the list that hides it.
Same intent as tickets' two comment read paths, expressed as a default rather than a second method
because there is one query shape here, not two.

**Search is a GIN EXPRESSION index — there is no `search_vector` column.**
`to_tsvector('english', title || ' ' || body)` is indexed as an expression, so Postgres recomputes
it on every write; a stored vector is a derived value somebody has to remember to update, and this
one cannot go stale. `plainto_tsquery` treats the input as literal words, so a user typing `&` or
`!` gets a search rather than a syntax error. There is deliberately **no `view_count`** either — a
counter nothing can recompute is unauditable the moment it drifts, and deriving one needs a
view-event table no consumer has asked for. The `00076` rule cuts both ways: don't store what can
be derived, and don't invent state nothing needs.

**`published_at` records first publication, not last edit.** Editing a published article leaves it
published — correcting a live policy must not silently unpublish it and leave employees reading
nothing — and re-publishing an archived article restores it without pretending it is newly written,
which a list ordered by `COALESCE(published_at, created_at)` would otherwise show at the top.
Articles are always born drafts: the first save is the least likely to be the one worth publishing.
Archive rather than delete, because superseded guidance still explains why somebody acted as they
did.

---

### CAPTURE — INBOUND EMAIL [🔵 EXTENDED r32 — now a router]

`internal/capture/email/`. Was a single-consumer pipeline: `ProcessInboundWebhook` hardcoded
`leads.CreateLead`. Phase 8D generalized it — `org_inbound_emails.destination`
(`'lead' | 'ticket'`, migration `00112`) decides which module an inbound email becomes, and the
receiving address is what carries that decision: `sales@` makes a lead, `support@` makes a ticket.

**`'lead'` is the default at three independent layers** — the column default, the service default
when the field is omitted, and the router's fallback for any value this build does not recognise.
All three must agree or every pre-8D address silently changes behaviour, so one test pins them
together. Migration `00112` alone changes nothing for an existing install.

**The ticket branch refuses rather than improvises.** `platform_tickets.requester_id` and
`.requester_user_id` are both NOT NULL, so an email from somebody who is not an employee of that
org produces NO ticket — recorded in `inbound_email_logs.error_message` naming the sender. Attaching
a stranger's email to a fallback employee would put words in that person's mouth; in an HR helpdesk
carrying grievances that is the worst available failure. The ticket is created AS the sender, so it
lands in their own list and they can comment on it.

Sender → employee resolution lives on **this package's own repository**, not in `platform/tickets`,
which must never reference `hrm_*` — the 7D `benefits.FindEmployeeIDByUserID` precedent that
resolving your own subject is the consuming package's job. `email.TicketRaiser` is consumer-owned
and names tickets' own types, so `tickets.Service` satisfies it structurally with no adapter, and
it is nil-safe: an install with no ticket engine logs the address as unroutable rather than
panicking inside a webhook handler.

⚠ **Every failure in this pipeline is recorded and swallowed**, because the webhook provider must
receive a 200 or it retries the same message forever. `inbound_email_logs.error_message` is the
only place an operator can see what went wrong — which is exactly how the `created_by` defect
below stayed invisible for so long.

⚠ **`crm_leads.created_by` was `NOT NULL` and every system capture had been failing** (r32). See
the changelog entry — one fix in `leads.CreateLead` repaired the email, social and visitor paths
together.

---

### CRM — LEADS [✅ DONE — extended r11]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/leads
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/leads/:leadId
POST             /api/v1/organizations/:orgId/crm/leads/:leadId/convert
```

Permissions: `crm.leads.view` · `crm.leads.create` · `crm.leads.update` · `crm.leads.delete` · `crm.leads.convert`
Statuses: `new` · `contacted` · `qualified` · `unqualified` · `converted`

**Post-r10 extensions:**

- **Round-robin auto-assignment** (00056 + `crm/settings`): when `crm_settings.lead_routing_enabled` and `round_robin_assignees` is non-empty, `CreateLead` rotates `owner_id` via `GetLastAssignedLeadOwner`.
- **Capture fields** (00058): `custom_fields JSONB` · `capture_source TEXT` · `capture_metadata JSONB` on `crm_leads`, present in model/scan/insert.
- **Email dedup**: `CreateLead` checks `FindLeadByEmail` first; on match, appends an engagement note to the existing lead and returns it instead of inserting. ⚠️ Currently applies to ALL creates including manual UI creates, match is case-sensitive, and the note uses the wrong module tag — all three are Fix Pass A items (scope to `CaptureSource != nil`, `LOWER()` match, module `"crm"`).

---

### CRM — PIPELINE & STAGES [✅ DONE]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/pipelines
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/pipelines/:pipelineId
GET/POST         /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages
POST             /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/reorder
PATCH/DELETE     /api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/:stageId
```

Permissions: `crm.deals.*`

---

### CRM — DEALS [✅ DONE]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/deals
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/deals/:dealId
POST             /api/v1/organizations/:orgId/crm/deals/:dealId/move
POST             /api/v1/organizations/:orgId/crm/deals/:dealId/won
POST             /api/v1/organizations/:orgId/crm/deals/:dealId/lost
GET              /api/v1/organizations/:orgId/crm/deals/:pipelineId/board
```

The board route's param is `:pipelineId` (board = all deals in one pipeline, not one deal) —
`internal/crm/deals/routes.go:37`. Its own doc-comment (same file, line 19) still says `:dealId`;
code and comment disagree with each other, not just with this doc. Corrected here to match the
actual registered route; the source comment itself is still wrong.

Permissions: `crm.deals.*` · `crm.deals.move_stage`

---

### CRM — TEMPLATES [✅ DONE — post-r10, folded into doc r11]

Routes:

```
GET/POST         /api/v1/organizations/:orgId/crm/templates
GET/PATCH/DELETE /api/v1/organizations/:orgId/crm/templates/:templateId
```

Permissions: `crm.templates.view` · `crm.templates.create` · `crm.templates.update` · `crm.templates.delete` (seeded 00055)
Table: `crm_templates` (00054) — `type IN ('email','note')`, optional `subject`, `body`. Email/note snippets for quick insertion by reps.

---

### CRM — SETTINGS [✅ DONE — post-r10, folded into doc r11]

Routes:

```
GET   /api/v1/organizations/:orgId/crm/settings     ← settings.view
PATCH /api/v1/organizations/:orgId/crm/settings     ← settings.update
```

Table: `crm_settings` (00056) — `lead_routing_enabled BOOLEAN`, `round_robin_assignees JSONB`. Backs the lead round-robin (see CRM — LEADS). Uses the generic `settings.*` permissions, not a `crm.settings.*` pair — acceptable for now, revisit if CRM settings grow.

---

### CRM — REPORTS [✅ DONE — agenda added post-r10]

Routes:

```
GET /api/v1/organizations/:orgId/crm/reports/overview
GET /api/v1/organizations/:orgId/crm/reports/summary
GET /api/v1/organizations/:orgId/crm/reports/deals/by-stage
GET /api/v1/organizations/:orgId/crm/reports/deals/by-owner
GET /api/v1/organizations/:orgId/crm/reports/leads/by-source
GET /api/v1/organizations/:orgId/crm/reports/tasks/overdue
GET /api/v1/organizations/:orgId/crm/reports/activities/stats
GET /api/v1/organizations/:orgId/crm/reports/agenda          ← post-r10, backs /crm/agenda page
GET /api/v1/organizations/:orgId/crm/reports/rep-performance
GET /api/v1/organizations/:orgId/crm/reports/forecast
```

Permissions: `crm.reports.view`

---

### CAPTURE [🔵 ACTIVE — backend written r11, Fix Pass A/B pending, frontend not started]

New top-level module: `internal/capture/`. All five sources wired in `main.go`; migrations 00057–00064.

**Auth-gated management routes (JWT + org match):**

```
GET/POST /api/v1/organizations/:orgId/capture/apikeys           ← capture.apikeys.view / .create
DELETE   /api/v1/organizations/:orgId/capture/apikeys/:keyId    ← capture.apikeys.delete
GET/POST /api/v1/organizations/:orgId/capture/email             ← ⚠️ uses settings.view; switch to capture.email.manage (Fix A)
DELETE   /api/v1/organizations/:orgId/capture/email/:id
GET/POST /api/v1/organizations/:orgId/capture/social            ← ⚠️ uses settings.view; switch to capture.social.manage (Fix A)
DELETE   /api/v1/organizations/:orgId/capture/social/:id
GET      /api/v1/organizations/:orgId/capture/visitors          ← capture.visitors.view (was nonexistent crm.view, 403'd for everyone; fixed 2026-08-03)
```

**Public routes:**

```
POST /api/v1/pub/leads                       ← RequireAPIKey, scope capture:leads — WORKS (web form / chat)
POST /api/v1/pub/visitors/identify           ← RequireAPIKey, scope capture:visitors
POST /api/v1/pub/email/webhook               ← ⚠️ NO verification (Fix B) — do not expose publicly yet
GET/POST /api/v1/pub/social/:platform/webhook ← ⚠️ NO signature check; GET accepts any verify_token (Fix B)
```

**API key contract:** raw key `bs_live_<64 hex>`, shown exactly once at creation; SHA-256 hash + 16-char prefix stored; scopes `capture:leads` (+ `capture:visitors`, constant pending); optional per-key `allowed_origins` and `expires_at`.

**Permissions seeded (00062):** `capture.apikeys.view/create/delete` · `capture.email.manage` · `capture.social.manage` · `capture.visitors.view` — granted to owner/admin. Apikeys and visitors are referenced correctly; email/social routes still use `settings.view` instead of their seeded keys (Fix A item 3, partially open — see below).

**Deliberate scope reductions (documented, not bugs):**

- Social is a **webhook skeleton**, not a real Facebook integration — no Graph API `leadgen_id` fetch, no OAuth connect, no field-mapping engine; it maps flat payload fields, which real FB webhooks don't send. Real integration is tracked separately (Section 9).
- Visitors is **manual identify** (Segment-style: `traits.email/name/company` → lead), not IP→company enrichment. No IPinfo, no Redis queue, no background worker.
- No hosted form / JS embed endpoint yet (was Step 4 second half) — capture frontend work.

**Known open items (r11 audit — this list IS the current work):**

_Fix Pass A — feature-breaking + correctness:_

1. `CreateLead(ctx, orgID, "", …)` in email/social/visitors → `created_by` gets empty string → invalid-UUID error on every system-generated lead. Fix: migration 00065 drops NOT NULL on `crm_leads.created_by`; model `CreatedBy *string`; pass nil for system captures; UI renders null as "System".
2. `social/repository.go` INSERTs into `access_token_enc`; column is `access_token` (00060) → connect always fails. Fix repo SQL.
3. ~~Visitors route used nonexistent `crm.view`, 403'd for everyone~~ — fixed 2026-08-03, now
   `capture.visitors.view` (caught by a new guard test, `TestPermissions_UsedStringsExistInMigrations`,
   that checks every `permFn(...)` string against seeded migration keys, codebase-wide). Still
   open: email/social routes use `settings.view` instead of their seeded `capture.email.manage` /
   `capture.social.manage` keys; `capture:visitors` scope constant + scope/name validation in
   `GenerateKey` not added.
4. `ValidateKey` never checks `expires_at` (`ErrKeyExpired` sentinel unused). Add the check.
5. Dedup: scope to `req.CaptureSource != nil` only; `LOWER(email)` match; note module `"crm"`; skip note when `userID == ""`; don't swallow the note error silently.
6. ~~Delete leftover AI-conversation comments in `public/handler.go`~~ — fixed 2026-08-03 (a
   five-line internal monologue about `created_by`/UUID handling). `leads/service.go` checked
   clean — no artifacts found there. A new hygiene test (`TestHygiene_NoAIConversationArtifacts`)
   now catches this class going forward.
7. Minor: `social` model `AccessToken` json tag → `"-"`; `org_api_keys.created_by` ON DELETE CASCADE → RESTRICT; UNIQUE `(org_id, session_id)` on `website_visitors`; UNIQUE on `org_api_keys.key_hash`.

_Fix Pass B — security, required before any public exposure:_ 8. Inbound email webhook HMAC verification (`WEBHOOK_EMAIL_SECRET`). 9. Facebook `X-Hub-Signature-256` verification (`FACEBOOK_APP_SECRET`) + real `hub.verify_token` comparison (`FACEBOOK_VERIFY_TOKEN`). 10. Redis rate limiting on every `/pub/*` route (new `NewPublicCaptureRateLimit` constructor; per-key where a key exists, per-IP on webhooks).

---

### HRM MODULE [✅ DONE — verified r9; dynamic statuses added post-r10; route count corrected 2026-08-03; leave balance engine added r18; onboarding checklist consumer added r19; Recruitment/ATS Phase 4A added r20, Phase 4B added r21; Performance/Goals Phase 5A added r22; appraisals 5B + 360 feedback/PIP 5C added r23]

All routes live under `/api/v1/organizations/:orgId/hrm/...`, permission-gated (`hrm.<submodule>.<action>`), **41 route-bearing sub-modules, 484 routes** (218 as of r19 + 34 recruitment r20 + 25 recruitment r21 + 19 performance r22 + 21 appraisal r23 + 12 feedback r23 + 9 PIP r23 + 3 correcting an r22 undercount + 29 learning r24 + 9 skills r24 + 9 certifications r24 + 1 payroll preview r25 + 22 compensation r26 + 7 loans r27 + 5 reimbursements r27 + 7 statutory r28 + 11 benefits r28 + 21 assets r29 + 22 travel/expense r30). Counts re-grepped from `internal/hrm/*/routes.go` at r30 rather than carried forward — the doc's own update rule, since these drift every revision. `internal/hrm/scope` has no routes and is not counted. This entry summarizes; per-route detail belongs in a dedicated `docs/modules/hrm.md`.

**Database:** 52 tables. 40 verified in r9 (migrations `00020`–`00050`, of which `00048` is unrelated CRM seed data) + `hrm_employee_statuses` (00053) + `hrm_legal_entities` (00070, previously undercounted here — Section 6 already had it) + `hrm_leave_policies`/`hrm_leave_transactions`/`hrm_leave_balances` (00074, r18) + `hrm_recruitment_pipelines`/`_stages`/`hrm_job_requisitions`/`_postings`/`hrm_candidates`/`hrm_applications`/`_stage_history` (00078, r20) + `hrm_interviews`/`_panelists`/`hrm_interview_scorecards`/`hrm_offers`/`hrm_referrals` (00080, r21).

**Resource-level permissions (r18):** `view_own`/`view_team`/`view_all` scoping (`internal/hrm/scope`) layered on top of every group below plus salary's per-employee endpoints — see Section 9 → PLATFORM PRIMITIVES #3 for the primitive itself; this entry just records that it's now live across the whole module, not a config-only subset.

**Group A — Setup/Config** (`departments, positions, salary, approvals, warningtypes, doctemplates, shifts, holidays, contracts`): 72 routes. Salary formula engine via `expr-lang/expr`. Approvals' instance-list endpoint (`GET /hrm/setup/approvals/instances`) exists — a prior "missing since r9" note here was stale; confirmed against `internal/hrm/approvals/routes.go` 2026-08-03.

**Group B — Lifecycle** (`employees, promotions, transfers, resignations, terminations`): 37 routes + 4 new:

```
GET/POST     /organizations/:orgId/hrm/employee-statuses        ← hrm.employees.view / hrm.employees.manage_setup
PATCH/DELETE /organizations/:orgId/hrm/employee-statuses/:id    ← hrm.employees.manage_setup
```

Dynamic statuses (00053): per-org status list with category (`active/inactive/on_leave/terminated`) + color token; `hrm_employees.status_id` FK; migration auto-seeded defaults and mapped existing employees.

**Onboarding checklist consumer (r19):** `internal/hrm/onboarding/` (thin — does not import `internal/hrm/employees`, its own `FindSubject` query instead, to avoid the import cycle) adds 2 routes:

```
GET  /organizations/:orgId/hrm/employees/:empId/checklists   ← hrm.employees.view
POST /organizations/:orgId/hrm/employees/:empId/checklists   ← hrm.employees.update
```

Reuses `hrm.employees.view`/`.update` rather than minting `hrm.onboarding.*` keys — new keys would need their own seeding, and an unseeded key silently 403s admins. `employees.Service.Create` gained a `ChecklistHook` interface param (breaking `NewService` signature, nil-safe) — on employee creation it auto-instantiates the org's default onboarding template if one exists, synchronously, with panic recovery inside the hook so a checklist-engine bug can never fail employee creation. The POST route above is the manual retry path for when the auto-hook's own failure (logged, not surfaced) needs re-running. See Section 9 → PLATFORM PRIMITIVES #4 for the checklist engine itself.

**Group C — Disciplinary** (`warnings, complaints, employeedocs, acknowledgements`): 34 routes. Cross-module writes via `ON CONFLICT DO NOTHING` + direct `pgPool.Exec` to avoid import cycles.

**Group D — Time & Compensation** (`attendance, payslips`): 20 routes. Multi-punch attendance, `ComputeSlab` progressive tax, immutable finalized payslips.

⚠ **Payroll was non-functional from `00053` until r25 and nothing noticed**, because `ComputeRun` had no integration test and read `*pgxpool.Pool` directly, out of reach of the stub-repo unit tests. r25 fixed four money defects here — a dropped-column employee filter that made every run error, order-dependent gross that let a `display_order` edit swing net pay 11%, silently zeroed negative net, and a partially-written run reporting success — then added the 7A engine columns (`run_type`, `line_type`, `is_employer_contribution`, `source_period_id`) and the mandatory dry-run preview. See the r25 changelog entry; the calculation is now three explicit stages, not one accumulating loop, and 7C/7D slot their line types into that order without re-deriving anyone's pay.

Attendance's `resolveShift` had a column-name bug (`wsa.scope`/`wsa.entity_id` vs the real `assignee_type`/`assignee_id`) that silently no-op'd shift resolution on every call — fixed 2026-08-03 (see Section 5 → PLATFORM — SCHEDULER for the `attendance.absence_sweep` job this also unblocked).

**Compensation (r26) — Phase 7B:** `internal/hrm/compensation/` (20 routes), `hrm.compensation_config.*` (untiered — bands and merit matrix carry no `employee_id`) + `hrm.salary_revisions.*` / `hrm.bonuses.*` (both scope-tiered). Salary revision cycles compute proposed pay per employee from a merit-matrix lookup (rating level × compa-ratio range → increase %), snapshot the whole `CompensationContext` used into a mandatory `calculation_snapshot`, and are batch-approved — one `hrm_approval_instances` row per cycle rather than one per employee. `ApplyCycle` is a distinct step from the approval decision (the `promotions.Apply` precedent) and writes real `hrm_employee_salary_records` rows for every non-excluded revision. Bonuses follow the same snapshot-and-approve shape, then pay out through a `run_type='bonus'` payroll run: `payslips.BonusSource` is a consumer-owned narrow interface declared in `hrm/payslips` (payslips is the consumer), satisfied by `hrm/compensation.Service` — the `recruitment.EmployeeCreator` / `pip.TerminationCreator` direction, so `hrm/compensation` imports `hrm/payslips` and never the reverse. A bonus run skips the normal salary-structure computation entirely (a dedicated branch in `computePayslips`, not a fallthrough) so a bonus never double-pays basic salary alongside itself, and `ComputeRun` marks the underlying bonuses paid atomically, in the same all-or-nothing failure discipline r25's `abortCompute` established. See the r26 changelog entry, including a real bug the integration tests (not inspection) caught: `SubmitCycle`/`ApplyCycle` building a bare `ListFilter{}` silently zero-valued `Scope` to "see nothing."

**Loans + Reimbursements (r27) — Phase 7C:** `internal/hrm/loans/` (7 routes) and `internal/hrm/reimbursements/` (5 routes), `hrm.loans.*` (+ `.disburse`/`.foreclose`, distinct from `.manage`) and `hrm.reimbursements.*`, both scope-tiered. `loans.Amortize` is pure decimal arithmetic (no float pass-through) generating a reducing-balance schedule ONCE at disbursement, never recomputed — the last installment absorbs the rounding remainder, the `ComputeSlab` boundary shape. Both feed lines into every OTHER run type's EXISTING per-employee computation in `computePayslips` (not a dedicated branch the way 7B's bonus run is) via two more consumer-owned narrow interfaces on `payslips`: `LoanSource` and `ReimbursementSource` — `hrm/loans` and `hrm/reimbursements` both import `hrm/payslips`, never the reverse. Loan recovery is capped so it never drives net negative (`headroom = gross - deductions + reimbursements`; a shortfall carries to the next run rather than being written off), which is why `hrm_loan_schedules` tracks `recovered_amount` separately from the frozen `total_amount` and `hrm_loan_recovery_events` exists as an append-only ledger — one schedule row can be recovered across several runs. Reimbursements land straight in `NetPay` without inflating `GrossPay` (non-taxable pass-through, ahead of 7D's statutory engine). Foreclosure marks remaining schedule rows `foreclosed` rather than deleting them. The "resignation" edge case is resolved narrowly: an employee who has fully left simply is not in a later period's eligible set, so their remaining installments stay `pending` — a receivable, not silently recovered or written off — auto-foreclosure-on-exit is explicitly NOT built (it would require redesigning the negative-net guard around the still-unbuilt F&F module). See the r27 changelog entry.

**Statutory + Benefits (r28) — Phase 7D, the final slice of Phase 7:** `internal/hrm/statutory/` (7 routes, `hrm.statutory.*`, untiered) and `internal/hrm/benefits/` (11 routes, `hrm.benefit_plans.*` untiered + `hrm.benefit_enrollments.*` scope-tiered). Statutory ships a country-pluggable `Provider` interface + `Registry` with ONE real, data-driven `SlabProvider` — reading effective-dated `hrm_statutory_slabs` and evaluating via `payslips.ComputeSlab`, the same function `hrm_salary_components`' slab components already use — rather than a bare interface with zero implementations. `computePayslips` grew a `taxableGross` accumulator (summing only earning components flagged `is_taxable`, `00023`) specifically for this: `GrossPay` stays gross salary, `TAXABLE_GROSS` is what a rule actually reads. Both statutory and benefits integrate into the SAME per-employee loop every non-bonus run already computes (the 7C loan/reimbursement shape) via two more consumer-owned narrow interfaces on `payslips` — `StatutorySource`/`BenefitsSource` — so `hrm/statutory` and `hrm/benefits` import `hrm/payslips`, never the reverse. Benefit tier costs are mutable catalog data; `employee_cost_snapshot`/`employer_cost_snapshot` freeze what an enrollment actually costs at signup (the `hrm_compensation_bands` pattern), proved by test: repricing a tier after enrollment leaves the existing payslip deduction unchanged. `benefits.activate_pending_enrollments` is the phase's scheduler consumer (the `certifications.expiry_sweep` shape). **Phase 7 (Compensation depth + Benefits) is now ✅ COMPLETE across all four slices.** See the r28 changelog entry.

**Assets (r29) — Phase 8A:** `internal/hrm/assets/` (21 routes), `hrm.asset_config.*` (untiered catalog — categories + software licences) and `hrm.assets.*` (scope-tiered, with `.assign` and `.request` as keys distinct from `.manage`). Seven tables. **The current holder is a derived query with no backing column** — the assignment row where `returned_at IS NULL`, made single-valued by the partial unique index `uq_hrm_asgn_active`; likewise licence `seats_used` (`COUNT(*)`) and `book_value` (straight-line stub, floored at zero, computed from the CATEGORY's useful life). An integration test introspects `information_schema` to prove eight such columns stay absent. Software licences are a deliberately separate table from physical assets — N seats and a renewal date versus one holder and a serial number. Handover sign-off reuses `hrm_acknowledgements` via `assets.HandoverAcknowledger`, which names acknowledgements' own types so the service satisfies it structurally (the corrected `certifications.SkillGranter` precedent). Asset requests are approval-gated, and fulfilment is a distinct call from the approval decision. ⚠ Scope-tier asymmetry mirrors `hrm.payroll`'s: the tiers govern assignments/requests (which carry `employee_id`), not `hrm_assets` itself — an unassigned asset is org inventory. See the r29 changelog entry, including a two-phase-old `AckType` enum/CHECK drift found and fixed here.

**Travel & Expense (r30) — Phase 8B:** `internal/hrm/expenses/` (22 routes), `hrm.expense_config.*` (untiered — policies, per-diem and mileage rates) plus `hrm.travel.*` and `hrm.expenses.*` (both scope-tiered; separate resources because approving a TRIP and settling the MONEY are different authorities). Nine tables. **Approval is per LINE**: `hrm_expense_lines` carries `amount` vs `approved_amount`, `hrm_expense_claims` carries neither total, and both are `SUM(lines)` at read time. `approved_amount` is nullable because NULL ("undecided") and 0 ("decided, nothing payable") are different states — `SettleClaim` refuses while any line is undecided rather than paying it as zero. Each line snapshots its `exchange_rate` and `base_amount` at claim time (no FX table exists; that is Phase 11), so a later rate change cannot rewrite a settled claim. Policy breaches are recorded warnings in `hrm_expense_policy_violations`, never blocks, checked AFTER the line persists. Per-diem, mileage and policy caps are all effective-dated via 7D's `SlabsAsOf` shape. All three advance-settlement outcomes are handled, and only a positive payable becomes a reimbursement — which is the 7C boundary: `expenses.ReimbursementCreator` (consumer-owned, naming reimbursements' own types) hands the shortfall to `hrm/reimbursements`, whose `payslips.ReimbursementSource` already pays it through payroll. **`internal/hrm/payslips` is untouched by 8B.** See the r30 changelog entry.

**Exit Management (r33) — Phase 9A:** `internal/hrm/exits/` (11 routes), `hrm.exits.*` scope-tiered plus `hrm.exit_config.*` untiered. Three tables. **`hrm_exits` is an UMBRELLA over `hrm_resignations`/`hrm_terminations`, not a replacement** — those own the decision (with their approval chains), this owns the process that follows. `source_type`/`source_id` are polymorphic and FK-FREE (the fourth instance of the `platform_checklist_instances.subject_type` pattern), so ownership is validated in the service instead: an exit pointing at another employee's resignation is refused. **No `hrm_notice_periods` table** despite the build plan naming one — `hrm_resignations` already holds all three notice facts; only the SHORTFALL was new, and it stores DAYS not money (9B applies the rate). A WAIVED notice period is never a shortfall. **Clearance completion is derived, never stored** — `hrm_terminations.exit_clearance_completed` is exactly the denormalized boolean this avoids and Phase 9 never touches it; only a non-zero `blocking_amount` blocks, and resolving never zeroes the amount because a forgiven debt must still show what was forgiven. `hrm_exit_clearance_items` is the seam 9B reads as F&F debits. **`hrm_rehire_eligibility` gives `hrm_terminations.is_rehire_eligible` (unread since `00034`) its first reader**, surfaced to recruitment through the consumer-owned nil-safe `recruitment.RehireChecker` as a derived WARNING on the candidate — never a block, never stored. `hrm.exits.settle` is seeded here but gates F&F approval in 9B. See the r33 changelog entry.

**F&F Settlement (r34) — Phase 9B:** `internal/hrm/exits/` gains gratuity + settlement (4 more routes), `hrm.gratuity.*` (untiered catalogue). Two tables. **F&F is an off-cycle payroll run (`run_type='fnf'`), not a separate calculator** — same line types, same statutory engine, same immutability — and it is the **ADDS-ON** integration shape, not REPLACES: the ordinary per-employee computation runs with the employee set narrowed to the leaver, and settlement credits/debits are appended alongside loan, reimbursement, statutory and benefit lines. `payslips.FnFSource` is the sixth consumer-owned narrow interface, nil-safe like the five before it, and it also answers WHICH employee a run settles — so no `employee_id` column was added to `hrm_payslip_runs`. An F&F run **bypasses the org-wide eligibility filter** (which drops anyone whose `termination_date` precedes the period start — exactly the person being settled). **The negative-net guard is run-type-aware**: negative net is a valid F&F outcome (a receivable), and r25's guard is unchanged for every other run type. **Clearance gates approval, never computation**, so HR can see the figure while clearance is open. `hrm_exit_settlement_lines` is an append-only audit trail with one line per claim and amounts always positive (direction in `is_credit`). Gratuity is effective-dated, partial years are not paid pro rata, below-minimum is zero *with a reason*, and the monthly-to-daily divisor is stored per rule and reused by the notice-shortfall line. **All three cross-module sources are wired (r35)**, each via a consumer-owned nil-safe narrow interface satisfied by the module that owns the data: leave encashment (F&F prices days `hrm/leave` recorded, using `encashment_rate_basis` — a Phase 2 column whose comment always said a future F&F phase would read it; a `fixed` basis produces a zero line saying so, because no column stores the amount), loan foreclosure (the FULL outstanding from schedule rows, never `principal_amount`, with the ordinary per-installment recovery **skipped for `fnf` runs** or the due installment is charged twice) and travel-advance recovery (same-currency only; no FX table exists, so a foreign advance is reported unrecovered rather than converted at a guessed rate). `MarkSettled` closes out the SOURCE, not just the audit trail — otherwise the loan stays active for the next process to charge again. See the r34 and r35 changelog entries.

**Exit Interviews, Documents & Access Revocation (r36) — Phase 9C:** `internal/hrm/exits/` gains 6 routes and 2 scheduler jobs; `hrm.exits.{interview,interview_view,revoke_access}` (actions on the existing resource, so its scope tiers are untouched). One table, `hrm_exit_interviews`, which stores **no responses** — the Phase 5 form engine owns those. **Confidentiality is a separate permission from administration**: `.interview` schedules and sends, `.interview_view` reads what was said, and `.interview_view` is granted to owner/admin and deliberately NOT to manager (who holds `view_team` over exits and is the most likely unwanted reader). Sent **post-departure** via `exits.send_exit_interviews` — `scheduled_for` defaults to the day after last working date and earlier sends are refused, because the timing IS the mechanism. **`relieving_letter` required BOTH `document_type` CHECKs widened** (fourth occurrence of that trap) plus the Go enum AND `IsValid()`; the relieving letter waits for clearance AND settlement, the experience letter never does. **`exits.revoke_departed_access`** suspends `organization_members.status` and calls the existing `auth.LogoutAll` — idempotent (filters on `access_revoked_at IS NULL`, stamps only on success), reversible (suspended not deleted, account intact), and it stamps employees with no platform account so the sweep does not retry them forever. See the r36 changelog entry.

**Group E — Recognition & Communication** (`awards, announcements, calendar, milestones`): 25 routes. Nightly crons for milestones/absences now genuinely run (see Section 5 → PLATFORM — SCHEDULER) — `milestones.generate_upcoming` previously errored on every run against a column `00053` had already dropped, silently, so it never generated anything despite being "wired."

**Leave** (`leave` — types + requests + balances): 22 routes (12 original + 10 added r18), `hrm.leave.*` permissions. Balance tracking (accrual, carry-forward, encashment recording) is opt-in per leave type — a type with no `hrm_leave_policies` row behaves exactly as it always has, zero enforcement. Two new scheduler jobs (`leave.accrue_and_snapshot` daily, `leave.year_end_carry_forward` annually — see Section 5 → PLATFORM — SCHEDULER) and two new owner/admin-only permissions (`hrm.leave.adjust_balance`, `hrm.leave.encash`); balance/ledger reads reuse the existing `hrm.leave.view_own/team/all` tiers, no new scope permissions needed. Encashment records days only — it does not compute a currency amount, that's Section 9 → HRM EXTENDED MODULES → Compensation depth (F&F)'s job once built.

**Reports:** 3 routes.

**Recruitment / ATS (r20 + r21) — internal-only, both sub-phases built:** `internal/hrm/recruitment/` (59 routes total: 34 from r20 + 25 from r21), `hrm.recruitment.*` + `hrm.candidates.*` + `hrm.interviews.*` + `hrm.offers.*` + `hrm.referrals.*` permissions. Phase 4A (r20) shipped the intake-and-pipeline half — requisitions (approval-gated), postings, candidates, applications, configurable pipelines/stages, and `hrm_application_stage_history` (append-only, in the first migration — `crm_deals` never got an equivalent, which is exactly why deal-velocity reporting is impossible today). Phase 4B (r21) shipped the selection-and-close half — interviews + panelists, fixed-shape interview scorecards (visibility: a panelist who hasn't submitted their own scorecard sees only their own draft; everyone else sees every *submitted* scorecard, never a draft — a bespoke service rule, not a Phase 1 scope tier, since those hard-code `FROM hrm_employees`), offers (approval-gated the same way requisitions are), referrals, and hire→employee conversion (`HireApplication` — atomic for the employee insert + application + requisition writes via a new `Repository.BeginTx`; `employees.Repository.CreateTx`/`Service.CreateEmployeeTx` mirror `contacts.CreateContactTx`; `recruitment.EmployeeCreator` is a consumer-owned narrow interface, the `crm/leads.ContactCreator`/`DealCreator` shape exactly — `employees` imports `recruitment`, never the reverse; a `SELECT ... FOR UPDATE` lock makes concurrent hire calls on the same application safe, proved live in an integration test). Deliberately **still no public `/pub/careers/*` route and no candidate email**: the build plan gates that surface on EMAIL SENDING and Capture Fix Pass B, and both were re-verified NOT actually done as of r21 — see the r20 and r21 changelog entries for the full audit.

**Performance / Goals & OKR (r22) — Phase 5A of 3:** `internal/hrm/performance/` (19 routes), `hrm.goals.*` + `hrm.goal_cycles.*` permissions. Goal cycles (`draft → active → locked → closed`, the `hrm_attendance_periods` lifecycle shape), goals with alignment, and append-only check-ins.

Unlike recruitment, goals rows carry an `employee_id`, so this module **does** use the Phase 1 scope tiers — which makes seeding all three (`view_own`/`view_team`/`view_all`) mandatory rather than optional, since `TestPermissions_ScopeTiersSeeded` is all-or-nothing. Writes are gated on `hrm.goals.set_own` (granted through `member`) and narrowed in the service: your own goal needs nothing more, someone else's additionally requires `hrm.goals.manage` **and** passing `AuthorizeRecordAccess` — `manage` is unscoped at the route, so only that second check stops a `view_team` manager editing outside their reporting line. `hrm.goals.manage` therefore never appears in a `permFn(...)` call.

Three properties worth knowing before touching this module: **progress is computed, never stored** on `hrm_goals` (one direction-agnostic formula; `direction` validates but is deliberately not an input to the arithmetic); **`current_value` moves only through a check-in**, which is what guarantees the history has no holes; and **`locked` is two-axis** — it freezes goal definitions while check-ins keep landing, the normal in-flight state for a quarter.

**Performance / Appraisal cycles (r23) — Phase 5B of 3:** 21 more routes in the same
`internal/hrm/performance/` package (rating scales, levels, appraisal cycles, appraisals),
`hrm.appraisals.*` + `hrm.rating_scales.*` permissions. Built as more sub-feature quartets rather
than a new package so appraisals read goal attainment through a plain method call — `goalAttainment()`
is the 5A→5B tie-in, computing Σ(weight × clamped progress) / Σ(weight) over the appraisee's OWN
goals only.

The **6-phase transition map** is the first explicit one in this codebase (`allowedPhaseTransitions`,
with `IsTerminal()` derived from it so they cannot disagree) — a deliberate deviation from the
house's inline-guard style, justified by 6 phases × per-transition guards ≈ 15 scattered checks.
Two transitions run backward so a rejected review is recoverable without cancelling it, and the
`manager_review → self_review` send-back must NOT re-demand the self form.

**`final_rating` is both a FK and a snapshot**, deliberately: `final_rating_level_id →
hrm_rating_scale_levels` is what Phase 7's merit matrix and Phase 10's 9-box query;
`final_rating_label` + `final_rating_value` survive the level being renamed or deleted (`ON DELETE
SET NULL`). **Publish snapshots** the self/manager scores and goal attainment rather than
recomputing them — 5A goals stay editable forever, so an immutable record recomputing from them
would not actually be immutable. Both properties are pinned by integration tests.

Scope tiers are mandatory here for the same reason as goals, and this is the module Section 9's
primitive note singles out: an unpublished appraisal is the most sensitive employee record the
system holds. `calibrate` and `publish` carry their own permissions that `manager` does not hold —
calibration exists precisely to adjust manager ratings.

**360 Feedback (r23) — Phase 5C part 1:** `internal/hrm/feedback/` (12 routes), `hrm.feedback.*`
permissions. Its own package because `performance`'s composite Repository hit 58 methods, against
the ~60 split threshold 5A recorded.

The anonymity contract is the whole point of the module and is structural, not documentary — the
codebase's one prior anonymity feature (`hrm_complaints.is_anonymous`) is a stored boolean nothing
branches on, and this module is built specifically not to repeat it:

- **Derived, never stored.** `Relationship.IsAnonymous()` is the single source of truth. No column
  exists that could be set to a value the system does not honour.
- **Two read paths, two types, no shared field.** `FindRequestSummaries` (identity, no answers)
  and `FindSubmittedForSubject` (relationship + form instance only, no identity) are separate
  queries; `RequestSummary` and `AnonymousResponse` are separate types, neither being the other
  with fields blanked. `hrm.feedback.coordinate` and `hrm.feedback.view` are separate keys and no
  role holds both paths' output together.
- **No form instance id ever reaches a subject.** `platform_form_instances` stores
  `respondent_user_id`, so an id plus `GET /forms/instances/:id` defeats everything above. The
  service reads instances server-side; `SubmittedRef.FormInstanceID` carries `json:"-"` as a
  second line of defence. This is the one leak living outside the module.

Two rules that look like oversights and are not. **`self` and `manager` are attributed by nature** —
there is exactly one manager, so anonymising them identifies them with certainty while pretending
otherwise, and a threshold they can never reach would make the cycle's most actionable feedback
permanently unreadable. **Suppression is per relationship group** (`min_responses` on the cycle,
default 3) — five responses of which one is a direct report still identify that direct report. A
suppressed group reports neither its count nor its score, and `TotalResponses` excludes it so the
hidden size cannot be recovered by subtraction. Suppression applies to `view_all` too.

**PIP (r23) — Phase 5C part 2:** `internal/hrm/pip/` (9 routes), `hrm.pips.*` permissions.

A `failed` outcome creates a **DRAFT** `hrm_terminations` row and stops — no submit, no approval
instance, no application. `hrm_terminations` owns its own approval-gated lifecycle, and a PIP that
advanced past draft would bypass the control that exists to gate dismissals; the integration test
asserts both the `draft` status and zero approval instances. The seam is `pip.TerminationCreator`,
declared in `internal/hrm/pip` with `terminations` importing `pip` — consumer-owned narrow
interface, the `recruitment.EmployeeCreator` direction exactly. The close is **not** atomic with
the handoff: the plan commits first and a handoff failure returns the closed plan alongside
`ErrTerminationHandoff`, because the reverse ordering risks a dismissal document with no process
behind it. `termination_id` is `ON DELETE SET NULL` so deleting a mistaken draft is not blocked and
does not erase the PIP's record of having failed.

`hrm.pips.close` is separate from `.manage` and `manager` holds only `.manage`. `end_date` moves
**only** through `/extend`, which writes the new date and its mandatory reason in one transaction;
`original_end_date` is frozen at creation so extensions stay legible. `uq_hrm_pip_employee_open` is
a partial unique index enforcing one open plan per employee — proved under 6 concurrent creates,
since the service pre-check alone loses that race.

**Learning & Development (r24) — Phase 6A of 2:** `internal/hrm/learning/` (29 routes),
`hrm.courses.*` + `hrm.enrollments.*` permissions, eight tables (`00092`/`00093`).

**Content hangs off a VERSION, not the course.** `hrm_course_modules` references
`hrm_course_versions`, and `hrm_enrollments` pins `version_id` with a **RESTRICT** FK. Publishing a
new version leaves an existing learner on the content they actually took, and the pinned version
cannot be deleted from under them — both proved by integration test. Only a `draft` version is
editable; every content write (module, lesson, version metadata) passes a single
`assertEditableVersion` gate rather than repeating the check six times, because the one place it
gets forgotten is the one that corrupts a published version.

**The form engine could not do assessments, and that reshaped the design.**
`platform_form_questions` has no correct-answer column and no pass mark — `computeScore` produces a
weighted RATING, not a mark. So `hrm_quiz_answer_keys` is owned here, keyed on
`platform_form_questions.id`. Three consequences worth knowing:

- `platform/forms` keeps **zero** assessment semantics, so appraisals and 360 feedback carry no
  `correct_answer` column they never read.
- The protection is **structural**: `hydrateAttempt` fetches the form instance and maps each
  response to `QuestionForAttempt` (a distinct type with no correct-answer field) and never calls
  `FindAnswerKeysForTemplate`. There is no key in scope to forget to strip — the 5C anonymity shape.
- **Grading happens once, at submit, and the result is stored.** It is never re-derived, because
  `platform_form_responses.question_id` is `ON DELETE SET NULL` and the key is `ON DELETE CASCADE`:
  deleting a question would make a re-grade silently score zero. `pass_mark_snapshot` is frozen on
  the attempt for the same reason — raising a lesson's pass mark must not retroactively fail
  somebody.

`GET .../lessons/:lessonId/answer-keys` is the ONE endpoint returning correct answers and gates on
`hrm.enrollments.grade` — a key `manager` deliberately does not hold, since a manager who can read
the answers to a quiz they are assigning has defeated it. The WRITE side gates on
`hrm.courses.manage` instead: authoring and marking are different jobs.

Completion is **computed** from `hrm_lesson_progress`, never stored (the `00076` rule); an
integration test introspects `information_schema` to assert no percentage column exists. A quiz
lesson is completed by PASSING an attempt, never by asserting completion. Grading arithmetic has
32 unit tests written before any dependent layer: multi-select partial credit is
`(hits − misses) / expected` floored at zero, so selecting every option scores **zero**, not full
marks — the failure mode a naive "count the hits" implementation ships.

**Certifications + Skills (r24) — Phase 6B of 2:** `internal/hrm/certifications/` (9 routes) and
`internal/hrm/skills/` (9 routes), `hrm.certifications.*` + `hrm.skills.*` permissions, five tables
(`00094`/`00095`).

Separate packages from `learning` for two reasons that agreed: its composite Repository hit 53
methods in 6A against the ~60 split threshold Phase 5A recorded, and skills is explicitly a
**shared taxonomy** — Phase 10 succession imports `internal/hrm/skills` directly rather than
reaching through an LMS dependency it has no other use for.

**The expiry sweep** is the phase's highest-value feature, registered as
`scheduler.Register("certifications.expiry_sweep", "0 4 * * *", …)` and running instance-wide like
the leave and absence sweeps. Three properties, each with an integration test:

- **Order is fixed** — mark `expiring` (within 30 days) first, THEN `expired`. Reversed, something
  that already lapsed gets flagged as an upcoming warning.
- **The boundary is strict**: `expires_at < CURRENT_DATE`. A credential expiring TODAY is still
  valid today; `<=` cuts somebody off a day early, which for a safety certification is a real
  operational error.
- **`expiry_notified_at` stops re-notification**, so the job does not re-flag the same credential
  every night for a month until the reminder becomes noise.

`expires_at` is derived from `validity_months` via `AddDate` (months are not a fixed number of
hours) and **frozen at issue** — changing the catalogue's validity never moves an issued
credential. A NULL validity means "never expires", which stays distinguishable from "expires
today". Revoking frees the employee for re-issue, which is what makes
`uq_hrm_ecrt_employee_cert_live` partial rather than absolute.

**`hrm_position_skills` is deliberately NOT built**, against the build plan's own list — see
Section 9 → HRM Extended → Learning & Development for why. Issuing a certification that carries a
`skill_id` records that skill with `source='certification'` and a pointer back to the credential;
that is the in-phase consumer justifying the taxonomy now. The grant is best-effort: a credential
that issued must not roll back because the derived skill record failed, so the handler reports the
partial success rather than an error.

The weight rule is `≤ cycle.weight_target` at write time and `== target` only at lock. Enforcement is in the repository, not the service: `CreateGoalGuarded` locks the **employee row** inside its transaction, because locking sibling goals cannot stop a competing INSERT that was in neither locked set. Phase 5B (form engine + appraisal cycles) and 5C (360 feedback + PIP) are designed but unbuilt — see Section 9 → HRM EXTENDED MODULES → Performance Management.

Resumes: PDF-only, content-sniffed (never trusts the file extension), stored in `backend/storage/resumes/` — **not** `./uploads`, which is served fully unauthenticated. Download is gated on its own `hrm.candidates.download_resume` permission, separate from `.view`.

Deliberately **no `authz.Service.ResolveScope` calls** anywhere in this module — candidates/applications have no `employee_id`, so the Phase 1 scope tiers (`internal/hrm/scope`) are structurally inexpressible for them. Flat RBAC only; any `hrm.candidates.view` holder sees every candidate in the org.

**Approval chain wiring:** callback registry on the approvals service; promotions, transfers, terminations, warnings, awards, (r20) job requisitions, and (r21) offers each register a decision callback in `main.go` (`HandleApprovalDecision` / `HandleOfferApprovalDecision`).

**Extension surface (r15):** scoped but unbuilt — see Section 9 → HRM EXTENDED MODULES. Recruitment/ATS (both Phase 4A and 4B) is now built (above); the rest of the extension surface (Phases 5–11) remains scoped but unbuilt.

---

### AUDIT [✅ DONE]

Internal only. Append-only log for security-sensitive events. No public API endpoints.

---

## 6. DATABASE

### Conventions

- UUID primary keys everywhere (`id` = internal, `public_id` = API-safe where exposed)
- `created_at`, `updated_at` on every table
- Indexes on lookup-heavy columns (org_id, user_id, email, status)
- All schema changes via Goose migration files — never manually
- Transactions for multi-step operations (org creation, membership changes, lead conversion, approval decisions)
- Audit logs and webhook logs are append-only

### Migration Count: 109

Files live in `backend/internal/migrations/`. Run via `goose` or `make migrate`.
r11 ended at 64. Post-r11: `00065` tasks `related_type`/`related_id` context + `tasks.view_all`
permission (not the `crm_leads.created_by` fix Fix Pass A originally expected — that's still
open) · `00066` HRM money `float64`→`decimal.Decimal` + `organizations.money_rounding_scale/mode`
+ payslip totals widened to `NUMERIC(18,4)` · `00067` scheduler tables
(`platform_scheduled_jobs`, `platform_job_runs`) · `00068` notification tables
(`platform_notifications`, `platform_notification_preferences`) · `00069` sentinel system user
for scheduler-triggered writes · `00070` `hrm_legal_entities` + `legal_entity_id` backfilled onto
36 HRM tables · `00071` `currency CHAR(3)` prep (new columns + standardized existing ones) ·
`00072` seeds `view_own`/`view_team`/`view_all` per resource per role (Phase 1) · `00073`
backfills that tier onto custom roles/member overrides holding bare `.view` · `00074`
`hrm_leave_policies`/`hrm_leave_transactions`/`hrm_leave_balances` (Phase 2) · `00075` seeds
`hrm.leave.adjust_balance`/`hrm.leave.encash`, owner/admin only · `00076`
`platform_checklist_templates`/`_template_items`/`_instances`/`_instance_items` (Phase 3) · `00077`
seeds `platform.checklists.view`/`.complete`/`.manage` · `00078`
`hrm_recruitment_pipelines`/`_stages`/`hrm_job_requisitions`/`_postings`/`hrm_candidates`/
`hrm_applications`/`_stage_history` (Phase 4A) + adds `'job_requisition'` to both approval CHECK
constraints · `00079` seeds `hrm.recruitment.view`/`.manage`, `hrm.candidates.view`/`.manage`/
`.download_resume` · `00080` `hrm_interviews`/`_panelists`/`hrm_interview_scorecards`/`hrm_offers`/
`hrm_referrals` (Phase 4B) + `hrm_employees.source_candidate_id` + adds `'offer'` to both approval
CHECK constraints and to `hrm_employee_documents.related_type` · `00082`
`hrm_goal_cycles`/`hrm_goals`/`hrm_goal_checkins` (Phase 5A) · `00083` seeds
`hrm.goals.view`/`.manage`/`.set_own` + the three mandatory scope tiers +
`hrm.goal_cycles.view`/`.manage` · `00081` seeds
`hrm.interviews.view`/`.manage`/`.scorecard`, `hrm.offers.view`/`.manage`,
`hrm.referrals.view`/`.manage` · `00084` `platform_form_templates`/`_sections`/`_questions`/
`_instances`/`_responses` (the form engine, Phase 5B) · `00085` seeds `platform.forms.view`/
`.respond`/`.manage` · `00086` `hrm_rating_scales`/`_levels`/`hrm_appraisal_cycles`/
`hrm_appraisals`/`hrm_appraisal_phase_history` (Phase 5B) + widens
`hrm_acknowledgements.acknowledgeable_type` with `'appraisal'` · `00087` seeds
`hrm.appraisals.view`/`.manage`/`.respond`/`.review`/`.calibrate`/`.publish` + the three mandatory
scope tiers + `hrm.rating_scales.view`/`.manage` · `00088` `hrm_feedback_cycles`/
`hrm_feedback_requests` (Phase 5C) · `00089` seeds `hrm.feedback.view`/`.manage`/`.coordinate`/
`.respond` + the three mandatory scope tiers · `00090` `hrm_pips`/`hrm_pip_checkins` (Phase 5C) ·
`00091` seeds `hrm.pips.view`/`.manage`/`.close` + the three mandatory scope tiers ·
`00092` `hrm_courses`/`_versions`/`_modules`/`_lessons`/`hrm_enrollments`/`hrm_lesson_progress`/
`hrm_quiz_attempts`/`hrm_quiz_answer_keys` (Phase 6A) · `00093` seeds `hrm.courses.view`/`.manage`
+ `hrm.enrollments.view`/`.manage`/`.enroll_self`/`.attempt`/`.grade` + the three mandatory scope
tiers · `00094` `hrm_skills`/`hrm_certifications`/`hrm_employee_certifications`/
`hrm_employee_skills`/`hrm_enrollment_rules` (Phase 6B) + widens
`hrm_acknowledgements.acknowledgeable_type` with `'course_completion'` · `00095` seeds
`hrm.certifications.*` and `hrm.skills.*` + two sets of the three mandatory scope tiers.

⚠ `00094` creates `hrm_skills` BEFORE `hrm_certifications` (the latter has a `skill_id` FK), and
its Down block drops them in the opposite order — certifications first, skills last. Reordering
either into "logical" order breaks the migration; the drop-order half was caught only by actually
running the rollback, which is why reversibility is proved rather than assumed.

`00096` Phase 7A payroll engine — **no new tables**: `hrm_payslip_runs.run_type` plus
`hrm_payslip_lines.line_type`/`is_employer_contribution`/`source_period_id`, both backfilled
before the NOT NULL lands · `00097` seeds `hrm.payroll.preview` to owner/admin/manager.

⚠ `00096` **DROPS `uq_hrm_pr_org_month`** and replaces it with the partial unique index
`uq_hrm_pr_org_month_regular … WHERE run_type = 'regular'`. Without that, `run_type` could never
hold a second value in a period and the whole feature is inert. Its Down block re-adds the
original blanket constraint, which **fails loudly** if the org has since created two runs in one
month — the state this migration makes reachable — rather than silently discarding one.

⚠ `00096`'s header records what must **never** be added to it: a
`CHECK (line_type <> 'arrear' OR source_period_id IS NOT NULL)`. `source_period_id` is
`ON DELETE SET NULL`, Postgres re-evaluates CHECKs on UPDATE, and `SET NULL` *is* an UPDATE — so
that pairing would make `DELETE FROM hrm_payslip_runs` fail 23514 for any org holding an arrear
line. This is the `00076` trap; the service validates the pairing instead.

`00098` Phase 7B compensation — 5 new tables: `hrm_compensation_bands`, `hrm_merit_matrix_cells`,
`hrm_salary_revision_cycles`, `hrm_salary_revisions`, `hrm_bonuses`; widens BOTH
`hrm_approval_templates.action_type` (short form) and `hrm_approval_instances.entity_type` (long
form) with `salary_revision`/`bonus` · `00099` seeds `hrm.compensation_config.*` (untiered) and
the three mandatory scope tiers each for `hrm.salary_revisions` and `hrm.bonuses`.

⚠ `00098` widens **two separate CHECK constraints with two separate vocabularies** for the same
two new entity types — `hrm_approval_templates_action_type_check` uses `'promotion'`/`'leave'`,
`hrm_approval_instances_entity_type_check` uses `'leave_request'`. Missing either one leaves a
template creatable but no instance of it possible, or the reverse. See the r26 changelog entry.

`00100` Phase 7C loans + reimbursements — 4 new tables: `hrm_loans`, `hrm_loan_schedules`,
`hrm_loan_recovery_events`, `hrm_reimbursements`; widens the SAME two CHECK constraints 00098
already widened once, again with `salary_revision`/`bonus` alongside the new `loan`/`reimbursement`
· `00101` seeds `hrm.loans.*` (+ `.disburse`/`.foreclose`, distinct from `.manage`) and
`hrm.reimbursements.*`, both fully scope-tiered.

⚠ `00100`'s amortization schedule is generated ONCE at disbursement and never recomputed —
`hrm_loan_schedules.total_amount` is frozen; only `recovered_amount` (and derived `status`) ever
change, tracked because the zero-net-pay guard can spread one installment's recovery across
multiple payroll runs. `hrm_loan_recovery_events` is the append-only ledger this makes necessary —
see the r27 changelog entry.

`00102` Phase 7D statutory — 2 new tables: `hrm_statutory_rules` (stable identity), `hrm_statutory_slabs`
(effective-dated bracket data, evaluated via `payslips.ComputeSlab`) · `00103` seeds `hrm.statutory.*`,
untiered, owner/admin only.
`00104` Phase 7D benefits — 4 new tables: `hrm_benefit_plans`, `hrm_benefit_tiers` (mutable catalog),
`hrm_benefit_enrollments` (`employee_cost_snapshot`/`employer_cost_snapshot` frozen at enrollment),
`hrm_dependents` (manually verified) · `00105` seeds `hrm.benefit_plans.*` (untiered) and
`hrm.benefit_enrollments.*` (scope-tiered + `.enroll_self` + `.verify_dependent`).

`00106` Phase 8A assets — 7 new tables: `hrm_asset_categories`, `hrm_assets`,
`hrm_asset_assignments`, `hrm_asset_maintenance_logs`, `hrm_asset_requests`,
`hrm_software_licenses`, `hrm_license_seat_assignments`; widens
`hrm_acknowledgements.acknowledgeable_type` with `'asset_handover'` AND both approval CHECKs with
`'asset_request'` · `00107` seeds `hrm.asset_config.*` (untiered) and `hrm.assets.*`
(scope-tiered, + `.assign` / `.request` as distinct keys).

⚠ `00106` deliberately creates NO `current_holder` column on `hrm_assets` and NO `seats_used` on
`hrm_software_licenses`. Both are derived; `uq_hrm_asgn_active` (a partial unique index over
`returned_at IS NULL`) is what makes the derived holder single-valued. An integration test
introspects `information_schema` to assert eight such columns stay absent — see the r29 entry.

`00108` Phase 8B travel & expense — 9 new tables: `hrm_travel_requests`,
`hrm_travel_itinerary_items`, `hrm_travel_advances`, `hrm_expense_claims`, `hrm_expense_lines`,
`hrm_expense_policies`, `hrm_per_diem_rates`, `hrm_mileage_rates`,
`hrm_expense_policy_violations`; widens both approval CHECKs with `'travel_request'` /
`'expense_claim'` · `00109` seeds `hrm.expense_config.*` (untiered) plus `hrm.travel.*` and
`hrm.expenses.*` (both scope-tiered, with `.approve_lines` and `.disburse_advance` as distinct keys).

⚠ `00108` deliberately gives `hrm_expense_claims` NO total columns — approval is per-line, so both
totals are `SUM(hrm_expense_lines)` at read time. `hrm_expense_lines.approved_amount` is nullable
on purpose: NULL is "undecided", 0 is "decided, nothing payable". An integration test introspects
`information_schema` to assert four forbidden total columns stay absent.

⚠ `00108`'s Down block drops lines BEFORE mileage rates, because `fk_hrm_expl_mileage_rate` is
added after both tables exist — not the mirror of CREATE order.

⚠ `00106` widens THREE constraints, and its Down block restores each to its exact r28 state —
verified by asserting `reimbursement` and `course_completion` are still present after rollback,
not just that the new values are gone. Over-reverting is as broken as under-reverting.

⚠ A statutory rule's bracket table is revised by inserting a WHOLE NEW SET of `hrm_statutory_slabs`
rows sharing one `effective_date` — never by editing a bracket in place. `SlabsAsOf` groups by
`MAX(effective_date) <= asOf`, which is what makes "the current table" well-defined instead of a
mix of brackets from different revisions; this grouping is the exact mechanism the r28 changelog's
mandatory effective-dating test proved non-vacuous by breaking.

### Key Tables (161 total)

**Core / auth / org (14):**
`users` · `organizations` · `organization_members` · `organization_invitations` · `permissions` · `roles` · `auth_accounts` · `sessions` · `login_events` · `verification_tokens` · `subscriptions` · `organization_usage` · `audit_logs` · `tasks`

**Platform (14):**
`platform_contacts` · `platform_companies` · `platform_notes` · `platform_tasks` · `platform_activities` · `platform_email_logs` · `platform_scheduled_jobs` · `platform_job_runs` · `platform_notifications` · `platform_notification_preferences` · `platform_checklist_templates` · `platform_checklist_template_items` · `platform_checklist_instances` · `platform_checklist_instance_items`

**CRM (6):**
`crm_leads` (+ `custom_fields`, `capture_source`, `capture_metadata` since 00058) · `crm_pipelines` · `crm_pipeline_stages` · `crm_deals` · `crm_templates` · `crm_settings`

**Capture (7):**
`org_api_keys` · `org_inbound_emails` · `social_integrations` · `website_visitors` · `visitor_pageviews` · `inbound_email_logs` · `social_lead_logs`

**HRM (52):**
`hrm_legal_entities` (prep migration `00070`, zero business logic yet — see PREP MIGRATIONS in Section 9) ·
Group A (19): `hrm_departments` · `hrm_positions` · `hrm_salary_components` · `hrm_salary_structures` · `hrm_salary_structure_components` · `hrm_approval_templates` · `hrm_approval_template_levels` · `hrm_approval_instances` · `hrm_approval_decisions` · `hrm_warning_types` · `hrm_warning_escalation_rules` · `hrm_document_templates` · `hrm_document_bulk_sends` · `hrm_shifts` · `hrm_work_schedule_assignments` · `hrm_holiday_calendars` · `hrm_holidays` · `hrm_calendar_assignments` · `hrm_employee_contracts`
Group B (6): `hrm_employees` · `hrm_promotions` · `hrm_transfers` · `hrm_resignations` · `hrm_terminations` · `hrm_employee_statuses`
Group C (4): `hrm_employee_warnings` · `hrm_complaints` · `hrm_employee_documents` · `hrm_acknowledgements`
Group D (6): `hrm_attendance_periods` · `hrm_attendance_records` · `hrm_employee_salary_records` · `hrm_payslip_runs` · `hrm_payslips` · `hrm_payslip_lines`
Group E (4): `hrm_awards` · `hrm_announcements` · `hrm_calendar_events` · `hrm_employee_milestones`
Leave (5): `hrm_leave_types` · `hrm_leave_requests` · `hrm_leave_policies` · `hrm_leave_transactions` · `hrm_leave_balances`
Performance (3): `hrm_goal_cycles` · `hrm_goals` · `hrm_goal_checkins`
Recruitment (12): `hrm_recruitment_pipelines` · `hrm_recruitment_stages` · `hrm_job_requisitions` · `hrm_job_postings` · `hrm_candidates` · `hrm_applications` · `hrm_application_stage_history` · `hrm_interviews` · `hrm_interview_panelists` · `hrm_interview_scorecards` · `hrm_offers` · `hrm_referrals`
Compensation (5): `hrm_compensation_bands` · `hrm_merit_matrix_cells` · `hrm_salary_revision_cycles` · `hrm_salary_revisions` · `hrm_bonuses`
Loans + Reimbursements (4): `hrm_loans` · `hrm_loan_schedules` · `hrm_loan_recovery_events` · `hrm_reimbursements`
Travel & Expense (9): `hrm_travel_requests` · `hrm_travel_itinerary_items` · `hrm_travel_advances` · `hrm_expense_claims` · `hrm_expense_lines` · `hrm_expense_policies` · `hrm_per_diem_rates` · `hrm_mileage_rates` · `hrm_expense_policy_violations`
Assets (7): `hrm_asset_categories` · `hrm_assets` · `hrm_asset_assignments` · `hrm_asset_maintenance_logs` · `hrm_asset_requests` · `hrm_software_licenses` · `hrm_license_seat_assignments`
Statutory (2): `hrm_statutory_rules` · `hrm_statutory_slabs`
Benefits (4): `hrm_benefit_plans` · `hrm_benefit_tiers` · `hrm_benefit_enrollments` · `hrm_dependents`

⚠ This listing is not exhaustive — appraisals/feedback/PIP/learning/certifications/skills tables
(r23/r24) are missing from it, a pre-existing gap this revision did not audit. Re-grep
`internal/migrations/*.sql` `CREATE TABLE` statements before trusting this section as a full
inventory; the doc's own drift-assumption rule applies here.

---

## 7. FRONTEND ARCHITECTURE

### Folder Structure

```
frontend/
  src/
    app/
      (auth)/                 ← login, signup
      (onboarding)/           ← create-organization, select-organization
      (dashboard)/
        [orgId]/
          crm/
            leads/  pipeline/  reports/  agenda/  visitors/    ← visitors added, undocumented until 2026-08-03
            setup/routing/  setup/templates/
          contacts/  companies/  companies/[companyId]/
          tasks/
          hrm/                ← all HRM pages incl. setup/statuses
          settings/ (members, roles, profile, apikeys, integrations)  ← apikeys/integrations added, undocumented until 2026-08-03
          security/sessions/
          coming-soon/        ← generic placeholder route, undocumented until 2026-08-03
      api/lead-capture/       ← Next.js route handler proxying to /pub/leads, undocumented until 2026-08-03
    components/
      ui/  layout/  crm/  tasks/  members/  roles/  settings/  hrm/  providers/
      notifications/NotificationDrawer.tsx   ← added 2026-08-03, see Section 8
    contexts/DrawerContext.tsx
    lib/
      api.ts  auth.ts  constants.ts  token.ts  jwt.ts  queryKeys.ts
      crm/ (leads, contacts, companies, deals, pipelines, reports, settings, templates)
      hrm/ (…)
      members.ts  roles.ts  org.ts  profile.ts  security.ts  tasks.ts  permissionGroups.ts
      apikeys.ts  integrations.ts  visitors.ts  dashboard.ts  notifications.ts   ← all added, undocumented until 2026-08-03
    stores/ (authStore, permissionStore, uiStore, commandStore)   ← commandStore was missing from this list
    hooks/ (useIsMobile, …)
    types/ (api, auth, crm, hrm, org, rbac, task, notification)   ← notification type added 2026-08-03
```

### Auth Flow (frontend)

1. Login → POST `/api/v1/auth/login` → receive `{ access_token, expires_in }` in body, refresh cookie set by server automatically
2. Store `access_token` in `lib/token.ts` module variable — never in Zustand, never in localStorage
3. Set user + org in `authStore`, permissions in `permissionStore`
4. Every API request adds `Authorization: Bearer <token>` via Axios request interceptor
5. On 401 → Axios response interceptor calls POST `/api/v1/auth/refresh` → receive new `access_token` → update module variable → retry original request
6. Logout → POST `/api/v1/auth/logout` → clear token variable → reset all stores
7. On page refresh → silent refresh on app mount: call `/auth/refresh` before rendering any protected UI; if it fails, redirect to login

### Zustand Store Definitions

**Four stores, not three** — `commandStore` was missing from this section entirely until the
2026-08-03 audit. Interfaces below are copied from the real source
(`frontend/src/stores/*.ts`), not the previous doc's assumed shape, which had drifted on both
`authStore` and `permissionStore`.

**`authStore`** — who the user is and which org they're in:

```ts
interface AuthState {
  user: SafeUser | null;
  currentOrg: Business | null;
  currentMembership: MembershipWithRole | null; // was documented as `membership`; there is no `status` field at all
  setUser: (user: SafeUser | null) => void;
  setOrg: (org: Business | null, membership: any) => void;
  reset: () => void;
}
```

**`permissionStore`** — what the user can do in the current org:

```ts
interface PermissionState {
  permissions: string[];
  setPermissions: (permissions: string[]) => void;
  hasPermission: (permission: string) => boolean; // was documented as `can`; `canAny` doesn't exist
  reset: () => void;
}
```

**`uiStore`** — sidebar, mobile menu, and theme (persisted: `sidebarCollapsed` + `theme` only):

```ts
interface UIState {
  sidebarCollapsed: boolean;
  mobileMenuOpen: boolean; // not previously documented
  theme: "light" | "dark";
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;
  toggleMobileMenu: () => void;
  setMobileMenuOpen: (open: boolean) => void;
  setUiTheme: (theme: "light" | "dark") => void;
}
```

**`commandStore`** — the ⌘K command menu's open state, nothing else (missing from this doc
entirely until now):

```ts
interface CommandState {
  isOpen: boolean;
  setOpen: (open: boolean) => void;
  toggle: () => void;
}
```

**Hard rules for Zustand:**

- Never add `persist` middleware to `authStore` or `permissionStore` — persisted auth state is a security risk
- `persist` is allowed on `uiStore` (and `commandStore`, though nothing in it needs persisting today)
- The access token lives in `lib/token.ts` as a plain module variable — not in any store
- If you see a PR that puts a token into Zustand, reject it

### Theme Implementation

`next-themes` wraps the app at the root layout level with `defaultTheme="light"`. Tailwind's `dark:` classes handle all visual switching. CSS variables define the token values per theme in `globals.css`:

```css
:root {
  --bg-canvas: #f8fafc;
  --bg-surface: #ffffff;
  --border: #e2e8f0;
  --accent: #4f46e5;
  --accent-hover: #4338ca;
  --success: #10b981;
  --warning: #f59e0b;
  --destructive: #ef4444;
}

.dark {
  --bg-canvas: #020617;
  --bg-surface: #0f172a;
  --border: #1e293b;
  --accent: #4f46e5;
  --accent-hover: #4338ca;
  --success: #10b981;
  --warning: #f59e0b;
  --destructive: #ef4444;
}
```

Components use `dark:` Tailwind classes — never inline theme checks in JS. `uiStore.theme` drives `setTheme()` and persists to localStorage (safe to persist).

### Organization Context Flow

1. After login, user lands on org selection screen if no org is active
2. User selects org → POST `/api/v1/organizations/:id/switch` → receive new `access_token` with `business_id` claim
3. Store new token, store `orgId` in context and URL (`/[orgId]/...`)
4. All subsequent API calls use the org-scoped URL pattern

### Permission Pattern (frontend)

```tsx
const { hasPermission } = usePermissionStore(); // no API call — reads the in-memory store directly

{
  hasPermission("tasks.create") && <Button onClick={openCreateModal}>New Task</Button>;
}
```

(This sample previously called a `usePermission()` hook with a `can()` method — neither exists.
Every real call site in the codebase uses `usePermissionStore()` + `hasPermission()` directly, e.g.
`frontend/src/app/(dashboard)/[orgId]/tasks/page.tsx`.)

`permissionStore` is populated after org switch by `GET /api/v1/members/me`. Backend enforces on every request — frontend gates are UX only.

### API Client Contract

```ts
// lib/token.ts — access token lives here, nowhere else
let accessToken: string | null = null;
export const getToken = () => accessToken;
export const setToken = (t: string | null) => {
  accessToken = t;
};

// lib/api.ts
const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL,
  withCredentials: true,
});
// Request interceptor → attach Authorization: Bearer <token>
// Response interceptor → on 401: /auth/refresh → setToken(new) → retry
```

**Why not Zustand for the token:** Zustand state can be accidentally persisted via the `persist` middleware, putting the token in `localStorage`. A plain module variable lives only in memory and is cleared on page refresh (triggering the silent refresh flow).

### Component Quality Standard

- Fully themed — light and dark both look intentional, not just inverted
- Indigo accent on interactive elements in both modes — actions and active states, not decoration
- One typeface family with a strict scale; no separate display font
- Clear action hierarchy: primary filled, secondary outlined, tertiary text links — never more than one primary action visible at once
- Skeleton loading states on every data-fetching component
- Keyboard accessible: visible focus rings, tab order matches visual order, escape closes modals
- WCAG AA contrast, explicit ARIA landmarks on layout regions
- Responsive (works on 1280px minimum, designed for 1440px)
- No placeholder "coming soon" states — if a page is in scope, it is built properly
- Error states and empty states are designed, not default browser behavior

### Dashboard Page Pattern

Default anatomy for data-overview pages (CRM Reports, HRM Reports, org dashboard home):

1. **KPI strip** — 3–4 metric cards: absolute value, small sparkline, percentage change with directional arrow
2. **Data visualization** — split-pane: primary time-series chart + secondary category bar chart
3. **Data table** — full-width: search, multi-select filters, export; hover states, status badges, `⋯` context menu

List views that aren't overview pages (Leads, Tasks, Members) skip the KPI/chart layers and go straight to the table.

### Collection View Pattern (Notion-style)

Default anatomy for single-entity collection pages where each record is a lightweight,
frequently-edited item — first example is Tasks. Distinct from the Dashboard Page Pattern:
no KPI strip, no charts, this is a working list, not a report.

1. **Toolbar** — page title + record count top-left; view switcher (e.g. Grouped/List) +
   primary "New" action top-right.
2. **Grouped view (default)** — records grouped by their primary status/category field into
   collapsible sections; each header shows a status dot, label, count, and a hairline divider —
   no bordered card around the section.
3. **List view (alternate)** — flat list with filter tabs above it (same status field), for
   scanning everything in one scroll.
4. **Row = borderless, chrome-less** — no grid lines, no card border; hover background is the
   only structural cue between rows. Primary fields (title, status, date, done-toggle) are
   inline-editable directly on the row — no separate edit mode. A drawer is reserved for
   secondary fields (e.g. description) via an explicit "Edit" action.
5. **Quick-add** — records are created inline at the bottom of a list/group ("+ New …"), not via
   modal or drawer. Enter commits and keeps the input open for consecutive entries; Escape/blur
   cancels.
6. **Status representation** — solid-tint pill (background tint + matching text color, no
   border), not an outlined badge.
7. **Loading state** — skeleton rows matching the row's real layout, never a spinner+text.

Applies to: Tasks (shipped). Candidate for CRM Leads, Members, and other simple single-entity
lists when they're next revisited — this does not retroactively obligate rewriting those pages.

---

## 8. FRONTEND MODULE REGISTRY

### AUTH PAGES [✅ DONE]

`/login` · `/signup` — own layout, no sidebar. (Forgot/reset-password pages: backend routes exist; page files not present in the current tree — build when email sending lands, see Section 9.)

### ONBOARDING [✅ DONE]

`/create-organization` · `/select-organization`

### DASHBOARD SHELL [✅ DONE]

Sidebar, Topbar, org switcher, drawer system (`DrawerContext` + `ui/Drawer`), org context in URL.
Topbar bell opens `components/notifications/NotificationDrawer` (added 2026-08-03 — see
PLATFORM — NOTIFICATIONS below); org-home page also renders the metrics widget via `lib/dashboard.ts`
against the backend DASHBOARD route (Section 5).

### TASKS [✅ DONE — redesigned to the Collection View Pattern]

`/[orgId]/tasks` — Grouped (collapsible, by status) and List (flat, filter tabs) view modes,
inline quick-add, full inline editing (title, status, due date, complete-toggle); drawer
reserved for secondary-field edits, permission-gated actions.

### SETTINGS — MEMBERS [✅ DONE]

`/[orgId]/settings/members` — members list, invite, role assignment, per-member action menu (reset password, manage/view permissions, remove), invitation resend/revoke. Menu hidden for own row and owner's row, matching backend guards.

### SETTINGS — ROLES & PERMISSIONS [✅ DONE]

`/[orgId]/settings/roles` — role list with System/Custom badges, create (via `PermissionForm` create mode), edit, clone, delete. Shared `lib/permissionGroups.ts` categorization (includes all HRM resources). ⚠️ `capture.*` permission group not yet added to `permissionGroups.ts` — do this with the capture frontend, or the six capture permissions will be invisible in the picker (same class of bug as the r10 HRM fix).

### PROFILE & SECURITY [✅ DONE]

`/[orgId]/settings/profile` (avatar crop modal included) · `/[orgId]/security/sessions`

### CRM — LEADS [✅ DONE]

`/[orgId]/crm/leads` — list, filters, `LeadForm`, `ConvertForm` (contact + deal creation).

### CRM — CONTACTS & COMPANIES [✅ DONE]

`/[orgId]/contacts` · `/[orgId]/companies` · `/[orgId]/companies/:companyId` — note the frontend URLs are NOT nested under `/crm/`; the backend API routes are (`/organizations/:orgId/crm/...`). Known and fine.

### CRM — PIPELINE & DEALS [✅ DONE]

`/[orgId]/crm/pipeline` — kanban, drag-to-move, `DealForm`, won/lost.

### CRM — REPORTS [✅ DONE]

`/[orgId]/crm/reports` — overview dashboard per the Dashboard Page Pattern.

### CRM — AGENDA [✅ DONE — post-r10, folded into doc r11]

`/[orgId]/crm/agenda` — "today view" over `GET /crm/reports/agenda` (TanStack Query, permission-gated).

### CRM — SETUP [✅ DONE — post-r10, folded into doc r11]

`/[orgId]/crm/setup/routing` — lead round-robin settings (crm_settings)
`/[orgId]/crm/setup/templates` — template CRUD (`TemplateForm`)

### PLATFORM — NOTIFICATIONS [🔵 ACTIVE — built 2026-08-03]

`components/notifications/NotificationDrawer.tsx`, opened from the Topbar bell (unread-count
badge, mark-read on click, mark-all-read action) via `lib/notifications.ts` +
`types/notification.ts`. Backend: Section 5 → PLATFORM — NOTIFICATIONS. No settings/preferences
page yet — the backend `GET/PATCH /notifications/preferences` routes exist but nothing in the UI
calls them.

### CRM — CAPTURE [🔵 PARTIAL — most of the frontend exists, previously documented as "zero"]

This entry was wrong: three of the four planned pieces are built, just under different paths than
originally planned (`settings/` instead of `crm/setup/capture`):

- `/[orgId]/settings/apikeys` (`lib/apikeys.ts`) — API key list, create drawer with show-once raw key, revoke.
- `/[orgId]/settings/integrations` (`lib/integrations.ts`) — inbound email address + social integration management (list/create/delete).
- `/[orgId]/crm/visitors` (`lib/visitors.ts`) — visitor list + JS embed snippet generator.
- `app/api/lead-capture/route.ts` — a live Next.js route handler proxying to the backend's public `/pub/leads` endpoint.

**Still genuinely missing:** the `capture.*` permission group in `lib/permissionGroups.ts` — the
six capture permissions (`capture.apikeys.*`, `capture.email.manage`, `capture.social.manage`,
`capture.visitors.view`) are invisible in the Roles picker even though the pages that need them
exist and work. This is the one item worth prioritizing here — it's the same class of bug as the
r10 HRM permission-picker fix. No dedicated `types/capture.ts` — types are inlined in each lib file.

### HRM [✅ DONE — statuses setup added post-r10]

All under `/[orgId]/hrm/...`: departments, positions, employees, leave, attendance, payroll, lifecycle, warnings, compliance, recognition, reports, and setup/{approvals, document-templates, holidays, salary, shifts, warning-types, **statuses**}. `ApprovalInstanceView` drawer wired into Warnings, Recognition, and Lifecycle pages.

---

## 9. UNBUILT MODULE REGISTRY

`⚪ NOT STARTED` = no code exists. When an item starts, promote it to `🔵 ACTIVE` here and give it a real entry in Section 5 and/or Section 8 with routes and permissions. When it ships, mark `✅ DONE` or remove the entry. Nothing here carries priority ordering — pick up whatever's needed, including because it connects to what you're currently building (Section 1).

PLATFORM PRIMITIVES and PREP MIGRATIONS sit first because most entries below reference them, not because they rank first. Position in this list still carries no priority.

---

### PLATFORM PRIMITIVES [✅ DONE — all seven built, see Section 5; #6 (tickets) r31, #7 (knowledge base) r32]

Seven pieces of shared infrastructure that multiple modules need, all now built. The last two were
not in the original five: `internal/platform/tickets` (r31) exists because the helpdesk fork
resolved toward platform rather than `internal/hrm/helpdesk`, so one engine serves both HR helpdesk
and the CRM list's customer-facing ticketing; `internal/platform/kb` (r32) is its companion, since
a knowledge base is what deflects the tickets nobody needed to raise. See Section 5 →
PLATFORM — TICKETS and PLATFORM — KNOWLEDGE BASE.

They live in `internal/platform/` for the same reason contacts and engagement do: building them per-module means schema duplication across CRM, HRM, and everything after.

Two of these (notification, scheduler) were previously named only inside the CRM Advanced Functionality Pass entry below, as a parenthetical dependency. That mention is now superseded by this entry — one fact, one owner. The CRM entry keeps the dependency note but not the description.

**1. Notification system — ✅ built, see Section 5 → PLATFORM — NOTIFICATIONS**
Delivery-path abstraction: in-app, email (Resend, real), (later) push. Per-user preference matrix, per-event-type opt-out, read state all built; digest batching not built. Consumer today: `auth` (password reset/invite) only — no CRM or HRM consumer wired yet. Consumers if HRM extends: every single module in the HRM Extended Modules entry below — onboarding reminders, appraisal cycle deadlines, certification expiry, benefit enrollment windows, helpdesk SLA breach, exit clearance nudges.
Hard dependency (EMAIL SENDING) resolved — Resend is wired and live, not stubbed.

**2. Scheduler registry — ✅ built, see Section 5 → PLATFORM — SCHEDULER**
Named recurring jobs with Redis distributed locking (multi-instance safe), run history, manual trigger built. Failure alerting (beyond `consecutive_failures` tracking) not built. The two ad-hoc HRM crons (milestones, absences) are migrated onto this — and both had latent bugs (one a stub, one silently erroring on a dropped column) fixed as part of the migration; see Section 5 entry for detail.
Consumers still to come: analytics snapshot runs, certification expiry sweeps, enrollment window open/close, ticket auto-close, system access revocation on last working date, cycle phase transitions.

**3. Resource-level permissions — ✅ built (r18), see Section 5 → HRM MODULE**
Beyond module/action RBAC — per-record filtering, and a `view_own / view_team / view_all` tier applied consistently. `view_team` resolves against a reporting-manager chain (a depth-parameterized recursive CTE over `hrm_employees.manager_id`, default depth 1 — see PREP MIGRATIONS). Shipped as `authz.Scope`/`ResolveScope` (`internal/authz`) + a new `internal/hrm/scope` package (`Predicate()` for list queries, `Resolver.AuthorizeRecordAccess()` for GET-by-ID), rolled out across all 12 employee-record HRM modules plus salary's per-employee endpoints.
This is the primitive with the highest severity of failure: appraisal draft leakage, salary visibility, anonymous 360 de-anonymization, and succession/flight-risk exposure are trust and legal issues, not UX polish. Every future module that returns employee-level records (performance, compensation depth, succession, etc.) should build on this from day one rather than retrofitting later — retrofitting is exactly what r18 just did for the 12 already-shipped modules, and it touched every one of their repository query layers.

**4. Checklist engine — ✅ built (r19), see Section 5 → PLATFORM — CHECKLISTS**
Template + typed items + `owner_type` → assignee resolution at instantiation + offset-based due dates + blocking-flag (reported, not yet enforced — nothing to gate until an offboarding consumer exists) + instance completion tracking, computed not stored.
One engine with a `checklist_type` discriminator, not one per consumer — all four values (`onboarding`/`offboarding`/`probation_confirmation`/`transfer_handover`) seeded now, only `onboarding` has a consumer.
First real consumer: HRM onboarding (`internal/hrm/onboarding/`, Section 5 → HRM MODULE → Group B). Still-unbuilt consumers: exit clearance (F&F, consumer #2 — needs `blocking_amount` added as a pure ALTER), probation confirmation, transfer handover.

**5. Form / question engine — ✅ built (r23), see Section 5 → PLATFORM — FORMS**
`internal/platform/forms/` (17 routes): templates → sections → typed questions → typed responses → scoring → aggregate. Definition snapshotted onto each instance as **real columns, never JSONB** — migration `00076`'s rule is that JSONB is for opaque config read as a whole, and form responses get aggregated, so they are rows. A response row per question is created at instantiation, so answering is an UPDATE rather than an insert-or-update.
Follows the `internal/platform/checklists/` template exactly, including both of its structural rules: **polymorphic subject with no FK** (`subject_type` CHECK + `subject_id`) so `platform/forms` never references `hrm_*`, and **no generic instantiate route** — a generic endpoint would have to trust a client-supplied subject id and respondent id, which is an impersonation vector, and a form response is attributable evidence about a person. Consumers instantiate from their own endpoints having resolved the subject from their own domain.
`SubjectContext` keeps `SubjectID` (who it is about) and `RespondentUserID` (who fills it in) as separate fields — the split appraisals and 360 both depend on, and the one that files every response under the wrong person if collapsed.
Real consumers today: appraisal self/manager forms (r23, consumer #1), 360 feedback (r23, consumer #2) and LMS quizzes (r24, consumer #3). Still unbuilt: exit interviews, potential-criteria assessment, employee surveys.

⚠ **The engine does NOT know what a correct answer is, and deliberately still does not after r24.** Phase 6's quizzes needed one; rather than adding `correct_answer` + `pass_mark` here — which appraisals and 360 would then carry and never read — the answer key lives in `hrm_quiz_answer_keys`, owned by `internal/hrm/learning`. `computeScore` remains a weighted RATING (0-1 normalised per question scale), which is the right thing for an appraisal and the wrong thing for a quiz; Phase 6 has its own `Grade()`. If a fourth consumer needs marking, it should own its key too.

**⚠️ `hrm_interview_scorecards` was NOT migrated onto this engine, and that is a decision, not an omission.** r21 shipped it as a deliberately fixed-shape table and described it as the engine's "consumer #1"; that framing is **superseded**. When the engine's real shape became concrete in 5B, the migration was reconsidered and declined: scorecards carry a bespoke reveal rule (a panelist who has not submitted their own sees only their own draft; everyone else sees every *submitted* scorecard, never a draft) that is a service-level rule the generic engine has no concept of, and the fixed shape is what makes that rule cheap to express. Revisit only if a second interview-form shape is genuinely needed.

---

### PREP MIGRATIONS [✅ DONE — 2026-08-03]

Schema hooks that cost one migration today and a system-wide rewrite if deferred:

- ~~`hrm_employees.reporting_manager_id` — verify whether this exists at all~~ — resolved: it
  exists as `hrm_employees.manager_id` (self-FK, `idx_hrm_emp_manager_id`, migration `00021`).
  There was never a missing column here — the open question was only ever "does the doc's assumed
  name match the source," and it didn't; the underlying capability was already there. Appraisal
  routing, expense approval, clearance ownership, `view_team` scoping, and org chart all resolve
  against `manager_id`, not a new column.
- `legal_entity_id` — done, migration `00070`. Minimal `hrm_legal_entities` table (zero business
  logic, per the original scope), one default entity per org, nullable FK backfilled onto every
  HRM table that carries a direct `org_id` (36 tables).
- `currency` — done, migration `00071`. `CHAR(3)` added alongside money columns that didn't have
  one (`hrm_employee_salary_records`, `hrm_promotions`, `hrm_salary_components`,
  `hrm_salary_structure_components`, `hrm_payslip_lines`), backfilled from each row's org
  currency; existing ad-hoc `TEXT` currency columns (`organizations`, `hrm_awards`,
  `hrm_terminations.severance_currency`, `hrm_payslips`, `hrm_payslip_runs`) standardized to the
  same `CHAR(3)`.

---

### CRM — ADVANCED FUNCTIONALITY PASS [🔵 ACTIVE — capture in flight]

Mridha's wish list, triaged by buildability. **In flight now:** lead auto-capture (Section 5 → CAPTURE: fix pass + frontend). **Shipped from this list:** lead routing round-robin, templates & snippets, agenda/smart-task view.

**Contained — no new infrastructure or one well-understood integration:**

- Activity metrics reporting (calls/meetings/deals-closed per rep)
- Sales forecasting (weighted pipeline by historical win rate per stage)
- Data enrichment on company domain (Clearbit or Apollo — real per-lookup cost)
- Trigger-based actions — small hardcoded set first ("on Closed Won, do X"), not a rule engine

**Needs new infrastructure first, or a real ongoing vendor cost:**

- Reminders / SLA alerts — blocked on notification + scheduler; see PLATFORM PRIMITIVES above
- Real Facebook/LinkedIn lead-ad integration (Graph API fetch, OAuth connect, field-mapping UI) — upgrades the current webhook skeleton
- Visitor IP→company enrichment (IPinfo/Clearbit + Redis queue + worker) — upgrades current manual identify
- Meeting scheduling via Calendly — mostly a webhook integration
- Sales velocity (time-in-stage) — needs a stage-transition history table first (`crm_deals` only stores current `stage_id`)
- SMS via Twilio — contained but new vendor + per-message cost

**Major, multi-week-plus:**

- Two-way email sync (Gmail/Outlook) — reconcile with Email Sending below, don't build twice
- Telephony / call recording / transcription
- Automated sequences/cadences — mini workflow engine; depends on email sync for reply detection
- CPQ + e-signature
- Invoicing/accounting integration — decide external-integration vs future in-house Accounting module
- Ticketing/Helpdesk — its own product, wasn't on the original module list

**Likely not worth pursuing:** LinkedIn message sync (no official API; scraping is fragile and ToS-risky).

---

### MFA / 2FA [⚪ NOT STARTED]

TOTP enrollment + verification, backup codes, per-org enforcement policy. Lives in `internal/auth/`; needs a secrets table and a frontend enroll/verify flow.

---

### SOCIAL LOGIN / SSO [⚪ NOT STARTED]

Google / Microsoft / GitHub OAuth flows. `POST /api/v1/auth/oauth/sync` already exists as the backend hook; this item is the provider-specific flow wiring.

---

### EMAIL SENDING [⚪ NOT STARTED]

Transactional provider (SES / Postmark / Resend) for verification, invites, password reset — currently token-only. Also unblocks the forgot/reset-password frontend pages. Distinct from capture's inbound email parsing, but if Postmark is chosen, one account can serve both inbound and outbound — decide together.

---

### RESOURCE-LEVEL PERMISSIONS [⚪ NOT STARTED]

Per-record access control ("only deals they own") beyond module/action RBAC. Touches every repository's query layer — needs its own ADR when it starts. Full rationale and consumer list: PLATFORM PRIMITIVES #3 above.

---

### BILLING & SUBSCRIPTION MANAGEMENT [⚪ NOT STARTED]

Payment provider, plan tiers, usage limits, invoices, billing UI. `organization_usage` and `organizations.max_seats` already exist as head starts.

---

### HRM FUNCTIONALITY PASS [⚪ NOT STARTED]

Post-completion polish pass over the shipped HRM module, same spirit as the CRM pass. Includes the known open item: approval-instance list endpoint.

---

### HRM EXTENDED MODULES [⚪ NOT STARTED — scoping only, no build decision implied]

The shipped HRM module (Section 5) covers setup, lifecycle, disciplinary, time & compensation, recognition, and leave. That is a complete core. This entry records the extension surface an enterprise-grade HRM would additionally cover — scoped in a five-part design discussion, not committed to. Nothing here is a plan; it is a map, so that a future decision to build any part of it starts from a design rather than a blank page.

Everything below depends on PLATFORM PRIMITIVES above. Those dependencies are stated once per module and not restated — see that entry for what each primitive is.

**Talent acquisition & entry**

- **Recruitment / ATS — 🔵 PARTIAL, Phase 4A ✅ built (r20), Phase 4B ✅ built (r21), see Section 5 → HRM MODULE → Recruitment / ATS.** Phase 4A shipped requisitions (approval-gated), postings, candidates, applications, configurable pipeline stages, and `hrm_application_stage_history` from the first migration — the two hard structural rules (candidate ≠ application, stage lives on the application; stage history from day one, since `crm_deals` skipping this is exactly why sales velocity is blocked above) were both honoured. Phase 4B shipped interviews + panelists, fixed-shape interview scorecards (deliberately NOT a mini form engine — Phase 5's form engine names interview scorecards as consumer #1, and building a bespoke one now would have pre-empted that primitive), offers (approval-gated, `'offer'` added to the same two approval CHECK constraints `'job_requisition'` uses), referrals, and hire→employee conversion. The "interviewer cannot see others' scores before submitting" rule was confirmed **not expressible with the Phase 1 scope tiers** (`internal/hrm/scope`'s `Predicate` hard-codes `FROM hrm_employees` and every tier is state-independent) and shipped as a purpose-built service-level rule instead — see Section 5 for the exact reveal semantics. Only the public surface remains outstanding — see below.
  **The public careers page (`GET/POST /pub/careers/*`) and candidate email remain unbuilt**, gated on the same two prerequisites the original build plan named — re-confirmed NOT actually done as of r21 despite this doc elsewhere claiming EMAIL SENDING resolved: resume parsing stays a vendor bolt-on, not in scope, whenever the public phase lands.
  Depends on: EMAIL SENDING (not optional — an ATS without candidate email is half a product; still not resolved despite doc drift claiming otherwise, see r20/r21 changelogs), scheduler (candidate data purge / GDPR — `hrm_candidates.purge_after` is written now, read by nothing yet; referral bonus payout also has no scheduler job, `bonus_pending`/`bonus_paid` are set manually via `PATCH .../referrals/:id`).

- **Onboarding — ✅ built (r19), see Section 5 → HRM MODULE → Group B and → PLATFORM — CHECKLISTS.** Checklist-driven, multi-owner, offset-based due dates relative to hire date. Not a table of its own — checklist engine consumer #1. Auto-instantiates the org's default onboarding template on employee creation (non-blocking — never fails employee creation), plus a manual retry endpoint. Reminders are explicitly out of scope for this build (no scheduler job, no notification dispatch) — the "Depends on: notification" line below is therefore not yet exercised.
  Depends on: checklist engine (built), notification (not yet wired — reminders deferred).

**Growth & development**

- **Performance Management — ✅ DONE (r23), see Section 5 → HRM MODULE → Performance, Appraisals, 360 Feedback, PIP.** All three sub-phases shipped. **5A (r22)** goals/OKR: goal cycles, alignment via `parent_goal_id` (`ON DELETE SET NULL`, and alignment-only — no progress roll-up, because roll-up would let a back-dated subordinate check-in mutate an already-published appraisal), append-only check-ins, concurrency-safe weight guard. **5B (r23)** the form engine (`internal/platform/forms`, now Platform Primitive #5) plus appraisal cycles: configurable rating scales, 6-phase transition map, `manager_employee_id_snapshot` frozen at instantiation, publish-immutable via snapshotted scores, and `final_rating` as a structured queryable FK **plus** a label/value snapshot — Phase 7's merit matrix and Phase 10's 9-box query the FK, the snapshot survives a level being renamed or deleted. **5C (r23)** 360 feedback with anonymity enforced structurally at the repository layer (two read paths, two types sharing no field; policy derived from `Relationship.IsAnonymous()` rather than stored; per-relationship-group suppression threshold that applies to `view_all` too) and PIP handing a `failed` outcome to `hrm_terminations` as a **draft only**, via the consumer-owned `pip.TerminationCreator` seam.

  One scope item from the original plan was **not** built and is deliberately deferred: **continuous (non-cycle-bound) 360 feedback**. 5C shipped the formal cycle-bound half only. Continuous feedback has no cycle to hang a suppression threshold on, which is the entire anonymity mechanism here — it needs its own design (rolling windows, or per-subject rather than per-cycle thresholds), not a nullable `cycle_id`.
  Depends on: form engine, resource-level permissions (draft leakage is the failure mode), notification, scheduler, `hrm_employees.manager_id` (exists — see PREP MIGRATIONS).

- **Learning & Development — ✅ DONE (r24), see Section 5 → HRM MODULE → Learning & Development and Certifications + Skills.** Both slices shipped. **6A** the LMS core: courses with content hanging off versions, enrollment pinning `version_id` (RESTRICT), modules/lessons, lesson progress, and quiz attempts graded against a Phase 6-owned answer key. **6B** certifications with the nightly expiry sweep, plus `hrm_skills`/`hrm_employee_skills` as a shared org taxonomy. No SCORM player, no video hosting — external links, PDF/text content, mark-complete, quiz, as scoped.

  Three deviations from the original scope, each deliberate and recorded:

  1. **The form engine could not do assessments as the build plan assumed** — it has no correct-answer column and no pass mark, only a weighted rating. `hrm_quiz_answer_keys` is owned by Phase 6 instead, which keeps `platform/forms` free of assessment semantics AND makes the "never leak the answer" rule structural. See the r24 changelog entry.
  2. **`hrm_position_skills` was NOT built.** Skills a POSITION requires has no reader until Phase 10's succession and gap analysis; recruitment and performance were both grepped and contain zero skills fields, so there is nothing to retrofit into. Building it now would be the speculative primitive rule 1 exists to prevent. `hrm_skills` and `hrm_employee_skills` DO ship, justified by a real in-phase consumer — issuing a certification that carries a skill records it. The build plan's "shared taxonomy, not an LMS-internal table" note is honoured at the PACKAGE level too: `internal/hrm/skills` is standalone, and Phase 10 imports it directly.
  3. **Instructor-led sessions and training requests + budgets are NOT built** and were never in the build plan's Phase 6 line item — they appear only in this scoping paragraph. They need a scheduling surface and a budget model respectively; neither has a consumer today. Revisit alongside Phase 7 compensation, which is where a training budget would actually be spent.
  Depends on: scheduler, form engine, reuses `hrm_acknowledgements` for compliance evidence.

**Compensation & benefits**

- **Payroll engine additions — ✅ DONE (r25, Phase 7A), see Section 5 → HRM MODULE → Time &
  Compensation.** `run_type` on payslip runs (regular/off_cycle/bonus/arrears/fnf), `line_type` +
  `is_employer_contribution` + `source_period_id` on lines, deterministic three-stage calculation
  order, a real negative-net guard (the prior code silently zeroed it), and the mandatory dry-run
  preview (`POST .../runs/:runId/preview`). Four money defects found and fixed in the pre-existing
  engine along the way — see the r25 changelog entry.

- **Salary revision cycles + bonus engine — ✅ DONE (r26, Phase 7B), see Section 5 → HRM MODULE →
  Compensation.** `internal/hrm/compensation` (20 routes), 5 tables. Effective-dated compensation
  bands and merit-matrix cells (real rows, not JSONB); batch-approved cycles — one approval
  instance per cycle, `HandleApprovalDecision` and `ApplyCycle` kept as distinct steps, the
  `promotions.Apply` precedent; mandatory `calculation_snapshot` JSONB on every revision and every
  bonus, built by a shared `CompensationContext`; bonuses paid out through a
  `run_type='bonus'` payroll run via `payslips.BonusSource`, a consumer-owned narrow interface —
  payslips declares it, `hrm/compensation` imports payslips (not the reverse) to satisfy it.
  **Not** built: an `expr-lang/expr` formula language for bonuses — the shared context feeds a
  small closed calc-method enum (fixed / pct-of-basic) instead of a full DSL, since payslips
  already owns formula evaluation for salary components and duplicating it here for two calc
  methods would be the speculative-primitive rule 1 forbids.

- **Loans + reimbursement payout — ✅ DONE (r27, Phase 7C), see Section 5 → HRM MODULE → Loans +
  Reimbursements.** `internal/hrm/loans` (amortization generated once at disbursement, a
  zero-net-pay-capped recovery stage feeding every non-bonus run type, an append-only recovery
  ledger, foreclosure) and `internal/hrm/reimbursements` (payout only, no claims workflow). Both
  follow 7B's bonus payout template exactly: a payslips-side narrow consumer interface
  (`LoanSource`/`ReimbursementSource`), not a payslips-side rewrite. **Not** built:
  auto-foreclosure on resignation — a resigned employee's remaining installments simply stay
  `pending` once they leave the eligible-employee set, a deliberate, narrower scope than full F&F
  settlement (see the r27 changelog entry for why building that now would be out of proportion).

- **Statutory compliance — ✅ DONE (r28, Phase 7D), see Section 5 → HRM MODULE → Statutory +
  Benefits.** `internal/hrm/statutory` — a country-pluggable `Provider` interface + `Registry`,
  shipped with ONE real, data-driven `SlabProvider` (effective-dated `hrm_statutory_slabs`,
  evaluated via `payslips.ComputeSlab`). Per-country Go providers for proration/eligibility rules a
  slab table cannot express register in the `Registry` per rule's `country_code` — zero schema
  change needed. **Not built**: employee-level country filtering (every active rule applies
  org-wide) — no `hrm_employees.country` field exists yet, that is explicitly Phase 11
  (multi-country/multi-currency) scope.

- **Benefits Administration — ✅ DONE (r28, Phase 7D).** `internal/hrm/benefits` — plans, mutable
  catalog tiers, enrollments (cost frozen at signup via `employee_cost_snapshot`/
  `employer_cost_snapshot`), dependents with manual verification (no document workflow, per the
  build plan), enrollment → payroll deduction line through `payslips.BenefitsSource`. Enrollment
  windows are the three named TYPES (`open`/`new_hire`/`qualifying_event`) on the enrollment row
  itself, not a separate window-definition table — none was asked for. The scheduler consumer
  (`benefits.activate_pending_enrollments`, daily) flips a `pending` enrollment to `active` once
  its `effective_date` arrives. Claims tracking remains explicitly out of scope; it lives with the
  insurance provider.

**Operations**

- **Asset Management — ✅ DONE (r29, Phase 8A), see Section 5 → HRM MODULE → Assets.** Categories with `requires_return`, asset instances, assignment history (current holder derived, never stored — proved by `information_schema` introspection), maintenance log, approval-gated requests, and software licence seats as the separate shape the build plan called for. Depreciation is a book-value stub, computed per read. Handover sign-off reuses `hrm_acknowledgements` via a new `'asset_handover'` type. Feeds exit clearance (Phase 9) and payroll recovery — `requires_return` is the flag those will read.

- **Travel & Expense — ✅ DONE (r30, Phase 8B), see Section 5 → HRM MODULE → Travel & Expense.** Travel requests with itineraries, advances with all three settlement outcomes, and expense claims with genuine **line-level** approval (`amount` vs `approved_amount` per line; the claim stores no total at all). Policy violations recorded as warnings, never blocks. Per-diem, mileage and policy caps effective-dated. Multi-currency handled by snapshotting the rate onto each line rather than introducing an FX rate table — real conversion infrastructure remains Phase 11. OCR is a nullable `ocr_raw` column with manual entry, as scoped; no vendor integration. The boundary holds: claim lifecycle here, payout via 7C's reimbursement seam into payroll.

- **HR Helpdesk — ✅ DONE (r31 core, r32 knowledge base + email-to-ticket).** Employee-facing tickets with a pausable SLA clock, internal-only comments, and sensitive categories with a restricted assignee pool shipped as **Platform Primitive #6**, `internal/platform/tickets/`; the knowledge base is **Platform Primitive #7**, `internal/platform/kb/`; email-to-ticket routes through `org_inbound_emails.destination`. See Section 5 → PLATFORM — TICKETS, PLATFORM — KNOWLEDGE BASE and CAPTURE — INBOUND EMAIL.
  ✅ **The architectural fork is decided: platform, not HRM.** The same data shape serves the customer-facing Ticketing/Helpdesk noted in the CRM entry above; `requester_type` is FK-free and CHECK-narrowed to `'employee'`, so widening to `'contact'` is a CHECK change rather than a rewrite. The `hrm_complaints` boundary holds: `MarkConverted` is a ONE-WAY path called from the HRM side (`hrm → platform` is the allowed direction), and a second conversion is refused — a formal complaint carries legal weight and must never degrade back into a ticket. The HRM-side caller that creates the complaint is **not built yet**; `MarkConverted` exists and is tested, with no consumer.
  ⏳ Still open: an HRM-side caller for the ticket→complaint conversion. `MarkConverted` is built and tested; `hrm/complaints` must create the complaint and call back.

- **Exit Management** — upgrade over the existing resignations/terminations decision records: an umbrella exit record, notice period tracking, exit interviews (confidential, aggregate-only, often sent post-departure), clearance checklists, F&F settlement, document issuance, rehire eligibility. Access revocation on last working date is scheduler-driven, not manual.
  F&F is an off-cycle payroll run (`run_type = 'fnf'`), not a separate calculator — same line types, same statutory engine, same immutability. It pulls from seven modules; negative net is a valid outcome and must be handled. Clearance blocking items gate finalization.
  Depends on: checklist engine (consumer #2), form engine, scheduler.

**Insight**

- **Org Chart & reporting structure** — effective-dated reporting relationships in their own table (not a column on employees), supporting matrix reporting via `relationship_type`. Cycle detection required. Recursive CTE first; optimize only on proven need. Position/seat modeling left nullable so vacant-seat → requisition can be retrofitted.
  Frontend is a genuine visualization (d3-hierarchy / react-flow) with lazy expand — the Collection View Pattern does not apply.

- **Succession / Talent Management** — 9-box (potential assessed separately from performance, never derived from it), critical positions, succession plans with readiness levels, signal-based flight risk (explainable indicators, not an ML score), individual development plans.
  ⚠️ Confidentiality here exceeds salary: 9-box position, flight risk, and successor nomination are never visible to the subject, while their development plan is — field-level filtering within a single record, not module-level RBAC.

- **People Analytics** — not a module, a consumer. Its ceiling is set entirely by whether the twelve above store structured, temporal, enum-typed data. Nightly snapshot + fact tables in the same Postgres, never live aggregation over OLTP. `hrm_metric_definitions` is the entry that looks skippable and is not: two dashboards disagreeing on "attrition rate" ends trust in the whole module permanently. Compensation analytics, DEI (aggregate-only, threshold-gated, country-configurable), and export are separately permissioned. Predictive attrition scoring is deliberately excluded.
  Depends on: scheduler (this is where it becomes non-negotiable), EMAIL SENDING (scheduled report delivery is what makes analytics actually get read).

**Cross-cutting**

- **Multi-country / Multi-currency** — not a module: a legal-entity layer between organization and employee, which re-scopes payroll runs, statutory resolution, approval chains, and every analytics view. Plus country-configurable working week, leave minimums, notice/termination law, name format, address schema, and ID types. Timezone-correct attendance attribution is a policy decision, not a storage one.
  Data residency (separate DB instance or region per jurisdiction) is explicitly out of scope and conflicts with the current single-Postgres deployment model — that, not schema, is where "multi-country SaaS" actually hits a wall.
  See PREP MIGRATIONS above: `legal_entity_id` and per-column currency are cheap today.

**Answers the original question this scoping started from:** the shipped HRM is entirely internal-facing (authenticated JWT entry only), which is why it needs no CRM-Capture equivalent. Recruitment/ATS is the one entry above that changes that — a public career page and application endpoint (`/pub/careers/*`) is the same shape as CRM capture: public routes, rate limiting, file validation, `LOWER(email)` dedup, and job-board webhooks if those are ever added. Whatever Capture Fix Pass A/B teaches transfers directly. Nothing else in HRM has an anonymous entry point. Attendance hardware push and payroll bank disbursement are integrations, but neither is inbound-webhook shaped.

**Vendor boundaries decided during scoping (consistent across all of the above):** resume parsing, receipt OCR, SCORM/video hosting, external certification verification, statutory filing submission, and predictive scoring are all either vendor bolt-ons or out of scope. The pattern: store the raw artifact, leave a nullable column for extracted data, add the vendor later. Do not build the extraction engine.

**If any part of this is picked up:** promote it here (⚪ → 🔵), add a real Section 5 entry with routes and permissions, and a Section 8 entry when frontend starts. Do not let this entry grow into the module documentation — it is a map, and a map that tries to be the territory goes stale first.

---

### PROJECT MANAGEMENT MODULE [⚪ NOT STARTED]

Full projects module (milestones, dependencies) — broader than the generic `task` module, which stays a simple RBAC-testing CRUD.

---

### E-COMMERCE ADMIN MODULE [⚪ NOT STARTED]

Per the original long-term vision.

---

### ACCOUNTING MODULE [⚪ NOT STARTED]

Per the original long-term vision.

---

### ERP MODULE [⚪ NOT STARTED — scope undefined]

Umbrella term; may end up being HRM + Accounting + Projects + Inventory combined rather than a standalone build. Real scoping needs those three to exist first, since that's what "ERP" would be built out of.

---

### FULL PRODUCTION DEPLOYMENT [⚪ NOT STARTED]

Confirmed not started (r9 audit; unchanged): no Sentry SDK, no Caddyfile, `deploy.yml` is a stub. Scope when picked up: `docker-compose.prod.yml`, TLS (Caddy), production env/secrets, Sentry, backups, VPS + DNS. **Hard prerequisite:** Capture Fix Pass B must be complete before any deployment — two webhook endpoints are currently unauthenticated.

---

### MOBILE APP [🔵 ACTIVE — real code exists; doc previously said "zero code written," which was false as of 2026-08-03]

**This entry was wrong.** `mobile/` was committed 2026-07-23 (`a350092`, "Mobile added," 93 files,
+5799/-144) — one day after r14 (2026-07-22) dated the "zero code" claim, so the commit simply
never got folded back into the doc. Verified directly against source, not against the r10/r14
plan. See Section 14 for the architecture-vs-actual diff and Section 15 for per-screen status.

**What's real:** full Expo Router app under `mobile/src/`, real dependencies (`expo ~57.0.8`,
`react-native 0.86.0`, `expo-router ~57.0.8`, `expo-secure-store ~57.0.1`, `zustand ^5.0.14`,
TanStack Query, axios, react-hook-form/zod), `node_modules` installed. Auth screens (login,
signup, forgot-password, reset-password), onboarding (create-organization, select-organization),
dashboard shell, tasks, a single-screen tabbed CRM view, a flat settings screen, real Zustand
stores (`authStore`, `permissionStore`, `uiStore`), real `expo-secure-store` token handling, real
API client hitting the actual backend (`api.post('/auth/mobile/login', ...)`,
`api.get('/organizations/:orgId/crm/leads')`).

**What's stub or missing:** `components/{crm,tasks,rbac}/` are empty directories, tracked as
nothing in git. `useAuth.ts`'s `forgotPassword`/`resetPassword` are `console.log` + fake
`setTimeout` — the *pages* exist and the backend routes exist, but the mobile hook doesn't call
them yet. No `security/` route at all. CRM is one screen with an internal tab switcher (Leads /
Pipeline / Reports / Agenda / Setup), not split into per-entity routes the way web is. Settings is
a single flat screen — no members/roles subroutes.

**Structural deviations from the original plan** (not wrong, just different — update Section 14
to match rather than "fix" the code to match a plan written before this was built): code lives
under `mobile/src/...`, not bare `mobile/...`; there's an extra `apps/` wrapper route
(`(dashboard)/[orgId]/apps/{index,crm}`) and an `alerts/` route neither one was planned for.

Suggested next step, given the above: wire the two stub password-recovery calls in `useAuth.ts` to
the real backend routes (`AUTH — MOBILE` already supports `password-reset/*` — no backend work
needed), then decide whether to keep CRM as one tabbed screen or split it, before building
anything further on top of it.

---

## 10. CORE SECURITY STANDARDS

These apply to every line of code in this project. Never compromise on these.

**Backend:**

- Never store plaintext passwords — bcrypt always
- Never store raw refresh/reset/email tokens — hash first, store hash
- Never store raw API keys — SHA-256 hash + display prefix only; raw value returned exactly once at creation
- Never log passwords, tokens, API keys, secrets, or any sensitive data
- Parameterized SQL only — never string-concatenate SQL
- Validate all request bodies at handler layer
- Sanitize and normalize email (lowercase, trim)
- Return generic errors for login and auth operations — never reveal why login failed
- Log detailed errors server-side only
- Secure CORS (explicit origins, no wildcard in production)
- `HttpOnly: true` on refresh cookie — always, not configurable
- Transactions for multi-step write operations
- Audit log for: login attempts, logout, password reset, role changes, permission changes
- **Webhook endpoints must verify provider signatures (HMAC / X-Hub-Signature-256) before processing — an unverified webhook is an unauthenticated write endpoint** (Fix Pass B closes the current gap)
- Every public (`/pub/*`) endpoint gets Redis-backed rate limiting — per API key where one exists, per IP otherwise
- Public endpoints never leak internal state: no validation detail in error responses, no internal IDs beyond `public_id`
- Third-party access tokens (social integrations) are encrypted at rest once real OAuth lands — never serialized in JSON responses
- **Money is `NUMERIC(18,4)` in Postgres and `shopspring/decimal` in Go — never `float64`** (r15). Every money column carries a currency alongside it from day one. Rounding policy (level and mode) is an explicit org setting, not an implicit language default. Verify the existing salary formula engine's evaluation type before extending it — `expr` defaults to float.

**Frontend:**

- Access token in `lib/token.ts` module variable only — never Zustand, never localStorage, never sessionStorage
- Refresh token is httpOnly cookie — frontend never reads it, never stores it
- Raw API keys shown once, never stored client-side beyond the creation modal
- Never add `persist` middleware to `authStore` or `permissionStore`
- `withCredentials: true` on all API requests
- Never show stack traces or backend error internals in UI
- Permission checks are UX only — backend enforces on every request
- No sensitive data in URL query params

---

## 11. DEVELOPMENT WORKFLOW

### Before implementing any feature

1. State what the feature is and which backend routes it uses
2. Identify which frontend files to create or modify
3. Identify any backend changes needed (new endpoint, field added, etc.)
4. Then provide code — complete files, not snippets, unless snippet is explicitly asked for

### When writing code

- Always include file paths as comments at top of file
- Prefer complete files when practical
- Use idiomatic Go for backend (`context.Context` in service/repo, explicit error handling)
- Use idiomatic Next.js + TypeScript for frontend (no `any`, proper async/await)
- Keep API calls in `lib/api.ts` or module-specific API helpers — not inside components
- Keep business logic out of components — components should be dumb consumers
- **Review AI-assisted output before committing** — no stream-of-consciousness comments, no "task says" references, no unresolved "Wait, let me check…" left in source. If a comment asks a question, answer it or delete it. (Added r11 after the capture audit found exactly this, hiding a feature-breaking bug.)

### When reviewing code

1. Score out of 10
2. What is good
3. Problems found
4. Security risks
5. Refactor suggestions
6. Corrected code if needed
7. Final recommendation

### When debugging

1. Ask for or look at the exact error, relevant file, and context
2. Explain root cause
3. Provide the smallest safe fix first
4. Then suggest a better long-term fix if different

---

## 12. RESPONSE FORMAT

### For implementation tasks

1. Goal
2. Files to create or modify
3. Code (complete files with path comments)
4. Explanation of key decisions
5. Security notes (if relevant)
6. What to do next

### For architecture questions

1. Simple explanation
2. Recommended choice for this project
3. Alternatives considered
4. Trade-offs
5. Final decision

### For feature planning / brainstorming

1. Summarize what we're solving
2. Options or approaches
3. Recommended approach
4. What it unlocks next

---

## 13. VERSION AWARENESS

Before writing code that depends on any of these, check for version-specific API differences:

| Package / Framework | Version in use                                                 |
| ------------------- | -------------------------------------------------------------- |
| Go                  | 1.25                                                           |
| Fiber               | v3.2.0                                                         |
| pgx                 | v5.6.0                                                         |
| go-redis            | v9.19.0                                                        |
| golang-jwt/jwt      | v5.3.1                                                         |
| expr-lang/expr      | v1.17.8                                                        |
| shopspring/decimal  | v1.4.0 (added 00066, HRM money fields — Section 10)            |
| robfig/cron/v3      | v3.0.1 (added 00067, `platform/scheduler`)                     |
| resend/resend-go/v2 | v2.28.0 (added 00068, `platform/notifications` email channel)  |
| Next.js             | 16.2.9 (check if unsure)                                       |
| Tailwind CSS        | v4                                                             |
| React               | latest stable                                                  |
| Zustand             | v5.0.14                                                        |
| next-themes         | latest stable                                                  |
| React Hook Form     | v7                                                             |
| TanStack Query      | v5                                                             |
| Axios               | v1                                                             |
| Expo SDK            | 57.0.8 confirmed (`mobile/package.json`, real app exists — see Section 9 → MOBILE APP) |
| Expo Router         | ~57.0.8, bundled with Expo SDK 57                               |
| React Native        | 0.86.0 confirmed (`mobile/package.json`)                        |
| expo-secure-store   | ~57.0.1 confirmed (`mobile/package.json`)                       |

`create-expo-app@latest` without `--template default@sdk-57` currently defaults to SDK 54 during
the transition window — pass the flag explicitly.

Do not use Fiber v2 API patterns with Fiber v3.

Tailwind v4 syntax — always use these, never the v3 equivalents:

- CSS entry point is `@import "tailwindcss";` — never the three `@tailwind base; @tailwind components; @tailwind utilities;` directives
- Theme config lives in CSS via `@theme { --color-x: ...; }` (this is already how `globals.css` is set up, Section 7) — never add a `tailwind.config.js` expecting `theme.extend` to work by default; v4 doesn't require one
- Gradients are `bg-linear-to-r`, `bg-linear-45`, etc. — not `bg-gradient-to-r`
- Referencing a CSS variable in an arbitrary value uses parentheses — `bg-(--brand-color)`, not `bg-[--brand-color]`. Square brackets stay for literal arbitrary values like `bg-[#4f46e5]`.
- Default border color is `gray-200`, not `currentColor` — don't assume a bare `border` inherits text color like it did in v3
- Custom utility classes are defined with `@utility` directly in CSS, not a JS plugin's `addUtilities()` callback

---

## 14. MOBILE ARCHITECTURE

Restored from the r10 archive, r14 — decided, not redesigned. Expo + React Native client. Ports
the web frontend's architecture (Section 7) wherever the platform allows; diverges only where
native constraints force it — token storage is the main one.

**Corrected 2026-08-03 against the real `mobile/` tree** (see Section 9 → MOBILE APP) — the
structure below is what was actually built, not the original r10/r14 plan. Differences from that
plan: everything lives under a `src/` root (not bare `mobile/app/...`); there's an `apps/` wrapper
route and an `alerts/` route neither one was planned for; CRM is one screen with an internal tab
switcher, not split into per-entity routes; settings is flat, no members/roles subroutes; there is
no `security/` route at all.

### Folder Structure

```
mobile/
  src/
    app/                        ← Expo Router file-based routes
      (auth)/
        login.tsx               ← real
        signup.tsx               ← real
        forgot-password.tsx      ← real page; hook behind it is a stub (see below)
        reset-password.tsx       ← real page; hook behind it is a stub (see below)
        _layout.tsx
      (dashboard)/
        _layout.tsx
        [orgId]/
          _layout.tsx
          index.tsx              ← dashboard home, real
          tasks/index.tsx        ← real
          apps/                  ← not in the original plan
            index.tsx
            crm/index.tsx        ← single screen, internal tabs: Leads/Pipeline/Reports/Agenda/Setup
          alerts/index.tsx       ← not in the original plan
          settings/              ← flat, no members/ or roles/ subroutes
            _layout.tsx
            index.tsx
      create-organization.tsx    ← real
      select-organization.tsx    ← real
      index.tsx
      _layout.tsx                ← root layout: theme provider + auth gate
    components/
      ui/  layout/                ← real
      crm/  tasks/  rbac/          ← empty directories, nothing tracked in git
    lib/
      api.ts  auth.ts  secureToken.ts  constants.ts   ← all real
      crmApi.ts  tasksApi.ts  orgApi.ts  dashboardApi.ts   ← not in the original plan, all real
    stores/
      authStore.ts  permissionStore.ts  uiStore.ts    ← real, same shape discipline as web
    hooks/
      useAuth.ts                 ← real for login/signup/logout; forgotPassword/resetPassword are
                                    console.log + fake setTimeout, no real API call
      usePermission.ts  useOrganization.ts             ← real (doc previously said `useOrg.ts`)
    theme/
      tokens.ts  ThemeProvider.tsx                     ← real
    types/
      index.ts                    ← real, single file (doc previously implied per-domain files
                                     mirroring frontend/src/types/*; it's one file instead)
```

### Auth Flow (mobile)

1. Login → `POST /api/v1/auth/mobile/login` → `{ access_token, refresh_token, expires_in }` in
   the body, no cookie involved
2. Store `refresh_token` via `expo-secure-store`; keep `access_token` in an in-memory module
   variable (`lib/secureToken.ts`) — same separation principle as web, different storage mechanism
3. Set user + org in `authStore`, permissions in `permissionStore` — identical shape to web
4. Axios request interceptor attaches `Authorization: Bearer <token>`
5. On 401 → read the refresh token from SecureStore → `POST mobile/refresh` → store the new
   (possibly rotated) tokens → retry the original request
6. Logout → read refresh token from SecureStore → `POST mobile/logout` with `{ refresh_token }` →
   clear SecureStore + in-memory token → reset all stores
7. Cold start → read refresh token from SecureStore → if present, call `mobile/refresh` before
   rendering any protected route; if it fails or is absent, route into `(auth)`

### Navigation & Route Protection

- Expo Router — file-based, same mental model as the Next.js App Router already in use
- `(auth)` / `(dashboard)` groups mirror the web's layout groups; `[orgId]` mirrors the web's
  URL-based org context
- Gate `(dashboard)` with Expo Router's Protected Routes, keyed off
  `authStore.status === 'authenticated'`

### State Management

Same three Zustand stores as web, ported with identical interfaces:

- `authStore`, `permissionStore` — never add `persist` middleware, ever
- `uiStore` may persist (theme, nav state) via `@react-native-async-storage/async-storage`, since
  `localStorage` doesn't exist in React Native

### Theming

- No CSS variables on native — port `globals.css` values into a plain `theme/tokens.ts` object
  with `dark`/`light` variants
- RN's `useColorScheme()` supplies the OS-level default; `uiStore.theme` overrides it once picked
  — same behavior as `next-themes`, different mechanism
- Load Inter via `expo-font` (`useFonts`) — one typeface family, matching web
- Plain `StyleSheet.create` for a first pass rather than a Tailwind-for-RN library — revisit only
  if styling velocity becomes a real problem

### API Client Contract

```ts
// lib/secureToken.ts
import * as SecureStore from "expo-secure-store";

let accessToken: string | null = null;
export const getAccessToken = () => accessToken;
export const setAccessToken = (t: string | null) => {
  accessToken = t;
};

const REFRESH_KEY = "bsaas_refresh_token";
export const getRefreshToken = () => SecureStore.getItemAsync(REFRESH_KEY);
export const setRefreshToken = (t: string | null) =>
  t
    ? SecureStore.setItemAsync(REFRESH_KEY, t)
    : SecureStore.deleteItemAsync(REFRESH_KEY);

// lib/api.ts
const api = axios.create({ baseURL: process.env.EXPO_PUBLIC_API_URL });
// Request interceptor  → attach Authorization: Bearer <accessToken>
// Response interceptor → on 401: getRefreshToken() → POST /auth/mobile/refresh → store new tokens → retry
```

### Component Quality Standard (mobile)

- Fully themed dark/light, same design tokens as web
- Indigo accent on interactive elements, one typeface (Inter), same scale hierarchy as web
- Native feel over pixel-parity — iOS vs Android navigation conventions, safe areas, haptics
  (`expo-haptics`) on key actions
- Designed loading/empty/error states, not default RN placeholders
- A screen isn't done until checked on both iOS and Android

### Deployment

- EAS Build for iOS/Android binaries, EAS Submit for store submission, EAS Update for OTA JS
  updates between store releases
- Expo Go for early development only — move to development builds before anything
  production-like; Expo Go tracks only the latest SDK

### Version note (added r14)

Current stable is **Expo SDK 57** (57.0.7 as of this week), riding **React Native 0.86** /
**React 19.2** — confirms the number this doc guessed pre-r10. One scaffolding gotcha:
`create-expo-app@latest` without a template flag is still defaulting to **SDK 54** during the
transition window — use `--template default@sdk-57` explicitly. Re-confirm both numbers the day
you actually scaffold; Expo SDKs move fast enough that this note itself may be stale by then.

---

## 15. MOBILE MODULE REGISTRY

Same status convention as Section 5/8/9. **This section previously said "nothing built yet" —
false as of 2026-08-03; see Section 9 → MOBILE APP for the full correction.** Statuses below are
verified directly against `mobile/src/`, not against the v1 plan this registry originally
described.

Scope note (added r14): this v1 list predates several web features shipped since r10 — Agenda,
Setup/Routing, Templates, Capture. Capture/Agenda status per-screen is noted inline below where
mobile happens to already cover them (folded into the single CRM screen); the rest still isn't
represented here — decide inclusion explicitly when next revisited.

---

### AUTH SCREENS [🔵 ACTIVE — pages built, two hooks still stubbed]

Login and signup screens are real and call the real backend (`AUTH — MOBILE`, Section 5). Forgot
password and reset password *pages* exist (`(auth)/forgot-password.tsx`, `reset-password.tsx`),
but the hook behind them (`useAuth.ts`) is `console.log` + a fake `setTimeout` — no real API call
yet, even though the backend routes they'd need already exist. This is the one clear next step
tracked in the Section 9 entry.

---

### ONBOARDING [✅ DONE]

`create-organization.tsx` and `select-organization.tsx` are real, top-level routes — shown after
login when no org context exists, same as web.

---

### DASHBOARD SHELL [✅ DONE]

Root and `(dashboard)` layouts, org-scoped `[orgId]` layout, dashboard home (`index.tsx`) all real.
Org switcher / profile-menu mechanics not independently re-verified beyond confirming the files
are real and non-trivial (172 lines for the dashboard home alone).

---

### TASKS [✅ DONE]

`tasks/index.tsx` (195 lines) is real, hits the same `tasks.*`-permissioned backend routes as web
via `lib/tasksApi.ts`.

---

### CRM — LEADS & PIPELINE [🔵 PARTIAL — exists, but as one tabbed screen, not split routes]

`apps/crm/index.tsx` (263 lines) is a single screen with an internal tab switcher covering Leads,
Pipeline, Reports, Agenda, and Setup — not the per-entity route split (`leads/`, `pipeline/`, etc.)
this registry originally called for. Real data, real API calls (`lib/crmApi.ts`), just a different
information architecture than planned. Decide whether to keep this shape or split it before
building further on top.

---

### CRM — CONTACTS [⚪ NOT STARTED]

No Contacts or Companies tab was found in the single CRM screen above, and no separate route
exists — still genuinely not built, unlike Leads/Pipeline/Reports/Agenda which got folded in.

---

### CRM — REPORTS [🔵 PARTIAL — folded into the CRM tab screen, not a dedicated summary-cards screen]

A "Reports" tab exists inside `apps/crm/index.tsx` rather than the standalone summary-cards screen
originally planned. Real data via `lib/crmApi.ts`; not independently verified whether it's cards
or full charts — worth a direct look before assuming either way.

---

### SETTINGS — MEMBERS, ROLES & PERMISSIONS [⚪ NOT STARTED — lower priority]

`settings/index.tsx` is a flat profile-style screen; it lists "Members" as a menu item but there is
no `settings/members/` or `settings/roles/` route behind it — the label exists, the screen doesn't.
Admin-heavy screens, arguably fine to stay web-only for v1. Revisit after the above ships.

---

### SECURITY [⚪ NOT STARTED — lower priority]

Session list, revoke, login events. Likely fine as web-only initially too.

---

## HOW TO UPDATE THIS DOCUMENT

When phase status changes:

- Update Section 2 STATUS blocks
- Update MODULE REGISTRY entries (change `🔵 ACTIVE` to `✅ DONE`)

When a new backend module is added:

- Add a new entry to Section 5 with its routes and permissions

When a new frontend page is built:

- Update Section 8

When a new mobile screen is built:

- Update Section 15 (Mobile Module Registry)
- Note any deviation from Section 14 (Mobile Architecture) in Section 14 itself if the plan
  changes mid-build — don't let the decided architecture silently drift from what's implemented

When a Zustand store gains new fields:

- Update the store interface in Section 7
- Never add token or sensitive credential fields — see hard rules

When a major architectural decision is made:

- Add a note to the relevant section (3, 4, or 7)

When design tokens change:

- Update Section 3 and `globals.css` together — keep them in sync

When something in the unbuilt module registry starts real work:

- Promote it in Section 9 (`⚪ NOT STARTED` → `🔵 ACTIVE`) and add a proper Section 5/8 entry
- Mark it `✅ DONE` in Section 9 (or remove the entry) once it ships
- Update Section 2's phase status if it changes the active phase

When something existing is modified, not added:

- Route shape changes — update the route block in Section 5/8 in place
- A permission key is renamed, split, or merged — update every place it's listed
- A status flag becomes inconsistent across sections — fix it on sight, don't wait for an audit pass

When a documented assumption gets resolved:

- Delete once confirmed true, or correct with the real behavior once confirmed false — an unresolved assumption sitting for multiple revisions means it was never checked

When the DB schema changes:

- New table → Section 6 Key Tables under the right group
- Migration count changes → update Section 6 same-day — this number goes stale fastest of anything in the doc
- A CHECK constraint, enum, or column change that affects API behavior → note it in the relevant Section 5 module entry

When a new dependency is added:

- Backend: Section 13 with version pinned
- Frontend: same, plus Section 3 if it's a new category of tool

When a module ships with known defects (r11):

- Ship status stays `🔵 ACTIVE`, never `✅ DONE`, until the known-open-items list is empty — a module that compiles but can't perform its core function is not done. Embed the defect list in the module's Section 5 entry so it can't be forgotten (see CAPTURE for the pattern).

When a scoping entry is written for unbuilt work (r15):

- It goes in Section 9 as `⚪ NOT STARTED`, carries no priority ordering and no effort estimate, and stays a map — the moment work starts, the real detail moves to a Section 5/8 entry rather than growing inside Section 9.
- If the same shared pattern is identified across three or more independent scoping passes, record it in PLATFORM PRIMITIVES instead of repeating it per module — but do not build it until a real consumer exists.

Periodic structural drift audit:

- Before any major "update the docs" pass, don't trust the document's own status flags as ground truth — grep the actual source for route counts, migration counts, and file existence, then reconcile. This document has been wrong about counts and statuses in every single revision audit so far (r9: HRM/deployment; r11: four whole shipped features missing; r15: the copy being edited was a full revision behind on mobile). Assume drift by default.
- Cross-check Section 5 (backend) against Section 8 (frontend) against Section 9 (unbuilt module registry) for the same module.
- A doc pasted into a chat conversation is not guaranteed to be the committed `docs/Project_Instruction.md` — whichever copy is more recent should overwrite the other after an update.

Keep this document current. A stale instruction is worse than no instruction.
