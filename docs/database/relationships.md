# Database Relationships

This document explains how records connect and how deletion behavior should be interpreted by the application.

## Relationship summary

| Table                | Relationship notes                                                                                                                         |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| users                | Parent of auth_accounts through auth_accounts.user_id.                                                                                     |
| users                | Parent of sessions through sessions.user_id.                                                                                               |
| users                | Parent of verification_tokens through verification_tokens.user_id.                                                                         |
| users                | Parent of organization_members through organization_members.user_id.                                                                       |
| users                | Referenced by audit_logs.user_id and organization_members.invited_by.                                                                      |
| organizations        | Parent of organization_members, roles, sessions, subscriptions, organization_usage, and audit_logs.                                        |
| permissions          | No FK from roles.permissions because roles stores permission keys as TEXT[]. Application must validate these keys against permissions.key. |
| roles                | May belong to organizations through roles.org_id.                                                                                          |
| roles                | Referenced by organization_members.role_id.                                                                                                |
| organization_members | Belongs to organizations.                                                                                                                  |
| organization_members | Belongs to users.                                                                                                                          |
| organization_members | Optionally belongs to roles.                                                                                                               |
| organization_members | invited_by references users.                                                                                                               |
| auth_accounts        | Belongs to users through user_id.                                                                                                          |
| sessions             | Belongs to users.                                                                                                                          |
| sessions             | Optionally belongs to organizations.                                                                                                       |
| verification_tokens  | Optionally belongs to users.                                                                                                               |
| subscriptions        | Belongs to organizations.                                                                                                                  |
| subscriptions        | Parent/optional reference for organization_usage.                                                                                          |
| organization_usage   | Belongs to organizations.                                                                                                                  |
| organization_usage   | Optionally belongs to subscriptions.                                                                                                       |
| audit_logs           | Optionally belongs to organizations.                                                                                                       |
| audit_logs           | Optionally belongs to users.                                                                                                               |
| tasks                | Belongs to organizations via org_id.                                                                                                       |
| tasks                | Optionally belongs to users via created_by.                                                                                                |
| tasks                | Optionally belongs to users via assigned_to.                                                                                               |

## Foreign key deletion behavior

| Child table          | Foreign key     | Parent table  | Delete behavior    | Business meaning                                                                |
| -------------------- | --------------- | ------------- | ------------------ | ------------------------------------------------------------------------------- |
| auth_accounts        | user_id         | users         | ON DELETE CASCADE  | If a user is physically removed, linked provider accounts are removed.          |
| sessions             | user_id         | users         | ON DELETE CASCADE  | If a user is physically removed, sessions are removed.                          |
| sessions             | org_id          | organizations | ON DELETE CASCADE  | Organization-scoped sessions are removed if organization is physically removed. |
| verification_tokens  | user_id         | users         | ON DELETE CASCADE  | User-bound verification/recovery tokens are removed.                            |
| roles                | org_id          | organizations | ON DELETE CASCADE  | Tenant-specific roles are removed with the tenant.                              |
| organization_members | org_id          | organizations | ON DELETE CASCADE  | Memberships are removed if organization is removed.                             |
| organization_members | user_id         | users         | ON DELETE CASCADE  | Memberships are removed if user is removed.                                     |
| organization_members | role_id         | roles         | ON DELETE SET NULL | Membership survives if assigned role is removed; role_key remains as fallback.  |
| organization_members | invited_by      | users         | ON DELETE SET NULL | Invitation record survives if inviter is removed.                               |
| subscriptions        | org_id          | organizations | ON DELETE CASCADE  | Subscription belongs to organization.                                           |
| organization_usage   | org_id          | organizations | ON DELETE CASCADE  | Usage belongs to organization.                                                  |
| organization_usage   | subscription_id | subscriptions | ON DELETE SET NULL | Usage history can remain even if subscription reference is removed.             |
| audit_logs           | org_id          | organizations | ON DELETE CASCADE  | Current schema removes tenant audit logs if organization is physically deleted. |
| audit_logs           | user_id         | users         | ON DELETE SET NULL | Audit event remains even if actor user is removed.                              |
| tasks                | org_id          | organizations | ON DELETE CASCADE  | All tasks are removed when organization is deleted.                             |
| tasks                | created_by      | users         | ON DELETE SET NULL | Task survives if creator is removed; created_by becomes NULL.                   |
| tasks                | assigned_to     | users         | ON DELETE SET NULL | Task survives if assignee is removed; assigned_to becomes NULL.                 |

