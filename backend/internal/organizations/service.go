// backend/internal/organizations/service.go
// Package name is kept as organizations. It manages organizations.
package organizations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/internal/authz"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Service interface {
	Create(ctx context.Context, ownerID string, req CreateBusinessRequest) (*Business, error)
	GetByID(ctx context.Context, businessID, requestingUserID string) (*Business, error)
	ListForUser(ctx context.Context, userID string) ([]*MembershipWithRole, error)
	FindAllIDs(ctx context.Context) ([]string, error)
	Switch(ctx context.Context, businessID, userID string) (accessToken string, role string, err error)
	Update(ctx context.Context, orgID, userID string, req UpdateBusinessRequest) (*Business, error)
}

type serviceImpl struct {
	repo       Repository
	authzRepo  authz.Repository
	jwtManager *jwtpkg.Manager
}

func NewService(repo Repository, authzRepo authz.Repository, jwtManager *jwtpkg.Manager) Service {
	return &serviceImpl{repo: repo, authzRepo: authzRepo, jwtManager: jwtManager}
}

func (s *serviceImpl) Create(ctx context.Context, ownerID string, req CreateBusinessRequest) (*Business, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	req.Name = strings.TrimSpace(req.Name)

	existing, err := s.repo.FindBySlug(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("organization: Create: slug check: %w", err)
	}
	if existing != nil {
		return nil, ErrSlugTaken
	}

	ownerRole, err := s.authzRepo.GetRoleByName(ctx, authz.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("organization: Create: get owner role: %w", err)
	}
	if ownerRole == nil {
		return nil, fmt.Errorf("organization: Create: owner role not seeded in database")
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("organization: Create: begin tx: %w", err)
	}
	// FIX: single deferred rollback — safe no-op after Commit().
	// Removes all manual tx.Rollback() calls and prevents leaking transactions
	// if any future code path returns early without an explicit rollback.
	defer func() { _ = tx.Rollback(ctx) }()

	b := &Business{
		Name: req.Name, Slug: req.Slug,
		LegalName: strings.TrimSpace(req.LegalName),
		Type:      strings.TrimSpace(req.Type),
		Industry:  strings.TrimSpace(req.Industry),
		Website:   strings.TrimSpace(req.Website),
		LogoURL:   strings.TrimSpace(req.LogoURL),
		Country:   strings.TrimSpace(req.Country),
		Timezone:  strings.TrimSpace(req.Timezone),
		Currency:  strings.TrimSpace(req.Currency),
	}
	if err := s.repo.CreateTx(ctx, tx, b); err != nil {
		return nil, fmt.Errorf("organization: Create: insert organization: %w", err)
	}

	roleID := ownerRole.ID
	membership := &authz.Membership{
		UserID:            ownerID,
		OrganizationID:    b.ID,
		RoleID:            &roleID,
		RoleKey:           authz.RoleOwner,
		Status:            "active",
		CustomPermissions: []string{},
		DeniedPermissions: []string{},
		InvitationStatus:  "accepted",
	}
	if err := s.authzRepo.CreateMembershipTx(ctx, tx, membership); err != nil {
		return nil, fmt.Errorf("organization: Create: insert owner membership: %w", err)
	}

	// Seed the HRM employee statuses this org will need before it can create
	// its first employee.
	//
	// Migration 00053 seeded these per-org, but only for orgs that existed
	// when it ran — so every org created through this endpoint since then had
	// none, and POST /hrm/employees failed on a NOT NULL status_id. Inside the
	// same transaction as the membership, so an org either exists fully usable
	// or not at all; a half-created org that cannot hire anyone is worse than
	// a failed create.
	if err := seedEmployeeStatusesTx(ctx, tx, b.ID); err != nil {
		return nil, fmt.Errorf("organization: Create: seed employee statuses: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("organization: Create: commit: %w", err)
	}
	slog.Info("organization: created",
		slog.String("organization_id", b.ID),
		slog.String("owner_id", ownerID),
		slog.String("slug", b.Slug),
	)
	return b, nil
}

func (s *serviceImpl) GetByID(ctx context.Context, businessID, requestingUserID string) (*Business, error) {
	b, err := s.repo.FindByID(ctx, businessID)
	if err != nil {
		return nil, fmt.Errorf("organization: GetByID: %w", err)
	}
	if b == nil {
		return nil, ErrNotFound
	}
	membership, err := s.authzRepo.GetMembership(ctx, requestingUserID, businessID)
	if err != nil {
		return nil, fmt.Errorf("organization: GetByID: membership check: %w", err)
	}
	if membership == nil || membership.Status != "active" || membership.InvitationStatus != "accepted" {
		return nil, ErrNotMember
	}
	return b, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, userID string, req UpdateBusinessRequest) (*Business, error) {
	// 1. Verify membership and permissions
	membership, err := s.authzRepo.GetMembership(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization: Update: get membership: %w", err)
	}
	if membership == nil || membership.Status != "active" || membership.InvitationStatus != "accepted" {
		return nil, ErrNotMember
	}
	// Basic RBAC check - only owners and admins can update org settings
	if membership.RoleKey != authz.RoleOwner && membership.RoleKey != authz.RoleAdmin {
		return nil, errors.New("insufficient permissions to update organization")
	}

	// 2. Validate request
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) < 2 || len(req.Name) > 100 {
		return nil, ErrInvalidName
	}

	// 3. Fetch existing org
	b, err := s.repo.FindByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization: Update: find org: %w", err)
	}
	if b == nil {
		return nil, ErrNotFound
	}

	// 4. Update fields
	b.Name = req.Name
	b.LegalName = strings.TrimSpace(req.LegalName)
	b.Type = strings.TrimSpace(req.Type)
	b.Industry = strings.TrimSpace(req.Industry)
	b.Website = strings.TrimSpace(req.Website)
	b.LogoURL = strings.TrimSpace(req.LogoURL)
	b.Country = strings.TrimSpace(req.Country)
	b.Timezone = strings.TrimSpace(req.Timezone)
	b.Currency = strings.TrimSpace(req.Currency)

	// 5. Save changes
	if err := s.repo.Update(ctx, b); err != nil {
		return nil, fmt.Errorf("organization: Update: %w", err)
	}
	return b, nil
}

