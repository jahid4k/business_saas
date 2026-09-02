// backend/internal/tests/unit/payslips/slab_test.go
package payslips

import (
	"testing"

	"github.com/shopspring/decimal"

	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
	hrmsalary "github.com/mridha/businesssaas/internal/hrm/salary"
)

func f(v float64) *decimal.Decimal {
	d := decimal.NewFromFloat(v)
	return &d
}

func d(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v)
}

// twoBracket mirrors the exact example documented on hrm_salary_components.slab_config
// (migration 00023) and salary.SlabConfig: 5% up to 30,000, 10% on everything above.
func twoBracket() *hrmsalary.SlabConfig {
	return &hrmsalary.SlabConfig{
		BaseVariable: "GROSS",
		Slabs: []hrmsalary.Slab{
			{UpTo: f(30000), Rate: d(0.05)},
			{UpTo: nil, Rate: d(0.10)},
		},
	}
}

func TestComputeSlab_BelowFirstBracket(t *testing.T) {
	got := hrmpayslips.ComputeSlab(d(20000), twoBracket())
	want := d(1000.0) // entirely inside the first bracket: 20000 * 0.05
	if !got.Equal(want) {
		t.Errorf("ComputeSlab(20000) = %v, want %v", got, want)
	}
}

func TestComputeSlab_ExactlyOnBoundary(t *testing.T) {
	got := hrmpayslips.ComputeSlab(d(30000), twoBracket())
	want := d(1500.0) // 30000 * 0.05 — the boundary amount itself stays in the first bracket
	if !got.Equal(want) {
		t.Errorf("ComputeSlab(30000) = %v, want %v", got, want)
	}
}

func TestComputeSlab_AboveHighestDefinedBracket(t *testing.T) {
	got := hrmpayslips.ComputeSlab(d(100000), twoBracket())
	want := d(8500.0) // 30000*0.05 (1500) + 70000*0.10 (7000)
	if !got.Equal(want) {
		t.Errorf("ComputeSlab(100000) = %v, want %v", got, want)
	}
}

func TestComputeSlab_ThreeBrackets_ProgressiveTaxStyle(t *testing.T) {
	cfg := &hrmsalary.SlabConfig{
		BaseVariable: "GROSS",
		Slabs: []hrmsalary.Slab{
			{UpTo: f(10000), Rate: d(0.0)},
			{UpTo: f(30000), Rate: d(0.10)},
			{UpTo: nil, Rate: d(0.20)},
		},
	}
	// 10000 @ 0% = 0, next 20000 (10000→30000) @ 10% = 2000, next 20000 (30000→50000) @ 20% = 4000
	got := hrmpayslips.ComputeSlab(d(50000), cfg)
	want := d(6000.0)
	if !got.Equal(want) {
		t.Errorf("ComputeSlab(50000) = %v, want %v", got, want)
	}
}

func TestComputeSlab_UnsortedInputIsSortedDefensively(t *testing.T) {
	// Same two-bracket config as TestComputeSlab_AboveHighestDefinedBracket,
	// but the uncapped slab appears first in the slice. Slab validation at the
	// Setup layer only guarantees the nil UpTo is the *last structural entry*
	// server-side — it doesn't guarantee callers always hand ComputeSlab an
	// already-ascending slice, so this must not depend on input order.
	cfg := &hrmsalary.SlabConfig{
		BaseVariable: "GROSS",
		Slabs: []hrmsalary.Slab{
			{UpTo: nil, Rate: d(0.10)},
			{UpTo: f(30000), Rate: d(0.05)},
		},
	}
	got := hrmpayslips.ComputeSlab(d(100000), cfg)
	want := d(8500.0)
	if !got.Equal(want) {
		t.Errorf("ComputeSlab with unsorted slabs = %v, want %v", got, want)
	}
}

func TestComputeSlab_ZeroOrNegativeBase(t *testing.T) {
	if got := hrmpayslips.ComputeSlab(d(0), twoBracket()); !got.IsZero() {
		t.Errorf("ComputeSlab(0) = %v, want 0", got)
	}
	if got := hrmpayslips.ComputeSlab(d(-500), twoBracket()); !got.IsZero() {
		t.Errorf("ComputeSlab(-500) = %v, want 0", got)
	}
}

func TestComputeSlab_NilConfigOrNoSlabs(t *testing.T) {
	if got := hrmpayslips.ComputeSlab(d(50000), nil); !got.IsZero() {
		t.Errorf("ComputeSlab with nil config = %v, want 0", got)
	}
	empty := &hrmsalary.SlabConfig{BaseVariable: "GROSS"}
	if got := hrmpayslips.ComputeSlab(d(50000), empty); !got.IsZero() {
		t.Errorf("ComputeSlab with no slabs = %v, want 0", got)
	}
}

// TestComputeSlab_ExactAtPaisaPrecision is the regression guard for the
// float64 → decimal conversion, and it is not a hypothetical.
//
// The bracket walk used to run in float64 with its decimal inputs converted
// via InexactFloat64(). Scanning 28,572 ordinary salary bases against a
// three-bracket table, 42 of them produced a figure that was still WRONG after
// rounding to paisa. This is the statutory deduction on every payslip, so that
// is a wrong number on somebody's pay, not a rounding curiosity.
//
// Each case below is one of those bases. Against the old float
// implementation the first one returns 51.50; the exact answer is 51.51.
func TestComputeSlab_ExactAtPaisaPrecision(t *testing.T) {
	cfg := &hrmsalary.SlabConfig{
		BaseVariable: "GROSS",
		Slabs: []hrmsalary.Slab{
			{UpTo: f(30000), Rate: d(0.05)},
			{UpTo: f(80000), Rate: d(0.13)},
			{UpTo: nil, Rate: d(0.27)},
		},
	}
	cases := []struct{ base, want string }{
		{"1030.10", "51.51"}, // float said 51.50
		{"1065.10", "53.26"}, // float said 53.25
		{"1100.10", "55.01"}, // float said 55.00
		{"1135.10", "56.76"}, // float said 56.75
	}
	for _, c := range cases {
		base := decimal.RequireFromString(c.base)
		got := hrmpayslips.ComputeSlab(base, cfg).Round(2)
		want := decimal.RequireFromString(c.want)
		if !got.Equal(want) {
			t.Errorf("ComputeSlab(%s).Round(2) = %s, want %s — the bracket walk has "+
				"reverted to lossy arithmetic", c.base, got, want)
		}
	}
}

// TestComputeSlab_RateAndBoundaryStayExact proves the slab's own decimal
// fields are no longer flattened either — UpTo and Rate were both being run
// through InexactFloat64() inside the loop.
func TestComputeSlab_RateAndBoundaryStayExact(t *testing.T) {
	third := decimal.RequireFromString("0.333333333333333333")
	cfg := &hrmsalary.SlabConfig{
		BaseVariable: "GROSS",
		Slabs:        []hrmsalary.Slab{{UpTo: nil, Rate: third}},
	}
	got := hrmpayslips.ComputeSlab(decimal.NewFromInt(3), cfg)
	want := third.Mul(decimal.NewFromInt(3)) // 0.999999999999999999, exactly
	if !got.Equal(want) {
		t.Errorf("ComputeSlab = %s, want %s — a high-precision rate was flattened to float64", got, want)
	}
}
