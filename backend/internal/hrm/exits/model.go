// backend/internal/hrm/exits/model.go
package exits

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

// ── Source ───────────────────────────────────────────────────────────────────

// SourceType is the polymorphic discriminator naming which decision record
// this exit follows from. Deliberately narrow and FK-free — see migration
// 00114's header. Widening to 'abandonment' or 'end_of_contract' is a CHECK
// change, not a rewrite.
type SourceType string

const (
	SourceResignation SourceType = "resignation"
	SourceTermination SourceType = "termination"
)

func (s SourceType) IsValid() bool {
	return s == SourceResignation || s == SourceTermination
}

// ── Status ───────────────────────────────────────────────────────────────────

type Status string

const (
	StatusInitiated         Status = "initiated"
	StatusInClearance       Status = "in_clearance"
	StatusPendingSettlement Status = "pending_settlement"
	StatusSettled           Status = "settled"
	StatusCompleted         Status = "completed"
	StatusCancelled         Status = "cancelled"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusInitiated, StatusInClearance, StatusPendingSettlement,
		StatusSettled, StatusCompleted, StatusCancelled:
		return true
	}
	return false
}

// IsTerminal reports whether the exit has finished, either way. Used by the
// partial unique index's Go-side counterpart and by the revocation sweep.
func (s Status) IsTerminal() bool {
	return s == StatusCompleted || s == StatusCancelled
}

// ── Exit ─────────────────────────────────────────────────────────────────────

// Exit is the process that follows a resignation or termination decision.
//
// It carries NO clearance-completion column. Completion is "every blocking
// clearance item is resolved", derived on read — the 00076 rule, and the
// reason hrm_terminations.exit_clearance_completed is treated as a legacy
// column this package never touches.
type Exit struct {
	ID         string     `db:"id"           json:"id"`
	PublicID   string     `db:"public_id"     json:"public_id"`
	OrgID      string     `db:"org_id"        json:"org_id"`
	EmployeeID string     `db:"employee_id"   json:"employee_id"`
	SourceType SourceType `db:"source_type"   json:"source_type"`
	SourceID   string     `db:"source_id"     json:"source_id"`

	LastWorkingDate         time.Time  `db:"last_working_date"          json:"last_working_date"`
	ExpectedLastWorkingDate *time.Time `db:"expected_last_working_date" json:"expected_last_working_date,omitempty"`
	NoticeShortfallDays     int        `db:"notice_shortfall_days"      json:"notice_shortfall_days"`

	ChecklistInstanceID *string `db:"checklist_instance_id" json:"checklist_instance_id,omitempty"`
	FnFPayslipRunID     *string `db:"fnf_payslip_run_id"    json:"fnf_payslip_run_id,omitempty"`

	Status          Status     `db:"status"            json:"status"`
	AccessRevokedAt *time.Time `db:"access_revoked_at" json:"access_revoked_at,omitempty"`
	Remarks         *string    `db:"remarks"           json:"remarks,omitempty"`

	CreatedBy string    `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// DERIVED on read — no backing columns.
	Clearance *ClearanceSummary `db:"-" json:"clearance,omitempty"`
	Items     []*ClearanceItem  `db:"-" json:"clearance_items,omitempty"`
}

// ClearanceSummary is computed from the clearance items on every read. There
// is deliberately no stored counterpart — see migration 00114's header.
type ClearanceSummary struct {
	TotalItems      int             `json:"total_items"`
	ResolvedItems   int             `json:"resolved_items"`
	BlockingItems   int             `json:"blocking_items"`
	OutstandingDues decimal.Decimal `json:"outstanding_dues"`
	IsComplete      bool            `json:"is_complete"`
}

type CreateExitRequest struct {
	EmployeeID string `json:"employee_id"`
	SourceType string `json:"source_type"`
	// SourceID references the resignation or termination this exit follows.
	// FK-free by design, so the service validates it exists.
	SourceID string  `json:"source_id"`
	Remarks  *string `json:"remarks"`
}

type UpdateExitRequest struct {
	Remarks *string `json:"remarks"`
	// LastWorkingDate may be corrected before settlement; the shortfall is
	// recomputed from it rather than being separately editable, so the two
	// can never disagree.
	LastWorkingDate *string `json:"last_working_date"`
}

// ── Clearance ────────────────────────────────────────────────────────────────

// ClearanceItem is the money overlay on an offboarding checklist step. The
// checklist engine owns who, whether it blocks, and whether it is done;
// this owns what is owed, which the engine has no concept of.
type ClearanceItem struct {
	ID              string          `db:"id"                 json:"id"`
	PublicID        string          `db:"public_id"           json:"public_id"`
	ExitID          string          `db:"exit_id"             json:"exit_id"`
	ChecklistItemID *string         `db:"checklist_item_id"   json:"checklist_item_id,omitempty"`
	Department      string          `db:"department"          json:"department"`
	Description     string          `db:"description"         json:"description"`
	BlockingAmount  decimal.Decimal `db:"blocking_amount"     json:"blocking_amount"`
	Currency        string          `db:"currency"            json:"currency"`
	IsResolved      bool            `db:"is_resolved"         json:"is_resolved"`
	ResolvedBy      *string         `db:"resolved_by"         json:"resolved_by,omitempty"`
	ResolvedAt      *time.Time      `db:"resolved_at"         json:"resolved_at,omitempty"`
	ResolutionNote  *string         `db:"resolution_note"     json:"resolution_note,omitempty"`
	CreatedBy       string          `db:"created_by"          json:"created_by"`
	CreatedAt       time.Time       `db:"created_at"          json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at"          json:"updated_at"`
}

