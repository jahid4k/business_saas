// backend/internal/hrm/recruitment/scorecards_model.go
package recruitment

import (
	"errors"
	"time"
)

type ScorecardRecommendation string

const (
	RecommendationStrongHire   ScorecardRecommendation = "strong_hire"
	RecommendationHire         ScorecardRecommendation = "hire"
	RecommendationNoHire       ScorecardRecommendation = "no_hire"
	RecommendationStrongNoHire ScorecardRecommendation = "strong_no_hire"
)

func (r ScorecardRecommendation) IsValid() bool {
	switch r {
	case RecommendationStrongHire, RecommendationHire, RecommendationNoHire, RecommendationStrongNoHire:
		return true
	}
	return false
}

// Scorecard is a fixed-shape per-panelist interview score — deliberately
// not a form engine; see migration 00080's header. SubmittedAt is the only
// status field: NULL = draft (visible only to the submitting panelist),
// non-nil = immutable and visible to every panelist on the interview.
type Scorecard struct {
	ID                 string                   `db:"id"                    json:"id"`
	PublicID           string                   `db:"public_id"             json:"public_id"`
	InterviewID        string                   `db:"interview_id"          json:"interview_id"`
	PanelistEmployeeID string                   `db:"panelist_employee_id"  json:"panelist_employee_id"`
	OverallRating      *int                     `db:"overall_rating"        json:"overall_rating,omitempty"`
	TechnicalScore     *int                     `db:"technical_score"       json:"technical_score,omitempty"`
	CommunicationScore *int                     `db:"communication_score"   json:"communication_score,omitempty"`
	CultureFitScore    *int                     `db:"culture_fit_score"     json:"culture_fit_score,omitempty"`
	Recommendation     *ScorecardRecommendation `db:"recommendation"        json:"recommendation,omitempty"`
	Strengths          *string                  `db:"strengths"             json:"strengths,omitempty"`
	Concerns           *string                  `db:"concerns"              json:"concerns,omitempty"`
	SubmittedAt        *time.Time               `db:"submitted_at"          json:"submitted_at,omitempty"`
	CreatedAt          time.Time                `db:"created_at"            json:"created_at"`
	UpdatedAt          time.Time                `db:"updated_at"            json:"updated_at"`
}

// UpsertScorecardRequest is the body for POST .../interviews/:id/scorecard —
// creates or updates the caller's own draft. Rejected once submitted.
type UpsertScorecardRequest struct {
	OverallRating      *int    `json:"overall_rating"`
	TechnicalScore     *int    `json:"technical_score"`
	CommunicationScore *int    `json:"communication_score"`
	CultureFitScore    *int    `json:"culture_fit_score"`
	Recommendation     *string `json:"recommendation"`
	Strengths          *string `json:"strengths"`
	Concerns           *string `json:"concerns"`
}

var (
	ErrScorecardNotFound         = errors.New("scorecard not found")
	ErrScorecardAlreadySubmitted = errors.New("this scorecard has already been submitted and cannot be changed")
	ErrNotAPanelist              = errors.New("only assigned panelists may submit a scorecard for this interview")
	ErrInvalidScoreRange         = errors.New("scores must be between 1 and 5")
	ErrInvalidRecommendation     = errors.New("recommendation must be one of: strong_hire, hire, no_hire, strong_no_hire")
	ErrCallerHasNoEmployeeRecord = errors.New("caller has no employee record in this organization")
)
