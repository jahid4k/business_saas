// backend/internal/hrm/exits/gratuity.go
package exits

import (
	"time"

	"github.com/shopspring/decimal"
)

// GratuityRule is the effective-dated policy in force for a settlement.
// Mirrors hrm_gratuity_rules; kept as a value type so the arithmetic below is
// pure and testable without a database.
type GratuityRule struct {
	MinYearsOfService   decimal.Decimal
	DaysPerYear         decimal.Decimal
	BaseComponent       string // "basic" | "gross"
	MonthlyDivisor      decimal.Decimal
	ForfeitOnMisconduct bool
}

// DailyRate converts a monthly figure to a daily one using the rule's own
// divisor.
//
// The divisor is a POLICY CHOICE, not an obvious fact — 30 is the common
// statutory convention, 26 excludes weekly offs — which is why it is stored
// per rule rather than hard-coded. A zero or negative divisor yields zero
// rather than dividing by zero: a misconfigured rule must not take down a
// settlement run.
func DailyRate(monthlyAmount, divisor decimal.Decimal) decimal.Decimal {
	if !divisor.IsPositive() || monthlyAmount.IsNegative() {
		return decimal.Zero
	}
	return monthlyAmount.Div(divisor)
}

// YearsOfService returns completed years between hire and last working date,
// as a decimal so a partial final year is representable.
//
// Returns zero rather than a negative when the dates are inverted — a
// negative tenure would flow into the eligibility comparison and produce
// nonsense. Uses 365.25 days per year so a leap year inside a long tenure
// does not quietly shave a fraction off the entitlement.
func YearsOfService(hireDate, lastWorkingDate time.Time) decimal.Decimal {
	hire := truncateToDay(hireDate)
	last := truncateToDay(lastWorkingDate)
	if !last.After(hire) {
		return decimal.Zero
	}
	days := decimal.NewFromInt(int64(last.Sub(hire).Hours() / 24))
	return days.Div(decimal.NewFromFloat(365.25))
}

// GratuityResult explains an entitlement rather than just stating it. A
// settlement figure a departing employee disputes has to be defensible line
// by line, and "zero" in particular needs a reason.
type GratuityResult struct {
	Amount         decimal.Decimal
	YearsOfService decimal.Decimal
	// CompletedYears is what the payout is actually calculated on — partial
	// years are NOT paid pro rata. See ComputeGratuity.
	CompletedYears int64
	DailyRate      decimal.Decimal
	Eligible       bool
	// Reason explains an ineligible or zero result: "below the 5 year
	// minimum", "forfeited for misconduct", "no rule configured".
	Reason string
}

// ComputeGratuity calculates a tenure gratuity entitlement.
//
// Rules with a defined answer, because each one is a real settlement that
// would otherwise be argued about:
//
//   - NO RULE CONFIGURED yields zero with a reason, never an error. An org
//     that has not set gratuity terms simply does not pay it, and a
//     settlement must still compute.
//   - BELOW THE MINIMUM yields zero with a reason. Not an error: failing to
//     qualify is an ordinary outcome, and an error would block the whole run.
//   - PARTIAL YEARS ARE NOT PAID. Entitlement is floor(years) × daysPerYear ×
//     dailyRate. This is the common statutory treatment, and paying a
//     fraction of a year would need a rounding rule nobody has specified —
//     silently choosing one would be inventing policy.
//   - MISCONDUCT FORFEITS only when the rule opts in. Forfeiture is legally
//     loaded, so it is never the default.
//   - Eligibility is checked against the FULL tenure including the partial
//     year, while payment uses only completed years. Someone with 4.9 years
//     against a 5-year minimum does not qualify; that is the minimum doing
//     its job, not a rounding artifact.
func ComputeGratuity(rule *GratuityRule, monthlyBase decimal.Decimal, hireDate, lastWorkingDate time.Time, forCauseMisconduct bool) GratuityResult {
	years := YearsOfService(hireDate, lastWorkingDate)

	if rule == nil {
		return GratuityResult{
			Amount: decimal.Zero, YearsOfService: years,
			Reason: "no gratuity rule is configured for this organization",
		}
	}
	if rule.ForfeitOnMisconduct && forCauseMisconduct {
		return GratuityResult{
			Amount: decimal.Zero, YearsOfService: years,
			Reason: "forfeited: termination for misconduct under the applicable gratuity rule",
		}
	}
	if years.LessThan(rule.MinYearsOfService) {
		return GratuityResult{
			Amount: decimal.Zero, YearsOfService: years,
			Reason: "below the minimum service period for gratuity",
		}
	}

	daily := DailyRate(monthlyBase, rule.MonthlyDivisor)
	completed := years.Floor()
	amount := completed.Mul(rule.DaysPerYear).Mul(daily)
	if amount.IsNegative() {
		amount = decimal.Zero
	}
	return GratuityResult{
		Amount:         amount,
		YearsOfService: years,
		CompletedYears: completed.IntPart(),
		DailyRate:      daily,
		Eligible:       true,
	}
}
