// backend/internal/hrm/attendance/service.go
package attendance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

type Service interface {
	// Records
	ListRecords(ctx context.Context, orgID string, filter RecordListFilter) (*RecordListResponse, error)
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
	// RunAbsenceSweep marks active employees absent for `date` when they have no
	// attendance record, it's a working day for their shift, it isn't a holiday, and
	// they have no approved leave covering it. Returns the number of records created.
	RunAbsenceSweep(ctx context.Context, orgID, createdBy, date string) (int, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) ListRecords(ctx context.Context, orgID string, filter RecordListFilter) (*RecordListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindRecords(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("attendance: ListRecords: %w", err)
	}
	if list == nil {
		list = []*AttendanceRecord{}
	}
	total, err := s.repo.CountRecords(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("attendance: ListRecords: count: %w", err)
	}
	return &RecordListResponse{Records: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetRecord(ctx context.Context, orgID, ref string) (*AttendanceRecord, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("attendance: GetRecord: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *serviceImpl) Record(ctx context.Context, orgID, createdBy string, req CreateRecordRequest) (*AttendanceRecord, error) {
	if strings.TrimSpace(req.EmployeeID) == "" {
		return nil, ErrEmployeeIDRequired
	}
	if strings.TrimSpace(req.Date) == "" {
		return nil, ErrDateRequired
	}
	if _, err := time.Parse(dateLayout, req.Date); err != nil {
		return nil, ErrInvalidDate
	}
	if !req.DayType.IsValid() {
		return nil, ErrInvalidDayType
	}

	// Check for duplicate
	existing, err := s.repo.FindByEmployeeDate(ctx, orgID, req.EmployeeID, req.Date)
	if err != nil {
		return nil, fmt.Errorf("attendance: Record: check duplicate: %w", err)
	}
	if existing != nil {
		return nil, ErrDuplicateRecord
	}

	// Check period is not finalized/locked
	if err := s.checkPeriodOpen(ctx, orgID, req.Date); err != nil {
		return nil, err
	}

	// Resolve shift for this employee
	var shiftID, shiftName *string
	var expectedIn, expectedOut *string
	s.resolveShift(ctx, orgID, req.EmployeeID, req.Date, &shiftID, &shiftName, &expectedIn, &expectedOut)

	// Compute hours
	breakMins := 0
	if req.BreakMinutes != nil {
		breakMins = *req.BreakMinutes
	}
	regHours, otHours := computeHours(req.CheckIn, req.CheckOut, breakMins, expectedIn, expectedOut)

	src := req.Source
	if src == "" {
		src = SourceManual
	}

	rec := &AttendanceRecord{
		OrgID: orgID, EmployeeID: req.EmployeeID,
		AttendanceDate: req.Date,
		ShiftID:        shiftID, ShiftName: shiftName,
		ExpectedIn: expectedIn, ExpectedOut: expectedOut,
		CheckInTime: req.CheckIn, CheckOutTime: req.CheckOut,
		BreakMinutes: breakMins,
		RegularHours: regHours, OvertimeHours: otHours,
		DayType: req.DayType, Source: src, Notes: req.Notes,
		Status: StatusApproved, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, ErrDuplicateRecord
		}
		return nil, fmt.Errorf("attendance: Record: %w", err)
	}
	return rec, nil
}

// resolveShift looks up the employee's assigned shift as of `date` (employee > dept > org),
// returning the shift's working_days (e.g. {mon,tue,wed,thu,fri}) so callers can determine
// whether `date` is a working day for this employee.
func (s *serviceImpl) resolveShift(ctx context.Context, orgID, employeeID, date string, shiftID, shiftName, expectedIn, expectedOut **string) []string {
	var id, name, sin, sout string
	var workingDays []string
	err := s.db.QueryRow(ctx,
		`SELECT sh.id::text, sh.name, sh.start_time::text, sh.end_time::text, sh.working_days
		FROM hrm_work_schedule_assignments wsa
		JOIN hrm_shifts sh ON sh.id = wsa.shift_id
		WHERE wsa.org_id=$1 AND sh.is_active=TRUE
		AND (
		    (wsa.assignee_type='employee' AND wsa.assignee_id=$2::uuid) OR
		    (wsa.assignee_type='department' AND wsa.assignee_id=(SELECT department_id FROM hrm_employees WHERE id=$2::uuid)) OR
		    (wsa.assignee_type='organization')
		)
		AND (wsa.end_date IS NULL OR wsa.end_date >= $3::date)
		AND wsa.effective_date <= $3::date
		ORDER BY CASE wsa.assignee_type WHEN 'employee' THEN 1 WHEN 'department' THEN 2 ELSE 3 END
		LIMIT 1`,
		orgID, employeeID, date,
	).Scan(&id, &name, &sin, &sout, &workingDays)
	if err == nil {
		*shiftID = &id
		*shiftName = &name
		*expectedIn = &sin
		*expectedOut = &sout
	}
	return workingDays
}

// computeHours returns (regular_hours, overtime_hours) from check_in/out times.
func computeHours(checkIn, checkOut *string, breakMins int, expectedIn, expectedOut *string) (float64, float64) {
	if checkIn == nil || checkOut == nil {
		return 0, 0
	}
	parse := func(t string) time.Time {
		v, _ := time.Parse("15:04", t)
		return v
	}
	inT := parse(*checkIn)
	outT := parse(*checkOut)
	if outT.Before(inT) || outT.Equal(inT) {
		return 0, 0
	}
	totalMins := outT.Sub(inT).Minutes() - float64(breakMins)
	if totalMins <= 0 {
		return 0, 0
	}
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

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func (s *serviceImpl) checkPeriodOpen(ctx context.Context, orgID, date string) error {
	t, _ := time.Parse(dateLayout, date)
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, t.Year(), int(t.Month()))
	if err != nil {
		return nil
	} // period not created yet = open
	if p == nil {
		return nil
	}
	if p.Status != PeriodOpen {
		return ErrPeriodFinalized
	}
	return nil
}

func (s *serviceImpl) Approve(ctx context.Context, orgID, ref, approvedBy string) (*AttendanceRecord, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("attendance: Approve: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status != StatusPending {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusApproved, &approvedBy); err != nil {
		return nil, fmt.Errorf("attendance: Approve: %w", err)
	}
	r.Status = StatusApproved
	return r, nil
}

func (s *serviceImpl) Reject(ctx context.Context, orgID, ref string) (*AttendanceRecord, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("attendance: Reject: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status != StatusPending {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusRejected, nil); err != nil {
		return nil, fmt.Errorf("attendance: Reject: %w", err)
	}
	r.Status = StatusRejected
	return r, nil
}

func (s *serviceImpl) Regularize(ctx context.Context, orgID, ref, requesterID string, req RegularizeRequest) (*AttendanceRecord, error) {
	if strings.TrimSpace(req.Reason) == "" {
		return nil, ErrReasonRequired
	}
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("attendance: Regularize: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if err := s.checkPeriodOpen(ctx, orgID, r.AttendanceDate); err != nil {
		return nil, err
	}

	// Update times + set pending status for HR review
	if req.NewCheckIn != nil {
		r.CheckInTime = req.NewCheckIn
	}
	if req.NewCheckOut != nil {
		r.CheckOutTime = req.NewCheckOut
	}
	r.RegularizationReason = &req.Reason
	// Recompute hours
	reg, ot := computeHours(r.CheckInTime, r.CheckOutTime, r.BreakMinutes, r.ExpectedIn, r.ExpectedOut)
	r.RegularHours = reg
	r.OvertimeHours = ot
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, fmt.Errorf("attendance: Regularize: %w", err)
	}
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusPending, nil); err != nil {
		return nil, fmt.Errorf("attendance: Regularize: status: %w", err)
	}
	r.Status = StatusPending
	return r, nil
}

func (s *serviceImpl) ListPeriods(ctx context.Context, orgID string, year, month int) (*PeriodListResponse, error) {
	list, err := s.repo.FindPeriods(ctx, orgID, year, month)
	if err != nil {
		return nil, fmt.Errorf("attendance: ListPeriods: %w", err)
	}
	if list == nil {
		list = []*AttendancePeriod{}
	}
	return &PeriodListResponse{Periods: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetOrCreatePeriod(ctx context.Context, orgID, createdBy string, year, month int) (*AttendancePeriod, error) {
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, year, month)
	if err != nil {
		return nil, fmt.Errorf("attendance: GetOrCreatePeriod: %w", err)
	}
	if p != nil {
		return p, nil
	}
	newP := &AttendancePeriod{OrgID: orgID, PeriodYear: year, PeriodMonth: month, Status: PeriodOpen, CreatedBy: createdBy}
	if err := s.repo.CreatePeriod(ctx, newP); err != nil {
		return nil, fmt.Errorf("attendance: GetOrCreatePeriod: create: %w", err)
	}
	return newP, nil
}

func (s *serviceImpl) FinalizePeriod(ctx context.Context, orgID, finalizedBy string, year, month int) (*AttendancePeriod, error) {
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, year, month)
	if err != nil {
		return nil, fmt.Errorf("attendance: FinalizePeriod: %w", err)
	}
	if p == nil {
		return nil, ErrPeriodNotFound
	}
	if p.Status != PeriodOpen {
		return nil, ErrPeriodAlreadyFinalizedOrLocked
	}

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
	if err := s.repo.UpdatePeriod(ctx, p); err != nil {
		return nil, fmt.Errorf("attendance: FinalizePeriod: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) LockPeriod(ctx context.Context, orgID, lockedBy string, year, month int) (*AttendancePeriod, error) {
	p, err := s.repo.FindPeriodByYearMonth(ctx, orgID, year, month)
	if err != nil {
		return nil, fmt.Errorf("attendance: LockPeriod: %w", err)
	}
	if p == nil {
		return nil, ErrPeriodNotFound
	}
	if p.Status != PeriodFinalized {
		return nil, ErrWrongStatus
	}
	now := time.Now()
	p.Status = PeriodLocked
	p.LockedAt = &now
	p.LockedBy = &lockedBy
	if err := s.repo.UpdatePeriod(ctx, p); err != nil {
		return nil, fmt.Errorf("attendance: LockPeriod: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) GetEmployeeSummary(ctx context.Context, orgID, employeeID string, year, month int) (*EmployeeSummary, error) {
	return s.repo.GetEmployeeSummary(ctx, orgID, employeeID, year, month)
}

func (s *serviceImpl) RunAbsenceSweep(ctx context.Context, orgID, createdBy, date string) (int, error) {
	parsedDate, err := time.Parse(dateLayout, date)
	if err != nil {
		return 0, fmt.Errorf("attendance: RunAbsenceSweep: invalid date: %w", err)
	}
	weekday := strings.ToLower(parsedDate.Format("Mon")) // "mon", "tue", ...

	rows, err := s.db.Query(ctx,
		`SELECT e.id::text, e.department_id::text
		FROM hrm_employees e
		JOIN hrm_employee_statuses st ON st.id = e.status_id
		WHERE e.org_id=$1 AND st.category='active'`,
		orgID)
	if err != nil {
		return 0, fmt.Errorf("attendance: RunAbsenceSweep: list active employees: %w", err)
	}
	type activeEmp struct {
		ID     string
		DeptID *string
	}
	var employees []activeEmp
	for rows.Next() {
		var e activeEmp
		if err := rows.Scan(&e.ID, &e.DeptID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("attendance: RunAbsenceSweep: scan employee: %w", err)
		}
		employees = append(employees, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("attendance: RunAbsenceSweep: %w", err)
	}

	created := 0
	for _, emp := range employees {
		existing, err := s.repo.FindByEmployeeDate(ctx, orgID, emp.ID, date)
		if err != nil || existing != nil {
			continue
		}

		var shiftID, shiftName, expectedIn, expectedOut *string
		workingDays := s.resolveShift(ctx, orgID, emp.ID, date, &shiftID, &shiftName, &expectedIn, &expectedOut)
		if len(workingDays) > 0 && !slices.Contains(workingDays, weekday) {
			continue // weekend/off day for this employee's shift
		}

		if s.isHoliday(ctx, orgID, emp.ID, emp.DeptID, date) {
			continue
		}
		if s.hasApprovedLeave(ctx, emp.ID, date) {
			continue
		}

		rec := &AttendanceRecord{
			OrgID: orgID, EmployeeID: emp.ID,
			AttendanceDate: date,
			ShiftID:        shiftID, ShiftName: shiftName,
			ExpectedIn: expectedIn, ExpectedOut: expectedOut,
			DayType: DayAbsent, Source: SourceSystem, Status: StatusApproved,
			CreatedBy: createdBy,
		}
		if err := s.repo.Create(ctx, rec); err != nil {
			continue
		}
		created++
	}
	return created, nil
}

// isHoliday reports whether `date` is a holiday under the calendar assigned to the
// employee, falling back to department then organization (documented cascade in
// hrm_calendar_assignments: employee > department > organization).
func (s *serviceImpl) isHoliday(ctx context.Context, orgID, employeeID string, deptID *string, date string) bool {
	calendarID := s.resolveHolidayCalendar(ctx, orgID, employeeID, deptID)
	if calendarID == "" {
		return false
	}
	var exists bool
	_ = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_holidays WHERE calendar_id=$1::uuid AND date=$2::date)`,
		calendarID, date,
	).Scan(&exists)
	return exists
}

func (s *serviceImpl) resolveHolidayCalendar(ctx context.Context, orgID, employeeID string, deptID *string) string {
	var calendarID string
	if err := s.db.QueryRow(ctx,
		`SELECT calendar_id::text FROM hrm_calendar_assignments WHERE assignee_type='employee' AND assignee_id=$1::uuid`,
		employeeID,
	).Scan(&calendarID); err == nil {
		return calendarID
	}
	if deptID != nil {
		if err := s.db.QueryRow(ctx,
			`SELECT calendar_id::text FROM hrm_calendar_assignments WHERE assignee_type='department' AND assignee_id=$1::uuid`,
			*deptID,
		).Scan(&calendarID); err == nil {
			return calendarID
		}
	}
	if err := s.db.QueryRow(ctx,
		`SELECT calendar_id::text FROM hrm_calendar_assignments WHERE assignee_type='organization' AND assignee_id=$1::uuid`,
		orgID,
	).Scan(&calendarID); err == nil {
		return calendarID
	}
	return ""
}

// hasApprovedLeave reports whether the employee has an approved leave request
// covering `date`.
func (s *serviceImpl) hasApprovedLeave(ctx context.Context, employeeID, date string) bool {
	var exists bool
	_ = s.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM hrm_leave_requests
			WHERE employee_id=$1::uuid AND status='approved'
			AND start_date <= $2::date AND end_date >= $2::date
		)`,
		employeeID, date,
	).Scan(&exists)
	return exists
}
