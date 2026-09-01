// backend/internal/hrm/succession/flightrisk.go
package succession

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// SignalType names one flight-risk indicator.
//
// ⚠ THERE IS NO SCORE, AND THERE IS DELIBERATELY NOWHERE TO PUT ONE.
// The build plan excludes predictive scoring, and this is where that
// exclusion has to hold: a single number would be acted on — pay rises,
// promotions, exclusions from projects — by people who cannot say what it
// means. Each signal below is a fact with a stated reason and a query behind
// it, so a reader can disagree with it on the evidence.
type SignalType string

const (
	SignalNoPromotion      SignalType = "no_promotion"
	SignalBelowBand        SignalType = "below_band"
	SignalManagerChurn     SignalType = "manager_churn"
	SignalAppraisalDecline SignalType = "appraisal_decline"
)

// Signal is one indicator. Detail is NOT optional: an indicator that cannot
// explain itself is the unexplainable score under another name.
type Signal struct {
	Type   SignalType `json:"type"`
	Detail string     `json:"detail"`
}

// Thresholds are the tuning of the signal set, gathered in one place so the
// numbers are visible rather than scattered through the evaluators.
const (
	// NoPromotionMonths — long enough that ordinary promotion cycles do not
	// trip it, short enough to notice before somebody has already decided.
	NoPromotionMonths = 36
	// MinTenureMonths — a new hire has not been passed over for anything.
	MinTenureMonths = 18
	// ManagerChurnWindowMonths / ManagerChurnThreshold — one manager change
	// is reorganisation; three in a year is nobody owning the relationship.
	ManagerChurnWindowMonths = 12
	ManagerChurnThreshold    = 3
)

// EvalNoPromotion fires when somebody with real tenure has gone a long time
// without a promotion. Tenure is required because "never promoted" is the
// normal state of a recent hire and flagging it says nothing.
func EvalNoPromotion(hireDate time.Time, lastPromotion *time.Time, asOf time.Time) (Signal, bool) {
	if monthsBetween(hireDate, asOf) < MinTenureMonths {
		return Signal{}, false
	}
	since := hireDate
	phrase := "since being hired"
	if lastPromotion != nil {
		since = *lastPromotion
		phrase = "since the last promotion"
	}
	months := monthsBetween(since, asOf)
	if months < NoPromotionMonths {
		return Signal{}, false
	}
	return Signal{
		Type:   SignalNoPromotion,
		Detail: fmt.Sprintf("%d months %s (threshold %d)", months, phrase, NoPromotionMonths),
	}, true
}

// EvalBelowBand fires when current basic pay sits under the minimum of the
// employee's own grade band. Below the MINIMUM, not below the midpoint:
// half of any healthy band is under its midpoint by construction, so a
// midpoint test would flag half the company and mean nothing.
func EvalBelowBand(basicPay, bandMin decimal.Decimal, gradeLabel string) (Signal, bool) {
	if bandMin.LessThanOrEqual(decimal.Zero) || basicPay.LessThanOrEqual(decimal.Zero) {
		return Signal{}, false
	}
	if !basicPay.LessThan(bandMin) {
		return Signal{}, false
	}
	shortfall := bandMin.Sub(basicPay)
	return Signal{
		Type: SignalBelowBand,
		Detail: fmt.Sprintf("basic pay %s is %s below the minimum %s of grade %s",
			basicPay.StringFixed(2), shortfall.StringFixed(2), bandMin.StringFixed(2), gradeLabel),
	}, true
}

// EvalManagerChurn fires on repeated changes of solid-line manager, counted
// from the 10A relationship history.
func EvalManagerChurn(changes int) (Signal, bool) {
	if changes < ManagerChurnThreshold {
		return Signal{}, false
	}
	return Signal{
		Type: SignalManagerChurn,
		Detail: fmt.Sprintf("%d manager changes in the last %d months (threshold %d)",
			changes, ManagerChurnWindowMonths, ManagerChurnThreshold),
	}, true
}

// EvalAppraisalDecline fires when the most recent published rating is lower
// than the one before it. Any drop counts: the signal is the direction, and
// deciding how big a drop matters is precisely the judgement this returns to
// a human rather than encoding as a threshold.
func EvalAppraisalDecline(previous, latest decimal.Decimal) (Signal, bool) {
	if previous.LessThanOrEqual(decimal.Zero) || latest.LessThanOrEqual(decimal.Zero) {
		return Signal{}, false
	}
	if !latest.LessThan(previous) {
		return Signal{}, false
	}
	return Signal{
		Type: SignalAppraisalDecline,
		Detail: fmt.Sprintf("appraisal rating fell from %s to %s",
			previous.StringFixed(2), latest.StringFixed(2)),
	}, true
}

// monthsBetween counts whole elapsed months. Calendar months rather than
// 30-day blocks: "36 months since your last promotion" is how the fact is
// stated to a person, and a day-count would drift away from that.
func monthsBetween(from, to time.Time) int {
	if to.Before(from) {
		return 0
	}
	months := int(to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}
