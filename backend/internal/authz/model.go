// Package authz implements role-based access control (RBAC) for BusinessSAAS.
//
// Data model:
//
//	User ──── Membership ────▶ Role ──── RolePermission ────▶ Permission
//	            (per business)
//
// The central check is: can(userID, businessID, resource, action) → bool
package authz

import "time"

// Role represents a system-defined role (Owner, Admin, Member, Viewer).
// Roles are seeded in migrations and are not user-editable in Phase 1.
type Role struct {
	ID          string    `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	Description string    `db:"description" json:"description"`
	IsSystem    bool      `db:"is_system"   json:"is_system"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
}

// Permission represents a granular capability.
// Format: "<resource>.<action>" e.g. "tasks.delete"
type Permission struct {
	ID          string    `db:"id"          json:"id"`
	Resource    string    `db:"resource"    json:"resource"` // "tasks", "members", "billing"
	Action      string    `db:"action"      json:"action"`   // "read", "create", "update", "delete"
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
}

// Key returns the canonical string form of the permission.
func (p *Permission) Key() string {
	return p.Resource + "." + p.Action
}

// Membership connects a User to a Business with a Role.
// One user can have at most one membership per business.
type Membership struct {
	ID         string    `db:"id"          json:"id"`
	UserID     string    `db:"user_id"     json:"user_id"`
	BusinessID string    `db:"business_id" json:"business_id"`
	RoleID     string    `db:"role_id"     json:"role_id"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}

// SystemRoles defines the names of seeded system roles.
// These names are stable — migrations seed them, code references them.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

// AssignRoleRequest is the request body for POST /api/v1/members/:userId/role.
type AssignRoleRequest struct {
	RoleName string `json:"role" validate:"required,oneof=admin member viewer"`
	// Note: owner role cannot be assigned via API — it is set during business creation.
}
