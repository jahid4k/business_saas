// backend/internal/tests/unit/crm/deals_service_test.go
package crm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mridha/businesssaas/internal/crm/deals"
	"github.com/mridha/businesssaas/internal/crm/leads"
	"github.com/mridha/businesssaas/internal/crm/pipeline"
)

// ── Stub Deals Repository ───────────────────────────────────────────────────

type stubDealRepo struct {
	deals map[string]*deals.Deal
	seq   int
}

func newStubDealRepo() *stubDealRepo {
	return &stubDealRepo{deals: map[string]*deals.Deal{}}
}

func (r *stubDealRepo) nextID() string {
	r.seq++
	n := r.seq
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return "deal_" + s
}

func (r *stubDealRepo) FindDeals(_ context.Context, orgID string) ([]*deals.Deal, error) {
	var out []*deals.Deal
	for _, d := range r.deals {
		if d.OrgID == orgID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *stubDealRepo) FindDealByID(_ context.Context, orgID, dealID string) (*deals.Deal, error) {
	d, ok := r.deals[dealID]
	if !ok || d.OrgID != orgID {
		return nil, nil
	}
	return d, nil
}

func (r *stubDealRepo) FindDealsByStage(_ context.Context, orgID, stageID string) ([]*deals.Deal, error) {
	var out []*deals.Deal
	for _, d := range r.deals {
		if d.OrgID == orgID && d.StageID == stageID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *stubDealRepo) FindDealsByContact(_ context.Context, orgID, contactID string) ([]*deals.Deal, error) {
	var out []*deals.Deal
	for _, d := range r.deals {
		if d.OrgID == orgID && d.ContactID != nil && *d.ContactID == contactID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *stubDealRepo) FindDealsByCompany(_ context.Context, orgID, companyID string) ([]*deals.Deal, error) {
	var out []*deals.Deal
	for _, d := range r.deals {
		if d.OrgID == orgID && d.CompanyID != nil && *d.CompanyID == companyID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *stubDealRepo) FindRecentDeals(_ context.Context, orgID string, limit int) ([]*deals.Deal, error) {
	var out []*deals.Deal
	for _, d := range r.deals {
		if d.OrgID == orgID {
			out = append(out, d)
		}
	}
	if len(out) > limit {
		return out[:limit], nil
	}
	return out, nil
}

func (r *stubDealRepo) CreateDeal(_ context.Context, d *deals.Deal) error {
	d.ID = r.nextID()
	d.PublicID = "pub_" + d.ID
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	r.deals[d.ID] = d
	return nil
}

func (r *stubDealRepo) CreateDealTx(ctx context.Context, tx pgx.Tx, d *deals.Deal) error {
	return r.CreateDeal(ctx, d)
}

func (r *stubDealRepo) UpdateDeal(_ context.Context, d *deals.Deal) error {
	if _, ok := r.deals[d.ID]; !ok {
		return deals.ErrDealNotFound
	}
	d.UpdatedAt = time.Now()
	r.deals[d.ID] = d
	return nil
}

func (r *stubDealRepo) SoftDeleteDeal(_ context.Context, orgID, dealID string) error {
	d, ok := r.deals[dealID]
	if !ok || d.OrgID != orgID {
		return deals.ErrDealNotFound
	}
	delete(r.deals, dealID)
	return nil
}

func (r *stubDealRepo) CountDeals(_ context.Context, orgID string) (int, error) {
	count := 0
	for _, d := range r.deals {
		if d.OrgID == orgID {
			count++
		}
	}
	return count, nil
}

func (r *stubDealRepo) GetDealsByStage(_ context.Context, orgID string) ([]*deals.DealsByStage, error) {
	return []*deals.DealsByStage{}, nil
}

func (r *stubDealRepo) GetDealsByOwner(_ context.Context, orgID string) ([]*deals.DealsByOwner, error) {
	return []*deals.DealsByOwner{}, nil
}

// ── Stub Pipeline Service ───────────────────────────────────────────────────

type stubPipelineSvc struct {
	pipeline.Service
	pipelines map[string]*pipeline.Pipeline
	stages    map[string]*pipeline.Stage
}

func (s *stubPipelineSvc) GetStage(_ context.Context, orgID, stageID string) (*pipeline.Stage, error) {
	stage, ok := s.stages[stageID]
	if !ok || stage.OrgID != orgID {
		return nil, pipeline.ErrStageNotFound
	}
	return stage, nil
}

func (s *stubPipelineSvc) GetPipeline(_ context.Context, orgID, pipelineID string) (*pipeline.Pipeline, error) {
	p, ok := s.pipelines[pipelineID]
	if !ok || p.OrgID != orgID {
		return nil, pipeline.ErrPipelineNotFound
	}
	return p, nil
}

// ── Tests ───────────────────────────────────────────────────────────────────

func TestCreateDeal_Success(t *testing.T) {
	repo := newStubDealRepo()
	pipeSvc := &stubPipelineSvc{}
	svc := deals.NewService(repo, pipeSvc)

	req := deals.CreateDealRequest{
		Title:      "New Deal",
		PipelineID: "pipe_1",
		StageID:    "stage_1",
		Value:      1000.50,
	}

	d, err := svc.CreateDeal(context.Background(), "org-1", "user-1", req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if d.Title != "New Deal" {
		t.Errorf("expected Title='New Deal', got %q", d.Title)
	}
	if d.Status != deals.DealStatusOpen {
		t.Errorf("expected default Status=open, got %q", d.Status)
	}
	if d.Currency != "USD" {
		t.Errorf("expected default Currency='USD', got %q", d.Currency)
	}
}

func TestCreateDeal_MissingFields(t *testing.T) {
	svc := deals.NewService(newStubDealRepo(), &stubPipelineSvc{})

	_, err := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{Title: ""})
	if !errors.Is(err, deals.ErrTitleRequired) {
		t.Errorf("expected ErrTitleRequired, got %v", err)
	}

	_, err = svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{Title: "Title", PipelineID: ""})
	if !errors.Is(err, deals.ErrPipelineRequired) {
		t.Errorf("expected ErrPipelineRequired, got %v", err)
	}

	_, err = svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{Title: "Title", PipelineID: "p1", StageID: ""})
	if !errors.Is(err, deals.ErrStageRequired) {
		t.Errorf("expected ErrStageRequired, got %v", err)
	}
}

func TestGetDeal_CrossOrgReturnsNotFound(t *testing.T) {
	repo := newStubDealRepo()
	svc := deals.NewService(repo, &stubPipelineSvc{})
	
	d, _ := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{
		Title: "Deal 1", PipelineID: "p1", StageID: "s1",
	})

	_, err := svc.GetDeal(context.Background(), "org-2", d.ID)
	if !errors.Is(err, deals.ErrDealNotFound) {
		t.Errorf("SECURITY: expected ErrDealNotFound for cross-org access, got %v", err)
	}
}

func TestUpdateDeal_Success(t *testing.T) {
	repo := newStubDealRepo()
	svc := deals.NewService(repo, &stubPipelineSvc{})
	
	d, _ := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{
		Title: "Old Title", PipelineID: "p1", StageID: "s1",
	})

	newTitle := "New Title"
	newVal := 5000.0
	updated, err := svc.UpdateDeal(context.Background(), "org-1", d.ID, deals.UpdateDealRequest{
		Title: &newTitle,
		Value: &newVal,
	})
	if err != nil {
		t.Fatalf("expected no error updating deal, got %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("expected Title=%q, got %q", newTitle, updated.Title)
	}
	if updated.Value != newVal {
		t.Errorf("expected Value=%f, got %f", newVal, updated.Value)
	}
}

func TestMoveDealStage_Success(t *testing.T) {
	repo := newStubDealRepo()
	pipeSvc := &stubPipelineSvc{
		stages: map[string]*pipeline.Stage{
			"s1": {ID: "s1", OrgID: "org-1", PipelineID: "p1"},
			"s2": {ID: "s2", OrgID: "org-1", PipelineID: "p1"},
		},
	}
	svc := deals.NewService(repo, pipeSvc)

	d, _ := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{
		Title: "Deal", PipelineID: "p1", StageID: "s1",
	})

	moved, err := svc.MoveDealStage(context.Background(), "org-1", d.ID, deals.MoveDealStageRequest{StageID: "s2"})
	if err != nil {
		t.Fatalf("expected no error moving deal, got %v", err)
	}
	if moved.StageID != "s2" {
		t.Errorf("expected StageID=s2, got %q", moved.StageID)
	}
}

