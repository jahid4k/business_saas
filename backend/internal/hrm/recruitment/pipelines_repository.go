// backend/internal/hrm/recruitment/pipelines_repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PipelineRepository is embedded into Repository — see repository.go.
type PipelineRepository interface {
	FindPipelines(ctx context.Context, orgID string) ([]*Pipeline, error)
	FindPipelineByRef(ctx context.Context, orgID, ref string) (*Pipeline, error)
	FindDefaultPipeline(ctx context.Context, orgID string) (*Pipeline, error)
	CreatePipeline(ctx context.Context, p *Pipeline) error
	UpdatePipeline(ctx context.Context, p *Pipeline) error
	DeletePipeline(ctx context.Context, orgID, pipelineID string) error
	// SetPipelineDefault atomically clears any existing default for orgID
	// then sets pipelineID — the guard crm_pipelines' UpdatePipeline lacks.
	SetPipelineDefault(ctx context.Context, orgID, pipelineID string) error

	FindStages(ctx context.Context, orgID, pipelineID string) ([]*Stage, error)
	FindStageByRef(ctx context.Context, orgID, pipelineID, ref string) (*Stage, error)
	// FindStageByRefAnyPipeline looks up a stage by ref within the org
	// WITHOUT scoping to a specific pipeline. Used only by application
	// stage-move validation, to distinguish "stage doesn't exist" from
	// "stage exists but belongs to a different pipeline than the
	// application" — FindStageByRef's pipeline-scoped query collapses both
	// into "not found", which would make ErrStageNotInPipeline unreachable.
	FindStageByRefAnyPipeline(ctx context.Context, orgID, ref string) (*Stage, error)
	CreateStage(ctx context.Context, s *Stage) error
	UpdateStage(ctx context.Context, s *Stage) error
	DeleteStage(ctx context.Context, orgID, pipelineID, stageID string) error
	ReorderStages(ctx context.Context, orgID, pipelineID string, stageIDs []string) error
}

const pipelineCols = `id, public_id, org_id, name, description, is_default, is_active, created_by, created_at, updated_at`

func scanPipeline(row interface{ Scan(...any) error }, p *Pipeline) error {
	return row.Scan(&p.ID, &p.PublicID, &p.OrgID, &p.Name, &p.Description, &p.IsDefault, &p.IsActive, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
}

func (r *repoImpl) FindPipelines(ctx context.Context, orgID string) ([]*Pipeline, error) {
	q := `SELECT ` + pipelineCols + ` FROM hrm_recruitment_pipelines WHERE org_id = $1 ORDER BY is_default DESC, created_at ASC`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindPipelines: %w", err)
	}
	defer rows.Close()
	list := make([]*Pipeline, 0)
	for rows.Next() {
		p := &Pipeline{}
		if err := scanPipeline(rows, p); err != nil {
			return nil, fmt.Errorf("recruitment: FindPipelines: scan: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindPipelineByRef(ctx context.Context, orgID, ref string) (*Pipeline, error) {
	q := `SELECT ` + pipelineCols + ` FROM hrm_recruitment_pipelines WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	p := &Pipeline{}
	err := scanPipeline(r.db.QueryRow(ctx, q, orgID, ref), p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindPipelineByRef: %w", err)
	}
	return p, nil
}

func (r *repoImpl) FindDefaultPipeline(ctx context.Context, orgID string) (*Pipeline, error) {
	q := `SELECT ` + pipelineCols + ` FROM hrm_recruitment_pipelines WHERE org_id = $1 AND is_default = TRUE AND is_active = TRUE`
	p := &Pipeline{}
	err := scanPipeline(r.db.QueryRow(ctx, q, orgID), p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindDefaultPipeline: %w", err)
	}
	return p, nil
}

func (r *repoImpl) CreatePipeline(ctx context.Context, p *Pipeline) error {
	if p.IsDefault {
		tx, err := r.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("recruitment: CreatePipeline: begin: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx,
			`UPDATE hrm_recruitment_pipelines SET is_default = FALSE, updated_at = NOW() WHERE org_id = $1 AND is_default = TRUE`,
			p.OrgID,
		); err != nil {
			return fmt.Errorf("recruitment: CreatePipeline: clear default: %w", err)
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO hrm_recruitment_pipelines (org_id, name, description, is_default, is_active, created_by)
			 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`,
			p.OrgID, p.Name, p.Description, p.IsDefault, p.IsActive, p.CreatedBy,
		).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return fmt.Errorf("recruitment: CreatePipeline: insert: %w", err)
		}
		return tx.Commit(ctx)
	}

	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_recruitment_pipelines (org_id, name, description, is_default, is_active, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.Name, p.Description, p.IsDefault, p.IsActive, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
}

// UpdatePipeline never writes is_default=TRUE — promoting a pipeline to
// default goes through SetPipelineDefault, which clears the sibling
// atomically. Writing is_default=FALSE here is safe (never conflicts).
func (r *repoImpl) UpdatePipeline(ctx context.Context, p *Pipeline) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_recruitment_pipelines SET name=$1, description=$2, is_default=$3, is_active=$4, updated_at=NOW()
		 WHERE id=$5 AND org_id=$6 RETURNING updated_at`,
		p.Name, p.Description, p.IsDefault, p.IsActive, p.ID, p.OrgID,
	).Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPipelineNotFound
	}
	return err
}

func (r *repoImpl) DeletePipeline(ctx context.Context, orgID, pipelineID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_recruitment_pipelines WHERE org_id=$1 AND id=$2`, orgID, pipelineID)
	if err != nil {
		return fmt.Errorf("recruitment: DeletePipeline: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrPipelineNotFound
	}
	return nil
}

