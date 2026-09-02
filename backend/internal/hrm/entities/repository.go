// backend/internal/hrm/entities/repository.go
package entities

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the legal-entity layer's data access.
type Repository interface {
	// Legal entities
	CreateEntity(ctx context.Context, e *LegalEntity) error
	UpdateEntity(ctx context.Context, e *LegalEntity) error
	FindEntityByRef(ctx context.Context, orgID, ref string) (*LegalEntity, error)
	FindDefaultEntity(ctx context.Context, orgID string) (*LegalEntity, error)
	ListEntities(ctx context.Context, orgID string, activeOnly bool) ([]*LegalEntity, error)
	CountEntities(ctx context.Context, orgID string) (int, error)
	// SetDefault promotes one entity and demotes the rest in a single
	// transaction. Two defaults would make step two of the resolution chain
	// ambiguous, and idx_hrm_legal_entities_org_default would reject the
	// second write anyway — but with a raw 23505 rather than an answer.
	SetDefault(ctx context.Context, orgID, entityID string) error

	// Country configs
	UpsertCountryConfig(ctx context.Context, c *CountryConfig) error
	FindCountryConfig(ctx context.Context, orgID, countryCode string) (*CountryConfig, error)
	FindConfigByRef(ctx context.Context, orgID, ref string) (*CountryConfig, error)
	ListCountryConfigs(ctx context.Context, orgID string, activeOnly bool) ([]*CountryConfig, error)

	// Locations
	CreateLocation(ctx context.Context, l *Location) error
	UpdateLocation(ctx context.Context, l *Location) error
	FindLocationByRef(ctx context.Context, orgID, ref string) (*Location, error)
	ListLocations(ctx context.Context, orgID string, entityID *string, activeOnly bool) ([]*Location, error)

	// OrganizationDefaults is the LAST link in the resolution chain and the
	// reason an org with no entities keeps working.
	OrganizationDefaults(ctx context.Context, orgID string) (Candidate, error)

	// EmployeeLegalEntityID returns the entity an employee belongs to, or nil
	// when they belong to none — which is every employee in this database.
	EmployeeLegalEntityID(ctx context.Context, orgID, employeeID string) (*string, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── legal entities ───────────────────────────────────────────────────────────

const entitySel = `id::text, public_id, org_id::text, name, is_default, country_code,
	base_currency, registration_number, tax_identifier, registered_address, timezone,
	is_active, created_by::text, created_at, updated_at`

func scanEntity(row pgx.Row) (*LegalEntity, error) {
	e := &LegalEntity{}
	err := row.Scan(&e.ID, &e.PublicID, &e.OrgID, &e.Name, &e.IsDefault, &e.CountryCode,
		&e.BaseCurrency, &e.RegistrationNumber, &e.TaxIdentifier, &e.RegisteredAddress,
		&e.Timezone, &e.IsActive, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *repoImpl) CreateEntity(ctx context.Context, e *LegalEntity) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_legal_entities
		   (org_id, name, country_code, base_currency, registration_number, tax_identifier,
		    registered_address, timezone, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, is_default, is_active, created_at, updated_at`,
		e.OrgID, e.Name, e.CountryCode, e.BaseCurrency, e.RegistrationNumber,
		e.TaxIdentifier, e.RegisteredAddress, e.Timezone, e.CreatedBy,
	).Scan(&e.ID, &e.PublicID, &e.IsDefault, &e.IsActive, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("entities: CreateEntity: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateEntity(ctx context.Context, e *LegalEntity) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_legal_entities
		    SET name=$3, country_code=$4, base_currency=$5, registration_number=$6,
		        tax_identifier=$7, registered_address=$8, timezone=$9, is_active=$10,
		        updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		e.OrgID, e.ID, e.Name, e.CountryCode, e.BaseCurrency, e.RegistrationNumber,
		e.TaxIdentifier, e.RegisteredAddress, e.Timezone, e.IsActive)
	if err != nil {
		return fmt.Errorf("entities: UpdateEntity: %w", err)
	}
	return nil
}

func (r *repoImpl) FindEntityByRef(ctx context.Context, orgID, ref string) (*LegalEntity, error) {
	e, err := scanEntity(r.db.QueryRow(ctx,
		`SELECT `+entitySel+` FROM hrm_legal_entities
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entities: FindEntityByRef: %w", err)
	}
	return e, nil
}

// FindDefaultEntity is step two of the resolution chain.
//
// It returns nil rather than an error when there is no default — an
// organization with no entities is the normal case, not a misconfiguration,
// and every caller has to survive it.
func (r *repoImpl) FindDefaultEntity(ctx context.Context, orgID string) (*LegalEntity, error) {
	e, err := scanEntity(r.db.QueryRow(ctx,
		`SELECT `+entitySel+` FROM hrm_legal_entities
		  WHERE org_id=$1 AND is_default AND is_active`, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entities: FindDefaultEntity: %w", err)
	}
	return e, nil
}

func (r *repoImpl) ListEntities(ctx context.Context, orgID string, activeOnly bool) ([]*LegalEntity, error) {
	q := `SELECT ` + entitySel + ` FROM hrm_legal_entities WHERE org_id=$1`
	if activeOnly {
		q += ` AND is_active`
	}
	q += ` ORDER BY is_default DESC, name`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("entities: ListEntities: %w", err)
	}
	defer rows.Close()
	out := []*LegalEntity{}
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("entities: ListEntities: scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountEntities(ctx context.Context, orgID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT count(*)::int FROM hrm_legal_entities WHERE org_id=$1 AND is_active`,
		orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("entities: CountEntities: %w", err)
	}
	return n, nil
}

// SetDefault demotes then promotes, inside one transaction.
//
// ⚠ The ORDER is load-bearing. idx_hrm_legal_entities_org_default is a
// partial unique index on (org_id) WHERE is_default, so promoting before
// demoting would violate it mid-transaction and fail. Demote-then-promote
// passes through a valid state at every step.
func (r *repoImpl) SetDefault(ctx context.Context, orgID, entityID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entities: SetDefault: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE hrm_legal_entities SET is_default=FALSE, updated_at=NOW()
		  WHERE org_id=$1 AND is_default AND id <> $2::uuid`, orgID, entityID); err != nil {
		return fmt.Errorf("entities: SetDefault: demote: %w", err)
	}
	ct, err := tx.Exec(ctx,
		`UPDATE hrm_legal_entities SET is_default=TRUE, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`, orgID, entityID)
	if err != nil {
		return fmt.Errorf("entities: SetDefault: promote: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrEntityNotFound
	}
	return tx.Commit(ctx)
}

// ── country configs ──────────────────────────────────────────────────────────

const configSel = `id::text, public_id, org_id::text, country_code, country_name,
	default_currency, payroll_cycle, pay_day_of_month, fiscal_year_start_month,
	standard_work_days_per_week, standard_hours_per_day, overtime_multiplier,
	annual_leave_days, notice_period_days, probation_days,
	gratuity_eligible_years, gratuity_days_per_year,
	is_active, created_by::text, created_at, updated_at`

func scanConfig(row pgx.Row) (*CountryConfig, error) {
	c := &CountryConfig{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.CountryCode, &c.CountryName,
		&c.DefaultCurrency, &c.PayrollCycle, &c.PayDayOfMonth, &c.FiscalYearStartMonth,
		&c.StandardWorkDaysPerWeek, &c.StandardHoursPerDay, &c.OvertimeMultiplier,
		&c.AnnualLeaveDays, &c.NoticePeriodDays, &c.ProbationDays,
		&c.GratuityEligibleYears, &c.GratuityDaysPerYear,
		&c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// UpsertCountryConfig treats a second write for the same country as an edit.
// Two configurations for one country would be two answers to "what is the
// statutory notice period in Germany", with nothing to say which wins.
func (r *repoImpl) UpsertCountryConfig(ctx context.Context, c *CountryConfig) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_country_configs
		   (org_id, country_code, country_name, default_currency, payroll_cycle,
		    pay_day_of_month, fiscal_year_start_month, standard_work_days_per_week,
		    standard_hours_per_day, overtime_multiplier, annual_leave_days,
		    notice_period_days, probation_days, gratuity_eligible_years,
		    gratuity_days_per_year, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		 ON CONFLICT (org_id, country_code) DO UPDATE SET
		    country_name=EXCLUDED.country_name,
		    default_currency=EXCLUDED.default_currency,
		    payroll_cycle=EXCLUDED.payroll_cycle,
		    pay_day_of_month=EXCLUDED.pay_day_of_month,
		    fiscal_year_start_month=EXCLUDED.fiscal_year_start_month,
		    standard_work_days_per_week=EXCLUDED.standard_work_days_per_week,
		    standard_hours_per_day=EXCLUDED.standard_hours_per_day,
		    overtime_multiplier=EXCLUDED.overtime_multiplier,
		    annual_leave_days=EXCLUDED.annual_leave_days,
		    notice_period_days=EXCLUDED.notice_period_days,
		    probation_days=EXCLUDED.probation_days,
		    gratuity_eligible_years=EXCLUDED.gratuity_eligible_years,
		    gratuity_days_per_year=EXCLUDED.gratuity_days_per_year,
		    updated_at=NOW()
		 RETURNING id, public_id, is_active, created_at, updated_at`,
		c.OrgID, c.CountryCode, c.CountryName, c.DefaultCurrency, c.PayrollCycle,
		c.PayDayOfMonth, c.FiscalYearStartMonth, c.StandardWorkDaysPerWeek,
		c.StandardHoursPerDay, c.OvertimeMultiplier, c.AnnualLeaveDays,
		c.NoticePeriodDays, c.ProbationDays, c.GratuityEligibleYears,
		c.GratuityDaysPerYear, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("entities: UpsertCountryConfig: %w", err)
	}
	return nil
}

func (r *repoImpl) FindCountryConfig(ctx context.Context, orgID, countryCode string) (*CountryConfig, error) {
	if strings.TrimSpace(countryCode) == "" {
		return nil, nil
	}
	c, err := scanConfig(r.db.QueryRow(ctx,
		`SELECT `+configSel+` FROM hrm_country_configs
		  WHERE org_id=$1 AND country_code=$2 AND is_active`, orgID, strings.ToUpper(countryCode)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entities: FindCountryConfig: %w", err)
	}
	return c, nil
}

func (r *repoImpl) FindConfigByRef(ctx context.Context, orgID, ref string) (*CountryConfig, error) {
	c, err := scanConfig(r.db.QueryRow(ctx,
		`SELECT `+configSel+` FROM hrm_country_configs
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entities: FindConfigByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) ListCountryConfigs(ctx context.Context, orgID string, activeOnly bool) ([]*CountryConfig, error) {
	q := `SELECT ` + configSel + ` FROM hrm_country_configs WHERE org_id=$1`
	if activeOnly {
		q += ` AND is_active`
	}
	q += ` ORDER BY country_code`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("entities: ListCountryConfigs: %w", err)
	}
	defer rows.Close()
	out := []*CountryConfig{}
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("entities: ListCountryConfigs: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ── locations ────────────────────────────────────────────────────────────────

const locationSel = `l.id::text, l.public_id, l.org_id::text, l.legal_entity_id::text,
	COALESCE(e.name,''), l.name, l.code, l.address_line1, l.address_line2, l.city, l.state,
	l.postal_code, l.country_code, l.timezone, l.is_headquarters, l.is_active,
	l.created_by::text, l.created_at, l.updated_at,
	(SELECT count(*)::int FROM hrm_employees emp
	  WHERE emp.location_id = l.id AND emp.termination_date IS NULL)`

const locationFrom = ` FROM hrm_locations l
	LEFT JOIN hrm_legal_entities e ON e.id = l.legal_entity_id`

func scanLocation(row pgx.Row) (*Location, error) {
	l := &Location{}
	err := row.Scan(&l.ID, &l.PublicID, &l.OrgID, &l.LegalEntityID, &l.LegalEntityName,
		&l.Name, &l.Code, &l.AddressLine1, &l.AddressLine2, &l.City, &l.State,
		&l.PostalCode, &l.CountryCode, &l.Timezone, &l.IsHeadquarters, &l.IsActive,
		&l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &l.EmployeeCount)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *repoImpl) CreateLocation(ctx context.Context, l *Location) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_locations
		   (org_id, legal_entity_id, name, code, address_line1, address_line2, city, state,
		    postal_code, country_code, timezone, is_headquarters, created_by)
		 VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 RETURNING id, public_id, is_active, created_at, updated_at`,
		l.OrgID, l.LegalEntityID, l.Name, l.Code, l.AddressLine1, l.AddressLine2, l.City,
		l.State, l.PostalCode, l.CountryCode, l.Timezone, l.IsHeadquarters, l.CreatedBy,
	).Scan(&l.ID, &l.PublicID, &l.IsActive, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return translateLocationErr(err, "CreateLocation")
	}
	return nil
}

func (r *repoImpl) UpdateLocation(ctx context.Context, l *Location) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_locations
		    SET legal_entity_id=$3::uuid, name=$4, code=$5, address_line1=$6, address_line2=$7,
		        city=$8, state=$9, postal_code=$10, country_code=$11, timezone=$12,
		        is_headquarters=$13, is_active=$14, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		l.OrgID, l.ID, l.LegalEntityID, l.Name, l.Code, l.AddressLine1, l.AddressLine2,
		l.City, l.State, l.PostalCode, l.CountryCode, l.Timezone, l.IsHeadquarters, l.IsActive)
	if err != nil {
		return translateLocationErr(err, "UpdateLocation")
	}
	return nil
}

// translateLocationErr turns the two partial unique indexes into answers.
// A raw 23505 tells a caller nothing about which rule they broke.
func translateLocationErr(err error, op string) error {
	switch {
	case strings.Contains(err.Error(), "uq_hrm_loc_active_code"):
		return ErrDuplicateCode
	case strings.Contains(err.Error(), "uq_hrm_loc_headquarters"):
		return ErrHeadquartersTaken
	default:
		return fmt.Errorf("entities: %s: %w", op, err)
	}
}

func (r *repoImpl) FindLocationByRef(ctx context.Context, orgID, ref string) (*Location, error) {
	l, err := scanLocation(r.db.QueryRow(ctx,
		`SELECT `+locationSel+locationFrom+`
		  WHERE l.org_id=$1 AND (l.id::text=$2 OR l.public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entities: FindLocationByRef: %w", err)
	}
	return l, nil
}

func (r *repoImpl) ListLocations(ctx context.Context, orgID string, entityID *string, activeOnly bool) ([]*Location, error) {
	q := `SELECT ` + locationSel + locationFrom + ` WHERE l.org_id=$1`
	args := []any{orgID}
	if entityID != nil && strings.TrimSpace(*entityID) != "" {
		q += ` AND l.legal_entity_id=$2::uuid`
		args = append(args, *entityID)
	}
	if activeOnly {
		q += ` AND l.is_active`
	}
	q += ` ORDER BY l.is_headquarters DESC, l.name`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("entities: ListLocations: %w", err)
	}
	defer rows.Close()
	out := []*Location{}
	for rows.Next() {
		l, err := scanLocation(rows)
		if err != nil {
			return nil, fmt.Errorf("entities: ListLocations: scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// OrganizationDefaults is the LAST link in the chain, and the reason an
// organization with no legal entities keeps working exactly as it did before
// Phase 11 existed.
func (r *repoImpl) OrganizationDefaults(ctx context.Context, orgID string) (Candidate, error) {
	var country, currency, timezone *string
	err := r.db.QueryRow(ctx,
		`SELECT country, currency, timezone FROM organizations WHERE id=$1`, orgID,
	).Scan(&country, &currency, &timezone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Candidate{Source: SourceOrg}, nil
	}
	if err != nil {
		return Candidate{}, fmt.Errorf("entities: OrganizationDefaults: %w", err)
	}
	return Candidate{
		Source: SourceOrg, Present: true,
		CountryCode: deref(country), Currency: deref(currency), Timezone: deref(timezone),
	}, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// EmployeeLegalEntityID reads hrm_employees.legal_entity_id.
//
// ⚠ nil is the NORMAL answer, not an error. The column is nullable and
// un-backfilled on every one of the 39 tables that carries it, and an
// employee with no entity resolves through the org default like everybody
// else.
func (r *repoImpl) EmployeeLegalEntityID(ctx context.Context, orgID, employeeID string) (*string, error) {
	var id *string
	err := r.db.QueryRow(ctx,
		`SELECT legal_entity_id::text FROM hrm_employees WHERE org_id=$1 AND id=$2::uuid`,
		orgID, employeeID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entities: EmployeeLegalEntityID: %w", err)
	}
	return id, nil
}
