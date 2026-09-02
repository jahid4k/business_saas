// backend/internal/hrm/analytics/model.go
package analytics

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrAccessDenied       = errors.New("analytics: access denied")
	ErrMetricNotFound     = errors.New("analytics: metric definition not found")
	ErrDuplicateMetric    = errors.New("analytics: a metric with this key already exists")
	ErrInvalidComputation = errors.New("analytics: unknown computation")
	ErrInvalidGrain       = errors.New("analytics: grain must be org, department or legal_entity")
	ErrKeyRequired        = errors.New("analytics: a metric needs a key and a name")
	ErrStatementRequired  = errors.New("analytics: a metric definition must state its formula in words")
	ErrThresholdTooLow    = errors.New("analytics: a suppression threshold below 2 discloses an individual")
	ErrInvalidPeriod      = errors.New("analytics: from must not be after to")
)

// Grain is the dimension a metric is reported at.
type Grain string

const (
	GrainOrg         Grain = "org"
	GrainDepartment  Grain = "department"
	GrainLegalEntity Grain = "legal_entity"
)

func (g Grain) IsValid() bool {
	switch g {
	case GrainOrg, GrainDepartment, GrainLegalEntity:
		return true
	}
	return false
}

// MetricDefinition is what a metric MEANS, stored as data so two consumers
// cannot compute "attrition" two ways.
//
// ⚠ FormulaStatement is documentation with a NOT NULL constraint on it. It is
// never parsed. Computation names the Go implementation; the fields below it
// are that implementation's parameters, and those are the part that is
// genuinely configurable.
type MetricDefinition struct {
	ID                    string      `json:"id"`
	PublicID              string      `json:"public_id"`
	OrgID                 string      `json:"org_id"`
	MetricKey             string      `json:"metric_key"`
	Name                  string      `json:"name"`
	Description           *string     `json:"description,omitempty"`
	Computation           Computation `json:"computation"`
	FormulaStatement      string      `json:"formula_statement"`
	Grain                 Grain       `json:"grain"`
	AttritionTypes        []string    `json:"attrition_types,omitempty"`
	IncludeProbationExits bool        `json:"include_probation_exits"`
	SuppressionThreshold  int         `json:"suppression_threshold"`
	IsActive              bool        `json:"is_active"`
	CreatedBy             string      `json:"created_by"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

type CreateMetricRequest struct {
	MetricKey             string   `json:"metric_key"`
	Name                  string   `json:"name"`
	Description           *string  `json:"description"`
	Computation           string   `json:"computation"`
	FormulaStatement      string   `json:"formula_statement"`
	Grain                 string   `json:"grain"`
	AttritionTypes        []string `json:"attrition_types"`
	IncludeProbationExits *bool    `json:"include_probation_exits"`
	SuppressionThreshold  *int     `json:"suppression_threshold"`
}

type UpdateMetricRequest struct {
	Name                  *string  `json:"name"`
	Description           *string  `json:"description"`
	FormulaStatement      *string  `json:"formula_statement"`
	Grain                 *string  `json:"grain"`
	AttritionTypes        []string `json:"attrition_types"`
	IncludeProbationExits *bool    `json:"include_probation_exits"`
	SuppressionThreshold  *int     `json:"suppression_threshold"`
	IsActive              *bool    `json:"is_active"`
}

// HeadcountSnapshot is one nightly fact row.
//
// ⚠ The compensation figures are nil below the suppression threshold, and
// they are nil because the JOB DID NOT WRITE THEM — not because a read path
// blanked them. A small team's pay distribution is never recorded in the
// first place, so there is no stored value for a later query to expose.
type HeadcountSnapshot struct {
	ID                 string           `json:"id"`
	PublicID           string           `json:"public_id"`
	OrgID              string           `json:"org_id"`
	SnapshotDate       time.Time        `json:"snapshot_date"`
	Dimension          Grain            `json:"dimension"`
	DimensionID        *string          `json:"dimension_id,omitempty"`
	DimensionLabel     string           `json:"dimension_label,omitempty"`
	Headcount          int              `json:"headcount"`
	Joiners            int              `json:"joiners"`
	Leavers            int              `json:"leavers"`
	VoluntaryLeavers   int              `json:"voluntary_leavers"`
	InvoluntaryLeavers int              `json:"involuntary_leavers"`
	RegrettedLeavers   int              `json:"regretted_leavers"`
	AvgTenureDays      int              `json:"avg_tenure_days"`
	CompP25            *decimal.Decimal `json:"comp_p25,omitempty"`
	CompMedian         *decimal.Decimal `json:"comp_median,omitempty"`
	CompP75            *decimal.Decimal `json:"comp_p75,omitempty"`
	CompCurrency       *string          `json:"comp_currency,omitempty"`
	ComputedAt         time.Time        `json:"computed_at"`
}

// AttritionFact is one immutable row per exit, denormalized at build time so
// a later department rename or transfer cannot rewrite last March.
type AttritionFact struct {
	ID              string    `json:"id"`
	PublicID        string    `json:"public_id"`
	OrgID           string    `json:"org_id"`
	EmployeeID      string    `json:"employee_id"`
	ExitID          *string   `json:"exit_id,omitempty"`
	ExitDate        time.Time `json:"exit_date"`
	HireDate        time.Time `json:"hire_date"`
	CohortMonth     time.Time `json:"cohort_month"`
	TenureDays      int       `json:"tenure_days"`
	IsFirstYear     bool      `json:"is_first_year"`
	SourceType      string    `json:"source_type"`
	TerminationType *string   `json:"termination_type,omitempty"`
	IsVoluntary     bool      `json:"is_voluntary"`
	// Nil means nobody has judged this departure yet. See
	// RegrettedFromRehireStatus.
	IsRegretted   *bool     `json:"is_regretted"`
	DepartmentID  *string   `json:"department_id,omitempty"`
	PositionID    *string   `json:"position_id,omitempty"`
	LegalEntityID *string   `json:"legal_entity_id,omitempty"`
	Gender        *string   `json:"gender,omitempty"`
	ComputedAt    time.Time `json:"computed_at"`
}

// ── Read-path results ────────────────────────────────────────────────────────

// AttritionSummary is the four-way split the plan requires.
//
// ⚠ RegrettedUnknown is reported alongside the split rather than folded into
// NonRegretted. A regretted-attrition figure computed over a population where
// half the exits were never reviewed is a different number from one where all
// were, and the reader has to be able to tell which they are looking at.
type AttritionSummary struct {
	From              time.Time        `json:"from"`
	To                time.Time        `json:"to"`
	Leavers           int              `json:"leavers"`
	Voluntary         int              `json:"voluntary"`
	Involuntary       int              `json:"involuntary"`
	Regretted         int              `json:"regretted"`
	NonRegretted      int              `json:"non_regretted"`
	RegrettedUnknown  int              `json:"regretted_unknown"`
	FirstYearExits    int              `json:"first_year_exits"`
	OpeningHeadcount  int              `json:"opening_headcount"`
	ClosingHeadcount  int              `json:"closing_headcount"`
	AverageHeadcount  decimal.Decimal  `json:"average_headcount"`
	AttritionRate     *decimal.Decimal `json:"attrition_rate"`
	FirstYearRate     *decimal.Decimal `json:"first_year_attrition_rate"`
	ByTerminationType []Group          `json:"by_termination_type"`
}

// CohortRow is one hire cohort's retention.
type CohortRow struct {
	CohortMonth time.Time        `json:"cohort_month"`
	CohortSize  int              `json:"cohort_size"`
	StillActive int              `json:"still_active"`
	Retention   *decimal.Decimal `json:"retention_pct"`
}

// CompensationBand is one dimension's pay distribution, read from the
// snapshot. There is no per-employee path anywhere in this package.
type CompensationBand struct {
	SnapshotDate   time.Time        `json:"snapshot_date"`
	Dimension      Grain            `json:"dimension"`
	DimensionID    *string          `json:"dimension_id,omitempty"`
	DimensionLabel string           `json:"dimension_label,omitempty"`
	Headcount      int              `json:"headcount"`
	P25            *decimal.Decimal `json:"p25"`
	Median         *decimal.Decimal `json:"median"`
	P75            *decimal.Decimal `json:"p75"`
	Currency       *string          `json:"currency,omitempty"`
	Suppressed     bool             `json:"suppressed"`
}

// SnapshotResult reports what a job run did.
type SnapshotResult struct {
	SnapshotDate  time.Time `json:"snapshot_date"`
	OrgsProcessed int       `json:"orgs_processed"`
	RowsWritten   int       `json:"rows_written"`
	FactsWritten  int       `json:"facts_written"`
}
