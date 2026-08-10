// backend/internal/hrm/performance/goals_repository.go
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

// GoalRepository is embedded into Repository — see repository.go.
type GoalRepository interface {
	FindGoals(ctx context.Context, orgID string, filter GoalListFilter) ([]*Goal, error)
	CountGoals(ctx context.Context, orgID string, filter GoalListFilter) (int, error)
	FindGoalByRef(ctx context.Context, orgID, ref string) (*Goal, error)
	FindChildGoals(ctx context.Context, orgID, parentGoalID string) ([]*Goal, error)

	// FindGoalRef returns the deliberately narrow alignment projection —
	// public_id, title, goal_level and nothing else. A separate type, never a
	// trimmed *Goal, so no column added in Phase 5B or 5C can leak through a
	// parent reference to a caller not scoped to the parent's owner.
	FindGoalRef(ctx context.Context, orgID, goalID string) (*GoalRef, error)

	// SumGoalWeights feeds the service's pre-check, which exists to produce a
	// good error message. It is NOT the enforcement point — see
	// CreateGoalGuarded.
	SumGoalWeights(ctx context.Context, employeeID, cycleID, excludeGoalID string) (decimal.Decimal, error)

	CreateGoalGuarded(ctx context.Context, g *Goal, weightTarget decimal.Decimal) error
	UpdateGoalGuarded(ctx context.Context, g *Goal, weightTarget decimal.Decimal) error

	SetGoalStatus(ctx context.Context, orgID, goalID string, status GoalStatus, outcome *GoalOutcome, reason *string) error
	DeleteGoal(ctx context.Context, orgID, goalID string) error
	CountCheckinsForGoal(ctx context.Context, goalID string) (int, error)
	CountChildGoals(ctx context.Context, orgID, goalID string) (int, error)

	// WouldCreateAlignmentCycle reports whether pointing goalID's parent at
	// newParentID would close a loop — i.e. whether goalID is already an
	// ancestor of newParentID. Named for the question rather than the
	// traversal so the argument order cannot be transposed at the call site.
	//
	// The alignment tree has no DB-level acyclicity constraint, so the CTE
	// carries the same `<> ALL(path)` guard as scope/predicate.go plus a depth
	// cap — an already-corrupt row must not hang the query.
	WouldCreateAlignmentCycle(ctx context.Context, orgID, goalID, newParentID string) (bool, error)
}

const goalCols = `id, public_id, org_id, cycle_id, employee_id, parent_goal_id, title, description,
	goal_level, category, measurement_type, direction, start_value, target_value, current_value,
	unit, currency_code, weight, status, outcome, start_date, due_date, completed_at, cancelled_at,
	cancel_reason, created_by, created_at, updated_at`

func scanGoal(row interface{ Scan(...any) error }, g *Goal) error {
	return row.Scan(
		&g.ID, &g.PublicID, &g.OrgID, &g.CycleID, &g.EmployeeID, &g.ParentGoalID, &g.Title, &g.Description,
		&g.GoalLevel, &g.Category, &g.MeasurementType, &g.Direction, &g.StartValue, &g.TargetValue, &g.CurrentValue,
		&g.Unit, &g.CurrencyCode, &g.Weight, &g.Status, &g.Outcome, &g.StartDate, &g.DueDate, &g.CompletedAt,
		&g.CancelledAt, &g.CancelReason, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt,
	)
}

