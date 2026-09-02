// backend/internal/tests/unit/platform/checklists_service_test.go
// Platform checklist engine service unit tests — no DB. Black-box against
// the exported Service, matching contacts_service_test.go's convention
// (package platform, not platform_test) since checklists' internals
// (resolveAssignee, computeDueDate) are unexported.
package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/platform/checklists"
)

// ── Stub checklist repository ───────────────────────────────────────────────

type stubChecklistRepo struct {
	seq           int
	templates     map[string]*checklists.Template
	templateItems map[string]*checklists.TemplateItem
	instances     map[string]*checklists.Instance
	instanceItems map[string]*checklists.InstanceItem
}

func newStubChecklistRepo() *stubChecklistRepo {
	return &stubChecklistRepo{
		templates:     map[string]*checklists.Template{},
		templateItems: map[string]*checklists.TemplateItem{},
		instances:     map[string]*checklists.Instance{},
		instanceItems: map[string]*checklists.InstanceItem{},
	}
}

func (r *stubChecklistRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d", prefix, r.seq)
}

func (r *stubChecklistRepo) FindTemplates(_ context.Context, orgID string, checklistType *checklists.ChecklistType) ([]*checklists.Template, error) {
	var out []*checklists.Template
	for _, t := range r.templates {
		if t.OrgID != orgID {
			continue
		}
		if checklistType != nil && t.ChecklistType != *checklistType {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *stubChecklistRepo) FindTemplateByID(_ context.Context, orgID, templateID string) (*checklists.Template, error) {
	t := r.templates[templateID]
	if t == nil || t.OrgID != orgID {
		return nil, nil
	}
	return t, nil
}

func (r *stubChecklistRepo) FindDefaultTemplate(_ context.Context, orgID string, checklistType checklists.ChecklistType) (*checklists.Template, error) {
	for _, t := range r.templates {
		if t.OrgID == orgID && t.ChecklistType == checklistType && t.IsDefault && t.IsActive {
			return t, nil
		}
	}
	return nil, nil
}

func (r *stubChecklistRepo) CreateTemplateWithItems(_ context.Context, t *checklists.Template, items []*checklists.TemplateItem) error {
	if t.IsDefault {
		for _, existing := range r.templates {
			if existing.OrgID == t.OrgID && existing.ChecklistType == t.ChecklistType {
				existing.IsDefault = false
			}
		}
	}
	t.ID = r.nextID("tpl")
	t.PublicID = "pub_" + t.ID
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	r.templates[t.ID] = t

	for _, item := range items {
		item.TemplateID = t.ID
		item.ID = r.nextID("tplitem")
		item.PublicID = "pub_" + item.ID
		item.CreatedAt = time.Now()
		item.UpdatedAt = time.Now()
		r.templateItems[item.ID] = item
	}
	return nil
}

func (r *stubChecklistRepo) UpdateTemplate(_ context.Context, t *checklists.Template) error {
	if _, ok := r.templates[t.ID]; !ok {
		return checklists.ErrTemplateNotFound
	}
	t.UpdatedAt = time.Now()
	r.templates[t.ID] = t
	return nil
}

func (r *stubChecklistRepo) DeleteTemplate(_ context.Context, orgID, templateID string) error {
	t := r.templates[templateID]
	if t == nil || t.OrgID != orgID {
		return checklists.ErrTemplateNotFound
	}
	delete(r.templates, templateID)
	return nil
}

func (r *stubChecklistRepo) SetTemplateDefault(_ context.Context, orgID, templateID string, checklistType checklists.ChecklistType) error {
	target := r.templates[templateID]
	if target == nil || target.OrgID != orgID {
		return checklists.ErrTemplateNotFound
	}
	for _, existing := range r.templates {
		if existing.OrgID == orgID && existing.ChecklistType == checklistType {
			existing.IsDefault = false
		}
	}
	target.IsDefault = true
	return nil
}

func (r *stubChecklistRepo) FindTemplateItems(_ context.Context, orgID, templateID string) ([]*checklists.TemplateItem, error) {
	t := r.templates[templateID]
	if t == nil || t.OrgID != orgID {
		return nil, nil
	}
	var out []*checklists.TemplateItem
	for _, it := range r.templateItems {
		if it.TemplateID == templateID {
			out = append(out, it)
		}
	}
	return out, nil
}

func (r *stubChecklistRepo) FindActiveTemplateItemCount(_ context.Context, orgID, templateID string) (int, error) {
	t := r.templates[templateID]
	if t == nil || t.OrgID != orgID {
		return 0, nil
	}
	n := 0
	for _, it := range r.templateItems {
		if it.TemplateID == templateID && it.IsActive {
			n++
		}
	}
	return n, nil
}

func (r *stubChecklistRepo) FindTemplateItemByID(_ context.Context, orgID, templateID, itemID string) (*checklists.TemplateItem, error) {
	t := r.templates[templateID]
	if t == nil || t.OrgID != orgID {
		return nil, nil
	}
	it := r.templateItems[itemID]
	if it == nil || it.TemplateID != templateID {
		return nil, nil
	}
	return it, nil
}

func (r *stubChecklistRepo) CreateTemplateItem(_ context.Context, item *checklists.TemplateItem) error {
	item.ID = r.nextID("tplitem")
	item.PublicID = "pub_" + item.ID
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	r.templateItems[item.ID] = item
	return nil
}

func (r *stubChecklistRepo) UpdateTemplateItem(_ context.Context, item *checklists.TemplateItem) error {
	if _, ok := r.templateItems[item.ID]; !ok {
		return checklists.ErrTemplateItemNotFound
	}
	item.UpdatedAt = time.Now()
	r.templateItems[item.ID] = item
	return nil
}

func (r *stubChecklistRepo) DeleteTemplateItem(_ context.Context, orgID, templateID, itemID string) error {
	it := r.templateItems[itemID]
	if it == nil || it.TemplateID != templateID {
		return checklists.ErrTemplateItemNotFound
	}
	delete(r.templateItems, itemID)
	return nil
}

func (r *stubChecklistRepo) InsertInstanceWithItems(_ context.Context, inst *checklists.Instance, items []*checklists.InstanceItem) error {
	inst.ID = r.nextID("inst")
	inst.PublicID = "pub_" + inst.ID
	inst.CreatedAt = time.Now()
	inst.UpdatedAt = time.Now()
	r.instances[inst.ID] = inst

	for _, item := range items {
		item.InstanceID = inst.ID
		item.ID = r.nextID("institem")
		item.PublicID = "pub_" + item.ID
		item.CreatedAt = time.Now()
		item.UpdatedAt = time.Now()
		r.instanceItems[item.ID] = item
	}
	return nil
}

func (r *stubChecklistRepo) FindInstances(_ context.Context, orgID string, f checklists.InstanceFilter) ([]*checklists.Instance, error) {
	var out []*checklists.Instance
	for _, inst := range r.instances {
		if inst.OrgID != orgID {
			continue
		}
		if f.SubjectType != nil && inst.SubjectType != *f.SubjectType {
			continue
		}
		if f.SubjectID != nil && inst.SubjectID != *f.SubjectID {
			continue
		}
		if f.Status != nil && inst.Status != *f.Status {
			continue
		}
		out = append(out, inst)
	}
	return out, nil
}

func (r *stubChecklistRepo) CountInstances(ctx context.Context, orgID string, f checklists.InstanceFilter) (int, error) {
	list, _ := r.FindInstances(ctx, orgID, f)
	return len(list), nil
}

func (r *stubChecklistRepo) FindInstanceByID(_ context.Context, orgID, instanceID string) (*checklists.Instance, error) {
	inst := r.instances[instanceID]
	if inst == nil || inst.OrgID != orgID {
		return nil, nil
	}
	return inst, nil
}

func (r *stubChecklistRepo) UpdateInstanceStatus(_ context.Context, orgID, instanceID string, status checklists.InstanceStatus, cancelReason *string) error {
	inst := r.instances[instanceID]
	if inst == nil || inst.OrgID != orgID {
		return checklists.ErrInstanceNotFound
	}
	inst.Status = status
	if status == checklists.InstanceStatusCancelled {
		inst.CancelReason = cancelReason
	}
	return nil
}

func (r *stubChecklistRepo) FindInstanceItems(_ context.Context, orgID, instanceID string) ([]*checklists.InstanceItem, error) {
	inst := r.instances[instanceID]
	if inst == nil || inst.OrgID != orgID {
		return nil, nil
	}
	var out []*checklists.InstanceItem
	for _, it := range r.instanceItems {
		if it.InstanceID == instanceID {
			out = append(out, it)
		}
	}
	return out, nil
}

func (r *stubChecklistRepo) FindInstanceItemByID(_ context.Context, orgID, itemID string) (*checklists.InstanceItem, error) {
	it := r.instanceItems[itemID]
	if it == nil {
		return nil, nil
	}
	inst := r.instances[it.InstanceID]
	if inst == nil || inst.OrgID != orgID {
		return nil, nil
	}
	return it, nil
}

func (r *stubChecklistRepo) FindInstanceItemsByAssignee(_ context.Context, orgID, userID string, status *checklists.ItemStatus) ([]*checklists.InstanceItem, error) {
	var out []*checklists.InstanceItem
	for _, it := range r.instanceItems {
		if it.AssigneeUserID == nil || *it.AssigneeUserID != userID {
			continue
		}
		inst := r.instances[it.InstanceID]
		if inst == nil || inst.OrgID != orgID {
			continue
		}
		if status != nil && it.Status != *status {
			continue
		}
		out = append(out, it)
	}
	return out, nil
}

func (r *stubChecklistRepo) UpdateInstanceItem(_ context.Context, item *checklists.InstanceItem) error {
	if _, ok := r.instanceItems[item.ID]; !ok {
		return checklists.ErrInstanceItemNotFound
	}
	item.UpdatedAt = time.Now()
	r.instanceItems[item.ID] = item
	return nil
}

func (r *stubChecklistRepo) computeProgress(instanceID string) *checklists.Progress {
	p := &checklists.Progress{InstanceID: instanceID}
	for _, it := range r.instanceItems {
		if it.InstanceID != instanceID {
			continue
		}
		p.TotalItems++
		switch it.Status {
		case checklists.ItemStatusCompleted:
			p.CompletedItems++
		case checklists.ItemStatusSkipped:
			p.SkippedItems++
		case checklists.ItemStatusPending:
			p.PendingItems++
			if it.IsBlocking {
				p.BlockingOpen++
			}
		}
	}
	if p.TotalItems > 0 {
		p.PercentDone = ((p.CompletedItems + p.SkippedItems) * 100) / p.TotalItems
	}
	return p
}

func (r *stubChecklistRepo) GetProgress(_ context.Context, orgID, instanceID string) (*checklists.Progress, error) {
	inst := r.instances[instanceID]
	if inst == nil || inst.OrgID != orgID {
		return nil, nil
	}
	return r.computeProgress(instanceID), nil
}

func (r *stubChecklistRepo) GetProgressBatch(_ context.Context, orgID string, instanceIDs []string) (map[string]*checklists.Progress, error) {
	out := map[string]*checklists.Progress{}
	for _, id := range instanceIDs {
		inst := r.instances[id]
		if inst == nil || inst.OrgID != orgID {
			continue
		}
		out[id] = r.computeProgress(id)
	}
	return out, nil
}

var _ checklists.Repository = (*stubChecklistRepo)(nil)

// ── Stub access directory ───────────────────────────────────────────────────

type stubAccessDirectory struct {
	knownRoles  map[string]bool   // lowercase role name -> exists
	userRoles   map[string]string // userID -> role name
	manageUsers map[string]bool   // userID -> holds platform.checklists.manage
}

func newStubAccessDirectory() *stubAccessDirectory {
	return &stubAccessDirectory{
		knownRoles:  map[string]bool{},
		userRoles:   map[string]string{},
		manageUsers: map[string]bool{},
	}
}

func (d *stubAccessDirectory) Can(_ context.Context, userID, _, _, action string) (bool, error) {
	if action == "manage" {
		return d.manageUsers[userID], nil
	}
	return false, nil
}

func (d *stubAccessDirectory) UserRoleName(_ context.Context, _, userID string) (string, error) {
	return d.userRoles[userID], nil
}

func (d *stubAccessDirectory) RoleExists(_ context.Context, _, roleName string) (bool, error) {
	return d.knownRoles[strings.ToLower(roleName)], nil
}

var _ checklists.AccessDirectory = (*stubAccessDirectory)(nil)

// ── Test helpers ─────────────────────────────────────────────────────────────

func newChecklistTestSvc() (checklists.Service, *stubChecklistRepo, *stubAccessDirectory) {
	repo := newStubChecklistRepo()
	dir := newStubAccessDirectory()
	return checklists.NewService(repo, dir), repo, dir
}

func strp(s string) *string { return &s }

const testOrg = "org_1"

// ============================================================
// Template validation
// ============================================================

func TestCreateTemplate_RequiresName(t *testing.T) {
	svc, _, _ := newChecklistTestSvc()
	_, err := svc.CreateTemplate(context.Background(), testOrg, "user_1", checklists.CreateTemplateRequest{
		Name: "  ", ChecklistType: checklists.ChecklistTypeOnboarding,
	})
	if !errors.Is(err, checklists.ErrNameRequired) {
		t.Fatalf("expected ErrNameRequired, got %v", err)
	}
}

func TestCreateTemplate_RejectsInvalidChecklistType(t *testing.T) {
	svc, _, _ := newChecklistTestSvc()
	_, err := svc.CreateTemplate(context.Background(), testOrg, "user_1", checklists.CreateTemplateRequest{
		Name: "Onboarding", ChecklistType: "not-a-real-type",
	})
	if !errors.Is(err, checklists.ErrInvalidChecklistType) {
		t.Fatalf("expected ErrInvalidChecklistType, got %v", err)
	}
}

func TestCreateTemplate_OwnerTypeRole_RequiresOwnerRole(t *testing.T) {
	svc, _, _ := newChecklistTestSvc()
	_, err := svc.CreateTemplate(context.Background(), testOrg, "user_1", checklists.CreateTemplateRequest{
		Name: "Onboarding", ChecklistType: checklists.ChecklistTypeOnboarding,
		Items: []checklists.CreateTemplateItemRequest{{Title: "Setup badge", OwnerType: checklists.OwnerTypeRole}},
	})
	if !errors.Is(err, checklists.ErrOwnerRoleRequired) {
		t.Fatalf("expected ErrOwnerRoleRequired, got %v", err)
	}
}

func TestCreateTemplate_OwnerTypeRole_RejectsUnknownRole(t *testing.T) {
	svc, _, _ := newChecklistTestSvc()
	_, err := svc.CreateTemplate(context.Background(), testOrg, "user_1", checklists.CreateTemplateRequest{
		Name: "Onboarding", ChecklistType: checklists.ChecklistTypeOnboarding,
		Items: []checklists.CreateTemplateItemRequest{{Title: "Setup badge", OwnerType: checklists.OwnerTypeRole, OwnerRole: strp("ghost-role")}},
	})
	if !errors.Is(err, checklists.ErrUnknownRole) {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}

func TestCreateTemplate_OwnerTypeSpecificUser_RequiresOwnerUserID(t *testing.T) {
	svc, _, _ := newChecklistTestSvc()
	_, err := svc.CreateTemplate(context.Background(), testOrg, "user_1", checklists.CreateTemplateRequest{
		Name: "Onboarding", ChecklistType: checklists.ChecklistTypeOnboarding,
		Items: []checklists.CreateTemplateItemRequest{{Title: "IT setup", OwnerType: checklists.OwnerTypeSpecificUser}},
	})
	if !errors.Is(err, checklists.ErrOwnerUserRequired) {
		t.Fatalf("expected ErrOwnerUserRequired, got %v", err)
	}
}

// ============================================================
// Instantiate — resolveAssignee matrix (indirect, via the exported Service)
// ============================================================

func mustCreateTemplate(t *testing.T, svc checklists.Service, dir *stubAccessDirectory, items []checklists.CreateTemplateItemRequest) *checklists.TemplateWithItems {
	t.Helper()
	dir.knownRoles["hr"] = true
	tpl, err := svc.CreateTemplate(context.Background(), testOrg, "creator_1", checklists.CreateTemplateRequest{
		Name: "Onboarding", ChecklistType: checklists.ChecklistTypeOnboarding, Items: items,
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	return tpl
}

func TestInstantiate_ResolvesAssigneeMatrix(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Subject item", OwnerType: checklists.OwnerTypeSubject},
		{Title: "Manager item", OwnerType: checklists.OwnerTypeManager},
		{Title: "Role item", OwnerType: checklists.OwnerTypeRole, OwnerRole: strp("hr")},
		{Title: "Specific user item", OwnerType: checklists.OwnerTypeSpecificUser, OwnerUserID: strp("user_zed")},
	})

	subjUser, mgrUser := "user_subject", "user_manager"
	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_1", SubjectLabel: "Jane Doe",
		SubjectUserID: &subjUser, ManagerUserID: &mgrUser, AnchorDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	byTitle := map[string]*checklists.InstanceItem{}
	for _, it := range result.Items {
		byTitle[it.Title] = it
	}

	if got := byTitle["Subject item"].AssigneeUserID; got == nil || *got != subjUser {
		t.Errorf("subject item: expected assignee %q, got %v", subjUser, got)
	}
	if got := byTitle["Manager item"].AssigneeUserID; got == nil || *got != mgrUser {
		t.Errorf("manager item: expected assignee %q, got %v", mgrUser, got)
	}
	if got := byTitle["Role item"].AssigneeUserID; got != nil {
		t.Errorf("role item: expected nil assignee (group claim), got %v", *got)
	}
	if got := byTitle["Role item"].OwnerRole; got == nil || *got != "hr" {
		t.Errorf("role item: expected owner_role 'hr' preserved on the instance item, got %v", got)
	}
	if got := byTitle["Specific user item"].AssigneeUserID; got == nil || *got != "user_zed" {
		t.Errorf("specific_user item: expected assignee 'user_zed', got %v", got)
	}
	if result.UnresolvedCount != 0 {
		t.Errorf("expected zero unresolved items when all inputs present, got %d", result.UnresolvedCount)
	}
}

func TestInstantiate_MissingSubjectOrManager_StillCreatesItem_CountsUnresolved(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Subject item", OwnerType: checklists.OwnerTypeSubject},
		{Title: "Manager item", OwnerType: checklists.OwnerTypeManager},
	})

	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_2", SubjectLabel: "Contractor",
		SubjectUserID: nil, ManagerUserID: nil, AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected both items still created despite unresolved owners, got %d", len(result.Items))
	}
	for _, it := range result.Items {
		if it.AssigneeUserID != nil {
			t.Errorf("expected nil assignee for %q when subject has no platform account/manager, got %v", it.Title, *it.AssigneeUserID)
		}
	}
	if result.UnresolvedCount != 2 {
		t.Errorf("expected UnresolvedCount=2, got %d", result.UnresolvedCount)
	}
}

