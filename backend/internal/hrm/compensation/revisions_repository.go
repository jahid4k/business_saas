// backend/internal/hrm/compensation/revisions_repository.go
package compensation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

// RevisionRepository covers salary revision cycles and the per-employee
// revisions within them.
type RevisionRepository interface {
	ListCycles(ctx context.Context, orgID string) ([]*RevisionCycle, error)
	FindCycleByRef(ctx context.Context, orgID, ref string) (*RevisionCycle, error)
	FindCycleByApprovalInstance(ctx context.Context, orgID, instanceID string) (*RevisionCycle, error)
	CreateCycle(ctx context.Context, c *RevisionCycle) error
	UpdateCycle(ctx context.Context, c *RevisionCycle) error

	// ListEligibleEmployees returns every active employee in the org, for
	// ComputeCycle to build one Revision row per employee.
	ListEligibleEmployees(ctx context.Context, orgID string) ([]string, error)
	// ReplaceRevisions deletes any existing rows for the cycle and inserts
	// the freshly computed set, atomically — recomputation must not leave a
	// stale row from a previous compute alongside new ones.
	ReplaceRevisions(ctx context.Context, cycleID string, revisions []*Revision) error
	ListRevisionsByCycle(ctx context.Context, orgID, cycleID string, filter ListFilter) ([]*Revision, int, error)
	FindRevisionByRef(ctx context.Context, orgID, ref string) (*Revision, error)
	UpdateRevisionOverride(ctx context.Context, r *Revision) error
	// MarkRevisionsApplied stamps salary_record_id for every non-excluded
	// revision in a cycle once each has produced a real salary record.
	MarkRevisionApplied(ctx context.Context, revisionID, salaryRecordID string) error
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

const cycleSel = `id, public_id, org_id, name, description, effective_date, status, approval_instance_id,
	created_by, created_at, updated_at, computed_at, submitted_at, applied_at, applied_by`

func scanCycle(row pgx.Row) (*RevisionCycle, error) {
	c := &RevisionCycle{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.EffectiveDate, &c.Status, &c.ApprovalInstanceID,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.ComputedAt, &c.SubmittedAt, &c.AppliedAt, &c.AppliedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) ListCycles(ctx context.Context, orgID string) ([]*RevisionCycle, error) {
	rows, err := r.db.Query(ctx, `SELECT `+cycleSel+` FROM hrm_salary_revision_cycles WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("compensation: ListCycles: %w", err)
	}
	defer rows.Close()
	list := make([]*RevisionCycle, 0)
	for rows.Next() {
		c, err := scanCycle(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindCycleByRef(ctx context.Context, orgID, ref string) (*RevisionCycle, error) {
	return scanCycle(r.db.QueryRow(ctx,
		`SELECT `+cycleSel+` FROM hrm_salary_revision_cycles WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindCycleByApprovalInstance(ctx context.Context, orgID, instanceID string) (*RevisionCycle, error) {
	return scanCycle(r.db.QueryRow(ctx,
		`SELECT `+cycleSel+` FROM hrm_salary_revision_cycles WHERE org_id=$1 AND approval_instance_id=$2::uuid`,
		orgID, instanceID))
}

func (r *repoImpl) CreateCycle(ctx context.Context, c *RevisionCycle) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_revision_cycles (org_id, name, description, effective_date, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.EffectiveDate, c.Status, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateCycle(ctx context.Context, c *RevisionCycle) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_salary_revision_cycles
		    SET status=$3, approval_instance_id=$4, computed_at=$5, submitted_at=$6,
		        applied_at=$7, applied_by=$8, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		c.OrgID, c.ID, c.Status, c.ApprovalInstanceID, c.ComputedAt, c.SubmittedAt, c.AppliedAt, c.AppliedBy)
	if err != nil {
		return fmt.Errorf("compensation: UpdateCycle: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrCycleNotFound
	}
	return nil
}

func (r *repoImpl) ListEligibleEmployees(ctx context.Context, orgID string) ([]string, error) {
	// Mirrors the "who gets paid" rule payslips.computePayslips uses (r25):
	// active and on_leave employees, by status CATEGORY not status name.
	// A leaver mid-cycle is not proposed a revision — there is no future pay
	// to revise.
	rows, err := r.db.Query(ctx,
		`SELECT e.id::text FROM hrm_employees e
		   JOIN hrm_employee_statuses est ON est.id = e.status_id
		  WHERE e.org_id=$1 AND est.category IN ('active','on_leave')`,
		orgID)
	if err != nil {
		return nil, fmt.Errorf("compensation: ListEligibleEmployees: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *repoImpl) ReplaceRevisions(ctx context.Context, cycleID string, revisions []*Revision) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("compensation: ReplaceRevisions: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM hrm_salary_revisions WHERE cycle_id=$1::uuid`, cycleID); err != nil {
		return fmt.Errorf("compensation: ReplaceRevisions: clear: %w", err)
	}
	for _, rv := range revisions {
		if err := tx.QueryRow(ctx,
			`INSERT INTO hrm_salary_revisions
			    (org_id, cycle_id, employee_id, current_basic_pay, proposed_basic_pay,
			     rating_level_id, calculation_snapshot, computation_warning)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, public_id, created_at, updated_at`,
			rv.OrgID, rv.CycleID, rv.EmployeeID, rv.CurrentBasicPay, rv.ProposedBasicPay,
			rv.RatingLevelID, rv.CalculationSnapshot, rv.ComputationWarning,
		).Scan(&rv.ID, &rv.PublicID, &rv.CreatedAt, &rv.UpdatedAt); err != nil {
			return fmt.Errorf("compensation: ReplaceRevisions: insert employee %s: %w", rv.EmployeeID, err)
		}
	}
	return tx.Commit(ctx)
}

const revisionSel = `id, public_id, org_id, cycle_id, employee_id, current_basic_pay, proposed_basic_pay,
	is_excluded, rating_level_id, calculation_snapshot, computation_warning, override_reason,
	salary_record_id, created_at, updated_at`

func scanRevision(row pgx.Row) (*Revision, error) {
	rv := &Revision{}
	err := row.Scan(&rv.ID, &rv.PublicID, &rv.OrgID, &rv.CycleID, &rv.EmployeeID,
		&rv.CurrentBasicPay, &rv.ProposedBasicPay, &rv.IsExcluded, &rv.RatingLevelID,
		&rv.CalculationSnapshot, &rv.ComputationWarning, &rv.OverrideReason,
		&rv.SalaryRecordID, &rv.CreatedAt, &rv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rv, nil
}

func (r *repoImpl) ListRevisionsByCycle(ctx context.Context, orgID, cycleID string, filter ListFilter) ([]*Revision, int, error) {
	clauses := []string{"org_id = $1", "cycle_id = $2::uuid"}
	args := []any{orgID, cycleID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	where := strings.Join(clauses, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_salary_revisions WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("compensation: ListRevisionsByCycle: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_salary_revisions WHERE %s ORDER BY created_at LIMIT $%d OFFSET $%d`,
		revisionSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("compensation: ListRevisionsByCycle: %w", err)
	}
	defer rows.Close()
	list := make([]*Revision, 0)
	for rows.Next() {
		rv, err := scanRevision(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, rv)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindRevisionByRef(ctx context.Context, orgID, ref string) (*Revision, error) {
	return scanRevision(r.db.QueryRow(ctx,
		`SELECT `+revisionSel+` FROM hrm_salary_revisions WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) UpdateRevisionOverride(ctx context.Context, rv *Revision) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_salary_revisions
		    SET proposed_basic_pay=$3, override_reason=$4, is_excluded=$5, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		rv.OrgID, rv.ID, rv.ProposedBasicPay, rv.OverrideReason, rv.IsExcluded)
	if err != nil {
		return fmt.Errorf("compensation: UpdateRevisionOverride: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrRevisionNotFound
	}
	return nil
}

func (r *repoImpl) MarkRevisionApplied(ctx context.Context, revisionID, salaryRecordID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_salary_revisions SET salary_record_id=$2::uuid, updated_at=NOW() WHERE id=$1::uuid`,
		revisionID, salaryRecordID)
	if err != nil {
		return fmt.Errorf("compensation: MarkRevisionApplied: %w", err)
	}
	return nil
}

func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM hrm_employees WHERE org_id=$1 AND user_id=$2 LIMIT 1`,
		orgID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("compensation: FindEmployeeIDByUserID: %w", err)
	}
	return id, nil
}
