// backend/internal/hrm/doctemplates/repository.go
package doctemplates

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID string, activeOnly bool, docType string) ([]*DocumentTemplate, error)
	FindByRef(ctx context.Context, orgID, ref string) (*DocumentTemplate, error)
	Create(ctx context.Context, t *DocumentTemplate) error
	Update(ctx context.Context, t *DocumentTemplate) error
	Delete(ctx context.Context, orgID, ref string) error
	NameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const tmplSelect = `id, public_id, org_id, name, document_type, description, body_markdown, available_variables, requires_acknowledgement, is_active, created_by, created_at, updated_at`

func scan(row pgx.Row) (*DocumentTemplate, error) {
	t := &DocumentTemplate{}
	err := row.Scan(&t.ID, &t.PublicID, &t.OrgID, &t.Name, &t.DocumentType, &t.Description,
		&t.BodyMarkdown, &t.AvailableVariables, &t.RequiresAcknowledgement,
		&t.IsActive, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return t, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID string, activeOnly bool, docType string) ([]*DocumentTemplate, error) {
	q := `SELECT ` + tmplSelect + ` FROM hrm_document_templates WHERE org_id=$1`
	args := []any{orgID}
	if activeOnly { q += ` AND is_active=TRUE` }
	if docType != "" { args = append(args, docType); q += fmt.Sprintf(` AND document_type=$%d`, len(args)) }
	q += ` ORDER BY document_type, name`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("doctemplates: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*DocumentTemplate, 0)
	for rows.Next() {
		t, err := scan(rows)
		if err != nil { return nil, err }
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*DocumentTemplate, error) {
	return scan(r.db.QueryRow(ctx,
		`SELECT `+tmplSelect+` FROM hrm_document_templates WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) Create(ctx context.Context, t *DocumentTemplate) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_document_templates (org_id,name,document_type,description,body_markdown,available_variables,requires_acknowledgement,is_active,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, public_id, created_at, updated_at`,
		t.OrgID, t.Name, t.DocumentType, t.Description, t.BodyMarkdown,
		t.AvailableVariables, t.RequiresAcknowledgement, t.IsActive, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, t *DocumentTemplate) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_document_templates SET name=$1,document_type=$2,description=$3,body_markdown=$4,
		available_variables=$5,requires_acknowledgement=$6,is_active=$7,updated_at=NOW()
		WHERE id=$8 AND org_id=$9 RETURNING updated_at`,
		t.Name, t.DocumentType, t.Description, t.BodyMarkdown,
		t.AvailableVariables, t.RequiresAcknowledgement, t.IsActive, t.ID, t.OrgID,
	).Scan(&t.UpdatedAt)
}

func (r *repoImpl) Delete(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_document_templates WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref)
	if err != nil { return fmt.Errorf("doctemplates: Delete: %w", err) }
	if cmd.RowsAffected() == 0 { return ErrTemplateNotFound }
	return nil
}

func (r *repoImpl) NameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var e bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_document_templates WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND is_active=TRUE AND id::text!=$3)`,
		orgID, name, excludeID).Scan(&e)
	return e, err
}
