// src/lib/security.ts
import api from "./api";

// ── Types ─────────────────────────────────────────────
// All fields camelCase — consistent with auth/user layer
export interface Session {
  id: string;
  publicId: string;
  userId: string;
  userPublicId: string;
  email: string;
  displayName: string;
  userAgent: string;
  ipAddress: string;
  lastActivityAt: string;
  createdAt: string;
  expiresAt: string;
  isActive: boolean;
  revokedAt?: string;
}

export interface LoginEvent {
  id: string;
  publicId: string;
  userId: string;
  userPublicId: string;
  email: string;
  provider: string;
  status: "success" | "failed";
  ipAddress: string;
  userAgent: string;
  createdAt: string;
}

// ── API ───────────────────────────────────────────────

// GET /security/sessions → data.sessions[]
export async function listSessions(orgId: string): Promise<Session[]> {
  const res = await api.get<{
    success: boolean;
    data: { sessions: Session[] };
  }>(`/api/v1/organizations/${orgId}/security/sessions`);
  return res.data.data.sessions ?? [];
}

// DELETE /security/sessions/:sessionId
export async function revokeSession(
  orgId: string,
  sessionId: string,
): Promise<void> {
  await api.delete(
    `/api/v1/organizations/${orgId}/security/sessions/${sessionId}`,
  );
}

// GET /security/login-events → data.loginEvents[]
export async function listLoginEvents(orgId: string): Promise<LoginEvent[]> {
  const res = await api.get<{
    success: boolean;
    data: { loginEvents: LoginEvent[] };
  }>(`/api/v1/organizations/${orgId}/security/login-events`);
  return res.data.data.loginEvents ?? [];
}
