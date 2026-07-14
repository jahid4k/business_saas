package payslips_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

type mockPayslipsRepo struct {
	runs     map[string]*payslips.PayslipRun
	payslips map[string]*payslips.Payslip
	lines    map[string][]*payslips.PayslipLine
}

func newMockPayslipsRepo() *mockPayslipsRepo {
	return &mockPayslipsRepo{
		runs:     make(map[string]*payslips.PayslipRun),
		payslips: make(map[string]*payslips.Payslip),
		lines:    make(map[string][]*payslips.PayslipLine),
	}
}

func (m *mockPayslipsRepo) FindRuns(ctx context.Context, orgID string) ([]*payslips.PayslipRun, error) {
	var list []*payslips.PayslipRun
	for _, r := range m.runs {
		if r.OrgID == orgID {
			list = append(list, r)
		}
	}
	return list, nil
}

func (m *mockPayslipsRepo) FindRunByRef(ctx context.Context, orgID, ref string) (*payslips.PayslipRun, error) {
	for _, r := range m.runs {
		if r.OrgID == orgID && (r.ID == ref || r.PublicID == ref) {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockPayslipsRepo) FindRunByPeriod(ctx context.Context, orgID string, year, month int) (*payslips.PayslipRun, error) {
	for _, r := range m.runs {
		if r.OrgID == orgID && r.PeriodYear == year && r.PeriodMonth == month {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockPayslipsRepo) CreateRun(ctx context.Context, r *payslips.PayslipRun) error {
	r.ID = "run-" + time.Now().String()
	r.PublicID = "pub-" + r.ID
	m.runs[r.ID] = r
	return nil
}

func (m *mockPayslipsRepo) UpdateRun(ctx context.Context, r *payslips.PayslipRun) error {
	if existing, ok := m.runs[r.ID]; ok {
		*existing = *r
	}
	return nil
}

func (m *mockPayslipsRepo) FindPayslips(ctx context.Context, orgID, runID, employeeID string) ([]*payslips.Payslip, error) {
	var list []*payslips.Payslip
	for _, p := range m.payslips {
		if p.OrgID != orgID {
			continue
		}
		if runID != "" && p.PayslipRunID != runID {
			continue
		}
		if employeeID != "" && p.EmployeeID != employeeID {
			continue
		}
		list = append(list, p)
	}
	return list, nil
}

func (m *mockPayslipsRepo) FindPayslipByRef(ctx context.Context, orgID, ref string) (*payslips.Payslip, error) {
	for _, p := range m.payslips {
		if p.OrgID == orgID && (p.ID == ref || p.PublicID == ref) {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockPayslipsRepo) CreatePayslip(ctx context.Context, p *payslips.Payslip) error {
	p.ID = "slip-" + p.EmployeeID
	m.payslips[p.ID] = p
	return nil
}

func (m *mockPayslipsRepo) CreatePayslipLines(ctx context.Context, lines []*payslips.PayslipLine) error {
	for _, l := range lines {
		l.ID = "line-" + time.Now().String()
		m.lines[l.PayslipID] = append(m.lines[l.PayslipID], l)
	}
	return nil
}

func (m *mockPayslipsRepo) LoadPayslipLines(ctx context.Context, payslipID string) ([]*payslips.PayslipLine, error) {
	return m.lines[payslipID], nil
}

func TestPayslipsService(t *testing.T) {
	repo := newMockPayslipsRepo()
	svc := payslips.NewService(repo, nil)
	ctx := context.Background()

	orgID := "org1"
	createdBy := "admin1"

	t.Run("CreateRun", func(t *testing.T) {
		req := payslips.CreateRunRequest{
			Year:        2024,
			Month:       1,
			Description: ptr("Jan 2024"),
		}
		run, err := svc.CreateRun(ctx, orgID, createdBy, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if run.Status != payslips.RunDraft {
			t.Errorf("expected draft status, got %v", run.Status)
		}
	})

	t.Run("CreateRun - Duplicate", func(t *testing.T) {
		req := payslips.CreateRunRequest{
			Year:        2024,
			Month:       1,
			Description: ptr("Jan 2024"),
		}
		_, err := svc.CreateRun(ctx, orgID, createdBy, req)
		if err != payslips.ErrDuplicateRun {
			t.Errorf("expected ErrDuplicateRun, got %v", err)
		}
	})

	t.Run("ApproveRun", func(t *testing.T) {
		// Mock computed state manually for testing ApproveRun
		run, _ := svc.GetRun(ctx, orgID, "pub-run-...") // Can't easily get ID, let's list
		runs, _ := svc.ListRuns(ctx, orgID)
		run = runs.Runs[0]
		run.Status = payslips.RunComputed
		repo.UpdateRun(ctx, run)

		approvedRun, err := svc.ApproveRun(ctx, orgID, run.ID, createdBy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if approvedRun.Status != payslips.RunApproved {
			t.Errorf("expected approved status, got %v", approvedRun.Status)
		}
	})

	t.Run("MarkPaid", func(t *testing.T) {
		runs, _ := svc.ListRuns(ctx, orgID)
		run := runs.Runs[0]

		paidRun, err := svc.MarkPaid(ctx, orgID, run.ID, createdBy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if paidRun.Status != payslips.RunPaid {
			t.Errorf("expected paid status, got %v", paidRun.Status)
		}
	})

	t.Run("CancelRun", func(t *testing.T) {
		req := payslips.CreateRunRequest{
			Year:  2024,
			Month: 2,
		}
		run, _ := svc.CreateRun(ctx, orgID, createdBy, req)

		cancelledRun, err := svc.CancelRun(ctx, orgID, run.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cancelledRun.Status != payslips.RunCancelled {
			t.Errorf("expected cancelled status, got %v", cancelledRun.Status)
		}
	})

	t.Run("ListPayslips", func(t *testing.T) {
		// manually insert a slip
		repo.CreatePayslip(ctx, &payslips.Payslip{
			OrgID:      orgID,
			EmployeeID: "emp1",
		})
		res, err := svc.ListPayslips(ctx, orgID, "", "emp1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("expected 1 payslip, got %d", res.Total)
		}
	})
	
	t.Run("Cross-Org Isolation", func(t *testing.T) {
		runs, _ := svc.ListRuns(ctx, orgID)
		run := runs.Runs[0]
		_, err := svc.GetRun(ctx, "org2", run.ID)
		if err != payslips.ErrNotFound {
			t.Errorf("expected ErrNotFound for cross-org fetch, got %v", err)
		}
	})
}

func ptr(s string) *string {
	return &s
}
