// backend/internal/platform/checklists/service.go
package checklists

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mridha/businesssaas/pkg/pagination"
)

// AccessDirectory is the minimal slice of authz.Service this package needs:
// a permission check plus the two role-directory queries added for Phase 3.
// Declared locally (the authz.SessionRevoker precedent) so this package gains
// no platform → authz import edge; authz.Service satisfies it structurally.
// Parameter order matches authz.Service's methods exactly so that structural
// typing holds.
type AccessDirectory interface {
	Can(ctx context.Context, userID, orgID, resource, action string) (bool, error)
	UserRoleName(ctx context.Context, orgID, userID string) (string, error)
	RoleExists(ctx context.Context, orgID, roleName string) (bool, error)
}

// Service defines the business logic interface for the checklist engine.
type Service interface {
	// Templates
	ListTemplates(ctx context.Context, orgID string, checklistType *ChecklistType) ([]*Template, error)
	GetTemplate(ctx context.Context, orgID, templateID string) (*TemplateWithItems, error)
	CreateTemplate(ctx context.Context, orgID, userID string, req CreateTemplateRequest) (*TemplateWithItems, error)
	UpdateTemplate(ctx context.Context, orgID, templateID string, req UpdateTemplateRequest) (*Template, error)
	DeleteTemplate(ctx context.Context, orgID, templateID string) error

	// Template items
	ListTemplateItems(ctx context.Context, orgID, templateID string) ([]*TemplateItem, error)
	CreateTemplateItem(ctx context.Context, orgID, templateID string, req CreateTemplateItemRequest) (*TemplateItem, error)
	UpdateTemplateItem(ctx context.Context, orgID, templateID, itemID string, req UpdateTemplateItemRequest) (*TemplateItem, error)
	DeleteTemplateItem(ctx context.Context, orgID, templateID, itemID string) error

	// Instantiation — deliberately NOT exposed over a generic HTTP route.
	// Callers must resolve SubjectContext themselves (see model.go's doc
	// comment on SubjectContext) — this is what stops a caller from pointing
	// checklist items at arbitrary users.
	Instantiate(ctx context.Context, orgID, templateID string, subj SubjectContext) (*InstantiateResult, error)
	// InstantiateDefault returns (nil, nil) — not an error — when the org has
	// no default template for checklistType. That is the auto-hook's normal
	// no-op path in every unconfigured org, not a failure worth logging.
	InstantiateDefault(ctx context.Context, orgID string, checklistType ChecklistType, subj SubjectContext) (*InstantiateResult, error)

	// Instances
	ListInstances(ctx context.Context, orgID string, f InstanceFilter) (*InstanceListResponse, error)
	GetInstance(ctx context.Context, orgID, instanceID string) (*InstanceWithItems, error)
	CancelInstance(ctx context.Context, orgID, instanceID string, req CancelInstanceRequest) (*Instance, error)

	// Items
	ListMyItems(ctx context.Context, orgID, userID string, status *ItemStatus) ([]*InstanceItem, error)
	CompleteItem(ctx context.Context, orgID, itemID, callerUserID string, req CompleteItemRequest) (*InstanceItem, error)
	ReopenItem(ctx context.Context, orgID, itemID, callerUserID string) (*InstanceItem, error)
	SkipItem(ctx context.Context, orgID, itemID, callerUserID string, req SkipItemRequest) (*InstanceItem, error)
}

type serviceImpl struct {
	repo      Repository
	directory AccessDirectory
}

func NewService(repo Repository, directory AccessDirectory) Service {
	return &serviceImpl{repo: repo, directory: directory}
}

// ============================================================
// Templates
// ============================================================

func (s *serviceImpl) ListTemplates(ctx context.Context, orgID string, checklistType *ChecklistType) ([]*Template, error) {
	list, err := s.repo.FindTemplates(ctx, orgID, checklistType)
	if err != nil {
		return nil, fmt.Errorf("checklists: ListTemplates: %w", err)
	}
	if list == nil {
		list = []*Template{}
	}
	return list, nil
}

