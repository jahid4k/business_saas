package awards_test

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/awards"
)

type mockAwardsRepo struct {
	data map[string]*awards.Award
}

func newMockAwardsRepo() *mockAwardsRepo {
	return &mockAwardsRepo{data: make(map[string]*awards.Award)}
}

func (m *mockAwardsRepo) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*awards.Award, error) {
	var list []*awards.Award
	for _, a := range m.data {
		if a.OrgID != orgID {
			continue
		}
		if employeeID != "" && a.EmployeeID != employeeID {
			continue
		}
		if status != "" && string(a.Status) != status {
			continue
		}
		list = append(list, a)
	}
	return list, nil
}

func (m *mockAwardsRepo) FindByRef(ctx context.Context, orgID, ref string) (*awards.Award, error) {
	for _, a := range m.data {
		if a.OrgID == orgID && (a.ID == ref || a.PublicID == ref) {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockAwardsRepo) Create(ctx context.Context, a *awards.Award) error {
	a.ID = "award-" + a.EmployeeID
	a.PublicID = "pub-" + a.ID
	m.data[a.ID] = a
	return nil
}

func (m *mockAwardsRepo) Update(ctx context.Context, a *awards.Award) error {
	if existing, ok := m.data[a.ID]; ok {
		*existing = *a
	}
	return nil
}

func (m *mockAwardsRepo) UpdateStatus(ctx context.Context, id string, status awards.AwardStatus) error {
	if a, ok := m.data[id]; ok {
		a.Status = status
	}
	return nil
}

func (m *mockAwardsRepo) SetApprovalInstance(ctx context.Context, id, instanceID string, status awards.AwardStatus) error {
	if a, ok := m.data[id]; ok {
		a.ApprovalInstanceID = &instanceID
		a.Status = status
	}
	return nil
}

func (m *mockAwardsRepo) SetAnnouncementID(ctx context.Context, id, announcementID string) error {
	if a, ok := m.data[id]; ok {
		a.AnnouncementID = &announcementID
	}
	return nil
}

// Dummy mock for approvals.Service
type mockApprovalsSvc struct {
	approvals.Service // embed to satisfy interface trivially
}

func (m *mockApprovalsSvc) FindDefault(ctx context.Context, orgID string, actionType approvals.ActionType) (*approvals.ApprovalTemplate, error) {
	return nil, nil // return nil means no approval chain configured
}

func TestAwardsService(t *testing.T) {
	repo := newMockAwardsRepo()
	appSvc := &mockApprovalsSvc{}
	svc := awards.NewService(repo, nil, appSvc)
	ctx := context.Background()

	orgID := "org1"
	employeeID := "emp1"
	createdBy := "admin1"

	t.Run("Create", func(t *testing.T) {
		req := awards.CreateAwardRequest{
			EmployeeID:  employeeID,
			AwardType:   awards.TypeSpotRecognition,
			Title:       "Great Job",
			Description: "Did great work",
		}
		aw, err := svc.Create(ctx, orgID, createdBy, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aw.Status != awards.StatusDraft {
			t.Errorf("expected draft status, got %v", aw.Status)
		}
	})

	t.Run("Update", func(t *testing.T) {
		title := "Amazing Job"
		aw, err := svc.Update(ctx, orgID, "award-emp1", awards.UpdateAwardRequest{
			Title: &title,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aw.Title != title {
			t.Errorf("expected title %s, got %s", title, aw.Title)
		}
	})

	t.Run("Submit", func(t *testing.T) {
		aw, err := svc.Submit(ctx, orgID, "award-emp1", createdBy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aw.Status != awards.StatusApproved { // Because we mocked FindDefault to return nil
			t.Errorf("expected approved status, got %v", aw.Status)
		}
	})

	t.Run("Issue", func(t *testing.T) {
		aw, err := svc.Issue(ctx, orgID, "award-emp1", createdBy, awards.IssueRequest{
			CreateAnnouncement: false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if aw.Status != awards.StatusIssued {
			t.Errorf("expected issued status, got %v", aw.Status)
		}
	})

	t.Run("Cancel", func(t *testing.T) {
		req := awards.CreateAwardRequest{
			EmployeeID:  "emp2",
			AwardType:   awards.TypeSpotRecognition,
			Title:       "To be cancelled",
			Description: "desc",
		}
		aw, _ := svc.Create(ctx, orgID, createdBy, req)
		aw, _ = svc.Submit(ctx, orgID, aw.ID, createdBy)

		cancelledAw, err := svc.Cancel(ctx, orgID, aw.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cancelledAw.Status != awards.StatusCancelled {
			t.Errorf("expected cancelled status, got %v", cancelledAw.Status)
		}
	})

	t.Run("Cross-Org Isolation", func(t *testing.T) {
		_, err := svc.Get(ctx, "org2", "award-emp1")
		if err != awards.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org fetch, got %v", err)
		}
	})
}
