// backend/internal/crm/pipeline/service.go
package pipeline

import (
	"context"
	"fmt"
	"strings"
)

// Service defines the business logic for pipelines and stages.
type Service interface {
	// Pipelines
	ListPipelines(ctx context.Context, orgID string) (*PipelineListResponse, error)
	GetPipeline(ctx context.Context, orgID, pipelineID string) (*Pipeline, error)
	CreatePipeline(ctx context.Context, orgID, userID string, req CreatePipelineRequest) (*Pipeline, error)
	UpdatePipeline(ctx context.Context, orgID, pipelineID string, req UpdatePipelineRequest) (*Pipeline, error)
	DeletePipeline(ctx context.Context, orgID, pipelineID string) error

	// Stages
	ListStages(ctx context.Context, orgID, pipelineID string) (*StageListResponse, error)
	GetStage(ctx context.Context, orgID, stageID string) (*Stage, error)
	CreateStage(ctx context.Context, orgID, pipelineID string, req CreateStageRequest) (*Stage, error)
	UpdateStage(ctx context.Context, orgID, pipelineID, stageID string, req UpdateStageRequest) (*Stage, error)
	DeleteStage(ctx context.Context, orgID, pipelineID, stageID string) error
	ReorderStages(ctx context.Context, orgID, pipelineID string, req ReorderStagesRequest) error
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new pipeline service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) ListPipelines(ctx context.Context, orgID string) (*PipelineListResponse, error) {
	pipelines, err := s.repo.FindPipelines(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: ListPipelines: %w", err)
	}
	if pipelines == nil {
		pipelines = []*Pipeline{}
	}
	total, err := s.repo.CountPipelines(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: ListPipelines: count: %w", err)
	}
	return &PipelineListResponse{Pipelines: pipelines, Total: total}, nil
}

func (s *serviceImpl) GetPipeline(ctx context.Context, orgID, pipelineID string) (*Pipeline, error) {
	p, err := s.repo.FindPipelineByID(ctx, orgID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: GetPipeline: %w", err)
	}
	if p == nil {
		return nil, ErrPipelineNotFound
	}
	stages, err := s.repo.FindStagesByPipeline(ctx, orgID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: GetPipeline: stages: %w", err)
	}
	if stages == nil {
		stages = []*Stage{}
	}
	p.Stages = stages
	return p, nil
}

func (s *serviceImpl) CreatePipeline(ctx context.Context, orgID, userID string, req CreatePipelineRequest) (*Pipeline, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrNameRequired
	}
	p := &Pipeline{
		OrgID:       orgID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		IsDefault:   req.IsDefault,
		CreatedBy:   userID,
	}
	if err := s.repo.CreatePipeline(ctx, p); err != nil {
		return nil, fmt.Errorf("pipeline: CreatePipeline: %w", err)
	}

	// Auto-provision default stages
	p0, p1, p2 := 0, 1, 2
	prob10, prob50, prob100 := 10, 50, 100
	defaultStages := []CreateStageRequest{
		{Name: "Prospecting", Position: &p0, Probability: &prob10},
		{Name: "Negotiation", Position: &p1, Probability: &prob50},
		{Name: "Closing", Position: &p2, Probability: &prob100},
	}
	for _, reqStage := range defaultStages {
		_, _ = s.CreateStage(ctx, orgID, p.ID, reqStage)
	}

	return p, nil
}

func (s *serviceImpl) UpdatePipeline(ctx context.Context, orgID, pipelineID string, req UpdatePipelineRequest) (*Pipeline, error) {
	p, err := s.repo.FindPipelineByID(ctx, orgID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: UpdatePipeline: %w", err)
	}
	if p == nil {
		return nil, ErrPipelineNotFound
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		p.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		p.Description = req.Description
	}
	if req.IsDefault != nil {
		p.IsDefault = *req.IsDefault
	}
	if err := s.repo.UpdatePipeline(ctx, p); err != nil {
		return nil, fmt.Errorf("pipeline: UpdatePipeline: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) DeletePipeline(ctx context.Context, orgID, pipelineID string) error {
	return s.repo.DeletePipeline(ctx, orgID, pipelineID)
}

func (s *serviceImpl) ListStages(ctx context.Context, orgID, pipelineID string) (*StageListResponse, error) {
	if _, err := s.GetPipeline(ctx, orgID, pipelineID); err != nil {
		return nil, err
	}
	stages, err := s.repo.FindStagesByPipeline(ctx, orgID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: ListStages: %w", err)
	}
	if stages == nil {
		stages = []*Stage{}
	}
	return &StageListResponse{Stages: stages, Total: len(stages)}, nil
}

func (s *serviceImpl) GetStage(ctx context.Context, orgID, stageID string) (*Stage, error) {
	st, err := s.repo.FindStageByID(ctx, orgID, stageID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: GetStage: %w", err)
	}
	if st == nil {
		return nil, ErrStageNotFound
	}
	return st, nil
}

func (s *serviceImpl) CreateStage(ctx context.Context, orgID, pipelineID string, req CreateStageRequest) (*Stage, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrNameRequired
	}
	if _, err := s.repo.FindPipelineByID(ctx, orgID, pipelineID); err != nil {
		return nil, fmt.Errorf("pipeline: CreateStage: verify pipeline: %w", err)
	}
	position := 0
	if req.Position != nil {
		position = *req.Position
	}
	probability := 0
	if req.Probability != nil {
		probability = *req.Probability
	}
	st := &Stage{
		OrgID:       orgID,
		PipelineID:  pipelineID,
		Name:        strings.TrimSpace(req.Name),
		Position:    position,
		Probability: probability,
	}
	if err := s.repo.CreateStage(ctx, st); err != nil {
		return nil, fmt.Errorf("pipeline: CreateStage: %w", err)
	}
	return st, nil
}

func (s *serviceImpl) UpdateStage(ctx context.Context, orgID, pipelineID, stageID string, req UpdateStageRequest) (*Stage, error) {
	st, err := s.repo.FindStageByID(ctx, orgID, stageID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: UpdateStage: %w", err)
	}
	if st == nil {
		return nil, ErrStageNotFound
	}
	if st.PipelineID != pipelineID {
		return nil, ErrStageNotInPipeline
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		st.Name = strings.TrimSpace(*req.Name)
	}
	if req.Position != nil {
		st.Position = *req.Position
	}
	if req.Probability != nil {
		st.Probability = *req.Probability
	}
	if err := s.repo.UpdateStage(ctx, st); err != nil {
		return nil, fmt.Errorf("pipeline: UpdateStage: %w", err)
	}
	return st, nil
}

func (s *serviceImpl) DeleteStage(ctx context.Context, orgID, pipelineID, stageID string) error {
	st, err := s.repo.FindStageByID(ctx, orgID, stageID)
	if err != nil {
		return fmt.Errorf("pipeline: DeleteStage: %w", err)
	}
	if st == nil {
		return ErrStageNotFound
	}
	if st.PipelineID != pipelineID {
		return ErrStageNotInPipeline
	}
	return s.repo.DeleteStage(ctx, orgID, stageID)
}

func (s *serviceImpl) ReorderStages(ctx context.Context, orgID, pipelineID string, req ReorderStagesRequest) error {
	if len(req.StageIDs) == 0 {
		return nil
	}
	return s.repo.ReorderStages(ctx, orgID, pipelineID, req.StageIDs)
}
