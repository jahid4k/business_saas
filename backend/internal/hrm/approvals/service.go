// backend/internal/hrm/approvals/service.go
package approvals

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// EntityCallback is invoked when an approval instance reaches a terminal
// state (approved or rejected), so the source record (termination, promotion,
// warning, etc.) can react — e.g. flip its own status out of pending_approval.
// entityID is the internal (non-public) ID that was passed as EntityID when
// the instance was created. Callback errors are logged by the caller but do
// NOT roll back the approval decision itself — the decision is the source of
// truth; a failed callback means the source record is out of sync and needs
// manual reconciliation, not that the approval never happened.
type EntityCallback func(ctx context.Context, orgID, entityID string, approved bool) error

// Service defines business logic for the approval engine.
// It is also used as a shared dependency by other HRM services (leave, promotion, etc.)
type Service interface {
	// Template management
	ListTemplates(ctx context.Context, orgID, actionType string) (*TemplateListResponse, error)
	GetTemplate(ctx context.Context, orgID, ref string) (*ApprovalTemplate, error)
	CreateTemplate(ctx context.Context, orgID, createdBy string, req CreateTemplateRequest) (*ApprovalTemplate, error)
	UpdateTemplate(ctx context.Context, orgID, ref string, req UpdateTemplateRequest) (*ApprovalTemplate, error)
	DeleteTemplate(ctx context.Context, orgID, ref string) error

	// Instance lifecycle — called by other services
	ListInstances(ctx context.Context, orgID string, limit, offset int, status string, requesterID string) (*InstanceListResponse, error)
	CreateInstance(ctx context.Context, orgID string, req CreateInstanceRequest) (*ApprovalInstance, error)
	GetInstance(ctx context.Context, orgID, ref string) (*ApprovalInstance, error)
	Decide(ctx context.Context, orgID, instanceRef, approverID string, req DecisionRequest) (*ApprovalInstance, error)
	CancelInstance(ctx context.Context, orgID, instanceRef, cancelledBy string) (*ApprovalInstance, error)

	// Convenience: find default template for action type
	FindDefault(ctx context.Context, orgID string, actionType ActionType) (*ApprovalTemplate, error)

	// RegisterCallback wires a source module (termination, promotion, ...) so that
	// when one of its instances completes, the module can update its own record.
	// entityType must match the EntityType used in CreateInstanceRequest (e.g. "termination").
	// Called once per module during app wiring (main.go), after all services exist.
	RegisterCallback(entityType string, fn EntityCallback)
}

type serviceImpl struct {
	repo      Repository
	callbacks map[string]EntityCallback
}

func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo, callbacks: make(map[string]EntityCallback)}
}

func (s *serviceImpl) RegisterCallback(entityType string, fn EntityCallback) {
	s.callbacks[entityType] = fn
}

