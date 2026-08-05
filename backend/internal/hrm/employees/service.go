// backend/internal/hrm/employees/service.go
package employees

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/hrm/recruitment"
)

// dateLayout is the ISO 8601 date format used for hire/termination/birth dates.
const dateLayout = "2006-01-02"

// ChecklistHook is the minimal slice of the HRM onboarding consumer that
// employee creation needs. Declared here and implemented in
// internal/hrm/onboarding so this package keeps zero cross-module imports —
// the authz.SessionRevoker shape. A nil hook is valid: Create then behaves
// exactly as it did before Phase 3.
type ChecklistHook interface {
	OnEmployeeCreated(ctx context.Context, orgID, actorID, employeeID string) error
}

// Service defines business logic for HRM employees.
type Service interface {
	List(ctx context.Context, orgID string, filter ListFilter) (*EmployeeListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Employee, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateEmployeeRequest) (*Employee, error)
	Update(ctx context.Context, orgID, ref string, req UpdateEmployeeRequest) (*Employee, error)
	Terminate(ctx context.Context, orgID, ref, actorID string, req TerminateEmployeeRequest) (*Employee, error)
	Delete(ctx context.Context, orgID, ref string) error
	ListStatuses(ctx context.Context, orgID string) ([]*EmployeeStatusModel, error)
	CreateStatus(ctx context.Context, orgID string, req CreateEmployeeStatusRequest) (*EmployeeStatusModel, error)
	UpdateStatus(ctx context.Context, orgID, statusID string, req UpdateEmployeeStatusRequest) (*EmployeeStatusModel, error)
	DeleteStatus(ctx context.Context, orgID, statusID string) error

	// CreateEmployeeTx and AfterHireCommit together implement
	// recruitment.EmployeeCreator (internal/hrm/recruitment declares that
	// interface; this package imports recruitment only for the request
	// type, the crm/leads ContactCreator/DealCreator precedent — recruitment
	// orchestrates the transaction, employees is the provider).
	//
	// CreateEmployeeTx does ONLY the insert, inside the caller's tx. It must
	// have zero post-commit side effects — the caller's outer transaction
	// (which also updates the application and requisition) might still roll
	// back after this returns, and both the audit log and the onboarding
	// checklist hook are independent, non-transactional writes (audit.Log
	// swaps to a background context specifically so it can outlive the
	// request; the checklist engine opens its own transaction). Firing
	// either one before the outer commit would record an employee that
	// might never actually exist.
	CreateEmployeeTx(ctx context.Context, tx pgx.Tx, orgID, createdBy string, req recruitment.HireEmployeeRequest) (employeeID, employeePublicID string, err error)

	// AfterHireCommit runs Create's normal post-commit side effects (audit
	// log + best-effort onboarding checklist) for an employee inserted via
	// CreateEmployeeTx. The orchestrator (recruitment.HireApplication) calls
	// this ONLY after its own transaction has committed successfully.
	AfterHireCommit(ctx context.Context, orgID, actorID, employeeID string)
}

type serviceImpl struct {
	repo      Repository
	audit     audit.Service
	checklist ChecklistHook
}

