// backend/internal/hrm/announcements/repository.go
package announcements

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID, category, status string) ([]*Announcement, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Announcement, error)
	Create(ctx context.Context, a *Announcement) error
	Update(ctx context.Context, a *Announcement) error
	UpdateStatus(ctx context.Context, id string, status AnnStatus, publishedAt *interface{}) error
	// GetTargetEmployeeIDs returns the employee UUIDs for a scoped announcement.
	GetTargetEmployeeIDs(ctx context.Context, orgID string, scopeType ScopeType, scopeIDs []string) ([]string, error)
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, title, content, category,
	scope_type, scope_ids,
	scheduled_at, published_at, expires_at,
	requires_acknowledgement, to_char(acknowledgement_deadline,'YYYY-MM-DD'),
	is_pinned, pin_order, author_id, status, created_by, created_at, updated_at`

func scan(row pgx.Row) (*Announcement, error) {
	a := &Announcement{}
	err := row.Scan(
		&a.ID, &a.PublicID, &a.OrgID, &a.Title, &a.Content, &a.Category,
		&a.ScopeType, &a.ScopeIDs,
		&a.ScheduledAt, &a.PublishedAt, &a.ExpiresAt,
		&a.RequiresAcknowledgement, &a.AcknowledgementDeadline,
		&a.IsPinned, &a.PinOrder, &a.AuthorID, &a.Status, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	if a.ScopeIDs == nil { a.ScopeIDs = []string{} }
	return a, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, category, status string) ([]*Announcement, error) {
	q := `SELECT ` + sel + ` FROM hrm_announcements WHERE org_id=$1`
	args := []any{orgID}
	if category != "" { args = append(args, category); q += fmt.Sprintf(` AND category=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	q += ` ORDER BY is_pinned DESC, pin_order, created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("announcements: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*Announcement, 0)
	for rows.Next() { a, err := scan(rows); if err != nil { return nil, err }; list = append(list, a) }
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Announcement, error) {
	return scan(r.db.QueryRow(ctx,
		`SELECT `+sel+` FROM hrm_announcements WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) Create(ctx context.Context, a *Announcement) error {
	if a.ScopeIDs == nil { a.ScopeIDs = []string{} }
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_announcements
		(org_id, title, content, category, scope_type, scope_ids,
		 scheduled_at, expires_at, requires_acknowledgement, acknowledgement_deadline,
		 is_pinned, pin_order, author_id, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::date,$11,$12,$13,$14,$15)
		RETURNING id, public_id, created_at, updated_at`,
		a.OrgID, a.Title, a.Content, a.Category, a.ScopeType, a.ScopeIDs,
		a.ScheduledAt, a.ExpiresAt, a.RequiresAcknowledgement, a.AcknowledgementDeadline,
		a.IsPinned, a.PinOrder, a.AuthorID, a.Status, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, a *Announcement) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_announcements SET
		title=$1, content=$2, category=$3, scope_ids=$4,
		scheduled_at=$5, expires_at=$6,
		requires_acknowledgement=$7, acknowledgement_deadline=$8::date,
		is_pinned=$9, pin_order=$10, updated_at=NOW()
		WHERE id=$11 AND org_id=$12 RETURNING updated_at`,
		a.Title, a.Content, a.Category, a.ScopeIDs,
		a.ScheduledAt, a.ExpiresAt,
		a.RequiresAcknowledgement, a.AcknowledgementDeadline,
		a.IsPinned, a.PinOrder, a.ID, a.OrgID,
	).Scan(&a.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status AnnStatus, publishedAt *interface{}) error {
	q := `UPDATE hrm_announcements SET status=$1`
	if publishedAt != nil { q += `, published_at=NOW()` }
	q += `, updated_at=NOW() WHERE id=$2`
	_, err := r.db.Exec(ctx, q, status, id)
	return err
}

// GetTargetEmployeeIDs resolves the target employees based on announcement scope.
func (r *repoImpl) GetTargetEmployeeIDs(ctx context.Context, orgID string, scopeType ScopeType, scopeIDs []string) ([]string, error) {
	var q string
	var args []any
	switch scopeType {
	// Active employees resolve through hrm_employee_statuses.category. There is
	// no hrm_employees.status column — migration 00053 replaced it with
	// status_id, and querying the dropped column failed 42703, taking the whole
	// audience resolution with it.
	case ScopeOrganization:
		q = `SELECT e.id::text FROM hrm_employees e JOIN hrm_employee_statuses es ON es.id=e.status_id WHERE e.org_id=$1 AND es.category='active'`
		args = []any{orgID}
	case ScopeDepartment:
		q = `SELECT e.id::text FROM hrm_employees e JOIN hrm_employee_statuses es ON es.id=e.status_id WHERE e.org_id=$1 AND es.category='active' AND e.department_id = ANY($2::uuid[])`
		args = []any{orgID, scopeIDs}
	case ScopeIndividual:
		q = `SELECT id::text FROM hrm_employees WHERE org_id=$1 AND id::text = ANY($2)`
		args = []any{orgID, scopeIDs}
	}
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var ids []string
	for rows.Next() { var id string; _ = rows.Scan(&id); ids = append(ids, id) }
	return ids, rows.Err()
}
