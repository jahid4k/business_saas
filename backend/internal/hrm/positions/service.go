// backend/internal/hrm/positions/service.go
package positions

import (
	"context"
	"fmt"
	"strings"
)

// Service defines business logic for HRM positions.
type Service interface {
	List(ctx context.Context, orgID, departmentID string, activeOnly bool) (*PositionListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Position, error)
	Create(ctx context.Context, orgID, createdBy string, req CreatePositionRequest) (*Position, error)
	Update(ctx context.Context, orgID, ref string, req UpdatePositionRequest) (*Position, error)
	Delete(ctx context.Context, orgID, ref string) error
}

type serviceImpl struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) List(ctx context.Context, orgID, departmentID string, activeOnly bool) (*PositionListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, departmentID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("positions: List: %w", err)
	}
	if list == nil {
		list = []*Position{}
	}
	total, err := s.repo.Count(ctx, orgID, departmentID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("positions: List: count: %w", err)
	}
	return &PositionListResponse{Positions: list, Total: total}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Position, error) {
	p, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("positions: Get: %w", err)
	}
	if p == nil {
		return nil, ErrPositionNotFound
	}
	return p, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreatePositionRequest) (*Position, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	if len(title) > 150 {
		return nil, ErrTitleTooLong
	}
	conflict, err := s.repo.ExistsByTitle(ctx, orgID, title, "")
	if err != nil {
		return nil, fmt.Errorf("positions: Create: title check: %w", err)
	}
	if conflict {
		return nil, ErrTitleConflict
	}

	p := &Position{
		OrgID:     orgID,
		Title:     title,
		IsActive:  true,
		CreatedBy: createdBy,
	}
	if req.Description != nil {
		p.Description = req.Description
	}
	if req.DepartmentID != nil && strings.TrimSpace(*req.DepartmentID) != "" {
		p.DepartmentID = req.DepartmentID
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("positions: Create: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdatePositionRequest) (*Position, error) {
	p, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("positions: Update: %w", err)
	}
	if p == nil {
		return nil, ErrPositionNotFound
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		if len(title) > 150 {
			return nil, ErrTitleTooLong
		}
		conflict, err := s.repo.ExistsByTitle(ctx, orgID, title, p.ID)
		if err != nil {
			return nil, fmt.Errorf("positions: Update: title check: %w", err)
		}
		if conflict {
			return nil, ErrTitleConflict
		}
		p.Title = title
	}
	if req.Description != nil {
		p.Description = req.Description
	}
	if req.DepartmentID != nil {
		dRef := strings.TrimSpace(*req.DepartmentID)
		if dRef == "" {
			p.DepartmentID = nil
		} else {
			p.DepartmentID = &dRef
		}
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("positions: Update: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) Delete(ctx context.Context, orgID, ref string) error {
	p, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("positions: Delete: %w", err)
	}
	if p == nil {
		return ErrPositionNotFound
	}
	if err := s.repo.Delete(ctx, orgID, ref); err != nil {
		return fmt.Errorf("positions: Delete: %w", err)
	}
	return nil
}
