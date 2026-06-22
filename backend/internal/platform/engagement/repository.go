// backend/internal/platform/engagement/repository.go
package engagement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for all engagement records.
//
// TENANT ISOLATION: every method takes orgID.
// MODULE SCOPING: methods that create records take a module string ("crm", "erp", etc.)
// Timeline queries accept an optional module filter — empty string means all modules.
type Repository interface {
	// Notes
	FindNotesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Note, error)
	FindNoteByID(ctx context.Context, orgID, noteID string) (*Note, error)
	CreateNote(ctx context.Context, n *Note) error
	UpdateNote(ctx context.Context, n *Note) error
	DeleteNote(ctx context.Context, orgID, noteID string) error
	CountNotes(ctx context.Context, orgID string) (int, error)

	// Tasks
	FindTasksByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Task, error)
	FindTasksByOrg(ctx context.Context, orgID string) ([]*Task, error)
	FindTaskByID(ctx context.Context, orgID, taskID string) (*Task, error)
	FindOverdueTasks(ctx context.Context, orgID string) ([]*Task, error)
	CreateTask(ctx context.Context, t *Task) error
	UpdateTask(ctx context.Context, t *Task) error
	DeleteTask(ctx context.Context, orgID, taskID string) error
	CountTasks(ctx context.Context, orgID string) (int, error)

	// Activities
	FindActivitiesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Activity, error)
	FindActivitiesByOrg(ctx context.Context, orgID string) ([]*Activity, error)
	FindActivityByID(ctx context.Context, orgID, activityID string) (*Activity, error)
	CreateActivity(ctx context.Context, a *Activity) error
	UpdateActivity(ctx context.Context, a *Activity) error
	DeleteActivity(ctx context.Context, orgID, activityID string) error
	CountActivities(ctx context.Context, orgID string) (int, error)
	GetActivityCountByType(ctx context.Context, orgID string) (map[string]int, error)

	// Email Logs
	FindEmailLogsByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*EmailLog, error)
	FindEmailLogsByOrg(ctx context.Context, orgID string) ([]*EmailLog, error)
	FindEmailLogByID(ctx context.Context, orgID, emailID string) (*EmailLog, error)
	CreateEmailLog(ctx context.Context, e *EmailLog) error
	DeleteEmailLog(ctx context.Context, orgID, emailID string) error
	CountEmailLogs(ctx context.Context, orgID string) (int, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new engagement repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// ============================================================
// Notes
// ============================================================

const noteCols = `id, public_id, org_id, module, content, related_type, related_id, created_by, created_at, updated_at`

func scanNote(row interface{ Scan(...any) error }, n *Note) error {
	return row.Scan(
		&n.ID, &n.PublicID, &n.OrgID, &n.Module, &n.Content,
		&n.RelatedType, &n.RelatedID, &n.CreatedBy, &n.CreatedAt, &n.UpdatedAt,
	)
}

func (r *repoImpl) FindNotesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Note, error) {
	q := `SELECT ` + noteCols + `
		FROM platform_notes
		WHERE org_id = $1 AND related_type = $2 AND related_id = $3
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindNotesByRelated: %w", err)
	}
	defer rows.Close()

	var out []*Note
	for rows.Next() {
		n := &Note{}
		if err := scanNote(rows, n); err != nil {
			return nil, fmt.Errorf("engagement: FindNotesByRelated: scan: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindNoteByID(ctx context.Context, orgID, noteID string) (*Note, error) {
	q := `SELECT ` + noteCols + ` FROM platform_notes WHERE org_id = $1 AND id = $2`
	n := &Note{}
	err := scanNote(r.db.QueryRow(ctx, q, orgID, noteID), n)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("engagement: FindNoteByID: %w", err)
	}
	return n, nil
}

func (r *repoImpl) CreateNote(ctx context.Context, n *Note) error {
	const q = `
		INSERT INTO platform_notes (org_id, module, content, related_type, related_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		n.OrgID, n.Module, n.Content, n.RelatedType, n.RelatedID, n.CreatedBy,
	).Scan(&n.ID, &n.PublicID, &n.CreatedAt, &n.UpdatedAt)
}

func (r *repoImpl) UpdateNote(ctx context.Context, n *Note) error {
	const q = `
		UPDATE platform_notes SET content = $1, updated_at = NOW()
		WHERE org_id = $2 AND id = $3
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q, n.Content, n.OrgID, n.ID).Scan(&n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoteNotFound
	}
	return err
}

func (r *repoImpl) DeleteNote(ctx context.Context, orgID, noteID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM platform_notes WHERE org_id = $1 AND id = $2`, orgID, noteID)
	if err != nil {
		return fmt.Errorf("engagement: DeleteNote: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNoteNotFound
	}
	return nil
}

