// backend/internal/hrm/learning/enrollments_repository.go
package learning

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

type EnrollmentRepository interface {
	FindEnrollments(ctx context.Context, orgID string, f EnrollmentListFilter) ([]*Enrollment, error)
	CountEnrollments(ctx context.Context, orgID string, f EnrollmentListFilter) (int, error)
	FindEnrollmentByRef(ctx context.Context, orgID, ref string) (*Enrollment, error)
	HasLiveEnrollment(ctx context.Context, orgID, employeeID, courseID string) (bool, error)
	CreateEnrollment(ctx context.Context, e *Enrollment) error
	UpdateEnrollment(ctx context.Context, e *Enrollment) error
	SetEnrollmentStatus(ctx context.Context, orgID, id string, status EnrollmentStatus) (*Enrollment, error)

	FindProgress(ctx context.Context, enrollmentID string) ([]*LessonProgress, error)
	// UpsertProgress writes one lesson's state AND recomputes the enrollment's
	// status in the same transaction, so a course cannot be observed with
	// every lesson complete but the enrollment still in progress.
	UpsertProgress(ctx context.Context, enrollmentID, lessonID string, status ProgressStatus) (*LessonProgress, error)
	CountCompletedRequired(ctx context.Context, enrollmentID string) (int, error)
	MarkEnrollmentCompleted(ctx context.Context, orgID, id string) error
	MarkEnrollmentStarted(ctx context.Context, orgID, id string) error
}

const enrollmentCols = `id, public_id, org_id, employee_id, course_id, version_id, status,
	assigned_via, due_date, started_at, completed_at, assigned_by, created_at, updated_at`

