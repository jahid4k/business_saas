# ADR-0015: HRM salary formula engine — expr-lang/expr

**Date:** 2026-07-07
**Status:** Accepted
**Deciders:** Mridha

---

## Context

HRM sub-module A1 (Salary Components) allows HR administrators to define how each salary
component — House Rent Allowance, Income Tax deduction, overtime pay, Loss of Pay, etc. —
is calculated when a payslip is generated.

The initial proposal was to support only fixed amounts and simple percentage-of-basic rules,
deferring formula evaluation to a later phase. The business requirement is a multi-country,
multi-currency payroll system where each customer's HR admin can define their own tax rules,
allowance tiers, and deduction logic without writing code or waiting for a software update.

This means formula evaluation must be available from the first production payroll run.

The core question: which formula evaluation approach is safe, flexible, and maintainable?

---

## Decision

Use **`github.com/expr-lang/expr` v1.16.9** as the sandboxed formula evaluation engine for
salary component calculation.

```go
// Safe. No reflection on arbitrary types. No system access.
env := map[string]any{
    "BASIC":          50000.0,
    "GROSS":          75000.0,   // sum of all earnings computed so far
    "WORKING_DAYS":   22.0,
    "ACTUAL_DAYS":    21.5,
    "ABSENT_DAYS":    0.5,
    "OVERTIME_HOURS": 3.0,
    "TENURE_YEARS":   3.0,
}
program, _ := expr.Compile(`BASIC * 0.40`, expr.Env(env), expr.AsFloat64())
result, _  := expr.Run(program, env)   // → 20000.0
```

Five calculation methods are supported per component:

| Method | Formula behaviour |
|--------|-------------------|
| `fixed` | Static decimal value, no formula |
| `pct_of_basic` | `BASIC × (fixed_value / 100)` |
| `pct_of_gross` | `GROSS × (fixed_value / 100)` |
| `formula` | Arbitrary expr-lang expression over the env vars |
| `slab` | Progressive bracket calculation from a JSONB config |
| `manual` | HR enters the value per payroll run; no formula |

---

## Reasoning

### Why a formula engine at all

Salary rules vary enormously by country and by employer. A Bangladeshi employer might apply
Income Tax at 5% of gross for the first BDT 300,000 and 10% above it. A UK employer might
apply National Insurance as a percentage of earnings above a threshold. A startup might pay
overtime at 1.5× the hourly rate calculated from monthly basic. None of these are expressible
as "fixed" or "percentage of basic" — they require conditional logic.

Without a formula engine, every new rule requires a code change and a deployment. With one,
HR admins define rules in the UI and the change takes effect on the next payroll run.

### Why expr-lang/expr specifically

**Security.** `expr-lang/expr` compiles expressions against a strictly typed environment.
The environment is an explicit Go map. The expression can only reference variables declared
in that map. There is no `reflect`, no file system access, no network calls, no goroutines.
`eval()` in JavaScript (or `text/template` + `os/exec` combinations) would be a security
disaster in a multi-tenant SaaS context where expressions are stored in the database.

**Performance.** Expressions are compiled once at validation time and can be cached as
`*vm.Program` objects. The `expr.Run()` call at payroll time is a simple tree walk —
measured in microseconds, not milliseconds. For a 500-employee organisation generating
500 payslips each with 10 component evaluations, this is 5,000 evaluations — well within
acceptable latency even without caching.

**Developer ergonomics.** The library is pure Go, well-documented, and actively maintained.
It does not require a CGO build, an external process, or a JVM. Deployment complexity is zero.

**Error messages.** `expr.Compile()` returns structured errors with column offsets when
a formula is syntactically invalid. These are surfaced directly to the HR admin via the
`POST /hrm/setup/salary/formula/test` endpoint before the formula is saved.

### Why not Go's text/template

`text/template` is not an expression evaluator — it is a document renderer. It cannot perform
arithmetic and cannot return a float64.

### Why not a custom parser

Writing a correct, secure expression parser is a months-long project with significant ongoing
maintenance. `expr-lang/expr` solves this problem already.

### Slab (progressive bracket) as a structured config, not a formula

Tax computations with multiple brackets are expressible as `expr-lang/expr` formulas but they
are verbose and error-prone. A JSONB `slab_config` structure is cleaner:

```json
{
  "base_variable": "GROSS",
  "slabs": [
    { "up_to": 30000,  "rate": 0.00 },
    { "up_to": 100000, "rate": 0.05 },
    { "up_to": null,   "rate": 0.10 }
  ]
}
```

The payslip engine evaluates slabs with a purpose-built Go function, not by generating a
formula string. This makes slab validation (last slab must have `up_to: null`, all must be
ascending) tractable at the service layer.

### Formula variable set

The following variables are available in every formula evaluation at payroll time:

```
BASIC          — active salary record basic_pay
GROSS          — running sum of all earning components computed so far
WORKING_DAYS   — total working days in the pay period (from finalized AttendancePeriod)
ACTUAL_DAYS    — days employee was present (including 0.5 for half-days)
ABSENT_DAYS    — WORKING_DAYS − ACTUAL_DAYS
OVERTIME_HOURS — total overtime hours in the period
LATE_MINUTES   — total late minutes accumulated in the period
TENURE_YEARS   — float: years since hire_date
TENURE_MONTHS  — float: months since hire_date
PERIOD_DAYS    — calendar days in the pay period month
```

Components are evaluated in `display_order` sequence. `GROSS` accumulates as each earning
component is computed, so a component that references `GROSS` sees the sum of all prior
earnings — this allows a "10% of gross" deduction to apply correctly.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Fixed + percentage only (defer formulas) | Insufficient for real payroll — income tax and LOP alone require conditional logic |
| JavaScript via Goja (embedded JS engine) | Full JS semantics increase the attack surface; `eval(code)` is possible inside Goja; no benefit over expr-lang for numeric computation |
| Lua via GopherLua | Adds Lua syntax learning curve for HR admins; no meaningful advantage over expr-lang |
| Python via subprocess | External process, CGO, or network call; unacceptable latency and security surface in a multi-tenant context |
| Go text/template | Cannot perform arithmetic; not an expression evaluator |
| Custom hand-written parser | Months of development; ongoing maintenance burden; `expr-lang/expr` already exists and is production-proven |

---

## Consequences

**Positive:**
- HR admins can define any numeric computation rule without a code change or deployment
- Formula validation at save time (before payroll runs) prevents runtime errors in production
- The `formula/test` endpoint lets HR admins verify a formula against sample values in the UI
- Tax rules and allowance structures are fully customer-configurable — the system is genuinely
  country-agnostic from day one

**Negative:**
- HR admins must learn the expr-lang expression syntax (documented in the UI)
- Complex multi-variable formulas with many conditionals are harder to audit than named rules
- A formula that references `GROSS` depends on evaluation order — a component must be placed
  after all earning components it should be summed with; this is a UI/UX concern to communicate

---

## Related decisions

- [ADR-0014](0014-hrm-extended-architecture.md) — Group A1 is the first group to implement;
  formula engine is available from the first HRM release
- [ADR-0017](0017-hrm-payslip-engine.md) — Payslip engine describes how the formula variables
  are populated from attendance data at payroll run time
