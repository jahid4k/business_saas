// backend/internal/hrm/attendance/model.go
package attendance

import (
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

type DayType string

const (
	DayPresent DayType = "present"
	DayAbsent  DayType = "absent"
	DayHalfDay DayType = "half_day"
	DayLate    DayType = "late"
	DayOnLeave DayType = "on_leave"
	DayHoliday DayType = "holiday"
	DayWeekend DayType = "weekend"
	DayWFH     DayType = "work_from_home"
)

func (d DayType) IsValid() bool {
	switch d {
	case DayPresent, DayAbsent, DayHalfDay, DayLate, DayOnLeave, DayHoliday, DayWeekend, DayWFH:
		return true
	}
	return false
}

type AttendanceSource string

const (
	SourceManual AttendanceSource = "manual"
	SourceDevice AttendanceSource = "device"
	SourceAPI    AttendanceSource = "api"
	SourceSystem AttendanceSource = "system"
)

type RecordStatus string

const (
	StatusApproved RecordStatus = "approved"
	StatusPending  RecordStatus = "pending"
	StatusRejected RecordStatus = "rejected"
)

type PeriodStatus string

const (
	PeriodOpen      PeriodStatus = "open"
	PeriodFinalized PeriodStatus = "finalized"
	PeriodLocked    PeriodStatus = "locked"
)

// AttendanceRecord is a single employee's attendance for one day.
type AttendanceRecord struct {
	ID                       string           `db:"id"                          json:"id"`
	PublicID                 string           `db:"public_id"                   json:"public_id"`
	OrgID                    string           `db:"org_id"                      json:"org_id"`
	EmployeeID               string           `db:"employee_id"                 json:"employee_id"`
	AttendanceDate           string           `db:"attendance_date"             json:"attendance_date"`
	ShiftID                  *string          `db:"shift_id"                    json:"shift_id,omitempty"`
	ShiftName                *string          `db:"shift_name"                  json:"shift_name,omitempty"`
	ExpectedIn               *string          `db:"expected_in"                 json:"expected_in,omitempty"`
	ExpectedOut              *string          `db:"expected_out"                json:"expected_out,omitempty"`
	CheckInTime              *string          `db:"check_in_time"               json:"check_in_time,omitempty"`
	CheckOutTime             *string          `db:"check_out_time"              json:"check_out_time,omitempty"`
	BreakMinutes             int              `db:"break_minutes"               json:"break_minutes"`
	RegularHours             float64          `db:"regular_hours"               json:"regular_hours"`
	OvertimeHours            float64          `db:"overtime_hours"              json:"overtime_hours"`
	DayType                  DayType          `db:"day_type"                    json:"day_type"`
	Source                   AttendanceSource `db:"source"                      json:"source"`
	Notes                    *string          `db:"notes"                       json:"notes,omitempty"`
	RegularizationReason     *string          `db:"regularization_reason"       json:"regularization_reason,omitempty"`
	RegularizationInstanceID *string          `db:"regularization_instance_id"  json:"regularization_instance_id,omitempty"`
	Status                   RecordStatus     `db:"status"                      json:"status"`
	ApprovedBy               *string          `db:"approved_by"                 json:"approved_by,omitempty"`
	ApprovedAt               *time.Time       `db:"approved_at"                 json:"approved_at,omitempty"`
	CreatedBy                string           `db:"created_by"                  json:"created_by"`
	CreatedAt                time.Time        `db:"created_at"                  json:"created_at"`
	UpdatedAt                time.Time        `db:"updated_at"                  json:"updated_at"`
}

// AttendancePeriod is a monthly attendance lock.
type AttendancePeriod struct {
	ID                 string       `db:"id"                   json:"id"`
	PublicID           string       `db:"public_id"            json:"public_id"`
	OrgID              string       `db:"org_id"               json:"org_id"`
	PeriodYear         int          `db:"period_year"          json:"period_year"`
	PeriodMonth        int          `db:"period_month"         json:"period_month"`
	Status             PeriodStatus `db:"status"               json:"status"`
	TotalEmployees     int          `db:"total_employees"      json:"total_employees"`
	TotalWorkDays      int          `db:"total_work_days"      json:"total_work_days"`
	TotalPresent       int          `db:"total_present"        json:"total_present"`
	TotalAbsent        int          `db:"total_absent"         json:"total_absent"`
	TotalHolidays      int          `db:"total_holidays"       json:"total_holidays"`
	TotalLeaves        int          `db:"total_leaves"         json:"total_leaves"`
	TotalOvertimeHours float64      `db:"total_overtime_hours" json:"total_overtime_hours"`
	FinalizedAt        *time.Time   `db:"finalized_at"         json:"finalized_at,omitempty"`
	FinalizedBy        *string      `db:"finalized_by"         json:"finalized_by,omitempty"`
	LockedAt           *time.Time   `db:"locked_at"            json:"locked_at,omitempty"`
	LockedBy           *string      `db:"locked_by"            json:"locked_by,omitempty"`
	CreatedBy          string       `db:"created_by"           json:"created_by"`
	CreatedAt          time.Time    `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time    `db:"updated_at"           json:"updated_at"`
}

// CreateRecordRequest creates a single attendance record.
type CreateRecordRequest struct {
	EmployeeID   string           `json:"employee_id"`
	Date         string           `json:"date"`
	CheckIn      *string          `json:"check_in"`  // "09:00" or nil
	CheckOut     *string          `json:"check_out"` // "18:00" or nil
	BreakMinutes *int             `json:"break_minutes"`
	DayType      DayType          `json:"day_type"`
	Source       AttendanceSource `json:"source"`
	Notes        *string          `json:"notes"`
}

// BulkCreateRequest creates attendance records for multiple employees on one date.
type BulkCreateRequest struct {
	Date    string                `json:"date"`
	Records []CreateRecordRequest `json:"records"`
}

// RegularizeRequest asks HR to correct an existing record.
type RegularizeRequest struct {
	NewCheckIn  *string `json:"new_check_in"`
	NewCheckOut *string `json:"new_check_out"`
	Reason      string  `json:"reason"`
}

// PeriodRequest creates a monthly attendance period if it doesn't exist.
type PeriodRequest struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

type RecordListResponse struct {
	Records []*AttendanceRecord `json:"records"`
	Total   int                 `json:"total"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
}

// RecordListFilter narrows the attendance record list query.
type RecordListFilter struct {
	EmployeeID string
	Status     string
	Year       int
	Month      int
	Limit      int
	Offset     int

	// Scope and CallerUserID are set by the handler (from authzSvc.ResolveScope)
	// before calling Service.ListRecords. Scope zero value (authz.ScopeNone)
	// means "no rows" — callers that intend no scoping must explicitly pass
	// authz.ScopeAll.
	Scope        authz.Scope
	CallerUserID string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *RecordListFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

type PeriodListResponse struct {
	Periods []*AttendancePeriod `json:"periods"`
	Total   int                 `json:"total"`
}

// EmployeeSummary is the attendance summary for one employee in a period.
type EmployeeSummary struct {
	EmployeeID    string  `json:"employee_id"`
	WorkDays      int     `json:"work_days"`
	PresentDays   int     `json:"present_days"`
	AbsentDays    int     `json:"absent_days"`
	LeaveDays     int     `json:"leave_days"`
	HolidayDays   int     `json:"holiday_days"`
	OvertimeHours float64 `json:"overtime_hours"`
}

var (
	ErrNotFound                       = errors.New("attendance record not found")
	ErrPeriodNotFound                 = errors.New("attendance period not found")
	ErrEmployeeIDRequired             = errors.New("employee_id is required")
	ErrDateRequired                   = errors.New("date is required (YYYY-MM-DD)")
	ErrInvalidDate                    = errors.New("date must be a valid YYYY-MM-DD")
	ErrInvalidDayType                 = errors.New("invalid day_type")
	ErrDuplicateRecord                = errors.New("attendance record already exists for this employee on this date")
	ErrPeriodFinalized                = errors.New("attendance period is finalized — no edits allowed")
	ErrPeriodAlreadyFinalizedOrLocked = errors.New("attendance period is already finalized or locked")
	ErrPeriodNotOpen                  = errors.New("attendance period must be open to finalize")
	ErrWrongStatus                    = errors.New("action not allowed in current status")
	ErrReasonRequired                 = errors.New("regularization reason is required")
)
