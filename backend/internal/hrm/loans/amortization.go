// backend/internal/hrm/loans/amortization.go
package loans

import "github.com/shopspring/decimal"

// InstallmentPlan is one row of a generated amortization schedule, before
// any DB identity exists. Amortize returns a slice of these; the service
// layer is what turns them into hrm_loan_schedules rows.
type InstallmentPlan struct {
	InstallmentNumber  int
	PrincipalComponent decimal.Decimal
	InterestComponent  decimal.Decimal
	TotalAmount        decimal.Decimal
}

// Amortize computes a reducing-balance amortization schedule for a loan.
// Exported and pure — no DB, no service receiver — so it is unit-testable
// directly, the payslips.ComputeSlab / compensation.ApplyIncrease precedent:
// the arithmetic that decides real money gets tested before anything that
// calls it. Called exactly ONCE, at disbursement — see migration 00100's
// header on why the schedule this produces is never recomputed afterward.
//
// interestRatePct is an ANNUAL percentage; the monthly rate used internally
// is interestRatePct/100/12. tenureMonths must be > 0.
//
// The EMI (equal monthly installment) formula for a reducing-balance loan is
//
//	EMI = P * r * (1+r)^n / ((1+r)^n - 1)
//
// where r is the monthly rate and n is tenureMonths. (1+r)^n is computed by
// repeated multiplication rather than decimal.Decimal's own exponentiation —
// n is small (a tenure in months, realistically well under a few hundred)
// and a multiplication loop keeps every intermediate value exact decimal
// arithmetic with no floating-point pass-through.
//
// When interestRatePct is zero, EMI degenerates to principal/n (an
// interest-free loan repaid in equal installments) — handled as its own
// branch rather than letting r=0 flow through the general formula, which
// would divide by (1+0)^n - 1 = 0.
//
// The LAST installment always absorbs whatever balance and rounding
// remainder is left, rather than being computed as "one more EMI slice" —
// otherwise per-installment rounding (r25/00098's roundDecimal precedent)
// would leave the schedule's sum a few cents off from the principal, either
// short-changing the org or over-charging the employee by the accumulated
// rounding error. This is the same boundary reasoning ComputeSlab
// (hrm/payslips) uses for its final open-ended slab.
func Amortize(principal, interestRatePct decimal.Decimal, tenureMonths int, roundingScale int32, roundingMode string) []InstallmentPlan {
	if tenureMonths <= 0 || principal.IsNegative() || principal.IsZero() {
		return nil
	}

	monthlyRate := interestRatePct.Div(decimal.NewFromInt(100)).Div(decimal.NewFromInt(12))

	var emi decimal.Decimal
	if monthlyRate.IsZero() {
		emi = roundDecimal(principal.Div(decimal.NewFromInt(int64(tenureMonths))), roundingScale, roundingMode)
	} else {
		one := decimal.NewFromInt(1)
		factor := one.Add(monthlyRate)
		compounded := one
		for i := 0; i < tenureMonths; i++ {
			compounded = compounded.Mul(factor)
		}
		numerator := principal.Mul(monthlyRate).Mul(compounded)
		denominator := compounded.Sub(one)
		emi = roundDecimal(numerator.Div(denominator), roundingScale, roundingMode)
	}

	plan := make([]InstallmentPlan, 0, tenureMonths)
	balance := principal
	for i := 1; i <= tenureMonths; i++ {
		interest := roundDecimal(balance.Mul(monthlyRate), roundingScale, roundingMode)

		var principalComp decimal.Decimal
		if i == tenureMonths {
			// Absorb whatever remains — see doc comment.
			principalComp = balance
		} else {
			principalComp = emi.Sub(interest)
			if principalComp.IsNegative() {
				// Only reachable with a pathological rate/tenure combination
				// where a single EMI slice cannot even cover its own
				// interest. Treat it as "nothing recovered from principal
				// this month" rather than letting balance grow, which the
				// fixed EMI formula never intends.
				principalComp = decimal.Zero
			}
			if principalComp.GreaterThan(balance) {
				principalComp = balance
			}
		}

		total := principalComp.Add(interest)
		balance = balance.Sub(principalComp)

		plan = append(plan, InstallmentPlan{
			InstallmentNumber: i, PrincipalComponent: principalComp,
			InterestComponent: interest, TotalAmount: total,
		})
	}
	return plan
}

func roundDecimal(d decimal.Decimal, scale int32, mode string) decimal.Decimal {
	switch mode {
	case "half_up":
		return d.Round(scale)
	case "half_even":
		return d.RoundBank(scale)
	case "down":
		return d.RoundDown(scale)
	case "up":
		return d.RoundUp(scale)
	case "ceiling":
		return d.RoundCeil(scale)
	case "floor":
		return d.RoundFloor(scale)
	default:
		return d.Round(scale)
	}
}
