// backend/internal/hrm/promotions/service.go
package promotions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
)

const dateLayout = "2006-01-02"

// Service defines business logic for employee promotions.
type Service interface {
	List(ctx context.Context, orgID, employeeID, status string) (*PromotionListResponse, error)
	Get(ctx context.Context, orgID, employeeID, ref string) (*Promotion, error)
	Create(ctx context.Context, orgID, employeeID, createdBy string, req CreatePromotionRequest) (*Promotion, error)
	Update(ctx context.Context, orgID, employeeID, ref string, req UpdatePromotionRequest) (*Promotion, error)
	// Submit moves draft → pending_approval (if an approval chain is configured
	// for "promotion") or straight to approved (fallback, unchanged behavior).
	Submit(ctx context.Context, orgID, employeeID, ref, submittedBy string) (*Promotion, error)
	Cancel(ctx context.Context, orgID, employeeID, ref string) (*Promotion, error)
	// Apply executes a transactional update: marks applied AND updates employee record + creates salary record.
	Apply(ctx context.Context, orgID, employeeID, ref, appliedBy string) (*Promotion, error)
	// HandleApprovalDecision is called back by the approvals service when a
	// promotion's approval instance reaches a terminal state.
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

func (s *serviceImpl) List(ctx context.Context, orgID, employeeID, status string) (*PromotionListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, employeeID, status)
	if err != nil { return nil, fmt.Errorf("promotions: List: %w", err) }
	if list == nil { list = []*Promotion{} }
	return &PromotionListResponse{Promotions: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, employeeID, ref string) (*Promotion, error) {
	p, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("promotions: Get: %w", err) }
	if p == nil { return nil, ErrNotFound }
	return p, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, employeeID, createdBy string, req CreatePromotionRequest) (*Promotion, error) {
	if strings.TrimSpace(req.ToPositionID) == "" { return nil, ErrToPositionRequired }
	if strings.TrimSpace(req.EffectiveDate) == "" { return nil, ErrEffectiveDateReq }
	if _, err := time.Parse(dateLayout, req.EffectiveDate); err != nil { return nil, ErrInvalidDate }

	// Snapshot current employee state
	var fromPosID, fromDeptID *string
	var fromBasicPay *float64
	_ = s.db.QueryRow(ctx,
		`SELECT position_id::text, department_id::text,
		(SELECT basic_pay FROM hrm_employee_salary_records
		 WHERE employee_id=e.id AND effective_date<=CURRENT_DATE ORDER BY effective_date DESC LIMIT 1)
		FROM hrm_employees e WHERE id=$1::uuid AND org_id=$2::uuid`,
		employeeID, orgID,
	).Scan(&fromPosID, &fromDeptID, &fromBasicPay)

	p := &Promotion{
		OrgID: orgID, EmployeeID: employeeID,
		FromPositionID: fromPosID, FromDepartmentID: fromDeptID, FromBasicPay: fromBasicPay,
		ToPositionID: req.ToPositionID, ToDepartmentID: req.ToDepartmentID,
		ToSalaryStructureID: req.ToSalaryStructureID, NewBasicPay: req.NewBasicPay,
		EffectiveDate: req.EffectiveDate, Reason: req.Reason, Notes: req.Notes,
		Status: StatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, p); err != nil { return nil, fmt.Errorf("promotions: Create: %w", err) }
	return p, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, employeeID, ref string, req UpdatePromotionRequest) (*Promotion, error) {
	p, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("promotions: Update: %w", err) }
	if p == nil { return nil, ErrNotFound }
	if p.Status != StatusDraft { return nil, ErrWrongStatus }

	if req.ToPositionID != nil { p.ToPositionID = *req.ToPositionID }
	if req.ToDepartmentID != nil { p.ToDepartmentID = req.ToDepartmentID }
	if req.ToSalaryStructureID != nil { p.ToSalaryStructureID = req.ToSalaryStructureID }
	if req.NewBasicPay != nil { p.NewBasicPay = req.NewBasicPay }
	if req.EffectiveDate != nil {
		if _, err := time.Parse(dateLayout, *req.EffectiveDate); err != nil { return nil, ErrInvalidDate }
		p.EffectiveDate = *req.EffectiveDate
	}
	if req.Reason != nil { p.Reason = req.Reason }
	if req.Notes != nil { p.Notes = req.Notes }
	if req.DocumentID != nil { p.DocumentID = req.DocumentID }
	if err := s.repo.Update(ctx, p); err != nil { return nil, fmt.Errorf("promotions: Update: %w", err) }
	return p, nil
}

