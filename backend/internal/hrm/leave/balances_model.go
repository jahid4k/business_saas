// backend/internal/hrm/leave/balances_model.go
package leave

import (
	"errors"
	"time"
)

// AccrualMethod defines how a leave policy grants days to employees.
type AccrualMethod string

const (
	AccrualMonthly   AccrualMethod = "monthly"
	AccrualAnnual    AccrualMethod = "annual"
	AccrualOnJoining AccrualMethod = "on_joining"
)

func (m AccrualMethod) IsValid() bool {
	switch m {
	case AccrualMonthly, AccrualAnnual, AccrualOnJoining:
		return true
	}
	return false
}

// EncashmentRateBasis records how an encashed day's currency value should be
// computed. Phase 2 stores this only — it is never evaluated here. A future
// F&F (Full & Final settlement) phase reads it to price an encashment.
type EncashmentRateBasis string

const (
	EncashmentBasisBasicPay EncashmentRateBasis = "basic_pay"
	EncashmentBasisGrossPay EncashmentRateBasis = "gross_pay"
	EncashmentBasisFixed    EncashmentRateBasis = "fixed"
)

func (b EncashmentRateBasis) IsValid() bool {
	switch b {
	case EncashmentBasisBasicPay, EncashmentBasisGrossPay, EncashmentBasisFixed:
		return true
	}
	return false
}

// TransactionType classifies a hrm_leave_transactions ledger row. days is
// signed: credits (accrual, usage_reversal, carry_forward, and a positive
// adjustment) are positive; debits (usage, encashment, forfeiture, and a
// negative adjustment) are negative.
type TransactionType string

const (
	TransactionAccrual       TransactionType = "accrual"
	TransactionUsage         TransactionType = "usage"
	TransactionUsageReversal TransactionType = "usage_reversal"
	TransactionEncashment    TransactionType = "encashment"
	TransactionCarryForward  TransactionType = "carry_forward"
	TransactionForfeiture    TransactionType = "forfeiture"
	TransactionAdjustment    TransactionType = "adjustment"
)

func (t TransactionType) IsValid() bool {
	switch t {
	case TransactionAccrual, TransactionUsage, TransactionUsageReversal,
		TransactionEncashment, TransactionCarryForward, TransactionForfeiture, TransactionAdjustment:
		return true
	}
	return false
}

// ─────────────────────────────────────────────────────────
// Leave Policies
// ─────────────────────────────────────────────────────────

// LeavePolicy is the accrual/carry-forward/encashment configuration for one
// (org, leave_type) pair. Mirrors hrm_leave_policies columns exactly.
// Balance tracking for a leave type only activates once a policy exists —
// leave types with no policy row behave exactly as they did before Phase 2.
type LeavePolicy struct {
	ID                  string               `db:"id"                     json:"id"`
	PublicID            string               `db:"public_id"              json:"public_id"`
	OrgID               string               `db:"org_id"                 json:"org_id"`
	LeaveTypeID         string               `db:"leave_type_id"          json:"leave_type_id"`
	AccrualMethod       AccrualMethod        `db:"accrual_method"         json:"accrual_method"`
	AccrualRate         float64              `db:"accrual_rate"           json:"accrual_rate"`
	CarryForwardEnabled bool                 `db:"carry_forward_enabled"  json:"carry_forward_enabled"`
	CarryForwardCap     *float64             `db:"carry_forward_cap"      json:"carry_forward_cap,omitempty"` // nil while enabled = uncapped
	Encashable          bool                 `db:"encashable"             json:"encashable"`
	EncashmentRateBasis *EncashmentRateBasis `db:"encashment_rate_basis"  json:"encashment_rate_basis,omitempty"`
	IsActive            bool                 `db:"is_active"              json:"is_active"`
	CreatedBy           string               `db:"created_by"             json:"created_by"`
	CreatedAt           time.Time            `db:"created_at"             json:"created_at"`
	UpdatedAt           time.Time            `db:"updated_at"             json:"updated_at"`
}

// CreatePolicyRequest is the body for POST /hrm/leave/policies. Creating a
// policy synchronously backfills historical usage from any pre-existing
// approved hrm_leave_requests for this leave type (see backfillHistoricalUsage).
type CreatePolicyRequest struct {
	LeaveTypeID         string               `json:"leave_type_id"`
	AccrualMethod       AccrualMethod        `json:"accrual_method"`
	AccrualRate         float64              `json:"accrual_rate"`
	CarryForwardEnabled bool                 `json:"carry_forward_enabled"`
	CarryForwardCap     *float64             `json:"carry_forward_cap"`
	Encashable          bool                 `json:"encashable"`
	EncashmentRateBasis *EncashmentRateBasis `json:"encashment_rate_basis"`
}

