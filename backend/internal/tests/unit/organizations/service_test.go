// backend/internal/tests/unit/organizations/service_test.go
// Organization service unit tests — no DB, no Redis.
// NOTE: organizations.Service.Create() uses a pgx.Tx internally. We pass
// a minimal fake implementation that satisfies the interface.
package organizations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/organizations"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
)

// ── Fake pgx.Tx ──────────────────────────────────────────────────────────────
// Implements pgx.Tx with no-ops so the service's Begin/Commit/Rollback calls work.

type fakeTx struct {
	// onQueryRow is called by CreateTx to return org fields
	onQueryRow func() pgx.Row
}

func (t *fakeTx) Begin(ctx context.Context) (pgx.Tx, error)   { return t, nil }
func (t *fakeTx) Commit(ctx context.Context) error            { return nil }
func (t *fakeTx) Rollback(ctx context.Context) error          { return nil }
func (t *fakeTx) Conn() *pgx.Conn                             { return nil }
func (t *fakeTx) LargeObjects() pgx.LargeObjects              { return pgx.LargeObjects{} }
func (t *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (t *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.onQueryRow != nil {
		return t.onQueryRow()
	}
	return &fakeRow{}
}

// fakeRow returns fixed values for Scan — used by CreateTx RETURNING clause
type fakeRow struct{ err error }

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	now := time.Now()
	for i, d := range dest {
		switch v := d.(type) {
		case *string:
			switch i {
			case 0: // id
				*v = "org_auto_id"
			case 1: // public_id
				*v = "pub_auto_id"
			}
		case *time.Time:
			*v = now
		}
	}
	return nil
}

// ── Stub org repo ─────────────────────────────────────────────────────────────

type stubOrgRepo struct {
	orgs   map[string]*organizations.Business // keyed by ID
	bySlug map[string]*organizations.Business // keyed by slug
}

func newStubOrgRepo() *stubOrgRepo {
	return &stubOrgRepo{
		orgs:   map[string]*organizations.Business{},
		bySlug: map[string]*organizations.Business{},
	}
}

func (r *stubOrgRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	return &fakeTx{}, nil
}

func (r *stubOrgRepo) CreateTx(_ context.Context, _ pgx.Tx, b *organizations.Business) error {
	b.ID = "org_" + b.Slug
	b.PublicID = b.ID
	b.Status = "active"
	b.CreatedAt = time.Now()
	b.UpdatedAt = time.Now()
	r.orgs[b.ID] = b
	r.bySlug[b.Slug] = b
	return nil
}

func (r *stubOrgRepo) FindByID(_ context.Context, id string) (*organizations.Business, error) {
	return r.orgs[id], nil
}

func (r *stubOrgRepo) FindBySlug(_ context.Context, slug string) (*organizations.Business, error) {
	return r.bySlug[slug], nil
}

func (r *stubOrgRepo) FindByUserID(_ context.Context, userID string) ([]*organizations.MembershipWithRole, error) {
	return nil, nil
}

func (r *stubOrgRepo) FindAllIDs(_ context.Context) ([]string, error) {
	return nil, nil
}

func (r *stubOrgRepo) Update(_ context.Context, b *organizations.Business) error {
	return nil
}

// ── Stub authz repo ──────────────────────────────────────────────────────────

type stubAuthzRepo struct {
	memberships map[string]*authz.Membership // key: userID+":"+orgID
	ownerRole   *authz.Role
}

func newStubAuthzRepo() *stubAuthzRepo {
	ownerRoleID := "role_owner"
	return &stubAuthzRepo{
		memberships: map[string]*authz.Membership{},
		ownerRole:   &authz.Role{ID: ownerRoleID, Name: "owner", IsSystem: true},
	}
}

func (r *stubAuthzRepo) GetRoleByName(_ context.Context, name string) (*authz.Role, error) {
	if name == "owner" {
		return r.ownerRole, nil
	}
	return nil, nil
}

func (r *stubAuthzRepo) GetMembership(_ context.Context, userID, orgID string) (*authz.Membership, error) {
	return r.memberships[userID+":"+orgID], nil
}

func (r *stubAuthzRepo) CreateMembershipTx(_ context.Context, _ pgx.Tx, m *authz.Membership) error {
	m.ID = "mem_" + m.UserID + "_" + m.OrganizationID
	m.PublicID = "pub_" + m.ID
	m.JoinedAt = time.Now()
	r.memberships[m.UserID+":"+m.OrganizationID] = m
	return nil
}

// Remaining interface methods — unused in these tests

