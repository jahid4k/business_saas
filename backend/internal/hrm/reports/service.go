// backend/internal/hrm/reports/service.go
package reports

import (
	"context"
	"fmt"
)

// Service defines business logic for HRM reports.
// It delegates directly to the repository; no compute-heavy transforms
// are required at this layer — all aggregation happens in SQL.
type Service interface {
	GetSummary(ctx context.Context, orgID string) (*HRMSummary, error)
	GetHeadcountByDepartment(ctx context.Context, orgID string) ([]*HeadcountByDepartment, error)
	GetLeaveSummary(ctx context.Context, orgID string) ([]*LeaveSummaryByType, error)
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new HRM reports service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) GetSummary(ctx context.Context, orgID string) (*HRMSummary, error) {
	summary, err := s.repo.GetSummary(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("hrm reports: GetSummary: %w", err)
	}
	return summary, nil
}

func (s *serviceImpl) GetHeadcountByDepartment(ctx context.Context, orgID string) ([]*HeadcountByDepartment, error) {
	result, err := s.repo.GetHeadcountByDepartment(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("hrm reports: GetHeadcountByDepartment: %w", err)
	}
	return result, nil
}

func (s *serviceImpl) GetLeaveSummary(ctx context.Context, orgID string) ([]*LeaveSummaryByType, error) {
	result, err := s.repo.GetLeaveSummary(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("hrm reports: GetLeaveSummary: %w", err)
	}
	return result, nil
}
