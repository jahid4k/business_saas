// types/domain.ts
// Business domain types — mirror the Go backend models.
// These are the shapes returned by the API inside response.data.

// ----------------------------------------------------------
// User
// ----------------------------------------------------------

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url?: string | null;
  created_at: string;
  updated_at: string;
}

// ----------------------------------------------------------
// Organization (also called Business or Workspace)
// ----------------------------------------------------------

export interface Organization {
  id: string;
  name: string;
  slug: string;
  logo_url?: string | null;
  created_at: string;
  updated_at: string;
}

// ----------------------------------------------------------
// Membership — connects User to Organization with a Role
// ----------------------------------------------------------

export interface Membership {
  id: string;
  org_id: string;
  user_id: string;
  role_id: string | null;
  role_key: string; // 'owner' | 'admin' | 'member' | 'viewer'
  status: "active" | "inactive" | "suspended";
  joined_at: string;
  created_at: string;
  updated_at: string;
}

export interface MembershipWithOrg extends Membership {
  organization: Organization;
}

export interface MemberWithUser extends Membership {
  user: User;
  role: Role | null;
  permissions: string[];
}

export interface MyMembership extends Membership {
  organization: Organization;
  user: User;
  role: Role | null;
  permissions: string[]; // flat list of permission keys e.g. ['tasks.view', 'tasks.create']
}

// ----------------------------------------------------------
// RBAC — Roles and Permissions
// ----------------------------------------------------------

export interface Permission {
  id: string;
  key: string; // e.g. 'tasks.view', 'crm.leads.create'
  name: string;
  description?: string;
  module: string; // e.g. 'tasks', 'crm', 'members'
}

export interface Role {
  id: string;
  org_id: string | null; // null for system roles
  name: string;
  key: string; // 'owner' | 'admin' | 'member' | 'viewer'
  description?: string;
  is_system: boolean;
  created_at: string;
}

export interface RoleWithPermissions extends Role {
  permissions: Permission[];
}

// ----------------------------------------------------------
// Task
// ----------------------------------------------------------

export type TaskStatus = "todo" | "in_progress" | "done";

export interface Task {
  id: string;
  org_id: string;
  title: string;
  description: string | null;
  status: TaskStatus;
  created_by: string; // user UUID
  assigned_to: string | null; // user UUID
  due_at: string | null; // ISO 8601
  created_at: string;
  updated_at: string;
  // Populated in list responses
  creator?: Pick<User, "id" | "name" | "email">;
  assignee?: Pick<User, "id" | "name" | "email"> | null;
}

// ----------------------------------------------------------
// CRM — Contacts
// ----------------------------------------------------------

export interface Contact {
  id: string;
  org_id: string;
  first_name: string;
  last_name: string;
  email: string | null;
  phone: string | null;
  company_id: string | null;
  created_at: string;
  updated_at: string;
  company?: Pick<Company, "id" | "name"> | null;
}

// ----------------------------------------------------------
// CRM — Companies
// ----------------------------------------------------------

export interface Company {
  id: string;
  org_id: string;
  name: string;
  industry: string | null;
  website: string | null;
  created_at: string;
  updated_at: string;
}

// ----------------------------------------------------------
// CRM — Leads
// ----------------------------------------------------------

export type LeadStatus = "new" | "contacted" | "qualified" | "lost";
export type LeadSource =
  | "website"
  | "referral"
  | "cold_call"
  | "event"
  | "other";

export interface Lead {
  id: string;
  org_id: string;
  first_name: string;
  last_name: string;
  email: string | null;
  phone: string | null;
  company_name: string | null;
  status: LeadStatus;
  source: LeadSource | null;
  notes: string | null;
  created_by: string;
  assigned_to: string | null;
  converted_at: string | null;
  created_at: string;
  updated_at: string;
  creator?: Pick<User, "id" | "name">;
  assignee?: Pick<User, "id" | "name"> | null;
}

// ----------------------------------------------------------
// CRM — Pipeline and Stages
// ----------------------------------------------------------

export interface Pipeline {
  id: string;
  org_id: string;
  name: string;
  description: string | null;
  is_default: boolean;
  created_at: string;
  updated_at: string;
  stages?: Stage[];
}

export interface Stage {
  id: string;
  pipeline_id: string;
  name: string;
  position: number;
  created_at: string;
  updated_at: string;
}

// ----------------------------------------------------------
// CRM — Deals
// ----------------------------------------------------------

export type DealStatus = "open" | "won" | "lost";

export interface Deal {
  id: string;
  org_id: string;
  pipeline_id: string;
  stage_id: string;
  title: string;
  value: number;
  currency: string;
  status: DealStatus;
  contact_id: string | null;
  company_id: string | null;
  lead_id: string | null;
  assigned_to: string | null;
  expected_close_date: string | null;
  closed_at: string | null;
  created_by: string;
  created_at: string;
  updated_at: string;
  stage?: Stage;
  pipeline?: Pick<Pipeline, "id" | "name">;
  contact?: Pick<Contact, "id" | "first_name" | "last_name"> | null;
  assignee?: Pick<User, "id" | "name"> | null;
}

// ----------------------------------------------------------
// Auth session (stored in next-auth, not from API directly)
// ----------------------------------------------------------

export interface SessionUser {
  id: string;
  email: string;
  name: string;
  accessToken: string; // Go backend JWT — stored in next-auth session
  activeOrgId?: string | null;
  activeOrgSlug?: string | null;
  activeRole?: string | null;
}
