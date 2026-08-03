// backend/internal/tests/unit/terminations/submit_test.go
package terminations

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/terminations"
)

// ── Stub termination repo ───────────────────────────────────────────────────

type stubTermRepo struct {
	byID map[string]*terminations.Termination
	seq  int
}

func newStubTermRepo() *stubTermRepo {
	return &stubTermRepo{byID: map[string]*terminations.Termination{}}
}

func (r *stubTermRepo) nextID() string {
	r.seq++
	return fmt.Sprintf("term-%d", r.seq)
}

func (r *stubTermRepo) FindAll(_ context.Context, orgID, employeeID, status string) ([]*terminations.Termination, error) {
	var out []*terminations.Termination
	for _, t := range r.byID {
		if t.OrgID == orgID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *stubTermRepo) FindByRef(_ context.Context, orgID, employeeID, ref string) (*terminations.Termination, error) {
	t, ok := r.byID[ref]
	if !ok || t.OrgID != orgID {
		return nil, nil
	}
	if employeeID != "" && t.EmployeeID != employeeID {
		return nil, nil
	}
	return t, nil
}

func (r *stubTermRepo) FindActiveByEmployee(_ context.Context, orgID, employeeID string) (*terminations.Termination, error) {
	for _, t := range r.byID {
		if t.OrgID == orgID && t.EmployeeID == employeeID {
			switch t.Status {
			case terminations.StatusDraft, terminations.StatusPendingApproval, terminations.StatusApproved:
				return t, nil
			}
		}
	}
	return nil, nil
}

func (r *stubTermRepo) Create(_ context.Context, t *terminations.Termination) error {
	t.ID = r.nextID()
	t.PublicID = "pub_" + t.ID
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	r.byID[t.ID] = t
	return nil
}

func (r *stubTermRepo) Update(_ context.Context, t *terminations.Termination) error {
	if _, ok := r.byID[t.ID]; !ok {
		return errors.New("termination not found")
	}
	t.UpdatedAt = time.Now()
	r.byID[t.ID] = t
	return nil
}

func (r *stubTermRepo) UpdateStatus(_ context.Context, id string, status terminations.TerminationStatus) error {
	t, ok := r.byID[id]
	if !ok {
		return errors.New("termination not found")
	}
	t.Status = status
	return nil
}

func (r *stubTermRepo) SetApprovalInstance(_ context.Context, id, instanceID string, status terminations.TerminationStatus) error {
	t, ok := r.byID[id]
	if !ok {
		return errors.New("termination not found")
	}
	t.ApprovalInstanceID = &instanceID
	t.Status = status
	return nil
}

// ── Stub approvals service ──────────────────────────────────────────────────
//
// Implements the full approvals.Service interface. Only FindDefault and
// CreateInstance are behaviorally interesting for Submit()/HandleApprovalDecision()
// tests — everything else is a trivial stub so the type satisfies the interface.

type stubApprovalsSvc struct {
	defaultTemplate  *approvals.ApprovalTemplate // nil = "no template configured for this org/action_type"
	createInstanceErr error
	createdInstances []approvals.CreateInstanceRequest
}

func (s *stubApprovalsSvc) FindDefault(_ context.Context, _ string, _ approvals.ActionType) (*approvals.ApprovalTemplate, error) {
	return s.defaultTemplate, nil
}

func (s *stubApprovalsSvc) CreateInstance(_ context.Context, orgID string, req approvals.CreateInstanceRequest) (*approvals.ApprovalInstance, error) {
	if s.createInstanceErr != nil {
		return nil, s.createInstanceErr
	}
	s.createdInstances = append(s.createdInstances, req)
	return &approvals.ApprovalInstance{
		ID: "inst-" + req.EntityID, OrgID: orgID, TemplateID: &req.TemplateID,
		EntityType: req.EntityType, EntityID: req.EntityID, RequestedBy: req.RequestedBy,
		CurrentLevel: 1, OverallStatus: approvals.InstanceStatusPending,
	}, nil
}

func (s *stubApprovalsSvc) ListTemplates(context.Context, string, string) (*approvals.TemplateListResponse, error) {
	return &approvals.TemplateListResponse{}, nil
}
func (s *stubApprovalsSvc) GetTemplate(context.Context, string, string) (*approvals.ApprovalTemplate, error) {
	return nil, approvals.ErrTemplateNotFound
}
func (s *stubApprovalsSvc) CreateTemplate(context.Context, string, string, approvals.CreateTemplateRequest) (*approvals.ApprovalTemplate, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) UpdateTemplate(context.Context, string, string, approvals.UpdateTemplateRequest) (*approvals.ApprovalTemplate, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) DeleteTemplate(context.Context, string, string) error { return nil }
func (s *stubApprovalsSvc) GetInstance(context.Context, string, string) (*approvals.ApprovalInstance, error) {
	return nil, approvals.ErrInstanceNotFound
}
func (s *stubApprovalsSvc) Decide(context.Context, string, string, string, approvals.DecisionRequest) (*approvals.ApprovalInstance, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) CancelInstance(context.Context, string, string, string) (*approvals.ApprovalInstance, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) RegisterCallback(string, approvals.EntityCallback) {}
func (s *stubApprovalsSvc) ListInstances(context.Context, string, int, int, string, string) (*approvals.InstanceListResponse, error) {
	return nil, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func newSvc(repo terminations.Repository, approvalsSvc approvals.Service) terminations.Service {
	return terminations.NewService(repo, nil, approvalsSvc) // db=nil: fine, Apply() (the only db user) isn't exercised here
}

func draftTermination(t *testing.T, svc terminations.Service) *terminations.Termination {
	t.Helper()
	created, err := svc.Create(context.Background(), "org-1", "emp-1", "hr-user-1", terminations.CreateTerminationRequest{
		TerminationType: terminations.TypeInvoluntary,
		TerminationDate: "2026-08-01",
		LastWorkingDate: "2026-08-15",
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	return created
}

// ── Submit: no template configured → unchanged auto-approve behavior ───────

func TestSubmit_NoTemplateConfigured_AutoApproves(t *testing.T) {
	repo := newStubTermRepo()
	approvalsSvc := &stubApprovalsSvc{defaultTemplate: nil} // no template for "termination"
	svc := newSvc(repo, approvalsSvc)

	created := draftTermination(t, svc)
	submitted, err := svc.Submit(context.Background(), "org-1", "emp-1", created.ID, "hr-user-1")
	if err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	if submitted.Status != terminations.StatusApproved {
		t.Errorf("expected status=approved (regression check — no template must auto-approve), got %q", submitted.Status)
	}
	if submitted.ApprovalInstanceID != nil {
		t.Errorf("expected no approval_instance_id when no template is configured, got %v", *submitted.ApprovalInstanceID)
	}
	if len(approvalsSvc.createdInstances) != 0 {
		t.Errorf("expected no approval instance to be created, got %d", len(approvalsSvc.createdInstances))
	}
}

// ── Submit: default template configured → routes to pending_approval ──────

func TestSubmit_DefaultTemplateConfigured_RoutesToPendingApproval(t *testing.T) {
	repo := newStubTermRepo()
	approvalsSvc := &stubApprovalsSvc{
		defaultTemplate: &approvals.ApprovalTemplate{ID: "tmpl-1", ActionType: approvals.ActionTypeTermination, IsDefault: true},
	}
	svc := newSvc(repo, approvalsSvc)

	created := draftTermination(t, svc)
	submitted, err := svc.Submit(context.Background(), "org-1", "emp-1", created.ID, "hr-user-1")
	if err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	if submitted.Status != terminations.StatusPendingApproval {
		t.Errorf("expected status=pending_approval, got %q", submitted.Status)
	}
	if submitted.ApprovalInstanceID == nil {
		t.Fatal("expected approval_instance_id to be set")
	}
	if len(approvalsSvc.createdInstances) != 1 {
		t.Fatalf("expected exactly 1 approval instance created, got %d", len(approvalsSvc.createdInstances))
	}
	req := approvalsSvc.createdInstances[0]
	if req.EntityType != "termination" {
		t.Errorf("expected entity_type=termination, got %q", req.EntityType)
	}
	if req.EntityID != created.ID {
		t.Errorf("expected entity_id=%s, got %s", created.ID, req.EntityID)
	}
	if req.RequestedBy != "hr-user-1" {
		t.Errorf("expected requested_by=hr-user-1, got %q", req.RequestedBy)
	}
}

func TestSubmit_WrongStatus_Rejected(t *testing.T) {
	repo := newStubTermRepo()
	svc := newSvc(repo, &stubApprovalsSvc{})
	created := draftTermination(t, svc)
	if _, err := svc.Submit(context.Background(), "org-1", "emp-1", created.ID, "hr-user-1"); err != nil {
		t.Fatalf("first Submit() error: %v", err)
	}
	if _, err := svc.Submit(context.Background(), "org-1", "emp-1", created.ID, "hr-user-1"); !errors.Is(err, terminations.ErrWrongStatus) {
		t.Fatalf("expected ErrWrongStatus on second Submit(), got %v", err)
	}
}

// ── HandleApprovalDecision ───────────────────────────────────────────────────

func TestHandleApprovalDecision_Approved_SetsApproved(t *testing.T) {
	repo := newStubTermRepo()
	approvalsSvc := &stubApprovalsSvc{
		defaultTemplate: &approvals.ApprovalTemplate{ID: "tmpl-1", ActionType: approvals.ActionTypeTermination},
	}
	svc := newSvc(repo, approvalsSvc)
	created := draftTermination(t, svc)
	submitted, _ := svc.Submit(context.Background(), "org-1", "emp-1", created.ID, "hr-user-1")

	if err := svc.HandleApprovalDecision(context.Background(), "org-1", submitted.ID, true); err != nil {
		t.Fatalf("HandleApprovalDecision() error: %v", err)
	}
	got, _ := svc.Get(context.Background(), "org-1", "emp-1", submitted.ID)
	if got.Status != terminations.StatusApproved {
		t.Errorf("expected status=approved after approval decision, got %q", got.Status)
	}
}

func TestHandleApprovalDecision_Rejected_SetsRejected(t *testing.T) {
	repo := newStubTermRepo()
	approvalsSvc := &stubApprovalsSvc{
		defaultTemplate: &approvals.ApprovalTemplate{ID: "tmpl-1", ActionType: approvals.ActionTypeTermination},
	}
	svc := newSvc(repo, approvalsSvc)
	created := draftTermination(t, svc)
	submitted, _ := svc.Submit(context.Background(), "org-1", "emp-1", created.ID, "hr-user-1")

	if err := svc.HandleApprovalDecision(context.Background(), "org-1", submitted.ID, false); err != nil {
		t.Fatalf("HandleApprovalDecision() error: %v", err)
	}
	got, _ := svc.Get(context.Background(), "org-1", "emp-1", submitted.ID)
	if got.Status != terminations.StatusRejected {
		t.Errorf("expected status=rejected after rejection decision, got %q", got.Status)
	}
}

func TestHandleApprovalDecision_NotPendingApproval_IsNoOp(t *testing.T) {
	// Guards against a late/duplicate callback (e.g. instance already cancelled
	// via a separate path) clobbering a status the record has already moved past.
	repo := newStubTermRepo()
	svc := newSvc(repo, &stubApprovalsSvc{})
	created := draftTermination(t, svc) // still in draft — never submitted

	if err := svc.HandleApprovalDecision(context.Background(), "org-1", created.ID, true); err != nil {
		t.Fatalf("HandleApprovalDecision() error: %v", err)
	}
	got, _ := svc.Get(context.Background(), "org-1", "emp-1", created.ID)
	if got.Status != terminations.StatusDraft {
		t.Errorf("expected status to remain draft (no-op), got %q", got.Status)
	}
}
