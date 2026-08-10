// backend/internal/hrm/performance/goals_service.go
package performance

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// GoalService is embedded into Service — see service.go.
type GoalService interface {
	ListGoals(ctx context.Context, orgID string, filter GoalListFilter) (*GoalListResponse, error)
	GetGoal(ctx context.Context, orgID, ref string, caller Caller) (*GoalDetail, error)
	CreateGoal(ctx context.Context, orgID string, caller Caller, req CreateGoalRequest) (*Goal, error)
	UpdateGoal(ctx context.Context, orgID, ref string, caller Caller, req UpdateGoalRequest) (*Goal, error)
	DeleteGoal(ctx context.Context, orgID, ref string, caller Caller) error
	SubmitGoal(ctx context.Context, orgID, ref string, caller Caller) (*Goal, error)
	CompleteGoal(ctx context.Context, orgID, ref string, caller Caller, req CompleteGoalRequest) (*Goal, error)
	CancelGoal(ctx context.Context, orgID, ref string, caller Caller, req CancelGoalRequest) (*Goal, error)
}

func (s *serviceImpl) ListGoals(ctx context.Context, orgID string, filter GoalListFilter) (*GoalListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindGoals(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListGoals: %w", err)
	}
	total, err := s.repo.CountGoals(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListGoals: count: %w", err)
	}
	// Progress is attached per row here rather than stored, and the parent
	// reference is deliberately NOT hydrated — see GoalDetail.Parent, which
	// keeps that disclosure surface to a single endpoint.
	items := make([]*GoalListItem, 0, len(list))
	for _, g := range list {
		items = append(items, &GoalListItem{
			Goal:               g,
			ProgressPercent:    g.ProgressPercent(),
			RawProgressPercent: g.RawProgressPercent(),
		})
	}
	return &GoalListResponse{Goals: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetGoal(ctx context.Context, orgID, ref string, caller Caller) (*GoalDetail, error) {
	g, err := s.repo.FindGoalByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: GetGoal: %w", err)
	}
	if g == nil {
		return nil, ErrGoalNotFound
	}
	if err := s.authorizeGoalAccess(ctx, orgID, caller, g); err != nil {
		return nil, err
	}

	detail := &GoalDetail{
		Goal:               g,
		ProgressPercent:    g.ProgressPercent(),
		RawProgressPercent: g.RawProgressPercent(),
	}

	// The parent is returned as a title-only GoalRef regardless of whether
	// the caller could view the parent's owner. Suppressing it would render
	// "no alignment", which is false, and would make a fact about the org
	// viewer-dependent — this codebase's scope layer is a row filter, never a
	// field filter.
	if g.ParentGoalID != nil {
		parent, err := s.repo.FindGoalRef(ctx, orgID, *g.ParentGoalID)
		if err != nil {
			return nil, fmt.Errorf("performance: GetGoal: parent: %w", err)
		}
		detail.Parent = parent
	}

	// Aggregate across ALL direct children regardless of caller scope: it is
	// a count and a mean, not a row disclosure, and making it scope-dependent
	// would show the same goal a different completion figure per viewer.
	children, err := s.repo.FindChildGoals(ctx, orgID, g.ID)
	if err != nil {
		return nil, fmt.Errorf("performance: GetGoal: children: %w", err)
	}
	detail.ChildrenCount = len(children)
	if len(children) > 0 {
		sum := decimal.Zero
		for _, c := range children {
			sum = sum.Add(c.ProgressPercent())
		}
		mean := sum.Div(decimal.NewFromInt(int64(len(children)))).Round(2)
		detail.ChildrenProgress = &mean
	}
	return detail, nil
}

// validateGoalShape applies the rules shared by create and update. It runs
// against the already-merged goal so update sees the post-merge combination
// rather than only the fields that changed.
func validateGoalShape(g *Goal) error {
	if strings.TrimSpace(g.Title) == "" {
		return ErrGoalTitleRequired
	}
	if !g.GoalLevel.IsValid() {
		return ErrGoalInvalidLevel
	}
	if !g.MeasurementType.IsValid() {
		return ErrGoalInvalidMeasure
	}
	if !g.Direction.IsValid() {
		return ErrGoalInvalidDir
	}
	if g.Weight != nil && (g.Weight.IsNegative() || g.Weight.GreaterThan(decimal.NewFromInt(100))) {
		return ErrGoalInvalidWeight
	}
	if g.DueDate != nil && g.StartDate != nil && g.DueDate.Before(*g.StartDate) {
		return ErrGoalDatesInvalid
	}

	// Boolean goals have no span to measure, so the start/target rules below
	// do not apply to them.
	if g.MeasurementType == MeasurementBoolean {
		return nil
	}
	if g.TargetValue.Equal(g.StartValue) {
		return ErrGoalTargetEqualsStart
	}
	// direction is validated here and used nowhere in the progress
	// arithmetic — see Goal.RawProgressPercent.
	if g.Direction == DirectionIncrease && g.TargetValue.LessThan(g.StartValue) {
		return ErrGoalDirectionMismatch
	}
	if g.Direction == DirectionDecrease && g.TargetValue.GreaterThan(g.StartValue) {
		return ErrGoalDirectionMismatch
	}
	return nil
}

// requireWritableCycle loads the goal's cycle and rejects definition writes
// once it is locked or closed. Check-ins deliberately do NOT go through here —
// locked freezes definitions while progress keeps landing.
func (s *serviceImpl) requireWritableCycle(ctx context.Context, orgID, cycleID string) (*GoalCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, cycleID)
	if err != nil {
		return nil, fmt.Errorf("performance: load cycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	if c.Status != CycleStatusActive {
		return nil, ErrCycleNotActive
	}
	return c, nil
}

// resolveParent validates a proposed alignment target: it must exist in the
// org, and the edge must not close a loop.
func (s *serviceImpl) resolveParent(ctx context.Context, orgID, goalID string, parentRef *string) (*string, error) {
	if parentRef == nil || strings.TrimSpace(*parentRef) == "" {
		return nil, nil
	}
	parent, err := s.repo.FindGoalByRef(ctx, orgID, strings.TrimSpace(*parentRef))
	if err != nil {
		return nil, fmt.Errorf("performance: resolve parent: %w", err)
	}
	if parent == nil {
		return nil, ErrGoalNotFound
	}
	// On create goalID is empty and no cycle is possible; on update the goal
	// must not become its own ancestor.
	if goalID != "" {
		if parent.ID == goalID {
			return nil, ErrGoalAlignmentCycle
		}
		cyclic, err := s.repo.WouldCreateAlignmentCycle(ctx, orgID, goalID, parent.ID)
		if err != nil {
			return nil, fmt.Errorf("performance: resolve parent: %w", err)
		}
		if cyclic {
			return nil, ErrGoalAlignmentCycle
		}
	}
	return &parent.ID, nil
}

func (s *serviceImpl) CreateGoal(ctx context.Context, orgID string, caller Caller, req CreateGoalRequest) (*Goal, error) {
	if strings.TrimSpace(req.CycleID) == "" {
		return nil, ErrGoalCycleRequired
	}
	cycle, err := s.requireWritableCycle(ctx, orgID, strings.TrimSpace(req.CycleID))
	if err != nil {
		return nil, err
	}

	// An omitted employee_id targets the caller's own record, which is what
	// makes self-service goal setting work for a member holding only set_own.
	targetEmployeeID := strings.TrimSpace(req.EmployeeID)
	if targetEmployeeID == "" {
		own, err := s.resolveCallerEmployeeID(ctx, orgID, caller)
		if err != nil {
			return nil, err
		}
		if own == "" {
			return nil, ErrCallerHasNoEmployee
		}
		targetEmployeeID = own
	} else {
		exists, err := s.repo.EmployeeExists(ctx, orgID, targetEmployeeID)
		if err != nil {
			return nil, fmt.Errorf("performance: CreateGoal: employee check: %w", err)
		}
		if !exists {
			return nil, ErrEmployeeNotFound
		}
	}

	if err := s.authorizeWrite(ctx, orgID, caller, targetEmployeeID); err != nil {
		return nil, err
	}

	parentID, err := s.resolveParent(ctx, orgID, "", req.ParentGoalID)
	if err != nil {
		return nil, err
	}

	g := &Goal{
		OrgID: orgID, CycleID: cycle.ID, EmployeeID: targetEmployeeID, ParentGoalID: parentID,
		Title:           strings.TrimSpace(req.Title),
		Description:     nilIfBlank(req.Description),
		GoalLevel:       GoalLevelIndividual,
		Category:        nilIfBlank(req.Category),
		MeasurementType: MeasurementPercentage,
		Direction:       DirectionIncrease,
		StartValue:      decimal.Zero,
		TargetValue:     decimal.NewFromInt(100),
		CurrentValue:    decimal.Zero,
		Unit:            nilIfBlank(req.Unit),
		CurrencyCode:    nilIfBlank(req.CurrencyCode),
		Weight:          req.Weight,
		CreatedBy:       caller.UserID,
	}
	if req.GoalLevel != nil && strings.TrimSpace(*req.GoalLevel) != "" {
		g.GoalLevel = GoalLevel(strings.TrimSpace(*req.GoalLevel))
	}
	if req.MeasurementType != nil && strings.TrimSpace(*req.MeasurementType) != "" {
		g.MeasurementType = MeasurementType(strings.TrimSpace(*req.MeasurementType))
	}
	if req.Direction != nil && strings.TrimSpace(*req.Direction) != "" {
		g.Direction = Direction(strings.TrimSpace(*req.Direction))
	}
	if req.StartValue != nil {
		g.StartValue = *req.StartValue
		g.CurrentValue = *req.StartValue
	}
	if req.TargetValue != nil {
		g.TargetValue = *req.TargetValue
	}
	if g.MeasurementType == MeasurementBoolean && req.TargetValue == nil {
		g.TargetValue = decimal.NewFromInt(1)
	}

	startDate, err := parseDate(req.StartDate)
	if err != nil {
		return nil, ErrGoalInvalidDate
	}
	dueDate, err := parseDate(req.DueDate)
	if err != nil {
		return nil, ErrGoalInvalidDate
	}
	g.StartDate, g.DueDate = startDate, dueDate

	if err := validateGoalShape(g); err != nil {
		return nil, err
	}

	// Cheap pre-check purely for the good error message. It is NOT the
	// enforcement point — CreateGoalGuarded re-checks under an employee-row
	// lock, because this read-then-write window loses to concurrent requests.
	if g.Weight != nil {
		existing, err := s.repo.SumGoalWeights(ctx, targetEmployeeID, cycle.ID, "")
		if err != nil {
			return nil, fmt.Errorf("performance: CreateGoal: weight pre-check: %w", err)
		}
		if existing.Add(*g.Weight).GreaterThan(cycle.WeightTarget) {
			return nil, ErrWeightExceedsCycleTarget
		}
	}

	if err := s.repo.CreateGoalGuarded(ctx, g, cycle.WeightTarget); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *serviceImpl) UpdateGoal(ctx context.Context, orgID, ref string, caller Caller, req UpdateGoalRequest) (*Goal, error) {
	g, err := s.repo.FindGoalByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: UpdateGoal: %w", err)
	}
	if g == nil {
		return nil, ErrGoalNotFound
	}
	if err := s.authorizeWrite(ctx, orgID, caller, g.EmployeeID); err != nil {
		return nil, err
	}
	if g.Status == GoalStatusCompleted || g.Status == GoalStatusCancelled {
		return nil, ErrGoalWrongStatus
	}
	cycle, err := s.requireWritableCycle(ctx, orgID, g.CycleID)
	if err != nil {
		return nil, err
	}

	if req.ParentGoalID != nil {
		parentID, err := s.resolveParent(ctx, orgID, g.ID, req.ParentGoalID)
		if err != nil {
			return nil, err
		}
		g.ParentGoalID = parentID
	}
	if req.Title != nil {
		g.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		g.Description = nilIfBlank(req.Description)
	}
	if req.GoalLevel != nil {
		g.GoalLevel = GoalLevel(strings.TrimSpace(*req.GoalLevel))
	}
	if req.Category != nil {
		g.Category = nilIfBlank(req.Category)
	}
	if req.MeasurementType != nil {
		g.MeasurementType = MeasurementType(strings.TrimSpace(*req.MeasurementType))
	}
	if req.Direction != nil {
		g.Direction = Direction(strings.TrimSpace(*req.Direction))
	}
	if req.StartValue != nil {
		g.StartValue = *req.StartValue
	}
	if req.TargetValue != nil {
		g.TargetValue = *req.TargetValue
	}
	if req.Unit != nil {
		g.Unit = nilIfBlank(req.Unit)
	}
	if req.CurrencyCode != nil {
		g.CurrencyCode = nilIfBlank(req.CurrencyCode)
	}
	if req.Weight != nil {
		g.Weight = req.Weight
	}
	if req.StartDate != nil {
		d, err := parseDate(req.StartDate)
		if err != nil {
			return nil, ErrGoalInvalidDate
		}
		g.StartDate = d
	}
	if req.DueDate != nil {
		d, err := parseDate(req.DueDate)
		if err != nil {
			return nil, ErrGoalInvalidDate
		}
		g.DueDate = d
	}

	// CurrentValue is deliberately untouched: progress moves only through a
	// check-in, which is what guarantees hrm_goal_checkins has no holes.

	if err := validateGoalShape(g); err != nil {
		return nil, err
	}

	if err := s.repo.UpdateGoalGuarded(ctx, g, cycle.WeightTarget); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *serviceImpl) DeleteGoal(ctx context.Context, orgID, ref string, caller Caller) error {
	g, err := s.repo.FindGoalByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("performance: DeleteGoal: %w", err)
	}
	if g == nil {
		return ErrGoalNotFound
	}
	if err := s.authorizeWrite(ctx, orgID, caller, g.EmployeeID); err != nil {
		return err
	}

	// Hard delete only when there is no history to lose. This is the real
	// guard; parent_goal_id's ON DELETE SET NULL is a backstop for the rows
	// that reach deletion, not the policy.
	checkins, err := s.repo.CountCheckinsForGoal(ctx, g.ID)
	if err != nil {
		return fmt.Errorf("performance: DeleteGoal: checkin count: %w", err)
	}
	children, err := s.repo.CountChildGoals(ctx, orgID, g.ID)
	if err != nil {
		return fmt.Errorf("performance: DeleteGoal: child count: %w", err)
	}
	if checkins > 0 || children > 0 {
		return ErrGoalHasHistory
	}

	if err := s.repo.DeleteGoal(ctx, orgID, g.ID); err != nil {
		return fmt.Errorf("performance: DeleteGoal: %w", err)
	}
	return nil
}

