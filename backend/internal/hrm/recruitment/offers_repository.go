// backend/internal/hrm/recruitment/offers_repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// OfferRepository is embedded into Repository — see repository.go.
type OfferRepository interface {
	FindOffers(ctx context.Context, orgID, applicationID string) ([]*Offer, error)
	FindOfferByRef(ctx context.Context, orgID, ref string) (*Offer, error)
	CreateOffer(ctx context.Context, o *Offer) error
	UpdateOffer(ctx context.Context, o *Offer) error
	SetOfferApprovalInstance(ctx context.Context, id, instanceID string, status OfferStatus) error
	UpdateOfferStatus(ctx context.Context, id string, status OfferStatus) error
}

const offerCols = `id, public_id, org_id, application_id, requisition_id, base_salary, salary_currency,
	signing_bonus, equity_details, start_date, expires_at, status, approval_instance_id, document_id,
	created_by, created_at, updated_at`

func scanOffer(row interface{ Scan(...any) error }, o *Offer) error {
	return row.Scan(
		&o.ID, &o.PublicID, &o.OrgID, &o.ApplicationID, &o.RequisitionID, &o.BaseSalary, &o.SalaryCurrency,
		&o.SigningBonus, &o.EquityDetails, &o.StartDate, &o.ExpiresAt, &o.Status, &o.ApprovalInstanceID, &o.DocumentID,
		&o.CreatedBy, &o.CreatedAt, &o.UpdatedAt,
	)
}

func (r *repoImpl) FindOffers(ctx context.Context, orgID, applicationID string) ([]*Offer, error) {
	q := `SELECT ` + offerCols + ` FROM hrm_offers WHERE org_id = $1 AND application_id = $2 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, orgID, applicationID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindOffers: %w", err)
	}
	defer rows.Close()
	list := make([]*Offer, 0)
	for rows.Next() {
		o := &Offer{}
		if err := scanOffer(rows, o); err != nil {
			return nil, fmt.Errorf("recruitment: FindOffers: scan: %w", err)
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindOfferByRef(ctx context.Context, orgID, ref string) (*Offer, error) {
	q := `SELECT ` + offerCols + ` FROM hrm_offers WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	o := &Offer{}
	err := scanOffer(r.db.QueryRow(ctx, q, orgID, ref), o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindOfferByRef: %w", err)
	}
	return o, nil
}

func (r *repoImpl) CreateOffer(ctx context.Context, o *Offer) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_offers (org_id, application_id, requisition_id, base_salary, salary_currency, signing_bonus, equity_details, start_date, expires_at, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, public_id, status, created_at, updated_at`,
		o.OrgID, o.ApplicationID, o.RequisitionID, o.BaseSalary, o.SalaryCurrency, o.SigningBonus,
		o.EquityDetails, o.StartDate, o.ExpiresAt, o.CreatedBy,
	).Scan(&o.ID, &o.PublicID, &o.Status, &o.CreatedAt, &o.UpdatedAt)
}

func (r *repoImpl) UpdateOffer(ctx context.Context, o *Offer) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_offers SET
		    base_salary = $1, salary_currency = $2, signing_bonus = $3, equity_details = $4,
		    start_date = $5, expires_at = $6, updated_at = NOW()
		 WHERE id = $7 AND org_id = $8
		 RETURNING updated_at`,
		o.BaseSalary, o.SalaryCurrency, o.SigningBonus, o.EquityDetails, o.StartDate, o.ExpiresAt, o.ID, o.OrgID,
	).Scan(&o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOfferNotFound
	}
	return err
}

func (r *repoImpl) SetOfferApprovalInstance(ctx context.Context, id, instanceID string, status OfferStatus) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_offers SET approval_instance_id = $1, status = $2, updated_at = NOW() WHERE id = $3`,
		instanceID, status, id,
	)
	if err != nil {
		return fmt.Errorf("recruitment: SetOfferApprovalInstance: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrOfferNotFound
	}
	return nil
}

func (r *repoImpl) UpdateOfferStatus(ctx context.Context, id string, status OfferStatus) error {
	cmd, err := r.db.Exec(ctx, `UPDATE hrm_offers SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("recruitment: UpdateOfferStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrOfferNotFound
	}
	return nil
}
