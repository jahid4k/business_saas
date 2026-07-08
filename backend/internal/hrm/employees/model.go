// backend/internal/hrm/employees/model.go
package employees

import (
	"errors"
	"time"
)

// EmploymentType defines the allowed values for an employee's employment type.
type EmploymentType string

const (
	EmploymentTypeFullTime   EmploymentType = "full_time"
	EmploymentTypePartTime   EmploymentType = "part_time"
	EmploymentTypeContractor EmploymentType = "contractor"
	EmploymentTypeIntern     EmploymentType = "intern"
)

func (t EmploymentType) IsValid() bool {
	switch t {
	case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeContractor, EmploymentTypeIntern:
		return true
	}
	return false
}

// EmployeeStatus defines the allowed lifecycle states of an employee record.
type EmployeeStatus string

const (
	EmployeeStatusActive     EmployeeStatus = "active"
	EmployeeStatusInactive   EmployeeStatus = "inactive"
	EmployeeStatusOnLeave    EmployeeStatus = "on_leave"
	EmployeeStatusTerminated EmployeeStatus = "terminated"
)

func (s EmployeeStatus) IsValid() bool {
	switch s {
	case EmployeeStatusActive, EmployeeStatusInactive, EmployeeStatusOnLeave, EmployeeStatusTerminated:
		return true
	}
	return false
}

// Gender defines the allowed values for the gender field.
type Gender string

const (
	GenderMale           Gender = "male"
	GenderFemale         Gender = "female"
	GenderOther          Gender = "other"
	GenderPreferNotToSay Gender = "prefer_not_to_say"
)

func (g Gender) IsValid() bool {
	switch g {
	case GenderMale, GenderFemale, GenderOther, GenderPreferNotToSay:
		return true
	}
	return false
}

// Employee is the core domain type for an HRM employee record.
// Mirrors hrm_employees table columns exactly.
type Employee struct {
	ID              string         `db:"id"               json:"id"`
	PublicID        string         `db:"public_id"        json:"public_id"`
	OrgID           string         `db:"org_id"           json:"org_id"`
	UserID          *string        `db:"user_id"          json:"user_id,omitempty"`
	EmployeeNumber  *string        `db:"employee_number"  json:"employee_number,omitempty"`
	FirstName       string         `db:"first_name"       json:"first_name"`
	LastName        *string        `db:"last_name"        json:"last_name,omitempty"`
	Email           *string        `db:"email"            json:"email,omitempty"`
	WorkEmail       *string        `db:"work_email"       json:"work_email,omitempty"`
	Phone           *string        `db:"phone"            json:"phone,omitempty"`
	WorkPhone       *string        `db:"work_phone"       json:"work_phone,omitempty"`
	DateOfBirth     *time.Time     `db:"date_of_birth"    json:"date_of_birth,omitempty"`
	Gender          *string        `db:"gender"           json:"gender,omitempty"`
	AvatarURL       *string        `db:"avatar_url"       json:"avatar_url,omitempty"`
	HireDate        time.Time      `db:"hire_date"        json:"hire_date"`
	TerminationDate *time.Time     `db:"termination_date" json:"termination_date,omitempty"`
	EmploymentType  EmploymentType `db:"employment_type"  json:"employment_type"`
	Status          EmployeeStatus `db:"status"           json:"status"`
	DepartmentID    *string        `db:"department_id"    json:"department_id,omitempty"`
	PositionID      *string        `db:"position_id"      json:"position_id,omitempty"`
	ManagerID       *string        `db:"manager_id"       json:"manager_id,omitempty"`
	Address         *string        `db:"address"          json:"address,omitempty"`
	City            *string        `db:"city"             json:"city,omitempty"`
	Country         *string        `db:"country"          json:"country,omitempty"`
	Notes           *string        `db:"notes"            json:"notes,omitempty"`
	CreatedBy       string         `db:"created_by"       json:"created_by"`
	CreatedAt       time.Time      `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"       json:"updated_at"`
}

