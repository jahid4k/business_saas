import { api } from './api';

export type TaskStatus = "todo" | "in_progress" | "done" | "cancelled";

export interface Task {
  id: string;
  publicId: string;
  organizationId: string;
  title: string;
  description?: string;
  status: TaskStatus;
  dueDate?: string; // ISO 8601, nullable
  assignedTo?: string; // user UUID
  createdBy: string;
  relatedType?: string;
  relatedId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface TaskListResponse {
  tasks: Task[];
  total: number;
  limit: number;
  offset: number;
}

export const tasksApi = {
  listTasks: async (orgId: string): Promise<TaskListResponse> => {
    const response = await api.get(`/organizations/${orgId}/tasks`);
    return response.data.data;
  },
  
  updateTask: async (orgId: string, taskId: string, body: Partial<Task>): Promise<Task> => {
    const response = await api.patch(`/organizations/${orgId}/tasks/${taskId}`, body);
    return response.data.data.task;
  }
};
