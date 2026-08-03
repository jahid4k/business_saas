// backend/internal/tests/unit/crm/pipeline_service_test.go
package crm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/crm/pipeline"
)

// ── Stub pipeline repo ────────────────────────────────────────────────────────

type stubPipelineRepo struct {
	pipelines map[string]*pipeline.Pipeline
	stages    map[string]*pipeline.Stage
	seq       int
}

func newStubPipelineRepo() *stubPipelineRepo {
	return &stubPipelineRepo{
		pipelines: map[string]*pipeline.Pipeline{},
		stages:    map[string]*pipeline.Stage{},
	}
}

func (r *stubPipelineRepo) nextID(prefix string) string {
	r.seq++
	n := r.seq
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return prefix + "_" + s
}

func (r *stubPipelineRepo) FindPipelines(_ context.Context, orgID string) ([]*pipeline.Pipeline, error) {
	var out []*pipeline.Pipeline
	for _, p := range r.pipelines {
		if p.OrgID == orgID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *stubPipelineRepo) CountPipelines(_ context.Context, orgID string) (int, error) {
	count := 0
	for _, p := range r.pipelines {
		if p.OrgID == orgID {
			count++
		}
	}
	return count, nil
}

func (r *stubPipelineRepo) FindPipelineByID(_ context.Context, orgID, id string) (*pipeline.Pipeline, error) {
	p, ok := r.pipelines[id]
	if !ok || p.OrgID != orgID {
		return nil, nil
	}
	return p, nil
}

func (r *stubPipelineRepo) CreatePipeline(_ context.Context, p *pipeline.Pipeline) error {
	p.ID = r.nextID("pipe")
	p.PublicID = "pub_" + p.ID
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	r.pipelines[p.ID] = p
	return nil
}

func (r *stubPipelineRepo) UpdatePipeline(_ context.Context, p *pipeline.Pipeline) error {
	if _, ok := r.pipelines[p.ID]; !ok {
		return errors.New("not found")
	}
	r.pipelines[p.ID] = p
	return nil
}

func (r *stubPipelineRepo) DeletePipeline(_ context.Context, orgID, id string) error {
	p, ok := r.pipelines[id]
	if !ok || p.OrgID != orgID {
		return pipeline.ErrPipelineNotFound
	}
	delete(r.pipelines, id)
	return nil
}

func (r *stubPipelineRepo) FindStagesByPipeline(_ context.Context, orgID, pipelineID string) ([]*pipeline.Stage, error) {
	var out []*pipeline.Stage
	for _, s := range r.stages {
		if s.OrgID == orgID && s.PipelineID == pipelineID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *stubPipelineRepo) FindStageByID(_ context.Context, orgID, id string) (*pipeline.Stage, error) {
	s, ok := r.stages[id]
	if !ok || s.OrgID != orgID {
		return nil, nil
	}
	return s, nil
}

// FindAllStagesByOrg satisfies the Repository interface.
func (r *stubPipelineRepo) FindAllStagesByOrg(_ context.Context, orgID string) ([]*pipeline.Stage, error) {
	var out []*pipeline.Stage
	for _, s := range r.stages {
		if s.OrgID == orgID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *stubPipelineRepo) CreateStage(_ context.Context, s *pipeline.Stage) error {
	s.ID = r.nextID("stage")
	s.PublicID = "pub_" + s.ID
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	r.stages[s.ID] = s
	return nil
}

func (r *stubPipelineRepo) UpdateStage(_ context.Context, s *pipeline.Stage) error {
	r.stages[s.ID] = s
	return nil
}

func (r *stubPipelineRepo) DeleteStage(_ context.Context, orgID, id string) error {
	s, ok := r.stages[id]
	if !ok || s.OrgID != orgID {
		return pipeline.ErrStageNotFound
	}
	delete(r.stages, id)
	return nil
}

func (r *stubPipelineRepo) ReorderStages(_ context.Context, orgID, pipelineID string, stageIDs []string) error {
	for i, id := range stageIDs {
		if s, ok := r.stages[id]; ok && s.OrgID == orgID && s.PipelineID == pipelineID {
			s.Position = i
		}
	}
	return nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func newPipelineSvc(repo pipeline.Repository) pipeline.Service {
	return pipeline.NewService(repo)
}

// ── Pipeline CRUD ─────────────────────────────────────────────────────────────

func TestCreatePipeline_Success(t *testing.T) {
	svc := newPipelineSvc(newStubPipelineRepo())
	p, err := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{
		Name: "Sales Pipeline",
	})
	if err != nil {
		t.Fatalf("CreatePipeline() error: %v", err)
	}
	if p.Name != "Sales Pipeline" {
		t.Errorf("expected name 'Sales Pipeline', got %q", p.Name)
	}
	if p.OrgID != "org-1" {
		t.Errorf("expected OrgID=org-1, got %q", p.OrgID)
	}
}

func TestCreatePipeline_NameRequired(t *testing.T) {
	svc := newPipelineSvc(newStubPipelineRepo())
	_, err := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{})
	if !errors.Is(err, pipeline.ErrNameRequired) {
		t.Fatalf("expected ErrNameRequired, got %v", err)
	}
}

func TestListPipelines_EmptySliceNotNil(t *testing.T) {
	svc := newPipelineSvc(newStubPipelineRepo())
	resp, err := svc.ListPipelines(context.Background(), "org-empty")
	if err != nil {
		t.Fatalf("ListPipelines() error: %v", err)
	}
	if resp.Pipelines == nil {
		t.Error("expected non-nil empty slice")
	}
}

func TestGetPipeline_NotFound(t *testing.T) {
	svc := newPipelineSvc(newStubPipelineRepo())
	_, err := svc.GetPipeline(context.Background(), "org-1", "nonexistent")
	if !errors.Is(err, pipeline.ErrPipelineNotFound) {
		t.Fatalf("expected ErrPipelineNotFound, got %v", err)
	}
}

func TestGetPipeline_CrossOrgReturnsNotFound(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	_, err := svc.GetPipeline(context.Background(), "org-2", p.ID)
	if !errors.Is(err, pipeline.ErrPipelineNotFound) {
		t.Fatalf("SECURITY: cross-org GetPipeline must return ErrPipelineNotFound, got %v", err)
	}
}

func TestDeletePipeline_CrossOrgReturnsError(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})

	// Delete from wrong org — repo returns ErrPipelineNotFound
	err := svc.DeletePipeline(context.Background(), "org-2", p.ID)
	if !errors.Is(err, pipeline.ErrPipelineNotFound) {
		t.Fatalf("expected ErrPipelineNotFound on cross-org delete, got %v", err)
	}

	// Must still exist in org-1
	got, err := svc.GetPipeline(context.Background(), "org-1", p.ID)
	if err != nil {
		t.Fatalf("GetPipeline after cross-org delete attempt failed: %v", err)
	}
	if got == nil {
		t.Error("SECURITY: pipeline was deleted from wrong org")
	}
}

