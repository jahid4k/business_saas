// backend/internal/tests/unit/hrm/transfers/service_test.go
package transfers_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/transfers"
)

type stubRepo struct {
	byID map[string]*transfers.Transfer
	seq  int
}

func newStubRepo() *stubRepo {
	return &stubRepo{byID: make(map[string]*transfers.Transfer)}
}

func (r *stubRepo) nextID() string {
	r.seq++
	return fmt.Sprintf("trf-%d", r.seq)
}

func (r *stubRepo) FindAll(ctx context.Context, orgID string, filter transfers.TransferListFilter) ([]*transfers.Transfer, error) {
	var out []*transfers.Transfer
	for _, t := range r.byID {
		if t.OrgID == orgID {
			if filter.EmployeeID != "" && t.EmployeeID != filter.EmployeeID {
				continue
			}
			if filter.Status != "" && string(t.Status) != filter.Status {
				continue
			}
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *stubRepo) Count(ctx context.Context, orgID string, filter transfers.TransferListFilter) (int, error) {
	out, err := r.FindAll(ctx, orgID, filter)
	return len(out), err
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*transfers.Transfer, error) {
	t, ok := r.byID[ref]
	if !ok || t.OrgID != orgID {
		return nil, nil
	}
	if employeeID != "" && t.EmployeeID != employeeID {
		return nil, nil
	}
	return t, nil
}

func (r *stubRepo) Create(ctx context.Context, t *transfers.Transfer) error {
	t.ID = r.nextID()
	t.PublicID = "pub_" + t.ID
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	r.byID[t.ID] = t
	return nil
}

func (r *stubRepo) Update(ctx context.Context, t *transfers.Transfer) error {
	if _, ok := r.byID[t.ID]; !ok {
		return errors.New("not found")
	}
	t.UpdatedAt = time.Now()
	r.byID[t.ID] = t
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status transfers.TransferStatus) error {
	t, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	t.Status = status
	return nil
}

func (r *stubRepo) SetApprovalInstance(ctx context.Context, id, instanceID string, status transfers.TransferStatus) error {
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

func TestTransfersService(t *testing.T) {
	repo := newStubRepo()
	appSvc := &stubApprovalsSvc{}
	svc := transfers.NewService(repo, newDummyPool(), appSvc)
	ctx := context.Background()

	t.Run("Create Success", func(t *testing.T) {
		req := transfers.CreateTransferRequest{
			TransferType:  transfers.TransferTypeDepartment,
			EffectiveDate: "2026-08-01",
		}
		trf, err := svc.Create(ctx, "org1", "emp1", "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if trf.TransferType != transfers.TransferTypeDepartment {
			t.Errorf("expected department transfer, got %s", trf.TransferType)
		}
	})

	t.Run("Create Validation Error", func(t *testing.T) {
		req := transfers.CreateTransferRequest{TransferType: "invalid"}
		_, err := svc.Create(ctx, "org1", "emp1", "admin", req)
		if err != transfers.ErrInvalidTransferType {
			t.Errorf("expected ErrInvalidTransferType, got %v", err)
		}
	})

	t.Run("Get and List", func(t *testing.T) {
		req := transfers.CreateTransferRequest{TransferType: transfers.TransferTypeLocation, EffectiveDate: "2026-08-01"}
		trf, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		fetched, err := svc.Get(ctx, "org1", "emp1", trf.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.ID != trf.ID {
			t.Errorf("ID mismatch")
		}

		list, err := svc.List(ctx, "org1", transfers.TransferListFilter{EmployeeID: "emp1", Scope: authz.ScopeAll})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if list.Total < 1 {
			t.Errorf("expected at least 1 transfer in list")
		}
	})

	t.Run("Update", func(t *testing.T) {
		req := transfers.CreateTransferRequest{TransferType: transfers.TransferTypeLocation, EffectiveDate: "2026-08-01"}
		trf, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		updateReq := transfers.UpdateTransferRequest{EffectiveDate: ptrStr("2026-09-01")}
		updated, err := svc.Update(ctx, "org1", "emp1", trf.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.EffectiveDate != "2026-09-01" {
			t.Errorf("expected updated date, got %s", updated.EffectiveDate)
		}
	})

	t.Run("Submit without template", func(t *testing.T) {
		req := transfers.CreateTransferRequest{TransferType: transfers.TransferTypeLocation, EffectiveDate: "2026-08-01"}
		trf, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		submitted, err := svc.Submit(ctx, "org1", "emp1", trf.ID, "admin")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if submitted.Status != transfers.StatusApproved {
			t.Errorf("expected approved status (fallback), got %s", submitted.Status)
		}
	})

	t.Run("Submit with template", func(t *testing.T) {
		req := transfers.CreateTransferRequest{TransferType: transfers.TransferTypeLocation, EffectiveDate: "2026-08-01"}
		trf, _ := svc.Create(ctx, "org1", "emp1", "admin", req)
		appSvc.defaultTemplate = &approvals.ApprovalTemplate{ID: "tmpl1"}

		submitted, err := svc.Submit(ctx, "org1", "emp1", trf.ID, "admin")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if submitted.Status != transfers.StatusPendingApproval {
			t.Errorf("expected pending_approval, got %s", submitted.Status)
		}
		appSvc.defaultTemplate = nil // reset
	})
	
	t.Run("Cancel", func(t *testing.T) {
		req := transfers.CreateTransferRequest{TransferType: transfers.TransferTypeLocation, EffectiveDate: "2026-08-01"}
		trf, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		cancelled, err := svc.Cancel(ctx, "org1", "emp1", trf.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cancelled.Status != transfers.StatusCancelled {
			t.Errorf("expected cancelled, got %s", cancelled.Status)
		}
	})
	
	t.Run("Cross-org isolation", func(t *testing.T) {
		req := transfers.CreateTransferRequest{TransferType: transfers.TransferTypeLocation, EffectiveDate: "2026-08-01"}
		trf, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		_, err := svc.Get(ctx, "org2", "emp1", trf.ID)
		if err != transfers.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org access, got %v", err)
		}
	})
}
