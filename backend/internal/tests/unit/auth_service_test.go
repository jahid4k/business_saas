// backend/internal/tests/unit/auth_service_test.go
package unit

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/password"
)

// ── Stubs ──────────────────────────────────────────────────────────────────

type stubUserRepo struct {
	users map[string]*user.User
}

func newStubUserRepo() *stubUserRepo { return &stubUserRepo{users: map[string]*user.User{}} }

func (r *stubUserRepo) FindByID(_ context.Context, id string) (*user.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}
func (r *stubUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}
func (r *stubUserRepo) Create(_ context.Context, u *user.User) error {
	u.ID = "usr_" + u.Email
	u.PublicID = u.ID
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	r.users[u.ID] = u
	return nil
}
func (r *stubUserRepo) Update(_ context.Context, u *user.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *stubUserRepo) UpdateSettings(_ context.Context, _ string, _ user.UpdateProfileRequest) (*user.User, error) {
	return nil, nil
}
func (r *stubUserRepo) UpdateAvatar(_ context.Context, _, _ string) (*user.User, error) {
	return nil, nil
}
func (r *stubUserRepo) RecordFailedLogin(_ context.Context, _ string) error  { return nil }
func (r *stubUserRepo) RecordSuccessfulLogin(_ context.Context, _ string) error { return nil }

type stubAuthRepo struct {
	sessions map[string]*auth.Session
}

func newStubAuthRepo() *stubAuthRepo { return &stubAuthRepo{sessions: map[string]*auth.Session{}} }

func (r *stubAuthRepo) CreateSession(_ context.Context, s *auth.Session) error {
	s.ID = "sess_1"
	s.PublicID = "sess_pub_1"
	s.CreatedAt = time.Now()
	r.sessions[s.TokenHash] = s
	return nil
}
func (r *stubAuthRepo) GetSessionByTokenHash(_ context.Context, hash string) (*auth.Session, error) {
	s, ok := r.sessions[hash]
	if !ok {
		return nil, auth.ErrSessionNotFound
	}
	return s, nil
}
func (r *stubAuthRepo) RotateSession(_ context.Context, old, newHash string, exp time.Time) (*auth.Session, error) {
	s, ok := r.sessions[old]
	if !ok {
		return nil, auth.ErrSessionNotFound
	}
	delete(r.sessions, old)
	s.TokenHash = newHash
	s.ExpiresAt = exp
	r.sessions[newHash] = s
	return s, nil
}
func (r *stubAuthRepo) RevokeSession(_ context.Context, _ string) error         { return nil }
func (r *stubAuthRepo) RevokeAllUserSessions(_ context.Context, _ string) error { return nil }
func (r *stubAuthRepo) FindAuthAccount(_ context.Context, _, _ string) (*auth.AuthAccount, error) {
	return nil, nil
}
func (r *stubAuthRepo) CreateAuthAccount(_ context.Context, _ *auth.AuthAccount, _, _, _ string) error {
	return nil
}
func (r *stubAuthRepo) UpdateAuthAccount(_ context.Context, _ *auth.AuthAccount, _, _, _ string) error {
	return nil
}
func (r *stubAuthRepo) CreateLoginEvent(_ context.Context, _ auth.LoginEvent) error { return nil }

// ── Helpers ────────────────────────────────────────────────────────────────

func newTestAuthService(userRepo user.Repository, authRepo auth.Repository) auth.Service {
	mgr := jwtpkg.NewManager("test-secret-32-bytes-padding-here", 15*time.Minute)
	cfg := config.JWTConfig{
		Secret:          "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
	return auth.NewService(authRepo, userRepo, mgr, cfg, audit.NewService(audit.NewNoopRepository()))
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestSignup_Success(t *testing.T) {
	svc := newTestAuthService(newStubUserRepo(), newStubAuthRepo())

	u, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:    "alice@example.com",
		Password: "securePassword1",
	})
	if err != nil {
		t.Fatalf("Signup() unexpected error: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", u.Email)
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	repo := newStubUserRepo()
	svc := newTestAuthService(repo, newStubAuthRepo())

	_, _ = svc.Signup(context.Background(), auth.SignupRequest{Email: "bob@example.com", Password: "pass12345"})
	_, err := svc.Signup(context.Background(), auth.SignupRequest{Email: "bob@example.com", Password: "pass12345"})
	if err == nil {
		t.Fatal("expected duplicate email error, got nil")
	}
}

func TestLogin_Success(t *testing.T) {
	repo := newStubUserRepo()
	hash, _ := password.Hash("mypassword")
	repo.users["usr_carol@example.com"] = &user.User{
		ID: "usr_carol@example.com", Email: "carol@example.com",
		PasswordHash: hash, Status: user.StatusActive,
	}
	svc := newTestAuthService(repo, newStubAuthRepo())

	pair, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "carol@example.com", Password: "mypassword",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if pair.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	repo := newStubUserRepo()
	hash, _ := password.Hash("correctpassword")
	repo.users["usr_dave@example.com"] = &user.User{
		ID: "usr_dave@example.com", Email: "dave@example.com",
		PasswordHash: hash, Status: user.StatusActive,
	}
	svc := newTestAuthService(repo, newStubAuthRepo())

	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "dave@example.com", Password: "wrongpassword",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := newTestAuthService(newStubUserRepo(), newStubAuthRepo())
	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "nobody@example.com", Password: "irrelevant",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Fatal("expected error for unknown email, got nil")
	}
}

func TestLogin_DisabledAccount(t *testing.T) {
	repo := newStubUserRepo()
	hash, _ := password.Hash("pass")
	repo.users["usr_eve@example.com"] = &user.User{
		ID: "usr_eve@example.com", Email: "eve@example.com",
		PasswordHash: hash, Status: user.StatusSuspended,
	}
	svc := newTestAuthService(repo, newStubAuthRepo())

	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "eve@example.com", Password: "pass",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Fatal("expected error for disabled account, got nil")
	}
}

func TestLogout_Success(t *testing.T) {
	repo := newStubUserRepo()
	authRepo := newStubAuthRepo()
	hash, _ := password.Hash("pw")
	repo.users["usr_frank@example.com"] = &user.User{
		ID: "usr_frank@example.com", Email: "frank@example.com",
		PasswordHash: hash, Status: user.StatusActive,
	}
	svc := newTestAuthService(repo, authRepo)

	pair, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "frank@example.com", Password: "pw",
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("Login() failed: %v", err)
	}

	if err := svc.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("Logout() failed: %v", err)
	}
}

func TestMe_ReturnsUser(t *testing.T) {
	repo := newStubUserRepo()
	repo.users["usr_1"] = &user.User{
		ID: "usr_1", Email: "grace@example.com", Status: user.StatusActive,
	}
	svc := newTestAuthService(repo, newStubAuthRepo())

	u, err := svc.Me(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("Me() unexpected error: %v", err)
	}
	if u.Email != "grace@example.com" {
		t.Errorf("expected grace@example.com, got %s", u.Email)
	}
}
