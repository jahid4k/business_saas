// backend/internal/hrm/compensation/revisions_service.go
package compensation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
)

// RevisionService covers salary revision cycles: create, compute against the
// merit matrix, override individual rows, submit for batch approval, and
// apply an approved cycle as real salary records.
type RevisionService interface {
	ListCycles(ctx context.Context, orgID string) ([]*RevisionCycle, error)
	GetCycle(ctx context.Context, orgID, ref string) (*RevisionCycle, error)
	CreateCycle(ctx context.Context, orgID, createdBy string, req CreateCycleRequest) (*RevisionCycle, error)
	// ComputeCycle builds one Revision per active employee using the merit
	// matrix (rating level x compa-ratio -> increase %). Safe to call again
	// on a draft or already-computed cycle — ReplaceRevisions clears the
	// previous set first, so recomputing never leaves stale rows alongside
	// fresh ones.
	ComputeCycle(ctx context.Context, orgID, ref string) (*RevisionCycle, error)
	ListRevisions(ctx context.Context, orgID, cycleRef string, filter ListFilter) (*RevisionListResponse, error)
	GetRevision(ctx context.Context, orgID, ref string) (*Revision, error)
	OverrideRevision(ctx context.Context, orgID, ref string, req OverrideRevisionRequest) (*Revision, error)
	SubmitCycle(ctx context.Context, orgID, ref, submittedBy string) (*RevisionCycle, error)
	// ApplyCycle writes hrm_employee_salary_records for every non-excluded
	// revision in an approved cycle. A distinct step from approval — the
	// promotions.Apply precedent (internal/hrm/promotions/service.go): a
	// decision and the money movement it authorizes are never the same call.
	ApplyCycle(ctx context.Context, orgID, ref, appliedBy string) (*RevisionCycle, error)
	// HandleApprovalDecision is registered via
	// approvalsSvc.RegisterCallback("salary_revision", ...) in main.go.
	HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error
}

func (s *serviceImpl) ListCycles(ctx context.Context, orgID string) ([]*RevisionCycle, error) {
	return s.repo.ListCycles(ctx, orgID)
}

func (s *serviceImpl) GetCycle(ctx context.Context, orgID, ref string) (*RevisionCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: GetCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateCycle(ctx context.Context, orgID, createdBy string, req CreateCycleRequest) (*RevisionCycle, error) {
	eff, err := parseDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}
	c := &RevisionCycle{
		OrgID: orgID, Name: strings.TrimSpace(req.Name), Description: nilIfBlank(req.Description),
		EffectiveDate: *eff, Status: CycleDraft, CreatedBy: createdBy,
	}
	if err := s.repo.CreateCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("compensation: CreateCycle: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) ComputeCycle(ctx context.Context, orgID, ref string) (*RevisionCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: ComputeCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	if c.Status != CycleDraft && c.Status != CycleComputed {
		return nil, ErrWrongCycleStatus
	}

	employeeIDs, err := s.repo.ListEligibleEmployees(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("compensation: ComputeCycle: %w", err)
	}

	revisions := make([]*Revision, 0, len(employeeIDs))
	for _, empID := range employeeIDs {
		rv, err := s.computeOneRevision(ctx, orgID, c, empID)
		if err != nil {
			return nil, fmt.Errorf("compensation: ComputeCycle: employee %s: %w", empID, err)
		}
		revisions = append(revisions, rv)
	}

	if err := s.repo.ReplaceRevisions(ctx, c.ID, revisions); err != nil {
		return nil, fmt.Errorf("compensation: ComputeCycle: %w", err)
	}

	now := time.Now()
	c.Status = CycleComputed
	c.ComputedAt = &now
	if err := s.repo.UpdateCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("compensation: ComputeCycle: %w", err)
	}
	return c, nil
}

