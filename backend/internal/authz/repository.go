package authz

import "context"

// Repository defines the data access interface for RBAC operations.
type Repository interface {
	// GetUserPermissions returns all permissions the user holds in the business.
	// This is the core query: user → membership → role → role_permissions → permissions.
	GetUserPermissions(ctx context.Context, userID, businessID string) ([]*Permission, error)

	// GetMembership returns the user's membership record for a business.
	GetMembership(ctx context.Context, userID, businessID string) (*Membership, error)

	// GetRoleByName returns a role by its system name (e.g. "admin").
	GetRoleByName(ctx context.Context, name string) (*Role, error)

	// UpdateMembershipRole changes the role on an existing membership.
	UpdateMembershipRole(ctx context.Context, userID, businessID, roleID string) error

	// CreateMembership creates a new membership record.
	// Used when a user creates a business (gets Owner role) or is invited.
	CreateMembership(ctx context.Context, m *Membership) error

	// ListRoles returns all roles with their associated permissions.
	ListRoles(ctx context.Context) ([]*Role, error)

	// ListPermissions returns all defined permissions.
	ListPermissions(ctx context.Context) ([]*Permission, error)
}

type repoImpl struct {
	// TODO (Phase 1-D): pgxpool.Pool
}

// NewRepository creates a new authz repository.
func NewRepository() Repository {
	return &repoImpl{}
}

func (r *repoImpl) GetUserPermissions(_ context.Context, _, _ string) ([]*Permission, error) {
	return nil, nil
}
func (r *repoImpl) GetMembership(_ context.Context, _, _ string) (*Membership, error) {
	return nil, nil
}
func (r *repoImpl) GetRoleByName(_ context.Context, _ string) (*Role, error)     { return nil, nil }
func (r *repoImpl) UpdateMembershipRole(_ context.Context, _, _, _ string) error { return nil }
func (r *repoImpl) CreateMembership(_ context.Context, _ *Membership) error      { return nil }
func (r *repoImpl) ListRoles(_ context.Context) ([]*Role, error)                 { return nil, nil }
func (r *repoImpl) ListPermissions(_ context.Context) ([]*Permission, error)     { return nil, nil }
