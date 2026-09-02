// backend/internal/hrm/performance/appraisals_repository.go
package performance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

// AppraisalRepository is embedded into Repository — see repository.go.
type AppraisalRepository interface {
	// Cycles
	FindAppraisalCycles(ctx context.Context, orgID string, filter AppraisalCycleListFilter) ([]*AppraisalCycle, error)
	CountAppraisalCycles(ctx context.Context, orgID string, filter AppraisalCycleListFilter) (int, error)
	FindAppraisalCycleByRef(ctx context.Context, orgID, ref string) (*AppraisalCycle, error)
	CreateAppraisalCycle(ctx context.Context, c *AppraisalCycle) error
	UpdateAppraisalCycle(ctx context.Context, c *AppraisalCycle) error
	SetAppraisalCycleStatus(ctx context.Context, orgID, id string, status AppraisalCycleStatus) error
	AppraisalCycleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)

	// Appraisals
	FindAppraisals(ctx context.Context, orgID string, filter AppraisalListFilter) ([]*Appraisal, error)
	CountAppraisals(ctx context.Context, orgID string, filter AppraisalListFilter) (int, error)
	FindAppraisalByRef(ctx context.Context, orgID, ref string) (*Appraisal, error)
	FindAppraisalForEmployee(ctx context.Context, orgID, cycleID, employeeID string) (*Appraisal, error)
	CreateAppraisal(ctx context.Context, a *Appraisal) error

	// AdvanceAppraisalPhase writes the new phase AND appends the history row
	// in one transaction. A transition without its audit row would leave the
	// history with holes, which is the one thing an append-only audit cannot
	// tolerate.
	AdvanceAppraisalPhase(ctx context.Context, orgID string, a *Appraisal, h *PhaseHistory) error
	// SetAppraisalRating writes the rating triple (FK + label + value
	// snapshot) and appends a history row, in one transaction. Used by both
	// the manager's initial rating and calibration overrides.
	SetAppraisalRating(ctx context.Context, orgID string, a *Appraisal, h *PhaseHistory) error
	// PublishAppraisal freezes the computed figures onto the row and appends
	// the transition, atomically.
	PublishAppraisal(ctx context.Context, orgID string, a *Appraisal, h *PhaseHistory) error

	FindPhaseHistory(ctx context.Context, appraisalID string) ([]*PhaseHistory, error)
}

// ── Cycles ───────────────────────────────────────────────────────────────────

const appraisalCycleCols = `id, public_id, org_id, name, description, period_start, period_end,
	goal_cycle_id, rating_scale_id, self_form_template_id, manager_form_template_id, status,
	created_by, created_at, updated_at`

func scanAppraisalCycle(row interface{ Scan(...any) error }, c *AppraisalCycle) error {
	return row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.PeriodStart, &c.PeriodEnd,
		&c.GoalCycleID, &c.RatingScaleID, &c.SelfFormTemplateID, &c.ManagerFormTemplateID, &c.Status,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
}

func buildAppraisalCyclesWhere(orgID string, f AppraisalCycleListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindAppraisalCycles(ctx context.Context, orgID string, f AppraisalCycleListFilter) ([]*AppraisalCycle, error) {
	where, args := buildAppraisalCyclesWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_appraisal_cycles WHERE %s ORDER BY period_start DESC LIMIT $%d OFFSET $%d`,
		appraisalCycleCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("performance: FindAppraisalCycles: %w", err)
	}
	defer rows.Close()
	list := make([]*AppraisalCycle, 0)
	for rows.Next() {
		c := &AppraisalCycle{}
		if err := scanAppraisalCycle(rows, c); err != nil {
			return nil, fmt.Errorf("performance: FindAppraisalCycles: scan: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountAppraisalCycles(ctx context.Context, orgID string, f AppraisalCycleListFilter) (int, error) {
	where, args := buildAppraisalCyclesWhere(orgID, f)
	var count int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_appraisal_cycles WHERE %s`, where), args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountAppraisalCycles: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindAppraisalCycleByRef(ctx context.Context, orgID, ref string) (*AppraisalCycle, error) {
	q := `SELECT ` + appraisalCycleCols + ` FROM hrm_appraisal_cycles WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	c := &AppraisalCycle{}
	err := scanAppraisalCycle(r.db.QueryRow(ctx, q, orgID, ref), c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindAppraisalCycleByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) CreateAppraisalCycle(ctx context.Context, c *AppraisalCycle) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_appraisal_cycles
		    (org_id, name, description, period_start, period_end, goal_cycle_id, rating_scale_id,
		     self_form_template_id, manager_form_template_id, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, public_id, status, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.PeriodStart, c.PeriodEnd, c.GoalCycleID, c.RatingScaleID,
		c.SelfFormTemplateID, c.ManagerFormTemplateID, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.Status, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateAppraisalCycle(ctx context.Context, c *AppraisalCycle) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_appraisal_cycles SET
		    name=$1, description=$2, period_start=$3, period_end=$4, goal_cycle_id=$5,
		    self_form_template_id=$6, manager_form_template_id=$7, updated_at=NOW()
		 WHERE id=$8 AND org_id=$9 RETURNING updated_at`,
		c.Name, c.Description, c.PeriodStart, c.PeriodEnd, c.GoalCycleID,
		c.SelfFormTemplateID, c.ManagerFormTemplateID, c.ID, c.OrgID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAppraisalCycleNotFound
	}
	return err
}

