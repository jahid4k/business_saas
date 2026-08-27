// backend/internal/hrm/expenses/model.go
package expenses

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/reimbursements"
)

// ReimbursementCreator is the one-method slice of hrm/reimbursements.Service
// this package uses when an approved claim becomes payable.
//
// It names reimbursements' own types rather than mirroring them, so
// reimbursements.Service satisfies it structurally with no adapter — the
// corrected certifications.SkillGranter precedent. The import direction is
// safe: hrm/reimbursements imports hrm/payslips (7C) and knows nothing of
// this package, so there is no cycle.
//
// This is THE 7C boundary: "claim lifecycle here, payout via payroll in
// compensation". An approved claim creates a reimbursement, which 7C's
// payslips.ReimbursementSource already pays out — so 8B adds no payroll
// coupling of its own and internal/hrm/payslips is untouched.
//
// A nil creator is valid and leaves an approved claim unpaid rather than
// panicking — the platform/checklists.ChecklistHook nil-hook precedent.
type ReimbursementCreator interface {
	Create(ctx context.Context, orgID, createdBy string, req reimbursements.CreateRequest) (*reimbursements.Reimbursement, error)
}

// ── Travel requests ──────────────────────────────────────────────────────────

type TravelStatus string

const (
	TravelDraft           TravelStatus = "draft"
	TravelPendingApproval TravelStatus = "pending_approval"
	TravelApproved        TravelStatus = "approved"
	TravelRejected        TravelStatus = "rejected"
	TravelCompleted       TravelStatus = "completed"
	TravelCancelled       TravelStatus = "cancelled"
)

type TravelRequest struct {
	ID                 string       `db:"id"                    json:"id"`
	PublicID           string       `db:"public_id"              json:"public_id"`
	OrgID              string       `db:"org_id"                 json:"org_id"`
	EmployeeID         string       `db:"employee_id"            json:"employee_id"`
	Purpose            string       `db:"purpose"                json:"purpose"`
	Destination        string       `db:"destination"            json:"destination"`
	DestinationCountry *string      `db:"destination_country"    json:"destination_country,omitempty"`
	StartDate          time.Time    `db:"start_date"             json:"start_date"`
	EndDate            time.Time    `db:"end_date"               json:"end_date"`
	Status             TravelStatus `db:"status"                 json:"status"`
	ApprovalInstanceID *string      `db:"approval_instance_id"   json:"approval_instance_id,omitempty"`
	Currency           string       `db:"currency"               json:"currency"`
	CreatedBy          string       `db:"created_by"             json:"created_by"`
	CreatedAt          time.Time    `db:"created_at"             json:"created_at"`
	UpdatedAt          time.Time    `db:"updated_at"             json:"updated_at"`
}

// DurationDays is inclusive of both endpoints — a same-day trip is one day,
// which is what per diem must pay. Computed, never stored.
func (t *TravelRequest) DurationDays() int {
	return int(t.EndDate.Sub(t.StartDate).Hours()/24) + 1
}

type CreateTravelRequest struct {
	Purpose            string  `json:"purpose"`
	Destination        string  `json:"destination"`
	DestinationCountry *string `json:"destination_country"`
	StartDate          string  `json:"start_date"`
	EndDate            string  `json:"end_date"`
	Currency           *string `json:"currency"`
}

type ItineraryItemType string

const (
	ItineraryFlight    ItineraryItemType = "flight"
	ItineraryTrain     ItineraryItemType = "train"
	ItineraryHotel     ItineraryItemType = "hotel"
	ItineraryCarRental ItineraryItemType = "car_rental"
	ItineraryOther     ItineraryItemType = "other"
)

func (t ItineraryItemType) IsValid() bool {
	switch t {
	case ItineraryFlight, ItineraryTrain, ItineraryHotel, ItineraryCarRental, ItineraryOther:
		return true
	}
	return false
}

