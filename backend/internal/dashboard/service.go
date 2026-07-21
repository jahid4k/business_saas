package dashboard

import (
	"context"
	"sort"

	"golang.org/x/sync/errgroup"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetDashboardMetrics(ctx context.Context, orgID string) (*DashboardResponse, error) {
	resp := &DashboardResponse{
		ActionItems: make([]*ActionItem, 0),
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		val, err := s.repo.GetActivePipelineValue(gCtx, orgID)
		if err == nil {
			resp.KPIs.ActivePipelineValue = val
		}
		return err
	})

	g.Go(func() error {
		count, err := s.repo.GetTotalHeadcount(gCtx, orgID)
		if err == nil {
			resp.KPIs.TotalHeadcount = count
		}
		return err
	})

	g.Go(func() error {
		count, err := s.repo.GetPendingApprovalsCount(gCtx, orgID)
		if err == nil {
			resp.KPIs.PendingApprovals = count
		}
		return err
	})

	var stagnantDeals []*ActionItem
	g.Go(func() error {
		items, err := s.repo.GetStagnantDeals(gCtx, orgID, 7, 5)
		if err == nil {
			stagnantDeals = items
		}
		return err
	})

	var pendingApprovals []*ActionItem
	g.Go(func() error {
		items, err := s.repo.GetPendingApprovalItems(gCtx, orgID, 5)
		if err == nil {
			pendingApprovals = items
		}
		// If created_at is missing, we ignore the error and just don't return pending approvals
		// as action items, to avoid breaking the whole dashboard.
		if err != nil {
			return nil
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Combine and sort action items by timestamp (oldest first, so they need attention!)
	resp.ActionItems = append(resp.ActionItems, stagnantDeals...)
	resp.ActionItems = append(resp.ActionItems, pendingApprovals...)

	sort.Slice(resp.ActionItems, func(i, j int) bool {
		return resp.ActionItems[i].Timestamp.Before(resp.ActionItems[j].Timestamp)
	})

	// Limit to top 5
	if len(resp.ActionItems) > 5 {
		resp.ActionItems = resp.ActionItems[:5]
	}

	return resp, nil
}
