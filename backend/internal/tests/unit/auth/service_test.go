// backend/internal/tests/unit/auth/service_test.go
// Complete auth service unit tests — no DB, no Redis.
// Every test runs in-process with stub repositories.
package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/user"
	"github.com/mridha/businesssaas/internal/platform/notifications"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/password"
	"github.com/mridha/businesssaas/pkg/token"
)

// ── Stubs ────────────────────────────────────────────────────────────────────

type stubUserRepo struct {
	users    map[string]*user.User // keyed by ID
	byEmail  map[string]*user.User // keyed by normalised email
	forceErr error
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{
		users:   map[string]*user.User{},
		byEmail: map[string]*user.User{},
	}
}

func (r *stubUserRepo) seed(u *user.User) {
	r.users[u.ID] = u
	r.byEmail[strings.ToLower(u.Email)] = u
}

func (r *stubUserRepo) FindByID(_ context.Context, id string) (*user.User, error) {
	if r.forceErr != nil {
		return nil, r.forceErr
	}
	return r.users[id], nil
}

func (r *stubUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	if r.forceErr != nil {
		return nil, r.forceErr
	}
	return r.byEmail[strings.ToLower(email)], nil
}

func (r *stubUserRepo) Create(_ context.Context, u *user.User) error {
	if r.forceErr != nil {
		return r.forceErr
	}
	u.ID = "usr_" + u.Email
	u.PublicID = u.ID
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	r.users[u.ID] = u
	r.byEmail[strings.ToLower(u.Email)] = u
	return nil
}

func (r *stubUserRepo) Update(_ context.Context, u *user.User) error {
	r.users[u.ID] = u
	return nil
}

func (r *stubUserRepo) UpdateSettings(_ context.Context, _ string, _ user.UpdateProfileRequest) (*user.User, error) {
	return nil, nil
}

func (r *stubUserRepo) RecordFailedLogin(_ context.Context, _ string) error     { return nil }
func (r *stubUserRepo) RecordSuccessfulLogin(_ context.Context, _ string) error { return nil }

// ─────────────────────────────────────────────────────────────────────────────

type stubAuthRepo struct {
	sessions map[string]*auth.Session
	accounts map[string]*auth.AuthAccount // key: provider+":"+providerAccountID
}

func newStubAuthRepo() *stubAuthRepo {
	return &stubAuthRepo{
		sessions: map[string]*auth.Session{},
		accounts: map[string]*auth.AuthAccount{},
	}
}

