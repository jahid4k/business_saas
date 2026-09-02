// backend/internal/hrm/statutory/service.go
package statutory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

// Service covers statutory rules + slabs, and ComputeForEmployee, which
// structurally satisfies payslips.StatutorySource.
type Service interface {
	// SetCountryResolver attaches the legal-entity layer (11B-2). Optional:
	// without it every active rule applies to everybody, as before.
	SetCountryResolver(r EmployeeCountryResolver)

	ListRules(ctx context.Context, orgID string) ([]*Rule, error)
	GetRule(ctx context.Context, orgID, ref string) (*Rule, error)
	CreateRule(ctx context.Context, orgID, createdBy string, req CreateRuleRequest) (*Rule, error)
	SetRuleActive(ctx context.Context, orgID, ref string, active bool) (*Rule, error)
	CreateSlab(ctx context.Context, orgID, ruleRef, createdBy string, req CreateSlabRequest) (*Slab, error)
	ListSlabs(ctx context.Context, orgID, ruleRef string) ([]*Slab, error)

	// ComputeForEmployee satisfies payslips.StatutorySource structurally.
	// See that interface's doc comment in hrm/payslips/model.go for why
	// statutory imports payslips and not the reverse.
	ComputeForEmployee(ctx context.Context, orgID, employeeID string, year, month int, gross, basic, taxableGross decimal.Decimal) ([]payslips.StatutoryLine, error)
}

type serviceImpl struct {
	repo     Repository
	registry *Registry
	// countries is OPTIONAL (11B-2). Nil means every active rule applies to
	// everybody, which is exactly the behaviour this package shipped with.
	countries EmployeeCountryResolver
}

// NewService takes an explicit *Registry rather than constructing one
// internally, so main.go decides what country-specific providers exist
// alongside the default SlabProvider — the registration point, not this
// package, is where a real country implementation gets wired in later.
func NewService(repo Repository, registry *Registry) Service {
	return &serviceImpl{repo: repo, registry: registry}
}

func (s *serviceImpl) ListRules(ctx context.Context, orgID string) ([]*Rule, error) {
	return s.repo.ListRules(ctx, orgID)
}

func (s *serviceImpl) GetRule(ctx context.Context, orgID, ref string) (*Rule, error) {
	r, err := s.repo.FindRuleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("statutory: GetRule: %w", err)
	}
	if r == nil {
		return nil, ErrRuleNotFound
	}
	return r, nil
}

