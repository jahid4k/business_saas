// backend/internal/hrm/salary/service.go
package salary

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/expr-lang/expr"
)

// dateLayout for effective_date parsing.
const dateLayout = "2006-01-02"

// Service defines business logic for the HRM salary engine.
type Service interface {
	// Salary components
	ListComponents(ctx context.Context, orgID string, activeOnly bool) (*ComponentListResponse, error)
	GetComponent(ctx context.Context, orgID, ref string) (*SalaryComponent, error)
	CreateComponent(ctx context.Context, orgID, createdBy string, req CreateComponentRequest) (*SalaryComponent, error)
	UpdateComponent(ctx context.Context, orgID, ref string, req UpdateComponentRequest) (*SalaryComponent, error)
	DeleteComponent(ctx context.Context, orgID, ref string) error

	// Salary structures
	ListStructures(ctx context.Context, orgID string, activeOnly bool) (*StructureListResponse, error)
	GetStructure(ctx context.Context, orgID, ref string) (*SalaryStructure, error)
	CreateStructure(ctx context.Context, orgID, createdBy string, req CreateStructureRequest) (*SalaryStructure, error)
	UpdateStructure(ctx context.Context, orgID, ref string, req UpdateStructureRequest) (*SalaryStructure, error)
	DeleteStructure(ctx context.Context, orgID, ref string) error
	AddComponentToStructure(ctx context.Context, orgID, structRef string, req AddComponentToStructureRequest) error
	RemoveComponentFromStructure(ctx context.Context, orgID, structRef, compRef string) error

	// Employee salary records (append-only)
	GetSalaryHistory(ctx context.Context, orgID, employeeID string) (*SalaryHistoryResponse, error)
	GetActiveSalary(ctx context.Context, orgID, employeeID string) (*EmployeeSalaryRecord, error)
	AssignSalary(ctx context.Context, orgID, employeeID, createdBy string, req AssignSalaryRequest) (*EmployeeSalaryRecord, error)

	// Formula utilities
	TestFormula(ctx context.Context, req TestFormulaRequest) *TestFormulaResponse
}

type serviceImpl struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

// ─────────────────────────────────────────────────────────────
// Components
// ─────────────────────────────────────────────────────────────

func (s *serviceImpl) ListComponents(ctx context.Context, orgID string, activeOnly bool) (*ComponentListResponse, error) {
	list, err := s.repo.FindAllComponents(ctx, orgID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("salary: ListComponents: %w", err)
	}
	if list == nil {
		list = []*SalaryComponent{}
	}
	total, err := s.repo.CountComponents(ctx, orgID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("salary: ListComponents count: %w", err)
	}
	return &ComponentListResponse{Components: list, Total: total}, nil
}

