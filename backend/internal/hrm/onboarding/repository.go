// backend/internal/hrm/onboarding/repository.go
package onboarding

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is this package's only reach into HRM data — deliberately not
// internal/hrm/employees.Repository, which would create the exact
// employees <-> onboarding import cycle this package exists to avoid.
type Repository interface {
	// FindSubject resolves employeeRef (an hrm_employees.id or public_id) to
	// the fields needed to build a checklists.SubjectContext. Returns
	// (nil, nil) when no matching employee exists in orgID.
	FindSubject(ctx context.Context, orgID, employeeRef string) (*subjectRef, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// The LEFT JOIN collapses "no manager", "manager row deleted", and "manager
// has no platform account" into a single NULL manager_user_id — exactly the
// fail-soft semantics checklists.SubjectContext.ManagerUserID expects, with
// no branching required here. manager_id references hrm_employees.id, not a
// user id, hence the self-join rather than a direct users lookup.
const findSubjectSQL = `
	SELECT e.id::text, e.user_id::text, m.user_id::text, e.hire_date,
	       TRIM(e.first_name || ' ' || COALESCE(e.last_name, '')), COALESCE(e.employee_number, '')
	FROM hrm_employees e
	LEFT JOIN hrm_employees m ON m.id = e.manager_id AND m.org_id = e.org_id
	WHERE e.org_id = $1 AND (e.id::text = $2 OR e.public_id = $2)`

func (r *repoImpl) FindSubject(ctx context.Context, orgID, employeeRef string) (*subjectRef, error) {
	ref := &subjectRef{}
	err := r.db.QueryRow(ctx, findSubjectSQL, orgID, employeeRef).Scan(
		&ref.EmployeeID, &ref.UserID, &ref.ManagerUserID, &ref.HireDate, &ref.DisplayName, &ref.EmployeeNumber,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("onboarding: FindSubject: %w", err)
	}
	return ref, nil
}
