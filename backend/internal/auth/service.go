// backend/internal/auth/service.go
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/password"
	"github.com/mridha/businesssaas/pkg/token"
)

type Service interface {
	Signup(ctx context.Context, req SignupRequest) (*user.SafeUser, error)
	Login(ctx context.Context, req LoginRequest, ip, userAgent string) (*TokenPair, error)
	OAuthSync(ctx context.Context, req OAuthSyncRequest, ip, userAgent string) (*OAuthSyncResponse, error)
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
	audit      audit.Service
}

// NewService creates the auth service.
// FIX: now accepts audit.Service so security events (login, signup, logout) are written
// to audit_logs rather than being silently discarded.
func NewService(repo Repository, userRepo user.Repository, jwtManager *jwtpkg.Manager, jwtCfg config.JWTConfig, auditSvc audit.Service) Service {
	return &serviceImpl{repo: repo, userRepo: userRepo, jwtManager: jwtManager, jwtCfg: jwtCfg, audit: auditSvc}
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
	s.audit.Log(ctx, audit.EventSignup, u.ID, "", "", "", map[string]string{"email": u.Email})
	return u.ToSafe(), nil
}

func (s *serviceImpl) Login(ctx context.Context, req LoginRequest, ip, userAgent string) (*TokenPair, error) {
	req.Email = normaliseEmail(req.Email)
	u, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("auth: login: %w", err)
	}
	if u == nil {
		_ = s.repo.CreateLoginEvent(ctx, LoginEvent{Email: req.Email, Provider: "credentials", Status: "failure", FailureReason: "user_not_found", IPAddress: ip, UserAgent: userAgent})
		s.audit.Log(ctx, audit.EventLoginFailed, "", "", ip, userAgent, map[string]string{"email": req.Email, "reason": "user_not_found"})
		return nil, ErrInvalidCredentials
	}
	if u.IsLocked() {
		uid := u.ID
		_ = s.repo.CreateLoginEvent(ctx, LoginEvent{UserID: &uid, Email: req.Email, Provider: "credentials", Status: "failure", FailureReason: "account_locked", IPAddress: ip, UserAgent: userAgent})
		s.audit.Log(ctx, audit.EventLoginFailed, u.ID, "", ip, userAgent, map[string]string{"reason": "account_locked"})
		return nil, ErrAccountLocked
	}
	if u.Status != user.StatusActive {
		uid := u.ID
		_ = s.repo.CreateLoginEvent(ctx, LoginEvent{UserID: &uid, Email: req.Email, Provider: "credentials", Status: "failure", FailureReason: "account_disabled", IPAddress: ip, UserAgent: userAgent})
		s.audit.Log(ctx, audit.EventLoginFailed, u.ID, "", ip, userAgent, map[string]string{"reason": "account_disabled"})
		return nil, ErrAccountDisabled
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		uid := u.ID
		_ = s.repo.CreateLoginEvent(ctx, LoginEvent{UserID: &uid, Email: req.Email, Provider: "credentials", Status: "failure", FailureReason: "password_login_disabled", IPAddress: ip, UserAgent: userAgent})
		return nil, ErrPasswordLoginDisabled
	}
	if err := password.Verify(req.Password, u.PasswordHash); err != nil {
		if errors.Is(err, password.ErrMismatch) {
			_ = s.userRepo.RecordFailedLogin(ctx, u.ID)
			uid := u.ID
			_ = s.repo.CreateLoginEvent(ctx, LoginEvent{UserID: &uid, Email: req.Email, Provider: "credentials", Status: "failure", FailureReason: "invalid_password", IPAddress: ip, UserAgent: userAgent})
			s.audit.Log(ctx, audit.EventLoginFailed, u.ID, "", ip, userAgent, map[string]string{"reason": "invalid_password"})
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: login: verify password: %w", err)
	}

	// FIX: transparently rehash passwords stored with an old (lower) bcrypt cost factor
	if password.NeedsRehash(u.PasswordHash) {
		if newHash, hashErr := password.Hash(req.Password); hashErr == nil {
			u.PasswordHash = newHash
			_ = s.userRepo.Update(ctx, u)
		}
	}

	_ = s.userRepo.RecordSuccessfulLogin(ctx, u.ID)
	pair, err := s.issueTokenPair(ctx, u.ID, u.Email, ip, userAgent, nil)
	if err != nil {
		return nil, err
	}
	uid := u.ID
	_ = s.repo.CreateLoginEvent(ctx, LoginEvent{UserID: &uid, Email: req.Email, Provider: "credentials", Status: "success", IPAddress: ip, UserAgent: userAgent})
	s.audit.Log(ctx, audit.EventLogin, u.ID, "", ip, userAgent, map[string]string{"provider": "credentials"})
	return pair, nil
}

