// backend/internal/tests/unit/hrm/compensation/context_test.go
// ApplyIncrease and ComputeCompaRatio are the arithmetic that decides real
// pay changes. Both are pure (no DB, no service receiver) specifically so
// they can be tested here, before ComputeCycle (which reaches *pgxpool.Pool
// directly and is therefore only reachable from an integration test) ever
// runs — the payslips.ComputeSlab / ReferencesGross precedent from r25.
package compensation_test

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/compensation"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal literal %q: %v", s, err)
	}
	return d
}

func TestApplyIncrease(t *testing.T) {
	cases := []struct {
		name    string
		current string
		pct     string
		want    string
	}{
		{"5 percent increase", "50000", "5", "52500"},
		{"zero percent is a no-op", "50000", "0", "50000"},
		{"fractional percent rounds to cents", "33333.33", "3.5", "34500.00"},
		{"negative percent (rare, e.g. a documented pay cut) is honoured, not clamped", "50000", "-10", "45000"},
		{"large increase", "10000", "150", "25000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := compensation.ApplyIncrease(dec(t, c.current), dec(t, c.pct))
			want := dec(t, c.want)
			if !got.Equal(want) {
				t.Errorf("ApplyIncrease(%s, %s) = %s, want %s", c.current, c.pct, got, want)
			}
		})
	}
}

func TestCompensationContext_ComputeCompaRatio(t *testing.T) {
	t.Run("nil band yields nil ratio, not zero", func(t *testing.T) {
		cc := &compensation.CompensationContext{CurrentBasicPay: dec(t, "50000")}
		if got := cc.ComputeCompaRatio(); got != nil {
			t.Errorf("expected nil ratio with no band, got %s", got)
		}
	})

	t.Run("band with zero mid_amount yields nil ratio, not a division panic", func(t *testing.T) {
		cc := &compensation.CompensationContext{
			CurrentBasicPay: dec(t, "50000"),
			Band:            &compensation.Band{MidAmount: decimal.Zero},
		}
		if got := cc.ComputeCompaRatio(); got != nil {
			t.Errorf("expected nil ratio with a zero mid_amount band, got %s", got)
		}
	})

	t.Run("pay exactly at midpoint is a 1.00 ratio", func(t *testing.T) {
		cc := &compensation.CompensationContext{
			CurrentBasicPay: dec(t, "50000"),
			Band:            &compensation.Band{MidAmount: dec(t, "50000")},
		}
		got := cc.ComputeCompaRatio()
		if got == nil || !got.Equal(dec(t, "1")) {
			t.Errorf("expected ratio 1, got %v", got)
		}
	})

	t.Run("pay below midpoint is below 1.00", func(t *testing.T) {
		cc := &compensation.CompensationContext{
			CurrentBasicPay: dec(t, "40000"),
			Band:            &compensation.Band{MidAmount: dec(t, "50000")},
		}
		got := cc.ComputeCompaRatio()
		if got == nil || !got.Equal(dec(t, "0.8")) {
			t.Errorf("expected ratio 0.8, got %v", got)
		}
	})
}

func TestRevision_PctIncrease(t *testing.T) {
	t.Run("computed at read time from current/proposed, never stored", func(t *testing.T) {
		rv := &compensation.Revision{CurrentBasicPay: dec(t, "50000"), ProposedBasicPay: dec(t, "52500")}
		got := rv.PctIncrease()
		if !got.Equal(dec(t, "5")) {
			t.Errorf("PctIncrease = %s, want 5", got)
		}
	})

	t.Run("zero current pay does not divide by zero", func(t *testing.T) {
		rv := &compensation.Revision{CurrentBasicPay: decimal.Zero, ProposedBasicPay: dec(t, "1000")}
		got := rv.PctIncrease()
		if !got.Equal(decimal.Zero) {
			t.Errorf("PctIncrease with zero current pay = %s, want 0 (not a panic)", got)
		}
	})

	t.Run("a pay cut is a negative percentage, not clamped to zero", func(t *testing.T) {
		rv := &compensation.Revision{CurrentBasicPay: dec(t, "50000"), ProposedBasicPay: dec(t, "45000")}
		got := rv.PctIncrease()
		if !got.Equal(dec(t, "-10")) {
			t.Errorf("PctIncrease = %s, want -10", got)
		}
	})
}

func TestBonusType_IsValid(t *testing.T) {
	valid := []compensation.BonusType{
		compensation.BonusPerformance, compensation.BonusDiscretionary, compensation.BonusSigning,
		compensation.BonusRetention, compensation.BonusReferral, compensation.BonusOther,
	}
	for _, bt := range valid {
		if !bt.IsValid() {
			t.Errorf("%q should be valid", bt)
		}
	}
	if compensation.BonusType("not_a_type").IsValid() {
		t.Error("unknown bonus type should not be valid")
	}
}

func TestCompensationContext_Snapshot(t *testing.T) {
	t.Run("minimal context still snapshots without panicking", func(t *testing.T) {
		cc := &compensation.CompensationContext{
			EmployeeID: "emp-1", CurrentBasicPay: dec(t, "1000"),
		}
		raw := cc.Snapshot(nil)
		if len(raw) == 0 {
			t.Fatal("expected a non-empty snapshot even with no band/rating")
		}
	})

	t.Run("extra fields are merged in", func(t *testing.T) {
		cc := &compensation.CompensationContext{EmployeeID: "emp-1", CurrentBasicPay: dec(t, "1000")}
		raw := cc.Snapshot(map[string]any{"cycle_id": "cyc-1"})
		if !strings.Contains(string(raw), `"cycle_id":"cyc-1"`) {
			t.Errorf("expected extra field in snapshot, got %s", raw)
		}
	})

	t.Run("band and rating are captured when present", func(t *testing.T) {
		label := "Exceeds Expectations"
		cc := &compensation.CompensationContext{
			EmployeeID: "emp-1", CurrentBasicPay: dec(t, "1000"),
			Band:        &compensation.Band{ID: "band-1", MinAmount: dec(t, "800"), MidAmount: dec(t, "1000"), MaxAmount: dec(t, "1200")},
			RatingLabel: &label,
		}
		raw := cc.Snapshot(nil)
		if !strings.Contains(string(raw), `"band"`) {
			t.Errorf("expected band in snapshot, got %s", raw)
		}
		if !strings.Contains(string(raw), `Exceeds Expectations`) {
			t.Errorf("expected rating label in snapshot, got %s", raw)
		}
	})
}
