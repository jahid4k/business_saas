// backend/internal/security/service.go
package security

import (
	"context"
	"errors"
	"fmt"
)

type Service interface {
	ListSessions(ctx context.Context, organizationID string, limit int) ([]*SessionView, error)
	RevokeSession(ctx context.Context, organizationID, sessionRef string) error
	ListLoginEvents(ctx context.Context, organizationID string, limit int) ([]*LoginEventView, error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

func (s *serviceImpl) ListSessions(ctx context.Context, organizationID string, limit int) ([]*SessionView, error) {
	out, err := s.repo.ListOrganizationSessions(ctx, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("security: ListSessions: %w", err)
	}
	if out == nil {
		out = []*SessionView{}
	}
	return out, nil
}

func (s *serviceImpl) RevokeSession(ctx context.Context, organizationID, sessionRef string) error {
	if err := s.repo.RevokeOrganizationSession(ctx, organizationID, sessionRef); err != nil {
		return fmt.Errorf("security: RevokeSession: %w", err)
	}
	return nil
}

func (s *serviceImpl) ListLoginEvents(ctx context.Context, organizationID string, limit int) ([]*LoginEventView, error) {
	out, err := s.repo.ListOrganizationLoginEvents(ctx, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("security: ListLoginEvents: %w", err)
	}
	if out == nil {
		out = []*LoginEventView{}
	}
	return out, nil
}

var ErrSessionNotFound = errors.New("session not found")