func (r *repoImpl) SetAppraisalCycleStatus(ctx context.Context, orgID, id string, status AppraisalCycleStatus) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_appraisal_cycles SET status=$1, updated_at=NOW() WHERE id=$2 AND org_id=$3`,
		status, id, orgID,
	)
	if err != nil {
		return fmt.Errorf("performance: SetAppraisalCycleStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrAppraisalCycleNotFound
	}
	return nil
}

func (r *repoImpl) AppraisalCycleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_appraisal_cycles WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND id::text <> $3)`,
		orgID, name, excludeID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("performance: AppraisalCycleNameExists: %w", err)
	}
	return exists, nil
}

// ── Appraisals ───────────────────────────────────────────────────────────────

const appraisalCols = `id, public_id, org_id, cycle_id, employee_id, manager_employee_id_snapshot,
	self_form_instance_id, manager_form_instance_id, phase,
	final_rating_level_id, final_rating_label, final_rating_value,
	self_score, manager_score, goal_attainment,
	published_at, published_by, acknowledged_at, cancelled_at, cancel_reason,
	created_by, created_at, updated_at`

func scanAppraisal(row interface{ Scan(...any) error }, a *Appraisal) error {
	return row.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.CycleID, &a.EmployeeID, &a.ManagerEmployeeIDSnapshot,
		&a.SelfFormInstanceID, &a.ManagerFormInstanceID, &a.Phase,
		&a.FinalRatingLevelID, &a.FinalRatingLabel, &a.FinalRatingValue,
		&a.SelfScore, &a.ManagerScore, &a.GoalAttainment,
		&a.PublishedAt, &a.PublishedBy, &a.AcknowledgedAt, &a.CancelledAt, &a.CancelReason,
		&a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
}

