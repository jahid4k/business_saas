// backend/internal/tests/unit/hrm/resignations/service_test.go
package resignations_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mridha/businesssaas/internal/hrm/resignations"
)

type stubRepo struct {
	byID map[string]*resignations.Resignation
	seq  int
}

func newStubRepo() *stubRepo {
	return &stubRepo{byID: make(map[string]*resignations.Resignation)}
}

func (r *stubRepo) nextID() string {
	r.seq++
	return fmt.Sprintf("res-%d", r.seq)
}

func (r *stubRepo) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*resignations.Resignation, error) {
	var out []*resignations.Resignation
	for _, res := range r.byID {
		if res.OrgID == orgID {
			if employeeID != "" && res.EmployeeID != employeeID {
				continue
			}
			if status != "" && string(res.Status) != status {
				continue
			}
			out = append(out, res)
		}
	}
	return out, nil
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*resignations.Resignation, error) {
	res, ok := r.byID[ref]
	if !ok || res.OrgID != orgID {
		return nil, nil
	}
	if employeeID != "" && res.EmployeeID != employeeID {
		return nil, nil
	}
	return res, nil
}

func (r *stubRepo) FindActiveByEmployee(ctx context.Context, orgID, employeeID string) (*resignations.Resignation, error) {
	for _, res := range r.byID {
		if res.OrgID == orgID && res.EmployeeID == employeeID {
			switch res.Status {
			case resignations.StatusSubmitted, resignations.StatusAccepted:
				return res, nil
			}
		}
	}
	return nil, nil
}

func (r *stubRepo) Create(ctx context.Context, res *resignations.Resignation) error {
	res.ID = r.nextID()
	res.PublicID = "pub_" + res.ID
	res.CreatedAt = time.Now()
	res.UpdatedAt = time.Now()
	r.byID[res.ID] = res
	return nil
}

func (r *stubRepo) Update(ctx context.Context, res *resignations.Resignation) error {
	if _, ok := r.byID[res.ID]; !ok {
		return errors.New("not found")
	}
	res.UpdatedAt = time.Now()
	r.byID[res.ID] = res
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status resignations.ResignationStatus) error {
	res, ok := r.byID[id]
	if !ok {
		return errors.New("not found")
	}
	res.Status = status
	return nil
}

func newDummyPool() *pgxpool.Pool {
	cfg, _ := pgxpool.ParseConfig("postgres://dummy:dummy@127.0.0.1:5432/dummy?sslmode=disable")
	pool, _ := pgxpool.NewWithConfig(context.Background(), cfg)
	return pool
}

func ptrStr(s string) *string { return &s }
func ptrBool(b bool) *bool { return &b }

func TestResignationsService(t *testing.T) {
	repo := newStubRepo()
	svc := resignations.NewService(repo, newDummyPool())
	ctx := context.Background()

	t.Run("Submit Success", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{
			ResignationDate: "2026-07-01",
			ReasonCategory:  resignations.ReasonBetterOpportunity,
		}
		res, err := svc.Submit(ctx, "org1", "emp1", "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res.ReasonCategory != resignations.ReasonBetterOpportunity {
			t.Errorf("expected better_opportunity, got %s", res.ReasonCategory)
		}
	})

	t.Run("Submit Validation Error", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{ResignationDate: ""} // missing date
		_, err := svc.Submit(ctx, "org1", "emp2", "admin", req)
		if err != resignations.ErrResignationDateReq {
			t.Errorf("expected ErrResignationDateReq, got %v", err)
		}
	})

	t.Run("Submit Already Active", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{ResignationDate: "2026-07-01"}
		_, err := svc.Submit(ctx, "org1", "emp1", "admin", req) // emp1 already has one from earlier test
		if err != resignations.ErrAlreadyActive {
			t.Errorf("expected ErrAlreadyActive, got %v", err)
		}
	})

	t.Run("Get and List", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{ResignationDate: "2026-07-01"}
		res, _ := svc.Submit(ctx, "org1", "emp3", "admin", req)

		fetched, err := svc.Get(ctx, "org1", "emp3", res.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched.ID != res.ID {
			t.Errorf("ID mismatch")
		}

		list, err := svc.List(ctx, "org1", "emp3", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if list.Total < 1 {
			t.Errorf("expected at least 1 resignation in list")
		}
	})

	t.Run("Update", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{ResignationDate: "2026-07-01"}
		res, _ := svc.Submit(ctx, "org1", "emp4", "admin", req)

		updateReq := resignations.UpdateResignationRequest{ExitInterviewCompleted: ptrBool(true)}
		updated, err := svc.Update(ctx, "org1", "emp4", res.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !updated.ExitInterviewCompleted {
			t.Errorf("expected exit interview completed true")
		}
	})
	
	t.Run("Withdraw", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{ResignationDate: "2026-07-01"}
		res, _ := svc.Submit(ctx, "org1", "emp5", "admin", req)

		withdrawn, err := svc.Withdraw(ctx, "org1", "emp5", res.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if withdrawn.Status != resignations.StatusWithdrawn {
			t.Errorf("expected withdrawn, got %s", withdrawn.Status)
		}
	})
	
	t.Run("Reject", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{ResignationDate: "2026-07-01"}
		res, _ := svc.Submit(ctx, "org1", "emp6", "admin", req)

		rejected, err := svc.Reject(ctx, "org1", "emp6", res.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if rejected.Status != resignations.StatusRejected {
			t.Errorf("expected rejected, got %s", rejected.Status)
		}
	})
	
	t.Run("Cross-org isolation", func(t *testing.T) {
		req := resignations.SubmitResignationRequest{ResignationDate: "2026-07-01"}
		res, _ := svc.Submit(ctx, "org1", "emp7", "admin", req)

		_, err := svc.Get(ctx, "org2", "emp7", res.ID)
		if err != resignations.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org access, got %v", err)
		}
	})
}
