package leave_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/hrm/leave"
)

type mockAudit struct{}

func (m *mockAudit) Log(ctx context.Context, event audit.EventType, userID, businessID, ip, ua string, metadata any) {
}

// newDummyPool returns an unconnected pool — sufficient for these stub-repo
// tests, which never seed a leave policy and therefore never exercise the
// s.db.Begin(ctx) transactional paths (those only run when an active policy
// exists for the request's leave type).
func newDummyPool() *pgxpool.Pool {
	cfg, _ := pgxpool.ParseConfig("postgres://dummy:dummy@127.0.0.1:5432/dummy?sslmode=disable")
	pool, _ := pgxpool.NewWithConfig(context.Background(), cfg)
	return pool
}

type stubRepo struct {
	leaveTypes   map[string]*leave.LeaveType
	requests     map[string]*leave.LeaveRequest
	policies     map[string]*leave.LeavePolicy
	balances     map[string]*leave.LeaveBalance
	transactions map[string]*leave.LeaveTransaction
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		leaveTypes:   make(map[string]*leave.LeaveType),
		requests:     make(map[string]*leave.LeaveRequest),
		policies:     make(map[string]*leave.LeavePolicy),
		balances:     make(map[string]*leave.LeaveBalance),
		transactions: make(map[string]*leave.LeaveTransaction),
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

// ── Balance repository stub methods ─────────────────────────────────────

func (s *stubRepo) FindPolicyByLeaveType(ctx context.Context, orgID, leaveTypeID string) (*leave.LeavePolicy, error) {
	for _, p := range s.policies {
		if p.OrgID == orgID && p.LeaveTypeID == leaveTypeID && p.IsActive {
			return p, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) FindPolicyByRef(ctx context.Context, orgID, ref string) (*leave.LeavePolicy, error) {
	for _, p := range s.policies {
		if p.OrgID == orgID && (p.ID == ref || p.PublicID == ref) {
			return p, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) FindAllPolicies(ctx context.Context, orgID string) ([]*leave.LeavePolicy, error) {
	var list []*leave.LeavePolicy
	for _, p := range s.policies {
		if p.OrgID == orgID {
			list = append(list, p)
		}
	}
	return list, nil
}

func (s *stubRepo) CreatePolicy(ctx context.Context, p *leave.LeavePolicy) error {
	p.ID = "lvp_" + time.Now().Format("20060102150405.000000")
	s.policies[p.ID] = p
	return nil
}

func (s *stubRepo) UpdatePolicy(ctx context.Context, p *leave.LeavePolicy) error {
	if _, ok := s.policies[p.ID]; !ok {
		return leave.ErrPolicyNotFound
	}
	s.policies[p.ID] = p
	return nil
}

func (s *stubRepo) FindLatestBalanceSnapshot(ctx context.Context, orgID, employeeID, leaveTypeID string) (*leave.LeaveBalance, error) {
	var latest *leave.LeaveBalance
	for _, b := range s.balances {
		if b.OrgID != orgID || b.EmployeeID != employeeID || b.LeaveTypeID != leaveTypeID {
			continue
		}
		if latest == nil || b.PeriodYear > latest.PeriodYear ||
			(b.PeriodYear == latest.PeriodYear && b.PeriodMonth > latest.PeriodMonth) {
			latest = b
		}
	}
	return latest, nil
}

func (s *stubRepo) FindLatestBalanceSnapshotAsOf(ctx context.Context, orgID, employeeID, leaveTypeID, throughDate string) (*leave.LeaveBalance, error) {
	var latest *leave.LeaveBalance
	for _, b := range s.balances {
		if b.OrgID != orgID || b.EmployeeID != employeeID || b.LeaveTypeID != leaveTypeID || b.AsOfDate > throughDate {
			continue
		}
		if latest == nil || b.AsOfDate > latest.AsOfDate {
			latest = b
		}
	}
	return latest, nil
}

func (s *stubRepo) FindBalanceHistory(ctx context.Context, orgID, employeeID, leaveTypeID string, limit, offset int) ([]*leave.LeaveBalance, int, error) {
	var list []*leave.LeaveBalance
	for _, b := range s.balances {
		if b.OrgID == orgID && b.EmployeeID == employeeID && b.LeaveTypeID == leaveTypeID {
			list = append(list, b)
		}
	}
	return list, len(list), nil
}

func (s *stubRepo) CreateBalanceSnapshot(ctx context.Context, b *leave.LeaveBalance) error {
	b.ID = "lvb_" + time.Now().Format("20060102150405.000000")
	s.balances[b.ID] = b
	return nil
}

// SumTransactionsSince/SumTransactionsByType: a snapshot's AsOfDate is the
// EXCLUSIVE upper bound of what it covers, so a transaction dated exactly
// sinceDate has NOT been folded into that snapshot and must count — the
// comparison is inclusive (>=), matching repoImpl's real SQL.
func (s *stubRepo) SumTransactionsSince(ctx context.Context, orgID, employeeID, leaveTypeID string, sinceDate *string) (float64, error) {
	var sum float64
	for _, t := range s.transactions {
		if t.OrgID != orgID || t.EmployeeID != employeeID || t.LeaveTypeID != leaveTypeID {
			continue
		}
		if sinceDate != nil && t.TransactionDate < *sinceDate {
			continue
		}
		sum += t.Days
	}
	return sum, nil
}

// SumEncashmentsByLeaveType mirrors the real query's sign handling:
// encashment rows are stored with NEGATIVE days, and the sum is negated so
// callers get the positive day count. Getting that backwards would turn an
// F&F credit into a debit.
func (s *stubRepo) SumEncashmentsByLeaveType(ctx context.Context, orgID, employeeID string) ([]*leave.EncashmentSummary, error) {
	byType := map[string]float64{}
	order := []string{}
	for _, t := range s.transactions {
		if t.OrgID != orgID || t.EmployeeID != employeeID {
			continue
		}
		if t.TransactionType != leave.TransactionEncashment {
			continue
		}
		if _, seen := byType[t.LeaveTypeID]; !seen {
			order = append(order, t.LeaveTypeID)
		}
		byType[t.LeaveTypeID] += -t.Days
	}
	out := make([]*leave.EncashmentSummary, 0, len(order))
	for _, id := range order {
		if byType[id] <= 0 {
			continue
		}
		e := &leave.EncashmentSummary{LeaveTypeID: id, LeaveTypeName: id, Days: byType[id]}
		// policies is keyed by policy id, not leave type — find by
		// LeaveTypeID exactly as FindPolicyByLeaveType does.
		for _, p := range s.policies {
			if p.OrgID == orgID && p.LeaveTypeID == id && p.IsActive {
				e.RateBasis = p.EncashmentRateBasis
				break
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *stubRepo) SumTransactionsByType(ctx context.Context, orgID, employeeID, leaveTypeID string, sinceDate *string, throughDate string) (*leave.PeriodTransactionSums, error) {
	sums := &leave.PeriodTransactionSums{}
	for _, t := range s.transactions {
		if t.OrgID != orgID || t.EmployeeID != employeeID || t.LeaveTypeID != leaveTypeID {
			continue
		}
		if sinceDate != nil && t.TransactionDate < *sinceDate {
			continue
		}
		if t.TransactionDate >= throughDate {
			continue
		}
		switch t.TransactionType {
		case leave.TransactionAccrual:
			sums.Accrued += t.Days
		case leave.TransactionUsage, leave.TransactionUsageReversal:
			sums.Taken -= t.Days
		case leave.TransactionEncashment:
			sums.Encashed -= t.Days
		case leave.TransactionCarryForward, leave.TransactionForfeiture:
			sums.CarriedForward += t.Days
		case leave.TransactionAdjustment:
			sums.Adjusted += t.Days
		}
	}
	return sums, nil
}

func (s *stubRepo) FindTransactions(ctx context.Context, orgID, employeeID string, filter leave.TransactionFilter) ([]*leave.LeaveTransaction, error) {
	var list []*leave.LeaveTransaction
	for _, t := range s.transactions {
		if t.OrgID != orgID || t.EmployeeID != employeeID {
			continue
		}
		if filter.LeaveTypeID != "" && t.LeaveTypeID != filter.LeaveTypeID {
			continue
		}
		if filter.TransactionType != "" && t.TransactionType != filter.TransactionType {
			continue
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *stubRepo) CountTransactions(ctx context.Context, orgID, employeeID string, filter leave.TransactionFilter) (int, error) {
	list, _ := s.FindTransactions(ctx, orgID, employeeID, filter)
	return len(list), nil
}

func (s *stubRepo) CreateTransaction(ctx context.Context, t *leave.LeaveTransaction) error {
	t.ID = "lvx_" + time.Now().Format("20060102150405.000000")
	s.transactions[t.ID] = t
	return nil
}

func (s *stubRepo) ExistsAccrualForPeriod(ctx context.Context, employeeID, leaveTypeID, transactionDate string) (bool, error) {
	for _, t := range s.transactions {
		if t.EmployeeID == employeeID && t.LeaveTypeID == leaveTypeID &&
			t.TransactionType == leave.TransactionAccrual && t.TransactionDate == transactionDate {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubRepo) ExistsAnyAccrual(ctx context.Context, employeeID, leaveTypeID string) (bool, error) {
	for _, t := range s.transactions {
		if t.EmployeeID == employeeID && t.LeaveTypeID == leaveTypeID && t.TransactionType == leave.TransactionAccrual {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubRepo) FindApprovedRequestsWithoutUsageEntry(ctx context.Context, orgID, leaveTypeID string) ([]*leave.LeaveRequest, error) {
	var list []*leave.LeaveRequest
	for _, r := range s.requests {
		if r.OrgID != orgID || r.LeaveTypeID != leaveTypeID || r.Status != leave.LeaveRequestStatusApproved {
			continue
		}
		hasUsage := false
		for _, t := range s.transactions {
			if t.LeaveRequestID != nil && *t.LeaveRequestID == r.ID && t.TransactionType == leave.TransactionUsage {
				hasUsage = true
				break
			}
		}
		if !hasUsage {
			list = append(list, r)
		}
	}
	return list, nil
}

func TestLeaveTypeService(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
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
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
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