func buildAppraisalsWhere(orgID string, f AppraisalListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if f.CycleID != "" {
		args = append(args, f.CycleID)
		clauses = append(clauses, fmt.Sprintf("cycle_id = $%d", len(args)))
	}
	if f.EmployeeID != "" {
		args = append(args, f.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if f.Phase != "" {
		args = append(args, f.Phase)
		clauses = append(clauses, fmt.Sprintf("phase = $%d", len(args)))
	}
	// Scope predicate last so its placeholder offset accounts for every
	// filter above it. This is the control that stops a peer reading an
	// unpublished appraisal.
	if f.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(f.Scope, "employee_id", len(args), orgID, f.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindAppraisals(ctx context.Context, orgID string, f AppraisalListFilter) ([]*Appraisal, error) {
	where, args := buildAppraisalsWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_appraisals WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		appraisalCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("performance: FindAppraisals: %w", err)
	}
	defer rows.Close()
	list := make([]*Appraisal, 0)
	for rows.Next() {
		a := &Appraisal{}
		if err := scanAppraisal(rows, a); err != nil {
			return nil, fmt.Errorf("performance: FindAppraisals: scan: %w", err)
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountAppraisals(ctx context.Context, orgID string, f AppraisalListFilter) (int, error) {
	where, args := buildAppraisalsWhere(orgID, f)
	var count int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_appraisals WHERE %s`, where), args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountAppraisals: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindAppraisalByRef(ctx context.Context, orgID, ref string) (*Appraisal, error) {
	q := `SELECT ` + appraisalCols + ` FROM hrm_appraisals WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	a := &Appraisal{}
	err := scanAppraisal(r.db.QueryRow(ctx, q, orgID, ref), a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindAppraisalByRef: %w", err)
	}
	return a, nil
}

func (r *repoImpl) FindAppraisalForEmployee(ctx context.Context, orgID, cycleID, employeeID string) (*Appraisal, error) {
	q := `SELECT ` + appraisalCols + ` FROM hrm_appraisals WHERE org_id = $1 AND cycle_id = $2 AND employee_id = $3`
	a := &Appraisal{}
	err := scanAppraisal(r.db.QueryRow(ctx, q, orgID, cycleID, employeeID), a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindAppraisalForEmployee: %w", err)
	}
	return a, nil
}

func (r *repoImpl) CreateAppraisal(ctx context.Context, a *Appraisal) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_appraisals
		    (org_id, cycle_id, employee_id, manager_employee_id_snapshot,
		     self_form_instance_id, manager_form_instance_id, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, public_id, phase, created_at, updated_at`,
		a.OrgID, a.CycleID, a.EmployeeID, a.ManagerEmployeeIDSnapshot,
		a.SelfFormInstanceID, a.ManagerFormInstanceID, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.Phase, &a.CreatedAt, &a.UpdatedAt)
}

// insertPhaseHistory appends one audit row inside an existing transaction.
// Every phase and rating write goes through it, which is what guarantees the
// history has no holes.
func insertPhaseHistory(ctx context.Context, tx pgx.Tx, h *PhaseHistory) error {
	return tx.QueryRow(ctx,
		`INSERT INTO hrm_appraisal_phase_history
		    (appraisal_id, from_phase, to_phase, from_rating_level_id, from_rating_label,
		     to_rating_level_id, to_rating_label, note, changed_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, changed_at`,
		h.AppraisalID, h.FromPhase, h.ToPhase, h.FromRatingLevelID, h.FromRatingLabel,
		h.ToRatingLevelID, h.ToRatingLabel, h.Note, h.ChangedBy,
	).Scan(&h.ID, &h.PublicID, &h.ChangedAt)
}

func (r *repoImpl) AdvanceAppraisalPhase(ctx context.Context, orgID string, a *Appraisal, h *PhaseHistory) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("performance: AdvanceAppraisalPhase: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cmd, err := tx.Exec(ctx,
		`UPDATE hrm_appraisals SET
		    phase = $1,
		    acknowledged_at = CASE WHEN $1 = 'acknowledged' THEN NOW() ELSE acknowledged_at END,
		    cancelled_at    = CASE WHEN $1 = 'cancelled'    THEN NOW() ELSE cancelled_at END,
		    cancel_reason   = CASE WHEN $1 = 'cancelled'    THEN $2   ELSE cancel_reason END,
		    updated_at = NOW()
		 WHERE id = $3 AND org_id = $4`,
		string(a.Phase), h.Note, a.ID, orgID,
	)
	if err != nil {
		return fmt.Errorf("performance: AdvanceAppraisalPhase: update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrAppraisalNotFound
	}
	if err := insertPhaseHistory(ctx, tx, h); err != nil {
		return fmt.Errorf("performance: AdvanceAppraisalPhase: history: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) SetAppraisalRating(ctx context.Context, orgID string, a *Appraisal, h *PhaseHistory) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("performance: SetAppraisalRating: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cmd, err := tx.Exec(ctx,
		`UPDATE hrm_appraisals SET
		    final_rating_level_id = $1, final_rating_label = $2, final_rating_value = $3, updated_at = NOW()
		 WHERE id = $4 AND org_id = $5`,
		a.FinalRatingLevelID, a.FinalRatingLabel, a.FinalRatingValue, a.ID, orgID,
	)
	if err != nil {
		return fmt.Errorf("performance: SetAppraisalRating: update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrAppraisalNotFound
	}
	if err := insertPhaseHistory(ctx, tx, h); err != nil {
		return fmt.Errorf("performance: SetAppraisalRating: history: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) PublishAppraisal(ctx context.Context, orgID string, a *Appraisal, h *PhaseHistory) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("performance: PublishAppraisal: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Freezing the computed figures and the phase in one statement: a
	// published appraisal whose scores were written separately could be read
	// mid-write with a published phase and null scores.
	cmd, err := tx.Exec(ctx,
		`UPDATE hrm_appraisals SET
		    phase = 'published',
		    self_score = $1, manager_score = $2, goal_attainment = $3,
		    published_at = NOW(), published_by = $4::uuid, updated_at = NOW()
		 WHERE id = $5 AND org_id = $6`,
		a.SelfScore, a.ManagerScore, a.GoalAttainment, h.ChangedBy, a.ID, orgID,
	)
	if err != nil {
		return fmt.Errorf("performance: PublishAppraisal: update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrAppraisalNotFound
	}
	if err := insertPhaseHistory(ctx, tx, h); err != nil {
		return fmt.Errorf("performance: PublishAppraisal: history: %w", err)
	}
	return tx.Commit(ctx)
}

const phaseHistoryCols = `id, public_id, appraisal_id, from_phase, to_phase,
	from_rating_level_id, from_rating_label, to_rating_level_id, to_rating_label,
	note, changed_by, changed_at`

func (r *repoImpl) FindPhaseHistory(ctx context.Context, appraisalID string) ([]*PhaseHistory, error) {
	q := `SELECT ` + phaseHistoryCols + ` FROM hrm_appraisal_phase_history
	      WHERE appraisal_id = $1 ORDER BY changed_at ASC`
	rows, err := r.db.Query(ctx, q, appraisalID)
	if err != nil {
		return nil, fmt.Errorf("performance: FindPhaseHistory: %w", err)
	}
	defer rows.Close()
	list := make([]*PhaseHistory, 0)
	for rows.Next() {
		h := &PhaseHistory{}
		if err := rows.Scan(&h.ID, &h.PublicID, &h.AppraisalID, &h.FromPhase, &h.ToPhase,
			&h.FromRatingLevelID, &h.FromRatingLabel, &h.ToRatingLevelID, &h.ToRatingLabel,
			&h.Note, &h.ChangedBy, &h.ChangedAt); err != nil {
			return nil, fmt.Errorf("performance: FindPhaseHistory: scan: %w", err)
		}
		list = append(list, h)
	}
	return list, rows.Err()
}

// decimalPtr is a small helper for the publish path, which snapshots
// optionally-present scores.
func decimalPtr(d decimal.Decimal) *decimal.Decimal { return &d }
