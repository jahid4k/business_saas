// backend/internal/tests/unit/hrm/promotions/service_test.go
package promotions_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/promotions"
)

type stubRepo struct {
	byID map[string]*promotions.Promotion
	seq  int
}

func newStubRepo() *stubRepo {
	return &stubRepo{byID: make(map[string]*promotions.Promotion)}
}

func (r *stubRepo) nextID() string {
	r.seq++
	return fmt.Sprintf("promo-%d", r.seq)
}

func (r *stubRepo) FindAll(ctx context.Context, orgID string, filter promotions.PromotionListFilter) ([]*promotions.Promotion, error) {
	var out []*promotions.Promotion
	for _, p := range r.byID {
		if p.OrgID == orgID {
			if filter.EmployeeID != "" && p.EmployeeID != filter.EmployeeID {
				continue
			}
			if filter.Status != "" && string(p.Status) != filter.Status {
				continue
			}
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *stubRepo) Count(ctx context.Context, orgID string, filter promotions.PromotionListFilter) (int, error) {
	out, err := r.FindAll(ctx, orgID, filter)
	return len(out), err
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*promotions.Promotion, error) {
	p, ok := r.byID[ref]
	if !ok || p.OrgID != orgID {
		return nil, nil
	}
	if employeeID != "" && p.EmployeeID != employeeID {
		return nil, nil
	}
	return p, nil
}

func (r *stubRepo) Create(ctx context.Context, p *promotions.Promotion) error {
	p.ID = r.nextID()
	p.PublicID = "pub_" + p.ID
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	r.byID[p.ID] = p
	return nil
}

func (r *stubRepo) Update(ctx context.Context, p *promotions.Promotion) error {
	if _, ok := r.byID[p.ID]; !ok {
		return errors.New("not found")
	}
	p.UpdatedAt = time.Now()
	r.byID[p.ID] = p
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status promotions.PromotionStatus) error {
	p, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = status
	return nil
}

func (r *stubRepo) SetApprovalInstance(ctx context.Context, id, instanceID string, status promotions.PromotionStatus) error {
	p, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	p.ApprovalInstanceID = &instanceID
	p.Status = status
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

func TestPromotionsService(t *testing.T) {
	repo := newStubRepo()
	appSvc := &stubApprovalsSvc{}
	svc := promotions.NewService(repo, newDummyPool(), appSvc)
	ctx := context.Background()

	t.Run("Create Success", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{
			ToPositionID:  "pos2",
			EffectiveDate: "2026-08-01",
		}
		promo, err := svc.Create(ctx, "org1", "emp1", "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if promo.ToPositionID != "pos2" {
			t.Errorf("expected to_position pos2, got %s", promo.ToPositionID)
		}
	})

	t.Run("Create Validation Error", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{EffectiveDate: "2026-08-01"} // missing ToPositionID
		_, err := svc.Create(ctx, "org1", "emp1", "admin", req)
		if err != promotions.ErrToPositionRequired {
			t.Errorf("expected ErrToPositionRequired, got %v", err)
		}
	})

	t.Run("Get and List", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{ToPositionID: "pos2", EffectiveDate: "2026-08-01"}
		promo, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		fetched, err := svc.Get(ctx, "org1", "emp1", promo.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.ID != promo.ID {
			t.Errorf("ID mismatch")
		}

		list, err := svc.List(ctx, "org1", promotions.PromotionListFilter{EmployeeID: "emp1", Scope: authz.ScopeAll})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if list.Total < 1 {
			t.Errorf("expected at least 1 promotion in list")
		}
	})

	t.Run("Update", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{ToPositionID: "pos2", EffectiveDate: "2026-08-01"}
		promo, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		updateReq := promotions.UpdatePromotionRequest{EffectiveDate: ptrStr("2026-09-01")}
		updated, err := svc.Update(ctx, "org1", "emp1", promo.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.EffectiveDate != "2026-09-01" {
			t.Errorf("expected updated date, got %s", updated.EffectiveDate)
		}
	})

	t.Run("Submit without template", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{ToPositionID: "pos2", EffectiveDate: "2026-08-01"}
		promo, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		submitted, err := svc.Submit(ctx, "org1", "emp1", promo.ID, "admin")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if submitted.Status != promotions.StatusApproved {
			t.Errorf("expected approved status (fallback), got %s", submitted.Status)
		}
	})

	t.Run("Submit with template", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{ToPositionID: "pos2", EffectiveDate: "2026-08-01"}
		promo, _ := svc.Create(ctx, "org1", "emp1", "admin", req)
		appSvc.defaultTemplate = &approvals.ApprovalTemplate{ID: "tmpl1", ActionType: approvals.ActionTypePromotion}

		submitted, err := svc.Submit(ctx, "org1", "emp1", promo.ID, "admin")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if submitted.Status != promotions.StatusPendingApproval {
			t.Errorf("expected pending_approval, got %s", submitted.Status)
		}
		appSvc.defaultTemplate = nil // reset
	})
	
	t.Run("Cancel", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{ToPositionID: "pos2", EffectiveDate: "2026-08-01"}
		promo, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		cancelled, err := svc.Cancel(ctx, "org1", "emp1", promo.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cancelled.Status != promotions.StatusCancelled {
			t.Errorf("expected cancelled, got %s", cancelled.Status)
		}
	})
	
	t.Run("Cross-org isolation", func(t *testing.T) {
		req := promotions.CreatePromotionRequest{ToPositionID: "pos2", EffectiveDate: "2026-08-01"}
		promo, _ := svc.Create(ctx, "org1", "emp1", "admin", req)

		_, err := svc.Get(ctx, "org2", "emp1", promo.ID)
		if err != promotions.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org access, got %v", err)
		}
	})
}
