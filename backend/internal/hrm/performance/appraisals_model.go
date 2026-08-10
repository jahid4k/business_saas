// backend/internal/hrm/performance/appraisals_model.go
package performance

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// FormEngine is the slice of forms.Service that appraisals consume. Declared
// here so the service can be unit-tested against a stub; the concrete
// forms.Service satisfies it structurally, so main.go passes it directly.
//
// This package imports internal/platform/forms for its types, which is the
// correct dependency direction — platform is the lower layer and never
// references hrm. It mirrors internal/hrm/onboarding consuming
// internal/platform/checklists.
type FormEngine interface {
	Instantiate(ctx context.Context, orgID, templateRef string, subj forms.SubjectContext) (*forms.InstanceWithResponses, error)
	GetInstance(ctx context.Context, orgID, ref string) (*forms.InstanceWithResponses, error)
	ScoreInstance(ctx context.Context, orgID, ref string) (forms.Score, error)
}

// ── Appraisal cycles ─────────────────────────────────────────────────────────

type AppraisalCycleStatus string

const (
	AppraisalCycleDraft  AppraisalCycleStatus = "draft"
	AppraisalCycleActive AppraisalCycleStatus = "active"
	AppraisalCycleClosed AppraisalCycleStatus = "closed"
)

func (s AppraisalCycleStatus) IsValid() bool {
	switch s {
	case AppraisalCycleDraft, AppraisalCycleActive, AppraisalCycleClosed:
		return true
	}
	return false
}

// AppraisalCycle is one review round. Deliberately separate from GoalCycle —
// see migration 00082's header for why fusing them would produce one status
// CHECK with nine values, half illegal per row.
type AppraisalCycle struct {
	ID          string    `db:"id"           json:"id"`
	PublicID    string    `db:"public_id"    json:"public_id"`
	OrgID       string    `db:"org_id"       json:"org_id"`
	Name        string    `db:"name"         json:"name"`
	Description *string   `db:"description"  json:"description,omitempty"`
	PeriodStart time.Time `db:"period_start" json:"period_start"`
	PeriodEnd   time.Time `db:"period_end"   json:"period_end"`
	// GoalCycleID links this review to a Phase 5A goal cycle, so an
	// appraisal can report the appraisee's weighted goal attainment. Nullable
	// — an appraisal with no goal component is legitimate.
	GoalCycleID           *string              `db:"goal_cycle_id"            json:"goal_cycle_id,omitempty"`
	RatingScaleID         string               `db:"rating_scale_id"          json:"rating_scale_id"`
	SelfFormTemplateID    *string              `db:"self_form_template_id"    json:"self_form_template_id,omitempty"`
	ManagerFormTemplateID *string              `db:"manager_form_template_id" json:"manager_form_template_id,omitempty"`
	Status                AppraisalCycleStatus `db:"status"                   json:"status"`
	CreatedBy             string               `db:"created_by"               json:"created_by"`
	CreatedAt             time.Time            `db:"created_at"               json:"created_at"`
	UpdatedAt             time.Time            `db:"updated_at"               json:"updated_at"`
}

// ── Appraisal phases ─────────────────────────────────────────────────────────

type Phase string

const (
	PhaseDraft         Phase = "draft"
	PhaseSelfReview    Phase = "self_review"
	PhaseManagerReview Phase = "manager_review"
	PhaseCalibration   Phase = "calibration"
	PhasePublished     Phase = "published"
	PhaseAcknowledged  Phase = "acknowledged"
	PhaseCancelled     Phase = "cancelled"
)

func (p Phase) IsValid() bool {
	_, ok := allowedPhaseTransitions[p]
	return ok
}

// allowedPhaseTransitions is the legal phase graph, declared once.
//
// This is a DELIBERATE deviation from house style: every other state machine
// in this codebase uses inline `if x.Status != Expected` guards, and there is
// no other transition map anywhere. With six phases plus two backward sends,
// inline guards would be roughly fifteen scattered checks with no single
// place to read the legal graph, and an illegal transition would become a
// missing-check bug rather than a visible gap in a table.
//
// Phase 5C's PIP machine should follow this shape rather than the older one.
//
// Two properties this encodes that are easy to lose in scattered guards:
//   - published → acknowledged is the ONLY move out of published. Publication
//     is irreversible; there is no un-publish, matching the payslip pattern.
//   - manager_review and calibration can each send BACK one step, which is
//     what makes a rejected review recoverable without cancelling it.
var allowedPhaseTransitions = map[Phase][]Phase{
	PhaseDraft:         {PhaseSelfReview, PhaseCancelled},
	PhaseSelfReview:    {PhaseManagerReview, PhaseCancelled},
	PhaseManagerReview: {PhaseCalibration, PhaseSelfReview, PhaseCancelled},
	PhaseCalibration:   {PhasePublished, PhaseManagerReview, PhaseCancelled},
	PhasePublished:     {PhaseAcknowledged},
	PhaseAcknowledged:  {},
	PhaseCancelled:     {},
}

