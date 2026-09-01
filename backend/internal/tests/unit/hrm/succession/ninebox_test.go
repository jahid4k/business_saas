// backend/internal/tests/unit/hrm/succession/ninebox_test.go
package succession_test

import (
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/succession"
	"github.com/shopspring/decimal"
)

// TestBox_PotentialIsNotDerivableFromPerformance is the claim the whole
// slice exists to protect. Two employees with the SAME appraisal rating and
// DIFFERENT assessed potential must land in different boxes; if they did
// not, the 9-box would be the rating drawn twice.
func TestBox_PotentialIsNotDerivableFromPerformance(t *testing.T) {
	for _, perf := range []succession.Band{succession.BandLow, succession.BandMedium, succession.BandHigh} {
		seen := map[int]succession.Band{}
		for _, pot := range []succession.Band{succession.BandLow, succession.BandMedium, succession.BandHigh} {
			box, ok := succession.Box(perf, pot)
			if !ok {
				t.Fatalf("Box(%s,%s) not ok", perf, pot)
			}
			if prior, dup := seen[box.Box]; dup {
				t.Fatalf("performance=%s: potential %s and %s both map to box %d — "+
					"potential is being collapsed into performance", perf, prior, pot, box.Box)
			}
			seen[box.Box] = pot
		}
		if len(seen) != 3 {
			t.Errorf("performance=%s produced %d distinct boxes across 3 potential bands, want 3", perf, len(seen))
		}
	}
}

// TestBox_PerformanceStillMovesTheBox is the other half: holding potential
// fixed, performance must also separate. A grid that only responded to one
// axis would be a list.
func TestBox_PerformanceStillMovesTheBox(t *testing.T) {
	for _, pot := range []succession.Band{succession.BandLow, succession.BandMedium, succession.BandHigh} {
		seen := map[int]bool{}
		for _, perf := range []succession.Band{succession.BandLow, succession.BandMedium, succession.BandHigh} {
			box, _ := succession.Box(perf, pot)
			seen[box.Box] = true
		}
		if len(seen) != 3 {
			t.Errorf("potential=%s produced %d distinct boxes, want 3", pot, len(seen))
		}
	}
}

// TestBox_GridIsTheDrawnGrid pins the actual numbering and names so a later
// refactor cannot silently rotate the grid — every consumer's "box 9 is a
// star" assumption would break invisibly.
func TestBox_GridIsTheDrawnGrid(t *testing.T) {
	cases := []struct {
		perf, pot succession.Band
		box       int
		label     string
	}{
		{succession.BandLow, succession.BandLow, 1, "Risk"},
		{succession.BandMedium, succession.BandLow, 2, "Average Performer"},
		{succession.BandHigh, succession.BandLow, 3, "Solid Performer"},
		{succession.BandLow, succession.BandMedium, 4, "Inconsistent Player"},
		{succession.BandMedium, succession.BandMedium, 5, "Core Player"},
		{succession.BandHigh, succession.BandMedium, 6, "High Performer"},
		{succession.BandLow, succession.BandHigh, 7, "Potential Gem"},
		{succession.BandMedium, succession.BandHigh, 8, "High Potential"},
		{succession.BandHigh, succession.BandHigh, 9, "Star"},
	}
	for _, c := range cases {
		got, ok := succession.Box(c.perf, c.pot)
		if !ok {
			t.Fatalf("Box(%s,%s) not ok", c.perf, c.pot)
		}
		if got.Box != c.box || got.Label != c.label {
			t.Errorf("Box(perf=%s, pot=%s) = %d %q, want %d %q",
				c.perf, c.pot, got.Box, got.Label, c.box, c.label)
		}
		if got.Performance != c.perf || got.Potential != c.pot {
			t.Errorf("Box(%s,%s) echoed back %s/%s", c.perf, c.pot, got.Performance, got.Potential)
		}
	}
}

// TestBox_RefusesUnknownBand — defaulting an unassessed band to "low" would
// put a real person in the corner of the grid that ends careers.
func TestBox_RefusesUnknownBand(t *testing.T) {
	for _, c := range []struct{ perf, pot succession.Band }{
		{"", succession.BandHigh},
		{succession.BandHigh, ""},
		{"exceptional", succession.BandHigh},
		{succession.BandHigh, "LOW"},
	} {
		if box, ok := succession.Box(c.perf, c.pot); ok {
			t.Errorf("Box(%q,%q) returned box %d, want refusal", c.perf, c.pot, box.Box)
		}
	}
}

func TestPerformanceBandFromRating(t *testing.T) {
	d := decimal.NewFromFloat
	cases := []struct {
		rating, max float64
		want        succession.Band
		ok          bool
		why         string
	}{
		{0, 5, succession.BandLow, true, "a zero rating is a real low rating"},
		{2.99, 5, succession.BandLow, true, "59.8% is under the 60% low ceiling"},
		{3.00, 5, succession.BandMedium, true, "exactly 60% crosses into medium"},
		{3.99, 5, succession.BandMedium, true, "79.8% is still medium"},
		{4.00, 5, succession.BandHigh, true, "exactly 80% crosses into high"},
		{5.00, 5, succession.BandHigh, true, "top of scale"},
		// The same fractions on a 10-point scale must land identically —
		// the thresholds are fractions precisely so a scale change does not
		// re-band the whole company.
		{6.00, 10, succession.BandMedium, true, "60% of a 10-point scale"},
		{8.00, 10, succession.BandHigh, true, "80% of a 10-point scale"},
		{5.99, 10, succession.BandLow, true, "59.9% of a 10-point scale"},
		// Refusals: an unplaceable rating must not become a low band.
		{4, 0, "", false, "no scale maximum"},
		{-1, 5, "", false, "negative rating"},
		{6, 5, "", false, "rating above the scale"},
	}
	for _, c := range cases {
		got, ok := succession.PerformanceBandFromRating(d(c.rating), d(c.max))
		if ok != c.ok || got != c.want {
			t.Errorf("PerformanceBandFromRating(%v,%v) = %q,%v want %q,%v — %s",
				c.rating, c.max, got, ok, c.want, c.ok, c.why)
		}
	}
}

// TestPerformanceBandFromRating_IsExactAtTheBoundary — the thresholds are
// decimal fractions, so 3/5 must be medium rather than falling to low on a
// float representation of 0.6.
func TestPerformanceBandFromRating_IsExactAtTheBoundary(t *testing.T) {
	for _, c := range []struct{ r, m string }{
		{"3", "5"}, {"6", "10"}, {"0.6", "1"}, {"60", "100"},
	} {
		r, _ := decimal.NewFromString(c.r)
		m, _ := decimal.NewFromString(c.m)
		got, ok := succession.PerformanceBandFromRating(r, m)
		if !ok || got != succession.BandMedium {
			t.Errorf("%s/%s = %q,%v — exactly 60%% must be medium, not low", c.r, c.m, got, ok)
		}
	}
}
