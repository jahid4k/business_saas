// backend/internal/hrm/performance/goals_model.go
package performance

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

// MeasurementType selects how a goal's progress is interpreted. The arithmetic
// is shared across percentage/numeric/currency; boolean is the only branch.
type MeasurementType string

const (
	MeasurementPercentage MeasurementType = "percentage"
	MeasurementNumeric    MeasurementType = "numeric"
	MeasurementCurrency   MeasurementType = "currency"
	MeasurementBoolean    MeasurementType = "boolean"
)

func (m MeasurementType) IsValid() bool {
	switch m {
	case MeasurementPercentage, MeasurementNumeric, MeasurementCurrency, MeasurementBoolean:
		return true
	}
	return false
}

// Direction records whether a goal is meant to grow or shrink. It drives
// validation and display ONLY — it is deliberately not an input to the
// progress formula, because start_value already makes that arithmetic
// direction-agnostic. Branching on direction is how the obvious
// implementation gets the sign wrong on a decrease goal.
type Direction string

const (
	DirectionIncrease Direction = "increase"
	DirectionDecrease Direction = "decrease"
)

func (d Direction) IsValid() bool {
	switch d {
	case DirectionIncrease, DirectionDecrease:
		return true
	}
	return false
}

// GoalLevel carries the org hierarchy a goal sits at. This — not
// parent_goal_id IS NULL — is what distinguishes an objective from a key
// result, because real OKR trees run company → department → individual → key
// results and a null-parent test misclassifies every intermediate level.
type GoalLevel string

const (
	GoalLevelIndividual GoalLevel = "individual"
	GoalLevelTeam       GoalLevel = "team"
	GoalLevelDepartment GoalLevel = "department"
	GoalLevelCompany    GoalLevel = "company"
)

func (l GoalLevel) IsValid() bool {
	switch l {
	case GoalLevelIndividual, GoalLevelTeam, GoalLevelDepartment, GoalLevelCompany:
		return true
	}
	return false
}

type GoalStatus string

const (
	GoalStatusDraft     GoalStatus = "draft"
	GoalStatusActive    GoalStatus = "active"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusCancelled GoalStatus = "cancelled"
)

func (s GoalStatus) IsValid() bool {
	switch s {
	case GoalStatusDraft, GoalStatusActive, GoalStatusCompleted, GoalStatusCancelled:
		return true
	}
	return false
}

// GoalOutcome is recorded when a goal is completed. Nullable alongside status,
// the hrm_interviews shape — and never auto-derived from progress, consistent
// with this codebase's "no implicit state machine" rule.
type GoalOutcome string

const (
	OutcomeExceeded          GoalOutcome = "exceeded"
	OutcomeAchieved          GoalOutcome = "achieved"
	OutcomePartiallyAchieved GoalOutcome = "partially_achieved"
	OutcomeMissed            GoalOutcome = "missed"
)

func (o GoalOutcome) IsValid() bool {
	switch o {
	case OutcomeExceeded, OutcomeAchieved, OutcomePartiallyAchieved, OutcomeMissed:
		return true
	}
	return false
}

// Goal is one goal or key result. ParentGoalID expresses ALIGNMENT ONLY — a
// parent's CurrentValue is never derived from its children.
type Goal struct {
	ID              string          `db:"id"               json:"id"`
	PublicID        string          `db:"public_id"        json:"public_id"`
	OrgID           string          `db:"org_id"           json:"org_id"`
	CycleID         string          `db:"cycle_id"         json:"cycle_id"`
	EmployeeID      string          `db:"employee_id"      json:"employee_id"`
	ParentGoalID    *string         `db:"parent_goal_id"   json:"parent_goal_id,omitempty"`
	Title           string          `db:"title"            json:"title"`
	Description     *string         `db:"description"      json:"description,omitempty"`
	GoalLevel       GoalLevel       `db:"goal_level"       json:"goal_level"`
	Category        *string         `db:"category"         json:"category,omitempty"`
	MeasurementType MeasurementType `db:"measurement_type" json:"measurement_type"`
	Direction       Direction       `db:"direction"        json:"direction"`
	StartValue      decimal.Decimal `db:"start_value"      json:"start_value"`
	TargetValue     decimal.Decimal `db:"target_value"     json:"target_value"`
	CurrentValue    decimal.Decimal `db:"current_value"    json:"current_value"`
	Unit            *string         `db:"unit"             json:"unit,omitempty"`
	CurrencyCode    *string         `db:"currency_code"    json:"currency_code,omitempty"`
	// Weight nil means "tracking only, not appraised" — excluded from the
	// cycle weight total. This is what lets an objective and its key results
	// coexist under one employee without double-counting.
	Weight       *decimal.Decimal `db:"weight"        json:"weight,omitempty"`
	Status       GoalStatus       `db:"status"        json:"status"`
	Outcome      *GoalOutcome     `db:"outcome"       json:"outcome,omitempty"`
	StartDate    *time.Time       `db:"start_date"    json:"start_date,omitempty"`
	DueDate      *time.Time       `db:"due_date"      json:"due_date,omitempty"`
	CompletedAt  *time.Time       `db:"completed_at"  json:"completed_at,omitempty"`
	CancelledAt  *time.Time       `db:"cancelled_at"  json:"cancelled_at,omitempty"`
	CancelReason *string          `db:"cancel_reason" json:"cancel_reason,omitempty"`
	CreatedBy    string           `db:"created_by"    json:"created_by"`
	CreatedAt    time.Time        `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at"    json:"updated_at"`
}

