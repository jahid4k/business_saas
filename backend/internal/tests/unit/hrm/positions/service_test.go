package positions_test

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/positions"
)

type stubRepo struct {
	pos            map[string]*positions.Position
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
		pos: make(map[string]*positions.Position),
	}
}

func (s *stubRepo) FindAll(ctx context.Context, orgID string, departmentID string, activeOnly bool) ([]*positions.Position, error) {
	if s.errFindAll != nil {
		return nil, s.errFindAll
	}
	var res []*positions.Position
	for _, p := range s.pos {
		if p.OrgID == orgID {
			if departmentID != "" && (p.DepartmentID == nil || *p.DepartmentID != departmentID) {
				continue
			}
			if activeOnly && !p.IsActive {
				continue
			}
			res = append(res, p)
		}
	}
	return res, nil
}

func (s *stubRepo) Count(ctx context.Context, orgID string, departmentID string, activeOnly bool) (int, error) {
	if s.errCount != nil {
		return 0, s.errCount
	}
	count := 0
	for _, p := range s.pos {
		if p.OrgID == orgID {
			if departmentID != "" && (p.DepartmentID == nil || *p.DepartmentID != departmentID) {
				continue
			}
			if activeOnly && !p.IsActive {
				continue
			}
			count++
		}
	}
	return count, nil
}

func (s *stubRepo) FindByRef(ctx context.Context, orgID, ref string) (*positions.Position, error) {
	if s.errFindByRef != nil {
		return nil, s.errFindByRef
	}
	for _, p := range s.pos {
		if p.OrgID == orgID && (p.ID == ref || p.PublicID == ref) {
			return p, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) Create(ctx context.Context, p *positions.Position) error {
	if s.errCreate != nil {
		return s.errCreate
	}
	p.ID = "id_" + p.Title
	s.pos[p.ID] = p
	return nil
}

func (s *stubRepo) Update(ctx context.Context, p *positions.Position) error {
	if s.errUpdate != nil {
		return s.errUpdate
	}
	s.pos[p.ID] = p
	return nil
}

func (s *stubRepo) Delete(ctx context.Context, orgID, ref string) error {
	if s.errDelete != nil {
		return s.errDelete
	}
	for id, p := range s.pos {
		if p.OrgID == orgID && (p.ID == ref || p.PublicID == ref) {
			delete(s.pos, id)
			return nil
		}
	}
	return positions.ErrPositionNotFound
}

func (s *stubRepo) ExistsByTitle(ctx context.Context, orgID, title, excludeID string) (bool, error) {
	if s.errExists != nil {
		return false, s.errExists
	}
	if s.existsOverride {
		return true, nil
	}
	for _, p := range s.pos {
		if p.OrgID == orgID && p.Title == title && p.ID != excludeID && p.IsActive {
			return true, nil
		}
	}
	return false, nil
}

func ptrStr(s string) *string {
	return &s
}

func TestPositionsService_Create(t *testing.T) {
	repo := newStubRepo()
	svc := positions.NewService(repo)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := positions.CreatePositionRequest{
			Title: "Software Engineer",
		}
		p, err := svc.Create(ctx, "org1", "user1", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Title != "Software Engineer" {
			t.Errorf("expected Software Engineer, got %s", p.Title)
		}
	})

	t.Run("validation_error", func(t *testing.T) {
		req := positions.CreatePositionRequest{
			Title: "",
		}
		_, err := svc.Create(ctx, "org1", "user1", req)
		if err != positions.ErrTitleRequired {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
	})

	t.Run("conflict_title", func(t *testing.T) {
		repo.pos["id_exist"] = &positions.Position{
			ID: "id_exist",
			OrgID: "org1",
			Title: "Manager",
			IsActive: true,
		}
		req := positions.CreatePositionRequest{
			Title: "Manager",
		}
		_, err := svc.Create(ctx, "org1", "user1", req)
		if err != positions.ErrTitleConflict {
			t.Errorf("expected ErrTitleConflict, got %v", err)
		}
	})
}

func TestPositionsService_Get(t *testing.T) {
	repo := newStubRepo()
	svc := positions.NewService(repo)
	ctx := context.Background()

	repo.pos["id1"] = &positions.Position{
		ID: "id1",
		OrgID: "org1",
		Title: "Engineer",
	}

	t.Run("success", func(t *testing.T) {
		p, err := svc.Get(ctx, "org1", "id1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Title != "Engineer" {
			t.Errorf("expected Engineer, got %s", p.Title)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := svc.Get(ctx, "org1", "id2")
		if err != positions.ErrPositionNotFound {
			t.Errorf("expected ErrPositionNotFound, got %v", err)
		}
	})
}

func TestPositionsService_Update(t *testing.T) {
	repo := newStubRepo()
	svc := positions.NewService(repo)
	ctx := context.Background()

	repo.pos["id1"] = &positions.Position{
		ID: "id1",
		OrgID: "org1",
		Title: "Engineer",
	}

	t.Run("success", func(t *testing.T) {
		req := positions.UpdatePositionRequest{
			Title: ptrStr("Senior Engineer"),
		}
		p, err := svc.Update(ctx, "org1", "id1", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Title != "Senior Engineer" {
			t.Errorf("expected Senior Engineer, got %s", p.Title)
		}
	})
}

func TestPositionsService_List(t *testing.T) {
	repo := newStubRepo()
	svc := positions.NewService(repo)
	ctx := context.Background()

	repo.pos["id1"] = &positions.Position{ID: "id1", OrgID: "org1", DepartmentID: ptrStr("dep1"), IsActive: true}
	repo.pos["id2"] = &positions.Position{ID: "id2", OrgID: "org1", DepartmentID: ptrStr("dep2"), IsActive: false}

	t.Run("success_all", func(t *testing.T) {
		res, err := svc.List(ctx, "org1", "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 2 {
			t.Errorf("expected 2, got %d", res.Total)
		}
	})

	t.Run("success_filtered_active", func(t *testing.T) {
		res, err := svc.List(ctx, "org1", "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("expected 1, got %d", res.Total)
		}
	})

	t.Run("success_filtered_dept", func(t *testing.T) {
		res, err := svc.List(ctx, "org1", "dep2", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("expected 1, got %d", res.Total)
		}
	})
}

func TestPositionsService_Delete(t *testing.T) {
	repo := newStubRepo()
	svc := positions.NewService(repo)
	ctx := context.Background()

	repo.pos["id1"] = &positions.Position{ID: "id1", OrgID: "org1"}

	t.Run("success", func(t *testing.T) {
		err := svc.Delete(ctx, "org1", "id1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := repo.pos["id1"]; ok {
			t.Errorf("expected position to be deleted")
		}
	})
}
