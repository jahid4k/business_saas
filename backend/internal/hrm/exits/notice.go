// backend/internal/hrm/exits/notice.go
package exits

import "time"

// NoticeShortfallDays returns how many days of notice the employee did not
// serve: the gap between the last working date their notice period entitled
// the employer to and the date they actually left.
//
// Phase 9B multiplies this by a daily rate to produce an F&F debit, which is
// why every degenerate case below has to have a defined answer rather than a
// panic or a negative — a wrong number here bills a departing person real
// money.
//
//   - A WAIVED notice period (expected == nil) is never a shortfall. The
//     employer agreed to forgo the time; charging for it anyway is the single
//     most damaging thing this function could get wrong.
//   - Leaving LATER than required is not a credit. The employee is owed
//     salary for those days through ordinary payroll, not a negative
//     shortfall that would quietly cancel out somebody else's debt.
//   - Only the calendar date matters. Two timestamps on the same day are
//     zero days apart however many hours separate them, so both sides are
//     truncated to midnight UTC before subtracting.
func NoticeShortfallDays(expectedLastWorkingDate *time.Time, actualLastWorkingDate time.Time) int {
	if expectedLastWorkingDate == nil {
		return 0
	}
	expected := truncateToDay(*expectedLastWorkingDate)
	actual := truncateToDay(actualLastWorkingDate)
	if !actual.Before(expected) {
		return 0
	}
	return int(expected.Sub(actual).Hours() / 24)
}

// truncateToDay drops the time of day, in UTC. Dates arrive from Postgres
// DATE columns (already midnight) and from request parsing (also midnight),
// but a caller passing time.Now() must not get an off-by-one from the clock.
func truncateToDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
