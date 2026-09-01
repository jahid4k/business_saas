// backend/internal/hrm/entities/service.go
package entities

import (
	"context"
	"regexp"
	"strings"
)

// Caller carries the manage authorities the route gate established.
type Caller struct {
	UserID             string
	CanManageEntities  bool
	CanManageLocations bool
}

// Service is the legal-entity layer's business logic.
type Service interface {
	// Legal entities
	CreateEntity(ctx context.Context, orgID string, caller Caller, req CreateEntityRequest) (*LegalEntity, error)
	UpdateEntity(ctx context.Context, orgID string, caller Caller, ref string, req UpdateEntityRequest) (*LegalEntity, error)
	GetEntity(ctx context.Context, orgID string, ref string) (*LegalEntity, error)
	ListEntities(ctx context.Context, orgID string, activeOnly bool) ([]*LegalEntity, error)

	// Country configs
	UpsertCountryConfig(ctx context.Context, orgID string, caller Caller, req CountryConfigRequest) (*CountryConfig, error)
	ListCountryConfigs(ctx context.Context, orgID string, activeOnly bool) ([]*CountryConfig, error)

	// Locations
	CreateLocation(ctx context.Context, orgID string, caller Caller, req CreateLocationRequest) (*Location, error)
	UpdateLocation(ctx context.Context, orgID string, caller Caller, ref string, req UpdateLocationRequest) (*Location, error)
	ListLocations(ctx context.Context, orgID string, entityID *string, activeOnly bool) ([]*Location, error)

	// ResolveContext is the function every 11B consumer will call.
	ResolveContext(ctx context.Context, orgID string, legalEntityID *string) (*EntityContext, error)

	// BaseCurrency is ResolveContext narrowed to the one field most
	// consumers want, returned as a PRIMITIVE.
	//
	// ⚠ The primitive return type is the point. It lets internal/hrm/expenses
	// and internal/hrm/exits each declare their own one-method interface that
	// this type satisfies structurally, with no adapter and without either
	// package importing this one. Handing back *EntityContext would force
	// that import and make the dependency run the wrong way.
	BaseCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error)

	// CountryForEmployee resolves an employee's country AND says whether it
	// came from a LEGAL ENTITY rather than the organization profile.
	//
	// ⚠ The second return value is the whole point and must not be dropped.
	// organizations.country is a loosely-maintained profile field; a legal
	// entity's country is a declaration about where a company operates.
	// Consumers that gate money on country — statutory withholding — may only
	// act on the second kind. See statutory.ComputeForEmployee.
	CountryForEmployee(ctx context.Context, orgID, employeeID string) (country string, fromLegalEntity bool, err error)

	// DeclaredCurrency returns a currency ONLY when a LEGAL ENTITY declared
	// one, and an empty string otherwise.
	//
	// ⚠ IT DELIBERATELY DOES NOT FALL BACK TO organizations.currency, unlike
	// BaseCurrency above. That column is NOT NULL DEFAULT 'USD', so every
	// organization carries USD whether or not a human ever chose it, and
	// there is no way to tell a deliberate USD from an untouched default.
	//
	// Callers that LABEL money with a currency — a payroll run, a severance
	// figure, an award — must use this one. Reading the profile default would
	// silently relabel every existing organization's payslips on the day it
	// shipped. Callers that CONVERT money (expenses) may use BaseCurrency,
	// because a conversion only happens once the organization has
	// deliberately recorded a rate, and it is recorded with its full audit
	// set. Same principle as CountryForEmployee's fromLegalEntity flag: a
	// profile field is not a declaration.
	DeclaredCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

var (
	countryRe  = regexp.MustCompile(`^[A-Z]{2}$`)
	currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)
	cycles     = map[string]bool{"monthly": true, "semi_monthly": true, "biweekly": true, "weekly": true}
)

// normCode uppercases and trims an ISO code, returning nil for an empty
// string so "not set" and "set to blank" cannot diverge in the database.
func normCode(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.ToUpper(strings.TrimSpace(*p))
	if v == "" {
		return nil
	}
	return &v
}

