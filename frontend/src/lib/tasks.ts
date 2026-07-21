// src/lib/tasks.ts
import api from "./api";
import type {
  Task,
  TaskListResponse,
  CreateTaskRequest,
  UpdateTaskRequest,
} from "@/types/task";

// Confirmed shapes:
// GET list  → data: { tasks: Task[], total, limit, offset }
// POST      → data: { task: Task }
// PATCH/GET → data: { task: Task }  (assumed same pattern as POST)

export async function listTasks(
  orgId: string,
  params?: {
    status?: string;
    assignedTo?: string;
    relatedType?: string;
    relatedId?: string;
    limit?: number;
    offset?: number;
  },
): Promise<TaskListResponse> {
  const res = await api.get<{ success: boolean; data: TaskListResponse }>(
    `/api/v1/organizations/${orgId}/tasks`,
    { params },
  );
  return res.data.data;
}

export async function createTask(
  orgId: string,
  body: CreateTaskRequest,
): Promise<Task> {
  const res = await api.post<{ success: boolean; data: { task: Task } }>(
    `/api/v1/organizations/${orgId}/tasks`,
    body,
  );
  return res.data.data.task;
}

export async function getTask(orgId: string, taskId: string): Promise<Task> {
  const res = await api.get<{ success: boolean; data: { task: Task } }>(
    `/api/v1/organizations/${orgId}/tasks/${taskId}`,
  );
  return res.data.data.task;
}

export async function updateTask(
  orgId: string,
  taskId: string,
  body: UpdateTaskRequest,
): Promise<Task> {
  const res = await api.patch<{ success: boolean; data: { task: Task } }>(
    `/api/v1/organizations/${orgId}/tasks/${taskId}`,
    body,
  );
  return res.data.data.task;
}

export async function deleteTask(orgId: string, taskId: string): Promise<void> {
  await api.delete(`/api/v1/organizations/${orgId}/tasks/${taskId}`);
}
