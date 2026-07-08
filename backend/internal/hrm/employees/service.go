// backend/internal/hrm/employees/service.go
package employees

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
)

// dateLayout is the ISO 8601 date format used for hire/termination/birth dates.
const dateLayout = "2006-01-02"

// Service defines business logic for HRM employees.
type Service interface {
	List(ctx context.Context, orgID string, filter ListFilter) (*EmployeeListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Employee, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateEmployeeRequest) (*Employee, error)
	Update(ctx context.Context, orgID, ref string, req UpdateEmployeeRequest) (*Employee, error)
	Terminate(ctx context.Context, orgID, ref, actorID string, req TerminateEmployeeRequest) (*Employee, error)
	Delete(ctx context.Context, orgID, ref string) error
}

type serviceImpl struct {
	repo  Repository
	audit audit.Service
}

func NewService(repo Repository, auditSvc audit.Service) Service {
	return &serviceImpl{repo: repo, audit: auditSvc}
}

func (s *serviceImpl) List(ctx context.Context, orgID string, filter ListFilter) (*EmployeeListResponse, error) {
	filter.Normalise()

	list, err := s.repo.FindAll(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("employees: List: %w", err)
	}
	if list == nil {
		list = []*Employee{}
	}
	total, err := s.repo.Count(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("employees: List: count: %w", err)
	}
	return &EmployeeListResponse{
		Employees: list, Total: total, Limit: filter.Limit, Offset: filter.Offset,
	}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Employee, error) {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("employees: Get: %w", err)
	}
	if e == nil {
		return nil, ErrEmployeeNotFound
	}
	return e, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateEmployeeRequest) (*Employee, error) {
	firstName := strings.TrimSpace(req.FirstName)
	if firstName == "" {
		return nil, ErrFirstNameRequired
	}
	if len(firstName) > 100 {
		return nil, ErrFirstNameTooLong
	}

	hireDateStr := strings.TrimSpace(req.HireDate)
	if hireDateStr == "" {
		return nil, ErrHireDateRequired
	}
	hireDate, err := time.Parse(dateLayout, hireDateStr)
	if err != nil {
		return nil, ErrInvalidHireDate
	}

	empType := EmploymentTypeFullTime
	if req.EmploymentType != nil && strings.TrimSpace(*req.EmploymentType) != "" {
		empType = EmploymentType(strings.TrimSpace(*req.EmploymentType))
		if !empType.IsValid() {
			return nil, ErrInvalidEmploymentType
		}
	}

	if req.Gender != nil && strings.TrimSpace(*req.Gender) != "" {
		if !Gender(strings.TrimSpace(*req.Gender)).IsValid() {
			return nil, ErrInvalidGender
		}
	}

	var dateOfBirth *time.Time
	if req.DateOfBirth != nil && strings.TrimSpace(*req.DateOfBirth) != "" {
		dob, err := time.Parse(dateLayout, strings.TrimSpace(*req.DateOfBirth))
		if err != nil {
			return nil, ErrInvalidDateOfBirth
		}
		dateOfBirth = &dob
	}

	// Unique employee_number check
	if req.EmployeeNumber != nil && strings.TrimSpace(*req.EmployeeNumber) != "" {
		exists, err := s.repo.ExistsByEmployeeNumber(ctx, orgID, *req.EmployeeNumber, "")
		if err != nil {
			return nil, fmt.Errorf("employees: Create: emp_number check: %w", err)
		}
		if exists {
			return nil, ErrEmployeeNumberConflict
		}
	}

	e := &Employee{
		OrgID:          orgID,
		FirstName:      firstName,
		HireDate:       hireDate,
		EmploymentType: empType,
		Status:         EmployeeStatusActive,
		CreatedBy:      createdBy,
	}
	if req.EmployeeNumber != nil && strings.TrimSpace(*req.EmployeeNumber) != "" {
		e.EmployeeNumber = req.EmployeeNumber
	}
	e.LastName = nilIfEmpty(req.LastName)
	e.Email = nilIfEmpty(req.Email)
	e.WorkEmail = nilIfEmpty(req.WorkEmail)
	e.Phone = nilIfEmpty(req.Phone)
	e.WorkPhone = nilIfEmpty(req.WorkPhone)
	e.Gender = nilIfEmpty(req.Gender)
	e.AvatarURL = nilIfEmpty(req.AvatarURL)
	e.DepartmentID = nilIfEmpty(req.DepartmentID)
	e.PositionID = nilIfEmpty(req.PositionID)
	e.ManagerID = nilIfEmpty(req.ManagerID)
	e.UserID = nilIfEmpty(req.UserID)
	e.Address = nilIfEmpty(req.Address)
	e.City = nilIfEmpty(req.City)
	e.Country = nilIfEmpty(req.Country)
	e.Notes = nilIfEmpty(req.Notes)
	e.DateOfBirth = dateOfBirth

	if err := s.repo.Create(ctx, e); err != nil {
		return nil, fmt.Errorf("employees: Create: %w", err)
	}

	s.audit.Log(ctx, audit.EventHRMEmployeeCreated, createdBy, orgID, "", "", map[string]string{
		"employee_id": e.ID, "first_name": e.FirstName,
	})

	return e, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdateEmployeeRequest) (*Employee, error) {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("employees: Update: %w", err)
	}
	if e == nil {
		return nil, ErrEmployeeNotFound
	}

	if req.FirstName != nil {
		fn := strings.TrimSpace(*req.FirstName)
		if fn == "" {
			return nil, ErrFirstNameRequired
		}
		if len(fn) > 100 {
			return nil, ErrFirstNameTooLong
		}
		e.FirstName = fn
	}
	if req.EmployeeNumber != nil {
		num := strings.TrimSpace(*req.EmployeeNumber)
		if num == "" {
			e.EmployeeNumber = nil
		} else {
			exists, err := s.repo.ExistsByEmployeeNumber(ctx, orgID, num, e.ID)
			if err != nil {
				return nil, fmt.Errorf("employees: Update: emp_number check: %w", err)
			}
			if exists {
				return nil, ErrEmployeeNumberConflict
			}
			e.EmployeeNumber = &num
		}
	}
	if req.Status != nil {
		st := EmployeeStatus(strings.TrimSpace(*req.Status))
		if !st.IsValid() {
			return nil, ErrInvalidStatus
		}
		e.Status = st
	}
	if req.EmploymentType != nil {
		et := EmploymentType(strings.TrimSpace(*req.EmploymentType))
		if !et.IsValid() {
			return nil, ErrInvalidEmploymentType
		}
		e.EmploymentType = et
	}
	if req.Gender != nil {
		g := strings.TrimSpace(*req.Gender)
		if g != "" && !Gender(g).IsValid() {
			return nil, ErrInvalidGender
		}
		e.Gender = nilIfEmpty(&g)
	}
	if req.DateOfBirth != nil {
		if strings.TrimSpace(*req.DateOfBirth) == "" {
			e.DateOfBirth = nil
		} else {
			dob, err := time.Parse(dateLayout, strings.TrimSpace(*req.DateOfBirth))
			if err != nil {
				return nil, ErrInvalidDateOfBirth
			}
			e.DateOfBirth = &dob
		}
	}
	if req.ManagerID != nil {
		mid := strings.TrimSpace(*req.ManagerID)
		if mid != "" && (mid == e.ID || mid == e.PublicID) {
			return nil, ErrSelfManager
		}
		e.ManagerID = nilIfEmpty(&mid)
	}

	// Simple pointer-swap fields — nil means "clear", non-empty means "set"
	applyPtr(&e.LastName, req.LastName)
	applyPtr(&e.Email, req.Email)
	applyPtr(&e.WorkEmail, req.WorkEmail)
	applyPtr(&e.Phone, req.Phone)
	applyPtr(&e.WorkPhone, req.WorkPhone)
	applyPtr(&e.AvatarURL, req.AvatarURL)
	applyPtr(&e.DepartmentID, req.DepartmentID)
	applyPtr(&e.PositionID, req.PositionID)
	applyPtr(&e.UserID, req.UserID)
	applyPtr(&e.Address, req.Address)
	applyPtr(&e.City, req.City)
	applyPtr(&e.Country, req.Country)
	applyPtr(&e.Notes, req.Notes)

	if err := s.repo.Update(ctx, e); err != nil {
		return nil, fmt.Errorf("employees: Update: %w", err)
	}
	return e, nil
}