// normText trims but does not case-fold. Applied to timezones because IANA
// zone names are case-sensitive — "Europe/London" is a zone and
// "EUROPE/LONDON" is not.
func normText(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return &v
}

func validCodes(country, currency *string) error {
	if country != nil && !countryRe.MatchString(*country) {
		return ErrInvalidCountry
	}
	if currency != nil && !currencyRe.MatchString(*currency) {
		return ErrInvalidCurrency
	}
	return nil
}

// ── legal entities ───────────────────────────────────────────────────────────

// CreateEntity records a company.
//
// ⚠ The FIRST entity an organization creates becomes its default
// automatically. Leaving an org with entities but no default would break step
// two of the resolution chain silently — every lookup would skip straight to
// the organization and quietly ignore the entity somebody just set up.
func (s *serviceImpl) CreateEntity(ctx context.Context, orgID string, caller Caller, req CreateEntityRequest) (*LegalEntity, error) {
	if !caller.CanManageEntities {
		return nil, ErrAccessDenied
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	country, currency := normCode(req.CountryCode), normCode(req.BaseCurrency)
	if err := validCodes(country, currency); err != nil {
		return nil, err
	}

	e := &LegalEntity{
		OrgID: orgID, Name: name, CountryCode: country, BaseCurrency: currency,
		RegistrationNumber: normText(req.RegistrationNumber),
		TaxIdentifier:      normText(req.TaxIdentifier),
		RegisteredAddress:  normText(req.RegisteredAddress),
		Timezone:           normText(req.Timezone),
		CreatedBy:          &caller.UserID,
	}
	if err := s.repo.CreateEntity(ctx, e); err != nil {
		return nil, err
	}

	existing, err := s.repo.CountEntities(ctx, orgID)
	if err != nil {
		return nil, err
	}
	makeDefault := existing == 1
	if req.IsDefault != nil && *req.IsDefault {
		makeDefault = true
	}
	if makeDefault {
		if err := s.repo.SetDefault(ctx, orgID, e.ID); err != nil {
			return nil, err
		}
	}
	return s.repo.FindEntityByRef(ctx, orgID, e.ID)
}

func (s *serviceImpl) UpdateEntity(ctx context.Context, orgID string, caller Caller, ref string, req UpdateEntityRequest) (*LegalEntity, error) {
	if !caller.CanManageEntities {
		return nil, ErrAccessDenied
	}
	e, err := s.repo.FindEntityByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, ErrEntityNotFound
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return nil, ErrNameRequired
		}
		e.Name = n
	}
	if req.CountryCode != nil {
		e.CountryCode = normCode(req.CountryCode)
	}
	if req.BaseCurrency != nil {
		e.BaseCurrency = normCode(req.BaseCurrency)
	}
	if err := validCodes(e.CountryCode, e.BaseCurrency); err != nil {
		return nil, err
	}
	if req.RegistrationNumber != nil {
		e.RegistrationNumber = normText(req.RegistrationNumber)
	}
	if req.TaxIdentifier != nil {
		e.TaxIdentifier = normText(req.TaxIdentifier)
	}
	if req.RegisteredAddress != nil {
		e.RegisteredAddress = normText(req.RegisteredAddress)
	}
	if req.Timezone != nil {
		e.Timezone = normText(req.Timezone)
	}
	if req.IsActive != nil {
		e.IsActive = *req.IsActive
	}

	// ⚠ Unsetting is_default is REFUSED, not obeyed. An organization with
	// entities and no default has no step two in its resolution chain, so
	// every lookup would silently skip to the organization and ignore the
	// entities entirely. A default is replaced by promoting another one.
	if req.IsDefault != nil && !*req.IsDefault && e.IsDefault {
		return nil, ErrCannotUndefault
	}
	if err := s.repo.UpdateEntity(ctx, e); err != nil {
		return nil, err
	}
	if req.IsDefault != nil && *req.IsDefault && !e.IsDefault {
		if err := s.repo.SetDefault(ctx, orgID, e.ID); err != nil {
			return nil, err
		}
	}
	return s.repo.FindEntityByRef(ctx, orgID, e.ID)
}

