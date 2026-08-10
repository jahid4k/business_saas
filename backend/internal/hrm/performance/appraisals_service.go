// backend/internal/hrm/performance/appraisals_service.go
package performance

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// AppraisalService is embedded into Service — see service.go.
type AppraisalService interface {
	// Cycles
	ListAppraisalCycles(ctx context.Context, orgID string, filter AppraisalCycleListFilter) (*AppraisalCycleListResponse, error)
	GetAppraisalCycle(ctx context.Context, orgID, ref string) (*AppraisalCycle, error)
	CreateAppraisalCycle(ctx context.Context, orgID, createdBy string, req CreateAppraisalCycleRequest) (*AppraisalCycle, error)
	UpdateAppraisalCycle(ctx context.Context, orgID, ref string, req UpdateAppraisalCycleRequest) (*AppraisalCycle, error)
	ActivateAppraisalCycle(ctx context.Context, orgID, ref string) (*AppraisalCycle, error)
	CloseAppraisalCycle(ctx context.Context, orgID, ref string) (*AppraisalCycle, error)

	// Appraisals
	ListAppraisals(ctx context.Context, orgID string, filter AppraisalListFilter) (*AppraisalListResponse, error)
	GetAppraisal(ctx context.Context, orgID, ref string, caller Caller) (*AppraisalDetail, error)
	// InstantiateAppraisal freezes the employee's manager and instantiates the
	// cycle's configured forms. This is the module-owned endpoint the form
	// engine deliberately does not expose generically.
	InstantiateAppraisal(ctx context.Context, orgID, cycleRef, createdBy string, req InstantiateAppraisalRequest) (*AppraisalDetail, error)
	AdvancePhase(ctx context.Context, orgID, ref string, caller Caller, req AdvancePhaseRequest) (*AppraisalDetail, error)
	SetRating(ctx context.Context, orgID, ref string, caller Caller, req SetRatingRequest) (*AppraisalDetail, error)
	Calibrate(ctx context.Context, orgID, ref string, caller Caller, req CalibrateRequest) (*AppraisalDetail, error)
	PublishAppraisal(ctx context.Context, orgID, ref string, caller Caller) (*AppraisalDetail, error)
}

