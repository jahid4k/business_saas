// backend/internal/hrm/performance/checkins_repository.go
package performance

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// CheckinRepository is embedded into Repository — see repository.go.
type CheckinRepository interface {
	FindCheckins(ctx context.Context, goalID string, limit, offset int) ([]*GoalCheckin, error)
	CountCheckins(ctx context.Context, goalID string) (int, error)

	// CreateCheckin locks the goal row, snapshots the previous value, derives
	// the progress percent, advances current_value and appends the check-in —
	// all in one transaction, the MoveApplicationStage precedent.
	// current_value is mutable through this path and no other, which is what
	// guarantees the history has no holes.
	CreateCheckin(ctx context.Context, orgID string, ck *GoalCheckin, newValue decimal.Decimal) (*Goal, error)
}

const checkinCols = `id, public_id, goal_id, previous_value, current_value, progress_percent,
	status_snapshot, confidence, note, checked_in_by, checked_in_at`

func scanCheckin(row interface{ Scan(...any) error }, c *GoalCheckin) error {
	return row.Scan(
		&c.ID, &c.PublicID, &c.GoalID, &c.PreviousValue, &c.CurrentValue, &c.ProgressPercent,
		&c.StatusSnapshot, &c.Confidence, &c.Note, &c.CheckedInBy, &c.CheckedInAt,
	)
}

func (r *repoImpl) FindCheckins(ctx context.Context, goalID string, limit, offset int) ([]*GoalCheckin, error) {
	q := `SELECT ` + checkinCols + ` FROM hrm_goal_checkins WHERE goal_id = $1 ORDER BY checked_in_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, goalID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("performance: FindCheckins: %w", err)
	}
	defer rows.Close()
	list := make([]*GoalCheckin, 0)
	for rows.Next() {
		c := &GoalCheckin{}
		if err := scanCheckin(rows, c); err != nil {
			return nil, fmt.Errorf("performance: FindCheckins: scan: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountCheckins(ctx context.Context, goalID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_goal_checkins WHERE goal_id = $1`, goalID).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountCheckins: %w", err)
	}
	return count, nil
}

func (r *repoImpl) CreateCheckin(ctx context.Context, orgID string, ck *GoalCheckin, newValue decimal.Decimal) (*Goal, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("performance: CreateCheckin: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the goal so previous_value is read under the same lock that the
	// advance is written under — two concurrent check-ins must serialize, or
	// both would record the same previous_value and the history would show a
	// jump that never happened.
	goal := &Goal{}
	err = scanGoal(tx.QueryRow(ctx,
		`SELECT `+goalCols+` FROM hrm_goals WHERE id = $1 AND org_id = $2 FOR UPDATE`,
		ck.GoalID, orgID,
	), goal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrGoalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("performance: CreateCheckin: lock goal: %w", err)
	}

	// Re-check under the lock: a concurrent complete/cancel may have landed
	// between the service's read and this transaction.
	if goal.Status == GoalStatusCompleted || goal.Status == GoalStatusCancelled {
		return nil, ErrCheckinGoalNotOpen
	}

	ck.PreviousValue = goal.CurrentValue
	ck.CurrentValue = newValue
	ck.StatusSnapshot = string(goal.Status)

	// Derived inside the transaction from the post-advance value, the
	// seconds_in_previous_stage precedent. Stored unclamped so history keeps
	// overshoot and regression intact.
	advanced := *goal
	advanced.CurrentValue = newValue
	ck.ProgressPercent = advanced.RawProgressPercent()

	if _, err := tx.Exec(ctx,
		`UPDATE hrm_goals SET current_value = $1, updated_at = NOW() WHERE id = $2`,
		newValue, goal.ID,
	); err != nil {
		return nil, fmt.Errorf("performance: CreateCheckin: advance goal: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO hrm_goal_checkins
		    (goal_id, previous_value, current_value, progress_percent, status_snapshot, confidence, note, checked_in_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, public_id, checked_in_at`,
		ck.GoalID, ck.PreviousValue, ck.CurrentValue, ck.ProgressPercent,
		ck.StatusSnapshot, ck.Confidence, ck.Note, ck.CheckedInBy,
	).Scan(&ck.ID, &ck.PublicID, &ck.CheckedInAt); err != nil {
		return nil, fmt.Errorf("performance: CreateCheckin: insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("performance: CreateCheckin: commit: %w", err)
	}

	goal.CurrentValue = newValue
	return goal, nil
}
