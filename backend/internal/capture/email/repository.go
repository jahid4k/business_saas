package email

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetOrgByAddress(ctx context.Context, address string) (string, error)
	CreateLog(ctx context.Context, log *InboundEmailLog) error

	// Settings
	ListOrgEmails(ctx context.Context, orgID string) ([]*OrgInboundEmail, error)
	CreateOrgEmail(ctx context.Context, orgID string, address string) (*OrgInboundEmail, error)
	DeleteOrgEmail(ctx context.Context, orgID string, id string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) GetOrgByAddress(ctx context.Context, address string) (string, error) {
	var orgID string
	err := r.db.QueryRow(ctx, `
		SELECT org_id FROM org_inbound_emails
		WHERE address = $1 AND is_active = true
	`, address).Scan(&orgID)
	if err == pgx.ErrNoRows {
		return "", nil // Not found
	}
	return orgID, err
}

func (r *repoImpl) CreateLog(ctx context.Context, log *InboundEmailLog) error {
	const q = `
		INSERT INTO inbound_email_logs
		(org_id, to_address, from_address, subject, raw_payload, processed, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, q,
		log.OrgID, log.ToAddress, log.FromAddress, log.Subject, log.RawPayload, log.Processed, log.ErrorMessage,
	).Scan(&log.ID, &log.CreatedAt)
}

func (r *repoImpl) ListOrgEmails(ctx context.Context, orgID string) ([]*OrgInboundEmail, error) {
	const q = `
		SELECT id, org_id, address, is_active, created_at
		FROM org_inbound_emails
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []*OrgInboundEmail
	for rows.Next() {
		var e OrgInboundEmail
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Address, &e.IsActive, &e.CreatedAt); err != nil {
			return nil, err
		}
		emails = append(emails, &e)
	}
	return emails, rows.Err()
}

func (r *repoImpl) CreateOrgEmail(ctx context.Context, orgID string, address string) (*OrgInboundEmail, error) {
	const q = `
		INSERT INTO org_inbound_emails (org_id, address, is_active)
		VALUES ($1, $2, true)
		RETURNING id, org_id, address, is_active, created_at
	`
	var e OrgInboundEmail
	err := r.db.QueryRow(ctx, q, orgID, address).Scan(&e.ID, &e.OrgID, &e.Address, &e.IsActive, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *repoImpl) DeleteOrgEmail(ctx context.Context, orgID string, id string) error {
	const q = `DELETE FROM org_inbound_emails WHERE org_id = $1 AND id = $2`
	_, err := r.db.Exec(ctx, q, orgID, id)
	return err
}
