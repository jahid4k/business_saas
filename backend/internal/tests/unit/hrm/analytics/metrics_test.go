// backend/internal/tests/unit/hrm/analytics/metrics_test.go
package analytics_test

import (
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/analytics"
	"github.com/shopspring/decimal"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// TestAttritionRate_UsesAverageNotClosingHeadcount — dividing by closing
// headcount inflates the rate in exactly the months somebody is looking at
// the chart, because that is when people left.
func TestAttritionRate_UsesAverageNotClosingHeadcount(t *testing.T) {
	// 100 at the start, 10 leave, 90 at the end.
	avg := analytics.AverageHeadcount(100, 90)
	if !avg.Equal(decimal.NewFromInt(95)) {
		t.Fatalf("average headcount = %s, want 95", avg)
	}
	got, ok := analytics.AttritionRate(10, avg)
	if !ok {
		t.Fatal("rate not computed")
	}
	// 10/95 = 10.53%, not 10/90 = 11.11% and not 10/100 = 10.00%.
	if !got.Equal(decimal.RequireFromString("10.53")) {
		t.Errorf("attrition rate = %s, want 10.53 (10 ÷ 95). "+
			"11.11 means it divided by closing headcount; 10.00 means opening", got)
	}
}

// TestAttritionRate_EmptyPeriodIsNotZeroPercent — an organization with no
// people did not achieve perfect retention, and a confident 0% is worse than
// a gap in the chart.
func TestAttritionRate_EmptyPeriodIsNotZeroPercent(t *testing.T) {
	if _, ok := analytics.AttritionRate(0, decimal.Zero); ok {
		t.Error("a zero denominator produced a rate")
	}
	if _, ok := analytics.AttritionRate(3, decimal.Zero); ok {
		t.Error("leavers with no headcount produced a rate")
	}
	if _, ok := analytics.AttritionRate(-1, decimal.NewFromInt(50)); ok {
		t.Error("negative leavers produced a rate")
	}
}

// TestAttritionRate_IsExactAtTheThirds — the rate is a percentage of real
// people and must not carry float drift, the r37 lesson.
func TestAttritionRate_IsExactAtTheThirds(t *testing.T) {
	cases := []struct {
		leavers int
		avg     int64
		want    string
	}{
		{1, 3, "33.33"},
		{2, 3, "66.67"},
		{1, 7, "14.29"},
		{1, 6, "16.67"},
		{1, 8, "12.5"},
	}
	for _, c := range cases {
		got, ok := analytics.AttritionRate(c.leavers, decimal.NewFromInt(c.avg))
		if !ok {
			t.Fatalf("%d/%d not computed", c.leavers, c.avg)
		}
		if !got.Equal(decimal.RequireFromString(c.want)) {
			t.Errorf("%d ÷ %d = %s, want %s", c.leavers, c.avg, got, c.want)
		}
	}
}

// TestIsVoluntaryExit pins the classification, including the two cases that
// are judgements rather than obvious.
func TestIsVoluntaryExit(t *testing.T) {
	cases := []struct {
		source, termType string
		want             bool
		why              string
	}{
		{"resignation", "", true, "nobody resigns on somebody else's behalf"},
		{"termination", "voluntary", true, "recorded as the employee's choice"},
		{"termination", "retirement", true, "retiring is a decision the person made"},
		{"termination", "involuntary", false, ""},
		{"termination", "layoff", false, "a layoff is not a resignation with better manners"},
		{"termination", "probation_fail", false, ""},
		{"termination", "contract_end", false,
			"a contract ending on its own terms was nobody's decision on the day"},
		{"termination", "", false, "an unclassified termination is not assumed voluntary"},
	}
	for _, c := range cases {
		if got := analytics.IsVoluntaryExit(c.source, c.termType); got != c.want {
			t.Errorf("IsVoluntaryExit(%q,%q) = %v, want %v — %s",
				c.source, c.termType, got, c.want, c.why)
		}
	}
}

// TestRegrettedFromRehireStatus_UnknownStaysUnknown is the claim that keeps
// the regretted split honest. Collapsing "nobody has judged this yet" into
// "not regretted" would report every un-reviewed departure as a good
// riddance, flattering the exact number the metric exists to expose.
func TestRegrettedFromRehireStatus_UnknownStaysUnknown(t *testing.T) {
	if got := analytics.RegrettedFromRehireStatus("", false); got != nil {
		t.Errorf("no rehire decision returned %v, want nil (unknown)", *got)
	}
	if got := analytics.RegrettedFromRehireStatus("conditional", true); got != nil {
		t.Errorf("a conditional decision returned %v, want nil — conditional is not a verdict", *got)
	}
	if got := analytics.RegrettedFromRehireStatus("something_new", true); got != nil {
		t.Errorf("an unrecognised status returned %v, want nil", *got)
	}
	got := analytics.RegrettedFromRehireStatus("eligible", true)
	if got == nil || !*got {
		t.Errorf("'eligible' returned %v, want regretted=true", got)
	}
	got = analytics.RegrettedFromRehireStatus("not_eligible", true)
	if got == nil || *got {
		t.Errorf("'not_eligible' returned %v, want regretted=false", got)
	}
}

func TestTenureAndFirstYear(t *testing.T) {
	hire := day("2025-01-01")
	if got := analytics.TenureDays(hire, day("2025-01-01")); got != 0 {
		t.Errorf("same-day tenure = %d, want 0", got)
	}
	if got := analytics.TenureDays(hire, day("2024-06-01")); got != 0 {
		t.Errorf("an exit before the hire date gave %d, want 0 rather than a negative tenure", got)
	}
	// 2025 is not a leap year, so 2026-01-01 is exactly 365 days out.
	if got := analytics.TenureDays(hire, day("2026-01-01")); got != 365 {
		t.Errorf("one year = %d days, want 365", got)
	}
	if !analytics.IsFirstYearExit(364) {
		t.Error("day 364 is not counted as a first-year exit")
	}
	if !analytics.IsFirstYearExit(0) {
		t.Error("a same-day exit is not counted as a first-year exit")
	}
	if analytics.IsFirstYearExit(365) {
		t.Error("day 365 was counted as a first-year exit — the first year is days 0 to 364")
	}
}

func TestCohortRetention(t *testing.T) {
	got, ok := analytics.CohortRetention(40, 30)
	if !ok || !got.Equal(decimal.RequireFromString("75")) {
		t.Errorf("30 of 40 = %s (ok=%v), want 75", got, ok)
	}
	if _, ok := analytics.CohortRetention(0, 0); ok {
		t.Error("an empty cohort produced a retention rate")
	}
	// More survivors than were hired is corrupt input, not 120% retention.
	if _, ok := analytics.CohortRetention(10, 12); ok {
		t.Error("more survivors than the cohort size produced a rate")
	}
}

func TestCohortMonth(t *testing.T) {
	got := analytics.CohortMonth(day("2025-03-17"))
	if got.Year() != 2025 || got.Month() != time.March || got.Day() != 1 {
		t.Errorf("CohortMonth = %s, want 2025-03-01", got.Format("2006-01-02"))
	}
}

// TestTenureBucket — buckets rather than a mean, because twenty people at ten
// years and twenty at six months average to a number describing nobody.
func TestTenureBucket(t *testing.T) {
	cases := []struct {
		days int
		want string
	}{
		{0, "under_6_months"},
		{179, "under_6_months"},
		{180, "6_to_12_months"},
		{364, "6_to_12_months"},
		{365, "1_to_2_years"},
		{729, "1_to_2_years"},
		{730, "2_to_5_years"},
		{1824, "2_to_5_years"},
		{1825, "over_5_years"},
		{-1, "unknown"},
	}
	for _, c := range cases {
		if got := analytics.TenureBucket(c.days); got != c.want {
			t.Errorf("TenureBucket(%d) = %q, want %q", c.days, got, c.want)
		}
	}
}

func TestComputation_IsValid(t *testing.T) {
	for _, c := range []analytics.Computation{
		analytics.CompHeadcount, analytics.CompAttritionRate, analytics.CompFirstYearAttrition,
		analytics.CompCohortRetention, analytics.CompTenureDistribution,
		analytics.CompDEIDistribution, analytics.CompCompDistribution,
	} {
		if !c.IsValid() {
			t.Errorf("%q is not valid but is in the CHECK vocabulary", c)
		}
	}
	for _, c := range []analytics.Computation{"", "predictive_attrition", "flight_risk_score"} {
		if analytics.Computation(c).IsValid() {
			t.Errorf("%q was accepted — predictive scoring is deliberately excluded", c)
		}
	}
}