func (s *serviceImpl) GetTemplate(ctx context.Context, orgID, templateID string) (*TemplateWithItems, error) {
	t, err := s.repo.FindTemplateByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("checklists: GetTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	items, err := s.repo.FindTemplateItems(ctx, orgID, t.ID)
	if err != nil {
		return nil, fmt.Errorf("checklists: GetTemplate: items: %w", err)
	}
	if items == nil {
		items = []*TemplateItem{}
	}
	// Best-effort enrichment so the UI can flag a template item whose
	// owner_role no longer names a live role (the mitigation for the
	// rename-fragility limitation — see the migration 00076 header).
	for _, it := range items {
		if it.OwnerType != OwnerTypeRole || it.OwnerRole == nil {
			continue
		}
		exists, err := s.directory.RoleExists(ctx, orgID, *it.OwnerRole)
		if err != nil {
			continue
		}
		it.OwnerRoleExists = &exists
	}
	return &TemplateWithItems{Template: t, Items: items}, nil
}

func (s *serviceImpl) CreateTemplate(ctx context.Context, orgID, userID string, req CreateTemplateRequest) (*TemplateWithItems, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if !req.ChecklistType.IsValid() {
		return nil, ErrInvalidChecklistType
	}

	items := make([]*TemplateItem, 0, len(req.Items))
	for _, itReq := range req.Items {
		item, err := s.buildTemplateItem(ctx, orgID, itReq)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	t := &Template{
		OrgID:         orgID,
		Name:          name,
		Description:   req.Description,
		ChecklistType: req.ChecklistType,
		IsDefault:     req.IsDefault,
		IsActive:      true,
		CreatedBy:     userID,
	}
	if err := s.repo.CreateTemplateWithItems(ctx, t, items); err != nil {
		return nil, fmt.Errorf("checklists: CreateTemplate: %w", err)
	}
	return &TemplateWithItems{Template: t, Items: items}, nil
}

func (s *serviceImpl) UpdateTemplate(ctx context.Context, orgID, templateID string, req UpdateTemplateRequest) (*Template, error) {
	t, err := s.repo.FindTemplateByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("checklists: UpdateTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		t.Name = name
	}
	if req.Description != nil {
		t.Description = req.Description
	}
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}

	wasDefault := t.IsDefault
	promoteToDefault := req.IsDefault != nil && *req.IsDefault && !wasDefault
	if req.IsDefault != nil && !*req.IsDefault {
		// Clearing a default never conflicts with the partial unique index —
		// safe as a plain update, no atomic clear-then-set needed.
		t.IsDefault = false
	}

	if err := s.repo.UpdateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("checklists: UpdateTemplate: %w", err)
	}

	if promoteToDefault {
		// SetTemplateDefault clears any existing default for this
		// org+checklist_type atomically, THEN sets this one — the approvals
		// engine's repository does not do this and a second is_default
		// raises a raw 23505 that would 500. Done as a second step, after
		// the plain field update above, specifically to avoid attempting
		// is_default=TRUE via the non-atomic UpdateTemplate path.
		if err := s.repo.SetTemplateDefault(ctx, orgID, t.ID, t.ChecklistType); err != nil {
			return nil, fmt.Errorf("checklists: UpdateTemplate: set default: %w", err)
		}
		t.IsDefault = true
	}

	return t, nil
}

func (s *serviceImpl) DeleteTemplate(ctx context.Context, orgID, templateID string) error {
	if err := s.repo.DeleteTemplate(ctx, orgID, templateID); err != nil {
		return fmt.Errorf("checklists: DeleteTemplate: %w", err)
	}
	return nil
}

// ============================================================
// Template items
// ============================================================

