# Entity Relationship Diagram

## High-level relationship flow

```text
users
  ├── auth_accounts
  ├── sessions
  ├── verification_tokens
  ├── organization_members
  └── audit_logs

organizations
  ├── organization_members
  ├── roles
  ├── sessions
  ├── subscriptions
  ├── organization_usage
  └── audit_logs

roles
  └── organization_members

subscriptions
  └── organization_usage

organizations
  └── tasks

tasks
  ├── users (created_by)
  └── users (assigned_to)
```

## Mermaid ERD

```mermaid
erDiagram
    users ||--o{ auth_accounts : "has linked providers"
    users ||--o{ sessions : "has sessions"
    users ||--o{ verification_tokens : "has tokens"
    users ||--o{ organization_members : "joins organizations"
    users ||--o{ audit_logs : "performs actions"
    users ||--o{ organization_members : "invites"

    organizations ||--o{ organization_members : "has members"
    organizations ||--o{ roles : "has custom roles"
    organizations ||--o{ sessions : "session context"
    organizations ||--o{ subscriptions : "has subscription"
    organizations ||--o{ organization_usage : "has usage"
    organizations ||--o{ audit_logs : "has audit events"

    roles ||--o{ organization_members : "assigned to members"
    subscriptions ||--o{ organization_usage : "measured by usage"
    organizations ||--o{ tasks : "has tasks"
    users ||--o{ tasks : "created tasks"
    users ||--o{ tasks : "assigned tasks"

    users {
        uuid id PK
        text public_id UK
        text email UK
        text username UK
        text status
        timestamptz created_at
        timestamptz deleted_at
    }

    organizations {
        uuid id PK
        text public_id UK
        text name
        text slug UK
        text status
        timestamptz created_at
        timestamptz deleted_at
    }

    permissions {
        uuid id PK
        text public_id UK
        text key UK
        text resource
        text action
    }

    roles {
        uuid id PK
        text public_id UK
        uuid org_id FK
        text name
        text_array permissions
        boolean is_system
    }

    organization_members {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid user_id FK
        uuid role_id FK
        text role_key
        text status
        text invitation_status
    }

    auth_accounts {
        uuid id PK
        text public_id UK
        uuid user_id FK
        text provider
        text provider_account_id
        text provider_type
    }

    sessions {
        uuid id PK
        text public_id UK
        uuid user_id FK
        uuid org_id FK
        text token_hash UK
        timestamptz expires_at
        timestamptz revoked_at
    }

    verification_tokens {
        uuid id PK
        text public_id UK
        uuid user_id FK
        text email
        text token_hash UK
        text type
        timestamptz expires_at
    }

    subscriptions {
        uuid id PK
        text public_id UK
        uuid org_id FK
        text plan
        text status
        text payment_provider
        text payment_subscription_id
    }

    organization_usage {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid subscription_id FK
        timestamptz period_start
        timestamptz period_end
        jsonb limits
        jsonb used
    }

    tasks {
        uuid id PK
        text public_id UK
        uuid org_id FK
        text title
        text description
        text status
        timestamptz due_date
        uuid created_by FK
        uuid assigned_to FK
        timestamptz created_at
        timestamptz updated_at
    }

    audit_logs {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid user_id FK
        text event_type
        text resource_type
        text resource_id
        jsonb changes
    }
```

## Important modeling notes

`permissions` does not currently have a physical many-to-many join table with `roles`. The `roles.permissions` field stores permission keys as `TEXT[]`. This is acceptable for an early SaaS phase because it keeps RBAC simple, but the application must validate all role permission keys against `permissions.key`.

For a larger enterprise-grade version, add a normalized `role_permissions` table.

---

## HRM (added r9)

40 tables — too many to put in one readable Mermaid diagram with full column lists (that level of detail lives in `data-dictionary.md`). Split by group instead, showing entities and relationships only. Every HRM table also has `organizations ||--o{ <table>` for tenant scoping, omitted below for readability.

### High-level relationship flow

