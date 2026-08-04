// backend/internal/hrm/leave/service.go
package leave

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/audit"
)

// Service defines business logic for HRM leave types and leave requests.
type Service interface {
	// Leave Types
	ListLeaveTypes(ctx context.Context, orgID string, activeOnly bool) (*LeaveTypeListResponse, error)
	GetLeaveType(ctx context.Context, orgID, ref string) (*LeaveType, error)
	CreateLeaveType(ctx context.Context, orgID, createdBy string, req CreateLeaveTypeRequest) (*LeaveType, error)
	UpdateLeaveType(ctx context.Context, orgID, ref string, req UpdateLeaveTypeRequest) (*LeaveType, error)
	DeleteLeaveType(ctx context.Context, orgID, ref string) error

	// Leave Requests
	ListRequests(ctx context.Context, orgID string, filter LeaveRequestFilter) (*LeaveRequestListResponse, error)
	GetRequest(ctx context.Context, orgID, ref string) (*LeaveRequest, error)
	CreateRequest(ctx context.Context, orgID, createdBy string, req CreateLeaveRequestRequest) (*LeaveRequest, error)
	ApproveRequest(ctx context.Context, orgID, ref, reviewerID string, req ReviewLeaveRequestRequest) (*LeaveRequest, error)
	RejectRequest(ctx context.Context, orgID, ref, reviewerID string, req ReviewLeaveRequestRequest) (*LeaveRequest, error)
	CancelRequest(ctx context.Context, orgID, ref, actorID string) (*LeaveRequest, error)
	DeleteRequest(ctx context.Context, orgID, ref, actorID string) error

	// Balances — see balances_service.go
	ListPolicies(ctx context.Context, orgID string) (*PolicyListResponse, error)
	GetPolicy(ctx context.Context, orgID, ref string) (*LeavePolicy, error)
	CreatePolicy(ctx context.Context, orgID, createdBy string, req CreatePolicyRequest) (*LeavePolicy, error)
	UpdatePolicy(ctx context.Context, orgID, ref string, req UpdatePolicyRequest) (*LeavePolicy, error)

	GetCurrentBalance(ctx context.Context, orgID, employeeID, leaveTypeID string) (*CurrentBalance, error)
	ListCurrentBalances(ctx context.Context, orgID, employeeID string) ([]*CurrentBalance, error)
	ListBalanceHistory(ctx context.Context, orgID, employeeID, leaveTypeID string, limit, offset int) (*BalanceHistoryResponse, error)
	ListTransactions(ctx context.Context, orgID, employeeID string, filter TransactionFilter) (*TransactionListResponse, error)

	PostAdjustment(ctx context.Context, orgID, employeeID, leaveTypeID, actorID string, req PostAdjustmentRequest) (*LeaveTransaction, error)
	PostEncashment(ctx context.Context, orgID, employeeID, leaveTypeID, actorID string, req PostEncashmentRequest) (*LeaveTransaction, error)

	// Scheduler entry points — asOfDate is an explicit param (mirrors
	// attendance.RunAbsenceSweep's own `date string` arg) so both jobs are
	// testable without manipulating wall-clock time.
	RunAccrual(ctx context.Context, orgID, systemUserID, asOfDate string) (int, error)
	RunCarryForward(ctx context.Context, orgID, systemUserID, asOfDate string) (int, error)
}

type serviceImpl struct {
	repo  Repository
	audit audit.Service
	db    *pgxpool.Pool
}

func NewService(repo Repository, auditSvc audit.Service, db *pgxpool.Pool) Service {
	return &serviceImpl{repo: repo, audit: auditSvc, db: db}
}

// ─────────────────────────────────────────────────────────
// Leave Type service methods
// ─────────────────────────────────────────────────────────

