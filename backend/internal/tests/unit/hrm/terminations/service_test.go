// backend/internal/tests/unit/hrm/terminations/service_test.go
package terminations_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/terminations"
	"github.com/shopspring/decimal"
)

func ptrDecimal(v decimal.Decimal) *decimal.Decimal { return &v }

type stubRepo struct {
	byID map[string]*terminations.Termination
	seq  int
}

func newStubRepo() *stubRepo {
	return &stubRepo{byID: make(map[string]*terminations.Termination)}
}

func (r *stubRepo) nextID() string {
	r.seq++
	return fmt.Sprintf("term-%d", r.seq)
}

func (r *stubRepo) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*terminations.Termination, error) {
	var out []*terminations.Termination
	for _, t := range r.byID {
		if t.OrgID == orgID {
			if employeeID != "" && t.EmployeeID != employeeID {
				continue
			}
			if status != "" && string(t.Status) != status {
				continue
			}
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*terminations.Termination, error) {
	t, ok := r.byID[ref]
	if !ok || t.OrgID != orgID {
		return nil, nil
	}
	if employeeID != "" && t.EmployeeID != employeeID {
		return nil, nil
	}
	return t, nil
}

func (r *stubRepo) FindActiveByEmployee(ctx context.Context, orgID, employeeID string) (*terminations.Termination, error) {
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

func (r *stubRepo) Create(ctx context.Context, t *terminations.Termination) error {
	t.ID = r.nextID()
	t.PublicID = "pub_" + t.ID
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	r.byID[t.ID] = t
	return nil
}

func (r *stubRepo) Update(ctx context.Context, t *terminations.Termination) error {
	if _, ok := r.byID[t.ID]; !ok {
		return errors.New("not found")
	}
	t.UpdatedAt = time.Now()
	r.byID[t.ID] = t
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status terminations.TerminationStatus) error {
	t, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	t.Status = status
	return nil
}

func (r *stubRepo) SetApprovalInstance(ctx context.Context, id, instanceID string, status terminations.TerminationStatus) error {
	t, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	t.ApprovalInstanceID = &instanceID
	t.Status = status
	return nil
}

type stubApprovalsSvc struct {
	defaultTemplate  *approvals.ApprovalTemplate
}

func (s *stubApprovalsSvc) FindDefault(_ context.Context, _ string, _ approvals.ActionType) (*approvals.ApprovalTemplate, error) { return s.defaultTemplate, nil }
func (s *stubApprovalsSvc) CreateInstance(_ context.Context, orgID string, req approvals.CreateInstanceRequest) (*approvals.ApprovalInstance, error) { return nil, nil }
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
func ptrFloat(f float64) *float64 { return &f }

func TestTerminationsService(t *testing.T) {
	repo := newStubRepo()
	appSvc := &stubApprovalsSvc{}
	svc := terminations.NewService(repo, newDummyPool(), appSvc)
	ctx := context.Background()

	t.Run("Create Success", func(t *testing.T) {
		req := terminations.CreateTerminationRequest{
			TerminationType: terminations.TypeInvoluntary,
			TerminationDate: "2026-08-01",
			LastWorkingDate: "2026-08-15",
		}
		term, err := svc.Create(ctx, "org1", "emp1", "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if term.TerminationType != terminations.TypeInvoluntary {
			t.Errorf("expected involuntary, got %s", term.TerminationType)
		}
	})

	t.Run("Create Validation Error", func(t *testing.T) {
		req := terminations.CreateTerminationRequest{TerminationDate: ""}
		_, err := svc.Create(ctx, "org1", "emp2", "admin", req)
		if err != terminations.ErrInvalidTerminationType {
			t.Errorf("expected ErrInvalidTerminationType, got %v", err)
		}
	})

	t.Run("Get and List", func(t *testing.T) {
		req := terminations.CreateTerminationRequest{TerminationType: terminations.TypeVoluntary, TerminationDate: "2026-08-01", LastWorkingDate: "2026-08-15"}
		term, _ := svc.Create(ctx, "org1", "emp3", "admin", req)

		fetched, err := svc.Get(ctx, "org1", "emp3", term.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.ID != term.ID {
			t.Errorf("ID mismatch")
		}

		list, err := svc.List(ctx, "org1", "emp3", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if list.Total < 1 {
			t.Errorf("expected at least 1 termination in list")
		}
	})

	t.Run("Update", func(t *testing.T) {
		req := terminations.CreateTerminationRequest{TerminationType: terminations.TypeVoluntary, TerminationDate: "2026-08-01", LastWorkingDate: "2026-08-15"}
		term, _ := svc.Create(ctx, "org1", "emp4", "admin", req)

		updateReq := terminations.UpdateTerminationRequest{SeveranceAmount: ptrDecimal(decimal.NewFromInt(1000))}
		updated, err := svc.Update(ctx, "org1", "emp4", term.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.SeveranceAmount == nil || !updated.SeveranceAmount.Equal(decimal.NewFromInt(1000)) {
			t.Errorf("expected updated severance amount")
		}
	})
	
	t.Run("Cancel", func(t *testing.T) {
		req := terminations.CreateTerminationRequest{TerminationType: terminations.TypeVoluntary, TerminationDate: "2026-08-01", LastWorkingDate: "2026-08-15"}
		term, _ := svc.Create(ctx, "org1", "emp5", "admin", req)

		cancelled, err := svc.Cancel(ctx, "org1", "emp5", term.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cancelled.Status != terminations.StatusCancelled {
			t.Errorf("expected cancelled, got %s", cancelled.Status)
		}
	})
	
	t.Run("Cross-org isolation", func(t *testing.T) {
		req := terminations.CreateTerminationRequest{TerminationType: terminations.TypeVoluntary, TerminationDate: "2026-08-01", LastWorkingDate: "2026-08-15"}
		term, _ := svc.Create(ctx, "org1", "emp6", "admin", req)

		_, err := svc.Get(ctx, "org2", "emp6", term.ID)
		if err != terminations.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org access, got %v", err)
		}
	})
}
