// backend/internal/hrm/performance/model.go
package performance

import (
	"context"

	"github.com/mridha/businesssaas/internal/authz"
)

// Package performance implements HRM Performance Management: Phase 5A's
// Goals/OKR and Phase 5B's appraisal cycles, as sub-feature quartets in one
// package so appraisals read goal attainment through a plain method call
// instead of a cross-package narrow interface.
//
// The 5A package doc set a threshold here: split if the composite Repository
// passed roughly 60 methods. It reached 58 at the end of 5B, so Phase 5C's
// two sub-systems went to their own packages rather than becoming a sixth and
// seventh quartet — internal/hrm/feedback (360 feedback) and internal/hrm/pip.
// Neither shares a query surface with goals or appraisals, and pip carries an
// outbound edge to terminations that would otherwise have been dragged in
// here. Each resolves the employee facts it needs with its own small query,
// the internal/hrm/onboarding precedent.
//
// If THIS package grows further, the same split applies: performance/
// (cycles + goals) and appraisals/ with a GoalAttainmentReader interface.
// That is a file move; merging two packages later is the harder direction.

// dateLayout is the ISO 8601 date format used for cycle periods and goal dates.
const dateLayout = "2006-01-02"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Caller carries the authorization facts the handler has already resolved, so
// the service layer needs no authz dependency of its own and stays testable
// against a stub repository alone.
type Caller struct {
	UserID string
	// Tier comes from authzSvc.ResolveScope(ctx, userID, orgID, "hrm.goals").
	Tier authz.Scope
	// CanManage reports whether the caller holds hrm.goals.manage, which
	// permits writing goals they do not own — still subject to Tier.
	CanManage bool
}

// RecordAuthorizer is the one-method slice of *scope.Resolver this package
// needs. Declared as an interface purely so unit tests can stub it;
// *scope.Resolver satisfies it structurally, so main.go passes the resolver
// directly with no adapter.
type RecordAuthorizer interface {
	AuthorizeRecordAccess(ctx context.Context, tier authz.Scope, orgID, callerUserID, recordEmployeeRef string) (bool, error)
}
