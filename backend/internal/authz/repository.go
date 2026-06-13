// backend/internal/authz/repository.go
package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetUserPermissions(ctx context.Context, userID, organizationID string) ([]*Permission, error)
	GetMembership(ctx context.Context, userID, organizationID string) (*Membership, error)
	GetMemberByRef(ctx context.Context, organizationID, memberRef string) (*Membership, error)
	GetMemberWithUserByRef(ctx context.Context, organizationID, memberRef string) (*MemberWithUser, error)
	ListMembers(ctx context.Context, organizationID string) ([]*MemberWithUser, error)
	GetRoleByName(ctx context.Context, name string) (*Role, error)
	GetRoleByID(ctx context.Context, roleID string) (*Role, error)
	GetRoleByRef(ctx context.Context, organizationID, roleRef string) (*Role, error)
	UpdateMembershipRole(ctx context.Context, userID, organizationID, roleID string) error
	UpdateMembership(ctx context.Context, organizationID, memberRef string, role *Role, req UpdateMemberRequest) (*Membership, error)
	UpdateMemberPermissions(ctx context.Context, organizationID, memberRef string, customPermissions, deniedPermissions []string) (*Membership, error)
	CreateMembership(ctx context.Context, m *Membership) error
	CreateMembershipTx(ctx context.Context, tx pgx.Tx, m *Membership) error
	ListRoles(ctx context.Context) ([]*Role, error)
	ListRolesForOrg(ctx context.Context, organizationID string) ([]*Role, error)
	ListRolesWithPermissions(ctx context.Context) ([]*RoleWithPermissions, error)
	ListPermissions(ctx context.Context) ([]*Permission, error)
	CreateRole(ctx context.Context, organizationID string, req CreateRoleRequest) (*Role, error)
	UpdateRole(ctx context.Context, organizationID, roleRef string, req UpdateRoleRequest) (*Role, error)
	UpdateRolePermissions(ctx context.Context, organizationID, roleRef string, permissionKeys []string) (*Role, error)
	DeleteRole(ctx context.Context, organizationID, roleRef string) error
	CloneRole(ctx context.Context, organizationID, roleRef, name, description string) (*Role, error)
	CreateInvitation(ctx context.Context, inv *OrganizationInvitation) error
	GetInvitationByRef(ctx context.Context, organizationID, invitationRef string) (*OrganizationInvitation, error)
	GetInvitationByTokenHash(ctx context.Context, organizationID, tokenHash string) (*OrganizationInvitation, error)
	ResendInvitation(ctx context.Context, organizationID, invitationRef, tokenHash string, expiresAt time.Time) (*OrganizationInvitation, error)
	RevokeInvitation(ctx context.Context, organizationID, invitationRef string) error
	AcceptInvitation(ctx context.Context, organizationID, tokenHash, userID string) (*Membership, *OrganizationInvitation, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

func scanPermissionRows(rows pgx.Rows) ([]*Permission, error) {
	var perms []*Permission
	for rows.Next() {
		p := &Permission{}
		if err := rows.Scan(&p.ID, &p.PublicID, &p.KeyName, &p.Resource, &p.Action, &p.Description, &p.IsSystem, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func scanRole(row pgx.Row) (*Role, error) {
	r := &Role{}
	err := row.Scan(&r.ID, &r.PublicID, &r.OrgID, &r.Name, &r.Description, &r.Permissions, &r.IsSystem, &r.IsCustom, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func scanMembership(row pgx.Row) (*Membership, error) {
	m := &Membership{}
	err := row.Scan(
		&m.ID, &m.PublicID, &m.UserID, &m.OrganizationID, &m.RoleID, &m.RoleKey,
		&m.Title, &m.Department, &m.Status, &m.CustomPermissions, &m.DeniedPermissions,
		&m.InvitationStatus, &m.InvitedBy, &m.InvitationSentAt, &m.InvitationAcceptedAt,
		&m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func scanInvitation(row pgx.Row) (*OrganizationInvitation, error) {
	inv := &OrganizationInvitation{}
	err := row.Scan(
		&inv.ID, &inv.PublicID, &inv.OrganizationID, &inv.Email, &inv.RoleID, &inv.RoleKey,
		&inv.Title, &inv.Department, &inv.CustomPermissions, &inv.DeniedPermissions,
		&inv.TokenHash, &inv.Status, &inv.InvitedBy, &inv.AcceptedBy, &inv.ExpiresAt,
		&inv.AcceptedAt, &inv.RevokedAt, &inv.LastSentAt, &inv.ResendCount, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return inv, nil
}

const membershipSelect = `
	id, public_id, user_id, org_id, role_id, role_key,
	COALESCE(title, ''), COALESCE(department, ''), status,
	COALESCE(custom_permissions, ARRAY[]::TEXT[]), COALESCE(denied_permissions, ARRAY[]::TEXT[]),
	invitation_status, invited_by, invitation_sent_at, invitation_accepted_at,
	joined_at, created_at, updated_at`

const roleSelect = `id, public_id, org_id, name, COALESCE(description, ''), COALESCE(permissions, ARRAY[]::TEXT[]), is_system, is_custom, created_at, updated_at`

const invitationSelect = `
	id, public_id, org_id, email, role_id, role_key,
	COALESCE(title, ''), COALESCE(department, ''),
	COALESCE(custom_permissions, ARRAY[]::TEXT[]), COALESCE(denied_permissions, ARRAY[]::TEXT[]),
	token_hash, status, invited_by, accepted_by, expires_at, accepted_at, revoked_at,
	last_sent_at, resend_count, created_at, updated_at`

func (r *repoImpl) GetUserPermissions(ctx context.Context, userID, organizationID string) ([]*Permission, error) {
	const q = `
		WITH member_perms AS (
			SELECT
				COALESCE(r.permissions, ARRAY[]::TEXT[]) || COALESCE(om.custom_permissions, ARRAY[]::TEXT[]) AS granted,
				COALESCE(om.denied_permissions, ARRAY[]::TEXT[]) AS denied
			FROM organization_members om
			LEFT JOIN roles r ON r.id = om.role_id OR (r.org_id IS NULL AND LOWER(r.name) = LOWER(om.role_key))
			WHERE om.user_id = $1
			  AND om.org_id = $2
			  AND om.status = 'active'
			  AND om.invitation_status = 'accepted'
		), member_perm_keys AS (
			SELECT DISTINCT permission_key
			FROM member_perms, UNNEST(granted) AS permission_key
			EXCEPT
			SELECT DISTINCT denied_key
			FROM member_perms, UNNEST(denied) AS denied_key
		)
		SELECT p.id, p.public_id, p.key, p.resource, p.action, COALESCE(p.description, ''), p.is_system, p.created_at, p.updated_at
		FROM permissions p
		JOIN member_perm_keys mpk ON mpk.permission_key = p.key
		ORDER BY p.resource, p.action`
	rows, err := r.db.Query(ctx, q, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: GetUserPermissions: %w", err)
	}
	defer rows.Close()
	perms, err := scanPermissionRows(rows)
	if err != nil {
		return nil, fmt.Errorf("authz: GetUserPermissions: scan: %w", err)
	}
	return perms, nil
}

func (r *repoImpl) GetMembership(ctx context.Context, userID, organizationID string) (*Membership, error) {
	q := `SELECT ` + membershipSelect + ` FROM organization_members WHERE user_id = $1 AND org_id = $2`
	m, err := scanMembership(r.db.QueryRow(ctx, q, userID, organizationID))
	if err != nil {
		return nil, fmt.Errorf("authz: GetMembership: %w", err)
	}
	return m, nil
}

func (r *repoImpl) GetMemberByRef(ctx context.Context, organizationID, memberRef string) (*Membership, error) {
	q := `SELECT ` + membershipSelect + `
		FROM organization_members om
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1
		  AND (om.id::TEXT = $2 OR om.public_id = $2 OR om.user_id::TEXT = $2 OR u.public_id = $2 OR LOWER(u.email) = LOWER($2))`
	m, err := scanMembership(r.db.QueryRow(ctx, q, organizationID, strings.TrimSpace(memberRef)))
	if err != nil {
		return nil, fmt.Errorf("authz: GetMemberByRef: %w", err)
	}
	return m, nil
}

func (r *repoImpl) GetMemberWithUserByRef(ctx context.Context, organizationID, memberRef string) (*MemberWithUser, error) {
	const q = `
		SELECT om.id, om.public_id, u.id, u.public_id, COALESCE(u.email, ''), u.display_name, u.first_name, u.last_name,
		       COALESCE(u.photo_url, ''), om.role_id, om.role_key, om.status,
		       COALESCE(om.title, ''), COALESCE(om.department, ''), om.joined_at
		FROM organization_members om
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1
		  AND (om.id::TEXT = $2 OR om.public_id = $2 OR om.user_id::TEXT = $2 OR u.public_id = $2 OR LOWER(u.email) = LOWER($2))`
	m := &MemberWithUser{}
	err := r.db.QueryRow(ctx, q, organizationID, strings.TrimSpace(memberRef)).Scan(
		&m.MembershipID, &m.MembershipPublic, &m.UserID, &m.UserPublicID, &m.Email, &m.DisplayName, &m.FirstName, &m.LastName,
		&m.PhotoURL, &m.RoleID, &m.Role, &m.Status, &m.Title, &m.Department, &m.JoinedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authz: GetMemberWithUserByRef: %w", err)
	}
	return m, nil
}

func (r *repoImpl) ListMembers(ctx context.Context, organizationID string) ([]*MemberWithUser, error) {
	const q = `
		SELECT om.id, om.public_id, u.id, u.public_id, COALESCE(u.email, ''), u.display_name, u.first_name, u.last_name,
		       COALESCE(u.photo_url, ''), om.role_id, om.role_key, om.status,
		       COALESCE(om.title, ''), COALESCE(om.department, ''), om.joined_at
		FROM organization_members om
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1
		ORDER BY CASE om.role_key
			WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 WHEN 'manager' THEN 3 WHEN 'member' THEN 4 WHEN 'viewer' THEN 5 ELSE 6 END,
			u.display_name ASC`
	rows, err := r.db.Query(ctx, q, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: ListMembers: %w", err)
	}
	defer rows.Close()
	var members []*MemberWithUser
	for rows.Next() {
		m := &MemberWithUser{}
		if err := rows.Scan(&m.MembershipID, &m.MembershipPublic, &m.UserID, &m.UserPublicID, &m.Email, &m.DisplayName, &m.FirstName, &m.LastName, &m.PhotoURL, &m.RoleID, &m.Role, &m.Status, &m.Title, &m.Department, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("authz: ListMembers: scan: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *repoImpl) GetRoleByName(ctx context.Context, name string) (*Role, error) {
	q := `SELECT ` + roleSelect + ` FROM roles WHERE org_id IS NULL AND LOWER(name) = LOWER($1)`
	role, err := scanRole(r.db.QueryRow(ctx, q, strings.TrimSpace(name)))
	if err != nil {
		return nil, fmt.Errorf("authz: GetRoleByName: %w", err)
	}
	return role, nil
}

func (r *repoImpl) GetRoleByID(ctx context.Context, roleID string) (*Role, error) {
	q := `SELECT ` + roleSelect + ` FROM roles WHERE id = $1`
	role, err := scanRole(r.db.QueryRow(ctx, q, roleID))
	if err != nil {
		return nil, fmt.Errorf("authz: GetRoleByID: %w", err)
	}
	return role, nil
}

func (r *repoImpl) GetRoleByRef(ctx context.Context, organizationID, roleRef string) (*Role, error) {
	ref := strings.TrimSpace(roleRef)
	if ref == "" {
		ref = RoleMember
	}
	q := `SELECT ` + roleSelect + `
		FROM roles
		WHERE (org_id IS NULL OR org_id = $1)
		  AND (id::TEXT = $2 OR public_id = $2 OR LOWER(name) = LOWER($2))
		ORDER BY CASE WHEN org_id = $1 THEN 0 ELSE 1 END
		LIMIT 1`
	role, err := scanRole(r.db.QueryRow(ctx, q, organizationID, ref))
	if err != nil {
		return nil, fmt.Errorf("authz: GetRoleByRef: %w", err)
	}
	return role, nil
}

func (r *repoImpl) UpdateMembershipRole(ctx context.Context, userID, organizationID, roleID string) error {
	role, err := r.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return ErrRoleNotFound
	}
	const q = `UPDATE organization_members SET role_id = $1, role_key = $2, updated_at = NOW() WHERE user_id = $3 AND org_id = $4`
	cmd, err := r.db.Exec(ctx, q, roleID, role.Name, userID, organizationID)
	if err != nil {
		return fmt.Errorf("authz: UpdateMembershipRole: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *repoImpl) UpdateMembership(ctx context.Context, organizationID, memberRef string, role *Role, req UpdateMemberRequest) (*Membership, error) {
	existing, err := r.GetMemberByRef(ctx, organizationID, memberRef)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrMemberNotFound
	}
	roleID := existing.RoleID
	roleKey := existing.RoleKey
	if role != nil {
		roleID = &role.ID
		roleKey = role.Name
	}
	status := existing.Status
	if strings.TrimSpace(req.Status) != "" {
		status = strings.ToLower(strings.TrimSpace(req.Status))
	}
	title := existing.Title
	if req.Title != "" {
		title = strings.TrimSpace(req.Title)
	}
	department := existing.Department
	if req.Department != "" {
		department = strings.TrimSpace(req.Department)
	}
	custom := existing.CustomPermissions
	if req.CustomPermissions != nil {
		custom = normaliseKeys(req.CustomPermissions)
	}
	denied := existing.DeniedPermissions
	if req.DeniedPermissions != nil {
		denied = normaliseKeys(req.DeniedPermissions)
	}

	q := `UPDATE organization_members
		SET role_id = $1, role_key = $2, status = $3, title = NULLIF($4, ''), department = NULLIF($5, ''),
		    custom_permissions = $6, denied_permissions = $7, updated_at = NOW()
		WHERE id = $8
		RETURNING ` + membershipSelect
	m, err := scanMembership(r.db.QueryRow(ctx, q, roleID, roleKey, status, title, department, custom, denied, existing.ID))
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateMembership: %w", err)
	}
	return m, nil
}

func (r *repoImpl) UpdateMemberPermissions(ctx context.Context, organizationID, memberRef string, customPermissions, deniedPermissions []string) (*Membership, error) {
	existing, err := r.GetMemberByRef(ctx, organizationID, memberRef)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrMemberNotFound
	}
	q := `UPDATE organization_members
		SET custom_permissions = $1, denied_permissions = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING ` + membershipSelect
	m, err := scanMembership(r.db.QueryRow(ctx, q, normaliseKeys(customPermissions), normaliseKeys(deniedPermissions), existing.ID))
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateMemberPermissions: %w", err)
	}
	return m, nil
}

func (r *repoImpl) CreateMembership(ctx context.Context, m *Membership) error {
	const q = `
		INSERT INTO organization_members (user_id, org_id, role_id, role_key, title, department, status, custom_permissions, denied_permissions, invitation_status, invited_by, invitation_sent_at, invitation_accepted_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), COALESCE(NULLIF($7, ''), 'active'), $8, $9, COALESCE(NULLIF($10, ''), 'accepted'), $11, $12, $13)
		RETURNING id, public_id, joined_at, created_at, updated_at`
	return r.db.QueryRow(ctx, q, m.UserID, m.OrganizationID, m.RoleID, m.RoleKey, m.Title, m.Department, m.Status, m.CustomPermissions, m.DeniedPermissions, m.InvitationStatus, m.InvitedBy, m.InvitationSentAt, m.InvitationAcceptedAt).
		Scan(&m.ID, &m.PublicID, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt)
}

func (r *repoImpl) CreateMembershipTx(ctx context.Context, tx pgx.Tx, m *Membership) error {
	const q = `
		INSERT INTO organization_members (user_id, org_id, role_id, role_key, title, department, status, custom_permissions, denied_permissions, invitation_status, invited_by, invitation_sent_at, invitation_accepted_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), COALESCE(NULLIF($7, ''), 'active'), $8, $9, COALESCE(NULLIF($10, ''), 'accepted'), $11, $12, $13)
		RETURNING id, public_id, joined_at, created_at, updated_at`
	return tx.QueryRow(ctx, q, m.UserID, m.OrganizationID, m.RoleID, m.RoleKey, m.Title, m.Department, m.Status, m.CustomPermissions, m.DeniedPermissions, m.InvitationStatus, m.InvitedBy, m.InvitationSentAt, m.InvitationAcceptedAt).
		Scan(&m.ID, &m.PublicID, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt)
}

func (r *repoImpl) ListRoles(ctx context.Context) ([]*Role, error) {
	q := `SELECT ` + roleSelect + `
		FROM roles
		WHERE org_id IS NULL
		ORDER BY CASE name WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 WHEN 'manager' THEN 3 WHEN 'member' THEN 4 WHEN 'viewer' THEN 5 ELSE 6 END, name`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRoles: %w", err)
	}
	defer rows.Close()
	return scanRoleRows(rows)
}

func (r *repoImpl) ListRolesForOrg(ctx context.Context, organizationID string) ([]*Role, error) {
	q := `SELECT ` + roleSelect + `
		FROM roles
		WHERE org_id IS NULL OR org_id = $1
		ORDER BY CASE WHEN org_id IS NULL THEN 0 ELSE 1 END,
		CASE name WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 WHEN 'manager' THEN 3 WHEN 'member' THEN 4 WHEN 'viewer' THEN 5 ELSE 6 END, name`
	rows, err := r.db.Query(ctx, q, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRolesForOrg: %w", err)
	}
	defer rows.Close()
	return scanRoleRows(rows)
}

func scanRoleRows(rows pgx.Rows) ([]*Role, error) {
	var roles []*Role
	for rows.Next() {
		role := &Role{}
		if err := rows.Scan(&role.ID, &role.PublicID, &role.OrgID, &role.Name, &role.Description, &role.Permissions, &role.IsSystem, &role.IsCustom, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *repoImpl) ListRolesWithPermissions(ctx context.Context) ([]*RoleWithPermissions, error) {
	roles, err := r.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	return rolesWithPermissions(ctx, r, roles)
}

func rolesWithPermissions(ctx context.Context, r *repoImpl, roles []*Role) ([]*RoleWithPermissions, error) {
	allPerms, err := r.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	permByKey := map[string]*Permission{}
	for _, p := range allPerms {
		permByKey[p.Key()] = p
	}
	result := make([]*RoleWithPermissions, 0, len(roles))
	for _, role := range roles {
		perms := []*Permission{}
		for _, key := range role.Permissions {
			if p := permByKey[key]; p != nil {
				perms = append(perms, p)
			}
		}
		result = append(result, &RoleWithPermissions{Role: role, Permissions: perms})
	}
	return result, nil
}

func (r *repoImpl) ListPermissions(ctx context.Context) ([]*Permission, error) {
	const q = `SELECT id, public_id, key, resource, action, COALESCE(description, ''), is_system, created_at, updated_at FROM permissions ORDER BY resource, action`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("authz: ListPermissions: %w", err)
	}
	defer rows.Close()
	perms, err := scanPermissionRows(rows)
	if err != nil {
		return nil, fmt.Errorf("authz: ListPermissions: scan: %w", err)
	}
	return perms, nil
}

func (r *repoImpl) CreateRole(ctx context.Context, organizationID string, req CreateRoleRequest) (*Role, error) {
	const q = `
		INSERT INTO roles (org_id, name, description, permissions, is_system, is_custom)
		VALUES ($1, $2, NULLIF($3, ''), $4, FALSE, TRUE)
		RETURNING id, public_id, org_id, name, COALESCE(description, ''), permissions, is_system, is_custom, created_at, updated_at`
	role, err := scanRole(r.db.QueryRow(ctx, q, organizationID, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), normaliseKeys(req.PermissionKeys)))
	if err != nil {
		return nil, fmt.Errorf("authz: CreateRole: %w", err)
	}
	return role, nil
}

func (r *repoImpl) UpdateRole(ctx context.Context, organizationID, roleRef string, req UpdateRoleRequest) (*Role, error) {
	role, err := r.GetRoleByRef(ctx, organizationID, roleRef)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, ErrRoleNotFound
	}
	if role.OrgID == nil || role.IsSystem {
		return nil, ErrCannotModifySystemRole
	}
	name := role.Name
	if strings.TrimSpace(req.Name) != "" {
		name = strings.TrimSpace(req.Name)
	}
	description := role.Description
	if req.Description != "" {
		description = strings.TrimSpace(req.Description)
	}
	permissions := role.Permissions
	if req.PermissionKeys != nil {
		permissions = normaliseKeys(req.PermissionKeys)
	}
	q := `UPDATE roles SET name = $1, description = NULLIF($2, ''), permissions = $3, updated_at = NOW()
		WHERE id = $4 AND org_id = $5 AND is_system = FALSE
		RETURNING ` + roleSelect
	updated, err := scanRole(r.db.QueryRow(ctx, q, name, description, permissions, role.ID, organizationID))
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateRole: %w", err)
	}
	return updated, nil
}

func (r *repoImpl) UpdateRolePermissions(ctx context.Context, organizationID, roleRef string, permissionKeys []string) (*Role, error) {
	role, err := r.GetRoleByRef(ctx, organizationID, roleRef)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, ErrRoleNotFound
	}
	if role.OrgID == nil || role.IsSystem {
		return nil, ErrCannotModifySystemRole
	}
	q := `UPDATE roles SET permissions = $1, updated_at = NOW()
		WHERE id = $2 AND org_id = $3 AND is_system = FALSE
		RETURNING ` + roleSelect
	updated, err := scanRole(r.db.QueryRow(ctx, q, normaliseKeys(permissionKeys), role.ID, organizationID))
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateRolePermissions: %w", err)
	}
	return updated, nil
}

func (r *repoImpl) DeleteRole(ctx context.Context, organizationID, roleRef string) error {
	role, err := r.GetRoleByRef(ctx, organizationID, roleRef)
	if err != nil {
		return err
	}
	if role == nil {
		return ErrRoleNotFound
	}
	if role.OrgID == nil || role.IsSystem {
		return ErrCannotModifySystemRole
	}
	fallback, err := r.GetRoleByRef(ctx, organizationID, RoleMember)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("authz: DeleteRole: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var fallbackID *string
	fallbackKey := RoleMember
	if fallback != nil {
		fallbackID = &fallback.ID
		fallbackKey = fallback.Name
	}
	if _, err := tx.Exec(ctx, `UPDATE organization_members SET role_id = $1, role_key = $2, updated_at = NOW() WHERE org_id = $3 AND role_id = $4`, fallbackID, fallbackKey, organizationID, role.ID); err != nil {
		return fmt.Errorf("authz: DeleteRole: reassign members: %w", err)
	}
	cmd, err := tx.Exec(ctx, `DELETE FROM roles WHERE id = $1 AND org_id = $2 AND is_system = FALSE`, role.ID, organizationID)
	if err != nil {
		return fmt.Errorf("authz: DeleteRole: delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("authz: DeleteRole: commit: %w", err)
	}
	return nil
}

func (r *repoImpl) CloneRole(ctx context.Context, organizationID, roleRef, name, description string) (*Role, error) {
	source, err := r.GetRoleByRef(ctx, organizationID, roleRef)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, ErrRoleNotFound
	}
	if strings.TrimSpace(name) == "" {
		name = source.Name + " copy"
	}
	if strings.TrimSpace(description) == "" {
		description = "Cloned from " + source.Name
	}
	return r.CreateRole(ctx, organizationID, CreateRoleRequest{Name: name, Description: description, PermissionKeys: source.Permissions})
}

func (r *repoImpl) CreateInvitation(ctx context.Context, inv *OrganizationInvitation) error {
	const q = `
		INSERT INTO organization_invitations (
			org_id, email, role_id, role_key, title, department, custom_permissions, denied_permissions,
			token_hash, status, invited_by, expires_at, last_sent_at
		)
		VALUES ($1, LOWER($2), $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9, 'pending', $10, $11, NOW())
		RETURNING ` + invitationSelect
	created, err := scanInvitation(r.db.QueryRow(ctx, q,
		inv.OrganizationID, inv.Email, inv.RoleID, inv.RoleKey, inv.Title, inv.Department,
		inv.CustomPermissions, inv.DeniedPermissions, inv.TokenHash, inv.InvitedBy, inv.ExpiresAt,
	))
	if err != nil {
		return fmt.Errorf("authz: CreateInvitation: %w", err)
	}
	*inv = *created
	return nil
}

func (r *repoImpl) GetInvitationByRef(ctx context.Context, organizationID, invitationRef string) (*OrganizationInvitation, error) {
	q := `SELECT ` + invitationSelect + ` FROM organization_invitations WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	inv, err := scanInvitation(r.db.QueryRow(ctx, q, organizationID, strings.TrimSpace(invitationRef)))
	if err != nil {
		return nil, fmt.Errorf("authz: GetInvitationByRef: %w", err)
	}
	return inv, nil
}

func (r *repoImpl) GetInvitationByTokenHash(ctx context.Context, organizationID, tokenHash string) (*OrganizationInvitation, error) {
	q := `SELECT ` + invitationSelect + ` FROM organization_invitations WHERE org_id = $1 AND token_hash = $2`
	inv, err := scanInvitation(r.db.QueryRow(ctx, q, organizationID, tokenHash))
	if err != nil {
		return nil, fmt.Errorf("authz: GetInvitationByTokenHash: %w", err)
	}
	return inv, nil
}

func (r *repoImpl) ResendInvitation(ctx context.Context, organizationID, invitationRef, tokenHash string, expiresAt time.Time) (*OrganizationInvitation, error) {
	inv, err := r.GetInvitationByRef(ctx, organizationID, invitationRef)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, ErrInvitationNotFound
	}
	q := `UPDATE organization_invitations
		SET token_hash = $1, expires_at = $2, last_sent_at = NOW(), resend_count = resend_count + 1, status = 'pending', updated_at = NOW()
		WHERE id = $3 AND org_id = $4 AND status = 'pending'
		RETURNING ` + invitationSelect
	updated, err := scanInvitation(r.db.QueryRow(ctx, q, tokenHash, expiresAt, inv.ID, organizationID))
	if err != nil {
		return nil, fmt.Errorf("authz: ResendInvitation: %w", err)
	}
	if updated == nil {
		return nil, ErrInvitationNotPending
	}
	return updated, nil
}

func (r *repoImpl) RevokeInvitation(ctx context.Context, organizationID, invitationRef string) error {
	const q = `UPDATE organization_invitations SET status = 'revoked', revoked_at = NOW(), updated_at = NOW()
		WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2) AND status = 'pending'`
	cmd, err := r.db.Exec(ctx, q, organizationID, strings.TrimSpace(invitationRef))
	if err != nil {
		return fmt.Errorf("authz: RevokeInvitation: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

func (r *repoImpl) AcceptInvitation(ctx context.Context, organizationID, tokenHash, userID string) (*Membership, *OrganizationInvitation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("authz: AcceptInvitation: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := `SELECT ` + invitationSelect + ` FROM organization_invitations WHERE org_id = $1 AND token_hash = $2 FOR UPDATE`
	inv, err := scanInvitation(tx.QueryRow(ctx, q, organizationID, tokenHash))
	if err != nil {
		return nil, nil, fmt.Errorf("authz: AcceptInvitation: select invitation: %w", err)
	}
	if inv == nil {
		return nil, nil, ErrInvitationNotFound
	}
	if inv.Status != "pending" {
		return nil, nil, ErrInvitationNotPending
	}
	if time.Now().After(inv.ExpiresAt) {
		_, _ = tx.Exec(ctx, `UPDATE organization_invitations SET status = 'expired', updated_at = NOW() WHERE id = $1`, inv.ID)
		return nil, nil, ErrInvitationExpired
	}
	var userEmail string
	if err := tx.QueryRow(ctx, `SELECT LOWER(COALESCE(email, '')) FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&userEmail); err != nil {
		return nil, nil, fmt.Errorf("authz: AcceptInvitation: user: %w", err)
	}
	if userEmail == "" || strings.ToLower(inv.Email) != userEmail {
		return nil, nil, ErrInvitationEmailMismatch
	}

	insertQ := `
		INSERT INTO organization_members (org_id, user_id, role_id, role_key, title, department, status, custom_permissions, denied_permissions, invitation_status, invited_by, invitation_sent_at, invitation_accepted_at, joined_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), 'active', $7, $8, 'accepted', $9, $10, NOW(), NOW())
		ON CONFLICT (org_id, user_id) DO UPDATE SET
			role_id = EXCLUDED.role_id,
			role_key = EXCLUDED.role_key,
			title = EXCLUDED.title,
			department = EXCLUDED.department,
			status = 'active',
			custom_permissions = EXCLUDED.custom_permissions,
			denied_permissions = EXCLUDED.denied_permissions,
			invitation_status = 'accepted',
			invited_by = EXCLUDED.invited_by,
			invitation_sent_at = EXCLUDED.invitation_sent_at,
			invitation_accepted_at = NOW(),
			updated_at = NOW()
		RETURNING ` + membershipSelect
	member, err := scanMembership(tx.QueryRow(ctx, insertQ,
		inv.OrganizationID, userID, inv.RoleID, inv.RoleKey, inv.Title, inv.Department,
		inv.CustomPermissions, inv.DeniedPermissions, inv.InvitedBy, inv.LastSentAt,
	))
	if err != nil {
		return nil, nil, fmt.Errorf("authz: AcceptInvitation: upsert member: %w", err)
	}
	updateQ := `UPDATE organization_invitations SET status = 'accepted', accepted_by = $1, accepted_at = NOW(), updated_at = NOW() WHERE id = $2 RETURNING ` + invitationSelect
	accepted, err := scanInvitation(tx.QueryRow(ctx, updateQ, userID, inv.ID))
	if err != nil {
		return nil, nil, fmt.Errorf("authz: AcceptInvitation: update invitation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("authz: AcceptInvitation: commit: %w", err)
	}
	return member, accepted, nil
}
