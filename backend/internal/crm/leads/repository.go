// backend/internal/crm/leads/repository.go
package leads

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for CRM leads.
type Repository interface {
	FindLeads(ctx context.Context, orgID string) ([]*Lead, error)
	FindLeadByID(ctx context.Context, orgID, leadID string) (*Lead, error)
	CreateLead(ctx context.Context, l *Lead) error
	UpdateLead(ctx context.Context, l *Lead) error
	SoftDeleteLead(ctx context.Context, orgID, leadID string) error
	CountLeads(ctx context.Context, orgID string) (int, error)
	GetLeadsBySource(ctx context.Context, orgID string) ([]*LeadsBySource, error)
	// BeginTx opens a new database transaction for use by callers that need
	// atomic multi-step operations (e.g. lead conversion).
	BeginTx(ctx context.Context) (pgx.Tx, error)
	// UpdateLeadTx writes the final converted state inside an existing transaction.
	UpdateLeadTx(ctx context.Context, tx pgx.Tx, l *Lead) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new leads repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

const leadCols = `
	id, public_id, org_id, first_name, last_name, email, phone,
	company_name, title, source, status, converted_at,
	converted_contact_id, converted_deal_id, owner_id,
	created_by, created_at, updated_at`

func scanLead(row interface{ Scan(...any) error }, l *Lead) error {
	return row.Scan(
		&l.ID, &l.PublicID, &l.OrgID, &l.FirstName, &l.LastName, &l.Email,
		&l.Phone, &l.CompanyName, &l.Title, &l.Source, &l.Status,
		&l.ConvertedAt, &l.ConvertedContactID, &l.ConvertedDealID,
		&l.OwnerID, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt,
	)
}

func (r *repoImpl) FindLeads(ctx context.Context, orgID string) ([]*Lead, error) {
	q := `SELECT ` + leadCols + `
		FROM crm_leads WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("leads: FindLeads: %w", err)
	}
	defer rows.Close()

	var out []*Lead
	for rows.Next() {
		l := &Lead{}
		if err := scanLead(rows, l); err != nil {
			return nil, fmt.Errorf("leads: FindLeads: scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindLeadByID(ctx context.Context, orgID, leadID string) (*Lead, error) {
	q := `SELECT ` + leadCols + `
		FROM crm_leads WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`

	l := &Lead{}
	err := scanLead(r.db.QueryRow(ctx, q, orgID, leadID), l)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leads: FindLeadByID: %w", err)
	}
	return l, nil
}

func (r *repoImpl) CreateLead(ctx context.Context, l *Lead) error {
	const q = `
		INSERT INTO crm_leads
		    (org_id, first_name, last_name, email, phone, company_name,
		     title, source, status, owner_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		l.OrgID, l.FirstName, l.LastName, l.Email, l.Phone,
		l.CompanyName, l.Title, l.Source, l.Status, l.OwnerID, l.CreatedBy,
	).Scan(&l.ID, &l.PublicID, &l.CreatedAt, &l.UpdatedAt)
}

const updateLeadSQL = `
	UPDATE crm_leads
	SET first_name = $1, last_name = $2, email = $3, phone = $4,
	    company_name = $5, title = $6, source = $7, status = $8,
	    owner_id = $9, converted_at = $10,
	    converted_contact_id = $11, converted_deal_id = $12,
	    updated_at = NOW()
	WHERE org_id = $13 AND id = $14 AND deleted_at IS NULL
	RETURNING updated_at`

func (r *repoImpl) UpdateLead(ctx context.Context, l *Lead) error {
	err := r.db.QueryRow(ctx, updateLeadSQL,
		l.FirstName, l.LastName, l.Email, l.Phone,
		l.CompanyName, l.Title, l.Source, l.Status, l.OwnerID,
		l.ConvertedAt, l.ConvertedContactID, l.ConvertedDealID,
		l.OrgID, l.ID,
	).Scan(&l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeadNotFound
	}
	return err
}

// BeginTx opens a new pgx transaction.
func (r *repoImpl) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("leads: BeginTx: %w", err)
	}
	return tx, nil
}

// UpdateLeadTx writes the lead's converted state inside the supplied transaction.
func (r *repoImpl) UpdateLeadTx(ctx context.Context, tx pgx.Tx, l *Lead) error {
	err := tx.QueryRow(ctx, updateLeadSQL,
		l.FirstName, l.LastName, l.Email, l.Phone,
		l.CompanyName, l.Title, l.Source, l.Status, l.OwnerID,
		l.ConvertedAt, l.ConvertedContactID, l.ConvertedDealID,
		l.OrgID, l.ID,
	).Scan(&l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeadNotFound
	}
	return err
}

func (r *repoImpl) SoftDeleteLead(ctx context.Context, orgID, leadID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE crm_leads SET deleted_at = NOW() WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, leadID,
	)
	if err != nil {
		return fmt.Errorf("leads: SoftDeleteLead: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrLeadNotFound
	}
	return nil
}

func (r *repoImpl) CountLeads(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM crm_leads WHERE org_id = $1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&n)
	return n, err
}

func (r *repoImpl) GetLeadsBySource(ctx context.Context, orgID string) ([]*LeadsBySource, error) {
	const q = `
		SELECT COALESCE(source, 'unknown') AS source, COUNT(*) AS count
		FROM crm_leads
		WHERE org_id = $1 AND deleted_at IS NULL
		GROUP BY source ORDER BY count DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("leads: GetLeadsBySource: %w", err)
	}
	defer rows.Close()

	var out []*LeadsBySource
	for rows.Next() {
		l := &LeadsBySource{}
		if err := rows.Scan(&l.Source, &l.Count); err != nil {
			return nil, fmt.Errorf("leads: GetLeadsBySource: scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