func (r *stubAuthzRepo) GetUserPermissions(_ context.Context, _, _ string) ([]*authz.Permission, error) {
	return nil, nil
}
func (r *stubAuthzRepo) GetMemberByRef(_ context.Context, _, _ string) (*authz.Membership, error) {
	return nil, nil
}
func (r *stubAuthzRepo) GetMemberWithUserByRef(_ context.Context, _, _ string) (*authz.MemberWithUser, error) {
	return nil, nil
}
func (r *stubAuthzRepo) ListMembers(_ context.Context, _ string) ([]*authz.MemberWithUser, error) {
	return nil, nil
}
func (r *stubAuthzRepo) CountActiveMembers(_ context.Context, _ string) (int, error) {
	return 1, nil
}
func (r *stubAuthzRepo) GetOrganizationMaxSeats(_ context.Context, _ string) (*int, error) {
	val := 100
	return &val, nil
}
func (r *stubAuthzRepo) SetUserPasswordHash(_ context.Context, _, _ string) error { return nil }
func (r *stubAuthzRepo) GetRoleByID(_ context.Context, _ string) (*authz.Role, error) { return nil, nil }
func (r *stubAuthzRepo) GetRoleByRef(_ context.Context, _, _ string) (*authz.Role, error) {
	return nil, nil
}
func (r *stubAuthzRepo) RoleExists(_ context.Context, _, _ string) (bool, error) { return false, nil }
func (r *stubAuthzRepo) UserRoleName(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (r *stubAuthzRepo) UpdateMembershipRole(_ context.Context, _, _, _ string) error { return nil }
func (r *stubAuthzRepo) UpdateMembership(_ context.Context, _, _ string, _ *authz.Role, _ authz.UpdateMemberRequest) (*authz.Membership, error) {
	return nil, nil
}
func (r *stubAuthzRepo) UpdateMemberPermissions(_ context.Context, _, _ string, _, _ []string) (*authz.Membership, error) {
	return nil, nil
}
func (r *stubAuthzRepo) CreateMembership(_ context.Context, _ *authz.Membership) error { return nil }
func (r *stubAuthzRepo) ListRoles(_ context.Context) ([]*authz.Role, error)             { return nil, nil }
func (r *stubAuthzRepo) ListRolesForOrg(_ context.Context, _ string) ([]*authz.Role, error) {
	return nil, nil
}
func (r *stubAuthzRepo) ListRolesWithPermissions(_ context.Context) ([]*authz.RoleWithPermissions, error) {
	return nil, nil
}
func (r *stubAuthzRepo) ListPermissions(_ context.Context) ([]*authz.Permission, error) {
	return nil, nil
}
func (r *stubAuthzRepo) CreateRole(_ context.Context, _ string, _ authz.CreateRoleRequest) (*authz.Role, error) {
	return nil, nil
}
func (r *stubAuthzRepo) UpdateRole(_ context.Context, _, _ string, _ authz.UpdateRoleRequest) (*authz.Role, error) {
	return nil, nil
}
func (r *stubAuthzRepo) UpdateRolePermissions(_ context.Context, _, _ string, _ []string) (*authz.Role, error) {
	return nil, nil
}
func (r *stubAuthzRepo) DeleteRole(_ context.Context, _, _ string) error { return nil }
func (r *stubAuthzRepo) CloneRole(_ context.Context, _, _, _, _ string) (*authz.Role, error) {
	return nil, nil
}
func (r *stubAuthzRepo) CreateInvitation(_ context.Context, _ *authz.OrganizationInvitation) error {
	return nil
}
func (r *stubAuthzRepo) GetInvitationByRef(_ context.Context, _, _ string) (*authz.OrganizationInvitation, error) {
	return nil, nil
}
func (r *stubAuthzRepo) GetInvitationByTokenHash(_ context.Context, _, _ string) (*authz.OrganizationInvitation, error) {
	return nil, nil
}
func (r *stubAuthzRepo) ResendInvitation(_ context.Context, _, _, _ string, _ time.Time) (*authz.OrganizationInvitation, error) {
	return nil, nil
}
func (r *stubAuthzRepo) RevokeInvitation(_ context.Context, _, _ string) error { return nil }
func (r *stubAuthzRepo) AcceptInvitation(_ context.Context, _, _, _ string) (*authz.Membership, *authz.OrganizationInvitation, error) {
	return nil, nil, nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func newSvc(orgRepo organizations.Repository, authzRepo authz.Repository) organizations.Service {
	mgr := jwtpkg.NewManager("test-secret-32bytes-long-padding!!", 15*time.Minute)
	return organizations.NewService(orgRepo, authzRepo, mgr)
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreate_Success(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	b, err := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "Acme Corp",
		Slug: "acme-corp",
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if b.Name != "Acme Corp" {
		t.Errorf("expected name 'Acme Corp', got %q", b.Name)
	}
}

func TestCreate_SlugNormalisedToLowercase(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	b, err := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "TestCo",
		Slug: "TESTCO",
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if b.Slug != "testco" {
		t.Errorf("expected slug 'testco', got %q", b.Slug)
	}
}

func TestCreate_DuplicateSlug(t *testing.T) {
	orgRepo := newStubOrgRepo()
	svc := newSvc(orgRepo, newStubAuthzRepo())
	_, _ = svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "Acme Corp",
		Slug: "acme",
	})
	_, err := svc.Create(context.Background(), "owner-2", organizations.CreateBusinessRequest{
		Name: "Acme Copy",
		Slug: "acme",
	})
	if !errors.Is(err, organizations.ErrSlugTaken) {
		t.Fatalf("expected ErrSlugTaken, got %v", err)
	}
}

func TestCreate_SlugTooShort(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	_, err := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "Valid Name",
		Slug: "x",
	})
	if err == nil {
		t.Fatal("expected error for slug too short, got nil")
	}
}