```text
hrm_departments
  ├── hrm_positions
  ├── hrm_employees (department_id)
  └── hrm_departments (parent_department_id, self-referencing)

hrm_employees
  ├── hrm_leave_requests
  ├── hrm_employee_salary_records
  ├── hrm_employee_contracts
  ├── hrm_promotions / hrm_transfers / hrm_resignations / hrm_terminations
  ├── hrm_employee_warnings
  ├── hrm_complaints (as complainant, and optionally as against_employee_id)
  ├── hrm_employee_documents
  ├── hrm_acknowledgements
  ├── hrm_attendance_records
  ├── hrm_payslips
  ├── hrm_awards
  └── hrm_employee_milestones

hrm_approval_templates
  ├── hrm_approval_template_levels
  └── hrm_approval_instances
        └── hrm_approval_decisions

hrm_approval_instances (polymorphic entity_type/entity_id, no FK)
  ← referenced by: hrm_promotions, hrm_transfers, hrm_resignations,
    hrm_terminations, hrm_employee_warnings, hrm_awards,
    hrm_attendance_records (regularization)

hrm_salary_components
  └── hrm_salary_structures (via hrm_salary_structure_components)
        ├── hrm_employee_salary_records
        └── hrm_payslips

hrm_attendance_periods
  ├── hrm_attendance_records (by date range, no direct FK)
  └── hrm_payslip_runs (optional link)

hrm_payslip_runs
  └── hrm_payslips
        └── hrm_payslip_lines

hrm_holiday_calendars
  ├── hrm_holidays
  └── hrm_calendar_assignments

hrm_document_templates
  ├── hrm_employee_documents
  └── hrm_document_bulk_sends

hrm_employee_milestones
  ├── hrm_awards (auto_award_id)
  ├── hrm_announcements (auto_announcement_id)
  └── hrm_calendar_events (auto_calendar_event_id)

hrm_acknowledgements (polymorphic acknowledgeable_type/acknowledgeable_id, no FK)
  ← referenced by: hrm_employee_warnings (via type), hrm_employee_documents,
    hrm_announcements, hrm_calendar_events
```

### Mermaid ERD — org structure & lifecycle (Groups A core + B)

```mermaid
erDiagram
    hrm_departments ||--o{ hrm_positions : "has"
    hrm_departments ||--o{ hrm_departments : "parent of"
    hrm_departments ||--o{ hrm_employees : "employs"
    hrm_positions ||--o{ hrm_employees : "holds"
    hrm_employees ||--o{ hrm_employees : "manages"
    hrm_employees ||--o{ hrm_employee_contracts : "has"
    hrm_employees ||--o{ hrm_promotions : "promoted"
    hrm_employees ||--o{ hrm_transfers : "transferred"
    hrm_employees ||--o{ hrm_resignations : "resigns"
    hrm_employees ||--o{ hrm_terminations : "terminated"
    hrm_approval_instances ||--o| hrm_promotions : "gates"
    hrm_approval_instances ||--o| hrm_transfers : "gates"
    hrm_approval_instances ||--o| hrm_resignations : "gates"
    hrm_approval_instances ||--o| hrm_terminations : "gates"

    hrm_departments {
        uuid id PK
        uuid org_id FK
        uuid parent_department_id FK
        uuid head_employee_id
        text name
        boolean is_active
    }
    hrm_positions {
        uuid id PK
        uuid department_id FK
        text title
    }
    hrm_employees {
        uuid id PK
        uuid user_id FK
        uuid department_id FK
        uuid position_id FK
        uuid manager_id FK
        text employee_number
        text status
        date hire_date
    }
    hrm_employee_contracts {
        uuid id PK
        uuid employee_id FK
        text contract_type
        date start_date
        date end_date
        int notice_period_days
    }
    hrm_promotions {
        uuid id PK
        uuid employee_id FK
        uuid to_position_id FK
        uuid approval_instance_id FK
        text status
    }
    hrm_transfers {
        uuid id PK
        uuid employee_id FK
        text transfer_type
        uuid approval_instance_id FK
        text status
    }
    hrm_resignations {
        uuid id PK
        uuid employee_id FK
        date last_working_date
        uuid approval_instance_id FK
        text status
    }
    hrm_terminations {
        uuid id PK
        uuid employee_id FK
        text termination_type
        uuid approval_instance_id FK
        text status
    }
```

### Mermaid ERD — approval chains, disciplinary & documents (Group C + approval engine)

```mermaid
erDiagram
    hrm_approval_templates ||--o{ hrm_approval_template_levels : "defines"
    hrm_approval_templates ||--o{ hrm_approval_instances : "instantiates"
    hrm_approval_instances ||--o{ hrm_approval_decisions : "records"
    hrm_employees ||--o{ hrm_employee_warnings : "receives"
    hrm_warning_types ||--o{ hrm_employee_warnings : "categorizes"
    hrm_warning_types ||--o{ hrm_warning_escalation_rules : "triggers"
    hrm_employees ||--o{ hrm_complaints : "files"
    hrm_employees ||--o{ hrm_complaints : "named against"
    hrm_document_templates ||--o{ hrm_employee_documents : "generates"
    hrm_document_templates ||--o{ hrm_document_bulk_sends : "sent via"
    hrm_employees ||--o{ hrm_employee_documents : "owns"
    hrm_employees ||--o{ hrm_acknowledgements : "acknowledges"

    hrm_approval_templates {
        uuid id PK
        text action_type
        boolean is_default
    }
    hrm_approval_template_levels {
        uuid id PK
        uuid template_id FK
        int level
        text approver_type
    }
    hrm_approval_instances {
        uuid id PK
        uuid template_id FK
        text entity_type
        uuid entity_id
        text overall_status
    }
    hrm_approval_decisions {
        uuid id PK
        uuid instance_id FK
        int level
        uuid approver_id FK
        text action
    }
    hrm_warning_types {
        uuid id PK
        text name
        int severity_level
    }
    hrm_employee_warnings {
        uuid id PK
        uuid employee_id FK
        uuid warning_type_id FK
        uuid approval_instance_id FK
        text status
    }
    hrm_complaints {
        uuid id PK
        uuid employee_id FK
        uuid against_employee_id FK
        text complaint_type
        text status
    }
    hrm_document_templates {
        uuid id PK
        text document_type
    }
    hrm_employee_documents {
        uuid id PK
        uuid employee_id FK
        uuid template_id FK
        text document_type
        text status
    }
    hrm_acknowledgements {
        uuid id PK
        uuid employee_id FK
        text acknowledgeable_type
        uuid acknowledgeable_id
        text status
    }
```

