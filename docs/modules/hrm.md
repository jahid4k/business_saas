# Module: HRM

> Verified directly against source (`backend/internal/hrm/`, `frontend/src/app/(dashboard)/[orgId]/hrm/`) — 2026-07-13.

## What this module does

HRM is the second major business module in BusinessSAAS (after CRM), covering the full employee
lifecycle: org structure, leave, attendance, payroll, disciplinary records, documents, recognition,
and HR communication. It's built as 25 sub-modules under `internal/hrm/`, all sharing the same
tenant-isolation and permission-gating pattern CRM established.

All HRM endpoints are tenant-scoped — they require a JWT with `bid` (org ID) embedded, and the
`:orgId` URL parameter must match the JWT `bid`, enforced via `RequireOrganizationParam` middleware
(the handler var name in HRM route files is `requireOrgMatch`, same middleware).

**Scale:** 30 migrations (`00020`–`00050`, with `00048` excluded as unrelated CRM seed data — see
Section 6 of the Master Instruction), 30 tables, 201 routes, 25 sub-modules, built in five dependency-
ordered groups (A → B → C → D → E). All 8 frontend phases are complete.

## Backend endpoints

Route prefix for everything below: `/api/v1/organizations/:orgId/hrm/...`

### Group A — Setup / Config

**Departments** — `hrm.departments.*`

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/departments` | `hrm.departments.view` |
| POST | `/hrm/departments` | `hrm.departments.create` |
| GET | `/hrm/departments/:deptId` | `hrm.departments.view` |
| PATCH | `/hrm/departments/:deptId` | `hrm.departments.update` |
| DELETE | `/hrm/departments/:deptId` | `hrm.departments.delete` |

**Positions** — `hrm.positions.*`

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/positions` | `hrm.positions.view` |
| POST | `/hrm/positions` | `hrm.positions.create` |
| GET | `/hrm/positions/:posId` | `hrm.positions.view` |
| PATCH | `/hrm/positions/:posId` | `hrm.positions.update` |
| DELETE | `/hrm/positions/:posId` | `hrm.positions.delete` |

**Salary** — `hrm.salary.*` / `hrm.salary.employee.*` — components, formula testing, structures, employee assignment

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/setup/salary/components` | `hrm.salary.view` |
| POST | `/hrm/setup/salary/components` | `hrm.salary.manage` |
| GET | `/hrm/setup/salary/components/:compId` | `hrm.salary.view` |
| PATCH | `/hrm/setup/salary/components/:compId` | `hrm.salary.manage` |
| DELETE | `/hrm/setup/salary/components/:compId` | `hrm.salary.manage` |
| POST | `/hrm/setup/salary/formula/test` | `hrm.salary.manage` |
| GET | `/hrm/setup/salary/structures` | `hrm.salary.view` |
| POST | `/hrm/setup/salary/structures` | `hrm.salary.manage` |
| GET | `/hrm/setup/salary/structures/:structId` | `hrm.salary.view` |
| PATCH | `/hrm/setup/salary/structures/:structId` | `hrm.salary.manage` |
| DELETE | `/hrm/setup/salary/structures/:structId` | `hrm.salary.manage` |
| POST | `/hrm/setup/salary/structures/:structId/components` | `hrm.salary.manage` |
| DELETE | `/hrm/setup/salary/structures/:structId/components/:compId` | `hrm.salary.manage` |
| GET | `/hrm/employees/:employeeId/salary` | `hrm.salary.employee.view` |
| POST | `/hrm/employees/:employeeId/salary` | `hrm.salary.employee.manage` |

**Approvals** — `hrm.approvals.*` — chain templates + instance decisions

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/setup/approvals` | `hrm.approvals.view` |
| POST | `/hrm/setup/approvals` | `hrm.approvals.manage` |
| GET | `/hrm/setup/approvals/:templateId` | `hrm.approvals.view` |
| PATCH | `/hrm/setup/approvals/:templateId` | `hrm.approvals.manage` |
| DELETE | `/hrm/setup/approvals/:templateId` | `hrm.approvals.manage` |
| GET | `/hrm/setup/approvals/instances/:instanceId` | `hrm.approvals.view` |
| POST | `/hrm/setup/approvals/instances/:instanceId/approve` | `hrm.approvals.action` |
| POST | `/hrm/setup/approvals/instances/:instanceId/reject` | `hrm.approvals.action` |

