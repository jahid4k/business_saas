// backend/internal/tests/unit/payslips/references_gross_test.go
// referencesGross decides which evaluation stage a salary component lands in,
// so it decides pay. A component wrongly judged gross-INdependent is evaluated
// against a gross of zero; one wrongly judged dependent is excluded from the
// gross it should have contributed to. Both are silent wrong-money bugs, which
// is why the predicate is tested directly rather than only through its effects.
package payslips_test

import (
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

func strPtr(s string) *string { return &s }

func TestReferencesGross(t *testing.T) {
	cases := []struct {
		name       string
		calcMethod string
		formula    *string
		slabConfig []byte
		want       bool
	}{
		// Unambiguous by method.
		{"pct_of_gross always does", "pct_of_gross", nil, nil, true},
		{"fixed never does", "fixed", nil, nil, false},
		{"pct_of_basic never does", "pct_of_basic", nil, nil, false},
		{"manual never does", "manual", nil, nil, false},

		// Formulas are inspected textually, because the expression is evaluated
		// against an env map whose GROSS key is set per stage.
		{"formula naming GROSS", "formula", strPtr("GROSS * 0.1"), nil, true},
		{"formula naming GROSS mid-expression", "formula", strPtr("(BASIC + GROSS) / 2"), nil, true},
		{"formula on BASIC only", "formula", strPtr("BASIC * 0.5"), nil, false},
		{"formula on attendance only", "formula", strPtr("PRESENT_DAYS / WORK_DAYS * BASIC"), nil, false},
		{"nil formula", "formula", nil, nil, false},

		// Slabs declare their base explicitly.
		{"slab based on GROSS", "slab", nil, []byte(`{"base_variable":"GROSS","slabs":[]}`), true},
		{"slab based on BASIC", "slab", nil, []byte(`{"base_variable":"BASIC","slabs":[]}`), false},
		{"nil slab config", "slab", nil, nil, false},
		// Malformed config must not panic and must not be silently treated as
		// gross-dependent — that would drop it out of the gross it feeds.
		{"malformed slab config", "slab", nil, []byte(`{not json`), false},

		{"unknown method", "something_else", nil, nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := payslips.ReferencesGross(tc.calcMethod, tc.formula, tc.slabConfig)
			if got != tc.want {
				t.Errorf("ReferencesGross(%q, %v, %s) = %v, want %v",
					tc.calcMethod, tc.formula, string(tc.slabConfig), got, tc.want)
			}
		})
	}
}