type ItineraryItem struct {
	ID               string            `db:"id"                  json:"id"`
	PublicID         string            `db:"public_id"            json:"public_id"`
	TravelRequestID  string            `db:"travel_request_id"    json:"travel_request_id"`
	ItemType         ItineraryItemType `db:"item_type"            json:"item_type"`
	Description      *string           `db:"description"          json:"description,omitempty"`
	FromLocation     *string           `db:"from_location"        json:"from_location,omitempty"`
	ToLocation       *string           `db:"to_location"          json:"to_location,omitempty"`
	StartsAt         *time.Time        `db:"starts_at"            json:"starts_at,omitempty"`
	EndsAt           *time.Time        `db:"ends_at"              json:"ends_at,omitempty"`
	BookingReference *string           `db:"booking_reference"    json:"booking_reference,omitempty"`
	EstimatedCost    decimal.Decimal   `db:"estimated_cost"       json:"estimated_cost"`
	Currency         string            `db:"currency"             json:"currency"`
	DisplayOrder     int               `db:"display_order"        json:"display_order"`
	CreatedAt        time.Time         `db:"created_at"           json:"created_at"`
	UpdatedAt        time.Time         `db:"updated_at"           json:"updated_at"`
}

type CreateItineraryItemRequest struct {
	ItemType         string  `json:"item_type"`
	Description      *string `json:"description"`
	FromLocation     *string `json:"from_location"`
	ToLocation       *string `json:"to_location"`
	StartsAt         *string `json:"starts_at"`
	EndsAt           *string `json:"ends_at"`
	BookingReference *string `json:"booking_reference"`
	EstimatedCost    *string `json:"estimated_cost"`
	Currency         *string `json:"currency"`
	DisplayOrder     *int    `json:"display_order"`
}

// ── Advances ─────────────────────────────────────────────────────────────────

type AdvanceStatus string

const (
	AdvancePending   AdvanceStatus = "pending"
	AdvanceDisbursed AdvanceStatus = "disbursed"
	AdvanceSettled   AdvanceStatus = "settled"
	AdvanceCancelled AdvanceStatus = "cancelled"
)

// Advance is money paid BEFORE a trip. settled_amount increases toward
// amount as claims settle against it — the hrm_loan_schedules shape.
type Advance struct {
	ID              string          `db:"id"                  json:"id"`
	PublicID        string          `db:"public_id"            json:"public_id"`
	OrgID           string          `db:"org_id"               json:"org_id"`
	EmployeeID      string          `db:"employee_id"          json:"employee_id"`
	TravelRequestID *string         `db:"travel_request_id"    json:"travel_request_id,omitempty"`
	Amount          decimal.Decimal `db:"amount"               json:"amount"`
	Currency        string          `db:"currency"             json:"currency"`
	SettledAmount   decimal.Decimal `db:"settled_amount"       json:"settled_amount"`
	Status          AdvanceStatus   `db:"status"               json:"status"`
	DisbursedAt     *time.Time      `db:"disbursed_at"         json:"disbursed_at,omitempty"`
	DisbursedBy     *string         `db:"disbursed_by"         json:"disbursed_by,omitempty"`
	CreatedBy       string          `db:"created_by"           json:"created_by"`
	CreatedAt       time.Time       `db:"created_at"           json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at"           json:"updated_at"`
}

// Outstanding is what remains unsettled. Computed, never stored — the 00076
// rule, and what SettleAgainstAdvance must be handed so a second claim
// against the same advance sees only what is left.
func (a *Advance) Outstanding() decimal.Decimal {
	return a.Amount.Sub(a.SettledAmount)
}

type CreateAdvanceRequest struct {
	EmployeeID      string  `json:"employee_id"`
	TravelRequestID *string `json:"travel_request_id"`
	Amount          string  `json:"amount"`
	Currency        *string `json:"currency"`
}

// ── Expense claims ───────────────────────────────────────────────────────────

type ClaimStatus string

