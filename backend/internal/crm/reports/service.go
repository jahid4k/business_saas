// backend/internal/crm/reports/service.go
package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/mridha/businesssaas/internal/crm/deals"
	"github.com/mridha/businesssaas/internal/crm/leads"
	"github.com/mridha/businesssaas/internal/platform/engagement"
)

// Service defines the business logic for CRM reports.
// It delegates to domain services and the reports repository;
// it never owns raw SQL or a database connection directly.
type Service interface {
	GetSummary(ctx context.Context, orgID string) (*CRMSummary, error)
	GetDealsByStage(ctx context.Context, orgID string) ([]*deals.DealsByStage, error)
	GetDealsByOwner(ctx context.Context, orgID string) ([]*deals.DealsByOwner, error)
	GetLeadsBySource(ctx context.Context, orgID string) ([]*leads.LeadsBySource, error)
	GetOverdueTasks(ctx context.Context, orgID string) ([]*OverdueTask, error)
	GetActivityStats(ctx context.Context, orgID string) ([]*ActivityStat, error)
	GetRecentDeals(ctx context.Context, orgID string) ([]*deals.Deal, error)
	GetAgenda(ctx context.Context, orgID string) (*Agenda, error)
	GetRepPerformance(ctx context.Context, orgID string) ([]*RepPerformance, error)
	GetForecast(ctx context.Context, orgID string) (*Forecast, error)
}

type serviceImpl struct {
	repo          Repository // reports.Repository — owns GetSummary SQL
	dealsSvc      deals.Service
	leadsSvc      leads.Service
	engagementSvc engagement.Service
}

// NewService creates a new reports service.
func NewService(
	repo Repository,
	dealsSvc deals.Service,
	leadsSvc leads.Service,
	engagementSvc engagement.Service,
) Service {
	return &serviceImpl{
		repo:          repo,
		dealsSvc:      dealsSvc,
		leadsSvc:      leadsSvc,
		engagementSvc: engagementSvc,
	}
}

func (s *serviceImpl) GetSummary(ctx context.Context, orgID string) (*CRMSummary, error) {
	summary, err := s.repo.GetSummary(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("reports: GetSummary: %w", err)
	}
	return summary, nil
}

func (s *serviceImpl) GetDealsByStage(ctx context.Context, orgID string) ([]*deals.DealsByStage, error) {
	return s.dealsSvc.GetDealsByStage(ctx, orgID)
}

func (s *serviceImpl) GetDealsByOwner(ctx context.Context, orgID string) ([]*deals.DealsByOwner, error) {
	return s.dealsSvc.GetDealsByOwner(ctx, orgID)
}

func (s *serviceImpl) GetLeadsBySource(ctx context.Context, orgID string) ([]*leads.LeadsBySource, error) {
	return s.leadsSvc.GetLeadsBySource(ctx, orgID)
}

func (s *serviceImpl) GetOverdueTasks(ctx context.Context, orgID string) ([]*OverdueTask, error) {
	tasks, err := s.engagementSvc.GetOverdueTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("reports: GetOverdueTasks: %w", err)
	}
	now := time.Now()
	result := make([]*OverdueTask, 0, len(tasks))
	for _, t := range tasks {
		days := 0
		if t.DueDate != nil {
			days = int(now.Sub(*t.DueDate).Hours() / 24)
		}
		result = append(result, &OverdueTask{
			TaskID:      t.ID,
			Title:       t.Title,
			DaysOverdue: days,
		})
	}
	return result, nil
}

func (s *serviceImpl) GetActivityStats(ctx context.Context, orgID string) ([]*ActivityStat, error) {
	counts, err := s.engagementSvc.GetActivityCountByType(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("reports: GetActivityStats: %w", err)
	}
	result := make([]*ActivityStat, 0, len(counts))
	for t, count := range counts {
		result = append(result, &ActivityStat{Type: t, Count: count})
	}
	return result, nil
}

func (s *serviceImpl) GetRecentDeals(ctx context.Context, orgID string) ([]*deals.Deal, error) {
	return s.dealsSvc.GetRecentDeals(ctx, orgID, 5)
}

func (s *serviceImpl) GetAgenda(ctx context.Context, orgID string) (*Agenda, error) {
	agenda := &Agenda{
		Items: make([]AgendaItem, 0),
	}

	// 1. Get Tasks (today's tasks + overdue)
	tasksRes, err := s.engagementSvc.ListTasksByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("reports: GetAgenda: tasks: %w", err)
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, t := range tasksRes.Tasks {
		if t.Status == "done" || t.Status == "cancelled" {
			if t.UpdatedAt.Before(startOfToday) {
				continue
			}
		}

		dueDateStr := ""
		if t.DueDate != nil {
			dueDateStr = t.DueDate.Format(time.RFC3339)
		}
		desc := ""
		if t.Description != nil {
			desc = *t.Description
		}
		agenda.Items = append(agenda.Items, AgendaItem{
			ID:          t.ID,
			Type:        "task",
			Title:       t.Title,
			Description: desc,
			Status:      string(t.Status),
			DueDate:     &dueDateStr,
			RelatedType: string(t.RelatedType),
			RelatedID:   t.RelatedID,
			UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
		})
	}

	return agenda, nil
}

func (s *serviceImpl) GetRepPerformance(ctx context.Context, orgID string) ([]*RepPerformance, error) {
	result, err := s.repo.GetRepPerformance(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("reports: GetRepPerformance: %w", err)
	}
	return result, nil
}

func (s *serviceImpl) GetForecast(ctx context.Context, orgID string) (*Forecast, error) {
	result, err := s.repo.GetForecast(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("reports: GetForecast: %w", err)
	}
	return result, nil
}