## Recommended SaaS policy

Prefer soft delete for `users` and `organizations`. Physical deletion should be rare and controlled because many child records cascade. For compliance or privacy deletion, create a formal deletion/anonymization procedure instead of casually deleting parent rows.

---

## HRM (added r9)

40 tables. Every one of them has `org_id → organizations(id) ON DELETE CASCADE` and most have `created_by → users(id)` with no delete action specified (defaults to `NO ACTION`/restrict at the DB level) — both omitted from the tables below since they're universal. What's documented here is everything that deviates from "belongs to org, created by a user."

### Relationship summary by pattern

**Org structure (self-contained hierarchy):** `hrm_departments` is self-referencing (`parent_department_id`) and is the parent of `hrm_positions` and `hrm_employees`. `hrm_employees` is also self-referencing (`manager_id`). `hrm_departments.head_employee_id` has **no FK constraint at all** — it's set as a plain UUID because `hrm_employees` doesn't exist yet when `hrm_departments` is created in the same migration; the application must enforce this link's integrity.

**`hrm_employees` is the hub.** Nearly every other HRM table hangs off `employee_id → hrm_employees(id)`, almost always `ON DELETE RESTRICT` rather than `CASCADE` — see the RESTRICT section below. This is deliberate: an employee record shouldn't be deletable while they have contracts, promotions, warnings, payslips, etc. attached. In practice employees are soft-deleted via `status = 'terminated'`, not row-deleted.

**Approval chains are referenced polymorphically.** `hrm_approval_instances.entity_id` and `hrm_acknowledgements.acknowledgeable_id` have no FK at all — they're matched against a sibling `entity_type`/`acknowledgeable_type` column at the application layer. Six source tables (`hrm_promotions`, `hrm_transfers`, `hrm_resignations`, `hrm_terminations`, `hrm_employee_warnings`, `hrm_awards`, plus `hrm_attendance_records` for regularization) hold `approval_instance_id → hrm_approval_instances(id) ON DELETE SET NULL` — if the approval instance is deleted, the record survives but loses the link, reverting effectively to un-gated.

**Snapshot fields duplicate data on purpose, with no FK to keep it live.** E.g. `hrm_payslip_lines.component_name` is a plain TEXT copy, not a live join — see `erd.md`'s "Notes specific to HRM's modeling" for why.

**Two independent polymorphic `_assignments` tables** — `hrm_work_schedule_assignments` and `hrm_calendar_assignments` — both use an `assignee_type`/`assignee_id` pair with no FK on `assignee_id`, matched against `organizations`, `hrm_departments`, or `hrm_employees` depending on `assignee_type`.

### Foreign key deletion behavior — exceptions to "CASCADE on org_id, unconstrained on created_by"

