// backend/internal/crm/templates/service.go
package templates

import (
	"context"
	"fmt"
)

// Service defines the business logic for CRM templates.
type Service interface {
	ListTemplates(ctx context.Context, orgID string) (*TemplateListResponse, error)
	GetTemplate(ctx context.Context, orgID, templateID string) (*Template, error)
	CreateTemplate(ctx context.Context, orgID, userID string, req CreateTemplateRequest) (*Template, error)
	UpdateTemplate(ctx context.Context, orgID, templateID string, req UpdateTemplateRequest) (*Template, error)
	DeleteTemplate(ctx context.Context, orgID, templateID string) error
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new templates service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) ListTemplates(ctx context.Context, orgID string) (*TemplateListResponse, error) {
	templates, err := s.repo.FindTemplates(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("templates: ListTemplates: %w", err)
	}
	if templates == nil {
		templates = []*Template{}
	}
	return &TemplateListResponse{
		Templates: templates,
		Total:     len(templates),
	}, nil
}

func (s *serviceImpl) GetTemplate(ctx context.Context, orgID, templateID string) (*Template, error) {
	t, err := s.repo.FindTemplateByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("templates: GetTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	return t, nil
}

func (s *serviceImpl) CreateTemplate(ctx context.Context, orgID, userID string, req CreateTemplateRequest) (*Template, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	t := &Template{
		OrgID:     orgID,
		Name:      req.Name,
		Type:      TemplateType(req.Type),
		Subject:   req.Subject,
		Body:      req.Body,
		CreatedBy: userID,
	}

	if err := s.repo.CreateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("templates: CreateTemplate: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) UpdateTemplate(ctx context.Context, orgID, templateID string, req UpdateTemplateRequest) (*Template, error) {
	t, err := s.repo.FindTemplateByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("templates: UpdateTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}

	if req.Name != nil && *req.Name != "" {
		t.Name = *req.Name
	}
	if req.Subject != nil {
		t.Subject = req.Subject
	}
	if req.Body != nil && *req.Body != "" {
		t.Body = *req.Body
	}

	if err := s.repo.UpdateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("templates: UpdateTemplate: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) DeleteTemplate(ctx context.Context, orgID, templateID string) error {
	return s.repo.DeleteTemplate(ctx, orgID, templateID)
}
