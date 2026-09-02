// backend/internal/hrm/performance/scales_service.go
package performance

import (
	"context"
	"fmt"
	"strings"
)

// ScaleService is embedded into Service — see service.go.
type ScaleService interface {
	ListScales(ctx context.Context, orgID string) ([]*RatingScale, error)
	GetScale(ctx context.Context, orgID, ref string) (*RatingScale, error)
	CreateScale(ctx context.Context, orgID, createdBy string, req CreateScaleRequest) (*RatingScale, error)
	UpdateScale(ctx context.Context, orgID, ref string, req UpdateScaleRequest) (*RatingScale, error)
	DeleteScale(ctx context.Context, orgID, ref string) error

	CreateLevel(ctx context.Context, orgID, scaleRef string, req CreateLevelRequest) (*RatingLevel, error)
	UpdateLevel(ctx context.Context, orgID, ref string, req UpdateLevelRequest) (*RatingLevel, error)
	DeleteLevel(ctx context.Context, orgID, ref string) error
}

func (s *serviceImpl) ListScales(ctx context.Context, orgID string) ([]*RatingScale, error) {
	list, err := s.repo.FindScales(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("performance: ListScales: %w", err)
	}
	if list == nil {
		list = []*RatingScale{}
	}
	return list, nil
}

func (s *serviceImpl) GetScale(ctx context.Context, orgID, ref string) (*RatingScale, error) {
	sc, err := s.repo.FindScaleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: GetScale: %w", err)
	}
	if sc == nil {
		return nil, ErrScaleNotFound
	}
	levels, err := s.repo.FindLevels(ctx, orgID, sc.ID)
	if err != nil {
		return nil, fmt.Errorf("performance: GetScale: levels: %w", err)
	}
	sc.Levels = levels
	return sc, nil
}

func (s *serviceImpl) CreateScale(ctx context.Context, orgID, createdBy string, req CreateScaleRequest) (*RatingScale, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrScaleNameRequired
	}
	taken, err := s.repo.ScaleNameExists(ctx, orgID, name, "")
	if err != nil {
		return nil, fmt.Errorf("performance: CreateScale: name check: %w", err)
	}
	if taken {
		return nil, ErrScaleNameTaken
	}

	sc := &RatingScale{
		OrgID: orgID, Name: name, Description: nilIfBlank(req.Description),
		IsDefault: req.IsDefault, CreatedBy: createdBy,
	}
	if err := s.repo.CreateScale(ctx, sc); err != nil {
		return nil, fmt.Errorf("performance: CreateScale: %w", err)
	}
	sc.Levels = []*RatingLevel{}
	return sc, nil
}

func (s *serviceImpl) UpdateScale(ctx context.Context, orgID, ref string, req UpdateScaleRequest) (*RatingScale, error) {
	sc, err := s.repo.FindScaleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: UpdateScale: %w", err)
	}
	if sc == nil {
		return nil, ErrScaleNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrScaleNameRequired
		}
		taken, err := s.repo.ScaleNameExists(ctx, orgID, name, sc.ID)
		if err != nil {
			return nil, fmt.Errorf("performance: UpdateScale: name check: %w", err)
		}
		if taken {
			return nil, ErrScaleNameTaken
		}
		sc.Name = name
	}
	if req.Description != nil {
		sc.Description = nilIfBlank(req.Description)
	}
	if req.IsActive != nil {
		sc.IsActive = *req.IsActive
	}

	// Promotion goes through the atomic clear-then-set path; a plain UPDATE
	// would 23505 against the partial unique index.
	promoting := req.IsDefault != nil && *req.IsDefault && !sc.IsDefault
	if req.IsDefault != nil && !*req.IsDefault {
		sc.IsDefault = false
	}

	if err := s.repo.UpdateScale(ctx, sc); err != nil {
		return nil, fmt.Errorf("performance: UpdateScale: %w", err)
	}
	if promoting {
		if err := s.repo.SetScaleDefault(ctx, orgID, sc.ID); err != nil {
			return nil, fmt.Errorf("performance: UpdateScale: promote: %w", err)
		}
		sc.IsDefault = true
	}
	return sc, nil
}

