// backend/internal/auth/service_test.go
package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/password"
	"github.com/mridha/businesssaas/pkg/token"
)

// ----------------------------------------------------------
// In-memory mock: auth.Repository
// ----------------------------------------------------------

type mockAuthRepo struct {
	sessions map[string]*auth.Session
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{sessions: make(map[string]*auth.Session)}
}

func (m *mockAuthRepo) CreateSession(_ context.Context, s *auth.Session) error {
	s.ID = "sess-" + s.TokenHash[:12]
	s.CreatedAt = time.Now()
	// Clone the session to avoid pointer aliasing
	clone := *s
	m.sessions[s.TokenHash] = &clone
	return nil
}

func (m *mockAuthRepo) GetSessionByTokenHash(_ context.Context, hash string) (*auth.Session, error) {
	s, ok := m.sessions[hash]
	if !ok {
		return nil, auth.ErrSessionNotFound
	}
	clone := *s
	return &clone, nil
}

func (m *mockAuthRepo) RevokeSession(_ context.Context, sessionID string) error {
	for _, s := range m.sessions {
		if s.ID == sessionID {
			now := time.Now()
			s.RevokedAt = &now
			return nil
		}
	}
	return nil
}

func (m *mockAuthRepo) RevokeAllUserSessions(_ context.Context, userID string) error {
	now := time.Now()
	for _, s := range m.sessions {
		if s.UserID == userID {
			s.RevokedAt = &now
		}
	}
	return nil
}

// ----------------------------------------------------------
// In-memory mock: user.Repository
// ----------------------------------------------------------

type mockUserRepo struct {
	byEmail map[string]*user.User
	byID    map[string]*user.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		byEmail: make(map[string]*user.User),
		byID:    make(map[string]*user.User),
	}
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	return m.byEmail[email], nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id string) (*user.User, error) {
	return m.byID[id], nil
}

func (m *mockUserRepo) Create(_ context.Context, u *user.User) error {
	u.ID = "user-" + u.Email
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	clone := *u
	m.byEmail[u.Email] = &clone
	m.byID[u.ID] = &clone
	return nil
}

func (m *mockUserRepo) Update(_ context.Context, u *user.User) error {
	clone := *u
	m.byEmail[u.Email] = &clone
	m.byID[u.ID] = &clone
	return nil
}

// ----------------------------------------------------------
// Test helper: build a wired service
// ----------------------------------------------------------

func newTestService(authRepo auth.Repository, userRepo user.Repository) auth.Service {
	cfg := config.JWTConfig{
		Secret:          "test-secret-minimum-32-chars-long!",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
	mgr := jwtpkg.NewManager(cfg.Secret, cfg.AccessTokenTTL)
	return auth.NewService(authRepo, userRepo, mgr, cfg)
}

// ----------------------------------------------------------
// Signup tests
// ----------------------------------------------------------

func TestSignup_Success(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	u, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:     "jane@example.com",
		Password:  "password123",
		FirstName: "Jane",
		LastName:  "Doe",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if u == nil {
		t.Fatal("expected user response, got nil")
	}
	if u.Email != "jane@example.com" {
		t.Errorf("email: want jane@example.com, got %s", u.Email)
	}
	if u.ID == "" {
		t.Error("expected non-empty user ID")
	}
}

func TestSignup_EmailNormalised(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	u, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:     "  JANE@EXAMPLE.COM  ",
		Password:  "password123",
		FirstName: "Jane",
		LastName:  "Doe",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Email != "jane@example.com" {
		t.Errorf("expected normalised email, got %s", u.Email)
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())
	req := auth.SignupRequest{
		Email:     "dup@example.com",
		Password:  "password123",
		FirstName: "A", LastName: "B",
	}

	if _, err := svc.Signup(context.Background(), req); err != nil {
		t.Fatalf("first signup: %v", err)
	}
	_, err := svc.Signup(context.Background(), req)
	if !errors.Is(err, auth.ErrEmailAlreadyExists) {
		t.Errorf("want ErrEmailAlreadyExists, got: %v", err)
	}
}

// ----------------------------------------------------------
// Login tests
// ----------------------------------------------------------

func setupUserAndLogin(t *testing.T, svc auth.Service, email, pw string) (*auth.TokenPair, error) {
	t.Helper()
	if _, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:     email,
		Password:  pw,
		FirstName: "Test", LastName: "User",
	}); err != nil {
		t.Fatalf("signup failed: %v", err)
	}
	return svc.Login(context.Background(), auth.LoginRequest{Email: email, Password: pw}, "127.0.0.1", "test-agent")
}

func TestLogin_Success(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	pair, err := setupUserAndLogin(t, svc, "login@example.com", "password123")
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if pair.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if pair.ExpiresIn <= 0 {
		t.Error("expected positive expires_in")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())
	_, _ = svc.Signup(context.Background(), auth.SignupRequest{
		Email: "bad@example.com", Password: "correct",
		FirstName: "A", LastName: "B",
	})

	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "bad@example.com", Password: "wrong",
	}, "127.0.0.1", "test-agent")

	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("want ErrInvalidCredentials, got: %v", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "nobody@example.com", Password: "anything",
	}, "127.0.0.1", "test-agent")

	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("want ErrInvalidCredentials, got: %v", err)
	}
}

func TestLogin_CaseInsensitiveEmail(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())
	_, _ = svc.Signup(context.Background(), auth.SignupRequest{
		Email: "case@example.com", Password: "password123",
		FirstName: "A", LastName: "B",
	})

	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "CASE@EXAMPLE.COM", Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Errorf("case-insensitive login should succeed, got: %v", err)
	}
}

// ----------------------------------------------------------
// Refresh tests
// ----------------------------------------------------------

