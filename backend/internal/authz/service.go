package authz

import (
	"context"
	"fmt"
)

// Service defines the authorization interface.
// The core function is Can() — all permission checks flow through it.
type Service interface {
	// Can checks whether userID has the given permission within businessID.
	// resource: "tasks", "members", "billing", etc.
	// action:   "read", "create", "update", "delete", "manage"
	Can(ctx context.Context, userID, businessID, resource, action string) (bool, error)

	// GetMembership returns the user's membership in a business.
	GetMembership(ctx context.Context, userID, businessID string) (*Membership, error)

	// AssignRole changes a member's role within a business.
	// Only an Owner or Admin can do this — enforced in the handler via RequirePermission.
	AssignRole(ctx context.Context, targetUserID, businessID, roleName string) error

	// ListRoles returns all system roles with their permissions.
	ListRoles(ctx context.Context) ([]*Role, error)

	// ListPermissions returns all defined permissions.
	ListPermissions(ctx context.Context) ([]*Permission, error)
}

type serviceImpl struct {
	repo Repository
	// TODO (Phase 1-D): add Redis client for permission caching
}

// NewService creates a new authz service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) Can(_ context.Context, _, _, _, _ string) (bool, error) {
	return false, errNotImplemented("Can")
}

func (s *serviceImpl) GetMembership(_ context.Context, _, _ string) (*Membership, error) {
	return nil, errNotImplemented("GetMembership")
}

func (s *serviceImpl) AssignRole(_ context.Context, _, _, _ string) error {
	return errNotImplemented("AssignRole")
}

func (s *serviceImpl) ListRoles(_ context.Context) ([]*Role, error) {
	return nil, errNotImplemented("ListRoles")
}

func (s *serviceImpl) ListPermissions(_ context.Context) ([]*Permission, error) {
	return nil, errNotImplemented("ListPermissions")
}

func errNotImplemented(method string) error {
	return fmt.Errorf("authz: %s: not yet implemented", method)
}