func TestInstantiate_ZeroActiveItems_ReturnsErrTemplateHasNoItems(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, nil)

	_, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_3", SubjectLabel: "X",
		AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if !errors.Is(err, checklists.ErrTemplateHasNoItems) {
		t.Fatalf("expected ErrTemplateHasNoItems for a template with zero items, got %v", err)
	}
}

func TestInstantiate_DueDateMath_NegativeOffsetCrossesYearBoundary(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Pre-boarding email", OwnerType: checklists.OwnerTypeSubject, DueOffsetDays: -5},
		{Title: "Month rollover", OwnerType: checklists.OwnerTypeSubject, DueOffsetDays: 1},
	})

	anchor := time.Date(2026, 1, 3, 15, 30, 0, 0, time.UTC) // deliberately non-midnight, to prove truncation
	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_4", SubjectLabel: "X",
		AnchorDate: anchor, CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	byTitle := map[string]*checklists.InstanceItem{}
	for _, it := range result.Items {
		byTitle[it.Title] = it
	}

	wantPre := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)
	if got := byTitle["Pre-boarding email"].DueDate; got == nil || !got.Equal(wantPre) {
		t.Errorf("negative offset crossing year boundary: expected %v, got %v", wantPre, got)
	}

	anchor2 := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	result2, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_5", SubjectLabel: "Y",
		AnchorDate: anchor2, CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate 2: %v", err)
	}
	for _, it := range result2.Items {
		if it.Title != "Month rollover" {
			continue
		}
		wantRollover := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		if it.DueDate == nil || !it.DueDate.Equal(wantRollover) {
			t.Errorf("month rollover: expected %v, got %v", wantRollover, it.DueDate)
		}
	}
}

