# Data Dictionary

This file combines all table-level column dictionaries. For deeper business explanation, see `tables/*.md`.

## users

Stores registered BusinessSAAS user accounts, profile data, login/security state, and user-level preferences.

| Column             | Type        | Required | Default                  | Business meaning                                                           | Sensitivity           |
| ------------------ | ----------- | -------- | ------------------------ | -------------------------------------------------------------------------- | --------------------- |
| id                 | UUID        | Yes      | gen_random_uuid()        | Internal primary key for joins and foreign keys.                           | Internal              |
| public_id          | TEXT        | Yes      | 'usr\_' + generated UUID | Stable API-facing user identifier.                                         | Public identifier     |
| email              | TEXT        | No       | NULL                     | Login/contact email; nullable for OAuth-only or incomplete provider data.  | PII                   |
| password_hash      | TEXT        | No       | NULL                     | Hashed password for credential login.                                      | Critical secret       |
| username           | TEXT        | No       | NULL                     | Optional unique handle for the user.                                       | PII/public            |
| display_name       | TEXT        | Yes      | ''                       | Name shown in the UI.                                                      | PII                   |
| first_name         | TEXT        | Yes      | ''                       | User first/given name.                                                     | PII                   |
| last_name          | TEXT        | Yes      | ''                       | User last/family name.                                                     | PII                   |
| full_name          | TEXT        | Yes      | ''                       | Full name for display/search/export.                                       | PII                   |
| photo_url          | TEXT        | No       | NULL                     | Profile image URL.                                                         | PII                   |
| cover_photo_url    | TEXT        | No       | NULL                     | Profile cover image URL.                                                   | PII                   |
| phone              | TEXT        | No       | NULL                     | User phone number.                                                         | PII                   |
| phone_verified     | BOOLEAN     | Yes      | false                    | Whether phone number ownership is verified.                                | Security/PII          |
| email_verified     | BOOLEAN     | Yes      | false                    | Whether email ownership is verified.                                       | Security              |
| email_verified_at  | TIMESTAMPTZ | No       | NULL                     | Timestamp when email verification completed.                               | Security              |
| country            | CHAR(2)     | No       | NULL                     | ISO country code for localization/compliance.                              | Location/PII          |
| timezone           | TEXT        | Yes      | 'UTC'                    | Preferred timezone for displaying dates.                                   | Preference            |
| locale             | TEXT        | Yes      | 'en'                     | Preferred locale for formatting.                                           | Preference            |
| language           | TEXT        | Yes      | 'en'                     | Preferred language.                                                        | Preference            |
| currency           | TEXT        | Yes      | 'USD'                    | Preferred/default currency.                                                | Preference            |
| status             | TEXT        | Yes      | 'active'                 | Account lifecycle state: active, suspended, deleted, pending_verification. | Operational           |
| account_type       | TEXT        | Yes      | 'saas_customer'          | Classifies account type for future product/business logic.                 | Operational           |
| suspended_at       | TIMESTAMPTZ | No       | NULL                     | When the account was suspended.                                            | Security              |
| suspension_reason  | TEXT        | No       | NULL                     | Reason for suspension.                                                     | Sensitive operational |
| login_redirect_url | TEXT        | Yes      | '/dashboard'             | Default post-login destination.                                            | Operational           |
| shortcuts          | TEXT[]      | Yes      | empty array              | Fuse app shortcut identifiers.                                             | Preference            |
| settings           | JSONB       | Yes      | {}                       | Fuse-compatible UI/user settings.                                          | Preference            |
| preferences        | JSONB       | Yes      | {}                       | Product preferences.                                                       | Preference            |
| onboarding         | JSONB       | Yes      | {}                       | Onboarding progress and completed steps.                                   | Operational           |
| feature_flags      | JSONB       | Yes      | {}                       | Per-user feature flag overrides.                                           | Operational           |
| two_fa_enabled     | BOOLEAN     | Yes      | false                    | Whether two-factor authentication is enabled.                              | Security              |
| two_fa_secret      | TEXT        | No       | NULL                     | 2FA secret; must be encrypted at rest.                                     | Critical secret       |
| backup_codes       | JSONB       | Yes      | []                       | Hashed/encrypted backup codes for 2FA recovery.                            | Critical secret       |
| last_login_at      | TIMESTAMPTZ | No       | NULL                     | Last successful login time.                                                | Security              |
| last_activity_at   | TIMESTAMPTZ | No       | NULL                     | Last meaningful product activity time.                                     | Operational           |
| failed_logins      | INTEGER     | Yes      | 0                        | Consecutive failed login counter for lockout.                              | Security              |
| locked_until       | TIMESTAMPTZ | No       | NULL                     | Temporary account lock expiry.                                             | Security              |
| created_at         | TIMESTAMPTZ | Yes      | now()                    | Record creation time.                                                      | Operational           |
| updated_at         | TIMESTAMPTZ | Yes      | now()                    | Record update time.                                                        | Operational           |
| deleted_at         | TIMESTAMPTZ | No       | NULL                     | Soft delete timestamp.                                                     | Operational           |

## organizations

Stores SaaS tenant/workspace/company records.

| Column     | Type        | Required | Default                  | Business meaning                              | Sensitivity        |
| ---------- | ----------- | -------- | ------------------------ | --------------------------------------------- | ------------------ |
| id         | UUID        | Yes      | gen_random_uuid()        | Internal organization primary key.            | Internal           |
| public_id  | TEXT        | Yes      | 'org\_' + generated UUID | API-facing organization identifier.           | Public identifier  |
| name       | TEXT        | Yes      | none                     | Organization display name.                    | Business data      |
| slug       | TEXT        | Yes      | none                     | Unique workspace slug used in URLs.           | Public/business    |
| legal_name | TEXT        | No       | NULL                     | Official legal business name.                 | Business sensitive |
| type       | TEXT        | No       | NULL                     | Organization type/category.                   | Business data      |
| industry   | TEXT        | No       | NULL                     | Industry classification.                      | Business data      |
| website    | TEXT        | No       | NULL                     | Company website.                              | Public/business    |
| logo_url   | TEXT        | No       | NULL                     | Organization logo URL.                        | Public/business    |
| country    | CHAR(2)     | No       | NULL                     | Organization country code.                    | Business/location  |
| timezone   | TEXT        | Yes      | 'UTC'                    | Default organization timezone.                | Operational        |
| currency   | TEXT        | Yes      | 'USD'                    | Default billing/display currency.             | Operational        |
| status     | TEXT        | Yes      | 'active'                 | Tenant lifecycle: active, suspended, deleted. | Operational        |
| metadata   | JSONB       | Yes      | {}                       | Flexible organization-level metadata.         | Varies             |
| created_at | TIMESTAMPTZ | Yes      | now()                    | Record creation time.                         | Operational        |
| updated_at | TIMESTAMPTZ | Yes      | now()                    | Record update time.                           | Operational        |
| deleted_at | TIMESTAMPTZ | No       | NULL                     | Soft delete timestamp.                        | Operational        |

## permissions

Stores canonical permission keys used by roles and authorization checks.

| Column      | Type        | Required | Default                   | Business meaning                          | Sensitivity       |
| ----------- | ----------- | -------- | ------------------------- | ----------------------------------------- | ----------------- |
| id          | UUID        | Yes      | gen_random_uuid()         | Internal permission primary key.          | Internal          |
| public_id   | TEXT        | Yes      | 'perm\_' + generated UUID | API-facing permission identifier.         | Public identifier |
| key         | TEXT        | Yes      | none                      | Dot-format permission key.                | Operational       |
| resource    | TEXT        | Yes      | none                      | Resource/domain controlled by permission. | Operational       |
| action      | TEXT        | Yes      | none                      | Action allowed on the resource.           | Operational       |
| description | TEXT        | No       | NULL                      | Human-readable explanation.               | Operational       |
| is_system   | BOOLEAN     | Yes      | true                      | Marks platform-defined permission.        | Operational       |
| created_at  | TIMESTAMPTZ | Yes      | now()                     | Record creation time.                     | Operational       |
| updated_at  | TIMESTAMPTZ | Yes      | now()                     | Record update time.                       | Operational       |

## roles

Stores system role templates and organization-specific custom roles.

| Column      | Type        | Required | Default                   | Business meaning                             | Sensitivity            |
| ----------- | ----------- | -------- | ------------------------- | -------------------------------------------- | ---------------------- |
| id          | UUID        | Yes      | gen_random_uuid()         | Internal role primary key.                   | Internal               |
| public_id   | TEXT        | Yes      | 'role\_' + generated UUID | API-facing role identifier.                  | Public identifier      |
| org_id      | UUID        | No       | NULL                      | Tenant owner; NULL for global templates.     | Internal/business      |
| name        | TEXT        | Yes      | none                      | Role name such as owner/admin/member.        | Operational            |
| description | TEXT        | No       | NULL                      | Human-readable role explanation.             | Operational            |
| permissions | TEXT[]      | Yes      | empty array               | Permission keys included in this role.       | Security/authorization |
| is_system   | BOOLEAN     | Yes      | false                     | Whether this is a platform-defined role.     | Operational            |
| is_custom   | BOOLEAN     | Yes      | false                     | Whether tenant created/customized this role. | Operational            |
| created_at  | TIMESTAMPTZ | Yes      | now()                     | Record creation time.                        | Operational            |
| updated_at  | TIMESTAMPTZ | Yes      | now()                     | Record update time.                          | Operational            |

## organization_members

Connects users to organizations and stores organization-specific access, role, title, department, and invitation state.

