// backend/internal/hrm/recruitment/model.go
package recruitment

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// dateLayout is the ISO 8601 date format used for target_start_date etc.
const dateLayout = "2006-01-02"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// EmploymentType mirrors hrm_employees.employment_type's values. Defined
// locally rather than imported from internal/hrm/employees — this package
// has no dependency on that one, and the two columns are independent (a
// requisition's employment_type is a plan, not a fact about a hired row).
type EmploymentType string

const (
	EmploymentTypeFullTime   EmploymentType = "full_time"
	EmploymentTypePartTime   EmploymentType = "part_time"
	EmploymentTypeContractor EmploymentType = "contractor"
	EmploymentTypeIntern     EmploymentType = "intern"
)

func (t EmploymentType) IsValid() bool {
	switch t {
	case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeContractor, EmploymentTypeIntern:
		return true
	}
	return false
}

// ============================================================
// Job Requisitions
// ============================================================

type RequisitionStatus string

const (
	RequisitionStatusDraft           RequisitionStatus = "draft"
	RequisitionStatusPendingApproval RequisitionStatus = "pending_approval"
	RequisitionStatusApproved        RequisitionStatus = "approved"
	RequisitionStatusRejected        RequisitionStatus = "rejected"
	RequisitionStatusOnHold          RequisitionStatus = "on_hold"
	RequisitionStatusClosed          RequisitionStatus = "closed"
	RequisitionStatusCancelled       RequisitionStatus = "cancelled"
)

// Requisition is an approval-gated headcount request.
type Requisition struct {
	ID                 string            `db:"id"                   json:"id"`
	PublicID           string            `db:"public_id"            json:"public_id"`
	OrgID              string            `db:"org_id"               json:"org_id"`
	Title              string            `db:"title"                json:"title"`
	DepartmentID       *string           `db:"department_id"        json:"department_id,omitempty"`
	PositionID         *string           `db:"position_id"          json:"position_id,omitempty"`
	HiringManagerID    *string           `db:"hiring_manager_id"    json:"hiring_manager_id,omitempty"`
	EmploymentType     EmploymentType    `db:"employment_type"      json:"employment_type"`
	Openings           int               `db:"openings"              json:"openings"`
	FilledCount        int               `db:"filled_count"          json:"filled_count"`
	Location           *string           `db:"location"              json:"location,omitempty"`
	SalaryMin          *decimal.Decimal  `db:"salary_min"            json:"salary_min,omitempty"`
	SalaryMax          *decimal.Decimal  `db:"salary_max"            json:"salary_max,omitempty"`
	SalaryCurrency     string            `db:"salary_currency"       json:"salary_currency"`
	Justification      *string           `db:"justification"         json:"justification,omitempty"`
	TargetStartDate    *time.Time        `db:"target_start_date"     json:"target_start_date,omitempty"`
	Status             RequisitionStatus `db:"status"                json:"status"`
	ApprovalInstanceID *string           `db:"approval_instance_id"  json:"approval_instance_id,omitempty"`
	ClosedAt           *time.Time        `db:"closed_at"             json:"closed_at,omitempty"`
	CloseReason        *string           `db:"close_reason"          json:"close_reason,omitempty"`
	CreatedBy          string            `db:"created_by"            json:"created_by"`
	CreatedAt          time.Time         `db:"created_at"            json:"created_at"`
	UpdatedAt          time.Time         `db:"updated_at"            json:"updated_at"`
}

type CreateRequisitionRequest struct {
	Title           string           `json:"title"`
	DepartmentID    *string          `json:"department_id"`
	PositionID      *string          `json:"position_id"`
	HiringManagerID *string          `json:"hiring_manager_id"`
	EmploymentType  *string          `json:"employment_type"`
	Openings        *int             `json:"openings"`
	Location        *string          `json:"location"`
	SalaryMin       *decimal.Decimal `json:"salary_min"`
	SalaryMax       *decimal.Decimal `json:"salary_max"`
	SalaryCurrency  *string          `json:"salary_currency"`
	Justification   *string          `json:"justification"`
	TargetStartDate *string          `json:"target_start_date"`
}