| Child table                                                                                                               | Foreign key             | Parent table           | Delete behavior    | Business meaning                                                                                                                              |
| ------------------------------------------------------------------------------------------------------------------------- | ----------------------- | ---------------------- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- |
| hrm_employees                                                                                                             | user_id                 | users                  | ON DELETE SET NULL | Employee record survives if the linked login account is removed — HRM employee identity is independent of platform user accounts.             |
| hrm_departments                                                                                                           | parent_department_id    | hrm_departments        | ON DELETE SET NULL | Child department becomes top-level if its parent is deleted, rather than being deleted itself.                                                |
| hrm_positions                                                                                                             | department_id           | hrm_departments        | ON DELETE SET NULL | Position survives an org restructure; just loses its department link.                                                                         |
| hrm_leave_requests                                                                                                        | leave_type_id           | hrm_leave_types        | ON DELETE RESTRICT | Can't delete a leave type that has requests against it — must deactivate (`is_active = false`) instead.                                       |
| hrm_promotions / hrm_transfers / hrm_resignations / hrm_terminations / hrm_awards / hrm_complaints                        | employee_id             | hrm_employees          | ON DELETE RESTRICT | Lifecycle and disciplinary records block employee deletion — this is the main enforcement of "soft-delete employees, don't hard-delete them." |
| hrm_employee_warnings                                                                                                     | employee_id             | hrm_employees          | ON DELETE RESTRICT | Same reasoning — a warning history can't be orphaned by deleting the employee.                                                                |
| hrm_employee_warnings                                                                                                     | issued_by               | users                  | ON DELETE RESTRICT | Can't delete a user who has issued warnings — preserves disciplinary accountability.                                                          |
| hrm_attendance_records / hrm_payslips / hrm_employee_milestones                                                           | employee_id             | hrm_employees          | ON DELETE RESTRICT | Same pattern for time & compensation and recognition records.                                                                                 |
| hrm_salary_structure_components                                                                                           | component_id            | hrm_salary_components  | ON DELETE RESTRICT | A component in active use by a structure can't be deleted — deactivate instead.                                                               |
| hrm_document_bulk_sends                                                                                                   | template_id             | hrm_document_templates | ON DELETE RESTRICT | Preserves the record of what was sent even if someone tries to delete the template afterward.                                                 |
| hrm_promotions                                                                                                            | to_position_id          | hrm_positions          | ON DELETE RESTRICT | Can't delete a position that's the target of a promotion record.                                                                              |
| hrm_salary_structures (via structure_id in payslips/promotions/etc.)                                                      | —                       | hrm_salary_structures  | ON DELETE SET NULL | Historical records survive a structure deletion; they just lose the live link (values are already snapshotted separately).                    |
| hrm_holidays / hrm_calendar_assignments                                                                                   | calendar_id             | hrm_holiday_calendars  | ON DELETE CASCADE  | Deleting a holiday calendar removes its holidays and assignments — the only CASCADE in HRM besides `org_id`.                                  |
| hrm_approval_template_levels                                                                                              | template_id             | hrm_approval_templates | ON DELETE CASCADE  | Levels have no meaning without their template.                                                                                                |
| hrm_approval_decisions                                                                                                    | instance_id             | hrm_approval_instances | ON DELETE CASCADE  | Decisions have no meaning without their instance.                                                                                             |
| hrm_payslip_lines                                                                                                         | payslip_id              | hrm_payslips           | ON DELETE CASCADE  | Line items have no meaning without their payslip.                                                                                             |
| hrm_payslips                                                                                                              | payslip_run_id          | hrm_payslip_runs       | ON DELETE CASCADE  | Individual payslips are removed if the whole run is deleted (rare — runs are normally cancelled, not deleted, once `computed`).               |
| hrm_warning_escalation_rules                                                                                              | trigger_warning_type_id | hrm_warning_types      | ON DELETE CASCADE  | An escalation rule is meaningless without the warning type it watches.                                                                        |
| _(all other `approval_instance_id`, `document_id`, `certificate_document_id`, `announcement_id`, `auto\__\_id` columns)\* | —                       | various                | ON DELETE SET NULL | The general pattern for "optional link to a generated/linked artifact" — the owning record always survives, it just loses the reference.      |

### Recommended policy for HRM specifically

Because so many tables `RESTRICT` on `employee_id`, there is effectively no supported path to hard-delete an `hrm_employees` row once any lifecycle, disciplinary, attendance, or payslip record exists against it — which in practice is almost immediately after hiring. Treat `hrm_employees.status = 'terminated'` as the deletion mechanism; a genuine hard-delete would require deleting or reassigning every dependent record first, table by table, in dependency order.
