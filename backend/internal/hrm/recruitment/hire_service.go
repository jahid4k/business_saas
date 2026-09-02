// backend/internal/hrm/recruitment/hire_service.go
package recruitment

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// HireService is embedded into Service — see service.go.
type HireService interface {
	// HireApplication materializes an employee record from a hired
	// application. Atomic for the employee insert + application's
	// converted_employee_id + requisition's filled_count — all inside one
	// transaction via Repository.BeginTx. Onboarding checklist instantiation
	// and the audit log entry stay a post-commit best-effort side effect
	// (EmployeeCreator.AfterHireCommit), exactly matching how
	// employees.Service.Create already behaves — see hire_model.go's
	// EmployeeCreator doc comment for why that boundary is deliberate.
	HireApplication(ctx context.Context, orgID, applicationRef, actorID string, req HireApplicationRequest) (*HireApplicationResponse, error)
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// HireApplication requires the application to already be in the "hired"
// status — reached via the existing MoveApplication path into a stage of
// kind "hired". This method performs the separate, idempotency-guarded step
// of materializing that outcome into an actual hrm_employees row.
func (s *serviceImpl) HireApplication(ctx context.Context, orgID, applicationRef, actorID string, req HireApplicationRequest) (*HireApplicationResponse, error) {
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	if app.ConvertedEmployeeID != nil {
		return nil, ErrApplicationAlreadyHired
	}
	if app.Status != ApplicationStatusHired {
		return nil, ErrApplicationNotHired
	}

	candidate, err := s.repo.FindCandidateByRef(ctx, orgID, app.CandidateID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: candidate: %w", err)
	}
	if candidate == nil {
		return nil, ErrCandidateNotFound
	}
	posting, err := s.repo.FindPostingByRef(ctx, orgID, app.PostingID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: posting: %w", err)
	}
	if posting == nil {
		return nil, ErrPostingNotFound
	}
	requisition, err := s.repo.FindRequisitionByRef(ctx, orgID, posting.RequisitionID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: requisition: %w", err)
	}
	if requisition == nil {
		return nil, ErrRequisitionNotFound
	}

	hireDate := time.Now().Format(dateLayout)
	if req.HireDate != nil && strings.TrimSpace(*req.HireDate) != "" {
		hireDate = strings.TrimSpace(*req.HireDate)
	}

	departmentID := requisition.DepartmentID
	if req.DepartmentID != nil {
		departmentID = req.DepartmentID
	}
	positionID := requisition.PositionID
	if req.PositionID != nil {
		positionID = req.PositionID
	}
	managerID := requisition.HiringManagerID
	if req.ManagerID != nil {
		managerID = req.ManagerID
	}
	employmentType := string(requisition.EmploymentType)
	if req.EmploymentType != nil && strings.TrimSpace(*req.EmploymentType) != "" {
		employmentType = strings.TrimSpace(*req.EmploymentType)
	}

	hireReq := HireEmployeeRequest{
		FirstName: candidate.FirstName, LastName: derefOrEmpty(candidate.LastName),
		Email: candidate.Email, HireDate: hireDate,
		DepartmentID: departmentID, PositionID: positionID, ManagerID: managerID,
		EmploymentType: &employmentType, SourceCandidateID: candidate.ID,
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Re-check preconditions under the row lock — closes the race between
	// the reads above and this point, so two concurrent hire calls on the
	// same application cannot both create an employee.
	locked, err := s.repo.LockApplicationForHireTx(ctx, tx, orgID, app.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: lock: %w", err)
	}
	if locked == nil {
		return nil, ErrApplicationNotFound
	}
	if locked.ConvertedEmployeeID != nil {
		return nil, ErrApplicationAlreadyHired
	}
	if locked.Status != ApplicationStatusHired {
		return nil, ErrApplicationNotHired
	}

	employeeID, employeePublicID, err := s.employeeCreator.CreateEmployeeTx(ctx, tx, orgID, actorID, hireReq)
	if err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: create employee: %w", err)
	}
	if err := s.repo.SetApplicationConvertedEmployeeTx(ctx, tx, app.ID, employeeID); err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: update application: %w", err)
	}
	if err := s.repo.IncrementRequisitionFilledCountTx(ctx, tx, requisition.ID); err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: update requisition: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("recruitment: HireApplication: commit: %w", err)
	}

	// Post-commit, best-effort side effects (audit log + onboarding
	// checklist) — must run only after the commit above succeeds; see
	// EmployeeCreator's doc comment in hire_model.go.
	s.employeeCreator.AfterHireCommit(ctx, orgID, actorID, employeeID)

	app.ConvertedEmployeeID = &employeeID
	return &HireApplicationResponse{Application: app, EmployeeID: employeeID, EmployeePublicID: employeePublicID}, nil
}
