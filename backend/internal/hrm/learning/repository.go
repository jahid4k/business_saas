// backend/internal/hrm/learning/repository.go
package learning

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository composes the sub-feature repositories. Split across files by
// sub-feature, the internal/hrm/performance shape.
//
// ⚠ The answer-key methods live on their own interface, AnswerKeyRepository,
// and are called from exactly two places: the authoring path (writing a key)
// and Grade (reading keys server-side, at submit). No learner-facing read
// touches them. That separation is the mechanism, not a convention — see
// migration 00092's header.
type Repository interface {
	CourseRepository
	VersionRepository
	EnrollmentRepository
	AttemptRepository
	AnswerKeyRepository

	// FindEmployeeRef resolves the facts an enrollment needs. This package
	// owns the query rather than importing internal/hrm/employees — the
	// onboarding / feedback / pip precedent, which keeps the dependency graph
	// free of an employees ↔ learning edge.
	FindEmployeeRef(ctx context.Context, orgID, employeeRef string) (*EmployeeRef, error)
	// FindEmployeeIDByUserID resolves the caller's own hrm_employees.id.
	// Returns "" and no error when the caller has no employee row, which is a
	// valid state for a non-employee admin, not a failure.
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Employee resolution ──────────────────────────────────────────────────────

func (r *repoImpl) FindEmployeeRef(ctx context.Context, orgID, employeeRef string) (*EmployeeRef, error) {
	e := &EmployeeRef{}
	err := r.db.QueryRow(ctx,
		`SELECT id,
		        TRIM(COALESCE(first_name,'') || ' ' || COALESCE(last_name,'')) AS display_name,
		        user_id
		   FROM hrm_employees
		  WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`,
		orgID, employeeRef).Scan(&e.EmployeeID, &e.DisplayName, &e.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("learning: FindEmployeeRef: %w", err)
	}
	return e, nil
}

func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id FROM hrm_employees WHERE org_id = $1 AND user_id = $2`, orgID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("learning: FindEmployeeIDByUserID: %w", err)
	}
	return id, nil
}