// CanTransition reports whether from → to is legal.
func CanTransition(from, to Phase) bool {
	for _, allowed := range allowedPhaseTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// IsTerminal reports whether a phase admits no further moves.
func (p Phase) IsTerminal() bool { return len(allowedPhaseTransitions[p]) == 0 }

// Appraisal is one employee's review within a cycle.
type Appraisal struct {
	ID         string `db:"id"         json:"id"`
	PublicID   string `db:"public_id"  json:"public_id"`
	OrgID      string `db:"org_id"     json:"org_id"`
	CycleID    string `db:"cycle_id"   json:"cycle_id"`
	EmployeeID string `db:"employee_id" json:"employee_id"`
	// Frozen at instantiation so a mid-cycle reorg cannot reassign a review
	// already underway.
	ManagerEmployeeIDSnapshot *string `db:"manager_employee_id_snapshot" json:"manager_employee_id_snapshot,omitempty"`

	SelfFormInstanceID    *string `db:"self_form_instance_id"    json:"self_form_instance_id,omitempty"`
	ManagerFormInstanceID *string `db:"manager_form_instance_id" json:"manager_form_instance_id,omitempty"`

	Phase Phase `db:"phase" json:"phase"`

	// FinalRatingLevelID is the structured FK Phase 7 and Phase 10 read;
	// Label and Value are the snapshot that survives a level being renamed or
	// re-valued. Both, deliberately — see migration 00086's header.
	FinalRatingLevelID *string          `db:"final_rating_level_id" json:"final_rating_level_id,omitempty"`
	FinalRatingLabel   *string          `db:"final_rating_label"    json:"final_rating_label,omitempty"`
	FinalRatingValue   *decimal.Decimal `db:"final_rating_value"    json:"final_rating_value,omitempty"`

	// Snapshotted at publish. Before then these are read live from the form
	// engine and from Phase 5A goals; after publish the record is immutable,
	// and an immutable record whose numbers are recomputed from mutable
	// sources is not actually immutable.
	SelfScore      *decimal.Decimal `db:"self_score"      json:"self_score,omitempty"`
	ManagerScore   *decimal.Decimal `db:"manager_score"   json:"manager_score,omitempty"`
	GoalAttainment *decimal.Decimal `db:"goal_attainment" json:"goal_attainment,omitempty"`

	PublishedAt    *time.Time `db:"published_at"    json:"published_at,omitempty"`
	PublishedBy    *string    `db:"published_by"    json:"published_by,omitempty"`
	AcknowledgedAt *time.Time `db:"acknowledged_at" json:"acknowledged_at,omitempty"`
	CancelledAt    *time.Time `db:"cancelled_at"    json:"cancelled_at,omitempty"`
	CancelReason   *string    `db:"cancel_reason"   json:"cancel_reason,omitempty"`

	CreatedBy string    `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// IsPublished reports whether the appraisal has reached an immutable phase.
func (a *Appraisal) IsPublished() bool {
	return a.Phase == PhasePublished || a.Phase == PhaseAcknowledged
}

// PhaseHistory is one append-only transition. Calibration rating changes live
// in these same rows: a calibration IS a transition, and splitting them would
// make "who changed this rating and why" a two-table question.
type PhaseHistory struct {
	ID                string    `db:"id"                    json:"id"`
	PublicID          string    `db:"public_id"             json:"public_id"`
	AppraisalID       string    `db:"appraisal_id"          json:"appraisal_id"`
	FromPhase         *string   `db:"from_phase"            json:"from_phase,omitempty"`
	ToPhase           string    `db:"to_phase"              json:"to_phase"`
	FromRatingLevelID *string   `db:"from_rating_level_id"  json:"from_rating_level_id,omitempty"`
	FromRatingLabel   *string   `db:"from_rating_label"     json:"from_rating_label,omitempty"`
	ToRatingLevelID   *string   `db:"to_rating_level_id"    json:"to_rating_level_id,omitempty"`
	ToRatingLabel     *string   `db:"to_rating_label"       json:"to_rating_label,omitempty"`
	Note              *string   `db:"note"                  json:"note,omitempty"`
	ChangedBy         *string   `db:"changed_by"            json:"changed_by,omitempty"`
	ChangedAt         time.Time `db:"changed_at"            json:"changed_at"`
}

// AppraisalDetail carries the live-computed figures alongside the record.
// Before publish these come from the form engine and Phase 5A goals; after
// publish they are read from the snapshot, so a published appraisal reports
// the same numbers forever.
type AppraisalDetail struct {
	*Appraisal
	SelfScore      *decimal.Decimal `json:"self_score,omitempty"`
	ManagerScore   *decimal.Decimal `json:"manager_score,omitempty"`
	GoalAttainment *decimal.Decimal `json:"goal_attainment,omitempty"`
	History        []*PhaseHistory  `json:"history"`
	// AllowedTransitions exposes the legal next phases so a client need not
	// re-implement the map.
	AllowedTransitions []Phase `json:"allowed_transitions"`
}

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateAppraisalCycleRequest struct {
	Name                  string  `json:"name"`
	Description           *string `json:"description"`
	PeriodStart           string  `json:"period_start"`
	PeriodEnd             string  `json:"period_end"`
	GoalCycleID           *string `json:"goal_cycle_id"`
	RatingScaleID         string  `json:"rating_scale_id"`
	SelfFormTemplateID    *string `json:"self_form_template_id"`
	ManagerFormTemplateID *string `json:"manager_form_template_id"`
}

type UpdateAppraisalCycleRequest struct {
	Name                  *string `json:"name"`
	Description           *string `json:"description"`
	PeriodStart           *string `json:"period_start"`
	PeriodEnd             *string `json:"period_end"`
	GoalCycleID           *string `json:"goal_cycle_id"`
	SelfFormTemplateID    *string `json:"self_form_template_id"`
	ManagerFormTemplateID *string `json:"manager_form_template_id"`
}

// InstantiateAppraisalRequest creates one employee's appraisal within a
// cycle, resolving and freezing their manager and instantiating the
// configured forms.
type InstantiateAppraisalRequest struct {
	EmployeeID string `json:"employee_id"`
}

// AdvancePhaseRequest moves an appraisal along the legal graph. The target is
// explicit rather than implied by an "advance" verb, because two phases can
// legally move backwards.
type AdvancePhaseRequest struct {
	ToPhase string  `json:"to_phase"`
	Note    *string `json:"note"`
}

// CalibrateRequest adjusts the final rating during calibration. Note is
// mandatory — an unexplained override of a manager's rating is exactly what
// the audit trail exists to prevent.
type CalibrateRequest struct {
	RatingLevelID string `json:"rating_level_id"`
	Note          string `json:"note"`
}

// SetRatingRequest records the manager's proposed final rating during
// manager_review, before any calibration.
type SetRatingRequest struct {
	RatingLevelID string `json:"rating_level_id"`
}

type AppraisalListFilter struct {
	CycleID    string
	EmployeeID string
	Phase      string
	Limit      int
	Offset     int

	Scope        authz.Scope
	CallerUserID string
}

func (f *AppraisalListFilter) Normalise() {
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

type AppraisalListResponse struct {
	Appraisals []*Appraisal `json:"appraisals"`
	Total      int          `json:"total"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
}

type AppraisalCycleListResponse struct {
	Cycles []*AppraisalCycle `json:"cycles"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

type AppraisalCycleListFilter struct {
	Status string
	Limit  int
	Offset int
}

func (f *AppraisalCycleListFilter) Normalise() {
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
	ErrAppraisalCycleNotFound = errors.New("appraisal cycle not found")
	ErrAppraisalNotFound      = errors.New("appraisal not found")
	ErrAppraisalExists        = errors.New("this employee already has an appraisal in this cycle")
	ErrAppraisalAccessDenied  = errors.New("you do not have access to this appraisal")

	ErrAppraisalCycleNameReq   = errors.New("name is required")
	ErrAppraisalCycleNameTaken = errors.New("an appraisal cycle with this name already exists in this organization")
	ErrAppraisalCycleNotActive = errors.New("the appraisal cycle is not active")
	ErrAppraisalCycleStatus    = errors.New("action not allowed in the cycle's current status")
	ErrRatingScaleRequired     = errors.New("rating_scale_id is required")
	ErrFormTemplateRequired    = errors.New("at least one of self_form_template_id or manager_form_template_id is required")

	// ErrIllegalPhaseTransition names the graph rather than the guard: the
	// legal moves live in allowedPhaseTransitions.
	ErrIllegalPhaseTransition  = errors.New("that phase transition is not allowed from the appraisal's current phase")
	ErrAppraisalPublished      = errors.New("this appraisal has been published and can no longer be changed")
	ErrCalibrationNoteReq      = errors.New("a calibration note is required — an unexplained rating override is not permitted")
	ErrNotInCalibration        = errors.New("ratings can only be calibrated while the appraisal is in the calibration phase")
	ErrRatingRequiredToPublish = errors.New("a final rating must be set before an appraisal can be published")
	ErrSelfReviewIncomplete    = errors.New("the self-review must be submitted before the appraisal can move to manager review")
	ErrManagerReviewIncomplete = errors.New("the manager review must be submitted before the appraisal can move to calibration")
	ErrRatingLevelWrongScale   = errors.New("that rating level does not belong to the cycle's rating scale")
)
