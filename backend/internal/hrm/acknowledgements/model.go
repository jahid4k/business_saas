// backend/internal/hrm/acknowledgements/model.go
//
// C4: Generic Acknowledgement System — cross-cutting module.
// Uses hrm_acknowledgements table (migration 00036).
// Used by: C1 (warnings), C3 (documents), E2 (announcements), E3 (calendar events).
package acknowledgements

import (
	"errors"
	"time"
)

type AckStatus string
const (
	StatusPending      AckStatus = "pending"
	StatusAcknowledged AckStatus = "acknowledged"
	StatusDeclined     AckStatus = "declined"
	StatusExpired      AckStatus = "expired"
)

type AckType string

// ⚠ This enum and the hrm_acknowledgements_acknowledgeable_type_check CHECK
// must be widened TOGETHER. They drifted twice: migration 00086 (5B) added
// 'appraisal' and 00094 (6B) added 'course_completion' to the DB, but neither
// updated this enum — and Create() gates on IsValid(), so both values were
// unreachable through the only typed write path. Fixed in 8A alongside
// 'asset_handover' (00106); widening the DB alone just adds a dead value.
const (
	TypeWarning          AckType = "warning"
	TypeDocument         AckType = "document"
	TypeAnnouncement     AckType = "announcement"
	TypeCalendarEvent    AckType = "calendar_event"
	TypePolicy           AckType = "policy"
	TypeAppraisal        AckType = "appraisal"
	TypeCourseCompletion AckType = "course_completion"
	TypeAssetHandover    AckType = "asset_handover"
)

func (t AckType) IsValid() bool {
	switch t {
	case TypeWarning, TypeDocument, TypeAnnouncement, TypeCalendarEvent, TypePolicy,
		TypeAppraisal, TypeCourseCompletion, TypeAssetHandover:
		return true
	}
	return false
}

// Acknowledgement records an employee's acknowledgement (or decline) of an entity.
type Acknowledgement struct {
	ID                   string     `db:"id"                    json:"id"`
	PublicID             string     `db:"public_id"             json:"public_id"`
	OrgID                string     `db:"org_id"                json:"org_id"`
	EmployeeID           string     `db:"employee_id"           json:"employee_id"`
	AcknowledgeableType  AckType    `db:"acknowledgeable_type"  json:"acknowledgeable_type"`
	AcknowledgeableID    string     `db:"acknowledgeable_id"    json:"acknowledgeable_id"`
	EntityTitle          string     `db:"entity_title"          json:"entity_title"`
	Notes                *string    `db:"notes"                 json:"notes,omitempty"`
	SignatureRequired    bool       `db:"signature_required"    json:"signature_required"`
	SignedAt             *time.Time `db:"signed_at"             json:"signed_at,omitempty"`
	SignatureData        *string    `db:"signature_data"        json:"signature_data,omitempty"`
	Status               AckStatus  `db:"status"                json:"status"`
	AcknowledgedAt       *time.Time `db:"acknowledged_at"       json:"acknowledged_at,omitempty"`
	DeclinedAt           *time.Time `db:"declined_at"           json:"declined_at,omitempty"`
	DeclineReason        *string    `db:"decline_reason"        json:"decline_reason,omitempty"`
	ExpiresAt            *string    `db:"expires_at"            json:"expires_at,omitempty"`
	RequestedBy          string     `db:"requested_by"          json:"requested_by"`
	RequestedAt          time.Time  `db:"requested_at"          json:"requested_at"`
	ReminderSentAt       *time.Time `db:"reminder_sent_at"      json:"reminder_sent_at,omitempty"`
	CreatedAt            time.Time  `db:"created_at"            json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at"            json:"updated_at"`
}

// CreateAcknowledgementRequest sends an acknowledgement request to an employee.
type CreateAcknowledgementRequest struct {
	EmployeeID          string  `json:"employee_id"`
	AcknowledgeableType AckType `json:"acknowledgeable_type"`
	AcknowledgeableID   string  `json:"acknowledgeable_id"`
	EntityTitle         string  `json:"entity_title"`
	SignatureRequired   bool    `json:"signature_required"`
	ExpiresAt           *string `json:"expires_at"` // YYYY-MM-DD
}

// RespondRequest is the employee's response.
type RespondRequest struct {
	Notes         *string `json:"notes"`
	SignatureData *string `json:"signature_data"` // required if signature_required=true
}

type DeclineRequest struct {
	Reason string `json:"reason"`
}

type AckListResponse struct {
	Acknowledgements []*Acknowledgement `json:"acknowledgements"`
	Total            int                `json:"total"`
}

var (
	ErrNotFound           = errors.New("acknowledgement not found")
	ErrEmployeeIDRequired = errors.New("employee_id is required")
	ErrEntityTitleRequired = errors.New("entity_title is required")
	ErrAckTypeRequired    = errors.New("acknowledgeable_type is required")
	ErrAckIDRequired      = errors.New("acknowledgeable_id is required")
	ErrInvalidAckType     = errors.New("invalid acknowledgeable_type")
	ErrInvalidDate        = errors.New("date must be a valid YYYY-MM-DD")
	ErrWrongStatus        = errors.New("action not allowed in current acknowledgement status")
	ErrSignatureRequired  = errors.New("signature_data is required when signature_required=true")
)