### Mermaid ERD — time, compensation & recognition (Groups D + E)

```mermaid
erDiagram
    hrm_salary_components ||--o{ hrm_salary_structure_components : "used in"
    hrm_salary_structures ||--o{ hrm_salary_structure_components : "composed of"
    hrm_salary_structures ||--o{ hrm_employee_salary_records : "assigned as"
    hrm_employees ||--o{ hrm_employee_salary_records : "has"
    hrm_employees ||--o{ hrm_attendance_records : "punches"
    hrm_attendance_periods ||--o{ hrm_payslip_runs : "feeds"
    hrm_payslip_runs ||--o{ hrm_payslips : "produces"
    hrm_payslips ||--o{ hrm_payslip_lines : "itemized as"
    hrm_salary_components ||--o{ hrm_payslip_lines : "computes"
    hrm_employees ||--o{ hrm_awards : "receives"
    hrm_employees ||--o{ hrm_employee_milestones : "reaches"
    hrm_employee_milestones ||--o| hrm_awards : "auto-drafts"
    hrm_employee_milestones ||--o| hrm_announcements : "auto-drafts"
    hrm_employee_milestones ||--o| hrm_calendar_events : "auto-drafts"

    hrm_salary_components {
        uuid id PK
        text component_type
        text calc_method
    }
    hrm_salary_structures {
        uuid id PK
        text name
    }
    hrm_employee_salary_records {
        uuid id PK
        uuid employee_id FK
        uuid structure_id FK
        numeric basic_pay
        date effective_date
    }
    hrm_attendance_periods {
        uuid id PK
        int period_year
        int period_month
        text status
    }
    hrm_attendance_records {
        uuid id PK
        uuid employee_id FK
        date attendance_date
        text day_type
        text status
    }
    hrm_payslip_runs {
        uuid id PK
        uuid attendance_period_id FK
        text status
    }
    hrm_payslips {
        uuid id PK
        uuid payslip_run_id FK
        uuid employee_id FK
        numeric net_pay
        text status
    }
    hrm_payslip_lines {
        uuid id PK
        uuid payslip_id FK
        uuid component_id FK
        numeric computed_amount
    }
    hrm_awards {
        uuid id PK
        uuid employee_id FK
        text award_type
        text status
    }
    hrm_employee_milestones {
        uuid id PK
        uuid employee_id FK
        text milestone_type
        uuid auto_award_id FK
        uuid auto_announcement_id FK
        uuid auto_calendar_event_id FK
    }
```

### Notes specific to HRM's modeling

- **Polymorphic tables have no physical FK** for their target — `hrm_approval_instances.entity_id`, `hrm_acknowledgements.acknowledgeable_id`, `hrm_employee_documents.related_id`, and the `assignee_id` columns in `hrm_work_schedule_assignments`/`hrm_calendar_assignments` all rely on application-level integrity, matched against a sibling `_type` column. Same trade-off as `roles.permissions` above — simpler schema, but the application must validate consistency.
- **Snapshot columns are intentional duplication, not denormalization debt.** Several tables (e.g. `hrm_payslip_lines.component_name`, `hrm_employee_warnings.warning_type_name`, `hrm_promotions.from_basic_pay`) copy a value at write time specifically so that later edits to the source record don't rewrite history. Don't "clean these up" — removing them would let editing a salary component retroactively change what a historical payslip claims it paid.
- **Two tables named `..._assignments` exist for unrelated things:** `hrm_work_schedule_assignments` (which shift an employee/dept/org uses) and `hrm_calendar_assignments` (which holiday calendar an employee/dept/org uses). Both use the same `assignee_type`/`assignee_id` polymorphic pattern but are otherwise independent.
