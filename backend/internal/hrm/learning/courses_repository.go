// backend/internal/hrm/learning/courses_repository.go
package learning

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type CourseRepository interface {
	FindCourses(ctx context.Context, orgID string, f CourseListFilter) ([]*Course, error)
	CountCourses(ctx context.Context, orgID string, f CourseListFilter) (int, error)
	FindCourseByRef(ctx context.Context, orgID, ref string) (*Course, error)
	CourseTitleExists(ctx context.Context, orgID, title, excludeID string) (bool, error)
	CreateCourse(ctx context.Context, c *Course) error
	UpdateCourse(ctx context.Context, c *Course) error
	DeleteCourse(ctx context.Context, orgID, id string) error
}

type VersionRepository interface {
	FindVersions(ctx context.Context, orgID, courseID string) ([]*CourseVersion, error)
	FindVersionByRef(ctx context.Context, orgID, ref string) (*CourseVersion, error)
	// FindPublishedVersion resolves the version a new enrollment should pin.
	FindPublishedVersion(ctx context.Context, orgID, courseID string) (*CourseVersion, error)
	NextVersionNumber(ctx context.Context, courseID string) (int, error)
	CreateVersion(ctx context.Context, v *CourseVersion) error
	UpdateVersion(ctx context.Context, v *CourseVersion) error
	SetVersionStatus(ctx context.Context, orgID, id string, status VersionStatus, actorID string) (*CourseVersion, error)
	DeleteVersion(ctx context.Context, orgID, id string) error
	VersionHasEnrollments(ctx context.Context, versionID string) (bool, error)

	// CopyVersionContent duplicates modules and lessons from one version onto
	// another inside a single transaction. A half-copied draft is worse than
	// no draft: the author cannot tell what is missing.
	CopyVersionContent(ctx context.Context, fromVersionID, toVersionID string) error

	FindModules(ctx context.Context, versionID string) ([]*Module, error)
	FindModuleByRef(ctx context.Context, orgID, ref string) (*Module, error)
	CreateModule(ctx context.Context, m *Module) error
	UpdateModule(ctx context.Context, m *Module) error
	DeleteModule(ctx context.Context, id string) error

	FindLessons(ctx context.Context, versionID string) ([]*Lesson, error)
	FindLessonByRef(ctx context.Context, orgID, ref string) (*Lesson, error)
	CreateLesson(ctx context.Context, l *Lesson) error
	UpdateLesson(ctx context.Context, l *Lesson) error
	DeleteLesson(ctx context.Context, id string) error
	// CountRequiredLessons is the denominator for completion percentage.
	CountRequiredLessons(ctx context.Context, versionID string) (int, error)
}

// ── Courses ──────────────────────────────────────────────────────────────────

const courseCols = `id, public_id, org_id, title, description, category, is_active,
	created_by, created_at, updated_at`