// buildTemplateItem validates a CreateTemplateItemRequest and constructs a
// TemplateItem ready for insert. owner_role validation calls RoleExists —
// a typo here would otherwise silently produce an item nobody can ever claim.
func (s *serviceImpl) buildTemplateItem(ctx context.Context, orgID string, req CreateTemplateItemRequest) (*TemplateItem, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	if !req.OwnerType.IsValid() {
		return nil, ErrInvalidOwnerType
	}

	var ownerRole *string
	if req.OwnerType == OwnerTypeRole {
		role := ""
		if req.OwnerRole != nil {
			role = strings.TrimSpace(*req.OwnerRole)
		}
		if role == "" {
			return nil, ErrOwnerRoleRequired
		}
		exists, err := s.directory.RoleExists(ctx, orgID, role)
		if err != nil {
			return nil, fmt.Errorf("checklists: buildTemplateItem: role check: %w", err)
		}
		if !exists {
			return nil, ErrUnknownRole
		}
		ownerRole = &role
	}

	var ownerUserID *string
	if req.OwnerType == OwnerTypeSpecificUser {
		if req.OwnerUserID == nil || strings.TrimSpace(*req.OwnerUserID) == "" {
			return nil, ErrOwnerUserRequired
		}
		uid := strings.TrimSpace(*req.OwnerUserID)
		ownerUserID = &uid
	}

	return &TemplateItem{
		Title:              title,
		Description:        req.Description,
		OwnerType:          req.OwnerType,
		OwnerRole:          ownerRole,
		OwnerUserID:        ownerUserID,
		DueOffsetDays:      req.DueOffsetDays,
		IsBlocking:         req.IsBlocking,
		RequiresAttachment: req.RequiresAttachment,
		DisplayOrder:       req.DisplayOrder,
		IsActive:           true,
	}, nil
}