var hundred = decimal.NewFromInt(100)

// RawProgressPercent is the goal's completion, UNCLAMPED. It may exceed 100
// (a beaten target is a real result an OKR org wants to see) and may go
// negative (regression below the starting point).
//
// Always computed, never stored on hrm_goals — migration 00076's rule:
// "Completion percentage is ALWAYS computed from instance items, never stored
// — no denormalized counter to drift."
//
// One formula covers percentage, numeric and currency because start_value
// makes it direction-agnostic: a decrease goal (start 100, target 20) yields a
// negative numerator over a negative denominator and comes out positive with
// no special case.
func (g *Goal) RawProgressPercent() decimal.Decimal {
	switch g.MeasurementType {
	case MeasurementBoolean:
		// No meaningful interpolation exists between done and not-done.
		if g.CurrentValue.GreaterThanOrEqual(decimal.NewFromInt(1)) {
			return hundred
		}
		return decimal.Zero

	case MeasurementPercentage, MeasurementNumeric, MeasurementCurrency:
		span := g.TargetValue.Sub(g.StartValue)
		if span.IsZero() {
			// Degenerate goal, rejected on write by ErrGoalTargetEqualsStart.
			// A hand-inserted or legacy row must still not divide by zero.
			if g.CurrentValue.GreaterThanOrEqual(g.TargetValue) {
				return hundred
			}
			return decimal.Zero
		}
		return g.CurrentValue.Sub(g.StartValue).Div(span).Mul(hundred).Round(2)

	default:
		return decimal.Zero
	}
}

// ProgressPercent is RawProgressPercent clamped to [0, 100]. This is the
// appraisal-facing number: Phase 5B's weighted attainment is
// SUM(weight * progress) / SUM(weight), which a single 130% goal would
// otherwise inflate past the top of the rating scale.
func (g *Goal) ProgressPercent() decimal.Decimal {
	p := g.RawProgressPercent()
	if p.IsNegative() {
		return decimal.Zero
	}
	if p.GreaterThan(hundred) {
		return hundred
	}
	return p
}

// GoalRef is the alignment projection: exactly what a caller needs to render
// "aligned to <objective>", and nothing else.
//
// Deliberately NOT a *Goal with fields blanked. A separate type means no
// column added to hrm_goals in Phase 5B or 5C can ever leak through a parent
// reference to a caller who is not scoped to the parent's owner. It carries
// zero fields describing that owner's performance, which is the thing the
// scope tiers exist to protect.
type GoalRef struct {
	PublicID  string    `json:"public_id"`
	Title     string    `json:"title"`
	GoalLevel GoalLevel `json:"goal_level"`
}

// GoalDetail is the single-goal response. ProgressPercent and
// ChildrenProgress are computed per request, never stored.
type GoalDetail struct {
	*Goal
	ProgressPercent    decimal.Decimal `json:"progress_percent"`
	RawProgressPercent decimal.Decimal `json:"raw_progress_percent"`
	// Parent is hydrated only here, never in list responses, so the
	// alignment disclosure surface is exactly one endpoint.
	Parent *GoalRef `json:"parent,omitempty"`
	// ChildrenProgress is the mean ProgressPercent of directly aligned goals,
	// nil when there are none. Computed across ALL children regardless of
	// caller scope: it is an aggregate, not a row disclosure, and making it
	// scope-dependent would mean the same goal shows different completion to
	// different viewers.
	ChildrenProgress *decimal.Decimal `json:"children_progress,omitempty"`
	ChildrenCount    int              `json:"children_count"`
}

