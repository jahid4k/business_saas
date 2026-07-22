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

export async function createPipeline(
  orgId: string,
  data: { name: string; is_default?: boolean },
): Promise<Pipeline> {
  const res = await api.post<{
    success: boolean;
    data: { pipeline: Pipeline };
  }>(`/api/v1/organizations/${orgId}/crm/pipelines`, data);
  return res.data.data.pipeline;
}

export async function deletePipeline(
  orgId: string,
  pipelineId: string,
): Promise<void> {
  await api.delete(
    `/api/v1/organizations/${orgId}/crm/pipelines/${pipelineId}`,
  );
}

export async function createStage(
  orgId: string,
  pipelineId: string,
  data: { name: string; position?: number; probability?: number },
): Promise<Stage> {
  const res = await api.post<{ success: boolean; data: { stage: Stage } }>(
    `/api/v1/organizations/${orgId}/crm/pipelines/${pipelineId}/stages`,
    data,
  );
  return res.data.data.stage;
}

export async function updateStage(
  orgId: string,
  pipelineId: string,
  stageId: string,
  data: { name?: string; position?: number; probability?: number },
): Promise<Stage> {
  const res = await api.patch<{ success: boolean; data: { stage: Stage } }>(
    `/api/v1/organizations/${orgId}/crm/pipelines/${pipelineId}/stages/${stageId}`,
    data,
  );
  return res.data.data.stage;
}

export async function deleteStage(
  orgId: string,
  pipelineId: string,
  stageId: string,
): Promise<void> {
  await api.delete(
    `/api/v1/organizations/${orgId}/crm/pipelines/${pipelineId}/stages/${stageId}`,
  );
}
