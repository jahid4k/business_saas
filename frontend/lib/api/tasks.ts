// lib/api/tasks.ts
import { apiGet, apiDelete, apiPost, apiPatch } from "@/lib/api";
import type { Task, TaskStatus } from "@/types/domain";

export interface TasksResponse {
  tasks: Task[];
  total: number;
}

export interface CreateTaskInput {
  title: string;
  description?: string;
  status?: TaskStatus;
  assigned_to?: string;
  due_at?: string;
}

export interface UpdateTaskInput {
  title?: string;
  description?: string;
  status?: TaskStatus;
  assigned_to?: string | null;
  due_at?: string | null;
}

const base = (orgId: string) => `api/v1/organizations/${orgId}/tasks`;

export const tasksApi = {
  list: (
    orgId: string,
    params?: { status?: string; page?: number; limit?: number },
  ) => apiGet<TasksResponse>(base(orgId), params),

  get: (orgId: string, taskId: string) =>
    apiGet<{ task: Task }>(`${base(orgId)}/${taskId}`),

  create: (orgId: string, body: CreateTaskInput) =>
    apiPost<{ task: Task }>(base(orgId), body),

  update: (orgId: string, taskId: string, body: UpdateTaskInput) =>
    apiPatch<{ task: Task }>(`${base(orgId)}/${taskId}`, body),

  delete: (orgId: string, taskId: string) =>
    apiDelete(`${base(orgId)}/${taskId}`),
};