const (
	ClaimDraft             ClaimStatus = "draft"
	ClaimPendingApproval   ClaimStatus = "pending_approval"
	ClaimApproved          ClaimStatus = "approved"
	ClaimPartiallyApproved ClaimStatus = "partially_approved"
	ClaimRejected          ClaimStatus = "rejected"
	ClaimPaid              ClaimStatus = "paid"
	ClaimCancelled         ClaimStatus = "cancelled"
)

// Claim is the header. It deliberately carries NO total columns — both
// totals are SUM over its lines, computed at read time, because approval is
// per-line. See migration 00108's header.
type Claim struct {
	ID                 string      `db:"id"                    json:"id"`
	PublicID           string      `db:"public_id"              json:"public_id"`
	OrgID              string      `db:"org_id"                 json:"org_id"`
	EmployeeID         string      `db:"employee_id"            json:"employee_id"`
	TravelRequestID    *string     `db:"travel_request_id"      json:"travel_request_id,omitempty"`
	AdvanceID          *string     `db:"advance_id"             json:"advance_id,omitempty"`
	Title              string      `db:"title"                  json:"title"`
	Description        *string     `db:"description"            json:"description,omitempty"`
	BaseCurrency       string      `db:"base_currency"          json:"base_currency"`
	Status             ClaimStatus `db:"status"                 json:"status"`
	ApprovalInstanceID *string     `db:"approval_instance_id"   json:"approval_instance_id,omitempty"`
	ReimbursementID    *string     `db:"reimbursement_id"       json:"reimbursement_id,omitempty"`
	SubmittedAt        *time.Time  `db:"submitted_at"           json:"submitted_at,omitempty"`
	DecidedAt          *time.Time  `db:"decided_at"             json:"decided_at,omitempty"`
	CreatedBy          string      `db:"created_by"             json:"created_by"`
	CreatedAt          time.Time   `db:"created_at"             json:"created_at"`
	UpdatedAt          time.Time   `db:"updated_at"             json:"updated_at"`

	// DERIVED on every read — there are no backing columns.
	TotalClaimed   *decimal.Decimal `db:"-" json:"total_claimed,omitempty"`
	TotalApproved  *decimal.Decimal `db:"-" json:"total_approved,omitempty"`
	UndecidedLines *int             `db:"-" json:"undecided_lines,omitempty"`
	Lines          []*Line          `db:"-" json:"lines,omitempty"`
}

type CreateClaimRequest struct {
	TravelRequestID *string `json:"travel_request_id"`
	AdvanceID       *string `json:"advance_id"`
	Title           string  `json:"title"`
	Description     *string `json:"description"`
	BaseCurrency    *string `json:"base_currency"`
}

type LineCategory string

const (
	CategoryAirfare         LineCategory = "airfare"
	CategoryLodging         LineCategory = "lodging"
	CategoryMeals           LineCategory = "meals"
	CategoryGroundTransport LineCategory = "ground_transport"
	CategoryMileage         LineCategory = "mileage"
	CategoryPerDiem         LineCategory = "per_diem"
	CategorySupplies        LineCategory = "supplies"
	CategoryOther           LineCategory = "other"
)

func (c LineCategory) IsValid() bool {
	switch c {
	case CategoryAirfare, CategoryLodging, CategoryMeals, CategoryGroundTransport,
		CategoryMileage, CategoryPerDiem, CategorySupplies, CategoryOther:
		return true
	}
	return false
}

