// backend/internal/hrm/holidays/model.go
package holidays

import (
	"errors"
	"time"
)

type HolidayType string

const (
	HolidayTypePublic   HolidayType = "public"
	HolidayTypeCompany  HolidayType = "company"
	HolidayTypeOptional HolidayType = "optional"
)

func (t HolidayType) IsValid() bool {
	return t == HolidayTypePublic || t == HolidayTypeCompany || t == HolidayTypeOptional
}

type CalendarAssigneeType string

const (
	AssigneeOrganization CalendarAssigneeType = "organization"
	AssigneeDepartment   CalendarAssigneeType = "department"
	AssigneeEmployee     CalendarAssigneeType = "employee"
)

func (t CalendarAssigneeType) IsValid() bool {
	return t == AssigneeOrganization || t == AssigneeDepartment || t == AssigneeEmployee
}

type HolidayCalendar struct {
	ID          string     `db:"id"           json:"id"`
	PublicID    string     `db:"public_id"    json:"public_id"`
	OrgID       string     `db:"org_id"       json:"org_id"`
	Name        string     `db:"name"         json:"name"`
	Description *string    `db:"description"  json:"description,omitempty"`
	CountryCode *string    `db:"country_code" json:"country_code,omitempty"`
	Year        int        `db:"year"         json:"year"`
	IsActive    bool       `db:"is_active"    json:"is_active"`
	CreatedBy   string     `db:"created_by"   json:"created_by"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"   json:"updated_at"`
	Holidays    []*Holiday `db:"-"           json:"holidays,omitempty"`
}

type Holiday struct {
	ID           string      `db:"id"            json:"id"`
	PublicID     string      `db:"public_id"     json:"public_id"`
	CalendarID   string      `db:"calendar_id"   json:"calendar_id"`
	Name         string      `db:"name"          json:"name"`
	Date         string      `db:"date"          json:"date"` // YYYY-MM-DD
	HolidayType  HolidayType `db:"holiday_type"  json:"holiday_type"`
	IsPaid       bool        `db:"is_paid"       json:"is_paid"`
	RepeatYearly bool        `db:"repeat_yearly" json:"repeat_yearly"`
	CreatedAt    time.Time   `db:"created_at"    json:"created_at"`
}

type CalendarAssignment struct {
	ID            string               `db:"id"             json:"id"`
	PublicID      string               `db:"public_id"      json:"public_id"`
	OrgID         string               `db:"org_id"         json:"org_id"`
	CalendarID    string               `db:"calendar_id"    json:"calendar_id"`
	AssigneeType  CalendarAssigneeType `db:"assignee_type"  json:"assignee_type"`
	AssigneeID    string               `db:"assignee_id"    json:"assignee_id"`
	EffectiveDate string               `db:"effective_date" json:"effective_date"`
	CreatedBy     string               `db:"created_by"     json:"created_by"`
	CreatedAt     time.Time            `db:"created_at"     json:"created_at"`
}

// ─── Requests ─────────────────────────────────────────────────────────────────

type CreateCalendarRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	CountryCode *string `json:"country_code"`
	Year        int     `json:"year"`
}
type UpdateCalendarRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}
type CreateHolidayRequest struct {
	Name         string      `json:"name"`
	Date         string      `json:"date"`
	HolidayType  HolidayType `json:"holiday_type"`
	IsPaid       bool        `json:"is_paid"`
	RepeatYearly bool        `json:"repeat_yearly"`
}
type UpdateHolidayRequest struct {
	Name         *string      `json:"name"`
	Date         *string      `json:"date"`
	HolidayType  *HolidayType `json:"holiday_type"`
	IsPaid       *bool        `json:"is_paid"`
	RepeatYearly *bool        `json:"repeat_yearly"`
}
type AssignCalendarRequest struct {
	CalendarID    string               `json:"calendar_id"`
	AssigneeType  CalendarAssigneeType `json:"assignee_type"`
	AssigneeID    string               `json:"assignee_id"`
	EffectiveDate string               `json:"effective_date"`
}

type CalendarListResponse struct {
	Calendars []*HolidayCalendar `json:"calendars"`
	Total     int                `json:"total"`
}
type HolidayListResponse struct {
	Holidays []*Holiday `json:"holidays"`
	Total    int        `json:"total"`
}

var (
	ErrCalendarNotFound      = errors.New("holiday calendar not found")
	ErrHolidayNotFound       = errors.New("holiday not found")
	ErrAssignmentNotFound    = errors.New("calendar assignment not found")
	ErrNameRequired          = errors.New("name is required")
	ErrNameConflict          = errors.New("a calendar with this name and year already exists")
	ErrInvalidYear           = errors.New("year must be between 2000 and 2100")
	ErrDateRequired          = errors.New("date is required (YYYY-MM-DD)")
	ErrInvalidDate           = errors.New("date must be a valid YYYY-MM-DD")
	ErrDateConflict          = errors.New("a holiday on this date already exists in this calendar")
	ErrInvalidHolidayType    = errors.New("holiday_type must be: public, company, or optional")
	ErrInvalidAssigneeType   = errors.New("assignee_type must be: organization, department, or employee")
	ErrAssigneeIDRequired    = errors.New("assignee_id is required")
	ErrEffectiveDateRequired = errors.New("effective_date is required (YYYY-MM-DD)")
)