func (s *serviceImpl) ListTemplates(ctx context.Context, orgID, actionType string) (*TemplateListResponse, error) {
	list, err := s.repo.FindAllTemplates(ctx, orgID, actionType)
	if err != nil {
		return nil, fmt.Errorf("approvals: ListTemplates: %w", err)
	}
	if list == nil {
		list = []*ApprovalTemplate{}
	}
	return &TemplateListResponse{Templates: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetTemplate(ctx context.Context, orgID, ref string) (*ApprovalTemplate, error) {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("approvals: GetTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	levels, err := s.repo.FindTemplateLevels(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("approvals: GetTemplate levels: %w", err)
	}
	t.Levels = levels
	return t, nil
}

func (s *serviceImpl) CreateTemplate(ctx context.Context, orgID, createdBy string, req CreateTemplateRequest) (*ApprovalTemplate, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if !req.ActionType.IsValid() {
		return nil, ErrInvalidActionType
	}
	if len(req.Levels) == 0 {
		return nil, ErrNoLevels
	}
	for i, lv := range req.Levels {
		if lv.Level != i+1 {
			return nil, ErrInvalidLevel
		}
		if !lv.ApproverType.IsValid() {
			return nil, ErrInvalidApproverType
		}
		if lv.SLAHours <= 0 {
			lv.SLAHours = 48
		}
		if lv.OnSLABreach == "" {
			req.Levels[i].OnSLABreach = SLABreachEscalateNext
		}
	}

	t := &ApprovalTemplate{
		OrgID: orgID, Name: name, Description: req.Description,
		ActionType: req.ActionType, ConditionExpression: req.ConditionExpression,
		IsDefault: req.IsDefault, CreatedBy: createdBy,
	}
	levels := make([]*ApprovalTemplateLevel, len(req.Levels))
	for i, lv := range req.Levels {
		levels[i] = &ApprovalTemplateLevel{
			Level: lv.Level, ApproverType: lv.ApproverType,
			ApproverRole: lv.ApproverRole, ApproverUserID: lv.ApproverUserID,
			SLAHours: lv.SLAHours, OnSLABreach: lv.OnSLABreach,
		}
	}
	if err := s.repo.CreateTemplate(ctx, t, levels); err != nil {
		return nil, fmt.Errorf("approvals: CreateTemplate: %w", err)
	}
	t.Levels = levels
	return t, nil
}

func (s *serviceImpl) UpdateTemplate(ctx context.Context, orgID, ref string, req UpdateTemplateRequest) (*ApprovalTemplate, error) {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("approvals: UpdateTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	if req.Name != nil {
		t.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		t.Description = req.Description
	}
	if req.ConditionExpression != nil {
		t.ConditionExpression = req.ConditionExpression
	}
	if req.IsDefault != nil {
		t.IsDefault = *req.IsDefault
	}
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("approvals: UpdateTemplate: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) DeleteTemplate(ctx context.Context, orgID, ref string) error {
	return s.repo.DeleteTemplate(ctx, orgID, ref)
}

func (s *serviceImpl) FindDefault(ctx context.Context, orgID string, actionType ActionType) (*ApprovalTemplate, error) {
	t, err := s.repo.FindDefaultTemplate(ctx, orgID, actionType)
	if err != nil {
		return nil, fmt.Errorf("approvals: FindDefault: %w", err)
	}
	if t != nil {
		levels, _ := s.repo.FindTemplateLevels(ctx, t.ID)
		t.Levels = levels
	}
	return t, nil
}

func (s *serviceImpl) ListInstances(ctx context.Context, orgID string, limit, offset int, status string, requesterID string) (*InstanceListResponse, error) {
	list, total, err := s.repo.FindAllInstances(ctx, orgID, limit, offset, status, requesterID)
	if err != nil {
		return nil, fmt.Errorf("approvals: ListInstances: %w", err)
	}
	if list == nil {
		list = []*ApprovalInstance{}
	}
	return &InstanceListResponse{Instances: list, Total: total}, nil
}

func (s *serviceImpl) CreateInstance(ctx context.Context, orgID string, req CreateInstanceRequest) (*ApprovalInstance, error) {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, req.TemplateID)
	if err != nil || t == nil {
		return nil, ErrTemplateNotFound
	}
	levels, err := s.repo.FindTemplateLevels(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("approvals: CreateInstance levels: %w", err)
	}

	inst := &ApprovalInstance{
		OrgID: orgID, TemplateID: &t.ID,
		EntityType: req.EntityType, EntityID: req.EntityID,
		CurrentLevel: 1, Snapshot: levels,
		RequestedBy: req.RequestedBy,
	}
	if err := s.repo.CreateInstance(ctx, inst); err != nil {
		return nil, fmt.Errorf("approvals: CreateInstance: %w", err)
	}
	return inst, nil
}

func (s *serviceImpl) GetInstance(ctx context.Context, orgID, ref string) (*ApprovalInstance, error) {
	inst, err := s.repo.FindInstanceByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("approvals: GetInstance: %w", err)
	}
	if inst == nil {
		return nil, ErrInstanceNotFound
	}
	decisions, err := s.repo.FindDecisions(ctx, inst.ID)
	if err != nil {
		return nil, fmt.Errorf("approvals: GetInstance decisions: %w", err)
	}
	inst.Decisions = decisions
	return inst, nil
}

func (s *serviceImpl) Decide(ctx context.Context, orgID, instanceRef, approverID string, req DecisionRequest) (*ApprovalInstance, error) {
	inst, err := s.repo.FindInstanceByRef(ctx, orgID, instanceRef)
	if err != nil {
		return nil, fmt.Errorf("approvals: Decide: %w", err)
	}
	if inst == nil {
		return nil, ErrInstanceNotFound
	}
	if inst.OverallStatus != InstanceStatusPending {
		return nil, ErrAlreadyCompleted
	}

	decision := &ApprovalDecision{
		InstanceID: inst.ID, Level: inst.CurrentLevel,
		ApproverID: approverID, Action: req.Action, Note: req.Note,
	}
	if err := s.repo.CreateDecision(ctx, decision); err != nil {
		return nil, fmt.Errorf("approvals: Decide: %w", err)
	}

	if req.Action == "rejected" || req.Action == "cancelled" {
		now := time.Now()
		inst.OverallStatus = InstanceStatus(req.Action)
		inst.CompletedAt = &now
	} else {
		// approved — advance to next level or complete
		inst.CurrentLevel++
		if inst.CurrentLevel > len(inst.Snapshot) {
			now := time.Now()
			inst.OverallStatus = InstanceStatusApproved
			inst.CompletedAt = &now
		}
	}
	if err := s.repo.UpdateInstance(ctx, inst); err != nil {
		return nil, fmt.Errorf("approvals: Decide update: %w", err)
	}

	// Notify the source module once the instance reaches a terminal state.
	// "cancelled" is intentionally excluded — cancellation is initiated by the
	// source module itself (via CancelInstance), so there is nothing to call back.
	if inst.OverallStatus == InstanceStatusApproved || inst.OverallStatus == InstanceStatusRejected {
		if cb, ok := s.callbacks[inst.EntityType]; ok {
			approved := inst.OverallStatus == InstanceStatusApproved
			if cbErr := cb(ctx, orgID, inst.EntityID, approved); cbErr != nil {
				return inst, fmt.Errorf("approvals: Decide: entity callback for %q failed (approval instance %s is saved as %s, source record may be out of sync): %w",
					inst.EntityType, inst.ID, inst.OverallStatus, cbErr)
			}
		}
	}
	return inst, nil
}

func (s *serviceImpl) CancelInstance(ctx context.Context, orgID, instanceRef, cancelledBy string) (*ApprovalInstance, error) {
	return s.Decide(ctx, orgID, instanceRef, cancelledBy, DecisionRequest{Action: "cancelled"})
}
