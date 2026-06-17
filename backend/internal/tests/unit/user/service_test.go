// backend/internal/tests/unit/user/service_test.go
package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/user"
)

// ── Stub ─────────────────────────────────────────────────────────────────────

type stubUserRepo struct {
	users    map[string]*user.User
	forceErr error
}

func newStubRepo() *stubUserRepo {
	return &stubUserRepo{users: map[string]*user.User{}}
}

func (r *stubUserRepo) FindByID(_ context.Context, id string) (*user.User, error) {
	if r.forceErr != nil {
		return nil, r.forceErr
	}
	u, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (r *stubUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	if r.forceErr != nil {
		return nil, r.forceErr
	}
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
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
	return nil
}

func (r *stubUserRepo) Update(_ context.Context, u *user.User) error {
	r.users[u.ID] = u
	return nil
}

func (r *stubUserRepo) UpdateSettings(_ context.Context, id string, req user.UpdateProfileRequest) (*user.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	if req.DisplayName != "" {
		u.DisplayName = req.DisplayName
	}
	return u, nil
}

func (r *stubUserRepo) UpdateAvatar(_ context.Context, id, photoURL string) (*user.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	u.PhotoURL = photoURL
	return u, nil
}

func (r *stubUserRepo) RecordFailedLogin(_ context.Context, _ string) error    { return nil }
func (r *stubUserRepo) RecordSuccessfulLogin(_ context.Context, _ string) error { return nil }

// ── Helpers ──────────────────────────────────────────────────────────────────

func newSvc(repo user.Repository) user.Service { return user.NewService(repo) }

// ── GetByID ──────────────────────────────────────────────────────────────────

func TestGetByID_Found(t *testing.T) {
	repo := newStubRepo()
	repo.users["usr_1"] = &user.User{ID: "usr_1", Email: "alice@example.com"}
	svc := newSvc(repo)
	u, err := svc.GetByID(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", u.Email)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	svc := newSvc(newStubRepo())
	_, err := svc.GetByID(context.Background(), "usr_missing")
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByID_RepoError(t *testing.T) {
	repo := newStubRepo()
	repo.forceErr = errors.New("db down")
	svc := newSvc(repo)
	_, err := svc.GetByID(context.Background(), "usr_any")
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

// ── GetByEmail ────────────────────────────────────────────────────────────────

func TestGetByEmail_Found(t *testing.T) {
	repo := newStubRepo()
	repo.users["usr_2"] = &user.User{ID: "usr_2", Email: "bob@example.com"}
	svc := newSvc(repo)
	u, err := svc.GetByEmail(context.Background(), "bob@example.com")
	if err != nil {
		t.Fatalf("GetByEmail() error: %v", err)
	}
	if u.ID != "usr_2" {
		t.Errorf("expected usr_2, got %s", u.ID)
	}
}

func TestGetByEmail_NotFound(t *testing.T) {
	svc := newSvc(newStubRepo())
	_, err := svc.GetByEmail(context.Background(), "ghost@example.com")
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreate_AssignsID(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)
	u := &user.User{Email: "carol@example.com"}
	if err := svc.Create(context.Background(), u); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if u.ID == "" {
		t.Error("Create() must assign a non-empty ID")
	}
}

// ── UpdateProfile ─────────────────────────────────────────────────────────────

func TestUpdateProfile_Success(t *testing.T) {
	repo := newStubRepo()
	repo.users["usr_3"] = &user.User{ID: "usr_3", Email: "dave@example.com", DisplayName: "Dave"}
	svc := newSvc(repo)
	updated, err := svc.UpdateProfile(context.Background(), "usr_3", user.UpdateProfileRequest{
		DisplayName: "Dave Updated",
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error: %v", err)
	}
	if updated.DisplayName != "Dave Updated" {
		t.Errorf("expected 'Dave Updated', got %q", updated.DisplayName)
	}
}

func TestUpdateProfile_UserNotFound(t *testing.T) {
	svc := newSvc(newStubRepo())
	_, err := svc.UpdateProfile(context.Background(), "usr_missing", user.UpdateProfileRequest{})
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ── UpdateAvatar ──────────────────────────────────────────────────────────────

func TestUpdateAvatar_Success(t *testing.T) {
	repo := newStubRepo()
	repo.users["usr_4"] = &user.User{ID: "usr_4", Email: "eve@example.com"}
	svc := newSvc(repo)
	updated, err := svc.UpdateAvatar(context.Background(), "usr_4", "https://example.com/avatar.jpg")
	if err != nil {
		t.Fatalf("UpdateAvatar() error: %v", err)
	}
	if updated.PhotoURL != "https://example.com/avatar.jpg" {
		t.Errorf("expected photo URL to be set, got %q", updated.PhotoURL)
	}
}

func TestUpdateAvatar_EmptyURL_ReturnsError(t *testing.T) {
	repo := newStubRepo()
	repo.users["usr_5"] = &user.User{ID: "usr_5", Email: "frank@example.com"}
	svc := newSvc(repo)
	_, err := svc.UpdateAvatar(context.Background(), "usr_5", "")
	if err == nil {
		t.Fatal("expected error for empty avatar URL, got nil")
	}
}

func TestUpdateAvatar_WhitespaceURL_ReturnsError(t *testing.T) {
	repo := newStubRepo()
	repo.users["usr_6"] = &user.User{ID: "usr_6", Email: "grace@example.com"}
	svc := newSvc(repo)
	_, err := svc.UpdateAvatar(context.Background(), "usr_6", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only avatar URL, got nil")
	}
}

// ── User model ────────────────────────────────────────────────────────────────

func TestUserModel_IsLocked_FutureLock(t *testing.T) {
	future := time.Now().Add(15 * time.Minute)
	u := &user.User{LockedUntil: &future}
	if !u.IsLocked() {
		t.Error("expected IsLocked()=true when LockedUntil is in the future")
	}
}

func TestUserModel_IsLocked_PastLock(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	u := &user.User{LockedUntil: &past}
	if u.IsLocked() {
		t.Error("expected IsLocked()=false when LockedUntil is in the past")
	}
}

func TestUserModel_IsLocked_NilLock(t *testing.T) {
	u := &user.User{}
	if u.IsLocked() {
		t.Error("expected IsLocked()=false when LockedUntil is nil")
	}
}

func TestUserModel_ToSafe_DoesNotExposePasswordHash(t *testing.T) {
	u := &user.User{
		ID:           "usr_toSafe",
		Email:        "tosafe@example.com",
		PasswordHash: "$2a$12$somehashedvalue",
		Status:       user.StatusActive,
	}
	safe := u.ToSafe()
	if safe == nil {
		t.Fatal("ToSafe() returned nil")
	}
	if safe.Email != "tosafe@example.com" {
		t.Errorf("SafeUser.Email wrong: got %q", safe.Email)
	}
	// SafeUser struct has no PasswordHash field — verified at compile time.
	// We just ensure the email is correct and the type is *user.SafeUser.
}

func TestUserModel_ToSafe_NilSlicesAreEmpty(t *testing.T) {
	u := &user.User{ID: "u1", Email: "e@e.com", Shortcuts: nil}
	safe := u.ToSafe()
	if safe.Shortcuts == nil {
		t.Error("SafeUser.Shortcuts must be an empty slice, not nil")
	}
}