// computeOneRevision resolves one employee's CompensationContext as of the
// cycle's effective_date, matches the merit matrix by rating level and
// compa-ratio, and proposes current_basic_pay * (1 + increase_pct/100).
//
// A missing rating or a missing band both leave proposed == current with
// computation_warning explaining why, rather than guessing an increase — see
// migration 00098's header on why that is a real column.
func (s *serviceImpl) computeOneRevision(ctx context.Context, orgID string, c *RevisionCycle, employeeID string) (*Revision, error) {
	cc, err := s.buildContext(ctx, orgID, employeeID, c.EffectiveDate)
	if err != nil {
		return nil, err
	}
	if err := s.attachLatestRating(ctx, orgID, cc, c.EffectiveDate); err != nil {
		return nil, err
	}

	rv := &Revision{
		OrgID: orgID, CycleID: c.ID, EmployeeID: employeeID,
		CurrentBasicPay: cc.CurrentBasicPay, ProposedBasicPay: cc.CurrentBasicPay,
		RatingLevelID: cc.RatingLevelID,
	}

	var warning string
	switch {
	case cc.Band == nil:
		warning = fmt.Sprintf("no compensation band found for grade %q as of %s", derefOr(cc.GradeLabel, "(none)"), c.EffectiveDate.Format("2006-01-02"))
	case cc.RatingLevelID == nil:
		warning = "no published appraisal to rate against as of " + c.EffectiveDate.Format("2006-01-02")
	default:
		compaRatioFloat, _ := cc.CompaRatio.Float64()
		cell, err := s.repo.FindMatrixCell(ctx, orgID, *cc.RatingLevelID, compaRatioFloat, c.EffectiveDate)
		if err != nil {
			return nil, err
		}
		if cell == nil {
			warning = "no merit matrix cell matches this rating and compa-ratio"
		} else {
			rv.ProposedBasicPay = ApplyIncrease(cc.CurrentBasicPay, cell.IncreasePct)
		}
	}
	if warning != "" {
		rv.ComputationWarning = &warning
	}
	rv.CalculationSnapshot = cc.Snapshot(map[string]any{"cycle_id": c.ID})
	return rv, nil
}

