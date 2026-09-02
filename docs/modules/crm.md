# Module: CRM

> ⚑ One of the two modules in the mission **CRM + HRM → deploy → business**
> (`docs/Project_Instruction.md` § Mission). Extended CRM is planned in
> `docs/CrmExtendedBuildPlan.md`.
>
> ⚠ **This document describes what exists TODAY, which is production-shaped but not
> production-grade.** Known gaps, verified 2026-09-01: `crm_deals.value` is `float64` in Go against
> `NUMERIC(15,2)`; CRM has **no scope tiers** (every reader sees every deal); `crm_leads.status` is
> a hardcoded CHECK so stages are not configurable, though deal stages are; there is no
> `crm_deal_stage_history`, which blocks sales velocity; and `crm.activities`, `crm.emails`,
> `crm.notes` and `crm.tasks` are seeded permissions with no consumer.

## What this module does

The CRM module manages the sales pipeline: leads, contacts, companies, deals, and reporting.
It is the first major business module in BusinessSAAS and sets the pattern for all future modules.

All CRM endpoints are tenant-scoped — they require a JWT with `bid` (org ID) embedded,
and the `:orgId` URL parameter must match the JWT `bid`. The backend enforces this via
`RequireOrganizationParam` middleware.

## Backend endpoints

### Contacts

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/organizations/:orgId/crm/contacts` | `crm.contacts.view` |
| POST | `/api/v1/organizations/:orgId/crm/contacts` | `crm.contacts.create` |
| GET | `/api/v1/organizations/:orgId/crm/contacts/:contactId` | `crm.contacts.view` |
| PATCH | `/api/v1/organizations/:orgId/crm/contacts/:contactId` | `crm.contacts.update` |
| DELETE | `/api/v1/organizations/:orgId/crm/contacts/:contactId` | `crm.contacts.delete` |

### Companies

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/organizations/:orgId/crm/companies` | `crm.companies.view` |
| POST | `/api/v1/organizations/:orgId/crm/companies` | `crm.companies.create` |
| GET | `/api/v1/organizations/:orgId/crm/companies/:companyId` | `crm.companies.view` |
| PATCH | `/api/v1/organizations/:orgId/crm/companies/:companyId` | `crm.companies.update` |
| DELETE | `/api/v1/organizations/:orgId/crm/companies/:companyId` | `crm.companies.delete` |
| GET | `/api/v1/organizations/:orgId/crm/companies/:companyId/contacts` | `crm.contacts.view` |

### Leads

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/organizations/:orgId/crm/leads` | `crm.leads.view` |
| POST | `/api/v1/organizations/:orgId/crm/leads` | `crm.leads.create` |
| GET | `/api/v1/organizations/:orgId/crm/leads/:leadId` | `crm.leads.view` |
| PATCH | `/api/v1/organizations/:orgId/crm/leads/:leadId` | `crm.leads.update` |
| DELETE | `/api/v1/organizations/:orgId/crm/leads/:leadId` | `crm.leads.delete` |
| POST | `/api/v1/organizations/:orgId/crm/leads/:leadId/convert` | `crm.leads.convert` |

### Pipelines + Stages

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/organizations/:orgId/crm/pipelines` | `crm.deals.view` |
| POST | `/api/v1/organizations/:orgId/crm/pipelines` | `crm.deals.create` |
| GET | `/api/v1/organizations/:orgId/crm/pipelines/:pipelineId` | `crm.deals.view` |
| PATCH | `/api/v1/organizations/:orgId/crm/pipelines/:pipelineId` | `crm.deals.update` |
| DELETE | `/api/v1/organizations/:orgId/crm/pipelines/:pipelineId` | `crm.deals.delete` |
| GET | `/api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages` | `crm.deals.view` |
| POST | `/api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages` | `crm.deals.create` |
| POST | `/api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/reorder` | `crm.deals.update` |
| PATCH | `/api/v1/organizations/:orgId/crm/pipelines/:pipelineId/stages/:stageId` | `crm.deals.update` |