// Line is where approval actually happens. ApprovedAmount is nullable
// because NULL ("undecided") and 0 ("decided, nothing payable") are
// genuinely different states.
type Line struct {
	ID              string           `db:"id"                json:"id"`
	PublicID        string           `db:"public_id"          json:"public_id"`
	ClaimID         string           `db:"claim_id"           json:"claim_id"`
	Category        LineCategory     `db:"category"           json:"category"`
	Description     *string          `db:"description"        json:"description,omitempty"`
	ExpenseDate     time.Time        `db:"expense_date"       json:"expense_date"`
	Amount          decimal.Decimal  `db:"amount"             json:"amount"`
	Currency        string           `db:"currency"           json:"currency"`
	ExchangeRate    decimal.Decimal  `db:"exchange_rate"      json:"exchange_rate"`
	BaseAmount      decimal.Decimal  `db:"base_amount"        json:"base_amount"`
	ApprovedAmount  *decimal.Decimal `db:"approved_amount"    json:"approved_amount,omitempty"`
	ReceiptURL      *string          `db:"receipt_url"        json:"receipt_url,omitempty"`
	OCRRaw          []byte           `db:"ocr_raw"            json:"-"`
	MileageDistance *decimal.Decimal `db:"mileage_distance"   json:"mileage_distance,omitempty"`
	MileageRateID   *string          `db:"mileage_rate_id"    json:"mileage_rate_id,omitempty"`
	DisplayOrder    int              `db:"display_order"      json:"display_order"`
	CreatedAt       time.Time        `db:"created_at"         json:"created_at"`
	UpdatedAt       time.Time        `db:"updated_at"         json:"updated_at"`

	// DERIVED — the policy warnings this line raised, if any.
	Violations []*PolicyViolation `db:"-" json:"violations,omitempty"`
}

// IsDecided distinguishes "not yet reviewed" from "reviewed at zero".
func (l *Line) IsDecided() bool { return l.ApprovedAmount != nil }

type CreateLineRequest struct {
	Category        string  `json:"category"`
	Description     *string `json:"description"`
	ExpenseDate     string  `json:"expense_date"`
	Amount          *string `json:"amount"`
	Currency        *string `json:"currency"`
	ExchangeRate    *string `json:"exchange_rate"`
	ReceiptURL      *string `json:"receipt_url"`
	MileageDistance *string `json:"mileage_distance"`
	DisplayOrder    *int    `json:"display_order"`
}

// ApproveLineRequest sets ONE line's approved amount. This is the
// line-level approval the whole module is shaped around.
type ApproveLineRequest struct {
	ApprovedAmount string `json:"approved_amount"`
}

// ── Config: policies and rates ───────────────────────────────────────────────

type Policy struct {
	ID            string          `db:"id"              json:"id"`
	PublicID      string          `db:"public_id"        json:"public_id"`
	OrgID         string          `db:"org_id"           json:"org_id"`
	Category      LineCategory    `db:"category"         json:"category"`
	MaxAmount     decimal.Decimal `db:"max_amount"       json:"max_amount"`
	Currency      string          `db:"currency"         json:"currency"`
	EffectiveDate time.Time       `db:"effective_date"   json:"effective_date"`
	CreatedBy     string          `db:"created_by"       json:"created_by"`
	CreatedAt     time.Time       `db:"created_at"       json:"created_at"`
}

type CreatePolicyRequest struct {
	Category      string  `json:"category"`
	MaxAmount     string  `json:"max_amount"`
	Currency      *string `json:"currency"`
	EffectiveDate string  `json:"effective_date"`
}

type PerDiemRate struct {
	ID            string          `db:"id"              json:"id"`
	PublicID      string          `db:"public_id"        json:"public_id"`
	OrgID         string          `db:"org_id"           json:"org_id"`
	CountryCode   *string         `db:"country_code"     json:"country_code,omitempty"`
	DailyAmount   decimal.Decimal `db:"daily_amount"     json:"daily_amount"`
	Currency      string          `db:"currency"         json:"currency"`
	EffectiveDate time.Time       `db:"effective_date"   json:"effective_date"`
	CreatedBy     string          `db:"created_by"       json:"created_by"`
	CreatedAt     time.Time       `db:"created_at"       json:"created_at"`
}

type CreatePerDiemRateRequest struct {
	CountryCode   *string `json:"country_code"`
	DailyAmount   string  `json:"daily_amount"`
	Currency      *string `json:"currency"`
	EffectiveDate string  `json:"effective_date"`
}

