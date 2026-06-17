// backend/internal/tests/integration/auth_integration_test.go
// End-to-end auth flow tests against a real Postgres + Redis.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mridha/businesssaas/internal/auth"
)

// TestIntegration_Auth_FullLoginFlow verifies the complete auth lifecycle:
// signup → login → refresh (token rotation) → old token invalidated → logout
func TestIntegration_Auth_FullLoginFlow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	email := uniqueEmail("authflow")

	// 1. Signup
	safe, err := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email:    email,
		Password: "TestPass123!",
	})
	if err != nil {
		t.Fatalf("Signup() error: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, safe.ID) })

	// 2. Login
	pair, err := env.authSvc.Login(ctx, auth.LoginRequest{
		Email:    email,
		Password: "TestPass123!",
	}, "127.0.0.1", "test-agent/1.0")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty tokens from Login()")
	}

	// 3. Refresh (token rotation)
	pair2, err := env.authSvc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}
	if pair2.AccessToken == "" || pair2.RefreshToken == "" {
		t.Fatal("expected non-empty tokens from Refresh()")
	}

	// 4. Old refresh token must be invalidated (reuse attack prevention)
	_, err = env.authSvc.Refresh(ctx, pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error reusing old refresh token after rotation — token reuse attack must be rejected")
	}

	// 5. New token still works
	_, err = env.authSvc.Refresh(ctx, pair2.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() with new token failed: %v", err)
	}

	// 6. Logout
	if err := env.authSvc.Logout(ctx, pair2.RefreshToken); err != nil {
		t.Fatalf("Logout() error: %v", err)
	}
}

func TestIntegration_Auth_DuplicateEmail_OnRealDB(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	email := uniqueEmail("dupemail")

	safe, err := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email:    email,
		Password: "TestPass123!",
	})
	if err != nil {
		t.Fatalf("first Signup() error: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, safe.ID) })

	_, err = env.authSvc.Signup(ctx, auth.SignupRequest{
		Email:    email,
		Password: "AnotherPass456!",
	})
	if !errors.Is(err, auth.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists on duplicate, got %v", err)
	}
}

func TestIntegration_Auth_WrongPassword(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	email := uniqueEmail("wrongpass")

	safe, _ := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email:    email,
		Password: "CorrectPass123!",
	})
	t.Cleanup(func() { cleanupUser(t, env, safe.ID) })

	_, err := env.authSvc.Login(ctx, auth.LoginRequest{
		Email:    email,
		Password: "WrongPass999!",
	}, "127.0.0.1", "agent")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for wrong password, got %v", err)
	}
}

func TestIntegration_Auth_LogoutAll_InvalidatesAllSessions(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	email := uniqueEmail("logoutall")

	safe, _ := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email:    email,
		Password: "TestPass123!",
	})
	t.Cleanup(func() { cleanupUser(t, env, safe.ID) })

	// Create two sessions
	pair1, _ := env.authSvc.Login(ctx, auth.LoginRequest{Email: email, Password: "TestPass123!"}, "10.0.0.1", "agent-1")
	pair2, _ := env.authSvc.Login(ctx, auth.LoginRequest{Email: email, Password: "TestPass123!"}, "10.0.0.2", "agent-2")

	// Logout all
	if err := env.authSvc.LogoutAll(ctx, safe.ID); err != nil {
		t.Fatalf("LogoutAll() error: %v", err)
	}

	// Both refresh tokens must now be invalid
	_, err1 := env.authSvc.Refresh(ctx, pair1.RefreshToken)
	_, err2 := env.authSvc.Refresh(ctx, pair2.RefreshToken)
	if err1 == nil || err2 == nil {
		t.Error("expected both refresh tokens to be invalidated after LogoutAll")
	}
}

func TestIntegration_Auth_Me_ReturnsCorrectUser(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	email := uniqueEmail("metest")

	safe, err := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email:    email,
		Password: "TestPass123!",
	})
	if err != nil {
		t.Fatalf("Signup() error: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, safe.ID) })

	me, err := env.authSvc.Me(ctx, safe.ID)
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if !strings.EqualFold(me.Email, email) {
		t.Errorf("Me() returned wrong email: got %q, want %q", me.Email, email)
	}
}

func TestIntegration_Auth_AccessTokenIsJWT(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	email := uniqueEmail("jwtcheck")

	safe, _ := env.authSvc.Signup(ctx, auth.SignupRequest{Email: email, Password: "TestPass123!"})
	t.Cleanup(func() { cleanupUser(t, env, safe.ID) })

	pair, _ := env.authSvc.Login(ctx, auth.LoginRequest{Email: email, Password: "TestPass123!"}, "127.0.0.1", "agent")

	// JWT has exactly 3 dot-separated parts
	parts := strings.Split(pair.AccessToken, ".")
	if len(parts) != 3 {
		t.Errorf("access token is not a valid JWT: expected 3 parts, got %d", len(parts))
	}

	// Refresh token must NOT be a JWT
	rtParts := strings.Split(pair.RefreshToken, ".")
	if len(rtParts) == 3 {
		t.Error("refresh token must be an opaque token, not a JWT")
	}
}