func TestInstantiate_SnapshotIntegrity_TemplateEditDoesNotAffectExistingInstance(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Original title", OwnerType: checklists.OwnerTypeSubject},
	})
	templateItemID := tpl.Items[0].ID

	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_6", SubjectLabel: "X",
		AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	newTitle := "Edited after instantiation"
	if _, err := svc.UpdateTemplateItem(context.Background(), testOrg, tpl.ID, templateItemID, checklists.UpdateTemplateItemRequest{
		Title: &newTitle,
	}); err != nil {
		t.Fatalf("update template item: %v", err)
	}

	inst, err := svc.GetInstance(context.Background(), testOrg, result.Instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if inst.Items[0].Title != "Original title" {
		t.Errorf("expected the instance item snapshot to be unaffected by the later template edit, got %q", inst.Items[0].Title)
	}
}

func TestInstantiateDefault_NoDefaultTemplate_ReturnsNilNilNotError(t *testing.T) {
	svc, _, _ := newChecklistTestSvc()
	result, err := svc.InstantiateDefault(context.Background(), testOrg, checklists.ChecklistTypeOnboarding, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_7", SubjectLabel: "X", AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("expected no error when there is no default template, got %v", err)
	}
	if result != nil {
		t.Errorf("expected a nil result when there is no default template, got %+v", result)
	}
}

