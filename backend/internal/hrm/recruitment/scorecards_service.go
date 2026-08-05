// backend/internal/hrm/recruitment/scorecards_service.go
package recruitment

import (
	"context"
	"fmt"
	"strings"
)

// ScorecardService is embedded into Service — see service.go.
type ScorecardService interface {
	// ListScorecards implements the visibility rule from migration 00080's
	// header: a caller who is an assigned panelist and has not yet
	// submitted their own scorecard sees only their own (possibly empty)
	// draft. Everyone else — a panelist who has submitted, or a caller with
	// view permission who isn't on the panel at all (e.g. an HR admin
	// auditing after the fact) — sees every SUBMITTED scorecard. Drafts are
	// always private to their author, full stop; that part is not
	// role-dependent.
	ListScorecards(ctx context.Context, orgID, interviewRef, callerUserID string) ([]*Scorecard, error)
	UpsertOwnScorecard(ctx context.Context, orgID, interviewRef, callerUserID string, req UpsertScorecardRequest) (*Scorecard, error)
	SubmitOwnScorecard(ctx context.Context, orgID, interviewRef, callerUserID string) (*Scorecard, error)
}

func (s *serviceImpl) ListScorecards(ctx context.Context, orgID, interviewRef, callerUserID string) ([]*Scorecard, error) {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, interviewRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListScorecards: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}

	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, callerUserID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListScorecards: %w", err)
	}
	if employeeID != "" {
		panelist, err := s.repo.FindPanelist(ctx, i.ID, employeeID)
		if err != nil {
			return nil, fmt.Errorf("recruitment: ListScorecards: %w", err)
		}
		if panelist != nil {
			own, err := s.repo.FindScorecard(ctx, i.ID, employeeID)
			if err != nil {
				return nil, fmt.Errorf("recruitment: ListScorecards: %w", err)
			}
			if own == nil {
				return []*Scorecard{}, nil
			}
			if own.SubmittedAt == nil {
				return []*Scorecard{own}, nil
			}
			// Own scorecard submitted — fall through to the "reveal
			// submitted" branch below.
		}
	}

	all, err := s.repo.FindScorecards(ctx, i.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListScorecards: %w", err)
	}
	visible := make([]*Scorecard, 0, len(all))
	for _, sc := range all {
		if sc.SubmittedAt != nil {
			visible = append(visible, sc)
		}
	}
	return visible, nil
}

func validScoreRange(v *int) bool {
	return v == nil || (*v >= 1 && *v <= 5)
}

func (s *serviceImpl) UpsertOwnScorecard(ctx context.Context, orgID, interviewRef, callerUserID string, req UpsertScorecardRequest) (*Scorecard, error) {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, interviewRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpsertOwnScorecard: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}

	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, callerUserID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpsertOwnScorecard: %w", err)
	}
	if employeeID == "" {
		return nil, ErrCallerHasNoEmployeeRecord
	}
	// Narrows the broadly-granted hrm.interviews.scorecard permission down
	// to "is this actually your panel assignment" — the route gate cannot
	// express that, so the service does (the platform.checklists.complete
	// precedent from Phase 3).
	panelist, err := s.repo.FindPanelist(ctx, i.ID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpsertOwnScorecard: %w", err)
	}
	if panelist == nil {
		return nil, ErrNotAPanelist
	}

	if !validScoreRange(req.OverallRating) || !validScoreRange(req.TechnicalScore) ||
		!validScoreRange(req.CommunicationScore) || !validScoreRange(req.CultureFitScore) {
		return nil, ErrInvalidScoreRange
	}

	var rec *ScorecardRecommendation
	if req.Recommendation != nil && strings.TrimSpace(*req.Recommendation) != "" {
		r := ScorecardRecommendation(strings.TrimSpace(*req.Recommendation))
		if !r.IsValid() {
			return nil, ErrInvalidRecommendation
		}
		rec = &r
	}

	sc := &Scorecard{
		InterviewID: i.ID, PanelistEmployeeID: employeeID,
		OverallRating: req.OverallRating, TechnicalScore: req.TechnicalScore,
		CommunicationScore: req.CommunicationScore, CultureFitScore: req.CultureFitScore,
		Recommendation: rec, Strengths: req.Strengths, Concerns: req.Concerns,
	}
	if err := s.repo.UpsertScorecardDraft(ctx, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *serviceImpl) SubmitOwnScorecard(ctx context.Context, orgID, interviewRef, callerUserID string) (*Scorecard, error) {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, interviewRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: SubmitOwnScorecard: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}

	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, callerUserID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: SubmitOwnScorecard: %w", err)
	}
	if employeeID == "" {
		return nil, ErrCallerHasNoEmployeeRecord
	}
	panelist, err := s.repo.FindPanelist(ctx, i.ID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: SubmitOwnScorecard: %w", err)
	}
	if panelist == nil {
		return nil, ErrNotAPanelist
	}

	existing, err := s.repo.FindScorecard(ctx, i.ID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: SubmitOwnScorecard: %w", err)
	}
	if existing == nil {
		return nil, ErrScorecardNotFound
	}
	if existing.SubmittedAt != nil {
		return nil, ErrScorecardAlreadySubmitted
	}

	sc, err := s.repo.SubmitScorecard(ctx, i.ID, employeeID)
	if err != nil {
		return nil, err
	}
	return sc, nil
}