func (r *stubAuthRepo) CreateSession(_ context.Context, s *auth.Session) error {
	s.ID = "sess_1"
	s.PublicID = "pub_1"
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

func (r *stubAuthRepo) FindAuthAccount(_ context.Context, provider, providerAccountID string) (*auth.AuthAccount, error) {
	key := provider + ":" + providerAccountID
	a, ok := r.accounts[key]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (r *stubAuthRepo) CreateAuthAccount(_ context.Context, a *auth.AuthAccount, _, _, _ string) error {
	key := a.Provider + ":" + a.ProviderAccountID
	a.ID = "acct_1"
	a.PublicID = "pub_acct_1"
	r.accounts[key] = a
	return nil
}

func (r *stubAuthRepo) UpdateAuthAccount(_ context.Context, _ *auth.AuthAccount, _, _, _ string) error {
	return nil
}

func (r *stubAuthRepo) CreateLoginEvent(_ context.Context, _ auth.LoginEvent) error { return nil }

// ── Helpers ──────────────────────────────────────────────────────────────────

func newSvc(userRepo user.Repository, authRepo auth.Repository) auth.Service {
	mgr := jwtpkg.NewManager("test-secret-32-bytes-long-padding!!", 15*time.Minute)
	cfg := config.JWTConfig{
		Secret:          "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
	return auth.NewService(authRepo, userRepo, mgr, cfg, audit.NewService(audit.NewNoopRepository()), &stubNotificationsService{})
}

func mustHash(plain string) string {
	h, err := password.Hash(plain)
	if err != nil {
		panic(err)
	}
	return h
}

func seedActiveUser(repo *stubUserRepo, id, email, plain string) *user.User {
	u := &user.User{
		ID:           id,
		PublicID:     id,
		Email:        email,
		PasswordHash: mustHash(plain),
		Status:       user.StatusActive,
	}
	repo.seed(u)
	return u
}

// ── Signup ───────────────────────────────────────────────────────────────────

func TestSignup_Success(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	u, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:    "alice@example.com",
		Password: "securePass1!",
	})
	if err != nil {
		t.Fatalf("Signup() unexpected error: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("want email alice@example.com, got %s", u.Email)
	}
}

func TestSignup_EmailNormalised(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	u, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:    "  ALICE@EXAMPLE.COM  ",
		Password: "securePass1!",
	})
	if err != nil {
		t.Fatalf("Signup() unexpected error: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email not normalised: got %q", u.Email)
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	repo := newStubUserRepo()
	svc := newSvc(repo, newStubAuthRepo())
	_, _ = svc.Signup(context.Background(), auth.SignupRequest{Email: "bob@example.com", Password: "pass12345"})
	_, err := svc.Signup(context.Background(), auth.SignupRequest{Email: "bob@example.com", Password: "pass12345"})
	if !errors.Is(err, auth.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestSignup_DuplicateEmail_CaseInsensitive(t *testing.T) {
	repo := newStubUserRepo()
	svc := newSvc(repo, newStubAuthRepo())
	_, _ = svc.Signup(context.Background(), auth.SignupRequest{Email: "carol@example.com", Password: "pass12345"})
	_, err := svc.Signup(context.Background(), auth.SignupRequest{Email: "CAROL@EXAMPLE.COM", Password: "pass12345"})
	if !errors.Is(err, auth.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists for same email different case, got %v", err)
	}
}

func TestSignup_DisplayNameFallsBackToEmail(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	u, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:    "nobody@example.com",
		Password: "pass12345",
	})
	if err != nil {
		t.Fatalf("Signup() error: %v", err)
	}
	if u.DisplayName == "" {
		t.Error("DisplayName must not be empty when no name provided")
	}
}

func TestSignup_PasswordNotStoredInPlaintext(t *testing.T) {
	repo := newStubUserRepo()
	svc := newSvc(repo, newStubAuthRepo())
	plain := "mySecurePassword!"
	_, err := svc.Signup(context.Background(), auth.SignupRequest{
		Email:    "dave@example.com",
		Password: plain,
	})
	if err != nil {
		t.Fatalf("Signup() error: %v", err)
	}
	stored := repo.byEmail["dave@example.com"]
	if stored.PasswordHash == plain {
		t.Error("password must not be stored in plaintext")
	}
}

// ── Login ────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_carol", "carol@example.com", "mypassword")
	svc := newSvc(repo, newStubAuthRepo())
	pair, err := svc.Login(context.Background(), auth.LoginRequest{
		Email:    "carol@example.com",
		Password: "mypassword",
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

func TestLogin_WrongPassword(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_dave", "dave@example.com", "correctpassword")
	svc := newSvc(repo, newStubAuthRepo())
	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email:    "dave@example.com",
		Password: "wrongpassword",
	}, "127.0.0.1", "agent")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmail_ReturnsGenericError(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "ghost@example.com", Password: "irrelevant",
	}, "127.0.0.1", "agent")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown email, got %v", err)
	}
}

func TestLogin_SuspendedAccount(t *testing.T) {
	repo := newStubUserRepo()
	h := mustHash("pass")
	repo.seed(&user.User{
		ID: "usr_eve", Email: "eve@example.com",
		PasswordHash: h, Status: user.StatusSuspended,
	})
	svc := newSvc(repo, newStubAuthRepo())
	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "eve@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	if err == nil {
		t.Fatal("expected error for suspended account, got nil")
	}
}