func buildGoalsWhere(orgID string, filter GoalListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.CycleID != "" {
		args = append(args, filter.CycleID)
		clauses = append(clauses, fmt.Sprintf("cycle_id = $%d", len(args)))
	}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.GoalLevel != "" {
		args = append(args, filter.GoalLevel)
		clauses = append(clauses, fmt.Sprintf("goal_level = $%d", len(args)))
	}
	if filter.ParentID != "" {
		args = append(args, filter.ParentID)
		clauses = append(clauses, fmt.Sprintf("parent_goal_id = $%d", len(args)))
	}
	// Scope predicate last, so its placeholder offset accounts for every
	// filter above it. ScopeAll short-circuits to TRUE and adds no args.
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindGoals(ctx context.Context, orgID string, filter GoalListFilter) ([]*Goal, error) {
	where, args := buildGoalsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_goals WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		goalCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("performance: FindGoals: %w", err)
	}
	defer rows.Close()
	list := make([]*Goal, 0)
	for rows.Next() {
		g := &Goal{}
		if err := scanGoal(rows, g); err != nil {
			return nil, fmt.Errorf("performance: FindGoals: scan: %w", err)
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountGoals(ctx context.Context, orgID string, filter GoalListFilter) (int, error) {
	where, args := buildGoalsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_goals WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountGoals: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindGoalByRef(ctx context.Context, orgID, ref string) (*Goal, error) {
	q := `SELECT ` + goalCols + ` FROM hrm_goals WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	g := &Goal{}
	err := scanGoal(r.db.QueryRow(ctx, q, orgID, ref), g)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindGoalByRef: %w", err)
	}
	return g, nil
}

func (r *repoImpl) FindChildGoals(ctx context.Context, orgID, parentGoalID string) ([]*Goal, error) {
	q := `SELECT ` + goalCols + ` FROM hrm_goals WHERE org_id = $1 AND parent_goal_id = $2 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, orgID, parentGoalID)
	if err != nil {
		return nil, fmt.Errorf("performance: FindChildGoals: %w", err)
	}
	defer rows.Close()
	list := make([]*Goal, 0)
	for rows.Next() {
		g := &Goal{}
		if err := scanGoal(rows, g); err != nil {
			return nil, fmt.Errorf("performance: FindChildGoals: scan: %w", err)
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindGoalRef(ctx context.Context, orgID, goalID string) (*GoalRef, error) {
	ref := &GoalRef{}
	err := r.db.QueryRow(ctx,
		`SELECT public_id, title, goal_level FROM hrm_goals WHERE org_id = $1 AND id = $2`,
		orgID, goalID,
	).Scan(&ref.PublicID, &ref.Title, &ref.GoalLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindGoalRef: %w", err)
	}
	return ref, nil
}

const sumWeightsSQL = `
	SELECT COALESCE(SUM(weight), 0) FROM hrm_goals
	 WHERE employee_id = $1 AND cycle_id = $2
	   AND weight IS NOT NULL AND status <> 'cancelled'
	   AND id::text <> $3`

func (r *repoImpl) SumGoalWeights(ctx context.Context, employeeID, cycleID, excludeGoalID string) (decimal.Decimal, error) {
	var total decimal.Decimal
	if err := r.db.QueryRow(ctx, sumWeightsSQL, employeeID, cycleID, excludeGoalID).Scan(&total); err != nil {
		return decimal.Zero, fmt.Errorf("performance: SumGoalWeights: %w", err)
	}
	return total, nil
}

// lockEmployeeAndSumWeights is the shared core of both guarded writes.
//
// The invariant's scope key is (employee_id, cycle_id), and on create no row
// exists for that pair — so there is nothing among the sibling goals to lock.
// SELECT ... FOR UPDATE over the siblings does NOT close the window: a
// competing transaction INSERTs a row that was in neither transaction's locked
// set, and under READ COMMITTED both weight totals pass. (SELECT SUM(...) FOR
// UPDATE is not even legal Postgres — "FOR UPDATE is not allowed with
// aggregate functions" — which is a good hint the shape is wrong.)
//
// Locking the EMPLOYEE row is what works: it already exists, there is exactly
// one per employee, and its granularity is exactly the invariant's domain, so
// two different people setting goals never contend. The SUM is then issued as
// its own subsequent statement, because under READ COMMITTED each statement
// takes a fresh snapshot — that is what makes it observe whatever a competing
// transaction committed while this one was blocked. Folding the SUM into the
// locking statement would defeat the entire mechanism.
func lockEmployeeAndSumWeights(ctx context.Context, tx pgx.Tx, orgID, employeeID, cycleID, excludeGoalID string) (decimal.Decimal, error) {
	var locked string
	err := tx.QueryRow(ctx,
		`SELECT id FROM hrm_employees WHERE id = $1 AND org_id = $2 FOR UPDATE`,
		employeeID, orgID,
	).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, ErrEmployeeNotFound
	}
	if err != nil {
		return decimal.Zero, fmt.Errorf("lock employee: %w", err)
	}

	var existing decimal.Decimal
	if err := tx.QueryRow(ctx, sumWeightsSQL, employeeID, cycleID, excludeGoalID).Scan(&existing); err != nil {
		return decimal.Zero, fmt.Errorf("weight sum: %w", err)
	}
	return existing, nil
}

// weightWouldExceed reports whether adding w to existing breaks the target. A
// nil weight is "tracking only" and never counts.
func weightWouldExceed(existing decimal.Decimal, w *decimal.Decimal, target decimal.Decimal) bool {
	if w == nil {
		return false
	}
	return existing.Add(*w).GreaterThan(target)
}

func (r *repoImpl) CreateGoalGuarded(ctx context.Context, g *Goal, weightTarget decimal.Decimal) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("performance: CreateGoalGuarded: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := lockEmployeeAndSumWeights(ctx, tx, g.OrgID, g.EmployeeID, g.CycleID, "")
	if err != nil {
		if errors.Is(err, ErrEmployeeNotFound) {
			return err
		}
		return fmt.Errorf("performance: CreateGoalGuarded: %w", err)
	}
	if weightWouldExceed(existing, g.Weight, weightTarget) {
		return ErrWeightExceedsCycleTarget
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO hrm_goals (
		    org_id, cycle_id, employee_id, parent_goal_id, title, description, goal_level, category,
		    measurement_type, direction, start_value, target_value, current_value, unit, currency_code,
		    weight, start_date, due_date, created_by
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		 RETURNING id, public_id, status, created_at, updated_at`,
		g.OrgID, g.CycleID, g.EmployeeID, g.ParentGoalID, g.Title, g.Description, g.GoalLevel, g.Category,
		g.MeasurementType, g.Direction, g.StartValue, g.TargetValue, g.CurrentValue, g.Unit, g.CurrencyCode,
		g.Weight, g.StartDate, g.DueDate, g.CreatedBy,
	).Scan(&g.ID, &g.PublicID, &g.Status, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return fmt.Errorf("performance: CreateGoalGuarded: insert: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) UpdateGoalGuarded(ctx context.Context, g *Goal, weightTarget decimal.Decimal) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("performance: UpdateGoalGuarded: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Excluding this goal's own id: its current weight is being replaced, not
	// added to.
	existing, err := lockEmployeeAndSumWeights(ctx, tx, g.OrgID, g.EmployeeID, g.CycleID, g.ID)
	if err != nil {
		if errors.Is(err, ErrEmployeeNotFound) {
			return err
		}
		return fmt.Errorf("performance: UpdateGoalGuarded: %w", err)
	}
	if weightWouldExceed(existing, g.Weight, weightTarget) {
		return ErrWeightExceedsCycleTarget
	}

	// current_value is deliberately absent: progress moves only through a
	// check-in, so this path can never leave hrm_goal_checkins with a hole.
	err = tx.QueryRow(ctx,
		`UPDATE hrm_goals SET
		    parent_goal_id = $1, title = $2, description = $3, goal_level = $4, category = $5,
		    measurement_type = $6, direction = $7, start_value = $8, target_value = $9,
		    unit = $10, currency_code = $11, weight = $12, start_date = $13, due_date = $14,
		    updated_at = NOW()
		 WHERE id = $15 AND org_id = $16
		 RETURNING updated_at`,
		g.ParentGoalID, g.Title, g.Description, g.GoalLevel, g.Category,
		g.MeasurementType, g.Direction, g.StartValue, g.TargetValue,
		g.Unit, g.CurrencyCode, g.Weight, g.StartDate, g.DueDate,
		g.ID, g.OrgID,
	).Scan(&g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrGoalNotFound
	}
	if err != nil {
		return fmt.Errorf("performance: UpdateGoalGuarded: update: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) SetGoalStatus(ctx context.Context, orgID, goalID string, status GoalStatus, outcome *GoalOutcome, reason *string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_goals SET
		    status        = $1,
		    outcome       = CASE WHEN $1 = 'completed' THEN $2 ELSE outcome END,
		    completed_at  = CASE WHEN $1 = 'completed' THEN NOW() ELSE completed_at END,
		    cancelled_at  = CASE WHEN $1 = 'cancelled' THEN NOW() ELSE cancelled_at END,
		    cancel_reason = CASE WHEN $1 = 'cancelled' THEN $3 ELSE cancel_reason END,
		    updated_at    = NOW()
		 WHERE id = $4 AND org_id = $5`,
		status, outcome, reason, goalID, orgID,
	)
	if err != nil {
		return fmt.Errorf("performance: SetGoalStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrGoalNotFound
	}
	return nil
}

func (r *repoImpl) DeleteGoal(ctx context.Context, orgID, goalID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_goals WHERE org_id = $1 AND id = $2`, orgID, goalID)
	if err != nil {
		return fmt.Errorf("performance: DeleteGoal: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrGoalNotFound
	}
	return nil
}

func (r *repoImpl) CountCheckinsForGoal(ctx context.Context, goalID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_goal_checkins WHERE goal_id = $1`, goalID).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountCheckinsForGoal: %w", err)
	}
	return count, nil
}

func (r *repoImpl) CountChildGoals(ctx context.Context, orgID, goalID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_goals WHERE org_id = $1 AND parent_goal_id = $2`, orgID, goalID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountChildGoals: %w", err)
	}
	return count, nil
}

// maxAlignmentDepth bounds the ancestor walk. Alignment chains in practice are
// company → department → team → individual → key result; 32 is far beyond any
// real tree and exists only so a corrupt row cannot spin.
const maxAlignmentDepth = 32

func (r *repoImpl) WouldCreateAlignmentCycle(ctx context.Context, orgID, goalID, newParentID string) (bool, error) {
	// Walks UP from the proposed parent collecting its ancestors, then asks
	// whether the goal itself is among them. If it is, the goal already sits
	// above its proposed parent and the edge would close a loop.
	//
	// The path array plus `<> ALL(path)` is the scope/predicate.go pattern:
	// without it an already-cyclic row set recurses to the depth cap, and
	// without the cap it would not terminate at all.
	const q = `
		WITH RECURSIVE ancestors AS (
		    SELECT id, parent_goal_id, 1 AS depth, ARRAY[id] AS path
		      FROM hrm_goals
		     WHERE org_id = $1 AND id = $2
		    UNION ALL
		    SELECT g.id, g.parent_goal_id, a.depth + 1, a.path || g.id
		      FROM hrm_goals g
		      JOIN ancestors a ON g.id = a.parent_goal_id
		     WHERE g.org_id = $1
		       AND a.depth < $4
		       AND g.id <> ALL(a.path)
		)
		SELECT EXISTS(SELECT 1 FROM ancestors WHERE id = $3)`
	var found bool
	if err := r.db.QueryRow(ctx, q, orgID, newParentID, goalID, maxAlignmentDepth).Scan(&found); err != nil {
		return false, fmt.Errorf("performance: WouldCreateAlignmentCycle: %w", err)
	}
	return found, nil
}
