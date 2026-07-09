// backend/internal/hrm/departments/model.go
package departments

import (
	"errors"
	"time"
)

// Department is the core domain type for an HRM department.
// Mirrors hrm_departments table columns exactly.
type Department struct {
	ID                 string    `db:"id"                   json:"id"`
	PublicID           string    `db:"public_id"            json:"public_id"`
	OrgID              string    `db:"org_id"               json:"org_id"`
	Name               string    `db:"name"                 json:"name"`
	Description        *string   `db:"description"          json:"description,omitempty"`
	ParentDepartmentID *string   `db:"parent_department_id" json:"parent_department_id,omitempty"`
	HeadEmployeeID     *string   `db:"head_employee_id"     json:"head_employee_id,omitempty"`
	IsActive           bool      `db:"is_active"            json:"is_active"`
	CreatedBy          string    `db:"created_by"           json:"created_by"`
	CreatedAt          time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"           json:"updated_at"`
}

// CreateDepartmentRequest is the body for POST /hrm/departments.
type CreateDepartmentRequest struct {
	Name               string  `json:"name"`
	Description        *string `json:"description"`
	ParentDepartmentID *string `json:"parent_department_id"` // optional
	HeadEmployeeID     *string `json:"head_employee_id"`     // optional
}

// UpdateDepartmentRequest is the body for PATCH /hrm/departments/:deptId.
// All fields are optional — only non-nil values are applied.
type UpdateDepartmentRequest struct {
	Name               *string `json:"name"`
	Description        *string `json:"description"`
	ParentDepartmentID *string `json:"parent_department_id"`
	HeadEmployeeID     *string `json:"head_employee_id"`
	IsActive           *bool   `json:"is_active"`
}

// DepartmentListResponse wraps paginated list results.
type DepartmentListResponse struct {
	Departments []*Department `json:"departments"`
	Total       int           `json:"total"`
}

// Sentinel errors
var (
	ErrDepartmentNotFound  = errors.New("department not found")
	ErrNameRequired        = errors.New("name is required")
	ErrNameTooLong         = errors.New("name must not exceed 150 characters")
	ErrNameConflict        = errors.New("a department with this name already exists in the organization")
	ErrCircularParent      = errors.New("a department cannot be its own parent")
)
