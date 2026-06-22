// lib/query-keys.ts
// Centralised query key factory — prevents typos and makes invalidation predictable.
// Import this everywhere instead of writing raw string arrays.

export const queryKeys = {
  // Auth
  me: () => ["me"] as const,

  // Organizations
  orgs: {
    list: () => ["orgs"] as const,
    one: (id: string) => ["orgs", id] as const,
  },

  // Members
  members: {
    list: (orgId: string) => ["members", orgId] as const,
    me: (orgId: string) => ["members", orgId, "me"] as const,
  },

  // RBAC
  roles: {
    list: (orgId?: string) => ["roles", orgId] as const,
    one: (orgId: string, roleId: string) => ["roles", orgId, roleId] as const,
  },
  permissions: {
    list: (orgId?: string) => ["permissions", orgId] as const,
    matrix: (orgId: string) => ["permissions", orgId, "matrix"] as const,
  },

  // Tasks
  tasks: {
    list: (orgId: string, params?: object) =>
      params ? ["tasks", orgId, params] : (["tasks", orgId] as const),
    one: (orgId: string, id: string) => ["tasks", orgId, id] as const,
  },

  // CRM — Contacts
  contacts: {
    list: (orgId: string, params?: object) =>
      params
        ? ["crm", orgId, "contacts", params]
        : (["crm", orgId, "contacts"] as const),
    one: (orgId: string, id: string) =>
      ["crm", orgId, "contacts", id] as const,
  },

  // CRM — Companies
  companies: {
    list: (orgId: string, params?: object) =>
      params
        ? ["crm", orgId, "companies", params]
        : (["crm", orgId, "companies"] as const),
    one: (orgId: string, id: string) =>
      ["crm", orgId, "companies", id] as const,
  },

  // CRM — Leads
  leads: {
    list: (orgId: string, params?: object) =>
      params
        ? ["crm", orgId, "leads", params]
        : (["crm", orgId, "leads"] as const),
    one: (orgId: string, id: string) => ["crm", orgId, "leads", id] as const,
  },

  // CRM — Pipelines + Stages
  pipelines: {
    list: (orgId: string) => ["crm", orgId, "pipelines"] as const,
    one: (orgId: string, id: string) =>
      ["crm", orgId, "pipelines", id] as const,
    stages: (orgId: string, pipelineId: string) =>
      ["crm", orgId, "pipelines", pipelineId, "stages"] as const,
  },

  // CRM — Deals
  deals: {
    list: (orgId: string, params?: object) =>
      params
        ? ["crm", orgId, "deals", params]
        : (["crm", orgId, "deals"] as const),
    one: (orgId: string, id: string) => ["crm", orgId, "deals", id] as const,
  },

  // CRM — Reports
  reports: {
    overview: (orgId: string) => ["crm", orgId, "reports", "overview"] as const,
    dealsByStage: (orgId: string) =>
      ["crm", orgId, "reports", "deals-by-stage"] as const,
  },
} as const;
