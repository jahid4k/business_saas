// backend/internal/hrm/succession/model.go
package succession

import (
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrAccessDenied        = errors.New("succession: access denied")
	ErrEmployeeNotFound    = errors.New("succession: employee not found")
	ErrPositionNotFound    = errors.New("succession: position not found")
	ErrCriticalNotFound    = errors.New("succession: critical position not found")
	ErrCandidateNotFound   = errors.New("succession: succession candidate not found")
	ErrPlanNotFound        = errors.New("succession: development plan not found")
	ErrItemNotFound        = errors.New("succession: development plan item not found")
	ErrAlreadyDesignated   = errors.New("succession: this position is already designated critical")
	ErrAlreadyNominated    = errors.New("succession: this employee is already an active candidate for this position")
	ErrAlreadyWithdrawn    = errors.New("succession: this nomination is no longer active")
	ErrInvalidBand         = errors.New("succession: performance and potential must each be low, medium or high")
	ErrRationaleRequired   = errors.New("succession: a potential band must state its rationale")
	ErrInvalidReadiness    = errors.New("succession: invalid readiness level")
	ErrInvalidCriticality  = errors.New("succession: invalid criticality level or vacancy risk")
	ErrNoEmployeeRecord    = errors.New("succession: the caller has no employee record in this organization")
	ErrTitleRequired       = errors.New("succession: a development plan needs a title")
	ErrDescriptionRequired = errors.New("succession: a development plan item needs a description")
	ErrInvalidStatus       = errors.New("succession: invalid status")
)

// ── Critical positions ───────────────────────────────────────────────────────

type Criticality string

const (
	MissionCritical Criticality = "mission_critical"
	HighCriticality Criticality = "high"
	ModerateCrit    Criticality = "moderate"
)

func (c Criticality) IsValid() bool {
	switch c {
	case MissionCritical, HighCriticality, ModerateCrit:
		return true
	}
	return false
}

type VacancyRisk string

const (
	RiskHigh   VacancyRisk = "high"
	RiskMedium VacancyRisk = "medium"
	RiskLow    VacancyRisk = "low"
)

func (v VacancyRisk) IsValid() bool {
	switch v {
	case RiskHigh, RiskMedium, RiskLow:
		return true
	}
	return false
}

// CriticalPosition is a designated role. This is the only part of succession
// that describes the ORGANIZATION rather than a named person, which is why
// it is the only part a manager may read.
type CriticalPosition struct {
	ID               string      `json:"id"`
	PublicID         string      `json:"public_id"`
	OrgID            string      `json:"org_id"`
	PositionID       string      `json:"position_id"`
	PositionTitle    string      `json:"position_title,omitempty"`
	CriticalityLevel Criticality `json:"criticality_level"`
	VacancyRisk      VacancyRisk `json:"vacancy_risk"`
	ImpactOfVacancy  *string     `json:"impact_of_vacancy,omitempty"`
	IdentifiedBy     string      `json:"identified_by"`
	ReviewDueDate    *time.Time  `json:"review_due_date,omitempty"`
	IsActive         bool        `json:"is_active"`
	DeactivatedAt    *time.Time  `json:"deactivated_at,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`

	// IncumbentCount is derived from hrm_employees.position_id at read time
	// rather than stored — an incumbent column would go stale on every
	// transfer with nothing to detect it (the 00076 rule).
	IncumbentCount int `json:"incumbent_count"`
	// ActiveCandidates is the bench depth. Zero on a mission-critical role
	// is the number this whole table exists to surface.
	ActiveCandidates int `json:"active_candidates"`
}

type CreateCriticalPositionRequest struct {
	PositionID       string  `json:"position_id"`
	CriticalityLevel string  `json:"criticality_level"`
	VacancyRisk      string  `json:"vacancy_risk"`
	ImpactOfVacancy  *string `json:"impact_of_vacancy"`
	ReviewDueDate    *string `json:"review_due_date"`
}

