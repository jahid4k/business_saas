// backend/internal/crm/templates/repository.go
package templates

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTemplateNotFound = errors.New("template not found")

// Repository defines data access for CRM templates.
type Repository interface {
	FindTemplates(ctx context.Context, orgID string) ([]*Template, error)
	FindTemplateByID(ctx context.Context, orgID, templateID string) (*Template, error)
	CreateTemplate(ctx context.Context, t *Template) error
	UpdateTemplate(ctx context.Context, t *Template) error
	DeleteTemplate(ctx context.Context, orgID, templateID string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new templates repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

const templateCols = `
	id, public_id, org_id, name, type, subject, body, created_by, created_at, updated_at`

func scanTemplate(row interface{ Scan(...any) error }, t *Template) error {
	return row.Scan(
		&t.ID, &t.PublicID, &t.OrgID, &t.Name, &t.Type, &t.Subject,
		&t.Body, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
}

func (r *repoImpl) FindTemplates(ctx context.Context, orgID string) ([]*Template, error) {
	q := `SELECT ` + templateCols + `
		FROM crm_templates WHERE org_id = $1
		ORDER BY type ASC, created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("templates: FindTemplates: %w", err)
	}
	defer rows.Close()

	var out []*Template
	for rows.Next() {
		t := &Template{}
		if err := scanTemplate(rows, t); err != nil {
			return nil, fmt.Errorf("templates: FindTemplates: scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindTemplateByID(ctx context.Context, orgID, templateID string) (*Template, error) {
	q := `SELECT ` + templateCols + `
		FROM crm_templates WHERE org_id = $1 AND id = $2`

	t := &Template{}
	err := scanTemplate(r.db.QueryRow(ctx, q, orgID, templateID), t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("templates: FindTemplateByID: %w", err)
	}
	return t, nil
}

func (r *repoImpl) CreateTemplate(ctx context.Context, t *Template) error {
	const q = `
		INSERT INTO crm_templates
		    (org_id, name, type, subject, body, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		t.OrgID, t.Name, t.Type, t.Subject, t.Body, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) UpdateTemplate(ctx context.Context, t *Template) error {
	const updateSQL = `
		UPDATE crm_templates
		SET name = $1, subject = $2, body = $3, updated_at = NOW()
		WHERE org_id = $4 AND id = $5
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, updateSQL,
		t.Name, t.Subject, t.Body,
		t.OrgID, t.ID,
	).Scan(&t.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTemplateNotFound
	}
	return err
}

func (r *repoImpl) DeleteTemplate(ctx context.Context, orgID, templateID string) error {
	const q = `DELETE FROM crm_templates WHERE org_id = $1 AND id = $2`
	tag, err := r.db.Exec(ctx, q, orgID, templateID)
	if err != nil {
		return fmt.Errorf("templates: DeleteTemplate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}
