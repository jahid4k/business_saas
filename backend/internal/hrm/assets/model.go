// backend/internal/hrm/assets/model.go
package assets

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

// ── Categories ───────────────────────────────────────────────────────────────

// Category is catalog data. requires_return lives here rather than on the
// instance because it is a property of the KIND of thing — see migration
// 00106's header.
type Category struct {
	ID               string    `db:"id"                  json:"id"`
	PublicID         string    `db:"public_id"           json:"public_id"`
	OrgID            string    `db:"org_id"              json:"org_id"`
	Name             string    `db:"name"                json:"name"`
	Description      *string   `db:"description"         json:"description,omitempty"`
	RequiresReturn   bool      `db:"requires_return"     json:"requires_return"`
	UsefulLifeMonths *int      `db:"useful_life_months"  json:"useful_life_months,omitempty"`
	IsActive         bool      `db:"is_active"           json:"is_active"`
	CreatedBy        string    `db:"created_by"          json:"created_by"`
	CreatedAt        time.Time `db:"created_at"          json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"          json:"updated_at"`
}

type CreateCategoryRequest struct {
	Name             string  `json:"name"`
	Description      *string `json:"description"`
	RequiresReturn   *bool   `json:"requires_return"`
	UsefulLifeMonths *int    `json:"useful_life_months"`
}

// ── Assets ───────────────────────────────────────────────────────────────────

type AssetStatus string

const (
	AssetAvailable     AssetStatus = "available"
	AssetAssigned      AssetStatus = "assigned"
	AssetInMaintenance AssetStatus = "in_maintenance"
	AssetRetired       AssetStatus = "retired"
	AssetLost          AssetStatus = "lost"
)

func (s AssetStatus) IsValid() bool {
	switch s {
	case AssetAvailable, AssetAssigned, AssetInMaintenance, AssetRetired, AssetLost:
		return true
	}
	return false
}

// Asset is one physical instance.
//
// There is deliberately NO CurrentHolderID field, and no such column exists —
// see migration 00106's header and the information_schema integration test.
// CurrentHolder below is populated by a JOIN at read time, from the
// assignment row with returned_at IS NULL.
type Asset struct {
	ID           string          `db:"id"             json:"id"`
	PublicID     string          `db:"public_id"       json:"public_id"`
	OrgID        string          `db:"org_id"          json:"org_id"`
	CategoryID   *string         `db:"category_id"     json:"category_id,omitempty"`
	Name         string          `db:"name"            json:"name"`
	AssetTag     *string         `db:"asset_tag"       json:"asset_tag,omitempty"`
	SerialNumber *string         `db:"serial_number"   json:"serial_number,omitempty"`
	PurchaseDate *time.Time      `db:"purchase_date"   json:"purchase_date,omitempty"`
	PurchaseCost decimal.Decimal `db:"purchase_cost"   json:"purchase_cost"`
	Currency     string          `db:"currency"        json:"currency"`
	Status       AssetStatus     `db:"status"          json:"status"`
	Notes        *string         `db:"notes"           json:"notes,omitempty"`
	CreatedBy    string          `db:"created_by"      json:"created_by"`
	CreatedAt    time.Time       `db:"created_at"      json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"      json:"updated_at"`

	// DERIVED, never stored. Both are filled in by the read path — the
	// current holder from hrm_asset_assignments, the book value from
	// BookValue(). Neither has a backing column.
	CurrentHolderEmployeeID *string          `db:"-" json:"current_holder_employee_id,omitempty"`
	BookValue               *decimal.Decimal `db:"-" json:"book_value,omitempty"`
}

type CreateAssetRequest struct {
	CategoryID   *string `json:"category_id"`
	Name         string  `json:"name"`
	AssetTag     *string `json:"asset_tag"`
	SerialNumber *string `json:"serial_number"`
	PurchaseDate *string `json:"purchase_date"`
	PurchaseCost *string `json:"purchase_cost"`
	Currency     *string `json:"currency"`
	Notes        *string `json:"notes"`
}

// ── Assignments ──────────────────────────────────────────────────────────────

type Condition string

const (
	ConditionNew     Condition = "new"
	ConditionGood    Condition = "good"
	ConditionFair    Condition = "fair"
	ConditionPoor    Condition = "poor"
	ConditionDamaged Condition = "damaged"
)

// IsValidOut is the narrower set: an asset cannot go OUT damaged.
func (c Condition) IsValidOut() bool {
	switch c {
	case ConditionNew, ConditionGood, ConditionFair, ConditionPoor:
		return true
	}
	return false
}

func (c Condition) IsValidIn() bool {
	switch c {
	case ConditionNew, ConditionGood, ConditionFair, ConditionPoor, ConditionDamaged:
		return true
	}
	return false
}

