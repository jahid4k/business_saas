package social

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetOrgByPageID(ctx context.Context, platform, pageID string) (string, error)
	CreateLog(ctx context.Context, log *SocialLeadLog) error

	ListOrgSocials(ctx context.Context, orgID string) ([]*SocialIntegration, error)
	CreateOrgSocial(ctx context.Context, orgID, platform, pageID string) (*SocialIntegration, error)
	DeleteOrgSocial(ctx context.Context, orgID, id string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) GetOrgByPageID(ctx context.Context, platform, pageID string) (string, error) {
	var orgID string
	err := r.db.QueryRow(ctx, `
		SELECT org_id FROM social_integrations
		WHERE platform = $1 AND page_id = $2 AND is_active = true
		LIMIT 1
	`, platform, pageID).Scan(&orgID)
	if err == pgx.ErrNoRows {
		return "", nil // Not found
	}
	return orgID, err
}

func (r *repoImpl) CreateLog(ctx context.Context, log *SocialLeadLog) error {
	const q = `
		INSERT INTO social_lead_logs
		(org_id, platform, page_id, raw_payload, processed, error_message)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, q,
		log.OrgID, log.Platform, log.PageID, log.RawPayload, log.Processed, log.ErrorMessage,
	).Scan(&log.ID, &log.CreatedAt)
}

func (r *repoImpl) ListOrgSocials(ctx context.Context, orgID string) ([]*SocialIntegration, error) {
	const q = `
		SELECT id, org_id, platform, page_id, is_active, created_at
		FROM social_integrations
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*SocialIntegration
	for rows.Next() {
		var s SocialIntegration
		if err := rows.Scan(&s.ID, &s.OrgID, &s.Platform, &s.PageID, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &s)
	}
	return list, rows.Err()
}

func (r *repoImpl) CreateOrgSocial(ctx context.Context, orgID, platform, pageID string) (*SocialIntegration, error) {
	const q = `
		INSERT INTO social_integrations (org_id, platform, page_id, access_token_enc, is_active)
		VALUES ($1, $2, $3, '', true)
		RETURNING id, org_id, platform, page_id, is_active, created_at
	`
	var s SocialIntegration
	err := r.db.QueryRow(ctx, q, orgID, platform, pageID).Scan(&s.ID, &s.OrgID, &s.Platform, &s.PageID, &s.IsActive, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *repoImpl) DeleteOrgSocial(ctx context.Context, orgID, id string) error {
	const q = `DELETE FROM social_integrations WHERE org_id = $1 AND id = $2`
	_, err := r.db.Exec(ctx, q, orgID, id)
	return err
}