func TestRefresh_Success(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	pair, _ := setupUserAndLogin(t, svc, "refresh@example.com", "password123")

	newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh error: %v", err)
	}
	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Error("expected full token pair after refresh")
	}
}

func TestRefresh_Rotates(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())
	pair, _ := setupUserAndLogin(t, svc, "rotate@example.com", "password123")

	newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh error: %v", err)
	}
	if newPair.RefreshToken == pair.RefreshToken {
		t.Error("refresh token should be rotated on each use")
	}
}

func TestRefresh_OldTokenInvalidAfterRotation(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())
	pair, _ := setupUserAndLogin(t, svc, "oldtoken@example.com", "password123")

	// First refresh consumes the token
	_, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh error: %v", err)
	}

	// Second refresh with same token must fail
	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	if err == nil {
		t.Error("expected error reusing old refresh token")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	_, err := svc.Refresh(context.Background(), "not-a-real-token")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("want ErrInvalidCredentials, got: %v", err)
	}
}

// ----------------------------------------------------------
// Logout tests
// ----------------------------------------------------------

func TestLogout_Success(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())
	pair, _ := setupUserAndLogin(t, svc, "logout@example.com", "password123")

	if err := svc.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Errorf("logout error: %v", err)
	}

	// Token should be invalid after logout
	_, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err == nil {
		t.Error("refresh should fail after logout")
	}
}

func TestLogout_Idempotent(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	// Logging out a non-existent token should not error
	if err := svc.Logout(context.Background(), "nonexistent"); err != nil {
		t.Errorf("logout of unknown token should be idempotent, got: %v", err)
	}
}

func TestLogout_EmptyToken(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Errorf("logout with empty token should be silent, got: %v", err)
	}
}

func TestLogoutAll_InvalidatesAllSessions(t *testing.T) {
	authRepo := newMockAuthRepo()
	userRepo := newMockUserRepo()
	svc := newTestService(authRepo, userRepo)

	// Signup
	_, _ = svc.Signup(context.Background(), auth.SignupRequest{
		Email: "all@example.com", Password: "password123",
		FirstName: "A", LastName: "B",
	})

	// Login from two devices
	pair1, _ := svc.Login(context.Background(), auth.LoginRequest{
		Email: "all@example.com", Password: "password123",
	}, "1.1.1.1", "device-1")
	pair2, _ := svc.Login(context.Background(), auth.LoginRequest{
		Email: "all@example.com", Password: "password123",
	}, "2.2.2.2", "device-2")

	// LogoutAll
	userID := "user-all@example.com"
	if err := svc.LogoutAll(context.Background(), userID); err != nil {
		t.Fatalf("logout-all error: %v", err)
	}

	// Both refresh tokens must be invalid
	if _, err := svc.Refresh(context.Background(), pair1.RefreshToken); err == nil {
		t.Error("pair1 should be invalid after logout-all")
	}
	if _, err := svc.Refresh(context.Background(), pair2.RefreshToken); err == nil {
		t.Error("pair2 should be invalid after logout-all")
	}
}

// ----------------------------------------------------------
// Security invariant tests
// ----------------------------------------------------------

func TestPasswordNotStoredPlaintext(t *testing.T) {
	userRepo := newMockUserRepo()
	svc := newTestService(newMockAuthRepo(), userRepo)

	_, _ = svc.Signup(context.Background(), auth.SignupRequest{
		Email: "hash@example.com", Password: "mysecretpassword",
		FirstName: "A", LastName: "B",
	})

	stored := userRepo.byEmail["hash@example.com"]
	if stored == nil {
		t.Fatal("user not in mock repo")
	}
	if stored.PasswordHash == "mysecretpassword" {
		t.Fatal("SECURITY VIOLATION: password stored in plaintext!")
	}
	if err := password.Verify("mysecretpassword", stored.PasswordHash); err != nil {
		t.Errorf("bcrypt verification failed: %v", err)
	}
}

func TestRefreshTokenNotStoredRaw(t *testing.T) {
	authRepo := newMockAuthRepo()
	userRepo := newMockUserRepo()
	svc := newTestService(authRepo, userRepo)

	_, _ = svc.Signup(context.Background(), auth.SignupRequest{
		Email: "sec@example.com", Password: "password123",
		FirstName: "A", LastName: "B",
	})
	pair, _ := svc.Login(context.Background(), auth.LoginRequest{
		Email: "sec@example.com", Password: "password123",
	}, "127.0.0.1", "test")

	expectedHash := token.Hash(pair.RefreshToken)

	for _, s := range authRepo.sessions {
		if s.UserID != "user-sec@example.com" {
			continue
		}
		if s.TokenHash == pair.RefreshToken {
			t.Fatal("SECURITY VIOLATION: raw refresh token stored in session!")
		}
		if s.TokenHash != expectedHash {
			t.Errorf("stored hash mismatch: want %s, got %s", expectedHash, s.TokenHash)
		}
	}
}

func TestMe_ReturnsUserWithoutPasswordHash(t *testing.T) {
	svc := newTestService(newMockAuthRepo(), newMockUserRepo())

	_, _ = svc.Signup(context.Background(), auth.SignupRequest{
		Email: "me@example.com", Password: "password123",
		FirstName: "John", LastName: "Doe",
	})

	safeUser, err := svc.Me(context.Background(), "user-me@example.com")
	if err != nil {
		t.Fatalf("Me error: %v", err)
	}
	if safeUser == nil {
		t.Fatal("expected user, got nil")
	}
	// SafeUser has no PasswordHash field — this is a compile-time guarantee
	// but we also verify the email is correct
	if safeUser.Email != "me@example.com" {
		t.Errorf("email: want me@example.com, got %s", safeUser.Email)
	}
}