func (s *serviceImpl) CreateRule(ctx context.Context, orgID, createdBy string, req CreateRuleRequest) (*Rule, error) {
	ruleType := RuleType(strings.TrimSpace(req.RuleType))
	if !ruleType.IsValid() {
		return nil, ErrInvalidRuleType
	}
	base := BaseVariable(strings.TrimSpace(req.BaseVariable))
	if base == "" {
		base = BaseTaxableGross
	}
	if !base.IsValid() {
		return nil, ErrInvalidBase
	}
	country := strings.ToUpper(strings.TrimSpace(req.CountryCode))
	if len(country) != 2 {
		return nil, fmt.Errorf("statutory: CreateRule: country_code must be a 2-letter ISO code")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("statutory: CreateRule: name is required")
	}

	r := &Rule{
		OrgID: orgID, Name: name, CountryCode: country, RuleType: ruleType, BaseVariable: base,
		IsEmployerContribution: req.IsEmployerContribution, IsActive: true, CreatedBy: createdBy,
	}
	if err := s.repo.CreateRule(ctx, r); err != nil {
		return nil, fmt.Errorf("statutory: CreateRule: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) SetRuleActive(ctx context.Context, orgID, ref string, active bool) (*Rule, error) {
	r, err := s.repo.FindRuleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("statutory: SetRuleActive: %w", err)
	}
	if r == nil {
		return nil, ErrRuleNotFound
	}
	r.IsActive = active
	if err := s.repo.UpdateRule(ctx, r); err != nil {
		return nil, fmt.Errorf("statutory: SetRuleActive: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) CreateSlab(ctx context.Context, orgID, ruleRef, createdBy string, req CreateSlabRequest) (*Slab, error) {
	rule, err := s.repo.FindRuleByRef(ctx, orgID, ruleRef)
	if err != nil {
		return nil, fmt.Errorf("statutory: CreateSlab: %w", err)
	}
	if rule == nil {
		return nil, ErrRuleNotFound
	}
	rate, err := decimal.NewFromString(strings.TrimSpace(req.RatePct))
	if err != nil || rate.IsNegative() {
		return nil, ErrInvalidAmount
	}
	var upTo *decimal.Decimal
	if req.UpTo != nil && strings.TrimSpace(*req.UpTo) != "" {
		v, err := decimal.NewFromString(strings.TrimSpace(*req.UpTo))
		if err != nil || v.IsNegative() {
			return nil, ErrInvalidAmount
		}
		upTo = &v
	}
	eff, err := parseDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}

	slab := &Slab{RuleID: rule.ID, UpTo: upTo, RatePct: rate, EffectiveDate: *eff, CreatedBy: createdBy}
	if err := s.repo.CreateSlab(ctx, rule.ID, slab); err != nil {
		return nil, fmt.Errorf("statutory: CreateSlab: %w", err)
	}
	return slab, nil
}

func (s *serviceImpl) ListSlabs(ctx context.Context, orgID, ruleRef string) ([]*Slab, error) {
	rule, err := s.repo.FindRuleByRef(ctx, orgID, ruleRef)
	if err != nil {
		return nil, fmt.Errorf("statutory: ListSlabs: %w", err)
	}
	if rule == nil {
		return nil, ErrRuleNotFound
	}
	return s.repo.ListSlabsByRule(ctx, rule.ID)
}

// SetCountryResolver attaches the legal-entity layer (11B-2).
//
// Separate from NewService so existing construction sites keep compiling, the
// SetBonusSource shape used throughout payslips.
func (s *serviceImpl) SetCountryResolver(r EmployeeCountryResolver) { s.countries = r }

// activeRulesFor selects which statutory rules apply to one employee.
//
// ⚠ THIS IS THE MOST DANGEROUS DECISION IN PHASE 11, AND IT DELIBERATELY
// FAILS OPEN.
//
// The defect being fixed is real: ListActiveRules returns EVERY active rule
// for an org regardless of country, so a company operating in Germany and
// Britain applied both countries' deductions to everyone. But the fix must
// not create a worse defect in the other direction — narrowing to a country
// that turns out to be wrong means withholding NOTHING, and under-withholding
// statutory tax is a liability the employee discovers at the end of the year.
//
// So the rule set is narrowed ONLY when a LEGAL ENTITY has declared a
// country. Specifically NOT when the country came from
// organizations.country: that is a profile field somebody filled in during
// signup, it is not a statement about where payroll is run, and gating
// withholding on it would be over-trusting it.
//
// ⚠ The test suite CANNOT catch a mistake here. Test organizations set no
// country at all, so a strict filter would leave every existing test green
// while silently zeroing statutory deductions for real organizations that
// happen to have a country on their profile. That is why the condition is
// written to the narrowest safe case rather than the most obvious one.
//
// Every path that is not "a legal entity said so" returns the full rule set —
// bit-for-bit the pre-11B-2 behaviour.
func (s *serviceImpl) activeRulesFor(ctx context.Context, orgID, employeeID string) ([]*Rule, error) {
	if s.countries == nil {
		return s.repo.ListActiveRules(ctx, orgID)
	}
	country, fromLegalEntity, err := s.countries.CountryForEmployee(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("statutory: activeRulesFor: %w", err)
	}
	if !fromLegalEntity || strings.TrimSpace(country) == "" {
		return s.repo.ListActiveRules(ctx, orgID)
	}

	scoped, err := s.repo.ListActiveRulesForCountry(ctx, orgID, country)
	if err != nil {
		return nil, err
	}
	// ⚠ An entity declaring a country the org has no rules for falls back to
	// the full set rather than withholding nothing. A company that opens a
	// German subsidiary before writing its German rules should keep paying
	// what it was paying, visibly, rather than silently stop deducting.
	if len(scoped) == 0 {
		return s.repo.ListActiveRules(ctx, orgID)
	}
	return scoped, nil
}

func (s *serviceImpl) ComputeForEmployee(ctx context.Context, orgID, employeeID string, year, month int, gross, basic, taxableGross decimal.Decimal) ([]payslips.StatutoryLine, error) {
	rules, err := s.activeRulesFor(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("statutory: ComputeForEmployee: %w", err)
	}
	if len(rules) == 0 {
		return nil, nil
	}

	asOf := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lines := make([]payslips.StatutoryLine, 0, len(rules))
	for _, rule := range rules {
		slabs, err := s.repo.SlabsAsOf(ctx, rule.ID, asOf)
		if err != nil {
			return nil, fmt.Errorf("statutory: ComputeForEmployee: rule %s: %w", rule.ID, err)
		}
		if len(slabs) == 0 {
			continue // no bracket table effective yet for this rule — nothing to withhold
		}
		provider := s.registry.For(rule.CountryCode)
		amount := provider.Compute(rule, slabs, gross, basic, taxableGross)
		if amount.IsZero() {
			continue
		}
		lines = append(lines, payslips.StatutoryLine{
			RuleID: rule.ID, Description: rule.Name, Amount: amount,
			IsEmployerContribution: rule.IsEmployerContribution,
		})
	}
	return lines, nil
}

const dateLayout = "2006-01-02"

func parseDate(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("statutory: date is required")
	}
	d, err := time.Parse(dateLayout, v)
	if err != nil {
		return nil, fmt.Errorf("statutory: invalid date %q: %w", v, err)
	}
	return &d, nil
}
