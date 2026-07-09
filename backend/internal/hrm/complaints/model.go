// backend/internal/hrm/complaints/model.go
package complaints

import (
	"errors"
	"time"
)

type ComplaintType string
const (
	TypeHarassment     ComplaintType = "harassment"
	TypeDiscrimination ComplaintType = "discrimination"
	TypeSafety         ComplaintType = "workplace_safety"
	TypePolicyViolation ComplaintType = "policy_violation"
	TypeManagerConduct ComplaintType = "manager_conduct"
	TypeWageDispute    ComplaintType = "wage_dispute"
	TypeRetaliation    ComplaintType = "retaliation"
	TypeGeneral        ComplaintType = "general"
)
func (t ComplaintType) IsValid() bool {
	switch t { case TypeHarassment, TypeDiscrimination, TypeSafety, TypePolicyViolation,
		TypeManagerConduct, TypeWageDispute, TypeRetaliation, TypeGeneral: return true }
	return false
}

type ComplaintStatus string
const (
	StatusSubmitted   ComplaintStatus = "submitted"
	StatusUnderReview ComplaintStatus = "under_review"
	StatusInvestigating ComplaintStatus = "investigating"
	StatusResolved    ComplaintStatus = "resolved"
	StatusDismissed   ComplaintStatus = "dismissed"
	StatusWithdrawn   ComplaintStatus = "withdrawn"
)

type Complaint struct {
	ID                     string          `db:"id"                      json:"id"`
	PublicID               string          `db:"public_id"               json:"public_id"`
	OrgID                  string          `db:"org_id"                  json:"org_id"`
	EmployeeID             string          `db:"employee_id"             json:"employee_id"`
	IsAnonymous            bool            `db:"is_anonymous"            json:"is_anonymous"`
	ComplaintType          ComplaintType   `db:"complaint_type"          json:"complaint_type"`
	Title                  string          `db:"title"                   json:"title"`
	Description            string          `db:"description"             json:"description"`
	IncidentDate           *string         `db:"incident_date"           json:"incident_date,omitempty"`
	AgainstEmployeeID      *string         `db:"against_employee_id"     json:"against_employee_id,omitempty"`
	AgainstDetails         *string         `db:"against_details"         json:"against_details,omitempty"`
	InvestigatorID         *string         `db:"investigator_id"         json:"investigator_id,omitempty"`
	InvestigationNotes     *string         `db:"investigation_notes"     json:"investigation_notes,omitempty"`
	InvestigationStartedAt *time.Time      `db:"investigation_started_at" json:"investigation_started_at,omitempty"`
	Resolution             *string         `db:"resolution"              json:"resolution,omitempty"`
	ResolutionAction       *string         `db:"resolution_action"       json:"resolution_action,omitempty"`
	ResolvedAt             *time.Time      `db:"resolved_at"             json:"resolved_at,omitempty"`
	ResolvedBy             *string         `db:"resolved_by"             json:"resolved_by,omitempty"`
	DocumentID             *string         `db:"document_id"             json:"document_id,omitempty"`
	Status                 ComplaintStatus `db:"status"                  json:"status"`
	CreatedBy              string          `db:"created_by"              json:"created_by"`
	CreatedAt              time.Time       `db:"created_at"              json:"created_at"`
	UpdatedAt              time.Time       `db:"updated_at"              json:"updated_at"`
}

type CreateComplaintRequest struct {
	ComplaintType     ComplaintType `json:"complaint_type"`
	Title             string        `json:"title"`
	Description       string        `json:"description"`
	IncidentDate      *string       `json:"incident_date"`
	AgainstEmployeeID *string       `json:"against_employee_id"`
	AgainstDetails    *string       `json:"against_details"`
	IsAnonymous       bool          `json:"is_anonymous"`
}

type UpdateComplaintRequest struct {
	Title             *string `json:"title"`
	Description       *string `json:"description"`
	IncidentDate      *string `json:"incident_date"`
	AgainstDetails    *string `json:"against_details"`
	DocumentID        *string `json:"document_id"`
}

type StartReviewRequest struct{}

type AssignRequest struct {
	InvestigatorID string `json:"investigator_id"`
}

type ResolveRequest struct {
	Resolution       string  `json:"resolution"`
	ResolutionAction *string `json:"resolution_action"` // warning_issued|termination|policy_updated|...
	DocumentID       *string `json:"document_id"`
}

type DismissRequest struct {
	Resolution string `json:"resolution"`
}

type ComplaintListResponse struct {
	Complaints []*Complaint `json:"complaints"`
	Total      int          `json:"total"`
}

var (
	ErrNotFound            = errors.New("complaint not found")
	ErrTitleRequired       = errors.New("title is required")
	ErrDescriptionRequired = errors.New("description is required")
	ErrInvalidType         = errors.New("invalid complaint_type")
	ErrInvalidDate         = errors.New("date must be a valid YYYY-MM-DD")
	ErrWrongStatus         = errors.New("action not allowed in current complaint status")
	ErrInvestigatorRequired = errors.New("investigator_id is required")
	ErrResolutionRequired   = errors.New("resolution text is required")
)
