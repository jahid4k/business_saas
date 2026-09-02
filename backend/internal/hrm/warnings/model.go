// backend/internal/hrm/warnings/model.go
package warnings

import (
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

// WarningStatus tracks the warning lifecycle.
// draft → pending_approval (if type requires it) → issued → acknowledged | appealed → closed
type WarningStatus string

const (
	StatusDraft           WarningStatus = "draft"
	StatusPendingApproval WarningStatus = "pending_approval"
	StatusIssued          WarningStatus = "issued"
	StatusAcknowledged    WarningStatus = "acknowledged"
	StatusAppealed        WarningStatus = "appealed"
	StatusClosed          WarningStatus = "closed"
	StatusCancelled       WarningStatus = "cancelled"
)

// EmployeeWarning is a formally issued warning record.
// warning_type_id references A3 (hrm_warning_types) for the category config.
// Key fields snapshotted at issuance: warning_type_name, severity_level,
// can_employee_respond, response_window_days — immune to config changes.
type EmployeeWarning struct {
	ID                  string        `db:"id"                     json:"id"`
	PublicID            string        `db:"public_id"              json:"public_id"`
	OrgID               string        `db:"org_id"                 json:"org_id"`
	EmployeeID          string        `db:"employee_id"            json:"employee_id"`
	WarningTypeID       string        `db:"warning_type_id"        json:"warning_type_id"`
	WarningTypeName     string        `db:"warning_type_name"      json:"warning_type_name"`
	SeverityLevel       int           `db:"severity_level"         json:"severity_level"`
	Title               string        `db:"title"                  json:"title"`
	Description         string        `db:"description"            json:"description"`
	IncidentDate        string        `db:"incident_date"          json:"incident_date"`
	IssuedBy            string        `db:"issued_by"              json:"issued_by"`
	WitnessIDs          []string      `db:"witness_ids"            json:"witness_ids"`
	ApprovalInstanceID  *string       `db:"approval_instance_id"   json:"approval_instance_id,omitempty"`
	DocumentID          *string       `db:"document_id"            json:"document_id,omitempty"`
	CanEmployeeRespond  bool          `db:"can_employee_respond"   json:"can_employee_respond"`
	ResponseWindowDays  int           `db:"response_window_days"   json:"response_window_days"`
	ResponseDeadline    *string       `db:"response_deadline"      json:"response_deadline,omitempty"`
	EmployeeResponse    *string       `db:"employee_response"      json:"employee_response,omitempty"`
	EmployeeRespondedAt *time.Time    `db:"employee_responded_at"  json:"employee_responded_at,omitempty"`
	AppealReason        *string       `db:"appeal_reason"          json:"appeal_reason,omitempty"`
	AppealSubmittedAt   *time.Time    `db:"appeal_submitted_at"    json:"appeal_submitted_at,omitempty"`
	AppealResolution    *string       `db:"appeal_resolution"      json:"appeal_resolution,omitempty"`
	AppealResolvedAt    *time.Time    `db:"appeal_resolved_at"     json:"appeal_resolved_at,omitempty"`
	ExpiresAt           *string       `db:"expires_at"             json:"expires_at,omitempty"`
	IsActive            bool          `db:"is_active"              json:"is_active"`
	IssuedAt            *time.Time    `db:"issued_at"              json:"issued_at,omitempty"`
	Status              WarningStatus `db:"status"                 json:"status"`
	CreatedBy           string        `db:"created_by"             json:"created_by"`
	CreatedAt           time.Time     `db:"created_at"             json:"created_at"`
	UpdatedAt           time.Time     `db:"updated_at"             json:"updated_at"`
}

type CreateWarningRequest struct {
	WarningTypeID string   `json:"warning_type_id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	IncidentDate  string   `json:"incident_date"`
	WitnessIDs    []string `json:"witness_ids"`
}

type UpdateWarningRequest struct {
	Title        *string  `json:"title"`
	Description  *string  `json:"description"`
	IncidentDate *string  `json:"incident_date"`
	WitnessIDs   []string `json:"witness_ids"`
	DocumentID   *string  `json:"document_id"`
}

// IssueRequest formally issues a draft warning to the employee.
type IssueRequest struct {
	DocumentID *string `json:"document_id"` // optional: pre-generated letter
}

// AcknowledgeRequest is submitted by the employee.
type AcknowledgeRequest struct {
	Response *string `json:"response"` // optional note from employee
}

// AppealRequest is submitted by the employee contesting the warning.
type AppealRequest struct {
	Reason string `json:"reason"`
}

// CloseRequest is used by HR to close (resolve/resolve appeal) or cancel.
type CloseRequest struct {
	AppealResolution *string `json:"appeal_resolution"` // required if closing an appeal
}

type WarningListResponse struct {
	Warnings []*EmployeeWarning `json:"warnings"`
	Total    int                `json:"total"`
	Limit    int                `json:"limit"`
	Offset   int                `json:"offset"`
}

// WarningListFilter narrows the warning list query.
type WarningListFilter struct {
	EmployeeID string
	Status     string
	ActiveOnly bool
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

func (f *WarningListFilter) Normalise() {
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
	ErrNotFound               = errors.New("warning not found")
	ErrWarningTypeNotFound    = errors.New("warning type not found")
	ErrTitleRequired          = errors.New("title is required")
	ErrDescriptionRequired    = errors.New("description is required")
	ErrIncidentDateRequired   = errors.New("incident_date is required (YYYY-MM-DD)")
	ErrInvalidDate            = errors.New("date must be a valid YYYY-MM-DD")
	ErrWarningTypeIDRequired  = errors.New("warning_type_id is required")
	ErrWrongStatus            = errors.New("action not allowed in current warning status")
	ErrAlreadyIssued          = errors.New("warning has already been issued")
	ErrCannotAppeal           = errors.New("warning type does not allow employee response")
	ErrResponseDeadlinePassed = errors.New("response deadline has passed")
)