type CreateClearanceItemRequest struct {
	ChecklistItemID *string `json:"checklist_item_id"`
	Department      string  `json:"department"`
	Description     string  `json:"description"`
	// String, not float — the TestHygiene_NoFloatMoneyFields discipline
	// applies to the wire too, and a JSON number cannot represent every
	// decimal exactly.
	BlockingAmount *string `json:"blocking_amount"`
	Currency       *string `json:"currency"`
}

type ResolveClearanceItemRequest struct {
	ResolutionNote *string `json:"resolution_note"`
	// WaiveAmount resolves the item while leaving blocking_amount on record,
	// so a forgiven debt still shows what was forgiven rather than being
	// rewritten to zero.
	WaiveAmount bool `json:"waive_amount"`
}

// ── Rehire eligibility ───────────────────────────────────────────────────────

type RehireStatus string

const (
	RehireEligible    RehireStatus = "eligible"
	RehireNotEligible RehireStatus = "not_eligible"
	RehireConditional RehireStatus = "conditional"
)

func (r RehireStatus) IsValid() bool {
	switch r {
	case RehireEligible, RehireNotEligible, RehireConditional:
		return true
	}
	return false
}

type RehireEligibility struct {
	ID         string       `db:"id"           json:"id"`
	PublicID   string       `db:"public_id"     json:"public_id"`
	OrgID      string       `db:"org_id"        json:"org_id"`
	EmployeeID string       `db:"employee_id"   json:"employee_id"`
	ExitID     *string      `db:"exit_id"       json:"exit_id,omitempty"`
	Status     RehireStatus `db:"status"        json:"status"`
	Reason     *string      `db:"reason"        json:"reason,omitempty"`
	DecidedBy  *string      `db:"decided_by"    json:"decided_by,omitempty"`
	DecidedAt  time.Time    `db:"decided_at"    json:"decided_at"`
	CreatedAt  time.Time    `db:"created_at"    json:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at"    json:"updated_at"`
}

type DecideRehireRequest struct {
	Status string  `json:"status"`
	Reason *string `json:"reason"`
}

// ── Listing ──────────────────────────────────────────────────────────────────

// ListFilter carries the caller's resolved scope tier. Zero-valuing Scope
// yields authz.ScopeNone, which matches nothing — callers must set it
// explicitly, including internal ones. 7B lost an afternoon to a
// ListFilter{Limit: n} that silently saw zero rows.
type ListFilter struct {
	Status     string
	EmployeeID string

	Scope        authz.Scope
	CallerUserID string

	Limit  int
	Offset int
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

type ExitListResponse struct {
	Exits  []*Exit `json:"exits"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrExitNotFound          = errors.New("exit record not found")
	ErrClearanceItemNotFound = errors.New("clearance item not found")
	ErrEmployeeRequired      = errors.New("employee_id is required")
	ErrNameRequired          = errors.New("name is required")
	ErrInvalidSourceType     = errors.New("source_type must be one of resignation, termination")
	ErrSourceNotFound        = errors.New("the referenced resignation or termination does not exist")
	ErrSourceMismatch        = errors.New("the referenced decision record belongs to a different employee")
	ErrExitAlreadyOpen       = errors.New("this employee already has an exit in progress")
	ErrInvalidAmount         = errors.New("blocking_amount must be a non-negative decimal")
	ErrInvalidRehireStatus   = errors.New("status must be one of eligible, not_eligible, conditional")
	ErrAlreadyResolved       = errors.New("clearance item is already resolved")
	ErrWrongStatus           = errors.New("action not allowed in the exit's current status")
	ErrAccessDenied          = errors.New("access denied")
	ErrInvalidGratuityRule   = errors.New("gratuity rule requires a positive days_per_year, a non-negative minimum service period, a positive divisor and a valid effective_date")
)
