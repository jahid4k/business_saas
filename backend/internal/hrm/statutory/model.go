// backend/internal/hrm/statutory/model.go
package statutory

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type RuleType string

const (
	RuleIncomeTax      RuleType = "income_tax"
	RuleSocialSecurity RuleType = "social_security"
	RuleProvidentFund  RuleType = "provident_fund"
	RuleOther          RuleType = "other"
)

func (t RuleType) IsValid() bool {
	switch t {
	case RuleIncomeTax, RuleSocialSecurity, RuleProvidentFund, RuleOther:
		return true
	}
	return false
}

// BaseVariable names which of computePayslips' already-resolved bases a rule
// is evaluated against. Reuses hrm_salary_components.slab_config's own
// vocabulary (GROSS, BASIC) plus TAXABLE_GROSS — see migration 00102.
type BaseVariable string

const (
	BaseGross        BaseVariable = "GROSS"
	BaseBasic        BaseVariable = "BASIC"
	BaseTaxableGross BaseVariable = "TAXABLE_GROSS"
)

func (b BaseVariable) IsValid() bool {
	switch b {
	case BaseGross, BaseBasic, BaseTaxableGross:
		return true
	}
	return false
}

// Rule is the stable identity of one statutory deduction or contribution.
// hrm_statutory_slabs (Slab) holds the effective-dated bracket data it
// evaluates — see migration 00102's header.
type Rule struct {
	ID                     string       `db:"id"                        json:"id"`
	PublicID               string       `db:"public_id"                 json:"public_id"`
	OrgID                  string       `db:"org_id"                    json:"org_id"`
	Name                   string       `db:"name"                      json:"name"`
	CountryCode            string       `db:"country_code"              json:"country_code"`
	RuleType               RuleType     `db:"rule_type"                 json:"rule_type"`
	BaseVariable           BaseVariable `db:"base_variable"             json:"base_variable"`
	IsEmployerContribution bool         `db:"is_employer_contribution"  json:"is_employer_contribution"`
	IsActive               bool         `db:"is_active"                 json:"is_active"`
	CreatedBy              string       `db:"created_by"                json:"created_by"`
	CreatedAt              time.Time    `db:"created_at"                json:"created_at"`
	UpdatedAt              time.Time    `db:"updated_at"                json:"updated_at"`
}

type CreateRuleRequest struct {
	Name                   string `json:"name"`
	CountryCode            string `json:"country_code"`
	RuleType               string `json:"rule_type"`
	BaseVariable           string `json:"base_variable"`
	IsEmployerContribution bool   `json:"is_employer_contribution"`
}

// Slab is one effective-dated bracket for a rule.
type Slab struct {
	ID            string           `db:"id"              json:"id"`
	PublicID      string           `db:"public_id"        json:"public_id"`
	RuleID        string           `db:"rule_id"          json:"rule_id"`
	UpTo          *decimal.Decimal `db:"up_to"            json:"up_to,omitempty"`
	RatePct       decimal.Decimal  `db:"rate_pct"         json:"rate_pct"`
	EffectiveDate time.Time        `db:"effective_date"   json:"effective_date"`
	CreatedBy     string           `db:"created_by"       json:"created_by"`
	CreatedAt     time.Time        `db:"created_at"       json:"created_at"`
}

type CreateSlabRequest struct {
	UpTo          *string `json:"up_to"`
	RatePct       string  `json:"rate_pct"`
	EffectiveDate string  `json:"effective_date"`
}

var (
	ErrRuleNotFound    = errors.New("statutory rule not found")
	ErrInvalidRuleType = errors.New("rule_type is not a recognised value")
	ErrInvalidBase     = errors.New("base_variable is not a recognised value")
	ErrInvalidAmount   = errors.New("value must be a valid non-negative number")
)
