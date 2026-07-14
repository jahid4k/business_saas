package salary_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/salary"
)

type stubRepo struct {
	components map[string]*salary.SalaryComponent
	structures map[string]*salary.SalaryStructure
	records    map[string]*salary.EmployeeSalaryRecord
	structComp map[string]map[string]bool // structureID -> componentID -> true
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		components: make(map[string]*salary.SalaryComponent),
		structures: make(map[string]*salary.SalaryStructure),
		records:    make(map[string]*salary.EmployeeSalaryRecord),
		structComp: make(map[string]map[string]bool),
	}
}

// Components
func (s *stubRepo) FindAllComponents(ctx context.Context, orgID string, activeOnly bool) ([]*salary.SalaryComponent, error) {
	var list []*salary.SalaryComponent
	for _, c := range s.components {
		if c.OrgID == orgID {
			if !activeOnly || c.IsActive {
				list = append(list, c)
			}
		}
	}
	return list, nil
}
func (s *stubRepo) CountComponents(ctx context.Context, orgID string, activeOnly bool) (int, error) {
	l, _ := s.FindAllComponents(ctx, orgID, activeOnly)
	return len(l), nil
}
func (s *stubRepo) FindComponentByRef(ctx context.Context, orgID, ref string) (*salary.SalaryComponent, error) {
	for _, c := range s.components {
		if c.OrgID == orgID && (c.ID == ref || c.PublicID == ref) {
			return c, nil
		}
	}
	return nil, nil
}
func (s *stubRepo) CreateComponent(ctx context.Context, c *salary.SalaryComponent, slabJSON []byte) error {
	c.ID = "comp_" + time.Now().Format("20060102150405.000")
	s.components[c.ID] = c
	return nil
}
func (s *stubRepo) UpdateComponent(ctx context.Context, c *salary.SalaryComponent, slabJSON []byte) error {
	s.components[c.ID] = c
	return nil
}
func (s *stubRepo) DeleteComponent(ctx context.Context, orgID, ref string) error {
	for id, c := range s.components {
		if c.OrgID == orgID && (c.ID == ref || c.PublicID == ref) {
			delete(s.components, id)
			return nil
		}
	}
	return salary.ErrComponentNotFound
}
func (s *stubRepo) ComponentNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	for _, c := range s.components {
		if c.OrgID == orgID && c.Name == name && c.IsActive && c.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

// Structures
func (s *stubRepo) FindAllStructures(ctx context.Context, orgID string, activeOnly bool) ([]*salary.SalaryStructure, error) {
	var list []*salary.SalaryStructure
	for _, st := range s.structures {
		if st.OrgID == orgID {
			if !activeOnly || st.IsActive {
				list = append(list, st)
			}
		}
	}
	return list, nil
}
func (s *stubRepo) CountStructures(ctx context.Context, orgID string, activeOnly bool) (int, error) {
	l, _ := s.FindAllStructures(ctx, orgID, activeOnly)
	return len(l), nil
}
func (s *stubRepo) FindStructureByRef(ctx context.Context, orgID, ref string) (*salary.SalaryStructure, error) {
	for _, st := range s.structures {
		if st.OrgID == orgID && (st.ID == ref || st.PublicID == ref) {
			return st, nil
		}
	}
	return nil, nil
}
func (s *stubRepo) CreateStructure(ctx context.Context, st *salary.SalaryStructure) error {
	st.ID = "struct_" + time.Now().Format("20060102150405.000")
	s.structures[st.ID] = st
	return nil
}
func (s *stubRepo) UpdateStructure(ctx context.Context, st *salary.SalaryStructure) error {
	s.structures[st.ID] = st
	return nil
}
func (s *stubRepo) DeleteStructure(ctx context.Context, orgID, ref string) error {
	for id, st := range s.structures {
		if st.OrgID == orgID && (st.ID == ref || st.PublicID == ref) {
			delete(s.structures, id)
			return nil
		}
	}
	return salary.ErrStructureNotFound
}
func (s *stubRepo) StructureNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	for _, st := range s.structures {
		if st.OrgID == orgID && st.Name == name && st.IsActive && st.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}
func (s *stubRepo) AddComponentToStructure(ctx context.Context, structureID, componentID string, overrideValue *float64, displayOrder int) error {
	if s.structComp[structureID] == nil {
		s.structComp[structureID] = make(map[string]bool)
	}
	if s.structComp[structureID][componentID] {
		return salary.ErrComponentInStructure
	}
	s.structComp[structureID][componentID] = true
	return nil
}
func (s *stubRepo) RemoveComponentFromStructure(ctx context.Context, structureID, componentID string) error {
	if s.structComp[structureID] != nil && s.structComp[structureID][componentID] {
		delete(s.structComp[structureID], componentID)
		return nil
	}
	return salary.ErrComponentNotInStructure
}
func (s *stubRepo) FindStructureComponents(ctx context.Context, structureID string) ([]*salary.StructureComponent, error) {
	var list []*salary.StructureComponent
	if cmap, ok := s.structComp[structureID]; ok {
		for cid := range cmap {
			sc := &salary.StructureComponent{ComponentID: cid, Component: s.components[cid]}
			list = append(list, sc)
		}
	}
	return list, nil
}

// Records
func (s *stubRepo) FindSalaryHistory(ctx context.Context, orgID, employeeID string) ([]*salary.EmployeeSalaryRecord, error) {
	var list []*salary.EmployeeSalaryRecord
	for _, r := range s.records {
		if r.OrgID == orgID && r.EmployeeID == employeeID {
			list = append(list, r)
		}
	}
	return list, nil
}
func (s *stubRepo) FindActiveSalaryRecord(ctx context.Context, orgID, employeeID string) (*salary.EmployeeSalaryRecord, error) {
	list, _ := s.FindSalaryHistory(ctx, orgID, employeeID)
	if len(list) > 0 {
		return list[0], nil
	}
	return nil, nil
}
func (s *stubRepo) CreateSalaryRecord(ctx context.Context, r *salary.EmployeeSalaryRecord) error {
	r.ID = "rec_" + time.Now().Format("20060102150405.000")
	s.records[r.ID] = r
	return nil
}

func TestSalaryService(t *testing.T) {
	repo := newStubRepo()
	svc := salary.NewService(repo)
	ctx := context.Background()

	orgID := "org_1"
	createdBy := "user_1"

	// Create Component
	req := salary.CreateComponentRequest{
		Name:          "Basic",
		ComponentType: salary.ComponentTypeEarning,
		CalcMethod:    salary.CalcMethodFixed,
	}
	comp, err := svc.CreateComponent(ctx, orgID, createdBy, req)
	if err != nil {
		t.Fatalf("CreateComponent failed: %v", err)
	}

	// Update Component
	newCompName := "Basic Pay"
	compReq := salary.UpdateComponentRequest{Name: &newCompName}
	_, err = svc.UpdateComponent(ctx, orgID, comp.ID, compReq)
	if err != nil {
		t.Fatalf("UpdateComponent failed: %v", err)
	}

	// Create Structure
	stReq := salary.CreateStructureRequest{
		Name: "Grade A",
	}
	st, err := svc.CreateStructure(ctx, orgID, createdBy, stReq)
	if err != nil {
		t.Fatalf("CreateStructure failed: %v", err)
	}

	// Add Component to Structure
	err = svc.AddComponentToStructure(ctx, orgID, st.ID, salary.AddComponentToStructureRequest{
		ComponentID: comp.ID,
	})
	if err != nil {
		t.Fatalf("AddComponentToStructure failed: %v", err)
	}

	// Assign Salary
	assignReq := salary.AssignSalaryRequest{
		StructureID:   &st.ID,
		BasicPay:      5000,
		EffectiveDate: "2024-01-01",
		ChangeReason:  salary.ChangeReasonJoining,
	}
	rec, err := svc.AssignSalary(ctx, orgID, "emp_1", createdBy, assignReq)
	if err != nil {
		t.Fatalf("AssignSalary failed: %v", err)
	}
	if rec.BasicPay != 5000 {
		t.Errorf("Expected basic pay 5000, got %f", rec.BasicPay)
	}

	// Remove Component
	err = svc.RemoveComponentFromStructure(ctx, orgID, st.ID, comp.ID)
	if err != nil {
		t.Fatalf("RemoveComponentFromStructure failed: %v", err)
	}

	// Delete Component
	err = svc.DeleteComponent(ctx, orgID, comp.ID)
	if err != nil {
		t.Fatalf("DeleteComponent failed: %v", err)
	}

	// Formula Test
	formReq := salary.TestFormulaRequest{
		Expression: "BASIC * 0.5",
		Variables:  map[string]float64{"BASIC": 5000},
	}
	res := svc.TestFormula(ctx, formReq)
	if !res.Valid || res.Result != 2500 {
		t.Errorf("Formula failed: %v", res)
	}
}
