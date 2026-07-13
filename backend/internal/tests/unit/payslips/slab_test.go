// backend/internal/tests/unit/payslips/slab_test.go
package payslips

import (
	"testing"

	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
	hrmsalary "github.com/mridha/businesssaas/internal/hrm/salary"
)

func f(v float64) *float64 { return &v }

// twoBracket mirrors the exact example documented on hrm_salary_components.slab_config
// (migration 00023) and salary.SlabConfig: 5% up to 30,000, 10% on everything above.
func twoBracket() *hrmsalary.SlabConfig {
	return &hrmsalary.SlabConfig{
		BaseVariable: "GROSS",
		Slabs: []hrmsalary.Slab{
			{UpTo: f(30000), Rate: 0.05},
			{UpTo: nil, Rate: 0.10},
		},
	}
}

func TestComputeSlab_BelowFirstBracket(t *testing.T) {
	got := hrmpayslips.ComputeSlab(20000, twoBracket())
	want := 1000.0 // entirely inside the first bracket: 20000 * 0.05
	if got != want {
		t.Errorf("ComputeSlab(20000) = %v, want %v", got, want)
	}
}

func TestComputeSlab_ExactlyOnBoundary(t *testing.T) {
	got := hrmpayslips.ComputeSlab(30000, twoBracket())
	want := 1500.0 // 30000 * 0.05 — the boundary amount itself stays in the first bracket
	if got != want {
		t.Errorf("ComputeSlab(30000) = %v, want %v", got, want)
	}
}

func TestComputeSlab_AboveHighestDefinedBracket(t *testing.T) {
	got := hrmpayslips.ComputeSlab(100000, twoBracket())
	want := 8500.0 // 30000*0.05 (1500) + 70000*0.10 (7000)
	if got != want {
		t.Errorf("ComputeSlab(100000) = %v, want %v", got, want)
	}
}

func TestComputeSlab_ThreeBrackets_ProgressiveTaxStyle(t *testing.T) {
	cfg := &hrmsalary.SlabConfig{
		BaseVariable: "GROSS",
		Slabs: []hrmsalary.Slab{
			{UpTo: f(10000), Rate: 0.0},
			{UpTo: f(30000), Rate: 0.10},
			{UpTo: nil, Rate: 0.20},
		},
	}
	// 10000 @ 0% = 0, next 20000 (10000→30000) @ 10% = 2000, next 20000 (30000→50000) @ 20% = 4000
	got := hrmpayslips.ComputeSlab(50000, cfg)
	want := 6000.0
	if got != want {
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
			{UpTo: nil, Rate: 0.10},
			{UpTo: f(30000), Rate: 0.05},
		},
	}
	got := hrmpayslips.ComputeSlab(100000, cfg)
	want := 8500.0
	if got != want {
		t.Errorf("ComputeSlab with unsorted slabs = %v, want %v", got, want)
	}
}

func TestComputeSlab_ZeroOrNegativeBase(t *testing.T) {
	if got := hrmpayslips.ComputeSlab(0, twoBracket()); got != 0 {
		t.Errorf("ComputeSlab(0) = %v, want 0", got)
	}
	if got := hrmpayslips.ComputeSlab(-500, twoBracket()); got != 0 {
		t.Errorf("ComputeSlab(-500) = %v, want 0", got)
	}
}

func TestComputeSlab_NilConfigOrNoSlabs(t *testing.T) {
	if got := hrmpayslips.ComputeSlab(50000, nil); got != 0 {
		t.Errorf("ComputeSlab with nil config = %v, want 0", got)
	}
	empty := &hrmsalary.SlabConfig{BaseVariable: "GROSS"}
	if got := hrmpayslips.ComputeSlab(50000, empty); got != 0 {
		t.Errorf("ComputeSlab with no slabs = %v, want 0", got)
	}
}
