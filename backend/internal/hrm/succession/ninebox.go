// backend/internal/hrm/succession/ninebox.go
package succession

import (
	"strings"

	"github.com/shopspring/decimal"
)

// Band is one axis position on the 9-box grid.
type Band string

const (
	BandLow    Band = "low"
	BandMedium Band = "medium"
	BandHigh   Band = "high"
)

func (b Band) IsValid() bool {
	switch b {
	case BandLow, BandMedium, BandHigh:
		return true
	}
	return false
}

// index maps a band to 0/1/2. Returns -1 for an unknown band so a caller
// cannot silently treat garbage as "low".
func (b Band) index() int {
	switch b {
	case BandLow:
		return 0
	case BandMedium:
		return 1
	case BandHigh:
		return 2
	}
	return -1
}

// NineBox is a position on the grid: a number, a name, and the two bands it
// came from. It is COMPUTED — nothing stores a box number, because a stored
// box could disagree with the two bands it claims to summarise (the 00076
// rule).
type NineBox struct {
	Box         int    `json:"box"`
	Label       string `json:"label"`
	Performance Band   `json:"performance_band"`
	Potential   Band   `json:"potential_band"`
}

// boxLabels is the grid, written the way it is drawn: potential rises up the
// rows, performance rises across the columns.
//
//	potential high │  7 Potential Gem   8 High Potential   9 Star
//	potential med  │  4 Inconsistent    5 Core Player      6 High Performer
//	potential low  │  1 Risk            2 Average          3 Solid Performer
//	               └───────────────────────────────────────────────────
//	                  perf low          perf medium        perf high
var boxLabels = [9]string{
	"Risk", "Average Performer", "Solid Performer",
	"Inconsistent Player", "Core Player", "High Performer",
	"Potential Gem", "High Potential", "Star",
}

// Box computes the 9-box position from the two axes.
//
// ⚠ THE TWO ARGUMENTS ARE INDEPENDENT AND MUST STAY THAT WAY. There is
// deliberately no path from performance to potential anywhere in this
// package. If potential were derived from performance, every employee would
// land on the grid's diagonal and the whole instrument would carry exactly
// as much information as the appraisal rating already did — the difference
// between a strong performer who has outgrown their role and a struggling
// one who is new to a hard role is the only thing a 9-box is for.
//
// Returns ok=false rather than a default box for an invalid band: guessing
// "low" for a value nobody assessed would put a real person in the corner of
// the grid that ends careers.
func Box(performance, potential Band) (NineBox, bool) {
	p, q := performance.index(), potential.index()
	if p < 0 || q < 0 {
		return NineBox{}, false
	}
	n := q*3 + p
	return NineBox{
		Box:         n + 1,
		Label:       boxLabels[n],
		Performance: performance,
		Potential:   potential,
	}, true
}

// Performance-band thresholds, expressed as a fraction of the rating scale's
// maximum so they survive an org changing from a 5-point to a 10-point scale.
var (
	perfLowCeiling    = decimal.NewFromFloat(0.60)
	perfMediumCeiling = decimal.NewFromFloat(0.80)
)

// PerformanceBandFromRating derives the PERFORMANCE axis from a published
// appraisal rating. This derivation is legitimate and is the only one in the
// package: the appraisal IS the performance record.
//
// ⚠ Never call this to fill a potential band. Potential has no upstream
// record by design, which is why hrm_talent_assessments.potential_rationale
// is NOT NULL — the reason is the evidence.
//
// Returns ok=false when the inputs cannot produce an honest band (a
// non-positive scale maximum, or a rating outside it), so an unrated
// employee is left unplaced rather than assumed to be a low performer.
func PerformanceBandFromRating(rating, scaleMax decimal.Decimal) (Band, bool) {
	if scaleMax.LessThanOrEqual(decimal.Zero) || rating.IsNegative() {
		return "", false
	}
	if rating.GreaterThan(scaleMax) {
		return "", false
	}
	ratio := rating.Div(scaleMax)
	switch {
	case ratio.LessThan(perfLowCeiling):
		return BandLow, true
	case ratio.LessThan(perfMediumCeiling):
		return BandMedium, true
	default:
		return BandHigh, true
	}
}

// ParseBand normalises user input to a band.
func ParseBand(s string) (Band, bool) {
	b := Band(strings.ToLower(strings.TrimSpace(s)))
	return b, b.IsValid()
}
