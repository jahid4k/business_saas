package attendance_test

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/attendance"
)

type stubRepo struct {
	recs map[string]*attendance.AttendanceRecord
	pers map[string]*attendance.AttendancePeriod
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		recs: make(map[string]*attendance.AttendanceRecord),
		pers: make(map[string]*attendance.AttendancePeriod),
	}
}

func (s *stubRepo) FindRecords(ctx context.Context, orgID string, filter attendance.RecordListFilter) ([]*attendance.AttendanceRecord, error) {
	var res []*attendance.AttendanceRecord
	for _, r := range s.recs {
		if r.OrgID == orgID {
			if filter.EmployeeID != "" && r.EmployeeID != filter.EmployeeID { continue }
			if filter.Status != "" && string(r.Status) != filter.Status { continue }
			res = append(res, r)
		}
	}
	return res, nil
}

func (s *stubRepo) CountRecords(ctx context.Context, orgID string, filter attendance.RecordListFilter) (int, error) {
	out, err := s.FindRecords(ctx, orgID, filter)
	return len(out), err
}

func (s *stubRepo) FindByRef(ctx context.Context, orgID, ref string) (*attendance.AttendanceRecord, error) {
	for _, r := range s.recs {
		if r.OrgID == orgID && (r.ID == ref || r.PublicID == ref) {
			return r, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) FindByEmployeeDate(ctx context.Context, orgID, employeeID, date string) (*attendance.AttendanceRecord, error) {
	for _, r := range s.recs {
		if r.OrgID == orgID && r.EmployeeID == employeeID && r.AttendanceDate == date {
			return r, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) Create(ctx context.Context, r *attendance.AttendanceRecord) error {
	r.ID = "id_" + r.EmployeeID + "_" + r.AttendanceDate
	s.recs[r.ID] = r
	return nil
}

func (s *stubRepo) Update(ctx context.Context, r *attendance.AttendanceRecord) error {
	s.recs[r.ID] = r
	return nil
}

func (s *stubRepo) UpdateStatus(ctx context.Context, id string, status attendance.RecordStatus, approvedBy *string) error {
	if r, ok := s.recs[id]; ok {
		r.Status = status
		r.ApprovedBy = approvedBy
	}
	return nil
}

func (s *stubRepo) FindPeriods(ctx context.Context, orgID string, year, month int) ([]*attendance.AttendancePeriod, error) {
	var res []*attendance.AttendancePeriod
	for _, p := range s.pers {
		if p.OrgID == orgID {
			if year > 0 && p.PeriodYear != year { continue }
			if month > 0 && p.PeriodMonth != month { continue }
			res = append(res, p)
		}
	}
	return res, nil
}

func (s *stubRepo) FindPeriodByYearMonth(ctx context.Context, orgID string, year, month int) (*attendance.AttendancePeriod, error) {
	for _, p := range s.pers {
		if p.OrgID == orgID && p.PeriodYear == year && p.PeriodMonth == month {
			return p, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) CreatePeriod(ctx context.Context, p *attendance.AttendancePeriod) error {
	p.ID = "id_period"
	s.pers[p.ID] = p
	return nil
}

func (s *stubRepo) UpdatePeriod(ctx context.Context, p *attendance.AttendancePeriod) error {
	s.pers[p.ID] = p
	return nil
}

func (s *stubRepo) GetEmployeeSummary(ctx context.Context, orgID, employeeID string, year, month int) (*attendance.EmployeeSummary, error) {
	return &attendance.EmployeeSummary{EmployeeID: employeeID}, nil
}

func ptrStr(s string) *string { return &s }

func TestAttendanceService_ApproveReject(t *testing.T) {
	repo := newStubRepo()
	svc := attendance.NewService(repo, nil)
	ctx := context.Background()

	repo.recs["id1"] = &attendance.AttendanceRecord{
		ID: "id1",
		OrgID: "org1",
		Status: attendance.StatusPending,
	}

	t.Run("approve_success", func(t *testing.T) {
		r, err := svc.Approve(ctx, "org1", "id1", "admin1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Status != attendance.StatusApproved {
			t.Errorf("expected approved, got %v", r.Status)
		}
	})

	repo.recs["id2"] = &attendance.AttendanceRecord{
		ID: "id2",
		OrgID: "org1",
		Status: attendance.StatusPending,
	}

	t.Run("reject_success", func(t *testing.T) {
		r, err := svc.Reject(ctx, "org1", "id2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Status != attendance.StatusRejected {
			t.Errorf("expected rejected, got %v", r.Status)
		}
	})
}

func TestAttendanceService_Regularize(t *testing.T) {
	repo := newStubRepo()
	svc := attendance.NewService(repo, nil)
	ctx := context.Background()

	repo.recs["id1"] = &attendance.AttendanceRecord{
		ID: "id1",
		OrgID: "org1",
		Status: attendance.StatusApproved,
		AttendanceDate: "2023-01-01",
	}

	t.Run("success", func(t *testing.T) {
		req := attendance.RegularizeRequest{
			Reason: "forgot to check in",
			NewCheckIn: ptrStr("09:00"),
		}
		r, err := svc.Regularize(ctx, "org1", "id1", "emp1", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Status != attendance.StatusPending {
			t.Errorf("expected pending, got %v", r.Status)
		}
		if r.RegularizationReason == nil || *r.RegularizationReason != "forgot to check in" {
			t.Errorf("expected reason to be set")
		}
	})
}

func TestAttendanceService_Periods(t *testing.T) {
	repo := newStubRepo()
	svc := attendance.NewService(repo, nil)
	ctx := context.Background()

	t.Run("get_or_create", func(t *testing.T) {
		p, err := svc.GetOrCreatePeriod(ctx, "org1", "admin1", 2023, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Status != attendance.PeriodOpen {
			t.Errorf("expected open, got %v", p.Status)
		}
	})

	repo.pers["p1"] = &attendance.AttendancePeriod{
		ID: "p1",
		OrgID: "org1",
		PeriodYear: 2023,
		PeriodMonth: 2,
		Status: attendance.PeriodFinalized,
	}

	t.Run("lock_period", func(t *testing.T) {
		p, err := svc.LockPeriod(ctx, "org1", "admin1", 2023, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Status != attendance.PeriodLocked {
			t.Errorf("expected locked, got %v", p.Status)
		}
	})
}

func TestAttendanceService_ListGet(t *testing.T) {
	repo := newStubRepo()
	svc := attendance.NewService(repo, nil)
	ctx := context.Background()

	repo.recs["id1"] = &attendance.AttendanceRecord{ID: "id1", OrgID: "org1"}
	
	t.Run("get", func(t *testing.T) {
		r, err := svc.GetRecord(ctx, "org1", "id1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.ID != "id1" {
			t.Errorf("expected id1, got %v", r.ID)
		}
	})

	t.Run("list", func(t *testing.T) {
		res, err := svc.ListRecords(ctx, "org1", attendance.RecordListFilter{Scope: authz.ScopeAll})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("expected 1, got %v", res.Total)
		}
	})
}