func (s *serviceImpl) ListLeaveTypes(ctx context.Context, orgID string, activeOnly bool) (*LeaveTypeListResponse, error) {
	list, err := s.repo.FindAllLeaveTypes(ctx, orgID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("leave: ListLeaveTypes: %w", err)
	}
	if list == nil {
		list = []*LeaveType{}
	}
	return &LeaveTypeListResponse{LeaveTypes: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetLeaveType(ctx context.Context, orgID, ref string) (*LeaveType, error) {
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: GetLeaveType: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}
	return lt, nil
}

func (s *serviceImpl) CreateLeaveType(ctx context.Context, orgID, createdBy string, req CreateLeaveTypeRequest) (*LeaveType, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrLeaveTypeNameReq
	}
	if len(name) > 100 {
		return nil, ErrLeaveTypeNameLong
	}
	conflict, err := s.repo.LeaveTypeExistsByName(ctx, orgID, name, "")
	if err != nil {
		return nil, fmt.Errorf("leave: CreateLeaveType: name check: %w", err)
	}
	if conflict {
		return nil, ErrLeaveTypeConflict
	}

	lt := &LeaveType{
		OrgID:            orgID,
		Name:             name,
		MaxDaysPerYear:   req.MaxDaysPerYear,
		IsPaid:           true,
		RequiresApproval: true,
		IsActive:         true,
		CreatedBy:        createdBy,
	}
	if req.Description != nil {
		lt.Description = req.Description
	}
	if req.IsPaid != nil {
		lt.IsPaid = *req.IsPaid
	}
	if req.RequiresApproval != nil {
		lt.RequiresApproval = *req.RequiresApproval
	}

	if err := s.repo.CreateLeaveType(ctx, lt); err != nil {
		return nil, fmt.Errorf("leave: CreateLeaveType: %w", err)
	}
	return lt, nil
}

func (s *serviceImpl) UpdateLeaveType(ctx context.Context, orgID, ref string, req UpdateLeaveTypeRequest) (*LeaveType, error) {
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: UpdateLeaveType: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrLeaveTypeNameReq
		}
		if len(name) > 100 {
			return nil, ErrLeaveTypeNameLong
		}
		conflict, err := s.repo.LeaveTypeExistsByName(ctx, orgID, name, lt.ID)
		if err != nil {
			return nil, fmt.Errorf("leave: UpdateLeaveType: name check: %w", err)
		}
		if conflict {
			return nil, ErrLeaveTypeConflict
		}
		lt.Name = name
	}
	if req.Description != nil {
		lt.Description = req.Description
	}
	if req.MaxDaysPerYear != nil {
		lt.MaxDaysPerYear = *req.MaxDaysPerYear
	}
	if req.IsPaid != nil {
		lt.IsPaid = *req.IsPaid
	}
	if req.RequiresApproval != nil {
		lt.RequiresApproval = *req.RequiresApproval
	}
	if req.IsActive != nil {
		lt.IsActive = *req.IsActive
	}

	if err := s.repo.UpdateLeaveType(ctx, lt); err != nil {
		return nil, fmt.Errorf("leave: UpdateLeaveType: %w", err)
	}
	return lt, nil
}

