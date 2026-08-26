// backend/internal/hrm/reimbursements/model.go
package reimbursements

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

type Category string

const (
	CategoryTravel    Category = "travel"
	CategoryMedical   Category = "medical"
	CategoryEquipment Category = "equipment"
	CategoryOther     Category = "other"
)

func (c Category) IsValid() bool {
	switch c {
	case CategoryTravel, CategoryMedical, CategoryEquipment, CategoryOther:
		return true
	}
	return false
}

type Status string

const (
	StatusDraft           Status = "draft"
	StatusPendingApproval Status = "pending_approval"
	StatusApproved        Status = "approved"
	StatusRejected        Status = "rejected"
	StatusPaid            Status = "paid"
	StatusCancelled       Status = "cancelled"
)

// Reimbursement is payout only — no claims/receipts workflow, no
// calculation_snapshot. See migration 00100's header: there is no formula
// here to audit, just a flat HR-entered amount.
type Reimbursement struct {
	ID                 string          `db:"id"                   json:"id"`
	PublicID           string          `db:"public_id"             json:"public_id"`
	OrgID              string          `db:"org_id"                json:"org_id"`
	EmployeeID         string          `db:"employee_id"           json:"employee_id"`
	Category           Category        `db:"category"              json:"category"`
	Description        *string         `db:"description"           json:"description,omitempty"`
	Amount             decimal.Decimal `db:"amount"                json:"amount"`
	Currency           string          `db:"currency"              json:"currency"`
	Status             Status          `db:"status"                json:"status"`
	ApprovalInstanceID *string         `db:"approval_instance_id"  json:"approval_instance_id,omitempty"`
	PayslipRunID       *string         `db:"payslip_run_id"        json:"payslip_run_id,omitempty"`
	PayslipLineID      *string         `db:"payslip_line_id"       json:"payslip_line_id,omitempty"`
	PaidAt             *time.Time      `db:"paid_at"               json:"paid_at,omitempty"`
	CreatedBy          string          `db:"created_by"            json:"created_by"`
	CreatedAt          time.Time       `db:"created_at"            json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at"            json:"updated_at"`
}

type CreateRequest struct {
	EmployeeID  string  `json:"employee_id"`
	Category    string  `json:"category"`
	Description *string `json:"description"`
	Amount      string  `json:"amount"`
	Currency    *string `json:"currency"`
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

type ListResponse struct {
	Reimbursements []*Reimbursement `json:"reimbursements"`
	Total          int              `json:"total"`
	Limit          int              `json:"limit"`
	Offset         int              `json:"offset"`
}

var (
	ErrNotFound        = errors.New("reimbursement not found")
	ErrInvalidCategory = errors.New("category is not a recognised value")
	ErrInvalidAmount   = errors.New("amount must be a valid positive number")
	ErrWrongStatus     = errors.New("action not allowed in the reimbursement's current status")
	ErrAccessDenied    = errors.New("access denied")
)
