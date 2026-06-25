// src/types/rbac.ts

export type MemberRole = "owner" | "admin" | "manager" | "member" | "viewer";
export type MemberStatus = "active" | "inactive" | "pending" | "removed";

// ── Members ───────────────────────────────────────────
export interface Member {
  membershipId: string;
  membershipPublicId: string;
  userId: string;
  userPublicId: string;
  email: string;
  displayName: string;
  firstName?: string;
  lastName?: string;
  roleId: string;
  role: MemberRole;
  status: MemberStatus;
  joinedAt: string;
}

// ── Roles ─────────────────────────────────────────────
export interface Role {
  id: string;
  publicId: string;
  name: string;
  description: string;
  permissionKeys: string[];
  isSystem: boolean;
  isCustom: boolean;
  createdAt: string;
  updatedAt: string;
}

// Confirmed: list response wraps each role in { role, permissions }
export interface RoleWithMeta {
  role: Role;
  permissions: Permission[]; // empty array in list response
}

// ── Permissions ───────────────────────────────────────
export interface Permission {
  id: string;
  publicId: string;
  key: string;
  resource: string;
  action: string;
  description: string;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
}

// ── Request bodies ────────────────────────────────────
export interface InviteRequest {
  email: string;
  role: MemberRole;
}