> **Known gap:** there's no `GET /hrm/setup/approvals/instances` list endpoint — only fetch-by-id. Documented separately as a standalone backend issue, not yet fixed.

**Warning Types** — `hrm.warning_types.*` — type config + escalation rules

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/setup/warning-types/escalations` | `hrm.warning_types.view` |
| POST | `/hrm/setup/warning-types/escalations` | `hrm.warning_types.manage` |
| PATCH | `/hrm/setup/warning-types/escalations/:ruleId` | `hrm.warning_types.manage` |
| DELETE | `/hrm/setup/warning-types/escalations/:ruleId` | `hrm.warning_types.manage` |
| GET | `/hrm/setup/warning-types` | `hrm.warning_types.view` |
| POST | `/hrm/setup/warning-types` | `hrm.warning_types.manage` |
| GET | `/hrm/setup/warning-types/:typeId` | `hrm.warning_types.view` |
| PATCH | `/hrm/setup/warning-types/:typeId` | `hrm.warning_types.manage` |
| DELETE | `/hrm/setup/warning-types/:typeId` | `hrm.warning_types.manage` |

**Document Templates** — `hrm.doc_templates.*`

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/setup/document-templates` | `hrm.doc_templates.view` |
| POST | `/hrm/setup/document-templates` | `hrm.doc_templates.manage` |
| POST | `/hrm/setup/document-templates/:templateId/preview` | `hrm.doc_templates.view` |
| GET | `/hrm/setup/document-templates/:templateId` | `hrm.doc_templates.view` |
| PATCH | `/hrm/setup/document-templates/:templateId` | `hrm.doc_templates.manage` |
| DELETE | `/hrm/setup/document-templates/:templateId` | `hrm.doc_templates.manage` |

**Shifts** — `hrm.shifts.*` — config + assignment

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/setup/shifts/assignments` | `hrm.shifts.view` |
| POST | `/hrm/setup/shifts/assignments` | `hrm.shifts.manage` |
| DELETE | `/hrm/setup/shifts/assignments/:assignmentId` | `hrm.shifts.manage` |
| GET | `/hrm/setup/shifts` | `hrm.shifts.view` |
| POST | `/hrm/setup/shifts` | `hrm.shifts.manage` |
| GET | `/hrm/setup/shifts/:shiftId` | `hrm.shifts.view` |
| PATCH | `/hrm/setup/shifts/:shiftId` | `hrm.shifts.manage` |
| DELETE | `/hrm/setup/shifts/:shiftId` | `hrm.shifts.manage` |

**Holidays** — `hrm.holidays.*` — calendars + holidays + assignment

| Method | Path | Permission |
|---|---|---|
| POST | `/hrm/setup/holiday-calendars/assignments` | `hrm.holidays.manage` |
| GET | `/hrm/setup/holiday-calendars` | `hrm.holidays.view` |
| POST | `/hrm/setup/holiday-calendars` | `hrm.holidays.manage` |
| GET | `/hrm/setup/holiday-calendars/:calendarId` | `hrm.holidays.view` |
| PATCH | `/hrm/setup/holiday-calendars/:calendarId` | `hrm.holidays.manage` |
| DELETE | `/hrm/setup/holiday-calendars/:calendarId` | `hrm.holidays.manage` |
| GET | `/hrm/setup/holiday-calendars/:calendarId/holidays` | `hrm.holidays.view` |
| POST | `/hrm/setup/holiday-calendars/:calendarId/holidays` | `hrm.holidays.manage` |
| PATCH | `/hrm/setup/holiday-calendars/:calendarId/holidays/:holidayId` | `hrm.holidays.manage` |
| DELETE | `/hrm/setup/holiday-calendars/:calendarId/holidays/:holidayId` | `hrm.holidays.manage` |

**Contracts** — `hrm.contracts.*` — employee-scoped, not setup

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/employees/:employeeId/contracts` | `hrm.contracts.view` |
| POST | `/hrm/employees/:employeeId/contracts` | `hrm.contracts.manage` |
| POST | `/hrm/employees/:employeeId/contracts/:contractId/deactivate` | `hrm.contracts.manage` |
| GET | `/hrm/employees/:employeeId/contracts/:contractId` | `hrm.contracts.view` |
| PATCH | `/hrm/employees/:employeeId/contracts/:contractId` | `hrm.contracts.manage` |

