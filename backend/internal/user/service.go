// backend/internal/user/service.go
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Service defines the user business logic interface.
type Service interface {
	GetByID(ctx context.Context, userID string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) error
	UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

func (s *serviceImpl) GetByID(ctx context.Context, userID string) (*User, error) {
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user: GetByID: %w", err)
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s *serviceImpl) GetByEmail(ctx context.Context, email string) (*User, error) {
	u, err := s.repo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, fmt.Errorf("user: GetByEmail: %w", err)
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s *serviceImpl) Create(ctx context.Context, u *User) error { return s.repo.Create(ctx, u) }

func (s *serviceImpl) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error) {
	u, err := s.repo.UpdateSettings(ctx, userID, req)
	if err != nil {
		return nil, fmt.Errorf("user: UpdateProfile: %w", err)
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
}

var ErrNotFound = errors.New("user not found")
var ErrEmailTaken = errors.New("email already registered")
