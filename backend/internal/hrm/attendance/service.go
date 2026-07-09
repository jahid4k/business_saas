// backend/internal/hrm/attendance/service.go
package attendance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

type Service interface {
	// Records
	ListRecords(ctx context.Context, orgID, employeeID, status string, year, month int) (*RecordListResponse, error)
	GetRecord(ctx context.Context, orgID, ref string) (*AttendanceRecord, error)
	Record(ctx context.Context, orgID, createdBy string, req CreateRecordRequest) (*AttendanceRecord, error)
	Approve(ctx context.Context, orgID, ref, approvedBy string) (*AttendanceRecord, error)
	Reject(ctx context.Context, orgID, ref string) (*AttendanceRecord, error)
	Regularize(ctx context.Context, orgID, ref, requesterID string, req RegularizeRequest) (*AttendanceRecord, error)
	// Periods
	ListPeriods(ctx context.Context, orgID string, year, month int) (*PeriodListResponse, error)
	GetOrCreatePeriod(ctx context.Context, orgID, createdBy string, year, month int) (*AttendancePeriod, error)
	FinalizePeriod(ctx context.Context, orgID, finalizedBy string, year, month int) (*AttendancePeriod, error)
	LockPeriod(ctx context.Context, orgID, lockedBy string, year, month int) (*AttendancePeriod, error)
	// Summary (called by D2 payslip compute)
	GetEmployeeSummary(ctx context.Context, orgID, employeeID string, year, month int) (*EmployeeSummary, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}
func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) ListRecords(ctx context.Context, orgID, employeeID, status string, year, month int) (*RecordListResponse, error) {
	list, err := s.repo.FindRecords(ctx, orgID, employeeID, status, year, month)
	if err != nil { return nil, fmt.Errorf("attendance: ListRecords: %w", err) }
	if list == nil { list = []*AttendanceRecord{} }
	return &RecordListResponse{Records: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetRecord(ctx context.Context, orgID, ref string) (*AttendanceRecord, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("attendance: GetRecord: %w", err) }
	if r == nil { return nil, ErrNotFound }
	return r, nil
}

func (s *serviceImpl) Record(ctx context.Context, orgID, createdBy string, req CreateRecordRequest) (*AttendanceRecord, error) {
	if strings.TrimSpace(req.EmployeeID) == "" { return nil, ErrEmployeeIDRequired }
	if strings.TrimSpace(req.Date) == "" { return nil, ErrDateRequired }
	if _, err := time.Parse(dateLayout, req.Date); err != nil { return nil, ErrInvalidDate }
	if !req.DayType.IsValid() { return nil, ErrInvalidDayType }

	// Check for duplicate
	existing, err := s.repo.FindByEmployeeDate(ctx, orgID, req.EmployeeID, req.Date)
	if err != nil { return nil, fmt.Errorf("attendance: Record: check duplicate: %w", err) }
	if existing != nil { return nil, ErrDuplicateRecord }

	// Check period is not finalized/locked
	if err := s.checkPeriodOpen(ctx, orgID, req.Date); err != nil { return nil, err }

	// Resolve shift for this employee
	var shiftID, shiftName *string
	var expectedIn, expectedOut *string
	s.resolveShift(ctx, orgID, req.EmployeeID, &shiftID, &shiftName, &expectedIn, &expectedOut)

	// Compute hours
	breakMins := 0
	if req.BreakMinutes != nil { breakMins = *req.BreakMinutes }
	regHours, otHours := computeHours(req.CheckIn, req.CheckOut, breakMins, expectedIn, expectedOut)

	src := req.Source
	if src == "" { src = SourceManual }

	rec := &AttendanceRecord{
		OrgID: orgID, EmployeeID: req.EmployeeID,
		AttendanceDate: req.Date,
		ShiftID: shiftID, ShiftName: shiftName,
		ExpectedIn: expectedIn, ExpectedOut: expectedOut,
		CheckInTime: req.CheckIn, CheckOutTime: req.CheckOut,
		BreakMinutes: breakMins,
		RegularHours: regHours, OvertimeHours: otHours,
		DayType: req.DayType, Source: src, Notes: req.Notes,
		Status: StatusApproved, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		if strings.Contains(err.Error(), "unique") { return nil, ErrDuplicateRecord }
		return nil, fmt.Errorf("attendance: Record: %w", err)
	}
	return rec, nil
}

// resolveShift looks up the employee's assigned shift (employee > dept > org > default).
func (s *serviceImpl) resolveShift(ctx context.Context, orgID, employeeID string, shiftID, shiftName, expectedIn, expectedOut **string) {
	var id, name, sin, sout string
	err := s.db.QueryRow(ctx,
		`SELECT sh.id::text, sh.name, sh.start_time::text, sh.end_time::text
		FROM hrm_work_schedule_assignments wsa
		JOIN hrm_shifts sh ON sh.id = wsa.shift_id
		WHERE wsa.org_id=$1 AND sh.is_active=TRUE
		AND (
		    (wsa.scope='employee' AND wsa.entity_id=$2::uuid) OR
		    (wsa.scope='department' AND wsa.entity_id=(SELECT department_id FROM hrm_employees WHERE id=$2::uuid)) OR
		    (wsa.scope='organization')
		)
		AND (wsa.end_date IS NULL OR wsa.end_date >= CURRENT_DATE)
		AND wsa.effective_date <= CURRENT_DATE
		ORDER BY CASE wsa.scope WHEN 'employee' THEN 1 WHEN 'department' THEN 2 ELSE 3 END
		LIMIT 1`,
		orgID, employeeID,
	).Scan(&id, &name, &sin, &sout)
	if err == nil {
		*shiftID = &id; *shiftName = &name; *expectedIn = &sin; *expectedOut = &sout
	}
}

// computeHours returns (regular_hours, overtime_hours) from check_in/out times.
func computeHours(checkIn, checkOut *string, breakMins int, expectedIn, expectedOut *string) (float64, float64) {
	if checkIn == nil || checkOut == nil { return 0, 0 }
	parse := func(t string) time.Time {
		v, _ := time.Parse("15:04", t)
		return v
	}
	inT := parse(*checkIn)
	outT := parse(*checkOut)
	if outT.Before(inT) || outT.Equal(inT) { return 0, 0 }
	totalMins := outT.Sub(inT).Minutes() - float64(breakMins)
	if totalMins <= 0 { return 0, 0 }
	totalHours := totalMins / 60

	// Compute expected hours from shift
	shiftHours := 8.0 // default if no shift
	if expectedIn != nil && expectedOut != nil {
		ein := parse(*expectedIn)
		eout := parse(*expectedOut)
		if eout.After(ein) {
			shiftHours = eout.Sub(ein).Minutes() / 60
		}
	}
	regular := min(totalHours, shiftHours)
	ot := max(0, totalHours-shiftHours)
	return regular, ot
}

func min(a, b float64) float64 { if a < b { return a }; return b }
func max(a, b float64) float64 { if a > b { return a }; return b }

func (s *serviceImpl) checkPeriodOpen(ctx context.Context, orgID, date string) error {
	t, _ := time.Parse(dateLayout, date)
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, t.Year(), int(t.Month()))
	if err != nil { return nil } // period not created yet = open
	if p == nil { return nil }
	if p.Status != PeriodOpen { return ErrPeriodFinalized }
	return nil
}

