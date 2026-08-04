// backend/internal/hrm/attendance/repository.go
package attendance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

type Repository interface {
	// Records
	FindRecords(ctx context.Context, orgID string, filter RecordListFilter) ([]*AttendanceRecord, error)
	CountRecords(ctx context.Context, orgID string, filter RecordListFilter) (int, error)
	FindByRef(ctx context.Context, orgID, ref string) (*AttendanceRecord, error)
	FindByEmployeeDate(ctx context.Context, orgID, employeeID, date string) (*AttendanceRecord, error)
	Create(ctx context.Context, r *AttendanceRecord) error
	Update(ctx context.Context, r *AttendanceRecord) error
	UpdateStatus(ctx context.Context, id string, status RecordStatus, approvedBy *string) error
	// Periods
	FindPeriods(ctx context.Context, orgID string, year, month int) ([]*AttendancePeriod, error)
	FindPeriodByYearMonth(ctx context.Context, orgID string, year, month int) (*AttendancePeriod, error)
	CreatePeriod(ctx context.Context, p *AttendancePeriod) error
	UpdatePeriod(ctx context.Context, p *AttendancePeriod) error
	// Summary (for payslip compute)
	GetEmployeeSummary(ctx context.Context, orgID, employeeID string, year, month int) (*EmployeeSummary, error)
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const recSel = `id, public_id, org_id, employee_id,
	to_char(attendance_date,'YYYY-MM-DD'),
	shift_id, shift_name, expected_in::text, expected_out::text,
	check_in_time::text, check_out_time::text, break_minutes,
	regular_hours, overtime_hours, day_type, source, notes,
	regularization_reason, regularization_instance_id,
	status, approved_by, approved_at,
	created_by, created_at, updated_at`

func scanRec(row pgx.Row) (*AttendanceRecord, error) {
	r := &AttendanceRecord{}
	err := row.Scan(
		&r.ID, &r.PublicID, &r.OrgID, &r.EmployeeID,
		&r.AttendanceDate,
		&r.ShiftID, &r.ShiftName, &r.ExpectedIn, &r.ExpectedOut,
		&r.CheckInTime, &r.CheckOutTime, &r.BreakMinutes,
		&r.RegularHours, &r.OvertimeHours, &r.DayType, &r.Source, &r.Notes,
		&r.RegularizationReason, &r.RegularizationInstanceID,
		&r.Status, &r.ApprovedBy, &r.ApprovedAt,
		&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return r, nil
}

func buildRecordsWhere(orgID string, filter RecordListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Year > 0 && filter.Month > 0 {
		args = append(args, filter.Year, filter.Month)
		clauses = append(clauses, fmt.Sprintf("EXTRACT(YEAR FROM attendance_date) = $%d AND EXTRACT(MONTH FROM attendance_date) = $%d", len(args)-1, len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindRecords(ctx context.Context, orgID string, filter RecordListFilter) ([]*AttendanceRecord, error) {
	where, args := buildRecordsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_attendance_records WHERE %s ORDER BY attendance_date DESC, employee_id LIMIT $%d OFFSET $%d`,
		recSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("attendance: FindRecords: %w", err) }
	defer rows.Close()
	list := make([]*AttendanceRecord, 0)
	for rows.Next() { rec, err := scanRec(rows); if err != nil { return nil, err }; list = append(list, rec) }
	return list, rows.Err()
}

func (r *repoImpl) CountRecords(ctx context.Context, orgID string, filter RecordListFilter) (int, error) {
	where, args := buildRecordsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_attendance_records WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("attendance: CountRecords: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*AttendanceRecord, error) {
	return scanRec(r.db.QueryRow(ctx,
		`SELECT `+recSel+` FROM hrm_attendance_records WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindByEmployeeDate(ctx context.Context, orgID, employeeID, date string) (*AttendanceRecord, error) {
	return scanRec(r.db.QueryRow(ctx,
		`SELECT `+recSel+` FROM hrm_attendance_records WHERE org_id=$1 AND employee_id=$2::uuid AND attendance_date=$3::date`,
		orgID, employeeID, date))
}

func (r *repoImpl) Create(ctx context.Context, rec *AttendanceRecord) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_attendance_records
		(org_id, employee_id, attendance_date, shift_id, shift_name, expected_in, expected_out,
		 check_in_time, check_out_time, break_minutes, regular_hours, overtime_hours,
		 day_type, source, notes, status, created_by)
		VALUES ($1,$2,$3::date,$4,$5,$6::time,$7::time,$8::time,$9::time,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id, public_id, created_at, updated_at`,
		rec.OrgID, rec.EmployeeID, rec.AttendanceDate,
		rec.ShiftID, rec.ShiftName, rec.ExpectedIn, rec.ExpectedOut,
		rec.CheckInTime, rec.CheckOutTime, rec.BreakMinutes,
		rec.RegularHours, rec.OvertimeHours,
		rec.DayType, rec.Source, rec.Notes, rec.Status, rec.CreatedBy,
	).Scan(&rec.ID, &rec.PublicID, &rec.CreatedAt, &rec.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, rec *AttendanceRecord) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_attendance_records SET
		check_in_time=$1::time, check_out_time=$2::time, break_minutes=$3,
		regular_hours=$4, overtime_hours=$5, day_type=$6, notes=$7,
		regularization_reason=$8, regularization_instance_id=$9,
		updated_at=NOW()
		WHERE id=$10 AND org_id=$11 RETURNING updated_at`,
		rec.CheckInTime, rec.CheckOutTime, rec.BreakMinutes,
		rec.RegularHours, rec.OvertimeHours, rec.DayType, rec.Notes,
		rec.RegularizationReason, rec.RegularizationInstanceID,
		rec.ID, rec.OrgID,
	).Scan(&rec.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status RecordStatus, approvedBy *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_attendance_records SET status=$1, approved_by=$2, approved_at=CASE WHEN $1='approved' THEN NOW() END, updated_at=NOW() WHERE id=$3`,
		status, approvedBy, id)
	return err
}

const perSel = `id, public_id, org_id, period_year, period_month, status,
	total_employees, total_work_days, total_present, total_absent,
	total_holidays, total_leaves, total_overtime_hours,
	finalized_at, finalized_by, locked_at, locked_by,
	created_by, created_at, updated_at`

func scanPeriod(row pgx.Row) (*AttendancePeriod, error) {
	p := &AttendancePeriod{}
	err := row.Scan(
		&p.ID, &p.PublicID, &p.OrgID, &p.PeriodYear, &p.PeriodMonth, &p.Status,
		&p.TotalEmployees, &p.TotalWorkDays, &p.TotalPresent, &p.TotalAbsent,
		&p.TotalHolidays, &p.TotalLeaves, &p.TotalOvertimeHours,
		&p.FinalizedAt, &p.FinalizedBy, &p.LockedAt, &p.LockedBy,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return p, nil
}

func (r *repoImpl) FindPeriods(ctx context.Context, orgID string, year, month int) ([]*AttendancePeriod, error) {
	q := `SELECT ` + perSel + ` FROM hrm_attendance_periods WHERE org_id=$1`
	args := []any{orgID}
	if year > 0 { args = append(args, year); q += fmt.Sprintf(` AND period_year=$%d`, len(args)) }
	if month > 0 { args = append(args, month); q += fmt.Sprintf(` AND period_month=$%d`, len(args)) }
	q += ` ORDER BY period_year DESC, period_month DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("attendance: FindPeriods: %w", err) }
	defer rows.Close()
	list := make([]*AttendancePeriod, 0)
	for rows.Next() { p, err := scanPeriod(rows); if err != nil { return nil, err }; list = append(list, p) }
	return list, rows.Err()
}

func (r *repoImpl) FindPeriodByYearMonth(ctx context.Context, orgID string, year, month int) (*AttendancePeriod, error) {
	return scanPeriod(r.db.QueryRow(ctx,
		`SELECT `+perSel+` FROM hrm_attendance_periods WHERE org_id=$1 AND period_year=$2 AND period_month=$3`,
		orgID, year, month))
}

func (r *repoImpl) CreatePeriod(ctx context.Context, p *AttendancePeriod) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_attendance_periods (org_id, period_year, period_month, status, created_by)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.PeriodYear, p.PeriodMonth, p.Status, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *repoImpl) UpdatePeriod(ctx context.Context, p *AttendancePeriod) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_attendance_periods SET
		status=$1, total_employees=$2, total_work_days=$3,
		total_present=$4, total_absent=$5, total_holidays=$6, total_leaves=$7,
		total_overtime_hours=$8, finalized_at=$9, finalized_by=$10,
		locked_at=$11, locked_by=$12, updated_at=NOW()
		WHERE id=$13 AND org_id=$14 RETURNING updated_at`,
		p.Status, p.TotalEmployees, p.TotalWorkDays,
		p.TotalPresent, p.TotalAbsent, p.TotalHolidays, p.TotalLeaves,
		p.TotalOvertimeHours, p.FinalizedAt, p.FinalizedBy,
		p.LockedAt, p.LockedBy, p.ID, p.OrgID,
	).Scan(&p.UpdatedAt)
}

func (r *repoImpl) GetEmployeeSummary(ctx context.Context, orgID, employeeID string, year, month int) (*EmployeeSummary, error) {
	s := &EmployeeSummary{EmployeeID: employeeID}
	err := r.db.QueryRow(ctx,
		`SELECT
		COUNT(*) FILTER (WHERE day_type IN ('present','late','half_day','work_from_home')),
		COUNT(*) FILTER (WHERE day_type='absent'),
		COUNT(*) FILTER (WHERE day_type='on_leave'),
		COUNT(*) FILTER (WHERE day_type='holiday'),
		COALESCE(SUM(overtime_hours),0)
		FROM hrm_attendance_records
		WHERE org_id=$1 AND employee_id=$2::uuid AND status='approved'
		AND EXTRACT(YEAR FROM attendance_date)=$3 AND EXTRACT(MONTH FROM attendance_date)=$4`,
		orgID, employeeID, year, month,
	).Scan(&s.PresentDays, &s.AbsentDays, &s.LeaveDays, &s.HolidayDays, &s.OvertimeHours)
	if err != nil { return nil, fmt.Errorf("attendance: GetEmployeeSummary: %w", err) }
	return s, nil
}
