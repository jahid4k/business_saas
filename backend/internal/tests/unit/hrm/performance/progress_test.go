// backend/internal/tests/unit/hrm/performance/progress_test.go
// The goal progress formula, tested in isolation before any layer depends on
// it. These cases need no stub repository — Goal.RawProgressPercent and
// Goal.ProgressPercent are pure functions of stored columns, which is exactly
// what makes a Phase 5B appraisal snapshot reproducible.
package performance_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/performance"
)

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal literal in test: " + s)
	}
	return d
}

func TestProgressPercent_PerMeasurementType(t *testing.T) {
	cases := []struct {
		name        string
		measurement performance.MeasurementType
		start       string
		target      string
		current     string
		wantRaw     string
		wantClamped string
	}{
		// ── percentage ───────────────────────────────────────────────────
		{"percentage at start", performance.MeasurementPercentage, "0", "100", "0", "0", "0"},
		{"percentage halfway", performance.MeasurementPercentage, "0", "100", "50", "50", "50"},
		{"percentage complete", performance.MeasurementPercentage, "0", "100", "100", "100", "100"},

		// ── numeric with a non-zero start ────────────────────────────────
		// The start value is what makes the formula work for goals that do
		// not begin at zero: 30 of the way from 20 to 70 is 20%, not 42.86%.
		{"numeric from non-zero start", performance.MeasurementNumeric, "20", "70", "30", "20", "20"},
		{"numeric at target", performance.MeasurementNumeric, "20", "70", "70", "100", "100"},

		// ── currency ─────────────────────────────────────────────────────
		{"currency quarter way", performance.MeasurementCurrency, "0", "400000", "100000", "25", "25"},

		// ── decrease goals ───────────────────────────────────────────────
		// The case the obvious implementation gets wrong. Direction is NOT an
		// input: negative numerator over negative denominator is positive.
		// From 100 toward 20, sitting at 60, is exactly half the distance.
		{"decrease halfway", performance.MeasurementNumeric, "100", "20", "60", "50", "50"},
		{"decrease complete", performance.MeasurementNumeric, "100", "20", "20", "100", "100"},
		{"decrease not started", performance.MeasurementNumeric, "100", "20", "100", "0", "0"},

		// ── overshoot and regression ─────────────────────────────────────
		// Raw keeps the real figure; clamped protects 5B's rating scale.
		{"overshoot beyond target", performance.MeasurementNumeric, "0", "100", "130", "130", "100"},
		{"regression below start", performance.MeasurementNumeric, "0", "100", "-30", "-30", "0"},
		{"decrease overshoot", performance.MeasurementNumeric, "100", "20", "10", "112.5", "100"},

		// ── boolean ──────────────────────────────────────────────────────
		{"boolean not done", performance.MeasurementBoolean, "0", "1", "0", "0", "0"},
		{"boolean done", performance.MeasurementBoolean, "0", "1", "1", "100", "100"},

		// ── degenerate: target == start ──────────────────────────────────
		// Rejected on write, but a hand-inserted or legacy row must not panic
		// with a division by zero.
		{"degenerate target equals start, reached", performance.MeasurementNumeric, "50", "50", "50", "100", "100"},
		{"degenerate target equals start, below", performance.MeasurementNumeric, "50", "50", "10", "0", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &performance.Goal{
				MeasurementType: tc.measurement,
				StartValue:      dec(tc.start),
				TargetValue:     dec(tc.target),
				CurrentValue:    dec(tc.current),
			}
			if got := g.RawProgressPercent(); !got.Equal(dec(tc.wantRaw)) {
				t.Errorf("RawProgressPercent() = %s, want %s", got, tc.wantRaw)
			}
			if got := g.ProgressPercent(); !got.Equal(dec(tc.wantClamped)) {
				t.Errorf("ProgressPercent() = %s, want %s", got, tc.wantClamped)
			}
		})
	}
}

// TestProgressPercent_IgnoresDirectionField pins the decision that Direction
// drives validation and display only. If someone later branches the formula on
// it, two otherwise-identical goals would report different progress and this
// fails.
func TestProgressPercent_IgnoresDirectionField(t *testing.T) {
	base := func(dir performance.Direction) *performance.Goal {
		return &performance.Goal{
			MeasurementType: performance.MeasurementNumeric,
			Direction:       dir,
			StartValue:      dec("100"),
			TargetValue:     dec("20"),
			CurrentValue:    dec("60"),
		}
	}
	asIncrease := base(performance.DirectionIncrease).RawProgressPercent()
	asDecrease := base(performance.DirectionDecrease).RawProgressPercent()

	if !asIncrease.Equal(asDecrease) {
		t.Errorf("progress must not depend on the direction field: increase=%s decrease=%s", asIncrease, asDecrease)
	}
	if !asDecrease.Equal(dec("50")) {
		t.Errorf("expected 50%% for a 100→20 goal sitting at 60, got %s", asDecrease)
	}
}

// TestProgressPercent_ClampedNeverEscapesRatingScale is the property Phase 5B
// depends on: weighted attainment sums ProgressPercent, so no single goal may
// contribute more than 100 or less than 0 no matter how extreme its raw value.
func TestProgressPercent_ClampedNeverEscapesRatingScale(t *testing.T) {
	extremes := []struct{ start, target, current string }{
		{"0", "100", "100000"},
		{"0", "100", "-100000"},
		{"100", "20", "-500"},
		{"0", "1", "999"},
	}
	for _, e := range extremes {
		g := &performance.Goal{
			MeasurementType: performance.MeasurementNumeric,
			StartValue:      dec(e.start),
			TargetValue:     dec(e.target),
			CurrentValue:    dec(e.current),
		}
		got := g.ProgressPercent()
		if got.IsNegative() || got.GreaterThan(dec("100")) {
			t.Errorf("start=%s target=%s current=%s: clamped progress escaped [0,100] with %s",
				e.start, e.target, e.current, got)
		}
	}
}
