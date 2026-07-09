// backend/internal/hrm/shifts/model.go
package shifts

import (
	"errors"
	"time"
)

type ShiftType string
const (
	ShiftTypeFixed    ShiftType = "fixed"
	ShiftTypeFlexible ShiftType = "flexible"
)
func (t ShiftType) IsValid() bool { return t == ShiftTypeFixed || t == ShiftTypeFlexible }

type AssigneeType string
const (
	AssigneeTypeOrganization AssigneeType = "organization"
	AssigneeTypeDepartment   AssigneeType = "department"
	AssigneeTypeEmployee     AssigneeType = "employee"
)
func (t AssigneeType) IsValid() bool {
	return t == AssigneeTypeOrganization || t == AssigneeTypeDepartment || t == AssigneeTypeEmployee
}

type Shift struct {
	ID                     string    `db:"id"                       json:"id"`
	PublicID               string    `db:"public_id"                json:"public_id"`
	OrgID                  string    `db:"org_id"                   json:"org_id"`
	Name                   string    `db:"name"                     json:"name"`
	Description            *string   `db:"description"              json:"description,omitempty"`
	ShiftType              ShiftType `db:"shift_type"               json:"shift_type"`
	StartTime              *string   `db:"start_time"               json:"start_time,omitempty"`
	EndTime                *string   `db:"end_time"                 json:"end_time,omitempty"`
	CoreStartTime          *string   `db:"core_start_time"          json:"core_start_time,omitempty"`
	CoreEndTime            *string   `db:"core_end_time"            json:"core_end_time,omitempty"`
	WeeklyHoursTarget      *float64  `db:"weekly_hours_target"      json:"weekly_hours_target,omitempty"`
	BreakMinutes           int       `db:"break_minutes"            json:"break_minutes"`
	WorkingDays            []string  `db:"working_days"             json:"working_days"`
	TrackOvertime          bool      `db:"track_overtime"           json:"track_overtime"`
	OvertimeThresholdHours *float64  `db:"overtime_threshold_hours" json:"overtime_threshold_hours,omitempty"`
	TrackBreaks            bool      `db:"track_breaks"             json:"track_breaks"`
	IsDefault              bool      `db:"is_default"               json:"is_default"`
	IsActive               bool      `db:"is_active"                json:"is_active"`
	CreatedBy              string    `db:"created_by"               json:"created_by"`
	CreatedAt              time.Time `db:"created_at"               json:"created_at"`
	UpdatedAt              time.Time `db:"updated_at"               json:"updated_at"`
}

type WorkScheduleAssignment struct {
	ID            string       `db:"id"             json:"id"`
	PublicID      string       `db:"public_id"      json:"public_id"`
	OrgID         string       `db:"org_id"         json:"org_id"`
	ShiftID       string       `db:"shift_id"       json:"shift_id"`
	AssigneeType  AssigneeType `db:"assignee_type"  json:"assignee_type"`
	AssigneeID    string       `db:"assignee_id"    json:"assignee_id"`
	EffectiveDate string       `db:"effective_date" json:"effective_date"`
	EndDate       *string      `db:"end_date"       json:"end_date,omitempty"`
	CreatedBy     string       `db:"created_by"     json:"created_by"`
	CreatedAt     time.Time    `db:"created_at"     json:"created_at"`
}

type CreateShiftRequest struct {
	Name                   string    `json:"name"`
	Description            *string   `json:"description"`
	ShiftType              ShiftType `json:"shift_type"`
	StartTime              *string   `json:"start_time"`
	EndTime                *string   `json:"end_time"`
	CoreStartTime          *string   `json:"core_start_time"`
	CoreEndTime            *string   `json:"core_end_time"`
	WeeklyHoursTarget      *float64  `json:"weekly_hours_target"`
	BreakMinutes           *int      `json:"break_minutes"`
	WorkingDays            []string  `json:"working_days"`
	TrackOvertime          bool      `json:"track_overtime"`
	OvertimeThresholdHours *float64  `json:"overtime_threshold_hours"`
	TrackBreaks            bool      `json:"track_breaks"`
	IsDefault              bool      `json:"is_default"`
}

type UpdateShiftRequest struct {
	Name                   *string   `json:"name"`
	Description            *string   `json:"description"`
	StartTime              *string   `json:"start_time"`
	EndTime                *string   `json:"end_time"`
	CoreStartTime          *string   `json:"core_start_time"`
	CoreEndTime            *string   `json:"core_end_time"`
	WeeklyHoursTarget      *float64  `json:"weekly_hours_target"`
	BreakMinutes           *int      `json:"break_minutes"`
	WorkingDays            []string  `json:"working_days"`
	TrackOvertime          *bool     `json:"track_overtime"`
	OvertimeThresholdHours *float64  `json:"overtime_threshold_hours"`
	TrackBreaks            *bool     `json:"track_breaks"`
	IsDefault              *bool     `json:"is_default"`
	IsActive               *bool     `json:"is_active"`
}

type AssignShiftRequest struct {
	ShiftID       string       `json:"shift_id"`
	AssigneeType  AssigneeType `json:"assignee_type"`
	AssigneeID    string       `json:"assignee_id"`
	EffectiveDate string       `json:"effective_date"`
	EndDate       *string      `json:"end_date"`
}

type ShiftListResponse struct {
	Shifts []*Shift `json:"shifts"`
	Total  int      `json:"total"`
}

type AssignmentListResponse struct {
	Assignments []*WorkScheduleAssignment `json:"assignments"`
	Total       int                       `json:"total"`
}

var (
	ErrShiftNotFound        = errors.New("shift not found")
	ErrAssignmentNotFound   = errors.New("schedule assignment not found")
	ErrNameRequired         = errors.New("name is required")
	ErrNameConflict         = errors.New("a shift with this name already exists")
	ErrInvalidShiftType     = errors.New("shift_type must be: fixed or flexible")
	ErrFixedTimeRequired    = errors.New("start_time and end_time are required for fixed shifts")
	ErrFlexHoursRequired    = errors.New("weekly_hours_target is required for flexible shifts")
	ErrInvalidAssigneeType  = errors.New("assignee_type must be: organization, department, or employee")
	ErrAssigneeIDRequired   = errors.New("assignee_id is required")
	ErrEffectiveDateRequired = errors.New("effective_date is required")
)
