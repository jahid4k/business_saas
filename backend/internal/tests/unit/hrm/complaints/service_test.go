package complaints_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/complaints"
)

type stubRepo struct {
	data map[string]*complaints.Complaint
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		data: make(map[string]*complaints.Complaint),
	}
}

func (r *stubRepo) FindAll(ctx context.Context, orgID string, filter complaints.ComplaintListFilter) ([]*complaints.Complaint, error) {
	var res []*complaints.Complaint
	for _, c := range r.data {
		if c.OrgID == orgID {
			if filter.EmployeeID != "" && c.EmployeeID != filter.EmployeeID {
				continue
			}
			if filter.Status != "" && string(c.Status) != filter.Status {
				continue
			}
			res = append(res, c)
		}
	}
	return res, nil
}

func (r *stubRepo) Count(ctx context.Context, orgID string, filter complaints.ComplaintListFilter) (int, error) {
	out, err := r.FindAll(ctx, orgID, filter)
	return len(out), err
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*complaints.Complaint, error) {
	for _, c := range r.data {
		if c.OrgID == orgID && (c.ID == ref || c.PublicID == ref) {
			if employeeID != "" && c.EmployeeID != employeeID {
				continue
			}
			return c, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) Create(ctx context.Context, c *complaints.Complaint) error {
	c.ID = uuid.NewString()
	c.PublicID = "cpl_" + c.ID
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	r.data[c.ID] = c
	return nil
}

func (r *stubRepo) Update(ctx context.Context, c *complaints.Complaint) error {
	c.UpdatedAt = time.Now()
	r.data[c.ID] = c
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status complaints.ComplaintStatus) error {
	if c, ok := r.data[id]; ok {
		c.Status = status
		c.UpdatedAt = time.Now()
	}
	return nil
}

func TestComplaintsService(t *testing.T) {
	repo := newStubRepo()
	svc := complaints.NewService(repo, nil)
	ctx := context.Background()
	orgID := "org1"
	empID := "emp1"

	t.Run("Create Complaint", func(t *testing.T) {
		req := complaints.CreateComplaintRequest{
			Title: "Test Complaint",
			Description: "Details",
			ComplaintType: complaints.TypeGeneral,
		}
		c, err := svc.Create(ctx, orgID, empID, "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if c.Title != "Test Complaint" {
			t.Errorf("expected Test Complaint, got %s", c.Title)
		}
		if c.Status != complaints.StatusSubmitted {
			t.Errorf("expected submitted status, got %s", c.Status)
		}
	})

	t.Run("Validation Create", func(t *testing.T) {
		req := complaints.CreateComplaintRequest{Title: ""}
		_, err := svc.Create(ctx, orgID, empID, "admin", req)
		if err != complaints.ErrTitleRequired {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
	})

	t.Run("Get and Update Complaint", func(t *testing.T) {
		req := complaints.CreateComplaintRequest{Title: "T", Description: "D"}
		c, _ := svc.Create(ctx, orgID, empID, "admin", req)
		
		newDesc := "New D"
		updateReq := complaints.UpdateComplaintRequest{Description: &newDesc}
		updated, err := svc.Update(ctx, orgID, empID, c.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Description != "New D" {
			t.Errorf("expected New D, got %s", updated.Description)
		}
		
		fetched, err := svc.Get(ctx, orgID, empID, c.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.Description != "New D" {
			t.Errorf("expected New D, got %s", fetched.Description)
		}
	})

	t.Run("Status Transitions", func(t *testing.T) {
		req := complaints.CreateComplaintRequest{Title: "Status Test", Description: "D"}
		c, _ := svc.Create(ctx, orgID, empID, "admin", req)

		// StartReview
		rev, err := svc.StartReview(ctx, orgID, empID, c.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if rev.Status != complaints.StatusUnderReview {
			t.Errorf("expected under review, got %s", rev.Status)
		}

		// Assign
		assignReq := complaints.AssignRequest{InvestigatorID: "inv1"}
		assigned, err := svc.Assign(ctx, orgID, empID, c.ID, assignReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if assigned.Status != complaints.StatusInvestigating {
			t.Errorf("expected investigating, got %s", assigned.Status)
		}

		// Resolve
		resolveReq := complaints.ResolveRequest{Resolution: "Fixed"}
		resolved, err := svc.Resolve(ctx, orgID, empID, c.ID, "hr1", resolveReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resolved.Status != complaints.StatusResolved {
			t.Errorf("expected resolved, got %s", resolved.Status)
		}
		
		// Withdraw (should fail since it's resolved)
		_, err = svc.Withdraw(ctx, orgID, empID, c.ID)
		if err == nil {
			t.Errorf("expected error when withdrawing resolved complaint")
		}
	})
	
	t.Run("Cross-Org and Cross-Employee Privacy", func(t *testing.T) {
		req := complaints.CreateComplaintRequest{Title: "T", Description: "D"}
		c, _ := svc.Create(ctx, orgID, empID, "admin", req)
		
		_, err := svc.Get(ctx, "org2", empID, c.ID)
		if err != complaints.ErrNotFound {
			t.Errorf("expected ErrNotFound for wrong org, got %v", err)
		}
		
		// Assuming getting it with a different employeeID should act as an access check
		_, err = svc.Get(ctx, orgID, "emp2", c.ID)
		if err != complaints.ErrNotFound {
			t.Errorf("expected ErrNotFound for wrong employee, got %v", err)
		}
	})
	
	t.Run("Dismiss and List", func(t *testing.T) {
		req := complaints.CreateComplaintRequest{Title: "Dismiss Me", Description: "D"}
		c, _ := svc.Create(ctx, orgID, empID, "admin", req)
		
		_, err := svc.StartReview(ctx, orgID, empID, c.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		
		dismissReq := complaints.DismissRequest{Resolution: "Not an issue"}
		dismissed, err := svc.Dismiss(ctx, orgID, empID, c.ID, "hr", dismissReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if dismissed.Status != complaints.StatusDismissed {
			t.Errorf("expected dismissed, got %s", dismissed.Status)
		}
		
		list, err := svc.List(ctx, orgID, complaints.ComplaintListFilter{EmployeeID: empID, Scope: authz.ScopeAll})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if list.Total == 0 {
			t.Errorf("expected to find complaints")
		}
	})
}