func TestCreate_NameTooShort(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	_, err := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "X",
		Slug: "valid-slug",
	})
	if !errors.Is(err, organizations.ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestCreate_NameTooLong(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	name := make([]byte, 101)
	for i := range name {
		name[i] = 'a'
	}
	_, err := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: string(name),
		Slug: "valid-slug",
	})
	if !errors.Is(err, organizations.ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName for name > 100 chars, got %v", err)
	}
}

func TestCreate_SlugWithSpaces_Rejected(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	_, err := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "Valid Name",
		Slug: "has spaces",
	})
	if !errors.Is(err, organizations.ErrInvalidSlug) {
		t.Fatalf("expected ErrInvalidSlug for slug with spaces, got %v", err)
	}
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestGetByID_MemberCanAccess(t *testing.T) {
	orgRepo := newStubOrgRepo()
	authzRepo := newStubAuthzRepo()
	svc := newSvc(orgRepo, authzRepo)

	b, _ := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "TestOrg",
		Slug: "testorg",
	})
	got, err := svc.GetByID(context.Background(), b.ID, "owner-1")
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.ID != b.ID {
		t.Errorf("expected org ID %s, got %s", b.ID, got.ID)
	}
}

func TestGetByID_NonMemberCannotAccess(t *testing.T) {
	orgRepo := newStubOrgRepo()
	authzRepo := newStubAuthzRepo()
	svc := newSvc(orgRepo, authzRepo)

	b, _ := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "PrivateOrg",
		Slug: "privateorg",
	})
	_, err := svc.GetByID(context.Background(), b.ID, "stranger")
	if !errors.Is(err, organizations.ErrNotMember) {
		t.Fatalf("SECURITY: expected ErrNotMember for non-member, got %v", err)
	}
}

func TestGetByID_OrgNotFound(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	_, err := svc.GetByID(context.Background(), "ghost-org", "any-user")
	if !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ── Switch ────────────────────────────────────────────────────────────────────

func TestSwitch_MemberReceivesToken(t *testing.T) {
	orgRepo := newStubOrgRepo()
	authzRepo := newStubAuthzRepo()
	svc := newSvc(orgRepo, authzRepo)

	b, _ := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "SwitchOrg",
		Slug: "switchorg",
	})
	accessToken, role, err := svc.Switch(context.Background(), b.ID, "owner-1")
	if err != nil {
		t.Fatalf("Switch() error: %v", err)
	}
	if accessToken == "" {
		t.Error("expected non-empty access token after switch")
	}
	if role == "" {
		t.Error("expected non-empty role after switch")
	}
}

func TestSwitch_NonMemberFails(t *testing.T) {
	orgRepo := newStubOrgRepo()
	authzRepo := newStubAuthzRepo()
	svc := newSvc(orgRepo, authzRepo)

	b, _ := svc.Create(context.Background(), "owner-1", organizations.CreateBusinessRequest{
		Name: "SwitchOrg2",
		Slug: "switchorg2",
	})
	_, _, err := svc.Switch(context.Background(), b.ID, "stranger")
	if !errors.Is(err, organizations.ErrNotMember) {
		t.Fatalf("expected ErrNotMember for non-member switch, got %v", err)
	}
}

func TestSwitch_NonexistentOrg(t *testing.T) {
	svc := newSvc(newStubOrgRepo(), newStubAuthzRepo())
	_, _, err := svc.Switch(context.Background(), "ghost-org", "user-1")
	if !errors.Is(err, organizations.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
