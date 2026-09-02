// backend/internal/tests/unit/hrm/loans/amortization_test.go
// Amortize is the arithmetic that decides real money — every employee loan's
// entire repayment schedule is frozen from this single call at disbursement
// (migration 00100) and never recomputed. Tested here, before anything that
// calls it, the payslips.ComputeSlab / compensation.ApplyIncrease precedent.
package loans_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/loans"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal literal %q: %v", s, err)
	}
	return d
}

func sumPrincipal(plan []loans.InstallmentPlan) decimal.Decimal {
	sum := decimal.Zero
	for _, p := range plan {
		sum = sum.Add(p.PrincipalComponent)
	}
	return sum
}

func sumTotal(plan []loans.InstallmentPlan) decimal.Decimal {
	sum := decimal.Zero
	for _, p := range plan {
		sum = sum.Add(p.TotalAmount)
	}
	return sum
}

func TestAmortize_InterestFree_EvenlyDivisible(t *testing.T) {
	plan := loans.Amortize(dec(t, "12000"), decimal.Zero, 12, 2, "half_up")
	if len(plan) != 12 {
		t.Fatalf("expected 12 installments, got %d", len(plan))
	}
	for i, p := range plan {
		if !p.InterestComponent.IsZero() {
			t.Errorf("installment %d: interest = %s, want 0 for an interest-free loan", i+1, p.InterestComponent)
		}
		if !p.TotalAmount.Equal(dec(t, "1000")) {
			t.Errorf("installment %d: total = %s, want 1000", i+1, p.TotalAmount)
		}
	}
	if !sumPrincipal(plan).Equal(dec(t, "12000")) {
		t.Errorf("sum of principal components = %s, want exactly 12000", sumPrincipal(plan))
	}
}

func TestAmortize_InterestFree_LastInstallmentAbsorbsRemainder(t *testing.T) {
	// 10000 / 3 = 3333.333... — the classic case where naive per-installment
	// division loses a cent somewhere. The schedule must still sum exactly.
	plan := loans.Amortize(dec(t, "10000"), decimal.Zero, 3, 2, "half_up")
	if len(plan) != 3 {
		t.Fatalf("expected 3 installments, got %d", len(plan))
	}
	if !sumTotal(plan).Equal(dec(t, "10000")) {
		t.Errorf("sum of all installments = %s, want exactly 10000 (no cent lost to rounding)", sumTotal(plan))
	}
	// First two installments are the rounded EMI; the third absorbs whatever
	// is left, which need not equal the rounded EMI.
	if !plan[0].TotalAmount.Equal(plan[1].TotalAmount) {
		t.Errorf("first two installments should match: %s vs %s", plan[0].TotalAmount, plan[1].TotalAmount)
	}
}

func TestAmortize_WithInterest_PrincipalFullyRecovered(t *testing.T) {
	// 12% annual, 12 months, principal 12000. The reducing-balance schedule
	// must recover EXACTLY the principal — no more, no less — regardless of
	// per-installment rounding.
	plan := loans.Amortize(dec(t, "12000"), dec(t, "12"), 12, 2, "half_up")
	if len(plan) != 12 {
		t.Fatalf("expected 12 installments, got %d", len(plan))
	}
	if !sumPrincipal(plan).Equal(dec(t, "12000")) {
		t.Errorf("sum of principal components = %s, want exactly 12000", sumPrincipal(plan))
	}
	// Interest must strictly decrease installment to installment as the
	// balance reduces (reducing-balance, not flat-rate).
	for i := 1; i < len(plan); i++ {
		if plan[i].InterestComponent.GreaterThan(plan[i-1].InterestComponent) {
			t.Errorf("interest increased from installment %d (%s) to %d (%s) — not reducing-balance",
				i, plan[i-1].InterestComponent, i+1, plan[i].InterestComponent)
		}
	}
	// EMI (total per installment) should be level across all but the last.
	for i := 1; i < len(plan)-1; i++ {
		if !plan[i].TotalAmount.Equal(plan[0].TotalAmount) {
			t.Errorf("installment %d total = %s, want level EMI %s", i+1, plan[i].TotalAmount, plan[0].TotalAmount)
		}
	}
}

func TestAmortize_SingleInstallment(t *testing.T) {
	plan := loans.Amortize(dec(t, "1000"), dec(t, "12"), 1, 2, "half_up")
	if len(plan) != 1 {
		t.Fatalf("expected 1 installment, got %d", len(plan))
	}
	if !plan[0].PrincipalComponent.Equal(dec(t, "1000")) {
		t.Errorf("single installment must recover the full principal, got %s", plan[0].PrincipalComponent)
	}
}

func TestAmortize_InvalidInputsReturnNil(t *testing.T) {
	cases := []struct {
		name      string
		principal decimal.Decimal
		tenure    int
	}{
		{"zero tenure", dec(t, "1000"), 0},
		{"negative tenure", dec(t, "1000"), -1},
		{"zero principal", decimal.Zero, 12},
		{"negative principal", dec(t, "-1000"), 12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := loans.Amortize(c.principal, decimal.Zero, c.tenure, 2, "half_up")
			if plan != nil {
				t.Errorf("expected nil plan for invalid input, got %d rows", len(plan))
			}
		})
	}
}

func TestAmortize_BalanceNeverGoesNegative(t *testing.T) {
	// A pathological rate/tenure combination where a single EMI slice cannot
	// cover its own interest must not let the balance grow or go negative —
	// capped principal_component, documented in the doc comment.
	plan := loans.Amortize(dec(t, "1000"), dec(t, "500"), 24, 2, "half_up")
	balance := dec(t, "1000")
	for i, p := range plan {
		balance = balance.Sub(p.PrincipalComponent)
		if balance.IsNegative() {
			t.Fatalf("installment %d drove the balance negative: %s", i+1, balance)
		}
	}
	if !balance.IsZero() {
		t.Errorf("final balance = %s, want exactly zero", balance)
	}
}