type UpdateRequisitionRequest struct {
	Title           *string          `json:"title"`
	DepartmentID    *string          `json:"department_id"`
	PositionID      *string          `json:"position_id"`
	HiringManagerID *string          `json:"hiring_manager_id"`
	EmploymentType  *string          `json:"employment_type"`
	Openings        *int             `json:"openings"`
	Location        *string          `json:"location"`
	SalaryMin       *decimal.Decimal `json:"salary_min"`
	SalaryMax       *decimal.Decimal `json:"salary_max"`
	SalaryCurrency  *string          `json:"salary_currency"`
	Justification   *string          `json:"justification"`
	TargetStartDate *string          `json:"target_start_date"`
}

type CloseRequisitionRequest struct {
	Reason string `json:"reason"`
}

type RequisitionListFilter struct {
	Status string
	Limit  int
	Offset int
}

func (f *RequisitionListFilter) Normalise() {
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

type RequisitionListResponse struct {
	Requisitions []*Requisition `json:"requisitions"`
	Total        int            `json:"total"`
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
}

// ============================================================
// Job Postings
// ============================================================

type PostingStatus string

const (
	PostingStatusDraft     PostingStatus = "draft"
	PostingStatusPublished PostingStatus = "published"
	PostingStatusClosed    PostingStatus = "closed"
)

// Posting is the advertised role. public_slug is written now even though
// Phase 4A has no public route to read it — see migration 00078's header.
type Posting struct {
	ID                  string         `db:"id"                    json:"id"`
	PublicID            string         `db:"public_id"             json:"public_id"`
	OrgID               string         `db:"org_id"                json:"org_id"`
	RequisitionID       string         `db:"requisition_id"        json:"requisition_id"`
	PipelineID          string         `db:"pipeline_id"           json:"pipeline_id"`
	Title               string         `db:"title"                 json:"title"`
	DescriptionMarkdown string         `db:"description_markdown"  json:"description_markdown"`
	PublicSlug          string         `db:"public_slug"           json:"public_slug"`
	Location            *string        `db:"location"              json:"location,omitempty"`
	IsRemote            bool           `db:"is_remote"             json:"is_remote"`
	EmploymentType      EmploymentType `db:"employment_type"       json:"employment_type"`
	Status              PostingStatus  `db:"status"                json:"status"`
	PublishedAt         *time.Time     `db:"published_at"          json:"published_at,omitempty"`
	ClosedAt            *time.Time     `db:"closed_at"              json:"closed_at,omitempty"`
	CreatedBy           string         `db:"created_by"            json:"created_by"`
	CreatedAt           time.Time      `db:"created_at"             json:"created_at"`
	UpdatedAt           time.Time      `db:"updated_at"             json:"updated_at"`
}

type CreatePostingRequest struct {
	RequisitionID       string  `json:"requisition_id"`
	PipelineID          *string `json:"pipeline_id"`
	Title               string  `json:"title"`
	DescriptionMarkdown *string `json:"description_markdown"`
	PublicSlug          *string `json:"public_slug"`
	Location            *string `json:"location"`
	IsRemote            *bool   `json:"is_remote"`
	EmploymentType      *string `json:"employment_type"`
}

type UpdatePostingRequest struct {
	Title               *string `json:"title"`
	DescriptionMarkdown *string `json:"description_markdown"`
	PublicSlug          *string `json:"public_slug"`
	Location            *string `json:"location"`
	IsRemote            *bool   `json:"is_remote"`
	EmploymentType      *string `json:"employment_type"`
}

type PostingListFilter struct {
	Status        string
	RequisitionID string
	Limit         int
	Offset        int
}

func (f *PostingListFilter) Normalise() {
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

type PostingListResponse struct {
	Postings []*Posting `json:"postings"`
	Total    int        `json:"total"`
	Limit    int        `json:"limit"`
	Offset   int        `json:"offset"`
}

// ============================================================
// Sentinel errors
// ============================================================

var (
	ErrRequisitionNotFound   = errors.New("job requisition not found")
	ErrPostingNotFound       = errors.New("job posting not found")
	ErrTitleRequired         = errors.New("title is required")
	ErrInvalidEmploymentType = errors.New("employment_type must be one of: full_time, part_time, contractor, intern")
	ErrInvalidSalaryRange    = errors.New("salary_max must be greater than or equal to salary_min")
	ErrWrongStatus           = errors.New("action not allowed in current status")
	ErrSlugRequired          = errors.New("public_slug is required")
	ErrSlugTaken             = errors.New("public_slug is already in use for this organization")
	ErrRequisitionRequired   = errors.New("requisition_id is required")
	ErrPipelineRequired      = errors.New("pipeline_id is required")
)
