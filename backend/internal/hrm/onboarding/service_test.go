// backend/internal/hrm/onboarding/service_test.go
// White-box (package onboarding, not onboarding_test) because subjectRef is
// unexported — the same reason internal/hrm/leave/balances_accrual_test.go
// lives inside its own package rather than under internal/tests/unit/.
package onboarding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/platform/checklists"
)

// ── Stub Repository ──────────────────────────────────────────────────────────

type stubRepo struct {
	ref *subjectRef
	err error
}

func (r *stubRepo) FindSubject(_ context.Context, _, _ string) (*subjectRef, error) {
	return r.ref, r.err
}

// ── Stub checklists.Service ─────────────────────────────────────────────────
// Only InstantiateDefault and ListInstances have configurable behaviour —
// everything else in the interface is unreachable from this package and
// returns a "not implemented" error so a stray call fails loudly.

type stubChecklistsSvc struct {
	instantiateDefaultFn func(ctx context.Context, orgID string, checklistType checklists.ChecklistType, subj checklists.SubjectContext) (*checklists.InstantiateResult, error)
	listInstancesFn      func(ctx context.Context, orgID string, f checklists.InstanceFilter) (*checklists.InstanceListResponse, error)
}

func notImplemented() error { return errors.New("stubChecklistsSvc: not implemented") }

func (s *stubChecklistsSvc) InstantiateDefault(ctx context.Context, orgID string, checklistType checklists.ChecklistType, subj checklists.SubjectContext) (*checklists.InstantiateResult, error) {
	if s.instantiateDefaultFn != nil {
		return s.instantiateDefaultFn(ctx, orgID, checklistType, subj)
	}
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) ListInstances(ctx context.Context, orgID string, f checklists.InstanceFilter) (*checklists.InstanceListResponse, error) {
	if s.listInstancesFn != nil {
		return s.listInstancesFn(ctx, orgID, f)
	}
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) ListTemplates(context.Context, string, *checklists.ChecklistType) ([]*checklists.Template, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) GetTemplate(context.Context, string, string) (*checklists.TemplateWithItems, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) CreateTemplate(context.Context, string, string, checklists.CreateTemplateRequest) (*checklists.TemplateWithItems, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) UpdateTemplate(context.Context, string, string, checklists.UpdateTemplateRequest) (*checklists.Template, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) DeleteTemplate(context.Context, string, string) error {
	return notImplemented()
}
func (s *stubChecklistsSvc) ListTemplateItems(context.Context, string, string) ([]*checklists.TemplateItem, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) CreateTemplateItem(context.Context, string, string, checklists.CreateTemplateItemRequest) (*checklists.TemplateItem, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) UpdateTemplateItem(context.Context, string, string, string, checklists.UpdateTemplateItemRequest) (*checklists.TemplateItem, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) DeleteTemplateItem(context.Context, string, string, string) error {
	return notImplemented()
}
func (s *stubChecklistsSvc) Instantiate(context.Context, string, string, checklists.SubjectContext) (*checklists.InstantiateResult, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) GetInstance(context.Context, string, string) (*checklists.InstanceWithItems, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) CancelInstance(context.Context, string, string, checklists.CancelInstanceRequest) (*checklists.Instance, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) ListMyItems(context.Context, string, string, *checklists.ItemStatus) ([]*checklists.InstanceItem, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) CompleteItem(context.Context, string, string, string, checklists.CompleteItemRequest) (*checklists.InstanceItem, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) ReopenItem(context.Context, string, string, string) (*checklists.InstanceItem, error) {
	return nil, notImplemented()
}
func (s *stubChecklistsSvc) SkipItem(context.Context, string, string, string, checklists.SkipItemRequest) (*checklists.InstanceItem, error) {
	return nil, notImplemented()
}

var _ checklists.Service = (*stubChecklistsSvc)(nil)

// ============================================================
// buildSubjectContext field mapping
// ============================================================

func TestBuildSubjectContext_FieldMapping(t *testing.T) {
	subjUser := "user_subject"
	mgrUser := "user_manager"
	hireDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	repo := &stubRepo{ref: &subjectRef{
		EmployeeID: "emp_123", UserID: &subjUser, ManagerUserID: &mgrUser,
		HireDate: hireDate, DisplayName: "Jane Doe", EmployeeNumber: "EMP-001",
	}}

	var captured checklists.SubjectContext
	stub := &stubChecklistsSvc{
		instantiateDefaultFn: func(_ context.Context, orgID string, ct checklists.ChecklistType, subj checklists.SubjectContext) (*checklists.InstantiateResult, error) {
			captured = subj
			return nil, nil // no default template — the auto-hook's normal no-op path
		},
	}

	svc := NewService(repo, stub)
	if err := svc.OnEmployeeCreated(context.Background(), "org_1", "actor_1", "emp_123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.SubjectType != checklists.SubjectTypeEmployee {
		t.Errorf("expected SubjectType=employee, got %q", captured.SubjectType)
	}
	if captured.SubjectID != "emp_123" {
		t.Errorf("expected SubjectID='emp_123', got %q", captured.SubjectID)
	}
	if captured.SubjectLabel != "Jane Doe" {
		t.Errorf("expected SubjectLabel='Jane Doe', got %q", captured.SubjectLabel)
	}
	if captured.SubjectUserID == nil || *captured.SubjectUserID != subjUser {
		t.Errorf("expected SubjectUserID=%q, got %v", subjUser, captured.SubjectUserID)
	}
	if captured.ManagerUserID == nil || *captured.ManagerUserID != mgrUser {
		t.Errorf("expected ManagerUserID=%q, got %v", mgrUser, captured.ManagerUserID)
	}
	if !captured.AnchorDate.Equal(hireDate) {
		t.Errorf("expected AnchorDate=%v (the employee's hire_date), got %v", hireDate, captured.AnchorDate)
	}
	if captured.CreatedBy != "actor_1" {
		t.Errorf("expected CreatedBy='actor_1' (the acting user, not the employee), got %q", captured.CreatedBy)
	}
}