// ListFilter narrows the employee list query.
// Zero values mean "no filter on this field".
type ListFilter struct {
	Status         EmployeeStatus
	EmploymentType EmploymentType
	DepartmentID   string
	ManagerID      string
	Search         string // fuzzy match on first_name, last_name, email, employee_number
	Limit          int
	Offset         int
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *ListFilter) Normalise() {
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

// CreateEmployeeRequest is the body for POST /hrm/employees.
type CreateEmployeeRequest struct {
	FirstName       string  `json:"first_name"`
	LastName        *string `json:"last_name"`
	Email           *string `json:"email"`
	WorkEmail       *string `json:"work_email"`
	Phone           *string `json:"phone"`
	WorkPhone       *string `json:"work_phone"`
	EmployeeNumber  *string `json:"employee_number"`
	DateOfBirth     *string `json:"date_of_birth"`      // ISO 8601 date: "YYYY-MM-DD"
	Gender          *string `json:"gender"`
	HireDate        string  `json:"hire_date"`           // required — ISO 8601 date: "YYYY-MM-DD"
	EmploymentType  *string `json:"employment_type"`     // default: full_time
	DepartmentID    *string `json:"department_id"`
	PositionID      *string `json:"position_id"`
	ManagerID       *string `json:"manager_id"`
	UserID          *string `json:"user_id"`            // optional platform user link
	Address         *string `json:"address"`
	City            *string `json:"city"`
	Country         *string `json:"country"`
	AvatarURL       *string `json:"avatar_url"`
	Notes           *string `json:"notes"`
}

// UpdateEmployeeRequest is the body for PATCH /hrm/employees/:empId.
// All fields are optional. Only non-nil pointers are applied.
type UpdateEmployeeRequest struct {
	FirstName      *string `json:"first_name"`
	LastName       *string `json:"last_name"`
	Email          *string `json:"email"`
	WorkEmail      *string `json:"work_email"`
	Phone          *string `json:"phone"`
	WorkPhone      *string `json:"work_phone"`
	EmployeeNumber *string `json:"employee_number"`
	DateOfBirth    *string `json:"date_of_birth"`
	Gender         *string `json:"gender"`
	AvatarURL      *string `json:"avatar_url"`
	EmploymentType *string `json:"employment_type"`
	Status         *string `json:"status"`
	DepartmentID   *string `json:"department_id"`
	PositionID     *string `json:"position_id"`
	ManagerID      *string `json:"manager_id"`
	UserID         *string `json:"user_id"`
	Address        *string `json:"address"`
	City           *string `json:"city"`
	Country        *string `json:"country"`
	Notes          *string `json:"notes"`
}

// TerminateEmployeeRequest is the body for POST /hrm/employees/:empId/terminate.
type TerminateEmployeeRequest struct {
	TerminationDate string  `json:"termination_date"` // required — ISO 8601 date: "YYYY-MM-DD"
	Notes           *string `json:"notes"`
}

// EmployeeListResponse wraps paginated list results.
type EmployeeListResponse struct {
	Employees []*Employee `json:"employees"`
	Total     int         `json:"total"`
	Limit     int         `json:"limit"`
	Offset    int         `json:"offset"`
}

// Sentinel errors
var (
	ErrEmployeeNotFound          = errors.New("employee not found")
	ErrFirstNameRequired         = errors.New("first_name is required")
	ErrFirstNameTooLong          = errors.New("first_name must not exceed 100 characters")
	ErrHireDateRequired          = errors.New("hire_date is required")
	ErrInvalidHireDate           = errors.New("hire_date must be a valid date in YYYY-MM-DD format")
	ErrInvalidDateOfBirth        = errors.New("date_of_birth must be a valid date in YYYY-MM-DD format")
	ErrInvalidTerminationDate    = errors.New("termination_date must be a valid date in YYYY-MM-DD format")
	ErrTerminationBeforeHire     = errors.New("termination_date cannot be before hire_date")
	ErrInvalidEmploymentType     = errors.New("employment_type must be one of: full_time, part_time, contractor, intern")
	ErrInvalidStatus             = errors.New("status must be one of: active, inactive, on_leave, terminated")
	ErrInvalidGender             = errors.New("gender must be one of: male, female, other, prefer_not_to_say")
	ErrAlreadyTerminated         = errors.New("employee is already terminated")
	ErrEmployeeNumberConflict    = errors.New("an employee with this employee_number already exists in the organization")
	ErrSelfManager               = errors.New("an employee cannot be their own manager")
)
