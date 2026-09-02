// backend/internal/tests/unit/hrm/exits/gratuity_test.go
// The gratuity arithmetic. This number is paid to a departing employee and
// is the figure most likely to be disputed, so it is pure and tested before
// anything calls it — the Amortize / BookValue / SettleAgainstAdvance /
// EvaluateSLA / NoticeShortfallDays precedent.
package exits_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/exits"
)

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// standardRule: 5-year minimum, 30 days per year, /30 divisor. With a 30,000
// monthly base the daily rate is exactly 1,000, so entitlements are round
// numbers and an arithmetic slip is obvious on sight.
func standardRule() *exits.GratuityRule {
	return &exits.GratuityRule{
		MinYearsOfService: dec("5"),
		DaysPerYear:       dec("30"),
		BaseComponent:     "basic",
		MonthlyDivisor:    dec("30"),
	}
}

func TestDailyRate(t *testing.T) {
	cases := []struct {
		name         string
		monthly, div string
		want         string
	}{
		{"statutory 30-day divisor", "30000", "30", "1000"},
		{"26-day divisor excluding weekly offs", "26000", "26", "1000"},
		{"fractional result is not truncated", "10000", "30", "333.3333333333333333"},
		// A misconfigured rule must not divide by zero and take down a run.
		{"zero divisor yields zero, not a panic", "30000", "0", "0"},
		{"negative divisor yields zero", "30000", "-30", "0"},
		{"negative monthly yields zero", "-30000", "30", "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := exits.DailyRate(dec(c.monthly), dec(c.div))
			if !got.Equal(dec(c.want)) {
				t.Errorf("DailyRate(%s, %s) = %s, want %s", c.monthly, c.div, got, c.want)
			}
		})
	}
}

func TestYearsOfService(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time {
		return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	}
	cases := []struct {
		name             string
		hire, last       time.Time
		wantMin, wantMax string // a range, since 365.25 makes exact equality brittle
	}{
		{"exactly ten years", d(2016, time.January, 1), d(2026, time.January, 1), "9.99", "10.01"},
		{"six months", d(2026, time.January, 1), d(2026, time.July, 1), "0.49", "0.51"},
		{"same day is zero", d(2026, time.January, 1), d(2026, time.January, 1), "0", "0"},
		// Inverted dates must not produce a negative tenure — it would flow
		// into the eligibility comparison and produce nonsense.
		{"inverted dates yield zero", d(2026, time.June, 1), d(2026, time.January, 1), "0", "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := exits.YearsOfService(c.hire, c.last)
			if got.LessThan(dec(c.wantMin)) || got.GreaterThan(dec(c.wantMax)) {
				t.Errorf("YearsOfService = %s, want between %s and %s", got, c.wantMin, c.wantMax)
			}
			if got.IsNegative() {
				t.Errorf("YearsOfService = %s — a negative tenure corrupts the eligibility check", got)
			}
		})
	}
}

