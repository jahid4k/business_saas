package employees_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/hrm/employees"
)

type mockAudit struct{}

func (m *mockAudit) Log(ctx context.Context, event audit.EventType, userID, businessID, ip, ua string, metadata any) {
}

type stubRepo struct {
	employees      map[string]*employees.Employee
	errCount       error
	errFindAll     error
	errFindByRef   error
	errCreate      error
	errUpdate      error
	errDelete      error
	errExists      error
	existsOverride bool
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		employees: make(map[string]*employees.Employee),
	}
}

func (s *stubRepo) FindAll(ctx context.Context, orgID string, filter employees.ListFilter) ([]*employees.Employee, error) {
	if s.errFindAll != nil {
		return nil, s.errFindAll
	}
	var res []*employees.Employee
	for _, e := range s.employees {
		if e.OrgID == orgID {
			res = append(res, e)
		}
	}
	return res, nil
}

func (s *stubRepo) Count(ctx context.Context, orgID string, filter employees.ListFilter) (int, error) {
	if s.errCount != nil {
		return 0, s.errCount
	}
	count := 0
	for _, e := range s.employees {
		if e.OrgID == orgID {
			count++
		}
	}
	return count, nil
}

func (s *stubRepo) FindByRef(ctx context.Context, orgID, ref string) (*employees.Employee, error) {
	if s.errFindByRef != nil {
		return nil, s.errFindByRef
	}
	for _, e := range s.employees {
		if e.OrgID == orgID && (e.ID == ref || e.PublicID == ref) {
			return e, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) Create(ctx context.Context, e *employees.Employee) error {
	if s.errCreate != nil {
		return s.errCreate
	}
	e.ID = "id_" + e.FirstName
	s.employees[e.ID] = e
	return nil
}

func (s *stubRepo) Update(ctx context.Context, e *employees.Employee) error {
	if s.errUpdate != nil {
		return s.errUpdate
	}
	s.employees[e.ID] = e
	return nil
}

func (s *stubRepo) Delete(ctx context.Context, orgID, ref string) error {
	if s.errDelete != nil {
		return s.errDelete
	}
	for id, e := range s.employees {
		if e.OrgID == orgID && (e.ID == ref || e.PublicID == ref) {
			delete(s.employees, id)
			return nil
		}
	}
	return employees.ErrEmployeeNotFound
}

func (s *stubRepo) ExistsByEmployeeNumber(ctx context.Context, orgID, number, excludeID string) (bool, error) {
	if s.errExists != nil {
		return false, s.errExists
	}
	if s.existsOverride {
		return true, nil
	}
	for _, e := range s.employees {
		if e.OrgID == orgID && e.EmployeeNumber != nil && *e.EmployeeNumber == number && e.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubRepo) GetDefaultStatusID(ctx context.Context, orgID string, category employees.EmployeeStatusCategory) (string, error) {
	return "status_" + string(category), nil
}

func (s *stubRepo) ListStatuses(ctx context.Context, orgID string) ([]*employees.EmployeeStatusModel, error) {
	return nil, nil
}

func (s *stubRepo) CreateStatus(ctx context.Context, m *employees.EmployeeStatusModel) error {
	m.ID = "new_status"
	return nil
}

func (s *stubRepo) UpdateStatus(ctx context.Context, m *employees.EmployeeStatusModel) error {
	return nil
}

func (s *stubRepo) DeleteStatus(ctx context.Context, orgID, statusID string) error {
	return nil
}

func ptrStr(s string) *string {
	return &s
}

func TestEmployeesService_Create(t *testing.T) {
	repo := newStubRepo()
	svc := employees.NewService(repo, &mockAudit{})
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := employees.CreateEmployeeRequest{
			FirstName: "John",
			HireDate:  "2023-01-01",
		}
		emp, err := svc.Create(ctx, "org1", "user1", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if emp.FirstName != "John" {
			t.Errorf("expected John, got %s", emp.FirstName)
		}
	})

	t.Run("validation_error", func(t *testing.T) {
		req := employees.CreateEmployeeRequest{
			FirstName: "",
		}
		_, err := svc.Create(ctx, "org1", "user1", req)
		if err != employees.ErrFirstNameRequired {
			t.Errorf("expected ErrFirstNameRequired, got %v", err)
		}
	})
	
	t.Run("conflict_emp_number", func(t *testing.T) {
		repo.employees["id_exist"] = &employees.Employee{
			ID: "id_exist",
			OrgID: "org1",
			EmployeeNumber: ptrStr("EMP001"),
		}
		req := employees.CreateEmployeeRequest{
			FirstName: "Jane",
			HireDate:  "2023-01-01",
			EmployeeNumber: ptrStr("EMP001"),
		}
		_, err := svc.Create(ctx, "org1", "user1", req)
		if err != employees.ErrEmployeeNumberConflict {
			t.Errorf("expected ErrEmployeeNumberConflict, got %v", err)
		}
	})
}

func TestEmployeesService_Get(t *testing.T) {
	repo := newStubRepo()
	svc := employees.NewService(repo, &mockAudit{})
	ctx := context.Background()

	repo.employees["id1"] = &employees.Employee{
		ID: "id1",
		OrgID: "org1",
		FirstName: "John",
	}
	
	t.Run("success", func(t *testing.T) {
		emp, err := svc.Get(ctx, "org1", "id1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if emp.FirstName != "John" {
			t.Errorf("expected John, got %s", emp.FirstName)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := svc.Get(ctx, "org1", "id2")
		if err != employees.ErrEmployeeNotFound {
			t.Errorf("expected ErrEmployeeNotFound, got %v", err)
		}
	})

	t.Run("wrong_org", func(t *testing.T) {
		_, err := svc.Get(ctx, "org2", "id1")
		if err != employees.ErrEmployeeNotFound {
			t.Errorf("expected ErrEmployeeNotFound, got %v", err)
		}
	})
}

func TestEmployeesService_Update(t *testing.T) {
	repo := newStubRepo()
	svc := employees.NewService(repo, &mockAudit{})
	ctx := context.Background()

	repo.employees["id1"] = &employees.Employee{
		ID: "id1",
		OrgID: "org1",
		FirstName: "John",
	}

	t.Run("success", func(t *testing.T) {
		req := employees.UpdateEmployeeRequest{
			FirstName: ptrStr("John updated"),
		}
		emp, err := svc.Update(ctx, "org1", "id1", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if emp.FirstName != "John updated" {
			t.Errorf("expected updated name, got %s", emp.FirstName)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		req := employees.UpdateEmployeeRequest{
			FirstName: ptrStr("John updated"),
		}
		_, err := svc.Update(ctx, "org1", "id2", req)
		if err != employees.ErrEmployeeNotFound {
			t.Errorf("expected ErrEmployeeNotFound, got %v", err)
		}
	})
}

func TestEmployeesService_Terminate(t *testing.T) {
	repo := newStubRepo()
	svc := employees.NewService(repo, &mockAudit{})
	ctx := context.Background()

	repo.employees["id1"] = &employees.Employee{
		ID: "id1",
		OrgID: "org1",
		FirstName: "John",
		HireDate: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		StatusID: "status_active",
	}

	t.Run("success", func(t *testing.T) {
		req := employees.TerminateEmployeeRequest{
			TerminationDate: "2023-01-01",
		}
		emp, err := svc.Terminate(ctx, "org1", "id1", "admin", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if emp.StatusID != "status_terminated" {
			t.Errorf("expected terminated, got %s", emp.StatusID)
		}
	})
	
	t.Run("already_terminated", func(t *testing.T) {
		req := employees.TerminateEmployeeRequest{
			TerminationDate: "2023-01-01",
		}
		_, err := svc.Terminate(ctx, "org1", "id1", "admin", req)
		if err != employees.ErrAlreadyTerminated {
			t.Errorf("expected ErrAlreadyTerminated, got %v", err)
		}
	})
}

func TestEmployeesService_List(t *testing.T) {
	repo := newStubRepo()
	svc := employees.NewService(repo, &mockAudit{})
	ctx := context.Background()

	repo.employees["id1"] = &employees.Employee{ID: "id1", OrgID: "org1"}
	repo.employees["id2"] = &employees.Employee{ID: "id2", OrgID: "org1"}
	repo.employees["id3"] = &employees.Employee{ID: "id3", OrgID: "org2"}

	t.Run("success", func(t *testing.T) {
		res, err := svc.List(ctx, "org1", employees.ListFilter{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 2 {
			t.Errorf("expected 2, got %d", res.Total)
		}
		if len(res.Employees) != 2 {
			t.Errorf("expected 2, got %d", len(res.Employees))
		}
	})
}

func TestEmployeesService_Delete(t *testing.T) {
	repo := newStubRepo()
	svc := employees.NewService(repo, &mockAudit{})
	ctx := context.Background()

	repo.employees["id1"] = &employees.Employee{ID: "id1", OrgID: "org1"}

	t.Run("success", func(t *testing.T) {
		err := svc.Delete(ctx, "org1", "id1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := repo.employees["id1"]; ok {
			t.Errorf("expected employee to be deleted")
		}
	})

	t.Run("not_found", func(t *testing.T) {
		err := svc.Delete(ctx, "org1", "id2")
		if err != employees.ErrEmployeeNotFound {
			t.Errorf("expected ErrEmployeeNotFound, got %v", err)
		}
	})
}
