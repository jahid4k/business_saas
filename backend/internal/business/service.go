// backend/internal/business/service.go
package business

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

// slugPattern enforces URL-safe slugs: lowercase letters, digits, hyphens only.
// No leading/trailing hyphens, no consecutive hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Service defines the business logic interface for workspace management.
type Service interface {
	// Create creates a new business and auto-assigns the caller as Owner.
	// Runs inside a transaction: business insert + membership insert are atomic.
	Create(ctx context.Context, ownerID string, req CreateBusinessRequest) (*Business, error)

	// GetByID returns a business if the requesting user is a member.
	// Returns ErrNotFound for non-existent businesses.
	// Returns ErrNotMember if the user has no membership in this business.
	GetByID(ctx context.Context, businessID, requestingUserID string) (*Business, error)

	// ListForUser returns all active businesses the user belongs to,
	// with the user's role in each.
	ListForUser(ctx context.Context, userID string) ([]*MembershipWithRole, error)

	// Switch verifies the user is a member of the business and returns
	// a new JWT access token with business_id and role embedded.
	// The client replaces their current access token with this new one.
	Switch(ctx context.Context, businessID, userID string) (accessToken string, role string, err error)
}

type serviceImpl struct {
	repo       Repository
	authzRepo  authz.Repository
	jwtManager *jwtpkg.Manager
}

// NewService creates a fully wired business service.
func NewService(
	repo Repository,
	authzRepo authz.Repository,
	jwtManager *jwtpkg.Manager,
) Service {
	return &serviceImpl{
		repo:       repo,
		authzRepo:  authzRepo,
		jwtManager: jwtManager,
	}
}

// ----------------------------------------------------------
// Create
// ----------------------------------------------------------

// Create creates a new business workspace and assigns the creator as Owner.
//
// Flow (single transaction):
//  1. Validate name and slug
//  2. Check slug uniqueness
//  3. BEGIN transaction
//  4. INSERT business
//  5. Look up the "owner" role ID
//  6. INSERT membership (user → business → owner role)
//  7. COMMIT
func (s *serviceImpl) Create(ctx context.Context, ownerID string, req CreateBusinessRequest) (*Business, error) {
	// Validate inputs
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Normalise slug to lowercase
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	req.Name = strings.TrimSpace(req.Name)

	// Slug uniqueness check before opening transaction
	existing, err := s.repo.FindBySlug(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("business: Create: slug check: %w", err)
	}
	if existing != nil {
		return nil, ErrSlugTaken
	}

	// Look up owner role
	ownerRole, err := s.authzRepo.GetRoleByName(ctx, authz.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("business: Create: get owner role: %w", err)
	}
	if ownerRole == nil {
		return nil, fmt.Errorf("business: Create: owner role not seeded in database")
	}

	// Start transaction
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("business: Create: begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	// Insert business
	b := &Business{
		Name:     req.Name,
		Slug:     req.Slug,
		OwnerID:  ownerID,
		IsActive: true,
	}
	if err := s.repo.CreateTx(ctx, tx, b); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("business: Create: insert business: %w", err)
	}

	// Insert owner membership
	membership := &authz.Membership{
		UserID:     ownerID,
		BusinessID: b.ID,
		RoleID:     ownerRole.ID,
	}
	if err := s.authzRepo.CreateMembershipTx(ctx, tx, membership); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("business: Create: insert membership: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("business: Create: commit: %w", err)
	}

	slog.Info("business: workspace created",
		slog.String("business_id", b.ID),
		slog.String("owner_id", ownerID),
		slog.String("slug", b.Slug),
	)

	return b, nil
}

// ----------------------------------------------------------
// GetByID
// ----------------------------------------------------------

func (s *serviceImpl) GetByID(ctx context.Context, businessID, requestingUserID string) (*Business, error) {
	b, err := s.repo.FindByID(ctx, businessID)
	if err != nil {
		return nil, fmt.Errorf("business: GetByID: %w", err)
	}
	if b == nil {
		return nil, ErrNotFound
	}

	// Verify the requesting user is a member of this business
	membership, err := s.authzRepo.GetMembership(ctx, requestingUserID, businessID)
	if err != nil {
		return nil, fmt.Errorf("business: GetByID: membership check: %w", err)
	}
	if membership == nil {
		return nil, ErrNotMember
	}

	return b, nil
}

// ----------------------------------------------------------
// ListForUser
// ----------------------------------------------------------

func (s *serviceImpl) ListForUser(ctx context.Context, userID string) ([]*MembershipWithRole, error) {
	results, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("business: ListForUser: %w", err)
	}
	// Always return a slice, never nil — makes JSON encode as [] not null
	if results == nil {
		results = []*MembershipWithRole{}
	}
	return results, nil
}

// ----------------------------------------------------------
// Switch
// ----------------------------------------------------------

// Switch verifies membership and issues a new JWT with business_id + role embedded.
//
// Flow:
//  1. Look up membership for (userID, businessID)
//  2. If no membership → ErrNotMember
//  3. Look up role name from the membership's role_id
//  4. Issue new access token with business_id and role embedded
//
// The client MUST replace their current access token with this new one.
// All subsequent requests to business-scoped endpoints must use this token.
func (s *serviceImpl) Switch(ctx context.Context, businessID, userID string) (string, string, error) {
	// Verify business exists
	b, err := s.repo.FindByID(ctx, businessID)
	if err != nil {
		return "", "", fmt.Errorf("business: Switch: find business: %w", err)
	}
	if b == nil {
		return "", "", ErrNotFound
	}

	// Verify membership
	membership, err := s.authzRepo.GetMembership(ctx, userID, businessID)
	if err != nil {
		return "", "", fmt.Errorf("business: Switch: get membership: %w", err)
	}
	if membership == nil {
		return "", "", ErrNotMember
	}

	// Get role name
	role, err := s.authzRepo.GetRoleByID(ctx, membership.RoleID)
	if err != nil {
		return "", "", fmt.Errorf("business: Switch: get role: %w", err)
	}
	if role == nil {
		return "", "", fmt.Errorf("business: Switch: role not found for membership")
	}

	// Issue new JWT with business context embedded
	// The user's email is not re-fetched here — the existing claims in c.Locals("email")
	// are passed in by the handler.
	accessToken, err := s.jwtManager.IssueAccessToken(userID, "", businessID, role.Name)
	if err != nil {
		return "", "", fmt.Errorf("business: Switch: issue token: %w", err)
	}

	slog.Info("business: context switched",
		slog.String("user_id", userID),
		slog.String("business_id", businessID),
		slog.String("role", role.Name),
	)

	return accessToken, role.Name, nil
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

var ErrNotFound = errors.New("business not found")
var ErrSlugTaken = errors.New("business slug already taken")
var ErrNotMember = errors.New("user is not a member of this business")
var ErrInvalidSlug = errors.New("slug must be lowercase letters, digits, and hyphens only")
var ErrInvalidName = errors.New("name must be between 2 and 100 characters")

// ----------------------------------------------------------
// Validation
// ----------------------------------------------------------

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

// isTxError is a helper to check if the error is a pgx transaction error.
// Used in deferred rollback to avoid double-rollback log noise.
func isTxError(err error) bool {
	return errors.Is(err, pgx.ErrTxClosed) ||
		errors.Is(err, pgx.ErrTxCommitRollback)
}

var _ = isTxError // suppress unused warning — used in future phases
