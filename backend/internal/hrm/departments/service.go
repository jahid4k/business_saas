// backend/internal/hrm/departments/service.go
package departments

import (
	"context"
	"fmt"
	"strings"
)

// Service defines the business logic interface for HRM departments.
type Service interface {
	List(ctx context.Context, orgID string, activeOnly bool) (*DepartmentListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Department, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateDepartmentRequest) (*Department, error)
	Update(ctx context.Context, orgID, ref string, req UpdateDepartmentRequest) (*Department, error)
	Delete(ctx context.Context, orgID, ref string) error
}

type serviceImpl struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) List(ctx context.Context, orgID string, activeOnly bool) (*DepartmentListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("departments: List: %w", err)
	}
	if list == nil {
		list = []*Department{}
	}
	total, err := s.repo.Count(ctx, orgID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("departments: List: count: %w", err)
	}
	return &DepartmentListResponse{Departments: list, Total: total}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Department, error) {
	d, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("departments: Get: %w", err)
	}
	if d == nil {
		return nil, ErrDepartmentNotFound
	}
	return d, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateDepartmentRequest) (*Department, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(name) > 150 {
		return nil, ErrNameTooLong
	}

	conflict, err := s.repo.ExistsByName(ctx, orgID, name, "")
	if err != nil {
		return nil, fmt.Errorf("departments: Create: name check: %w", err)
	}
	if conflict {
		return nil, ErrNameConflict
	}

	d := &Department{
		OrgID:     orgID,
		Name:      name,
		IsActive:  true,
		CreatedBy: createdBy,
	}
	if req.Description != nil {
		d.Description = req.Description
	}
	if req.ParentDepartmentID != nil && strings.TrimSpace(*req.ParentDepartmentID) != "" {
		d.ParentDepartmentID = req.ParentDepartmentID
	}
	if req.HeadEmployeeID != nil && strings.TrimSpace(*req.HeadEmployeeID) != "" {
		d.HeadEmployeeID = req.HeadEmployeeID
	}

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("departments: Create: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdateDepartmentRequest) (*Department, error) {
	d, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("departments: Update: %w", err)
	}
	if d == nil {
		return nil, ErrDepartmentNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		if len(name) > 150 {
			return nil, ErrNameTooLong
		}
		conflict, err := s.repo.ExistsByName(ctx, orgID, name, d.ID)
		if err != nil {
			return nil, fmt.Errorf("departments: Update: name check: %w", err)
		}
		if conflict {
			return nil, ErrNameConflict
		}
		d.Name = name
	}
	if req.Description != nil {
		d.Description = req.Description
	}
	if req.ParentDepartmentID != nil {
		ref := strings.TrimSpace(*req.ParentDepartmentID)
		if ref == "" {
			d.ParentDepartmentID = nil
		} else {
			if ref == d.ID || ref == d.PublicID {
				return nil, ErrCircularParent
			}
			d.ParentDepartmentID = &ref
		}
	}
	if req.HeadEmployeeID != nil {
		ref := strings.TrimSpace(*req.HeadEmployeeID)
		if ref == "" {
			d.HeadEmployeeID = nil
		} else {
			d.HeadEmployeeID = &ref
		}
	}
	if req.IsActive != nil {
		d.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, d); err != nil {
		return nil, fmt.Errorf("departments: Update: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) Delete(ctx context.Context, orgID, ref string) error {
	d, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("departments: Delete: %w", err)
	}
	if d == nil {
		return ErrDepartmentNotFound
	}
	if err := s.repo.Delete(ctx, orgID, ref); err != nil {
		return fmt.Errorf("departments: Delete: %w", err)
	}
	return nil
}
