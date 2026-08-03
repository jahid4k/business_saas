package approvals_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
)

type mockApprovalsRepo struct {
	templates map[string]*approvals.ApprovalTemplate
	levels    map[string][]*approvals.ApprovalTemplateLevel
	instances map[string]*approvals.ApprovalInstance
	decisions map[string][]*approvals.ApprovalDecision
}

func newMockApprovalsRepo() *mockApprovalsRepo {
	return &mockApprovalsRepo{
		templates: make(map[string]*approvals.ApprovalTemplate),
		levels:    make(map[string][]*approvals.ApprovalTemplateLevel),
		instances: make(map[string]*approvals.ApprovalInstance),
		decisions: make(map[string][]*approvals.ApprovalDecision),
	}
}

func (m *mockApprovalsRepo) FindAllTemplates(ctx context.Context, orgID string, actionType string) ([]*approvals.ApprovalTemplate, error) {
	var list []*approvals.ApprovalTemplate
	for _, t := range m.templates {
		if t.OrgID == orgID && (actionType == "" || string(t.ActionType) == actionType) && t.IsActive {
			list = append(list, t)
		}
	}
	return list, nil
}

func (m *mockApprovalsRepo) FindTemplateByRef(ctx context.Context, orgID, ref string) (*approvals.ApprovalTemplate, error) {
	for _, t := range m.templates {
		if t.OrgID == orgID && (t.ID == ref || t.PublicID == ref) {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockApprovalsRepo) FindDefaultTemplate(ctx context.Context, orgID string, actionType approvals.ActionType) (*approvals.ApprovalTemplate, error) {
	for _, t := range m.templates {
		if t.OrgID == orgID && t.ActionType == actionType && t.IsDefault && t.IsActive {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockApprovalsRepo) CreateTemplate(ctx context.Context, t *approvals.ApprovalTemplate, levels []*approvals.ApprovalTemplateLevel) error {
	t.ID = "tmpl-" + t.Name
	t.PublicID = "pub-" + t.ID
	m.templates[t.ID] = t
	for _, l := range levels {
		l.TemplateID = t.ID
	}
	m.levels[t.ID] = levels
	return nil
}

func (m *mockApprovalsRepo) UpdateTemplate(ctx context.Context, t *approvals.ApprovalTemplate) error {
	if existing, ok := m.templates[t.ID]; ok {
		*existing = *t
	}
	return nil
}

func (m *mockApprovalsRepo) DeleteTemplate(ctx context.Context, orgID, ref string) error {
	for id, t := range m.templates {
		if t.OrgID == orgID && (t.ID == ref || t.PublicID == ref) {
			delete(m.templates, id)
			return nil
		}
	}
	return approvals.ErrTemplateNotFound
}

func (m *mockApprovalsRepo) FindTemplateLevels(ctx context.Context, templateID string) ([]*approvals.ApprovalTemplateLevel, error) {
	return m.levels[templateID], nil
}

func (m *mockApprovalsRepo) FindInstanceByRef(ctx context.Context, orgID, ref string) (*approvals.ApprovalInstance, error) {
	for _, inst := range m.instances {
		if inst.OrgID == orgID && (inst.ID == ref || inst.PublicID == ref) {
			return inst, nil
		}
	}
	return nil, nil
}

func (m *mockApprovalsRepo) FindInstanceByEntity(ctx context.Context, entityType, entityID string) (*approvals.ApprovalInstance, error) {
	for _, inst := range m.instances {
		if inst.EntityType == entityType && inst.EntityID == entityID && inst.OverallStatus == approvals.InstanceStatusPending {
			return inst, nil
		}
	}
	return nil, nil
}

func (m *mockApprovalsRepo) CreateInstance(ctx context.Context, inst *approvals.ApprovalInstance) error {
	inst.ID = "inst-" + inst.EntityID
	inst.PublicID = "pub-" + inst.ID
	inst.OverallStatus = approvals.InstanceStatusPending
	m.instances[inst.ID] = inst
	return nil
}

func (m *mockApprovalsRepo) UpdateInstance(ctx context.Context, inst *approvals.ApprovalInstance) error {
	if existing, ok := m.instances[inst.ID]; ok {
		*existing = *inst
	}
	return nil
}

func (m *mockApprovalsRepo) CreateDecision(ctx context.Context, d *approvals.ApprovalDecision) error {
	d.ID = "dec-" + time.Now().String()
	m.decisions[d.InstanceID] = append(m.decisions[d.InstanceID], d)
	return nil
}

func (m *mockApprovalsRepo) FindDecisions(ctx context.Context, instanceID string) ([]*approvals.ApprovalDecision, error) {
	return m.decisions[instanceID], nil
}

func (m *mockApprovalsRepo) FindAllInstances(ctx context.Context, orgID string, limit, offset int, status string, requesterID string) ([]*approvals.ApprovalInstance, int, error) {
	return nil, 0, nil
}


func TestApprovalsService(t *testing.T) {
	repo := newMockApprovalsRepo()
	svc := approvals.NewService(repo)
	ctx := context.Background()

	orgID := "org1"
	createdBy := "admin1"

	t.Run("CreateTemplate", func(t *testing.T) {
		req := approvals.CreateTemplateRequest{
			Name:       "Test Template",
			ActionType: approvals.ActionTypeAward,
			IsDefault:  true,
			Levels: []approvals.CreateTemplateLevelRequest{
				{
					Level:        1,
					ApproverType: approvals.ApproverTypeReportingManager,
				},
			},
		}

		tmpl, err := svc.CreateTemplate(ctx, orgID, createdBy, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tmpl.Name != "Test Template" {
			t.Errorf("expected name Test Template, got %s", tmpl.Name)
		}
	})

	t.Run("GetTemplate", func(t *testing.T) {
		tmpl, err := svc.GetTemplate(ctx, orgID, "tmpl-Test Template")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tmpl.Levels) != 1 {
			t.Errorf("expected 1 level, got %d", len(tmpl.Levels))
		}
	})

	t.Run("UpdateTemplate", func(t *testing.T) {
		name := "Updated Template"
		tmpl, err := svc.UpdateTemplate(ctx, orgID, "tmpl-Test Template", approvals.UpdateTemplateRequest{
			Name: &name,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tmpl.Name != name {
			t.Errorf("expected name %s, got %s", name, tmpl.Name)
		}
	})

	t.Run("CreateInstance", func(t *testing.T) {
		req := approvals.CreateInstanceRequest{
			TemplateID:  "tmpl-Test Template",
			EntityType:  "award",
			EntityID:    "award1",
			RequestedBy: "emp1",
		}

		inst, err := svc.CreateInstance(ctx, orgID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inst.OverallStatus != approvals.InstanceStatusPending {
			t.Errorf("expected pending status, got %v", inst.OverallStatus)
		}
	})

	t.Run("Decide", func(t *testing.T) {
		inst, err := svc.Decide(ctx, orgID, "inst-award1", "mgr1", approvals.DecisionRequest{
			Action: "approved",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inst.OverallStatus != approvals.InstanceStatusApproved {
			t.Errorf("expected approved status, got %v", inst.OverallStatus)
		}
	})

	t.Run("CancelInstance", func(t *testing.T) {
		// create another instance to cancel
		req := approvals.CreateInstanceRequest{
			TemplateID:  "tmpl-Test Template",
			EntityType:  "award",
			EntityID:    "award2",
			RequestedBy: "emp1",
		}
		inst, _ := svc.CreateInstance(ctx, orgID, req)

		cancelledInst, err := svc.CancelInstance(ctx, orgID, inst.ID, "emp1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cancelledInst.OverallStatus != approvals.InstanceStatusCancelled {
			t.Errorf("expected cancelled status, got %v", cancelledInst.OverallStatus)
		}
	})

	t.Run("Cross-Org Isolation", func(t *testing.T) {
		_, err := svc.GetTemplate(ctx, "org2", "tmpl-Updated Template")
		if err != approvals.ErrTemplateNotFound {
			t.Errorf("expected ErrTemplateNotFound for cross-org fetch, got %v", err)
		}
	})
}
