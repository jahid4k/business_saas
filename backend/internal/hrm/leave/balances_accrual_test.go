// backend/internal/hrm/leave/balances_accrual_test.go
package leave

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return d
}

func TestIsAccrualDue_Monthly(t *testing.T) {
	policyCreated := mustDate(t, "2026-01-15")
	hireDate := mustDate(t, "2020-01-01") // irrelevant for monthly

	cases := []struct {
		name string
		asOf string
		want bool
	}{
		{"not the 1st", "2026-02-15", false},
		{"1st of the same month the policy was created — no same-month grant", "2026-01-01", false},
		{"1st of the very next month after creation", "2026-02-01", true},
		{"1st of a much later month", "2026-06-01", true},
		{"1st of an earlier month than policy creation", "2025-12-01", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isAccrualDue(AccrualMonthly, policyCreated, hireDate, mustDate(t, c.asOf), false)
			if got != c.want {
				t.Errorf("isAccrualDue(monthly, asOf=%s) = %v, want %v", c.asOf, got, c.want)
			}
		})
	}
}

func TestIsAccrualDue_Annual(t *testing.T) {
	policyCreated := mustDate(t, "2026-03-15")
	hireDate := mustDate(t, "2020-01-01") // irrelevant for annual

	cases := []struct {
		name string
		asOf string
		want bool
	}{
		{"exact anniversary, next year", "2027-03-15", true},
		{"exact anniversary, same year (policy creation day itself)", "2026-03-15", true},
		{"wrong day, right month", "2027-03-16", false},
		{"wrong month, right day", "2027-04-15", false},
		{"unrelated date", "2027-07-01", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isAccrualDue(AccrualAnnual, policyCreated, hireDate, mustDate(t, c.asOf), false)
			if got != c.want {
				t.Errorf("isAccrualDue(annual, asOf=%s) = %v, want %v", c.asOf, got, c.want)
			}
		})
	}
}

func TestIsAccrualDue_OnJoining(t *testing.T) {
	policyCreated := mustDate(t, "2026-01-01")

	cases := []struct {
		name           string
		hireDate       string
		asOf           string
		alreadyAccrued bool
		want           bool
	}{
		{"hired after policy existed, never accrued — due", "2026-02-01", "2026-02-05", false, true},
		{"hired exactly when policy was created — due", "2026-01-01", "2026-01-02", false, true},
		{"hired before policy existed — never due, no retroactive grant", "2025-06-01", "2026-06-01", false, false},
		{"already accrued — never due again, regardless of date", "2026-02-01", "2026-06-01", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isAccrualDue(AccrualOnJoining, policyCreated, mustDate(t, c.hireDate), mustDate(t, c.asOf), c.alreadyAccrued)
			if got != c.want {
				t.Errorf("isAccrualDue(on_joining, hire=%s, asOf=%s, alreadyAccrued=%v) = %v, want %v",
					c.hireDate, c.asOf, c.alreadyAccrued, got, c.want)
			}
		})
	}
}

func TestIsAccrualDue_UnknownMethod(t *testing.T) {
	if isAccrualDue(AccrualMethod("bogus"), time.Now(), time.Now(), time.Now(), false) {
		t.Error("expected an unrecognized accrual method to never be due")
	}
}
