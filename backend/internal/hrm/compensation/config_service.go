// backend/internal/hrm/compensation/config_service.go
package compensation

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// ConfigService covers compensation bands and merit matrix cells.
type ConfigService interface {
	ListBands(ctx context.Context, orgID string) ([]*Band, error)
	GetBand(ctx context.Context, orgID, ref string) (*Band, error)
	CreateBand(ctx context.Context, orgID, createdBy string, req CreateBandRequest) (*Band, error)
	UpdateBand(ctx context.Context, orgID, ref string, req UpdateBandRequest) (*Band, error)
	DeleteBand(ctx context.Context, orgID, ref string) error

	ListMatrixCells(ctx context.Context, orgID string) ([]*MatrixCell, error)
	CreateMatrixCell(ctx context.Context, orgID, createdBy string, req CreateMatrixCellRequest) (*MatrixCell, error)
	DeleteMatrixCell(ctx context.Context, orgID, ref string) error
}

func (s *serviceImpl) ListBands(ctx context.Context, orgID string) ([]*Band, error) {
	return s.repo.ListBands(ctx, orgID)
}

func (s *serviceImpl) GetBand(ctx context.Context, orgID, ref string) (*Band, error) {
	b, err := s.repo.FindBandByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: GetBand: %w", err)
	}
	if b == nil {
		return nil, ErrBandNotFound
	}
	return b, nil
}

func parseMoney(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil || d.IsNegative() {
		return decimal.Zero, ErrInvalidAmount
	}
	return d, nil
}

func (s *serviceImpl) CreateBand(ctx context.Context, orgID, createdBy string, req CreateBandRequest) (*Band, error) {
	min, err := parseMoney(req.MinAmount)
	if err != nil {
		return nil, err
	}
	mid, err := parseMoney(req.MidAmount)
	if err != nil {
		return nil, err
	}
	max, err := parseMoney(req.MaxAmount)
	if err != nil {
		return nil, err
	}
	if min.GreaterThan(mid) || mid.GreaterThan(max) {
		return nil, ErrInvalidBandRange
	}
	eff, err := parseDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}
	currency := "USD"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	grade := strings.TrimSpace(req.GradeLabel)
	if grade == "" {
		return nil, fmt.Errorf("compensation: CreateBand: grade_label is required")
	}
	b := &Band{
		OrgID: orgID, GradeLabel: grade, Currency: currency,
		MinAmount: min, MidAmount: mid, MaxAmount: max,
		EffectiveDate: *eff, CreatedBy: createdBy,
	}
	if err := s.repo.CreateBand(ctx, b); err != nil {
		return nil, fmt.Errorf("compensation: CreateBand: %w", err)
	}
	return b, nil
}

func (s *serviceImpl) UpdateBand(ctx context.Context, orgID, ref string, req UpdateBandRequest) (*Band, error) {
	b, err := s.repo.FindBandByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: UpdateBand: %w", err)
	}
	if b == nil {
		return nil, ErrBandNotFound
	}
	if req.MinAmount != nil {
		v, err := parseMoney(*req.MinAmount)
		if err != nil {
			return nil, err
		}
		b.MinAmount = v
	}
	if req.MidAmount != nil {
		v, err := parseMoney(*req.MidAmount)
		if err != nil {
			return nil, err
		}
		b.MidAmount = v
	}
	if req.MaxAmount != nil {
		v, err := parseMoney(*req.MaxAmount)
		if err != nil {
			return nil, err
		}
		b.MaxAmount = v
	}
	if req.EffectiveDate != nil {
		eff, err := parseDate(*req.EffectiveDate)
		if err != nil {
			return nil, err
		}
		b.EffectiveDate = *eff
	}
	if b.MinAmount.GreaterThan(b.MidAmount) || b.MidAmount.GreaterThan(b.MaxAmount) {
		return nil, ErrInvalidBandRange
	}
	if err := s.repo.UpdateBand(ctx, b); err != nil {
		return nil, fmt.Errorf("compensation: UpdateBand: %w", err)
	}
	return b, nil
}

func (s *serviceImpl) DeleteBand(ctx context.Context, orgID, ref string) error {
	b, err := s.repo.FindBandByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("compensation: DeleteBand: %w", err)
	}
	if b == nil {
		return ErrBandNotFound
	}
	if err := s.repo.DeleteBand(ctx, orgID, b.ID); err != nil {
		return fmt.Errorf("compensation: DeleteBand: %w", err)
	}
	return nil
}

func (s *serviceImpl) ListMatrixCells(ctx context.Context, orgID string) ([]*MatrixCell, error) {
	return s.repo.ListMatrixCells(ctx, orgID)
}

func (s *serviceImpl) CreateMatrixCell(ctx context.Context, orgID, createdBy string, req CreateMatrixCellRequest) (*MatrixCell, error) {
	min, err := decimal.NewFromString(strings.TrimSpace(req.CompaRatioMin))
	if err != nil || min.IsNegative() {
		return nil, ErrInvalidAmount
	}
	var max *decimal.Decimal
	if req.CompaRatioMax != nil && strings.TrimSpace(*req.CompaRatioMax) != "" {
		v, err := decimal.NewFromString(strings.TrimSpace(*req.CompaRatioMax))
		if err != nil || !v.GreaterThan(min) {
			return nil, fmt.Errorf("compensation: CreateMatrixCell: compa_ratio_max must be greater than compa_ratio_min")
		}
		max = &v
	}
	increase, err := decimal.NewFromString(strings.TrimSpace(req.IncreasePct))
	if err != nil {
		return nil, fmt.Errorf("compensation: CreateMatrixCell: invalid increase_pct")
	}
	eff, err := parseDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}
	ratingLevelID := strings.TrimSpace(req.RatingLevelID)
	if ratingLevelID == "" {
		return nil, fmt.Errorf("compensation: CreateMatrixCell: rating_level_id is required")
	}
	c := &MatrixCell{
		OrgID: orgID, RatingLevelID: ratingLevelID,
		CompaRatioMin: min, CompaRatioMax: max, IncreasePct: increase,
		EffectiveDate: *eff, CreatedBy: createdBy,
	}
	if err := s.repo.CreateMatrixCell(ctx, c); err != nil {
		return nil, fmt.Errorf("compensation: CreateMatrixCell: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) DeleteMatrixCell(ctx context.Context, orgID, ref string) error {
	c, err := s.repo.FindMatrixCellByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("compensation: DeleteMatrixCell: %w", err)
	}
	if c == nil {
		return ErrMatrixCellNotFound
	}
	if err := s.repo.DeleteMatrixCell(ctx, orgID, c.ID); err != nil {
		return fmt.Errorf("compensation: DeleteMatrixCell: %w", err)
	}
	return nil
}
