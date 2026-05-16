// backend/internal/authz/model.go
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

// Permission represents a granular capability in the format resource.action.
type Permission struct {
	ID          string    `db:"id"          json:"id"`
	Resource    string    `db:"resource"    json:"resource"`
	Action      string    `db:"action"      json:"action"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
}

// Key returns the canonical dot-separated permission string e.g. "tasks.delete".
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

// MemberWithUser is the enriched membership returned by GET /api/v1/members.
// Joins: memberships + users + roles in one query.
type MemberWithUser struct {
	MembershipID string    `json:"membership_id"`
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Role         string    `json:"role"`
	JoinedAt     time.Time `json:"joined_at"`
}

// RoleWithPermissions enriches a Role with its full permission list.
// Returned by GET /api/v1/roles.
type RoleWithPermissions struct {
	Role        *Role         `json:"role"`
	Permissions []*Permission `json:"permissions"`
}

// MyMembershipResponse is returned by GET /api/v1/members/me.
type MyMembershipResponse struct {
	MembershipID string    `json:"membership_id"`
	BusinessID   string    `json:"business_id"`
	Role         string    `json:"role"`
	Permissions  []string  `json:"permissions"` // list of "resource.action" strings
	JoinedAt     time.Time `json:"joined_at"`
}

// AssignRoleRequest is the body for POST /api/v1/members/:userId/role.
type AssignRoleRequest struct {
	Role string `json:"role"` // one of: admin, member, viewer (not owner)
}

// SystemRoles defines the names of seeded system roles.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)