func (s *serviceImpl) Terminate(ctx context.Context, orgID, ref, actorID string, req TerminateEmployeeRequest) (*Employee, error) {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("employees: Terminate: %w", err)
	}
	if e == nil {
		return nil, ErrEmployeeNotFound
	}
	if e.Status == EmployeeStatusTerminated {
		return nil, ErrAlreadyTerminated
	}

	termDateStr := strings.TrimSpace(req.TerminationDate)
	if termDateStr == "" {
		return nil, ErrInvalidTerminationDate
	}
	termDate, err := time.Parse(dateLayout, termDateStr)
	if err != nil {
		return nil, ErrInvalidTerminationDate
	}
	if termDate.Before(e.HireDate) {
		return nil, ErrTerminationBeforeHire
	}

	e.Status = EmployeeStatusTerminated
	e.TerminationDate = &termDate
	if req.Notes != nil {
		e.Notes = req.Notes
	}

	if err := s.repo.Update(ctx, e); err != nil {
		return nil, fmt.Errorf("employees: Terminate: %w", err)
	}

	s.audit.Log(ctx, audit.EventHRMEmployeeTerminated, actorID, orgID, "", "", map[string]string{
		"employee_id": e.ID, "termination_date": termDateStr,
	})

	return e, nil
}

func (s *serviceImpl) Delete(ctx context.Context, orgID, ref string) error {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("employees: Delete: %w", err)
	}
	if e == nil {
		return ErrEmployeeNotFound
	}
	if err := s.repo.Delete(ctx, orgID, ref); err != nil {
		return fmt.Errorf("employees: Delete: %w", err)
	}
	return nil
}

// ----------------------------------------------------------
// Private helpers
// ----------------------------------------------------------

// nilIfEmpty returns nil when the pointed-to string is empty after trimming.
func nilIfEmpty(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

// applyPtr sets *dst to nil when src points to an empty string,
// or to a trimmed copy of *src when src is non-nil and non-empty.
// When src is nil the destination is left unchanged.
func applyPtr(dst **string, src *string) {
	if src == nil {
		return
	}
	v := strings.TrimSpace(*src)
	if v == "" {
		*dst = nil
	} else {
		*dst = &v
	}
}
