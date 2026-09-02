package employeedocs_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/employeedocs"
)

type stubRepo struct {
	data map[string]*employeedocs.EmployeeDocument
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		data: make(map[string]*employeedocs.EmployeeDocument),
	}
}

func (r *stubRepo) FindAll(ctx context.Context, orgID string, filter employeedocs.DocListFilter) ([]*employeedocs.EmployeeDocument, error) {
	var res []*employeedocs.EmployeeDocument
	for _, d := range r.data {
		if d.OrgID == orgID {
			if filter.EmployeeID != "" && d.EmployeeID != filter.EmployeeID {
				continue
			}
			if filter.Status != "" && string(d.Status) != filter.Status {
				continue
			}
			if filter.RelatedType != "" && d.RelatedType != nil && *d.RelatedType != filter.RelatedType {
				continue
			}
			res = append(res, d)
		}
	}
	return res, nil
}

func (r *stubRepo) Count(ctx context.Context, orgID string, filter employeedocs.DocListFilter) (int, error) {
	out, err := r.FindAll(ctx, orgID, filter)
	return len(out), err
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*employeedocs.EmployeeDocument, error) {
	for _, d := range r.data {
		if d.OrgID == orgID && (d.ID == ref || d.PublicID == ref) {
			if employeeID != "" && d.EmployeeID != employeeID {
				continue
			}
			return d, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) Create(ctx context.Context, d *employeedocs.EmployeeDocument) error {
	d.ID = uuid.NewString()
	d.PublicID = "doc_" + d.ID
	d.Version = 1
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	r.data[d.ID] = d
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status employeedocs.DocStatus) error {
	if d, ok := r.data[id]; ok {
		d.Status = status
		if status == employeedocs.StatusSent {
			now := time.Now()
			d.SentAt = &now
		}
		d.UpdatedAt = time.Now()
	}
	return nil
}

func (r *stubRepo) Acknowledge(ctx context.Context, id string, note, signature *string) error {
	if d, ok := r.data[id]; ok {
		d.Status = employeedocs.StatusAcknowledged
		now := time.Now()
		d.AcknowledgedAt = &now
		d.AcknowledgementNote = note
		d.UpdatedAt = time.Now()
	}
	return nil
}

func TestEmployeeDocsService(t *testing.T) {
	repo := newStubRepo()
	svc := employeedocs.NewService(repo, nil)
	ctx := context.Background()
	orgID := "org1"
	empID := "emp1"

	t.Run("Create Document", func(t *testing.T) {
		req := employeedocs.CreateDocumentRequest{
			Title: "Contract",
			FileURL: "https://example.com/doc.pdf",
			FileName: "doc.pdf",
			DocumentType: "contract",
		}
		d, err := svc.Create(ctx, orgID, empID, "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if d.Title != "Contract" {
			t.Errorf("expected Contract, got %s", d.Title)
		}
		if d.Status != employeedocs.StatusDraft {
			t.Errorf("expected draft status, got %s", d.Status)
		}
	})

	t.Run("Create Validation", func(t *testing.T) {
		req := employeedocs.CreateDocumentRequest{
			Title: "",
			FileURL: "url",
			FileName: "name",
			DocumentType: "policy",
		}
		_, err := svc.Create(ctx, orgID, empID, "admin", req)
		if err != employeedocs.ErrTitleRequired {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
	})

	t.Run("Status Workflow", func(t *testing.T) {
		req := employeedocs.CreateDocumentRequest{
			Title: "Policy",
			FileURL: "http://url",
			FileName: "policy.pdf",
			DocumentType: "policy",
		}
		d, _ := svc.Create(ctx, orgID, empID, "admin", req)

		// Send
		sent, err := svc.Send(ctx, orgID, empID, d.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if sent.Status != employeedocs.StatusSent {
			t.Errorf("expected sent, got %s", sent.Status)
		}

		// List
		list, err := svc.List(ctx, orgID, employeedocs.DocListFilter{EmployeeID: empID, Status: string(employeedocs.StatusSent), Scope: authz.ScopeAll})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if list.Total != 1 {
			t.Errorf("expected 1, got %d", list.Total)
		}

		// Acknowledge
		ackReq := employeedocs.AcknowledgeDocRequest{Note: nil}
		ack, err := svc.Acknowledge(ctx, orgID, empID, d.ID, ackReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ack.Status != employeedocs.StatusAcknowledged {
			t.Errorf("expected acknowledged, got %s", ack.Status)
		}
		
		// Withdraw (should fail since it's acknowledged)
		_, err = svc.Withdraw(ctx, orgID, empID, d.ID)
		if err != employeedocs.ErrWrongStatus {
			t.Errorf("expected ErrWrongStatus, got %v", err)
		}
	})

	t.Run("Decline Document", func(t *testing.T) {
		req := employeedocs.CreateDocumentRequest{
			Title: "Warning",
			FileURL: "url",
			FileName: "warn.pdf",
			DocumentType: "warning",
		}
		d, _ := svc.Create(ctx, orgID, empID, "admin", req)
		_, _ = svc.Send(ctx, orgID, empID, d.ID)
		
		declined, err := svc.Decline(ctx, orgID, empID, d.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if declined.Status != employeedocs.StatusDeclined {
			t.Errorf("expected declined, got %s", declined.Status)
		}
	})

	t.Run("Withdraw Document", func(t *testing.T) {
		req := employeedocs.CreateDocumentRequest{
			Title: "Wrong Doc",
			FileURL: "url",
			FileName: "wrong.pdf",
			DocumentType: "other",
		}
		d, _ := svc.Create(ctx, orgID, empID, "admin", req)
		_, _ = svc.Send(ctx, orgID, empID, d.ID)
		
		withdrawn, err := svc.Withdraw(ctx, orgID, empID, d.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if withdrawn.Status != employeedocs.StatusWithdrawn {
			t.Errorf("expected withdrawn, got %s", withdrawn.Status)
		}
	})
	
	t.Run("Privacy Checks", func(t *testing.T) {
		req := employeedocs.CreateDocumentRequest{
			Title: "Confidential",
			FileURL: "url",
			FileName: "confidential.pdf",
			DocumentType: "contract",
		}
		d, _ := svc.Create(ctx, orgID, empID, "admin", req)
		
		_, err := svc.Get(ctx, "org2", empID, d.ID)
		if err != employeedocs.ErrNotFound {
			t.Errorf("expected ErrNotFound for wrong org, got %v", err)
		}
		
		_, err = svc.Get(ctx, orgID, "emp2", d.ID)
		if err != employeedocs.ErrNotFound {
			t.Errorf("expected ErrNotFound for wrong employee, got %v", err)
		}
	})
}