### Group B — Employee Lifecycle

**Employees** — `hrm.employees.*` — core CRUD

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/employees` | `hrm.employees.view` |
| POST | `/hrm/employees` | `hrm.employees.create` |
| GET | `/hrm/employees/:empId` | `hrm.employees.view` |
| PATCH | `/hrm/employees/:empId` | `hrm.employees.update` |
| DELETE | `/hrm/employees/:empId` | `hrm.employees.delete` |
| POST | `/hrm/employees/:empId/terminate` | `hrm.employees.terminate` |

**Promotions** — `hrm.promotions.*` — approval-chain gated, implements `HandleApprovalDecision`

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/promotions` | `hrm.promotions.view` (HR org-level list) |
| GET | `/hrm/employees/:employeeId/promotions` | `hrm.promotions.view` |
| POST | `/hrm/employees/:employeeId/promotions` | `hrm.promotions.manage` |
| POST | `/hrm/employees/:employeeId/promotions/:promotionId/submit` | `hrm.promotions.manage` |
| POST | `/hrm/employees/:employeeId/promotions/:promotionId/cancel` | `hrm.promotions.manage` |
| POST | `/hrm/employees/:employeeId/promotions/:promotionId/apply` | `hrm.promotions.apply` |
| GET | `/hrm/employees/:employeeId/promotions/:promotionId` | `hrm.promotions.view` |
| PATCH | `/hrm/employees/:employeeId/promotions/:promotionId` | `hrm.promotions.manage` |

**Transfers** — `hrm.transfers.*` — HR/manager-initiated only, approval-chain gated

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/transfers` | `hrm.transfers.view` (HR org-level list) |
| GET | `/hrm/employees/:employeeId/transfers` | `hrm.transfers.view` |
| POST | `/hrm/employees/:employeeId/transfers` | `hrm.transfers.manage` |
| POST | `/hrm/employees/:employeeId/transfers/:transferId/submit` | `hrm.transfers.manage` |
| POST | `/hrm/employees/:employeeId/transfers/:transferId/cancel` | `hrm.transfers.manage` |
| POST | `/hrm/employees/:employeeId/transfers/:transferId/apply` | `hrm.transfers.apply` |
| GET | `/hrm/employees/:employeeId/transfers/:transferId` | `hrm.transfers.view` |
| PATCH | `/hrm/employees/:employeeId/transfers/:transferId` | `hrm.transfers.manage` |

**Resignations** — `hrm.resignations.*` — notice period pulled from contract

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/resignations` | `hrm.resignations.view` (HR org-level list) |
| GET | `/hrm/employees/:employeeId/resignations` | `hrm.resignations.view` |
| POST | `/hrm/employees/:employeeId/resignations` | `hrm.resignations.manage` (submit) |
| POST | `/hrm/employees/:employeeId/resignations/:resignationId/withdraw` | `hrm.resignations.manage` |
| POST | `/hrm/employees/:employeeId/resignations/:resignationId/accept` | `hrm.resignations.process` |
| POST | `/hrm/employees/:employeeId/resignations/:resignationId/reject` | `hrm.resignations.process` |
| GET | `/hrm/employees/:employeeId/resignations/:resignationId` | `hrm.resignations.view` |
| PATCH | `/hrm/employees/:employeeId/resignations/:resignationId` | `hrm.resignations.process` |

