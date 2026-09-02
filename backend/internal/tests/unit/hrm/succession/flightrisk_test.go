// backend/internal/tests/unit/hrm/succession/flightrisk_test.go
package succession_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/succession"
	"github.com/shopspring/decimal"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
func ptr(t time.Time) *time.Time { return &t }

// TestEveryFiredSignalExplainsItself — the plan bans predictive scoring, and
// a signal with no stated reason is that score under another name. This is
// the property that makes the indicator set defensible to the person it is
// about.
func TestEveryFiredSignalExplainsItself(t *testing.T) {
	asOf := day("2026-08-31")
	fired := []succession.Signal{}

	if s, ok := succession.EvalNoPromotion(day("2020-01-01"), ptr(day("2021-01-01")), asOf); ok {
		fired = append(fired, s)
	}
	if s, ok := succession.EvalBelowBand(decimal.NewFromInt(40000), decimal.NewFromInt(50000), "L3"); ok {
		fired = append(fired, s)
	}
	if s, ok := succession.EvalManagerChurn(4); ok {
		fired = append(fired, s)
	}
	if s, ok := succession.EvalAppraisalDecline(decimal.NewFromFloat(4.2), decimal.NewFromFloat(3.1)); ok {
		fired = append(fired, s)
	}

	if len(fired) != 4 {
		t.Fatalf("%d of 4 signals fired on inputs chosen to trip all of them", len(fired))
	}
	for _, s := range fired {
		if strings.TrimSpace(s.Detail) == "" {
			t.Errorf("signal %q fired with no explanation", s.Type)
		}
		if s.Type == "" {
			t.Errorf("signal fired with no type: %q", s.Detail)
		}
	}
}

func TestEvalNoPromotion(t *testing.T) {
	asOf := day("2026-08-31")
	cases := []struct {
		name       string
		hire       time.Time
		lastPromo  *time.Time
		want       bool
		wantDetail string
	}{
		{
			// A recent hire has not been passed over for anything, and
			// saying so about them is noise that discredits the whole set.
			name: "new hire never promoted does not fire",
			hire: day("2025-06-01"), lastPromo: nil, want: false,
		},
		{
			name: "18 months tenure but under the promotion threshold",
			hire: day("2024-06-01"), lastPromo: nil, want: false,
		},
		{
			name: "long tenure never promoted fires and says since hire",
			hire: day("2021-01-01"), lastPromo: nil, want: true,
			wantDetail: "since being hired",
		},
		{
			name: "promoted recently does not fire despite long tenure",
			hire: day("2015-01-01"), lastPromo: ptr(day("2025-06-01")), want: false,
		},
		{
			name: "promoted long ago fires and says since last promotion",
			hire: day("2015-01-01"), lastPromo: ptr(day("2021-01-01")), want: true,
			wantDetail: "since the last promotion",
		},
		{
			// Exactly at the threshold fires: 36 months is the line, and a
			// person on it has waited the full stated period.
			name: "exactly 36 months fires",
			hire: day("2015-01-01"), lastPromo: ptr(day("2023-08-31")), want: true,
		},
		{
			name: "35 months does not fire",
			hire: day("2015-01-01"), lastPromo: ptr(day("2023-10-01")), want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := succession.EvalNoPromotion(c.hire, c.lastPromo, asOf)
			if ok != c.want {
				t.Fatalf("fired=%v want %v (detail %q)", ok, c.want, got.Detail)
			}
			if ok && c.wantDetail != "" && !strings.Contains(got.Detail, c.wantDetail) {
				t.Errorf("detail %q does not contain %q", got.Detail, c.wantDetail)
			}
		})
	}
}

