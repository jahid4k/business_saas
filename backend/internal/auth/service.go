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

func NewService(repo Repository, userRepo user.Repository, jwtManager *jwtpkg.Manager, jwtCfg config.JWTConfig) Service {
	return &serviceImpl{repo: repo, userRepo: userRepo, jwtManager: jwtManager, jwtCfg: jwtCfg}
}

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

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(req.FirstName + " " + req.LastName)
	}
	if displayName == "" {
		displayName = req.Email
	}

	u := &user.User{
		Email: req.Email, PasswordHash: hash,
		FirstName: req.FirstName, LastName: req.LastName,
		DisplayName: displayName, FullName: displayName,
		EmailVerified: false, Status: user.StatusActive,
	}
	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("auth: signup: create user: %w", err)
	}
	slog.Info("auth: new user registered", slog.String("user_id", u.ID))
	return u.ToSafe(), nil
}

func (s *serviceImpl) Login(ctx context.Context, req LoginRequest, ip, userAgent string) (*TokenPair, error) {
	req.Email = normaliseEmail(req.Email)
	u, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("auth: login: %w", err)
	}
	if u == nil {
		return nil, ErrInvalidCredentials
	}
	if u.IsLocked() {
		return nil, ErrAccountLocked
	}
	if u.Status != user.StatusActive {
		return nil, ErrAccountDisabled
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return nil, ErrPasswordLoginDisabled
	}

	if err := password.Verify(req.Password, u.PasswordHash); err != nil {
		if errors.Is(err, password.ErrMismatch) {
			_ = s.userRepo.RecordFailedLogin(ctx, u.ID)
			slog.Warn("auth: bad password attempt", slog.String("user_id", u.ID), slog.String("ip", ip))
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: login: verify password: %w", err)
	}
	_ = s.userRepo.RecordSuccessfulLogin(ctx, u.ID)

	accessToken, err := s.jwtManager.IssueAccessToken(u.ID, u.Email, "", "")
	if err != nil {
		return nil, fmt.Errorf("auth: login: issue access token: %w", err)
	}
	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("auth: login: generate refresh token: %w", err)
	}

	session := &Session{UserID: u.ID, TokenHash: tokenHash, UserAgent: userAgent, IPAddress: ip, ExpiresAt: time.Now().Add(s.jwtCfg.RefreshTokenTTL)}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("auth: login: create session: %w", err)
	}
	slog.Info("auth: user logged in", slog.String("user_id", u.ID), slog.String("ip", ip))

	return &TokenPair{AccessToken: accessToken, RefreshToken: rawToken, ExpiresIn: int64(s.jwtCfg.AccessTokenTTL.Seconds())}, nil
}

func (s *serviceImpl) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	oldHash := token.Hash(refreshToken)
	rawToken, newHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: generate token: %w", err)
	}

	newSession, err := s.repo.RotateSession(ctx, oldHash, newHash, time.Now().Add(s.jwtCfg.RefreshTokenTTL))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	u, err := s.userRepo.FindByID(ctx, newSession.UserID)
	if err != nil || u == nil {
		return nil, fmt.Errorf("auth: refresh: user not found")
	}
	if u.Status != user.StatusActive {
		return nil, ErrAccountDisabled
	}

	orgID := ""
	if newSession.OrgID != nil {
		orgID = *newSession.OrgID
	}
	accessToken, err := s.jwtManager.IssueAccessToken(u.ID, u.Email, orgID, "")
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: issue access token: %w", err)
	}
	return &TokenPair{AccessToken: accessToken, RefreshToken: rawToken, ExpiresIn: int64(s.jwtCfg.AccessTokenTTL.Seconds())}, nil
}

func (s *serviceImpl) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}
	session, err := s.repo.GetSessionByTokenHash(ctx, token.Hash(refreshToken))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return fmt.Errorf("auth: logout: %w", err)
	}
	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return fmt.Errorf("auth: logout: %w", err)
	}
	return nil
}

func (s *serviceImpl) LogoutAll(ctx context.Context, userID string) error {
	if err := s.repo.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("auth: logout-all: %w", err)
	}
	return nil
}

func (s *serviceImpl) Me(ctx context.Context, userID string) (*user.SafeUser, error) {
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: me: %w", err)
	}
	if u == nil {
		return nil, ErrInvalidCredentials
	}
	return u.ToSafe(), nil
}

func (s *serviceImpl) RequestPasswordReset(_ context.Context, _ string) error { return nil }
func (s *serviceImpl) ConfirmPasswordReset(_ context.Context, _, _ string) error {
	return fmt.Errorf("auth: password reset: not yet implemented")
}

func normaliseEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
