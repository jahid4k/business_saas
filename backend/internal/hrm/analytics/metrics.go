// backend/internal/hrm/analytics/metrics.go
package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// Computation names a Go implementation. It is the vocabulary
// hrm_metric_definitions.computation is CHECKed against.
//
// ⚠ These are NAMES, not expressions. hrm_metric_definitions is deliberately
// not a formula engine: this codebase already carries one interpreted-
// expression defect (learning's evalFormula, still on float64), and adding a
// second interpreter to solve a definitional problem would trade a small
// problem for a larger one. The PARAMETERS of each computation are data; the
// computation itself is code.
type Computation string

const (
	CompHeadcount          Computation = "headcount"
	CompAttritionRate      Computation = "attrition_rate"
	CompFirstYearAttrition Computation = "first_year_attrition"
	CompCohortRetention    Computation = "cohort_retention"
	CompTenureDistribution Computation = "tenure_distribution"
	CompDEIDistribution    Computation = "dei_distribution"
	CompCompDistribution   Computation = "compensation_distribution"
)

func (c Computation) IsValid() bool {
	switch c {
	case CompHeadcount, CompAttritionRate, CompFirstYearAttrition, CompCohortRetention,
		CompTenureDistribution, CompDEIDistribution, CompCompDistribution:
		return true
	}
	return false
}

// FirstYearDays is the boundary for first-year attrition. A leaver on their
// 365th day is a first-year loss; on day 366 they are not.
const FirstYearDays = 365

// TenureDays counts whole days between hire and exit.
func TenureDays(hire, exit time.Time) int {
	d := int(exit.Sub(hire).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// IsFirstYearExit reports whether a leaver went inside their first year.
func IsFirstYearExit(tenureDays int) bool { return tenureDays < FirstYearDays }

// AverageHeadcount is the denominator of the attrition rate: the mean of the
// opening and closing populations.
//
// Using closing headcount alone is the classic mistake — in a shrinking
// month it divides by the smaller number and reports an attrition rate higher
// than reality, which is precisely when somebody is looking at the chart.
func AverageHeadcount(opening, closing int) decimal.Decimal {
	if opening < 0 {
		opening = 0
	}
	if closing < 0 {
		closing = 0
	}
	return decimal.NewFromInt(int64(opening + closing)).Div(decimal.NewFromInt(2))
}

// AttritionRate returns leavers ÷ average headcount as a percentage, rounded
// to two places.
//
// ⚠ Returns ok=false when the denominator is zero rather than 0%. An
// organization with no people did not achieve perfect retention, and a chart
// that draws a confident zero for an empty period is worse than a gap.
func AttritionRate(leavers int, avgHeadcount decimal.Decimal) (decimal.Decimal, bool) {
	if leavers < 0 || avgHeadcount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, false
	}
	return decimal.NewFromInt(int64(leavers)).
		Div(avgHeadcount).
		Mul(decimal.NewFromInt(100)).
		Round(2), true
}

// CohortRetention returns the share of a hire cohort still employed, rounded
// to two places.
func CohortRetention(cohortSize, stillActive int) (decimal.Decimal, bool) {
	if cohortSize <= 0 || stillActive < 0 || stillActive > cohortSize {
		return decimal.Zero, false
	}
	return decimal.NewFromInt(int64(stillActive)).
		Div(decimal.NewFromInt(int64(cohortSize))).
		Mul(decimal.NewFromInt(100)).
		Round(2), true
}

// CohortMonth truncates a hire date to the first of its month, which is the
// grain cohort retention is reported at.
func CohortMonth(hire time.Time) time.Time {
	return time.Date(hire.Year(), hire.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// IsVoluntaryExit classifies an exit for the voluntary/involuntary split.
//
// A resignation is voluntary by construction — nobody resigns on somebody
// else's behalf. A termination is voluntary only where the termination type
// says the person chose to go; 'retirement' counts as voluntary and
// 'contract_end' does not, because a contract ending on its own terms was
// nobody's decision on the day.
func IsVoluntaryExit(sourceType, terminationType string) bool {
	if sourceType == "resignation" {
		return true
	}
	switch terminationType {
	case "voluntary", "retirement":
		return true
	default:
		return false
	}
}

// RegrettedFromRehireStatus maps Phase 9's rehire eligibility onto the
// regretted/non-regretted split.
//
// ⚠ RETURNS nil FOR UNKNOWN, AND THAT NIL MUST SURVIVE TO THE DATABASE.
// 'conditional', or no rehire decision at all, means nobody has judged the
// departure yet. Collapsing that to "not regretted" would report every
// un-reviewed exit as a good riddance and flatter the exact number this
// metric exists to expose. hrm_attrition_facts.is_regretted is nullable for
// this reason.
func RegrettedFromRehireStatus(status string, present bool) *bool {
	if !present {
		return nil
	}
	yes, no := true, false
	switch status {
	case "eligible":
		return &yes
	case "not_eligible":
		return &no
	default: // "conditional" and anything unrecognised
		return nil
	}
}

// TenureBucket labels a tenure in days for the tenure distribution. Buckets
// rather than a mean, because a mean tenure hides the shape that matters:
// twenty people at ten years and twenty at six months average out to a
// number describing nobody.
func TenureBucket(days int) string {
	switch {
	case days < 0:
		return "unknown"
	case days < 180:
		return "under_6_months"
	case days < FirstYearDays:
		return "6_to_12_months"
	case days < 2*FirstYearDays:
		return "1_to_2_years"
	case days < 5*FirstYearDays:
		return "2_to_5_years"
	default:
		return "over_5_years"
	}
}