func scanCourse(row pgx.Row) (*Course, error) {
	c := &Course{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Title, &c.Description, &c.Category,
		&c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func buildCourseWhere(orgID string, f CourseListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.Category != "" {
		args = append(args, f.Category)
		clauses = append(clauses, fmt.Sprintf("category = $%d", len(args)))
	}
	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		clauses = append(clauses, fmt.Sprintf("is_active = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		clauses = append(clauses, fmt.Sprintf("title ILIKE $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindCourses(ctx context.Context, orgID string, f CourseListFilter) ([]*Course, error) {
	where, args := buildCourseWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_courses WHERE %s
		ORDER BY title LIMIT $%d OFFSET $%d`, courseCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("learning: FindCourses: %w", err)
	}
	defer rows.Close()

	out := make([]*Course, 0)
	for rows.Next() {
		c, err := scanCourse(rows)
		if err != nil {
			return nil, fmt.Errorf("learning: FindCourses: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountCourses(ctx context.Context, orgID string, f CourseListFilter) (int, error) {
	where, args := buildCourseWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_courses WHERE %s`, where), args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("learning: CountCourses: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindCourseByRef(ctx context.Context, orgID, ref string) (*Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_courses
			WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`, courseCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("learning: FindCourseByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) CourseTitleExists(ctx context.Context, orgID, title, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_courses
		  WHERE org_id = $1 AND LOWER(title) = LOWER($2) AND ($3 = '' OR id::text <> $3))`,
		orgID, title, excludeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("learning: CourseTitleExists: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) CreateCourse(ctx context.Context, c *Course) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_courses (org_id, title, description, category, created_by)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, public_id, is_active, created_at, updated_at`,
		c.OrgID, c.Title, c.Description, c.Category, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("learning: CreateCourse: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateCourse(ctx context.Context, c *Course) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_courses SET title=$1, description=$2, category=$3, is_active=$4, updated_at=NOW()
		 WHERE org_id=$5 AND id=$6 RETURNING updated_at`,
		c.Title, c.Description, c.Category, c.IsActive, c.OrgID, c.ID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCourseNotFound
	}
	if err != nil {
		return fmt.Errorf("learning: UpdateCourse: %w", err)
	}
	return nil
}

func (r *repoImpl) DeleteCourse(ctx context.Context, orgID, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_courses WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return fmt.Errorf("learning: DeleteCourse: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrCourseNotFound
	}
	return nil
}

// ── Versions ─────────────────────────────────────────────────────────────────

const versionCols = `id, public_id, org_id, course_id, version_number, title_snapshot,
	change_note, status, pass_threshold, published_at, published_by, archived_at,
	created_by, created_at, updated_at`

func scanVersion(row pgx.Row) (*CourseVersion, error) {
	v := &CourseVersion{}
	err := row.Scan(&v.ID, &v.PublicID, &v.OrgID, &v.CourseID, &v.VersionNumber,
		&v.TitleSnapshot, &v.ChangeNote, &v.Status, &v.PassThreshold,
		&v.PublishedAt, &v.PublishedBy, &v.ArchivedAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *repoImpl) FindVersions(ctx context.Context, orgID, courseID string) ([]*CourseVersion, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_course_versions WHERE org_id=$1 AND course_id=$2
			ORDER BY version_number DESC`, versionCols), orgID, courseID)
	if err != nil {
		return nil, fmt.Errorf("learning: FindVersions: %w", err)
	}
	defer rows.Close()

	out := make([]*CourseVersion, 0)
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("learning: FindVersions: scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindVersionByRef(ctx context.Context, orgID, ref string) (*CourseVersion, error) {
	v, err := scanVersion(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_course_versions
			WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, versionCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("learning: FindVersionByRef: %w", err)
	}
	return v, nil
}

func (r *repoImpl) FindPublishedVersion(ctx context.Context, orgID, courseID string) (*CourseVersion, error) {
	v, err := scanVersion(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_course_versions
			WHERE org_id=$1 AND course_id=$2 AND status='published'
			ORDER BY version_number DESC LIMIT 1`, versionCols), orgID, courseID))
	if err != nil {
		return nil, fmt.Errorf("learning: FindPublishedVersion: %w", err)
	}
	return v, nil
}

func (r *repoImpl) NextVersionNumber(ctx context.Context, courseID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(version_number), 0) + 1 FROM hrm_course_versions WHERE course_id=$1`,
		courseID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("learning: NextVersionNumber: %w", err)
	}
	return n, nil
}

func (r *repoImpl) CreateVersion(ctx context.Context, v *CourseVersion) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_course_versions
		    (org_id, course_id, version_number, title_snapshot, change_note, pass_threshold, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, public_id, status, created_at, updated_at`,
		v.OrgID, v.CourseID, v.VersionNumber, v.TitleSnapshot, v.ChangeNote, v.PassThreshold, v.CreatedBy,
	).Scan(&v.ID, &v.PublicID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("learning: CreateVersion: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateVersion(ctx context.Context, v *CourseVersion) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_course_versions SET change_note=$1, pass_threshold=$2, updated_at=NOW()
		 WHERE org_id=$3 AND id=$4 RETURNING updated_at`,
		v.ChangeNote, v.PassThreshold, v.OrgID, v.ID,
	).Scan(&v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionNotFound
	}
	if err != nil {
		return fmt.Errorf("learning: UpdateVersion: %w", err)
	}
	return nil
}

func (r *repoImpl) SetVersionStatus(ctx context.Context, orgID, id string, status VersionStatus, actorID string) (*CourseVersion, error) {
	var publishedBy any
	if status == VersionPublished && actorID != "" {
		publishedBy = actorID
	}
	v, err := scanVersion(r.db.QueryRow(ctx,
		fmt.Sprintf(`UPDATE hrm_course_versions SET
		    status = $1,
		    published_at = CASE WHEN $1 = 'published' THEN NOW() ELSE published_at END,
		    published_by = CASE WHEN $1 = 'published' THEN $4::uuid ELSE published_by END,
		    archived_at  = CASE WHEN $1 = 'archived'  THEN NOW() ELSE archived_at END,
		    updated_at = NOW()
		 WHERE org_id=$2 AND id=$3 RETURNING %s`, versionCols), status, orgID, id, publishedBy))
	if err != nil {
		return nil, fmt.Errorf("learning: SetVersionStatus: %w", err)
	}
	if v == nil {
		return nil, ErrVersionNotFound
	}
	return v, nil
}

func (r *repoImpl) DeleteVersion(ctx context.Context, orgID, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_course_versions WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return fmt.Errorf("learning: DeleteVersion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrVersionNotFound
	}
	return nil
}

func (r *repoImpl) VersionHasEnrollments(ctx context.Context, versionID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_enrollments WHERE version_id=$1)`, versionID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("learning: VersionHasEnrollments: %w", err)
	}
	return exists, nil
}

// CopyVersionContent duplicates modules and their lessons in one transaction.
// The module id mapping is rebuilt as it goes so lessons land under the right
// new module — a straight INSERT ... SELECT cannot do that, because the new
// module ids are generated by the insert.
func (r *repoImpl) CopyVersionContent(ctx context.Context, fromVersionID, toVersionID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("learning: CopyVersionContent: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx,
		`SELECT id, title, description, display_order FROM hrm_course_modules
		  WHERE version_id=$1 ORDER BY display_order`, fromVersionID)
	if err != nil {
		return fmt.Errorf("learning: CopyVersionContent: modules: %w", err)
	}
	type srcModule struct {
		id, title string
		desc      *string
		order     int
	}
	var srcs []srcModule
	for rows.Next() {
		var m srcModule
		if err := rows.Scan(&m.id, &m.title, &m.desc, &m.order); err != nil {
			rows.Close()
			return fmt.Errorf("learning: CopyVersionContent: scan module: %w", err)
		}
		srcs = append(srcs, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("learning: CopyVersionContent: modules: %w", err)
	}

	for _, m := range srcs {
		var newModuleID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO hrm_course_modules (version_id, title, description, display_order)
			 VALUES ($1,$2,$3,$4) RETURNING id`,
			toVersionID, m.title, m.desc, m.order).Scan(&newModuleID); err != nil {
			return fmt.Errorf("learning: CopyVersionContent: insert module: %w", err)
		}

		// Lessons carry their form_template_id across unchanged: the same quiz
		// serves the new version until an author changes it, and the answer
		// keys hang off the template's questions, not off the lesson.
		if _, err := tx.Exec(ctx,
			`INSERT INTO hrm_course_lessons
			    (module_id, title, lesson_type, content_url, content_text,
			     form_template_id, pass_mark, max_attempts, is_required, display_order)
			 SELECT $1, title, lesson_type, content_url, content_text,
			        form_template_id, pass_mark, max_attempts, is_required, display_order
			   FROM hrm_course_lessons WHERE module_id=$2`,
			newModuleID, m.id); err != nil {
			return fmt.Errorf("learning: CopyVersionContent: insert lessons: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("learning: CopyVersionContent: commit: %w", err)
	}
	return nil
}

// ── Modules ──────────────────────────────────────────────────────────────────

const moduleCols = `id, public_id, version_id, title, description, display_order, created_at, updated_at`

func scanModule(row pgx.Row) (*Module, error) {
	m := &Module{}
	err := row.Scan(&m.ID, &m.PublicID, &m.VersionID, &m.Title, &m.Description,
		&m.DisplayOrder, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *repoImpl) FindModules(ctx context.Context, versionID string) ([]*Module, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_course_modules WHERE version_id=$1
			ORDER BY display_order, created_at`, moduleCols), versionID)
	if err != nil {
		return nil, fmt.Errorf("learning: FindModules: %w", err)
	}
	defer rows.Close()

	out := make([]*Module, 0)
	for rows.Next() {
		m, err := scanModule(rows)
		if err != nil {
			return nil, fmt.Errorf("learning: FindModules: scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// FindModuleByRef joins up to the version to enforce org scoping — modules
// carry no org_id of their own, being reached through the version.
func (r *repoImpl) FindModuleByRef(ctx context.Context, orgID, ref string) (*Module, error) {
	m, err := scanModule(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_course_modules m
			WHERE (m.id::text=$2 OR m.public_id=$2)
			  AND EXISTS (SELECT 1 FROM hrm_course_versions v
			               WHERE v.id = m.version_id AND v.org_id = $1)`,
			prefixCols("m", moduleCols)), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("learning: FindModuleByRef: %w", err)
	}
	return m, nil
}

func (r *repoImpl) CreateModule(ctx context.Context, m *Module) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_course_modules (version_id, title, description, display_order)
		 VALUES ($1,$2,$3,$4) RETURNING id, public_id, created_at, updated_at`,
		m.VersionID, m.Title, m.Description, m.DisplayOrder,
	).Scan(&m.ID, &m.PublicID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("learning: CreateModule: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateModule(ctx context.Context, m *Module) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_course_modules SET title=$1, description=$2, display_order=$3, updated_at=NOW()
		 WHERE id=$4 RETURNING updated_at`,
		m.Title, m.Description, m.DisplayOrder, m.ID,
	).Scan(&m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrModuleNotFound
	}
	if err != nil {
		return fmt.Errorf("learning: UpdateModule: %w", err)
	}
	return nil
}

func (r *repoImpl) DeleteModule(ctx context.Context, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_course_modules WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("learning: DeleteModule: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrModuleNotFound
	}
	return nil
}

// ── Lessons ──────────────────────────────────────────────────────────────────

const lessonCols = `id, public_id, module_id, title, lesson_type, content_url, content_text,
	form_template_id, pass_mark, max_attempts, is_required, display_order, created_at, updated_at`

func scanLesson(row pgx.Row) (*Lesson, error) {
	l := &Lesson{}
	err := row.Scan(&l.ID, &l.PublicID, &l.ModuleID, &l.Title, &l.LessonType,
		&l.ContentURL, &l.ContentText, &l.FormTemplateID, &l.PassMark, &l.MaxAttempts,
		&l.IsRequired, &l.DisplayOrder, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return l, nil
}

// FindLessons returns every lesson in a version, ordered by module then
// lesson. The join is what makes "the lessons of a version" answerable —
// lessons hang off modules, and modules off the version.
func (r *repoImpl) FindLessons(ctx context.Context, versionID string) ([]*Lesson, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_course_lessons l
			JOIN hrm_course_modules m ON m.id = l.module_id
			WHERE m.version_id = $1
			ORDER BY m.display_order, l.display_order, l.created_at`,
			prefixCols("l", lessonCols)), versionID)
	if err != nil {
		return nil, fmt.Errorf("learning: FindLessons: %w", err)
	}
	defer rows.Close()

	out := make([]*Lesson, 0)
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return nil, fmt.Errorf("learning: FindLessons: scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindLessonByRef(ctx context.Context, orgID, ref string) (*Lesson, error) {
	l, err := scanLesson(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_course_lessons l
			JOIN hrm_course_modules m ON m.id = l.module_id
			JOIN hrm_course_versions v ON v.id = m.version_id
			WHERE v.org_id = $1 AND (l.id::text = $2 OR l.public_id = $2)`,
			prefixCols("l", lessonCols)), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("learning: FindLessonByRef: %w", err)
	}
	return l, nil
}

