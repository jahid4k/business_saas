/**
 * lib/permissions.ts — Client-side permission helpers.
 *
 * These functions are for UX ONLY — hiding and disabling buttons.
 * The backend enforces real permission checks on every request.
 * A user who bypasses the UI and calls the API directly is rejected
 * by the backend's RequirePermission middleware.
 */

import { getCurrentRole } from "./auth";
import {
  roleHasPermission,
  type PermissionKey,
  type RoleName,
} from "@/types/permission";

/**
 * can() — check if the current user has a permission.
 * Reads the role from the JWT in localStorage.
 *
 * Usage:
 *   if (can("tasks.delete")) { showDeleteButton() }
 */
export function can(permission: PermissionKey): boolean {
  const role = getCurrentRole() as RoleName | null;
  return roleHasPermission(role, permission);
}

/**
 * canAny() — check if the current user has any of the given permissions.
 */
export function canAny(...permissions: PermissionKey[]): boolean {
  return permissions.some(can);
}

/**
 * canAll() — check if the current user has all of the given permissions.
 */
export function canAll(...permissions: PermissionKey[]): boolean {
  return permissions.every(can);
}
