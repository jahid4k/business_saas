// backend/internal/hrm/loans/service.go
package loans

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

// Service covers employee loans end to end: create, batch-style single-loan
// approval, disbursement (which generates the amortization schedule exactly
// once), foreclosure, and the two methods that structurally satisfy
// payslips.LoanSource.
type Service interface {
	ListLoans(ctx context.Context, orgID string, filter ListFilter) (*LoanListResponse, error)
	GetLoan(ctx context.Context, orgID, ref string) (*Loan, error)
	CreateLoan(ctx context.Context, orgID, createdBy string, req CreateLoanRequest) (*Loan, error)
	SubmitLoan(ctx context.Context, orgID, ref, submittedBy string) (*Loan, error)
	// HandleApprovalDecision is registered via
	// approvalsSvc.RegisterCallback("loan", ...) in main.go. Only flips
	// approved/rejected — DisburseLoan is a distinct step, the
	// promotions.Apply / compensation.ApplyCycle precedent: a decision and
	// the money movement it authorizes are never the same call.
	HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error
	// DisburseLoan generates the amortization schedule ONCE — see migration
	// 00100's header — and marks the loan active.
	DisburseLoan(ctx context.Context, orgID, ref, disbursedBy string) (*Loan, error)
	ListSchedule(ctx context.Context, orgID, ref string) ([]*ScheduleRow, error)
	ForecloseLoan(ctx context.Context, orgID, ref, foreclosedBy string, req ForecloseLoanRequest) (*Loan, error)

	// PendingInstallmentsForEmployee and RecordRecoveries satisfy
	// payslips.LoanSource structurally. See that interface's doc comment in
	// hrm/payslips/model.go for why loans imports payslips and not the
	// reverse.
	PendingInstallmentsForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]payslips.PendingInstallment, error)
	RecordRecoveries(ctx context.Context, runID string, applications []payslips.RecoveryApplication) error
}

type serviceImpl struct {
	repo         Repository
	db           *pgxpool.Pool
	approvalsSvc approvals.Service
}

func NewService(repo Repository, db *pgxpool.Pool, approvalsSvc approvals.Service) Service {
	return &serviceImpl{repo: repo, db: db, approvalsSvc: approvalsSvc}
}

func (s *serviceImpl) ListLoans(ctx context.Context, orgID string, filter ListFilter) (*LoanListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListLoans(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("loans: ListLoans: %w", err)
	}
	return &LoanListResponse{Loans: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetLoan(ctx context.Context, orgID, ref string) (*Loan, error) {
	l, err := s.repo.FindLoanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("loans: GetLoan: %w", err)
	}
	if l == nil {
		return nil, ErrLoanNotFound
	}
	return l, nil
}

func (s *serviceImpl) CreateLoan(ctx context.Context, orgID, createdBy string, req CreateLoanRequest) (*Loan, error) {
	loanType := LoanType(strings.TrimSpace(req.LoanType))
	if !loanType.IsValid() {
		return nil, ErrInvalidLoanType
	}
	principal, err := decimal.NewFromString(strings.TrimSpace(req.PrincipalAmount))
	if err != nil || !principal.IsPositive() {
		return nil, ErrInvalidAmount
	}
	rate := decimal.Zero
	if strings.TrimSpace(req.InterestRatePct) != "" {
		rate, err = decimal.NewFromString(strings.TrimSpace(req.InterestRatePct))
		if err != nil || rate.IsNegative() {
			return nil, ErrInvalidAmount
		}
	}
	if req.TenureMonths <= 0 {
		return nil, ErrInvalidTenure
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, fmt.Errorf("loans: CreateLoan: employee_id is required")
	}

	l := &Loan{
		OrgID: orgID, EmployeeID: employeeID, LoanType: loanType,
		PrincipalAmount: principal, InterestRatePct: rate, TenureMonths: req.TenureMonths,
		Status: LoanDraft, CreatedBy: createdBy,
	}
	if err := s.repo.CreateLoan(ctx, l); err != nil {
		return nil, fmt.Errorf("loans: CreateLoan: %w", err)
	}
	return l, nil
}

func (s *serviceImpl) SubmitLoan(ctx context.Context, orgID, ref, submittedBy string) (*Loan, error) {
	l, err := s.repo.FindLoanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("loans: SubmitLoan: %w", err)
	}
	if l == nil {
		return nil, ErrLoanNotFound
	}
	if l.Status != LoanDraft {
		return nil, ErrWrongLoanStatus
	}

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeLoan)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "loan", EntityID: l.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("loans: SubmitLoan: creating approval instance: %w", iErr)
		}
		l.ApprovalInstanceID = &inst.ID
		l.Status = LoanPendingApproval
		if err := s.repo.UpdateLoan(ctx, l); err != nil {
			return nil, fmt.Errorf("loans: SubmitLoan: %w", err)
		}
		return l, nil
	}

	// No approval template configured — unchanged fallback behavior, the
	// promotions/compensation precedent.
	l.Status = LoanApproved
	if err := s.repo.UpdateLoan(ctx, l); err != nil {
		return nil, fmt.Errorf("loans: SubmitLoan: %w", err)
	}
	return l, nil
}

