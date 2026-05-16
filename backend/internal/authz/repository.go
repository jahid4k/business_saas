// backend/internal/authz/repository.go
package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for RBAC operations.
type Repository interface {
	// GetUserPermissions returns all permissions a user holds in a business.
	// Core query: memberships → role_permissions → permissions.
	GetUserPermissions(ctx context.Context, userID, businessID string) ([]*Permission, error)

	// GetMembership returns a user's membership in a business.
	// Returns nil, nil when no membership exists.
	GetMembership(ctx context.Context, userID, businessID string) (*Membership, error)

	// ListMembers returns all members of a business enriched with user profile + role.
	ListMembers(ctx context.Context, businessID string) ([]*MemberWithUser, error)

	// GetRoleByName returns a role by its name. Returns nil, nil when not found.
	GetRoleByName(ctx context.Context, name string) (*Role, error)

	// GetRoleByID returns a role by its UUID. Returns nil, nil when not found.
	GetRoleByID(ctx context.Context, roleID string) (*Role, error)

	// UpdateMembershipRole changes the role on an existing membership.
	UpdateMembershipRole(ctx context.Context, userID, businessID, roleID string) error

	// CreateMembership inserts a new membership (outside a transaction).
	CreateMembership(ctx context.Context, m *Membership) error

	// CreateMembershipTx inserts a new membership inside an existing transaction.
	// Used by business.Service.Create to keep business + membership atomic.
	CreateMembershipTx(ctx context.Context, tx pgx.Tx, m *Membership) error

	// ListRoles returns all roles ordered by name.
	ListRoles(ctx context.Context) ([]*Role, error)

	// ListRolesWithPermissions returns all roles with their associated permissions.
	ListRolesWithPermissions(ctx context.Context) ([]*RoleWithPermissions, error)

	// ListPermissions returns all defined permissions ordered by resource + action.
	ListPermissions(ctx context.Context) ([]*Permission, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new authz repository backed by a pgxpool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// ----------------------------------------------------------
// GetUserPermissions
// ----------------------------------------------------------

// GetUserPermissions is the hot-path permission query.
// It joins memberships → role_permissions → permissions in one trip.
// Phase 1-D caches results in Redis — this is the cache-miss fallback.
func (r *repoImpl) GetUserPermissions(ctx context.Context, userID, businessID string) ([]*Permission, error) {
	const q = `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM memberships      m
		JOIN role_permissions rp ON rp.role_id       = m.role_id
		JOIN permissions      p  ON p.id             = rp.permission_id
		WHERE m.user_id     = $1
		  AND m.business_id = $2`

	rows, err := r.db.Query(ctx, q, userID, businessID)
	if err != nil {
		return nil, fmt.Errorf("authz: GetUserPermissions: %w", err)
	}
	defer rows.Close()

	var perms []*Permission
	for rows.Next() {
		p := &Permission{}
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("authz: GetUserPermissions: scan: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ----------------------------------------------------------
// GetMembership
// ----------------------------------------------------------

// GetMembership returns a single membership record. Returns nil, nil when not found.
func (r *repoImpl) GetMembership(ctx context.Context, userID, businessID string) (*Membership, error) {
	const q = `
		SELECT id, user_id, business_id, role_id, created_at, updated_at
		FROM memberships
		WHERE user_id     = $1
		  AND business_id = $2`

	m := &Membership{}
	err := r.db.QueryRow(ctx, q, userID, businessID).Scan(
		&m.ID, &m.UserID, &m.BusinessID, &m.RoleID, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authz: GetMembership: %w", err)
	}
	return m, nil
}

// ----------------------------------------------------------
// ListMembers
// ----------------------------------------------------------

// ListMembers returns all members of a business, enriched with user profile + role name.
// Query joins: memberships → users → roles
// Ordered by role weight (owner first) then by user name.
func (r *repoImpl) ListMembers(ctx context.Context, businessID string) ([]*MemberWithUser, error) {
	const q = `
		SELECT
			m.id          AS membership_id,
			u.id          AS user_id,
			u.email,
			u.first_name,
			u.last_name,
			r.name        AS role,
			m.created_at  AS joined_at
		FROM memberships m
		JOIN users u ON u.id = m.user_id
		JOIN roles r ON r.id = m.role_id
		WHERE m.business_id = $1
		ORDER BY
			CASE r.name
				WHEN 'owner'  THEN 1
				WHEN 'admin'  THEN 2
				WHEN 'member' THEN 3
				WHEN 'viewer' THEN 4
				ELSE 5
			END,
			u.first_name ASC,
			u.last_name  ASC`

	rows, err := r.db.Query(ctx, q, businessID)
	if err != nil {
		return nil, fmt.Errorf("authz: ListMembers: %w", err)
	}
	defer rows.Close()

	var members []*MemberWithUser
	for rows.Next() {
		m := &MemberWithUser{}
		if err := rows.Scan(
			&m.MembershipID, &m.UserID, &m.Email,
			&m.FirstName, &m.LastName, &m.Role, &m.JoinedAt,
		); err != nil {
			return nil, fmt.Errorf("authz: ListMembers: scan: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// ----------------------------------------------------------
// GetRoleByName / GetRoleByID
// ----------------------------------------------------------

func (r *repoImpl) GetRoleByName(ctx context.Context, name string) (*Role, error) {
	const q = `SELECT id, name, description, is_system, created_at FROM roles WHERE name = $1`

	role := &Role{}
	err := r.db.QueryRow(ctx, q, name).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authz: GetRoleByName: %w", err)
	}
	return role, nil
}

func (r *repoImpl) GetRoleByID(ctx context.Context, roleID string) (*Role, error) {
	const q = `SELECT id, name, description, is_system, created_at FROM roles WHERE id = $1`

	role := &Role{}
	err := r.db.QueryRow(ctx, q, roleID).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authz: GetRoleByID: %w", err)
	}
	return role, nil
}

// ----------------------------------------------------------
// UpdateMembershipRole
// ----------------------------------------------------------

func (r *repoImpl) UpdateMembershipRole(ctx context.Context, userID, businessID, roleID string) error {
	const q = `
		UPDATE memberships
		SET role_id    = $1,
		    updated_at = NOW()
		WHERE user_id     = $2
		  AND business_id = $3`

	cmd, err := r.db.Exec(ctx, q, roleID, userID, businessID)
	if err != nil {
		return fmt.Errorf("authz: UpdateMembershipRole: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// ----------------------------------------------------------
// CreateMembership / CreateMembershipTx
// ----------------------------------------------------------

func (r *repoImpl) CreateMembership(ctx context.Context, m *Membership) error {
	const q = `
		INSERT INTO memberships (user_id, business_id, role_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRow(ctx, q, m.UserID, m.BusinessID, m.RoleID).
		Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
}

func (r *repoImpl) CreateMembershipTx(ctx context.Context, tx pgx.Tx, m *Membership) error {
	const q = `
		INSERT INTO memberships (user_id, business_id, role_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	return tx.QueryRow(ctx, q, m.UserID, m.BusinessID, m.RoleID).
		Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
}

// ----------------------------------------------------------
// ListRoles / ListRolesWithPermissions / ListPermissions
// ----------------------------------------------------------

func (r *repoImpl) ListRoles(ctx context.Context) ([]*Role, error) {
	const q = `SELECT id, name, description, is_system, created_at FROM roles ORDER BY name`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRoles: %w", err)
	}
	defer rows.Close()

	var roles []*Role
	for rows.Next() {
		role := &Role{}
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("authz: ListRoles: scan: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// ListRolesWithPermissions returns each role with its full permission list.
// Uses two queries: one for roles, one for all role_permissions joined to permissions.
// This avoids N+1 queries while remaining readable.
func (r *repoImpl) ListRolesWithPermissions(ctx context.Context) ([]*RoleWithPermissions, error) {
	// Step 1: load all roles
	roles, err := r.ListRoles(ctx)
	if err != nil {
		return nil, err
	}

	// Step 2: load all role_permissions in one query
	const q = `
		SELECT rp.role_id, p.id, p.resource, p.action, p.description, p.created_at
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		ORDER BY rp.role_id, p.resource, p.action`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRolesWithPermissions: %w", err)
	}
	defer rows.Close()

	// Build roleID → []*Permission index
	permsByRole := make(map[string][]*Permission)
	for rows.Next() {
		var roleID string
		p := &Permission{}
		if err := rows.Scan(&roleID, &p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("authz: ListRolesWithPermissions: scan: %w", err)
		}
		permsByRole[roleID] = append(permsByRole[roleID], p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("authz: ListRolesWithPermissions: rows: %w", err)
	}

	// Step 3: assemble result
	result := make([]*RoleWithPermissions, len(roles))
	for i, role := range roles {
		perms := permsByRole[role.ID]
		if perms == nil {
			perms = []*Permission{}
		}
		result[i] = &RoleWithPermissions{Role: role, Permissions: perms}
	}
	return result, nil
}

func (r *repoImpl) ListPermissions(ctx context.Context) ([]*Permission, error) {
	const q = `
		SELECT id, resource, action, description, created_at
		FROM permissions
		ORDER BY resource, action`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("authz: ListPermissions: %w", err)
	}
	defer rows.Close()

	var perms []*Permission
	for rows.Next() {
		p := &Permission{}
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("authz: ListPermissions: scan: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}
