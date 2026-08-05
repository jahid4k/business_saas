// backend/internal/hrm/recruitment/referrals_service.go
package recruitment

import (
	"context"
	"fmt"
	"strings"
)

// ReferralService is embedded into Service — see service.go.
type ReferralService interface {
	ListReferrals(ctx context.Context, orgID string, filter ReferralListFilter) (*ReferralListResponse, error)
	GetReferral(ctx context.Context, orgID, ref string) (*Referral, error)
	CreateReferral(ctx context.Context, orgID, createdBy string, req CreateReferralRequest) (*Referral, error)
	UpdateReferral(ctx context.Context, orgID, ref string, req UpdateReferralRequest) (*Referral, error)
}

func (s *serviceImpl) ListReferrals(ctx context.Context, orgID string, filter ReferralListFilter) (*ReferralListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindReferrals(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListReferrals: %w", err)
	}
	if list == nil {
		list = []*Referral{}
	}
	total, err := s.repo.CountReferrals(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListReferrals: count: %w", err)
	}
	return &ReferralListResponse{Referrals: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetReferral(ctx context.Context, orgID, ref string) (*Referral, error) {
	rf, err := s.repo.FindReferralByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetReferral: %w", err)
	}
	if rf == nil {
		return nil, ErrReferralNotFound
	}
	return rf, nil
}

func (s *serviceImpl) CreateReferral(ctx context.Context, orgID, createdBy string, req CreateReferralRequest) (*Referral, error) {
	candidateRef := strings.TrimSpace(req.CandidateID)
	if candidateRef == "" {
		return nil, ErrReferralCandidateRequired
	}
	candidate, err := s.repo.FindCandidateByRef(ctx, orgID, candidateRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateReferral: candidate: %w", err)
	}
	if candidate == nil {
		return nil, ErrCandidateNotFound
	}

	var applicationID *string
	if req.ApplicationID != nil && strings.TrimSpace(*req.ApplicationID) != "" {
		app, err := s.repo.FindApplicationByRef(ctx, orgID, strings.TrimSpace(*req.ApplicationID))
		if err != nil {
			return nil, fmt.Errorf("recruitment: CreateReferral: application: %w", err)
		}
		if app == nil {
			return nil, ErrApplicationNotFound
		}
		applicationID = &app.ID
	}

	rf := &Referral{
		OrgID: orgID, CandidateID: candidate.ID, ReferredByEmployeeID: nilIfEmptyRecruitment(req.ReferredByEmployeeID),
		ApplicationID: applicationID, Notes: req.Notes, CreatedBy: createdBy,
	}
	if err := s.repo.CreateReferral(ctx, rf); err != nil {
		return nil, fmt.Errorf("recruitment: CreateReferral: %w", err)
	}
	return rf, nil
}

func (s *serviceImpl) UpdateReferral(ctx context.Context, orgID, ref string, req UpdateReferralRequest) (*Referral, error) {
	rf, err := s.repo.FindReferralByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateReferral: %w", err)
	}
	if rf == nil {
		return nil, ErrReferralNotFound
	}

	if req.Status != nil {
		status := ReferralStatus(strings.TrimSpace(*req.Status))
		if !status.IsValid() {
			return nil, ErrInvalidReferralStatus
		}
		rf.Status = status
	}
	if req.BonusAmount != nil {
		rf.BonusAmount = req.BonusAmount
	}
	if req.BonusCurrency != nil && strings.TrimSpace(*req.BonusCurrency) != "" {
		rf.BonusCurrency = strings.ToUpper(strings.TrimSpace(*req.BonusCurrency))
	}
	if req.Notes != nil {
		rf.Notes = req.Notes
	}

	if err := s.repo.UpdateReferral(ctx, rf); err != nil {
		return nil, fmt.Errorf("recruitment: UpdateReferral: %w", err)
	}
	return rf, nil
}

// nilIfEmptyRecruitment mirrors the employees package's nilIfEmpty helper —
// duplicated locally rather than imported, since these are two unrelated
// packages and the helper is a two-line triviality.
func nilIfEmptyRecruitment(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}
