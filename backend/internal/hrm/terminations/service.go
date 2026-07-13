// backend/internal/hrm/terminations/service.go
package terminations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
)

const dateLayout = "2006-01-02"

// Service defines business logic for employee terminations.
// Termination is always HR-initiated — employees use resignations.
type Service interface {
	List(ctx context.Context, orgID, employeeID, status string) (*TerminationListResponse, error)
	Get(ctx context.Context, orgID, employeeID, ref string) (*Termination, error)
	Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateTerminationRequest) (*Termination, error)
	Update(ctx context.Context, orgID, employeeID, ref string, req UpdateTerminationRequest) (*Termination, error)
	// Submit moves draft → pending_approval (if an approval chain is configured
	// for "termination") or straight to approved (fallback, unchanged behavior).
	Submit(ctx context.Context, orgID, employeeID, ref, submittedBy string) (*Termination, error)
	Cancel(ctx context.Context, orgID, employeeID, ref string) (*Termination, error)
	// Apply is a transactional operation: marks applied AND sets employee.status=terminated.
	Apply(ctx context.Context, orgID, employeeID, ref, appliedBy string) (*Termination, error)
	// HandleApprovalDecision is called back by the approvals service when a
	// termination's approval instance reaches a terminal state. Not meant to
	// be called directly by handlers — registered in main.go via RegisterCallback.
	HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error
}

type serviceImpl struct {
	repo         Repository
	db           *pgxpool.Pool // for cross-table Apply transaction
	approvalsSvc approvals.Service
}

func NewService(repo Repository, db *pgxpool.Pool, approvalsSvc approvals.Service) Service {
	return &serviceImpl{repo: repo, db: db, approvalsSvc: approvalsSvc}
}

func (s *serviceImpl) List(ctx context.Context, orgID, employeeID, status string) (*TerminationListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, employeeID, status)
	if err != nil { return nil, fmt.Errorf("terminations: List: %w", err) }
	if list == nil { list = []*Termination{} }
	return &TerminationListResponse{Terminations: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, employeeID, ref string) (*Termination, error) {
	t, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("terminations: Get: %w", err) }
	if t == nil { return nil, ErrNotFound }
	return t, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateTerminationRequest) (*Termination, error) {
	if !req.TerminationType.IsValid() { return nil, ErrInvalidTerminationType }
	if strings.TrimSpace(req.TerminationDate) == "" { return nil, ErrTerminationDateRequired }
	if strings.TrimSpace(req.LastWorkingDate) == "" { return nil, ErrLastWorkingDateRequired }
	if _, err := time.Parse(dateLayout, req.TerminationDate); err != nil { return nil, ErrInvalidDate }
	if _, err := time.Parse(dateLayout, req.LastWorkingDate); err != nil { return nil, ErrInvalidDate }

	// Prevent duplicate active termination (enforced by DB partial unique index too)
	existing, err := s.repo.FindActiveByEmployee(ctx, orgID, employeeID)
	if err != nil { return nil, fmt.Errorf("terminations: Create: check active: %w", err) }
	if existing != nil { return nil, ErrAlreadyActiveTermination }

	currency := "BDT"
	if req.SeveranceCurrency != nil && strings.TrimSpace(*req.SeveranceCurrency) != "" {
		currency = *req.SeveranceCurrency
	}
	rehireEligible := true
	if req.IsRehireEligible != nil { rehireEligible = *req.IsRehireEligible }

	t := &Termination{
		OrgID: orgID, EmployeeID: employeeID,
		TerminationType: req.TerminationType,
		TerminationDate: req.TerminationDate,
		LastWorkingDate: req.LastWorkingDate,
		Reason:          req.Reason,
		InternalNotes:   req.InternalNotes,
		SeveranceAmount: req.SeveranceAmount,
		SeveranceCurrency: currency,
		IsRehireEligible: rehireEligible,
		Status:          StatusDraft,
		CreatedBy:       createdBy,
	}
	if err := s.repo.Create(ctx, t); err != nil { return nil, fmt.Errorf("terminations: Create: %w", err) }
	return t, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, employeeID, ref string, req UpdateTerminationRequest) (*Termination, error) {
	t, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("terminations: Update: %w", err) }
	if t == nil { return nil, ErrNotFound }
	if t.Status == StatusApplied || t.Status == StatusCancelled || t.Status == StatusRejected {
		return nil, ErrWrongStatus
	}

	if req.TerminationDate != nil {
		if _, err := time.Parse(dateLayout, *req.TerminationDate); err != nil { return nil, ErrInvalidDate }
		t.TerminationDate = *req.TerminationDate
	}
	if req.LastWorkingDate != nil {
		if _, err := time.Parse(dateLayout, *req.LastWorkingDate); err != nil { return nil, ErrInvalidDate }
		t.LastWorkingDate = *req.LastWorkingDate
	}
	if req.Reason != nil { t.Reason = req.Reason }
	if req.InternalNotes != nil { t.InternalNotes = req.InternalNotes }
	if req.SeveranceAmount != nil { t.SeveranceAmount = req.SeveranceAmount }
	if req.IsRehireEligible != nil { t.IsRehireEligible = *req.IsRehireEligible }
	if req.ExitClearanceCompleted != nil { t.ExitClearanceCompleted = *req.ExitClearanceCompleted }
	if req.DocumentID != nil { t.DocumentID = req.DocumentID }

	if err := s.repo.Update(ctx, t); err != nil { return nil, fmt.Errorf("terminations: Update: %w", err) }
	return t, nil
}

