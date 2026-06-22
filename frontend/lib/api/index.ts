// lib/api/index.ts
// Import everything from here. Never import from individual modules directly
// in components — always go through this barrel file.

export { tasksApi } from "./tasks";
export { orgsApi } from "./orgs";
export { membersApi } from "./members";
export { authzApi } from "./authz";
export { authApi } from "./auth";

// Re-export types that consumers need
export type { TasksResponse, CreateTaskInput, UpdateTaskInput } from "./tasks";
export type { SignupInput } from "./auth";
export type {
  OrgsListResponse,
  CreateOrgInput,
  SwitchOrgResponse,
} from "./orgs";
