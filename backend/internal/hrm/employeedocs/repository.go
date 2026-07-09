// backend/internal/hrm/employeedocs/repository.go
package employeedocs

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID, status, relatedType string) ([]*EmployeeDocument, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error)
	Create(ctx context.Context, d *EmployeeDocument) error
	UpdateStatus(ctx context.Context, id string, status DocStatus) error
	Acknowledge(ctx context.Context, id string, note, signature *string) error
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id, template_id,
	title, document_type, file_url, file_name, file_size_bytes, mime_type,
	generated_content, related_type, related_id::text,
	version, superseded_by, bulk_send_batch_id,
	to_char(expiry_date,'YYYY-MM-DD'),
	status, issued_by, sent_at, acknowledged_at, acknowledgement_note,
	created_by, created_at, updated_at`

func scanDoc(row pgx.Row) (*EmployeeDocument, error) {
	d := &EmployeeDocument{}
	err := row.Scan(
		&d.ID, &d.PublicID, &d.OrgID, &d.EmployeeID, &d.TemplateID,
		&d.Title, &d.DocumentType, &d.FileURL, &d.FileName, &d.FileSizeBytes, &d.MimeType,
		&d.GeneratedContent, &d.RelatedType, &d.RelatedID,
		&d.Version, &d.SupersededBy, &d.BulkSendBatchID,
		&d.ExpiryDate,
		&d.Status, &d.IssuedBy, &d.SentAt, &d.AcknowledgedAt, &d.AcknowledgementNote,
		&d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return d, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID, status, relatedType string) ([]*EmployeeDocument, error) {
	q := `SELECT ` + sel + ` FROM hrm_employee_documents WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	if relatedType != "" { args = append(args, relatedType); q += fmt.Sprintf(` AND related_type=$%d`, len(args)) }
	q += ` ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("employeedocs: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*EmployeeDocument, 0)
	for rows.Next() { d, err := scanDoc(rows); if err != nil { return nil, err }; list = append(list, d) }
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error) {
	q := `SELECT ` + sel + ` FROM hrm_employee_documents WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	args := []any{orgID, ref}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	return scanDoc(r.db.QueryRow(ctx, q, args...))
}

func (r *repoImpl) Create(ctx context.Context, d *EmployeeDocument) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_employee_documents
		(org_id, employee_id, template_id, title, document_type, file_url, file_name,
		 file_size_bytes, mime_type, related_type, related_id, expiry_date, status, issued_by, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::uuid,$12::date,$13,$14,$15)
		RETURNING id, public_id, version, created_at, updated_at`,
		d.OrgID, d.EmployeeID, d.TemplateID, d.Title, d.DocumentType, d.FileURL, d.FileName,
		d.FileSizeBytes, d.MimeType, d.RelatedType, d.RelatedID, d.ExpiryDate, d.Status, d.IssuedBy, d.CreatedBy,
	).Scan(&d.ID, &d.PublicID, &d.Version, &d.CreatedAt, &d.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status DocStatus) error {
	extra := ""
	if status == StatusSent { extra = `, sent_at=NOW()` }
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_employee_documents SET status=$1`+extra+`, updated_at=NOW() WHERE id=$2`,
		status, id)
	return err
}

func (r *repoImpl) Acknowledge(ctx context.Context, id string, note, signature *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_employee_documents SET
		status='acknowledged', acknowledged_at=NOW(),
		acknowledgement_note=$1, updated_at=NOW()
		WHERE id=$2`,
		note, id)
	return err
}
