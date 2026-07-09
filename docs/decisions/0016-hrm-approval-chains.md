# ADR-0016: HRM approval chains — sequential levels with snapshot isolation

**Date:** 2026-07-07
**Status:** Accepted
**Deciders:** Mridha

---

## Context

Multiple HRM sub-modules require human approval before an action takes effect: leave requests,
resignations, promotions, transfers, warnings that require HR sign-off, document signatures,
and terminations. Each of these needs a configurable chain of approvers — not a hardcoded
"manager approves, then HR approves" flow.

The design must handle:
- Different chains per action type (leave might need manager only; termination needs manager +
  HR director)
- Multiple levels in a chain (level 1 → level 2 → level 3)
- Different approver types per level: the employee's reporting manager, the department head,
  a named role, or a specific user
- What happens when no action is taken within a deadline (SLA breach)
- The historical accuracy requirement: if a template changes after instances have been created,
  existing in-flight instances must not be affected

---

## Decision

Use a **sequential, SLA-gated, snapshot-isolated approval chain** with four approver types
and a configurable SLA breach action per level.

**Structure:**

```
ApprovalTemplate — org-defined chain for an action_type
  └─ ApprovalTemplateLevel (1..N, ordered) — who approves at this level
       ├─ approver_type: reporting_manager | dept_head | role | specific_user
       ├─ sla_hours: how long before the breach action fires
       └─ on_sla_breach: escalate_next | auto_approve | auto_reject

ApprovalInstance — runtime record created when an entity needs approval
  ├─ instance_snapshot (JSONB) — frozen copy of all levels at creation time
  ├─ current_level — which level is currently pending
  └─ overall_status: pending | approved | rejected | cancelled

ApprovalDecision — one row per level, written when an approver acts
  └─ action: approved | rejected | cancelled
```

**Flow:**

```
HR creates ApprovalTemplate with N levels.

Entity (leave request, resignation, etc.) enters "pending_approval" state.
→ ApprovalInstance is created, template levels are FROZEN into instance_snapshot.
→ Notification sent to the level-1 approver.

Approver calls POST .../approve or .../reject.
→ ApprovalDecision row written.
→ If approved AND more levels remain: current_level incremented, level-2 notified.
→ If approved AND no more levels: instance overall_status = approved, entity updated.
→ If rejected: instance overall_status = rejected, entity updated, requester notified.

SLA breach (background job, checked every N minutes):
→ Finds instances where the current level's deadline has passed.
→ Applies on_sla_breach: escalate_next | auto_approve | auto_reject.
```

**Multiple templates per action type** are supported. A `condition_expression` (evaluated
via expr-lang/expr, same engine as salary formulas) selects which template applies:
`leave_days > 10` routes long-leave requests to a stricter template; `null` condition means
"use this as the default."

---

## Reasoning

### Sequential, not parallel

Parallel approval at a level (any one of three managers can approve) was considered and
deferred. Sequential approval is simpler to implement, simpler to audit, and covers 95% of
real HR workflows. Parallel approval requires tracking who was notified vs who can still act —
a first-come-first-served race condition that complicates the UI significantly. It can be added
as a Level 3 enhancement when a customer explicitly requires it.

### Snapshot isolation is mandatory

This is the most critical design decision in the approval engine.

Without snapshot isolation, the following bug would occur in production:
1. HR defines Template A: Manager → HR Director (two levels)
2. Employee submits a leave request; ApprovalInstance is created referencing Template A
3. Manager approves — level 1 done, now at level 2
4. HR admin modifies Template A to add a new level 3: CFO approval for any leave > 10 days
5. The existing in-flight instance now has three levels in the template, but level 1 is already
   decided. The system cannot determine whether to apply the new level 3 retroactively.

With snapshot isolation, the in-flight instance holds a JSONB copy of the template levels
exactly as they were at the time of creation. Template changes never affect it.

This is the same principle as PayslipLine storing formula expression snapshots (ADR-0017)
and the same reason event sourcing systems store the event payload rather than a reference.

### Why four approver types

Real HR workflows involve these four distinct delegation patterns:

- `reporting_manager` — the `manager_id` FK on the employee record. Common for leave.
- `dept_head` — the `head_employee_id` FK on the department record. Used when the
  approver is the functional head, not the direct line manager.
- `role` — any member with the specified role name can approve. First-come-first-served.
  Useful for "any HR Manager can approve" without naming a specific person.
- `specific_user` — a named user UUID. Used for "the CFO must approve all transfers."

A fifth type — "next in hierarchy above the previous approver" — was considered and rejected.
It depends on the org chart being complete and accurate at all times, which cannot be guaranteed.

### SLA breach actions

Three options cover all practical scenarios:
- `escalate_next` — move to the next level without the current approver's action. Appropriate
  when the current approver is unavailable and the chain should not be blocked indefinitely.
- `auto_approve` — approve and advance. Appropriate for low-stakes actions where delay is
  worse than an unapproved pass-through (e.g. routine short-leave requests).
- `auto_reject` — reject and close. Appropriate when inaction implies denial (e.g. a
  short-validity expense reimbursement).

All three are logged with `action: auto_approved_sla` or equivalent so the audit trail
is clear that the decision was system-generated.

### One shared engine, not per-module approval logic

The approval engine is built as `internal/hrm/approvals` — a standalone package with its
own repository, service, and handler. Other services (leave, promotions, transfers) call
`approvals.Service.CreateInstance()` and check `approvals.Service.GetInstance()` status.
They do not implement their own approval logic.

This means approval behaviour is consistent across all modules, and a bug fix in the approval
engine benefits all modules simultaneously.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Hardcoded "manager → HR" for all action types | Cannot accommodate customer-specific workflows; inflexible from the first customer |
| Parallel approval at a level | Complex tracking, race conditions, deferred to Phase 2 |
| No snapshot — live template reference | Retroactive template changes corrupt in-flight instances; see Reasoning above |
| Per-module approval logic | Duplicated code, inconsistent behaviour, bugs fixed in one place not reflected in others |
| External workflow engine (Temporal, Conductor) | Adds a significant infrastructure dependency and operational overhead for what is, at its core, a simple sequential state machine |

---

## Consequences

**Positive:**
- HR admins can configure any sequential approval chain without a code change
- Template changes never corrupt in-flight instances
- All HRM modules (leave, promotion, transfer, warning, termination) use the same approval
  mechanism — one UI, one set of endpoints, one audit trail pattern
- SLA enforcement means no approval can be silently forgotten

**Negative:**
- Parallel approval at a level is not available in this version; customers requiring it must
  wait for a future ADR
- The snapshot mechanism means the approval UI must clearly show "this was the chain at
  submission time" rather than "this is the current chain" — a UX distinction to communicate
- Condition expression evaluation at template selection time adds a dependency on expr-lang/expr;
  a formula bug in the condition can route a request to the wrong template

---

## Related decisions

- [ADR-0014](0014-hrm-extended-architecture.md) — Approval chains are Group A2; all groups
  B–E depend on this being complete first
- [ADR-0015](0015-hrm-formula-engine.md) — The same expr-lang/expr engine is used for
  template condition expressions as for salary formula evaluation
