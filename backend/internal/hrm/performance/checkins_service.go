// backend/internal/hrm/performance/checkins_service.go
package performance

import (
	"context"
	"fmt"
	"strings"
)

// CheckinService is embedded into Service — see service.go.
type CheckinService interface {
	ListCheckins(ctx context.Context, orgID, goalRef string, caller Caller, limit, offset int) (*CheckinListResponse, error)
	CreateCheckin(ctx context.Context, orgID, goalRef string, caller Caller, req CreateCheckinRequest) (*CreateCheckinResult, error)
}

func (s *serviceImpl) ListCheckins(ctx context.Context, orgID, goalRef string, caller Caller, limit, offset int) (*CheckinListResponse, error) {
	g, err := s.repo.FindGoalByRef(ctx, orgID, goalRef)
	if err != nil {
		return nil, fmt.Errorf("performance: ListCheckins: %w", err)
	}
	if g == nil {
		return nil, ErrGoalNotFound
	}
	// Reading history is a read of the goal, so it is gated by the same
	// record-access rule as GetGoal.
	if err := s.authorizeGoalAccess(ctx, orgID, caller, g); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}

	list, err := s.repo.FindCheckins(ctx, g.ID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("performance: ListCheckins: %w", err)
	}
	if list == nil {
		list = []*GoalCheckin{}
	}
	total, err := s.repo.CountCheckins(ctx, g.ID)
	if err != nil {
		return nil, fmt.Errorf("performance: ListCheckins: count: %w", err)
	}
	return &CheckinListResponse{Checkins: list, Total: total, Limit: limit, Offset: offset}, nil
}

// CreateCheckin records progress. This is the only path that mutates a goal's
// current_value.
//
// Deliberately NOT gated on the cycle being active: 'locked' freezes goal
// DEFINITIONS while check-ins keep landing, which is the normal in-flight
// state for a quarter. Only the goal's own status can block a check-in.
func (s *serviceImpl) CreateCheckin(ctx context.Context, orgID, goalRef string, caller Caller, req CreateCheckinRequest) (*CreateCheckinResult, error) {
	g, err := s.repo.FindGoalByRef(ctx, orgID, goalRef)
	if err != nil {
		return nil, fmt.Errorf("performance: CreateCheckin: %w", err)
	}
	if g == nil {
		return nil, ErrGoalNotFound
	}
	if err := s.authorizeWrite(ctx, orgID, caller, g.EmployeeID); err != nil {
		return nil, err
	}
	if g.Status == GoalStatusCompleted || g.Status == GoalStatusCancelled {
		return nil, ErrCheckinGoalNotOpen
	}

	var confidence *Confidence
	if req.Confidence != nil && strings.TrimSpace(*req.Confidence) != "" {
		c := Confidence(strings.TrimSpace(*req.Confidence))
		if !c.IsValid() {
			return nil, ErrCheckinInvalidConfidence
		}
		confidence = &c
	}

	ck := &GoalCheckin{
		GoalID:     g.ID,
		Confidence: confidence,
		Note:       nilIfBlank(req.Note),
	}
	if caller.UserID != "" {
		ck.CheckedInBy = &caller.UserID
	}

	// The repository owns the transaction: it locks the goal, snapshots the
	// previous value, derives progress and appends the row atomically. Doing
	// the read here and the write there would let two concurrent check-ins
	// record the same previous_value.
	advanced, err := s.repo.CreateCheckin(ctx, orgID, ck, req.CurrentValue)
	if err != nil {
		return nil, err
	}

	return &CreateCheckinResult{
		Checkin: ck,
		Goal: &GoalDetail{
			Goal:               advanced,
			ProgressPercent:    advanced.ProgressPercent(),
			RawProgressPercent: advanced.RawProgressPercent(),
		},
	}, nil
}