func TestComputeGratuity(t *testing.T) {
	hire := time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)
	base := dec("30000") // daily rate 1000 under the standard rule

	t.Run("ten years pays ten times the annual entitlement", func(t *testing.T) {
		last := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		got := exits.ComputeGratuity(standardRule(), base, hire, last, false)
		if !got.Eligible {
			t.Fatalf("not eligible after ~10 years: %s", got.Reason)
		}
		// 9 completed years (365.25 puts this just under 10) x 30 days x 1000.
		want := decimal.NewFromInt(got.CompletedYears).Mul(dec("30")).Mul(dec("1000"))
		if !got.Amount.Equal(want) {
			t.Errorf("amount = %s, want %s (%d completed years)", got.Amount, want, got.CompletedYears)
		}
		if !got.DailyRate.Equal(dec("1000")) {
			t.Errorf("daily rate = %s, want 1000", got.DailyRate)
		}
	})

	t.Run("below the minimum is zero WITH A REASON, not an error", func(t *testing.T) {
		last := time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC) // ~3 years
		got := exits.ComputeGratuity(standardRule(), base, hire, last, false)
		if !got.Amount.IsZero() {
			t.Errorf("amount = %s, want 0 below the 5-year minimum", got.Amount)
		}
		if got.Eligible {
			t.Error("reported eligible below the minimum")
		}
		if got.Reason == "" {
			t.Error("no reason given — a disputed zero has to be explainable")
		}
	})

	t.Run("just short of the minimum does not qualify", func(t *testing.T) {
		// ~4.9 years. The minimum is doing its job; this is not a rounding bug.
		last := time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)
		got := exits.ComputeGratuity(standardRule(), base, hire, last, false)
		if got.Eligible {
			t.Errorf("qualified at %s years against a 5-year minimum", got.YearsOfService)
		}
	})

	t.Run("partial years are NOT paid pro rata", func(t *testing.T) {
		// ~7.5 years must pay 7 completed years, not 7.5.
		last := time.Date(2023, time.July, 1, 0, 0, 0, 0, time.UTC)
		got := exits.ComputeGratuity(standardRule(), base, hire, last, false)
		if !got.Eligible {
			t.Fatalf("not eligible: %s", got.Reason)
		}
		if got.CompletedYears != 7 {
			t.Errorf("completed years = %d, want 7", got.CompletedYears)
		}
		want := dec("7").Mul(dec("30")).Mul(dec("1000"))
		if !got.Amount.Equal(want) {
			t.Errorf("amount = %s, want %s — partial years must not be paid", got.Amount, want)
		}
	})

	t.Run("no rule configured is zero, never a crash", func(t *testing.T) {
		last := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		got := exits.ComputeGratuity(nil, base, hire, last, false)
		if !got.Amount.IsZero() || got.Eligible {
			t.Errorf("nil rule gave amount=%s eligible=%v, want 0 and false", got.Amount, got.Eligible)
		}
		if got.Reason == "" {
			t.Error("no reason given for a nil rule")
		}
	})

	t.Run("misconduct forfeits ONLY when the rule opts in", func(t *testing.T) {
		last := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

		// Default rule: forfeiture is off, so misconduct changes nothing.
		lenient := exits.ComputeGratuity(standardRule(), base, hire, last, true)
		if lenient.Amount.IsZero() {
			t.Error("gratuity forfeited by a rule that never opted into forfeiture — " +
				"forfeiture is legally loaded and must not be the default")
		}

		strict := standardRule()
		strict.ForfeitOnMisconduct = true
		forfeited := exits.ComputeGratuity(strict, base, hire, last, true)
		if !forfeited.Amount.IsZero() {
			t.Errorf("amount = %s, want 0 when an opted-in rule meets misconduct", forfeited.Amount)
		}
		// And the same strict rule pays normally without misconduct.
		clean := exits.ComputeGratuity(strict, base, hire, last, false)
		if clean.Amount.IsZero() {
			t.Error("an opted-in rule withheld gratuity from a clean exit")
		}
	})

	t.Run("a 26-day divisor pays more than a 30-day one", func(t *testing.T) {
		last := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		r26 := standardRule()
		r26.MonthlyDivisor = dec("26")
		a30 := exits.ComputeGratuity(standardRule(), base, hire, last, false)
		a26 := exits.ComputeGratuity(r26, base, hire, last, false)
		if !a26.Amount.GreaterThan(a30.Amount) {
			t.Errorf("26-divisor gave %s, 30-divisor gave %s — the divisor is not being applied",
				a26.Amount, a30.Amount)
		}
	})

	t.Run("amount is never negative", func(t *testing.T) {
		last := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		got := exits.ComputeGratuity(standardRule(), dec("0"), hire, last, false)
		if got.Amount.IsNegative() {
			t.Errorf("amount = %s", got.Amount)
		}
	})
}
