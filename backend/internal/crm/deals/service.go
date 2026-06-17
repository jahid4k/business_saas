// backend/internal/crm/deals/service.go
package deals

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/internal/crm/leads"
	"github.com/mridha/businesssaas/internal/crm/pipeline"
)

// Service defines the business logic for CRM deals.
type Service interface {
	ListDeals(ctx context.Context, orgID string) (*DealListResponse, error)
	GetDeal(ctx context.Context, orgID, dealID string) (*Deal, error)
	CreateDeal(ctx context.Context, orgID, userID string, req CreateDealRequest) (*Deal, error)
	UpdateDeal(ctx context.Context, orgID, dealID string, req UpdateDealRequest) (*Deal, error)
	DeleteDeal(ctx context.Context, orgID, dealID string) error
	MoveDealStage(ctx context.Context, orgID, dealID string, req MoveDealStageRequest) (*Deal, error)
	MarkDealWon(ctx context.Context, orgID, dealID string) (*Deal, error)
	MarkDealLost(ctx context.Context, orgID, dealID string, req MarkLostRequest) (*Deal, error)
	GetDealsByContact(ctx context.Context, orgID, contactID string) ([]*Deal, error)
	GetDealsByCompany(ctx context.Context, orgID, companyID string) ([]*Deal, error)
	GetPipelineBoard(ctx context.Context, orgID, pipelineID string) (*PipelineBoard, error)
	GetRecentDeals(ctx context.Context, orgID string, limit int) ([]*Deal, error)
	// Report queries
	GetDealsByStage(ctx context.Context, orgID string) ([]*DealsByStage, error)
	GetDealsByOwner(ctx context.Context, orgID string) ([]*DealsByOwner, error)
	// CreateDealFromLeadTx implements leads.DealCreator.
	// Inserts a deal inside an existing transaction so the full lead conversion
	// (contact + deal + lead status) is atomic.
	CreateDealFromLeadTx(ctx context.Context, tx pgx.Tx, orgID, userID string, lead *leads.Lead, req leads.ConvertLeadRequest) (string, error)
}

type serviceImpl struct {
	repo        Repository
	pipelineSvc pipeline.Service
}

// NewService creates a new deals service.
// pipelineSvc is injected so deals can validate stage ownership without
// duplicating pipeline queries.
func NewService(repo Repository, pipelineSvc pipeline.Service) Service {
	return &serviceImpl{repo: repo, pipelineSvc: pipelineSvc}
}

func (s *serviceImpl) ListDeals(ctx context.Context, orgID string) (*DealListResponse, error) {
	ds, err := s.repo.FindDeals(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("deals: ListDeals: %w", err)
	}
	if ds == nil {
		ds = []*Deal{}
	}
	total, err := s.repo.CountDeals(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("deals: ListDeals: count: %w", err)
	}
	return &DealListResponse{Deals: ds, Total: total}, nil
}

func (s *serviceImpl) GetDeal(ctx context.Context, orgID, dealID string) (*Deal, error) {
	d, err := s.repo.FindDealByID(ctx, orgID, dealID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetDeal: %w", err)
	}
	if d == nil {
		return nil, ErrDealNotFound
	}
	return d, nil
}

