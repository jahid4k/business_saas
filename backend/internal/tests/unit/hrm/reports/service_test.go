package reports_test

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/reports"
)

type mockReportsRepo struct{}

func (m *mockReportsRepo) GetSummary(ctx context.Context, orgID string) (*reports.HRMSummary, error) {
	if orgID != "org1" {
		return &reports.HRMSummary{}, nil
	}
	return &reports.HRMSummary{
		TotalEmployees: 10,
	}, nil
}

func (m *mockReportsRepo) GetHeadcountByDepartment(ctx context.Context, orgID string) ([]*reports.HeadcountByDepartment, error) {
	if orgID != "org1" {
		return []*reports.HeadcountByDepartment{}, nil
	}
	return []*reports.HeadcountByDepartment{
		{DepartmentName: "Engineering", Headcount: 10},
	}, nil
}

func (m *mockReportsRepo) GetLeaveSummary(ctx context.Context, orgID string) ([]*reports.LeaveSummaryByType, error) {
	if orgID != "org1" {
		return []*reports.LeaveSummaryByType{}, nil
	}
	return []*reports.LeaveSummaryByType{
		{LeaveTypeName: "Sick Leave", TotalRequests: 5},
	}, nil
}

func TestReportsService(t *testing.T) {
	repo := &mockReportsRepo{}
	svc := reports.NewService(repo)
	ctx := context.Background()

	orgID := "org1"

	t.Run("GetSummary", func(t *testing.T) {
		summary, err := svc.GetSummary(ctx, orgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.TotalEmployees != 10 {
			t.Errorf("expected 10 total employees, got %d", summary.TotalEmployees)
		}
	})

	t.Run("GetHeadcountByDepartment", func(t *testing.T) {
		hc, err := svc.GetHeadcountByDepartment(ctx, orgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hc) != 1 || hc[0].DepartmentName != "Engineering" {
			t.Errorf("unexpected headcount result: %v", hc)
		}
	})

	t.Run("GetLeaveSummary", func(t *testing.T) {
		ls, err := svc.GetLeaveSummary(ctx, orgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ls) != 1 || ls[0].LeaveTypeName != "Sick Leave" {
			t.Errorf("unexpected leave summary result: %v", ls)
		}
	})

	t.Run("Cross-Org Isolation", func(t *testing.T) {
		summary, _ := svc.GetSummary(ctx, "org2")
		if summary.TotalEmployees != 0 {
			t.Errorf("expected 0 employees for org2, got %d", summary.TotalEmployees)
		}
	})
}
