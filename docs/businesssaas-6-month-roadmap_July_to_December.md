# BusinessSAAS — 6-Month Roadmap, Architecture Skill Plan & Mentor Questions

> Prepared: July 2026 · Companion to the mentor review
> Treat this like the Master Instruction: check items off, revise the plan at each month's end.
> If a month slips, shift everything — do not skip a month's Definition of Done.

---

## 0. The one-line diagnosis

Your engineering quality is roughly 12 months ahead of your product maturity, and 18 months ahead of your operational experience. The next 6 months exist to rebalance that: **ship, operate, get real users, and let reality pressure-test the architecture.** Almost nothing below adds a new module. That is deliberate.

---

## Ground rules (apply to all 6 months)

1. **WIP limit = 1.** One workstream at a time. Nothing new starts while an integration gap is open.
2. **No designing ahead.** Chat and lead-capture architectures are done — they wait until Month 6.
3. **Every month ends with a written 10-line retrospective** (what shipped, what broke, what surprised you). Keep them in `docs/retros/`.
4. **Master Instruction stays truthful.** The day reality changes, the doc changes. A stale doc is a bug — your own rule.
5. **The gate question for every task:** "Would a real user notice this?" If no, it needs a very good reason.

---

## Month 1 — Close every loop, then ship to production

**Goal: a URL you can show a stranger.**

### Close the open integration gaps first
- [ ] Contacts module: `main.go` DI wiring + route registration
- [ ] Contacts: run the pending migration; resolve the `crm.ts` type conflicts
- [ ] File upload: avatar wiring — `user/service.go`, `user/repository.go`, `user/routes.go`, `main.go`
- [ ] Seed migration for `files.view` / `files.upload` / `files.delete` permissions
- [ ] Full pass: `make test`, `npx tsc --noEmit`, ESLint — all green

### Fix the source-of-truth drift
- [ ] Master Instruction **r3**: HRM = ✅ DONE (31 endpoints), TanStack Query v5 (not Axios/Zustand-only), real migration count, module registry synced against `/api/v1/routes` output
- [ ] Update repo `README.md` — it still tells the Phase-1 story
- [ ] Decide and write down: **which document governs** (recommendation: Master Instruction governs intent, `/routes` + ADRs govern reality)

### Production deployment
- [ ] VPS: real domain, TLS (Caddy or nginx + certbot), production Docker Compose profile
- [ ] Uncomment and wire `deploy.yml` (SSH + GitHub Secrets)
- [ ] Security headers middleware (CSP, HSTS, X-Frame-Options, etc.)
- [ ] Global rate limiting + request body size limits (both already on your deferred list — this is the month)
- [ ] Environment hardening: prod CORS origins explicit, `/routes` confirmed hidden, secrets never in images

### Operations baseline
- [ ] Nightly `pg_dump` → offsite storage (Cloudflare R2 or Backblaze B2)
- [ ] **One timed restore drill.** A backup that has never been restored is not a backup. Write down how long it took.
- [ ] Sentry: backend + frontend
- [ ] Uptime monitoring (healthchecks.io or UptimeRobot) on `/api/v1/health`

**Definition of Done:** production URL live · uptime monitor green for 7 straight days · restore-drill notes committed.

---

## Month 2 — Real users + E2E safety net

**Goal: 3–5 humans using it who are not you.**

- [ ] Recruit 3–5 real users: a friend's shop, a freelancer managing clients, your own project pipeline. Free. You want feedback, not revenue yet.
- [ ] **Friction log:** write down every single place they get stuck. Especially the invited-user redirect chain (3 sequential API calls before dashboard — you already flagged it) and the viewer-role sparse sidebar.
- [ ] Playwright E2E on the money paths only:
  - [ ] signup → create org → invite member → accept → invited member logs in
  - [ ] lead → convert → deal → move stage → won
  - [ ] login → refresh → logout; session revoke actually kills the session
- [ ] Wire E2E into CI
- [ ] Enable `pg_stat_statements` + `log_min_duration_statement = 250ms`; review weekly
- [ ] Fix the **top 5 user-reported frictions**. Nothing else this month.

**Definition of Done:** 3+ users active for 2+ weeks · E2E green in CI · friction log with fixes checked off.

---

## Month 3 — One vertical, chosen by users

**Goal: ship one complete module surface — and let users pick which.**