func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func (s *serviceImpl) ListRevisions(ctx context.Context, orgID, cycleRef string, filter ListFilter) (*RevisionListResponse, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, cycleRef)
	if err != nil {
		return nil, fmt.Errorf("compensation: ListRevisions: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	filter.Normalise()
	list, total, err := s.repo.ListRevisionsByCycle(ctx, orgID, c.ID, filter)
	if err != nil {
		return nil, fmt.Errorf("compensation: ListRevisions: %w", err)
	}
	return &RevisionListResponse{Revisions: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetRevision(ctx context.Context, orgID, ref string) (*Revision, error) {
	rv, err := s.repo.FindRevisionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: GetRevision: %w", err)
	}
	if rv == nil {
		return nil, ErrRevisionNotFound
	}
	return rv, nil
}

func (s *serviceImpl) OverrideRevision(ctx context.Context, orgID, ref string, req OverrideRevisionRequest) (*Revision, error) {
	rv, err := s.repo.FindRevisionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: OverrideRevision: %w", err)
	}
	if rv == nil {
		return nil, ErrRevisionNotFound
	}
	c, err := s.repo.FindCycleByRef(ctx, orgID, rv.CycleID)
	if err != nil {
		return nil, fmt.Errorf("compensation: OverrideRevision: %w", err)
	}
	if c == nil || (c.Status != CycleDraft && c.Status != CycleComputed) {
		return nil, ErrWrongCycleStatus
	}
	amt, err := decimal.NewFromString(strings.TrimSpace(req.ProposedBasicPay))
	if err != nil || amt.IsNegative() {
		return nil, ErrInvalidAmount
	}
	rv.ProposedBasicPay = roundMoney(amt)
	reason := strings.TrimSpace(req.Reason)
	rv.OverrideReason = &reason
	if err := s.repo.UpdateRevisionOverride(ctx, rv); err != nil {
		return nil, fmt.Errorf("compensation: OverrideRevision: %w", err)
	}
	return rv, nil
}

func (s *serviceImpl) SubmitCycle(ctx context.Context, orgID, ref, submittedBy string) (*RevisionCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: SubmitCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	if c.Status != CycleComputed {
		return nil, ErrWrongCycleStatus
	}

	filter := ListFilter{Limit: 1, Scope: authz.ScopeAll} // internal existence check, not a caller-scoped read
	_, total, err := s.repo.ListRevisionsByCycle(ctx, orgID, c.ID, filter)
	if err != nil {
		return nil, fmt.Errorf("compensation: SubmitCycle: %w", err)
	}
	if total == 0 {
		return nil, ErrCycleHasNoRevisions
	}

	now := time.Now()
	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeSalaryRevision)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "salary_revision", EntityID: c.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("compensation: SubmitCycle: creating approval instance: %w", iErr)
		}
		c.ApprovalInstanceID = &inst.ID
		c.Status = CyclePendingApproval
		c.SubmittedAt = &now
		if err := s.repo.UpdateCycle(ctx, c); err != nil {
			return nil, fmt.Errorf("compensation: SubmitCycle: %w", err)
		}
		return c, nil
	}

	// No approval template configured — unchanged fallback behavior, the
	// promotions/transfers/etc. precedent.
	c.Status = CycleApproved
	c.SubmittedAt = &now
	if err := s.repo.UpdateCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("compensation: SubmitCycle: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	c, err := s.repo.FindCycleByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("compensation: HandleApprovalDecision: %w", err)
	}
	if c == nil {
		return ErrCycleNotFound
	}
	if c.Status != CyclePendingApproval {
		return nil
	}
	c.Status = CycleApproved
	if !approved {
		c.Status = CycleRejected
	}
	if err := s.repo.UpdateCycle(ctx, c); err != nil {
		return fmt.Errorf("compensation: HandleApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) ApplyCycle(ctx context.Context, orgID, ref, appliedBy string) (*RevisionCycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("compensation: ApplyCycle: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	if c.Status != CycleApproved {
		return nil, ErrWrongCycleStatus
	}

	filter := ListFilter{Limit: MaxLimit, Scope: authz.ScopeAll} // internal aggregate operation, not a caller-scoped read
	list, total, err := s.repo.ListRevisionsByCycle(ctx, orgID, c.ID, filter)
	if err != nil {
		return nil, fmt.Errorf("compensation: ApplyCycle: %w", err)
	}
	for total > len(list) { // paginate past MaxLimit for very large cycles
		filter.Offset += filter.Limit
		more, _, err := s.repo.ListRevisionsByCycle(ctx, orgID, c.ID, filter)
		if err != nil {
			return nil, fmt.Errorf("compensation: ApplyCycle: %w", err)
		}
		if len(more) == 0 {
			break
		}
		list = append(list, more...)
	}

	for _, rv := range list {
		if rv.IsExcluded || rv.SalaryRecordID != nil {
			continue
		}
		var salaryRecordID string
		reason := "annual_revision"
		if err := s.db.QueryRow(ctx,
			`INSERT INTO hrm_employee_salary_records
			    (org_id, employee_id, basic_pay, effective_date, change_reason, created_by)
			 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text`,
			orgID, rv.EmployeeID, rv.ProposedBasicPay, c.EffectiveDate, reason, appliedBy,
		).Scan(&salaryRecordID); err != nil {
			return nil, fmt.Errorf("compensation: ApplyCycle: write salary record for employee %s: %w", rv.EmployeeID, err)
		}
		if err := s.repo.MarkRevisionApplied(ctx, rv.ID, salaryRecordID); err != nil {
			return nil, fmt.Errorf("compensation: ApplyCycle: %w", err)
		}
	}

	now := time.Now()
	c.Status = CycleApplied
	c.AppliedAt = &now
	c.AppliedBy = &appliedBy
	if err := s.repo.UpdateCycle(ctx, c); err != nil {
		return nil, fmt.Errorf("compensation: ApplyCycle: %w", err)
	}
	return c, nil
}