func (s *serviceImpl) GetEntity(ctx context.Context, orgID string, ref string) (*LegalEntity, error) {
	e, err := s.repo.FindEntityByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, ErrEntityNotFound
	}
	return e, nil
}

func (s *serviceImpl) ListEntities(ctx context.Context, orgID string, activeOnly bool) ([]*LegalEntity, error) {
	return s.repo.ListEntities(ctx, orgID, activeOnly)
}

// ── country configs ──────────────────────────────────────────────────────────

func (s *serviceImpl) UpsertCountryConfig(ctx context.Context, orgID string, caller Caller, req CountryConfigRequest) (*CountryConfig, error) {
	if !caller.CanManageEntities {
		return nil, ErrAccessDenied
	}
	country := normCode(&req.CountryCode)
	if country == nil || !countryRe.MatchString(*country) {
		return nil, ErrInvalidCountry
	}
	currency := normCode(req.DefaultCurrency)
	if currency != nil && !currencyRe.MatchString(*currency) {
		return nil, ErrInvalidCurrency
	}
	if req.PayrollCycle != nil {
		c := strings.ToLower(strings.TrimSpace(*req.PayrollCycle))
		if c != "" && !cycles[c] {
			return nil, ErrInvalidCycle
		}
		req.PayrollCycle = &c
	}
	if req.PayDayOfMonth != nil && (*req.PayDayOfMonth < 1 || *req.PayDayOfMonth > 31) {
		return nil, ErrInvalidPayDay
	}
	if req.FiscalYearStartMonth != nil && (*req.FiscalYearStartMonth < 1 || *req.FiscalYearStartMonth > 12) {
		return nil, ErrInvalidMonth
	}

	c := &CountryConfig{
		OrgID: orgID, CountryCode: *country, CountryName: normText(req.CountryName),
		DefaultCurrency: currency, PayrollCycle: normText(req.PayrollCycle),
		PayDayOfMonth: req.PayDayOfMonth, FiscalYearStartMonth: req.FiscalYearStartMonth,
		StandardWorkDaysPerWeek: req.StandardWorkDaysPerWeek,
		StandardHoursPerDay:     req.StandardHoursPerDay,
		OvertimeMultiplier:      req.OvertimeMultiplier,
		AnnualLeaveDays:         req.AnnualLeaveDays,
		NoticePeriodDays:        req.NoticePeriodDays,
		ProbationDays:           req.ProbationDays,
		GratuityEligibleYears:   req.GratuityEligibleYears,
		GratuityDaysPerYear:     req.GratuityDaysPerYear,
		CreatedBy:               caller.UserID,
	}
	if err := s.repo.UpsertCountryConfig(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) ListCountryConfigs(ctx context.Context, orgID string, activeOnly bool) ([]*CountryConfig, error) {
	return s.repo.ListCountryConfigs(ctx, orgID, activeOnly)
}

// ── locations ────────────────────────────────────────────────────────────────

func (s *serviceImpl) CreateLocation(ctx context.Context, orgID string, caller Caller, req CreateLocationRequest) (*Location, error) {
	if !caller.CanManageLocations {
		return nil, ErrAccessDenied
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	country := normCode(req.CountryCode)
	if err := validCodes(country, nil); err != nil {
		return nil, err
	}
	// A site may name an entity, but only one in THIS organization. The FK
	// enforces existence, not tenancy.
	entityID, err := s.resolveEntityRef(ctx, orgID, req.LegalEntityID)
	if err != nil {
		return nil, err
	}

	l := &Location{
		OrgID: orgID, LegalEntityID: entityID, Name: name, Code: normCode(req.Code),
		AddressLine1: normText(req.AddressLine1), AddressLine2: normText(req.AddressLine2),
		City: normText(req.City), State: normText(req.State),
		PostalCode: normText(req.PostalCode), CountryCode: country,
		Timezone: normText(req.Timezone), CreatedBy: caller.UserID,
	}
	if req.IsHeadquarters != nil {
		l.IsHeadquarters = *req.IsHeadquarters
	}
	if err := s.repo.CreateLocation(ctx, l); err != nil {
		return nil, err
	}
	return s.repo.FindLocationByRef(ctx, orgID, l.ID)
}

func (s *serviceImpl) UpdateLocation(ctx context.Context, orgID string, caller Caller, ref string, req UpdateLocationRequest) (*Location, error) {
	if !caller.CanManageLocations {
		return nil, ErrAccessDenied
	}
	l, err := s.repo.FindLocationByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, ErrLocationNotFound
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return nil, ErrNameRequired
		}
		l.Name = n
	}
	if req.LegalEntityID != nil {
		entityID, err := s.resolveEntityRef(ctx, orgID, req.LegalEntityID)
		if err != nil {
			return nil, err
		}
		l.LegalEntityID = entityID
	}
	if req.Code != nil {
		l.Code = normCode(req.Code)
	}
	if req.CountryCode != nil {
		l.CountryCode = normCode(req.CountryCode)
		if err := validCodes(l.CountryCode, nil); err != nil {
			return nil, err
		}
	}
	for _, f := range []struct{ src, dst **string }{
		{&req.AddressLine1, &l.AddressLine1}, {&req.AddressLine2, &l.AddressLine2},
		{&req.City, &l.City}, {&req.State, &l.State}, {&req.PostalCode, &l.PostalCode},
		{&req.Timezone, &l.Timezone},
	} {
		if *f.src != nil {
			*f.dst = normText(*f.src)
		}
	}
	if req.IsHeadquarters != nil {
		l.IsHeadquarters = *req.IsHeadquarters
	}
	if req.IsActive != nil {
		l.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateLocation(ctx, l); err != nil {
		return nil, err
	}
	return s.repo.FindLocationByRef(ctx, orgID, l.ID)
}

// resolveEntityRef maps a caller-supplied reference to an entity id in THIS
// organization, or nil for "no entity", which stays a valid answer.
func (s *serviceImpl) resolveEntityRef(ctx context.Context, orgID string, ref *string) (*string, error) {
	if ref == nil || strings.TrimSpace(*ref) == "" {
		return nil, nil
	}
	e, err := s.repo.FindEntityByRef(ctx, orgID, strings.TrimSpace(*ref))
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, ErrEntityNotFound
	}
	return &e.ID, nil
}

