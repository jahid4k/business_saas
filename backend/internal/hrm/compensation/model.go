// backend/internal/hrm/compensation/model.go
package compensation

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

// ── Compensation bands ──────────────────────────────────────────────────────

// Band is a min/mid/max pay range for a grade, effective-dated. Mutable
// catalog data — see migration 00098's header for why this is not
// append-only the way hrm_employee_salary_records is.
type Band struct {
	ID            string          `db:"id"             json:"id"`
	PublicID      string          `db:"public_id"       json:"public_id"`
	OrgID         string          `db:"org_id"          json:"org_id"`
	GradeLabel    string          `db:"grade_label"     json:"grade_label"`
	Currency      string          `db:"currency"        json:"currency"`
	MinAmount     decimal.Decimal `db:"min_amount"      json:"min_amount"`
	MidAmount     decimal.Decimal `db:"mid_amount"      json:"mid_amount"`
	MaxAmount     decimal.Decimal `db:"max_amount"      json:"max_amount"`
	EffectiveDate time.Time       `db:"effective_date"  json:"effective_date"`
	CreatedBy     string          `db:"created_by"      json:"created_by"`
	CreatedAt     time.Time       `db:"created_at"      json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"      json:"updated_at"`
}

type CreateBandRequest struct {
	GradeLabel    string  `json:"grade_label"`
	Currency      *string `json:"currency"`
	MinAmount     string  `json:"min_amount"`
	MidAmount     string  `json:"mid_amount"`
	MaxAmount     string  `json:"max_amount"`
	EffectiveDate string  `json:"effective_date"`
}

type UpdateBandRequest struct {
	MinAmount     *string `json:"min_amount"`
	MidAmount     *string `json:"mid_amount"`
	MaxAmount     *string `json:"max_amount"`
	EffectiveDate *string `json:"effective_date"`
}

// ── Merit matrix ─────────────────────────────────────────────────────────────

// MatrixCell maps a rating level and a compa-ratio range to a merit increase
// percentage. Real rows, not JSONB — see migration 00098's header.
type MatrixCell struct {
	ID            string           `db:"id"               json:"id"`
	PublicID      string           `db:"public_id"        json:"public_id"`
	OrgID         string           `db:"org_id"           json:"org_id"`
	RatingLevelID string           `db:"rating_level_id"  json:"rating_level_id"`
	CompaRatioMin decimal.Decimal  `db:"compa_ratio_min"  json:"compa_ratio_min"`
	CompaRatioMax *decimal.Decimal `db:"compa_ratio_max" json:"compa_ratio_max,omitempty"`
	IncreasePct   decimal.Decimal  `db:"increase_pct"     json:"increase_pct"`
	EffectiveDate time.Time        `db:"effective_date"   json:"effective_date"`
	CreatedBy     string           `db:"created_by"       json:"created_by"`
	CreatedAt     time.Time        `db:"created_at"       json:"created_at"`
	UpdatedAt     time.Time        `db:"updated_at"       json:"updated_at"`
}

type CreateMatrixCellRequest struct {
	RatingLevelID string  `json:"rating_level_id"`
	CompaRatioMin string  `json:"compa_ratio_min"`
	CompaRatioMax *string `json:"compa_ratio_max"`
	IncreasePct   string  `json:"increase_pct"`
	EffectiveDate string  `json:"effective_date"`
}

// ── Salary revision cycles ──────────────────────────────────────────────────

type CycleStatus string

const (
	CycleDraft           CycleStatus = "draft"
	CycleComputed        CycleStatus = "computed"
	CyclePendingApproval CycleStatus = "pending_approval"
	CycleApproved        CycleStatus = "approved"
	CycleApplied         CycleStatus = "applied"
	CycleRejected        CycleStatus = "rejected"
	CycleCancelled       CycleStatus = "cancelled"
)

type RevisionCycle struct {
	ID                 string      `db:"id"                    json:"id"`
	PublicID           string      `db:"public_id"              json:"public_id"`
	OrgID              string      `db:"org_id"                 json:"org_id"`
	Name               string      `db:"name"                   json:"name"`
	Description        *string     `db:"description"            json:"description,omitempty"`
	EffectiveDate      time.Time   `db:"effective_date"         json:"effective_date"`
	Status             CycleStatus `db:"status"                 json:"status"`
	ApprovalInstanceID *string     `db:"approval_instance_id"   json:"approval_instance_id,omitempty"`
	CreatedBy          string      `db:"created_by"             json:"created_by"`
	CreatedAt          time.Time   `db:"created_at"              json:"created_at"`
	UpdatedAt          time.Time   `db:"updated_at"              json:"updated_at"`
	ComputedAt         *time.Time  `db:"computed_at"            json:"computed_at,omitempty"`
	SubmittedAt        *time.Time  `db:"submitted_at"           json:"submitted_at,omitempty"`
	AppliedAt          *time.Time  `db:"applied_at"             json:"applied_at,omitempty"`
	AppliedBy          *string     `db:"applied_by"             json:"applied_by,omitempty"`
}

type CreateCycleRequest struct {
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	EffectiveDate string  `json:"effective_date"`
}

