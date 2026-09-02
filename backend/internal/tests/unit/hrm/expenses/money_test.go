// backend/internal/tests/unit/hrm/expenses/money_test.go
// The arithmetic that decides what an employee is actually paid back, and how
// an advance is divided. Pure (no DB) so it is tested here before anything
// calls it — the payslips.ComputeSlab / compensation.ApplyIncrease /
// loans.Amortize / assets.BookValue precedent.
package expenses_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/expenses"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal literal %q: %v", s, err)
	}
	return d
}

func decp(t *testing.T, s string) *decimal.Decimal {
	d := dec(t, s)
	return &d
}

func TestConvertToBase(t *testing.T) {
	cases := []struct{ name, amount, rate, want string }{
		{"rate of 1 is a no-op — the single-currency path", "100", "1", "100"},
		{"EUR to USD at 1.08", "100", "1.08", "108"},
		{"a rate below 1", "100", "0.79", "79"},
		{"rounds to cents", "33.33", "1.085", "36.16"},
		{"zero amount stays zero", "0", "1.08", "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := expenses.ConvertToBase(dec(t, c.amount), dec(t, c.rate))
			if !got.Equal(dec(t, c.want)) {
				t.Errorf("ConvertToBase(%s, %s) = %s, want %s", c.amount, c.rate, got, c.want)
			}
		})
	}

	t.Run("a zero or negative rate returns the amount unconverted, never zero", func(t *testing.T) {
		// Erasing a real expense is strictly worse than failing to convert it.
		for _, bad := range []string{"0", "-1.5"} {
			got := expenses.ConvertToBase(dec(t, "250"), dec(t, bad))
			if !got.Equal(dec(t, "250")) {
				t.Errorf("rate %s: got %s, want the unconverted 250", bad, got)
			}
		}
	})
}

func TestMileageAndPerDiem(t *testing.T) {
	t.Run("mileage is distance x rate", func(t *testing.T) {
		got := expenses.MileageAmount(dec(t, "120.5"), dec(t, "0.45"))
		if !got.Equal(dec(t, "54.23")) {
			t.Errorf("got %s, want 54.23", got)
		}
	})
	t.Run("negative distance yields zero, not a negative claim", func(t *testing.T) {
		if got := expenses.MileageAmount(dec(t, "-10"), dec(t, "0.45")); !got.IsZero() {
			t.Errorf("got %s, want 0", got)
		}
	})
	t.Run("a one-day trip earns one day of per diem, not zero", func(t *testing.T) {
		got := expenses.PerDiemAmount(1, dec(t, "75"))
		if !got.Equal(dec(t, "75")) {
			t.Errorf("got %s, want 75", got)
		}
	})
	t.Run("multi-day per diem", func(t *testing.T) {
		got := expenses.PerDiemAmount(5, dec(t, "75"))
		if !got.Equal(dec(t, "375")) {
			t.Errorf("got %s, want 375", got)
		}
	})
	t.Run("zero or negative days yields zero", func(t *testing.T) {
		for _, d := range []int{0, -3} {
			if got := expenses.PerDiemAmount(d, dec(t, "75")); !got.IsZero() {
				t.Errorf("days %d: got %s, want 0", d, got)
			}
		}
	})
}

func TestSumClaim_LineLevelApproval(t *testing.T) {
	t.Run("all lines undecided — nothing approved yet", func(t *testing.T) {
		totals := expenses.SumClaim(
			[]decimal.Decimal{dec(t, "100"), dec(t, "250")},
			[]*decimal.Decimal{nil, nil},
		)
		if !totals.Claimed.Equal(dec(t, "350")) {
			t.Errorf("claimed = %s, want 350", totals.Claimed)
		}
		if !totals.Approved.IsZero() {
			t.Errorf("approved = %s, want 0", totals.Approved)
		}
		if totals.Undecided != 2 {
			t.Errorf("undecided = %d, want 2", totals.Undecided)
		}
	})

	t.Run("reducing ONE line changes the total and leaves the others alone", func(t *testing.T) {
		// The whole point of line-level approval: the flight is fine, the
		// minibar is trimmed.
		totals := expenses.SumClaim(
			[]decimal.Decimal{dec(t, "400"), dec(t, "120"), dec(t, "60")},
			[]*decimal.Decimal{decp(t, "400"), decp(t, "40"), decp(t, "60")},
		)
		if !totals.Claimed.Equal(dec(t, "580")) {
			t.Errorf("claimed = %s, want 580", totals.Claimed)
		}
		if !totals.Approved.Equal(dec(t, "500")) {
			t.Errorf("approved = %s, want 500 (400 + trimmed 40 + 60)", totals.Approved)
		}
		if totals.Undecided != 0 {
			t.Errorf("undecided = %d, want 0", totals.Undecided)
		}
		if totals.Rejected != 0 {
			t.Errorf("rejected = %d, want 0 — a REDUCED line is not a rejected one", totals.Rejected)
		}
	})

	t.Run("a line decided at zero is REJECTED, not undecided", func(t *testing.T) {
		// This distinction is why approved_amount is nullable rather than
		// DEFAULT 0 — "not looked at" and "looked at, nothing payable" are
		// different states.
		totals := expenses.SumClaim(
			[]decimal.Decimal{dec(t, "100"), dec(t, "80")},
			[]*decimal.Decimal{decp(t, "100"), decp(t, "0")},
		)
		if totals.Rejected != 1 {
			t.Errorf("rejected = %d, want 1", totals.Rejected)
		}
		if totals.Undecided != 0 {
			t.Errorf("undecided = %d, want 0 — a zero decision IS a decision", totals.Undecided)
		}
		if !totals.Approved.Equal(dec(t, "100")) {
			t.Errorf("approved = %s, want 100", totals.Approved)
		}
	})

	t.Run("partial review — some decided, some not", func(t *testing.T) {
		totals := expenses.SumClaim(
			[]decimal.Decimal{dec(t, "100"), dec(t, "200"), dec(t, "50")},
			[]*decimal.Decimal{decp(t, "90"), nil, decp(t, "50")},
		)
		if totals.Undecided != 1 {
			t.Errorf("undecided = %d, want 1", totals.Undecided)
		}
		if !totals.Approved.Equal(dec(t, "140")) {
			t.Errorf("approved = %s, want 140 — undecided lines contribute nothing", totals.Approved)
		}
	})

	t.Run("an empty claim is zero, not a panic", func(t *testing.T) {
		totals := expenses.SumClaim(nil, nil)
		if !totals.Claimed.IsZero() || !totals.Approved.IsZero() || totals.Undecided != 0 {
			t.Errorf("empty claim gave %+v", totals)
		}
	})
}

