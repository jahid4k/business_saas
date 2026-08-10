// backend/internal/hrm/performance/scales_repository.go
package performance

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ScaleRepository is embedded into Repository — see repository.go.
//
// Levels have no org_id of their own: every query reaches them by joining
// through hrm_rating_scales and filtering that org_id, the same tenant
// isolation shape platform_form_questions and hrm_approval_decisions use.
type ScaleRepository interface {
	FindScales(ctx context.Context, orgID string) ([]*RatingScale, error)
	FindScaleByRef(ctx context.Context, orgID, ref string) (*RatingScale, error)
	FindDefaultScale(ctx context.Context, orgID string) (*RatingScale, error)
	CreateScale(ctx context.Context, s *RatingScale) error
	UpdateScale(ctx context.Context, s *RatingScale) error
	SetScaleDefault(ctx context.Context, orgID, scaleID string) error
	DeleteScale(ctx context.Context, orgID, scaleID string) error
	ScaleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)
	CountCyclesUsingScale(ctx context.Context, scaleID string) (int, error)

	FindLevels(ctx context.Context, orgID, scaleID string) ([]*RatingLevel, error)
	FindLevelByRef(ctx context.Context, orgID, ref string) (*RatingLevel, error)
	CreateLevel(ctx context.Context, l *RatingLevel) error
	UpdateLevel(ctx context.Context, orgID string, l *RatingLevel) error
	DeleteLevel(ctx context.Context, orgID, levelID string) error
	LevelLabelExists(ctx context.Context, scaleID, label, excludeID string) (bool, error)
}

const scaleCols = `id, public_id, org_id, name, description, is_default, is_active,
	created_by, created_at, updated_at`

func scanScale(row interface{ Scan(...any) error }, s *RatingScale) error {
	return row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.Name, &s.Description, &s.IsDefault, &s.IsActive,
		&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) FindScales(ctx context.Context, orgID string) ([]*RatingScale, error) {
	q := `SELECT ` + scaleCols + ` FROM hrm_rating_scales WHERE org_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("performance: FindScales: %w", err)
	}
	defer rows.Close()
	list := make([]*RatingScale, 0)
	for rows.Next() {
		s := &RatingScale{}
		if err := scanScale(rows, s); err != nil {
			return nil, fmt.Errorf("performance: FindScales: scan: %w", err)
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindScaleByRef(ctx context.Context, orgID, ref string) (*RatingScale, error) {
	q := `SELECT ` + scaleCols + ` FROM hrm_rating_scales WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	s := &RatingScale{}
	err := scanScale(r.db.QueryRow(ctx, q, orgID, ref), s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindScaleByRef: %w", err)
	}
	return s, nil
}

func (r *repoImpl) FindDefaultScale(ctx context.Context, orgID string) (*RatingScale, error) {
	q := `SELECT ` + scaleCols + ` FROM hrm_rating_scales WHERE org_id = $1 AND is_default AND is_active`
	s := &RatingScale{}
	err := scanScale(r.db.QueryRow(ctx, q, orgID), s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindDefaultScale: %w", err)
	}
	return s, nil
}

const insertScaleSQL = `
	INSERT INTO hrm_rating_scales (org_id, name, description, is_default, created_by)
	VALUES ($1,$2,$3,$4,$5) RETURNING id, public_id, is_active, created_at, updated_at`

