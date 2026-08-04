// backend/internal/tests/unit/hrm/warnings/service_test.go
package warnings_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/warnings"
)

type stubRepo struct {
	byID map[string]*warnings.EmployeeWarning
	seq  int
}

func newStubRepo() *stubRepo {
	return &stubRepo{byID: make(map[string]*warnings.EmployeeWarning)}
}

func (r *stubRepo) nextID() string {
	r.seq++
	return fmt.Sprintf("warn-%d", r.seq)
}

func (r *stubRepo) FindAll(ctx context.Context, orgID string, filter warnings.WarningListFilter) ([]*warnings.EmployeeWarning, error) {
	var out []*warnings.EmployeeWarning
	for _, w := range r.byID {
		if w.OrgID == orgID {
			if filter.EmployeeID != "" && w.EmployeeID != filter.EmployeeID {
				continue
			}
			if filter.Status != "" && string(w.Status) != filter.Status {
				continue
			}
			if filter.ActiveOnly && !w.IsActive {
				continue
			}
			out = append(out, w)
		}
	}
	return out, nil
}

func (r *stubRepo) Count(ctx context.Context, orgID string, filter warnings.WarningListFilter) (int, error) {
	out, err := r.FindAll(ctx, orgID, filter)
	return len(out), err
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*warnings.EmployeeWarning, error) {
	w, ok := r.byID[ref]
	if !ok || w.OrgID != orgID {
		return nil, nil
	}
	if employeeID != "" && w.EmployeeID != employeeID {
		return nil, nil
	}
	return w, nil
}

func (r *stubRepo) Create(ctx context.Context, w *warnings.EmployeeWarning) error {
	w.ID = r.nextID()
	w.PublicID = "pub_" + w.ID
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	r.byID[w.ID] = w
	return nil
}

func (r *stubRepo) Update(ctx context.Context, w *warnings.EmployeeWarning) error {
	if _, ok := r.byID[w.ID]; !ok {
		return errors.New("not found")
	}
	w.UpdatedAt = time.Now()
	r.byID[w.ID] = w
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status warnings.WarningStatus) error {
	w, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	w.Status = status
	return nil
}

func (r *stubRepo) SetApprovalInstance(ctx context.Context, id, instanceID string, status warnings.WarningStatus) error {
	w, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	w.ApprovalInstanceID = &instanceID
	w.Status = status
	return nil
}

func (r *stubRepo) CountActiveByTypeAndEmployee(ctx context.Context, orgID, employeeID, warningTypeID string, withinDays int) (int, error) {
	return 0, nil
}

type stubApprovalsSvc struct {
	defaultTemplate  *approvals.ApprovalTemplate
}

func (s *stubApprovalsSvc) FindDefault(_ context.Context, _ string, _ approvals.ActionType) (*approvals.ApprovalTemplate, error) {
	return s.defaultTemplate, nil
}
func (s *stubApprovalsSvc) CreateInstance(_ context.Context, orgID string, req approvals.CreateInstanceRequest) (*approvals.ApprovalInstance, error) {
	return &approvals.ApprovalInstance{
		ID: "inst-" + req.EntityID, OrgID: orgID, TemplateID: &req.TemplateID,
		EntityType: req.EntityType, EntityID: req.EntityID, RequestedBy: req.RequestedBy,
		CurrentLevel: 1, OverallStatus: approvals.InstanceStatusPending,
	}, nil
}
func (s *stubApprovalsSvc) ListTemplates(context.Context, string, string) (*approvals.TemplateListResponse, error) { return nil, nil }
func (s *stubApprovalsSvc) GetTemplate(context.Context, string, string) (*approvals.ApprovalTemplate, error) { return nil, nil }
func (s *stubApprovalsSvc) CreateTemplate(context.Context, string, string, approvals.CreateTemplateRequest) (*approvals.ApprovalTemplate, error) { return nil, nil }
func (s *stubApprovalsSvc) UpdateTemplate(context.Context, string, string, approvals.UpdateTemplateRequest) (*approvals.ApprovalTemplate, error) { return nil, nil }
func (s *stubApprovalsSvc) DeleteTemplate(context.Context, string, string) error { return nil }
func (s *stubApprovalsSvc) GetInstance(context.Context, string, string) (*approvals.ApprovalInstance, error) { return nil, nil }
func (s *stubApprovalsSvc) Decide(context.Context, string, string, string, approvals.DecisionRequest) (*approvals.ApprovalInstance, error) { return nil, nil }
func (s *stubApprovalsSvc) CancelInstance(context.Context, string, string, string) (*approvals.ApprovalInstance, error) { return nil, nil }
func (s *stubApprovalsSvc) RegisterCallback(string, approvals.EntityCallback) {}
func (s *stubApprovalsSvc) ListInstances(context.Context, string, int, int, string, string) (*approvals.InstanceListResponse, error) {
	return nil, nil
}