// TestSettleAgainstAdvance covers all THREE outcomes the build plan names.
// None of them is an error state.
func TestSettleAgainstAdvance(t *testing.T) {
	t.Run("exact — the advance covered the claim precisely", func(t *testing.T) {
		s := expenses.SettleAgainstAdvance(dec(t, "500"), dec(t, "500"))
		if s.Outcome != expenses.SettlementExact {
			t.Errorf("outcome = %s, want exact", s.Outcome)
		}
		if !s.PayableToEmployee.IsZero() {
			t.Errorf("payable = %s, want 0", s.PayableToEmployee)
		}
		if !s.RecoverableFromEmployee.IsZero() {
			t.Errorf("recoverable = %s, want 0", s.RecoverableFromEmployee)
		}
		if !s.AppliedToAdvance.Equal(dec(t, "500")) {
			t.Errorf("applied = %s, want 500", s.AppliedToAdvance)
		}
	})

	t.Run("employee owes — the advance exceeded the claim", func(t *testing.T) {
		s := expenses.SettleAgainstAdvance(dec(t, "300"), dec(t, "500"))
		if s.Outcome != expenses.SettlementEmployeeOwes {
			t.Errorf("outcome = %s, want employee_owes", s.Outcome)
		}
		if !s.RecoverableFromEmployee.Equal(dec(t, "200")) {
			t.Errorf("recoverable = %s, want 200", s.RecoverableFromEmployee)
		}
		if !s.PayableToEmployee.IsZero() {
			t.Errorf("payable = %s, want 0 — the org owes nothing here", s.PayableToEmployee)
		}
		if !s.AppliedToAdvance.Equal(dec(t, "300")) {
			t.Errorf("applied = %s, want 300 (only what the claim used)", s.AppliedToAdvance)
		}
	})

	t.Run("org owes — the claim exceeded the advance", func(t *testing.T) {
		s := expenses.SettleAgainstAdvance(dec(t, "800"), dec(t, "500"))
		if s.Outcome != expenses.SettlementOrgOwes {
			t.Errorf("outcome = %s, want org_owes", s.Outcome)
		}
		if !s.PayableToEmployee.Equal(dec(t, "300")) {
			t.Errorf("payable = %s, want 300", s.PayableToEmployee)
		}
		if !s.RecoverableFromEmployee.IsZero() {
			t.Errorf("recoverable = %s, want 0", s.RecoverableFromEmployee)
		}
		if !s.AppliedToAdvance.Equal(dec(t, "500")) {
			t.Errorf("applied = %s, want the whole 500 advance", s.AppliedToAdvance)
		}
	})

	t.Run("no advance at all — the whole claim is payable", func(t *testing.T) {
		s := expenses.SettleAgainstAdvance(dec(t, "420"), decimal.Zero)
		if s.Outcome != expenses.SettlementOrgOwes {
			t.Errorf("outcome = %s, want org_owes", s.Outcome)
		}
		if !s.PayableToEmployee.Equal(dec(t, "420")) {
			t.Errorf("payable = %s, want the full 420", s.PayableToEmployee)
		}
	})

	t.Run("a PARTLY-used advance only offers what is left", func(t *testing.T) {
		// A 500 advance with 200 already settled leaves 300 outstanding; a
		// 450 claim against it should pay out 150, not 0 and not 450.
		s := expenses.SettleAgainstAdvance(dec(t, "450"), dec(t, "300"))
		if s.Outcome != expenses.SettlementOrgOwes {
			t.Errorf("outcome = %s, want org_owes", s.Outcome)
		}
		if !s.PayableToEmployee.Equal(dec(t, "150")) {
			t.Errorf("payable = %s, want 150", s.PayableToEmployee)
		}
	})

	t.Run("zero claim against a live advance recovers the whole advance", func(t *testing.T) {
		s := expenses.SettleAgainstAdvance(decimal.Zero, dec(t, "500"))
		if s.Outcome != expenses.SettlementEmployeeOwes {
			t.Errorf("outcome = %s, want employee_owes", s.Outcome)
		}
		if !s.RecoverableFromEmployee.Equal(dec(t, "500")) {
			t.Errorf("recoverable = %s, want the full 500", s.RecoverableFromEmployee)
		}
	})

	t.Run("negative inputs are clamped, never propagated", func(t *testing.T) {
		s := expenses.SettleAgainstAdvance(dec(t, "-100"), dec(t, "-50"))
		if s.Outcome != expenses.SettlementExact {
			t.Errorf("outcome = %s, want exact (both clamped to zero)", s.Outcome)
		}
		if s.PayableToEmployee.IsNegative() || s.RecoverableFromEmployee.IsNegative() {
			t.Errorf("negative money escaped: %+v", s)
		}
	})
}