// UpdatePolicyRequest is the body for PATCH /hrm/leave/policies/:policyId.
// Never re-triggers backfill — that only happens at creation time.
type UpdatePolicyRequest struct {
	AccrualMethod       *AccrualMethod       `json:"accrual_method"`
	AccrualRate         *float64             `json:"accrual_rate"`
	CarryForwardEnabled *bool                `json:"carry_forward_enabled"`
	CarryForwardCap     *float64             `json:"carry_forward_cap"`
	Encashable          *bool                `json:"encashable"`
	EncashmentRateBasis *EncashmentRateBasis `json:"encashment_rate_basis"`
	IsActive            *bool                `json:"is_active"`
}

// PolicyListResponse wraps the policy list.
type PolicyListResponse struct {
	Policies []*LeavePolicy `json:"policies"`
	Total    int            `json:"total"`
}

// ─────────────────────────────────────────────────────────
// Leave Balances (immutable monthly snapshots)
// ─────────────────────────────────────────────────────────

// LeaveBalance is one immutable monthly snapshot row. Mirrors
// hrm_leave_balances columns exactly. Never updated after insert — a
// correction lands as a new ledger transaction, reflected in a later
// snapshot, not a mutation of this row.
type LeaveBalance struct {
	ID             string    `db:"id"                json:"id"`
	PublicID       string    `db:"public_id"         json:"public_id"`
	OrgID          string    `db:"org_id"            json:"org_id"`
	EmployeeID     string    `db:"employee_id"       json:"employee_id"`
	LeaveTypeID    string    `db:"leave_type_id"     json:"leave_type_id"`
	PolicyID       string    `db:"policy_id"         json:"policy_id"`
	PeriodYear     int       `db:"period_year"       json:"period_year"`
	PeriodMonth    int       `db:"period_month"      json:"period_month"`
	AsOfDate       string    `db:"as_of_date"        json:"as_of_date"` // YYYY-MM-DD, always the 1st of PeriodMonth
	OpeningBalance float64   `db:"opening_balance"   json:"opening_balance"`
	Accrued        float64   `db:"accrued"           json:"accrued"`
	Taken          float64   `db:"taken"             json:"taken"`
	Encashed       float64   `db:"encashed"          json:"encashed"`
	CarriedForward float64   `db:"carried_forward"   json:"carried_forward"`
	Adjusted       float64   `db:"adjusted"          json:"adjusted"`
	ClosingBalance float64   `db:"closing_balance"   json:"closing_balance"`
	CreatedBy      string    `db:"created_by"        json:"created_by"`
	CreatedAt      time.Time `db:"created_at"        json:"created_at"`
}

