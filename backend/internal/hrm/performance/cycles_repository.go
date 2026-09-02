// backend/internal/hrm/performance/cycles_repository.go
package performance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CycleRepository is embedded into Repository — see repository.go.
type CycleRepository interface {
	FindCycles(ctx context.Context, orgID string, filter CycleListFilter) ([]*GoalCycle, error)
	CountCycles(ctx context.Context, orgID string, filter CycleListFilter) (int, error)
	FindCycleByRef(ctx context.Context, orgID, ref string) (*GoalCycle, error)
	CreateCycle(ctx context.Context, c *GoalCycle) error
	UpdateCycle(ctx context.Context, c *GoalCycle) error
	SetCycleStatus(ctx context.Context, orgID, id string, status CycleStatus, actorID *string) error
	CycleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)

	// FindCycleWeightTotals returns one row per employee holding at least one
	// weighted, non-cancelled goal in the cycle. The lock gate and the
	// read-only weight-audit endpoint share this single query, so there is
	// never a second definition of "whose weights are wrong".
	FindCycleWeightTotals(ctx context.Context, orgID, cycleID string) ([]*EmployeeWeightTotal, error)
}

const cycleCols = `id, public_id, org_id, name, description, period_start, period_end, status,
	weight_target, locked_at, locked_by, closed_at, created_by, created_at, updated_at`

func scanCycle(row interface{ Scan(...any) error }, c *GoalCycle) error {
	return row.Scan(
		&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.PeriodStart, &c.PeriodEnd, &c.Status,
		&c.WeightTarget, &c.LockedAt, &c.LockedBy, &c.ClosedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
}

func buildCyclesWhere(orgID string, filter CycleListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindCycles(ctx context.Context, orgID string, filter CycleListFilter) ([]*GoalCycle, error) {
	where, args := buildCyclesWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_goal_cycles WHERE %s ORDER BY period_start DESC LIMIT $%d OFFSET $%d`,
		cycleCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("performance: FindCycles: %w", err)
	}
	defer rows.Close()
	list := make([]*GoalCycle, 0)
	for rows.Next() {
		c := &GoalCycle{}
		if err := scanCycle(rows, c); err != nil {
			return nil, fmt.Errorf("performance: FindCycles: scan: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountCycles(ctx context.Context, orgID string, filter CycleListFilter) (int, error) {
	where, args := buildCyclesWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_goal_cycles WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountCycles: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindCycleByRef(ctx context.Context, orgID, ref string) (*GoalCycle, error) {
	q := `SELECT ` + cycleCols + ` FROM hrm_goal_cycles WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	c := &GoalCycle{}
	err := scanCycle(r.db.QueryRow(ctx, q, orgID, ref), c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindCycleByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) CreateCycle(ctx context.Context, c *GoalCycle) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_goal_cycles (org_id, name, description, period_start, period_end, weight_target, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, public_id, status, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.PeriodStart, c.PeriodEnd, c.WeightTarget, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.Status, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateCycle(ctx context.Context, c *GoalCycle) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_goal_cycles SET
		    name = $1, description = $2, period_start = $3, period_end = $4,
		    weight_target = $5, updated_at = NOW()
		 WHERE id = $6 AND org_id = $7
		 RETURNING updated_at`,
		c.Name, c.Description, c.PeriodStart, c.PeriodEnd, c.WeightTarget, c.ID, c.OrgID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCycleNotFound
	}
	return err
}

// SetCycleStatus stamps the actor and timestamp appropriate to the target
// state — the payslip pattern, where each transition records who moved it and
// when rather than relying on updated_at alone.
func (r *repoImpl) SetCycleStatus(ctx context.Context, orgID, id string, status CycleStatus, actorID *string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_goal_cycles SET
		    status    = $1,
		    locked_at = CASE WHEN $1 = 'locked' THEN NOW() ELSE locked_at END,
		    locked_by = CASE WHEN $1 = 'locked' THEN $2::uuid ELSE locked_by END,
		    closed_at = CASE WHEN $1 = 'closed' THEN NOW() ELSE closed_at END,
		    updated_at = NOW()
		 WHERE id = $3 AND org_id = $4`,
		status, actorID, id, orgID,
	)
	if err != nil {
		return fmt.Errorf("performance: SetCycleStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrCycleNotFound
	}
	return nil
}

func (r *repoImpl) CycleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var exists bool
	// id::text handles the empty-string "no exclusion" case on create; a bare
	// id <> '' would fail casting to UUID.
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_goal_cycles WHERE org_id = $1 AND LOWER(name) = LOWER($2) AND id::text <> $3)`,
		orgID, name, excludeID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("performance: CycleNameExists: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) FindCycleWeightTotals(ctx context.Context, orgID, cycleID string) ([]*EmployeeWeightTotal, error) {
	const q = `
		SELECT g.employee_id,
		       TRIM(COALESCE(e.first_name, '') || ' ' || COALESCE(e.last_name, '')) AS employee_name,
		       COALESCE(SUM(g.weight), 0) AS total_weight,
		       COUNT(*) AS goal_count
		FROM hrm_goals g
		JOIN hrm_employees e ON e.id = g.employee_id
		WHERE g.org_id = $1 AND g.cycle_id = $2
		  AND g.weight IS NOT NULL AND g.status <> 'cancelled'
		GROUP BY g.employee_id, e.first_name, e.last_name
		ORDER BY employee_name ASC`
	rows, err := r.db.Query(ctx, q, orgID, cycleID)
	if err != nil {
		return nil, fmt.Errorf("performance: FindCycleWeightTotals: %w", err)
	}
	defer rows.Close()
	list := make([]*EmployeeWeightTotal, 0)
	for rows.Next() {
		t := &EmployeeWeightTotal{}
		if err := rows.Scan(&t.EmployeeID, &t.EmployeeName, &t.TotalWeight, &t.GoalCount); err != nil {
			return nil, fmt.Errorf("performance: FindCycleWeightTotals: scan: %w", err)
		}
		list = append(list, t)
	}
	return list, rows.Err()
}
