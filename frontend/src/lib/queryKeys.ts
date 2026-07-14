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
    permissions: (orgId: string, memberId: string) =>
      ["members", orgId, memberId, "permissions"] as const,
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

  hrm: {
    departments: {
      all: (orgId: string) => ["hrm", orgId, "departments"] as const,
      list: (orgId: string) => ["hrm", orgId, "departments", "list"] as const,
      detail: (orgId: string, deptId: string) =>
        ["hrm", orgId, "departments", deptId] as const,
    },
    positions: {
      all: (orgId: string) => ["hrm", orgId, "positions"] as const,
      list: (orgId: string) => ["hrm", orgId, "positions", "list"] as const,
      detail: (orgId: string, posId: string) =>
        ["hrm", orgId, "positions", posId] as const,
    },
    employees: {
      all: (orgId: string) => ["hrm", orgId, "employees"] as const,
      list: (orgId: string) => ["hrm", orgId, "employees", "list"] as const,
      detail: (orgId: string, empId: string) =>
        ["hrm", orgId, "employees", empId] as const,
    },
    leaveTypes: {
      all: (orgId: string) => ["hrm", orgId, "leave-types"] as const,
      list: (orgId: string) => ["hrm", orgId, "leave-types", "list"] as const,
      detail: (orgId: string, typeId: string) =>
        ["hrm", orgId, "leave-types", typeId] as const,
    },
    leaveRequests: {
      all: (orgId: string) => ["hrm", orgId, "leave-requests"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "leave-requests", "list"] as const,
      detail: (orgId: string, reqId: string) =>
        ["hrm", orgId, "leave-requests", reqId] as const,
    },
    promotions: {
      all: (orgId: string) => ["hrm", orgId, "promotions"] as const,
      list: (orgId: string) => ["hrm", orgId, "promotions", "list"] as const,
    },
    transfers: {
      all: (orgId: string) => ["hrm", orgId, "transfers"] as const,
      list: (orgId: string) => ["hrm", orgId, "transfers", "list"] as const,
    },
    resignations: {
      all: (orgId: string) => ["hrm", orgId, "resignations"] as const,
      list: (orgId: string) => ["hrm", orgId, "resignations", "list"] as const,
    },
    terminations: {
      all: (orgId: string) => ["hrm", orgId, "terminations"] as const,
      list: (orgId: string) => ["hrm", orgId, "terminations", "list"] as const,
    },
    attendance: {
      records: (orgId: string, year: number, month: number) =>
        ["hrm", orgId, "attendance", "records", year, month] as const,
      periods: (orgId: string) =>
        ["hrm", orgId, "attendance", "periods"] as const,
    },
    complaints: {
      all: (orgId: string) => ["hrm", orgId, "complaints"] as const,
      list: (orgId: string) => ["hrm", orgId, "complaints", "list"] as const,
    },
    documents: {
      all: (orgId: string) => ["hrm", orgId, "documents"] as const,
      list: (orgId: string) => ["hrm", orgId, "documents", "list"] as const,
    },
    acknowledgements: {
      all: (orgId: string) => ["hrm", orgId, "acknowledgements"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "acknowledgements", "list"] as const,
    },
    awards: {
      all: (orgId: string) => ["hrm", orgId, "awards"] as const,
      list: (orgId: string) => ["hrm", orgId, "awards", "list"] as const,
    },
    announcements: {
      all: (orgId: string) => ["hrm", orgId, "announcements"] as const,
      list: (orgId: string) => ["hrm", orgId, "announcements", "list"] as const,
    },
    calendar: {
      all: (orgId: string) => ["hrm", orgId, "calendar"] as const,
      list: (orgId: string) => ["hrm", orgId, "calendar", "list"] as const,
    },
    milestones: {
      all: (orgId: string) => ["hrm", orgId, "milestones"] as const,
      list: (orgId: string) => ["hrm", orgId, "milestones", "list"] as const,
    },
    salaryComponents: {
      all: (orgId: string) => ["hrm", orgId, "salary-components"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "salary-components", "list"] as const,
    },
    salaryStructures: {
      all: (orgId: string) => ["hrm", orgId, "salary-structures"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "salary-structures", "list"] as const,
      detail: (orgId: string, structId: string) =>
        ["hrm", orgId, "salary-structures", structId] as const,
    },
    payrollRuns: {
      all: (orgId: string) => ["hrm", orgId, "payroll-runs"] as const,
      list: (orgId: string) => ["hrm", orgId, "payroll-runs", "list"] as const,
    },
    payslips: {
      all: (orgId: string) => ["hrm", orgId, "payslips"] as const,
      list: (orgId: string, runId?: string) =>
        ["hrm", orgId, "payslips", "list", runId ?? "all"] as const,
    },
    warningTypes: {
      all: (orgId: string) => ["hrm", orgId, "warning-types"] as const,
      list: (orgId: string) => ["hrm", orgId, "warning-types", "list"] as const,
    },
    escalationRules: {
      all: (orgId: string) => ["hrm", orgId, "escalation-rules"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "escalation-rules", "list"] as const,
    },
    warnings: {
      all: (orgId: string) => ["hrm", orgId, "warnings"] as const,
      list: (orgId: string) => ["hrm", orgId, "warnings", "list"] as const,
    },
    shifts: {
      all: (orgId: string) => ["hrm", orgId, "shifts"] as const,
      list: (orgId: string) => ["hrm", orgId, "shifts", "list"] as const,
    },
    shiftAssignments: {
      all: (orgId: string) => ["hrm", orgId, "shift-assignments"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "shift-assignments", "list"] as const,
    },
    holidayCalendars: {
      all: (orgId: string) => ["hrm", orgId, "holiday-calendars"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "holiday-calendars", "list"] as const,
      detail: (orgId: string, calId: string) =>
        ["hrm", orgId, "holiday-calendars", calId] as const,
    },
    documentTemplates: {
      all: (orgId: string) => ["hrm", orgId, "document-templates"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "document-templates", "list"] as const,
    },
    approvalTemplates: {
      all: (orgId: string) => ["hrm", orgId, "approval-templates"] as const,
      list: (orgId: string) =>
        ["hrm", orgId, "approval-templates", "list"] as const,
    },
  },
} as const;
