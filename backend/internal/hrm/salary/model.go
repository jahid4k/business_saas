// backend/internal/hrm/salary/model.go
package salary

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// ─────────────────────────────────────────────────────────────
// Enums
// ─────────────────────────────────────────────────────────────

type ComponentType string

const (
	ComponentTypeEarning              ComponentType = "earning"
	ComponentTypeDeduction            ComponentType = "deduction"
	ComponentTypeEmployerContribution ComponentType = "employer_contribution"
)

func (t ComponentType) IsValid() bool {
	switch t {
	case ComponentTypeEarning, ComponentTypeDeduction, ComponentTypeEmployerContribution:
		return true
	}
	return false
}

type CalcMethod string

const (
	CalcMethodFixed       CalcMethod = "fixed"
	CalcMethodPctOfBasic  CalcMethod = "pct_of_basic"
	CalcMethodPctOfGross  CalcMethod = "pct_of_gross"
	CalcMethodFormula     CalcMethod = "formula"
	CalcMethodManual      CalcMethod = "manual"
	CalcMethodSlab        CalcMethod = "slab"
)

func (m CalcMethod) IsValid() bool {
	switch m {
	case CalcMethodFixed, CalcMethodPctOfBasic, CalcMethodPctOfGross,
		CalcMethodFormula, CalcMethodManual, CalcMethodSlab:
		return true
	}
	return false
}

type ChangeReason string

const (
	ChangeReasonJoining         ChangeReason = "joining"
	ChangeReasonPromotion       ChangeReason = "promotion"
	ChangeReasonAnnualRevision  ChangeReason = "annual_revision"
	ChangeReasonTransfer        ChangeReason = "transfer"
	ChangeReasonCorrection      ChangeReason = "correction"
	ChangeReasonOther           ChangeReason = "other"
)

func (r ChangeReason) IsValid() bool {
	switch r {
	case ChangeReasonJoining, ChangeReasonPromotion, ChangeReasonAnnualRevision,
		ChangeReasonTransfer, ChangeReasonCorrection, ChangeReasonOther:
		return true
	}
	return false
}

// ─────────────────────────────────────────────────────────────
// Slab configuration
// ─────────────────────────────────────────────────────────────

// SlabConfig is stored as JSONB in hrm_salary_components.slab_config.
// It defines a progressive bracket calculation (e.g. income tax).
//
// Example:
//
//	{"base_variable":"GROSS","slabs":[{"up_to":30000,"rate":0.05},{"up_to":null,"rate":0.10}]}
type SlabConfig struct {
	BaseVariable string  `json:"base_variable"` // env variable name: "GROSS", "BASIC", etc.
	Slabs        []Slab  `json:"slabs"`
}

type Slab struct {
	UpTo *decimal.Decimal `json:"up_to"` // null = no upper limit (last slab)
	Rate decimal.Decimal  `json:"rate"`  // fractional: 0.05 = 5%
}

// ─────────────────────────────────────────────────────────────
// SalaryComponent
// ─────────────────────────────────────────────────────────────

type SalaryComponent struct {
	ID                 string         `db:"id"                 json:"id"`
	PublicID           string         `db:"public_id"          json:"public_id"`
	OrgID              string         `db:"org_id"             json:"org_id"`
	Name               string         `db:"name"               json:"name"`
	Description        *string        `db:"description"        json:"description,omitempty"`
	ComponentType      ComponentType  `db:"component_type"     json:"component_type"`
	CalcMethod         CalcMethod     `db:"calc_method"        json:"calc_method"`
	FixedValue         decimal.Decimal        `db:"fixed_value"        json:"fixed_value"`
	FormulaExpression  *string        `db:"formula_expression" json:"formula_expression,omitempty"`
	FormulaVariables   []string       `db:"formula_variables"  json:"formula_variables,omitempty"`
	SlabConfig         *SlabConfig    `db:"-"                  json:"slab_config,omitempty"` // scanned from JSONB
	IsTaxable          bool           `db:"is_taxable"         json:"is_taxable"`
	DisplayOrder       int            `db:"display_order"      json:"display_order"`
	IsActive           bool           `db:"is_active"          json:"is_active"`
	CreatedBy          string         `db:"created_by"         json:"created_by"`
	CreatedAt          time.Time      `db:"created_at"         json:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"         json:"updated_at"`
}