func (s *serviceImpl) Submit(ctx context.Context, orgID, employeeID, ref, submittedBy string) (*Promotion, error) {
	p, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("promotions: Submit: %w", err) }
	if p == nil { return nil, ErrNotFound }
	if p.Status != StatusDraft { return nil, ErrWrongStatus }

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypePromotion)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "promotion", EntityID: p.ID, RequestedBy: submittedBy,
		})
		if iErr != nil { return nil, fmt.Errorf("promotions: Submit: creating approval instance: %w", iErr) }
		if err := s.repo.SetApprovalInstance(ctx, p.ID, inst.ID, StatusPendingApproval); err != nil {
			return nil, fmt.Errorf("promotions: Submit: %w", err)
		}
		p.ApprovalInstanceID = &inst.ID
		p.Status = StatusPendingApproval
		return p, nil
	}

	// No approval template configured — unchanged fallback behavior
	if err := s.repo.UpdateStatus(ctx, p.ID, StatusApproved); err != nil {
		return nil, fmt.Errorf("promotions: Submit: %w", err)
	}
	p.Status = StatusApproved
	return p, nil
}

// HandleApprovalDecision reacts to the promotion's approval instance completing.
func (s *serviceImpl) HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	p, err := s.repo.FindByRef(ctx, orgID, "", entityID)
	if err != nil { return fmt.Errorf("promotions: HandleApprovalDecision: %w", err) }
	if p == nil { return ErrNotFound }
	if p.Status != StatusPendingApproval { return nil }
	status := StatusApproved
	if !approved { status = StatusRejected }
	if err := s.repo.UpdateStatus(ctx, p.ID, status); err != nil {
		return fmt.Errorf("promotions: HandleApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID, employeeID, ref string) (*Promotion, error) {
	p, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("promotions: Cancel: %w", err) }
	if p == nil { return nil, ErrNotFound }
	if p.Status == StatusApplied { return nil, ErrAlreadyApplied }
	if p.Status == StatusCancelled { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, p.ID, StatusCancelled); err != nil {
		return nil, fmt.Errorf("promotions: Cancel: %w", err)
	}
	p.Status = StatusCancelled
	return p, nil
}

// Apply executes three operations in one transaction:
//  1. Mark promotion record as applied
//  2. Update employee.position_id and optionally employee.department_id
//  3. Insert a new salary record if pay is changing (append-only pattern from A1)
func (s *serviceImpl) Apply(ctx context.Context, orgID, employeeID, ref, appliedBy string) (*Promotion, error) {
	p, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("promotions: Apply: %w", err) }
	if p == nil { return nil, ErrNotFound }
	if p.Status == StatusApplied { return nil, ErrAlreadyApplied }
	if p.Status != StatusApproved { return nil, ErrNotApproved }

	tx, err := s.db.Begin(ctx)
	if err != nil { return nil, fmt.Errorf("promotions: Apply: begin tx: %w", err) }
	defer tx.Rollback(ctx)

	// 1. Mark promotion applied
	if _, err = tx.Exec(ctx,
		`UPDATE hrm_promotions SET status='applied', applied_at=NOW(), applied_by=$1::uuid, updated_at=NOW() WHERE id=$2::uuid`,
		appliedBy, p.ID,
	); err != nil {
		return nil, fmt.Errorf("promotions: Apply: update promo: %w", err)
	}

	// 2. Update employee position and optionally department
	if _, err = tx.Exec(ctx,
		`UPDATE hrm_employees SET
		 position_id   = $1::uuid,
		 department_id = COALESCE($2::uuid, department_id),
		 updated_at    = NOW()
		WHERE id=$3::uuid AND org_id=$4::uuid`,
		p.ToPositionID, p.ToDepartmentID, p.EmployeeID, p.OrgID,
	); err != nil {
		return nil, fmt.Errorf("promotions: Apply: update employee: %w", err)
	}

	// 3. Create salary record if pay is changing (A1 pattern: append-only)
	if p.NewBasicPay != nil {
		if _, err = tx.Exec(ctx,
			`INSERT INTO hrm_employee_salary_records
			(org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, change_notes, created_by)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::date, 'promotion', 'Applied via promotion '||$6, $7::uuid)`,
			p.OrgID, p.EmployeeID, p.ToSalaryStructureID, *p.NewBasicPay,
			p.EffectiveDate, p.PublicID, appliedBy,
		); err != nil {
			return nil, fmt.Errorf("promotions: Apply: create salary record: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil { return nil, fmt.Errorf("promotions: Apply: commit: %w", err) }

	now := time.Now()
	p.Status = StatusApplied
	p.AppliedAt = &now
	p.AppliedBy = &appliedBy
	return p, nil
}