**Terminations** — `hrm.terminations.*` — approval-chain gated, immediate access revocation on decision

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/terminations` | `hrm.terminations.view` (HR org-level list — **no employee-scoped list; intentional, employees can't see their own termination records**) |
| POST | `/hrm/employees/:employeeId/terminations` | `hrm.terminations.manage` |
| POST | `/hrm/employees/:employeeId/terminations/:terminationId/submit` | `hrm.terminations.manage` |
| POST | `/hrm/employees/:employeeId/terminations/:terminationId/cancel` | `hrm.terminations.manage` |
| POST | `/hrm/employees/:employeeId/terminations/:terminationId/apply` | `hrm.terminations.apply` |
| GET | `/hrm/employees/:employeeId/terminations/:terminationId` | `hrm.terminations.view` |
| PATCH | `/hrm/employees/:employeeId/terminations/:terminationId` | `hrm.terminations.manage` |

### Group C — Disciplinary

**Warnings** — `hrm.warnings.*` — status-gated visibility, lazy expiry, approval-chain gated

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/warnings` | `hrm.warnings.view` (HR org-level list) |
| GET | `/hrm/employees/:employeeId/warnings` | `hrm.warnings.view` |
| POST | `/hrm/employees/:employeeId/warnings` | `hrm.warnings.manage` |
| POST | `/hrm/employees/:employeeId/warnings/:warningId/issue` | `hrm.warnings.issue` |
| POST | `/hrm/employees/:employeeId/warnings/:warningId/acknowledge` | `hrm.warnings.acknowledge` |
| POST | `/hrm/employees/:employeeId/warnings/:warningId/appeal` | `hrm.warnings.acknowledge` |
| POST | `/hrm/employees/:employeeId/warnings/:warningId/close` | `hrm.warnings.close` |
| POST | `/hrm/employees/:employeeId/warnings/:warningId/cancel` | `hrm.warnings.close` |
| GET | `/hrm/employees/:employeeId/warnings/:warningId` | `hrm.warnings.view` |
| PATCH | `/hrm/employees/:employeeId/warnings/:warningId` | `hrm.warnings.manage` |

**Complaints** — `hrm.complaints.*` — person-against-person, required subject

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/complaints` | `hrm.complaints.view` (org-level list) |
| GET | `/hrm/employees/:employeeId/complaints` | `hrm.complaints.view` |
| POST | `/hrm/employees/:employeeId/complaints` | `hrm.complaints.manage` |
| POST | `/hrm/employees/:employeeId/complaints/:complaintId/start-review` | `hrm.complaints.process` |
| POST | `/hrm/employees/:employeeId/complaints/:complaintId/assign` | `hrm.complaints.process` |
| POST | `/hrm/employees/:employeeId/complaints/:complaintId/resolve` | `hrm.complaints.process` |
| POST | `/hrm/employees/:employeeId/complaints/:complaintId/dismiss` | `hrm.complaints.process` |
| POST | `/hrm/employees/:employeeId/complaints/:complaintId/withdraw` | `hrm.complaints.manage` |
| GET | `/hrm/employees/:employeeId/complaints/:complaintId` | `hrm.complaints.view` |
| PATCH | `/hrm/employees/:employeeId/complaints/:complaintId` | `hrm.complaints.manage` |

**Employee Documents** — `hrm.documents.*` — bulk send, versioning

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/documents` | `hrm.documents.view` (HR view all) |
| GET | `/hrm/employees/:employeeId/documents` | `hrm.documents.view` |
| POST | `/hrm/employees/:employeeId/documents` | `hrm.documents.manage` |
| POST | `/hrm/employees/:employeeId/documents/:documentId/send` | `hrm.documents.manage` |
| POST | `/hrm/employees/:employeeId/documents/:documentId/acknowledge` | `hrm.documents.acknowledge` |
| POST | `/hrm/employees/:employeeId/documents/:documentId/decline` | `hrm.documents.acknowledge` |
| POST | `/hrm/employees/:employeeId/documents/:documentId/withdraw` | `hrm.documents.manage` |
| GET | `/hrm/employees/:employeeId/documents/:documentId` | `hrm.documents.view` |