func TestInstantiate_DuplicateInstantiation_CreatesTwoIndependentInstances(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Item", OwnerType: checklists.OwnerTypeSubject},
	})
	subj := checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_8", SubjectLabel: "X",
		AnchorDate: time.Now(), CreatedBy: "creator_1",
	}
	r1, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, subj)
	if err != nil {
		t.Fatalf("first instantiate: %v", err)
	}
	r2, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, subj)
	if err != nil {
		t.Fatalf("second instantiate: %v", err)
	}
	if r1.Instance.ID == r2.Instance.ID {
		t.Error("expected two independent instances — duplicate instantiation is not prevented by design")
	}
}

// ============================================================
// Completion, skip, reopen, and instance auto-transition
// ============================================================

func TestCompletion_ZeroItemsToFullPercent_SkippedCountsAsDone(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "A", OwnerType: checklists.OwnerTypeSubject},
		{Title: "B", OwnerType: checklists.OwnerTypeSubject},
	})
	subjUser := "user_subject"
	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_9", SubjectLabel: "X",
		SubjectUserID: &subjUser, AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	itemA, itemB := result.Items[0], result.Items[1]
	if _, err := svc.CompleteItem(context.Background(), testOrg, itemA.ID, subjUser, checklists.CompleteItemRequest{}); err != nil {
		t.Fatalf("complete A: %v", err)
	}
	dir.manageUsers["admin_1"] = true
	if _, err := svc.SkipItem(context.Background(), testOrg, itemB.ID, "admin_1", checklists.SkipItemRequest{Reason: "not applicable"}); err != nil {
		t.Fatalf("skip B: %v", err)
	}

	inst, err := svc.GetInstance(context.Background(), testOrg, result.Instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if inst.Progress.PercentDone != 100 {
		t.Errorf("expected 100%% (completed + skipped both count as done), got %d", inst.Progress.PercentDone)
	}
	if inst.Progress.PendingItems != 0 {
		t.Errorf("expected zero pending items, got %d", inst.Progress.PendingItems)
	}
	if inst.Instance.Status != checklists.InstanceStatusCompleted {
		t.Errorf("expected instance to auto-transition to completed once all items are terminal, got %q", inst.Instance.Status)
	}
}