| Column                 | Type        | Required | Default                  | Business meaning                                        | Sensitivity            |
| ---------------------- | ----------- | -------- | ------------------------ | ------------------------------------------------------- | ---------------------- |
| id                     | UUID        | Yes      | gen_random_uuid()        | Internal membership primary key.                        | Internal               |
| public_id              | TEXT        | Yes      | 'mem\_' + generated UUID | API-facing membership identifier.                       | Public identifier      |
| org_id                 | UUID        | Yes      | none                     | Organization this member belongs to.                    | Internal               |
| user_id                | UUID        | Yes      | none                     | User who is a member.                                   | Internal               |
| role_id                | UUID        | No       | NULL                     | Optional FK to roles table.                             | Internal/authorization |
| role_key               | TEXT        | Yes      | 'member'                 | Role snapshot used by API/session.                      | Authorization          |
| title                  | TEXT        | No       | NULL                     | Job title inside organization.                          | PII/business           |
| department             | TEXT        | No       | NULL                     | Department/team inside organization.                    | Business data          |
| status                 | TEXT        | Yes      | 'active'                 | Membership status: active, inactive, suspended.         | Authorization          |
| custom_permissions     | TEXT[]      | Yes      | empty array              | Extra permission keys directly granted to member.       | Authorization/security |
| invitation_status      | TEXT        | Yes      | 'accepted'               | Invitation state: pending, accepted, rejected, expired. | Operational            |
| invited_by             | UUID        | No       | NULL                     | User who invited this member.                           | Audit/PII              |
| invitation_sent_at     | TIMESTAMPTZ | No       | NULL                     | When invitation was sent.                               | Operational            |
| invitation_accepted_at | TIMESTAMPTZ | No       | NULL                     | When invitation was accepted.                           | Operational            |
| joined_at              | TIMESTAMPTZ | Yes      | now()                    | When user joined organization.                          | Operational            |
| created_at             | TIMESTAMPTZ | Yes      | now()                    | Record creation time.                                   | Operational            |
| updated_at             | TIMESTAMPTZ | Yes      | now()                    | Record update time.                                     | Operational            |

## auth_accounts

Stores linked authentication provider accounts such as credentials, Google, Facebook, GitHub, OIDC, email, and WebAuthn.

| Column              | Type        | Required | Default                   | Business meaning                                          | Sensitivity          |
| ------------------- | ----------- | -------- | ------------------------- | --------------------------------------------------------- | -------------------- |
| id                  | UUID        | Yes      | gen_random_uuid()         | Internal account link primary key.                        | Internal             |
| public_id           | TEXT        | Yes      | 'acct\_' + generated UUID | API-facing account-link identifier.                       | Public identifier    |
| user_id             | UUID        | Yes      | none                      | Internal user this provider account belongs to.           | Internal             |
| provider            | TEXT        | Yes      | none                      | Provider id such as credentials/google/facebook.          | Operational          |
| provider_account_id | TEXT        | Yes      | none                      | Unique account id received from provider.                 | Sensitive identifier |
| provider_type       | TEXT        | Yes      | 'oauth'                   | Provider type: oauth, oidc, credentials, email, webauthn. | Operational          |
| access_token        | TEXT        | No       | NULL                      | OAuth access token.                                       | Critical secret      |
| refresh_token       | TEXT        | No       | NULL                      | OAuth refresh token.                                      | Critical secret      |
| id_token            | TEXT        | No       | NULL                      | OIDC ID token.                                            | Critical secret      |
| token_type          | TEXT        | No       | NULL                      | OAuth token type.                                         | Operational          |
| scope               | TEXT        | No       | NULL                      | Granted provider scopes.                                  | Security             |
| expires_at          | TIMESTAMPTZ | No       | NULL                      | Provider token expiry time.                               | Security             |
| connected_at        | TIMESTAMPTZ | Yes      | now()                     | When provider was linked.                                 | Operational          |
| last_used_at        | TIMESTAMPTZ | No       | NULL                      | Last time this provider was used.                         | Security             |
| created_at          | TIMESTAMPTZ | Yes      | now()                     | Record creation time.                                     | Operational          |
| updated_at          | TIMESTAMPTZ | Yes      | now()                     | Record update time.                                       | Operational          |

## sessions

Stores active and historical user sessions/devices with revocation support.

| Column           | Type        | Required | Default                   | Business meaning                                | Sensitivity       |
| ---------------- | ----------- | -------- | ------------------------- | ----------------------------------------------- | ----------------- |
| id               | UUID        | Yes      | gen_random_uuid()         | Internal session primary key.                   | Internal          |
| public_id        | TEXT        | Yes      | 'sess\_' + generated UUID | API-facing session identifier.                  | Public identifier |
| user_id          | UUID        | Yes      | none                      | User who owns the session.                      | Internal          |
| org_id           | UUID        | No       | NULL                      | Organization context for session if applicable. | Internal          |
| token_hash       | TEXT        | Yes      | none                      | Hash of refresh/session token.                  | Critical secret   |
| device_name      | TEXT        | No       | NULL                      | Human-friendly device name.                     | Device data       |
| device_type      | TEXT        | No       | NULL                      | Desktop/mobile/tablet/etc.                      | Device data       |
| browser          | TEXT        | No       | NULL                      | Browser name/version.                           | Device data       |
| os               | TEXT        | No       | NULL                      | Operating system.                               | Device data       |
| user_agent       | TEXT        | No       | NULL                      | Full user-agent string.                         | Device/PII-like   |
| ip_address       | INET        | No       | NULL                      | IP address at session creation/activity.        | PII/security      |
| country          | TEXT        | No       | NULL                      | GeoIP country.                                  | Location/PII      |
| city             | TEXT        | No       | NULL                      | GeoIP city.                                     | Location/PII      |
| region           | TEXT        | No       | NULL                      | GeoIP region.                                   | Location/PII      |
| last_activity_at | TIMESTAMPTZ | Yes      | now()                     | Last activity timestamp.                        | Security          |
| created_at       | TIMESTAMPTZ | Yes      | now()                     | Session creation time.                          | Operational       |
| expires_at       | TIMESTAMPTZ | Yes      | none                      | Session expiry timestamp.                       | Security          |
| revoked_at       | TIMESTAMPTZ | No       | NULL                      | When user/admin revoked the session.            | Security          |

## verification_tokens

Stores one-time email verification, password reset, magic link, invitation, and 2FA tokens.

| Column      | Type        | Required | Default                 | Business meaning                         | Sensitivity       |
| ----------- | ----------- | -------- | ----------------------- | ---------------------------------------- | ----------------- |
| id          | UUID        | Yes      | gen_random_uuid()       | Internal verification token primary key. | Internal          |
| public_id   | TEXT        | Yes      | 'vt\_' + generated UUID | API-facing token record id.              | Public identifier |
| user_id     | UUID        | No       | NULL                    | Related user if known.                   | Internal          |
| email       | TEXT        | No       | NULL                    | Email target for token.                  | PII               |
| token_hash  | TEXT        | Yes      | none                    | Hash of one-time token.                  | Critical secret   |
| type        | TEXT        | Yes      | none                    | Token purpose.                           | Security          |
| verified_at | TIMESTAMPTZ | No       | NULL                    | When verification completed.             | Security          |
| used_at     | TIMESTAMPTZ | No       | NULL                    | When token was consumed.                 | Security          |
| expires_at  | TIMESTAMPTZ | Yes      | none                    | Expiry time.                             | Security          |
| created_at  | TIMESTAMPTZ | Yes      | now()                   | Record creation time.                    | Operational       |

## subscriptions

Stores organization subscription and billing state.

| Column                  | Type          | Required | Default                  | Business meaning                                                   | Sensitivity                  |
| ----------------------- | ------------- | -------- | ------------------------ | ------------------------------------------------------------------ | ---------------------------- |
| id                      | UUID          | Yes      | gen_random_uuid()        | Internal subscription primary key.                                 | Internal                     |
| public_id               | TEXT          | Yes      | 'sub\_' + generated UUID | API-facing subscription id.                                        | Public identifier            |
| org_id                  | UUID          | Yes      | none                     | Organization that owns subscription.                               | Internal                     |
| plan                    | TEXT          | Yes      | 'free'                   | Machine-readable plan: free/pro/business/enterprise.               | Operational/billing          |
| plan_name               | TEXT          | Yes      | 'Free'                   | Human-readable plan label.                                         | Operational/billing          |
| status                  | TEXT          | Yes      | 'active'                 | Billing lifecycle: trialing, active, past_due, cancelled, expired. | Billing                      |
| billing_cycle           | TEXT          | No       | NULL                     | monthly/yearly/lifetime.                                           | Billing                      |
| currency                | TEXT          | Yes      | 'USD'                    | Billing currency.                                                  | Billing                      |
| amount                  | NUMERIC(12,2) | Yes      | 0                        | Subscription price amount.                                         | Billing                      |
| trial_started_at        | TIMESTAMPTZ   | No       | NULL                     | Trial start time.                                                  | Billing                      |
| trial_ends_at           | TIMESTAMPTZ   | No       | NULL                     | Trial end time.                                                    | Billing                      |
| current_period_start    | TIMESTAMPTZ   | No       | NULL                     | Current billing period start.                                      | Billing                      |
| current_period_end      | TIMESTAMPTZ   | No       | NULL                     | Current billing period end.                                        | Billing                      |
| cancel_at_period_end    | BOOLEAN       | Yes      | false                    | Whether cancellation is scheduled for period end.                  | Billing                      |
| payment_provider        | TEXT          | No       | NULL                     | External payment provider name.                                    | Billing                      |
| payment_customer_id     | TEXT          | No       | NULL                     | External customer id.                                              | Sensitive billing identifier |
| payment_subscription_id | TEXT          | No       | NULL                     | External subscription id.                                          | Sensitive billing identifier |
| created_at              | TIMESTAMPTZ   | Yes      | now()                    | Record creation time.                                              | Operational                  |
| updated_at              | TIMESTAMPTZ   | Yes      | now()                    | Record update time.                                                | Operational                  |