func (r *repoImpl) SetPipelineDefault(ctx context.Context, orgID, pipelineID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("recruitment: SetPipelineDefault: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE hrm_recruitment_pipelines SET is_default = FALSE, updated_at = NOW() WHERE org_id = $1 AND is_default = TRUE`,
		orgID,
	); err != nil {
		return fmt.Errorf("recruitment: SetPipelineDefault: clear: %w", err)
	}
	cmd, err := tx.Exec(ctx,
		`UPDATE hrm_recruitment_pipelines SET is_default = TRUE, updated_at = NOW() WHERE org_id = $1 AND id = $2`,
		orgID, pipelineID,
	)
	if err != nil {
		return fmt.Errorf("recruitment: SetPipelineDefault: set: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrPipelineNotFound
	}
	return tx.Commit(ctx)
}

// ============================================================
// Stages
// ============================================================

const stageCols = `id, public_id, org_id, pipeline_id, name, position, stage_kind, created_at, updated_at`

func scanStage(row interface{ Scan(...any) error }, s *Stage) error {
	return row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.PipelineID, &s.Name, &s.Position, &s.StageKind, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) FindStages(ctx context.Context, orgID, pipelineID string) ([]*Stage, error) {
	q := `SELECT ` + stageCols + ` FROM hrm_recruitment_stages WHERE org_id = $1 AND pipeline_id = $2 ORDER BY position ASC`
	rows, err := r.db.Query(ctx, q, orgID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindStages: %w", err)
	}
	defer rows.Close()
	list := make([]*Stage, 0)
	for rows.Next() {
		s := &Stage{}
		if err := scanStage(rows, s); err != nil {
			return nil, fmt.Errorf("recruitment: FindStages: scan: %w", err)
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindStageByRef(ctx context.Context, orgID, pipelineID, ref string) (*Stage, error) {
	q := `SELECT ` + stageCols + ` FROM hrm_recruitment_stages WHERE org_id = $1 AND pipeline_id = $2 AND (id::text = $3 OR public_id = $3)`
	s := &Stage{}
	err := scanStage(r.db.QueryRow(ctx, q, orgID, pipelineID, ref), s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindStageByRef: %w", err)
	}
	return s, nil
}

func (r *repoImpl) FindStageByRefAnyPipeline(ctx context.Context, orgID, ref string) (*Stage, error) {
	q := `SELECT ` + stageCols + ` FROM hrm_recruitment_stages WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	s := &Stage{}
	err := scanStage(r.db.QueryRow(ctx, q, orgID, ref), s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindStageByRefAnyPipeline: %w", err)
	}
	return s, nil
}

func (r *repoImpl) CreateStage(ctx context.Context, s *Stage) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_recruitment_stages (org_id, pipeline_id, name, position, stage_kind)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, public_id, created_at, updated_at`,
		s.OrgID, s.PipelineID, s.Name, s.Position, s.StageKind,
	).Scan(&s.ID, &s.PublicID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) UpdateStage(ctx context.Context, s *Stage) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_recruitment_stages SET name=$1, stage_kind=$2, updated_at=NOW()
		 WHERE id=$3 AND org_id=$4 AND pipeline_id=$5 RETURNING updated_at`,
		s.Name, s.StageKind, s.ID, s.OrgID, s.PipelineID,
	).Scan(&s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStageNotFound
	}
	return err
}

func (r *repoImpl) DeleteStage(ctx context.Context, orgID, pipelineID, stageID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_recruitment_stages WHERE org_id=$1 AND pipeline_id=$2 AND id=$3`, orgID, pipelineID, stageID)
	if err != nil {
		return fmt.Errorf("recruitment: DeleteStage: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrStageNotFound
	}
	return nil
}

// ReorderStages rewrites position = i from the array index, inside one
// transaction — the crm_pipeline_stages precedent. No unique index on
// position, so this never 23505s mid-rewrite.
func (r *repoImpl) ReorderStages(ctx context.Context, orgID, pipelineID string, stageIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("recruitment: ReorderStages: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `UPDATE hrm_recruitment_stages SET position = $1, updated_at = NOW() WHERE org_id = $2 AND pipeline_id = $3 AND id = $4`
	for i, id := range stageIDs {
		if _, err := tx.Exec(ctx, q, i, orgID, pipelineID, id); err != nil {
			return fmt.Errorf("recruitment: ReorderStages: update %d: %w", i, err)
		}
	}
	return tx.Commit(ctx)
}