- [ ] Decision (from Month 2 feedback, not from the roadmap): **HRM frontend** (backend is ready) *or* **CRM depth**. One. Write a 5-line decision note.
- [ ] Ship it fully: all pages, empty states, error states, permission gates, reports — no "coming soon"
- [ ] E2E on its critical path
- [ ] TanStack Virtual on your heaviest list; record before/after render numbers
- [ ] Fix viewer-role empty-sidebar UX (explicit "your role limits what you see" state)

**Definition of Done:** module flipped to ✅ in the registry · at least one real user actively using it.

---

## Month 4 — Billing & quotas

**This is an architecture course disguised as a feature.** Webhooks, retries, idempotency, state machines, eventual consistency — on your own stack, with real stakes.

- [ ] ADR: payment provider (Stripe vs SSLCommerz/local — depends on your target market; write the trade-off)
- [ ] Plans + subscription **state machine on paper first** (trialing → active → past_due → canceled; downgrade behavior defined in writing before code)
- [ ] You already have `subscriptions` and `organization_usage` tables — wire them into enforcement
- [ ] Webhook handling: signature verification + **idempotency keys** (on your deferred list — now). Replay a webhook; confirm no double effect.
- [ ] Quota enforcement in the **service layer** (not UI): file storage bytes, member seats
- [ ] Grace states and dunning behavior

**Definition of Done:** a test card can subscribe an org · webhook replay is a no-op · quota block actually blocks.

---

## Month 5 — Break it on purpose

**Goal: know your numbers. Architecture skill is 50% knowing how things fail.**

- [ ] k6 load scripts: login+refresh storm · deals board · timeline query · permission-middleware hot path
- [ ] Find the breaking point; profile with `pprof` (CPU + heap); fix the top 3 bottlenecks; **write the numbers down**
- [ ] `EXPLAIN ANALYZE` the 10 heaviest queries — start with the unified timeline query (multi-source timeline queries are where these designs classically die at scale)
- [ ] Add missing indexes; re-measure
- [ ] Chaos-lite in staging:
  - [ ] Kill Redis → does the permission check degrade to DB fallback, or does the app fall over?
  - [ ] Kill Postgres → is the user-facing error designed, or a stack trace?
- [ ] Restore drill #2 — beat Month 1's time

**Definition of Done:** `docs/LOAD-TEST.md` with real numbers, what changed, and the current known ceiling (e.g., "breaks at ~N RPS on the board endpoint because X").

---

## Month 6 — One differentiator + the decision

- [ ] Ship **one** of: real-time chat *or* lead auto-capture (both already designed — pick by user demand, not preference)
- [ ] Vertical and complete: backend + frontend + E2E + docs
- [ ] **Public write-up:** the architecture, what broke in Months 1–5, the numbers. This is a career artifact and the single best thinking-sharpener available to you.
- [ ] **The decision memo (1 page, honest):** is BusinessSAAS a *product* (pursue paying customers seriously) or a *portfolio* (extract learnings, maybe open-source parts)? Both are legitimate wins. Drifting between them for another year is the only losing move.

**Definition of Done:** feature live for real users · post published · decision written.

---

# Part 2 — Skill Plan: The Architecture Track

## The mental-model shift

| Level | Optimizes for | Thinks in |
|---|---|---|
| Junior | Working code | Functions |
| Mid | Shipped features | Modules & patterns |
| **Senior** | **System outcomes** | **Trade-offs, failure modes, cost of change** |
| Architect | Decision quality over years | Reversible vs irreversible; what the org can operate |

You currently write code at a solid mid-to-senior level. What separates you from senior is not code — it's **consequences**: nothing you've built has ever failed under real load, been operated for a year, or been constrained by other people. The roadmap above is the cure; the items below accelerate it.

Architecture, concretely, is: (1) knowing failure modes, (2) getting the reversible-vs-irreversible call right, (3) communicating decisions so others can execute them, (4) operating what you design. You already do (3) — the ADR habit is genuinely rare, even among seniors. The roadmap builds (1) and (4). Books below build vocabulary for all of it.

## Reading order — one book at a time, applied to BusinessSAAS

1. **Release It!** (Michael Nygard) — read during Months 1–2. It maps 1:1 onto going to production: timeouts, bulkheads, circuit breakers, what actually breaks. Every chapter, ask "where is this failure mode in my stack?"
2. **Designing Data-Intensive Applications** (Kleppmann) — 1 chapter/week, with notes. The canonical book. Chapters on indexes, replication, and transactions will directly change how you look at your Postgres usage.
3. **A Philosophy of Software Design** (Ousterhout) — short; sharpens your instinct for when abstraction earns its cost. You'll recognize your platform/engagement split in it.
4. **Fundamentals of Software Architecture**, then **Software Architecture: The Hard Parts** (Richards & Ford) — the formal vocabulary of trade-off analysis.
5. Ongoing drills: **System Design Interview vol 1 & 2** (Alex Xu) — one design per week on paper (rate limiter, notification system, news feed), then compare against what *you* would do with your stack.