**Acknowledgements** — `hrm.acknowledgements.*` — polymorphic target, declined status

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/acknowledgements/entity/:type/:id` | `hrm.acknowledgements.view` |
| POST | `/hrm/acknowledgements/:ackId/acknowledge` | `hrm.acknowledgements.respond` |
| POST | `/hrm/acknowledgements/:ackId/decline` | `hrm.acknowledgements.respond` |
| GET | `/hrm/acknowledgements` | `hrm.acknowledgements.view` |
| POST | `/hrm/acknowledgements` | `hrm.acknowledgements.manage` |
| GET | `/hrm/acknowledgements/:ackId` | `hrm.acknowledgements.view` |

### Group D — Time & Compensation

**Attendance** — `hrm.attendance.*` — multi-punch, 3 sources (webhook + API key), regularization via approval chain, nightly absent cron, period lock

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/attendance/periods` | `hrm.attendance.view` |
| POST | `/hrm/attendance/periods` | `hrm.attendance.manage` |
| POST | `/hrm/attendance/periods/:year/:month/finalize` | `hrm.attendance.finalize` |
| POST | `/hrm/attendance/periods/:year/:month/lock` | `hrm.attendance.finalize` |
| POST | `/hrm/attendance/:recordId/approve` | `hrm.attendance.approve` |
| POST | `/hrm/attendance/:recordId/reject` | `hrm.attendance.approve` |
| POST | `/hrm/attendance/:recordId/regularize` | `hrm.attendance.manage` |
| GET | `/hrm/attendance` | `hrm.attendance.view` |
| POST | `/hrm/attendance` | `hrm.attendance.manage` |
| GET | `/hrm/attendance/:recordId` | `hrm.attendance.view` |

**Payslips / Payroll** — `hrm.payroll.*` — payroll runs, `ComputeSlab` progressive tax formula engine, attendance period must finalize first, immutable once finalized, dispute via acknowledgement decline

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/payroll/runs` | `hrm.payroll.view` |
| POST | `/hrm/payroll/runs` | `hrm.payroll.manage` |
| POST | `/hrm/payroll/runs/:runId/compute` | `hrm.payroll.compute` |
| POST | `/hrm/payroll/runs/:runId/approve` | `hrm.payroll.approve` |
| POST | `/hrm/payroll/runs/:runId/pay` | `hrm.payroll.pay` |
| POST | `/hrm/payroll/runs/:runId/cancel` | `hrm.payroll.manage` |
| GET | `/hrm/payroll/runs/:runId` | `hrm.payroll.view` |
| GET | `/hrm/payroll/payslips` | `hrm.payroll.view` (filter by `run_id` / `employee_id`) |
| GET | `/hrm/payroll/payslips/:payslipId` | `hrm.payroll.view` (with component lines) |

### Group E — Recognition & Communication

**Awards** — `hrm.awards.*` — per-type nomination restriction, optional monetary value, approval-chain gated

| Method | Path | Permission |
|---|---|---|
| POST | `/hrm/awards/:awardId/submit` | `hrm.awards.approve` |
| POST | `/hrm/awards/:awardId/issue` | `hrm.awards.issue` |
| POST | `/hrm/awards/:awardId/cancel` | `hrm.awards.manage` |
| GET | `/hrm/awards` | `hrm.awards.view` |
| POST | `/hrm/awards` | `hrm.awards.manage` |
| GET | `/hrm/awards/:awardId` | `hrm.awards.view` |
| PATCH | `/hrm/awards/:awardId` | `hrm.awards.manage` |

**Announcements** — `hrm.announcements.*` — Markdown, audience targeting, scheduling

| Method | Path | Permission |
|---|---|---|
| POST | `/hrm/announcements/:announcementId/publish` | `hrm.announcements.publish` |
| POST | `/hrm/announcements/:announcementId/schedule` | `hrm.announcements.publish` |
| POST | `/hrm/announcements/:announcementId/archive` | `hrm.announcements.publish` |
| GET | `/hrm/announcements` | `hrm.announcements.view` |
| POST | `/hrm/announcements` | `hrm.announcements.manage` |
| GET | `/hrm/announcements/:announcementId` | `hrm.announcements.view` |
| PATCH | `/hrm/announcements/:announcementId` | `hrm.announcements.manage` |

**HR Calendar** — `hrm.calendar.*` — separate from holiday calendar, simple recurrence, RSVP via acknowledgement

| Method | Path | Permission |
|---|---|---|
| POST | `/hrm/calendar/:eventId/cancel` | `hrm.calendar.manage` |
| POST | `/hrm/calendar/:eventId/rsvp` | `hrm.calendar.manage` |
| GET | `/hrm/calendar` | `hrm.calendar.view` |
| POST | `/hrm/calendar` | `hrm.calendar.manage` |
| GET | `/hrm/calendar/:eventId` | `hrm.calendar.view` |
| PATCH | `/hrm/calendar/:eventId` | `hrm.calendar.manage` |

**Milestones** — `hrm.milestones.*` — nightly cron, configurable rules, anniversary `year_intervals`, auto-drafts awards/announcements

| Method | Path | Permission |
|---|---|---|
| POST | `/hrm/milestones/generate` | `hrm.milestones.generate` |
| POST | `/hrm/milestones/:milestoneId/acknowledge` | `hrm.milestones.manage` |
| GET | `/hrm/milestones` | `hrm.milestones.view` |
| POST | `/hrm/milestones` | `hrm.milestones.manage` |
| GET | `/hrm/milestones/:milestoneId` | `hrm.milestones.view` |

### Reports

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/reports/overview` | `hrm.reports.view` |
| GET | `/hrm/reports/headcount` | `hrm.reports.view` |
| GET | `/hrm/reports/leave-summary` | `hrm.reports.view` |

