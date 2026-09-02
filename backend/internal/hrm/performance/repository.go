// backend/internal/hrm/performance/repository.go
package performance

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the data access interface for the performance module,
// composed of the sub-feature interfaces declared in cycles_repository.go,
// goals_repository.go and checkins_repository.go — the internal/hrm/leave and
// internal/hrm/recruitment precedent for a module too large for the 5-file
// shape.
type Repository interface {
	CycleRepository
	GoalRepository
	CheckinRepository
	ScaleRepository
	AppraisalRepository

	// FindEmployeeIDByUserID resolves the caller's own hrm_employees.id from
	// their platform user_id — the recruitment.InterviewRepository precedent.
	// Returns "" and no error when the caller has no employee row, which is a
	// valid state for a non-employee admin acting on the org, not a failure.
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)

	// FindEmployeeSubject resolves the facts appraisal instantiation freezes:
	// display name, linked user account, and reporting manager. This package
	// owns the query rather than importing internal/hrm/employees — the
	// internal/hrm/onboarding precedent, which keeps the dependency graph
	// free of an employees ↔ performance edge.
	FindEmployeeSubject(ctx context.Context, orgID, employeeRef string) (*EmployeeSubject, error)

	// EmployeeExists reports whether an employee id belongs to the org, so a
	// goal can never be created against another tenant's employee.
	EmployeeExists(ctx context.Context, orgID, employeeID string) (bool, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var employeeID string
	err := r.db.QueryRow(ctx,
		`SELECT id FROM hrm_employees WHERE org_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("performance: FindEmployeeIDByUserID: %w", err)
	}
	return employeeID, nil
}

func (r *repoImpl) EmployeeExists(ctx context.Context, orgID, employeeID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_employees WHERE org_id = $1 AND id::text = $2)`,
		orgID, employeeID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("performance: EmployeeExists: %w", err)
	}
	return exists, nil
}

// EmployeeSubject is the slice of an employee row appraisal instantiation
// needs. ManagerEmployeeID and ManagerUserID are nil when the employee has no
// manager, the manager was deleted, or the manager has no platform account —
// the LEFT JOIN collapses all three into NULL, which is the right behaviour:
// each means "no manager to assign a review to".
type EmployeeSubject struct {
	EmployeeID        string
	DisplayName       string
	UserID            *string
	ManagerEmployeeID *string
	ManagerUserID     *string
}

func (r *repoImpl) FindEmployeeSubject(ctx context.Context, orgID, employeeRef string) (*EmployeeSubject, error) {
	const q = `
		SELECT e.id,
		       TRIM(COALESCE(e.first_name, '') || ' ' || COALESCE(e.last_name, '')) AS display_name,
		       e.user_id, e.manager_id, m.user_id AS manager_user_id
		FROM hrm_employees e
		LEFT JOIN hrm_employees m ON m.id = e.manager_id
		WHERE e.org_id = $1 AND (e.id::text = $2 OR e.public_id = $2)`
	s := &EmployeeSubject{}
	err := r.db.QueryRow(ctx, q, orgID, employeeRef).
		Scan(&s.EmployeeID, &s.DisplayName, &s.UserID, &s.ManagerEmployeeID, &s.ManagerUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindEmployeeSubject: %w", err)
	}
	return s, nil
}