// Assignment is one row of an asset's holding history. returned_at IS NULL
// means "current" — uq_hrm_asgn_active guarantees at most one such row per
// asset.
type Assignment struct {
	ID           string     `db:"id"             json:"id"`
	PublicID     string     `db:"public_id"       json:"public_id"`
	OrgID        string     `db:"org_id"          json:"org_id"`
	AssetID      string     `db:"asset_id"        json:"asset_id"`
	EmployeeID   string     `db:"employee_id"     json:"employee_id"`
	AssignedAt   time.Time  `db:"assigned_at"     json:"assigned_at"`
	AssignedBy   string     `db:"assigned_by"     json:"assigned_by"`
	ConditionOut *Condition `db:"condition_out"   json:"condition_out,omitempty"`
	ReturnedAt   *time.Time `db:"returned_at"     json:"returned_at,omitempty"`
	ReturnedBy   *string    `db:"returned_by"     json:"returned_by,omitempty"`
	ConditionIn  *Condition `db:"condition_in"    json:"condition_in,omitempty"`
	Notes        *string    `db:"notes"           json:"notes,omitempty"`
	CreatedAt    time.Time  `db:"created_at"      json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"      json:"updated_at"`
}

// IsCurrent reports whether this assignment is the live one. Derived, the
// 00076 rule — there is no is_current column.
func (a *Assignment) IsCurrent() bool { return a.ReturnedAt == nil }

type AssignAssetRequest struct {
	EmployeeID   string  `json:"employee_id"`
	ConditionOut *string `json:"condition_out"`
	Notes        *string `json:"notes"`
}

type ReturnAssetRequest struct {
	ConditionIn *string `json:"condition_in"`
	Notes       *string `json:"notes"`
}

// ── Maintenance ──────────────────────────────────────────────────────────────

type MaintenanceType string

const (
	MaintenanceRepair     MaintenanceType = "repair"
	MaintenanceService    MaintenanceType = "service"
	MaintenanceUpgrade    MaintenanceType = "upgrade"
	MaintenanceInspection MaintenanceType = "inspection"
	MaintenanceOther      MaintenanceType = "other"
)

func (t MaintenanceType) IsValid() bool {
	switch t {
	case MaintenanceRepair, MaintenanceService, MaintenanceUpgrade, MaintenanceInspection, MaintenanceOther:
		return true
	}
	return false
}

type MaintenanceLog struct {
	ID              string          `db:"id"                json:"id"`
	PublicID        string          `db:"public_id"          json:"public_id"`
	OrgID           string          `db:"org_id"             json:"org_id"`
	AssetID         string          `db:"asset_id"           json:"asset_id"`
	MaintenanceType MaintenanceType `db:"maintenance_type"   json:"maintenance_type"`
	Description     *string         `db:"description"        json:"description,omitempty"`
	Cost            decimal.Decimal `db:"cost"               json:"cost"`
	Currency        string          `db:"currency"           json:"currency"`
	PerformedAt     time.Time       `db:"performed_at"       json:"performed_at"`
	Vendor          *string         `db:"vendor"             json:"vendor,omitempty"`
	CreatedBy       string          `db:"created_by"         json:"created_by"`
	CreatedAt       time.Time       `db:"created_at"         json:"created_at"`
}

type CreateMaintenanceRequest struct {
	MaintenanceType string  `json:"maintenance_type"`
	Description     *string `json:"description"`
	Cost            *string `json:"cost"`
	Currency        *string `json:"currency"`
	PerformedAt     string  `json:"performed_at"`
	Vendor          *string `json:"vendor"`
}

// ── Requests ─────────────────────────────────────────────────────────────────

type RequestStatus string

const (
	RequestDraft           RequestStatus = "draft"
	RequestPendingApproval RequestStatus = "pending_approval"
	RequestApproved        RequestStatus = "approved"
	RequestFulfilled       RequestStatus = "fulfilled"
	RequestRejected        RequestStatus = "rejected"
	RequestCancelled       RequestStatus = "cancelled"
)

type AssetRequest struct {
	ID                 string        `db:"id"                    json:"id"`
	PublicID           string        `db:"public_id"              json:"public_id"`
	OrgID              string        `db:"org_id"                 json:"org_id"`
	EmployeeID         string        `db:"employee_id"            json:"employee_id"`
	CategoryID         *string       `db:"category_id"            json:"category_id,omitempty"`
	Justification      *string       `db:"justification"          json:"justification,omitempty"`
	Status             RequestStatus `db:"status"                 json:"status"`
	ApprovalInstanceID *string       `db:"approval_instance_id"   json:"approval_instance_id,omitempty"`
	FulfilledAssetID   *string       `db:"fulfilled_asset_id"     json:"fulfilled_asset_id,omitempty"`
	FulfilledAt        *time.Time    `db:"fulfilled_at"           json:"fulfilled_at,omitempty"`
	CreatedBy          string        `db:"created_by"             json:"created_by"`
	CreatedAt          time.Time     `db:"created_at"             json:"created_at"`
	UpdatedAt          time.Time     `db:"updated_at"             json:"updated_at"`
}

