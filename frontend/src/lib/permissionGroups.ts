// src/lib/permissionGroups.ts
//
// Shared permission categorization, used by both the role permission editor
// (components/roles/PermissionForm.tsx) and the per-member permission
// override panel (components/members/MemberPermissionsForm.tsx). Extracted
// here so a new resource — e.g. a future module beyond HRM — only needs
// updating in one place instead of drifting between the two forms.

export const GROUPS: { label: string; resources: string[] }[] = [
  {
    label: "General",
    resources: [
      "dashboard",
      "organization",
      "settings",
      "subscription",
      "billing",
    ],
  },
  {
    label: "Members & Roles",
    resources: ["members", "roles"],
  },
  {
    label: "Security & Audit",
    resources: ["security", "audit_logs", "api_keys"],
  },
  {
    label: "Tasks",
    resources: ["tasks"],
  },
  {
    label: "CRM",
    resources: [
      "crm.leads",
      "crm.contacts",
      "crm.companies",
      "crm.deals",
      "crm.tasks",
      "crm.activities",
      "crm.notes",
      "crm.emails",
      "crm.reports",
    ],
  },
  {
    label: "Platform & Projects",
    resources: ["platform.contacts", "platform.companies", "projects"],
  },
  {
    label: "HRM — Setup & Configuration",
    resources: [
      "hrm.departments",
      "hrm.positions",
      "hrm.salary",
      "hrm.approvals",
      "hrm.warning_types",
      "hrm.doc_templates",
      "hrm.shifts",
      "hrm.holidays",
      "hrm.contracts",
    ],
  },
  {
    label: "HRM — Employee Lifecycle",
    resources: [
      "hrm.employees",
      "hrm.promotions",
      "hrm.transfers",
      "hrm.resignations",
      "hrm.terminations",
    ],
  },
  {
    label: "HRM — Disciplinary",
    resources: [
      "hrm.warnings",
      "hrm.complaints",
      "hrm.documents",
      "hrm.acknowledgements",
    ],
  },
  {
    label: "HRM — Time & Compensation",
    resources: [
      "hrm.leave",
      "hrm.attendance",
      "hrm.salary.employee",
      "hrm.payroll",
    ],
  },
  {
    label: "HRM — Recognition & Communication",
    resources: [
      "hrm.awards",
      "hrm.announcements",
      "hrm.calendar",
      "hrm.milestones",
    ],
  },
  {
    label: "HRM — Reports",
    resources: ["hrm.reports"],
  },
];

export const RESOURCE_LABEL: Record<string, string> = {
  api_keys: "API Keys",
  audit_logs: "Audit Logs",
  billing: "Billing",
  "crm.activities": "Activities",
  "crm.companies": "Companies",
  "crm.contacts": "Contacts",
  "crm.deals": "Deals",
  "crm.emails": "Emails",
  "crm.leads": "Leads",
  "crm.notes": "Notes",
  "crm.reports": "Reports",
  "crm.tasks": "CRM Tasks",
  dashboard: "Dashboard",
  members: "Members",
  organization: "Organization",
  "platform.companies": "Platform Companies",
  "platform.contacts": "Platform Contacts",
  projects: "Projects",
  roles: "Roles",
  security: "Security",
  settings: "Settings",
  subscription: "Subscription",
  tasks: "Tasks",
  "hrm.departments": "Departments",
  "hrm.positions": "Positions",
  "hrm.salary": "Salary",
  "hrm.approvals": "Approval Chains",
  "hrm.warning_types": "Warning Types",
  "hrm.doc_templates": "Document Templates",
  "hrm.shifts": "Shifts",
  "hrm.holidays": "Holidays",
  "hrm.contracts": "Contracts",
  "hrm.employees": "Employees",
  "hrm.promotions": "Promotions",
  "hrm.transfers": "Transfers",
  "hrm.resignations": "Resignations",
  "hrm.terminations": "Terminations",
  "hrm.warnings": "Warnings",
  "hrm.complaints": "Complaints",
  "hrm.documents": "Employee Documents",
  "hrm.acknowledgements": "Acknowledgements",
  "hrm.leave": "Leave",
  "hrm.attendance": "Attendance",
  "hrm.salary.employee": "Employee Salary Records",
  "hrm.payroll": "Payroll",
  "hrm.awards": "Awards",
  "hrm.announcements": "Announcements",
  "hrm.calendar": "HR Calendar",
  "hrm.milestones": "Milestones",
  "hrm.reports": "HRM Reports",
};
