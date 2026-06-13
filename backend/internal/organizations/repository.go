// backend/internal/business/repository.go
package organizations

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateTx(ctx context.Context, tx pgx.Tx, b *Business) error
	FindByID(ctx context.Context, businessID string) (*Business, error)
	FindBySlug(ctx context.Context, slug string) (*Business, error)
	FindByUserID(ctx context.Context, userID string) ([]*MembershipWithRole, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

func (r *repoImpl) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("organization: BeginTx: %w", err)
	}
	return tx, nil
}

func (r *repoImpl) CreateTx(ctx context.Context, tx pgx.Tx, b *Business) error {
	const q = `
		INSERT INTO organizations (name, slug, legal_name, type, industry, website, logo_url, country, timezone, currency, status)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), COALESCE(NULLIF($9, ''), 'UTC'), COALESCE(NULLIF($10, ''), 'USD'), 'active')
		RETURNING id, public_id, created_at, updated_at`
	err := tx.QueryRow(ctx, q, b.Name, b.Slug, b.LegalName, b.Type, b.Industry, b.Website, b.LogoURL, b.Country, b.Timezone, b.Currency).
		Scan(&b.ID, &b.PublicID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return fmt.Errorf("organization: CreateTx: %w", err)
	}
	b.Status = "active"
	return nil
}

const orgSelect = `id, public_id, name, slug, COALESCE(legal_name, ''), COALESCE(type, ''), COALESCE(industry, ''), COALESCE(website, ''), COALESCE(logo_url, ''), COALESCE(country, ''), timezone, currency, status, created_at, updated_at, deleted_at`

func scanOrg(row pgx.Row) (*Business, error) {
	b := &Business{}
	err := row.Scan(&b.ID, &b.PublicID, &b.Name, &b.Slug, &b.LegalName, &b.Type, &b.Industry, &b.Website, &b.LogoURL, &b.Country, &b.Timezone, &b.Currency, &b.Status, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *repoImpl) FindByID(ctx context.Context, businessID string) (*Business, error) {
	q := `SELECT ` + orgSelect + ` FROM organizations WHERE id = $1 AND deleted_at IS NULL`
	b, err := scanOrg(r.db.QueryRow(ctx, q, businessID))
	if err != nil {
		return nil, fmt.Errorf("organization: FindByID: %w", err)
	}
	return b, nil
}

func (r *repoImpl) FindBySlug(ctx context.Context, slug string) (*Business, error) {
	q := `SELECT ` + orgSelect + ` FROM organizations WHERE LOWER(slug) = LOWER($1) AND deleted_at IS NULL`
	b, err := scanOrg(r.db.QueryRow(ctx, q, slug))
	if err != nil {
		return nil, fmt.Errorf("organization: FindBySlug: %w", err)
	}
	return b, nil
}

func (r *repoImpl) FindByUserID(ctx context.Context, userID string) ([]*MembershipWithRole, error) {
	const q = `
		SELECT
			o.id, o.public_id, o.name, o.slug, COALESCE(o.legal_name, ''), COALESCE(o.type, ''), COALESCE(o.industry, ''), COALESCE(o.website, ''), COALESCE(o.logo_url, ''), COALESCE(o.country, ''), o.timezone, o.currency, o.status, o.created_at, o.updated_at, o.deleted_at,
			om.role_key, om.id
		FROM organization_members om
		JOIN organizations o ON o.id = om.org_id
		WHERE om.user_id = $1
		  AND om.status = 'active'
		  AND om.invitation_status = 'accepted'
		  AND o.status = 'active'
		  AND o.deleted_at IS NULL
		ORDER BY o.name ASC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("organization: FindByUserID: %w", err)
	}
	defer rows.Close()
	var results []*MembershipWithRole
	for rows.Next() {
		b := &Business{}
		mwr := &MembershipWithRole{Business: b}
		if err := rows.Scan(&b.ID, &b.PublicID, &b.Name, &b.Slug, &b.LegalName, &b.Type, &b.Industry, &b.Website, &b.LogoURL, &b.Country, &b.Timezone, &b.Currency, &b.Status, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt, &mwr.Role, &mwr.MemberID); err != nil {
			return nil, fmt.Errorf("organization: FindByUserID: scan: %w", err)
		}
		results = append(results, mwr)
	}
	return results, rows.Err()
}