// SlabConfigJSON returns SlabConfig as JSON for DB storage.
func (sc *SalaryComponent) SlabConfigJSON() ([]byte, error) {
	if sc.SlabConfig == nil {
		return nil, nil
	}
	return json.Marshal(sc.SlabConfig)
}

// ─────────────────────────────────────────────────────────────
// SalaryStructure
// ─────────────────────────────────────────────────────────────

type SalaryStructure struct {
	ID          string     `db:"id"          json:"id"`
	PublicID    string     `db:"public_id"   json:"public_id"`
	OrgID       string     `db:"org_id"      json:"org_id"`
	Name        string     `db:"name"        json:"name"`
	Description *string    `db:"description" json:"description,omitempty"`
	GradeLabel  *string    `db:"grade_label" json:"grade_label,omitempty"`
	IsActive    bool       `db:"is_active"   json:"is_active"`
	CreatedBy   string     `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time  `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"  json:"updated_at"`

	// Populated by join queries — not columns
	Components []*StructureComponent `db:"-" json:"components,omitempty"`
}

// StructureComponent is a component within a structure (join result).
type StructureComponent struct {
	ComponentID   string         `json:"component_id"`
	Component     *SalaryComponent `json:"component,omitempty"`
	OverrideValue *decimal.Decimal       `json:"override_value,omitempty"`
	DisplayOrder  int            `json:"display_order"`
}

// ─────────────────────────────────────────────────────────────
// EmployeeSalaryRecord (append-only)
// ─────────────────────────────────────────────────────────────

type EmployeeSalaryRecord struct {
	ID            string       `db:"id"             json:"id"`
	PublicID      string       `db:"public_id"      json:"public_id"`
	OrgID         string       `db:"org_id"         json:"org_id"`
	EmployeeID    string       `db:"employee_id"    json:"employee_id"`
	StructureID   *string      `db:"structure_id"   json:"structure_id,omitempty"`
	BasicPay      decimal.Decimal      `db:"basic_pay"      json:"basic_pay"`
	EffectiveDate string       `db:"effective_date" json:"effective_date"` // YYYY-MM-DD
	ChangeReason  ChangeReason `db:"change_reason"  json:"change_reason"`
	ChangeNotes   *string      `db:"change_notes"   json:"change_notes,omitempty"`
	CreatedBy     string       `db:"created_by"     json:"created_by"`
	CreatedAt     time.Time    `db:"created_at"     json:"created_at"`

	// Populated by join — not a column
	Structure *SalaryStructure `db:"-" json:"structure,omitempty"`
}

// ─────────────────────────────────────────────────────────────
// Request types
// ─────────────────────────────────────────────────────────────

type CreateComponentRequest struct {
	Name               string         `json:"name"`
	Description        *string        `json:"description"`
	ComponentType      ComponentType  `json:"component_type"`
	CalcMethod         CalcMethod     `json:"calc_method"`
	FixedValue         *decimal.Decimal       `json:"fixed_value"`
	FormulaExpression  *string        `json:"formula_expression"`
	FormulaVariables   []string       `json:"formula_variables"`
	SlabConfig         *SlabConfig    `json:"slab_config"`
	IsTaxable          bool           `json:"is_taxable"`
	DisplayOrder       *int           `json:"display_order"`
}

