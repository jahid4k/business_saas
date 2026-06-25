// src/types/task.ts

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
  createdAt: string;
  updatedAt: string;
}

// GET list → data.tasks (confirmed from backend)
export interface TaskListResponse {
  tasks: Task[];
  total: number;
  limit: number;
  offset: number;
}

export interface CreateTaskRequest {
  title: string;
  description?: string;
  status?: TaskStatus;
  dueDate?: string;
  assignedTo?: string;
}

export interface UpdateTaskRequest {
  title?: string;
  description?: string;
  status?: TaskStatus;
  dueDate?: string | null; // null = clear the date
  assignedTo?: string | null;
}