func (s *serviceImpl) GetComponent(ctx context.Context, orgID, ref string) (*SalaryComponent, error) {
	c, err := s.repo.FindComponentByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("salary: GetComponent: %w", err)
	}
	if c == nil {
		return nil, ErrComponentNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateComponent(ctx context.Context, orgID, createdBy string, req CreateComponentRequest) (*SalaryComponent, error) {
	if err := validateComponentRequest(req.Name, req.ComponentType, req.CalcMethod, req.FormulaExpression, req.SlabConfig); err != nil {
		return nil, err
	}
	exists, err := s.repo.ComponentNameExists(ctx, orgID, req.Name, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrNameConflict
	}

	c := &SalaryComponent{
		OrgID:             orgID,
		Name:              strings.TrimSpace(req.Name),
		Description:       req.Description,
		ComponentType:     req.ComponentType,
		CalcMethod:        req.CalcMethod,
		FixedValue:        0,
		FormulaExpression: req.FormulaExpression,
		FormulaVariables:  req.FormulaVariables,
		SlabConfig:        req.SlabConfig,
		IsTaxable:         req.IsTaxable,
		DisplayOrder:      0,
		IsActive:          true,
		CreatedBy:         createdBy,
	}
	if req.FixedValue != nil {
		c.FixedValue = *req.FixedValue
	}
	if req.DisplayOrder != nil {
		c.DisplayOrder = *req.DisplayOrder
	}

	slabJSON, err := c.SlabConfigJSON()
	if err != nil {
		return nil, fmt.Errorf("salary: CreateComponent: marshal slab: %w", err)
	}
	if err := s.repo.CreateComponent(ctx, c, slabJSON); err != nil {
		return nil, fmt.Errorf("salary: CreateComponent: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) UpdateComponent(ctx context.Context, orgID, ref string, req UpdateComponentRequest) (*SalaryComponent, error) {
	c, err := s.repo.FindComponentByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("salary: UpdateComponent: %w", err)
	}
	if c == nil {
		return nil, ErrComponentNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		if len(name) > 150 {
			return nil, ErrNameTooLong
		}
		exists, _ := s.repo.ComponentNameExists(ctx, orgID, name, c.ID)
		if exists {
			return nil, ErrNameConflict
		}
		c.Name = name
	}
	if req.Description != nil {
		c.Description = req.Description
	}
	if req.ComponentType != nil {
		if !req.ComponentType.IsValid() {
			return nil, ErrInvalidComponentType
		}
		c.ComponentType = *req.ComponentType
	}
	if req.CalcMethod != nil {
		if !req.CalcMethod.IsValid() {
			return nil, ErrInvalidCalcMethod
		}
		c.CalcMethod = *req.CalcMethod
	}
	if req.FixedValue != nil {
		c.FixedValue = *req.FixedValue
	}
	if req.FormulaExpression != nil {
		c.FormulaExpression = req.FormulaExpression
	}
	if req.FormulaVariables != nil {
		c.FormulaVariables = req.FormulaVariables
	}
	if req.SlabConfig != nil {
		c.SlabConfig = req.SlabConfig
	}
	if req.IsTaxable != nil {
		c.IsTaxable = *req.IsTaxable
	}
	if req.DisplayOrder != nil {
		c.DisplayOrder = *req.DisplayOrder
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}

	// Re-validate formula/slab after update
	if c.CalcMethod == CalcMethodFormula && c.FormulaExpression != nil {
		if _, err := compileFormula(*c.FormulaExpression); err != nil {
			return nil, ErrInvalidFormula
		}
	}

	slabJSON, err := c.SlabConfigJSON()
	if err != nil {
		return nil, fmt.Errorf("salary: UpdateComponent: marshal slab: %w", err)
	}
	if err := s.repo.UpdateComponent(ctx, c, slabJSON); err != nil {
		return nil, fmt.Errorf("salary: UpdateComponent: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) DeleteComponent(ctx context.Context, orgID, ref string) error {
	return s.repo.DeleteComponent(ctx, orgID, ref)
}

// ─────────────────────────────────────────────────────────────
// Structures
// ─────────────────────────────────────────────────────────────

func (s *serviceImpl) ListStructures(ctx context.Context, orgID string, activeOnly bool) (*StructureListResponse, error) {
	list, err := s.repo.FindAllStructures(ctx, orgID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("salary: ListStructures: %w", err)
	}
	if list == nil {
		list = []*SalaryStructure{}
	}
	total, _ := s.repo.CountStructures(ctx, orgID, activeOnly)
	return &StructureListResponse{Structures: list, Total: total}, nil
}

func (s *serviceImpl) GetStructure(ctx context.Context, orgID, ref string) (*SalaryStructure, error) {
	st, err := s.repo.FindStructureByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("salary: GetStructure: %w", err)
	}
	if st == nil {
		return nil, ErrStructureNotFound
	}
	comps, err := s.repo.FindStructureComponents(ctx, st.ID)
	if err != nil {
		return nil, fmt.Errorf("salary: GetStructure components: %w", err)
	}
	st.Components = comps
	return st, nil
}

func (s *serviceImpl) CreateStructure(ctx context.Context, orgID, createdBy string, req CreateStructureRequest) (*SalaryStructure, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if len(name) > 150 {
		return nil, ErrNameTooLong
	}
	exists, _ := s.repo.StructureNameExists(ctx, orgID, name, "")
	if exists {
		return nil, ErrStructureNameConflict
	}

	st := &SalaryStructure{
		OrgID:       orgID,
		Name:        name,
		Description: req.Description,
		GradeLabel:  req.GradeLabel,
		IsActive:    true,
		CreatedBy:   createdBy,
	}
	if err := s.repo.CreateStructure(ctx, st); err != nil {
		return nil, fmt.Errorf("salary: CreateStructure: %w", err)
	}
	return st, nil
}

func (s *serviceImpl) UpdateStructure(ctx context.Context, orgID, ref string, req UpdateStructureRequest) (*SalaryStructure, error) {
	st, err := s.repo.FindStructureByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("salary: UpdateStructure: %w", err)
	}
	if st == nil {
		return nil, ErrStructureNotFound
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		exists, _ := s.repo.StructureNameExists(ctx, orgID, name, st.ID)
		if exists {
			return nil, ErrStructureNameConflict
		}
		st.Name = name
	}
	if req.Description != nil {
		st.Description = req.Description
	}
	if req.GradeLabel != nil {
		st.GradeLabel = req.GradeLabel
	}
	if req.IsActive != nil {
		st.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateStructure(ctx, st); err != nil {
		return nil, fmt.Errorf("salary: UpdateStructure: %w", err)
	}
	return st, nil
}

func (s *serviceImpl) DeleteStructure(ctx context.Context, orgID, ref string) error {
	return s.repo.DeleteStructure(ctx, orgID, ref)
}

func (s *serviceImpl) AddComponentToStructure(ctx context.Context, orgID, structRef string, req AddComponentToStructureRequest) error {
	st, err := s.repo.FindStructureByRef(ctx, orgID, structRef)
	if err != nil || st == nil {
		return ErrStructureNotFound
	}
	comp, err := s.repo.FindComponentByRef(ctx, orgID, req.ComponentID)
	if err != nil || comp == nil {
		return ErrComponentNotFound
	}
	order := 0
	if req.DisplayOrder != nil {
		order = *req.DisplayOrder
	}
	return s.repo.AddComponentToStructure(ctx, st.ID, comp.ID, req.OverrideValue, order)
}

func (s *serviceImpl) RemoveComponentFromStructure(ctx context.Context, orgID, structRef, compRef string) error {
	st, err := s.repo.FindStructureByRef(ctx, orgID, structRef)
	if err != nil || st == nil {
		return ErrStructureNotFound
	}
	comp, err := s.repo.FindComponentByRef(ctx, orgID, compRef)
	if err != nil || comp == nil {
		return ErrComponentNotFound
	}
	return s.repo.RemoveComponentFromStructure(ctx, st.ID, comp.ID)
}

// ─────────────────────────────────────────────────────────────
// Employee salary records
// ─────────────────────────────────────────────────────────────

func (s *serviceImpl) GetSalaryHistory(ctx context.Context, orgID, employeeID string) (*SalaryHistoryResponse, error) {
	list, err := s.repo.FindSalaryHistory(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("salary: GetSalaryHistory: %w", err)
	}
	if list == nil {
		list = []*EmployeeSalaryRecord{}
	}
	return &SalaryHistoryResponse{Records: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetActiveSalary(ctx context.Context, orgID, employeeID string) (*EmployeeSalaryRecord, error) {
	rec, err := s.repo.FindActiveSalaryRecord(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("salary: GetActiveSalary: %w", err)
	}
	if rec == nil {
		return nil, ErrSalaryRecordNotFound
	}
	return rec, nil
}

func (s *serviceImpl) AssignSalary(ctx context.Context, orgID, employeeID, createdBy string, req AssignSalaryRequest) (*EmployeeSalaryRecord, error) {
	if req.BasicPay < 0 {
		return nil, ErrBasicPayRequired
	}
	if req.EffectiveDate == "" {
		return nil, ErrEffectiveDateRequired
	}
	if _, err := time.Parse(dateLayout, req.EffectiveDate); err != nil {
		return nil, ErrInvalidEffectiveDate
	}
	if !req.ChangeReason.IsValid() {
		return nil, ErrInvalidChangeReason
	}
	if req.StructureID != nil {
		// Verify structure belongs to org
		st, err := s.repo.FindStructureByRef(ctx, orgID, *req.StructureID)
		if err != nil || st == nil {
			return nil, ErrStructureNotFound
		}
	}

	rec := &EmployeeSalaryRecord{
		OrgID:         orgID,
		EmployeeID:    employeeID,
		StructureID:   req.StructureID,
		BasicPay:      req.BasicPay,
		EffectiveDate: req.EffectiveDate,
		ChangeReason:  req.ChangeReason,
		ChangeNotes:   req.ChangeNotes,
		CreatedBy:     createdBy,
	}
	if err := s.repo.CreateSalaryRecord(ctx, rec); err != nil {
		return nil, fmt.Errorf("salary: AssignSalary: %w", err)
	}
	return rec, nil
}

// ─────────────────────────────────────────────────────────────
// Formula utilities
// ─────────────────────────────────────────────────────────────

// TestFormula evaluates a formula expression with the provided test variables.
// Uses expr-lang/expr sandboxed evaluator — no arbitrary code execution.
func (s *serviceImpl) TestFormula(_ context.Context, req TestFormulaRequest) *TestFormulaResponse {
	env := make(map[string]any, len(req.Variables))
	for k, v := range req.Variables {
		env[k] = v
	}
	result, err := evaluateFormula(req.Expression, env)
	if err != nil {
		msg := err.Error()
		return &TestFormulaResponse{Valid: false, Error: &msg}
	}
	return &TestFormulaResponse{Valid: true, Result: math.Round(result*100) / 100}
}

// ─────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────

// compileFormula compiles an expression string for syntax validation.
func compileFormula(expression string) (any, error) {
	env := map[string]any{
		"BASIC":          0.0,
		"GROSS":          0.0,
		"WORKING_DAYS":   0.0,
		"ACTUAL_DAYS":    0.0,
		"ABSENT_DAYS":    0.0,
		"OVERTIME_HOURS": 0.0,
		"LATE_MINUTES":   0.0,
		"TENURE_YEARS":   0.0,
		"TENURE_MONTHS":  0.0,
		"PERIOD_DAYS":    0.0,
	}
	return expr.Compile(expression, expr.Env(env), expr.AsFloat64())
}

// evaluateFormula runs a formula expression with the provided environment.
func evaluateFormula(expression string, env map[string]any) (float64, error) {
	program, err := expr.Compile(expression, expr.Env(env), expr.AsFloat64())
	if err != nil {
		return 0, fmt.Errorf("invalid formula: %w", err)
	}
	result, err := expr.Run(program, env)
	if err != nil {
		return 0, fmt.Errorf("formula evaluation failed: %w", err)
	}
	f, ok := result.(float64)
	if !ok {
		return 0, fmt.Errorf("formula must return a number")
	}
	return f, nil
}

// validateComponentRequest validates fields common to create and used in create path.
func validateComponentRequest(name string, cType ComponentType, method CalcMethod, formula *string, slab *SlabConfig) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameRequired
	}
	if len(name) > 150 {
		return ErrNameTooLong
	}
	if !cType.IsValid() {
		return ErrInvalidComponentType
	}
	if !method.IsValid() {
		return ErrInvalidCalcMethod
	}
	if method == CalcMethodFormula {
		if formula == nil || strings.TrimSpace(*formula) == "" {
			return ErrFormulaRequired
		}
		if _, err := compileFormula(*formula); err != nil {
			return ErrInvalidFormula
		}
	}
	if method == CalcMethodSlab {
		if slab == nil || len(slab.Slabs) == 0 {
			return ErrSlabRequired
		}
		// Validate slab ordering: exactly one null up_to at the end
		for i, sl := range slab.Slabs {
			if sl.UpTo == nil && i != len(slab.Slabs)-1 {
				return ErrInvalidSlab
			}
		}
		if slab.Slabs[len(slab.Slabs)-1].UpTo != nil {
			return ErrInvalidSlab
		}
	}
	return nil
}
