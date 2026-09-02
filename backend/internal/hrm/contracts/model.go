// backend/internal/hrm/contracts/model.go
package contracts

import (
	"errors"
	"time"
)

type ContractType string

const (
	ContractTypePermanent  ContractType = "permanent"
	ContractTypeFixedTerm  ContractType = "fixed_term"
	ContractTypeProbation  ContractType = "probation"
	ContractTypeInternship ContractType = "internship"
	ContractTypeConsultant ContractType = "consultant"
)

func (t ContractType) IsValid() bool {
	switch t {
	case ContractTypePermanent, ContractTypeFixedTerm, ContractTypeProbation, ContractTypeInternship, ContractTypeConsultant:
		return true
	}
	return false
}

type EmployeeContract struct {
	ID                string       `db:"id"                  json:"id"`
	PublicID          string       `db:"public_id"           json:"public_id"`
	OrgID             string       `db:"org_id"              json:"org_id"`
	EmployeeID        string       `db:"employee_id"         json:"employee_id"`
	ContractType      ContractType `db:"contract_type"       json:"contract_type"`
	StartDate         string       `db:"start_date"          json:"start_date"`
	EndDate           *string      `db:"end_date"            json:"end_date,omitempty"`
	ProbationEndDate  *string      `db:"probation_end_date"  json:"probation_end_date,omitempty"`
	NoticePeriodDays  int          `db:"notice_period_days"  json:"notice_period_days"`
	SalaryStructureID *string      `db:"salary_structure_id" json:"salary_structure_id,omitempty"`
	WorkHoursPerWeek  *float64     `db:"work_hours_per_week" json:"work_hours_per_week,omitempty"`
	DocumentID        *string      `db:"document_id"         json:"document_id,omitempty"`
	IsActive          bool         `db:"is_active"           json:"is_active"`
	Notes             *string      `db:"notes"               json:"notes,omitempty"`
	CreatedBy         string       `db:"created_by"          json:"created_by"`
	CreatedAt         time.Time    `db:"created_at"          json:"created_at"`
	UpdatedAt         time.Time    `db:"updated_at"          json:"updated_at"`
}

type CreateContractRequest struct {
	ContractType      ContractType `json:"contract_type"`
	StartDate         string       `json:"start_date"`
	EndDate           *string      `json:"end_date"`
	ProbationEndDate  *string      `json:"probation_end_date"`
	NoticePeriodDays  *int         `json:"notice_period_days"`
	SalaryStructureID *string      `json:"salary_structure_id"`
	WorkHoursPerWeek  *float64     `json:"work_hours_per_week"`
	DocumentID        *string      `json:"document_id"`
	Notes             *string      `json:"notes"`
}

type UpdateContractRequest struct {
	EndDate           *string  `json:"end_date"`
	ProbationEndDate  *string  `json:"probation_end_date"`
	NoticePeriodDays  *int     `json:"notice_period_days"`
	SalaryStructureID *string  `json:"salary_structure_id"`
	WorkHoursPerWeek  *float64 `json:"work_hours_per_week"`
	DocumentID        *string  `json:"document_id"`
	Notes             *string  `json:"notes"`
}

type ContractListResponse struct {
	Contracts []*EmployeeContract `json:"contracts"`
	Total     int                 `json:"total"`
}

var (
	ErrContractNotFound     = errors.New("contract not found")
	ErrInvalidContractType  = errors.New("contract_type must be: permanent, fixed_term, probation, internship, or consultant")
	ErrStartDateRequired    = errors.New("start_date is required (YYYY-MM-DD)")
	ErrInvalidStartDate     = errors.New("start_date must be a valid YYYY-MM-DD date")
	ErrActiveContractExists = errors.New("employee already has an active contract — deactivate it first")
)