func TestLogin_LockedAccount(t *testing.T) {
	repo := newStubUserRepo()
	future := time.Now().Add(15 * time.Minute)
	h := mustHash("pass")
	repo.seed(&user.User{
		ID: "usr_locked", Email: "locked@example.com",
		PasswordHash: h, Status: user.StatusActive,
		LockedUntil: &future,
	})
	svc := newSvc(repo, newStubAuthRepo())
	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "locked@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	if !errors.Is(err, auth.ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked, got %v", err)
	}
}

func TestLogin_CaseInsensitiveEmail(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_frank", "frank@example.com", "pass")
	svc := newSvc(repo, newStubAuthRepo())
	_, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "FRANK@EXAMPLE.COM", Password: "pass",
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("Login() with uppercase email failed: %v", err)
	}
}

func TestLogin_AccessAndRefreshTokensAreDistinct(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_g", "g@example.com", "pass")
	svc := newSvc(repo, newStubAuthRepo())
	pair, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "g@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Error("access and refresh tokens must differ")
	}
}

func TestLogin_RefreshTokenIsOpaqueNotJWT(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_h", "h@example.com", "pass")
	svc := newSvc(repo, newStubAuthRepo())
	pair, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "h@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	// A JWT has exactly 2 dots; an opaque base64url token has none
	parts := strings.Split(pair.RefreshToken, ".")
	if len(parts) == 3 {
		t.Error("refresh token looks like a JWT — it should be an opaque token")
	}
}

// ── Refresh ──────────────────────────────────────────────────────────────────

func TestRefresh_Success(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_ref", "ref@example.com", "pass")
	authRepo := newStubAuthRepo()
	svc := newSvc(repo, authRepo)

	pair, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "ref@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	pair2, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}
	if pair2.AccessToken == "" {
		t.Error("expected non-empty access token after refresh")
	}
	if pair2.RefreshToken == "" {
		t.Error("expected non-empty refresh token after refresh")
	}
}