func (s *serviceImpl) ListLocations(ctx context.Context, orgID string, entityID *string, activeOnly bool) ([]*Location, error) {
	return s.repo.ListLocations(ctx, orgID, entityID, activeOnly)
}

// ── resolution ───────────────────────────────────────────────────────────────

// ResolveContext answers "what country, currency and working calendar apply
// here" by walking entity-specific → org default → organization.
//
// ⚠ A NIL legalEntityID IS NOT AN ERROR AND NEVER WILL BE. Every one of the
// 38 legal_entity_id columns in this schema is nullable and un-backfilled, so
// nil is what the overwhelming majority of rows carry. It means "whatever the
// organization's default is", and that answer stays correct when the default
// changes — which is exactly why nothing backfills them.
//
// ⚠ A MISSING COUNTRY CONFIG IS ALSO NOT AN ERROR. Most organizations will
// never record one, and every consumer must work without it.
func (s *serviceImpl) ResolveContext(ctx context.Context, orgID string, legalEntityID *string) (*EntityContext, error) {
	out := &EntityContext{OrgID: orgID}

	count, err := s.repo.CountEntities(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out.EntityCount = count
	out.SingleEntity = IsSingleEntity(count)

	specific := Candidate{Source: SourceEntity}
	if legalEntityID != nil && strings.TrimSpace(*legalEntityID) != "" {
		e, err := s.repo.FindEntityByRef(ctx, orgID, strings.TrimSpace(*legalEntityID))
		if err != nil {
			return nil, err
		}
		if e == nil {
			return nil, ErrEntityNotFound
		}
		specific = candidateFrom(e, SourceEntity)
		out.LegalEntityID, out.LegalEntityName = &e.ID, e.Name
	}

	fallback := Candidate{Source: SourceDefault}
	if def, err := s.repo.FindDefaultEntity(ctx, orgID); err != nil {
		return nil, err
	} else if def != nil {
		fallback = candidateFrom(def, SourceDefault)
		if out.LegalEntityID == nil {
			out.LegalEntityID, out.LegalEntityName = &def.ID, def.Name
		}
	}

	orgCandidate, err := s.repo.OrganizationDefaults(ctx, orgID)
	if err != nil {
		return nil, err
	}

	out.CountryCode, out.Currency, out.Timezone = Resolve(specific, fallback, orgCandidate)

	if out.CountryCode.Value != "" {
		cfg, err := s.repo.FindCountryConfig(ctx, orgID, out.CountryCode.Value)
		if err != nil {
			return nil, err
		}
		out.Config = cfg
		// ⚠ The config's currency is the LAST resort, below the
		// organization's own. A country config says what is typical in that
		// country; the organization saying it pays in USD is a statement
		// about itself and outranks a generality.
		if out.Currency.Value == "" && cfg != nil && cfg.DefaultCurrency != nil {
			out.Currency = Resolved{Value: *cfg.DefaultCurrency, Source: SourceOrg}
		}
	}
	return out, nil
}

func candidateFrom(e *LegalEntity, src Source) Candidate {
	return Candidate{
		Source: src, Present: true,
		CountryCode: deref(e.CountryCode), Currency: deref(e.BaseCurrency),
		Timezone: deref(e.Timezone), EntityID: e.ID, EntityName: e.Name,
	}
}

// BaseCurrency resolves just the currency, through the same chain.
//
// ⚠ Returns an EMPTY STRING, never a guess, when no currency is set anywhere.
// A caller that receives "" must decide what to do about it — usually skip
// the conversion and say so. Substituting a popular currency here would
// produce payslips and settlements in the wrong money, silently.
func (s *serviceImpl) BaseCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error) {
	entCtx, err := s.ResolveContext(ctx, orgID, legalEntityID)
	if err != nil {
		return "", err
	}
	return entCtx.Currency.Value, nil
}

