package departments_test

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/departments"
)

type stubRepo struct {
	deps           map[string]*departments.Department
	errFindAll     error
	errCount       error
	errFindByRef   error
	errCreate      error
	errUpdate      error
	errDelete      error
	errExists      error
	existsOverride bool
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		deps: make(map[string]*departments.Department),
	}
}

func (s *stubRepo) FindAll(ctx context.Context, orgID string, activeOnly bool) ([]*departments.Department, error) {
	if s.errFindAll != nil {
		return nil, s.errFindAll
	}
	var res []*departments.Department
	for _, d := range s.deps {
		if d.OrgID == orgID {
			if activeOnly && !d.IsActive {
				continue
			}
			res = append(res, d)
		}
	}
	return res, nil
}

func (s *stubRepo) Count(ctx context.Context, orgID string, activeOnly bool) (int, error) {
	if s.errCount != nil {
		return 0, s.errCount
	}
	count := 0
	for _, d := range s.deps {
		if d.OrgID == orgID {
			if activeOnly && !d.IsActive {
				continue
			}
			count++
		}
	}
	return count, nil
}

func (s *stubRepo) FindByRef(ctx context.Context, orgID, ref string) (*departments.Department, error) {
	if s.errFindByRef != nil {
		return nil, s.errFindByRef
	}
	for _, d := range s.deps {
		if d.OrgID == orgID && (d.ID == ref || d.PublicID == ref) {
			return d, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) Create(ctx context.Context, d *departments.Department) error {
	if s.errCreate != nil {
		return s.errCreate
	}
	d.ID = "id_" + d.Name
	s.deps[d.ID] = d
	return nil
}

func (s *stubRepo) Update(ctx context.Context, d *departments.Department) error {
	if s.errUpdate != nil {
		return s.errUpdate
	}
	s.deps[d.ID] = d
	return nil
}

func (s *stubRepo) Delete(ctx context.Context, orgID, ref string) error {
	if s.errDelete != nil {
		return s.errDelete
	}
	for id, d := range s.deps {
		if d.OrgID == orgID && (d.ID == ref || d.PublicID == ref) {
			delete(s.deps, id)
			return nil
		}
	}
	return departments.ErrDepartmentNotFound
}

func (s *stubRepo) ExistsByName(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	if s.errExists != nil {
		return false, s.errExists
	}
	if s.existsOverride {
		return true, nil
	}
	for _, d := range s.deps {
		if d.OrgID == orgID && d.Name == name && d.ID != excludeID && d.IsActive {
			return true, nil
		}
	}
	return false, nil
}

func ptrStr(s string) *string {
	return &s
}

func TestDepartmentsService_Create(t *testing.T) {
	repo := newStubRepo()
	svc := departments.NewService(repo)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := departments.CreateDepartmentRequest{
			Name: "Engineering",
		}
		d, err := svc.Create(ctx, "org1", "user1", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name != "Engineering" {
			t.Errorf("expected Engineering, got %s", d.Name)
		}
	})

	t.Run("validation_error", func(t *testing.T) {
		req := departments.CreateDepartmentRequest{
			Name: "",
		}
		_, err := svc.Create(ctx, "org1", "user1", req)
		if err != departments.ErrNameRequired {
			t.Errorf("expected ErrNameRequired, got %v", err)
		}
	})

	t.Run("conflict_name", func(t *testing.T) {
		repo.deps["id_conflict"] = &departments.Department{
			ID: "id_conflict",
			OrgID: "org1",
			Name: "HR",
			IsActive: true,
		}
		req := departments.CreateDepartmentRequest{
			Name: "HR",
		}
		_, err := svc.Create(ctx, "org1", "user1", req)
		if err != departments.ErrNameConflict {
			t.Errorf("expected ErrNameConflict, got %v", err)
		}
	})
}

func TestDepartmentsService_Get(t *testing.T) {
	repo := newStubRepo()
	svc := departments.NewService(repo)
	ctx := context.Background()

	repo.deps["id1"] = &departments.Department{
		ID: "id1",
		OrgID: "org1",
		Name: "HR",
	}

	t.Run("success", func(t *testing.T) {
		d, err := svc.Get(ctx, "org1", "id1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name != "HR" {
			t.Errorf("expected HR, got %s", d.Name)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := svc.Get(ctx, "org1", "id2")
		if err != departments.ErrDepartmentNotFound {
			t.Errorf("expected ErrDepartmentNotFound, got %v", err)
		}
	})
}

func TestDepartmentsService_Update(t *testing.T) {
	repo := newStubRepo()
	svc := departments.NewService(repo)
	ctx := context.Background()

	repo.deps["id1"] = &departments.Department{
		ID: "id1",
		OrgID: "org1",
		Name: "HR",
	}

	t.Run("success", func(t *testing.T) {
		req := departments.UpdateDepartmentRequest{
			Name: ptrStr("HR Updated"),
		}
		d, err := svc.Update(ctx, "org1", "id1", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name != "HR Updated" {
			t.Errorf("expected HR Updated, got %s", d.Name)
		}
	})

	t.Run("circular_parent", func(t *testing.T) {
		req := departments.UpdateDepartmentRequest{
			ParentDepartmentID: ptrStr("id1"),
		}
		_, err := svc.Update(ctx, "org1", "id1", req)
		if err != departments.ErrCircularParent {
			t.Errorf("expected ErrCircularParent, got %v", err)
		}
	})
}

func TestDepartmentsService_List(t *testing.T) {
	repo := newStubRepo()
	svc := departments.NewService(repo)
	ctx := context.Background()

	repo.deps["id1"] = &departments.Department{ID: "id1", OrgID: "org1", IsActive: true}
	repo.deps["id2"] = &departments.Department{ID: "id2", OrgID: "org1", IsActive: false}

	t.Run("success_all", func(t *testing.T) {
		res, err := svc.List(ctx, "org1", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 2 {
			t.Errorf("expected 2, got %d", res.Total)
		}
	})

	t.Run("success_active", func(t *testing.T) {
		res, err := svc.List(ctx, "org1", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("expected 1, got %d", res.Total)
		}
	})
}

func TestDepartmentsService_Delete(t *testing.T) {
	repo := newStubRepo()
	svc := departments.NewService(repo)
	ctx := context.Background()

	repo.deps["id1"] = &departments.Department{ID: "id1", OrgID: "org1"}

	t.Run("success", func(t *testing.T) {
		err := svc.Delete(ctx, "org1", "id1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := repo.deps["id1"]; ok {
			t.Errorf("expected dept to be deleted")
		}
	})
}
