// src/lib/crm/pipelines.ts
import api from "../api";
import type {
  Pipeline,
  Stage,
  PipelineListResponse,
  StageListResponse,
} from "@/types/crm";

// GET /crm/pipelines → data: { pipelines[], total }
// Note: pipelines in list response do NOT include stages
export async function listPipelines(orgId: string): Promise<Pipeline[]> {
  const res = await api.get<{ success: boolean; data: PipelineListResponse }>(
    `/api/v1/organizations/${orgId}/crm/pipelines`,
  );
  return res.data.data.pipelines ?? [];
}

// GET /crm/pipelines/:id/stages → data: { stages[], total }
export async function listStages(
  orgId: string,
  pipelineId: string,
): Promise<Stage[]> {
  const res = await api.get<{ success: boolean; data: StageListResponse }>(
    `/api/v1/organizations/${orgId}/crm/pipelines/${pipelineId}/stages`,
  );
  return res.data.data.stages ?? [];
}
