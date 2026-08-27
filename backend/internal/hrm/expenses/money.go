// backend/internal/hrm/expenses/money.go
package expenses

import "github.com/shopspring/decimal"

// ConvertToBase applies a snapshotted exchange rate. Trivial arithmetic, but
// it is the ONE place the conversion happens, so a rate is never applied
// twice or inverted by accident.
//
// The rate is frozen onto the line at claim time and never re-derived —
// there is no FX rate table in this codebase (Phase 11 scope), and a later
// rate change must not rewrite a settled claim. See migration 00108's header.
//
// A rate of 1 (the default) makes base_amount == amount, so a
// single-currency org never encounters any of this.
func ConvertToBase(amount, exchangeRate decimal.Decimal) decimal.Decimal {
	if exchangeRate.IsZero() || exchangeRate.IsNegative() {
		// Guarded by a CHECK too; returning the unconverted amount is safer
		// than returning zero, which would silently erase a real expense.
		return amount.Round(2)
	}
	return amount.Mul(exchangeRate).Round(2)
}

// MileageAmount computes distance x rate for a mileage line.
func MileageAmount(distance, ratePerUnit decimal.Decimal) decimal.Decimal {
	if distance.IsNegative() || ratePerUnit.IsNegative() {
		return decimal.Zero
	}
	return distance.Mul(ratePerUnit).Round(2)
}

// PerDiemAmount computes days x daily rate. Days is inclusive of both the
// start and end date: a one-day trip earns one day's per diem, not zero.
func PerDiemAmount(days int, dailyAmount decimal.Decimal) decimal.Decimal {
	if days <= 0 || dailyAmount.IsNegative() {
		return decimal.Zero
	}
	return dailyAmount.Mul(decimal.NewFromInt(int64(days))).Round(2)
}

// ClaimTotals is the computed-not-stored answer to "what is this claim
// worth". hrm_expense_claims deliberately has no total columns — approval is
// per-line, so a stored total would drift the moment one line was adjusted.
// See migration 00108's header.
type ClaimTotals struct {
	// Claimed is the sum of every line's base_amount — what was asked for.
	Claimed decimal.Decimal
	// Approved is the sum of approved_amount over DECIDED lines only.
	Approved decimal.Decimal
	// Undecided counts lines whose approved_amount is still NULL. A claim
	// with any undecided line has not been fully reviewed, which is what
	// separates 'approved' from 'partially_approved'.
	Undecided int
	// Rejected counts lines explicitly decided at zero — a real outcome,
	// distinct from undecided, which is why approved_amount is nullable
	// rather than defaulting to 0.
	Rejected int
}

// SumClaim folds a claim's lines into its totals. Takes primitives rather
// than the row type so it is unit-testable directly without a database —
// the payslips.ComputeSlab / ReferencesGross precedent.
//
// approvedAmounts[i] == nil means line i is undecided.
func SumClaim(baseAmounts []decimal.Decimal, approvedAmounts []*decimal.Decimal) ClaimTotals {
	t := ClaimTotals{Claimed: decimal.Zero, Approved: decimal.Zero}
	for i, base := range baseAmounts {
		t.Claimed = t.Claimed.Add(base)
		if i >= len(approvedAmounts) || approvedAmounts[i] == nil {
			t.Undecided++
			continue
		}
		amt := *approvedAmounts[i]
		t.Approved = t.Approved.Add(amt)
		if amt.IsZero() {
			t.Rejected++
		}
	}
	return t
}

// SettlementOutcome names which of the three advance-settlement cases applies.
// The build plan requires all three be handled; none is an error.
type SettlementOutcome string

const (
	// SettlementExact — the advance covered the claim precisely.
	SettlementExact SettlementOutcome = "exact"
	// SettlementEmployeeOwes — the advance exceeded the claim, so the
	// employee returns the balance.
	SettlementEmployeeOwes SettlementOutcome = "employee_owes"
	// SettlementOrgOwes — the claim exceeded the advance, so the org pays
	// the difference (through payroll, via a reimbursement).
	SettlementOrgOwes SettlementOutcome = "org_owes"
)

// Settlement is the result of applying an approved claim against an advance.
type Settlement struct {
	Outcome SettlementOutcome
	// AppliedToAdvance is how much of the claim the advance absorbed —
	// min(approved, outstanding advance).
	AppliedToAdvance decimal.Decimal
	// PayableToEmployee is what still has to reach the employee, and is what
	// becomes a reimbursement. Zero in the exact and employee-owes cases.
	PayableToEmployee decimal.Decimal
	// RecoverableFromEmployee is what the employee must return. Zero unless
	// the advance exceeded the claim.
	RecoverableFromEmployee decimal.Decimal
}

// SettleAgainstAdvance divides an approved claim total between an outstanding
// advance and what is still owed either way. Pure, and tested directly:
// getting this wrong either pays an employee twice or silently keeps money
// that is theirs.
//
// outstandingAdvance is the advance's amount MINUS what it has already
// settled, so a second claim against the same advance sees only what is left.
func SettleAgainstAdvance(approvedTotal, outstandingAdvance decimal.Decimal) Settlement {
	if outstandingAdvance.IsNegative() {
		outstandingAdvance = decimal.Zero
	}
	if approvedTotal.IsNegative() {
		approvedTotal = decimal.Zero
	}

	switch {
	case approvedTotal.Equal(outstandingAdvance):
		return Settlement{
			Outcome: SettlementExact, AppliedToAdvance: approvedTotal,
			PayableToEmployee: decimal.Zero, RecoverableFromEmployee: decimal.Zero,
		}
	case approvedTotal.LessThan(outstandingAdvance):
		return Settlement{
			Outcome: SettlementEmployeeOwes, AppliedToAdvance: approvedTotal,
			PayableToEmployee:       decimal.Zero,
			RecoverableFromEmployee: outstandingAdvance.Sub(approvedTotal),
		}
	default:
		return Settlement{
			Outcome: SettlementOrgOwes, AppliedToAdvance: outstandingAdvance,
			PayableToEmployee:       approvedTotal.Sub(outstandingAdvance),
			RecoverableFromEmployee: decimal.Zero,
		}
	}
}
