// backend/internal/hrm/loans/model.go
package loans

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

type LoanType string

const (
	LoanPersonal  LoanType = "personal"
	LoanEmergency LoanType = "emergency"
	LoanAdvance   LoanType = "advance"
	LoanOther     LoanType = "other"
)

func (t LoanType) IsValid() bool {
	switch t {
	case LoanPersonal, LoanEmergency, LoanAdvance, LoanOther:
		return true
	}
	return false
}

type LoanStatus string

const (
	LoanDraft           LoanStatus = "draft"
	LoanPendingApproval LoanStatus = "pending_approval"
	LoanApproved        LoanStatus = "approved"
	LoanActive          LoanStatus = "active"
	LoanForeclosed      LoanStatus = "foreclosed"
	LoanCompleted       LoanStatus = "completed"
	LoanRejected        LoanStatus = "rejected"
	LoanCancelled       LoanStatus = "cancelled"
)

// Loan is an employee loan. installment_amount is the amortization's OUTPUT,
// set only once DisburseLoan generates the schedule — see migration 00100.
type Loan struct {
	ID                 string           `db:"id"                    json:"id"`
	PublicID           string           `db:"public_id"              json:"public_id"`
	OrgID              string           `db:"org_id"                 json:"org_id"`
	EmployeeID         string           `db:"employee_id"            json:"employee_id"`
	LoanType           LoanType         `db:"loan_type"              json:"loan_type"`
	PrincipalAmount    decimal.Decimal  `db:"principal_amount"       json:"principal_amount"`
	InterestRatePct    decimal.Decimal  `db:"interest_rate_pct"      json:"interest_rate_pct"`
	TenureMonths       int              `db:"tenure_months"          json:"tenure_months"`
	InstallmentAmount  *decimal.Decimal `db:"installment_amount"    json:"installment_amount,omitempty"`
	Status             LoanStatus       `db:"status"                 json:"status"`
	ApprovalInstanceID *string          `db:"approval_instance_id"   json:"approval_instance_id,omitempty"`
	DisbursedAt        *time.Time       `db:"disbursed_at"           json:"disbursed_at,omitempty"`
	DisbursedBy        *string          `db:"disbursed_by"           json:"disbursed_by,omitempty"`
	ForeclosedAt       *time.Time       `db:"foreclosed_at"          json:"foreclosed_at,omitempty"`
	ForeclosureAmount  *decimal.Decimal `db:"foreclosure_amount"    json:"foreclosure_amount,omitempty"`
	CreatedBy          string           `db:"created_by"             json:"created_by"`
	CreatedAt          time.Time        `db:"created_at"             json:"created_at"`
	UpdatedAt          time.Time        `db:"updated_at"             json:"updated_at"`
}

type CreateLoanRequest struct {
	EmployeeID      string `json:"employee_id"`
	LoanType        string `json:"loan_type"`
	PrincipalAmount string `json:"principal_amount"`
	InterestRatePct string `json:"interest_rate_pct"`
	TenureMonths    int    `json:"tenure_months"`
}

// ScheduleStatus tracks recovery progress for one installment. total_amount
// never changes after generation — see migration 00100's header.
type ScheduleStatus string

const (
	SchedulePending            ScheduleStatus = "pending"
	SchedulePartiallyRecovered ScheduleStatus = "partially_recovered"
	ScheduleRecovered          ScheduleStatus = "recovered"
	ScheduleForeclosed         ScheduleStatus = "foreclosed"
)

type ScheduleRow struct {
	ID                 string          `db:"id"                   json:"id"`
	PublicID           string          `db:"public_id"             json:"public_id"`
	LoanID             string          `db:"loan_id"               json:"loan_id"`
	InstallmentNumber  int             `db:"installment_number"    json:"installment_number"`
	DuePeriodYear      int             `db:"due_period_year"       json:"due_period_year"`
	DuePeriodMonth     int             `db:"due_period_month"      json:"due_period_month"`
	PrincipalComponent decimal.Decimal `db:"principal_component"   json:"principal_component"`
	InterestComponent  decimal.Decimal `db:"interest_component"    json:"interest_component"`
	TotalAmount        decimal.Decimal `db:"total_amount"          json:"total_amount"`
	RecoveredAmount    decimal.Decimal `db:"recovered_amount"      json:"recovered_amount"`
	Status             ScheduleStatus  `db:"status"                json:"status"`
	CreatedAt          time.Time       `db:"created_at"            json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at"            json:"updated_at"`
}

// RemainingOwed is computed, never stored — the 00076 rule.
func (r *ScheduleRow) RemainingOwed() decimal.Decimal {
	return r.TotalAmount.Sub(r.RecoveredAmount)
}

type RecoveryEvent struct {
	ID              string          `db:"id"                json:"id"`
	LoanID          string          `db:"loan_id"            json:"loan_id"`
	ScheduleID      string          `db:"schedule_id"        json:"schedule_id"`
	PayslipRunID    *string         `db:"payslip_run_id"     json:"payslip_run_id,omitempty"`
	PayslipLineID   *string         `db:"payslip_line_id"    json:"payslip_line_id,omitempty"`
	AmountRecovered decimal.Decimal `db:"amount_recovered"   json:"amount_recovered"`
	RecoveredAt     time.Time       `db:"recovered_at"       json:"recovered_at"`
}

type ForecloseLoanRequest struct {
	ForeclosureAmount string `json:"foreclosure_amount"`
}

type ListFilter struct {
	EmployeeID   string
	Limit        int
	Offset       int
	Scope        authz.Scope
	CallerUserID string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *ListFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

type LoanListResponse struct {
	Loans  []*Loan `json:"loans"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

var (
	ErrNotFound         = errors.New("loans: not found")
	ErrLoanNotFound     = errors.New("loan not found")
	ErrScheduleNotFound = errors.New("loan schedule not found")
	ErrInvalidLoanType  = errors.New("loan_type is not a recognised value")
	ErrInvalidAmount    = errors.New("amount must be a valid positive number")
	ErrInvalidTenure    = errors.New("tenure_months must be a positive integer")
	ErrWrongLoanStatus  = errors.New("action not allowed in the loan's current status")
	ErrAccessDenied     = errors.New("access denied")
)
