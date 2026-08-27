// backend/internal/tests/unit/hrm/exits/notice_test.go
// The notice-shortfall arithmetic. Phase 9B turns this number into money
// charged to a departing employee, so it is pure and tested before anything
// calls it — the Amortize / BookValue / SettleAgainstAdvance / EvaluateSLA
// precedent.
package exits_test

import (
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/exits"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func ptr(t time.Time) *time.Time { return &t }

func TestNoticeShortfallDays(t *testing.T) {
	cases := []struct {
		name     string
		expected *time.Time
		actual   time.Time
		want     int
	}{
		{
			// The ordinary case: 30 days' notice required, left 10 days early.
			"left before the notice period ended",
			ptr(day(2026, time.March, 31)), day(2026, time.March, 21), 10,
		},
		{
			"served the full notice period",
			ptr(day(2026, time.March, 31)), day(2026, time.March, 31), 0,
		},
		{
			// Not a credit. Those extra days are ordinary salary, not a
			// negative shortfall that would cancel out a real debt elsewhere.
			"stayed longer than required is not a negative shortfall",
			ptr(day(2026, time.March, 31)), day(2026, time.April, 10), 0,
		},
		{
			// The case that matters most: the employer AGREED to forgo the
			// notice. Billing for it anyway is the worst failure available.
			"waived notice is never a shortfall",
			nil, day(2026, time.March, 1), 0,
		},
		{
			"a single day short",
			ptr(day(2026, time.March, 31)), day(2026, time.March, 30), 1,
		},
		{
			"across a month boundary",
			ptr(day(2026, time.April, 5)), day(2026, time.March, 20), 16,
		},
		{
			// 2028 is a leap year; Feb has 29 days.
			"across a leap day",
			ptr(day(2028, time.March, 1)), day(2028, time.February, 27), 3,
		},
		{
			"across a year boundary",
			ptr(day(2027, time.January, 10)), day(2026, time.December, 26), 15,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := exits.NoticeShortfallDays(c.expected, c.actual)
			if got != c.want {
				t.Errorf("NoticeShortfallDays = %d, want %d", got, c.want)
			}
			if got < 0 {
				t.Errorf("NoticeShortfallDays returned %d — a negative shortfall would credit a leaver", got)
			}
		})
	}
}

func TestNoticeShortfallDays_IgnoresTimeOfDay(t *testing.T) {
	// A caller passing a wall-clock time must not get an off-by-one. Same
	// calendar day, 23 hours apart.
	expected := time.Date(2026, time.March, 31, 23, 59, 0, 0, time.UTC)
	actual := time.Date(2026, time.March, 31, 0, 30, 0, 0, time.UTC)
	if got := exits.NoticeShortfallDays(&expected, actual); got != 0 {
		t.Errorf("NoticeShortfallDays = %d, want 0 — the same day is zero days short", got)
	}

	// And a whole day apart is exactly one, not two, regardless of hours.
	expected2 := time.Date(2026, time.April, 1, 1, 0, 0, 0, time.UTC)
	actual2 := time.Date(2026, time.March, 31, 22, 0, 0, 0, time.UTC)
	if got := exits.NoticeShortfallDays(&expected2, actual2); got != 1 {
		t.Errorf("NoticeShortfallDays = %d, want 1", got)
	}
}

func TestNoticeShortfallDays_HandlesNonUTCInput(t *testing.T) {
	// Postgres DATE columns come back as midnight UTC, but a request-parsed
	// date in another zone must not shift the day.
	zone := time.FixedZone("UTC+6", 6*3600)
	expected := time.Date(2026, time.March, 31, 0, 0, 0, 0, zone)
	actual := time.Date(2026, time.March, 25, 0, 0, 0, 0, zone)
	if got := exits.NoticeShortfallDays(&expected, actual); got != 6 {
		t.Errorf("NoticeShortfallDays = %d, want 6 across a non-UTC zone", got)
	}
}