type CreateGoalRequest struct {
	CycleID         string           `json:"cycle_id"`
	EmployeeID      string           `json:"employee_id"` // omit to target the caller's own employee record
	ParentGoalID    *string          `json:"parent_goal_id"`
	Title           string           `json:"title"`
	Description     *string          `json:"description"`
	GoalLevel       *string          `json:"goal_level"`
	Category        *string          `json:"category"`
	MeasurementType *string          `json:"measurement_type"`
	Direction       *string          `json:"direction"`
	StartValue      *decimal.Decimal `json:"start_value"`
	TargetValue     *decimal.Decimal `json:"target_value"`
	Unit            *string          `json:"unit"`
	CurrencyCode    *string          `json:"currency_code"`
	Weight          *decimal.Decimal `json:"weight"`
	StartDate       *string          `json:"start_date"`
	DueDate         *string          `json:"due_date"`
}

// UpdateGoalRequest deliberately has no CurrentValue field: progress moves
// only through a check-in, so hrm_goal_checkins can never have holes.
type UpdateGoalRequest struct {
	ParentGoalID    *string          `json:"parent_goal_id"`
	Title           *string          `json:"title"`
	Description     *string          `json:"description"`
	GoalLevel       *string          `json:"goal_level"`
	Category        *string          `json:"category"`
	MeasurementType *string          `json:"measurement_type"`
	Direction       *string          `json:"direction"`
	StartValue      *decimal.Decimal `json:"start_value"`
	TargetValue     *decimal.Decimal `json:"target_value"`
	Unit            *string          `json:"unit"`
	CurrencyCode    *string          `json:"currency_code"`
	Weight          *decimal.Decimal `json:"weight"`
	StartDate       *string          `json:"start_date"`
	DueDate         *string          `json:"due_date"`
}

type CompleteGoalRequest struct {
	Outcome *string `json:"outcome"`
}

type CancelGoalRequest struct {
	Reason string `json:"reason"`
}

type GoalListFilter struct {
	CycleID    string
	EmployeeID string
	Status     string
	GoalLevel  string
	ParentID   string
	Limit      int
	Offset     int

	// Scope and CallerUserID are set by the handler from
	// authzSvc.ResolveScope — never resolved in the service or repository.
	// The zero value (authz.ScopeNone) means "no rows", not "no filter".
	Scope        authz.Scope
	CallerUserID string
}

func (f *GoalListFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

type GoalListResponse struct {
	Goals  []*GoalListItem `json:"goals"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// GoalListItem carries the computed progress alongside the row, but never the
// hydrated parent — see GoalDetail.Parent.
type GoalListItem struct {
	*Goal
	ProgressPercent    decimal.Decimal `json:"progress_percent"`
	RawProgressPercent decimal.Decimal `json:"raw_progress_percent"`
}

var (
	ErrGoalNotFound       = errors.New("goal not found")
	ErrGoalTitleRequired  = errors.New("title is required")
	ErrGoalCycleRequired  = errors.New("cycle_id is required")
	ErrGoalAccessDenied   = errors.New("you do not have access to this goal")
	ErrGoalInvalidLevel   = errors.New("goal_level must be one of: individual, team, department, company")
	ErrGoalInvalidMeasure = errors.New("measurement_type must be one of: percentage, numeric, currency, boolean")
	ErrGoalInvalidDir     = errors.New("direction must be one of: increase, decrease")
	ErrGoalInvalidOutcome = errors.New("outcome must be one of: exceeded, achieved, partially_achieved, missed")
	ErrGoalInvalidStatus  = errors.New("invalid goal status")
	ErrGoalInvalidDate    = errors.New("dates must be valid and in YYYY-MM-DD format")
	ErrGoalInvalidWeight  = errors.New("weight must be between 0 and 100")
	ErrGoalDatesInvalid   = errors.New("due_date must be on or after start_date")
	// ErrGoalTargetEqualsStart rejects a goal whose target equals its start,
	// which has no meaningful progress denominator.
	ErrGoalTargetEqualsStart = errors.New("target_value must differ from start_value")
	ErrGoalDirectionMismatch = errors.New("target_value must be greater than start_value for an increase goal, and less for a decrease goal")
	// ErrWeightExceedsCycleTarget is raised when a write would push the
	// employee's total weight for the cycle above the cycle's weight_target.
	ErrWeightExceedsCycleTarget = errors.New("this would push the employee's total goal weight above the cycle target")
	ErrGoalAlignmentCycle       = errors.New("a goal cannot be aligned to itself or to one of its own descendants")
	ErrGoalHasHistory           = errors.New("this goal has check-ins or aligned goals and cannot be deleted — cancel it instead")
	ErrGoalWrongStatus          = errors.New("action not allowed in the goal's current status")
	ErrEmployeeNotFound         = errors.New("employee not found in this organization")
	ErrCallerHasNoEmployee      = errors.New("you have no employee record in this organization")
)
