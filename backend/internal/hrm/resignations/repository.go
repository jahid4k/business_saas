// backend/internal/hrm/resignations/repository.go
package resignations

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Resignation, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error)
	FindActiveByEmployee(ctx context.Context, orgID, employeeID string) (*Resignation, error)
	Create(ctx context.Context, r *Resignation) error
	Update(ctx context.Context, r *Resignation) error
	UpdateStatus(ctx context.Context, id string, status ResignationStatus) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id,
	to_char(resignation_date,'YYYY-MM-DD'), notice_period_days, is_notice_waived,
	to_char(last_working_date,'YYYY-MM-DD'),
	reason_category, reason_remarks,
	approval_instance_id, document_id,
	exit_interview_completed, exit_clearance_completed,
	status, accepted_at, accepted_by, created_by, created_at, updated_at`

func scanRes(row pgx.Row) (*Resignation, error) {
	r := &Resignation{}
	err := row.Scan(
		&r.ID, &r.PublicID, &r.OrgID, &r.EmployeeID,
		&r.ResignationDate, &r.NoticePeriodDays, &r.IsNoticeWaived,
		&r.LastWorkingDate,
		&r.ReasonCategory, &r.ReasonRemarks,
		&r.ApprovalInstanceID, &r.DocumentID,
		&r.ExitInterviewCompleted, &r.ExitClearanceCompleted,
		&r.Status, &r.AcceptedAt, &r.AcceptedBy, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return r, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Resignation, error) {
	q := `SELECT ` + sel + ` FROM hrm_resignations WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	q += ` ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("resignations: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*Resignation, 0)
	for rows.Next() {
		res, err := scanRes(rows)
		if err != nil { return nil, err }
		list = append(list, res)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error) {
	q := `SELECT ` + sel + ` FROM hrm_resignations WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	args := []any{orgID, ref}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	return scanRes(r.db.QueryRow(ctx, q, args...))
}

func (r *repoImpl) FindActiveByEmployee(ctx context.Context, orgID, employeeID string) (*Resignation, error) {
	return scanRes(r.db.QueryRow(ctx,
		`SELECT `+sel+` FROM hrm_resignations WHERE org_id=$1 AND employee_id=$2 AND status IN ('submitted','accepted')`,
		orgID, employeeID))
}

func (r *repoImpl) Create(ctx context.Context, res *Resignation) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_resignations
		(org_id, employee_id, resignation_date, notice_period_days, is_notice_waived,
		 last_working_date, reason_category, reason_remarks, status, created_by)
		VALUES ($1,$2,$3::date,$4,$5,$6::date,$7,$8,$9,$10)
		RETURNING id, public_id, created_at, updated_at`,
		res.OrgID, res.EmployeeID, res.ResignationDate, res.NoticePeriodDays, res.IsNoticeWaived,
		res.LastWorkingDate, res.ReasonCategory, res.ReasonRemarks, res.Status, res.CreatedBy,
	).Scan(&res.ID, &res.PublicID, &res.CreatedAt, &res.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, res *Resignation) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_resignations SET
		last_working_date=$1::date, document_id=$2,
		exit_interview_completed=$3, exit_clearance_completed=$4, updated_at=NOW()
		WHERE id=$5 AND org_id=$6 RETURNING updated_at`,
		res.LastWorkingDate, res.DocumentID,
		res.ExitInterviewCompleted, res.ExitClearanceCompleted, res.ID, res.OrgID,
	).Scan(&res.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status ResignationStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_resignations SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}
