// backend/internal/hrm/payslips/model.go
package payslips

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type RunStatus string
const (
	RunDraft     RunStatus = "draft"
	RunComputing RunStatus = "computing"
	RunComputed  RunStatus = "computed"
	RunApproved  RunStatus = "approved"
	RunPaid      RunStatus = "paid"
	RunCancelled RunStatus = "cancelled"
)

type SlipStatus string
const (
	SlipDraft    SlipStatus = "draft"
	SlipComputed SlipStatus = "computed"
	SlipApproved SlipStatus = "approved"
	SlipPaid     SlipStatus = "paid"
)

// PayslipRun is the monthly payroll batch for an organization.
type PayslipRun struct {
	ID                  string     `db:"id"                    json:"id"`
	PublicID            string     `db:"public_id"             json:"public_id"`
	OrgID               string     `db:"org_id"                json:"org_id"`
	PeriodYear          int        `db:"period_year"           json:"period_year"`
	PeriodMonth         int        `db:"period_month"          json:"period_month"`
	Description         *string    `db:"description"           json:"description,omitempty"`
	Currency            string     `db:"currency"              json:"currency"`
	AttendancePeriodID  *string    `db:"attendance_period_id"  json:"attendance_period_id,omitempty"`
	TotalEmployees      int        `db:"total_employees"       json:"total_employees"`
	TotalGrossPay       decimal.Decimal `db:"total_gross_pay"       json:"total_gross_pay"`
	TotalDeductions     decimal.Decimal `db:"total_deductions"      json:"total_deductions"`
	TotalNetPay         decimal.Decimal `db:"total_net_pay"         json:"total_net_pay"`
	Status              RunStatus  `db:"status"                json:"status"`
	ComputedAt          *time.Time `db:"computed_at"           json:"computed_at,omitempty"`
	ComputedBy          *string    `db:"computed_by"           json:"computed_by,omitempty"`
	ApprovedAt          *time.Time `db:"approved_at"           json:"approved_at,omitempty"`
	ApprovedBy          *string    `db:"approved_by"           json:"approved_by,omitempty"`
	PaidAt              *time.Time `db:"paid_at"               json:"paid_at,omitempty"`
	PaidBy              *string    `db:"paid_by"               json:"paid_by,omitempty"`
	CreatedBy           string     `db:"created_by"            json:"created_by"`
	CreatedAt           time.Time  `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"            json:"updated_at"`
}

// Payslip is an individual employee payslip within a run.
type Payslip struct {
	ID                  string     `db:"id"                    json:"id"`
	PublicID            string     `db:"public_id"             json:"public_id"`
	OrgID               string     `db:"org_id"                json:"org_id"`
	EmployeeID          string     `db:"employee_id"           json:"employee_id"`
	PayslipRunID        string     `db:"payslip_run_id"        json:"payslip_run_id"`
	PeriodYear          int        `db:"period_year"           json:"period_year"`
	PeriodMonth         int        `db:"period_month"          json:"period_month"`
	SalaryStructureID   *string    `db:"salary_structure_id"   json:"salary_structure_id,omitempty"`
	SalaryStructureName *string    `db:"salary_structure_name" json:"salary_structure_name,omitempty"`
	GrossPay            decimal.Decimal `db:"gross_pay"             json:"gross_pay"`
	TotalDeductions     decimal.Decimal `db:"total_deductions"      json:"total_deductions"`
	NetPay              decimal.Decimal `db:"net_pay"               json:"net_pay"`
	BasicPay            decimal.Decimal `db:"basic_pay"             json:"basic_pay"`
	WorkDays            int        `db:"work_days"             json:"work_days"`
	PresentDays         int        `db:"present_days"          json:"present_days"`
	AbsentDays          int        `db:"absent_days"           json:"absent_days"`
	LeaveDays           int        `db:"leave_days"            json:"leave_days"`
	HolidayDays         int        `db:"holiday_days"          json:"holiday_days"`
	OvertimeHours       float64    `db:"overtime_hours"        json:"overtime_hours"`
	Currency            string     `db:"currency"              json:"currency"`
	Status              SlipStatus `db:"status"                json:"status"`
	PaymentReference    *string    `db:"payment_reference"     json:"payment_reference,omitempty"`
	PaymentDate         *string    `db:"payment_date"          json:"payment_date,omitempty"`
	PaidAt              *time.Time `db:"paid_at"               json:"paid_at,omitempty"`
	Lines               []*PayslipLine `db:"-"               json:"lines,omitempty"`
	CreatedAt           time.Time  `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"            json:"updated_at"`
}

// PayslipLine is one salary component row within a payslip.
type PayslipLine struct {
	ID             string    `db:"id"              json:"id"`
	PayslipID      string    `db:"payslip_id"      json:"payslip_id"`
	OrgID          string    `db:"org_id"          json:"org_id"`
	ComponentID    *string   `db:"component_id"    json:"component_id,omitempty"`
	ComponentName  string    `db:"component_name"  json:"component_name"`
	ComponentType  string    `db:"component_type"  json:"component_type"`
	CalcMethod     string    `db:"calc_method"     json:"calc_method"`
	FormulaUsed    *string   `db:"formula_used"    json:"formula_used,omitempty"`
	ComputedAmount decimal.Decimal `db:"computed_amount" json:"computed_amount"`
	DisplayOrder   int       `db:"display_order"   json:"display_order"`
	CreatedAt      time.Time `db:"created_at"      json:"created_at"`
}

// CreateRunRequest creates a new payroll run.
type CreateRunRequest struct {
	Year               int     `json:"year"`
	Month              int     `json:"month"`
	Description        *string `json:"description"`
	Currency           *string `json:"currency"`
	AttendancePeriodID *string `json:"attendance_period_id"` // optional D1 link
}

type RunListResponse struct {
	Runs  []*PayslipRun `json:"runs"`
	Total int           `json:"total"`
}

type SlipListResponse struct {
	Payslips []*Payslip `json:"payslips"`
	Total    int        `json:"total"`
}

var (
	ErrNotFound                = errors.New("payslip run not found")
	ErrPayslipNotFound         = errors.New("payslip not found")
	ErrYearRequired            = errors.New("year is required")
	ErrMonthRequired           = errors.New("month is required (1-12)")
	ErrInvalidMonth            = errors.New("month must be between 1 and 12")
	ErrDuplicateRun            = errors.New("payslip run already exists for this period")
	ErrAttendanceNotFinalized  = errors.New("attendance period must be finalized before computing payroll")
	ErrWrongStatus             = errors.New("action not allowed in current payslip run status")
	ErrAlreadyComputed         = errors.New("payslip run has already been computed")
	ErrNotComputed             = errors.New("payslip run must be computed before approving")
	ErrNotApproved             = errors.New("payslip run must be approved before marking as paid")
)