func (s *serviceImpl) ListTemplateItems(ctx context.Context, orgID, templateID string) ([]*TemplateItem, error) {
	t, err := s.repo.FindTemplateByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("checklists: ListTemplateItems: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	items, err := s.repo.FindTemplateItems(ctx, orgID, t.ID)
	if err != nil {
		return nil, fmt.Errorf("checklists: ListTemplateItems: %w", err)
	}
	if items == nil {
		items = []*TemplateItem{}
	}
	return items, nil
}

func (s *serviceImpl) CreateTemplateItem(ctx context.Context, orgID, templateID string, req CreateTemplateItemRequest) (*TemplateItem, error) {
	t, err := s.repo.FindTemplateByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("checklists: CreateTemplateItem: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	item, err := s.buildTemplateItem(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	item.TemplateID = t.ID
	if err := s.repo.CreateTemplateItem(ctx, item); err != nil {
		return nil, fmt.Errorf("checklists: CreateTemplateItem: %w", err)
	}
	return item, nil
}

func (s *serviceImpl) UpdateTemplateItem(ctx context.Context, orgID, templateID, itemID string, req UpdateTemplateItemRequest) (*TemplateItem, error) {
	item, err := s.repo.FindTemplateItemByID(ctx, orgID, templateID, itemID)
	if err != nil {
		return nil, fmt.Errorf("checklists: UpdateTemplateItem: %w", err)
	}
	if item == nil {
		return nil, ErrTemplateItemNotFound
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		item.Title = title
	}
	if req.Description != nil {
		item.Description = req.Description
	}
	if req.OwnerType != nil {
		if !req.OwnerType.IsValid() {
			return nil, ErrInvalidOwnerType
		}
		item.OwnerType = *req.OwnerType
	}
	if req.OwnerRole != nil {
		item.OwnerRole = req.OwnerRole
	}
	if req.OwnerUserID != nil {
		item.OwnerUserID = req.OwnerUserID
	}

	// Re-validate the owner_type/owner_role/owner_user_id triple as a whole
	// whenever any of the three changed — a partial update could otherwise
	// leave owner_type='role' with a stale or absent owner_role.
	switch item.OwnerType {
	case OwnerTypeRole:
		if item.OwnerRole == nil || strings.TrimSpace(*item.OwnerRole) == "" {
			return nil, ErrOwnerRoleRequired
		}
		role := strings.TrimSpace(*item.OwnerRole)
		exists, err := s.directory.RoleExists(ctx, orgID, role)
		if err != nil {
			return nil, fmt.Errorf("checklists: UpdateTemplateItem: role check: %w", err)
		}
		if !exists {
			return nil, ErrUnknownRole
		}
		item.OwnerRole = &role
	case OwnerTypeSpecificUser:
		if item.OwnerUserID == nil || strings.TrimSpace(*item.OwnerUserID) == "" {
			return nil, ErrOwnerUserRequired
		}
	}

	if req.DueOffsetDays != nil {
		item.DueOffsetDays = *req.DueOffsetDays
	}
	if req.IsBlocking != nil {
		item.IsBlocking = *req.IsBlocking
	}
	if req.RequiresAttachment != nil {
		item.RequiresAttachment = *req.RequiresAttachment
	}
	if req.DisplayOrder != nil {
		item.DisplayOrder = *req.DisplayOrder
	}
	if req.IsActive != nil {
		item.IsActive = *req.IsActive
	}

	if err := s.repo.UpdateTemplateItem(ctx, item); err != nil {
		return nil, fmt.Errorf("checklists: UpdateTemplateItem: %w", err)
	}
	return item, nil
}

func (s *serviceImpl) DeleteTemplateItem(ctx context.Context, orgID, templateID, itemID string) error {
	if err := s.repo.DeleteTemplateItem(ctx, orgID, templateID, itemID); err != nil {
		return fmt.Errorf("checklists: DeleteTemplateItem: %w", err)
	}
	return nil
}

// ============================================================
// Instantiation
// ============================================================

// resolveAssignee never errors on a resolution miss — a miss is a normal
// outcome (no manager, deleted user, no platform account), not a failure.
// Errors are reserved for infrastructure failures elsewhere in this file.
func resolveAssignee(ownerType OwnerType, ownerUserID *string, subj SubjectContext) *string {
	switch ownerType {
	case OwnerTypeSubject:
		return subj.SubjectUserID
	case OwnerTypeManager:
		return subj.ManagerUserID
	case OwnerTypeSpecificUser:
		return ownerUserID
	case OwnerTypeRole:
		// Always nil — group claim per decision 2. No role→users query runs
		// at instantiation; any role holder can claim it via /items/mine.
		return nil
	}
	return nil
}

// computeDueDate anchors at UTC midnight before adding the offset —
// AddDate, never raw duration arithmetic, so this is DST-safe. Negative
// offsets are valid (pre-boarding items) and are not clamped.
func computeDueDate(anchor time.Time, offsetDays int) time.Time {
	y, m, d := anchor.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, offsetDays)
}

func (s *serviceImpl) Instantiate(ctx context.Context, orgID, templateID string, subj SubjectContext) (*InstantiateResult, error) {
	if !subj.SubjectType.IsValid() {
		return nil, ErrInvalidSubjectType
	}

	t, err := s.repo.FindTemplateByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("checklists: Instantiate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	if !t.IsActive {
		return nil, ErrTemplateInactive
	}

	activeCount, err := s.repo.FindActiveTemplateItemCount(ctx, orgID, t.ID)
	if err != nil {
		return nil, fmt.Errorf("checklists: Instantiate: count items: %w", err)
	}
	if activeCount == 0 {
		return nil, ErrTemplateHasNoItems
	}

	tplItems, err := s.repo.FindTemplateItems(ctx, orgID, t.ID)
	if err != nil {
		return nil, fmt.Errorf("checklists: Instantiate: items: %w", err)
	}

	inst := &Instance{
		OrgID:         orgID,
		TemplateID:    &t.ID,
		TemplateName:  t.Name,
		ChecklistType: t.ChecklistType,
		SubjectType:   subj.SubjectType,
		SubjectID:     subj.SubjectID,
		SubjectLabel:  subj.SubjectLabel,
		SubjectUserID: subj.SubjectUserID,
		AnchorDate:    subj.AnchorDate,
		Status:        InstanceStatusInProgress,
		CreatedBy:     subj.CreatedBy,
	}

	items := make([]*InstanceItem, 0, activeCount)
	unresolved := 0
	for _, ti := range tplItems {
		if !ti.IsActive {
			continue
		}
		assignee := resolveAssignee(ti.OwnerType, ti.OwnerUserID, subj)
		if assignee == nil && ti.OwnerType != OwnerTypeRole {
			unresolved++
		}
		due := computeDueDate(subj.AnchorDate, ti.DueOffsetDays)
		items = append(items, &InstanceItem{
			TemplateItemID:     &ti.ID,
			Title:              ti.Title,
			Description:        ti.Description,
			OwnerType:          ti.OwnerType,
			OwnerRole:          ti.OwnerRole,
			IsBlocking:         ti.IsBlocking,
			RequiresAttachment: ti.RequiresAttachment,
			DisplayOrder:       ti.DisplayOrder,
			DueOffsetDays:      ti.DueOffsetDays,
			AssigneeUserID:     assignee,
			DueDate:            &due,
			Status:             ItemStatusPending,
		})
	}

	if err := s.repo.InsertInstanceWithItems(ctx, inst, items); err != nil {
		return nil, fmt.Errorf("checklists: Instantiate: %w", err)
	}

	return &InstantiateResult{Instance: inst, Items: items, UnresolvedCount: unresolved}, nil
}

func (s *serviceImpl) InstantiateDefault(ctx context.Context, orgID string, checklistType ChecklistType, subj SubjectContext) (*InstantiateResult, error) {
	t, err := s.repo.FindDefaultTemplate(ctx, orgID, checklistType)
	if err != nil {
		return nil, fmt.Errorf("checklists: InstantiateDefault: %w", err)
	}
	if t == nil {
		return nil, nil
	}
	return s.Instantiate(ctx, orgID, t.ID, subj)
}

// ============================================================
// Instances
// ============================================================

func (s *serviceImpl) ListInstances(ctx context.Context, orgID string, f InstanceFilter) (*InstanceListResponse, error) {
	if f.Limit <= 0 {
		f.Limit = pagination.DefaultLimit
	}
	if f.Limit > pagination.MaxLimit {
		f.Limit = pagination.MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	instances, err := s.repo.FindInstances(ctx, orgID, f)
	if err != nil {
		return nil, fmt.Errorf("checklists: ListInstances: %w", err)
	}
	total, err := s.repo.CountInstances(ctx, orgID, f)
	if err != nil {
		return nil, fmt.Errorf("checklists: ListInstances: count: %w", err)
	}

	ids := make([]string, len(instances))
	for i, inst := range instances {
		ids[i] = inst.ID
	}
	progressByID, err := s.repo.GetProgressBatch(ctx, orgID, ids)
	if err != nil {
		return nil, fmt.Errorf("checklists: ListInstances: progress: %w", err)
	}

	out := make([]*InstanceListItem, len(instances))
	for i, inst := range instances {
		out[i] = &InstanceListItem{Instance: inst, Progress: progressByID[inst.ID]}
	}

	return &InstanceListResponse{
		Instances: out,
		Meta:      pagination.Meta{Total: total, Limit: f.Limit, Offset: f.Offset},
	}, nil
}

func (s *serviceImpl) GetInstance(ctx context.Context, orgID, instanceID string) (*InstanceWithItems, error) {
	inst, err := s.repo.FindInstanceByID(ctx, orgID, instanceID)
	if err != nil {
		return nil, fmt.Errorf("checklists: GetInstance: %w", err)
	}
	if inst == nil {
		return nil, ErrInstanceNotFound
	}
	items, err := s.repo.FindInstanceItems(ctx, orgID, inst.ID)
	if err != nil {
		return nil, fmt.Errorf("checklists: GetInstance: items: %w", err)
	}
	if items == nil {
		items = []*InstanceItem{}
	}
	markUnassigned(items)

	progress, err := s.repo.GetProgress(ctx, orgID, inst.ID)
	if err != nil {
		return nil, fmt.Errorf("checklists: GetInstance: progress: %w", err)
	}

	return &InstanceWithItems{Instance: inst, Items: items, Progress: progress}, nil
}

// markUnassigned sets the derived IsUnassigned flag — true only when
// resolution genuinely found nobody. A role-owned item is a group claim,
// not "unassigned", even though assignee_user_id is also NULL for it.
func markUnassigned(items []*InstanceItem) {
	for _, it := range items {
		it.IsUnassigned = it.AssigneeUserID == nil && it.OwnerType != OwnerTypeRole
	}
}

func (s *serviceImpl) CancelInstance(ctx context.Context, orgID, instanceID string, req CancelInstanceRequest) (*Instance, error) {
	inst, err := s.repo.FindInstanceByID(ctx, orgID, instanceID)
	if err != nil {
		return nil, fmt.Errorf("checklists: CancelInstance: %w", err)
	}
	if inst == nil {
		return nil, ErrInstanceNotFound
	}
	if inst.Status == InstanceStatusCancelled {
		return nil, ErrInstanceAlreadyClosed
	}

	reason := strings.TrimSpace(req.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	if err := s.repo.UpdateInstanceStatus(ctx, orgID, inst.ID, InstanceStatusCancelled, reasonPtr); err != nil {
		return nil, fmt.Errorf("checklists: CancelInstance: %w", err)
	}

	inst.Status = InstanceStatusCancelled
	inst.CancelReason = reasonPtr
	return inst, nil
}

// ============================================================
// Items
// ============================================================

func (s *serviceImpl) ListMyItems(ctx context.Context, orgID, userID string, status *ItemStatus) ([]*InstanceItem, error) {
	items, err := s.repo.FindInstanceItemsByAssignee(ctx, orgID, userID, status)
	if err != nil {
		return nil, fmt.Errorf("checklists: ListMyItems: %w", err)
	}
	if items == nil {
		items = []*InstanceItem{}
	}
	return items, nil
}

// authorizeItemAction implements the completion-authorization narrowing the
// route gate cannot express: assignee may act, or — for a role-owned group
// claim — any holder of that role, or a caller with .manage as the fallback.
func (s *serviceImpl) authorizeItemAction(ctx context.Context, orgID, callerUserID string, item *InstanceItem) (bool, error) {
	if item.AssigneeUserID != nil && *item.AssigneeUserID == callerUserID {
		return true, nil
	}
	if item.OwnerType == OwnerTypeRole && item.OwnerRole != nil {
		roleName, err := s.directory.UserRoleName(ctx, orgID, callerUserID)
		if err != nil {
			return false, fmt.Errorf("authorizeItemAction: role lookup: %w", err)
		}
		if roleName != "" && strings.EqualFold(roleName, *item.OwnerRole) {
			return true, nil
		}
	}
	canManage, err := s.directory.Can(ctx, callerUserID, orgID, "platform.checklists", "manage")
	if err != nil {
		return false, fmt.Errorf("authorizeItemAction: manage check: %w", err)
	}
	return canManage, nil
}

func (s *serviceImpl) CompleteItem(ctx context.Context, orgID, itemID, callerUserID string, req CompleteItemRequest) (*InstanceItem, error) {
	item, err := s.repo.FindInstanceItemByID(ctx, orgID, itemID)
	if err != nil {
		return nil, fmt.Errorf("checklists: CompleteItem: %w", err)
	}
	if item == nil {
		return nil, ErrInstanceItemNotFound
	}
	if item.Status.IsTerminal() {
		return nil, ErrItemAlreadyTerminal
	}
	if item.RequiresAttachment && (req.AttachmentURL == nil || strings.TrimSpace(*req.AttachmentURL) == "") {
		return nil, ErrAttachmentRequired
	}

	authorized, err := s.authorizeItemAction(ctx, orgID, callerUserID, item)
	if err != nil {
		return nil, fmt.Errorf("checklists: CompleteItem: %w", err)
	}
	if !authorized {
		return nil, ErrNotItemOwner
	}

	now := time.Now()
	item.Status = ItemStatusCompleted
	item.CompletedBy = &callerUserID
	item.CompletedAt = &now
	item.CompletionNote = req.CompletionNote
	item.AttachmentURL = req.AttachmentURL
	item.SkipReason = nil

	if err := s.repo.UpdateInstanceItem(ctx, item); err != nil {
		return nil, fmt.Errorf("checklists: CompleteItem: %w", err)
	}
	if err := s.syncInstanceStatus(ctx, orgID, item.InstanceID); err != nil {
		return nil, fmt.Errorf("checklists: CompleteItem: sync instance: %w", err)
	}
	return item, nil
}

func (s *serviceImpl) ReopenItem(ctx context.Context, orgID, itemID, callerUserID string) (*InstanceItem, error) {
	item, err := s.repo.FindInstanceItemByID(ctx, orgID, itemID)
	if err != nil {
		return nil, fmt.Errorf("checklists: ReopenItem: %w", err)
	}
	if item == nil {
		return nil, ErrInstanceItemNotFound
	}
	if !item.Status.IsTerminal() {
		return nil, ErrItemNotTerminal
	}

	authorized, err := s.authorizeItemAction(ctx, orgID, callerUserID, item)
	if err != nil {
		return nil, fmt.Errorf("checklists: ReopenItem: %w", err)
	}
	if !authorized {
		return nil, ErrNotItemOwner
	}

	item.Status = ItemStatusPending
	item.CompletedBy = nil
	item.CompletedAt = nil
	item.CompletionNote = nil
	item.AttachmentURL = nil
	item.SkipReason = nil

	if err := s.repo.UpdateInstanceItem(ctx, item); err != nil {
		return nil, fmt.Errorf("checklists: ReopenItem: %w", err)
	}
	if err := s.syncInstanceStatus(ctx, orgID, item.InstanceID); err != nil {
		return nil, fmt.Errorf("checklists: ReopenItem: sync instance: %w", err)
	}
	return item, nil
}

func (s *serviceImpl) SkipItem(ctx context.Context, orgID, itemID, callerUserID string, req SkipItemRequest) (*InstanceItem, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, ErrSkipReasonRequired
	}

	item, err := s.repo.FindInstanceItemByID(ctx, orgID, itemID)
	if err != nil {
		return nil, fmt.Errorf("checklists: SkipItem: %w", err)
	}
	if item == nil {
		return nil, ErrInstanceItemNotFound
	}
	if item.Status.IsTerminal() {
		return nil, ErrItemAlreadyTerminal
	}
	// Skip is gated by platform.checklists.manage at the route level — no
	// per-item ownership narrowing here, unlike complete/reopen.
	_ = callerUserID

	item.Status = ItemStatusSkipped
	item.SkipReason = &reason

	if err := s.repo.UpdateInstanceItem(ctx, item); err != nil {
		return nil, fmt.Errorf("checklists: SkipItem: %w", err)
	}
	if err := s.syncInstanceStatus(ctx, orgID, item.InstanceID); err != nil {
		return nil, fmt.Errorf("checklists: SkipItem: sync instance: %w", err)
	}
	return item, nil
}

// syncInstanceStatus auto-transitions the instance status after any item
// mutation: all items terminal -> completed; reopening the last terminal
// item flips a completed instance back to in_progress. A cancelled instance
// is never resurrected by this — cancellation is a one-way door.
func (s *serviceImpl) syncInstanceStatus(ctx context.Context, orgID, instanceID string) error {
	inst, err := s.repo.FindInstanceByID(ctx, orgID, instanceID)
	if err != nil {
		return err
	}
	if inst == nil || inst.Status == InstanceStatusCancelled {
		return nil
	}

	progress, err := s.repo.GetProgress(ctx, orgID, instanceID)
	if err != nil {
		return err
	}
	if progress == nil {
		return nil
	}

	allTerminal := progress.TotalItems > 0 && progress.PendingItems == 0
	switch {
	case allTerminal && inst.Status != InstanceStatusCompleted:
		return s.repo.UpdateInstanceStatus(ctx, orgID, instanceID, InstanceStatusCompleted, nil)
	case !allTerminal && inst.Status == InstanceStatusCompleted:
		return s.repo.UpdateInstanceStatus(ctx, orgID, instanceID, InstanceStatusInProgress, nil)
	}
	return nil
}
