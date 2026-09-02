// backend/internal/hrm/doctemplates/service.go
package doctemplates

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var placeholderRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

type Service interface {
	List(ctx context.Context, orgID string, activeOnly bool, docType string) (*DocumentTemplateListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*DocumentTemplate, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateDocumentTemplateRequest) (*DocumentTemplate, error)
	Update(ctx context.Context, orgID, ref string, req UpdateDocumentTemplateRequest) (*DocumentTemplate, error)
	Delete(ctx context.Context, orgID, ref string) error
	Preview(ctx context.Context, orgID, ref string, req PreviewTemplateRequest) (*PreviewResult, error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

func (s *serviceImpl) List(ctx context.Context, orgID string, activeOnly bool, docType string) (*DocumentTemplateListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, activeOnly, docType)
	if err != nil {
		return nil, fmt.Errorf("doctemplates: List: %w", err)
	}
	if list == nil {
		list = []*DocumentTemplate{}
	}
	return &DocumentTemplateListResponse{Templates: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*DocumentTemplate, error) {
	t, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("doctemplates: Get: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	return t, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateDocumentTemplateRequest) (*DocumentTemplate, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(name) > 100 {
		return nil, ErrNameTooLong
	}
	if !req.DocumentType.IsValid() {
		return nil, ErrInvalidDocumentType
	}
	if strings.TrimSpace(req.BodyMarkdown) == "" {
		return nil, ErrBodyRequired
	}
	if exists, _ := s.repo.NameExists(ctx, orgID, name, ""); exists {
		return nil, ErrNameConflict
	}

	vars := req.AvailableVariables
	if len(vars) == 0 {
		vars = []string{}
	}

	t := &DocumentTemplate{
		OrgID: orgID, Name: name, DocumentType: req.DocumentType,
		Description: req.Description, BodyMarkdown: req.BodyMarkdown,
		AvailableVariables:      vars,
		RequiresAcknowledgement: req.RequiresAcknowledgement,
		IsActive:                true, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("doctemplates: Create: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdateDocumentTemplateRequest) (*DocumentTemplate, error) {
	t, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("doctemplates: Update: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		if len(name) > 100 {
			return nil, ErrNameTooLong
		}
		if exists, _ := s.repo.NameExists(ctx, orgID, name, t.ID); exists {
			return nil, ErrNameConflict
		}
		t.Name = name
	}
	if req.DocumentType != nil {
		if !req.DocumentType.IsValid() {
			return nil, ErrInvalidDocumentType
		}
		t.DocumentType = *req.DocumentType
	}
	if req.Description != nil {
		t.Description = req.Description
	}
	if req.BodyMarkdown != nil {
		if strings.TrimSpace(*req.BodyMarkdown) == "" {
			return nil, ErrBodyRequired
		}
		t.BodyMarkdown = *req.BodyMarkdown
	}
	if req.AvailableVariables != nil {
		t.AvailableVariables = req.AvailableVariables
	}
	if req.RequiresAcknowledgement != nil {
		t.RequiresAcknowledgement = *req.RequiresAcknowledgement
	}
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("doctemplates: Update: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) Delete(ctx context.Context, orgID, ref string) error {
	return s.repo.Delete(ctx, orgID, ref)
}

func (s *serviceImpl) Preview(ctx context.Context, orgID, ref string, req PreviewTemplateRequest) (*PreviewResult, error) {
	t, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("doctemplates: Preview: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}

	// Find all {{placeholder}} tokens in the body
	matches := placeholderRe.FindAllStringSubmatch(t.BodyMarkdown, -1)
	filled := t.BodyMarkdown
	used := make([]string, 0)
	missing := make([]string, 0)

	for _, m := range matches {
		tok := m[0] // {{variable.path}}
		key := m[1] // variable.path
		if val, ok := req.Variables[key]; ok {
			filled = strings.ReplaceAll(filled, tok, val)
			used = append(used, key)
		} else {
			missing = append(missing, key)
		}
	}

	return &PreviewResult{FilledContent: filled, VariablesUsed: used, Missing: missing}, nil
}
