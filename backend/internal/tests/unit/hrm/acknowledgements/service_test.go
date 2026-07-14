package acknowledgements_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/acknowledgements"
)

type mockAckRepo struct {
	data map[string]*acknowledgements.Acknowledgement
}

func newMockAckRepo() *mockAckRepo {
	return &mockAckRepo{data: make(map[string]*acknowledgements.Acknowledgement)}
}

func (m *mockAckRepo) FindAll(ctx context.Context, orgID, employeeID, ackType, status string) ([]*acknowledgements.Acknowledgement, error) {
	var list []*acknowledgements.Acknowledgement
	for _, a := range m.data {
		if a.OrgID != orgID {
			continue
		}
		if employeeID != "" && a.EmployeeID != employeeID {
			continue
		}
		if ackType != "" && string(a.AcknowledgeableType) != ackType {
			continue
		}
		if status != "" && string(a.Status) != status {
			continue
		}
		list = append(list, a)
	}
	return list, nil
}

func (m *mockAckRepo) FindByEntity(ctx context.Context, orgID, ackType, ackID string) ([]*acknowledgements.Acknowledgement, error) {
	var list []*acknowledgements.Acknowledgement
	for _, a := range m.data {
		if a.OrgID == orgID && string(a.AcknowledgeableType) == ackType && a.AcknowledgeableID == ackID {
			list = append(list, a)
		}
	}
	return list, nil
}

func (m *mockAckRepo) FindByRef(ctx context.Context, orgID, ref string) (*acknowledgements.Acknowledgement, error) {
	for _, a := range m.data {
		if a.OrgID == orgID && (a.ID == ref || a.PublicID == ref) {
			return a, nil
		}
	}
	return nil, nil // Return nil, nil when not found to match behavior
}

func (m *mockAckRepo) Create(ctx context.Context, a *acknowledgements.Acknowledgement) error {
	a.ID = "ack-id-" + a.EmployeeID
	a.PublicID = "pub-" + a.ID
	m.data[a.ID] = a
	return nil
}

func (m *mockAckRepo) UpdateStatus(ctx context.Context, id string, status acknowledgements.AckStatus) error {
	if a, ok := m.data[id]; ok {
		a.Status = status
	}
	return nil
}

func (m *mockAckRepo) Acknowledge(ctx context.Context, id string, notes, signature *string) error {
	if a, ok := m.data[id]; ok {
		a.Status = acknowledgements.StatusAcknowledged
		a.Notes = notes
		a.SignatureData = signature
		now := time.Now()
		a.AcknowledgedAt = &now
		if signature != nil {
			a.SignedAt = &now
		}
	}
	return nil
}

func (m *mockAckRepo) Decline(ctx context.Context, id, reason string) error {
	if a, ok := m.data[id]; ok {
		a.Status = acknowledgements.StatusDeclined
		a.DeclineReason = &reason
		now := time.Now()
		a.DeclinedAt = &now
	}
	return nil
}

func TestAcknowledgementsService(t *testing.T) {
	repo := newMockAckRepo()
	svc := acknowledgements.NewService(repo, nil) // db pool is not used by serviceImpl except for initialization
	ctx := context.Background()

	orgID := "org1"
	employeeID := "emp1"
	reqBy := "admin1"

	t.Run("Create", func(t *testing.T) {
		req := acknowledgements.CreateAcknowledgementRequest{
			EmployeeID:          employeeID,
			AcknowledgeableType: acknowledgements.TypeWarning,
			AcknowledgeableID:   "warn1",
			EntityTitle:         "Warning 1",
			SignatureRequired:   true,
		}

		ack, err := svc.Create(ctx, orgID, reqBy, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ack.Status != acknowledgements.StatusPending {
			t.Errorf("expected status pending, got %v", ack.Status)
		}
	})

	t.Run("Create - Validation Errors", func(t *testing.T) {
		req := acknowledgements.CreateAcknowledgementRequest{
			EmployeeID:          "",
			AcknowledgeableType: acknowledgements.TypeWarning,
			AcknowledgeableID:   "warn1",
			EntityTitle:         "Warning 1",
		}
		_, err := svc.Create(ctx, orgID, reqBy, req)
		if err != acknowledgements.ErrEmployeeIDRequired {
			t.Errorf("expected ErrEmployeeIDRequired, got %v", err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		ack, err := svc.Get(ctx, orgID, "ack-id-emp1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ack.EmployeeID != employeeID {
			t.Errorf("expected employee %s, got %s", employeeID, ack.EmployeeID)
		}
	})

	t.Run("List", func(t *testing.T) {
		resp, err := svc.List(ctx, orgID, employeeID, string(acknowledgements.TypeWarning), string(acknowledgements.StatusPending))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Total != 1 {
			t.Errorf("expected 1 result, got %d", resp.Total)
		}
	})

	t.Run("ListByEntity", func(t *testing.T) {
		resp, err := svc.ListByEntity(ctx, orgID, string(acknowledgements.TypeWarning), "warn1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Total != 1 {
			t.Errorf("expected 1 result, got %d", resp.Total)
		}
	})

	t.Run("Respond - Missing Signature", func(t *testing.T) {
		_, err := svc.Respond(ctx, orgID, "ack-id-emp1", acknowledgements.RespondRequest{})
		if err != acknowledgements.ErrSignatureRequired {
			t.Errorf("expected ErrSignatureRequired, got %v", err)
		}
	})

	t.Run("Respond - Success", func(t *testing.T) {
		sig := "signature-data"
		ack, err := svc.Respond(ctx, orgID, "ack-id-emp1", acknowledgements.RespondRequest{
			SignatureData: &sig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ack.Status != acknowledgements.StatusAcknowledged {
			t.Errorf("expected status acknowledged, got %v", ack.Status)
		}
	})

	t.Run("Decline", func(t *testing.T) {
		// create a new one to decline
		req := acknowledgements.CreateAcknowledgementRequest{
			EmployeeID:          "emp2",
			AcknowledgeableType: acknowledgements.TypeDocument,
			AcknowledgeableID:   "doc1",
			EntityTitle:         "Doc 1",
		}
		ack, _ := svc.Create(ctx, orgID, reqBy, req)

		declinedAck, err := svc.Decline(ctx, orgID, ack.ID, acknowledgements.DeclineRequest{
			Reason: "Not applicable",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if declinedAck.Status != acknowledgements.StatusDeclined {
			t.Errorf("expected status declined, got %v", declinedAck.Status)
		}
	})
	
	t.Run("Cross-Org Isolation", func(t *testing.T) {
		_, err := svc.Get(ctx, "other-org", "ack-id-emp1")
		if err != acknowledgements.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org fetch, got %v", err)
		}
	})
}
