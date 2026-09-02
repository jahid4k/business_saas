// backend/internal/hrm/recruitment/pipelines_service.go
package recruitment

import (
	"context"
	"fmt"
	"strings"
)

// PipelineService is embedded into Service — see service.go.
type PipelineService interface {
	ListPipelines(ctx context.Context, orgID string) ([]*Pipeline, error)
	GetPipeline(ctx context.Context, orgID, ref string) (*Pipeline, error)
	CreatePipeline(ctx context.Context, orgID, createdBy string, req CreatePipelineRequest) (*Pipeline, error)
	UpdatePipeline(ctx context.Context, orgID, ref string, req UpdatePipelineRequest) (*Pipeline, error)
	DeletePipeline(ctx context.Context, orgID, ref string) error

	ListStages(ctx context.Context, orgID, pipelineRef string) ([]*Stage, error)
	CreateStage(ctx context.Context, orgID, pipelineRef string, req CreateStageRequest) (*Stage, error)
	UpdateStage(ctx context.Context, orgID, pipelineRef, stageRef string, req UpdateStageRequest) (*Stage, error)
	DeleteStage(ctx context.Context, orgID, pipelineRef, stageRef string) error
	ReorderStages(ctx context.Context, orgID, pipelineRef string, req ReorderStagesRequest) error
}

func (s *serviceImpl) ListPipelines(ctx context.Context, orgID string) ([]*Pipeline, error) {
	list, err := s.repo.FindPipelines(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListPipelines: %w", err)
	}
	if list == nil {
		list = []*Pipeline{}
	}
	return list, nil
}

func (s *serviceImpl) GetPipeline(ctx context.Context, orgID, ref string) (*Pipeline, error) {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetPipeline: %w", err)
	}
	if p == nil {
		return nil, ErrPipelineNotFound
	}
	stages, err := s.repo.FindStages(ctx, orgID, p.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetPipeline: stages: %w", err)
	}
	if stages == nil {
		stages = []*Stage{}
	}
	p.Stages = stages
	return p, nil
}

func (s *serviceImpl) CreatePipeline(ctx context.Context, orgID, createdBy string, req CreatePipelineRequest) (*Pipeline, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrPipelineNameReq
	}
	p := &Pipeline{
		OrgID: orgID, Name: name, Description: req.Description,
		IsDefault: req.IsDefault, IsActive: true, CreatedBy: createdBy,
	}
	if err := s.repo.CreatePipeline(ctx, p); err != nil {
		return nil, fmt.Errorf("recruitment: CreatePipeline: %w", err)
	}
	p.Stages = []*Stage{}
	return p, nil
}

func (s *serviceImpl) UpdatePipeline(ctx context.Context, orgID, ref string, req UpdatePipelineRequest) (*Pipeline, error) {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdatePipeline: %w", err)
	}
	if p == nil {
		return nil, ErrPipelineNotFound
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrPipelineNameReq
		}
		p.Name = name
	}
	if req.Description != nil {
		p.Description = req.Description
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}

	wasDefault := p.IsDefault
	promote := req.IsDefault != nil && *req.IsDefault && !wasDefault
	if req.IsDefault != nil && !*req.IsDefault {
		p.IsDefault = false
	}

	if err := s.repo.UpdatePipeline(ctx, p); err != nil {
		return nil, fmt.Errorf("recruitment: UpdatePipeline: %w", err)
	}

	if promote {
		if err := s.repo.SetPipelineDefault(ctx, orgID, p.ID); err != nil {
			return nil, fmt.Errorf("recruitment: UpdatePipeline: set default: %w", err)
		}
		p.IsDefault = true
	}

	return p, nil
}

func (s *serviceImpl) DeletePipeline(ctx context.Context, orgID, ref string) error {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("recruitment: DeletePipeline: %w", err)
	}
	if p == nil {
		return ErrPipelineNotFound
	}
	if err := s.repo.DeletePipeline(ctx, orgID, p.ID); err != nil {
		return fmt.Errorf("recruitment: DeletePipeline: %w", err)
	}
	return nil
}

// ============================================================
// Stages
// ============================================================

func (s *serviceImpl) ListStages(ctx context.Context, orgID, pipelineRef string) ([]*Stage, error) {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, pipelineRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListStages: %w", err)
	}
	if p == nil {
		return nil, ErrPipelineNotFound
	}
	list, err := s.repo.FindStages(ctx, orgID, p.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListStages: %w", err)
	}
	if list == nil {
		list = []*Stage{}
	}
	return list, nil
}

func (s *serviceImpl) CreateStage(ctx context.Context, orgID, pipelineRef string, req CreateStageRequest) (*Stage, error) {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, pipelineRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateStage: %w", err)
	}
	if p == nil {
		return nil, ErrPipelineNotFound
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrStageNameReq
	}
	kind := StageKindInProgress
	if req.StageKind != nil {
		kind = StageKind(strings.TrimSpace(*req.StageKind))
		if !kind.IsValid() {
			return nil, ErrInvalidStageKind
		}
	}
	position := 0
	if req.Position != nil {
		position = *req.Position
	}
	st := &Stage{OrgID: orgID, PipelineID: p.ID, Name: name, Position: position, StageKind: kind}
	if err := s.repo.CreateStage(ctx, st); err != nil {
		return nil, fmt.Errorf("recruitment: CreateStage: %w", err)
	}
	return st, nil
}

func (s *serviceImpl) UpdateStage(ctx context.Context, orgID, pipelineRef, stageRef string, req UpdateStageRequest) (*Stage, error) {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, pipelineRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateStage: %w", err)
	}
	if p == nil {
		return nil, ErrPipelineNotFound
	}
	st, err := s.repo.FindStageByRef(ctx, orgID, p.ID, stageRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateStage: %w", err)
	}
	if st == nil {
		return nil, ErrStageNotFound
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrStageNameReq
		}
		st.Name = name
	}
	if req.StageKind != nil {
		kind := StageKind(strings.TrimSpace(*req.StageKind))
		if !kind.IsValid() {
			return nil, ErrInvalidStageKind
		}
		st.StageKind = kind
	}
	if err := s.repo.UpdateStage(ctx, st); err != nil {
		return nil, fmt.Errorf("recruitment: UpdateStage: %w", err)
	}
	return st, nil
}

func (s *serviceImpl) DeleteStage(ctx context.Context, orgID, pipelineRef, stageRef string) error {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, pipelineRef)
	if err != nil {
		return fmt.Errorf("recruitment: DeleteStage: %w", err)
	}
	if p == nil {
		return ErrPipelineNotFound
	}
	st, err := s.repo.FindStageByRef(ctx, orgID, p.ID, stageRef)
	if err != nil {
		return fmt.Errorf("recruitment: DeleteStage: %w", err)
	}
	if st == nil {
		return ErrStageNotFound
	}
	if err := s.repo.DeleteStage(ctx, orgID, p.ID, st.ID); err != nil {
		return fmt.Errorf("recruitment: DeleteStage: %w", err)
	}
	return nil
}

func (s *serviceImpl) ReorderStages(ctx context.Context, orgID, pipelineRef string, req ReorderStagesRequest) error {
	p, err := s.repo.FindPipelineByRef(ctx, orgID, pipelineRef)
	if err != nil {
		return fmt.Errorf("recruitment: ReorderStages: %w", err)
	}
	if p == nil {
		return ErrPipelineNotFound
	}
	if err := s.repo.ReorderStages(ctx, orgID, p.ID, req.StageIDs); err != nil {
		return fmt.Errorf("recruitment: ReorderStages: %w", err)
	}
	return nil
}