func (s *serviceImpl) Approve(ctx context.Context, orgID, ref, approvedBy string) (*AttendanceRecord, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("attendance: Approve: %w", err) }
	if r == nil { return nil, ErrNotFound }
	if r.Status != StatusPending { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusApproved, &approvedBy); err != nil {
		return nil, fmt.Errorf("attendance: Approve: %w", err)
	}
	r.Status = StatusApproved
	return r, nil
}

func (s *serviceImpl) Reject(ctx context.Context, orgID, ref string) (*AttendanceRecord, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("attendance: Reject: %w", err) }
	if r == nil { return nil, ErrNotFound }
	if r.Status != StatusPending { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusRejected, nil); err != nil {
		return nil, fmt.Errorf("attendance: Reject: %w", err)
	}
	r.Status = StatusRejected
	return r, nil
}

func (s *serviceImpl) Regularize(ctx context.Context, orgID, ref, requesterID string, req RegularizeRequest) (*AttendanceRecord, error) {
	if strings.TrimSpace(req.Reason) == "" { return nil, ErrReasonRequired }
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("attendance: Regularize: %w", err) }
	if r == nil { return nil, ErrNotFound }
	if err := s.checkPeriodOpen(ctx, orgID, r.AttendanceDate); err != nil { return nil, err }

	// Update times + set pending status for HR review
	if req.NewCheckIn != nil { r.CheckInTime = req.NewCheckIn }
	if req.NewCheckOut != nil { r.CheckOutTime = req.NewCheckOut }
	r.RegularizationReason = &req.Reason
	// Recompute hours
	reg, ot := computeHours(r.CheckInTime, r.CheckOutTime, r.BreakMinutes, r.ExpectedIn, r.ExpectedOut)
	r.RegularHours = reg; r.OvertimeHours = ot
	if err := s.repo.Update(ctx, r); err != nil { return nil, fmt.Errorf("attendance: Regularize: %w", err) }
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusPending, nil); err != nil { return nil, fmt.Errorf("attendance: Regularize: status: %w", err) }
	r.Status = StatusPending
	return r, nil
}

