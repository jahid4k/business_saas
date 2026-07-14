package warningtypes_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mridha/businesssaas/internal/hrm/warningtypes"
)

type stubRepo struct {
	types map[string]*warningtypes.WarningType
	rules map[string]*warningtypes.WarningEscalationRule
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		types: make(map[string]*warningtypes.WarningType),
		rules: make(map[string]*warningtypes.WarningEscalationRule),
	}
}

func (r *stubRepo) FindAllTypes(ctx context.Context, orgID string, activeOnly bool) ([]*warningtypes.WarningType, error) {
	var res []*warningtypes.WarningType
	for _, t := range r.types {
		if t.OrgID == orgID {
			if activeOnly && !t.IsActive {
				continue
			}
			res = append(res, t)
		}
	}
	return res, nil
}

func (r *stubRepo) FindTypeByRef(ctx context.Context, orgID, ref string) (*warningtypes.WarningType, error) {
	for _, t := range r.types {
		if t.OrgID == orgID && (t.ID == ref || t.PublicID == ref) {
			return t, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateType(ctx context.Context, wt *warningtypes.WarningType) error {
	wt.ID = uuid.NewString()
	wt.PublicID = "wt_" + wt.ID
	r.types[wt.ID] = wt
	return nil
}

func (r *stubRepo) UpdateType(ctx context.Context, wt *warningtypes.WarningType) error {
	r.types[wt.ID] = wt
	return nil
}

func (r *stubRepo) DeleteType(ctx context.Context, orgID, ref string) error {
	for id, t := range r.types {
		if t.OrgID == orgID && (t.ID == ref || t.PublicID == ref) {
			delete(r.types, id)
			return nil
		}
	}
	return warningtypes.ErrWarningTypeNotFound
}

func (r *stubRepo) TypeNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	for _, t := range r.types {
		if t.OrgID == orgID && strings.EqualFold(t.Name, name) && t.IsActive && t.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) FindEscalationRules(ctx context.Context, orgID, warningTypeID string) ([]*warningtypes.WarningEscalationRule, error) {
	var res []*warningtypes.WarningEscalationRule
	for _, rule := range r.rules {
		if rule.OrgID == orgID {
			if warningTypeID == "" || rule.TriggerWarningTypeID == warningTypeID {
				res = append(res, rule)
			}
		}
	}
	return res, nil
}

func (r *stubRepo) FindEscalationRuleByRef(ctx context.Context, orgID, ref string) (*warningtypes.WarningEscalationRule, error) {
	for _, rule := range r.rules {
		if rule.OrgID == orgID && (rule.ID == ref || rule.PublicID == ref) {
			return rule, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateEscalationRule(ctx context.Context, r2 *warningtypes.WarningEscalationRule) error {
	r2.ID = uuid.NewString()
	r2.PublicID = "er_" + r2.ID
	r.rules[r2.ID] = r2
	return nil
}

func (r *stubRepo) UpdateEscalationRule(ctx context.Context, r2 *warningtypes.WarningEscalationRule) error {
	r.rules[r2.ID] = r2
	return nil
}

func (r *stubRepo) DeleteEscalationRule(ctx context.Context, orgID, ref string) error {
	for id, rule := range r.rules {
		if rule.OrgID == orgID && (rule.ID == ref || rule.PublicID == ref) {
			delete(r.rules, id)
			return nil
		}
	}
	return warningtypes.ErrEscalationRuleNotFound
}

func TestWarningTypesService(t *testing.T) {
	repo := newStubRepo()
	svc := warningtypes.NewService(repo)
	ctx := context.Background()
	orgID := "org1"

	t.Run("CreateType Success", func(t *testing.T) {
		req := warningtypes.CreateWarningTypeRequest{
			Name: "Verbal Warning",
		}
		wt, err := svc.CreateType(ctx, orgID, "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if wt.Name != "Verbal Warning" {
			t.Errorf("expected name Verbal Warning, got %s", wt.Name)
		}
	})

	t.Run("CreateType Validation", func(t *testing.T) {
		req := warningtypes.CreateWarningTypeRequest{Name: ""}
		_, err := svc.CreateType(ctx, orgID, "admin", req)
		if err != warningtypes.ErrNameRequired {
			t.Errorf("expected ErrNameRequired, got %v", err)
		}
	})

	t.Run("GetType and UpdateType", func(t *testing.T) {
		req := warningtypes.CreateWarningTypeRequest{Name: "Type1"}
		wt, _ := svc.CreateType(ctx, orgID, "admin", req)

		desc := "description"
		updateReq := warningtypes.UpdateWarningTypeRequest{Description: &desc}
		updated, err := svc.UpdateType(ctx, orgID, wt.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if *updated.Description != "description" {
			t.Errorf("expected updated description, got %v", updated.Description)
		}

		fetched, err := svc.GetType(ctx, orgID, wt.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if *fetched.Description != "description" {
			t.Errorf("expected description description, got %v", fetched.Description)
		}
	})

	t.Run("Cross-Org Isolation", func(t *testing.T) {
		req := warningtypes.CreateWarningTypeRequest{Name: "Org2 Type"}
		wt, _ := svc.CreateType(ctx, "org2", "admin", req)

		_, err := svc.GetType(ctx, orgID, wt.ID)
		if err != warningtypes.ErrWarningTypeNotFound {
			t.Errorf("expected ErrWarningTypeNotFound for cross-org access, got %v", err)
		}
	})

	t.Run("DeleteType", func(t *testing.T) {
		req := warningtypes.CreateWarningTypeRequest{Name: "To Delete"}
		wt, _ := svc.CreateType(ctx, orgID, "admin", req)

		err := svc.DeleteType(ctx, orgID, wt.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = svc.GetType(ctx, orgID, wt.ID)
		if err != warningtypes.ErrWarningTypeNotFound {
			t.Errorf("expected ErrWarningTypeNotFound, got %v", err)
		}
	})
	
	t.Run("Escalation Rules", func(t *testing.T) {
		wtReq := warningtypes.CreateWarningTypeRequest{Name: "Trigger"}
		wt, _ := svc.CreateType(ctx, orgID, "admin", wtReq)
		
		req := warningtypes.CreateEscalationRuleRequest{
			TriggerWarningTypeID: wt.ID,
			TriggerCount: 3,
			Action: warningtypes.EscalationActionNotifyHR,
		}
		
		rule, err := svc.CreateEscalationRule(ctx, orgID, "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if rule.TriggerCount != 3 {
			t.Errorf("expected trigger count 3, got %d", rule.TriggerCount)
		}
		
		newCount := 4
		updateReq := warningtypes.UpdateEscalationRuleRequest{
			TriggerCount: &newCount,
		}
		updated, err := svc.UpdateEscalationRule(ctx, orgID, rule.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.TriggerCount != 4 {
			t.Errorf("expected 4, got %d", updated.TriggerCount)
		}
		
		listResp, err := svc.ListEscalationRules(ctx, orgID, wt.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if listResp.Total != 1 {
			t.Errorf("expected total 1, got %d", listResp.Total)
		}
		
		fetched, err := svc.GetEscalationRule(ctx, orgID, rule.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.TriggerCount != 4 {
			t.Errorf("expected 4, got %d", fetched.TriggerCount)
		}
		
		err = svc.DeleteEscalationRule(ctx, orgID, rule.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}