// TestEvalBelowBand_UsesMinimumNotMidpoint — half of any healthy band sits
// under its midpoint by construction, so a midpoint test would flag roughly
// half the company and mean nothing at all.
func TestEvalBelowBand_UsesMinimumNotMidpoint(t *testing.T) {
	i := decimal.NewFromInt
	// Band 50,000 – 75,000 – 100,000. Someone on 60,000 is under the
	// midpoint but properly inside their band.
	if s, ok := succession.EvalBelowBand(i(60000), i(50000), "L3"); ok {
		t.Errorf("pay inside the band fired: %q", s.Detail)
	}
	s, ok := succession.EvalBelowBand(i(45000), i(50000), "L3")
	if !ok {
		t.Fatal("pay below the band minimum did not fire")
	}
	for _, want := range []string{"45000.00", "5000.00", "50000.00", "L3"} {
		if !strings.Contains(s.Detail, want) {
			t.Errorf("detail %q is missing %q — the shortfall must be stated, not implied", s.Detail, want)
		}
	}
	// Exactly at the minimum is inside the band.
	if _, ok := succession.EvalBelowBand(i(50000), i(50000), "L3"); ok {
		t.Error("pay exactly at the band minimum fired")
	}
	// Missing data must not fire — an employee with no band recorded is not
	// underpaid, they are unmeasured.
	if _, ok := succession.EvalBelowBand(i(45000), decimal.Zero, ""); ok {
		t.Error("fired with no band recorded")
	}
	if _, ok := succession.EvalBelowBand(decimal.Zero, i(50000), "L3"); ok {
		t.Error("fired with no salary recorded")
	}
}

func TestEvalManagerChurn(t *testing.T) {
	if _, ok := succession.EvalManagerChurn(2); ok {
		t.Error("2 manager changes fired — one reorganisation is not churn")
	}
	s, ok := succession.EvalManagerChurn(3)
	if !ok {
		t.Fatal("3 manager changes did not fire")
	}
	if !strings.Contains(s.Detail, "3 manager changes") {
		t.Errorf("detail %q does not state the count", s.Detail)
	}
	if _, ok := succession.EvalManagerChurn(0); ok {
		t.Error("0 changes fired")
	}
}

func TestEvalAppraisalDecline(t *testing.T) {
	d := decimal.NewFromFloat
	s, ok := succession.EvalAppraisalDecline(d(4.2), d(3.1))
	if !ok {
		t.Fatal("a falling rating did not fire")
	}
	if !strings.Contains(s.Detail, "4.20") || !strings.Contains(s.Detail, "3.10") {
		t.Errorf("detail %q must name both ratings so the reader can judge the size of the drop", s.Detail)
	}
	// Any drop counts. How big a drop matters is the judgement this hands
	// back to a human rather than hiding in a threshold.
	if _, ok := succession.EvalAppraisalDecline(d(4.20), d(4.19)); !ok {
		t.Error("a small drop did not fire — the signal is the direction")
	}
	if _, ok := succession.EvalAppraisalDecline(d(3.0), d(4.0)); ok {
		t.Error("an improving rating fired")
	}
	if _, ok := succession.EvalAppraisalDecline(d(4.0), d(4.0)); ok {
		t.Error("an unchanged rating fired")
	}
	// One appraisal is not a trend.
	if _, ok := succession.EvalAppraisalDecline(decimal.Zero, d(3.0)); ok {
		t.Error("fired with no previous appraisal to compare against")
	}
}

// TestEvalNoPromotion_PartialMonthIsNotAWholeMonth pins the day-of-month
// correction in monthsBetween.
//
// ⚠ This test exists because an injection that DELETED that correction left
// the rest of the table green — none of its cases straddled a partial month,
// so they proved nothing about it. The pair below differs by a single day
// across the threshold, which is the only shape that can catch it.
func TestEvalNoPromotion_PartialMonthIsNotAWholeMonth(t *testing.T) {
	hire := day("2015-01-01")
	promo := ptr(day("2023-09-15"))

	// One day short of 36 months. Telling somebody they have waited three
	// years when they have waited 36 months minus a day is a false fact in
	// a report a human is meant to act on.
	if s, ok := succession.EvalNoPromotion(hire, promo, day("2026-09-14")); ok {
		t.Errorf("fired one day early: %q", s.Detail)
	}
	// The threshold day itself fires.
	s, ok := succession.EvalNoPromotion(hire, promo, day("2026-09-15"))
	if !ok {
		t.Fatal("did not fire on the threshold day itself")
	}
	if !strings.Contains(s.Detail, "36 months") {
		t.Errorf("detail %q should state 36 months", s.Detail)
	}
}
