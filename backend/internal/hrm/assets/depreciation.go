// backend/internal/hrm/assets/depreciation.go
package assets

import (
	"time"

	"github.com/shopspring/decimal"
)

// BookValue computes straight-line depreciated book value — a STUB, and
// deliberately so. Real fixed-asset accounting (revaluation, disposal
// gain/loss, tax schedules, declining-balance methods) belongs to the
// Accounting module; the build plan says so directly. What this gives an HRM
// user is "roughly what is this laptop still worth", nothing more.
//
// Computed on every read, never stored — the 00076 rule. A stored book_value
// would be wrong the day after it was written.
//
// Exported and pure (no DB, no service receiver) so it is unit-testable
// directly: the payslips.ComputeSlab / compensation.ApplyIncrease /
// loans.Amortize precedent — arithmetic that a user will read as a number
// gets tested before anything that calls it.
//
// Rules, each of which has a test:
//   - A nil useful life means "not depreciated": the asset holds its purchase
//     cost forever. Dividing by a zero-or-absent life is the obvious crash.
//   - A nil purchase date means depreciation has not started: full cost.
//   - Book value FLOORS AT ZERO. An asset past its useful life is worth
//     nothing, not a negative number — unlike net pay (r25), where a negative
//     is a real, meaningful outcome that must survive. Here it is arithmetic
//     running off the end of a schedule, so clamping is correct rather than
//     concealing.
//   - A future purchase date yields full cost, not a negative elapsed term.
func BookValue(purchaseCost decimal.Decimal, purchaseDate *time.Time, usefulLifeMonths *int, asOf time.Time) decimal.Decimal {
	if usefulLifeMonths == nil || *usefulLifeMonths <= 0 || purchaseDate == nil {
		return purchaseCost
	}
	if purchaseCost.IsZero() || purchaseCost.IsNegative() {
		return purchaseCost
	}

	elapsed := monthsBetween(*purchaseDate, asOf)
	if elapsed <= 0 {
		return purchaseCost
	}
	if elapsed >= *usefulLifeMonths {
		return decimal.Zero
	}

	perMonth := purchaseCost.Div(decimal.NewFromInt(int64(*usefulLifeMonths)))
	depreciated := perMonth.Mul(decimal.NewFromInt(int64(elapsed)))
	remaining := purchaseCost.Sub(depreciated)
	if remaining.IsNegative() {
		return decimal.Zero
	}
	return remaining.Round(2)
}

// monthsBetween counts whole elapsed months from start to end. Day-of-month
// is honoured: a purchase on the 31st has not completed a month on the 1st.
func monthsBetween(start, end time.Time) int {
	months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
	if end.Day() < start.Day() {
		months--
	}
	return months
}
