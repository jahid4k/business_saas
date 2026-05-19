# Data Dictionary

This file combines all table-level column dictionaries. For deeper business explanation, see `tables/*.md`.

## users

Stores registered BusinessSAAS user accounts, profile data, login/security state, and user-level preferences.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal primary key for joins and foreign keys. | Internal |
| public_id | TEXT | Yes | 'usr_' + generated UUID | Stable API-facing user identifier. | Public identifier |
| email | TEXT | No | NULL | Login/contact email; nullable for OAuth-only or incomplete provider data. | PII |
| password_hash | TEXT | No | NULL | Hashed password for credential login. | Critical secret |
| username | TEXT | No | NULL | Optional unique handle for the user. | PII/public |
| display_name | TEXT | Yes | '' | Name shown in the UI. | PII |
| first_name | TEXT | Yes | '' | User first/given name. | PII |
| last_name | TEXT | Yes | '' | User last/family name. | PII |
| full_name | TEXT | Yes | '' | Full name for display/search/export. | PII |
| photo_url | TEXT | No | NULL | Profile image URL. | PII |
| cover_photo_url | TEXT | No | NULL | Profile cover image URL. | PII |
| phone | TEXT | No | NULL | User phone number. | PII |
| phone_verified | BOOLEAN | Yes | false | Whether phone number ownership is verified. | Security/PII |
| email_verified | BOOLEAN | Yes | false | Whether email ownership is verified. | Security |
| email_verified_at | TIMESTAMPTZ | No | NULL | Timestamp when email verification completed. | Security |
| country | CHAR(2) | No | NULL | ISO country code for localization/compliance. | Location/PII |
| timezone | TEXT | Yes | 'UTC' | Preferred timezone for displaying dates. | Preference |
| locale | TEXT | Yes | 'en' | Preferred locale for formatting. | Preference |
| language | TEXT | Yes | 'en' | Preferred language. | Preference |
| currency | TEXT | Yes | 'USD' | Preferred/default currency. | Preference |
| status | TEXT | Yes | 'active' | Account lifecycle state: active, suspended, deleted, pending_verification. | Operational |
| account_type | TEXT | Yes | 'saas_customer' | Classifies account type for future product/business logic. | Operational |
| suspended_at | TIMESTAMPTZ | No | NULL | When the account was suspended. | Security |
| suspension_reason | TEXT | No | NULL | Reason for suspension. | Sensitive operational |
| login_redirect_url | TEXT | Yes | '/dashboard' | Default post-login destination. | Operational |
| shortcuts | TEXT[] | Yes | empty array | Fuse app shortcut identifiers. | Preference |
| settings | JSONB | Yes | {} | Fuse-compatible UI/user settings. | Preference |
| preferences | JSONB | Yes | {} | Product preferences. | Preference |
| onboarding | JSONB | Yes | {} | Onboarding progress and completed steps. | Operational |
| feature_flags | JSONB | Yes | {} | Per-user feature flag overrides. | Operational |
| two_fa_enabled | BOOLEAN | Yes | false | Whether two-factor authentication is enabled. | Security |
| two_fa_secret | TEXT | No | NULL | 2FA secret; must be encrypted at rest. | Critical secret |
| backup_codes | JSONB | Yes | [] | Hashed/encrypted backup codes for 2FA recovery. | Critical secret |
| last_login_at | TIMESTAMPTZ | No | NULL | Last successful login time. | Security |
| last_activity_at | TIMESTAMPTZ | No | NULL | Last meaningful product activity time. | Operational |
| failed_logins | INTEGER | Yes | 0 | Consecutive failed login counter for lockout. | Security |
| locked_until | TIMESTAMPTZ | No | NULL | Temporary account lock expiry. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |
| deleted_at | TIMESTAMPTZ | No | NULL | Soft delete timestamp. | Operational |

## organizations

