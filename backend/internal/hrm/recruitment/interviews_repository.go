// backend/internal/hrm/recruitment/interviews_repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// InterviewRepository is embedded into Repository — see repository.go.
type InterviewRepository interface {
	FindInterviews(ctx context.Context, orgID, applicationID string) ([]*Interview, error)
	FindInterviewByRef(ctx context.Context, orgID, ref string) (*Interview, error)
	CreateInterview(ctx context.Context, i *Interview) error
	UpdateInterview(ctx context.Context, i *Interview) error
	DeleteInterview(ctx context.Context, orgID, interviewID string) error

	FindPanelists(ctx context.Context, interviewID string) ([]*Panelist, error)
	FindPanelist(ctx context.Context, interviewID, employeeID string) (*Panelist, error)
	AddPanelist(ctx context.Context, p *Panelist) error
	RemovePanelist(ctx context.Context, interviewID, employeeID string) error

	// FindEmployeeIDByUserID resolves the caller's own hrm_employees.id from
	// their platform user_id — the internal/hrm/scope Predicate precedent
	// (`SELECT id FROM hrm_employees WHERE org_id = $1 AND user_id = $2`).
	// Returns "" (no error) when the caller has no employee row — a valid
	// state, not a failure, for a non-employee admin acting on the org.
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

const interviewCols = `id, public_id, org_id, application_id, scheduled_at, duration_minutes, mode,
	location, meeting_url, status, outcome, notes, created_by, created_at, updated_at`

func scanInterview(row interface{ Scan(...any) error }, i *Interview) error {
	return row.Scan(
		&i.ID, &i.PublicID, &i.OrgID, &i.ApplicationID, &i.ScheduledAt, &i.DurationMinutes, &i.Mode,
		&i.Location, &i.MeetingURL, &i.Status, &i.Outcome, &i.Notes, &i.CreatedBy, &i.CreatedAt, &i.UpdatedAt,
	)
}

func (r *repoImpl) FindInterviews(ctx context.Context, orgID, applicationID string) ([]*Interview, error) {
	q := `SELECT ` + interviewCols + ` FROM hrm_interviews WHERE org_id = $1 AND application_id = $2 ORDER BY scheduled_at ASC`
	rows, err := r.db.Query(ctx, q, orgID, applicationID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindInterviews: %w", err)
	}
	defer rows.Close()
	list := make([]*Interview, 0)
	for rows.Next() {
		i := &Interview{}
		if err := scanInterview(rows, i); err != nil {
			return nil, fmt.Errorf("recruitment: FindInterviews: scan: %w", err)
		}
		list = append(list, i)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindInterviewByRef(ctx context.Context, orgID, ref string) (*Interview, error) {
	q := `SELECT ` + interviewCols + ` FROM hrm_interviews WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	i := &Interview{}
	err := scanInterview(r.db.QueryRow(ctx, q, orgID, ref), i)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindInterviewByRef: %w", err)
	}
	return i, nil
}

func (r *repoImpl) CreateInterview(ctx context.Context, i *Interview) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_interviews (org_id, application_id, scheduled_at, duration_minutes, mode, location, meeting_url, notes, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, status, created_at, updated_at`,
		i.OrgID, i.ApplicationID, i.ScheduledAt, i.DurationMinutes, i.Mode, i.Location, i.MeetingURL, i.Notes, i.CreatedBy,
	).Scan(&i.ID, &i.PublicID, &i.Status, &i.CreatedAt, &i.UpdatedAt)
}

func (r *repoImpl) UpdateInterview(ctx context.Context, i *Interview) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_interviews SET
		    scheduled_at = $1, duration_minutes = $2, mode = $3, location = $4, meeting_url = $5,
		    status = $6, outcome = $7, notes = $8, updated_at = NOW()
		 WHERE id = $9 AND org_id = $10
		 RETURNING updated_at`,
		i.ScheduledAt, i.DurationMinutes, i.Mode, i.Location, i.MeetingURL, i.Status, i.Outcome, i.Notes, i.ID, i.OrgID,
	).Scan(&i.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInterviewNotFound
	}
	return err
}

func (r *repoImpl) DeleteInterview(ctx context.Context, orgID, interviewID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_interviews WHERE org_id = $1 AND id = $2`, orgID, interviewID)
	if err != nil {
		return fmt.Errorf("recruitment: DeleteInterview: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrInterviewNotFound
	}
	return nil
}

const panelistCols = `id, public_id, interview_id, employee_id, panelist_role, is_lead, created_at`

func scanPanelist(row interface{ Scan(...any) error }, p *Panelist) error {
	return row.Scan(&p.ID, &p.PublicID, &p.InterviewID, &p.EmployeeID, &p.PanelistRole, &p.IsLead, &p.CreatedAt)
}

func (r *repoImpl) FindPanelists(ctx context.Context, interviewID string) ([]*Panelist, error) {
	q := `SELECT ` + panelistCols + ` FROM hrm_interview_panelists WHERE interview_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, interviewID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindPanelists: %w", err)
	}
	defer rows.Close()
	list := make([]*Panelist, 0)
	for rows.Next() {
		p := &Panelist{}
		if err := scanPanelist(rows, p); err != nil {
			return nil, fmt.Errorf("recruitment: FindPanelists: scan: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindPanelist(ctx context.Context, interviewID, employeeID string) (*Panelist, error) {
	q := `SELECT ` + panelistCols + ` FROM hrm_interview_panelists WHERE interview_id = $1 AND employee_id = $2`
	p := &Panelist{}
	err := scanPanelist(r.db.QueryRow(ctx, q, interviewID, employeeID), p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindPanelist: %w", err)
	}
	return p, nil
}

// AddPanelist does not itself guard against the uq_hrm_panl_interview_employee
// duplicate — the service checks FindPanelist first (the SlugExists
// precedent), so this stays a plain insert.
func (r *repoImpl) AddPanelist(ctx context.Context, p *Panelist) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_interview_panelists (interview_id, employee_id, panelist_role, is_lead)
		 VALUES ($1,$2,$3,$4) RETURNING id, public_id, created_at`,
		p.InterviewID, p.EmployeeID, p.PanelistRole, p.IsLead,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt)
}

func (r *repoImpl) RemovePanelist(ctx context.Context, interviewID, employeeID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_interview_panelists WHERE interview_id = $1 AND employee_id = $2`, interviewID, employeeID)
	if err != nil {
		return fmt.Errorf("recruitment: RemovePanelist: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrPanelistNotFound
	}
	return nil
}

func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var employeeID string
	err := r.db.QueryRow(ctx, `SELECT id FROM hrm_employees WHERE org_id = $1 AND user_id = $2`, orgID, userID).Scan(&employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("recruitment: FindEmployeeIDByUserID: %w", err)
	}
	return employeeID, nil
}
