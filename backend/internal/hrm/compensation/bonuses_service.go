// backend/internal/hrm/compensation/bonuses_service.go
package compensation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

// BonusService covers hrm_bonuses: creation, batch-style single-entity
// approval (one approval instance per bonus — unlike revision cycles, a
// bonus has no natural batch container of its own), and the two methods that
// structurally satisfy payslips.BonusSource.
type BonusService interface {
	ListBonuses(ctx context.Context, orgID string, filter ListFilter) (*BonusListResponse, error)
	GetBonus(ctx context.Context, orgID, ref string) (*Bonus, error)
	CreateBonus(ctx context.Context, orgID, createdBy string, req CreateBonusRequest) (*Bonus, error)
	SubmitBonus(ctx context.Context, orgID, ref, submittedBy string) (*Bonus, error)
	CancelBonus(ctx context.Context, orgID, ref string) (*Bonus, error)
	// HandleBonusApprovalDecision is registered via
	// approvalsSvc.RegisterCallback("bonus", ...) in main.go. Named distinctly
	// from RevisionService.HandleApprovalDecision because Service embeds both
	// and each entity_type needs its own callback target — the
	// recruitment.Service.HandleOfferApprovalDecision precedent alongside its
	// requisition HandleApprovalDecision.
	HandleBonusApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error

	// PendingBonusesForPeriod and MarkBonusesPaid satisfy payslips.BonusSource
	// structurally. See that interface's doc comment in
	// hrm/payslips/model.go for why compensation imports payslips and not the
	// reverse.
	PendingBonusesForPeriod(ctx context.Context, orgID string, year, month int) ([]payslips.PendingBonus, error)
	MarkBonusesPaid(ctx context.Context, runID string, paid []payslips.PaidBonusLine) error
}

func (s *serviceImpl) ListBonuses(ctx context.Context, orgID string, filter ListFilter) (*BonusListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListBonuses(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("compensation: ListBonuses: %w", err)
	}
	return &BonusListResponse{Bonuses: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetBonus(ctx context.Context, orgID, ref string) (*Bonus, error) {
	b, err := s.repo.FindBonusByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: GetBonus: %w", err)
	}
	if b == nil {
		return nil, ErrBonusNotFound
	}
	return b, nil
}

// CreateBonus builds the CompensationContext for the employee (for the
// snapshot's audit value — see migration 00098's header) and records the
// bonus at the amount HR provided directly. There is no pct-of-basic
// calc-method in this first cut: unlike salary revisions, which MUST compute
// from the merit matrix, an HR-entered bonus amount is the common case for
// every bonus_type this table supports, and CompensationContext.Snapshot
// still captures current_basic_pay/band/compa-ratio for audit even when the
// amount was typed, not derived.
func (s *serviceImpl) CreateBonus(ctx context.Context, orgID, createdBy string, req CreateBonusRequest) (*Bonus, error) {
	bonusType := BonusType(strings.TrimSpace(req.BonusType))
	if !bonusType.IsValid() {
		return nil, ErrInvalidBonusType
	}
	amount, err := parseMoney(req.Amount)
	if err != nil {
		return nil, err
	}
	if req.PeriodYear <= 0 {
		return nil, fmt.Errorf("compensation: CreateBonus: period_year is required")
	}
	if req.PeriodMonth != nil && (*req.PeriodMonth < 1 || *req.PeriodMonth > 12) {
		return nil, fmt.Errorf("compensation: CreateBonus: period_month must be between 1 and 12")
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, fmt.Errorf("compensation: CreateBonus: employee_id is required")
	}

	refDate := time.Now()
	if req.PeriodMonth != nil {
		refDate = time.Date(req.PeriodYear, time.Month(*req.PeriodMonth), 1, 0, 0, 0, 0, time.UTC)
	} else {
		refDate = time.Date(req.PeriodYear, time.December, 31, 0, 0, 0, 0, time.UTC)
	}
	cc, err := s.buildContext(ctx, orgID, employeeID, refDate)
	if err != nil {
		return nil, err
	}

	currency := "USD"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}

	b := &Bonus{
		OrgID: orgID, EmployeeID: employeeID, BonusType: bonusType,
		Description: nilIfBlank(req.Description),
		PeriodYear:  req.PeriodYear, PeriodMonth: req.PeriodMonth,
		Amount: roundMoney(amount), Currency: currency,
		CalculationSnapshot: cc.Snapshot(map[string]any{"bonus_type": string(bonusType), "amount": amount.String()}),
		Status:              BonusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.CreateBonus(ctx, b); err != nil {
		return nil, fmt.Errorf("compensation: CreateBonus: %w", err)
	}
	return b, nil
}

func (s *serviceImpl) SubmitBonus(ctx context.Context, orgID, ref, submittedBy string) (*Bonus, error) {
	b, err := s.repo.FindBonusByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: SubmitBonus: %w", err)
	}
	if b == nil {
		return nil, ErrBonusNotFound
	}
	if b.Status != BonusDraft {
		return nil, ErrWrongBonusStatus
	}

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeBonus)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "bonus", EntityID: b.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("compensation: SubmitBonus: creating approval instance: %w", iErr)
		}
		b.ApprovalInstanceID = &inst.ID
		b.Status = BonusPendingApproval
		if err := s.repo.UpdateBonus(ctx, b); err != nil {
			return nil, fmt.Errorf("compensation: SubmitBonus: %w", err)
		}
		return b, nil
	}

	// No approval template configured — unchanged fallback behavior, the
	// promotions/transfers/salary-revision-cycle precedent.
	b.Status = BonusApproved
	if err := s.repo.UpdateBonus(ctx, b); err != nil {
		return nil, fmt.Errorf("compensation: SubmitBonus: %w", err)
	}
	return b, nil
}