## organization_usage

Stores per-period organization usage and plan limits.

| Column          | Type        | Required | Default                    | Business meaning                                           | Sensitivity         |
| --------------- | ----------- | -------- | -------------------------- | ---------------------------------------------------------- | ------------------- |
| id              | UUID        | Yes      | gen_random_uuid()          | Internal usage primary key.                                | Internal            |
| public_id       | TEXT        | Yes      | 'usage\_' + generated UUID | API-facing usage id.                                       | Public identifier   |
| org_id          | UUID        | Yes      | none                       | Organization whose usage is tracked.                       | Internal            |
| subscription_id | UUID        | No       | NULL                       | Related subscription for the period.                       | Internal/billing    |
| period_start    | TIMESTAMPTZ | Yes      | none                       | Usage period start.                                        | Billing/operational |
| period_end      | TIMESTAMPTZ | Yes      | none                       | Usage period end.                                          | Billing/operational |
| limits          | JSONB       | Yes      | {}                         | Plan limit object such as members/projects/storageGB.      | Operational/billing |
| used            | JSONB       | Yes      | {}                         | Actual usage object such as projects/storage/api requests. | Operational/billing |
| created_at      | TIMESTAMPTZ | Yes      | now()                      | Record creation time.                                      | Operational         |
| updated_at      | TIMESTAMPTZ | Yes      | now()                      | Record update time.                                        | Operational         |

## audit_logs

Stores security and business audit events.

| Column        | Type        | Required | Default                    | Business meaning                                                | Sensitivity           |
| ------------- | ----------- | -------- | -------------------------- | --------------------------------------------------------------- | --------------------- |
| id            | UUID        | Yes      | gen_random_uuid()          | Internal audit log primary key.                                 | Internal              |
| public_id     | TEXT        | Yes      | 'audit\_' + generated UUID | API-facing audit event id.                                      | Public identifier     |
| org_id        | UUID        | No       | NULL                       | Organization context of event.                                  | Internal/business     |
| user_id       | UUID        | No       | NULL                       | Actor user if known.                                            | Internal/PII link     |
| event_type    | TEXT        | Yes      | none                       | Event key such as auth.sign_in or billing.subscription_changed. | Operational/security  |
| description   | TEXT        | No       | NULL                       | Human-readable event summary.                                   | Operational           |
| resource_type | TEXT        | No       | NULL                       | Type of resource affected.                                      | Operational           |
| resource_id   | TEXT        | No       | NULL                       | Public or internal resource id affected.                        | Operational           |
| changes       | JSONB       | No       | NULL                       | Before/after change details.                                    | Potentially sensitive |
| metadata      | JSONB       | Yes      | {}                         | Additional structured context.                                  | Potentially sensitive |
| ip_address    | INET        | No       | NULL                       | IP address of actor/request.                                    | PII/security          |
| user_agent    | TEXT        | No       | NULL                       | User-agent string.                                              | Device/PII-like       |
| status        | TEXT        | No       | NULL                       | Event outcome: success/failure/warning.                         | Operational/security  |
| error_message | TEXT        | No       | NULL                       | Error message when event failed.                                | Sensitive operational |
| created_at    | TIMESTAMPTZ | Yes      | now()                      | Event time.                                                     | Operational           |

## tasks

Stores org-scoped tasks. Reference CRUD module for permission enforcement and tenant isolation.

| Column      | Type        | Required | Default                   | Business meaning                                                                    | Sensitivity       |
| ----------- | ----------- | -------- | ------------------------- | ----------------------------------------------------------------------------------- | ----------------- |
| id          | UUID        | Yes      | gen_random_uuid()         | Internal primary key.                                                               | Internal          |
| public_id   | TEXT        | Yes      | 'task\_' + generated UUID | API-facing task identifier.                                                         | Public identifier |
| org_id      | UUID        | Yes      | none                      | Organization this task belongs to. Enforces tenant isolation.                       | Internal          |
| title       | TEXT        | Yes      | none                      | Short task summary. Max 255 characters.                                             | Operational       |
| description | TEXT        | Yes      | ''                        | Longer task detail. Max 2000 characters.                                            | Operational       |
| status      | TEXT        | Yes      | 'todo'                    | Lifecycle state: todo, in_progress, done, cancelled.                                | Operational       |
| due_date    | TIMESTAMPTZ | No       | NULL                      | Optional deadline. UTC.                                                             | Operational       |
| created_by  | UUID        | No       | NULL                      | User who created the task. SET NULL on user deletion.                               | Internal/PII link |
| assigned_to | UUID        | No       | NULL                      | User the task is assigned to. Must be active org member. SET NULL on user deletion. | Internal/PII link |
| created_at  | TIMESTAMPTZ | Yes      | NOW()                     | When the task was created.                                                          | Operational       |
| updated_at  | TIMESTAMPTZ | Yes      | NOW()                     | When the task was last modified.                                                    | Operational       |

---

# HRM Tables (added r9 — verified against `backend/internal/migrations/00021`–`00046`)

> The tables above this line (`users` through `tasks`) were the only ones documented before r9 — this file never covered CRM or platform tables either, so HRM is now ahead of CRM in dictionary coverage. Worth closing that gap in a future pass.

All HRM tables carry `org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE` for tenant isolation and a `public_id` with a table-specific prefix (e.g. `emp_`, `promo_`) as the API-safe identifier — these two columns are omitted from the row-by-row listing below where they follow this identical pattern, to keep the tables scannable; assume every table has them unless noted.

## Group A — Setup / Config

### hrm_departments

