// backend/internal/crm/pipeline/repository.go
package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for pipelines and stages.
type Repository interface {
	// Pipelines
	FindPipelines(ctx context.Context, orgID string) ([]*Pipeline, error)
	FindPipelineByID(ctx context.Context, orgID, pipelineID string) (*Pipeline, error)
	CreatePipeline(ctx context.Context, p *Pipeline) error
	UpdatePipeline(ctx context.Context, p *Pipeline) error
	DeletePipeline(ctx context.Context, orgID, pipelineID string) error
	CountPipelines(ctx context.Context, orgID string) (int, error)

	// Stages
	FindStagesByPipeline(ctx context.Context, orgID, pipelineID string) ([]*Stage, error)
	FindStageByID(ctx context.Context, orgID, stageID string) (*Stage, error)
	FindAllStagesByOrg(ctx context.Context, orgID string) ([]*Stage, error)
	CreateStage(ctx context.Context, s *Stage) error
	UpdateStage(ctx context.Context, s *Stage) error
	DeleteStage(ctx context.Context, orgID, stageID string) error
	ReorderStages(ctx context.Context, orgID, pipelineID string, stageIDs []string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new pipeline repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// ============================================================
// Pipelines
// ============================================================

const pipelineCols = `id, public_id, org_id, name, description, is_default, created_by, created_at, updated_at`

func scanPipeline(row interface{ Scan(...any) error }, p *Pipeline) error {
	return row.Scan(
		&p.ID, &p.PublicID, &p.OrgID, &p.Name, &p.Description,
		&p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
}

func (r *repoImpl) FindPipelines(ctx context.Context, orgID string) ([]*Pipeline, error) {
	q := `SELECT ` + pipelineCols + `
		FROM crm_pipelines WHERE org_id = $1
		ORDER BY is_default DESC, created_at ASC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: FindPipelines: %w", err)
	}
	defer rows.Close()

	var out []*Pipeline
	for rows.Next() {
		p := &Pipeline{}
		if err := scanPipeline(rows, p); err != nil {
			return nil, fmt.Errorf("pipeline: FindPipelines: scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindPipelineByID(ctx context.Context, orgID, pipelineID string) (*Pipeline, error) {
	q := `SELECT ` + pipelineCols + `
		FROM crm_pipelines WHERE org_id = $1 AND id = $2`

	p := &Pipeline{}
	err := scanPipeline(r.db.QueryRow(ctx, q, orgID, pipelineID), p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pipeline: FindPipelineByID: %w", err)
	}
	return p, nil
}

func (r *repoImpl) CreatePipeline(ctx context.Context, p *Pipeline) error {
	const q = `
		INSERT INTO crm_pipelines (org_id, name, description, is_default, created_by)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		p.OrgID, p.Name, p.Description, p.IsDefault, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *repoImpl) UpdatePipeline(ctx context.Context, p *Pipeline) error {
	const q = `
		UPDATE crm_pipelines
		SET name = $1, description = $2, is_default = $3, updated_at = NOW()
		WHERE org_id = $4 AND id = $5
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		p.Name, p.Description, p.IsDefault, p.OrgID, p.ID,
	).Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPipelineNotFound
	}
	return err
}

func (r *repoImpl) DeletePipeline(ctx context.Context, orgID, pipelineID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM crm_pipelines WHERE org_id = $1 AND id = $2`,
		orgID, pipelineID,
	)
	if err != nil {
		return fmt.Errorf("pipeline: DeletePipeline: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrPipelineNotFound
	}
	return nil
}

func (r *repoImpl) CountPipelines(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM crm_pipelines WHERE org_id = $1`, orgID,
	).Scan(&n)
	return n, err
}

// ============================================================
// Stages
// ============================================================

const stageCols = `id, public_id, org_id, pipeline_id, name, position, probability, created_at, updated_at`

func scanStage(row interface{ Scan(...any) error }, s *Stage) error {
	return row.Scan(
		&s.ID, &s.PublicID, &s.OrgID, &s.PipelineID,
		&s.Name, &s.Position, &s.Probability, &s.CreatedAt, &s.UpdatedAt,
	)
}

func (r *repoImpl) FindStagesByPipeline(ctx context.Context, orgID, pipelineID string) ([]*Stage, error) {
	q := `SELECT ` + stageCols + `
		FROM crm_pipeline_stages
		WHERE org_id = $1 AND pipeline_id = $2
		ORDER BY position ASC`

	rows, err := r.db.Query(ctx, q, orgID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: FindStagesByPipeline: %w", err)
	}
	defer rows.Close()

	var out []*Stage
	for rows.Next() {
		s := &Stage{}
		if err := scanStage(rows, s); err != nil {
			return nil, fmt.Errorf("pipeline: FindStagesByPipeline: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindStageByID(ctx context.Context, orgID, stageID string) (*Stage, error) {
	q := `SELECT ` + stageCols + `
		FROM crm_pipeline_stages WHERE org_id = $1 AND id = $2`

	s := &Stage{}
	err := scanStage(r.db.QueryRow(ctx, q, orgID, stageID), s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pipeline: FindStageByID: %w", err)
	}
	return s, nil
}

func (r *repoImpl) FindAllStagesByOrg(ctx context.Context, orgID string) ([]*Stage, error) {
	q := `SELECT ` + stageCols + `
		FROM crm_pipeline_stages WHERE org_id = $1
		ORDER BY pipeline_id, position ASC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: FindAllStagesByOrg: %w", err)
	}
	defer rows.Close()

	var out []*Stage
	for rows.Next() {
		s := &Stage{}
		if err := scanStage(rows, s); err != nil {
			return nil, fmt.Errorf("pipeline: FindAllStagesByOrg: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *repoImpl) CreateStage(ctx context.Context, s *Stage) error {
	const q = `
		INSERT INTO crm_pipeline_stages (org_id, pipeline_id, name, position, probability)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		s.OrgID, s.PipelineID, s.Name, s.Position, s.Probability,
	).Scan(&s.ID, &s.PublicID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) UpdateStage(ctx context.Context, s *Stage) error {
	const q = `
		UPDATE crm_pipeline_stages
		SET name = $1, position = $2, probability = $3, updated_at = NOW()
		WHERE org_id = $4 AND id = $5
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		s.Name, s.Position, s.Probability, s.OrgID, s.ID,
	).Scan(&s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStageNotFound
	}
	return err
}

func (r *repoImpl) DeleteStage(ctx context.Context, orgID, stageID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM crm_pipeline_stages WHERE org_id = $1 AND id = $2`,
		orgID, stageID,
	)
	if err != nil {
		return fmt.Errorf("pipeline: DeleteStage: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrStageNotFound
	}
	return nil
}

func (r *repoImpl) ReorderStages(ctx context.Context, orgID, pipelineID string, stageIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pipeline: ReorderStages: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `
		UPDATE crm_pipeline_stages SET position = $1, updated_at = NOW()
		WHERE org_id = $2 AND pipeline_id = $3 AND id = $4`

	for i, id := range stageIDs {
		if _, err := tx.Exec(ctx, q, i, orgID, pipelineID, id); err != nil {
			return fmt.Errorf("pipeline: ReorderStages: update %d: %w", i, err)
		}
	}
	return tx.Commit(ctx)
}