func (s *serviceImpl) SubmitGoal(ctx context.Context, orgID, ref string, caller Caller) (*Goal, error) {
	g, err := s.loadForStatusChange(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if g.Status != GoalStatusDraft {
		return nil, ErrGoalWrongStatus
	}
	if err := s.repo.SetGoalStatus(ctx, orgID, g.ID, GoalStatusActive, nil, nil); err != nil {
		return nil, fmt.Errorf("performance: SubmitGoal: %w", err)
	}
	g.Status = GoalStatusActive
	return g, nil
}

func (s *serviceImpl) CompleteGoal(ctx context.Context, orgID, ref string, caller Caller, req CompleteGoalRequest) (*Goal, error) {
	g, err := s.loadForStatusChange(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if g.Status != GoalStatusActive {
		return nil, ErrGoalWrongStatus
	}

	// Outcome is recorded, never inferred from progress — the "no implicit
	// state machine" rule this codebase applies to interview outcomes and
	// offer acceptance alike.
	var outcome *GoalOutcome
	if req.Outcome != nil && strings.TrimSpace(*req.Outcome) != "" {
		o := GoalOutcome(strings.TrimSpace(*req.Outcome))
		if !o.IsValid() {
			return nil, ErrGoalInvalidOutcome
		}
		outcome = &o
	}

	if err := s.repo.SetGoalStatus(ctx, orgID, g.ID, GoalStatusCompleted, outcome, nil); err != nil {
		return nil, fmt.Errorf("performance: CompleteGoal: %w", err)
	}
	g.Status = GoalStatusCompleted
	g.Outcome = outcome
	return g, nil
}

func (s *serviceImpl) CancelGoal(ctx context.Context, orgID, ref string, caller Caller, req CancelGoalRequest) (*Goal, error) {
	g, err := s.loadForStatusChange(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if g.Status == GoalStatusCancelled || g.Status == GoalStatusCompleted {
		return nil, ErrGoalWrongStatus
	}
	reason := nilIfBlank(&req.Reason)
	if err := s.repo.SetGoalStatus(ctx, orgID, g.ID, GoalStatusCancelled, nil, reason); err != nil {
		return nil, fmt.Errorf("performance: CancelGoal: %w", err)
	}
	g.Status = GoalStatusCancelled
	return g, nil
}

// loadForStatusChange fetches a goal and authorizes a write against it. Status
// transitions deliberately do not require an active cycle: cancelling or
// completing a goal must stay possible after the cycle locks, which is the
// whole point of locking definitions rather than the goals themselves.
func (s *serviceImpl) loadForStatusChange(ctx context.Context, orgID, ref string, caller Caller) (*Goal, error) {
	g, err := s.repo.FindGoalByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: load goal: %w", err)
	}
	if g == nil {
		return nil, ErrGoalNotFound
	}
	if err := s.authorizeWrite(ctx, orgID, caller, g.EmployeeID); err != nil {
		return nil, err
	}
	return g, nil
}