type CreateAssetRequestRequest struct {
	CategoryID    *string `json:"category_id"`
	Justification *string `json:"justification"`
}

type FulfillRequestRequest struct {
	AssetID string `json:"asset_id"`
}

// ── Software licences ────────────────────────────────────────────────────────

// License is a DIFFERENT shape from Asset — N seats, a renewal date, per-seat
// cost, no single holder and no serial number. See migration 00106's header
// on why this is not just an asset category.
//
// There is deliberately NO SeatsUsed field with a backing column; SeatsUsed
// below is COUNT(*) over unreleased seat assignments, filled at read time.
type License struct {
	OrgID       string          `db:"org_id"         json:"org_id"`
	ID          string          `db:"id"             json:"id"`
	PublicID    string          `db:"public_id"       json:"public_id"`
	Name        string          `db:"name"            json:"name"`
	Vendor      *string         `db:"vendor"          json:"vendor,omitempty"`
	SeatsTotal  int             `db:"seats_total"     json:"seats_total"`
	CostPerSeat decimal.Decimal `db:"cost_per_seat"   json:"cost_per_seat"`
	Currency    string          `db:"currency"        json:"currency"`
	RenewalDate *time.Time      `db:"renewal_date"    json:"renewal_date,omitempty"`
	IsActive    bool            `db:"is_active"       json:"is_active"`
	CreatedBy   string          `db:"created_by"      json:"created_by"`
	CreatedAt   time.Time       `db:"created_at"      json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"      json:"updated_at"`

	// DERIVED, never stored — COUNT(*) of seats with released_at IS NULL.
	SeatsUsed *int `db:"-" json:"seats_used,omitempty"`
}

type CreateLicenseRequest struct {
	Name        string  `json:"name"`
	Vendor      *string `json:"vendor"`
	SeatsTotal  int     `json:"seats_total"`
	CostPerSeat *string `json:"cost_per_seat"`
	Currency    *string `json:"currency"`
	RenewalDate *string `json:"renewal_date"`
}

type SeatAssignment struct {
	ID         string     `db:"id"            json:"id"`
	PublicID   string     `db:"public_id"      json:"public_id"`
	OrgID      string     `db:"org_id"         json:"org_id"`
	LicenseID  string     `db:"license_id"     json:"license_id"`
	EmployeeID string     `db:"employee_id"    json:"employee_id"`
	AssignedAt time.Time  `db:"assigned_at"    json:"assigned_at"`
	AssignedBy string     `db:"assigned_by"    json:"assigned_by"`
	ReleasedAt *time.Time `db:"released_at"    json:"released_at,omitempty"`
	CreatedAt  time.Time  `db:"created_at"     json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"     json:"updated_at"`
}

type AssignSeatRequest struct {
	EmployeeID string `json:"employee_id"`
}

// ── Shared list filter ───────────────────────────────────────────────────────

type ListFilter struct {
	EmployeeID   string
	CategoryID   string
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

type AssetListResponse struct {
	Assets []*Asset `json:"assets"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

type AssignmentListResponse struct {
	Assignments []*Assignment `json:"assignments"`
	Total       int           `json:"total"`
	Limit       int           `json:"limit"`
	Offset      int           `json:"offset"`
}

type RequestListResponse struct {
	Requests []*AssetRequest `json:"requests"`
	Total    int             `json:"total"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrCategoryNotFound   = errors.New("asset category not found")
	ErrAssetNotFound      = errors.New("asset not found")
	ErrAssignmentNotFound = errors.New("asset assignment not found")
	ErrRequestNotFound    = errors.New("asset request not found")
	ErrLicenseNotFound    = errors.New("software licence not found")
	ErrSeatNotFound       = errors.New("licence seat assignment not found")

	ErrInvalidAmount          = errors.New("amount must be a valid non-negative number")
	ErrInvalidCondition       = errors.New("condition is not a recognised value")
	ErrInvalidMaintenanceType = errors.New("maintenance_type is not a recognised value")
	ErrInvalidSeatsTotal      = errors.New("seats_total must be a positive integer")

	// ErrAlreadyAssigned is what uq_hrm_asgn_active surfaces as. An asset can
	// only be in one person's hands at a time — that is what makes "current
	// holder" a derived query rather than a guess.
	ErrAlreadyAssigned = errors.New("asset is already assigned; record its return first")
	ErrNotAssigned     = errors.New("asset is not currently assigned to anyone")
	ErrNoSeatsLeft     = errors.New("licence has no free seats")
	ErrSeatAlreadyHeld = errors.New("employee already holds a seat on this licence")
	ErrWrongStatus     = errors.New("action not allowed in the record's current status")
	ErrAccessDenied    = errors.New("access denied")
)