func (s *serviceImpl) Submit(ctx context.Context, orgID, employeeID, ref, submittedBy string) (*Termination, error) {
	t, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("terminations: Submit: %w", err) }
	if t == nil { return nil, ErrNotFound }
	if t.Status != StatusDraft { return nil, ErrWrongStatus }

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeTermination)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "termination", EntityID: t.ID, RequestedBy: submittedBy,
		})
		if iErr != nil { return nil, fmt.Errorf("terminations: Submit: creating approval instance: %w", iErr) }
		if err := s.repo.SetApprovalInstance(ctx, t.ID, inst.ID, StatusPendingApproval); err != nil {
			return nil, fmt.Errorf("terminations: Submit: %w", err)
		}
		t.ApprovalInstanceID = &inst.ID
		t.Status = StatusPendingApproval
		return t, nil
	}

	// No approval template configured — unchanged fallback behavior
	if err := s.repo.UpdateStatus(ctx, t.ID, StatusApproved); err != nil {
		return nil, fmt.Errorf("terminations: Submit: %w", err)
	}
	t.Status = StatusApproved
	return t, nil
}

// HandleApprovalDecision reacts to the termination's approval instance completing.
// Approved  → status becomes 'approved' (same state Submit would have set directly).
// Rejected  → status becomes 'rejected', a terminal state; HR must create a new record.
func (s *serviceImpl) HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	t, err := s.repo.FindByRef(ctx, orgID, "", entityID)
	if err != nil { return fmt.Errorf("terminations: HandleApprovalDecision: %w", err) }
	if t == nil { return ErrNotFound }
	if t.Status != StatusPendingApproval { return nil } // already moved on (e.g. cancelled) — nothing to do
	status := StatusApproved
	if !approved { status = StatusRejected }
	if err := s.repo.UpdateStatus(ctx, t.ID, status); err != nil {
		return fmt.Errorf("terminations: HandleApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID, employeeID, ref string) (*Termination, error) {
	t, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("terminations: Cancel: %w", err) }
	if t == nil { return nil, ErrNotFound }
	if t.Status == StatusApplied { return nil, ErrAlreadyApplied }
	if t.Status == StatusCancelled { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, t.ID, StatusCancelled); err != nil {
		return nil, fmt.Errorf("terminations: Cancel: %w", err)
	}
	t.Status = StatusCancelled
	return t, nil
}

// Apply executes the termination atomically:
//  1. Sets termination.status = 'applied'
//  2. Sets employee.status = 'terminated', employee.termination_date = last_working_date
func (s *serviceImpl) Apply(ctx context.Context, orgID, employeeID, ref, appliedBy string) (*Termination, error) {
	t, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("terminations: Apply: %w", err) }
	if t == nil { return nil, ErrNotFound }
	if t.Status == StatusApplied { return nil, ErrAlreadyApplied }
	if t.Status != StatusApproved { return nil, ErrNotApproved }

	tx, err := s.db.Begin(ctx)
	if err != nil { return nil, fmt.Errorf("terminations: Apply: begin tx: %w", err) }
	defer tx.Rollback(ctx)

	// 1. Mark termination applied
	if _, err = tx.Exec(ctx,
		`UPDATE hrm_terminations SET status='applied', applied_at=NOW(), applied_by=$1::uuid, updated_at=NOW() WHERE id=$2::uuid`,
		appliedBy, t.ID,
	); err != nil {
		return nil, fmt.Errorf("terminations: Apply: update termination: %w", err)
	}

	// 2. Update employee: terminated status + termination_date
	if _, err = tx.Exec(ctx,
		`UPDATE hrm_employees SET
		 status = 'terminated',
		 termination_date = $1::date,
		 updated_at = NOW()
		WHERE id=$2::uuid AND org_id=$3::uuid`,
		t.LastWorkingDate, t.EmployeeID, t.OrgID,
	); err != nil {
		return nil, fmt.Errorf("terminations: Apply: update employee: %w", err)
	}

	if err := tx.Commit(ctx); err != nil { return nil, fmt.Errorf("terminations: Apply: commit: %w", err) }

	now := time.Now()
	t.Status = StatusApplied
	t.AppliedAt = &now
	t.AppliedBy = &appliedBy
	return t, nil
}