### Leave (standalone — not in Groups A–E, part of original Phase-1-of-HRM scope)

| Method | Path | Permission |
|---|---|---|
| GET | `/hrm/leave/types` | `hrm.leave.view` |
| POST | `/hrm/leave/types` | `hrm.leave.create` |
| GET | `/hrm/leave/types/:typeId` | `hrm.leave.view` |
| PATCH | `/hrm/leave/types/:typeId` | `hrm.leave.update` |
| DELETE | `/hrm/leave/types/:typeId` | `hrm.leave.delete` |
| GET | `/hrm/leave/requests` | `hrm.leave.view` |
| POST | `/hrm/leave/requests` | `hrm.leave.request` |
| POST | `/hrm/leave/requests/:reqId/approve` | `hrm.leave.approve` |
| POST | `/hrm/leave/requests/:reqId/reject` | `hrm.leave.approve` |
| POST | `/hrm/leave/requests/:reqId/cancel` | `hrm.leave.request` |
| GET | `/hrm/leave/requests/:reqId` | `hrm.leave.view` |
| DELETE | `/hrm/leave/requests/:reqId` | `hrm.leave.delete` |

## Frontend pages

```
frontend/src/app/(dashboard)/[orgId]/hrm/
├── departments/page.tsx
├── positions/page.tsx
├── employees/                    → employee list + detail
├── leave/page.tsx
├── attendance/page.tsx
├── payroll/page.tsx              → payslips, payroll runs
├── lifecycle/page.tsx            → promotions, transfers, resignations, terminations
├── warnings/page.tsx             → ApprovalInstanceView wired in
├── compliance/page.tsx           → complaints, employee documents, acknowledgements
├── recognition/page.tsx          → awards, announcements — ApprovalInstanceView wired in
├── reports/page.tsx
└── setup/
    ├── approvals/page.tsx        → approval chain templates
    ├── document-templates/page.tsx
    ├── holidays/page.tsx
    ├── salary/page.tsx
    ├── shifts/page.tsx
    └── warning-types/page.tsx
```

Components live in `frontend/src/components/hrm/{acknowledgements→ via compliance,approvals,attendance,compliance,departments,doctemplates,employees,holidays,leave,lifecycle,payroll,positions,recognition,salary,shifts,warnings,warningtypes}/`, with `ApprovalInstanceView.tsx` living under `components/hrm/approvals/`.

