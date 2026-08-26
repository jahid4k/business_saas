// backend/internal/hrm/statutory/provider.go
package statutory

import (
	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/payslips"
	hrmsalary "github.com/mridha/businesssaas/internal/hrm/salary"
)

// Provider computes ONE statutory rule's amount against already-resolved
// bases. Pure — no ctx, no DB — so a provider is unit-testable directly; the
// service layer fetches a rule's slabs and calls this.
//
// This is the country-pluggable seam the build plan asks for. The bare
// interface would be a speculative primitive with zero real use — so
// SlabProvider below is shipped as ONE real, DATA-DRIVEN implementation,
// giving every org a working statutory engine from day one. A country
// needing proration rules or eligibility thresholds a slab table cannot
// express (the build plan's own words) registers its own Provider in the
// Registry without any schema change.
type Provider interface {
	Compute(rule *Rule, slabs []*Slab, gross, basic, taxableGross decimal.Decimal) decimal.Decimal
}

// Registry maps a rule's country_code to the Provider that evaluates it,
// falling back to a default when no country-specific override is
// registered. Country routing happens per RULE, not per org — see migration
// 00102's header on why country_code is not filtered by employee today: this
// still lets one org's provident-fund rule (say) use a country-specific
// provider while its income-tax rule falls through to the generic slab
// evaluator.
type Registry struct {
	providers map[string]Provider
	fallback  Provider
}

// NewRegistry builds a Registry with fallback as the provider used for any
// country_code with no explicit registration — SlabProvider in ordinary use.
func NewRegistry(fallback Provider) *Registry {
	return &Registry{providers: make(map[string]Provider), fallback: fallback}
}

// Register adds a country-specific provider. Calling it twice for the same
// code replaces the earlier registration.
func (r *Registry) Register(countryCode string, p Provider) {
	r.providers[countryCode] = p
}

// For returns the provider registered for countryCode, or the fallback if
// none was registered.
func (r *Registry) For(countryCode string) Provider {
	if p, ok := r.providers[countryCode]; ok {
		return p
	}
	return r.fallback
}

// baseValue resolves which of gross/basic/taxableGross a rule reads.
func baseValue(v BaseVariable, gross, basic, taxableGross decimal.Decimal) decimal.Decimal {
	switch v {
	case BaseGross:
		return gross
	case BaseBasic:
		return basic
	case BaseTaxableGross:
		return taxableGross
	default:
		return decimal.Zero
	}
}

// SlabProvider is the one shipped, data-driven Provider: it evaluates a
// rule's effective-dated bracket table via payslips.ComputeSlab — the exact
// function hrm_salary_components' own slab calc_method already uses (00023)
// — so the progressive-bracket arithmetic is shared and already tested, not
// reimplemented here.
type SlabProvider struct{}

func (SlabProvider) Compute(rule *Rule, slabs []*Slab, gross, basic, taxableGross decimal.Decimal) decimal.Decimal {
	if len(slabs) == 0 {
		return decimal.Zero
	}
	base := baseValue(rule.BaseVariable, gross, basic, taxableGross)

	cfg := toSlabConfig(rule.BaseVariable, slabs)
	amount := payslips.ComputeSlab(base.InexactFloat64(), cfg)
	return decimal.NewFromFloat(amount).Round(2)
}

// toSlabConfig adapts our effective-dated Slab rows (already filtered to the
// latest bracket table as of the compute date, by the caller) into the
// hrm/salary.SlabConfig shape ComputeSlab expects — the same struct
// hrm_salary_components.slab_config already unmarshals into. RatePct is
// stored as a percentage (5.00 = 5%); Slab.Rate is fractional (0.05).
func toSlabConfig(base BaseVariable, slabs []*Slab) *hrmsalary.SlabConfig {
	cfg := &hrmsalary.SlabConfig{
		BaseVariable: string(base),
		Slabs:        make([]hrmsalary.Slab, len(slabs)),
	}
	hundred := decimal.NewFromInt(100)
	for i, s := range slabs {
		cfg.Slabs[i] = hrmsalary.Slab{UpTo: s.UpTo, Rate: s.RatePct.Div(hundred)}
	}
	return cfg
}