func (s *serviceImpl) CreateDeal(ctx context.Context, orgID, userID string, req CreateDealRequest) (*Deal, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, ErrTitleRequired
	}
	if req.PipelineID == "" {
		return nil, ErrPipelineRequired
	}
	if req.StageID == "" {
		return nil, ErrStageRequired
	}
	currency := "USD"
	if req.Currency != nil && *req.Currency != "" {
		currency = *req.Currency
	}
	d := &Deal{
		OrgID:      orgID,
		Title:      strings.TrimSpace(req.Title),
		Value:      req.Value,
		Currency:   currency,
		PipelineID: req.PipelineID,
		StageID:    req.StageID,
		ContactID:  req.ContactID,
		CompanyID:  req.CompanyID,
		Status:     DealStatusOpen,
		OwnerID:    req.OwnerID,
		CreatedBy:  userID,
	}
	if err := s.repo.CreateDeal(ctx, d); err != nil {
		return nil, fmt.Errorf("deals: CreateDeal: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) UpdateDeal(ctx context.Context, orgID, dealID string, req UpdateDealRequest) (*Deal, error) {
	d, err := s.repo.FindDealByID(ctx, orgID, dealID)
	if err != nil {
		return nil, fmt.Errorf("deals: UpdateDeal: %w", err)
	}
	if d == nil {
		return nil, ErrDealNotFound
	}
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		d.Title = strings.TrimSpace(*req.Title)
	}
	if req.Value != nil {
		d.Value = *req.Value
	}
	if req.Currency != nil {
		d.Currency = *req.Currency
	}
	if req.ContactID != nil {
		d.ContactID = req.ContactID
	}
	if req.CompanyID != nil {
		d.CompanyID = req.CompanyID
	}
	if req.OwnerID != nil {
		d.OwnerID = req.OwnerID
	}
	if err := s.repo.UpdateDeal(ctx, d); err != nil {
		return nil, fmt.Errorf("deals: UpdateDeal: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) DeleteDeal(ctx context.Context, orgID, dealID string) error {
	return s.repo.SoftDeleteDeal(ctx, orgID, dealID)
}

func (s *serviceImpl) MoveDealStage(ctx context.Context, orgID, dealID string, req MoveDealStageRequest) (*Deal, error) {
	if req.StageID == "" {
		return nil, ErrStageRequired
	}
	d, err := s.repo.FindDealByID(ctx, orgID, dealID)
	if err != nil {
		return nil, fmt.Errorf("deals: MoveDealStage: %w", err)
	}
	if d == nil {
		return nil, ErrDealNotFound
	}
	// Verify the target stage belongs to the same pipeline as the deal.
	stage, err := s.pipelineSvc.GetStage(ctx, orgID, req.StageID)
	if err != nil {
		return nil, fmt.Errorf("deals: MoveDealStage: get stage: %w", err)
	}
	if stage == nil {
		return nil, ErrStageNotFound
	}
	if stage.PipelineID != d.PipelineID {
		return nil, ErrStageNotInPipeline
	}
	d.StageID = req.StageID
	if err := s.repo.UpdateDeal(ctx, d); err != nil {
		return nil, fmt.Errorf("deals: MoveDealStage: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) MarkDealWon(ctx context.Context, orgID, dealID string) (*Deal, error) {
	d, err := s.repo.FindDealByID(ctx, orgID, dealID)
	if err != nil {
		return nil, fmt.Errorf("deals: MarkDealWon: %w", err)
	}
	if d == nil {
		return nil, ErrDealNotFound
	}
	now := time.Now()
	d.Status = DealStatusWon
	d.WonAt = &now
	d.LostAt = nil
	d.LostReason = nil
	if err := s.repo.UpdateDeal(ctx, d); err != nil {
		return nil, fmt.Errorf("deals: MarkDealWon: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) MarkDealLost(ctx context.Context, orgID, dealID string, req MarkLostRequest) (*Deal, error) {
	d, err := s.repo.FindDealByID(ctx, orgID, dealID)
	if err != nil {
		return nil, fmt.Errorf("deals: MarkDealLost: %w", err)
	}
	if d == nil {
		return nil, ErrDealNotFound
	}
	now := time.Now()
	d.Status = DealStatusLost
	d.LostAt = &now
	d.WonAt = nil
	d.LostReason = req.LostReason
	if err := s.repo.UpdateDeal(ctx, d); err != nil {
		return nil, fmt.Errorf("deals: MarkDealLost: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) GetDealsByContact(ctx context.Context, orgID, contactID string) ([]*Deal, error) {
	ds, err := s.repo.FindDealsByContact(ctx, orgID, contactID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetDealsByContact: %w", err)
	}
	if ds == nil {
		ds = []*Deal{}
	}
	return ds, nil
}

func (s *serviceImpl) GetDealsByCompany(ctx context.Context, orgID, companyID string) ([]*Deal, error) {
	ds, err := s.repo.FindDealsByCompany(ctx, orgID, companyID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetDealsByCompany: %w", err)
	}
	if ds == nil {
		ds = []*Deal{}
	}
	return ds, nil
}

func (s *serviceImpl) GetPipelineBoard(ctx context.Context, orgID, pipelineID string) (*PipelineBoard, error) {
	p, err := s.pipelineSvc.GetPipeline(ctx, orgID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetPipelineBoard: %w", err)
	}

	board := &PipelineBoard{
		PipelineID:   p.ID,
		PipelineName: p.Name,
		Stages:       make([]*StageWithDeals, 0, len(p.Stages)),
	}

	for _, stage := range p.Stages {
		stageDeals, err := s.repo.FindDealsByStage(ctx, orgID, stage.ID)
		if err != nil {
			return nil, fmt.Errorf("deals: GetPipelineBoard: stage %s: %w", stage.ID, err)
		}
		if stageDeals == nil {
			stageDeals = []*Deal{}
		}
		board.Stages = append(board.Stages, &StageWithDeals{
			StageID:   stage.ID,
			StageName: stage.Name,
			Position:  stage.Position,
			Deals:     stageDeals,
			Total:     len(stageDeals),
		})
	}
	return board, nil
}

func (s *serviceImpl) GetRecentDeals(ctx context.Context, orgID string, limit int) ([]*Deal, error) {
	ds, err := s.repo.FindRecentDeals(ctx, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("deals: GetRecentDeals: %w", err)
	}
	if ds == nil {
		ds = []*Deal{}
	}
	return ds, nil
}

func (s *serviceImpl) GetDealsByStage(ctx context.Context, orgID string) ([]*DealsByStage, error) {
	result, err := s.repo.GetDealsByStage(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetDealsByStage: %w", err)
	}
	if result == nil {
		result = []*DealsByStage{}
	}
	return result, nil
}

func (s *serviceImpl) GetDealsByOwner(ctx context.Context, orgID string) ([]*DealsByOwner, error) {
	result, err := s.repo.GetDealsByOwner(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("deals: GetDealsByOwner: %w", err)
	}
	if result == nil {
		result = []*DealsByOwner{}
	}
	return result, nil
}

// CreateDealFromLeadTx implements leads.DealCreator.
// Inserts a deal inside the supplied transaction so the contact insert,
// deal insert, and lead status update are all committed or rolled back together.
func (s *serviceImpl) CreateDealFromLeadTx(ctx context.Context, tx pgx.Tx, orgID, userID string, lead *leads.Lead, req leads.ConvertLeadRequest) (string, error) {
	title := lead.FirstName
	if req.DealTitle != nil && *req.DealTitle != "" {
		title = *req.DealTitle
	}
	value := 0.0
	if req.DealValue != nil {
		value = *req.DealValue
	}
	d := &Deal{
		OrgID:      orgID,
		Title:      title,
		Value:      value,
		Currency:   "USD",
		PipelineID: *req.PipelineID,
		StageID:    *req.StageID,
		Status:     DealStatusOpen,
		OwnerID:    lead.OwnerID,
		CreatedBy:  userID,
	}
	if err := s.repo.CreateDealTx(ctx, tx, d); err != nil {
		return "", fmt.Errorf("deals: CreateDealFromLeadTx: %w", err)
	}
	return d.ID, nil
}
