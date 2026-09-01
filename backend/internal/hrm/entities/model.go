// backend/internal/hrm/entities/model.go
package entities

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrAccessDenied      = errors.New("entities: access denied")
	ErrEntityNotFound    = errors.New("entities: legal entity not found")
	ErrConfigNotFound    = errors.New("entities: country configuration not found")
	ErrLocationNotFound  = errors.New("entities: location not found")
	ErrNameRequired      = errors.New("entities: a name is required")
	ErrInvalidCountry    = errors.New("entities: country_code must be a two-letter ISO 3166-1 code")
	ErrInvalidCurrency   = errors.New("entities: currency must be a three-letter ISO 4217 code")
	ErrInvalidCycle      = errors.New("entities: payroll_cycle must be monthly, semi_monthly, biweekly or weekly")
	ErrInvalidPayDay     = errors.New("entities: pay_day_of_month must be between 1 and 31")
	ErrInvalidMonth      = errors.New("entities: fiscal_year_start_month must be between 1 and 12")
	ErrDuplicateConfig   = errors.New("entities: this country already has a configuration")
	ErrDuplicateCode     = errors.New("entities: another active location already uses this code")
	ErrDefaultConflict   = errors.New("entities: another entity is already the default")
	ErrHeadquartersTaken = errors.New("entities: another active location is already the headquarters")
	ErrCannotUndefault   = errors.New("entities: an organization's default entity is cleared by making another one default, not by unsetting this one")
)

// LegalEntity is a company within an organization.
//
// ⚠ CountryCode, BaseCurrency and Timezone are all POINTERS. Entities created
// before 11A have none, and there is no honest value to write for them:
// guessing a country for somebody's subsidiary is worse than resolving the
// organization's at read time, because a guess is indistinguishable from a
// fact once stored.
type LegalEntity struct {
	ID                 string    `json:"id"`
	PublicID           string    `json:"public_id"`
	OrgID              string    `json:"org_id"`
	Name               string    `json:"name"`
	IsDefault          bool      `json:"is_default"`
	CountryCode        *string   `json:"country_code,omitempty"`
	BaseCurrency       *string   `json:"base_currency,omitempty"`
	RegistrationNumber *string   `json:"registration_number,omitempty"`
	TaxIdentifier      *string   `json:"tax_identifier,omitempty"`
	RegisteredAddress  *string   `json:"registered_address,omitempty"`
	Timezone           *string   `json:"timezone,omitempty"`
	IsActive           bool      `json:"is_active"`
	CreatedBy          *string   `json:"created_by,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateEntityRequest struct {
	Name               string  `json:"name"`
	CountryCode        *string `json:"country_code"`
	BaseCurrency       *string `json:"base_currency"`
	RegistrationNumber *string `json:"registration_number"`
	TaxIdentifier      *string `json:"tax_identifier"`
	RegisteredAddress  *string `json:"registered_address"`
	Timezone           *string `json:"timezone"`
	IsDefault          *bool   `json:"is_default"`
}

type UpdateEntityRequest struct {
	Name               *string `json:"name"`
	CountryCode        *string `json:"country_code"`
	BaseCurrency       *string `json:"base_currency"`
	RegistrationNumber *string `json:"registration_number"`
	TaxIdentifier      *string `json:"tax_identifier"`
	RegisteredAddress  *string `json:"registered_address"`
	Timezone           *string `json:"timezone"`
	IsDefault          *bool   `json:"is_default"`
	IsActive           *bool   `json:"is_active"`
}

// CountryConfig holds per-country defaults an entity in that country
// inherits.
//
// ⚠ Every value is a DEFAULT, not a rule. Nothing in payroll reads this and
// refuses to run; it supplies a value where a caller has not given one. A
// config that hard-required a statutory notice period would break the first
// organization whose contracts are more generous, which is most of them.
type CountryConfig struct {
	ID                      string           `json:"id"`
	PublicID                string           `json:"public_id"`
	OrgID                   string           `json:"org_id"`
	CountryCode             string           `json:"country_code"`
	CountryName             *string          `json:"country_name,omitempty"`
	DefaultCurrency         *string          `json:"default_currency,omitempty"`
	PayrollCycle            *string          `json:"payroll_cycle,omitempty"`
	PayDayOfMonth           *int             `json:"pay_day_of_month,omitempty"`
	FiscalYearStartMonth    *int             `json:"fiscal_year_start_month,omitempty"`
	StandardWorkDaysPerWeek *decimal.Decimal `json:"standard_work_days_per_week,omitempty"`
	StandardHoursPerDay     *decimal.Decimal `json:"standard_hours_per_day,omitempty"`
	OvertimeMultiplier      *decimal.Decimal `json:"overtime_multiplier,omitempty"`
	AnnualLeaveDays         *int             `json:"annual_leave_days,omitempty"`
	NoticePeriodDays        *int             `json:"notice_period_days,omitempty"`
	ProbationDays           *int             `json:"probation_days,omitempty"`
	GratuityEligibleYears   *decimal.Decimal `json:"gratuity_eligible_years,omitempty"`
	GratuityDaysPerYear     *decimal.Decimal `json:"gratuity_days_per_year,omitempty"`
	IsActive                bool             `json:"is_active"`
	CreatedBy               string           `json:"created_by"`
	CreatedAt               time.Time        `json:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"`
}