Stores SaaS tenant/workspace/company records.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal organization primary key. | Internal |
| public_id | TEXT | Yes | 'org_' + generated UUID | API-facing organization identifier. | Public identifier |
| name | TEXT | Yes | none | Organization display name. | Business data |
| slug | TEXT | Yes | none | Unique workspace slug used in URLs. | Public/business |
| legal_name | TEXT | No | NULL | Official legal business name. | Business sensitive |
| type | TEXT | No | NULL | Organization type/category. | Business data |
| industry | TEXT | No | NULL | Industry classification. | Business data |
| website | TEXT | No | NULL | Company website. | Public/business |
| logo_url | TEXT | No | NULL | Organization logo URL. | Public/business |
| country | CHAR(2) | No | NULL | Organization country code. | Business/location |
| timezone | TEXT | Yes | 'UTC' | Default organization timezone. | Operational |
| currency | TEXT | Yes | 'USD' | Default billing/display currency. | Operational |
| status | TEXT | Yes | 'active' | Tenant lifecycle: active, suspended, deleted. | Operational |
| metadata | JSONB | Yes | {} | Flexible organization-level metadata. | Varies |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |
| deleted_at | TIMESTAMPTZ | No | NULL | Soft delete timestamp. | Operational |

## permissions

Stores canonical permission keys used by roles and authorization checks.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal permission primary key. | Internal |
| public_id | TEXT | Yes | 'perm_' + generated UUID | API-facing permission identifier. | Public identifier |
| key | TEXT | Yes | none | Dot-format permission key. | Operational |
| resource | TEXT | Yes | none | Resource/domain controlled by permission. | Operational |
| action | TEXT | Yes | none | Action allowed on the resource. | Operational |
| description | TEXT | No | NULL | Human-readable explanation. | Operational |
| is_system | BOOLEAN | Yes | true | Marks platform-defined permission. | Operational |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## roles

Stores system role templates and organization-specific custom roles.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal role primary key. | Internal |
| public_id | TEXT | Yes | 'role_' + generated UUID | API-facing role identifier. | Public identifier |
| org_id | UUID | No | NULL | Tenant owner; NULL for global templates. | Internal/business |
| name | TEXT | Yes | none | Role name such as owner/admin/member. | Operational |
| description | TEXT | No | NULL | Human-readable role explanation. | Operational |
| permissions | TEXT[] | Yes | empty array | Permission keys included in this role. | Security/authorization |
| is_system | BOOLEAN | Yes | false | Whether this is a platform-defined role. | Operational |
| is_custom | BOOLEAN | Yes | false | Whether tenant created/customized this role. | Operational |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## organization_members

Connects users to organizations and stores organization-specific access, role, title, department, and invitation state.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal membership primary key. | Internal |
| public_id | TEXT | Yes | 'mem_' + generated UUID | API-facing membership identifier. | Public identifier |
| org_id | UUID | Yes | none | Organization this member belongs to. | Internal |
| user_id | UUID | Yes | none | User who is a member. | Internal |
| role_id | UUID | No | NULL | Optional FK to roles table. | Internal/authorization |
| role_key | TEXT | Yes | 'member' | Role snapshot used by API/session. | Authorization |
| title | TEXT | No | NULL | Job title inside organization. | PII/business |
| department | TEXT | No | NULL | Department/team inside organization. | Business data |
| status | TEXT | Yes | 'active' | Membership status: active, inactive, suspended. | Authorization |
| custom_permissions | TEXT[] | Yes | empty array | Extra permission keys directly granted to member. | Authorization/security |
| invitation_status | TEXT | Yes | 'accepted' | Invitation state: pending, accepted, rejected, expired. | Operational |
| invited_by | UUID | No | NULL | User who invited this member. | Audit/PII |
| invitation_sent_at | TIMESTAMPTZ | No | NULL | When invitation was sent. | Operational |
| invitation_accepted_at | TIMESTAMPTZ | No | NULL | When invitation was accepted. | Operational |
| joined_at | TIMESTAMPTZ | Yes | now() | When user joined organization. | Operational |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## auth_accounts

