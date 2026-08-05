// backend/internal/hrm/recruitment/applications_repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ApplicationRepository is embedded into Repository — see repository.go.
type ApplicationRepository interface {
	FindApplications(ctx context.Context, orgID string, filter ApplicationListFilter) ([]*Application, error)
	CountApplications(ctx context.Context, orgID string, filter ApplicationListFilter) (int, error)
	FindApplicationByRef(ctx context.Context, orgID, ref string) (*Application, error)
	FindActiveApplication(ctx context.Context, orgID, candidateID, postingID string) (*Application, error)
	// CreateApplication inserts the application AND its initial placement
	// history row (from_stage_id NULL) in one transaction — every
	// application gets a stage_history trail from the moment it exists.
	CreateApplication(ctx context.Context, app *Application, initialStageName string) error
	// MoveApplicationStage is the atomicity-critical write: it locks the
	// application row, computes seconds_in_previous_stage from the most
	// recent history entry (or applied_at if this is the first move),
	// updates stage_id (+ status/hired_at/rejected_at when newStatus is
	// terminal), and inserts the history row — all in one transaction.
	MoveApplicationStage(ctx context.Context, orgID, applicationID, toStageID, toStageName string, newStatus ApplicationStatus, movedBy, note *string) (*Application, *ApplicationStageHistory, error)
	UpdateApplicationStatus(ctx context.Context, orgID, applicationID string, status ApplicationStatus, reason *string) (*Application, error)
	FindStageHistory(ctx context.Context, orgID, applicationID string) ([]*ApplicationStageHistory, error)
}

const applicationCols = `id, public_id, org_id, candidate_id, posting_id, pipeline_id, stage_id, status,
	rejection_reason, rejected_at, withdrawn_at, hired_at, converted_employee_id, cover_letter, source,
	applied_at, created_by, created_at, updated_at`

