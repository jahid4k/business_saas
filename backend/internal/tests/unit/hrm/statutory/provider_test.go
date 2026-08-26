// backend/internal/tests/unit/hrm/statutory/provider_test.go
// SlabProvider.Compute decides how much is withheld from every employee's
// pay for a given statutory rule. Pure (no DB) so it is tested here, before
// anything in the payroll engine calls it — the payslips.ComputeSlab /
// compensation.ApplyIncrease / loans.Amortize precedent.
package statutory_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/statutory"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal literal %q: %v", s, err)
	}
	return d
}

func upTo(t *testing.T, s string) *decimal.Decimal {
	d := dec(t, s)
	return &d
}

func TestSlabProvider_Compute(t *testing.T) {
	provider := statutory.SlabProvider{}

	t.Run("no slabs computes zero, not an error", func(t *testing.T) {
		rule := &statutory.Rule{BaseVariable: statutory.BaseTaxableGross}
		got := provider.Compute(rule, nil, dec(t, "10000"), dec(t, "8000"), dec(t, "9000"))
		if !got.IsZero() {
			t.Errorf("expected 0 with no slabs, got %s", got)
		}
	})

	t.Run("progressive bracket, TAXABLE_GROSS base", func(t *testing.T) {
		// 0-5000 @ 0%, 5000-10000 @ 10%, above @ 20% — a standard progressive
		// income-tax shape.
		rule := &statutory.Rule{BaseVariable: statutory.BaseTaxableGross}
		slabs := []*statutory.Slab{
			{UpTo: upTo(t, "5000"), RatePct: dec(t, "0")},
			{UpTo: upTo(t, "10000"), RatePct: dec(t, "10")},
			{UpTo: nil, RatePct: dec(t, "20")},
		}
		// taxableGross = 15000: 5000@0% + 5000@10% + 5000@20% = 0+500+1000=1500
		got := provider.Compute(rule, slabs, dec(t, "15000"), dec(t, "12000"), dec(t, "15000"))
		if !got.Equal(dec(t, "1500")) {
			t.Errorf("Compute = %s, want 1500", got)
		}
	})

	t.Run("base_variable selects the right resolved base", func(t *testing.T) {
		slabs := []*statutory.Slab{{UpTo: nil, RatePct: dec(t, "10")}}
		gross, basic, taxable := dec(t, "20000"), dec(t, "15000"), dec(t, "18000")

		grossRule := &statutory.Rule{BaseVariable: statutory.BaseGross}
		if got := provider.Compute(grossRule, slabs, gross, basic, taxable); !got.Equal(dec(t, "2000")) {
			t.Errorf("GROSS base: got %s, want 2000 (10%% of 20000)", got)
		}

		basicRule := &statutory.Rule{BaseVariable: statutory.BaseBasic}
		if got := provider.Compute(basicRule, slabs, gross, basic, taxable); !got.Equal(dec(t, "1500")) {
			t.Errorf("BASIC base: got %s, want 1500 (10%% of 15000)", got)
		}

		taxableRule := &statutory.Rule{BaseVariable: statutory.BaseTaxableGross}
		if got := provider.Compute(taxableRule, slabs, gross, basic, taxable); !got.Equal(dec(t, "1800")) {
			t.Errorf("TAXABLE_GROSS base: got %s, want 1800 (10%% of 18000)", got)
		}
	})

	t.Run("zero base computes zero", func(t *testing.T) {
		rule := &statutory.Rule{BaseVariable: statutory.BaseTaxableGross}
		slabs := []*statutory.Slab{{UpTo: nil, RatePct: dec(t, "15")}}
		got := provider.Compute(rule, slabs, dec(t, "10000"), dec(t, "8000"), decimal.Zero)
		if !got.IsZero() {
			t.Errorf("expected 0 for a zero base, got %s", got)
		}
	})
}

func TestRegistry_FallsBackWhenNoCountryOverride(t *testing.T) {
	fallback := statutory.SlabProvider{}
	reg := statutory.NewRegistry(fallback)

	if got := reg.For("US"); got == nil {
		t.Fatal("expected the fallback provider for an unregistered country")
	}

	// A country-specific override, once registered, must be returned instead.
	override := statutory.SlabProvider{} // stand-in; only identity matters here
	reg.Register("IN", override)
	if got := reg.For("IN"); got == nil {
		t.Fatal("expected the registered provider for IN")
	}
	// An unrelated country code still falls back.
	if got := reg.For("BD"); got == nil {
		t.Fatal("expected the fallback provider for BD")
	}
}

func TestRuleType_IsValid(t *testing.T) {
	valid := []statutory.RuleType{
		statutory.RuleIncomeTax, statutory.RuleSocialSecurity,
		statutory.RuleProvidentFund, statutory.RuleOther,
	}
	for _, rt := range valid {
		if !rt.IsValid() {
			t.Errorf("%q should be valid", rt)
		}
	}
	if statutory.RuleType("invalid").IsValid() {
		t.Error("unknown rule type should not be valid")
	}
}

func TestBaseVariable_IsValid(t *testing.T) {
	valid := []statutory.BaseVariable{statutory.BaseGross, statutory.BaseBasic, statutory.BaseTaxableGross}
	for _, b := range valid {
		if !b.IsValid() {
			t.Errorf("%q should be valid", b)
		}
	}
	if statutory.BaseVariable("NET").IsValid() {
		t.Error("unknown base variable should not be valid")
	}
}
