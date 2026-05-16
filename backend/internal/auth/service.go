// backend/internal/auth/service.go
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/password"
	"github.com/mridha/businesssaas/pkg/token"
)

// Service defines the auth business logic interface.
type Service interface {
	Signup(ctx context.Context, req SignupRequest) (*user.SafeUser, error)
	Login(ctx context.Context, req LoginRequest, ip, userAgent string) (*TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID string) error
	Me(ctx context.Context, userID string) (*user.SafeUser, error)
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, tok, newPassword string) error
}

type serviceImpl struct {
	repo       Repository
	userRepo   user.Repository
	jwtManager *jwtpkg.Manager
	jwtCfg     config.JWTConfig
}

// NewService creates a fully wired auth service.
func NewService(
	repo Repository,
	userRepo user.Repository,
	jwtManager *jwtpkg.Manager,
	jwtCfg config.JWTConfig,
) Service {
	return &serviceImpl{
		repo:       repo,
		userRepo:   userRepo,
		jwtManager: jwtManager,
		jwtCfg:     jwtCfg,
	}
}

// ----------------------------------------------------------
// Signup
// ----------------------------------------------------------

func (s *serviceImpl) Signup(ctx context.Context, req SignupRequest) (*user.SafeUser, error) {
	req.Email = normaliseEmail(req.Email)

	existing, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("auth: signup: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("auth: signup: hash password: %w", err)
	}

	u := &user.User{
		Email:        req.Email,
		PasswordHash: hash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		IsVerified:   false,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("auth: signup: create user: %w", err)
	}

	slog.Info("auth: new user registered", slog.String("user_id", u.ID))
	safeUser := u.ToSafe()
	return safeUser, nil
}

// ----------------------------------------------------------
// Login
// ----------------------------------------------------------

func (s *serviceImpl) Login(ctx context.Context, req LoginRequest, ip, userAgent string) (*TokenPair, error) {
	req.Email = normaliseEmail(req.Email)

	u, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("auth: login: %w", err)
	}
	if u == nil {
		// User not found — return generic error. NEVER say "email not found".
		return nil, ErrInvalidCredentials
	}

	if u.IsLocked() {
		slog.Warn("auth: login on locked account",
			slog.String("user_id", u.ID),
			slog.String("ip", ip),
		)
		return nil, ErrAccountLocked
	}

	if err := password.Verify(req.Password, u.PasswordHash); err != nil {
		if errors.Is(err, password.ErrMismatch) {
			slog.Warn("auth: bad password attempt",
				slog.String("user_id", u.ID),
				slog.String("ip", ip),
			)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: login: verify password: %w", err)
	}

	// Issue JWT — no business context at bare login
	accessToken, err := s.jwtManager.IssueAccessToken(u.ID, u.Email, "", "")
	if err != nil {
		return nil, fmt.Errorf("auth: login: issue access token: %w", err)
	}

	// Generate opaque refresh token and store only the hash
	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("auth: login: generate refresh token: %w", err)
	}

	session := &Session{
		UserID:    u.ID,
		TokenHash: tokenHash,
		UserAgent: userAgent,
		IPAddress: ip,
		ExpiresAt: time.Now().Add(s.jwtCfg.RefreshTokenTTL),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("auth: login: create session: %w", err)
	}

	slog.Info("auth: user logged in",
		slog.String("user_id", u.ID),
		slog.String("ip", ip),
	)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawToken,
		ExpiresIn:    int64(s.jwtCfg.AccessTokenTTL.Seconds()),
	}, nil
}

// ----------------------------------------------------------
// Refresh — rotates the token on every call
// ----------------------------------------------------------

func (s *serviceImpl) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	tokenHash := token.Hash(refreshToken)

	session, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: refresh: %w", err)
	}

	if session.IsRevoked() {
		slog.Warn("auth: revoked token reuse",
			slog.String("session_id", session.ID),
			slog.String("user_id", session.UserID),
		)
		return nil, ErrSessionRevoked
	}
	if session.IsExpired() {
		return nil, ErrSessionExpired
	}

	u, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil || u == nil {
		return nil, fmt.Errorf("auth: refresh: user not found")
	}

	// Revoke old session (rotation — every token is single-use)
	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("auth: refresh: revoke old session: %w", err)
	}

	accessToken, err := s.jwtManager.IssueAccessToken(u.ID, u.Email, "", "")
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: issue access token: %w", err)
	}

	rawToken, newHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: generate token: %w", err)
	}

	newSession := &Session{
		UserID:    u.ID,
		TokenHash: newHash,
		UserAgent: session.UserAgent,
		IPAddress: session.IPAddress,
		ExpiresAt: time.Now().Add(s.jwtCfg.RefreshTokenTTL),
	}
	if err := s.repo.CreateSession(ctx, newSession); err != nil {
		return nil, fmt.Errorf("auth: refresh: create session: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawToken,
		ExpiresIn:    int64(s.jwtCfg.AccessTokenTTL.Seconds()),
	}, nil
}

// ----------------------------------------------------------
// Logout
// ----------------------------------------------------------

func (s *serviceImpl) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil // nothing to revoke
	}
	tokenHash := token.Hash(refreshToken)

	session, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil // already gone — idempotent
		}
		return fmt.Errorf("auth: logout: %w", err)
	}

	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return fmt.Errorf("auth: logout: %w", err)
	}

	slog.Info("auth: session revoked", slog.String("user_id", session.UserID))
	return nil
}

// ----------------------------------------------------------
// LogoutAll
// ----------------------------------------------------------

func (s *serviceImpl) LogoutAll(ctx context.Context, userID string) error {
	if err := s.repo.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("auth: logout-all: %w", err)
	}
	slog.Info("auth: all sessions revoked", slog.String("user_id", userID))
	return nil
}

// ----------------------------------------------------------
// Me
// ----------------------------------------------------------

func (s *serviceImpl) Me(ctx context.Context, userID string) (*user.SafeUser, error) {
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: me: %w", err)
	}
	if u == nil {
		return nil, ErrInvalidCredentials
	}
	safeUser := u.ToSafe()
	return safeUser, nil
}

// ----------------------------------------------------------
// Password reset (Phase 2)
// ----------------------------------------------------------

func (s *serviceImpl) RequestPasswordReset(_ context.Context, _ string) error {
	// Always return nil — never reveal whether the email exists
	return nil
}

func (s *serviceImpl) ConfirmPasswordReset(_ context.Context, _, _ string) error {
	return fmt.Errorf("auth: password reset: not yet implemented")
}

// ----------------------------------------------------------
// Helpers
// ----------------------------------------------------------

func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