func newDummyPool() *pgxpool.Pool {
	cfg, _ := pgxpool.ParseConfig("postgres://dummy:dummy@127.0.0.1:5432/dummy?sslmode=disable")
	pool, _ := pgxpool.NewWithConfig(context.Background(), cfg)
	return pool
}

func ptrStr(s string) *string { return &s }

func TestWarningsService(t *testing.T) {
	repo := newStubRepo()
	appSvc := &stubApprovalsSvc{}
	svc := warnings.NewService(repo, newDummyPool(), appSvc)
	ctx := context.Background()

	t.Run("Create expects ErrWarningTypeNotFound without DB", func(t *testing.T) {
		req := warnings.CreateWarningRequest{
			WarningTypeID: "type1",
			Title:         "title",
			Description:   "desc",
			IncidentDate:  "2026-07-01",
		}
		_, err := svc.Create(ctx, "org1", "emp1", "admin", req)
		if err != warnings.ErrWarningTypeNotFound {
			t.Errorf("expected ErrWarningTypeNotFound due to dummy db, got %v", err)
		}
	})

	// Inject a draft warning to test other methods
	w := &warnings.EmployeeWarning{
		OrgID: "org1", EmployeeID: "emp1",
		WarningTypeID: "type1", Title: "title", Description: "desc", IncidentDate: "2026-07-01",
		CanEmployeeRespond: true, ResponseWindowDays: 7,
		Status: warnings.StatusDraft, CreatedBy: "admin",
	}
	repo.Create(ctx, w)

	t.Run("Get and List", func(t *testing.T) {
		fetched, err := svc.Get(ctx, "org1", "emp1", w.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.ID != w.ID {
			t.Errorf("ID mismatch")
		}

		list, err := svc.List(ctx, "org1", warnings.WarningListFilter{EmployeeID: "emp1", Scope: authz.ScopeAll})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if list.Total < 1 {
			t.Errorf("expected at least 1 warning in list")
		}
	})

	t.Run("Update", func(t *testing.T) {
		updateReq := warnings.UpdateWarningRequest{Title: ptrStr("updated title")}
		updated, err := svc.Update(ctx, "org1", "emp1", w.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Title != "updated title" {
			t.Errorf("expected updated title, got %s", updated.Title)
		}
	})

	t.Run("Issue", func(t *testing.T) {
		issued, err := svc.Issue(ctx, "org1", "emp1", w.ID, "admin", warnings.IssueRequest{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if issued.Status != warnings.StatusIssued {
			t.Errorf("expected issued status, got %s", issued.Status)
		}
	})

	t.Run("Acknowledge", func(t *testing.T) {
		ackReq := warnings.AcknowledgeRequest{Response: ptrStr("my response")}
		acked, err := svc.Acknowledge(ctx, "org1", "emp1", w.ID, ackReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if acked.Status != warnings.StatusAcknowledged {
			t.Errorf("expected acknowledged, got %s", acked.Status)
		}
	})

	t.Run("Appeal", func(t *testing.T) {
		appealReq := warnings.AppealRequest{Reason: "unfair"}
		appealed, err := svc.Appeal(ctx, "org1", "emp1", w.ID, appealReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if appealed.Status != warnings.StatusAppealed {
			t.Errorf("expected appealed, got %s", appealed.Status)
		}
	})
	
	t.Run("Close", func(t *testing.T) {
		closeReq := warnings.CloseRequest{AppealResolution: ptrStr("upheld")}
		closed, err := svc.Close(ctx, "org1", "emp1", w.ID, closeReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if closed.Status != warnings.StatusClosed {
			t.Errorf("expected closed, got %s", closed.Status)
		}
	})

	// Inject another for Cancel
	w2 := &warnings.EmployeeWarning{
		OrgID: "org1", EmployeeID: "emp1",
		WarningTypeID: "type1", Status: warnings.StatusDraft,
	}
	repo.Create(ctx, w2)
	t.Run("Cancel", func(t *testing.T) {
		cancelled, err := svc.Cancel(ctx, "org1", "emp1", w2.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cancelled.Status != warnings.StatusCancelled {
			t.Errorf("expected cancelled, got %s", cancelled.Status)
		}
	})
	
	t.Run("Cross-org isolation", func(t *testing.T) {
		_, err := svc.Get(ctx, "org2", "emp1", w.ID)
		if err != warnings.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org access, got %v", err)
		}
	})
}