// ── Stage CRUD ────────────────────────────────────────────────────────────────

func TestCreateStage_Success(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	stage, err := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{
		Name: "Prospecting",
	})
	if err != nil {
		t.Fatalf("CreateStage() error: %v", err)
	}
	if stage.Name != "Prospecting" {
		t.Errorf("expected name 'Prospecting', got %q", stage.Name)
	}
}

func TestCreateStage_NameRequired(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	_, err := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{})
	if !errors.Is(err, pipeline.ErrNameRequired) {
		t.Fatalf("expected ErrNameRequired, got %v", err)
	}
}

func TestGetStage_CrossOrgReturnsNotFound(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	stage, _ := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "Stage"})

	_, err := svc.GetStage(context.Background(), "org-2", stage.ID)
	if !errors.Is(err, pipeline.ErrStageNotFound) {
		t.Fatalf("SECURITY: cross-org GetStage must return ErrStageNotFound, got %v", err)
	}
}

func TestListStages_CrossOrgPipelineReturnsNotFound(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})

	_, err := svc.ListStages(context.Background(), "org-2", p.ID)
	if !errors.Is(err, pipeline.ErrPipelineNotFound) {
		t.Fatalf("SECURITY: cross-org ListStages must return ErrPipelineNotFound, got %v", err)
	}
}