func NewService(repo Repository, auditSvc audit.Service, checklist ChecklistHook) Service {
	return &serviceImpl{repo: repo, audit: auditSvc, checklist: checklist}
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

	defaultStatusID, err := s.repo.GetDefaultStatusID(ctx, orgID, EmployeeStatusCategoryActive)
	if err != nil {
		return nil, fmt.Errorf("employees: Create: fetch default status: %w", err)
	}

	e := &Employee{
		OrgID:          orgID,
		FirstName:      firstName,
		HireDate:       hireDate,
		EmploymentType: empType,
		StatusID:       defaultStatusID,
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

	// Auto-instantiate the org's default onboarding checklist, if any is
	// configured. Never fails employee creation: OnEmployeeCreated recovers
	// its own panics, and any error it returns is logged, not propagated.
	if s.checklist != nil {
		if err := s.checklist.OnEmployeeCreated(ctx, orgID, createdBy, e.ID); err != nil {
			slog.Error("employees: onboarding checklist hook failed", slog.Any("error", err), slog.String("employee_id", e.ID))
		}
	}

	return e, nil
}

// CreateEmployeeTx implements recruitment.EmployeeCreator's insert half. It
// deliberately does NOT call s.audit.Log or s.checklist — see the Service
// interface doc comment for why those must wait for AfterHireCommit.
func (s *serviceImpl) CreateEmployeeTx(ctx context.Context, tx pgx.Tx, orgID, createdBy string, req recruitment.HireEmployeeRequest) (string, string, error) {
	firstName := strings.TrimSpace(req.FirstName)
	if firstName == "" {
		return "", "", ErrFirstNameRequired
	}
	if len(firstName) > 100 {
		return "", "", ErrFirstNameTooLong
	}

	hireDateStr := strings.TrimSpace(req.HireDate)
	if hireDateStr == "" {
		return "", "", ErrHireDateRequired
	}
	hireDate, err := time.Parse(dateLayout, hireDateStr)
	if err != nil {
		return "", "", ErrInvalidHireDate
	}

	empType := EmploymentTypeFullTime
	if req.EmploymentType != nil && strings.TrimSpace(*req.EmploymentType) != "" {
		empType = EmploymentType(strings.TrimSpace(*req.EmploymentType))
		if !empType.IsValid() {
			return "", "", ErrInvalidEmploymentType
		}
	}

	// GetDefaultStatusID is a stable reference-data read (org statuses
	// change rarely), so it goes through the pool rather than needing its
	// own Tx variant — matching how Create reads it before ever touching
	// row-mutating SQL.
	defaultStatusID, err := s.repo.GetDefaultStatusID(ctx, orgID, EmployeeStatusCategoryActive)
	if err != nil {
		return "", "", fmt.Errorf("employees: CreateEmployeeTx: fetch default status: %w", err)
	}

	sourceCandidateID := strings.TrimSpace(req.SourceCandidateID)

	e := &Employee{
		OrgID:          orgID,
		FirstName:      firstName,
		HireDate:       hireDate,
		EmploymentType: empType,
		StatusID:       defaultStatusID,
		CreatedBy:      createdBy,
	}
	e.LastName = nilIfEmpty(&req.LastName)
	e.Email = req.Email
	e.DepartmentID = nilIfEmpty(req.DepartmentID)
	e.PositionID = nilIfEmpty(req.PositionID)
	e.ManagerID = nilIfEmpty(req.ManagerID)
	if sourceCandidateID != "" {
		e.SourceCandidateID = &sourceCandidateID
	}

	if err := s.repo.CreateTx(ctx, tx, e); err != nil {
		return "", "", fmt.Errorf("employees: CreateEmployeeTx: %w", err)
	}

	return e.ID, e.PublicID, nil
}

// AfterHireCommit runs Create's normal post-commit side effects for an
// employee inserted via CreateEmployeeTx. See the Service interface doc
// comment: the caller must invoke this only after its own transaction has
// committed successfully.
func (s *serviceImpl) AfterHireCommit(ctx context.Context, orgID, actorID, employeeID string) {
	s.audit.Log(ctx, audit.EventHRMEmployeeCreated, actorID, orgID, "", "", map[string]string{
		"employee_id": employeeID, "source": "hire_conversion",
	})

	if s.checklist != nil {
		if err := s.checklist.OnEmployeeCreated(ctx, orgID, actorID, employeeID); err != nil {
			slog.Error("employees: onboarding checklist hook failed (hire conversion)", slog.Any("error", err), slog.String("employee_id", employeeID))
		}
	}
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
	if req.StatusID != nil {
		st := strings.TrimSpace(*req.StatusID)
		if st == "" {
			return nil, errors.New("status_id cannot be empty")
		}
		e.StatusID = st
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
	termStatusID, err := s.repo.GetDefaultStatusID(ctx, orgID, EmployeeStatusCategoryTerminated)
	if err != nil {
		return nil, fmt.Errorf("employees: Terminate: fetch terminated status: %w", err)
	}
	if e.StatusID == termStatusID {
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

	e.StatusID = termStatusID
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

func (s *serviceImpl) ListStatuses(ctx context.Context, orgID string) ([]*EmployeeStatusModel, error) {
	list, err := s.repo.ListStatuses(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("employees: ListStatuses: %w", err)
	}
	if list == nil {
		list = []*EmployeeStatusModel{}
	}
	return list, nil
}

func (s *serviceImpl) CreateStatus(ctx context.Context, orgID string, req CreateEmployeeStatusRequest) (*EmployeeStatusModel, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrStatusNameRequired
	}
	if !req.Category.IsValid() {
		return nil, ErrInvalidStatusCategory
	}
	color := strings.TrimSpace(req.Color)
	if color == "" {
		return nil, ErrStatusColorRequired
	}

	model := &EmployeeStatusModel{
		OrgID:    orgID,
		Name:     name,
		Category: req.Category,
		Color:    color,
	}

	if err := s.repo.CreateStatus(ctx, model); err != nil {
		return nil, fmt.Errorf("employees: CreateStatus: %w", err)
	}
	return model, nil
}

func (s *serviceImpl) UpdateStatus(ctx context.Context, orgID, statusID string, req UpdateEmployeeStatusRequest) (*EmployeeStatusModel, error) {
	// First, fetch existing statuses to find the one we are updating.
	list, err := s.repo.ListStatuses(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("employees: UpdateStatus: fetch: %w", err)
	}

	var existing *EmployeeStatusModel
	for _, st := range list {
		if st.ID == statusID {
			existing = st
			break
		}
	}
	if existing == nil {
		return nil, errors.New("status not found")
	}

	// Protect default seeded statuses from being renamed/recategorised if we want
	// But let's assume they can change their color. Let's just prevent changing their category.
	isDefault := existing.Name == "Active" || existing.Name == "Inactive" || existing.Name == "On leave" || existing.Name == "Terminated" || existing.Name == "Resigned"

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrStatusNameRequired
		}
		if isDefault && name != existing.Name {
			return nil, ErrCannotModifyDefaultStatus
		}
		existing.Name = name
	}
	if req.Category != nil {
		if !req.Category.IsValid() {
			return nil, ErrInvalidStatusCategory
		}
		if isDefault && *req.Category != existing.Category {
			return nil, ErrCannotModifyDefaultStatus
		}
		existing.Category = *req.Category
	}
	if req.Color != nil {
		color := strings.TrimSpace(*req.Color)
		if color == "" {
			return nil, ErrStatusColorRequired
		}
		existing.Color = color
	}

	if err := s.repo.UpdateStatus(ctx, existing); err != nil {
		return nil, fmt.Errorf("employees: UpdateStatus: %w", err)
	}
	return existing, nil
}

func (s *serviceImpl) DeleteStatus(ctx context.Context, orgID, statusID string) error {
	list, err := s.repo.ListStatuses(ctx, orgID)
	if err != nil {
		return fmt.Errorf("employees: DeleteStatus: fetch: %w", err)
	}

	var existing *EmployeeStatusModel
	for _, st := range list {
		if st.ID == statusID {
			existing = st
			break
		}
	}
	if existing == nil {
		return errors.New("status not found")
	}

	isDefault := existing.Name == "Active" || existing.Name == "Inactive" || existing.Name == "On leave" || existing.Name == "Terminated" || existing.Name == "Resigned"
	if isDefault {
		return ErrCannotModifyDefaultStatus
	}

	if err := s.repo.DeleteStatus(ctx, orgID, statusID); err != nil {
		return fmt.Errorf("employees: DeleteStatus: %w", err)
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