func (s *serviceImpl) ListPeriods(ctx context.Context, orgID string, year, month int) (*PeriodListResponse, error) {
	list, err := s.repo.FindPeriods(ctx, orgID, year, month)
	if err != nil { return nil, fmt.Errorf("attendance: ListPeriods: %w", err) }
	if list == nil { list = []*AttendancePeriod{} }
	return &PeriodListResponse{Periods: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetOrCreatePeriod(ctx context.Context, orgID, createdBy string, year, month int) (*AttendancePeriod, error) {
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, year, month)
	if err != nil { return nil, fmt.Errorf("attendance: GetOrCreatePeriod: %w", err) }
	if p != nil { return p, nil }
	newP := &AttendancePeriod{OrgID: orgID, PeriodYear: year, PeriodMonth: month, Status: PeriodOpen, CreatedBy: createdBy}
	if err := s.repo.CreatePeriod(ctx, newP); err != nil { return nil, fmt.Errorf("attendance: GetOrCreatePeriod: create: %w", err) }
	return newP, nil
}

func (s *serviceImpl) FinalizePeriod(ctx context.Context, orgID, finalizedBy string, year, month int) (*AttendancePeriod, error) {
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, year, month)
	if err != nil { return nil, fmt.Errorf("attendance: FinalizePeriod: %w", err) }
	if p == nil { return nil, ErrPeriodNotFound }
	if p.Status != PeriodOpen { return nil, ErrPeriodAlreadyFinalizedOrLocked }

	// Aggregate stats from attendance_records
	row := s.db.QueryRow(ctx,
		`SELECT
		COUNT(DISTINCT employee_id),
		COUNT(*) FILTER (WHERE day_type IN ('present','late','half_day','work_from_home')),
		COUNT(*) FILTER (WHERE day_type='absent'),
		COUNT(*) FILTER (WHERE day_type='holiday'),
		COUNT(*) FILTER (WHERE day_type='on_leave'),
		COALESCE(SUM(overtime_hours),0)
		FROM hrm_attendance_records
		WHERE org_id=$1 AND status='approved'
		AND EXTRACT(YEAR FROM attendance_date)=$2 AND EXTRACT(MONTH FROM attendance_date)=$3`,
		orgID, year, month)
	_ = row.Scan(&p.TotalEmployees, &p.TotalPresent, &p.TotalAbsent, &p.TotalHolidays, &p.TotalLeaves, &p.TotalOvertimeHours)

	now := time.Now()
	p.Status = PeriodFinalized
	p.FinalizedAt = &now
	p.FinalizedBy = &finalizedBy
	if err := s.repo.UpdatePeriod(ctx, p); err != nil { return nil, fmt.Errorf("attendance: FinalizePeriod: %w", err) }
	return p, nil
}

func (s *serviceImpl) LockPeriod(ctx context.Context, orgID, lockedBy string, year, month int) (*AttendancePeriod, error) {
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, year, month)
	if err != nil { return nil, fmt.Errorf("attendance: LockPeriod: %w", err) }
	if p == nil { return nil, ErrPeriodNotFound }
	if p.Status != PeriodFinalized { return nil, ErrWrongStatus }
	now := time.Now()
	p.Status = PeriodLocked
	p.LockedAt = &now
	p.LockedBy = &lockedBy
	if err := s.repo.UpdatePeriod(ctx, p); err != nil { return nil, fmt.Errorf("attendance: LockPeriod: %w", err) }
	return p, nil
}

func (s *serviceImpl) GetEmployeeSummary(ctx context.Context, orgID, employeeID string, year, month int) (*EmployeeSummary, error) {
	return s.repo.GetEmployeeSummary(ctx, orgID, employeeID, year, month)
}