func (s *serviceImpl) DeleteLeaveType(ctx context.Context, orgID, ref string) error {
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("leave: DeleteLeaveType: %w", err)
	}
	if lt == nil {
		return ErrLeaveTypeNotFound
	}
	if err := s.repo.DeleteLeaveType(ctx, orgID, ref); err != nil {
		return fmt.Errorf("leave: DeleteLeaveType: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────
// Leave Request service methods
// ─────────────────────────────────────────────────────────

func (s *serviceImpl) ListRequests(ctx context.Context, orgID string, filter LeaveRequestFilter) (*LeaveRequestListResponse, error) {
	filter.Normalise()

	list, err := s.repo.FindAllRequests(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("leave: ListRequests: %w", err)
	}
	if list == nil {
		list = []*LeaveRequest{}
	}
	total, err := s.repo.CountRequests(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("leave: ListRequests: count: %w", err)
	}
	return &LeaveRequestListResponse{
		Requests: list, Total: total, Limit: filter.Limit, Offset: filter.Offset,
	}, nil
}

func (s *serviceImpl) GetRequest(ctx context.Context, orgID, ref string) (*LeaveRequest, error) {
	lr, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: GetRequest: %w", err)
	}
	if lr == nil {
		return nil, ErrLeaveRequestNotFound
	}
	return lr, nil
}

func (s *serviceImpl) CreateRequest(ctx context.Context, orgID, createdBy string, req CreateLeaveRequestRequest) (*LeaveRequest, error) {
	// Validate required fields
	if strings.TrimSpace(req.EmployeeID) == "" {
		return nil, ErrEmployeeIDRequired
	}
	if strings.TrimSpace(req.LeaveTypeID) == "" {
		return nil, ErrLeaveTypeIDRequired
	}
	if strings.TrimSpace(req.StartDate) == "" {
		return nil, ErrStartDateRequired
	}
	if strings.TrimSpace(req.EndDate) == "" {
		return nil, ErrEndDateRequired
	}
	if req.TotalDays <= 0 {
		return nil, ErrInvalidTotalDays
	}

	startDate, err := time.Parse(dateLayout, strings.TrimSpace(req.StartDate))
	if err != nil {
		return nil, ErrInvalidStartDate
	}
	endDate, err := time.Parse(dateLayout, strings.TrimSpace(req.EndDate))
	if err != nil {
		return nil, ErrInvalidEndDate
	}
	if endDate.Before(startDate) {
		return nil, ErrEndBeforeStart
	}

	// Verify leave type exists and is active
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, strings.TrimSpace(req.LeaveTypeID))
	if err != nil {
		return nil, fmt.Errorf("leave: CreateRequest: type check: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}
	if !lt.IsActive {
		return nil, ErrLeaveTypeInactive
	}

	// If leave type requires approval → start as pending; else auto-approve
	status := LeaveRequestStatusPending
	if !lt.RequiresApproval {
		status = LeaveRequestStatusApproved
	}

	lr := &LeaveRequest{
		OrgID:       orgID,
		EmployeeID:  strings.TrimSpace(req.EmployeeID),
		LeaveTypeID: strings.TrimSpace(req.LeaveTypeID),
		StartDate:   startDate,
		EndDate:     endDate,
		TotalDays:   req.TotalDays,
		Reason:      req.Reason,
		Status:      status,
		CreatedBy:   createdBy,
	}

	// Auto-approved requests (leave type doesn't require approval) post a
	// usage debit immediately if a balance policy is active for this leave
	// type — this branch never goes through ApproveRequest, so posting only
	// there would silently skip balance tracking for every leave type with
	// requires_approval=false. Leave types with no policy are untouched
	// (opt-in): this call returns nil, nil in that case.
	var policy *LeavePolicy
	if status == LeaveRequestStatusApproved {
		var perr error
		policy, perr = s.repo.FindPolicyByLeaveType(ctx, orgID, lt.ID)
		if perr != nil {
			return nil, fmt.Errorf("leave: CreateRequest: policy check: %w", perr)
		}
	}

	if policy != nil {
		if err := s.createRequestWithUsage(ctx, lr, policy, createdBy); err != nil {
			return nil, fmt.Errorf("leave: CreateRequest: %w", err)
		}
	} else if err := s.repo.CreateRequest(ctx, lr); err != nil {
		return nil, fmt.Errorf("leave: CreateRequest: %w", err)
	}

	s.audit.Log(ctx, audit.EventHRMLeaveRequested, createdBy, orgID, "", "", map[string]string{
		"leave_request_id": lr.ID, "employee_id": lr.EmployeeID, "leave_type_id": lr.LeaveTypeID,
	})

	return lr, nil
}

func (s *serviceImpl) ApproveRequest(ctx context.Context, orgID, ref, reviewerID string, req ReviewLeaveRequestRequest) (*LeaveRequest, error) {
	lr, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: ApproveRequest: %w", err)
	}
	if lr == nil {
		return nil, ErrLeaveRequestNotFound
	}
	if lr.Status != LeaveRequestStatusPending {
		return nil, ErrNotPending
	}

	now := time.Now()
	lr.Status = LeaveRequestStatusApproved
	lr.ReviewedBy = &reviewerID
	lr.ReviewedAt = &now
	lr.ReviewNote = req.Note

	// Posts a usage debit if a balance policy is active for this leave type
	// (opt-in). Approving a request that drives the balance negative always
	// succeeds — there is no balance-sufficiency gate anywhere in this path.
	policy, err := s.repo.FindPolicyByLeaveType(ctx, orgID, lr.LeaveTypeID)
	if err != nil {
		return nil, fmt.Errorf("leave: ApproveRequest: policy check: %w", err)
	}
	if policy != nil {
		if err := s.updateRequestWithLedger(ctx, lr, policy, TransactionUsage, -lr.TotalDays, reviewerID); err != nil {
			return nil, fmt.Errorf("leave: ApproveRequest: %w", err)
		}
	} else if err := s.repo.UpdateRequest(ctx, lr); err != nil {
		return nil, fmt.Errorf("leave: ApproveRequest: %w", err)
	}

	s.audit.Log(ctx, audit.EventHRMLeaveApproved, reviewerID, orgID, "", "", map[string]string{
		"leave_request_id": lr.ID, "employee_id": lr.EmployeeID,
	})

	return lr, nil
}

func (s *serviceImpl) RejectRequest(ctx context.Context, orgID, ref, reviewerID string, req ReviewLeaveRequestRequest) (*LeaveRequest, error) {
	lr, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: RejectRequest: %w", err)
	}
	if lr == nil {
		return nil, ErrLeaveRequestNotFound
	}
	if lr.Status != LeaveRequestStatusPending {
		return nil, ErrNotPending
	}

	now := time.Now()
	lr.Status = LeaveRequestStatusRejected
	lr.ReviewedBy = &reviewerID
	lr.ReviewedAt = &now
	lr.ReviewNote = req.Note

	if err := s.repo.UpdateRequest(ctx, lr); err != nil {
		return nil, fmt.Errorf("leave: RejectRequest: %w", err)
	}

	s.audit.Log(ctx, audit.EventHRMLeaveRejected, reviewerID, orgID, "", "", map[string]string{
		"leave_request_id": lr.ID, "employee_id": lr.EmployeeID,
	})

	return lr, nil
}

