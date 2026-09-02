// backend/internal/hrm/benefits/model.go
package benefits

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

type PlanType string

const (
	PlanHealth     PlanType = "health"
	PlanDental     PlanType = "dental"
	PlanVision     PlanType = "vision"
	PlanLife       PlanType = "life"
	PlanRetirement PlanType = "retirement"
	PlanOther      PlanType = "other"
)

func (t PlanType) IsValid() bool {
	switch t {
	case PlanHealth, PlanDental, PlanVision, PlanLife, PlanRetirement, PlanOther:
		return true
	}
	return false
}

type Plan struct {
	ID          string    `db:"id"           json:"id"`
	PublicID    string    `db:"public_id"     json:"public_id"`
	OrgID       string    `db:"org_id"        json:"org_id"`
	Name        string    `db:"name"          json:"name"`
	PlanType    PlanType  `db:"plan_type"     json:"plan_type"`
	Description *string   `db:"description"   json:"description,omitempty"`
	IsActive    bool      `db:"is_active"     json:"is_active"`
	CreatedBy   string    `db:"created_by"    json:"created_by"`
	CreatedAt   time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"    json:"updated_at"`
}

type CreatePlanRequest struct {
	Name        string  `json:"name"`
	PlanType    string  `json:"plan_type"`
	Description *string `json:"description"`
}

// Tier is mutable catalog data — repricing it does not retroactively change
// an already-enrolled employee's cost, frozen on Enrollment. See migration
// 00104's header.
type Tier struct {
	ID           string          `db:"id"             json:"id"`
	PublicID     string          `db:"public_id"       json:"public_id"`
	PlanID       string          `db:"plan_id"         json:"plan_id"`
	TierName     string          `db:"tier_name"       json:"tier_name"`
	EmployeeCost decimal.Decimal `db:"employee_cost"   json:"employee_cost"`
	EmployerCost decimal.Decimal `db:"employer_cost"   json:"employer_cost"`
	IsActive     bool            `db:"is_active"       json:"is_active"`
	CreatedBy    string          `db:"created_by"      json:"created_by"`
	CreatedAt    time.Time       `db:"created_at"      json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"      json:"updated_at"`
}

type CreateTierRequest struct {
	TierName     string `json:"tier_name"`
	EmployeeCost string `json:"employee_cost"`
	EmployerCost string `json:"employer_cost"`
}

type WindowType string

const (
	WindowOpen            WindowType = "open"
	WindowNewHire         WindowType = "new_hire"
	WindowQualifyingEvent WindowType = "qualifying_event"
)

func (w WindowType) IsValid() bool {
	switch w {
	case WindowOpen, WindowNewHire, WindowQualifyingEvent:
		return true
	}
	return false
}

type EnrollmentStatus string

const (
	EnrollmentPending    EnrollmentStatus = "pending"
	EnrollmentActive     EnrollmentStatus = "active"
	EnrollmentWaived     EnrollmentStatus = "waived"
	EnrollmentTerminated EnrollmentStatus = "terminated"
)

// Enrollment is one employee's enrollment in one tier. employee_cost_snapshot
// / employer_cost_snapshot are frozen at enrollment time — see migration
// 00104's header on why re-pricing the tier later must not change these.
type Enrollment struct {
	ID                   string           `db:"id"                      json:"id"`
	PublicID             string           `db:"public_id"                json:"public_id"`
	OrgID                string           `db:"org_id"                   json:"org_id"`
	EmployeeID           string           `db:"employee_id"              json:"employee_id"`
	PlanID               string           `db:"plan_id"                  json:"plan_id"`
	TierID               string           `db:"tier_id"                  json:"tier_id"`
	EnrollmentWindowType WindowType       `db:"enrollment_window_type"   json:"enrollment_window_type"`
	Status               EnrollmentStatus `db:"status"                   json:"status"`
	EffectiveDate        time.Time        `db:"effective_date"           json:"effective_date"`
	EndDate              *time.Time       `db:"end_date"                 json:"end_date,omitempty"`
	EmployeeCostSnapshot decimal.Decimal  `db:"employee_cost_snapshot"   json:"employee_cost_snapshot"`
	EmployerCostSnapshot decimal.Decimal  `db:"employer_cost_snapshot"   json:"employer_cost_snapshot"`
	CreatedBy            string           `db:"created_by"               json:"created_by"`
	CreatedAt            time.Time        `db:"created_at"               json:"created_at"`
	UpdatedAt            time.Time        `db:"updated_at"               json:"updated_at"`
}