func (s *serviceImpl) DeleteScale(ctx context.Context, orgID, ref string) error {
	sc, err := s.repo.FindScaleByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("performance: DeleteScale: %w", err)
	}
	if sc == nil {
		return ErrScaleNotFound
	}

	// The FK is RESTRICT; checking first turns a raw 23503 into a message
	// that says what to do about it.
	count, err := s.repo.CountCyclesUsingScale(ctx, sc.ID)
	if err != nil {
		return fmt.Errorf("performance: DeleteScale: usage check: %w", err)
	}
	if count > 0 {
		return ErrScaleInUse
	}

	if err := s.repo.DeleteScale(ctx, orgID, sc.ID); err != nil {
		return fmt.Errorf("performance: DeleteScale: %w", err)
	}
	return nil
}

func (s *serviceImpl) CreateLevel(ctx context.Context, orgID, scaleRef string, req CreateLevelRequest) (*RatingLevel, error) {
	sc, err := s.repo.FindScaleByRef(ctx, orgID, scaleRef)
	if err != nil {
		return nil, fmt.Errorf("performance: CreateLevel: %w", err)
	}
	if sc == nil {
		return nil, ErrScaleNotFound
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return nil, ErrLevelLabelReq
	}
	taken, err := s.repo.LevelLabelExists(ctx, sc.ID, label, "")
	if err != nil {
		return nil, fmt.Errorf("performance: CreateLevel: label check: %w", err)
	}
	if taken {
		return nil, ErrLevelLabelTaken
	}

	l := &RatingLevel{
		ScaleID: sc.ID, Label: label, Description: nilIfBlank(req.Description),
		Value: req.Value, Color: nilIfBlank(req.Color),
	}
	if req.DisplayOrder != nil {
		l.DisplayOrder = *req.DisplayOrder
	}
	if err := s.repo.CreateLevel(ctx, l); err != nil {
		return nil, fmt.Errorf("performance: CreateLevel: %w", err)
	}
	return l, nil
}

func (s *serviceImpl) UpdateLevel(ctx context.Context, orgID, ref string, req UpdateLevelRequest) (*RatingLevel, error) {
	l, err := s.repo.FindLevelByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: UpdateLevel: %w", err)
	}
	if l == nil {
		return nil, ErrLevelNotFound
	}

	if req.Label != nil {
		label := strings.TrimSpace(*req.Label)
		if label == "" {
			return nil, ErrLevelLabelReq
		}
		taken, err := s.repo.LevelLabelExists(ctx, l.ScaleID, label, l.ID)
		if err != nil {
			return nil, fmt.Errorf("performance: UpdateLevel: label check: %w", err)
		}
		if taken {
			return nil, ErrLevelLabelTaken
		}
		l.Label = label
	}
	if req.Description != nil {
		l.Description = nilIfBlank(req.Description)
	}
	if req.Value != nil {
		l.Value = *req.Value
	}
	if req.DisplayOrder != nil {
		l.DisplayOrder = *req.DisplayOrder
	}
	if req.Color != nil {
		l.Color = nilIfBlank(req.Color)
	}

	if err := s.repo.UpdateLevel(ctx, orgID, l); err != nil {
		return nil, fmt.Errorf("performance: UpdateLevel: %w", err)
	}
	return l, nil
}

func (s *serviceImpl) DeleteLevel(ctx context.Context, orgID, ref string) error {
	l, err := s.repo.FindLevelByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("performance: DeleteLevel: %w", err)
	}
	if l == nil {
		return ErrLevelNotFound
	}
	// Appraisals referencing this level keep their label+value snapshot; the
	// FK is ON DELETE SET NULL precisely so historical ratings survive.
	if err := s.repo.DeleteLevel(ctx, orgID, l.ID); err != nil {
		return fmt.Errorf("performance: DeleteLevel: %w", err)
	}
	return nil
}