// CountryForEmployee resolves an employee's country through the 11A chain and
// reports whether a legal entity supplied it.
//
// ⚠ fromLegalEntity is false when the country came from organizations.country
// or from nowhere at all. A caller deciding statutory deductions must treat
// that as "no country declared" rather than as a country, because
// organizations.country is a profile field somebody filled in once and
// withholding real money against it would be over-trusting it.
func (s *serviceImpl) CountryForEmployee(ctx context.Context, orgID, employeeID string) (string, bool, error) {
	entityID, err := s.repo.EmployeeLegalEntityID(ctx, orgID, employeeID)
	if err != nil {
		return "", false, err
	}
	entCtx, err := s.ResolveContext(ctx, orgID, entityID)
	if err != nil {
		return "", false, err
	}
	fromEntity := entCtx.CountryCode.Source == SourceEntity ||
		entCtx.CountryCode.Source == SourceDefault
	return entCtx.CountryCode.Value, fromEntity, nil
}

// DeclaredCurrency returns a currency only when a legal entity declared it.
func (s *serviceImpl) DeclaredCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error) {
	entCtx, err := s.ResolveContext(ctx, orgID, legalEntityID)
	if err != nil {
		return "", err
	}
	if entCtx.Currency.Source == SourceEntity || entCtx.Currency.Source == SourceDefault {
		return entCtx.Currency.Value, nil
	}
	return "", nil
}
