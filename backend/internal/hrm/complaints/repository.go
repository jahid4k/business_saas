// backend/internal/hrm/complaints/repository.go
package complaints

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Complaint, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error)
	Create(ctx context.Context, c *Complaint) error
	Update(ctx context.Context, c *Complaint) error
	UpdateStatus(ctx context.Context, id string, status ComplaintStatus) error
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id, is_anonymous, complaint_type,
	title, description, to_char(incident_date,'YYYY-MM-DD'),
	against_employee_id, against_details,
	investigator_id, investigation_notes, investigation_started_at,
	resolution, resolution_action, resolved_at, resolved_by,
	document_id, status, created_by, created_at, updated_at`

func scan(row pgx.Row) (*Complaint, error) {
	c := &Complaint{}
	err := row.Scan(
		&c.ID, &c.PublicID, &c.OrgID, &c.EmployeeID, &c.IsAnonymous, &c.ComplaintType,
		&c.Title, &c.Description, &c.IncidentDate,
		&c.AgainstEmployeeID, &c.AgainstDetails,
		&c.InvestigatorID, &c.InvestigationNotes, &c.InvestigationStartedAt,
		&c.Resolution, &c.ResolutionAction, &c.ResolvedAt, &c.ResolvedBy,
		&c.DocumentID, &c.Status, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return c, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Complaint, error) {
	q := `SELECT ` + sel + ` FROM hrm_complaints WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	q += ` ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("complaints: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*Complaint, 0)
	for rows.Next() { c, err := scan(rows); if err != nil { return nil, err }; list = append(list, c) }
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error) {
	q := `SELECT ` + sel + ` FROM hrm_complaints WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	args := []any{orgID, ref}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	return scan(r.db.QueryRow(ctx, q, args...))
}

func (r *repoImpl) Create(ctx context.Context, c *Complaint) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_complaints
		(org_id, employee_id, is_anonymous, complaint_type, title, description,
		 incident_date, against_employee_id, against_details, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7::date,$8,$9,$10,$11)
		RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.EmployeeID, c.IsAnonymous, c.ComplaintType, c.Title, c.Description,
		c.IncidentDate, c.AgainstEmployeeID, c.AgainstDetails, c.Status, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, c *Complaint) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_complaints SET
		title=$1, description=$2, incident_date=$3::date, against_details=$4, document_id=$5,
		investigator_id=$6, investigation_notes=$7, investigation_started_at=$8,
		resolution=$9, resolution_action=$10, resolved_at=$11, resolved_by=$12,
		updated_at=NOW()
		WHERE id=$13 AND org_id=$14 RETURNING updated_at`,
		c.Title, c.Description, c.IncidentDate, c.AgainstDetails, c.DocumentID,
		c.InvestigatorID, c.InvestigationNotes, c.InvestigationStartedAt,
		c.Resolution, c.ResolutionAction, c.ResolvedAt, c.ResolvedBy,
		c.ID, c.OrgID,
	).Scan(&c.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status ComplaintStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_complaints SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}