Stores linked authentication provider accounts such as credentials, Google, Facebook, GitHub, OIDC, email, and WebAuthn.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal account link primary key. | Internal |
| public_id | TEXT | Yes | 'acct_' + generated UUID | API-facing account-link identifier. | Public identifier |
| user_id | UUID | Yes | none | Internal user this provider account belongs to. | Internal |
| provider | TEXT | Yes | none | Provider id such as credentials/google/facebook. | Operational |
| provider_account_id | TEXT | Yes | none | Unique account id received from provider. | Sensitive identifier |
| provider_type | TEXT | Yes | 'oauth' | Provider type: oauth, oidc, credentials, email, webauthn. | Operational |
| access_token | TEXT | No | NULL | OAuth access token. | Critical secret |
| refresh_token | TEXT | No | NULL | OAuth refresh token. | Critical secret |
| id_token | TEXT | No | NULL | OIDC ID token. | Critical secret |
| token_type | TEXT | No | NULL | OAuth token type. | Operational |
| scope | TEXT | No | NULL | Granted provider scopes. | Security |
| expires_at | TIMESTAMPTZ | No | NULL | Provider token expiry time. | Security |
| connected_at | TIMESTAMPTZ | Yes | now() | When provider was linked. | Operational |
| last_used_at | TIMESTAMPTZ | No | NULL | Last time this provider was used. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## sessions

Stores active and historical user sessions/devices with revocation support.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal session primary key. | Internal |
| public_id | TEXT | Yes | 'sess_' + generated UUID | API-facing session identifier. | Public identifier |
| user_id | UUID | Yes | none | User who owns the session. | Internal |
| org_id | UUID | No | NULL | Organization context for session if applicable. | Internal |
| token_hash | TEXT | Yes | none | Hash of refresh/session token. | Critical secret |
| device_name | TEXT | No | NULL | Human-friendly device name. | Device data |
| device_type | TEXT | No | NULL | Desktop/mobile/tablet/etc. | Device data |
| browser | TEXT | No | NULL | Browser name/version. | Device data |
| os | TEXT | No | NULL | Operating system. | Device data |
| user_agent | TEXT | No | NULL | Full user-agent string. | Device/PII-like |
| ip_address | INET | No | NULL | IP address at session creation/activity. | PII/security |
| country | TEXT | No | NULL | GeoIP country. | Location/PII |
| city | TEXT | No | NULL | GeoIP city. | Location/PII |
| region | TEXT | No | NULL | GeoIP region. | Location/PII |
| last_activity_at | TIMESTAMPTZ | Yes | now() | Last activity timestamp. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Session creation time. | Operational |
| expires_at | TIMESTAMPTZ | Yes | none | Session expiry timestamp. | Security |
| revoked_at | TIMESTAMPTZ | No | NULL | When user/admin revoked the session. | Security |

## verification_tokens

Stores one-time email verification, password reset, magic link, invitation, and 2FA tokens.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal verification token primary key. | Internal |
| public_id | TEXT | Yes | 'vt_' + generated UUID | API-facing token record id. | Public identifier |
| user_id | UUID | No | NULL | Related user if known. | Internal |
| email | TEXT | No | NULL | Email target for token. | PII |
| token_hash | TEXT | Yes | none | Hash of one-time token. | Critical secret |
| type | TEXT | Yes | none | Token purpose. | Security |
| verified_at | TIMESTAMPTZ | No | NULL | When verification completed. | Security |
| used_at | TIMESTAMPTZ | No | NULL | When token was consumed. | Security |
| expires_at | TIMESTAMPTZ | Yes | none | Expiry time. | Security |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |

## subscriptions

