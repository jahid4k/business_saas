package contracts_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/contracts"
)

type stubRepo struct {
	contracts map[string]*contracts.EmployeeContract
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		contracts: make(map[string]*contracts.EmployeeContract),
	}
}

func (s *stubRepo) FindAll(ctx context.Context, orgID, employeeID string) ([]*contracts.EmployeeContract, error) {
	var list []*contracts.EmployeeContract
	for _, c := range s.contracts {
		if c.OrgID == orgID && c.EmployeeID == employeeID {
			list = append(list, c)
		}
	}
	return list, nil
}

func (s *stubRepo) FindActive(ctx context.Context, orgID, employeeID string) (*contracts.EmployeeContract, error) {
	for _, c := range s.contracts {
		if c.OrgID == orgID && c.EmployeeID == employeeID && c.IsActive {
			return c, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*contracts.EmployeeContract, error) {
	for _, c := range s.contracts {
		if c.OrgID == orgID && c.EmployeeID == employeeID && (c.ID == ref || c.PublicID == ref) {
			return c, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) Create(ctx context.Context, c *contracts.EmployeeContract) error {
	c.ID = "ctr_" + time.Now().Format("20060102150405.000")
	s.contracts[c.ID] = c
	return nil
}

func (s *stubRepo) Update(ctx context.Context, c *contracts.EmployeeContract) error {
	s.contracts[c.ID] = c
	return nil
}

func (s *stubRepo) Deactivate(ctx context.Context, orgID, employeeID, ref string) error {
	for _, c := range s.contracts {
		if c.OrgID == orgID && c.EmployeeID == employeeID && (c.ID == ref || c.PublicID == ref) {
			c.IsActive = false
			return nil
		}
	}
	return contracts.ErrContractNotFound
}

func TestContractsService(t *testing.T) {
	repo := newStubRepo()
	svc := contracts.NewService(repo)
	ctx := context.Background()

	orgID := "org_1"
	empID := "emp_1"
	createdBy := "user_1"

	// Create Contract
	req := contracts.CreateContractRequest{
		ContractType: contracts.ContractTypePermanent,
		StartDate:    "2024-01-01",
	}
	ctr, err := svc.Create(ctx, orgID, empID, createdBy, req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !ctr.IsActive {
		t.Errorf("Expected contract to be active")
	}

	// Active Conflict
	_, err = svc.Create(ctx, orgID, empID, createdBy, req)
	if err != contracts.ErrActiveContractExists {
		t.Errorf("Expected ErrActiveContractExists, got %v", err)
	}

	// Update Contract
	newNotes := "Updated"
	updateReq := contracts.UpdateContractRequest{
		Notes: &newNotes,
	}
	updated, err := svc.Update(ctx, orgID, empID, ctr.ID, updateReq)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if *updated.Notes != "Updated" {
		t.Errorf("Expected updated notes")
	}

	// List
	list, err := svc.List(ctx, orgID, empID)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if list.Total != 1 {
		t.Errorf("Expected 1 contract")
	}

	// Get Active
	active, err := svc.GetActive(ctx, orgID, empID)
	if err != nil {
		t.Fatalf("GetActive failed: %v", err)
	}
	if active.ID != ctr.ID {
		t.Errorf("ID mismatch")
	}

	// Get by Ref
	fetched, err := svc.Get(ctx, orgID, empID, ctr.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.ID != ctr.ID {
		t.Errorf("ID mismatch")
	}

	// Deactivate
	err = svc.Deactivate(ctx, orgID, empID, ctr.ID)
	if err != nil {
		t.Fatalf("Deactivate failed: %v", err)
	}

	// Verify inactive
	_, err = svc.GetActive(ctx, orgID, empID)
	if err != contracts.ErrContractNotFound {
		t.Errorf("Expected ErrContractNotFound after deactivation")
	}
}
