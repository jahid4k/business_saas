package doctemplates_test

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/doctemplates"
)

type mockDocRepo struct {
	data map[string]*doctemplates.DocumentTemplate
}

func newMockDocRepo() *mockDocRepo {
	return &mockDocRepo{data: make(map[string]*doctemplates.DocumentTemplate)}
}

func (m *mockDocRepo) FindAll(ctx context.Context, orgID string, activeOnly bool, docType string) ([]*doctemplates.DocumentTemplate, error) {
	var list []*doctemplates.DocumentTemplate
	for _, t := range m.data {
		if t.OrgID != orgID {
			continue
		}
		if activeOnly && !t.IsActive {
			continue
		}
		if docType != "" && string(t.DocumentType) != docType {
			continue
		}
		list = append(list, t)
	}
	return list, nil
}

func (m *mockDocRepo) FindByRef(ctx context.Context, orgID, ref string) (*doctemplates.DocumentTemplate, error) {
	for _, t := range m.data {
		if t.OrgID == orgID && (t.ID == ref || t.PublicID == ref) {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockDocRepo) Create(ctx context.Context, t *doctemplates.DocumentTemplate) error {
	t.ID = "doc-" + t.Name
	t.PublicID = "pub-" + t.ID
	m.data[t.ID] = t
	return nil
}

func (m *mockDocRepo) Update(ctx context.Context, t *doctemplates.DocumentTemplate) error {
	if existing, ok := m.data[t.ID]; ok {
		*existing = *t
	}
	return nil
}

func (m *mockDocRepo) Delete(ctx context.Context, orgID, ref string) error {
	for id, t := range m.data {
		if t.OrgID == orgID && (t.ID == ref || t.PublicID == ref) {
			delete(m.data, id)
			return nil
		}
	}
	return doctemplates.ErrTemplateNotFound
}

func (m *mockDocRepo) NameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	for _, t := range m.data {
		if t.OrgID == orgID && t.Name == name && t.ID != excludeID && t.IsActive {
			return true, nil
		}
	}
	return false, nil
}


func TestDocTemplatesService(t *testing.T) {
	repo := newMockDocRepo()
	svc := doctemplates.NewService(repo)
	ctx := context.Background()
	orgID := "org1"
	createdBy := "admin1"

	t.Run("Create", func(t *testing.T) {
		req := doctemplates.CreateDocumentTemplateRequest{
			Name:         "Offer Letter",
			DocumentType: doctemplates.DocTypeOfferLetter,
			BodyMarkdown: "Hello {{employee.name}}",
		}

		tmpl, err := svc.Create(ctx, orgID, createdBy, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tmpl.Name != "Offer Letter" {
			t.Errorf("expected name Offer Letter, got %s", tmpl.Name)
		}
	})

	t.Run("Create - Name Conflict", func(t *testing.T) {
		req := doctemplates.CreateDocumentTemplateRequest{
			Name:         "Offer Letter",
			DocumentType: doctemplates.DocTypeOfferLetter,
			BodyMarkdown: "Hello",
		}
		_, err := svc.Create(ctx, orgID, createdBy, req)
		if err != doctemplates.ErrNameConflict {
			t.Errorf("expected ErrNameConflict, got %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		desc := "Updated description"
		tmpl, err := svc.Update(ctx, orgID, "doc-Offer Letter", doctemplates.UpdateDocumentTemplateRequest{
			Description: &desc,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tmpl.Description == nil || *tmpl.Description != desc {
			t.Errorf("expected description %s, got %v", desc, tmpl.Description)
		}
	})

	t.Run("Get", func(t *testing.T) {
		tmpl, err := svc.Get(ctx, orgID, "doc-Offer Letter")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tmpl.Name != "Offer Letter" {
			t.Errorf("expected name Offer Letter, got %s", tmpl.Name)
		}
	})

	t.Run("Preview", func(t *testing.T) {
		res, err := svc.Preview(ctx, orgID, "doc-Offer Letter", doctemplates.PreviewTemplateRequest{
			Variables: map[string]string{
				"employee.name": "John Doe",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.FilledContent != "Hello John Doe" {
			t.Errorf("expected 'Hello John Doe', got %s", res.FilledContent)
		}
		if len(res.VariablesUsed) != 1 || res.VariablesUsed[0] != "employee.name" {
			t.Errorf("unexpected variables used: %v", res.VariablesUsed)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := svc.Delete(ctx, orgID, "doc-Offer Letter")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = svc.Get(ctx, orgID, "doc-Offer Letter")
		if err != doctemplates.ErrTemplateNotFound {
			t.Errorf("expected ErrTemplateNotFound, got %v", err)
		}
	})
	
	t.Run("Cross-Org Isolation", func(t *testing.T) {
		// Recreate first
		svc.Create(ctx, orgID, createdBy, doctemplates.CreateDocumentTemplateRequest{
			Name:         "Cross Org",
			DocumentType: doctemplates.DocTypeOfferLetter,
			BodyMarkdown: "Hello",
		})
		_, err := svc.Get(ctx, "org2", "doc-Cross Org")
		if err != doctemplates.ErrTemplateNotFound {
			t.Errorf("expected ErrTemplateNotFound for cross-org fetch, got %v", err)
		}
	})
}
