// backend/internal/hrm/recruitment/applications_model.go
package recruitment

import (
	"errors"
	"time"
)

type ApplicationStatus string

const (
	ApplicationStatusActive    ApplicationStatus = "active"
	ApplicationStatusHired     ApplicationStatus = "hired"
	ApplicationStatusRejected  ApplicationStatus = "rejected"
	ApplicationStatusWithdrawn ApplicationStatus = "withdrawn"
)

// Application is a candidate applying to a posting. Stage lives HERE, not
// on the candidate — a candidate may have many applications, each at its
// own point in its own pipeline.
type Application struct {
	ID                  string            `db:"id"                     json:"id"`
	PublicID            string            `db:"public_id"              json:"public_id"`
	OrgID               string            `db:"org_id"                 json:"org_id"`
	CandidateID         string            `db:"candidate_id"           json:"candidate_id"`
	PostingID           string            `db:"posting_id"             json:"posting_id"`
	PipelineID          string            `db:"pipeline_id"            json:"pipeline_id"`
	StageID             string            `db:"stage_id"               json:"stage_id"`
	Status              ApplicationStatus `db:"status"                 json:"status"`
	RejectionReason     *string           `db:"rejection_reason"       json:"rejection_reason,omitempty"`
	RejectedAt          *time.Time        `db:"rejected_at"            json:"rejected_at,omitempty"`
	WithdrawnAt         *time.Time        `db:"withdrawn_at"           json:"withdrawn_at,omitempty"`
	HiredAt             *time.Time        `db:"hired_at"               json:"hired_at,omitempty"`
	ConvertedEmployeeID *string           `db:"converted_employee_id"  json:"converted_employee_id,omitempty"`
	CoverLetter         *string           `db:"cover_letter"           json:"cover_letter,omitempty"`
	Source              *string           `db:"source"                 json:"source,omitempty"`
	AppliedAt           time.Time         `db:"applied_at"             json:"applied_at"`
	// Nullable by design — see migration 00078's header. A public
	// application (a later phase) has no authenticated actor to attribute.
	CreatedBy *string   `db:"created_by" json:"created_by,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ApplicationStageHistory is an append-only pipeline movement record. It is
// the table crm_deals never got — see migration 00078's header for why that
// matters.
type ApplicationStageHistory struct {
	ID                     string    `db:"id"                          json:"id"`
	ApplicationID          string    `db:"application_id"              json:"application_id"`
	FromStageID            *string   `db:"from_stage_id"               json:"from_stage_id,omitempty"`
	ToStageID              *string   `db:"to_stage_id"                 json:"to_stage_id,omitempty"`
	FromStageName          *string   `db:"from_stage_name"             json:"from_stage_name,omitempty"`
	ToStageName            string    `db:"to_stage_name"               json:"to_stage_name"`
	MovedBy                *string   `db:"moved_by"                    json:"moved_by,omitempty"`
	MovedAt                time.Time `db:"moved_at"                    json:"moved_at"`
	SecondsInPreviousStage *int64    `db:"seconds_in_previous_stage"   json:"seconds_in_previous_stage,omitempty"`
	Note                   *string   `db:"note"                        json:"note,omitempty"`
}

type CreateApplicationRequest struct {
	CandidateID string  `json:"candidate_id"`
	PostingID   string  `json:"posting_id"`
	CoverLetter *string `json:"cover_letter"`
	Source      *string `json:"source"`
}

type MoveApplicationRequest struct {
	StageID string  `json:"stage_id"`
	Note    *string `json:"note"`
}

type RejectApplicationRequest struct {
	Reason string `json:"reason"`
}

type ApplicationListFilter struct {
	CandidateID string
	PostingID   string
	Status      string
	Limit       int
	Offset      int
}

func (f *ApplicationListFilter) Normalise() {
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

type ApplicationListResponse struct {
	Applications []*Application `json:"applications"`
	Total        int            `json:"total"`
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
}

var (
	ErrApplicationNotFound  = errors.New("application not found")
	ErrDuplicateApplication = errors.New("this candidate already has an active application for this posting")
	ErrApplicationNotActive = errors.New("action not allowed — application is not active")
	ErrRejectReasonRequired = errors.New("reason is required to reject an application")
	ErrCandidateIDRequired  = errors.New("candidate_id is required")
	ErrPostingIDRequired    = errors.New("posting_id is required")
)
