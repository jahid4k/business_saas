// src/lib/members.ts
import api from "./api";
import type { Member, MemberRole, InviteRequest } from "@/types/rbac";

// GET /organizations/:orgId/members → data.members: Member[]
export async function listMembers(orgId: string): Promise<Member[]> {
  const res = await api.get<{ success: boolean; data: { members: Member[] } }>(
    `/api/v1/organizations/${orgId}/members`,
  );
  return res.data.data.members ?? [];
}

// POST /organizations/:orgId/members/invite
export async function inviteMember(
  orgId: string,
  body: InviteRequest,
): Promise<void> {
  await api.post(`/api/v1/organizations/${orgId}/members/invite`, body);
}

// PATCH /organizations/:orgId/members/:membershipId/role
// Note: uses membershipId as the path param
export async function updateMemberRole(
  orgId: string,
  membershipId: string,
  role: MemberRole,
): Promise<void> {
  await api.patch(
    `/api/v1/organizations/${orgId}/members/${membershipId}/role`,
    { role },
  );
}

// PATCH /organizations/:orgId/members/:membershipId/status
// Passing status: 'inactive' to deactivate / remove
export async function updateMemberStatus(
  orgId: string,
  membershipId: string,
  status: string,
): Promise<void> {
  await api.patch(
    `/api/v1/organizations/${orgId}/members/${membershipId}/status`,
    { status },
  );
}

// POST /organizations/:orgId/invitations/:invitationId/resend
export async function resendInvitation(
  orgId: string,
  invitationId: string,
): Promise<void> {
  await api.post(
    `/api/v1/organizations/${orgId}/invitations/${invitationId}/resend`,
  );
}

// DELETE /organizations/:orgId/invitations/:invitationId
export async function cancelInvitation(
  orgId: string,
  invitationId: string,
): Promise<void> {
  await api.delete(
    `/api/v1/organizations/${orgId}/invitations/${invitationId}`,
  );
}

// POST /organizations/:orgId/members/:membershipId/reset-password
// Admin-initiated — sets the password directly, no token/email step.
// Revokes the member's sessions server-side; nothing to do here for that.
export async function resetMemberPassword(
  orgId: string,
  membershipId: string,
  newPassword: string,
): Promise<void> {
  await api.post(
    `/api/v1/organizations/${orgId}/members/${membershipId}/reset-password`,
    { newPassword },
  );
}
