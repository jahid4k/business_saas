package auth

import "context"

// Service defines the auth business logic interface.
// The handler depends on this interface, not the concrete implementation.
// This makes the handler testable without a real DB.
type Service interface {
	Signup(ctx context.Context, req SignupRequest) error
	Login(ctx context.Context, req LoginRequest, ip, userAgent string) (*TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
}

// serviceImpl is the concrete implementation of Service.
// It is unexported — only accessible via NewService.
type serviceImpl struct {
	repo Repository
	// TODO (Phase 1-B): add jwtPkg, passwordPkg, auditService, config fields
}

// NewService creates a new auth service with the given repository.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

// TODO (Phase 1-B): implement all methods below.

func (s *serviceImpl) Signup(_ context.Context, _ SignupRequest) error {
	return errNotImplemented("Signup")
}

func (s *serviceImpl) Login(_ context.Context, _ LoginRequest, _, _ string) (*TokenPair, error) {
	return nil, errNotImplemented("Login")
}

func (s *serviceImpl) Refresh(_ context.Context, _ string) (*TokenPair, error) {
	return nil, errNotImplemented("Refresh")
}

func (s *serviceImpl) Logout(_ context.Context, _ string) error {
	return errNotImplemented("Logout")
}

func (s *serviceImpl) LogoutAll(_ context.Context, _ string) error {
	return errNotImplemented("LogoutAll")
}

func (s *serviceImpl) RequestPasswordReset(_ context.Context, _ string) error {
	return errNotImplemented("RequestPasswordReset")
}

func (s *serviceImpl) ConfirmPasswordReset(_ context.Context, _, _ string) error {
	return errNotImplemented("ConfirmPasswordReset")
}