func TestBuildSubjectContext_NoManagerOrUserAccount_BothNil(t *testing.T) {
	repo := &stubRepo{ref: &subjectRef{
		EmployeeID: "emp_456", UserID: nil, ManagerUserID: nil,
		HireDate: time.Now(), DisplayName: "Contractor",
	}}
	var captured checklists.SubjectContext
	stub := &stubChecklistsSvc{
		instantiateDefaultFn: func(_ context.Context, _ string, _ checklists.ChecklistType, subj checklists.SubjectContext) (*checklists.InstantiateResult, error) {
			captured = subj
			return nil, nil
		},
	}
	svc := NewService(repo, stub)
	if err := svc.OnEmployeeCreated(context.Background(), "org_1", "actor_1", "emp_456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.SubjectUserID != nil || captured.ManagerUserID != nil {
		t.Errorf("expected both nil (no platform account, no manager), got SubjectUserID=%v ManagerUserID=%v", captured.SubjectUserID, captured.ManagerUserID)
	}
}

// ============================================================
// OnEmployeeCreated — errors, panics, unknown employees
// ============================================================

func TestOnEmployeeCreated_EmployeeNotFound(t *testing.T) {
	repo := &stubRepo{ref: nil, err: nil} // (nil, nil) — no matching employee
	stub := &stubChecklistsSvc{}
	svc := NewService(repo, stub)

	err := svc.OnEmployeeCreated(context.Background(), "org_1", "actor_1", "does-not-exist")
	if !errors.Is(err, ErrEmployeeNotFound) {
		t.Fatalf("expected ErrEmployeeNotFound, got %v", err)
	}
}

func TestOnEmployeeCreated_RecoversPanicInChecklistsCall(t *testing.T) {
	repo := &stubRepo{ref: &subjectRef{EmployeeID: "emp_1", HireDate: time.Now(), DisplayName: "X"}}
	stub := &stubChecklistsSvc{
		instantiateDefaultFn: func(context.Context, string, checklists.ChecklistType, checklists.SubjectContext) (*checklists.InstantiateResult, error) {
			panic("simulated bug in the checklist engine")
		},
	}
	svc := NewService(repo, stub)

	// The property under test: this call must return an error, not panic.
	// A panic escaping here would hit Fiber's recover middleware and 500 the
	// request AFTER employees.Create's row is already committed — strictly
	// worse than a logged, swallowed failure.
	err := svc.OnEmployeeCreated(context.Background(), "org_1", "actor_1", "emp_1")
	if err == nil {
		t.Fatal("expected a non-nil error recovered from the panic, got nil")
	}
}

func TestOnEmployeeCreated_NoDefaultTemplate_ReturnsNilNotError(t *testing.T) {
	repo := &stubRepo{ref: &subjectRef{EmployeeID: "emp_1", HireDate: time.Now(), DisplayName: "X"}}
	stub := &stubChecklistsSvc{
		instantiateDefaultFn: func(context.Context, string, checklists.ChecklistType, checklists.SubjectContext) (*checklists.InstantiateResult, error) {
			return nil, nil // the engine's own "no default configured" signal
		},
	}
	svc := NewService(repo, stub)
	if err := svc.OnEmployeeCreated(context.Background(), "org_1", "actor_1", "emp_1"); err != nil {
		t.Errorf("expected nil error when there is no default template, got %v", err)
	}
}

// ============================================================
// InstantiateForEmployee — the manual retry path
// ============================================================

func TestInstantiateForEmployee_NoDefaultTemplate_ReturnsErrNoDefaultTemplate(t *testing.T) {
	repo := &stubRepo{ref: &subjectRef{EmployeeID: "emp_1", HireDate: time.Now(), DisplayName: "X"}}
	stub := &stubChecklistsSvc{
		instantiateDefaultFn: func(context.Context, string, checklists.ChecklistType, checklists.SubjectContext) (*checklists.InstantiateResult, error) {
			return nil, nil
		},
	}
	svc := NewService(repo, stub)

	_, err := svc.InstantiateForEmployee(context.Background(), "org_1", "emp_1", "actor_1")
	if !errors.Is(err, ErrNoDefaultTemplate) {
		t.Fatalf("expected ErrNoDefaultTemplate, got %v", err)
	}
}

func TestInstantiateForEmployee_EmployeeNotFound(t *testing.T) {
	repo := &stubRepo{ref: nil}
	stub := &stubChecklistsSvc{}
	svc := NewService(repo, stub)

	_, err := svc.InstantiateForEmployee(context.Background(), "org_1", "ghost", "actor_1")
	if !errors.Is(err, ErrEmployeeNotFound) {
		t.Fatalf("expected ErrEmployeeNotFound, got %v", err)
	}
}