func (s *serviceImpl) OAuthSync(ctx context.Context, req OAuthSyncRequest, ip, userAgent string) (*OAuthSyncResponse, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	providerAccountID := strings.TrimSpace(req.ProviderAccountID)
	if provider == "" {
		return nil, ErrOAuthProviderRequired
	}
	if providerAccountID == "" {
		return nil, ErrOAuthAccountIDRequired
	}
	email := normaliseEmail(req.Email)
	account, err := s.repo.FindAuthAccount(ctx, provider, providerAccountID)
	if err != nil {
		return nil, fmt.Errorf("auth: OAuthSync: find account: %w", err)
	}
	var u *user.User
	if account != nil {
		u, err = s.userRepo.FindByID(ctx, account.UserID)
		if err != nil {
			return nil, fmt.Errorf("auth: OAuthSync: find linked user: %w", err)
		}
		if u == nil {
			return nil, ErrInvalidCredentials
		}
		account.TokenType = strings.TrimSpace(req.TokenType)
		account.Scope = strings.TrimSpace(req.Scope)
		account.ExpiresAt = req.ExpiresAt
		_ = s.repo.UpdateAuthAccount(ctx, account, req.AccessToken, req.RefreshToken, req.IDToken)
	} else {
		if email == "" {
			return nil, ErrOAuthEmailRequired
		}
		u, err = s.userRepo.FindByEmail(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("auth: OAuthSync: find email: %w", err)
		}
		if u == nil {
			displayName := strings.TrimSpace(req.DisplayName)
			if displayName == "" {
				displayName = strings.TrimSpace(req.FirstName + " " + req.LastName)
			}
			if displayName == "" {
				displayName = email
			}
			u = &user.User{
				Email: email, FirstName: strings.TrimSpace(req.FirstName),
				LastName: strings.TrimSpace(req.LastName), DisplayName: displayName,
				FullName: displayName, PhotoURL: strings.TrimSpace(req.PhotoURL),
				EmailVerified: req.EmailVerified, Status: user.StatusActive,
			}
			if req.EmailVerified {
				now := time.Now()
				u.EmailVerifiedAt = &now
			}
			if err := s.userRepo.Create(ctx, u); err != nil {
				return nil, fmt.Errorf("auth: OAuthSync: create user: %w", err)
			}
			s.audit.Log(ctx, audit.EventSignup, u.ID, "", ip, userAgent, map[string]string{"provider": provider})
		} else {
			updated := false
			if req.PhotoURL != "" && u.PhotoURL == "" {
				u.PhotoURL = strings.TrimSpace(req.PhotoURL)
				updated = true
			}
			if req.EmailVerified && !u.EmailVerified {
				now := time.Now()
				u.EmailVerified = true
				u.EmailVerifiedAt = &now
				updated = true
			}
			if updated {
				_ = s.userRepo.Update(ctx, u)
			}
		}
		providerType := strings.TrimSpace(req.ProviderType)
		if providerType == "" {
			providerType = "oauth"
		}
		account = &AuthAccount{
			UserID: u.ID, Provider: provider, ProviderAccountID: providerAccountID,
			ProviderType: providerType, TokenType: strings.TrimSpace(req.TokenType),
			Scope: strings.TrimSpace(req.Scope), ExpiresAt: req.ExpiresAt,
		}
		if err := s.repo.CreateAuthAccount(ctx, account, req.AccessToken, req.RefreshToken, req.IDToken); err != nil {
			return nil, fmt.Errorf("auth: OAuthSync: create account: %w", err)
		}
	}
	if u.Status != user.StatusActive {
		return nil, ErrAccountDisabled
	}
	// FIX: also check for locked accounts in the OAuth path
	if u.IsLocked() {
		return nil, ErrAccountLocked
	}
	_ = s.userRepo.RecordSuccessfulLogin(ctx, u.ID)
	issueTokens := true
	if req.IssueTokens != nil {
		issueTokens = *req.IssueTokens
	}
	var pair *TokenPair
	if issueTokens {
		pair, err = s.issueTokenPair(ctx, u.ID, u.Email, ip, userAgent, nil)
		if err != nil {
			return nil, err
		}
	}
	uid := u.ID
	_ = s.repo.CreateLoginEvent(ctx, LoginEvent{UserID: &uid, Email: u.Email, Provider: provider, Status: "success", IPAddress: ip, UserAgent: userAgent})
	s.audit.Log(ctx, audit.EventLogin, u.ID, "", ip, userAgent, map[string]string{"provider": provider})
	return &OAuthSyncResponse{User: u.ToSafe(), Account: account, Tokens: pair}, nil
}

func (s *serviceImpl) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	oldHash := token.Hash(refreshToken)
	rawToken, newHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: generate token: %w", err)
	}
	newSession, err := s.repo.RotateSession(ctx, oldHash, newHash, time.Now().Add(s.jwtCfg.RefreshTokenTTL))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionRevoked) || errors.Is(err, ErrSessionExpired) {
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
	s.audit.Log(ctx, audit.EventLogout, session.UserID, "", "", "", nil)
	return nil
}

func (s *serviceImpl) LogoutAll(ctx context.Context, userID string) error {
	if err := s.repo.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("auth: logout-all: %w", err)
	}
	s.audit.Log(ctx, audit.EventLogoutAll, userID, "", "", "", nil)
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

var ErrNotImplemented = errors.New("auth: feature not yet implemented")

func (s *serviceImpl) ConfirmPasswordReset(_ context.Context, _, _ string) error {
	return ErrNotImplemented
}

func (s *serviceImpl) issueTokenPair(ctx context.Context, userID, email, ip, userAgent string, orgID *string) (*TokenPair, error) {
	activeOrgID := ""
	if orgID != nil {
		activeOrgID = *orgID
	}
	accessToken, err := s.jwtManager.IssueAccessToken(userID, email, activeOrgID, "")
	if err != nil {
		return nil, fmt.Errorf("auth: issue access token: %w", err)
	}
	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("auth: generate refresh token: %w", err)
	}
	session := &Session{
		UserID: userID, OrgID: orgID, TokenHash: tokenHash,
		UserAgent: userAgent, IPAddress: ip,
		ExpiresAt: time.Now().Add(s.jwtCfg.RefreshTokenTTL),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("auth: create session: %w", err)
	}
	return &TokenPair{AccessToken: accessToken, RefreshToken: rawToken, ExpiresIn: int64(s.jwtCfg.AccessTokenTTL.Seconds())}, nil
}

func normaliseEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