func TestMoveDealStage_WrongPipeline(t *testing.T) {
	repo := newStubDealRepo()
	pipeSvc := &stubPipelineSvc{
		stages: map[string]*pipeline.Stage{
			"s1": {ID: "s1", OrgID: "org-1", PipelineID: "p1"},
			"s2": {ID: "s2", OrgID: "org-1", PipelineID: "p2"}, // Different pipeline
		},
	}
	svc := deals.NewService(repo, pipeSvc)

	d, _ := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{
		Title: "Deal", PipelineID: "p1", StageID: "s1",
	})

	_, err := svc.MoveDealStage(context.Background(), "org-1", d.ID, deals.MoveDealStageRequest{StageID: "s2"})
	if !errors.Is(err, deals.ErrStageNotInPipeline) {
		t.Errorf("expected ErrStageNotInPipeline, got %v", err)
	}
}

func TestMarkDealWon(t *testing.T) {
	repo := newStubDealRepo()
	svc := deals.NewService(repo, &stubPipelineSvc{})
	
	d, _ := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{
		Title: "Deal", PipelineID: "p1", StageID: "s1",
	})

	won, err := svc.MarkDealWon(context.Background(), "org-1", d.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if won.Status != deals.DealStatusWon {
		t.Errorf("expected Status=won, got %q", won.Status)
	}
	if won.WonAt == nil {
		t.Error("expected WonAt to be set")
	}
}

