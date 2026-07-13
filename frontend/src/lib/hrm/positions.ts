// src/lib/hrm/positions.ts
import api from "../api";
import type {
  Position,
  PositionListResponse,
  CreatePositionPayload,
  UpdatePositionPayload,
} from "@/types/hrm";

const base = (orgId: string) => `/api/v1/organizations/${orgId}/hrm/positions`;

export async function listPositions(
  orgId: string,
): Promise<PositionListResponse> {
  const res = await api.get<{ success: boolean; data: PositionListResponse }>(
    base(orgId),
  );
  return res.data.data;
}

export async function getPosition(
  orgId: string,
  posId: string,
): Promise<Position> {
  const res = await api.get<{ success: boolean; data: { position: Position } }>(
    `${base(orgId)}/${posId}`,
  );
  return res.data.data.position;
}

export async function createPosition(
  orgId: string,
  body: CreatePositionPayload,
): Promise<Position> {
  const res = await api.post<{
    success: boolean;
    data: { position: Position };
  }>(base(orgId), body);
  return res.data.data.position;
}

export async function updatePosition(
  orgId: string,
  posId: string,
  body: UpdatePositionPayload,
): Promise<Position> {
  const res = await api.patch<{
    success: boolean;
    data: { position: Position };
  }>(`${base(orgId)}/${posId}`, body);
  return res.data.data.position;
}

export async function deletePosition(
  orgId: string,
  posId: string,
): Promise<void> {
  await api.delete(`${base(orgId)}/${posId}`);
}
