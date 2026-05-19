// backend/internal/authz/repository.go
package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetUserPermissions(ctx context.Context, userID, organizationID string) ([]*Permission, error)
	GetMembership(ctx context.Context, userID, organizationID string) (*Membership, error)
	ListMembers(ctx context.Context, organizationID string) ([]*MemberWithUser, error)
	GetRoleByName(ctx context.Context, name string) (*Role, error)
	GetRoleByID(ctx context.Context, roleID string) (*Role, error)
	UpdateMembershipRole(ctx context.Context, userID, organizationID, roleID string) error
	CreateMembership(ctx context.Context, m *Membership) error
	CreateMembershipTx(ctx context.Context, tx pgx.Tx, m *Membership) error
	ListRoles(ctx context.Context) ([]*Role, error)
	ListRolesWithPermissions(ctx context.Context) ([]*RoleWithPermissions, error)
	ListPermissions(ctx context.Context) ([]*Permission, error)
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

func (r *repoImpl) GetUserPermissions(ctx context.Context, userID, organizationID string) ([]*Permission, error) {
	const q = `
		WITH member_perm_keys AS (
			SELECT DISTINCT UNNEST(COALESCE(r.permissions, ARRAY[]::TEXT[]) || COALESCE(om.custom_permissions, ARRAY[]::TEXT[])) AS permission_key
			FROM organization_members om
			LEFT JOIN roles r ON r.id = om.role_id OR (r.org_id IS NULL AND LOWER(r.name) = LOWER(om.role_key))
			WHERE om.user_id = $1
			  AND om.org_id = $2
			  AND om.status = 'active'
			  AND om.invitation_status = 'accepted'
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
	const q = `
		SELECT id, public_id, user_id, org_id, role_id, role_key,
		       COALESCE(title, ''), COALESCE(department, ''), status, custom_permissions,
		       invitation_status, invited_by, invitation_sent_at, invitation_accepted_at,
		       joined_at, created_at, updated_at
		FROM organization_members
		WHERE user_id = $1 AND org_id = $2`
	m := &Membership{}
	err := r.db.QueryRow(ctx, q, userID, organizationID).Scan(
		&m.ID, &m.PublicID, &m.UserID, &m.OrganizationID, &m.RoleID, &m.RoleKey,
		&m.Title, &m.Department, &m.Status, &m.CustomPermissions,
		&m.InvitationStatus, &m.InvitedBy, &m.InvitationSentAt, &m.InvitationAcceptedAt,
		&m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authz: GetMembership: %w", err)
	}
	return m, nil
}

func (r *repoImpl) ListMembers(ctx context.Context, organizationID string) ([]*MemberWithUser, error) {
	const q = `
		SELECT om.id, u.id, COALESCE(u.email, ''), u.display_name, u.first_name, u.last_name,
		       COALESCE(u.photo_url, ''), om.role_key, om.status,
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
		if err := rows.Scan(&m.MembershipID, &m.UserID, &m.Email, &m.DisplayName, &m.FirstName, &m.LastName, &m.PhotoURL, &m.Role, &m.Status, &m.Title, &m.Department, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("authz: ListMembers: scan: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
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

func (r *repoImpl) GetRoleByName(ctx context.Context, name string) (*Role, error) {
	const q = `
		SELECT id, public_id, org_id, name, COALESCE(description, ''), permissions, is_system, is_custom, created_at, updated_at
		FROM roles
		WHERE org_id IS NULL AND LOWER(name) = LOWER($1)`
	role, err := scanRole(r.db.QueryRow(ctx, q, strings.TrimSpace(name)))
	if err != nil {
		return nil, fmt.Errorf("authz: GetRoleByName: %w", err)
	}
	return role, nil
}

func (r *repoImpl) GetRoleByID(ctx context.Context, roleID string) (*Role, error) {
	const q = `
		SELECT id, public_id, org_id, name, COALESCE(description, ''), permissions, is_system, is_custom, created_at, updated_at
		FROM roles
		WHERE id = $1`
	role, err := scanRole(r.db.QueryRow(ctx, q, roleID))
	if err != nil {
		return nil, fmt.Errorf("authz: GetRoleByID: %w", err)
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

func (r *repoImpl) CreateMembership(ctx context.Context, m *Membership) error {
	const q = `
		INSERT INTO organization_members (user_id, org_id, role_id, role_key, title, department, status, custom_permissions, invitation_status)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), COALESCE(NULLIF($7, ''), 'active'), $8, COALESCE(NULLIF($9, ''), 'accepted'))
		RETURNING id, public_id, joined_at, created_at, updated_at`
	return r.db.QueryRow(ctx, q, m.UserID, m.OrganizationID, m.RoleID, m.RoleKey, m.Title, m.Department, m.Status, m.CustomPermissions, m.InvitationStatus).
		Scan(&m.ID, &m.PublicID, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt)
}

func (r *repoImpl) CreateMembershipTx(ctx context.Context, tx pgx.Tx, m *Membership) error {
	const q = `
		INSERT INTO organization_members (user_id, org_id, role_id, role_key, title, department, status, custom_permissions, invitation_status)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), COALESCE(NULLIF($7, ''), 'active'), $8, COALESCE(NULLIF($9, ''), 'accepted'))
		RETURNING id, public_id, joined_at, created_at, updated_at`
	return tx.QueryRow(ctx, q, m.UserID, m.OrganizationID, m.RoleID, m.RoleKey, m.Title, m.Department, m.Status, m.CustomPermissions, m.InvitationStatus).
		Scan(&m.ID, &m.PublicID, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt)
}

func (r *repoImpl) ListRoles(ctx context.Context) ([]*Role, error) {
	const q = `
		SELECT id, public_id, org_id, name, COALESCE(description, ''), permissions, is_system, is_custom, created_at, updated_at
		FROM roles
		WHERE org_id IS NULL
		ORDER BY CASE name WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 WHEN 'manager' THEN 3 WHEN 'member' THEN 4 WHEN 'viewer' THEN 5 ELSE 6 END, name`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRoles: %w", err)
	}
	defer rows.Close()
	var roles []*Role
	for rows.Next() {
		role := &Role{}
		if err := rows.Scan(&role.ID, &role.PublicID, &role.OrgID, &role.Name, &role.Description, &role.Permissions, &role.IsSystem, &role.IsCustom, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, fmt.Errorf("authz: ListRoles: scan: %w", err)
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
	allPerms, err := r.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	permByKey := map[string]*Permission{}
	for _, p := range allPerms {
		permByKey[p.Key()] = p
	}
	result := make([]*RoleWithPermissions, len(roles))
	for i, role := range roles {
		var perms []*Permission
		for _, key := range role.Permissions {
			if p := permByKey[key]; p != nil {
				perms = append(perms, p)
			}
		}
		if perms == nil {
			perms = []*Permission{}
		}
		result[i] = &RoleWithPermissions{Role: role, Permissions: perms}
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
