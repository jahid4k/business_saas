// backend/internal/hrm/resignations/model.go
package resignations

import (
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

type ResignationStatus string
const (
	StatusSubmitted ResignationStatus = "submitted"
	StatusAccepted  ResignationStatus = "accepted"
	StatusWithdrawn ResignationStatus = "withdrawn"
	StatusRejected  ResignationStatus = "rejected"
)

type ReasonCategory string
const (
	ReasonPersonal          ReasonCategory = "personal"
	ReasonCareerGrowth      ReasonCategory = "career_growth"
	ReasonBetterOpportunity ReasonCategory = "better_opportunity"
	ReasonRelocation        ReasonCategory = "relocation"
	ReasonHealth            ReasonCategory = "health"
	ReasonRetirement        ReasonCategory = "retirement"
	ReasonOther             ReasonCategory = "other"
)

func (r ReasonCategory) IsValid() bool {
	switch r {
	case ReasonPersonal, ReasonCareerGrowth, ReasonBetterOpportunity,
		ReasonRelocation, ReasonHealth, ReasonRetirement, ReasonOther:
		return true
	}
	return false
}

// Resignation records an employee's decision to leave.
// last_working_date = resignation_date + notice_period_days (from active contract).
// HR may override last_working_date by setting is_notice_waived = true.
type Resignation struct {
	ID                     string            `db:"id"                         json:"id"`
	PublicID               string            `db:"public_id"                  json:"public_id"`
	OrgID                  string            `db:"org_id"                     json:"org_id"`
	EmployeeID             string            `db:"employee_id"                json:"employee_id"`
	ResignationDate        string            `db:"resignation_date"           json:"resignation_date"`
	NoticePeriodDays       int               `db:"notice_period_days"         json:"notice_period_days"`
	IsNoticeWaived         bool              `db:"is_notice_waived"           json:"is_notice_waived"`
	LastWorkingDate        string            `db:"last_working_date"          json:"last_working_date"`
	ReasonCategory         ReasonCategory    `db:"reason_category"            json:"reason_category"`
	ReasonRemarks          *string           `db:"reason_remarks"             json:"reason_remarks,omitempty"`
	ApprovalInstanceID     *string           `db:"approval_instance_id"       json:"approval_instance_id,omitempty"`
	DocumentID             *string           `db:"document_id"                json:"document_id,omitempty"`
	ExitInterviewCompleted bool              `db:"exit_interview_completed"   json:"exit_interview_completed"`
	ExitClearanceCompleted bool              `db:"exit_clearance_completed"   json:"exit_clearance_completed"`
	Status                 ResignationStatus `db:"status"                     json:"status"`
	AcceptedAt             *time.Time        `db:"accepted_at"                json:"accepted_at,omitempty"`
	AcceptedBy             *string           `db:"accepted_by"                json:"accepted_by,omitempty"`
	CreatedBy              string            `db:"created_by"                 json:"created_by"`
	CreatedAt              time.Time         `db:"created_at"                 json:"created_at"`
	UpdatedAt              time.Time         `db:"updated_at"                 json:"updated_at"`
}

type SubmitResignationRequest struct {
	ResignationDate  string         `json:"resignation_date"`
	ReasonCategory   ReasonCategory `json:"reason_category"`
	ReasonRemarks    *string        `json:"reason_remarks"`
	// Optional — if not set, computed from active contract notice_period_days
	LastWorkingDate  *string        `json:"last_working_date"`
	IsNoticeWaived   bool           `json:"is_notice_waived"`
}

type UpdateResignationRequest struct {
	ExitInterviewCompleted *bool   `json:"exit_interview_completed"`
	ExitClearanceCompleted *bool   `json:"exit_clearance_completed"`
	DocumentID             *string `json:"document_id"`
	LastWorkingDate        *string `json:"last_working_date"` // HR override
}

type ResignationListResponse struct {
	Resignations []*Resignation `json:"resignations"`
	Total        int            `json:"total"`
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
}

// ResignationListFilter narrows the resignation list query.
type ResignationListFilter struct {
	EmployeeID string
	Status     string
	Limit      int
	Offset     int

	// Scope and CallerUserID are set by the handler (from authzSvc.ResolveScope)
	// before calling Service.List. Scope zero value (authz.ScopeNone) means "no
	// rows" — callers that intend no scoping must explicitly pass authz.ScopeAll.
	Scope        authz.Scope
	CallerUserID string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *ResignationListFilter) Normalise() {
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

var (
	ErrNotFound              = errors.New("resignation not found")
	ErrAlreadyActive         = errors.New("employee already has an active resignation — withdraw first")
	ErrResignationDateReq    = errors.New("resignation_date is required (YYYY-MM-DD)")
	ErrInvalidDate           = errors.New("date must be a valid YYYY-MM-DD")
	ErrInvalidReasonCategory = errors.New("invalid reason_category")
	ErrWrongStatus           = errors.New("action not allowed in current resignation status")
	ErrAlreadyAccepted       = errors.New("resignation has already been accepted")
)