type UpdateComponentRequest struct {
	Name               *string        `json:"name"`
	Description        *string        `json:"description"`
	ComponentType      *ComponentType `json:"component_type"`
	CalcMethod         *CalcMethod    `json:"calc_method"`
	FixedValue         *decimal.Decimal       `json:"fixed_value"`
	FormulaExpression  *string        `json:"formula_expression"`
	FormulaVariables   []string       `json:"formula_variables"`
	SlabConfig         *SlabConfig    `json:"slab_config"`
	IsTaxable          *bool          `json:"is_taxable"`
	DisplayOrder       *int           `json:"display_order"`
	IsActive           *bool          `json:"is_active"`
}

type CreateStructureRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	GradeLabel  *string `json:"grade_label"`
}

type UpdateStructureRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	GradeLabel  *string `json:"grade_label"`
	IsActive    *bool   `json:"is_active"`
}

type AddComponentToStructureRequest struct {
	ComponentID   string   `json:"component_id"`
	OverrideValue *decimal.Decimal `json:"override_value"`
	DisplayOrder  *int     `json:"display_order"`
}

type AssignSalaryRequest struct {
	StructureID   *string      `json:"structure_id"`
	BasicPay      decimal.Decimal      `json:"basic_pay"`
	EffectiveDate string       `json:"effective_date"` // YYYY-MM-DD
	ChangeReason  ChangeReason `json:"change_reason"`
	ChangeNotes   *string      `json:"change_notes"`
}

type TestFormulaRequest struct {
	Expression string             `json:"expression"`
	Variables  map[string]float64 `json:"variables"` // test values for each env var
}

type TestFormulaResponse struct {
	Result float64 `json:"result"`
	Valid  bool    `json:"valid"`
	Error  *string `json:"error,omitempty"`
}

// ─────────────────────────────────────────────────────────────
// List responses
// ─────────────────────────────────────────────────────────────

type ComponentListResponse struct {
	Components []*SalaryComponent `json:"components"`
	Total      int                `json:"total"`
}

type StructureListResponse struct {
	Structures []*SalaryStructure `json:"structures"`
	Total      int                `json:"total"`
}

type SalaryHistoryResponse struct {
	Records []*EmployeeSalaryRecord `json:"records"`
	Total   int                     `json:"total"`
}

// ─────────────────────────────────────────────────────────────
// Sentinel errors
// ─────────────────────────────────────────────────────────────

var (
	ErrComponentNotFound    = errors.New("salary component not found")
	ErrStructureNotFound    = errors.New("salary structure not found")
	ErrSalaryRecordNotFound = errors.New("salary record not found")

	ErrNameRequired         = errors.New("name is required")
	ErrNameTooLong          = errors.New("name must not exceed 150 characters")
	ErrNameConflict         = errors.New("a component with this name already exists")
	ErrStructureNameConflict = errors.New("a structure with this name already exists")

	ErrInvalidComponentType = errors.New("component_type must be: earning, deduction, or employer_contribution")
	ErrInvalidCalcMethod    = errors.New("calc_method must be: fixed, pct_of_basic, pct_of_gross, formula, manual, or slab")
	ErrFormulaRequired      = errors.New("formula_expression is required when calc_method is formula")
	ErrSlabRequired         = errors.New("slab_config is required when calc_method is slab")
	ErrInvalidFormula       = errors.New("formula_expression is invalid")
	ErrInvalidSlab          = errors.New("slab_config is invalid: slabs must be in ascending order with exactly one null up_to at the end")

	ErrBasicPayRequired      = errors.New("basic_pay is required and must be >= 0")
	ErrEffectiveDateRequired = errors.New("effective_date is required (YYYY-MM-DD)")
	ErrInvalidEffectiveDate  = errors.New("effective_date must be a valid date in YYYY-MM-DD format")
	ErrInvalidChangeReason   = errors.New("change_reason must be: joining, promotion, annual_revision, transfer, correction, or other")

	ErrComponentInStructure = errors.New("component is already in this structure")
	ErrComponentNotInStructure = errors.New("component is not in this structure")
)