### Deals

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/organizations/:orgId/crm/deals` | `crm.deals.view` |
| POST | `/api/v1/organizations/:orgId/crm/deals` | `crm.deals.create` |
| GET | `/api/v1/organizations/:orgId/crm/deals/:dealId` | `crm.deals.view` |
| PATCH | `/api/v1/organizations/:orgId/crm/deals/:dealId` | `crm.deals.update` |
| DELETE | `/api/v1/organizations/:orgId/crm/deals/:dealId` | `crm.deals.delete` |
| POST | `/api/v1/organizations/:orgId/crm/deals/:dealId/move` | `crm.deals.move_stage` |
| POST | `/api/v1/organizations/:orgId/crm/deals/:dealId/won` | `crm.deals.update` |
| POST | `/api/v1/organizations/:orgId/crm/deals/:dealId/lost` | `crm.deals.update` |

### Reports

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/organizations/:orgId/crm/reports/overview` | `crm.reports.view` |
| GET | `/api/v1/organizations/:orgId/crm/reports/deals/by-stage` | `crm.reports.view` |
| GET | `/api/v1/organizations/:orgId/crm/reports/deals/by-owner` | `crm.reports.view` |
| GET | `/api/v1/organizations/:orgId/crm/reports/leads/by-source` | `crm.reports.view` |
| GET | `/api/v1/organizations/:orgId/crm/reports/tasks/overdue` | `crm.reports.view` |

## Frontend pages

⚠ **Corrected 2026-09-01** — the tree below was documented as `app/(app)/[orgSlug]/crm/` with
`deals/` and `contacts/` subtrees that do not exist. This is what is actually on disk:

```
app/(dashboard)/[orgId]/
├── crm/
│   ├── pipeline/page.tsx      → deal pipeline board (1305 lines)
│   ├── leads/page.tsx         → leads table with filters
│   ├── agenda/page.tsx        → activity agenda
│   ├── reports/page.tsx       → charts + tables
│   ├── visitors/page.tsx      → capture / visitor tracking
│   └── setup/
│       ├── routing/page.tsx   → lead assignment rules
│       └── templates/page.tsx → message templates
├── contacts/page.tsx          → contacts (top level, not under crm/)
└── companies/
    ├── page.tsx
    └── [companyId]/page.tsx
```

⚠ There is **no `crm/deals/[dealId]` detail page** — the pipeline board is the only deal surface.

## Lead conversion flow

Lead conversion is an atomic operation — it creates a Contact, a Company (if new), and a Deal
in a single database transaction. The frontend sends one request:

```typescript
POST /crm/leads/:leadId/convert
{
  "pipeline_id": "...",
  "stage_id": "...",
  "deal_title": "Acme Corp — Enterprise License",
  "deal_value": 25000,
  "create_contact": true,   // create a Contact from the lead's info
  "create_company": true    // create a Company if company_name was set
}
```

On success, the response includes the created `deal_id`. The frontend navigates to the deal:
`router.push(`/app/${orgSlug}/crm/deals/${dealId}`)`.

## TanStack Query keys

```typescript
export const crmKeys = {
  contacts: (orgId: string) => ['crm', orgId, 'contacts'] as const,
  contact: (orgId: string, id: string) => ['crm', orgId, 'contacts', id] as const,
  companies: (orgId: string) => ['crm', orgId, 'companies'] as const,
  leads: (orgId: string, params?: object) => ['crm', orgId, 'leads', params] as const,
  lead: (orgId: string, id: string) => ['crm', orgId, 'leads', id] as const,
  pipelines: (orgId: string) => ['crm', orgId, 'pipelines'] as const,
  deals: (orgId: string, params?: object) => ['crm', orgId, 'deals', params] as const,
  reports: (orgId: string, type: string) => ['crm', orgId, 'reports', type] as const,
}
```

After converting a lead:
```typescript
queryClient.invalidateQueries({ queryKey: crmKeys.leads(orgId) })
queryClient.invalidateQueries({ queryKey: crmKeys.deals(orgId) })
queryClient.invalidateQueries({ queryKey: crmKeys.contacts(orgId) })
```

## Backend architecture (for reference)

The CRM module uses a shared platform layer for contacts and engagement records:

```
internal/platform/contacts/    ← shared contacts + companies storage
internal/platform/engagement/  ← shared notes, tasks, activities, email logs
internal/crm/leads/            ← lead-specific logic (uses platform contacts)
internal/crm/pipeline/         ← pipeline + stage management
internal/crm/deals/            ← deal CRUD + stage movement (uses platform engagement)
internal/crm/reports/          ← aggregation queries
```

This architecture means HRM, ERP, and other modules can also use the same contacts/engagement
layer without duplicating schema or queries.

## Related ADRs

- [ADR-0007](../decisions/0007-state-management.md) — TanStack Query for all CRM data fetching
- [ADR-0011](../decisions/0011-data-tables.md) — TanStack Table for leads/deals/contacts tables
