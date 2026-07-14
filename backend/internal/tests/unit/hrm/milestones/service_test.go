package milestones_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mridha/businesssaas/internal/hrm/milestones"
)

type stubRepo struct {
	data map[string]*milestones.Milestone
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		data: make(map[string]*milestones.Milestone),
	}
}

func (r *stubRepo) FindAll(ctx context.Context, orgID, employeeID, milestoneType string, upcoming bool) ([]*milestones.Milestone, error) {
	var res []*milestones.Milestone
	for _, m := range r.data {
		if m.OrgID == orgID {
			if employeeID != "" && m.EmployeeID != employeeID {
				continue
			}
			if milestoneType != "" && string(m.MilestoneType) != milestoneType {
				continue
			}
			if upcoming && m.IsAcknowledged {
				continue
			}
			res = append(res, m)
		}
	}
	return res, nil
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, ref string) (*milestones.Milestone, error) {
	for _, m := range r.data {
		if m.OrgID == orgID && (m.ID == ref || m.PublicID == ref) {
			return m, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) Create(ctx context.Context, m *milestones.Milestone) error {
	m.ID = uuid.NewString()
	m.PublicID = "mil_" + m.ID
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	r.data[m.ID] = m
	return nil
}

func (r *stubRepo) SetCascadeLinks(ctx context.Context, id string, awardID, announcementID, calendarEventID *string) error {
	if m, ok := r.data[id]; ok {
		if awardID != nil {
			m.AutoAwardID = awardID
		}
		if announcementID != nil {
			m.AutoAnnouncementID = announcementID
		}
		if calendarEventID != nil {
			m.AutoCalendarEventID = calendarEventID
		}
		m.UpdatedAt = time.Now()
	}
	return nil
}

func (r *stubRepo) Acknowledge(ctx context.Context, id string) error {
	if m, ok := r.data[id]; ok {
		m.IsAcknowledged = true
		now := time.Now()
		m.AcknowledgedAt = &now
		m.UpdatedAt = time.Now()
	}
	return nil
}

func TestMilestonesService(t *testing.T) {
	repo := newStubRepo()
	svc := milestones.NewService(repo, nil) // passing nil for db
	ctx := context.Background()
	orgID := "org1"
	empID := "emp1"

	t.Run("Create Milestone", func(t *testing.T) {
		req := milestones.CreateMilestoneRequest{
			EmployeeID: empID,
			Title: "5 Year Anniversary",
			MilestoneType: milestones.TypeWorkAnniversary,
			MilestoneDate: "2026-08-01",
		}
		m, err := svc.Create(ctx, orgID, "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if m.Title != "5 Year Anniversary" {
			t.Errorf("expected 5 Year Anniversary, got %s", m.Title)
		}
		if m.IsAcknowledged {
			t.Errorf("expected not acknowledged")
		}
	})

	t.Run("Validation Create", func(t *testing.T) {
		req := milestones.CreateMilestoneRequest{Title: ""}
		_, err := svc.Create(ctx, orgID, "admin", req)
		if err != milestones.ErrEmployeeIDRequired {
			t.Errorf("expected ErrEmployeeIDRequired, got %v", err)
		}
		
		req2 := milestones.CreateMilestoneRequest{EmployeeID: empID, Title: ""}
		_, err = svc.Create(ctx, orgID, "admin", req2)
		if err != milestones.ErrTitleRequired {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
	})

	t.Run("Get and Acknowledge", func(t *testing.T) {
		req := milestones.CreateMilestoneRequest{
			EmployeeID: empID, 
			Title: "Test",
			MilestoneType: milestones.TypeProbationComplete,
			MilestoneDate: "2026-08-01",
		}
		m, _ := svc.Create(ctx, orgID, "admin", req)

		fetched, err := svc.Get(ctx, orgID, m.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.Title != "Test" {
			t.Errorf("expected Test, got %s", fetched.Title)
		}

		ack, err := svc.Acknowledge(ctx, orgID, m.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !ack.IsAcknowledged {
			t.Errorf("expected to be acknowledged")
		}
		if ack.AcknowledgedAt == nil {
			t.Errorf("expected acknowledged_at to be set")
		}
		
		_, err = svc.Acknowledge(ctx, orgID, m.ID)
		if err != milestones.ErrAlreadyAcknowledged {
			t.Errorf("expected ErrAlreadyAcknowledged, got %v", err)
		}
	})

	t.Run("List Milestones", func(t *testing.T) {
		req := milestones.CreateMilestoneRequest{
			EmployeeID: empID, 
			Title: "Test List",
			MilestoneType: milestones.TypeContractRenewal,
			MilestoneDate: "2026-08-01",
		}
		m, _ := svc.Create(ctx, orgID, "admin", req)

		list, err := svc.List(ctx, orgID, empID, "", false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		found := false
		for _, v := range list.Milestones {
			if v.ID == m.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("expected to find milestone in list")
		}
	})

	t.Run("GenerateUpcoming Validation", func(t *testing.T) {
		req := milestones.GenerateRequest{Year: 0, Month: 0}
		_, err := svc.GenerateUpcoming(ctx, orgID, "admin", req)
		if err == nil {
			t.Errorf("expected error for missing year/month")
		}
	})
	
	t.Run("Cross-Org Privacy", func(t *testing.T) {
		req := milestones.CreateMilestoneRequest{
			EmployeeID: empID, 
			Title: "T",
			MilestoneType: milestones.TypeBirthday,
			MilestoneDate: "2026-08-01",
		}
		m, _ := svc.Create(ctx, orgID, "admin", req)

		_, err := svc.Get(ctx, "org2", m.ID)
		if err != milestones.ErrNotFound {
			t.Errorf("expected ErrNotFound for wrong org, got %v", err)
		}
	})
}
