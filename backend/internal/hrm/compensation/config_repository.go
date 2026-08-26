// backend/internal/hrm/compensation/config_repository.go
package compensation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ConfigRepository covers the two catalog tables — bands and merit matrix
// cells — that share the hrm.compensation_config permission resource (see
// migration 00099's header) and carry no employee_id, so neither is
// scope-tiered.
type ConfigRepository interface {
	ListBands(ctx context.Context, orgID string) ([]*Band, error)
	FindBandByRef(ctx context.Context, orgID, ref string) (*Band, error)
	// FindActiveBand returns the band whose grade_label matches (case
	// insensitively) and whose effective_date is the latest not after asOf.
	// Returns (nil, nil) when no band matches — an undefined compa-ratio, not
	// an error.
	FindActiveBand(ctx context.Context, orgID, gradeLabel string, asOf time.Time) (*Band, error)
	CreateBand(ctx context.Context, b *Band) error
	UpdateBand(ctx context.Context, b *Band) error
	DeleteBand(ctx context.Context, orgID, id string) error

	ListMatrixCells(ctx context.Context, orgID string) ([]*MatrixCell, error)
	FindMatrixCellByRef(ctx context.Context, orgID, ref string) (*MatrixCell, error)
	// FindMatrixCell returns the cell whose range contains compaRatio for
	// ratingLevelID, latest effective_date not after asOf. (nil, nil) when
	// no cell matches.
	FindMatrixCell(ctx context.Context, orgID, ratingLevelID string, compaRatio float64, asOf time.Time) (*MatrixCell, error)
	CreateMatrixCell(ctx context.Context, c *MatrixCell) error
	DeleteMatrixCell(ctx context.Context, orgID, id string) error
}

const bandSel = `id, public_id, org_id, grade_label, currency, min_amount, mid_amount, max_amount, effective_date, created_by, created_at, updated_at`

func scanBand(row pgx.Row) (*Band, error) {
	b := &Band{}
	err := row.Scan(&b.ID, &b.PublicID, &b.OrgID, &b.GradeLabel, &b.Currency,
		&b.MinAmount, &b.MidAmount, &b.MaxAmount, &b.EffectiveDate,
		&b.CreatedBy, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *repoImpl) ListBands(ctx context.Context, orgID string) ([]*Band, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+bandSel+` FROM hrm_compensation_bands WHERE org_id=$1 ORDER BY grade_label, effective_date DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("compensation: ListBands: %w", err)
	}
	defer rows.Close()
	list := make([]*Band, 0)
	for rows.Next() {
		b, err := scanBand(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindBandByRef(ctx context.Context, orgID, ref string) (*Band, error) {
	return scanBand(r.db.QueryRow(ctx,
		`SELECT `+bandSel+` FROM hrm_compensation_bands WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindActiveBand(ctx context.Context, orgID, gradeLabel string, asOf time.Time) (*Band, error) {
	return scanBand(r.db.QueryRow(ctx,
		`SELECT `+bandSel+` FROM hrm_compensation_bands
		  WHERE org_id=$1 AND LOWER(grade_label)=LOWER($2) AND effective_date <= $3
		  ORDER BY effective_date DESC LIMIT 1`,
		orgID, gradeLabel, asOf))
}

func (r *repoImpl) CreateBand(ctx context.Context, b *Band) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_compensation_bands (org_id, grade_label, currency, min_amount, mid_amount, max_amount, effective_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, public_id, created_at, updated_at`,
		b.OrgID, b.GradeLabel, b.Currency, b.MinAmount, b.MidAmount, b.MaxAmount, b.EffectiveDate, b.CreatedBy,
	).Scan(&b.ID, &b.PublicID, &b.CreatedAt, &b.UpdatedAt)
}

func (r *repoImpl) UpdateBand(ctx context.Context, b *Band) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_compensation_bands
		    SET min_amount=$3, mid_amount=$4, max_amount=$5, effective_date=$6, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		b.OrgID, b.ID, b.MinAmount, b.MidAmount, b.MaxAmount, b.EffectiveDate)
	if err != nil {
		return fmt.Errorf("compensation: UpdateBand: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrBandNotFound
	}
	return nil
}

func (r *repoImpl) DeleteBand(ctx context.Context, orgID, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM hrm_compensation_bands WHERE org_id=$1 AND id=$2::uuid`, orgID, id)
	if err != nil {
		return fmt.Errorf("compensation: DeleteBand: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrBandNotFound
	}
	return nil
}

const matrixCellSel = `id, public_id, org_id, rating_level_id, compa_ratio_min, compa_ratio_max, increase_pct, effective_date, created_by, created_at, updated_at`

func scanMatrixCell(row pgx.Row) (*MatrixCell, error) {
	c := &MatrixCell{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.RatingLevelID,
		&c.CompaRatioMin, &c.CompaRatioMax, &c.IncreasePct, &c.EffectiveDate,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) ListMatrixCells(ctx context.Context, orgID string) ([]*MatrixCell, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+matrixCellSel+` FROM hrm_merit_matrix_cells WHERE org_id=$1 ORDER BY rating_level_id, compa_ratio_min`, orgID)
	if err != nil {
		return nil, fmt.Errorf("compensation: ListMatrixCells: %w", err)
	}
	defer rows.Close()
	list := make([]*MatrixCell, 0)
	for rows.Next() {
		c, err := scanMatrixCell(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindMatrixCellByRef(ctx context.Context, orgID, ref string) (*MatrixCell, error) {
	return scanMatrixCell(r.db.QueryRow(ctx,
		`SELECT `+matrixCellSel+` FROM hrm_merit_matrix_cells WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindMatrixCell(ctx context.Context, orgID, ratingLevelID string, compaRatio float64, asOf time.Time) (*MatrixCell, error) {
	return scanMatrixCell(r.db.QueryRow(ctx,
		`SELECT `+matrixCellSel+` FROM hrm_merit_matrix_cells
		  WHERE org_id=$1 AND rating_level_id=$2::uuid AND effective_date <= $4
		    AND compa_ratio_min <= $3 AND (compa_ratio_max IS NULL OR compa_ratio_max > $3)
		  ORDER BY effective_date DESC LIMIT 1`,
		orgID, ratingLevelID, compaRatio, asOf))
}

func (r *repoImpl) CreateMatrixCell(ctx context.Context, c *MatrixCell) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_merit_matrix_cells (org_id, rating_level_id, compa_ratio_min, compa_ratio_max, increase_pct, effective_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.RatingLevelID, c.CompaRatioMin, c.CompaRatioMax, c.IncreasePct, c.EffectiveDate, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) DeleteMatrixCell(ctx context.Context, orgID, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM hrm_merit_matrix_cells WHERE org_id=$1 AND id=$2::uuid`, orgID, id)
	if err != nil {
		return fmt.Errorf("compensation: DeleteMatrixCell: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrMatrixCellNotFound
	}
	return nil
}
