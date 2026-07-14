// backend/internal/tests/unit/crm/reports_service_test.go
package crm

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/crm/deals"
	"github.com/mridha/businesssaas/internal/crm/leads"
	"github.com/mridha/businesssaas/internal/crm/reports"
	"github.com/mridha/businesssaas/internal/platform/engagement"
)

// ── Stubs ───────────────────────────────────────────────────────────────────

type stubReportsRepo struct {
	summary *reports.CRMSummary
}

func (r *stubReportsRepo) GetSummary(ctx context.Context, orgID string) (*reports.CRMSummary, error) {
	return r.summary, nil
}

type stubDealsSvc struct {
	deals.Service
	dealsByStage []*deals.DealsByStage
	dealsByOwner []*deals.DealsByOwner
	recentDeals  []*deals.Deal
}

func (s *stubDealsSvc) GetDealsByStage(ctx context.Context, orgID string) ([]*deals.DealsByStage, error) {
	return s.dealsByStage, nil
}

func (s *stubDealsSvc) GetDealsByOwner(ctx context.Context, orgID string) ([]*deals.DealsByOwner, error) {
	return s.dealsByOwner, nil
}

func (s *stubDealsSvc) GetRecentDeals(ctx context.Context, orgID string, limit int) ([]*deals.Deal, error) {
	return s.recentDeals, nil
}

type stubLeadsSvc struct {
	leads.Service
	leadsBySource []*leads.LeadsBySource
}

func (s *stubLeadsSvc) GetLeadsBySource(ctx context.Context, orgID string) ([]*leads.LeadsBySource, error) {
	return s.leadsBySource, nil
}

type stubEngagementSvc struct {
	engagement.Service
	overdueTasks  []*engagement.Task
	activityStats map[string]int
}

func (s *stubEngagementSvc) GetOverdueTasks(ctx context.Context, orgID string) ([]*engagement.Task, error) {
	return s.overdueTasks, nil
}

func (s *stubEngagementSvc) GetActivityCountByType(ctx context.Context, orgID string) (map[string]int, error) {
	return s.activityStats, nil
}

// ── Tests ───────────────────────────────────────────────────────────────────

func TestGetSummary(t *testing.T) {
	repo := &stubReportsRepo{
		summary: &reports.CRMSummary{
			TotalDeals: 10,
			WonDeals:   5,
		},
	}
	svc := reports.NewService(repo, &stubDealsSvc{}, &stubLeadsSvc{}, &stubEngagementSvc{})

	summary, err := svc.GetSummary(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if summary.TotalDeals != 10 {
		t.Errorf("expected 10 total deals, got %d", summary.TotalDeals)
	}
	if summary.WonDeals != 5 {
		t.Errorf("expected 5 won deals, got %d", summary.WonDeals)
	}
}

func TestGetDealsByStage(t *testing.T) {
	dealsSvc := &stubDealsSvc{
		dealsByStage: []*deals.DealsByStage{
			{StageID: "s1", Count: 5},
		},
	}
	svc := reports.NewService(&stubReportsRepo{}, dealsSvc, &stubLeadsSvc{}, &stubEngagementSvc{})

	res, err := svc.GetDealsByStage(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 1 || res[0].Count != 5 {
		t.Errorf("expected 1 result with count 5")
	}
}

func TestGetDealsByOwner(t *testing.T) {
	dealsSvc := &stubDealsSvc{
		dealsByOwner: []*deals.DealsByOwner{
			{OwnerName: "Alice", Count: 3},
		},
	}
	svc := reports.NewService(&stubReportsRepo{}, dealsSvc, &stubLeadsSvc{}, &stubEngagementSvc{})

	res, err := svc.GetDealsByOwner(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 1 || res[0].OwnerName != "Alice" {
		t.Errorf("expected 1 result with OwnerName 'Alice'")
	}
}

func TestGetLeadsBySource(t *testing.T) {
	leadsSvc := &stubLeadsSvc{
		leadsBySource: []*leads.LeadsBySource{
			{Source: "Website", Count: 10},
		},
	}
	svc := reports.NewService(&stubReportsRepo{}, &stubDealsSvc{}, leadsSvc, &stubEngagementSvc{})

	res, err := svc.GetLeadsBySource(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 1 || res[0].Source != "Website" {
		t.Errorf("expected 1 result with Source 'Website'")
	}
}

func TestGetOverdueTasks(t *testing.T) {
	// A task that was due 2 days ago
	twoDaysAgo := time.Now().Add(-2 * 24 * time.Hour)
	engSvc := &stubEngagementSvc{
		overdueTasks: []*engagement.Task{
			{ID: "t1", Title: "Call Bob", DueDate: &twoDaysAgo},
		},
	}
	svc := reports.NewService(&stubReportsRepo{}, &stubDealsSvc{}, &stubLeadsSvc{}, engSvc)

	res, err := svc.GetOverdueTasks(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result")
	}
	if res[0].DaysOverdue != 2 {
		t.Errorf("expected DaysOverdue to be 2, got %d", res[0].DaysOverdue)
	}
	if res[0].Title != "Call Bob" {
		t.Errorf("expected Title 'Call Bob', got %q", res[0].Title)
	}
}

func TestGetActivityStats(t *testing.T) {
	engSvc := &stubEngagementSvc{
		activityStats: map[string]int{
			"email": 10,
			"call":  5,
		},
	}
	svc := reports.NewService(&stubReportsRepo{}, &stubDealsSvc{}, &stubLeadsSvc{}, engSvc)

	res, err := svc.GetActivityStats(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 activity stats, got %d", len(res))
	}
	// Note: Map iteration is random, so we just check total length
	foundEmail := false
	for _, stat := range res {
		if stat.Type == "email" && stat.Count == 10 {
			foundEmail = true
		}
	}
	if !foundEmail {
		t.Error("expected email stat with count 10")
	}
}

func TestGetRecentDeals(t *testing.T) {
	dealsSvc := &stubDealsSvc{
		recentDeals: []*deals.Deal{
			{ID: "d1", Title: "Deal 1"},
		},
	}
	svc := reports.NewService(&stubReportsRepo{}, dealsSvc, &stubLeadsSvc{}, &stubEngagementSvc{})

	res, err := svc.GetRecentDeals(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 1 || res[0].Title != "Deal 1" {
		t.Errorf("expected 1 result with Title 'Deal 1'")
	}
}