| Column               | Type    | Required | Default | Business meaning                                                                                         | Sensitivity   |
| -------------------- | ------- | -------- | ------- | -------------------------------------------------------------------------------------------------------- | ------------- |
| name                 | TEXT    | Yes      | none    | Department name.                                                                                         | Business data |
| description          | TEXT    | No       | NULL    | Free-text description.                                                                                   | Business data |
| parent_department_id | UUID    | No       | NULL    | Self-referencing FK for department hierarchy.                                                            | Internal      |
| head_employee_id     | UUID    | No       | NULL    | Department head; FK added after `hrm_employees` exists (no FK constraint in this table's own migration). | Internal      |
| is_active            | BOOLEAN | Yes      | true    | Soft-disable without deleting.                                                                           | Operational   |
| created_by           | UUID    | Yes      | none    | User who created the record.                                                                             | Internal      |

### hrm_positions

| Column        | Type    | Required | Default | Business meaning   | Sensitivity   |
| ------------- | ------- | -------- | ------- | ------------------ | ------------- |
| department_id | UUID    | No       | NULL    | Owning department. | Internal      |
| title         | TEXT    | Yes      | none    | Job title.         | Business data |
| description   | TEXT    | No       | NULL    | Role description.  | Business data |
| is_active     | BOOLEAN | Yes      | true    | Soft-disable.      | Operational   |

### hrm_employees

| Column                      | Type | Required       | Default   | Business meaning                                                                      | Sensitivity       |
| --------------------------- | ---- | -------------- | --------- | ------------------------------------------------------------------------------------- | ----------------- |
| user_id                     | UUID | No             | NULL      | Optional link to a platform login account — an employee record can exist without one. | Internal          |
| employee_number             | TEXT | No             | NULL      | Org-assigned employee ID.                                                             | PII               |
| first_name / last_name      | TEXT | first required | none      | Legal name.                                                                           | PII               |
| email / work_email          | TEXT | No             | NULL      | Personal / work email.                                                                | PII               |
| phone / work_phone          | TEXT | No             | NULL      | Contact numbers.                                                                      | PII               |
| date_of_birth               | DATE | No             | NULL      | DOB.                                                                                  | Sensitive PII     |
| gender                      | TEXT | No             | NULL      | `male`/`female`/`other`/`prefer_not_to_say`.                                          | Sensitive PII     |
| avatar_url                  | TEXT | No             | NULL      | Profile photo.                                                                        | PII               |
| hire_date                   | DATE | Yes            | none      | Employment start date.                                                                | Employment record |
| termination_date            | DATE | No             | NULL      | Set on termination.                                                                   | Employment record |
| employment_type             | TEXT | Yes            | full_time | `full_time`/`part_time`/`contractor`/`intern`.                                        | Employment record |
| status                      | TEXT | Yes            | active    | `active`/`inactive`/`on_leave`/`terminated`.                                          | Employment record |
| department_id / position_id | UUID | No             | NULL      | Org chart placement.                                                                  | Internal          |
| manager_id                  | UUID | No             | NULL      | Self-referencing FK, added after table creation.                                      | Internal          |
| address / city / country    | TEXT | No             | NULL      | Home address.                                                                         | Sensitive PII     |
| notes                       | TEXT | No             | NULL      | Free-text HR notes.                                                                   | Sensitive HR data |

### hrm_leave_types

| Column             | Type    | Required      | Default | Business meaning                          | Sensitivity   |
| ------------------ | ------- | ------------- | ------- | ----------------------------------------- | ------------- |
| name / description | TEXT    | name required | none    | Leave type label (e.g. "Annual", "Sick"). | Business data |
| max_days_per_year  | INTEGER | Yes           | 0       | Annual cap; 0 = unlimited.                | Policy config |
| is_paid            | BOOLEAN | Yes           | true    | Whether this leave type is paid.          | Policy config |
| requires_approval  | BOOLEAN | Yes           | true    | Whether requests need approval.           | Policy config |
| is_active          | BOOLEAN | Yes           | true    | Soft-disable.                             | Operational   |

### hrm_leave_requests

| Column                                  | Type         | Required | Default | Business meaning                                                                               | Sensitivity       |
| --------------------------------------- | ------------ | -------- | ------- | ---------------------------------------------------------------------------------------------- | ----------------- |
| employee_id                             | UUID         | Yes      | none    | Requesting employee.                                                                           | Internal          |
| leave_type_id                           | UUID         | Yes      | none    | FK to `hrm_leave_types` (`ON DELETE RESTRICT` — can't delete a type with requests against it). | Internal          |
| start_date / end_date                   | DATE         | Yes      | none    | Leave range; `CHECK (end_date >= start_date)`.                                                 | Employment record |
| total_days                              | NUMERIC(5,1) | Yes      | none    | Computed day count; `CHECK (total_days > 0)`.                                                  | Employment record |
| reason                                  | TEXT         | No       | NULL    | Employee-provided reason.                                                                      | Sensitive HR data |
| status                                  | TEXT         | Yes      | pending | `pending`/`approved`/`rejected`/`cancelled`.                                                   | Operational       |
| reviewed_by / reviewed_at / review_note | mixed        | No       | NULL    | Approver decision trail.                                                                       | Sensitive HR data |

### hrm_salary_components

| Column             | Type          | Required      | Default | Business meaning                                                    | Sensitivity      |
| ------------------ | ------------- | ------------- | ------- | ------------------------------------------------------------------- | ---------------- |
| name / description | TEXT          | name required | none    | Component label (e.g. "Basic Pay", "House Rent Allowance").         | Business data    |
| component_type     | TEXT          | Yes           | none    | `earning`/`deduction`/`employer_contribution`.                      | Financial config |
| calc_method        | TEXT          | Yes           | none    | `fixed`/`pct_of_basic`/`pct_of_gross`/`formula`/`manual`/`slab`.    | Financial config |
| fixed_value        | NUMERIC(15,4) | Yes           | 0       | Used by `fixed`/`pct_of_basic`/`pct_of_gross` methods.              | Financial config |
| formula_expression | TEXT          | No            | NULL    | `expr-lang` expression, evaluated at payroll time.                  | Financial config |
| formula_variables  | TEXT[]        | No            | NULL    | Documents which env vars the formula uses.                          | Financial config |
| slab_config        | JSONB         | No            | NULL    | Progressive bracket config for `slab` method (feeds `ComputeSlab`). | Financial config |
| is_taxable         | BOOLEAN       | Yes           | false   | Whether this component counts toward taxable income.                | Financial config |
| display_order      | INTEGER       | Yes           | 0       | UI/payslip ordering.                                                | Operational      |
| is_active          | BOOLEAN       | Yes           | true    | Soft-disable.                                                       | Operational      |

### hrm_salary_structures

| Column             | Type    | Required      | Default | Business meaning                          | Sensitivity   |
| ------------------ | ------- | ------------- | ------- | ----------------------------------------- | ------------- |
| name / description | TEXT    | name required | none    | Structure label.                          | Business data |
| grade_label        | TEXT    | No            | NULL    | Optional grade tag ("Grade A", "Senior"). | Business data |
| is_active          | BOOLEAN | Yes           | true    | Soft-disable.                             | Operational   |

### hrm_salary_structure_components

| Column         | Type          | Required | Default | Business meaning                                                 | Sensitivity      |
| -------------- | ------------- | -------- | ------- | ---------------------------------------------------------------- | ---------------- |
| structure_id   | UUID          | Yes      | none    | Owning structure.                                                | Internal         |
| component_id   | UUID          | Yes      | none    | FK to `hrm_salary_components` (`ON DELETE RESTRICT`).            | Internal         |
| override_value | NUMERIC(15,4) | No       | NULL    | Per-structure override of the component's default `fixed_value`. | Financial config |
| display_order  | INTEGER       | Yes      | 0       | Ordering. `UNIQUE(structure_id, component_id)`.                  | Operational      |

### hrm_employee_salary_records

Append-only — no `updated_at`; a change creates a new row rather than editing history.

| Column         | Type          | Required | Default | Business meaning                                                         | Sensitivity             |
| -------------- | ------------- | -------- | ------- | ------------------------------------------------------------------------ | ----------------------- |
| employee_id    | UUID          | Yes      | none    | Whose salary.                                                            | Internal                |
| structure_id   | UUID          | No       | NULL    | Applied salary structure.                                                | Internal                |
| basic_pay      | NUMERIC(15,2) | Yes      | none    | `CHECK (basic_pay >= 0)`.                                                | Compensation — critical |
| effective_date | DATE          | Yes      | none    | When this record takes effect.                                           | Compensation — critical |
| change_reason  | TEXT          | Yes      | none    | `joining`/`promotion`/`annual_revision`/`transfer`/`correction`/`other`. | Compensation — critical |
| change_notes   | TEXT          | No       | NULL    | Free text.                                                               | Sensitive HR data       |

### hrm_approval_templates

| Column               | Type    | Required      | Default | Business meaning                                                                     | Sensitivity   |
| -------------------- | ------- | ------------- | ------- | ------------------------------------------------------------------------------------ | ------------- |
| name / description   | TEXT    | name required | none    | Template label.                                                                      | Business data |
| action_type          | TEXT    | Yes           | none    | Which entity type this chain applies to (`leave`, `promotion`, `termination`, etc.). | Policy config |
| condition_expression | TEXT    | No            | NULL    | `expr-lang` condition; NULL = default template for the action type.                  | Policy config |
| is_default           | BOOLEAN | Yes           | false   | Whether this is the fallback template for its `action_type`.                         | Policy config |
| is_active            | BOOLEAN | Yes           | true    | Soft-disable.                                                                        | Operational   |

### hrm_approval_template_levels

| Column           | Type    | Required | Default       | Business meaning                                              | Sensitivity   |
| ---------------- | ------- | -------- | ------------- | ------------------------------------------------------------- | ------------- |
| template_id      | UUID    | Yes      | none          | Owning template.                                              | Internal      |
| level            | INTEGER | Yes      | none          | Sequence position, `CHECK (level >= 1)`, unique per template. | Policy config |
| approver_type    | TEXT    | Yes      | none          | `reporting_manager`/`dept_head`/`role`/`specific_user`.       | Policy config |
| approver_role    | TEXT    | No       | NULL          | Used when `approver_type = 'role'`.                           | Policy config |
| approver_user_id | UUID    | No       | NULL          | Used when `approver_type = 'specific_user'`.                  | Internal      |
| sla_hours        | INTEGER | Yes      | 48            | Time before SLA breach.                                       | Policy config |
| on_sla_breach    | TEXT    | Yes      | escalate_next | `escalate_next`/`auto_approve`/`auto_reject`.                 | Policy config |

### hrm_approval_instances

| Column            | Type        | Required | Default | Business meaning                                                                                                     | Sensitivity |
| ----------------- | ----------- | -------- | ------- | -------------------------------------------------------------------------------------------------------------------- | ----------- |
| template_id       | UUID        | No       | NULL    | Template this instance was created from.                                                                             | Internal    |
| entity_type       | TEXT        | Yes      | none    | Polymorphic target type (`leave_request`, `promotion`, `termination`, ...).                                          | Operational |
| entity_id         | UUID        | Yes      | none    | Polymorphic target ID (no FK — cross-table by design).                                                               | Internal    |
| instance_snapshot | JSONB       | Yes      | `[]`    | Frozen copy of the level chain at creation time, so later template edits don't retroactively change a live approval. | Operational |
| current_level     | INTEGER     | Yes      | 1       | Which level is currently pending.                                                                                    | Operational |
| overall_status    | TEXT        | Yes      | pending | `pending`/`approved`/`rejected`/`cancelled`.                                                                         | Operational |
| requested_by      | UUID        | Yes      | none    | Who triggered the approval.                                                                                          | Internal    |
| completed_at      | TIMESTAMPTZ | No       | NULL    | When the chain resolved.                                                                                             | Operational |

### hrm_approval_decisions

| Column      | Type        | Required | Default | Business meaning                                             | Sensitivity       |
| ----------- | ----------- | -------- | ------- | ------------------------------------------------------------ | ----------------- |
| instance_id | UUID        | Yes      | none    | Owning instance.                                             | Internal          |
| level       | INTEGER     | Yes      | none    | Which level this decision is for; unique per instance+level. | Operational       |
| approver_id | UUID        | Yes      | none    | Who decided.                                                 | Internal          |
| action      | TEXT        | Yes      | none    | `approved`/`rejected`/`cancelled`.                           | Operational       |
| note        | TEXT        | No       | NULL    | Decision rationale.                                          | Sensitive HR data |
| decided_at  | TIMESTAMPTZ | Yes      | now()   | Decision timestamp.                                          | Operational       |

### hrm_warning_types

| Column                 | Type    | Required      | Default              | Business meaning                                                       | Sensitivity   |
| ---------------------- | ------- | ------------- | -------------------- | ---------------------------------------------------------------------- | ------------- |
| name / description     | TEXT    | name required | none                 | Type label.                                                            | Business data |
| severity_level         | INTEGER | Yes           | 5                    | 1 (minor) to 10 (final warning), `CHECK (BETWEEN 1 AND 10)`.           | Policy config |
| can_be_issued_by       | TEXT[]  | Yes           | `{admin,hr_manager}` | Which roles may issue this type.                                       | Policy config |
| requires_hr_approval   | BOOLEAN | Yes           | false                | Gate before issuing.                                                   | Policy config |
| approval_template_id   | UUID    | No            | NULL                 | Which chain to use if approval required.                               | Internal      |
| employee_can_respond   | BOOLEAN | Yes           | true                 | Whether the employee gets a response window.                           | Policy config |
| response_window_days   | INTEGER | Yes           | 5                    | Days to respond.                                                       | Policy config |
| auto_generate_document | BOOLEAN | Yes           | false                | Whether issuing auto-creates a letter.                                 | Policy config |
| document_template_id   | UUID    | No            | NULL                 | FK added in migration `00024` (after `hrm_document_templates` exists). | Internal      |
| valid_duration_days    | INTEGER | Yes           | 0                    | How long the warning counts toward escalation; 0 = never expires.      | Policy config |
| is_active              | BOOLEAN | Yes           | true                 | Soft-disable.                                                          | Operational   |

### hrm_warning_escalation_rules

| Column                  | Type    | Required | Default   | Business meaning                                                                                              | Sensitivity   |
| ----------------------- | ------- | -------- | --------- | ------------------------------------------------------------------------------------------------------------- | ------------- |
| trigger_warning_type_id | UUID    | Yes      | none      | Which warning type this rule watches.                                                                         | Internal      |
| trigger_count           | INTEGER | Yes      | 3         | How many active warnings of that type trigger the rule.                                                       | Policy config |
| within_days             | INTEGER | Yes      | 0         | Lookback window; 0 = all time.                                                                                | Policy config |
| action                  | TEXT    | Yes      | notify_hr | `notify_hr`/`notify_management`/`flag_termination_review` — system only notifies, never auto-creates records. | Policy config |
| notification_roles      | TEXT[]  | Yes      | `{admin}` | Who gets notified.                                                                                            | Policy config |
| is_active               | BOOLEAN | Yes      | true      | Soft-disable.                                                                                                 | Operational   |

### hrm_document_templates

| Column                   | Type    | Required | Default | Business meaning                                                     | Sensitivity   |
| ------------------------ | ------- | -------- | ------- | -------------------------------------------------------------------- | ------------- |
| name                     | TEXT    | Yes      | none    | Template label.                                                      | Business data |
| document_type            | TEXT    | Yes      | none    | `offer_letter`/`contract`/`warning_letter`/.../`custom` (11 values). | Business data |
| description              | TEXT    | No       | NULL    | Free text.                                                           | Business data |
| body_markdown            | TEXT    | Yes      | ''      | Template body with `{{placeholder}}` syntax.                         | Business data |
| available_variables      | TEXT[]  | Yes      | `{}`    | Which placeholders this template supports.                           | Operational   |
| requires_acknowledgement | BOOLEAN | Yes      | false   | Whether generated docs need an acknowledgement flow.                 | Policy config |
| is_active                | BOOLEAN | Yes      | true    | Soft-disable.                                                        | Operational   |

### hrm_employee_documents

| Column                                           | Type    | Required | Default         | Business meaning                                                                                | Sensitivity        |
| ------------------------------------------------ | ------- | -------- | --------------- | ----------------------------------------------------------------------------------------------- | ------------------ |
| employee_id                                      | UUID    | Yes      | none            | Subject of the document.                                                                        | Internal           |
| template_id                                      | UUID    | No       | NULL            | NULL = direct upload; set = generated from a template.                                          | Internal           |
| title                                            | TEXT    | Yes      | none            | Document title.                                                                                 | Sensitive HR data  |
| document_type                                    | TEXT    | Yes      | none            | 15 possible values incl. `passport`, `visa`, `id_proof`.                                        | Sensitive HR data  |
| file_url / file_name                             | TEXT    | Yes      | none            | Storage location — always populated (uploaded or generated-then-stored).                        | Sensitive document |
| file_size_bytes                                  | BIGINT  | No       | NULL            | File size.                                                                                      | Operational        |
| mime_type                                        | TEXT    | Yes      | application/pdf | Content type.                                                                                   | Operational        |
| generated_content                                | TEXT    | No       | NULL            | Resolved Markdown/HTML before PDF render; NULL for direct uploads.                              | Sensitive document |
| related_type / related_id                        | mixed   | No       | NULL            | Polymorphic link to the source entity (warning, promotion, transfer, resignation, termination). | Internal           |
| version                                          | INTEGER | Yes      | 1               | Version number.                                                                                 | Operational        |
| superseded_by                                    | UUID    | No       | NULL            | Points to the newer version, if any — supersede rather than overwrite.                          | Internal           |
| bulk_send_batch_id                               | UUID    | No       | NULL            | Links to `hrm_document_bulk_sends.batch_id` if sent as part of a batch.                         | Internal           |
| expiry_date                                      | DATE    | No       | NULL            | For visas, certifications, contracts.                                                           | Sensitive document |
| status                                           | TEXT    | Yes      | draft           | `draft`/`sent`/`acknowledged`/`declined`/`expired`/`withdrawn`/`superseded`.                    | Operational        |
| issued_by                                        | UUID    | No       | NULL            | Who issued it.                                                                                  | Internal           |
| sent_at / acknowledged_at / acknowledgement_note | mixed   | No       | NULL            | Delivery + response trail.                                                                      | Sensitive HR data  |

### hrm_document_bulk_sends

| Column                                        | Type        | Required | Default | Business meaning                                                         | Sensitivity |
| --------------------------------------------- | ----------- | -------- | ------- | ------------------------------------------------------------------------ | ----------- |
| template_id                                   | UUID        | Yes      | none    | Which template was sent (`ON DELETE RESTRICT`).                          | Internal    |
| sender_id                                     | UUID        | Yes      | none    | Who initiated the send.                                                  | Internal    |
| recipient_type                                | TEXT        | Yes      | none    | `all`/`department`/`employee_list`.                                      | Operational |
| recipient_ids                                 | JSONB       | Yes      | `[]`    | Array of department or employee IDs (shape depends on `recipient_type`). | Internal    |
| batch_id                                      | UUID        | Yes      | random  | Links to `hrm_employee_documents.bulk_send_batch_id`.                    | Internal    |
| total_count / pending_count / completed_count | INTEGER     | Yes      | 0       | Batch progress counters.                                                 | Operational |
| sent_at                                       | TIMESTAMPTZ | Yes      | now()   | Batch start time.                                                        | Operational |

### hrm_shifts

| Column                          | Type         | Required               | Default      | Business meaning                                                  | Sensitivity   |
| ------------------------------- | ------------ | ---------------------- | ------------ | ----------------------------------------------------------------- | ------------- |
| name / description              | TEXT         | name required          | none         | Shift label.                                                      | Business data |
| shift_type                      | TEXT         | Yes                    | fixed        | `fixed`/`flexible`.                                               | Policy config |
| start_time / end_time           | TIME         | conditionally required | NULL         | Required when `shift_type = 'fixed'` (enforced by a table CHECK). | Policy config |
| core_start_time / core_end_time | TIME         | conditionally required | NULL         | "Must be online" window for flexible shifts.                      | Policy config |
| weekly_hours_target             | NUMERIC(5,2) | conditionally required | NULL         | Required when `shift_type = 'flexible'`.                          | Policy config |
| break_minutes                   | INTEGER      | Yes                    | 60           | Standard break length.                                            | Policy config |
| working_days                    | TEXT[]       | Yes                    | `{mon..fri}` | Subset of `{mon,tue,wed,thu,fri,sat,sun}`.                        | Policy config |
| track_overtime                  | BOOLEAN      | Yes                    | false        | Whether OT is tracked for this shift.                             | Policy config |
| overtime_threshold_hours        | NUMERIC(5,2) | No                     | NULL         | Hours/day beyond which time counts as OT.                         | Policy config |
| track_breaks                    | BOOLEAN      | Yes                    | false        | Per-punch break tracking vs. fixed `break_minutes`.               | Policy config |
| is_default                      | BOOLEAN      | Yes                    | false        | Default shift for new assignments.                                | Operational   |
| is_active                       | BOOLEAN      | Yes                    | true         | Soft-disable.                                                     | Operational   |

### hrm_work_schedule_assignments

| Column         | Type | Required | Default | Business meaning                                                  | Sensitivity |
| -------------- | ---- | -------- | ------- | ----------------------------------------------------------------- | ----------- |
| shift_id       | UUID | Yes      | none    | Which shift is assigned.                                          | Internal    |
| assignee_type  | TEXT | Yes      | none    | `organization`/`department`/`employee` — polymorphic scope.       | Operational |
| assignee_id    | UUID | Yes      | none    | ID matching `assignee_type` (no FK — polymorphic by design).      | Internal    |
| effective_date | DATE | Yes      | today   | Start of assignment.                                              | Operational |
| end_date       | DATE | No       | NULL    | NULL = indefinite; `CHECK (end_date >= effective_date)` when set. | Operational |

### hrm_holiday_calendars

| Column             | Type    | Required      | Default | Business meaning                      | Sensitivity   |
| ------------------ | ------- | ------------- | ------- | ------------------------------------- | ------------- |
| name / description | TEXT    | name required | none    | Calendar label.                       | Business data |
| country_code       | TEXT    | No            | NULL    | ISO 3166-1 alpha-2, for display only. | Business data |
| year               | INTEGER | Yes           | none    | `CHECK (BETWEEN 2000 AND 2100)`.      | Operational   |
| is_active          | BOOLEAN | Yes           | true    | Soft-disable.                         | Operational   |

### hrm_holidays

| Column        | Type    | Required | Default | Business meaning                                                       | Sensitivity   |
| ------------- | ------- | -------- | ------- | ---------------------------------------------------------------------- | ------------- |
| calendar_id   | UUID    | Yes      | none    | Owning calendar.                                                       | Internal      |
| name          | TEXT    | Yes      | none    | Holiday name.                                                          | Business data |
| date          | DATE    | Yes      | none    | Unique per calendar.                                                   | Operational   |
| holiday_type  | TEXT    | Yes      | public  | `public`/`company`/`optional`.                                         | Operational   |
| is_paid       | BOOLEAN | Yes      | true    | Whether it's a paid holiday.                                           | Policy config |
| repeat_yearly | BOOLEAN | Yes      | false   | If true, a job creates the equivalent holiday in next year's calendar. | Operational   |

### hrm_calendar_assignments

Assigns a **holiday calendar** to an org/department/employee — distinct from the Group E `hrm_calendar_events` (HR events calendar); both were originally miscategorized as the same thing in an earlier draft of this doc's grouping.

| Column         | Type | Required | Default | Business meaning                                                                        | Sensitivity |
| -------------- | ---- | -------- | ------- | --------------------------------------------------------------------------------------- | ----------- |
| calendar_id    | UUID | Yes      | none    | Which holiday calendar.                                                                 | Internal    |
| assignee_type  | TEXT | Yes      | none    | `organization`/`department`/`employee`.                                                 | Operational |
| assignee_id    | UUID | Yes      | none    | Polymorphic target.                                                                     | Internal    |
| effective_date | DATE | Yes      | today   | When the assignment starts.                                                             | Operational |
| —              | —    | —        | —       | `UNIQUE(assignee_type, assignee_id)` — only one active calendar per assignee at a time. | —           |

### hrm_employee_contracts

| Column                | Type         | Required       | Default | Business meaning                                                           | Sensitivity       |
| --------------------- | ------------ | -------------- | ------- | -------------------------------------------------------------------------- | ----------------- |
| employee_id           | UUID         | Yes            | none    | Whose contract.                                                            | Internal          |
| contract_type         | TEXT         | Yes            | none    | `permanent`/`fixed_term`/`probation`/`internship`/`consultant`.            | Employment record |
| start_date / end_date | DATE         | start required | NULL    | End NULL = permanent/open-ended; `CHECK (end_date > start_date)` when set. | Employment record |
| probation_end_date    | DATE         | No             | NULL    | `CHECK (>= start_date)` when set.                                          | Employment record |
| notice_period_days    | INTEGER      | Yes            | 30      | Feeds the resignation flow's notice calculation.                           | Employment record |
| salary_structure_id   | UUID         | No             | NULL    | Optional compensation link.                                                | Internal          |
| work_hours_per_week   | NUMERIC(5,2) | No             | NULL    | Contracted hours.                                                          | Employment record |
| document_id           | UUID         | No             | NULL    | Signed contract document.                                                  | Internal          |
| is_active             | BOOLEAN      | Yes            | true    | Current vs. superseded contract.                                           | Operational       |
| notes                 | TEXT         | No             | NULL    | Free text.                                                                 | Sensitive HR data |

## Group B — Employee Lifecycle

### hrm_promotions

| Column                                                                            | Type          | Required | Default | Business meaning                                                        | Sensitivity                         |
| --------------------------------------------------------------------------------- | ------------- | -------- | ------- | ----------------------------------------------------------------------- | ----------------------------------- |
| employee_id                                                                       | UUID          | Yes      | none    | Who's being promoted (`ON DELETE RESTRICT`).                            | Internal                            |
| from_position_id / from_department_id / from_salary_structure_id / from_basic_pay | mixed         | No       | NULL    | Snapshot of current state at record creation.                           | Compensation — critical (pay field) |
| to_position_id                                                                    | UUID          | Yes      | none    | Target position (`ON DELETE RESTRICT`).                                 | Internal                            |
| to_department_id / to_salary_structure_id                                         | UUID          | No       | NULL    | Target state.                                                           | Internal                            |
| new_basic_pay                                                                     | NUMERIC(15,2) | No       | NULL    | NULL = no pay change.                                                   | Compensation — critical             |
| effective_date                                                                    | DATE          | Yes      | none    | When the promotion takes effect.                                        | Employment record                   |
| reason / notes                                                                    | TEXT          | No       | NULL    | Free text.                                                              | Sensitive HR data                   |
| approval_instance_id                                                              | UUID          | No       | NULL    | NULL if no approval required.                                           | Internal                            |
| document_id                                                                       | UUID          | No       | NULL    | Generated promotion letter.                                             | Internal                            |
| status                                                                            | TEXT          | Yes      | draft   | `draft`/`pending_approval`/`approved`/`rejected`/`cancelled`/`applied`. | Operational                         |
| applied_at / applied_by                                                           | mixed         | No       | NULL    | When/who executed the change against the employee record.               | Operational                         |

### hrm_transfers

| Column                                                        | Type  | Required | Default    | Business meaning                            | Sensitivity       |
| ------------------------------------------------------------- | ----- | -------- | ---------- | ------------------------------------------- | ----------------- |
| employee_id                                                   | UUID  | Yes      | none       | Who's transferring.                         | Internal          |
| transfer_type                                                 | TEXT  | Yes      | department | `department`/`location`/`reporting`/`full`. | Operational       |
| from_department_id / from_manager_employee_id / from_location | mixed | No       | NULL       | Snapshot of current state.                  | Internal          |
| to_department_id / to_manager_employee_id / to_location       | mixed | No       | NULL       | Target state.                               | Internal          |
| effective_date                                                | DATE  | Yes      | none       | When it takes effect.                       | Employment record |
| reason / notes                                                | TEXT  | No       | NULL       | Free text.                                  | Sensitive HR data |
| approval_instance_id / document_id                            | UUID  | No       | NULL       | Approval chain + generated letter.          | Internal          |
| status                                                        | TEXT  | Yes      | draft      | Same 6-state lifecycle as promotions.       | Operational       |
| applied_at / applied_by                                       | mixed | No       | NULL       | Execution trail.                            | Operational       |

### hrm_resignations

| Column                                              | Type    | Required | Default   | Business meaning                                                                                            | Sensitivity       |
| --------------------------------------------------- | ------- | -------- | --------- | ----------------------------------------------------------------------------------------------------------- | ----------------- |
| employee_id                                         | UUID    | Yes      | none      | Who's resigning (`ON DELETE RESTRICT`).                                                                     | Internal          |
| resignation_date                                    | DATE    | Yes      | today     | Submission date; backdating allowed.                                                                        | Employment record |
| notice_period_days                                  | INTEGER | Yes      | 30        | Snapshotted from `hrm_employee_contracts` at submission.                                                    | Employment record |
| is_notice_waived                                    | BOOLEAN | Yes      | false     | Whether HR waived notice.                                                                                   | Operational       |
| last_working_date                                   | DATE    | Yes      | none      | Computed: `resignation_date + notice_period_days`, or HR override if waived. `CHECK (>= resignation_date)`. | Employment record |
| reason_category                                     | TEXT    | Yes      | other     | `personal`/`career_growth`/`better_opportunity`/`relocation`/`health`/`retirement`/`other`.                 | Sensitive HR data |
| reason_remarks                                      | TEXT    | No       | NULL      | Free text.                                                                                                  | Sensitive HR data |
| approval_instance_id                                | UUID    | No       | NULL      | Manager + HR acknowledgement chain.                                                                         | Internal          |
| document_id                                         | UUID    | No       | NULL      | Acceptance letter.                                                                                          | Internal          |
| exit_interview_completed / exit_clearance_completed | BOOLEAN | Yes      | false     | Exit process checkboxes.                                                                                    | Operational       |
| status                                              | TEXT    | Yes      | submitted | `submitted`/`accepted`/`withdrawn`/`rejected`.                                                              | Operational       |
| accepted_at / accepted_by                           | mixed   | No       | NULL      | Decision trail.                                                                                             | Operational       |

### hrm_terminations

| Column                                | Type    | Required | Default    | Business meaning                                                                 | Sensitivity              |
| ------------------------------------- | ------- | -------- | ---------- | -------------------------------------------------------------------------------- | ------------------------ |
| employee_id                           | UUID    | Yes      | none       | Who's being terminated (`ON DELETE RESTRICT`).                                   | Internal                 |
| termination_type                      | TEXT    | Yes      | none       | `voluntary`/`involuntary`/`layoff`/`retirement`/`contract_end`/`probation_fail`. | Sensitive HR data        |
| termination_date                      | DATE    | Yes      | none       | Official date.                                                                   | Employment record        |
| last_working_date                     | DATE    | Yes      | none       | `CHECK (<= termination_date)`.                                                   | Employment record        |
| reason                                | TEXT    | No       | NULL       | Shared reason.                                                                   | Sensitive HR data        |
| internal_notes                        | TEXT    | No       | NULL       | Private HR notes — explicitly **not** shared with the employee.                  | Highly sensitive HR data |
| approval_instance_id                  | UUID    | No       | NULL       | E.g. HR + legal approval for involuntary terminations.                           | Internal                 |
| document_id                           | UUID    | No       | NULL       | Termination letter.                                                              | Internal                 |
| severance_amount / severance_currency | mixed   | No       | NULL / BDT | Informational — actual disbursal happens via payroll.                            | Compensation — critical  |
| is_rehire_eligible                    | BOOLEAN | Yes      | true       | Rehire flag.                                                                     | Sensitive HR data        |
| exit_clearance_completed              | BOOLEAN | Yes      | false      | Exit checklist.                                                                  | Operational              |
| status                                | TEXT    | Yes      | draft      | Same 6-state lifecycle as promotions/transfers.                                  | Operational              |
| applied_at / applied_by               | mixed   | No       | NULL       | When access was actually revoked.                                                | Operational              |

## Group C — Disciplinary

### hrm_employee_warnings

| Column                                                                       | Type        | Required | Default  | Business meaning                                                                    | Sensitivity       |
| ---------------------------------------------------------------------------- | ----------- | -------- | -------- | ----------------------------------------------------------------------------------- | ----------------- |
| employee_id                                                                  | UUID        | Yes      | none     | Subject (`ON DELETE RESTRICT`).                                                     | Internal          |
| warning_type_id                                                              | UUID        | Yes      | none     | FK, `ON DELETE RESTRICT`.                                                           | Internal          |
| warning_type_name                                                            | TEXT        | Yes      | none     | Snapshot — a later type rename doesn't rewrite history.                             | Sensitive HR data |
| severity_level                                                               | INTEGER     | Yes      | 5        | 1–10.                                                                               | Sensitive HR data |
| title / description                                                          | TEXT        | Yes      | none     | Incident summary.                                                                   | Sensitive HR data |
| incident_date                                                                | DATE        | Yes      | none     | When it happened.                                                                   | Sensitive HR data |
| issued_by                                                                    | UUID        | Yes      | none     | Issuing manager/HR (`ON DELETE RESTRICT`).                                          | Internal          |
| witness_ids                                                                  | UUID[]      | Yes      | `{}`     | Witnessing employees.                                                               | Sensitive HR data |
| approval_instance_id                                                         | UUID        | No       | NULL     | Present only if the warning type requires HR approval.                              | Internal          |
| document_id                                                                  | UUID        | No       | NULL     | Auto-generated warning letter.                                                      | Internal          |
| can_employee_respond / response_window_days                                  | mixed       | Yes      | true / 5 | Snapshotted from the warning type at issuance.                                      | Policy config     |
| response_deadline                                                            | DATE        | No       | NULL     | Computed: issue date + window.                                                      | Operational       |
| employee_response / employee_responded_at                                    | mixed       | No       | NULL     | Employee's reply.                                                                   | Sensitive HR data |
| appeal_reason / appeal_submitted_at / appeal_resolution / appeal_resolved_at | mixed       | No       | NULL     | Appeal trail.                                                                       | Sensitive HR data |
| expires_at                                                                   | DATE        | No       | NULL     | Computed: issue date + `valid_duration_days`; NULL = permanent.                     | Operational       |
| is_active                                                                    | BOOLEAN     | Yes      | true     | Whether it still counts toward escalation.                                          | Operational       |
| issued_at                                                                    | TIMESTAMPTZ | No       | NULL     | When formally issued (vs. just drafted).                                            | Operational       |
| status                                                                       | TEXT        | Yes      | draft    | `draft`/`pending_approval`/`issued`/`acknowledged`/`appealed`/`closed`/`cancelled`. | Operational       |

### hrm_complaints

| Column                                                           | Type    | Required | Default   | Business meaning                                                                                                              | Sensitivity              |
| ---------------------------------------------------------------- | ------- | -------- | --------- | ----------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| employee_id                                                      | UUID    | Yes      | none      | Complainant (`ON DELETE RESTRICT`).                                                                                           | Internal                 |
| is_anonymous                                                     | BOOLEAN | Yes      | false     | Whether the complainant's identity is withheld downstream.                                                                    | Sensitive HR data        |
| complaint_type                                                   | TEXT    | Yes      | general   | `harassment`/`discrimination`/`workplace_safety`/`policy_violation`/`manager_conduct`/`wage_dispute`/`retaliation`/`general`. | Highly sensitive HR data |
| title / description                                              | TEXT    | Yes      | none      | Complaint content.                                                                                                            | Highly sensitive HR data |
| incident_date                                                    | DATE    | No       | NULL      | When it happened.                                                                                                             | Highly sensitive HR data |
| against_employee_id                                              | UUID    | No       | NULL      | Who it's against, if an employee.                                                                                             | Highly sensitive HR data |
| against_details                                                  | TEXT    | No       | NULL      | Free text if not against an employee.                                                                                         | Highly sensitive HR data |
| investigator_id / investigation_notes / investigation_started_at | mixed   | No       | NULL      | Investigation trail.                                                                                                          | Highly sensitive HR data |
| resolution / resolution_action / resolved_at / resolved_by       | mixed   | No       | NULL      | Outcome. `resolution_action` is one of 7 values incl. `warning_issued`, `termination`.                                        | Highly sensitive HR data |
| document_id                                                      | UUID    | No       | NULL      | Resolution/outcome letter.                                                                                                    | Internal                 |
| status                                                           | TEXT    | Yes      | submitted | `submitted`/`under_review`/`investigating`/`resolved`/`dismissed`/`withdrawn`.                                                | Operational              |

### hrm_acknowledgements

Shared polymorphic acknowledgement mechanism used by warnings, documents, announcements, calendar events, and policies.

| Column                                         | Type    | Required | Default    | Business meaning                                                                                                                                                           | Sensitivity       |
| ---------------------------------------------- | ------- | -------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------- |
| employee_id                                    | UUID    | Yes      | none       | Who is acknowledging (`ON DELETE RESTRICT`).                                                                                                                               | Internal          |
| acknowledgeable_type                           | TEXT    | Yes      | none       | `warning`/`document`/`announcement`/`calendar_event`/`policy`.                                                                                                             | Operational       |
| acknowledgeable_id                             | UUID    | Yes      | none       | Polymorphic target (no FK by design).                                                                                                                                      | Internal          |
| entity_title                                   | TEXT    | Yes      | none       | Denormalized snapshot of the target's title, for audit even if the source entity changes.                                                                                  | Sensitive HR data |
| notes                                          | TEXT    | No       | NULL       | Employee note.                                                                                                                                                             | Sensitive HR data |
| signature_required                             | BOOLEAN | Yes      | false      | Whether a signature is needed.                                                                                                                                             | Policy config     |
| signed_at / signature_data                     | mixed   | No       | NULL       | Signature capture — base64 image or typed name.                                                                                                                            | Sensitive/legal   |
| status                                         | TEXT    | Yes      | pending    | `pending`/`acknowledged`/`declined`/`expired`.                                                                                                                             | Operational       |
| acknowledged_at / declined_at / decline_reason | mixed   | No       | NULL       | Response trail.                                                                                                                                                            | Sensitive HR data |
| expires_at                                     | DATE    | No       | NULL       | For annual re-acknowledgement flows.                                                                                                                                       | Operational       |
| requested_by / requested_at / reminder_sent_at | mixed   | Yes/No   | now()/NULL | Who asked and when, plus reminder tracking.                                                                                                                                | Internal          |
| —                                              | —       | —        | —          | `UNIQUE(employee_id, acknowledgeable_type, acknowledgeable_id, status)`, deferrable — one active ack per (employee, entity), multiple allowed over time for expiring ones. | —                 |

## Group D — Time & Compensation

### hrm_attendance_records

| Column                                             | Type         | Required | Default  | Business meaning                                                                                 | Sensitivity               |
| -------------------------------------------------- | ------------ | -------- | -------- | ------------------------------------------------------------------------------------------------ | ------------------------- |
| employee_id                                        | UUID         | Yes      | none     | Whose record (`ON DELETE RESTRICT`).                                                             | Internal                  |
| attendance_date                                    | DATE         | Yes      | none     | Unique per employee+date.                                                                        | Operational               |
| shift_id / shift_name / expected_in / expected_out | mixed        | No       | NULL     | Shift snapshot at record creation — immune to later shift edits.                                 | Operational               |
| check_in_time / check_out_time                     | TIME         | No       | NULL     | Actual punches; NULL if absent/holiday/weekend.                                                  | Sensitive attendance data |
| break_minutes                                      | INTEGER      | Yes      | 0        | Break time taken.                                                                                | Operational               |
| regular_hours / overtime_hours                     | NUMERIC(5,2) | Yes      | 0        | Computed by the service layer on save.                                                           | Operational               |
| day_type                                           | TEXT         | Yes      | present  | `present`/`absent`/`half_day`/`late`/`on_leave`/`holiday`/`weekend`/`work_from_home`.            | Sensitive attendance data |
| source                                             | TEXT         | Yes      | manual   | `manual`/`device`/`api`/`system` — which of the 3 ingestion paths (+ manual) created this row.   | Operational               |
| notes                                              | TEXT         | No       | NULL     | Free text.                                                                                       | Sensitive HR data         |
| regularization_reason / regularization_instance_id | mixed        | No       | NULL     | Employee-requested correction on an already-approved record, routed through an approval chain.   | Sensitive HR data         |
| status                                             | TEXT         | Yes      | approved | `pending`/`approved`/`rejected` — auto-approved for system records, pending for regularizations. | Operational               |
| approved_by / approved_at                          | mixed        | No       | NULL     | Approval trail.                                                                                  | Operational               |

### hrm_attendance_periods

| Column                                                                                                                  | Type    | Required | Default | Business meaning                                                              | Sensitivity |
| ----------------------------------------------------------------------------------------------------------------------- | ------- | -------- | ------- | ----------------------------------------------------------------------------- | ----------- |
| period_year / period_month                                                                                              | INTEGER | Yes      | none    | Unique per org+month.                                                         | Operational |
| status                                                                                                                  | TEXT    | Yes      | open    | `open` → `finalized` (payroll can run) → `locked` (payslips paid, immutable). | Operational |
| total_employees / total_work_days / total_present / total_absent / total_holidays / total_leaves / total_overtime_hours | mixed   | Yes      | 0       | Org-level summary stats, computed at finalization.                            | Operational |
| finalized_at / finalized_by / locked_at / locked_by                                                                     | mixed   | No       | NULL    | Lifecycle trail.                                                              | Operational |

### hrm_payslip_runs

| Column                                                                    | Type    | Required | Default | Business meaning                                                                          | Sensitivity             |
| ------------------------------------------------------------------------- | ------- | -------- | ------- | ----------------------------------------------------------------------------------------- | ----------------------- |
| period_year / period_month                                                | INTEGER | Yes      | none    | Unique per org+month.                                                                     | Operational             |
| description                                                               | TEXT    | No       | NULL    | Free text.                                                                                | Business data           |
| currency                                                                  | TEXT    | Yes      | BDT     | Payout currency.                                                                          | Financial               |
| attendance_period_id                                                      | UUID    | No       | NULL    | Optional link to Group D attendance period — orgs not using attendance tracking may omit. | Internal                |
| total_employees / total_gross_pay / total_deductions / total_net_pay      | mixed   | Yes      | 0       | Aggregate stats, populated at compute time.                                               | Compensation — critical |
| status                                                                    | TEXT    | Yes      | draft   | `draft`/`computing`/`computed`/`approved`/`paid`/`cancelled`.                             | Operational             |
| computed_at / computed_by / approved_at / approved_by / paid_at / paid_by | mixed   | No       | NULL    | Full lifecycle trail.                                                                     | Operational             |

### hrm_payslips

| Column                                                                              | Type          | Required | Default | Business meaning                                             | Sensitivity               |
| ----------------------------------------------------------------------------------- | ------------- | -------- | ------- | ------------------------------------------------------------ | ------------------------- |
| employee_id                                                                         | UUID          | Yes      | none    | Whose payslip (`ON DELETE RESTRICT`).                        | Internal                  |
| payslip_run_id                                                                      | UUID          | Yes      | none    | Owning run; unique per (run, employee).                      | Internal                  |
| period_year / period_month                                                          | INTEGER       | Yes      | none    | Denormalized from the run for query convenience.             | Operational               |
| salary_structure_id / salary_structure_name                                         | mixed         | No       | NULL    | Snapshot at compute time.                                    | Internal                  |
| gross_pay / total_deductions / net_pay / basic_pay                                  | NUMERIC(15,2) | Yes      | 0       | Financial summary, derived from `hrm_payslip_lines`.         | Compensation — critical   |
| work_days / present_days / absent_days / leave_days / holiday_days / overtime_hours | mixed         | Yes      | 0       | Attendance summary for the month, from Group D if available. | Sensitive attendance data |
| currency                                                                            | TEXT          | Yes      | BDT     | Payout currency.                                             | Financial                 |
| status                                                                              | TEXT          | Yes      | draft   | `draft`/`computed`/`approved`/`paid`.                        | Operational               |
| payment_reference / payment_date / paid_at                                          | mixed         | No       | NULL    | Payment tracking.                                            | Compensation — critical   |

### hrm_payslip_lines

No `org_id`-only tenant check needed beyond the FK chain; no `updated_at` — line items are recomputed, not edited.

| Column                          | Type          | Required | Default | Business meaning                                                          | Sensitivity             |
| ------------------------------- | ------------- | -------- | ------- | ------------------------------------------------------------------------- | ----------------------- |
| payslip_id                      | UUID          | Yes      | none    | Owning payslip.                                                           | Internal                |
| component_id                    | UUID          | No       | NULL    | May be NULL if the component was deleted after this payslip was computed. | Internal                |
| component_name / component_type | TEXT          | Yes      | none    | Snapshot at compute time — immune to later component edits/deletes.       | Financial               |
| calc_method                     | TEXT          | Yes      | none    | Snapshot of how the amount was derived.                                   | Financial               |
| formula_used                    | TEXT          | No       | NULL    | Snapshot of the formula expression, for audit.                            | Financial               |
| computed_amount                 | NUMERIC(15,2) | Yes      | 0       | The actual line amount.                                                   | Compensation — critical |
| display_order                   | INTEGER       | Yes      | 0       | From the structure's component ordering.                                  | Operational             |

## Group E — Recognition & Communication

### hrm_awards

| Column                    | Type        | Required | Default          | Business meaning                                                                           | Sensitivity              |
| ------------------------- | ----------- | -------- | ---------------- | ------------------------------------------------------------------------------------------ | ------------------------ |
| employee_id               | UUID        | Yes      | none             | Recipient (`ON DELETE RESTRICT`).                                                          | Internal                 |
| award_type                | TEXT        | Yes      | spot_recognition | `spot_recognition`/`performance`/`tenure`/`team`/`innovation`/`customer_service`/`custom`. | Business data            |
| title / description       | TEXT        | Yes      | none             | Award content.                                                                             | Business data            |
| points                    | INTEGER     | Yes      | 0                | Recognition points, if the org uses a points system.                                       | Business data            |
| monetary_value / currency | mixed       | No       | BDT              | Optional cash component.                                                                   | Compensation — sensitive |
| award_date                | DATE        | Yes      | today            | When granted.                                                                              | Operational              |
| issued_by                 | UUID        | Yes      | none             | Who nominated/issued.                                                                      | Internal                 |
| approval_instance_id      | UUID        | No       | NULL             | Optional approval chain.                                                                   | Internal                 |
| certificate_document_id   | UUID        | No       | NULL             | Auto-generated certificate, if a template is configured.                                   | Internal                 |
| announcement_id           | UUID        | No       | NULL             | Linked announcement; FK added in a later migration (`00042`).                              | Internal                 |
| status                    | TEXT        | Yes      | draft            | `draft`/`pending_approval`/`approved`/`issued`/`cancelled`.                                | Operational              |
| issued_at                 | TIMESTAMPTZ | No       | NULL             | Formal issuance time.                                                                      | Operational              |

### hrm_announcements

| Column                                              | Type        | Required | Default      | Business meaning                                                             | Sensitivity   |
| --------------------------------------------------- | ----------- | -------- | ------------ | ---------------------------------------------------------------------------- | ------------- |
| title / content                                     | TEXT        | Yes      | none         | Markdown-supported body.                                                     | Business data |
| category                                            | TEXT        | Yes      | general      | `general`/`policy`/`event`/`award`/`reminder`/`emergency`/`hr_update`.       | Business data |
| scope_type                                          | TEXT        | Yes      | organization | `organization`/`department`/`individual`.                                    | Operational   |
| scope_ids                                           | UUID[]      | Yes      | `{}`         | Target IDs; empty = whole org.                                               | Internal      |
| scheduled_at / published_at / expires_at            | TIMESTAMPTZ | No       | NULL         | Scheduling — NULL `scheduled_at` means publish immediately on status change. | Operational   |
| requires_acknowledgement / acknowledgement_deadline | mixed       | No       | false / NULL | Ties into `hrm_acknowledgements`.                                            | Policy config |
| is_pinned / pin_order                               | mixed       | Yes      | false / 0    | Visibility controls.                                                         | Operational   |
| author_id                                           | UUID        | Yes      | none         | Who wrote it.                                                                | Internal      |
| status                                              | TEXT        | Yes      | draft        | `draft`/`scheduled`/`published`/`expired`/`archived`.                        | Operational   |

### hrm_calendar_events

HR events calendar — distinct from the Group A holiday calendar (`hrm_holiday_calendars`/`hrm_holidays`).

| Column                        | Type    | Required       | Default             | Business meaning                                                                                     | Sensitivity   |
| ----------------------------- | ------- | -------------- | ------------------- | ---------------------------------------------------------------------------------------------------- | ------------- |
| title / description           | TEXT    | title required | none                | Event content.                                                                                       | Business data |
| event_type                    | TEXT    | Yes            | company_event       | `holiday`/`training`/`company_event`/`team_event`/`deadline`/`birthday`/`work_anniversary`/`custom`. | Business data |
| start_date / end_date         | DATE    | Yes            | none                | Inclusive range; `CHECK (end_date >= start_date)`.                                                   | Operational   |
| is_all_day                    | BOOLEAN | Yes            | true                | Whether `start_time`/`end_time` apply.                                                               | Operational   |
| start_time / end_time         | TIME    | No             | NULL                | NULL if `is_all_day`.                                                                                | Operational   |
| location                      | TEXT    | No             | NULL                | Event location.                                                                                      | Business data |
| scope_type / scope_ids        | mixed   | Yes            | organization / `{}` | Same polymorphic scope pattern as announcements.                                                     | Internal      |
| requires_rsvp / rsvp_deadline | mixed   | No             | false / NULL        | Ties into `hrm_acknowledgements`.                                                                    | Policy config |
| organizer_id                  | UUID    | No             | NULL                | Who's running it.                                                                                    | Internal      |
| is_auto_generated             | BOOLEAN | Yes            | false               | Whether the milestone engine or holiday sync created this row.                                       | Operational   |
| source / source_id            | mixed   | No             | NULL                | `milestone`/`holiday_calendar`/`manual`; ID of the originating record.                               | Internal      |
| status                        | TEXT    | Yes            | upcoming            | `upcoming`/`ongoing`/`completed`/`cancelled`.                                                        | Operational   |

### hrm_employee_milestones

| Column                                                        | Type    | Required       | Default      | Business meaning                                                                                         | Sensitivity       |
| ------------------------------------------------------------- | ------- | -------------- | ------------ | -------------------------------------------------------------------------------------------------------- | ----------------- |
| employee_id                                                   | UUID    | Yes            | none         | Whose milestone (`ON DELETE RESTRICT`).                                                                  | Internal          |
| milestone_type                                                | TEXT    | Yes            | none         | `work_anniversary`/`birthday`/`probation_complete`/`promotion`/`contract_renewal`/`retirement`/`custom`. | Sensitive HR data |
| title / description                                           | TEXT    | title required | none         | Milestone content.                                                                                       | Business data     |
| milestone_date                                                | DATE    | Yes            | none         | Unique per (employee, type, date).                                                                       | Sensitive HR data |
| years_count                                                   | INTEGER | No             | NULL         | e.g. 5 for a 5-year anniversary; NULL for non-anniversary types.                                         | Sensitive HR data |
| is_auto_generated                                             | BOOLEAN | Yes            | false        | Whether the nightly cron created this.                                                                   | Operational       |
| auto_award_id / auto_announcement_id / auto_calendar_event_id | UUID    | No             | NULL         | Cascade links to records this milestone auto-drafted.                                                    | Internal          |
| is_acknowledged / acknowledged_at                             | mixed   | No             | false / NULL | Whether HR/employee acknowledged or celebrated it.                                                       | Operational       |