// ── Cycles ───────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListAppraisalCycles(ctx context.Context, orgID string, filter AppraisalCycleListFilter) (*AppraisalCycleListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindAppraisalCycles(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListAppraisalCycles: %w", err)
	}
	if list == nil {
		list = []*AppraisalCycle{}
	}
	total, err := s.repo.CountAppraisalCycles(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListAppraisalCycles: count: %w", err)
	}
	return &AppraisalCycleListResponse{Cycles: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetAppraisalCycle(ctx context.Context, orgID, ref string) (*AppraisalCycle, error) {
	c, err := s.repo.FindAppraisalCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: GetAppraisalCycle: %w", err)
	}
	if c == nil {
		return nil, ErrAppraisalCycleNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateAppraisalCycle(ctx context.Context, orgID, createdBy string, req CreateAppraisalCycleRequest) (*AppraisalCycle, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrAppraisalCycleNameReq
	}
	if strings.TrimSpace(req.RatingScaleID) == "" {
		return nil, ErrRatingScaleRequired
	}
	// A cycle with neither form configured would instantiate appraisals with
	// nothing to fill in.
	if req.SelfFormTemplateID == nil && req.ManagerFormTemplateID == nil {
		return nil, ErrFormTemplateRequired
	}

	start, err := parseDate(&req.PeriodStart)
	if err != nil || start == nil {
		return nil, ErrCycleInvalidDate
	}
	end, err := parseDate(&req.PeriodEnd)
	if err != nil || end == nil {
		return nil, ErrCycleInvalidDate
	}
	if end.Before(*start) {
		return nil, ErrCyclePeriodInvalid
	}

	scale, err := s.repo.FindScaleByRef(ctx, orgID, strings.TrimSpace(req.RatingScaleID))
	if err != nil {
		return nil, fmt.Errorf("performance: CreateAppraisalCycle: scale: %w", err)
	}
	if scale == nil {
		return nil, ErrScaleNotFound
	}
	// A scale with no levels cannot express a rating, so a cycle using it
	// could never be published.
	levels, err := s.repo.FindLevels(ctx, orgID, scale.ID)
	if err != nil {
		return nil, fmt.Errorf("performance: CreateAppraisalCycle: levels: %w", err)
	}
	if len(levels) == 0 {
		return nil, ErrScaleNoLevels
	}

	taken, err := s.repo.AppraisalCycleNameExists(ctx, orgID, name, "")
	if err != nil {
		return nil, fmt.Errorf("performance: CreateAppraisalCycle: name check: %w", err)
	}
	if taken {
		return nil, ErrAppraisalCycleNameTaken
	}

	var goalCycleID *string
	if req.GoalCycleID != nil && strings.TrimSpace(*req.GoalCycleID) != "" {
		gc, err := s.repo.FindCycleByRef(ctx, orgID, strings.TrimSpace(*req.GoalCycleID))
		if err != nil {
			return nil, fmt.Errorf("performance: CreateAppraisalCycle: goal cycle: %w", err)
		}
		if gc == nil {
			return nil, ErrCycleNotFound
		}
		goalCycleID = &gc.ID
	}

	c := &AppraisalCycle{
		OrgID: orgID, Name: name, Description: nilIfBlank(req.Description),
		PeriodStart: *start, PeriodEnd: *end, GoalCycleID: goalCycleID,
		RatingScaleID:         scale.ID,
		SelfFormTemplateID:    nilIfBlank(req.SelfFormTemplateID),
		ManagerFormTemplateID: nilIfBlank(req.ManagerFormTemplateID),
		CreatedBy:             createdBy,
	}
	if err := s.repo.CreateAppraisalCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("performance: CreateAppraisalCycle: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) UpdateAppraisalCycle(ctx context.Context, orgID, ref string, req UpdateAppraisalCycleRequest) (*AppraisalCycle, error) {
	c, err := s.repo.FindAppraisalCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: UpdateAppraisalCycle: %w", err)
	}
	if c == nil {
		return nil, ErrAppraisalCycleNotFound
	}
	if c.Status == AppraisalCycleClosed {
		return nil, ErrAppraisalCycleStatus
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrAppraisalCycleNameReq
		}
		taken, err := s.repo.AppraisalCycleNameExists(ctx, orgID, name, c.ID)
		if err != nil {
			return nil, fmt.Errorf("performance: UpdateAppraisalCycle: name check: %w", err)
		}
		if taken {
			return nil, ErrAppraisalCycleNameTaken
		}
		c.Name = name
	}
	if req.Description != nil {
		c.Description = nilIfBlank(req.Description)
	}
	if req.PeriodStart != nil {
		d, err := parseDate(req.PeriodStart)
		if err != nil || d == nil {
			return nil, ErrCycleInvalidDate
		}
		c.PeriodStart = *d
	}
	if req.PeriodEnd != nil {
		d, err := parseDate(req.PeriodEnd)
		if err != nil || d == nil {
			return nil, ErrCycleInvalidDate
		}
		c.PeriodEnd = *d
	}
	if c.PeriodEnd.Before(c.PeriodStart) {
		return nil, ErrCyclePeriodInvalid
	}
	if req.GoalCycleID != nil {
		c.GoalCycleID = nilIfBlank(req.GoalCycleID)
	}
	if req.SelfFormTemplateID != nil {
		c.SelfFormTemplateID = nilIfBlank(req.SelfFormTemplateID)
	}
	if req.ManagerFormTemplateID != nil {
		c.ManagerFormTemplateID = nilIfBlank(req.ManagerFormTemplateID)
	}

	if err := s.repo.UpdateAppraisalCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("performance: UpdateAppraisalCycle: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) ActivateAppraisalCycle(ctx context.Context, orgID, ref string) (*AppraisalCycle, error) {
	c, err := s.GetAppraisalCycle(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if c.Status != AppraisalCycleDraft {
		return nil, ErrAppraisalCycleStatus
	}
	if err := s.repo.SetAppraisalCycleStatus(ctx, orgID, c.ID, AppraisalCycleActive); err != nil {
		return nil, fmt.Errorf("performance: ActivateAppraisalCycle: %w", err)
	}
	c.Status = AppraisalCycleActive
	return c, nil
}

func (s *serviceImpl) CloseAppraisalCycle(ctx context.Context, orgID, ref string) (*AppraisalCycle, error) {
	c, err := s.GetAppraisalCycle(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if c.Status != AppraisalCycleActive {
		return nil, ErrAppraisalCycleStatus
	}
	if err := s.repo.SetAppraisalCycleStatus(ctx, orgID, c.ID, AppraisalCycleClosed); err != nil {
		return nil, fmt.Errorf("performance: CloseAppraisalCycle: %w", err)
	}
	c.Status = AppraisalCycleClosed
	return c, nil
}

// ── Appraisals ───────────────────────────────────────────────────────────────

func (s *serviceImpl) ListAppraisals(ctx context.Context, orgID string, filter AppraisalListFilter) (*AppraisalListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindAppraisals(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListAppraisals: %w", err)
	}
	if list == nil {
		list = []*Appraisal{}
	}
	total, err := s.repo.CountAppraisals(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListAppraisals: count: %w", err)
	}
	return &AppraisalListResponse{Appraisals: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetAppraisal(ctx context.Context, orgID, ref string, caller Caller) (*AppraisalDetail, error) {
	a, err := s.loadAppraisal(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	return s.hydrate(ctx, orgID, a)
}

// loadAppraisal fetches an appraisal and gates it on the caller's scope tier.
// Draft-leakage is this module's named failure mode — an unpublished appraisal
// is the most sensitive employee record the system holds.
func (s *serviceImpl) loadAppraisal(ctx context.Context, orgID, ref string, caller Caller) (*Appraisal, error) {
	a, err := s.repo.FindAppraisalByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: load appraisal: %w", err)
	}
	if a == nil {
		return nil, ErrAppraisalNotFound
	}
	allowed, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, a.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("performance: load appraisal: %w", err)
	}
	if !allowed {
		return nil, ErrAppraisalAccessDenied
	}
	return a, nil
}

// hydrate attaches the figures and history a client needs.
//
// Before publish the scores are read LIVE from the form engine and Phase 5A
// goals. After publish they come from the snapshot on the row — a published
// appraisal must report the same numbers forever, and recomputing them from
// mutable sources would quietly break that.
func (s *serviceImpl) hydrate(ctx context.Context, orgID string, a *Appraisal) (*AppraisalDetail, error) {
	detail := &AppraisalDetail{
		Appraisal:          a,
		AllowedTransitions: allowedPhaseTransitions[a.Phase],
	}
	if detail.AllowedTransitions == nil {
		detail.AllowedTransitions = []Phase{}
	}

	history, err := s.repo.FindPhaseHistory(ctx, a.ID)
	if err != nil {
		return nil, fmt.Errorf("performance: hydrate: history: %w", err)
	}
	if history == nil {
		history = []*PhaseHistory{}
	}
	detail.History = history

	if a.IsPublished() {
		detail.SelfScore, detail.ManagerScore, detail.GoalAttainment = a.SelfScore, a.ManagerScore, a.GoalAttainment
		return detail, nil
	}

	self, manager, err := s.liveFormScores(ctx, orgID, a)
	if err != nil {
		return nil, err
	}
	attainment, err := s.goalAttainment(ctx, orgID, a)
	if err != nil {
		return nil, err
	}
	detail.SelfScore, detail.ManagerScore, detail.GoalAttainment = self, manager, attainment
	return detail, nil
}

// liveFormScores reads both form instances' scores from the engine. A form
// that has not been instantiated contributes nil rather than zero — "not
// configured" and "scored zero" must stay distinguishable.
func (s *serviceImpl) liveFormScores(ctx context.Context, orgID string, a *Appraisal) (*decimal.Decimal, *decimal.Decimal, error) {
	score := func(ref *string) (*decimal.Decimal, error) {
		if ref == nil || s.forms == nil {
			return nil, nil
		}
		sc, err := s.forms.ScoreInstance(ctx, orgID, *ref)
		if err != nil {
			return nil, fmt.Errorf("performance: form score: %w", err)
		}
		v := sc.Percent
		return &v, nil
	}
	self, err := score(a.SelfFormInstanceID)
	if err != nil {
		return nil, nil, err
	}
	manager, err := score(a.ManagerFormInstanceID)
	if err != nil {
		return nil, nil, err
	}
	return self, manager, nil
}

// goalAttainment is the appraisee's OWN weighted goal attainment for the
// cycle's linked goal cycle — Σ(weight × clamped progress) / Σ(weight).
//
// It reads the appraisee's own goals only. Phase 5A's decision that
// parent_goal_id is alignment-only, with no roll-up, is what makes this
// number stable: otherwise a subordinate back-dating a check-in would change
// an already-published appraisal's inputs.
//
// Returns nil when the cycle has no goal cycle linked, or the employee has no
// weighted goals — again, nil rather than zero, so "no goal component" reads
// differently from "scored nothing".
func (s *serviceImpl) goalAttainment(ctx context.Context, orgID string, a *Appraisal) (*decimal.Decimal, error) {
	cycle, err := s.repo.FindAppraisalCycleByRef(ctx, orgID, a.CycleID)
	if err != nil {
		return nil, fmt.Errorf("performance: goal attainment: cycle: %w", err)
	}
	if cycle == nil || cycle.GoalCycleID == nil {
		return nil, nil
	}

	goals, err := s.repo.FindGoals(ctx, orgID, GoalListFilter{
		CycleID: *cycle.GoalCycleID, EmployeeID: a.EmployeeID,
		Scope: authz.ScopeAll, Limit: MaxLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("performance: goal attainment: goals: %w", err)
	}

	weighted, totalWeight := decimal.Zero, decimal.Zero
	for _, g := range goals {
		if g.Weight == nil || g.Weight.IsZero() || g.Status == GoalStatusCancelled {
			continue
		}
		// ProgressPercent, not Raw: 5A clamps precisely so one overachieving
		// goal cannot push attainment past the top of the rating scale.
		weighted = weighted.Add(g.ProgressPercent().Mul(*g.Weight))
		totalWeight = totalWeight.Add(*g.Weight)
	}
	if totalWeight.IsZero() {
		return nil, nil
	}
	v := weighted.Div(totalWeight).Round(2)
	return &v, nil
}

func (s *serviceImpl) InstantiateAppraisal(ctx context.Context, orgID, cycleRef, createdBy string, req InstantiateAppraisalRequest) (*AppraisalDetail, error) {
	cycle, err := s.GetAppraisalCycle(ctx, orgID, cycleRef)
	if err != nil {
		return nil, err
	}
	if cycle.Status != AppraisalCycleActive {
		return nil, ErrAppraisalCycleNotActive
	}

	subject, err := s.repo.FindEmployeeSubject(ctx, orgID, strings.TrimSpace(req.EmployeeID))
	if err != nil {
		return nil, fmt.Errorf("performance: InstantiateAppraisal: %w", err)
	}
	if subject == nil {
		return nil, ErrEmployeeNotFound
	}

	existing, err := s.repo.FindAppraisalForEmployee(ctx, orgID, cycle.ID, subject.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("performance: InstantiateAppraisal: %w", err)
	}
	if existing != nil {
		return nil, ErrAppraisalExists
	}

	a := &Appraisal{
		OrgID: orgID, CycleID: cycle.ID, EmployeeID: subject.EmployeeID,
		// Frozen here, deliberately: a reorg mid-cycle must not reassign a
		// review already underway.
		ManagerEmployeeIDSnapshot: subject.ManagerEmployeeID,
		CreatedBy:                 createdBy,
	}

	// The self form is answered by the appraisee; the manager form by their
	// manager. This subject/respondent split is exactly why the form engine
	// keeps them as separate columns.
	if cycle.SelfFormTemplateID != nil {
		inst, err := s.instantiateForm(ctx, orgID, *cycle.SelfFormTemplateID, subject, subject.UserID, "self", createdBy)
		if err != nil {
			return nil, err
		}
		a.SelfFormInstanceID = inst
	}
	if cycle.ManagerFormTemplateID != nil {
		inst, err := s.instantiateForm(ctx, orgID, *cycle.ManagerFormTemplateID, subject, subject.ManagerUserID, "manager", createdBy)
		if err != nil {
			return nil, err
		}
		a.ManagerFormInstanceID = inst
	}

	if err := s.repo.CreateAppraisal(ctx, a); err != nil {
		return nil, fmt.Errorf("performance: InstantiateAppraisal: %w", err)
	}
	return s.hydrate(ctx, orgID, a)
}

func (s *serviceImpl) instantiateForm(ctx context.Context, orgID, templateID string, subject *EmployeeSubject, respondent *string, role, createdBy string) (*string, error) {
	if s.forms == nil {
		return nil, nil
	}
	inst, err := s.forms.Instantiate(ctx, orgID, templateID, forms.SubjectContext{
		SubjectType:  forms.SubjectEmployee,
		SubjectID:    subject.EmployeeID,
		SubjectLabel: subject.DisplayName,
		// A nil respondent is legitimate — an employee with no manager, or a
		// manager with no platform account. The form exists and can be
		// completed by a forms.manage holder on their behalf.
		RespondentUserID: respondent,
		RespondentRole:   role,
		CreatedBy:        createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("performance: instantiate %s form: %w", role, err)
	}
	return &inst.ID, nil
}

// AdvancePhase moves an appraisal along the legal graph declared in
// allowedPhaseTransitions, after checking the preconditions each specific
// move requires.
func (s *serviceImpl) AdvancePhase(ctx context.Context, orgID, ref string, caller Caller, req AdvancePhaseRequest) (*AppraisalDetail, error) {
	a, err := s.loadAppraisal(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}

	to := Phase(strings.TrimSpace(req.ToPhase))
	if !to.IsValid() {
		return nil, ErrIllegalPhaseTransition
	}
	// One lookup replaces what would otherwise be roughly fifteen scattered
	// inline guards.
	if !CanTransition(a.Phase, to) {
		if a.IsPublished() {
			return nil, ErrAppraisalPublished
		}
		return nil, ErrIllegalPhaseTransition
	}

	// Publication has its own entry point, because it snapshots figures.
	if to == PhasePublished {
		return s.PublishAppraisal(ctx, orgID, ref, caller)
	}

	if err := s.assertPhasePreconditions(ctx, orgID, a, to); err != nil {
		return nil, err
	}

	from := string(a.Phase)
	h := &PhaseHistory{AppraisalID: a.ID, FromPhase: &from, ToPhase: string(to), Note: nilIfBlank(req.Note)}
	if caller.UserID != "" {
		h.ChangedBy = &caller.UserID
	}
	a.Phase = to

	if err := s.repo.AdvanceAppraisalPhase(ctx, orgID, a, h); err != nil {
		return nil, fmt.Errorf("performance: AdvancePhase: %w", err)
	}
	return s.hydrate(ctx, orgID, a)
}

// assertPhasePreconditions enforces the requirements a given move carries
// beyond mere legality — the transition map says what CAN happen, these say
// when it is ready to.
func (s *serviceImpl) assertPhasePreconditions(ctx context.Context, orgID string, a *Appraisal, to Phase) error {
	switch to {
	case PhaseManagerReview:
		// Only when moving FORWARD from self_review; a send-back from
		// calibration must not re-demand the self form.
		if a.Phase == PhaseSelfReview {
			ok, err := s.formSubmitted(ctx, orgID, a.SelfFormInstanceID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrSelfReviewIncomplete
			}
		}
	case PhaseCalibration:
		ok, err := s.formSubmitted(ctx, orgID, a.ManagerFormInstanceID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrManagerReviewIncomplete
		}
	}
	return nil
}

// formSubmitted reports whether a form instance has been submitted. A nil
// reference counts as satisfied: a cycle that configured no such form cannot
// be blocked on it.
func (s *serviceImpl) formSubmitted(ctx context.Context, orgID string, ref *string) (bool, error) {
	if ref == nil || s.forms == nil {
		return true, nil
	}
	inst, err := s.forms.GetInstance(ctx, orgID, *ref)
	if err != nil {
		return false, fmt.Errorf("performance: read form instance: %w", err)
	}
	return inst.SubmittedAt != nil, nil
}

// resolveLevel loads a rating level and checks it belongs to the cycle's
// scale — a rating from another scale would be numerically meaningless.
func (s *serviceImpl) resolveLevel(ctx context.Context, orgID, cycleID, levelRef string) (*RatingLevel, error) {
	level, err := s.repo.FindLevelByRef(ctx, orgID, strings.TrimSpace(levelRef))
	if err != nil {
		return nil, fmt.Errorf("performance: resolve rating level: %w", err)
	}
	if level == nil {
		return nil, ErrLevelNotFound
	}
	cycle, err := s.repo.FindAppraisalCycleByRef(ctx, orgID, cycleID)
	if err != nil {
		return nil, fmt.Errorf("performance: resolve rating level: cycle: %w", err)
	}
	if cycle == nil {
		return nil, ErrAppraisalCycleNotFound
	}
	if level.ScaleID != cycle.RatingScaleID {
		return nil, ErrRatingLevelWrongScale
	}
	return level, nil
}

// applyRating writes the FK plus the label/value snapshot together. All three
// move as a unit — a FK without its snapshot would lose the historical
// reading if the level were later renamed.
func applyRating(a *Appraisal, level *RatingLevel) {
	a.FinalRatingLevelID = &level.ID
	label := level.Label
	value := level.Value
	a.FinalRatingLabel = &label
	a.FinalRatingValue = &value
}

func (s *serviceImpl) SetRating(ctx context.Context, orgID, ref string, caller Caller, req SetRatingRequest) (*AppraisalDetail, error) {
	a, err := s.loadAppraisal(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if a.IsPublished() {
		return nil, ErrAppraisalPublished
	}
	if a.Phase != PhaseManagerReview {
		return nil, ErrIllegalPhaseTransition
	}

	level, err := s.resolveLevel(ctx, orgID, a.CycleID, req.RatingLevelID)
	if err != nil {
		return nil, err
	}

	h := s.ratingHistory(a, level, caller, nil)
	applyRating(a, level)
	if err := s.repo.SetAppraisalRating(ctx, orgID, a, h); err != nil {
		return nil, fmt.Errorf("performance: SetRating: %w", err)
	}
	return s.hydrate(ctx, orgID, a)
}

// Calibrate overrides the manager's rating. The note is mandatory: an
// unexplained override of someone else's assessment is exactly what the audit
// trail exists to prevent, and the build plan requires it.
func (s *serviceImpl) Calibrate(ctx context.Context, orgID, ref string, caller Caller, req CalibrateRequest) (*AppraisalDetail, error) {
	a, err := s.loadAppraisal(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if a.IsPublished() {
		return nil, ErrAppraisalPublished
	}
	if a.Phase != PhaseCalibration {
		return nil, ErrNotInCalibration
	}
	if strings.TrimSpace(req.Note) == "" {
		return nil, ErrCalibrationNoteReq
	}

	level, err := s.resolveLevel(ctx, orgID, a.CycleID, req.RatingLevelID)
	if err != nil {
		return nil, err
	}

	note := strings.TrimSpace(req.Note)
	h := s.ratingHistory(a, level, caller, &note)
	applyRating(a, level)
	if err := s.repo.SetAppraisalRating(ctx, orgID, a, h); err != nil {
		return nil, fmt.Errorf("performance: Calibrate: %w", err)
	}
	return s.hydrate(ctx, orgID, a)
}

// ratingHistory builds the audit row for a rating change, capturing the
// BEFORE values from the appraisal as it currently stands. It must be called
// before applyRating overwrites them.
func (s *serviceImpl) ratingHistory(a *Appraisal, level *RatingLevel, caller Caller, note *string) *PhaseHistory {
	phase := string(a.Phase)
	h := &PhaseHistory{
		AppraisalID: a.ID,
		FromPhase:   &phase,
		// The phase does not change; recording it on both sides keeps the
		// history readable as a single ordered narrative.
		ToPhase:           phase,
		FromRatingLevelID: a.FinalRatingLevelID,
		FromRatingLabel:   a.FinalRatingLabel,
		ToRatingLevelID:   &level.ID,
		Note:              note,
	}
	label := level.Label
	h.ToRatingLabel = &label
	if caller.UserID != "" {
		h.ChangedBy = &caller.UserID
	}
	return h
}

// PublishAppraisal freezes the appraisal. This is the irreversible step: the
// transition map admits no move out of published except acknowledged.
func (s *serviceImpl) PublishAppraisal(ctx context.Context, orgID, ref string, caller Caller) (*AppraisalDetail, error) {
	a, err := s.loadAppraisal(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if a.IsPublished() {
		return nil, ErrAppraisalPublished
	}
	if !CanTransition(a.Phase, PhasePublished) {
		return nil, ErrIllegalPhaseTransition
	}
	// Publishing without a rating would produce a record Phase 7's merit
	// matrix and Phase 10's 9-box both read as empty.
	if a.FinalRatingLevelID == nil {
		return nil, ErrRatingRequiredToPublish
	}

	// Snapshot the live figures NOW. After this the appraisal is immutable,
	// and an immutable record whose numbers are recomputed from mutable
	// sources is not actually immutable.
	self, manager, err := s.liveFormScores(ctx, orgID, a)
	if err != nil {
		return nil, err
	}
	attainment, err := s.goalAttainment(ctx, orgID, a)
	if err != nil {
		return nil, err
	}
	a.SelfScore, a.ManagerScore, a.GoalAttainment = self, manager, attainment

	from := string(a.Phase)
	h := &PhaseHistory{AppraisalID: a.ID, FromPhase: &from, ToPhase: string(PhasePublished)}
	if caller.UserID != "" {
		h.ChangedBy = &caller.UserID
	}

	if err := s.repo.PublishAppraisal(ctx, orgID, a, h); err != nil {
		return nil, fmt.Errorf("performance: PublishAppraisal: %w", err)
	}
	a.Phase = PhasePublished
	return s.hydrate(ctx, orgID, a)
}
