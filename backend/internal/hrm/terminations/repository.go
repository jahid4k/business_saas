// backend/internal/hrm/terminations/repository.go
package terminations

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for employee terminations.
// TENANT ISOLATION: every query includes org_id.
type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Termination, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Termination, error)
	FindActiveByEmployee(ctx context.Context, orgID, employeeID string) (*Termination, error)
	Create(ctx context.Context, t *Termination) error
	Update(ctx context.Context, t *Termination) error
	UpdateStatus(ctx context.Context, id string, status TerminationStatus) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id, termination_type,
	to_char(termination_date,'YYYY-MM-DD'), to_char(last_working_date,'YYYY-MM-DD'),
	reason, internal_notes,
	approval_instance_id, document_id,
	severance_amount, severance_currency,
	is_rehire_eligible, exit_clearance_completed,
	status, applied_at, applied_by, created_by, created_at, updated_at`

func scanTerm(row pgx.Row) (*Termination, error) {
	t := &Termination{}
	err := row.Scan(
		&t.ID, &t.PublicID, &t.OrgID, &t.EmployeeID, &t.TerminationType,
		&t.TerminationDate, &t.LastWorkingDate,
		&t.Reason, &t.InternalNotes,
		&t.ApprovalInstanceID, &t.DocumentID,
		&t.SeveranceAmount, &t.SeveranceCurrency,
		&t.IsRehireEligible, &t.ExitClearanceCompleted,
		&t.Status, &t.AppliedAt, &t.AppliedBy, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return t, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Termination, error) {
	q := `SELECT ` + sel + ` FROM hrm_terminations WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	q += ` ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("terminations: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*Termination, 0)
	for rows.Next() {
		t, err := scanTerm(rows)
		if err != nil { return nil, err }
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Termination, error) {
	q := `SELECT ` + sel + ` FROM hrm_terminations WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	args := []any{orgID, ref}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	return scanTerm(r.db.QueryRow(ctx, q, args...))
}

func (r *repoImpl) FindActiveByEmployee(ctx context.Context, orgID, employeeID string) (*Termination, error) {
	return scanTerm(r.db.QueryRow(ctx,
		`SELECT `+sel+` FROM hrm_terminations WHERE org_id=$1 AND employee_id=$2
		AND status IN ('draft','pending_approval','approved')`,
		orgID, employeeID))
}

func (r *repoImpl) Create(ctx context.Context, t *Termination) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_terminations
		(org_id, employee_id, termination_type, termination_date, last_working_date,
		 reason, internal_notes, severance_amount, severance_currency,
		 is_rehire_eligible, status, created_by)
		VALUES ($1,$2,$3,$4::date,$5::date,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, public_id, created_at, updated_at`,
		t.OrgID, t.EmployeeID, t.TerminationType, t.TerminationDate, t.LastWorkingDate,
		t.Reason, t.InternalNotes, t.SeveranceAmount, t.SeveranceCurrency,
		t.IsRehireEligible, t.Status, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, t *Termination) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_terminations SET
		termination_date=$1::date, last_working_date=$2::date, reason=$3, internal_notes=$4,
		severance_amount=$5, is_rehire_eligible=$6, exit_clearance_completed=$7, document_id=$8,
		updated_at=NOW()
		WHERE id=$9 AND org_id=$10 RETURNING updated_at`,
		t.TerminationDate, t.LastWorkingDate, t.Reason, t.InternalNotes,
		t.SeveranceAmount, t.IsRehireEligible, t.ExitClearanceCompleted, t.DocumentID,
		t.ID, t.OrgID,
	).Scan(&t.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status TerminationStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_terminations SET status=$1, updated_at=NOW() WHERE id=$2`,
		status, id)
	return err
}
