// backend/internal/hrm/awards/repository.go
package awards

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Award, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Award, error)
	Create(ctx context.Context, a *Award) error
	Update(ctx context.Context, a *Award) error
	UpdateStatus(ctx context.Context, id string, status AwardStatus) error
	SetAnnouncementID(ctx context.Context, id, announcementID string) error
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id, award_type,
	title, description, points, monetary_value, currency,
	to_char(award_date,'YYYY-MM-DD'), issued_by,
	approval_instance_id, certificate_document_id, announcement_id,
	status, issued_at, created_by, created_at, updated_at`

func scan(row pgx.Row) (*Award, error) {
	a := &Award{}
	err := row.Scan(
		&a.ID, &a.PublicID, &a.OrgID, &a.EmployeeID, &a.AwardType,
		&a.Title, &a.Description, &a.Points, &a.MonetaryValue, &a.Currency,
		&a.AwardDate, &a.IssuedBy,
		&a.ApprovalInstanceID, &a.CertificateDocumentID, &a.AnnouncementID,
		&a.Status, &a.IssuedAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return a, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Award, error) {
	q := `SELECT ` + sel + ` FROM hrm_awards WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	q += ` ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("awards: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*Award, 0)
	for rows.Next() { a, err := scan(rows); if err != nil { return nil, err }; list = append(list, a) }
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Award, error) {
	return scan(r.db.QueryRow(ctx,
		`SELECT `+sel+` FROM hrm_awards WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) Create(ctx context.Context, a *Award) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_awards
		(org_id, employee_id, award_type, title, description, points, monetary_value, currency,
		 award_date, issued_by, approval_instance_id, certificate_document_id, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::date,$10,$11,$12,$13,$14)
		RETURNING id, public_id, created_at, updated_at`,
		a.OrgID, a.EmployeeID, a.AwardType, a.Title, a.Description, a.Points, a.MonetaryValue, a.Currency,
		a.AwardDate, a.IssuedBy, a.ApprovalInstanceID, a.CertificateDocumentID, a.Status, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, a *Award) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_awards SET title=$1, description=$2, points=$3, monetary_value=$4,
		award_date=$5::date, certificate_document_id=$6, issued_at=$7, updated_at=NOW()
		WHERE id=$8 AND org_id=$9 RETURNING updated_at`,
		a.Title, a.Description, a.Points, a.MonetaryValue,
		a.AwardDate, a.CertificateDocumentID, a.IssuedAt,
		a.ID, a.OrgID,
	).Scan(&a.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status AwardStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_awards SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (r *repoImpl) SetAnnouncementID(ctx context.Context, id, announcementID string) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_awards SET announcement_id=$1, updated_at=NOW() WHERE id=$2`, announcementID, id)
	return err
}
