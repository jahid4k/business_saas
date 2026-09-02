package email

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	// GetRouteByAddress resolves an inbound address to its org AND its
	// destination. Replaces the org-only lookup that was sufficient while
	// lead creation was the pipeline's only consumer.
	GetRouteByAddress(ctx context.Context, address string) (*InboundRoute, error)
	CreateLog(ctx context.Context, log *InboundEmailLog) error

	// FindEmployeeRequester resolves a sender's email address to the
	// employee who can own a ticket raised from it. Lives here rather than
	// in platform/tickets, which must never reference hrm_* — the 7D
	// benefits.FindEmployeeIDByUserID precedent.
	FindEmployeeRequester(ctx context.Context, orgID, email string) (*EmployeeRequester, error)

	// Settings
	ListOrgEmails(ctx context.Context, orgID string) ([]*OrgInboundEmail, error)
	CreateOrgEmail(ctx context.Context, orgID string, address string, destination Destination) (*OrgInboundEmail, error)
	DeleteOrgEmail(ctx context.Context, orgID string, id string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) GetRouteByAddress(ctx context.Context, address string) (*InboundRoute, error) {
	var route InboundRoute
	err := r.db.QueryRow(ctx, `
		SELECT org_id, destination FROM org_inbound_emails
		WHERE address = $1 AND is_active = true
	`, address).Scan(&route.OrgID, &route.Destination)
	if err == pgx.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *repoImpl) FindEmployeeRequester(ctx context.Context, orgID, email string) (*EmployeeRequester, error) {
	// An employee record linked to a platform user whose login address is
	// the sender. Both halves are required: platform_tickets.requester_id
	// and .requester_user_id are each NOT NULL, and inventing either would
	// attach a ticket to somebody who did not raise it.
	var req EmployeeRequester
	err := r.db.QueryRow(ctx, `
		SELECT e.user_id::text, e.id::text
		FROM hrm_employees e
		JOIN users u ON u.id = e.user_id
		WHERE e.org_id = $1 AND LOWER(u.email) = LOWER($2)
		LIMIT 1
	`, orgID, email).Scan(&req.UserID, &req.EmployeeID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
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
		SELECT id, org_id, address, destination, is_active, created_at
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
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Address, &e.Destination, &e.IsActive, &e.CreatedAt); err != nil {
			return nil, err
		}
		emails = append(emails, &e)
	}
	return emails, rows.Err()
}

func (r *repoImpl) CreateOrgEmail(ctx context.Context, orgID string, address string, destination Destination) (*OrgInboundEmail, error) {
	const q = `
		INSERT INTO org_inbound_emails (org_id, address, destination, is_active)
		VALUES ($1, $2, $3, true)
		RETURNING id, org_id, address, destination, is_active, created_at
	`
	var e OrgInboundEmail
	err := r.db.QueryRow(ctx, q, orgID, address, destination).
		Scan(&e.ID, &e.OrgID, &e.Address, &e.Destination, &e.IsActive, &e.CreatedAt)
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