func (s *serviceImpl) HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	l, err := s.repo.FindLoanByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("loans: HandleApprovalDecision: %w", err)
	}
	if l == nil {
		return ErrLoanNotFound
	}
	if l.Status != LoanPendingApproval {
		return nil
	}
	l.Status = LoanApproved
	if !approved {
		l.Status = LoanRejected
	}
	if err := s.repo.UpdateLoan(ctx, l); err != nil {
		return fmt.Errorf("loans: HandleApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) DisburseLoan(ctx context.Context, orgID, ref, disbursedBy string) (*Loan, error) {
	l, err := s.repo.FindLoanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("loans: DisburseLoan: %w", err)
	}
	if l == nil {
		return nil, ErrLoanNotFound
	}
	if l.Status != LoanApproved {
		return nil, ErrWrongLoanStatus
	}

	var roundingScale int32 = 2
	roundingMode := "half_up"
	_ = s.db.QueryRow(ctx, `SELECT money_rounding_scale, money_rounding_mode FROM organizations WHERE id=$1::uuid`, orgID).
		Scan(&roundingScale, &roundingMode)

	plan := Amortize(l.PrincipalAmount, l.InterestRatePct, l.TenureMonths, roundingScale, roundingMode)
	if len(plan) == 0 {
		return nil, fmt.Errorf("loans: DisburseLoan: amortization produced no schedule")
	}

	now := time.Now()
	startYear, startMonth := int(now.Year()), int(now.Month())
	rows := make([]*ScheduleRow, len(plan))
	for i, p := range plan {
		y, m := addMonths(startYear, startMonth, i)
		rows[i] = &ScheduleRow{
			InstallmentNumber: p.InstallmentNumber, DuePeriodYear: y, DuePeriodMonth: m,
			PrincipalComponent: p.PrincipalComponent, InterestComponent: p.InterestComponent,
			TotalAmount: p.TotalAmount, Status: SchedulePending,
		}
	}
	if err := s.repo.CreateSchedule(ctx, l.ID, rows); err != nil {
		return nil, fmt.Errorf("loans: DisburseLoan: %w", err)
	}

	// The level EMI most installments share — see model.go's doc comment on
	// installment_amount for why the final installment may differ.
	emi := plan[0].TotalAmount
	l.InstallmentAmount = &emi
	l.Status = LoanActive
	l.DisbursedAt = &now
	l.DisbursedBy = &disbursedBy
	if err := s.repo.UpdateLoan(ctx, l); err != nil {
		return nil, fmt.Errorf("loans: DisburseLoan: %w", err)
	}
	return l, nil
}

// addMonths returns (year, month) offset by n months from (year, month),
// wrapping December -> January correctly. Recovery due dates start the
// month AFTER disbursement — the first repayment is not due the same month
// the money was received.
func addMonths(year, month, n int) (int, int) {
	total := (year*12 + (month - 1)) + n + 1
	return total / 12, total%12 + 1
}

func (s *serviceImpl) ListSchedule(ctx context.Context, orgID, ref string) ([]*ScheduleRow, error) {
	l, err := s.repo.FindLoanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("loans: ListSchedule: %w", err)
	}
	if l == nil {
		return nil, ErrLoanNotFound
	}
	return s.repo.ListScheduleByLoan(ctx, l.ID)
}

func (s *serviceImpl) ForecloseLoan(ctx context.Context, orgID, ref, foreclosedBy string, req ForecloseLoanRequest) (*Loan, error) {
	l, err := s.repo.FindLoanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("loans: ForecloseLoan: %w", err)
	}
	if l == nil {
		return nil, ErrLoanNotFound
	}
	if l.Status != LoanActive {
		return nil, ErrWrongLoanStatus
	}
	amt, err := decimal.NewFromString(strings.TrimSpace(req.ForeclosureAmount))
	if err != nil || amt.IsNegative() {
		return nil, ErrInvalidAmount
	}
	if err := s.repo.ForecloseLoan(ctx, orgID, l.ID, foreclosedBy, amt.String()); err != nil {
		return nil, fmt.Errorf("loans: ForecloseLoan: %w", err)
	}
	return s.repo.FindLoanByRef(ctx, orgID, ref)
}

func (s *serviceImpl) PendingInstallmentsForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]payslips.PendingInstallment, error) {
	rows, err := s.repo.PendingInstallmentsForEmployee(ctx, orgID, employeeID, year, month)
	if err != nil {
		return nil, fmt.Errorf("loans: PendingInstallmentsForEmployee: %w", err)
	}
	out := make([]payslips.PendingInstallment, 0, len(rows))
	for _, r := range rows {
		out = append(out, payslips.PendingInstallment{
			LoanID: r.LoanID, ScheduleID: r.ID,
			Description: fmt.Sprintf("Loan Recovery (installment %d)", r.InstallmentNumber),
			AmountDue:   r.RemainingOwed(),
		})
	}
	return out, nil
}

func (s *serviceImpl) RecordRecoveries(ctx context.Context, runID string, applications []payslips.RecoveryApplication) error {
	in := make([]RecoveryApplicationInput, len(applications))
	for i, a := range applications {
		in[i] = RecoveryApplicationInput{ScheduleID: a.ScheduleID, LineID: a.LineID, AmountApplied: a.AmountApplied.String()}
	}
	if err := s.repo.RecordRecoveryEvents(ctx, runID, in); err != nil {
		return fmt.Errorf("loans: RecordRecoveries: %w", err)
	}
	return nil
}
