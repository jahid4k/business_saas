// backend/internal/business/service.go
// Package name is kept as business for backward compatibility. It now manages organizations.
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
	Switch(ctx context.Context, businessID, userID string) (accessToken string, role string, err error)
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
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	b := &Business{
		Name: req.Name, Slug: req.Slug, LegalName: strings.TrimSpace(req.LegalName),
		Type: strings.TrimSpace(req.Type), Industry: strings.TrimSpace(req.Industry), Website: strings.TrimSpace(req.Website), LogoURL: strings.TrimSpace(req.LogoURL),
		Country: strings.TrimSpace(req.Country), Timezone: strings.TrimSpace(req.Timezone), Currency: strings.TrimSpace(req.Currency),
	}
	if err := s.repo.CreateTx(ctx, tx, b); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("organization: Create: insert organization: %w", err)
	}

	roleID := ownerRole.ID
	membership := &authz.Membership{UserID: ownerID, OrganizationID: b.ID, RoleID: &roleID, RoleKey: authz.RoleOwner, Status: "active", InvitationStatus: "accepted"}
	if err := s.authzRepo.CreateMembershipTx(ctx, tx, membership); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("organization: Create: insert owner membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("organization: Create: commit: %w", err)
	}
	slog.Info("organization: created", slog.String("organization_id", b.ID), slog.String("owner_id", ownerID), slog.String("slug", b.Slug))
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
	slog.Info("organization: context switched", slog.String("user_id", userID), slog.String("organization_id", businessID), slog.String("role", membership.RoleKey))
	return accessToken, membership.RoleKey, nil
}

var ErrNotFound = errors.New("organization not found")
var ErrSlugTaken = errors.New("organization slug already taken")
var ErrNotMember = errors.New("user is not a member of this organization")
var ErrInvalidSlug = errors.New("slug must be lowercase letters, digits, and hyphens only")
var ErrInvalidName = errors.New("name must be between 2 and 100 characters")

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

func isTxError(err error) bool {
	return errors.Is(err, pgx.ErrTxClosed) || errors.Is(err, pgx.ErrTxCommitRollback)
}

var _ = isTxError