func (s *serviceImpl) ListForUser(ctx context.Context, userID string) ([]*MembershipWithRole, error) {
	results, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("organization: ListForUser: %w", err)
	}
	if results == nil {
		results = []*MembershipWithRole{}
	}
	return results, nil
}

func (s *serviceImpl) FindAllIDs(ctx context.Context) ([]string, error) {
	return s.repo.FindAllIDs(ctx)
}

func (s *serviceImpl) Switch(ctx context.Context, businessID, userID string) (string, string, error) {
	b, err := s.repo.FindByID(ctx, businessID)
	if err != nil {
		return "", "", fmt.Errorf("organization: Switch: find organization: %w", err)
	}
	if b == nil {
		return "", "", ErrNotFound
	}
	membership, err := s.authzRepo.GetMembership(ctx, userID, businessID)
	if err != nil {
		return "", "", fmt.Errorf("organization: Switch: get membership: %w", err)
	}
	if membership == nil || membership.Status != "active" || membership.InvitationStatus != "accepted" {
		return "", "", ErrNotMember
	}
	accessToken, err := s.jwtManager.IssueAccessToken(userID, "", businessID, membership.RoleKey)
	if err != nil {
		return "", "", fmt.Errorf("organization: Switch: issue token: %w", err)
	}
	slog.Info("organization: context switched",
		slog.String("user_id", userID),
		slog.String("organization_id", businessID),
		slog.String("role", membership.RoleKey),
	)
	return accessToken, membership.RoleKey, nil
}

var (
	ErrNotFound    = errors.New("organization not found")
	ErrSlugTaken   = errors.New("organization slug already taken")
	ErrNotMember   = errors.New("user is not a member of this organization")
	ErrInvalidSlug = errors.New("slug must be lowercase letters, digits, and hyphens only")
	ErrInvalidName = errors.New("name must be between 2 and 100 characters")
)

func validateCreateRequest(req CreateBusinessRequest) error {
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 100 {
		return ErrInvalidName
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if len(slug) < 2 || len(slug) > 50 {
		return fmt.Errorf("slug must be between 2 and 50 characters")
	}
	if !slugPattern.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}

// defaultEmployeeStatuses mirrors migration 00053 exactly — same names, same
// categories, same colors — so an org created here is indistinguishable from
// one the migration seeded.
//
// 'Resigned' and 'Terminated' deliberately share the 'terminated' category:
// they are different names for one lifecycle state, and HRM code filters on
// CATEGORY, never on name. Names are org-customisable; categories are
// CHECK-constrained, which is why payroll's eligible-employee filter reads the
// category and would silently unpay people if these were miscategorised.
var defaultEmployeeStatuses = []struct{ Name, Category, Color string }{
	{"Active", "active", "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"},
	{"Inactive", "inactive", "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"},
	{"On Leave", "on_leave", "bg-amber-500/10 text-amber-400 border-amber-500/20"},
	{"Resigned", "terminated", "bg-orange-500/10 text-orange-400 border-orange-500/20"},
	{"Terminated", "terminated", "bg-red-500/10 text-red-400 border-red-500/20"},
}

// seedEmployeeStatusesTx inserts the default statuses for a new organization
// within the caller's transaction.
func seedEmployeeStatusesTx(ctx context.Context, tx pgx.Tx, orgID string) error {
	for _, st := range defaultEmployeeStatuses {
		if _, err := tx.Exec(ctx,
			`INSERT INTO hrm_employee_statuses (org_id, name, category, color)
			 VALUES ($1::uuid, $2, $3, $4)`,
			orgID, st.Name, st.Category, st.Color); err != nil {
			return fmt.Errorf("insert %q: %w", st.Name, err)
		}
	}
	return nil
}
