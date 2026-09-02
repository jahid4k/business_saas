// backend/internal/hrm/performance/cycles_service.go
package performance

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// CycleService is embedded into Service — see service.go.
type CycleService interface {
	ListCycles(ctx context.Context, orgID string, filter CycleListFilter) (*CycleListResponse, error)
	GetCycle(ctx context.Context, orgID, ref string) (*GoalCycle, error)
	CreateCycle(ctx context.Context, orgID, createdBy string, req CreateCycleRequest) (*GoalCycle, error)
	UpdateCycle(ctx context.Context, orgID, ref string, req UpdateCycleRequest) (*GoalCycle, error)
	ActivateCycle(ctx context.Context, orgID, ref string) (*GoalCycle, error)
	// LockCycle freezes goal definitions. It is the ONLY place the
	// "weights must total exactly the target" rule is enforced — at write
	// time the rule is deliberately only "must not exceed", because requiring
	// equality would make creating the first goal impossible.
	LockCycle(ctx context.Context, orgID, ref, actorID string) (*GoalCycle, error)
	CloseCycle(ctx context.Context, orgID, ref string) (*GoalCycle, error)
	GetCycleWeightAudit(ctx context.Context, orgID, ref string) (*CycleWeightAudit, error)
}

func (s *serviceImpl) ListCycles(ctx context.Context, orgID string, filter CycleListFilter) (*CycleListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindCycles(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListCycles: %w", err)
	}
	if list == nil {
		list = []*GoalCycle{}
	}
	total, err := s.repo.CountCycles(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("performance: ListCycles: count: %w", err)
	}
	return &CycleListResponse{Cycles: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetCycle(ctx context.Context, orgID, ref string) (*GoalCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: GetCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateCycle(ctx context.Context, orgID, createdBy string, req CreateCycleRequest) (*GoalCycle, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrCycleNameRequired
	}
	if strings.TrimSpace(req.PeriodStart) == "" || strings.TrimSpace(req.PeriodEnd) == "" {
		return nil, ErrCycleDateRequired
	}
	start, err := parseDate(&req.PeriodStart)
	if err != nil {
		return nil, ErrCycleInvalidDate
	}
	end, err := parseDate(&req.PeriodEnd)
	if err != nil {
		return nil, ErrCycleInvalidDate
	}
	if end.Before(*start) {
		return nil, ErrCyclePeriodInvalid
	}

	weightTarget := decimal.NewFromInt(100)
	if req.WeightTarget != nil {
		if req.WeightTarget.LessThanOrEqual(decimal.Zero) {
			return nil, ErrInvalidWeightTarget
		}
		weightTarget = *req.WeightTarget
	}

	taken, err := s.repo.CycleNameExists(ctx, orgID, name, "")
	if err != nil {
		return nil, fmt.Errorf("performance: CreateCycle: name check: %w", err)
	}
	if taken {
		return nil, ErrCycleNameTaken
	}

	c := &GoalCycle{
		OrgID: orgID, Name: name, Description: nilIfBlank(req.Description),
		PeriodStart: *start, PeriodEnd: *end, WeightTarget: weightTarget, CreatedBy: createdBy,
	}
	if err := s.repo.CreateCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("performance: CreateCycle: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) UpdateCycle(ctx context.Context, orgID, ref string, req UpdateCycleRequest) (*GoalCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: UpdateCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	// A closed cycle is fully immutable. Locked still permits metadata edits
	// (renaming a quarter mid-flight is legitimate); only goal DEFINITIONS
	// freeze at lock.
	if c.Status == CycleStatusClosed {
		return nil, ErrCycleWrongStatus
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrCycleNameRequired
		}
		taken, err := s.repo.CycleNameExists(ctx, orgID, name, c.ID)
		if err != nil {
			return nil, fmt.Errorf("performance: UpdateCycle: name check: %w", err)
		}
		if taken {
			return nil, ErrCycleNameTaken
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
	if req.WeightTarget != nil {
		if req.WeightTarget.LessThanOrEqual(decimal.Zero) {
			return nil, ErrInvalidWeightTarget
		}
		c.WeightTarget = *req.WeightTarget
	}

	if err := s.repo.UpdateCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("performance: UpdateCycle: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) ActivateCycle(ctx context.Context, orgID, ref string) (*GoalCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: ActivateCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	if c.Status != CycleStatusDraft {
		return nil, ErrCycleWrongStatus
	}
	if err := s.repo.SetCycleStatus(ctx, orgID, c.ID, CycleStatusActive, nil); err != nil {
		return nil, fmt.Errorf("performance: ActivateCycle: %w", err)
	}
	c.Status = CycleStatusActive
	return c, nil
}

func (s *serviceImpl) LockCycle(ctx context.Context, orgID, ref, actorID string) (*GoalCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: LockCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	if c.Status != CycleStatusActive {
		return nil, ErrCycleWrongStatus
	}

	// The equality check the write path deliberately does not make. Locking
	// asserts "goal setting is finished", which is the only point at which
	// requiring an exact total is meaningful. No force flag in 5A: a force
	// flag becomes the default within a quarter.
	audit, err := s.buildWeightAudit(ctx, orgID, c)
	if err != nil {
		return nil, err
	}
	if len(audit.Incomplete) > 0 {
		return nil, ErrCycleWeightsIncomplete
	}

	if err := s.repo.SetCycleStatus(ctx, orgID, c.ID, CycleStatusLocked, &actorID); err != nil {
		return nil, fmt.Errorf("performance: LockCycle: %w", err)
	}
	c.Status = CycleStatusLocked
	return c, nil
}

func (s *serviceImpl) CloseCycle(ctx context.Context, orgID, ref string) (*GoalCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: CloseCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	if c.Status == CycleStatusClosed || c.Status == CycleStatusDraft {
		return nil, ErrCycleWrongStatus
	}
	if err := s.repo.SetCycleStatus(ctx, orgID, c.ID, CycleStatusClosed, nil); err != nil {
		return nil, fmt.Errorf("performance: CloseCycle: %w", err)
	}
	c.Status = CycleStatusClosed
	return c, nil
}

func (s *serviceImpl) GetCycleWeightAudit(ctx context.Context, orgID, ref string) (*CycleWeightAudit, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("performance: GetCycleWeightAudit: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	return s.buildWeightAudit(ctx, orgID, c)
}

// buildWeightAudit is shared by the lock gate and the read-only audit
// endpoint, so there is exactly one definition of "whose weights are wrong".
func (s *serviceImpl) buildWeightAudit(ctx context.Context, orgID string, c *GoalCycle) (*CycleWeightAudit, error) {
	totals, err := s.repo.FindCycleWeightTotals(ctx, orgID, c.ID)
	if err != nil {
		return nil, fmt.Errorf("performance: weight audit: %w", err)
	}
	if totals == nil {
		totals = []*EmployeeWeightTotal{}
	}
	incomplete := make([]*EmployeeWeightTotal, 0)
	for _, t := range totals {
		if !t.TotalWeight.Equal(c.WeightTarget) {
			incomplete = append(incomplete, t)
		}
	}
	return &CycleWeightAudit{
		CycleID: c.PublicID, WeightTarget: c.WeightTarget,
		Employees: totals, Incomplete: incomplete,
	}, nil
}
