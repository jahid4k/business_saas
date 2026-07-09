// backend/internal/hrm/contracts/service.go
package contracts

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

type Service interface {
	List(ctx context.Context, orgID, employeeID string) (*ContractListResponse, error)
	GetActive(ctx context.Context, orgID, employeeID string) (*EmployeeContract, error)
	Get(ctx context.Context, orgID, employeeID, ref string) (*EmployeeContract, error)
	Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateContractRequest) (*EmployeeContract, error)
	Update(ctx context.Context, orgID, employeeID, ref string, req UpdateContractRequest) (*EmployeeContract, error)
	Deactivate(ctx context.Context, orgID, employeeID, ref string) error
}

type serviceImpl struct{ repo Repository }
func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

func (s *serviceImpl) List(ctx context.Context, orgID, employeeID string) (*ContractListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, employeeID)
	if err != nil { return nil, fmt.Errorf("contracts: List: %w", err) }
	if list == nil { list = []*EmployeeContract{} }
	return &ContractListResponse{Contracts: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetActive(ctx context.Context, orgID, employeeID string) (*EmployeeContract, error) {
	c, err := s.repo.FindActive(ctx, orgID, employeeID)
	if err != nil { return nil, fmt.Errorf("contracts: GetActive: %w", err) }
	if c == nil { return nil, ErrContractNotFound }
	return c, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, employeeID, ref string) (*EmployeeContract, error) {
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("contracts: Get: %w", err) }
	if c == nil { return nil, ErrContractNotFound }
	return c, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateContractRequest) (*EmployeeContract, error) {
	if !req.ContractType.IsValid() { return nil, ErrInvalidContractType }
	if strings.TrimSpace(req.StartDate) == "" { return nil, ErrStartDateRequired }
	if _, err := time.Parse(dateLayout, req.StartDate); err != nil { return nil, ErrInvalidStartDate }

	// Check for existing active contract — deactivate first
	existing, err := s.repo.FindActive(ctx, orgID, employeeID)
	if err != nil { return nil, fmt.Errorf("contracts: Create: check active: %w", err) }
	if existing != nil { return nil, ErrActiveContractExists }

	noticeDays := 30
	if req.NoticePeriodDays != nil { noticeDays = *req.NoticePeriodDays }

	c := &EmployeeContract{
		OrgID: orgID, EmployeeID: employeeID, ContractType: req.ContractType,
		StartDate: req.StartDate, EndDate: req.EndDate, ProbationEndDate: req.ProbationEndDate,
		NoticePeriodDays: noticeDays, SalaryStructureID: req.SalaryStructureID,
		WorkHoursPerWeek: req.WorkHoursPerWeek, DocumentID: req.DocumentID,
		IsActive: true, Notes: req.Notes, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, c); err != nil { return nil, fmt.Errorf("contracts: Create: %w", err) }
	return c, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, employeeID, ref string, req UpdateContractRequest) (*EmployeeContract, error) {
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("contracts: Update: %w", err) }
	if c == nil { return nil, ErrContractNotFound }
	if req.EndDate != nil { c.EndDate = req.EndDate }
	if req.ProbationEndDate != nil { c.ProbationEndDate = req.ProbationEndDate }
	if req.NoticePeriodDays != nil { c.NoticePeriodDays = *req.NoticePeriodDays }
	if req.SalaryStructureID != nil { c.SalaryStructureID = req.SalaryStructureID }
	if req.WorkHoursPerWeek != nil { c.WorkHoursPerWeek = req.WorkHoursPerWeek }
	if req.DocumentID != nil { c.DocumentID = req.DocumentID }
	if req.Notes != nil { c.Notes = req.Notes }
	if err := s.repo.Update(ctx, c); err != nil { return nil, fmt.Errorf("contracts: Update: %w", err) }
	return c, nil
}

func (s *serviceImpl) Deactivate(ctx context.Context, orgID, employeeID, ref string) error {
	return s.repo.Deactivate(ctx, orgID, employeeID, ref)
}