func TestDeleteStage_WrongPipelineReturnsError(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	stage, _ := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "Stage"})

	// Wrong pipeline ID — service returns ErrStageNotInPipeline
	err := svc.DeleteStage(context.Background(), "org-1", "wrong-pipeline-id", stage.ID)
	if !errors.Is(err, pipeline.ErrStageNotInPipeline) {
		t.Fatalf("expected ErrStageNotInPipeline for wrong pipeline, got %v", err)
	}

	// Stage must still exist
	got, err := svc.GetStage(context.Background(), "org-1", stage.ID)
	if err != nil || got == nil {
		t.Errorf("stage should still exist after wrong-pipeline delete attempt: err=%v, got=%v", err, got)
	}
}

func TestReorderStages_Success(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	s1, _ := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "Stage 1"})
	s2, _ := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "Stage 2"})

	err := svc.ReorderStages(context.Background(), "org-1", p.ID, pipeline.ReorderStagesRequest{
		StageIDs: []string{s2.ID, s1.ID},
	})
	if err != nil {
		t.Fatalf("ReorderStages() error: %v", err)
	}
}

func TestUpdatePipeline_Success(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Old Name"})

	newName := "New Name"
	updated, err := svc.UpdatePipeline(context.Background(), "org-1", p.ID, pipeline.UpdatePipelineRequest{Name: &newName})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected Name=%q, got %q", newName, updated.Name)
	}
}

func TestUpdatePipeline_CrossOrg(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Old Name"})

	newName := "New Name"
	_, err := svc.UpdatePipeline(context.Background(), "org-2", p.ID, pipeline.UpdatePipelineRequest{Name: &newName})
	if !errors.Is(err, pipeline.ErrPipelineNotFound) {
		t.Fatalf("SECURITY: expected ErrPipelineNotFound on cross-org update, got %v", err)
	}
}

func TestDeletePipeline_Success(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})

	err := svc.DeletePipeline(context.Background(), "org-1", p.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, _ := svc.GetPipeline(context.Background(), "org-1", p.ID)
	if got != nil {
		t.Fatal("expected pipeline to be deleted")
	}
}

func TestListStages_Success(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "S1"})
	svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "S2"})

	resp, err := svc.ListStages(context.Background(), "org-1", p.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Stages) != 5 {
		t.Errorf("expected 5 stages, got %d", len(resp.Stages))
	}
}

func TestListStages_PipelineNotFound(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	_, err := svc.ListStages(context.Background(), "org-1", "nonexistent")
	if !errors.Is(err, pipeline.ErrPipelineNotFound) {
		t.Fatalf("expected ErrPipelineNotFound, got %v", err)
	}
}

func TestUpdateStage_Success(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	s, _ := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "Old Name"})

	newName := "New Name"
	updated, err := svc.UpdateStage(context.Background(), "org-1", p.ID, s.ID, pipeline.UpdateStageRequest{Name: &newName})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected Name=%q, got %q", newName, updated.Name)
	}
}

func TestDeleteStage_Success(t *testing.T) {
	repo := newStubPipelineRepo()
	svc := newPipelineSvc(repo)
	p, _ := svc.CreatePipeline(context.Background(), "org-1", "user-1", pipeline.CreatePipelineRequest{Name: "Pipeline"})
	s, _ := svc.CreateStage(context.Background(), "org-1", p.ID, pipeline.CreateStageRequest{Name: "Stage"})

	err := svc.DeleteStage(context.Background(), "org-1", p.ID, s.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, _ := svc.GetStage(context.Background(), "org-1", s.ID)
	if got != nil {
		t.Fatal("expected stage to be deleted")
	}
}