func (r *repoImpl) CreateLesson(ctx context.Context, l *Lesson) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_course_lessons
		    (module_id, title, lesson_type, content_url, content_text,
		     form_template_id, pass_mark, max_attempts, is_required, display_order)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, public_id, created_at, updated_at`,
		l.ModuleID, l.Title, l.LessonType, l.ContentURL, l.ContentText,
		l.FormTemplateID, l.PassMark, l.MaxAttempts, l.IsRequired, l.DisplayOrder,
	).Scan(&l.ID, &l.PublicID, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return fmt.Errorf("learning: CreateLesson: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateLesson(ctx context.Context, l *Lesson) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_course_lessons SET
		    title=$1, content_url=$2, content_text=$3, form_template_id=$4,
		    pass_mark=$5, max_attempts=$6, is_required=$7, display_order=$8, updated_at=NOW()
		 WHERE id=$9 RETURNING updated_at`,
		l.Title, l.ContentURL, l.ContentText, l.FormTemplateID,
		l.PassMark, l.MaxAttempts, l.IsRequired, l.DisplayOrder, l.ID,
	).Scan(&l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLessonNotFound
	}
	if err != nil {
		return fmt.Errorf("learning: UpdateLesson: %w", err)
	}
	return nil
}

func (r *repoImpl) DeleteLesson(ctx context.Context, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_course_lessons WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("learning: DeleteLesson: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrLessonNotFound
	}
	return nil
}

func (r *repoImpl) CountRequiredLessons(ctx context.Context, versionID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_course_lessons l
		   JOIN hrm_course_modules m ON m.id = l.module_id
		  WHERE m.version_id = $1 AND l.is_required = TRUE`, versionID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("learning: CountRequiredLessons: %w", err)
	}
	return n, nil
}

// prefixCols qualifies a bare column list with a table alias, so a SELECT can
// be reused in a joined query without ambiguity. Phase 3 shipped a bug from an
// unqualified column list in a JOIN; this is the fix generalised.
func prefixCols(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