func scanEnrollment(row pgx.Row) (*Enrollment, error) {
	e := &Enrollment{}
	err := row.Scan(&e.ID, &e.PublicID, &e.OrgID, &e.EmployeeID, &e.CourseID, &e.VersionID,
		&e.Status, &e.AssignedVia, &e.DueDate, &e.StartedAt, &e.CompletedAt,
		&e.AssignedBy, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

func buildEnrollmentWhere(orgID string, f EnrollmentListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.EmployeeID != "" {
		args = append(args, f.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if f.CourseID != "" {
		args = append(args, f.CourseID)
		clauses = append(clauses, fmt.Sprintf("course_id = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Overdue {
		clauses = append(clauses,
			"due_date IS NOT NULL AND due_date < CURRENT_DATE AND status IN ('assigned','in_progress')")
	}
	// Scope predicate last so its placeholder offset accounts for every filter
	// above it. This is the control that stops a peer reading which courses a
	// colleague failed.
	if f.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(f.Scope, "employee_id", len(args), orgID, f.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindEnrollments(ctx context.Context, orgID string, f EnrollmentListFilter) ([]*Enrollment, error) {
	where, args := buildEnrollmentWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	// No table alias: the correlated subquery names hrm_enrollments explicitly
	// instead. An alias would force the WHERE clause to be qualified, and the
	// scope predicate embeds its own `org_id` inside a FROM hrm_employees
	// subquery — any textual qualification of the clause would rewrite that
	// inner reference to point at the outer table.
	q := fmt.Sprintf(`SELECT %s,
		(SELECT title FROM hrm_courses c WHERE c.id = hrm_enrollments.course_id)
		FROM hrm_enrollments WHERE %s
		ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		enrollmentCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("learning: FindEnrollments: %w", err)
	}
	defer rows.Close()

	out := make([]*Enrollment, 0)
	for rows.Next() {
		e := &Enrollment{}
		if err := rows.Scan(&e.ID, &e.PublicID, &e.OrgID, &e.EmployeeID, &e.CourseID, &e.VersionID,
			&e.Status, &e.AssignedVia, &e.DueDate, &e.StartedAt, &e.CompletedAt,
			&e.AssignedBy, &e.CreatedAt, &e.UpdatedAt, &e.CourseTitle); err != nil {
			return nil, fmt.Errorf("learning: FindEnrollments: scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountEnrollments(ctx context.Context, orgID string, f EnrollmentListFilter) (int, error) {
	where, args := buildEnrollmentWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_enrollments WHERE %s`, where),
		args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("learning: CountEnrollments: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindEnrollmentByRef(ctx context.Context, orgID, ref string) (*Enrollment, error) {
	e, err := scanEnrollment(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_enrollments
			WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, enrollmentCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("learning: FindEnrollmentByRef: %w", err)
	}
	return e, nil
}

// HasLiveEnrollment mirrors the partial unique index
// uq_hrm_enr_employee_course_live. The index is the guarantee; this is the
// friendly message. Both read EnrollmentStatus.IsLive's definition of "live".
func (r *repoImpl) HasLiveEnrollment(ctx context.Context, orgID, employeeID, courseID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_enrollments
		  WHERE org_id=$1 AND employee_id=$2 AND course_id=$3
		    AND status IN ('assigned','in_progress'))`,
		orgID, employeeID, courseID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("learning: HasLiveEnrollment: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) CreateEnrollment(ctx context.Context, e *Enrollment) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_enrollments
		    (org_id, employee_id, course_id, version_id, assigned_via, due_date, assigned_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, public_id, status, created_at, updated_at`,
		e.OrgID, e.EmployeeID, e.CourseID, e.VersionID, e.AssignedVia, e.DueDate, e.AssignedBy,
	).Scan(&e.ID, &e.PublicID, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("learning: CreateEnrollment: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateEnrollment(ctx context.Context, e *Enrollment) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_enrollments SET due_date=$1, updated_at=NOW()
		 WHERE org_id=$2 AND id=$3 RETURNING updated_at`,
		e.DueDate, e.OrgID, e.ID,
	).Scan(&e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEnrollmentNotFound
	}
	if err != nil {
		return fmt.Errorf("learning: UpdateEnrollment: %w", err)
	}
	return nil
}

func (r *repoImpl) SetEnrollmentStatus(ctx context.Context, orgID, id string, status EnrollmentStatus) (*Enrollment, error) {
	e, err := scanEnrollment(r.db.QueryRow(ctx,
		fmt.Sprintf(`UPDATE hrm_enrollments SET
		    status=$1,
		    completed_at = CASE WHEN $1='completed' THEN NOW() ELSE completed_at END,
		    updated_at=NOW()
		 WHERE org_id=$2 AND id=$3 RETURNING %s`, enrollmentCols), status, orgID, id))
	if err != nil {
		return nil, fmt.Errorf("learning: SetEnrollmentStatus: %w", err)
	}
	if e == nil {
		return nil, ErrEnrollmentNotFound
	}
	return e, nil
}

func (r *repoImpl) MarkEnrollmentStarted(ctx context.Context, orgID, id string) error {
	// Idempotent: started_at is stamped once, and the status only advances
	// from 'assigned' so a completed enrollment cannot be dragged backwards.
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_enrollments SET
		    status = CASE WHEN status='assigned' THEN 'in_progress' ELSE status END,
		    started_at = COALESCE(started_at, NOW()),
		    updated_at = NOW()
		 WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return fmt.Errorf("learning: MarkEnrollmentStarted: %w", err)
	}
	return nil
}

func (r *repoImpl) MarkEnrollmentCompleted(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_enrollments SET status='completed', completed_at=COALESCE(completed_at, NOW()),
		    updated_at=NOW()
		 WHERE org_id=$1 AND id=$2 AND status IN ('assigned','in_progress')`, orgID, id)
	if err != nil {
		return fmt.Errorf("learning: MarkEnrollmentCompleted: %w", err)
	}
	return nil
}

// ── Lesson progress ──────────────────────────────────────────────────────────

const progressCols = `id, public_id, enrollment_id, lesson_id, status, completed_at, created_at, updated_at`

func scanProgress(row pgx.Row) (*LessonProgress, error) {
	p := &LessonProgress{}
	err := row.Scan(&p.ID, &p.PublicID, &p.EnrollmentID, &p.LessonID, &p.Status,
		&p.CompletedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repoImpl) FindProgress(ctx context.Context, enrollmentID string) ([]*LessonProgress, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_lesson_progress WHERE enrollment_id=$1
			ORDER BY created_at`, progressCols), enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("learning: FindProgress: %w", err)
	}
	defer rows.Close()

	out := make([]*LessonProgress, 0)
	for rows.Next() {
		p, err := scanProgress(rows)
		if err != nil {
			return nil, fmt.Errorf("learning: FindProgress: scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpsertProgress creates the row lazily on first interaction. Lazy rather
// than pre-seeded at enrollment: a version's lessons can be added to while a
// draft, and pre-seeding would leave rows referencing lessons that no longer
// exist.
func (r *repoImpl) UpsertProgress(ctx context.Context, enrollmentID, lessonID string, status ProgressStatus) (*LessonProgress, error) {
	p, err := scanProgress(r.db.QueryRow(ctx,
		fmt.Sprintf(`INSERT INTO hrm_lesson_progress (enrollment_id, lesson_id, status, completed_at)
		 VALUES ($1,$2,$3, CASE WHEN $3='completed' THEN NOW() ELSE NULL END)
		 ON CONFLICT (enrollment_id, lesson_id) DO UPDATE SET
		    status = EXCLUDED.status,
		    completed_at = CASE WHEN EXCLUDED.status='completed'
		                        THEN COALESCE(hrm_lesson_progress.completed_at, NOW())
		                        ELSE NULL END,
		    updated_at = NOW()
		 RETURNING %s`, progressCols), enrollmentID, lessonID, status))
	if err != nil {
		return nil, fmt.Errorf("learning: UpsertProgress: %w", err)
	}
	return p, nil
}

// CountCompletedRequired is the numerator for completion percentage. It counts
// only REQUIRED lessons, matching CountRequiredLessons' denominator — counting
// optional lessons in one and not the other is how a course reports 120%.
func (r *repoImpl) CountCompletedRequired(ctx context.Context, enrollmentID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)
		   FROM hrm_lesson_progress p
		   JOIN hrm_course_lessons l ON l.id = p.lesson_id
		  WHERE p.enrollment_id = $1 AND p.status = 'completed' AND l.is_required = TRUE`,
		enrollmentID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("learning: CountCompletedRequired: %w", err)
	}
	return n, nil
}