func (s *serviceImpl) CancelRequest(ctx context.Context, orgID, ref, actorID string) (*LeaveRequest, error) {
	lr, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: CancelRequest: %w", err)
	}
	if lr == nil {
		return nil, ErrLeaveRequestNotFound
	}
	if lr.Status == LeaveRequestStatusCancelled {
		return nil, ErrAlreadyCancelled
	}
	wasApproved := lr.Status == LeaveRequestStatusApproved

	now := time.Now()
	lr.Status = LeaveRequestStatusCancelled
	lr.ReviewedBy = &actorID
	lr.ReviewedAt = &now

	// Cancelling an already-approved (usage-posted) request reverses the
	// usage debit if a balance policy is active. Cancelling from `pending`
	// posts nothing — nothing was ever debited.
	var policy *LeavePolicy
	if wasApproved {
		var perr error
		policy, perr = s.repo.FindPolicyByLeaveType(ctx, orgID, lr.LeaveTypeID)
		if perr != nil {
			return nil, fmt.Errorf("leave: CancelRequest: policy check: %w", perr)
		}
	}

	if policy != nil {
		if err := s.updateRequestWithLedger(ctx, lr, policy, TransactionUsageReversal, lr.TotalDays, actorID); err != nil {
			return nil, fmt.Errorf("leave: CancelRequest: %w", err)
		}
	} else if err := s.repo.UpdateRequest(ctx, lr); err != nil {
		return nil, fmt.Errorf("leave: CancelRequest: %w", err)
	}
	return lr, nil
}

// DeleteRequest hard-deletes a leave request. If it was approved (and thus
// usage-posted) and a balance policy is active for its leave type, the
// usage is reversed first — this is not in the original Phase 2 ask, but
// has the identical bug shape as CancelRequest: DeleteRequest is reachable
// from any status today, including approved, and the FK is ON DELETE SET
// NULL, so an un-reversed delete would permanently overstate usage with an
// orphaned debit.
func (s *serviceImpl) DeleteRequest(ctx context.Context, orgID, ref, actorID string) error {
	lr, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("leave: DeleteRequest: %w", err)
	}
	if lr == nil {
		return ErrLeaveRequestNotFound
	}

	if lr.Status == LeaveRequestStatusApproved {
		policy, perr := s.repo.FindPolicyByLeaveType(ctx, orgID, lr.LeaveTypeID)
		if perr != nil {
			return fmt.Errorf("leave: DeleteRequest: policy check: %w", perr)
		}
		if policy != nil {
			if err := s.deleteRequestWithReversal(ctx, lr, policy, actorID); err != nil {
				return fmt.Errorf("leave: DeleteRequest: %w", err)
			}
			return nil
		}
	}

	if err := s.repo.DeleteRequest(ctx, orgID, ref); err != nil {
		return fmt.Errorf("leave: DeleteRequest: %w", err)
	}
	return nil
}