func scanApplication(row interface{ Scan(...any) error }, a *Application) error {
	return row.Scan(
		&a.ID, &a.PublicID, &a.OrgID, &a.CandidateID, &a.PostingID, &a.PipelineID, &a.StageID, &a.Status,
		&a.RejectionReason, &a.RejectedAt, &a.WithdrawnAt, &a.HiredAt, &a.ConvertedEmployeeID, &a.CoverLetter, &a.Source,
		&a.AppliedAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
}

func buildApplicationsWhere(orgID string, filter ApplicationListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.CandidateID != "" {
		args = append(args, filter.CandidateID)
		clauses = append(clauses, fmt.Sprintf("candidate_id = $%d", len(args)))
	}
	if filter.PostingID != "" {
		args = append(args, filter.PostingID)
		clauses = append(clauses, fmt.Sprintf("posting_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindApplications(ctx context.Context, orgID string, filter ApplicationListFilter) ([]*Application, error) {
	where, args := buildApplicationsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_applications WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		applicationCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindApplications: %w", err)
	}
	defer rows.Close()
	list := make([]*Application, 0)
	for rows.Next() {
		a := &Application{}
		if err := scanApplication(rows, a); err != nil {
			return nil, fmt.Errorf("recruitment: FindApplications: scan: %w", err)
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountApplications(ctx context.Context, orgID string, filter ApplicationListFilter) (int, error) {
	where, args := buildApplicationsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_applications WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("recruitment: CountApplications: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindApplicationByRef(ctx context.Context, orgID, ref string) (*Application, error) {
	q := `SELECT ` + applicationCols + ` FROM hrm_applications WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	a := &Application{}
	err := scanApplication(r.db.QueryRow(ctx, q, orgID, ref), a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindApplicationByRef: %w", err)
	}
	return a, nil
}

func (r *repoImpl) FindActiveApplication(ctx context.Context, orgID, candidateID, postingID string) (*Application, error) {
	q := `SELECT ` + applicationCols + ` FROM hrm_applications
		WHERE org_id = $1 AND candidate_id = $2 AND posting_id = $3 AND status <> 'withdrawn'`
	a := &Application{}
	err := scanApplication(r.db.QueryRow(ctx, q, orgID, candidateID, postingID), a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindActiveApplication: %w", err)
	}
	return a, nil
}

func (r *repoImpl) CreateApplication(ctx context.Context, app *Application, initialStageName string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("recruitment: CreateApplication: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx,
		`INSERT INTO hrm_applications
		    (org_id, candidate_id, posting_id, pipeline_id, stage_id, status, cover_letter, source, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, applied_at, created_at, updated_at`,
		app.OrgID, app.CandidateID, app.PostingID, app.PipelineID, app.StageID, app.Status, app.CoverLetter, app.Source, app.CreatedBy,
	).Scan(&app.ID, &app.PublicID, &app.AppliedAt, &app.CreatedAt, &app.UpdatedAt); err != nil {
		return fmt.Errorf("recruitment: CreateApplication: insert: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO hrm_application_stage_history (application_id, from_stage_id, to_stage_id, from_stage_name, to_stage_name, moved_by, seconds_in_previous_stage, note)
		 VALUES ($1, NULL, $2, NULL, $3, $4, NULL, 'Initial application')`,
		app.ID, app.StageID, initialStageName, app.CreatedBy,
	); err != nil {
		return fmt.Errorf("recruitment: CreateApplication: initial history: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *repoImpl) MoveApplicationStage(ctx context.Context, orgID, applicationID, toStageID, toStageName string, newStatus ApplicationStatus, movedBy, note *string) (*Application, *ApplicationStageHistory, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("recruitment: MoveApplicationStage: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var fromStageID string
	var appliedAt time.Time
	err = tx.QueryRow(ctx,
		`SELECT stage_id, applied_at FROM hrm_applications WHERE id = $1 AND org_id = $2 FOR UPDATE`,
		applicationID, orgID,
	).Scan(&fromStageID, &appliedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrApplicationNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("recruitment: MoveApplicationStage: lock: %w", err)
	}

	var fromStageName string
	if err := tx.QueryRow(ctx, `SELECT name FROM hrm_recruitment_stages WHERE id = $1`, fromStageID).Scan(&fromStageName); err != nil {
		return nil, nil, fmt.Errorf("recruitment: MoveApplicationStage: from-stage name: %w", err)
	}

	// seconds_in_previous_stage is computed inside this same transaction,
	// under the row lock above, so a concurrent move cannot race the
	// baseline it's measured from — the single most important correctness
	// property of this method (see migration 00078's header).
	var lastMovedAt *time.Time
	if err := tx.QueryRow(ctx,
		`SELECT MAX(moved_at) FROM hrm_application_stage_history WHERE application_id = $1`,
		applicationID,
	).Scan(&lastMovedAt); err != nil {
		return nil, nil, fmt.Errorf("recruitment: MoveApplicationStage: baseline: %w", err)
	}
	baseline := appliedAt
	if lastMovedAt != nil {
		baseline = *lastMovedAt
	}
	now := time.Now()
	seconds := int64(now.Sub(baseline).Seconds())

	var hiredAt, rejectedAt *time.Time
	if newStatus == ApplicationStatusHired {
		hiredAt = &now
	}
	if newStatus == ApplicationStatusRejected {
		rejectedAt = &now
	}

	app := &Application{}
	if err := tx.QueryRow(ctx,
		`UPDATE hrm_applications SET
		    stage_id = $1, status = $2,
		    hired_at = COALESCE($3, hired_at),
		    rejected_at = COALESCE($4, rejected_at),
		    updated_at = NOW()
		 WHERE id = $5
		 RETURNING `+applicationCols,
		toStageID, newStatus, hiredAt, rejectedAt, applicationID,
	).Scan(
		&app.ID, &app.PublicID, &app.OrgID, &app.CandidateID, &app.PostingID, &app.PipelineID, &app.StageID, &app.Status,
		&app.RejectionReason, &app.RejectedAt, &app.WithdrawnAt, &app.HiredAt, &app.ConvertedEmployeeID, &app.CoverLetter, &app.Source,
		&app.AppliedAt, &app.CreatedBy, &app.CreatedAt, &app.UpdatedAt,
	); err != nil {
		return nil, nil, fmt.Errorf("recruitment: MoveApplicationStage: update: %w", err)
	}

	hist := &ApplicationStageHistory{
		ApplicationID: applicationID, FromStageID: &fromStageID, ToStageID: &toStageID,
		FromStageName: &fromStageName, ToStageName: toStageName, MovedBy: movedBy,
		SecondsInPreviousStage: &seconds, Note: note,
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO hrm_application_stage_history
		    (application_id, from_stage_id, to_stage_id, from_stage_name, to_stage_name, moved_by, seconds_in_previous_stage, note)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, moved_at`,
		hist.ApplicationID, hist.FromStageID, hist.ToStageID, hist.FromStageName, hist.ToStageName, hist.MovedBy, hist.SecondsInPreviousStage, hist.Note,
	).Scan(&hist.ID, &hist.MovedAt); err != nil {
		return nil, nil, fmt.Errorf("recruitment: MoveApplicationStage: history insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("recruitment: MoveApplicationStage: commit: %w", err)
	}
	return app, hist, nil
}

func (r *repoImpl) UpdateApplicationStatus(ctx context.Context, orgID, applicationID string, status ApplicationStatus, reason *string) (*Application, error) {
	app := &Application{}
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_applications SET
		    status = $1,
		    rejection_reason = CASE WHEN $1 = 'rejected' THEN $2 ELSE rejection_reason END,
		    rejected_at = CASE WHEN $1 = 'rejected' THEN NOW() ELSE rejected_at END,
		    withdrawn_at = CASE WHEN $1 = 'withdrawn' THEN NOW() ELSE withdrawn_at END,
		    updated_at = NOW()
		 WHERE id = $3 AND org_id = $4
		 RETURNING `+applicationCols,
		status, reason, applicationID, orgID,
	).Scan(
		&app.ID, &app.PublicID, &app.OrgID, &app.CandidateID, &app.PostingID, &app.PipelineID, &app.StageID, &app.Status,
		&app.RejectionReason, &app.RejectedAt, &app.WithdrawnAt, &app.HiredAt, &app.ConvertedEmployeeID, &app.CoverLetter, &app.Source,
		&app.AppliedAt, &app.CreatedBy, &app.CreatedAt, &app.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrApplicationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateApplicationStatus: %w", err)
	}
	return app, nil
}

func (r *repoImpl) FindStageHistory(ctx context.Context, orgID, applicationID string) ([]*ApplicationStageHistory, error) {
	const q = `
		SELECT h.id, h.application_id, h.from_stage_id, h.to_stage_id, h.from_stage_name, h.to_stage_name,
		       h.moved_by, h.moved_at, h.seconds_in_previous_stage, h.note
		FROM hrm_application_stage_history h
		JOIN hrm_applications a ON a.id = h.application_id
		WHERE a.org_id = $1 AND h.application_id = $2
		ORDER BY h.moved_at ASC`
	rows, err := r.db.Query(ctx, q, orgID, applicationID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindStageHistory: %w", err)
	}
	defer rows.Close()
	list := make([]*ApplicationStageHistory, 0)
	for rows.Next() {
		h := &ApplicationStageHistory{}
		if err := rows.Scan(&h.ID, &h.ApplicationID, &h.FromStageID, &h.ToStageID, &h.FromStageName, &h.ToStageName,
			&h.MovedBy, &h.MovedAt, &h.SecondsInPreviousStage, &h.Note); err != nil {
			return nil, fmt.Errorf("recruitment: FindStageHistory: scan: %w", err)
		}
		list = append(list, h)
	}
	return list, rows.Err()
}
