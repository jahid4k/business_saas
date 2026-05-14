package user

import (
	"context"
	"errors"
	"fmt"
)

// Service defines the user business logic interface.
type Service interface {
	GetByID(ctx context.Context, userID string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) error
	UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error)
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new user service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) GetByID(_ context.Context, _ string) (*User, error) {
	return nil, errNotImplemented("GetByID")
}

func (s *serviceImpl) GetByEmail(_ context.Context, _ string) (*User, error) {
	return nil, errNotImplemented("GetByEmail")
}

func (s *serviceImpl) Create(_ context.Context, _ *User) error {
	return errNotImplemented("Create")
}

func (s *serviceImpl) UpdateProfile(_ context.Context, _ string, _ UpdateProfileRequest) (*User, error) {
	return nil, errNotImplemented("UpdateProfile")
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

// ErrNotFound is returned when a user does not exist.
var ErrNotFound = errors.New("user not found")

// ErrEmailTaken is returned when the email is already registered.
var ErrEmailTaken = errors.New("email already registered")

func errNotImplemented(method string) error {
	return fmt.Errorf("user: %s: not yet implemented", method)
}