func TestRefresh_OldTokenInvalidatedAfterRotation(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_rot", "rot@example.com", "pass")
	authRepo := newStubAuthRepo()
	svc := newSvc(repo, authRepo)

	pair, _ := svc.Login(context.Background(), auth.LoginRequest{
		Email: "rot@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	_, _ = svc.Refresh(context.Background(), pair.RefreshToken)

	// Old token must no longer work
	_, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error reusing old refresh token after rotation (token reuse attack)")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	_, err := svc.Refresh(context.Background(), "garbage-token")
	if err == nil {
		t.Fatal("expected error for invalid refresh token, got nil")
	}
}

// ── Logout ───────────────────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_lo", "lo@example.com", "pass")
	authRepo := newStubAuthRepo()
	svc := newSvc(repo, authRepo)

	pair, _ := svc.Login(context.Background(), auth.LoginRequest{
		Email: "lo@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	if err := svc.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("Logout() error: %v", err)
	}
}

func TestLogout_EmptyTokenIsNoOp(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Fatalf("Logout with empty token must be a no-op, got error: %v", err)
	}
}

func TestLogout_UnknownTokenIsNoOp(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	if err := svc.Logout(context.Background(), "unknown-token"); err != nil {
		t.Fatalf("Logout with unknown token must be a no-op, got error: %v", err)
	}
}

// ── LogoutAll ────────────────────────────────────────────────────────────────

func TestLogoutAll_Success(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	if err := svc.LogoutAll(context.Background(), "usr_any"); err != nil {
		t.Fatalf("LogoutAll() error: %v", err)
	}
}

// ── Me ───────────────────────────────────────────────────────────────────────

func TestMe_ReturnsUser(t *testing.T) {
	repo := newStubUserRepo()
	repo.seed(&user.User{ID: "usr_me", Email: "me@example.com", Status: user.StatusActive})
	svc := newSvc(repo, newStubAuthRepo())
	u, err := svc.Me(context.Background(), "usr_me")
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if u.Email != "me@example.com" {
		t.Errorf("expected me@example.com, got %s", u.Email)
	}
}

func TestMe_SafeUserDoesNotExposePasswordHash(t *testing.T) {
	repo := newStubUserRepo()
	repo.seed(&user.User{
		ID:           "usr_safe",
		Email:        "safe@example.com",
		PasswordHash: mustHash("secret"),
		Status:       user.StatusActive,
	})
	svc := newSvc(repo, newStubAuthRepo())
	u, err := svc.Me(context.Background(), "usr_safe")
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	// SafeUser has no PasswordHash field — this is a compile-time guarantee.
	// At runtime confirm the email is correct and no accidental plaintext leak.
	if u.Email == "" {
		t.Error("SafeUser.Email should not be empty")
	}
	_ = u // type assertion: u is *user.SafeUser, which has no PasswordHash
}

func TestMe_UnknownUserReturnsError(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	_, err := svc.Me(context.Background(), "usr_does_not_exist")
	if err == nil {
		t.Fatal("expected error for unknown user ID, got nil")
	}
}

// ── OAuthSync ────────────────────────────────────────────────────────────────

func TestOAuthSync_NewUserCreated(t *testing.T) {
	repo := newStubUserRepo()
	authRepo := newStubAuthRepo()
	svc := newSvc(repo, authRepo)

	boolTrue := true
	resp, err := svc.OAuthSync(context.Background(), auth.OAuthSyncRequest{
		Provider:          "google",
		ProviderAccountID: "google-uid-001",
		Email:             "oauth@example.com",
		DisplayName:       "OAuth User",
		IssueTokens:       &boolTrue,
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("OAuthSync() error: %v", err)
	}
	if resp.Tokens == nil {
		t.Error("expected tokens in response when IssueTokens=true")
	}
}

func TestOAuthSync_ExistingEmailLinksAccount(t *testing.T) {
	repo := newStubUserRepo()
	repo.seed(&user.User{
		ID:     "usr_existing",
		Email:  "existing@example.com",
		Status: user.StatusActive,
	})
	authRepo := newStubAuthRepo()
	svc := newSvc(repo, authRepo)

	boolFalse := false
	_, err := svc.OAuthSync(context.Background(), auth.OAuthSyncRequest{
		Provider:          "github",
		ProviderAccountID: "github-uid-999",
		Email:             "existing@example.com",
		IssueTokens:       &boolFalse,
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("OAuthSync() for existing email failed: %v", err)
	}
}

func TestOAuthSync_MissingProvider_Error(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	_, err := svc.OAuthSync(context.Background(), auth.OAuthSyncRequest{
		ProviderAccountID: "uid-1",
		Email:             "x@example.com",
	}, "127.0.0.1", "agent")
	if !errors.Is(err, auth.ErrOAuthProviderRequired) {
		t.Fatalf("expected ErrOAuthProviderRequired, got %v", err)
	}
}

func TestOAuthSync_MissingProviderAccountID_Error(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	_, err := svc.OAuthSync(context.Background(), auth.OAuthSyncRequest{
		Provider: "google",
		Email:    "x@example.com",
	}, "127.0.0.1", "agent")
	if !errors.Is(err, auth.ErrOAuthAccountIDRequired) {
		t.Fatalf("expected ErrOAuthAccountIDRequired, got %v", err)
	}
}

func TestOAuthSync_MissingEmail_WhenNoExistingAccount_Error(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	_, err := svc.OAuthSync(context.Background(), auth.OAuthSyncRequest{
		Provider:          "google",
		ProviderAccountID: "uid-no-email",
		Email:             "",
	}, "127.0.0.1", "agent")
	if !errors.Is(err, auth.ErrOAuthEmailRequired) {
		t.Fatalf("expected ErrOAuthEmailRequired, got %v", err)
	}
}

func TestOAuthSync_IssueTokensFalse_ReturnsNoTokens(t *testing.T) {
	svc := newSvc(newStubUserRepo(), newStubAuthRepo())
	boolFalse := false
	resp, err := svc.OAuthSync(context.Background(), auth.OAuthSyncRequest{
		Provider:          "google",
		ProviderAccountID: "google-no-tok",
		Email:             "notok@example.com",
		IssueTokens:       &boolFalse,
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("OAuthSync() error: %v", err)
	}
	if resp.Tokens != nil {
		t.Error("expected no tokens when IssueTokens=false")
	}
}

func TestOAuthSync_SuspendedOAuthUser_Error(t *testing.T) {
	repo := newStubUserRepo()
	repo.seed(&user.User{
		ID:     "usr_susp",
		Email:  "suspended@example.com",
		Status: user.StatusSuspended,
	})
	authRepo := newStubAuthRepo()
	// Pre-seed an existing auth account so lookup finds the suspended user
	authRepo.accounts["google:google-uid-susp"] = &auth.AuthAccount{
		ID:                "acct_susp",
		UserID:            "usr_susp",
		Provider:          "google",
		ProviderAccountID: "google-uid-susp",
	}
	svc := newSvc(repo, authRepo)
	_, err := svc.OAuthSync(context.Background(), auth.OAuthSyncRequest{
		Provider:          "google",
		ProviderAccountID: "google-uid-susp",
		Email:             "suspended@example.com",
	}, "127.0.0.1", "agent")
	if err == nil {
		t.Fatal("expected error for suspended OAuth user, got nil")
	}
}

// ── Token format invariants ──────────────────────────────────────────────────

func TestLogin_AccessTokenIsJWT(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_jwt", "jwt@example.com", "pass")
	svc := newSvc(repo, newStubAuthRepo())
	pair, err := svc.Login(context.Background(), auth.LoginRequest{
		Email: "jwt@example.com", Password: "pass",
	}, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	// JWT has format: header.payload.signature (exactly 2 dots)
	parts := strings.Split(pair.AccessToken, ".")
	if len(parts) != 3 {
		t.Errorf("access token does not look like a JWT: got %d parts", len(parts))
	}
}

func TestLogin_RefreshTokenStoredAsHash_NotPlaintext(t *testing.T) {
	repo := newStubUserRepo()
	seedActiveUser(repo, "usr_hash", "hash@example.com", "pass")
	authRepo := newStubAuthRepo()
	svc := newSvc(repo, authRepo)
	pair, _ := svc.Login(context.Background(), auth.LoginRequest{
		Email: "hash@example.com", Password: "pass",
	}, "127.0.0.1", "agent")

	// The session stored in authRepo must store the HASH, not the raw token
	rawHash := token.Hash(pair.RefreshToken)
	found := false
	for k := range authRepo.sessions {
		if k == pair.RefreshToken {
			found = true
			break
		}
	}
	if found {
		t.Error("raw refresh token must not be stored — only its SHA-256 hash should be stored")
	}
	if _, ok := authRepo.sessions[rawHash]; !ok {
		t.Error("SHA-256 hash of refresh token must be stored in sessions")
	}
}
func (r *stubUserRepo) UpdatePassword(_ context.Context, id string, hash string) error {
	if u, ok := r.users[id]; ok {
		u.PasswordHash = hash
		return nil
	}
	return errors.New("not found")
}

func (r *stubAuthRepo) CreateVerificationToken(_ context.Context, vt *auth.VerificationToken) error {
	vt.ID = "vt_1"
	return nil
}

func (r *stubAuthRepo) GetVerificationTokenByHash(_ context.Context, hash, tokenType string) (*auth.VerificationToken, error) {
	// For tests, we'll just return a valid token if the hash is not empty, unless we want to simulate failure.
	if hash == token.Hash("invalid") {
		return nil, nil
	}
	userID := "usr_test@example.com"
	email := "test@example.com"
	return &auth.VerificationToken{
		ID:        "vt_1",
		UserID:    &userID,
		Email:     &email,
		TokenHash: hash,
		Type:      tokenType,
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (r *stubAuthRepo) MarkVerificationTokenUsed(_ context.Context, _ string) error { return nil }

type stubNotificationsService struct {
	dispatched []notifications.DispatchRequest
}

func (s *stubNotificationsService) Dispatch(ctx context.Context, req notifications.DispatchRequest) error {
	s.dispatched = append(s.dispatched, req)
	return nil
}

func (s *stubNotificationsService) ListInApp(ctx context.Context, userID uuid.UUID, limit, offset int) (*notifications.NotificationListResponse, error) {
	return &notifications.NotificationListResponse{}, nil
}
func (s *stubNotificationsService) MarkRead(ctx context.Context, userID, notifID uuid.UUID) error {
	return nil
}
func (s *stubNotificationsService) MarkAllRead(ctx context.Context, userID uuid.UUID) error { return nil }
func (s *stubNotificationsService) ListPreferences(ctx context.Context, userID uuid.UUID) ([]*notifications.NotificationPreference, error) {
	return nil, nil
}
func (s *stubNotificationsService) UpdatePreference(ctx context.Context, userID uuid.UUID, eventType, channel string, enabled bool) error {
	return nil
}

func TestRequestPasswordReset(t *testing.T) {
	authRepo := newStubAuthRepo()
	userRepo := newStubUserRepo()
	
	mgr := jwtpkg.NewManager("test-secret-32-bytes-long-padding!!", 15*time.Minute)
	cfg := config.JWTConfig{
		Secret:          "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
	notifSvc := &stubNotificationsService{}
	svc := auth.NewService(authRepo, userRepo, mgr, cfg, audit.NewService(audit.NewNoopRepository()), notifSvc)

	ctx := context.Background()
	existingUser := &user.User{ID: "00000000-0000-0000-0000-000000000000", Email: "test@example.com"}
	userRepo.seed(existingUser)

	// User exists
	err := svc.RequestPasswordReset(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(notifSvc.dispatched) != 1 {
		t.Fatalf("expected 1 notification dispatched, got %d", len(notifSvc.dispatched))
	}
	
	if notifSvc.dispatched[0].EventType != notifications.EventPasswordReset {
		t.Fatalf("expected password reset event, got %s", notifSvc.dispatched[0].EventType)
	}

	// User does not exist (should not fail, but shouldn't dispatch)
	notifSvc.dispatched = nil
	err = svc.RequestPasswordReset(ctx, "notfound@example.com")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(notifSvc.dispatched) != 0 {
		t.Fatalf("expected 0 notifications dispatched, got %d", len(notifSvc.dispatched))
	}
}

func TestConfirmPasswordReset(t *testing.T) {
	authRepo := newStubAuthRepo()
	userRepo := newStubUserRepo()
	
	mgr := jwtpkg.NewManager("test-secret-32-bytes-long-padding!!", 15*time.Minute)
	cfg := config.JWTConfig{
		Secret:          "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
	notifSvc := &stubNotificationsService{}
	svc := auth.NewService(authRepo, userRepo, mgr, cfg, audit.NewService(audit.NewNoopRepository()), notifSvc)

	ctx := context.Background()
	existingUser := &user.User{ID: "usr_test@example.com", Email: "test@example.com"}
	userRepo.seed(existingUser)

	err := svc.ConfirmPasswordReset(ctx, "validtoken", "newpassword")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if existingUser.PasswordHash == "" {
		t.Fatalf("expected password hash to be set")
	}

	// Invalid token
	err = svc.ConfirmPasswordReset(ctx, "invalid", "newpassword")
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
