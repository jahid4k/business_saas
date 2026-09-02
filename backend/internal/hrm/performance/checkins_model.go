// backend/internal/hrm/performance/checkins_model.go
package performance

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Confidence is the check-in author's own read on whether the goal will land.
// Independent of the computed progress percent — a goal can be at 80% and
// off_track, or at 20% and on_track.
type Confidence string

const (
	ConfidenceOnTrack  Confidence = "on_track"
	ConfidenceAtRisk   Confidence = "at_risk"
	ConfidenceOffTrack Confidence = "off_track"
)

func (c Confidence) IsValid() bool {
	switch c {
	case ConfidenceOnTrack, ConfidenceAtRisk, ConfidenceOffTrack:
		return true
	}
	return false
}

// GoalCheckin is one append-only progress report. Creating a check-in is the
// ONLY path that mutates Goal.CurrentValue, which is what guarantees the
// history has no holes.
type GoalCheckin struct {
	ID            string          `db:"id"             json:"id"`
	PublicID      string          `db:"public_id"      json:"public_id"`
	GoalID        string          `db:"goal_id"        json:"goal_id"`
	PreviousValue decimal.Decimal `db:"previous_value" json:"previous_value"`
	CurrentValue  decimal.Decimal `db:"current_value"  json:"current_value"`
	// ProgressPercent is derived at write time inside the same transaction as
	// the value change and stored UNCLAMPED — overshoot and regression are
	// both real facts that history must not lose. Storing it does not violate
	// the computed-not-stored rule: this is an immutable historical value,
	// not a current aggregate that could drift from its inputs.
	ProgressPercent decimal.Decimal `db:"progress_percent" json:"progress_percent"`
	// StatusSnapshot freezes the goal's status as reported, so a later rename
	// cannot rewrite history — the from_stage_name precedent.
	StatusSnapshot string      `db:"status_snapshot" json:"status_snapshot"`
	Confidence     *Confidence `db:"confidence"      json:"confidence,omitempty"`
	Note           *string     `db:"note"            json:"note,omitempty"`
	CheckedInBy    *string     `db:"checked_in_by"   json:"checked_in_by,omitempty"`
	CheckedInAt    time.Time   `db:"checked_in_at"   json:"checked_in_at"`
}

type CreateCheckinRequest struct {
	CurrentValue decimal.Decimal `json:"current_value"`
	Confidence   *string         `json:"confidence"`
	Note         *string         `json:"note"`
}

type CheckinListResponse struct {
	Checkins []*GoalCheckin `json:"checkins"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
}

// CreateCheckinResult carries both writes the transaction performed, so the
// caller sees the advanced goal without a second round trip.
type CreateCheckinResult struct {
	Checkin *GoalCheckin `json:"checkin"`
	Goal    *GoalDetail  `json:"goal"`
}

var (
	ErrCheckinInvalidConfidence = errors.New("confidence must be one of: on_track, at_risk, off_track")
	// ErrCheckinGoalNotOpen blocks progress reports against a goal that is
	// completed or cancelled.
	ErrCheckinGoalNotOpen = errors.New("cannot check in on a completed or cancelled goal")
)