Stores organization subscription and billing state.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal subscription primary key. | Internal |
| public_id | TEXT | Yes | 'sub_' + generated UUID | API-facing subscription id. | Public identifier |
| org_id | UUID | Yes | none | Organization that owns subscription. | Internal |
| plan | TEXT | Yes | 'free' | Machine-readable plan: free/pro/business/enterprise. | Operational/billing |
| plan_name | TEXT | Yes | 'Free' | Human-readable plan label. | Operational/billing |
| status | TEXT | Yes | 'active' | Billing lifecycle: trialing, active, past_due, cancelled, expired. | Billing |
| billing_cycle | TEXT | No | NULL | monthly/yearly/lifetime. | Billing |
| currency | TEXT | Yes | 'USD' | Billing currency. | Billing |
| amount | NUMERIC(12,2) | Yes | 0 | Subscription price amount. | Billing |
| trial_started_at | TIMESTAMPTZ | No | NULL | Trial start time. | Billing |
| trial_ends_at | TIMESTAMPTZ | No | NULL | Trial end time. | Billing |
| current_period_start | TIMESTAMPTZ | No | NULL | Current billing period start. | Billing |
| current_period_end | TIMESTAMPTZ | No | NULL | Current billing period end. | Billing |
| cancel_at_period_end | BOOLEAN | Yes | false | Whether cancellation is scheduled for period end. | Billing |
| payment_provider | TEXT | No | NULL | External payment provider name. | Billing |
| payment_customer_id | TEXT | No | NULL | External customer id. | Sensitive billing identifier |
| payment_subscription_id | TEXT | No | NULL | External subscription id. | Sensitive billing identifier |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## organization_usage

Stores per-period organization usage and plan limits.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal usage primary key. | Internal |
| public_id | TEXT | Yes | 'usage_' + generated UUID | API-facing usage id. | Public identifier |
| org_id | UUID | Yes | none | Organization whose usage is tracked. | Internal |
| subscription_id | UUID | No | NULL | Related subscription for the period. | Internal/billing |
| period_start | TIMESTAMPTZ | Yes | none | Usage period start. | Billing/operational |
| period_end | TIMESTAMPTZ | Yes | none | Usage period end. | Billing/operational |
| limits | JSONB | Yes | {} | Plan limit object such as members/projects/storageGB. | Operational/billing |
| used | JSONB | Yes | {} | Actual usage object such as projects/storage/api requests. | Operational/billing |
| created_at | TIMESTAMPTZ | Yes | now() | Record creation time. | Operational |
| updated_at | TIMESTAMPTZ | Yes | now() | Record update time. | Operational |

## audit_logs

Stores security and business audit events.

| Column | Type | Required | Default | Business meaning | Sensitivity |
| --- | --- | --- | --- | --- | --- |
| id | UUID | Yes | gen_random_uuid() | Internal audit log primary key. | Internal |
| public_id | TEXT | Yes | 'audit_' + generated UUID | API-facing audit event id. | Public identifier |
| org_id | UUID | No | NULL | Organization context of event. | Internal/business |
| user_id | UUID | No | NULL | Actor user if known. | Internal/PII link |
| event_type | TEXT | Yes | none | Event key such as auth.sign_in or billing.subscription_changed. | Operational/security |
| description | TEXT | No | NULL | Human-readable event summary. | Operational |
| resource_type | TEXT | No | NULL | Type of resource affected. | Operational |
| resource_id | TEXT | No | NULL | Public or internal resource id affected. | Operational |
| changes | JSONB | No | NULL | Before/after change details. | Potentially sensitive |
| metadata | JSONB | Yes | {} | Additional structured context. | Potentially sensitive |
| ip_address | INET | No | NULL | IP address of actor/request. | PII/security |
| user_agent | TEXT | No | NULL | User-agent string. | Device/PII-like |
| status | TEXT | No | NULL | Event outcome: success/failure/warning. | Operational/security |
| error_message | TEXT | No | NULL | Error message when event failed. | Sensitive operational |
| created_at | TIMESTAMPTZ | Yes | now() | Event time. | Operational |