API helpers live in `frontend/src/lib/hrm/{approvals,attendance,compliance,departments,doctemplates,employees,holidays,leave,lifecycle,payroll,positions,recognition,reports,salary,shifts,warnings,warningtypes}.ts` and types in `frontend/src/types/hrm.ts`.

**Lifecycle page's ApprovalInstanceView wiring:** the `lifecycle`, `warnings`, and `recognition` pages each check for a non-null `approval_instance_id` on the relevant record and, when present, render the shared `ApprovalInstanceView` drawer with approve/reject actions bound to `/hrm/setup/approvals/instances/:instanceId/{approve,reject}`.

## Notable business logic

- **Approval chains (Phase 7.7):** a callback-registry pattern on the approvals service. Five modules — terminations, promotions, transfers, warnings, awards — each implement `HandleApprovalDecision` and register with `hrmApprovalsSvc.RegisterCallback(entityType, fn)` in `main.go`. When an approval instance resolves, the approvals service invokes the registered callback for that entity type rather than the source module calling back into approvals directly — avoids import cycles. See [ADR-0016](../decisions/0016-hrm-approval-chains.md).
- **Payslip formula engine:** salary components use `expr-lang/expr` for arbitrary formulas; income tax uses `ComputeSlab` for progressive bracket calculation. Payslips are immutable once a payroll run is finalized; disputes go through the acknowledgement-decline flow rather than a direct edit. See [ADR-0017](../decisions/0017-hrm-payslip-engine.md).
- **Attendance sources:** three ingestion paths (manual, webhook, API key), a nightly cron job marks unrecorded days absent, and a period must be finalized (then optionally locked) before payroll can run against it. See [ADR-0018](../decisions/0018-hrm-attendance-sources.md).
- **Document templates:** Markdown source, rendered to PDF in-browser rather than server-side. See [ADR-0019](../decisions/0019-hrm-document-templates.md).
- **Cross-module writes:** acknowledgements are written to from other HRM modules (e.g. warnings, documents) via `ON CONFLICT DO NOTHING` and direct `pgPool.Exec`, not through the acknowledgements service interface — same import-cycle-avoidance pattern used elsewhere in the codebase (repository interfaces never expose external package types like `pgx.Tx`).
- **Routing gotcha, applied throughout:** every sub-module with both a static segment (e.g. `/leave/requests`, `/reorder`, `/escalations`, `/assignments`) and a parameterized one (`/:reqId`, `/:typeId`) registers the static route first — this bites in Fiber v3 if reversed. Comments in the source consistently call this out.

## Backend architecture (for reference)

```
internal/hrm/
├── departments/ positions/ employees/            ← org structure
├── salary/ approvals/ warningtypes/ doctemplates/
├── shifts/ holidays/ contracts/                  ← Group A setup
├── promotions/ transfers/ resignations/
├── terminations/                                 ← Group B lifecycle
├── warnings/ complaints/ employeedocs/
├── acknowledgements/                              ← Group C disciplinary
├── attendance/ payslips/                          ← Group D time & comp
├── awards/ announcements/ calendar/ milestones/   ← Group E recognition
├── leave/                                         ← standalone, pre-dates Groups A–E
└── reports/                                       ← cross-cutting HRM analytics
```

Each sub-module follows the same handler → service → repository layering as the rest of the codebase (Section 4 of the Master Instruction), with its own `routes.go` using the `PermissionFunc` factory pattern to avoid an import cycle with `middleware`.

## Known open items

- Missing `GET /hrm/setup/approvals/instances` list endpoint (documented separately as a standalone backend issue) — the only unresolved backend gap found during the r9 codebase audit.

## Related ADRs

- [ADR-0014](../decisions/0014-hrm-extended-architecture.md) — HRM extended architecture (Groups A–E)
- [ADR-0015](../decisions/0015-hrm-formula-engine.md) — salary formula engine
- [ADR-0016](../decisions/0016-hrm-approval-chains.md) — approval chains
- [ADR-0017](../decisions/0017-hrm-payslip-engine.md) — payslip engine
- [ADR-0018](../decisions/0018-hrm-attendance-sources.md) — attendance sources
- [ADR-0019](../decisions/0019-hrm-document-templates.md) — document templates
