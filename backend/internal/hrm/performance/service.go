// backend/internal/hrm/performance/service.go
package performance

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Service defines business logic for the performance module. Composed of the
// sub-feature interfaces declared in cycles_service.go, goals_service.go and
// checkins_service.go.
//
// This interface takes no authz.Service. The handler resolves the caller's
// scope tier and manage-permission once and passes them on a Caller value, so
// the service stays testable against a stub repository plus a stub
// RecordAuthorizer alone.
type Service interface {
	CycleService
	GoalService
	CheckinService
	ScaleService
	AppraisalService
}

type serviceImpl struct {
	repo    Repository
	records RecordAuthorizer
	// forms is the form engine appraisals consume. A nil engine is valid and
	// degrades appraisals to rating-only — the checklists.ChecklistHook
	// precedent, where a nil hook is a silent no-op rather than a panic.
	forms FormEngine
}

func NewService(repo Repository, records RecordAuthorizer, formEngine FormEngine) Service {
	return &serviceImpl{repo: repo, records: records, forms: formEngine}
}

// ── Shared helpers ───────────────────────────────────────────────────────────

// parseDate converts an ISO 8601 date string to a time.Time. Empty input
// yields nil with no error, so optional date fields need no caller branching.
func parseDate(v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	d, err := time.Parse(dateLayout, strings.TrimSpace(*v))
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// nilIfBlank normalises an all-whitespace optional string to nil so blank
// input and absent input are stored identically.
func nilIfBlank(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	return &trimmed
}

// resolveCallerEmployeeID returns the caller's own hrm_employees.id, or "" when
// they have no employee record — a valid state for a non-employee admin, not
// an error.
func (s *serviceImpl) resolveCallerEmployeeID(ctx context.Context, orgID string, caller Caller) (string, error) {
	id, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, caller.UserID)
	if err != nil {
		return "", fmt.Errorf("performance: resolve caller employee: %w", err)
	}
	return id, nil
}

// authorizeGoalAccess gates a single-record read against the caller's scope
// tier, resolved against the goal's owning employee.
func (s *serviceImpl) authorizeGoalAccess(ctx context.Context, orgID string, caller Caller, g *Goal) error {
	allowed, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, g.EmployeeID)
	if err != nil {
		return fmt.Errorf("performance: authorize goal access: %w", err)
	}
	if !allowed {
		return ErrGoalAccessDenied
	}
	return nil
}

// authorizeWrite is the single narrowing point for every goal mutation.
//
// The route gate (hrm.goals.set_own) is granted through 'member' and cannot
// express "is this your own goal", so it does not try — the
// platform.checklists.complete and hrm.interviews.scorecard precedent.
//
// The second half is the part that is easy to omit and expensive to miss:
// hrm.goals.manage is unscoped at the route, so a manager holding view_team
// must still be blocked from editing a goal outside their reporting line.
// Only the AuthorizeRecordAccess call enforces that.
func (s *serviceImpl) authorizeWrite(ctx context.Context, orgID string, caller Caller, targetEmployeeID string) error {
	ownEmployeeID, err := s.resolveCallerEmployeeID(ctx, orgID, caller)
	if err != nil {
		return err
	}
	if ownEmployeeID != "" && ownEmployeeID == targetEmployeeID {
		return nil // own goal: set_own alone is sufficient
	}
	if !caller.CanManage {
		return ErrGoalAccessDenied
	}
	allowed, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, targetEmployeeID)
	if err != nil {
		return fmt.Errorf("performance: authorize write: %w", err)
	}
	if !allowed {
		return ErrGoalAccessDenied
	}
	return nil
}
