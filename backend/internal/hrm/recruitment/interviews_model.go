// backend/internal/hrm/recruitment/interviews_model.go
package recruitment

import (
	"errors"
	"time"
)

type InterviewMode string

const (
	InterviewModeOnsite InterviewMode = "onsite"
	InterviewModePhone  InterviewMode = "phone"
	InterviewModeVideo  InterviewMode = "video"
)

func (m InterviewMode) IsValid() bool {
	switch m {
	case InterviewModeOnsite, InterviewModePhone, InterviewModeVideo:
		return true
	}
	return false
}

type InterviewStatus string

const (
	InterviewStatusScheduled InterviewStatus = "scheduled"
	InterviewStatusCompleted InterviewStatus = "completed"
	InterviewStatusCancelled InterviewStatus = "cancelled"
	InterviewStatusNoShow    InterviewStatus = "no_show"
)

func (s InterviewStatus) IsValid() bool {
	switch s {
	case InterviewStatusScheduled, InterviewStatusCompleted, InterviewStatusCancelled, InterviewStatusNoShow:
		return true
	}
	return false
}

type InterviewOutcome string

const (
	InterviewOutcomeAdvance InterviewOutcome = "advance"
	InterviewOutcomeReject  InterviewOutcome = "reject"
	InterviewOutcomeHold    InterviewOutcome = "hold"
)

func (o InterviewOutcome) IsValid() bool {
	switch o {
	case InterviewOutcomeAdvance, InterviewOutcomeReject, InterviewOutcomeHold:
		return true
	}
	return false
}

// Interview is one scheduled interview round for an application. Outcome is
// a recommendation signal only — it does NOT auto-move the application's
// pipeline stage (see migration 00080's header); the recruiter still calls
// MoveApplication explicitly.
type Interview struct {
	ID              string            `db:"id"                json:"id"`
	PublicID        string            `db:"public_id"         json:"public_id"`
	OrgID           string            `db:"org_id"            json:"org_id"`
	ApplicationID   string            `db:"application_id"    json:"application_id"`
	ScheduledAt     time.Time         `db:"scheduled_at"      json:"scheduled_at"`
	DurationMinutes int               `db:"duration_minutes"  json:"duration_minutes"`
	Mode            InterviewMode     `db:"mode"              json:"mode"`
	Location        *string           `db:"location"          json:"location,omitempty"`
	MeetingURL      *string           `db:"meeting_url"       json:"meeting_url,omitempty"`
	Status          InterviewStatus   `db:"status"            json:"status"`
	Outcome         *InterviewOutcome `db:"outcome"           json:"outcome,omitempty"`
	Notes           *string           `db:"notes"             json:"notes,omitempty"`
	CreatedBy       string            `db:"created_by"        json:"created_by"`
	CreatedAt       time.Time         `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time         `db:"updated_at"        json:"updated_at"`

	Panelists []*Panelist `db:"-" json:"panelists,omitempty"`
}

// Panelist is one employee assigned to an interview panel.
type Panelist struct {
	ID           string    `db:"id"             json:"id"`
	PublicID     string    `db:"public_id"      json:"public_id"`
	InterviewID  string    `db:"interview_id"   json:"interview_id"`
	EmployeeID   string    `db:"employee_id"    json:"employee_id"`
	PanelistRole *string   `db:"panelist_role"  json:"panelist_role,omitempty"`
	IsLead       bool      `db:"is_lead"        json:"is_lead"`
	CreatedAt    time.Time `db:"created_at"     json:"created_at"`
}

type CreateInterviewRequest struct {
	ScheduledAt     string  `json:"scheduled_at"` // RFC 3339
	DurationMinutes *int    `json:"duration_minutes"`
	Mode            *string `json:"mode"`
	Location        *string `json:"location"`
	MeetingURL      *string `json:"meeting_url"`
	Notes           *string `json:"notes"`
}

type UpdateInterviewRequest struct {
	ScheduledAt     *string `json:"scheduled_at"`
	DurationMinutes *int    `json:"duration_minutes"`
	Mode            *string `json:"mode"`
	Location        *string `json:"location"`
	MeetingURL      *string `json:"meeting_url"`
	Status          *string `json:"status"`
	Outcome         *string `json:"outcome"`
	Notes           *string `json:"notes"`
}

type AddPanelistRequest struct {
	EmployeeID   string  `json:"employee_id"`
	PanelistRole *string `json:"panelist_role"`
	IsLead       *bool   `json:"is_lead"`
}

var (
	ErrInterviewNotFound       = errors.New("interview not found")
	ErrInvalidInterviewMode    = errors.New("mode must be one of: onsite, phone, video")
	ErrInvalidInterviewStatus  = errors.New("status must be one of: scheduled, completed, cancelled, no_show")
	ErrInvalidInterviewOutcome = errors.New("outcome must be one of: advance, reject, hold")
	ErrScheduledAtRequired     = errors.New("scheduled_at is required")
	ErrInvalidScheduledAt      = errors.New("scheduled_at must be a valid RFC 3339 timestamp")
	ErrPanelistEmployeeID      = errors.New("employee_id is required")
	ErrPanelistNotFound        = errors.New("panelist not found")
	ErrPanelistAlreadyOnPanel  = errors.New("this employee is already on the interview panel")
)