type UpdateCriticalPositionRequest struct {
	CriticalityLevel *string `json:"criticality_level"`
	VacancyRisk      *string `json:"vacancy_risk"`
	ImpactOfVacancy  *string `json:"impact_of_vacancy"`
	ReviewDueDate    *string `json:"review_due_date"`
	IsActive         *bool   `json:"is_active"`
}

// ── Talent assessments (CONFIDENTIAL) ────────────────────────────────────────

// TalentAssessment is one 9-box placement.
//
// ⚠ There is no Box field on the struct and no box_number column behind it.
// The box is computed by Box(PerformanceBand, PotentialBand) at read time,
// so a stored box can never disagree with the bands it claims to summarise.
type TalentAssessment struct {
	ID                        string           `json:"id"`
	PublicID                  string           `json:"public_id"`
	OrgID                     string           `json:"org_id"`
	EmployeeID                string           `json:"employee_id"`
	AsOfDate                  time.Time        `json:"as_of_date"`
	PerformanceBand           Band             `json:"performance_band"`
	PerformanceAppraisalID    *string          `json:"performance_appraisal_id,omitempty"`
	PerformanceRatingSnapshot *decimal.Decimal `json:"performance_rating_snapshot,omitempty"`
	PotentialBand             Band             `json:"potential_band"`
	PotentialRationale        string           `json:"potential_rationale"`
	AssessedBy                string           `json:"assessed_by"`
	CreatedAt                 time.Time        `json:"created_at"`
	UpdatedAt                 time.Time        `json:"updated_at"`
}

type RecordAssessmentRequest struct {
	EmployeeID string `json:"employee_id"`
	AsOfDate   string `json:"as_of_date"`
	// PerformanceBand may be left empty to have it derived from the
	// employee's most recent published appraisal. PotentialBand never can be.
	PerformanceBand    string `json:"performance_band"`
	PotentialBand      string `json:"potential_band"`
	PotentialRationale string `json:"potential_rationale"`
}

// GridCell is one occupied cell of the 9-box grid.
type GridCell struct {
	Box         int      `json:"box"`
	Label       string   `json:"label"`
	Performance Band     `json:"performance_band"`
	Potential   Band     `json:"potential_band"`
	EmployeeIDs []string `json:"employee_ids"`
}

// ── Succession candidates (CONFIDENTIAL) ─────────────────────────────────────

type Readiness string

const (
	ReadyNow       Readiness = "ready_now"
	Ready1To2Years Readiness = "ready_1_2_years"
	Ready3To5Years Readiness = "ready_3_5_years"
	EmergencyCover Readiness = "emergency_cover"
)

func (r Readiness) IsValid() bool {
	switch r {
	case ReadyNow, Ready1To2Years, Ready3To5Years, EmergencyCover:
		return true
	}
	return false
}

type Candidate struct {
	ID                  string     `json:"id"`
	PublicID            string     `json:"public_id"`
	OrgID               string     `json:"org_id"`
	CriticalPositionID  string     `json:"critical_position_id"`
	EmployeeID          string     `json:"employee_id"`
	EmployeeName        string     `json:"employee_name,omitempty"`
	Readiness           Readiness  `json:"readiness"`
	NominationRationale *string    `json:"nomination_rationale,omitempty"`
	DevelopmentPlanID   *string    `json:"development_plan_id,omitempty"`
	Status              string     `json:"status"`
	WithdrawnAt         *time.Time `json:"withdrawn_at,omitempty"`
	WithdrawnReason     *string    `json:"withdrawn_reason,omitempty"`
	NominatedBy         string     `json:"nominated_by"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type NominateRequest struct {
	EmployeeID          string  `json:"employee_id"`
	Readiness           string  `json:"readiness"`
	NominationRationale *string `json:"nomination_rationale"`
	DevelopmentPlanID   *string `json:"development_plan_id"`
}

type WithdrawRequest struct {
	Reason *string `json:"reason"`
}

// ── Development plans (SUBJECT-VISIBLE) ──────────────────────────────────────

// DevelopmentPlan is the only succession record an employee may read about
// themselves.
//
// ⚠ It deliberately carries NO plan type and NO reference to a nomination.
// Either would tell the subject, through a field they are entitled to read,
// that they are a named successor for a specific role. The FK runs the other
// way: Candidate.DevelopmentPlanID points here.
type DevelopmentPlan struct {
	ID          string     `json:"id"`
	PublicID    string     `json:"public_id"`
	OrgID       string     `json:"org_id"`
	EmployeeID  string     `json:"employee_id"`
	Title       string     `json:"title"`
	Objective   *string    `json:"objective,omitempty"`
	TargetDate  *time.Time `json:"target_date,omitempty"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Items []*PlanItem `json:"items,omitempty"`
}

