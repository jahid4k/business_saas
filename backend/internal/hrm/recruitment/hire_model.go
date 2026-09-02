// backend/internal/hrm/recruitment/hire_model.go
package recruitment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// EmployeeCreator is the minimal slice of employees.Service hire conversion
// needs. Declared here, not imported from internal/hrm/employees —
// recruitment orchestrates and owns the transaction, so it owns the
// interface and request type; employees imports recruitment to implement
// it. Mirrors crm/leads/service.go's ContactCreator/DealCreator exactly:
// the orchestrator never imports the provider's types.
type EmployeeCreator interface {
	CreateEmployeeTx(ctx context.Context, tx pgx.Tx, orgID, createdBy string, req HireEmployeeRequest) (employeeID, employeePublicID string, err error)
	// AfterHireCommit runs the provider's normal post-commit side effects
	// (audit log, onboarding checklist) for an employee inserted via
	// CreateEmployeeTx. Called ONLY after HireApplication's own transaction
	// has committed successfully.
	AfterHireCommit(ctx context.Context, orgID, actorID, employeeID string)
}

// HireEmployeeRequest is the data HireApplication passes to EmployeeCreator
// to materialize an employee record from a hired candidate/application.
type HireEmployeeRequest struct {
	FirstName, LastName                 string
	Email                               *string
	HireDate                            string
	DepartmentID, PositionID, ManagerID *string
	EmploymentType                      *string
	SourceCandidateID                   string
}

// HireApplicationRequest is the body for POST .../applications/:applicationId/hire.
// All fields are optional overrides — when nil, HireApplication defaults
// from the candidate and requisition.
type HireApplicationRequest struct {
	DepartmentID   *string `json:"department_id"`
	PositionID     *string `json:"position_id"`
	ManagerID      *string `json:"manager_id"`
	EmploymentType *string `json:"employment_type"`
	HireDate       *string `json:"hire_date"` // ISO 8601 date; defaults to today
}

// HireApplicationResponse is the result of a successful hire conversion.
type HireApplicationResponse struct {
	Application      *Application `json:"application"`
	EmployeeID       string       `json:"employee_id"`
	EmployeePublicID string       `json:"employee_public_id"`
}

var (
	ErrApplicationNotHired     = errors.New("application must be in a hired stage before it can be converted to an employee")
	ErrApplicationAlreadyHired = errors.New("application has already been converted to an employee")
)
