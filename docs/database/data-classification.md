# Data Classification

This document classifies database tables by business and security sensitivity.

| Table | Sensitivity | Why | Required controls |
| --- | --- | --- | --- |
| users | High | Contains identity, contact, profile, 2FA, lockout, and preference data. | Never store raw passwords. Encrypt 2FA secrets and backup codes. |
| organizations | Medium | Contains business identity/workspace data. | Legal name and metadata may be sensitive. |
| permissions | Low/Operational | Authorization vocabulary. | Should be controlled by migration/seed process. |
| roles | High | Controls access to resources. | Role changes affect authorization and must be audited. |
| organization_members | High | Connects users to tenants and access levels. | Membership/role changes must be audited. |
| auth_accounts | Critical | Can contain OAuth/OIDC tokens. | Encrypt tokens. Minimize scopes. Rotate where possible. |
| sessions | High/Critical | Contains token hashes, IP, device, location data. | Never store raw tokens. Set expiry and cleanup. |
| verification_tokens | Critical | Contains one-time token hashes and email addresses. | Never store raw tokens. Enforce expiry and single-use. |
| subscriptions | High | Billing state and external customer/subscription ids. | Protect payment provider identifiers. |
| organization_usage | Medium | Usage counters and plan limits. | Useful for billing enforcement; keep accurate. |
| audit_logs | High | Security/business event trail with IP/user agent/changes. | Avoid secrets in metadata/changes. Define retention policy. |

## Critical controls

- Passwords: store only strong password hashes.
- OAuth tokens: encrypt at application/storage layer before production.
- Session and verification tokens: store only hashes, never raw tokens.
- 2FA secrets and backup codes: encrypt or hash depending on flow requirements.
- Audit logs: do not write passwords, raw tokens, full authorization headers, or unnecessary secrets into `changes` or `metadata`.
- PII fields: restrict access through RBAC and API-level checks.
