package leave_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/hrm/leave"
)

type mockAudit struct{}

func (m *mockAudit) Log(ctx context.Context, event audit.EventType, userID, businessID, ip, ua string, metadata any) {
}

type stubRepo struct {
	leaveTypes map[string]*leave.LeaveType
	requests   map[string]*leave.LeaveRequest
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		leaveTypes: make(map[string]*leave.LeaveType),
		requests:   make(map[string]*leave.LeaveRequest),
	}
}

func (s *stubRepo) FindAllLeaveTypes(ctx context.Context, orgID string, activeOnly bool) ([]*leave.LeaveType, error) {
	var list []*leave.LeaveType
	for _, lt := range s.leaveTypes {
		if lt.OrgID == orgID {
			if !activeOnly || lt.IsActive {
				list = append(list, lt)
			}
		}
	}
	return list, nil
}

func (s *stubRepo) FindLeaveTypeByRef(ctx context.Context, orgID, ref string) (*leave.LeaveType, error) {
	for _, lt := range s.leaveTypes {
		if lt.OrgID == orgID && (lt.ID == ref || lt.PublicID == ref) {
			return lt, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) CreateLeaveType(ctx context.Context, lt *leave.LeaveType) error {
	lt.ID = "lt_" + time.Now().Format("20060102150405.000")
	s.leaveTypes[lt.ID] = lt
	return nil
}

func (s *stubRepo) UpdateLeaveType(ctx context.Context, lt *leave.LeaveType) error {
	s.leaveTypes[lt.ID] = lt
	return nil
}

func (s *stubRepo) DeleteLeaveType(ctx context.Context, orgID, ref string) error {
	for id, lt := range s.leaveTypes {
		if lt.OrgID == orgID && (lt.ID == ref || lt.PublicID == ref) {
			delete(s.leaveTypes, id)
			return nil
		}
	}
	return leave.ErrLeaveTypeNotFound
}

func (s *stubRepo) LeaveTypeExistsByName(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	for _, lt := range s.leaveTypes {
		if lt.OrgID == orgID && lt.Name == name && lt.IsActive && lt.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubRepo) FindAllRequests(ctx context.Context, orgID string, filter leave.LeaveRequestFilter) ([]*leave.LeaveRequest, error) {
	var list []*leave.LeaveRequest
	for _, r := range s.requests {
		if r.OrgID == orgID {
			if filter.EmployeeID != "" && r.EmployeeID != filter.EmployeeID {
				continue
			}
			if filter.LeaveTypeID != "" && r.LeaveTypeID != filter.LeaveTypeID {
				continue
			}
			if filter.Status != "" && r.Status != filter.Status {
				continue
			}
			list = append(list, r)
		}
	}
	return list, nil
}

func (s *stubRepo) CountRequests(ctx context.Context, orgID string, filter leave.LeaveRequestFilter) (int, error) {
	list, _ := s.FindAllRequests(ctx, orgID, filter)
	return len(list), nil
}

func (s *stubRepo) FindRequestByRef(ctx context.Context, orgID, ref string) (*leave.LeaveRequest, error) {
	for _, r := range s.requests {
		if r.OrgID == orgID && (r.ID == ref || r.PublicID == ref) {
			return r, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) CreateRequest(ctx context.Context, r *leave.LeaveRequest) error {
	r.ID = "req_" + time.Now().Format("20060102150405.000")
	s.requests[r.ID] = r
	return nil
}

func (s *stubRepo) UpdateRequest(ctx context.Context, r *leave.LeaveRequest) error {
	s.requests[r.ID] = r
	return nil
}

func (s *stubRepo) DeleteRequest(ctx context.Context, orgID, ref string) error {
	for id, r := range s.requests {
		if r.OrgID == orgID && (r.ID == ref || r.PublicID == ref) {
			delete(s.requests, id)
			return nil
		}
	}
	return leave.ErrLeaveRequestNotFound
}

func TestLeaveTypeService(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{})
	ctx := context.Background()

	orgID := "org_1"
	createdBy := "user_1"

	// Create
	req := leave.CreateLeaveTypeRequest{
		Name: "Annual Leave",
	}
	lt, err := svc.CreateLeaveType(ctx, orgID, createdBy, req)
	if err != nil {
		t.Fatalf("CreateLeaveType failed: %v", err)
	}
	if lt.Name != "Annual Leave" {
		t.Errorf("Expected Name 'Annual Leave', got %s", lt.Name)
	}

	// Conflict
	_, err = svc.CreateLeaveType(ctx, orgID, createdBy, req)
	if err != leave.ErrLeaveTypeConflict {
		t.Errorf("Expected ErrLeaveTypeConflict, got %v", err)
	}

	// Update
	newName := "Updated Leave"
	updateReq := leave.UpdateLeaveTypeRequest{
		Name: &newName,
	}
	updated, err := svc.UpdateLeaveType(ctx, orgID, lt.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateLeaveType failed: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("Expected Name '%s', got %s", newName, updated.Name)
	}

	// Get
	fetched, err := svc.GetLeaveType(ctx, orgID, lt.ID)
	if err != nil {
		t.Fatalf("GetLeaveType failed: %v", err)
	}
	if fetched.ID != lt.ID {
		t.Errorf("ID mismatch")
	}

	// Delete
	err = svc.DeleteLeaveType(ctx, orgID, lt.ID)
	if err != nil {
		t.Fatalf("DeleteLeaveType failed: %v", err)
	}

	// Cross-org
	err = svc.DeleteLeaveType(ctx, "org_2", lt.ID)
	if err != leave.ErrLeaveTypeNotFound {
		t.Errorf("Expected ErrLeaveTypeNotFound for cross-org delete")
	}
}

func TestLeaveRequestService(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{})
	ctx := context.Background()

	orgID := "org_1"
	createdBy := "user_1"

	// Add leave type
	ltReq := leave.CreateLeaveTypeRequest{
		Name: "Sick Leave",
	}
	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, ltReq)

	// Create request
	req := leave.CreateLeaveRequestRequest{
		EmployeeID:  "emp_1",
		LeaveTypeID: lt.ID,
		StartDate:   "2023-10-01",
		EndDate:     "2023-10-05",
		TotalDays:   5,
	}
	lr, err := svc.CreateRequest(ctx, orgID, createdBy, req)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}
	if lr.Status != leave.LeaveRequestStatusPending {
		t.Errorf("Expected Pending status")
	}

	// Approve
	note := "Approved"
	approved, err := svc.ApproveRequest(ctx, orgID, lr.ID, "reviewer_1", leave.ReviewLeaveRequestRequest{Note: &note})
	if err != nil {
		t.Fatalf("ApproveRequest failed: %v", err)
	}
	if approved.Status != leave.LeaveRequestStatusApproved {
		t.Errorf("Expected Approved status")
	}

	// Reject (should fail because not pending)
	_, err = svc.RejectRequest(ctx, orgID, lr.ID, "reviewer_1", leave.ReviewLeaveRequestRequest{Note: &note})
	if err != leave.ErrNotPending {
		t.Errorf("Expected ErrNotPending, got %v", err)
	}

	// Create another and Cancel
	lr2, _ := svc.CreateRequest(ctx, orgID, createdBy, req)
	cancelled, err := svc.CancelRequest(ctx, orgID, lr2.ID, createdBy)
	if err != nil {
		t.Fatalf("CancelRequest failed: %v", err)
	}
	if cancelled.Status != leave.LeaveRequestStatusCancelled {
		t.Errorf("Expected Cancelled status")
	}

	// Get
	fetched, err := svc.GetRequest(ctx, orgID, lr.ID)
	if err != nil {
		t.Fatalf("GetRequest failed: %v", err)
	}
	if fetched.ID != lr.ID {
		t.Errorf("ID mismatch")
	}
}
