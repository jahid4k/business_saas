// backend/internal/hrm/onboarding/service.go
package onboarding

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mridha/businesssaas/internal/platform/checklists"
)

// Service is the HRM onboarding consumer of the checklist engine. It is the
// only piece of this package that knows both "employee" and "checklist" —
// internal/platform/checklists never imports internal/hrm/employees, and
// this package deliberately does not import internal/hrm/employees either
// (that would recreate the cycle it exists to avoid); it gets its own
// Repository.FindSubject instead.
type Service interface {
	// OnEmployeeCreated implements employees.ChecklistHook. Called
	// synchronously, immediately after employees.Service.Create commits and
	// audit-logs the new employee. Must never surface an error that would
	// cause the caller to fail employee creation — see main.go's wiring
	// comment and employees.Service.Create's hook call site, which logs any
	// returned error and continues regardless.
	OnEmployeeCreated(ctx context.Context, orgID, actorID, employeeID string) error

	// InstantiateForEmployee is the manual retry path for
	// OnEmployeeCreated — the auto-hook has a partial-write window with no
	// reconciliation (the employee row commits before this runs; a failure
	// here leaves no checklist and no automatic retry), so this endpoint IS
	// the retry. Returns ErrNoDefaultTemplate, not a generic error, when the
	// org has none configured.
	InstantiateForEmployee(ctx context.Context, orgID, employeeRef, actorID string) (*checklists.InstantiateResult, error)

	// ListForEmployee resolves employeeRef then lists that employee's
	// checklist instances (onboarding, and anything else keyed to the same
	// subject in the future).
	ListForEmployee(ctx context.Context, orgID, employeeRef string) (*checklists.InstanceListResponse, error)
}

type serviceImpl struct {
	repo       Repository
	checklists checklists.Service
}

func NewService(repo Repository, checklistsSvc checklists.Service) Service {
	return &serviceImpl{repo: repo, checklists: checklistsSvc}
}

// buildSubjectContext is the one place hrm_employees fields get translated
// into the opaque SubjectContext the checklist engine accepts. This
// boundary is what stops an HTTP caller from ever supplying their own
// subject_user_id/manager_user_id (see checklists' routes.go doc comment on
// why there is no generic instantiate route).
func (s *serviceImpl) buildSubjectContext(ctx context.Context, orgID, actorID, employeeRef string) (checklists.SubjectContext, error) {
	ref, err := s.repo.FindSubject(ctx, orgID, employeeRef)
	if err != nil {
		return checklists.SubjectContext{}, fmt.Errorf("onboarding: buildSubjectContext: %w", err)
	}
	if ref == nil {
		return checklists.SubjectContext{}, ErrEmployeeNotFound
	}
	return checklists.SubjectContext{
		SubjectType:   checklists.SubjectTypeEmployee,
		SubjectID:     ref.EmployeeID,
		SubjectLabel:  ref.DisplayName,
		SubjectUserID: ref.UserID,
		ManagerUserID: ref.ManagerUserID,
		AnchorDate:    ref.HireDate,
		CreatedBy:     actorID,
	}, nil
}

// OnEmployeeCreated returns errors normally — employees.Service.Create logs
// whatever this returns and continues either way, so there is no need to
// swallow errors twice. The one thing that MUST be caught here, not there,
// is a panic: employees.Create only guards against a returned error, and an
// unrecovered panic on this path would otherwise hit Fiber's recover
// middleware and 500 the request AFTER the employee row is already
// committed — strictly worse than a logged, swallowed failure.
func (s *serviceImpl) OnEmployeeCreated(ctx context.Context, orgID, actorID, employeeID string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("onboarding: OnEmployeeCreated: recovered panic: %v", r)
		}
	}()

	subj, err := s.buildSubjectContext(ctx, orgID, actorID, employeeID)
	if err != nil {
		return fmt.Errorf("onboarding: OnEmployeeCreated: %w", err)
	}

	result, err := s.checklists.InstantiateDefault(ctx, orgID, checklists.ChecklistTypeOnboarding, subj)
	if err != nil {
		return fmt.Errorf("onboarding: OnEmployeeCreated: instantiate: %w", err)
	}
	if result == nil {
		// No default onboarding template configured for this org — the
		// normal state for an org that hasn't set one up. Not an error.
		return nil
	}
	if result.UnresolvedCount > 0 {
		slog.Warn("onboarding: checklist instantiated with unresolved item owners",
			slog.String("org_id", orgID), slog.String("employee_id", employeeID),
			slog.Int("unresolved_count", result.UnresolvedCount),
		)
	}
	return nil
}

func (s *serviceImpl) InstantiateForEmployee(ctx context.Context, orgID, employeeRef, actorID string) (*checklists.InstantiateResult, error) {
	subj, err := s.buildSubjectContext(ctx, orgID, actorID, employeeRef)
	if err != nil {
		return nil, err
	}
	result, err := s.checklists.InstantiateDefault(ctx, orgID, checklists.ChecklistTypeOnboarding, subj)
	if err != nil {
		return nil, fmt.Errorf("onboarding: InstantiateForEmployee: %w", err)
	}
	if result == nil {
		return nil, ErrNoDefaultTemplate
	}
	return result, nil
}

func (s *serviceImpl) ListForEmployee(ctx context.Context, orgID, employeeRef string) (*checklists.InstanceListResponse, error) {
	ref, err := s.repo.FindSubject(ctx, orgID, employeeRef)
	if err != nil {
		return nil, fmt.Errorf("onboarding: ListForEmployee: %w", err)
	}
	if ref == nil {
		return nil, ErrEmployeeNotFound
	}
	subjType := checklists.SubjectTypeEmployee
	resp, err := s.checklists.ListInstances(ctx, orgID, checklists.InstanceFilter{
		SubjectType: &subjType,
		SubjectID:   &ref.EmployeeID,
	})
	if err != nil {
		return nil, fmt.Errorf("onboarding: ListForEmployee: %w", err)
	}
	return resp, nil
}
