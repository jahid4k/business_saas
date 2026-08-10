// backend/internal/hrm/performance/cycles_model.go
package performance

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// CycleStatus mirrors hrm_attendance_periods' lifecycle shape: a period that
// opens, freezes, then closes.
type CycleStatus string

const (
	// CycleStatusDraft — being configured; goals cannot be attached yet.
	CycleStatusDraft CycleStatus = "draft"
	// CycleStatusActive — goals are writable.
	CycleStatusActive CycleStatus = "active"
	// CycleStatusLocked — goal DEFINITIONS are frozen, but check-ins still
	// land. This is the normal in-flight state for a quarter, not an end state.
	CycleStatusLocked CycleStatus = "locked"
	// CycleStatusClosed — fully immutable.
	CycleStatusClosed CycleStatus = "closed"
)

func (s CycleStatus) IsValid() bool {
	switch s {
	case CycleStatusDraft, CycleStatusActive, CycleStatusLocked, CycleStatusClosed:
		return true
	}
	return false
}

// GoalCycle is the org-scoped period goals belong to, and the scope key for
// weight-sum validation. Deliberately separate from Phase 5B's appraisal
// cycles: the lifecycles differ, the cardinality is not 1:1 (an annual
// appraisal can span two half-year goal cycles), and appraisal cycles snapshot
// a rating scale and form template that a goal cycle has no use for.
type GoalCycle struct {
	ID           string          `db:"id"            json:"id"`
	PublicID     string          `db:"public_id"     json:"public_id"`
	OrgID        string          `db:"org_id"        json:"org_id"`
	Name         string          `db:"name"          json:"name"`
	Description  *string         `db:"description"   json:"description,omitempty"`
	PeriodStart  time.Time       `db:"period_start"  json:"period_start"`
	PeriodEnd    time.Time       `db:"period_end"    json:"period_end"`
	Status       CycleStatus     `db:"status"        json:"status"`
	WeightTarget decimal.Decimal `db:"weight_target" json:"weight_target"`
	LockedAt     *time.Time      `db:"locked_at"     json:"locked_at,omitempty"`
	LockedBy     *string         `db:"locked_by"     json:"locked_by,omitempty"`
	ClosedAt     *time.Time      `db:"closed_at"     json:"closed_at,omitempty"`
	CreatedBy    string          `db:"created_by"    json:"created_by"`
	CreatedAt    time.Time       `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"    json:"updated_at"`
}

// EmployeeWeightTotal is one employee's total goal weight within a cycle. The
// lock gate and the read-only weight-audit endpoint share this single shape,
// so there is exactly one definition of "whose weights are wrong".
type EmployeeWeightTotal struct {
	EmployeeID   string          `json:"employee_id"`
	EmployeeName string          `json:"employee_name"`
	TotalWeight  decimal.Decimal `json:"total_weight"`
	GoalCount    int             `json:"goal_count"`
}

// CycleWeightAudit reports every employee holding weighted goals in a cycle,
// and which of them do not sum to the cycle's target.
type CycleWeightAudit struct {
	CycleID      string                 `json:"cycle_id"`
	WeightTarget decimal.Decimal        `json:"weight_target"`
	Employees    []*EmployeeWeightTotal `json:"employees"`
	// Incomplete lists only the employees whose total differs from the target
	// — the exact set that blocks a lock.
	Incomplete []*EmployeeWeightTotal `json:"incomplete"`
}

type CreateCycleRequest struct {
	Name         string           `json:"name"`
	Description  *string          `json:"description"`
	PeriodStart  string           `json:"period_start"` // ISO 8601 date
	PeriodEnd    string           `json:"period_end"`   // ISO 8601 date
	WeightTarget *decimal.Decimal `json:"weight_target"`
}

type UpdateCycleRequest struct {
	Name         *string          `json:"name"`
	Description  *string          `json:"description"`
	PeriodStart  *string          `json:"period_start"`
	PeriodEnd    *string          `json:"period_end"`
	WeightTarget *decimal.Decimal `json:"weight_target"`
}

type CycleListFilter struct {
	Status string
	Limit  int
	Offset int
}

func (f *CycleListFilter) Normalise() {
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

type CycleListResponse struct {
	Cycles []*GoalCycle `json:"cycles"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

var (
	ErrCycleNotFound       = errors.New("goal cycle not found")
	ErrCycleNameRequired   = errors.New("name is required")
	ErrCycleNameTaken      = errors.New("a goal cycle with this name already exists in this organization")
	ErrCyclePeriodInvalid  = errors.New("period_end must be on or after period_start")
	ErrCycleDateRequired   = errors.New("period_start and period_end are required (YYYY-MM-DD)")
	ErrCycleInvalidDate    = errors.New("dates must be valid and in YYYY-MM-DD format")
	ErrCycleWrongStatus    = errors.New("action not allowed in the cycle's current status")
	ErrCycleNotActive      = errors.New("goal cycle is not active")
	ErrInvalidWeightTarget = errors.New("weight_target must be greater than zero")
	// ErrCycleWeightsIncomplete blocks a lock while some employee's goals do
	// not total the cycle's weight target. The handler surfaces the offending
	// employee list alongside it.
	ErrCycleWeightsIncomplete = errors.New("some employees' goal weights do not total the cycle target")
)