type CountryConfigRequest struct {
	CountryCode             string           `json:"country_code"`
	CountryName             *string          `json:"country_name"`
	DefaultCurrency         *string          `json:"default_currency"`
	PayrollCycle            *string          `json:"payroll_cycle"`
	PayDayOfMonth           *int             `json:"pay_day_of_month"`
	FiscalYearStartMonth    *int             `json:"fiscal_year_start_month"`
	StandardWorkDaysPerWeek *decimal.Decimal `json:"standard_work_days_per_week"`
	StandardHoursPerDay     *decimal.Decimal `json:"standard_hours_per_day"`
	OvertimeMultiplier      *decimal.Decimal `json:"overtime_multiplier"`
	AnnualLeaveDays         *int             `json:"annual_leave_days"`
	NoticePeriodDays        *int             `json:"notice_period_days"`
	ProbationDays           *int             `json:"probation_days"`
	GratuityEligibleYears   *decimal.Decimal `json:"gratuity_eligible_years"`
	GratuityDaysPerYear     *decimal.Decimal `json:"gratuity_days_per_year"`
	IsActive                *bool            `json:"is_active"`
}

// Location is a work site.
type Location struct {
	ID              string    `json:"id"`
	PublicID        string    `json:"public_id"`
	OrgID           string    `json:"org_id"`
	LegalEntityID   *string   `json:"legal_entity_id,omitempty"`
	LegalEntityName string    `json:"legal_entity_name,omitempty"`
	Name            string    `json:"name"`
	Code            *string   `json:"code,omitempty"`
	AddressLine1    *string   `json:"address_line1,omitempty"`
	AddressLine2    *string   `json:"address_line2,omitempty"`
	City            *string   `json:"city,omitempty"`
	State           *string   `json:"state,omitempty"`
	PostalCode      *string   `json:"postal_code,omitempty"`
	CountryCode     *string   `json:"country_code,omitempty"`
	Timezone        *string   `json:"timezone,omitempty"`
	IsHeadquarters  bool      `json:"is_headquarters"`
	IsActive        bool      `json:"is_active"`
	EmployeeCount   int       `json:"employee_count"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateLocationRequest struct {
	LegalEntityID  *string `json:"legal_entity_id"`
	Name           string  `json:"name"`
	Code           *string `json:"code"`
	AddressLine1   *string `json:"address_line1"`
	AddressLine2   *string `json:"address_line2"`
	City           *string `json:"city"`
	State          *string `json:"state"`
	PostalCode     *string `json:"postal_code"`
	CountryCode    *string `json:"country_code"`
	Timezone       *string `json:"timezone"`
	IsHeadquarters *bool   `json:"is_headquarters"`
}

type UpdateLocationRequest struct {
	LegalEntityID  *string `json:"legal_entity_id"`
	Name           *string `json:"name"`
	Code           *string `json:"code"`
	AddressLine1   *string `json:"address_line1"`
	AddressLine2   *string `json:"address_line2"`
	City           *string `json:"city"`
	State          *string `json:"state"`
	PostalCode     *string `json:"postal_code"`
	CountryCode    *string `json:"country_code"`
	Timezone       *string `json:"timezone"`
	IsHeadquarters *bool   `json:"is_headquarters"`
	IsActive       *bool   `json:"is_active"`
}

// EntityContext is the resolved answer to "what country, currency and
// working calendar apply here" — the value every 11B consumer will ask for.
//
// ⚠ Each field carries its SOURCE. A caller that needs to know whether the
// currency is the entity's own or the organization's fallback can see it, and
// the day somebody adds a second entity the difference stops being academic.
type EntityContext struct {
	OrgID           string   `json:"org_id"`
	LegalEntityID   *string  `json:"legal_entity_id,omitempty"`
	LegalEntityName string   `json:"legal_entity_name,omitempty"`
	CountryCode     Resolved `json:"country_code"`
	Currency        Resolved `json:"currency"`
	Timezone        Resolved `json:"timezone"`

	// Config is the country configuration for the RESOLVED country, or nil.
	// Nil is normal and never an error: most organizations will never record
	// one, and every consumer must work without it.
	Config *CountryConfig `json:"country_config,omitempty"`

	// SingleEntity is true when the organization has at most one legal
	// entity — the state every organization in this database is in, and the
	// regression guard for the whole of Phase 11.
	SingleEntity bool `json:"single_entity"`
	EntityCount  int  `json:"entity_count"`
}
