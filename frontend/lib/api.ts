// frontend/lib/api.ts
// All API calls to the real backend.
// Every function maps 1:1 to a backend route.

import type {
  TokenPair,
  User,
  Business,
  MembershipWithRole,
  MemberWithUser,
  MyMembership,
  Role,
  RoleWithPermissions,
  Permission,
  Task,
  ApiResponse,
} from "@/types";
import { store } from "./store";

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ----------------------------------------------------------
// Core fetch wrapper
// ----------------------------------------------------------

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<ApiResponse<T>> {
  const token = store.getAccessToken();

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers,
  });

  // 204 No Content
  if (res.status === 204) {
    return { success: true };
  }

  const json = await res.json();
  return json as ApiResponse<T>;
}

// ----------------------------------------------------------
// System
// ----------------------------------------------------------

export async function getHealth() {
  return request("/api/v1/health");
}

export async function getHello() {
  return request("/api/v1/hello");
}

// ----------------------------------------------------------
// Auth
// ----------------------------------------------------------

export async function signup(data: {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
}): Promise<ApiResponse<{ user: User }>> {
  return request("/api/v1/auth/signup", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function login(data: {
  email: string;
  password: string;
}): Promise<ApiResponse<TokenPair>> {
  return request("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function refresh(
  refreshToken: string,
): Promise<ApiResponse<TokenPair>> {
  return request("/api/v1/auth/refresh", {
    method: "POST",
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}

export async function logout(refreshToken: string): Promise<ApiResponse> {
  return request("/api/v1/auth/logout", {
    method: "POST",
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}

export async function logoutAll(): Promise<ApiResponse> {
  return request("/api/v1/auth/logout-all", { method: "POST" });
}

export async function getAuthMe(): Promise<ApiResponse<{ user: User }>> {
  return request("/api/v1/auth/me");
}

export async function passwordResetRequest(
  email: string,
): Promise<ApiResponse> {
  return request("/api/v1/auth/password-reset/request", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export async function passwordResetConfirm(data: {
  token: string;
  new_password: string;
}): Promise<ApiResponse> {
  return request("/api/v1/auth/password-reset/confirm", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// ----------------------------------------------------------
// Users
// ----------------------------------------------------------

export async function getMe(): Promise<ApiResponse<{ user: User }>> {
  return request("/api/v1/users/me");
}

export async function updateMe(data: {
  first_name: string;
  last_name: string;
}): Promise<ApiResponse<{ user: User }>> {
  return request("/api/v1/users/me", {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

// ----------------------------------------------------------
// Businesses
// ----------------------------------------------------------

export async function createBusiness(data: {
  name: string;
  slug: string;
}): Promise<ApiResponse<{ business: Business }>> {
  return request("/api/v1/businesses", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function listBusinesses(): Promise<
  ApiResponse<{ businesses: MembershipWithRole[] }>
> {
  return request("/api/v1/businesses");
}

export async function getBusiness(
  id: string,
): Promise<ApiResponse<{ business: Business }>> {
  return request(`/api/v1/businesses/${id}`);
}

export async function switchBusiness(
  id: string,
): Promise<ApiResponse<{ access_token: string; role: string }>> {
  return request(`/api/v1/businesses/${id}/switch`, { method: "POST" });
}

// ----------------------------------------------------------
// Members (business-scoped — needs business_id in token)
// ----------------------------------------------------------

export async function getMyMembership(): Promise<
  ApiResponse<{ membership: MyMembership }>
> {
  return request("/api/v1/members/me");
}

export async function listMembers(): Promise<
  ApiResponse<{ members: MemberWithUser[] }>
> {
  return request("/api/v1/members");
}

export async function assignRole(
  userId: string,
  role: string,
): Promise<ApiResponse> {
  return request(`/api/v1/members/${userId}/role`, {
    method: "POST",
    body: JSON.stringify({ role }),
  });
}

// ----------------------------------------------------------
// Roles & Permissions (JWT only, no business context needed)
// ----------------------------------------------------------

export async function listRoles(): Promise<
  ApiResponse<{ roles: RoleWithPermissions[] }>
> {
  return request("/api/v1/roles");
}

export async function listPermissions(): Promise<
  ApiResponse<{ permissions: Permission[] }>
> {
  return request("/api/v1/permissions");
}

// ----------------------------------------------------------
// Tasks (business-scoped + permission-gated)
// ----------------------------------------------------------

export async function listTasks(): Promise<ApiResponse<Task[]>> {
  return request("/api/v1/tasks");
}

export async function createTask(data: {
  title: string;
  description?: string;
  status?: string;
}): Promise<ApiResponse<{ task: Task }>> {
  return request("/api/v1/tasks", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function getTask(
  id: string,
): Promise<ApiResponse<{ task: Task }>> {
  return request(`/api/v1/tasks/${id}`);
}

export async function updateTask(
  id: string,
  data: {
    title?: string;
    description?: string;
    status?: string;
  },
): Promise<ApiResponse<{ task: Task }>> {
  return request(`/api/v1/tasks/${id}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function deleteTask(id: string): Promise<ApiResponse> {
  return request(`/api/v1/tasks/${id}`, { method: "DELETE" });
}