func TestMarkDealLost(t *testing.T) {
	repo := newStubDealRepo()
	svc := deals.NewService(repo, &stubPipelineSvc{})
	
	d, _ := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{
		Title: "Deal", PipelineID: "p1", StageID: "s1",
	})

	reason := "Price too high"
	lost, err := svc.MarkDealLost(context.Background(), "org-1", d.ID, deals.MarkLostRequest{LostReason: &reason})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if lost.Status != deals.DealStatusLost {
		t.Errorf("expected Status=lost, got %q", lost.Status)
	}
	if lost.LostAt == nil {
		t.Error("expected LostAt to be set")
	}
	if lost.LostReason == nil || *lost.LostReason != reason {
		t.Errorf("expected LostReason=%q, got %v", reason, lost.LostReason)
	}
}

func TestDeleteDeal_CrossOrgIsError(t *testing.T) {
	repo := newStubDealRepo()
	svc := deals.NewService(repo, &stubPipelineSvc{})
	
	d, _ := svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{
		Title: "Deal", PipelineID: "p1", StageID: "s1",
	})

	err := svc.DeleteDeal(context.Background(), "org-2", d.ID)
	if !errors.Is(err, deals.ErrDealNotFound) {
		t.Errorf("SECURITY: cross-org delete should return ErrDealNotFound, got %v", err)
	}
}

func TestListDeals(t *testing.T) {
	repo := newStubDealRepo()
	svc := deals.NewService(repo, &stubPipelineSvc{})
	
	svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{Title: "D1", PipelineID: "p1", StageID: "s1"})
	svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{Title: "D2", PipelineID: "p1", StageID: "s1"})
	svc.CreateDeal(context.Background(), "org-2", "user-2", deals.CreateDealRequest{Title: "D3", PipelineID: "p1", StageID: "s1"})

	resp, err := svc.ListDeals(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 deals for org-1, got %d", resp.Total)
	}
	for _, d := range resp.Deals {
		if d.OrgID != "org-1" {
			t.Errorf("SECURITY: got deal from org %s", d.OrgID)
		}
	}
}

func TestCreateDealFromLeadTx(t *testing.T) {
	repo := newStubDealRepo()
	svc := deals.NewService(repo, &stubPipelineSvc{})

	lead := &leads.Lead{
		ID:        "lead-1",
		OrgID:     "org-1",
		FirstName: "Lead Name",
	}

	pID := "p1"
	sID := "s1"
	title := "Custom Deal Title"
	val := 999.99

	req := leads.ConvertLeadRequest{
		PipelineID: &pID,
		StageID:    &sID,
		DealTitle:  &title,
		DealValue:  &val,
	}

	dealID, err := svc.CreateDealFromLeadTx(context.Background(), nil, "org-1", "user-1", lead, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	d, _ := svc.GetDeal(context.Background(), "org-1", dealID)
	if d.Title != title {
		t.Errorf("expected Title=%q, got %q", title, d.Title)
	}
	if d.Value != val {
		t.Errorf("expected Value=%f, got %f", val, d.Value)
	}
}

func TestGetPipelineBoard(t *testing.T) {
	repo := newStubDealRepo()
	pipeSvc := &stubPipelineSvc{
		pipelines: map[string]*pipeline.Pipeline{
			"p1": {
				ID: "p1", OrgID: "org-1", Name: "Pipe 1",
				Stages: []*pipeline.Stage{
					{ID: "s1", Name: "Stage 1", Position: 0},
					{ID: "s2", Name: "Stage 2", Position: 1},
				},
			},
		},
	}
	svc := deals.NewService(repo, pipeSvc)

	svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{Title: "D1", PipelineID: "p1", StageID: "s1"})
	svc.CreateDeal(context.Background(), "org-1", "user-1", deals.CreateDealRequest{Title: "D2", PipelineID: "p1", StageID: "s2"})

	board, err := svc.GetPipelineBoard(context.Background(), "org-1", "p1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if board.PipelineID != "p1" {
		t.Errorf("expected PipelineID=p1, got %q", board.PipelineID)
	}
	if len(board.Stages) != 2 {
		t.Errorf("expected 2 stages in board, got %d", len(board.Stages))
	}
	
	// Stage 1
	if board.Stages[0].Total != 1 || len(board.Stages[0].Deals) != 1 {
		t.Errorf("expected 1 deal in stage 1, got %d", board.Stages[0].Total)
	}
	// Stage 2
	if board.Stages[1].Total != 1 || len(board.Stages[1].Deals) != 1 {
		t.Errorf("expected 1 deal in stage 2, got %d", board.Stages[1].Total)
	}
}