func TestReopenItem_FlipsCompletedInstanceBackToInProgress(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Only item", OwnerType: checklists.OwnerTypeSubject},
	})
	subjUser := "user_subject"
	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_10", SubjectLabel: "X",
		SubjectUserID: &subjUser, AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	item := result.Items[0]

	if _, err := svc.CompleteItem(context.Background(), testOrg, item.ID, subjUser, checklists.CompleteItemRequest{}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	inst, _ := svc.GetInstance(context.Background(), testOrg, result.Instance.ID)
	if inst.Instance.Status != checklists.InstanceStatusCompleted {
		t.Fatalf("precondition failed: expected instance completed, got %q", inst.Instance.Status)
	}

	if _, err := svc.ReopenItem(context.Background(), testOrg, item.ID, subjUser); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	inst, err = svc.GetInstance(context.Background(), testOrg, result.Instance.ID)
	if err != nil {
		t.Fatalf("get instance after reopen: %v", err)
	}
	if inst.Instance.Status != checklists.InstanceStatusInProgress {
		t.Errorf("expected reopening the last completed item to flip the instance back to in_progress, got %q", inst.Instance.Status)
	}
}

func TestCompleteItem_RequiresAttachment_Enforced(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Upload signed offer letter", OwnerType: checklists.OwnerTypeSubject, RequiresAttachment: true},
	})
	subjUser := "user_subject"
	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_11", SubjectLabel: "X",
		SubjectUserID: &subjUser, AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	item := result.Items[0]

	if _, err := svc.CompleteItem(context.Background(), testOrg, item.ID, subjUser, checklists.CompleteItemRequest{}); !errors.Is(err, checklists.ErrAttachmentRequired) {
		t.Fatalf("expected ErrAttachmentRequired without an attachment, got %v", err)
	}
	if _, err := svc.CompleteItem(context.Background(), testOrg, item.ID, subjUser, checklists.CompleteItemRequest{AttachmentURL: strp("https://files.example.com/offer.pdf")}); err != nil {
		t.Fatalf("expected completion to succeed with an attachment, got %v", err)
	}
}

