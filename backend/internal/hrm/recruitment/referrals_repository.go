// backend/internal/hrm/recruitment/referrals_repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ReferralRepository is embedded into Repository — see repository.go.
type ReferralRepository interface {
	FindReferrals(ctx context.Context, orgID string, filter ReferralListFilter) ([]*Referral, error)
	CountReferrals(ctx context.Context, orgID string, filter ReferralListFilter) (int, error)
	FindReferralByRef(ctx context.Context, orgID, ref string) (*Referral, error)
	CreateReferral(ctx context.Context, r *Referral) error
	UpdateReferral(ctx context.Context, r *Referral) error
}

const referralCols = `id, public_id, org_id, candidate_id, referred_by_employee_id, application_id, status,
	bonus_amount, bonus_currency, paid_at, notes, created_by, created_at, updated_at`

func scanReferral(row interface{ Scan(...any) error }, r *Referral) error {
	return row.Scan(
		&r.ID, &r.PublicID, &r.OrgID, &r.CandidateID, &r.ReferredByEmployeeID, &r.ApplicationID, &r.Status,
		&r.BonusAmount, &r.BonusCurrency, &r.PaidAt, &r.Notes, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
}

func buildReferralsWhere(orgID string, filter ReferralListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.CandidateID != "" {
		args = append(args, filter.CandidateID)
		clauses = append(clauses, fmt.Sprintf("candidate_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindReferrals(ctx context.Context, orgID string, filter ReferralListFilter) ([]*Referral, error) {
	where, args := buildReferralsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_referrals WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		referralCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindReferrals: %w", err)
	}
	defer rows.Close()
	list := make([]*Referral, 0)
	for rows.Next() {
		ref := &Referral{}
		if err := scanReferral(rows, ref); err != nil {
			return nil, fmt.Errorf("recruitment: FindReferrals: scan: %w", err)
		}
		list = append(list, ref)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountReferrals(ctx context.Context, orgID string, filter ReferralListFilter) (int, error) {
	where, args := buildReferralsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_referrals WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("recruitment: CountReferrals: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindReferralByRef(ctx context.Context, orgID, ref string) (*Referral, error) {
	q := `SELECT ` + referralCols + ` FROM hrm_referrals WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	rf := &Referral{}
	err := scanReferral(r.db.QueryRow(ctx, q, orgID, ref), rf)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindReferralByRef: %w", err)
	}
	return rf, nil
}

func (r *repoImpl) CreateReferral(ctx context.Context, rf *Referral) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_referrals (org_id, candidate_id, referred_by_employee_id, application_id, notes, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, public_id, status, bonus_currency, created_at, updated_at`,
		rf.OrgID, rf.CandidateID, rf.ReferredByEmployeeID, rf.ApplicationID, rf.Notes, rf.CreatedBy,
	).Scan(&rf.ID, &rf.PublicID, &rf.Status, &rf.BonusCurrency, &rf.CreatedAt, &rf.UpdatedAt)
}

func (r *repoImpl) UpdateReferral(ctx context.Context, rf *Referral) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_referrals SET
		    status = $1, bonus_amount = $2, bonus_currency = $3,
		    paid_at = CASE WHEN $1 = 'bonus_paid' THEN COALESCE(paid_at, NOW()) ELSE paid_at END,
		    notes = $4, updated_at = NOW()
		 WHERE id = $5 AND org_id = $6
		 RETURNING paid_at, updated_at`,
		rf.Status, rf.BonusAmount, rf.BonusCurrency, rf.Notes, rf.ID, rf.OrgID,
	).Scan(&rf.PaidAt, &rf.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrReferralNotFound
	}
	return err
}
