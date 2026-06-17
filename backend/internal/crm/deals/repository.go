// backend/internal/crm/deals/repository.go
package deals

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for CRM deals.
type Repository interface {
	FindDeals(ctx context.Context, orgID string) ([]*Deal, error)
	FindDealByID(ctx context.Context, orgID, dealID string) (*Deal, error)
	FindDealsByStage(ctx context.Context, orgID, stageID string) ([]*Deal, error)
	FindDealsByContact(ctx context.Context, orgID, contactID string) ([]*Deal, error)
	FindDealsByCompany(ctx context.Context, orgID, companyID string) ([]*Deal, error)
	FindRecentDeals(ctx context.Context, orgID string, limit int) ([]*Deal, error)
	CreateDeal(ctx context.Context, d *Deal) error
	// CreateDealTx inserts a deal inside an existing transaction.
	// Used by CreateDealFromLeadTx so the deal, contact, and lead-status
	// update are all committed or rolled back together.
	CreateDealTx(ctx context.Context, tx pgx.Tx, d *Deal) error
	UpdateDeal(ctx context.Context, d *Deal) error
	SoftDeleteDeal(ctx context.Context, orgID, dealID string) error
	CountDeals(ctx context.Context, orgID string) (int, error)
	GetDealsByStage(ctx context.Context, orgID string) ([]*DealsByStage, error)
	GetDealsByOwner(ctx context.Context, orgID string) ([]*DealsByOwner, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new deals repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

const dealCols = `
	id, public_id, org_id, title, value, currency, pipeline_id, stage_id,
	contact_id, company_id, status, close_date, lost_reason, owner_id,
	won_at, lost_at, created_by, created_at, updated_at`

func scanDeal(row interface{ Scan(...any) error }, d *Deal) error {
	return row.Scan(
		&d.ID, &d.PublicID, &d.OrgID, &d.Title, &d.Value, &d.Currency,
		&d.PipelineID, &d.StageID, &d.ContactID, &d.CompanyID,
		&d.Status, &d.CloseDate, &d.LostReason, &d.OwnerID,
		&d.WonAt, &d.LostAt, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)
}

func (r *repoImpl) FindDeals(ctx context.Context, orgID string) ([]*Deal, error) {
	q := `SELECT ` + dealCols + `
		FROM crm_deals WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("deals: FindDeals: %w", err)
	}
	defer rows.Close()

	var out []*Deal
	for rows.Next() {
		d := &Deal{}
		if err := scanDeal(rows, d); err != nil {
			return nil, fmt.Errorf("deals: FindDeals: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindDealByID(ctx context.Context, orgID, dealID string) (*Deal, error) {
	q := `SELECT ` + dealCols + `
		FROM crm_deals WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`

	d := &Deal{}
	err := scanDeal(r.db.QueryRow(ctx, q, orgID, dealID), d)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deals: FindDealByID: %w", err)
	}
	return d, nil
}

func (r *repoImpl) FindDealsByStage(ctx context.Context, orgID, stageID string) ([]*Deal, error) {
	q := `SELECT ` + dealCols + `
		FROM crm_deals WHERE org_id = $1 AND stage_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, stageID)
	if err != nil {
		return nil, fmt.Errorf("deals: FindDealsByStage: %w", err)
	}
	defer rows.Close()

	var out []*Deal
	for rows.Next() {
		d := &Deal{}
		if err := scanDeal(rows, d); err != nil {
			return nil, fmt.Errorf("deals: FindDealsByStage: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindDealsByContact(ctx context.Context, orgID, contactID string) ([]*Deal, error) {
	q := `SELECT ` + dealCols + `
		FROM crm_deals WHERE org_id = $1 AND contact_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, contactID)
	if err != nil {
		return nil, fmt.Errorf("deals: FindDealsByContact: %w", err)
	}
	defer rows.Close()

	var out []*Deal
	for rows.Next() {
		d := &Deal{}
		if err := scanDeal(rows, d); err != nil {
			return nil, fmt.Errorf("deals: FindDealsByContact: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindDealsByCompany(ctx context.Context, orgID, companyID string) ([]*Deal, error) {
	q := `SELECT ` + dealCols + `
		FROM crm_deals WHERE org_id = $1 AND company_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, companyID)
	if err != nil {
		return nil, fmt.Errorf("deals: FindDealsByCompany: %w", err)
	}
	defer rows.Close()

	var out []*Deal
	for rows.Next() {
		d := &Deal{}
		if err := scanDeal(rows, d); err != nil {
			return nil, fmt.Errorf("deals: FindDealsByCompany: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindRecentDeals(ctx context.Context, orgID string, limit int) ([]*Deal, error) {
	q := `SELECT ` + dealCols + `
		FROM crm_deals WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $2`

	rows, err := r.db.Query(ctx, q, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("deals: FindRecentDeals: %w", err)
	}
	defer rows.Close()

	var out []*Deal
	for rows.Next() {
		d := &Deal{}
		if err := scanDeal(rows, d); err != nil {
			return nil, fmt.Errorf("deals: FindRecentDeals: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

const insertDealSQL = `
	INSERT INTO crm_deals
	    (org_id, title, value, currency, pipeline_id, stage_id,
	     contact_id, company_id, status, close_date, owner_id, created_by)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	RETURNING id, public_id, created_at, updated_at`

func (r *repoImpl) CreateDeal(ctx context.Context, d *Deal) error {
	return r.db.QueryRow(ctx, insertDealSQL,
		d.OrgID, d.Title, d.Value, d.Currency, d.PipelineID, d.StageID,
		d.ContactID, d.CompanyID, d.Status, d.CloseDate, d.OwnerID, d.CreatedBy,
	).Scan(&d.ID, &d.PublicID, &d.CreatedAt, &d.UpdatedAt)
}

// CreateDealTx inserts a deal within an existing pgx.Tx.
// The caller owns the transaction and is responsible for Commit/Rollback.
func (r *repoImpl) CreateDealTx(ctx context.Context, tx pgx.Tx, d *Deal) error {
	return tx.QueryRow(ctx, insertDealSQL,
		d.OrgID, d.Title, d.Value, d.Currency, d.PipelineID, d.StageID,
		d.ContactID, d.CompanyID, d.Status, d.CloseDate, d.OwnerID, d.CreatedBy,
	).Scan(&d.ID, &d.PublicID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *repoImpl) UpdateDeal(ctx context.Context, d *Deal) error {
	const q = `
		UPDATE crm_deals
		SET title = $1, value = $2, currency = $3, pipeline_id = $4, stage_id = $5,
		    contact_id = $6, company_id = $7, status = $8, close_date = $9,
		    lost_reason = $10, owner_id = $11, won_at = $12, lost_at = $13,
		    updated_at = NOW()
		WHERE org_id = $14 AND id = $15 AND deleted_at IS NULL
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		d.Title, d.Value, d.Currency, d.PipelineID, d.StageID,
		d.ContactID, d.CompanyID, d.Status, d.CloseDate,
		d.LostReason, d.OwnerID, d.WonAt, d.LostAt,
		d.OrgID, d.ID,
	).Scan(&d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDealNotFound
	}
	return err
}

func (r *repoImpl) SoftDeleteDeal(ctx context.Context, orgID, dealID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE crm_deals SET deleted_at = NOW() WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, dealID,
	)
	if err != nil {
		return fmt.Errorf("deals: SoftDeleteDeal: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrDealNotFound
	}
	return nil
}

func (r *repoImpl) CountDeals(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM crm_deals WHERE org_id = $1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&n)
	return n, err
}

func (r *repoImpl) GetDealsByStage(ctx context.Context, orgID string) ([]*DealsByStage, error) {
	const q = `
		SELECT s.id, s.name, COUNT(d.id), COALESCE(SUM(d.value), 0)
		FROM crm_pipeline_stages s
		LEFT JOIN crm_deals d
		    ON d.stage_id = s.id AND d.deleted_at IS NULL AND d.status = 'open'
		WHERE s.org_id = $1
		GROUP BY s.id, s.name, s.position
		ORDER BY s.position ASC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetDealsByStage: %w", err)
	}
	defer rows.Close()

	var out []*DealsByStage
	for rows.Next() {
		d := &DealsByStage{}
		if err := rows.Scan(&d.StageID, &d.StageName, &d.Count, &d.TotalValue); err != nil {
			return nil, fmt.Errorf("deals: GetDealsByStage: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *repoImpl) GetDealsByOwner(ctx context.Context, orgID string) ([]*DealsByOwner, error) {
	const q = `
		SELECT d.owner_id,
		       COALESCE(u.first_name || ' ' || COALESCE(u.last_name, ''), 'Unassigned') AS owner_name,
		       COUNT(d.id), COALESCE(SUM(d.value), 0)
		FROM crm_deals d
		LEFT JOIN users u ON u.id = d.owner_id
		WHERE d.org_id = $1 AND d.deleted_at IS NULL AND d.status = 'open'
		GROUP BY d.owner_id, owner_name
		ORDER BY COUNT(d.id) DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetDealsByOwner: %w", err)
	}
	defer rows.Close()

	var out []*DealsByOwner
	for rows.Next() {
		d := &DealsByOwner{}
		if err := rows.Scan(&d.OwnerID, &d.OwnerName, &d.Count, &d.TotalValue); err != nil {
			return nil, fmt.Errorf("deals: GetDealsByOwner: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