type CreateEnrollmentRequest struct {
	EmployeeID           string `json:"employee_id"`
	PlanID               string `json:"plan_id"`
	TierID               string `json:"tier_id"`
	EnrollmentWindowType string `json:"enrollment_window_type"`
	EffectiveDate        string `json:"effective_date"`
}

type Relationship string

const (
	RelSpouse          Relationship = "spouse"
	RelChild           Relationship = "child"
	RelDomesticPartner Relationship = "domestic_partner"
	RelOther           Relationship = "other"
)

func (r Relationship) IsValid() bool {
	switch r {
	case RelSpouse, RelChild, RelDomesticPartner, RelOther:
		return true
	}
	return false
}

// Dependent is manually verified — no document-upload/review workflow, per
// the build plan.
type Dependent struct {
	ID           string       `db:"id"              json:"id"`
	PublicID     string       `db:"public_id"        json:"public_id"`
	OrgID        string       `db:"org_id"           json:"org_id"`
	EmployeeID   string       `db:"employee_id"      json:"employee_id"`
	EnrollmentID *string      `db:"enrollment_id"    json:"enrollment_id,omitempty"`
	FullName     string       `db:"full_name"        json:"full_name"`
	Relationship Relationship `db:"relationship"     json:"relationship"`
	DateOfBirth  *time.Time   `db:"date_of_birth"    json:"date_of_birth,omitempty"`
	IsVerified   bool         `db:"is_verified"      json:"is_verified"`
	VerifiedBy   *string      `db:"verified_by"      json:"verified_by,omitempty"`
	VerifiedAt   *time.Time   `db:"verified_at"      json:"verified_at,omitempty"`
	CreatedBy    string       `db:"created_by"       json:"created_by"`
	CreatedAt    time.Time    `db:"created_at"       json:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at"       json:"updated_at"`
}

type CreateDependentRequest struct {
	EmployeeID   string  `json:"employee_id"`
	EnrollmentID *string `json:"enrollment_id"`
	FullName     string  `json:"full_name"`
	Relationship string  `json:"relationship"`
	DateOfBirth  *string `json:"date_of_birth"`
}

type ListFilter struct {
	EmployeeID   string
	Limit        int
	Offset       int
	Scope        authz.Scope
	CallerUserID string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *ListFilter) Normalise() {
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

type EnrollmentListResponse struct {
	Enrollments []*Enrollment `json:"enrollments"`
	Total       int           `json:"total"`
	Limit       int           `json:"limit"`
	Offset      int           `json:"offset"`
}

var (
	ErrPlanNotFound        = errors.New("benefit plan not found")
	ErrTierNotFound        = errors.New("benefit tier not found")
	ErrEnrollmentNotFound  = errors.New("enrollment not found")
	ErrDependentNotFound   = errors.New("dependent not found")
	ErrInvalidPlanType     = errors.New("plan_type is not a recognised value")
	ErrInvalidWindowType   = errors.New("enrollment_window_type is not a recognised value")
	ErrInvalidRelationship = errors.New("relationship is not a recognised value")
	ErrInvalidAmount       = errors.New("amount must be a valid non-negative number")
	ErrAlreadyEnrolled     = errors.New("employee already has a pending or active enrollment in this plan")
	ErrWrongStatus         = errors.New("action not allowed in the enrollment's current status")
	ErrAccessDenied        = errors.New("access denied")
)