// ============================================================
// Authorization matrix
// ============================================================

func TestCompleteItem_AuthorizationMatrix(t *testing.T) {
	svc, _, dir := newChecklistTestSvc()
	tpl := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "Assignee item", OwnerType: checklists.OwnerTypeSubject},
		{Title: "HR role item", OwnerType: checklists.OwnerTypeRole, OwnerRole: strp("hr")},
	})
	subjUser := "user_subject"
	result, err := svc.Instantiate(context.Background(), testOrg, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_12", SubjectLabel: "X",
		SubjectUserID: &subjUser, AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	var assigneeItem, roleItem *checklists.InstanceItem
	for _, it := range result.Items {
		if it.Title == "Assignee item" {
			assigneeItem = it
		} else {
			roleItem = it
		}
	}

	// A stranger with no assignment, no matching role, and no .manage is denied.
	if _, err := svc.CompleteItem(context.Background(), testOrg, assigneeItem.ID, "stranger", checklists.CompleteItemRequest{}); !errors.Is(err, checklists.ErrNotItemOwner) {
		t.Errorf("expected ErrNotItemOwner for an unrelated caller, got %v", err)
	}

	// The assignee may complete their own item.
	if _, err := svc.CompleteItem(context.Background(), testOrg, assigneeItem.ID, subjUser, checklists.CompleteItemRequest{}); err != nil {
		t.Errorf("expected the assignee to be authorized, got %v", err)
	}

	// A caller holding the matching role (case-insensitive) may complete a role-owned item.
	dir.userRoles["hr_holder"] = "HR"
	if _, err := svc.CompleteItem(context.Background(), testOrg, roleItem.ID, "hr_holder", checklists.CompleteItemRequest{}); err != nil {
		t.Errorf("expected a case-insensitive role match to be authorized, got %v", err)
	}

	// Re-fetch a fresh role item for the next two checks (the one above is now terminal).
	tpl2 := mustCreateTemplate(t, svc, dir, []checklists.CreateTemplateItemRequest{
		{Title: "HR role item 2", OwnerType: checklists.OwnerTypeRole, OwnerRole: strp("hr")},
	})
	result2, err := svc.Instantiate(context.Background(), testOrg, tpl2.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "emp_13", SubjectLabel: "Y",
		AnchorDate: time.Now(), CreatedBy: "creator_1",
	})
	if err != nil {
		t.Fatalf("instantiate 2: %v", err)
	}
	roleItem2 := result2.Items[0]

	// A caller with a different role is denied.
	dir.userRoles["it_holder"] = "IT"
	if _, err := svc.CompleteItem(context.Background(), testOrg, roleItem2.ID, "it_holder", checklists.CompleteItemRequest{}); !errors.Is(err, checklists.ErrNotItemOwner) {
		t.Errorf("expected a non-matching role to be denied, got %v", err)
	}

	// A caller with .manage may complete any item as a fallback.
	dir.manageUsers["admin_2"] = true
	if _, err := svc.CompleteItem(context.Background(), testOrg, roleItem2.ID, "admin_2", checklists.CompleteItemRequest{}); err != nil {
		t.Errorf("expected a caller with .manage to be authorized as a fallback, got %v", err)
	}
}