func (r *repoImpl) CreateScale(ctx context.Context, s *RatingScale) error {
	// Creating as the default must clear the sibling atomically, or the
	// partial unique index raises a bare 23505.
	if s.IsDefault {
		tx, err := r.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("performance: CreateScale: begin: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx,
			`UPDATE hrm_rating_scales SET is_default = FALSE, updated_at = NOW() WHERE org_id = $1 AND is_default`,
			s.OrgID,
		); err != nil {
			return fmt.Errorf("performance: CreateScale: clear default: %w", err)
		}
		if err := tx.QueryRow(ctx, insertScaleSQL, s.OrgID, s.Name, s.Description, s.IsDefault, s.CreatedBy).
			Scan(&s.ID, &s.PublicID, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return fmt.Errorf("performance: CreateScale: insert: %w", err)
		}
		return tx.Commit(ctx)
	}

	return r.db.QueryRow(ctx, insertScaleSQL, s.OrgID, s.Name, s.Description, s.IsDefault, s.CreatedBy).
		Scan(&s.ID, &s.PublicID, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
}

// UpdateScale never writes is_default = TRUE — promotion goes through
// SetScaleDefault, which clears the sibling atomically.
func (r *repoImpl) UpdateScale(ctx context.Context, s *RatingScale) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_rating_scales SET name=$1, description=$2, is_default=$3, is_active=$4, updated_at=NOW()
		 WHERE id=$5 AND org_id=$6 RETURNING updated_at`,
		s.Name, s.Description, s.IsDefault, s.IsActive, s.ID, s.OrgID,
	).Scan(&s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrScaleNotFound
	}
	return err
}

func (r *repoImpl) SetScaleDefault(ctx context.Context, orgID, scaleID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("performance: SetScaleDefault: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE hrm_rating_scales SET is_default = FALSE, updated_at = NOW() WHERE org_id = $1 AND is_default`,
		orgID,
	); err != nil {
		return fmt.Errorf("performance: SetScaleDefault: clear: %w", err)
	}
	cmd, err := tx.Exec(ctx,
		`UPDATE hrm_rating_scales SET is_default = TRUE, updated_at = NOW() WHERE org_id = $1 AND id = $2`,
		orgID, scaleID,
	)
	if err != nil {
		return fmt.Errorf("performance: SetScaleDefault: set: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrScaleNotFound
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) DeleteScale(ctx context.Context, orgID, scaleID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_rating_scales WHERE org_id=$1 AND id=$2`, orgID, scaleID)
	if err != nil {
		return fmt.Errorf("performance: DeleteScale: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrScaleNotFound
	}
	return nil
}

func (r *repoImpl) ScaleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_rating_scales WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND id::text <> $3)`,
		orgID, name, excludeID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("performance: ScaleNameExists: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) CountCyclesUsingScale(ctx context.Context, scaleID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_appraisal_cycles WHERE rating_scale_id = $1`, scaleID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("performance: CountCyclesUsingScale: %w", err)
	}
	return count, nil
}

// ── Levels ───────────────────────────────────────────────────────────────────

const levelCols = `l.id, l.public_id, l.scale_id, l.label, l.description, l.value,
	l.display_order, l.color, l.created_at, l.updated_at`

func scanLevel(row interface{ Scan(...any) error }, l *RatingLevel) error {
	return row.Scan(&l.ID, &l.PublicID, &l.ScaleID, &l.Label, &l.Description, &l.Value,
		&l.DisplayOrder, &l.Color, &l.CreatedAt, &l.UpdatedAt)
}

func (r *repoImpl) FindLevels(ctx context.Context, orgID, scaleID string) ([]*RatingLevel, error) {
	const q = `SELECT ` + levelCols + `
		FROM hrm_rating_scale_levels l
		JOIN hrm_rating_scales s ON s.id = l.scale_id
		WHERE s.org_id = $1 AND l.scale_id = $2
		ORDER BY l.display_order ASC, l.value ASC`
	rows, err := r.db.Query(ctx, q, orgID, scaleID)
	if err != nil {
		return nil, fmt.Errorf("performance: FindLevels: %w", err)
	}
	defer rows.Close()
	list := make([]*RatingLevel, 0)
	for rows.Next() {
		l := &RatingLevel{}
		if err := scanLevel(rows, l); err != nil {
			return nil, fmt.Errorf("performance: FindLevels: scan: %w", err)
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindLevelByRef(ctx context.Context, orgID, ref string) (*RatingLevel, error) {
	const q = `SELECT ` + levelCols + `
		FROM hrm_rating_scale_levels l
		JOIN hrm_rating_scales s ON s.id = l.scale_id
		WHERE s.org_id = $1 AND (l.id::text = $2 OR l.public_id = $2)`
	l := &RatingLevel{}
	err := scanLevel(r.db.QueryRow(ctx, q, orgID, ref), l)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("performance: FindLevelByRef: %w", err)
	}
	return l, nil
}

func (r *repoImpl) CreateLevel(ctx context.Context, l *RatingLevel) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_rating_scale_levels (scale_id, label, description, value, display_order, color)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`,
		l.ScaleID, l.Label, l.Description, l.Value, l.DisplayOrder, l.Color,
	).Scan(&l.ID, &l.PublicID, &l.CreatedAt, &l.UpdatedAt)
}

func (r *repoImpl) UpdateLevel(ctx context.Context, orgID string, l *RatingLevel) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_rating_scale_levels SET label=$1, description=$2, value=$3, display_order=$4, color=$5, updated_at=NOW()
		 WHERE id=$6 AND scale_id IN (SELECT id FROM hrm_rating_scales WHERE org_id=$7)
		 RETURNING updated_at`,
		l.Label, l.Description, l.Value, l.DisplayOrder, l.Color, l.ID, orgID,
	).Scan(&l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLevelNotFound
	}
	return err
}

func (r *repoImpl) DeleteLevel(ctx context.Context, orgID, levelID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM hrm_rating_scale_levels
		 WHERE id=$1 AND scale_id IN (SELECT id FROM hrm_rating_scales WHERE org_id=$2)`,
		levelID, orgID,
	)
	if err != nil {
		return fmt.Errorf("performance: DeleteLevel: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrLevelNotFound
	}
	return nil
}

func (r *repoImpl) LevelLabelExists(ctx context.Context, scaleID, label, excludeID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_rating_scale_levels WHERE scale_id=$1 AND LOWER(label)=LOWER($2) AND id::text <> $3)`,
		scaleID, label, excludeID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("performance: LevelLabelExists: %w", err)
	}
	return exists, nil
}
