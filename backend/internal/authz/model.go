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

func (p *Permission) Key() string { return p.PermissionKey() }

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
	DeniedPermissions    []string   `db:"denied_permissions" json:"deniedPermissions"`
	InvitationStatus     string     `db:"invitation_status" json:"invitationStatus"`
	InvitedBy            *string    `db:"invited_by" json:"invitedBy,omitempty"`
	InvitationSentAt     *time.Time `db:"invitation_sent_at" json:"invitationSentAt,omitempty"`
	InvitationAcceptedAt *time.Time `db:"invitation_accepted_at" json:"invitationAcceptedAt,omitempty"`
	JoinedAt             time.Time  `db:"joined_at" json:"joinedAt"`
	CreatedAt            time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updatedAt"`
}

type MemberWithUser struct {
	MembershipID     string    `json:"membershipId"`
	MembershipPublic string    `json:"membershipPublicId,omitempty"`
	UserID           string    `json:"userId"`
	UserPublicID     string    `json:"userPublicId,omitempty"`
	Email            string    `json:"email,omitempty"`
	DisplayName      string    `json:"displayName"`
	FirstName        string    `json:"firstName,omitempty"`
	LastName         string    `json:"lastName,omitempty"`
	PhotoURL         string    `json:"photoURL,omitempty"`
	RoleID           *string   `json:"roleId,omitempty"`
	Role             string    `json:"role"`
	Status           string    `json:"status"`
	Title            string    `json:"title,omitempty"`
	Department       string    `json:"department,omitempty"`
	JoinedAt         time.Time `json:"joinedAt"`
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

type UpdateMemberRequest struct {
	RoleID            string   `json:"roleId"`
	Role              string   `json:"role"`
	Status            string   `json:"status"`
	Title             string   `json:"title"`
	Department        string   `json:"department"`
	CustomPermissions []string `json:"customPermissions"`
	DeniedPermissions []string `json:"deniedPermissions"`
}

type CreateRoleRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	PermissionKeys []string `json:"permissionKeys"`
}

type UpdateRoleRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	PermissionKeys []string `json:"permissionKeys"`
}

type UpdateRolePermissionsRequest struct {
	PermissionKeys []string `json:"permissionKeys"`
}

type CloneRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type MemberPermissionsResponse struct {
	MemberID           string   `json:"memberId"`
	UserID             string   `json:"userId"`
	RolePermissionKeys []string `json:"rolePermissionKeys"`
	CustomPermissions  []string `json:"customPermissions"`
	DeniedPermissions  []string `json:"deniedPermissions"`
	Effective          []string `json:"effectivePermissions"`
}

type UpdateMemberPermissionsRequest struct {
	CustomPermissions []string `json:"customPermissions"`
	DeniedPermissions []string `json:"deniedPermissions"`
	Grant             []string `json:"grant"`
	Deny              []string `json:"deny"`
}

type CheckMemberPermissionRequest struct {
	MemberID   string `json:"memberId"`
	UserID     string `json:"userId"`
	Permission string `json:"permission"`
	Resource   string `json:"resource"`
	Action     string `json:"action"`
}

type CheckMemberPermissionResponse struct {
	Allowed    bool   `json:"allowed"`
	Permission string `json:"permission"`
	MemberID   string `json:"memberId"`
	UserID     string `json:"userId"`
}

type RolePermissionMatrixRow struct {
	Role           *Role           `json:"role"`
	PermissionKeys map[string]bool `json:"permissionKeys"`
}

type PermissionMatrixResponse struct {
	Permissions []*Permission              `json:"permissions"`
	Roles       []*Role                    `json:"roles"`
	Matrix      []*RolePermissionMatrixRow `json:"matrix"`
}

type OrganizationInvitation struct {
	ID                string     `json:"id"`
	PublicID          string     `json:"publicId"`
	OrganizationID    string     `json:"organizationId"`
	Email             string     `json:"email"`
	RoleID            *string    `json:"roleId,omitempty"`
	RoleKey           string     `json:"role"`
	Title             string     `json:"title,omitempty"`
	Department        string     `json:"department,omitempty"`
	CustomPermissions []string   `json:"customPermissions"`
	DeniedPermissions []string   `json:"deniedPermissions"`
	TokenHash         string     `json:"-"`
	Status            string     `json:"status"`
	InvitedBy         *string    `json:"invitedBy,omitempty"`
	AcceptedBy        *string    `json:"acceptedBy,omitempty"`
	ExpiresAt         time.Time  `json:"expiresAt"`
	AcceptedAt        *time.Time `json:"acceptedAt,omitempty"`
	RevokedAt         *time.Time `json:"revokedAt,omitempty"`
	LastSentAt        time.Time  `json:"lastSentAt"`
	ResendCount       int        `json:"resendCount"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type InviteMemberRequest struct {
	Email             string   `json:"email"`
	RoleID            string   `json:"roleId"`
	Role              string   `json:"role"`
	Title             string   `json:"title"`
	Department        string   `json:"department"`
	CustomPermissions []string `json:"customPermissions"`
	DeniedPermissions []string `json:"deniedPermissions"`
}

type InviteMemberResponse struct {
	Invitation *OrganizationInvitation `json:"invitation"`
	Token      string                  `json:"token,omitempty"`
}

type ResendInvitationResponse struct {
	Invitation *OrganizationInvitation `json:"invitation"`
	Token      string                  `json:"token,omitempty"`
}

const (
	RoleOwner   = "owner"
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleMember  = "member"
	RoleViewer  = "viewer"
)

const (
	MemberStatusActive    = "active"
	MemberStatusInactive  = "inactive"
	MemberStatusSuspended = "suspended"
)
