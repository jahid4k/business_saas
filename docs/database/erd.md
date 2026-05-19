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