// BalanceHistoryResponse wraps a paginated snapshot history for one employee/leave type.
type BalanceHistoryResponse struct {
	Balances []*LeaveBalance `json:"balances"`
	Total    int             `json:"total"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
}

// CurrentBalance is a computed read model, not a table — the answer to
// "what does this employee have available right now" for one leave type.
// Balance = latest snapshot's ClosingBalance + every ledger transaction
// posted after that snapshot's AsOfDate (see (*serviceImpl).GetCurrentBalance).
type CurrentBalance struct {
	LeaveTypeID        string  `json:"leave_type_id"`
	LeaveTypeName      string  `json:"leave_type_name"`
	HasPolicy          bool    `json:"has_policy"` // false = balance tracking not opted in for this leave type
	PolicyID           *string `json:"policy_id,omitempty"`
	SnapshotAsOfDate   *string `json:"snapshot_as_of_date,omitempty"` // nil if no snapshot has ever been written yet
	SnapshotClosing    float64 `json:"snapshot_closing"`
	DeltaSinceSnapshot float64 `json:"delta_since_snapshot"`
	Balance            float64 `json:"balance"` // SnapshotClosing + DeltaSinceSnapshot
	IsNegative         bool    `json:"is_negative"`
}

// EncashmentSummary is one leave type's recorded-but-unpriced encashment for
// an employee, for the F&F settlement to price.
//
// This package records encashed DAYS and deliberately never computes money —
// see PostEncashment. encashment_rate_basis has been stored since Phase 2
// with the note that "a future F&F phase reads this"; Phase 9B is that phase,
// and this type is the handover. Leave still owns how many days; F&F owns
// what a day is worth.
type EncashmentSummary struct {
	LeaveTypeID   string               `json:"leave_type_id"`
	LeaveTypeName string               `json:"leave_type_name"`
	Days          float64              `json:"days"`
	RateBasis     *EncashmentRateBasis `json:"encashment_rate_basis,omitempty"`
}

// ─────────────────────────────────────────────────────────
// Leave Transactions (append-only ledger)
// ─────────────────────────────────────────────────────────

// LeaveTransaction is one append-only ledger row. Mirrors
// hrm_leave_transactions columns exactly. Never updated or deleted.
type LeaveTransaction struct {
	ID              string          `db:"id"                json:"id"`
	PublicID        string          `db:"public_id"         json:"public_id"`
	OrgID           string          `db:"org_id"            json:"org_id"`
	EmployeeID      string          `db:"employee_id"       json:"employee_id"`
	LeaveTypeID     string          `db:"leave_type_id"     json:"leave_type_id"`
	PolicyID        string          `db:"policy_id"         json:"policy_id"`
	TransactionType TransactionType `db:"transaction_type"  json:"transaction_type"`
	Days            float64         `db:"days"              json:"days"`             // signed
	TransactionDate string          `db:"transaction_date"  json:"transaction_date"` // YYYY-MM-DD
	LeaveRequestID  *string         `db:"leave_request_id"  json:"leave_request_id,omitempty"`
	Note            *string         `db:"note"              json:"note,omitempty"`
	CreatedBy       string          `db:"created_by"        json:"created_by"`
	CreatedAt       time.Time       `db:"created_at"        json:"created_at"`
}

// TransactionFilter narrows the ledger list query.
type TransactionFilter struct {
	EmployeeID      string
	LeaveTypeID     string
	TransactionType TransactionType
	Limit           int
	Offset          int
}

// PeriodTransactionSums holds per-type signed sums for one snapshot period,
// used by the accrual job to compute a monthly hrm_leave_balances row. Taken
// and Encashed are positive magnitudes (net of usage+usage_reversal, and
// encashment, respectively — both stored negative in the ledger, negated
// here to match the "days taken" / "days encashed" reading a balance
// snapshot's columns are meant to have). CarriedForward and Adjusted are
// signed net sums, added as-is into the closing-balance formula.
type PeriodTransactionSums struct {
	Accrued        float64
	Taken          float64
	Encashed       float64
	CarriedForward float64
	Adjusted       float64
}

func (f *TransactionFilter) Normalise() {
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

// TransactionListResponse wraps a paginated ledger read.
type TransactionListResponse struct {
	Transactions []*LeaveTransaction `json:"transactions"`
	Total        int                 `json:"total"`
	Limit        int                 `json:"limit"`
	Offset       int                 `json:"offset"`
}

// PostAdjustmentRequest is the body for POST .../balances/:leaveTypeId/adjust.
// Days is signed: positive credits the balance, negative debits it. A note
// is mandatory — every manual correction must explain itself in the ledger.
type PostAdjustmentRequest struct {
	Days float64 `json:"days"`
	Note string  `json:"note"`
}

// PostEncashmentRequest is the body for POST .../balances/:leaveTypeId/encash.
// Days must be positive (the number of days being encashed) — the service
// posts the corresponding negative ledger debit. No money amount is
// accepted or computed here, per Phase 2 scope (decision #3).
type PostEncashmentRequest struct {
	Days int     `json:"days"`
	Note *string `json:"note"`
}

// ─────────────────────────────────────────────────────────
// Balance sentinel errors
// ─────────────────────────────────────────────────────────

var (
	ErrPolicyNotFound         = errors.New("leave policy not found")
	ErrPolicyAlreadyExists    = errors.New("an active leave policy already exists for this leave type")
	ErrInvalidAccrualMethod   = errors.New("accrual_method must be one of: monthly, annual, on_joining")
	ErrInvalidAccrualRate     = errors.New("accrual_rate must be zero or greater")
	ErrInvalidCarryForwardCap = errors.New("carry_forward_cap must be zero or greater when set")
	ErrInvalidEncashmentBasis = errors.New("encashment_rate_basis must be one of: basic_pay, gross_pay, fixed")
	ErrAdjustmentNoteRequired = errors.New("a note is required for a manual balance adjustment")
	ErrAdjustmentDaysZero     = errors.New("adjustment days must not be zero")
	ErrEncashmentNotAllowed   = errors.New("this leave type's policy does not allow encashment")
	ErrEncashmentDaysInvalid  = errors.New("encashment days must be greater than zero")
	ErrNoActivePolicy         = errors.New("no active leave policy exists for this leave type")
)