type PlanItem struct {
	ID          string     `json:"id"`
	PublicID    string     `json:"public_id"`
	OrgID       string     `json:"org_id"`
	PlanID      string     `json:"plan_id"`
	Description string     `json:"description"`
	TargetDate  *time.Time `json:"target_date,omitempty"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	SortOrder   int        `json:"sort_order"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

var planStatuses = map[string]bool{"draft": true, "active": true, "completed": true, "cancelled": true}
var itemStatuses = map[string]bool{"pending": true, "in_progress": true, "completed": true, "cancelled": true}

type CreatePlanRequest struct {
	EmployeeID string  `json:"employee_id"`
	Title      string  `json:"title"`
	Objective  *string `json:"objective"`
	TargetDate *string `json:"target_date"`
	Status     *string `json:"status"`
}

type UpdatePlanRequest struct {
	Title      *string `json:"title"`
	Objective  *string `json:"objective"`
	TargetDate *string `json:"target_date"`
	Status     *string `json:"status"`
}

type CreateItemRequest struct {
	Description string  `json:"description"`
	TargetDate  *string `json:"target_date"`
	SortOrder   *int    `json:"sort_order"`
}

type UpdateItemRequest struct {
	Description *string `json:"description"`
	TargetDate  *string `json:"target_date"`
	Status      *string `json:"status"`
	SortOrder   *int    `json:"sort_order"`
}

// ── The two read paths ───────────────────────────────────────────────────────
//
// ⚠ THESE TWO TYPES ARE THE CONFIDENTIALITY MECHANISM, AND THEY SHARE NO
// FIELD BEYOND THE EMPLOYEE ID THAT IDENTIFIES WHO IS BEING DESCRIBED.
//
// The build plan assumed field-level filtering as the enabler. That was
// never built, so confidentiality here is structural instead: SubjectView is
// populated by a repository method whose SQL never touches
// hrm_talent_assessments or hrm_succession_candidates, and never computes a
// flight-risk signal. There is therefore nothing loaded into memory for a
// later handler to forget to strip — the same shape as 5C's 360 anonymity,
// 8C's internal ticket comments and 9C's exit interview responses.
//
// Adding an assessment, nomination or signal field to SubjectView would
// defeat the entire design, and the integration test walks this type
// reflectively to make that impossible to do quietly.

// SubjectView is what an employee sees about their own development.
type SubjectView struct {
	EmployeeID string             `json:"employee_id"`
	Plans      []*DevelopmentPlan `json:"plans"`
}

// ReviewerView is what a holder of hrm.succession.view_confidential sees.
type ReviewerView struct {
	EmployeeID  string            `json:"employee_id"`
	DisplayName string            `json:"display_name"`
	Assessment  *TalentAssessment `json:"assessment,omitempty"`
	NineBox     *NineBox          `json:"nine_box,omitempty"`
	// FlightRisk is a list of explained facts and deliberately has no
	// accompanying score, level or total.
	FlightRisk  []Signal     `json:"flight_risk"`
	Nominations []*Candidate `json:"nominations"`
}

// ── helpers ──────────────────────────────────────────────────────────────────

func parseDate(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", strings.TrimSpace(*s))
	if err != nil {
		return nil, err
	}
	return &d, nil
}
