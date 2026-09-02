// backend/internal/hrm/reimbursements/service.go
package reimbursements

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

// Service covers reimbursements end to end: create, submit for approval, and
// the two methods that structurally satisfy payslips.ReimbursementSource.
type Service interface {
	List(ctx context.Context, orgID string, filter ListFilter) (*ListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Reimbursement, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateRequest) (*Reimbursement, error)
	Submit(ctx context.Context, orgID, ref, submittedBy string) (*Reimbursement, error)
	Cancel(ctx context.Context, orgID, ref string) (*Reimbursement, error)
	// HandleApprovalDecision is registered via
	// approvalsSvc.RegisterCallback("reimbursement", ...) in main.go.
	HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error

	// PendingForEmployee and MarkReimbursementsPaid satisfy
	// payslips.ReimbursementSource structurally. See that interface's doc
	// comment in hrm/payslips/model.go.
	PendingForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]payslips.PendingReimbursement, error)
	MarkReimbursementsPaid(ctx context.Context, runID string, paid []payslips.PaidReimbursementLine) error
}

type serviceImpl struct {
	repo         Repository
	db           *pgxpool.Pool
	approvalsSvc approvals.Service
}

func NewService(repo Repository, db *pgxpool.Pool, approvalsSvc approvals.Service) Service {
	return &serviceImpl{repo: repo, db: db, approvalsSvc: approvalsSvc}
}

func (s *serviceImpl) List(ctx context.Context, orgID string, filter ListFilter) (*ListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.List(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("reimbursements: List: %w", err)
	}
	return &ListResponse{Reimbursements: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Reimbursement, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("reimbursements: Get: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateRequest) (*Reimbursement, error) {
	cat := Category(strings.TrimSpace(req.Category))
	if !cat.IsValid() {
		return nil, ErrInvalidCategory
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, fmt.Errorf("reimbursements: Create: employee_id is required")
	}
	currency := "USD"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}

	r := &Reimbursement{
		OrgID: orgID, EmployeeID: employeeID, Category: cat, Description: nilIfBlank(req.Description),
		Amount: amount.Round(2), Currency: currency, Status: StatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("reimbursements: Create: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) Submit(ctx context.Context, orgID, ref, submittedBy string) (*Reimbursement, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("reimbursements: Submit: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status != StatusDraft {
		return nil, ErrWrongStatus
	}

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeReimbursement)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "reimbursement", EntityID: r.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("reimbursements: Submit: creating approval instance: %w", iErr)
		}
		r.ApprovalInstanceID = &inst.ID
		r.Status = StatusPendingApproval
		if err := s.repo.Update(ctx, r); err != nil {
			return nil, fmt.Errorf("reimbursements: Submit: %w", err)
		}
		return r, nil
	}

	// No approval template configured — unchanged fallback behavior.
	r.Status = StatusApproved
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, fmt.Errorf("reimbursements: Submit: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID, ref string) (*Reimbursement, error) {
	r, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("reimbursements: Cancel: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status == StatusPaid || r.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	r.Status = StatusCancelled
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, fmt.Errorf("reimbursements: Cancel: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	r, err := s.repo.FindByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("reimbursements: HandleApprovalDecision: %w", err)
	}
	if r == nil {
		return ErrNotFound
	}
	if r.Status != StatusPendingApproval {
		return nil
	}
	r.Status = StatusApproved
	if !approved {
		r.Status = StatusRejected
	}
	if err := s.repo.Update(ctx, r); err != nil {
		return fmt.Errorf("reimbursements: HandleApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) PendingForEmployee(ctx context.Context, orgID, employeeID string, _, _ int) ([]payslips.PendingReimbursement, error) {
	rows, err := s.repo.PendingForEmployee(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("reimbursements: PendingForEmployee: %w", err)
	}
	out := make([]payslips.PendingReimbursement, 0, len(rows))
	for _, r := range rows {
		desc := "Reimbursement (" + string(r.Category) + ")"
		if r.Description != nil && strings.TrimSpace(*r.Description) != "" {
			desc = *r.Description
		}
		out = append(out, payslips.PendingReimbursement{ReimbursementID: r.ID, Description: desc, Amount: r.Amount})
	}
	return out, nil
}

// MarkReimbursementsPaid follows compensation.MarkBonusesPaid's shape
// exactly: atomic across the whole batch (one transaction), and — like
// BonusSource.MarkBonusesPaid — matched by bare id, not org-scoped, because
// every id here was sourced from OUR OWN PendingForEmployee call earlier in
// the same run, not from caller input.
func (s *serviceImpl) MarkReimbursementsPaid(ctx context.Context, runID string, paid []payslips.PaidReimbursementLine) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("reimbursements: MarkReimbursementsPaid: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, p := range paid {
		ct, err := tx.Exec(ctx,
			`UPDATE hrm_reimbursements
			    SET status='paid', payslip_run_id=$2::uuid, payslip_line_id=$3::uuid, paid_at=NOW(), updated_at=NOW()
			  WHERE id=$1::uuid AND status='approved'`,
			p.ReimbursementID, runID, p.LineID)
		if err != nil {
			return fmt.Errorf("reimbursements: MarkReimbursementsPaid: %s: %w", p.ReimbursementID, err)
		}
		if ct.RowsAffected() == 0 {
			return fmt.Errorf("reimbursements: MarkReimbursementsPaid: %s was not in approved status", p.ReimbursementID)
		}
	}
	return tx.Commit(ctx)
}

func nilIfBlank(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	return &trimmed
}