type MileageRate struct {
	ID            string          `db:"id"              json:"id"`
	PublicID      string          `db:"public_id"        json:"public_id"`
	OrgID         string          `db:"org_id"           json:"org_id"`
	RatePerUnit   decimal.Decimal `db:"rate_per_unit"    json:"rate_per_unit"`
	Unit          string          `db:"unit"             json:"unit"`
	Currency      string          `db:"currency"         json:"currency"`
	EffectiveDate time.Time       `db:"effective_date"   json:"effective_date"`
	CreatedBy     string          `db:"created_by"       json:"created_by"`
	CreatedAt     time.Time       `db:"created_at"       json:"created_at"`
}

type CreateMileageRateRequest struct {
	RatePerUnit   string  `json:"rate_per_unit"`
	Unit          *string `json:"unit"`
	Currency      *string `json:"currency"`
	EffectiveDate string  `json:"effective_date"`
}

// PolicyViolation is a recorded WARNING. Submitting an over-policy claim
// succeeds; this is what the approver sees. See migration 00108's header.
type PolicyViolation struct {
	ID           string          `db:"id"              json:"id"`
	LineID       string          `db:"line_id"          json:"line_id"`
	PolicyID     *string         `db:"policy_id"        json:"policy_id,omitempty"`
	Category     string          `db:"category"         json:"category"`
	MaxAmount    decimal.Decimal `db:"max_amount"       json:"max_amount"`
	ActualAmount decimal.Decimal `db:"actual_amount"    json:"actual_amount"`
	Message      string          `db:"message"          json:"message"`
	CreatedAt    time.Time       `db:"created_at"       json:"created_at"`
}

// ── Shared list filter ───────────────────────────────────────────────────────

type ListFilter struct {
	EmployeeID   string
	Status       string
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

type TravelListResponse struct {
	Requests []*TravelRequest `json:"travel_requests"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
}

type AdvanceListResponse struct {
	Advances []*Advance `json:"advances"`
	Total    int        `json:"total"`
	Limit    int        `json:"limit"`
	Offset   int        `json:"offset"`
}

type ClaimListResponse struct {
	Claims []*Claim `json:"claims"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

// SettlementResult is what a caller sees after settling a claim.
type SettlementResult struct {
	Claim       *Claim            `json:"claim"`
	Outcome     SettlementOutcome `json:"outcome"`
	Applied     decimal.Decimal   `json:"applied_to_advance"`
	Payable     decimal.Decimal   `json:"payable_to_employee"`
	Recoverable decimal.Decimal   `json:"recoverable_from_employee"`
	// ReimbursementID is set only when something was actually payable.
	ReimbursementID *string `json:"reimbursement_id,omitempty"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrTravelNotFound    = errors.New("travel request not found")
	ErrItineraryNotFound = errors.New("itinerary item not found")
	ErrAdvanceNotFound   = errors.New("travel advance not found")
	ErrClaimNotFound     = errors.New("expense claim not found")
	ErrLineNotFound      = errors.New("expense line not found")
	ErrPolicyNotFound    = errors.New("expense policy not found")

	ErrInvalidAmount       = errors.New("amount must be a valid non-negative number")
	ErrInvalidCategory     = errors.New("category is not a recognised value")
	ErrInvalidItemType     = errors.New("item_type is not a recognised value")
	ErrInvalidDateRange    = errors.New("end_date must be on or after start_date")
	ErrInvalidExchangeRate = errors.New("exchange_rate must be a positive number")
	// ErrApprovedExceedsClaimed guards the CHECK: an approver may reduce a
	// line, never inflate it beyond what was actually spent.
	ErrApprovedExceedsClaimed = errors.New("approved_amount cannot exceed the line's claimed amount")
	ErrWrongStatus            = errors.New("action not allowed in the record's current status")
	ErrClaimHasNoLines        = errors.New("claim has no lines to submit")
	ErrLinesUndecided         = errors.New("every line must be decided before the claim can be settled")
	ErrAlreadySettled         = errors.New("claim has already been settled")
	ErrAccessDenied           = errors.New("access denied")
)
