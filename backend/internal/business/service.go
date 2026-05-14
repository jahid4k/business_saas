package business

import (
	"context"
	"errors"
	"fmt"
)

// Service defines the business logic interface for workspace management.
type Service interface {
	Create(ctx context.Context, ownerID string, req CreateBusinessRequest) (*Business, error)
	GetByID(ctx context.Context, businessID, requestingUserID string) (*Business, error)
	ListForUser(ctx context.Context, userID string) ([]*Business, error)
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new business service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) Create(_ context.Context, _ string, _ CreateBusinessRequest) (*Business, error) {
	return nil, errNotImplemented("Create")
}

func (s *serviceImpl) GetByID(_ context.Context, _, _ string) (*Business, error) {
	return nil, errNotImplemented("GetByID")
}

func (s *serviceImpl) ListForUser(_ context.Context, _ string) ([]*Business, error) {
	return nil, errNotImplemented("ListForUser")
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

// ErrNotFound is returned when a business does not exist.
var ErrNotFound = errors.New("business not found")

// ErrSlugTaken is returned when the slug is already registered.
var ErrSlugTaken = errors.New("business slug already taken")

// ErrNotMember is returned when a user tries to access a business they don't belong to.
var ErrNotMember = errors.New("user is not a member of this business")

func errNotImplemented(method string) error {
	return fmt.Errorf("business: %s: not yet implemented", method)
}
