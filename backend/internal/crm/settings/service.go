package settings

import "context"

type Service interface {
	GetSettings(ctx context.Context, orgID string) (*CRMSettings, error)
	UpdateSettings(ctx context.Context, orgID string, req UpdateCRMSettingsRequest) (*CRMSettings, error)
}

type serviceImpl struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) GetSettings(ctx context.Context, orgID string) (*CRMSettings, error) {
	return s.repo.Get(ctx, orgID)
}

func (s *serviceImpl) UpdateSettings(ctx context.Context, orgID string, req UpdateCRMSettingsRequest) (*CRMSettings, error) {
	current, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if req.LeadRoutingEnabled != nil {
		current.LeadRoutingEnabled = *req.LeadRoutingEnabled
	}
	if req.RoundRobinAssignees != nil {
		current.RoundRobinAssignees = req.RoundRobinAssignees
	}

	if err := s.repo.Upsert(ctx, current); err != nil {
		return nil, err
	}

	return current, nil
}
