// backend/internal/authz/model.go
package authz

import "time"

type Role struct {
	ID          string    `db:"id" json:"id"`
	PublicID    string    `db:"public_id" json:"publicId"`
	OrgID       *string   `db:"org_id" json:"organizationId,omitempty"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Permissions []string  `db:"permissions" json:"permissionKeys"`
	IsSystem    bool      `db:"is_system" json:"isSystem"`
	IsCustom    bool      `db:"is_custom" json:"isCustom"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

type Permission struct {
	ID          string    `db:"id" json:"id"`
	PublicID    string    `db:"public_id" json:"publicId"`
	KeyName     string    `db:"key" json:"key"`
	Resource    string    `db:"resource" json:"resource"`
	Action      string    `db:"action" json:"action"`
	Description string    `db:"description" json:"description"`
	IsSystem    bool      `db:"is_system" json:"isSystem"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

func (p *Permission) PermissionKey() string {
	if p.KeyName != "" {
		return p.KeyName
	}
	return p.Resource + "." + p.Action
}

// Key returns the canonical dot-separated permission string, e.g. crm.deals.create.
func (p *Permission) Key() string { return p.PermissionKey() }

// Membership connects a User to an Organization with a Role.
type Membership struct {
	ID                   string     `db:"id" json:"id"`
	PublicID             string     `db:"public_id" json:"publicId"`
	UserID               string     `db:"user_id" json:"userId"`
	OrganizationID       string     `db:"org_id" json:"organizationId"`
	RoleID               *string    `db:"role_id" json:"roleId,omitempty"`
	RoleKey              string     `db:"role_key" json:"role"`
	Title                string     `db:"title" json:"title,omitempty"`
	Department           string     `db:"department" json:"department,omitempty"`
	Status               string     `db:"status" json:"status"`
	CustomPermissions    []string   `db:"custom_permissions" json:"customPermissions"`
	InvitationStatus     string     `db:"invitation_status" json:"invitationStatus"`
	InvitedBy            *string    `db:"invited_by" json:"invitedBy,omitempty"`
	InvitationSentAt     *time.Time `db:"invitation_sent_at" json:"invitationSentAt,omitempty"`
	InvitationAcceptedAt *time.Time `db:"invitation_accepted_at" json:"invitationAcceptedAt,omitempty"`
	JoinedAt             time.Time  `db:"joined_at" json:"joinedAt"`
	CreatedAt            time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updatedAt"`
}

type MemberWithUser struct {
	MembershipID string    `json:"membershipId"`
	UserID       string    `json:"userId"`
	Email        string    `json:"email,omitempty"`
	DisplayName  string    `json:"displayName"`
	FirstName    string    `json:"firstName,omitempty"`
	LastName     string    `json:"lastName,omitempty"`
	PhotoURL     string    `json:"photoURL,omitempty"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	Title        string    `json:"title,omitempty"`
	Department   string    `json:"department,omitempty"`
	JoinedAt     time.Time `json:"joinedAt"`
}

type RoleWithPermissions struct {
	Role        *Role         `json:"role"`
	Permissions []*Permission `json:"permissions"`
}

type MyMembershipResponse struct {
	MembershipID   string    `json:"membershipId"`
	OrganizationID string    `json:"organizationId"`
	Role           string    `json:"role"`
	Permissions    []string  `json:"permissions"`
	JoinedAt       time.Time `json:"joinedAt"`
}

type AssignRoleRequest struct {
	Role string `json:"role"`
}

const (
	RoleOwner   = "owner"
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleMember  = "member"
	RoleViewer  = "viewer"
)