func (r *repoImpl) CountNotes(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_notes WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

// ============================================================
// Tasks
// ============================================================

const taskCols = `
	id, public_id, org_id, module, title, description, due_date, status, priority,
	related_type, related_id, assigned_to, created_by, completed_at, created_at, updated_at`

func scanTask(row interface{ Scan(...any) error }, t *Task) error {
	return row.Scan(
		&t.ID, &t.PublicID, &t.OrgID, &t.Module, &t.Title, &t.Description,
		&t.DueDate, &t.Status, &t.Priority,
		&t.RelatedType, &t.RelatedID, &t.AssignedTo,
		&t.CreatedBy, &t.CompletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
}

func (r *repoImpl) FindTasksByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Task, error) {
	q := `SELECT ` + taskCols + `
		FROM platform_tasks
		WHERE org_id = $1 AND related_type = $2 AND related_id = $3
		ORDER BY due_date ASC NULLS LAST, created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindTasksByRelated: %w", err)
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t := &Task{}
		if err := scanTask(rows, t); err != nil {
			return nil, fmt.Errorf("engagement: FindTasksByRelated: scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindTasksByOrg(ctx context.Context, orgID string) ([]*Task, error) {
	q := `SELECT ` + taskCols + `
		FROM platform_tasks WHERE org_id = $1
		ORDER BY due_date ASC NULLS LAST, created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindTasksByOrg: %w", err)
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t := &Task{}
		if err := scanTask(rows, t); err != nil {
			return nil, fmt.Errorf("engagement: FindTasksByOrg: scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindTaskByID(ctx context.Context, orgID, taskID string) (*Task, error) {
	q := `SELECT ` + taskCols + ` FROM platform_tasks WHERE org_id = $1 AND id = $2`
	t := &Task{}
	err := scanTask(r.db.QueryRow(ctx, q, orgID, taskID), t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("engagement: FindTaskByID: %w", err)
	}
	return t, nil
}

func (r *repoImpl) FindOverdueTasks(ctx context.Context, orgID string) ([]*Task, error) {
	q := `SELECT ` + taskCols + `
		FROM platform_tasks
		WHERE org_id = $1 AND status = 'open' AND due_date < NOW()
		ORDER BY due_date ASC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindOverdueTasks: %w", err)
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t := &Task{}
		if err := scanTask(rows, t); err != nil {
			return nil, fmt.Errorf("engagement: FindOverdueTasks: scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *repoImpl) CreateTask(ctx context.Context, t *Task) error {
	const q = `
		INSERT INTO platform_tasks
		    (org_id, module, title, description, due_date, status, priority,
		     related_type, related_id, assigned_to, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		t.OrgID, t.Module, t.Title, t.Description, t.DueDate,
		t.Status, t.Priority, t.RelatedType, t.RelatedID,
		t.AssignedTo, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) UpdateTask(ctx context.Context, t *Task) error {
	const q = `
		UPDATE platform_tasks
		SET title = $1, description = $2, due_date = $3, status = $4, priority = $5,
		    assigned_to = $6, completed_at = $7, updated_at = NOW()
		WHERE org_id = $8 AND id = $9
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		t.Title, t.Description, t.DueDate, t.Status, t.Priority,
		t.AssignedTo, t.CompletedAt,
		t.OrgID, t.ID,
	).Scan(&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTaskNotFound
	}
	return err
}

func (r *repoImpl) DeleteTask(ctx context.Context, orgID, taskID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM platform_tasks WHERE org_id = $1 AND id = $2`, orgID, taskID)
	if err != nil {
		return fmt.Errorf("engagement: DeleteTask: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (r *repoImpl) CountTasks(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_tasks WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

// ============================================================
// Activities
// ============================================================

const activityCols = `
	id, public_id, org_id, module, type, subject, description, outcome,
	related_type, related_id, occurred_at, duration_mins, created_by, created_at, updated_at`

func scanActivity(row interface{ Scan(...any) error }, a *Activity) error {
	return row.Scan(
		&a.ID, &a.PublicID, &a.OrgID, &a.Module, &a.Type, &a.Subject,
		&a.Description, &a.Outcome, &a.RelatedType, &a.RelatedID,
		&a.OccurredAt, &a.DurationMins, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
}

func (r *repoImpl) FindActivitiesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Activity, error) {
	q := `SELECT ` + activityCols + `
		FROM platform_activities
		WHERE org_id = $1 AND related_type = $2 AND related_id = $3
		ORDER BY occurred_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindActivitiesByRelated: %w", err)
	}
	defer rows.Close()
	var out []*Activity
	for rows.Next() {
		a := &Activity{}
		if err := scanActivity(rows, a); err != nil {
			return nil, fmt.Errorf("engagement: FindActivitiesByRelated: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindActivitiesByOrg(ctx context.Context, orgID string) ([]*Activity, error) {
	q := `SELECT ` + activityCols + `
		FROM platform_activities WHERE org_id = $1
		ORDER BY occurred_at DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindActivitiesByOrg: %w", err)
	}
	defer rows.Close()
	var out []*Activity
	for rows.Next() {
		a := &Activity{}
		if err := scanActivity(rows, a); err != nil {
			return nil, fmt.Errorf("engagement: FindActivitiesByOrg: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindActivityByID(ctx context.Context, orgID, activityID string) (*Activity, error) {
	q := `SELECT ` + activityCols + ` FROM platform_activities WHERE org_id = $1 AND id = $2`
	a := &Activity{}
	err := scanActivity(r.db.QueryRow(ctx, q, orgID, activityID), a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("engagement: FindActivityByID: %w", err)
	}
	return a, nil
}

func (r *repoImpl) CreateActivity(ctx context.Context, a *Activity) error {
	const q = `
		INSERT INTO platform_activities
		    (org_id, module, type, subject, description, outcome,
		     related_type, related_id, occurred_at, duration_mins, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		a.OrgID, a.Module, a.Type, a.Subject, a.Description, a.Outcome,
		a.RelatedType, a.RelatedID, a.OccurredAt, a.DurationMins, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *repoImpl) UpdateActivity(ctx context.Context, a *Activity) error {
	const q = `
		UPDATE platform_activities
		SET type = $1, subject = $2, description = $3, outcome = $4,
		    occurred_at = $5, duration_mins = $6, updated_at = NOW()
		WHERE org_id = $7 AND id = $8
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		a.Type, a.Subject, a.Description, a.Outcome,
		a.OccurredAt, a.DurationMins, a.OrgID, a.ID,
	).Scan(&a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrActivityNotFound
	}
	return err
}

func (r *repoImpl) DeleteActivity(ctx context.Context, orgID, activityID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM platform_activities WHERE org_id = $1 AND id = $2`, orgID, activityID)
	if err != nil {
		return fmt.Errorf("engagement: DeleteActivity: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrActivityNotFound
	}
	return nil
}

func (r *repoImpl) CountActivities(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_activities WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

func (r *repoImpl) GetActivityCountByType(ctx context.Context, orgID string) (map[string]int, error) {
	const q = `
		SELECT type, COUNT(*) FROM platform_activities
		WHERE org_id = $1 GROUP BY type ORDER BY COUNT(*) DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetActivityCountByType: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var t string
		var count int
		if err := rows.Scan(&t, &count); err != nil {
			return nil, fmt.Errorf("engagement: GetActivityCountByType: scan: %w", err)
		}
		result[t] = count
	}
	return result, rows.Err()
}

// ============================================================
// Email Logs
// ============================================================

const emailCols = `
	id, public_id, org_id, module, subject, body, from_email, to_email,
	direction, status, related_type, related_id, sent_at, created_by, created_at`

func scanEmailLog(row interface{ Scan(...any) error }, e *EmailLog) error {
	return row.Scan(
		&e.ID, &e.PublicID, &e.OrgID, &e.Module, &e.Subject, &e.Body,
		&e.FromEmail, &e.ToEmail, &e.Direction, &e.Status,
		&e.RelatedType, &e.RelatedID, &e.SentAt, &e.CreatedBy, &e.CreatedAt,
	)
}

func (r *repoImpl) FindEmailLogsByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*EmailLog, error) {
	q := `SELECT ` + emailCols + `
		FROM platform_email_logs
		WHERE org_id = $1 AND related_type = $2 AND related_id = $3
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindEmailLogsByRelated: %w", err)
	}
	defer rows.Close()
	var out []*EmailLog
	for rows.Next() {
		e := &EmailLog{}
		if err := scanEmailLog(rows, e); err != nil {
			return nil, fmt.Errorf("engagement: FindEmailLogsByRelated: scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindEmailLogsByOrg(ctx context.Context, orgID string) ([]*EmailLog, error) {
	q := `SELECT ` + emailCols + `
		FROM platform_email_logs WHERE org_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: FindEmailLogsByOrg: %w", err)
	}
	defer rows.Close()
	var out []*EmailLog
	for rows.Next() {
		e := &EmailLog{}
		if err := scanEmailLog(rows, e); err != nil {
			return nil, fmt.Errorf("engagement: FindEmailLogsByOrg: scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindEmailLogByID(ctx context.Context, orgID, emailID string) (*EmailLog, error) {
	q := `SELECT ` + emailCols + ` FROM platform_email_logs WHERE org_id = $1 AND id = $2`
	e := &EmailLog{}
	err := scanEmailLog(r.db.QueryRow(ctx, q, orgID, emailID), e)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("engagement: FindEmailLogByID: %w", err)
	}
	return e, nil
}

func (r *repoImpl) CreateEmailLog(ctx context.Context, e *EmailLog) error {
	const q = `
		INSERT INTO platform_email_logs
		    (org_id, module, subject, body, from_email, to_email,
		     direction, status, related_type, related_id, sent_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, public_id, created_at`

	return r.db.QueryRow(ctx, q,
		e.OrgID, e.Module, e.Subject, e.Body, e.FromEmail, e.ToEmail,
		e.Direction, e.Status, e.RelatedType, e.RelatedID, e.SentAt, e.CreatedBy,
	).Scan(&e.ID, &e.PublicID, &e.CreatedAt)
}

func (r *repoImpl) DeleteEmailLog(ctx context.Context, orgID, emailID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM platform_email_logs WHERE org_id = $1 AND id = $2`, orgID, emailID)
	if err != nil {
		return fmt.Errorf("engagement: DeleteEmailLog: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrEmailLogNotFound
	}
	return nil
}

func (r *repoImpl) CountEmailLogs(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_email_logs WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

// ensure time import is used
var _ = time.Now
