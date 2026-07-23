export interface User {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  displayName?: string;
  photoURL?: string;
}

export interface Organization {
  id?: string;
  publicId?: string;
  name?: string;
  legalName?: string;
  slug?: string;
  logoURL?: string;
}

export interface MembershipWithRole {
  organization: Organization;
  role: string;
  membershipId: string;
}

export interface Membership {
  membershipId: string;
  organizationId: string;
  role: string;
  joinedAt: string;
}
