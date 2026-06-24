// src/types/org.ts
// Exact match to backend organizations/model.go JSON tags

export interface Business {
  id: string;
  publicId: string;
  name: string;
  slug: string;
  legalName?: string;
  type?: string;
  industry?: string;
  website?: string;
  logoURL?: string;
  country?: string;
  timezone: string;
  currency: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string;
}

// list response: data.organizations = MembershipWithRole[]
export interface MembershipWithRole {
  organization: Business; // ← "organization" key (not "business")
  role: string;
  membershipId: string;
}

// GET /api/v1/members/me → data.membership
export interface MyMembershipResponse {
  membershipId: string;
  organizationId: string;
  role: string;
  permissions: string[]; // ← this is what goes into permissionStore
  joinedAt: string;
}

// POST /api/v1/organizations/:id/switch → data
export interface SwitchOrgResponse {
  access_token: string;
  role: string;
  organization_id: string;
}

export interface CreateOrgRequest {
  name: string;
  slug: string;
  legalName?: string;
  type?: string;
  industry?: string;
  website?: string;
  country?: string;
  timezone?: string;
  currency?: string;
}
