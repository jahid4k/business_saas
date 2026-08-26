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

func (s *serviceImpl) ComputeForEmployee(ctx context.Context, orgID, employeeID string, year, month int, gross, basic, taxableGross decimal.Decimal) ([]payslips.StatutoryLine, error) {
	rules, err := s.repo.ListActiveRules(ctx, orgID)
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
