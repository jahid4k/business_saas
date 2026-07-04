// src/lib/queryKeys.ts
//
// Centralised TanStack Query key factory — see docs/decisions/0007-state-management.md.
//
// Convention: [domain, orgId, resource, ...params] as const.
// Always go through this file instead of writing array literals inline —
// it's what makes `invalidateQueries` predictable as more modules
// (HRM, Accounting, Projects, E-commerce) land on the same platform-layer
// entities (contacts, companies, engagement).

export const queryKeys = {
  // Not org-scoped — /api/v1/me is per-user, independent of which
  // organization is currently active.
  profile: {
    me: () => ["profile", "me"] as const,
    avatars: () => ["profile", "avatars"] as const,
  },

  tasks: {
    all: (orgId: string) => ["tasks", orgId] as const,
    list: (orgId: string) => ["tasks", orgId, "list"] as const,
    detail: (orgId: string, taskId: string) =>
      ["tasks", orgId, taskId] as const,
  },

  members: {
    all: (orgId: string) => ["members", orgId] as const,
    list: (orgId: string) => ["members", orgId, "list"] as const,
    me: (orgId: string) => ["members", orgId, "me"] as const,
  },

  roles: {
    all: (orgId: string) => ["roles", orgId] as const,
    list: (orgId: string) => ["roles", orgId, "list"] as const,
    detail: (orgId: string, roleId: string) =>
      ["roles", orgId, roleId] as const,
    permissions: (orgId: string) => ["roles", orgId, "permissions"] as const,
    permissionsMatrix: (orgId: string) =>
      ["roles", orgId, "permissions", "matrix"] as const,
  },

  security: {
    sessions: (orgId: string) => ["security", orgId, "sessions"] as const,
    loginEvents: (orgId: string) =>
      ["security", orgId, "login-events"] as const,
  },

  crm: {
    leads: {
      all: (orgId: string) => ["crm", orgId, "leads"] as const,
      list: (orgId: string) => ["crm", orgId, "leads", "list"] as const,
      detail: (orgId: string, leadId: string) =>
        ["crm", orgId, "leads", leadId] as const,
    },

    contacts: {
      all: (orgId: string) => ["crm", orgId, "contacts"] as const,
      list: (orgId: string) => ["crm", orgId, "contacts", "list"] as const,
      detail: (orgId: string, contactId: string) =>
        ["crm", orgId, "contacts", contactId] as const,
    },

    companies: {
      all: (orgId: string) => ["crm", orgId, "companies"] as const,
      list: (orgId: string) => ["crm", orgId, "companies", "list"] as const,
      detail: (orgId: string, companyId: string) =>
        ["crm", orgId, "companies", companyId] as const,
      contacts: (orgId: string, companyId: string) =>
        ["crm", orgId, "companies", companyId, "contacts"] as const,
    },

    pipelines: {
      all: (orgId: string) => ["crm", orgId, "pipelines"] as const,
      list: (orgId: string) => ["crm", orgId, "pipelines", "list"] as const,
      stages: (orgId: string, pipelineId: string) =>
        ["crm", orgId, "pipelines", pipelineId, "stages"] as const,
    },

    deals: {
      all: (orgId: string) => ["crm", orgId, "deals"] as const,
      list: (orgId: string) => ["crm", orgId, "deals", "list"] as const,
      detail: (orgId: string, dealId: string) =>
        ["crm", orgId, "deals", dealId] as const,
      board: (orgId: string, pipelineId: string) =>
        ["crm", orgId, "deals", "board", pipelineId] as const,
    },

    reports: {
      overview: (orgId: string) =>
        ["crm", orgId, "reports", "overview"] as const,
      dealsByStage: (orgId: string) =>
        ["crm", orgId, "reports", "deals-by-stage"] as const,
      leadsBySource: (orgId: string) =>
        ["crm", orgId, "reports", "leads-by-source"] as const,
    },
  },
} as const;
