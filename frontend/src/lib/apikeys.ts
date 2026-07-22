import api from "./api";

export interface OrgAPIKey {
  id: string;
  org_id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  allowed_origins: string[];
  is_active: boolean;
  last_used_at: string | null;
  expires_at: string | null;
  created_by: string;
  created_at: string;
}

export interface CreateKeyRequest {
  name: string;
  scopes: string[];
  allowed_origins: string[];
  expires_at?: string;
}

export interface CreateKeyResponse {
  key: OrgAPIKey;
  raw_key: string;
}

export async function listAPIKeys(orgId: string): Promise<OrgAPIKey[]> {
  const { data } = await api.get<{ data: OrgAPIKey[] }>(
    `/api/v1/organizations/${orgId}/capture/apikeys`,
  );
  return data.data;
}

export async function createAPIKey(
  orgId: string,
  req: CreateKeyRequest,
): Promise<CreateKeyResponse> {
  const { data } = await api.post<{ data: CreateKeyResponse }>(
    `/api/v1/organizations/${orgId}/capture/apikeys`,
    req,
  );
  return data.data;
}

export async function revokeAPIKey(
  orgId: string,
  keyId: string,
): Promise<void> {
  await api.delete(`/api/v1/organizations/${orgId}/capture/apikeys/${keyId}`);
}