## Habits (these compound more than the books)

- **ADRs, upgraded:** add two sections to your template — *"Failure modes"* and *"What would make this decision wrong?"*
- **Postmortems** for every production incident, even with an audience of one. Blameless format: timeline → root cause → what changes.
- **Napkin math, monthly:** at 10 / 100 / 1,000 orgs — how many rows in `crm_deals`? What's the timeline query cost? When does the permission cache stop fitting? Estimation is a core architect muscle and it only grows with reps.
- **One architecture study per month** from engineering blogs (Figma's multiplayer, Slack's message delivery, Shopify's pods, Discord's database migrations). Write 10 lines: what constraint drove the design, what they traded away.

## The solo-dev deficit — and its fix

Your biggest structural gap is **never experiencing other people's code, constraints, and reviews.** Two fixes:

- **One quality PR per month to a Go OSS project** — Gitea, Mattermost, or PocketBase (all SaaS-shaped Go codebases). The value is not the commit; it's reading mature architecture and receiving review from strangers.
- **Read stdlib source:** `net/http` and `context` first. Go's stdlib is the best Go architecture textbook that exists.

## What NOT to do

- Don't collect languages/frameworks. Depth in Go + Postgres + one frontend beats breadth, every time, for the architect path.
- Don't read five books in parallel. One, applied, finished.
- Don't confuse architecture with diagrams. It's decision-making under constraints with failure modes priced in. Diagrams are the receipt, not the work.

---

# Part 3 — Questions to Ask a Real Senior Engineer

## How to ask (this matters more than the questions)

- **Bring artifacts, not abstractions.** "Any advice?" gets platitudes. "Here's my tenant-isolation design — attack it" gets gold.
- **Ask about failures, not successes.** Success stories are survivorship bias; scar tissue is where judgment lives.
- **Never ask what Google answers.** Ask what only experience answers.
- After any answer: *"What would you have done differently, knowing what you know now?"*

## Scar tissue (judgment through failure)

- "What's a design decision that looked right and took a year to hurt? What early signal did you ignore?"
- "Tell me about your worst production incident. What changed permanently in how you build afterward?"
- "What's a system you're ashamed of that's still running? Why is it still running?"
- "When did you last delete a big abstraction you'd built? What taught you it was wrong?"

## Trade-off judgment

- "How do you decide when an abstraction earns its cost? What's your actual test?"
- "When do you deliberately choose the boring or worse-on-paper technology?"
- "What's your real heuristic for tech-debt paydown vs feature work — not the theory, the one you use under deadline?"
- "How do you tell 'this design is wrong' from 'this design is unfamiliar'?"

## Attack my work (highest ROI — use your 30 minutes here)

- "Here's my tenant isolation: JWT org claim → middleware param check → repository returns not-found cross-org. Where would you try to break it?"
- "Here's my ADR for [X]. What's missing from the trade-off analysis?"
- "Here's my unified-timeline query design. At what scale does it die, and what would you do then?"
- "I'm solo. What's the first thing that falls over when a second engineer joins this codebase?"

## Calibrating your own growth

- "When you were at my stage, what did you misunderstand about what 'senior' means?"
- "What do engineers with good judgment have in common — and how did they get it?"
- "What made someone 'architect material' in your org — the actual signal, not the job-ladder text?"
- "What should I be doing at 100 users that I'm not doing at 0?"

## Where to find them

OSS maintainers (a good PR earns a conversation) · authors of engineering-blog posts you studied (short, specific emails get replies) · local Go/backend meetups · r/ExperiencedDevs for async questions. Specificity is the whole game — generic requests get ignored, specific artifacts get engagement.

---

# Anti-Pattern Watchlist (self-check at each month's end)

- [ ] Did I start something new while an integration gap was open?
- [ ] Does any doc say X while the code says Y?
- [ ] Did I ship anything this month that no user asked for?
- [ ] Did a full month pass with zero production learning?
- [ ] Am I designing Month-6 features in Month 2 because it's more fun?

Any "yes" = correct course next month. That's the whole system.