// Revision is one employee's proposed (then applied) salary change within a
// cycle.
type Revision struct {
	ID                  string          `db:"id"                    json:"id"`
	PublicID            string          `db:"public_id"              json:"public_id"`
	OrgID               string          `db:"org_id"                 json:"org_id"`
	CycleID             string          `db:"cycle_id"               json:"cycle_id"`
	EmployeeID          string          `db:"employee_id"            json:"employee_id"`
	CurrentBasicPay     decimal.Decimal `db:"current_basic_pay"      json:"current_basic_pay"`
	ProposedBasicPay    decimal.Decimal `db:"proposed_basic_pay"     json:"proposed_basic_pay"`
	IsExcluded          bool            `db:"is_excluded"            json:"is_excluded"`
	RatingLevelID       *string         `db:"rating_level_id"        json:"rating_level_id,omitempty"`
	CalculationSnapshot []byte          `db:"calculation_snapshot"   json:"calculation_snapshot"`
	ComputationWarning  *string         `db:"computation_warning"    json:"computation_warning,omitempty"`
	OverrideReason      *string         `db:"override_reason"        json:"override_reason,omitempty"`
	SalaryRecordID      *string         `db:"salary_record_id"       json:"salary_record_id,omitempty"`
	CreatedAt           time.Time       `db:"created_at"             json:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at"             json:"updated_at"`
}

// PctIncrease is computed at read time, never stored — the 00076 rule.
func (r *Revision) PctIncrease() decimal.Decimal {
	if r.CurrentBasicPay.IsZero() {
		return decimal.Zero
	}
	return r.ProposedBasicPay.Sub(r.CurrentBasicPay).
		Div(r.CurrentBasicPay).Mul(decimal.NewFromInt(100))
}

type OverrideRevisionRequest struct {
	ProposedBasicPay string `json:"proposed_basic_pay"`
	Reason           string `json:"reason"`
}

// ── Bonuses ──────────────────────────────────────────────────────────────────

type BonusType string

const (
	BonusPerformance   BonusType = "performance"
	BonusDiscretionary BonusType = "discretionary"
	BonusSigning       BonusType = "signing"
	BonusRetention     BonusType = "retention"
	BonusReferral      BonusType = "referral"
	BonusOther         BonusType = "other"
)

func (t BonusType) IsValid() bool {
	switch t {
	case BonusPerformance, BonusDiscretionary, BonusSigning, BonusRetention, BonusReferral, BonusOther:
		return true
	}
	return false
}

type BonusStatus string

const (
	BonusDraft           BonusStatus = "draft"
	BonusPendingApproval BonusStatus = "pending_approval"
	BonusApproved        BonusStatus = "approved"
	BonusRejected        BonusStatus = "rejected"
	BonusPaid            BonusStatus = "paid"
	BonusCancelled       BonusStatus = "cancelled"
)

type Bonus struct {
	ID                  string          `db:"id"                   json:"id"`
	PublicID            string          `db:"public_id"             json:"public_id"`
	OrgID               string          `db:"org_id"                json:"org_id"`
	EmployeeID          string          `db:"employee_id"           json:"employee_id"`
	BonusType           BonusType       `db:"bonus_type"            json:"bonus_type"`
	Description         *string         `db:"description"           json:"description,omitempty"`
	PeriodYear          int             `db:"period_year"           json:"period_year"`
	PeriodMonth         *int            `db:"period_month"          json:"period_month,omitempty"`
	Amount              decimal.Decimal `db:"amount"                json:"amount"`
	Currency            string          `db:"currency"              json:"currency"`
	CalculationSnapshot []byte          `db:"calculation_snapshot"  json:"calculation_snapshot"`
	Status              BonusStatus     `db:"status"                json:"status"`
	ApprovalInstanceID  *string         `db:"approval_instance_id"  json:"approval_instance_id,omitempty"`
	PayslipRunID        *string         `db:"payslip_run_id"        json:"payslip_run_id,omitempty"`
	PayslipLineID       *string         `db:"payslip_line_id"       json:"payslip_line_id,omitempty"`
	PaidAt              *time.Time      `db:"paid_at"               json:"paid_at,omitempty"`
	CreatedBy           string          `db:"created_by"            json:"created_by"`
	CreatedAt           time.Time       `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at"            json:"updated_at"`
}

type CreateBonusRequest struct {
	EmployeeID  string  `json:"employee_id"`
	BonusType   string  `json:"bonus_type"`
	Description *string `json:"description"`
	PeriodYear  int     `json:"period_year"`
	PeriodMonth *int    `json:"period_month"`
	Amount      string  `json:"amount"`
	Currency    *string `json:"currency"`
}

// ── Shared list filter (the payslips.SlipListFilter shape) ─────────────────

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

type RevisionListResponse struct {
	Revisions []*Revision `json:"revisions"`
	Total     int         `json:"total"`
	Limit     int         `json:"limit"`
	Offset    int         `json:"offset"`
}

type BonusListResponse struct {
	Bonuses []*Bonus `json:"bonuses"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrNotFound            = errors.New("compensation: not found")
	ErrBandNotFound        = errors.New("compensation band not found")
	ErrMatrixCellNotFound  = errors.New("merit matrix cell not found")
	ErrCycleNotFound       = errors.New("salary revision cycle not found")
	ErrRevisionNotFound    = errors.New("salary revision not found")
	ErrBonusNotFound       = errors.New("bonus not found")
	ErrInvalidAmount       = errors.New("amount must be a valid non-negative number")
	ErrInvalidBandRange    = errors.New("min_amount must be <= mid_amount <= max_amount")
	ErrInvalidBonusType    = errors.New("bonus_type is not a recognised value")
	ErrWrongCycleStatus    = errors.New("action not allowed in the cycle's current status")
	ErrWrongBonusStatus    = errors.New("action not allowed in the bonus's current status")
	ErrCycleHasNoRevisions = errors.New("cycle has no revisions to submit")
	ErrAccessDenied        = errors.New("access denied")
)
