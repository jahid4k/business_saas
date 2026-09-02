// backend/internal/tests/unit/hrm/assets/depreciation_test.go
// BookValue is a number a user reads off an asset record. Pure (no DB) so it
// is tested here, before anything in the service calls it — the
// payslips.ComputeSlab / compensation.ApplyIncrease / loans.Amortize
// precedent.
package assets_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/assets"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal literal %q: %v", s, err)
	}
	return d
}

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func months(n int) *int { return &n }

func TestBookValue(t *testing.T) {
	cases := []struct {
		name      string
		cost      string
		purchased *time.Time
		life      *int
		asOf      time.Time
		want      string
	}{
		{
			name: "nil useful life means not depreciated — holds full cost",
			cost: "1200", purchased: date(2020, time.January, 1), life: nil,
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "1200",
		},
		{
			name: "zero useful life is treated as not depreciated, never a divide by zero",
			cost: "1200", purchased: date(2020, time.January, 1), life: months(0),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "1200",
		},
		{
			name: "nil purchase date means depreciation has not started",
			cost: "1200", purchased: nil, life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "1200",
		},
		{
			name: "brand new asset holds full cost",
			cost: "2400", purchased: date(2026, time.January, 1), life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "2400",
		},
		{
			name: "halfway through its life is worth half",
			cost: "2400", purchased: date(2025, time.January, 1), life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "1200",
		},
		{
			name: "one month in, 24-month life",
			cost: "2400", purchased: date(2026, time.January, 1), life: months(24),
			asOf: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC), want: "2300",
		},
		{
			name: "exactly at end of life is worth zero",
			cost: "2400", purchased: date(2024, time.January, 1), life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "0",
		},
		{
			name: "PAST end of life FLOORS at zero, never negative",
			cost: "2400", purchased: date(2020, time.January, 1), life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "0",
		},
		{
			name: "a future purchase date yields full cost, not a negative term",
			cost: "2400", purchased: date(2030, time.January, 1), life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "2400",
		},
		{
			name: "day of month is honoured — the 31st has not completed a month on the 1st",
			cost: "2400", purchased: date(2025, time.December, 31), life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "2400",
		},
		{
			name: "zero cost stays zero",
			cost: "0", purchased: date(2020, time.January, 1), life: months(24),
			asOf: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), want: "0",
		},
		{
			name: "non-divisible cost rounds to cents",
			cost: "1000", purchased: date(2025, time.October, 1), life: months(3),
			asOf: time.Date(2025, time.November, 1, 0, 0, 0, 0, time.UTC), want: "666.67",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := assets.BookValue(dec(t, c.cost), c.purchased, c.life, c.asOf)
			want := dec(t, c.want)
			if !got.Equal(want) {
				t.Errorf("BookValue = %s, want %s", got, want)
			}
		})
	}
}

func TestBookValue_NeverGoesNegativeAcrossALongLife(t *testing.T) {
	cost := dec(t, "5000")
	purchased := date(2020, time.January, 1)
	life := months(36)
	for i := 0; i < 120; i++ {
		asOf := purchased.AddDate(0, i, 0)
		got := assets.BookValue(cost, purchased, life, asOf)
		if got.IsNegative() {
			t.Fatalf("month %d: book value went negative: %s", i, got)
		}
		if got.GreaterThan(cost) {
			t.Fatalf("month %d: book value exceeded purchase cost: %s", i, got)
		}
	}
}