func (s *serviceImpl) CancelBonus(ctx context.Context, orgID, ref string) (*Bonus, error) {
	b, err := s.repo.FindBonusByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: CancelBonus: %w", err)
	}
	if b == nil {
		return nil, ErrBonusNotFound
	}
	if b.Status == BonusPaid || b.Status == BonusCancelled {
		return nil, ErrWrongBonusStatus
	}
	b.Status = BonusCancelled
	if err := s.repo.UpdateBonus(ctx, b); err != nil {
		return nil, fmt.Errorf("compensation: CancelBonus: %w", err)
	}
	return b, nil
}

func (s *serviceImpl) HandleBonusApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	b, err := s.repo.FindBonusByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("compensation: HandleBonusApprovalDecision: %w", err)
	}
	if b == nil {
		return ErrBonusNotFound
	}
	if b.Status != BonusPendingApproval {
		return nil
	}
	b.Status = BonusApproved
	if !approved {
		b.Status = BonusRejected
	}
	if err := s.repo.UpdateBonus(ctx, b); err != nil {
		return fmt.Errorf("compensation: HandleBonusApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) PendingBonusesForPeriod(ctx context.Context, orgID string, year, month int) ([]payslips.PendingBonus, error) {
	bonuses, err := s.repo.PendingForPeriod(ctx, orgID, year, month)
	if err != nil {
		return nil, fmt.Errorf("compensation: PendingBonusesForPeriod: %w", err)
	}
	out := make([]payslips.PendingBonus, 0, len(bonuses))
	for _, b := range bonuses {
		desc := "Bonus (" + string(b.BonusType) + ")"
		if b.Description != nil && strings.TrimSpace(*b.Description) != "" {
			desc = *b.Description
		}
		out = append(out, payslips.PendingBonus{
			BonusID: b.ID, EmployeeID: b.EmployeeID, Description: desc, Amount: b.Amount,
		})
	}
	return out, nil
}

func (s *serviceImpl) MarkBonusesPaid(ctx context.Context, runID string, paid []payslips.PaidBonusLine) error {
	// Atomic across the whole batch — see payslips.BonusSource's doc comment
	// on why a partial failure here must not leave some bonuses 'paid' with
	// a run reference and others still 'approved' after the SAME run
	// committed all of their payslip lines.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("compensation: MarkBonusesPaid: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, p := range paid {
		ct, err := tx.Exec(ctx,
			`UPDATE hrm_bonuses
			    SET status='paid', payslip_run_id=$2::uuid, payslip_line_id=$3::uuid, paid_at=NOW(), updated_at=NOW()
			  WHERE id=$1::uuid AND status='approved'`,
			p.BonusID, runID, p.LineID)
		if err != nil {
			return fmt.Errorf("compensation: MarkBonusesPaid: bonus %s: %w", p.BonusID, err)
		}
		if ct.RowsAffected() == 0 {
			return fmt.Errorf("compensation: MarkBonusesPaid: bonus %s was not in approved status", p.BonusID)
		}
	}
	return tx.Commit(ctx)
}
