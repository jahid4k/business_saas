// backend/internal/hrm/resignations/service.go
package resignations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

// Service defines business logic for employee resignations.
type Service interface {
	List(ctx context.Context, orgID string, filter ResignationListFilter) (*ResignationListResponse, error)
	Get(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error)
	Submit(ctx context.Context, orgID, employeeID, createdBy string, req SubmitResignationRequest) (*Resignation, error)
	Update(ctx context.Context, orgID, employeeID, ref string, req UpdateResignationRequest) (*Resignation, error)
	Withdraw(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error)
	// Accept marks resigned and updates employee status in one transaction.
	Accept(ctx context.Context, orgID, employeeID, ref, acceptedBy string) (*Resignation, error)
	Reject(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service {
	return &serviceImpl{repo: repo, db: db}
}

func (s *serviceImpl) List(ctx context.Context, orgID string, filter ResignationListFilter) (*ResignationListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindAll(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("resignations: List: %w", err)
	}
	if list == nil {
		list = []*Resignation{}
	}
	total, err := s.repo.Count(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("resignations: List: count: %w", err)
	}
	return &ResignationListResponse{Resignations: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error) {
	r, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("resignations: Get: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *serviceImpl) Submit(ctx context.Context, orgID, employeeID, createdBy string, req SubmitResignationRequest) (*Resignation, error) {
	if strings.TrimSpace(req.ResignationDate) == "" {
		return nil, ErrResignationDateReq
	}
	resDate, err := time.Parse(dateLayout, req.ResignationDate)
	if err != nil {
		return nil, ErrInvalidDate
	}
	if !req.ReasonCategory.IsValid() {
		req.ReasonCategory = ReasonOther
	}

	// Only one active resignation per employee
	existing, err := s.repo.FindActiveByEmployee(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("resignations: Submit: check active: %w", err)
	}
	if existing != nil {
		return nil, ErrAlreadyActive
	}

	// Look up notice period from active contract; default to 30 days
	noticeDays := 30
	var contractNoticeDays *int
	_ = s.db.QueryRow(ctx,
		`SELECT notice_period_days FROM hrm_employee_contracts WHERE employee_id=$1::uuid AND is_active=TRUE LIMIT 1`,
		employeeID).Scan(&contractNoticeDays)
	if contractNoticeDays != nil {
		noticeDays = *contractNoticeDays
	}

	// Compute last working date
	lwd := resDate.AddDate(0, 0, noticeDays).Format(dateLayout)
	if req.LastWorkingDate != nil && strings.TrimSpace(*req.LastWorkingDate) != "" {
		if _, err := time.Parse(dateLayout, *req.LastWorkingDate); err != nil {
			return nil, ErrInvalidDate
		}
		lwd = *req.LastWorkingDate
	}

	r := &Resignation{
		OrgID: orgID, EmployeeID: employeeID,
		ResignationDate:  req.ResignationDate,
		NoticePeriodDays: noticeDays,
		IsNoticeWaived:   req.IsNoticeWaived,
		LastWorkingDate:  lwd,
		ReasonCategory:   req.ReasonCategory,
		ReasonRemarks:    req.ReasonRemarks,
		Status:           StatusSubmitted,
		CreatedBy:        createdBy,
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("resignations: Submit: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, employeeID, ref string, req UpdateResignationRequest) (*Resignation, error) {
	r, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("resignations: Update: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status == StatusWithdrawn || r.Status == StatusRejected {
		return nil, ErrWrongStatus
	}
	if req.ExitInterviewCompleted != nil {
		r.ExitInterviewCompleted = *req.ExitInterviewCompleted
	}
	if req.ExitClearanceCompleted != nil {
		r.ExitClearanceCompleted = *req.ExitClearanceCompleted
	}
	if req.DocumentID != nil {
		r.DocumentID = req.DocumentID
	}
	if req.LastWorkingDate != nil {
		if _, err := time.Parse(dateLayout, *req.LastWorkingDate); err != nil {
			return nil, ErrInvalidDate
		}
		r.LastWorkingDate = *req.LastWorkingDate
	}
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, fmt.Errorf("resignations: Update: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) Withdraw(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error) {
	r, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("resignations: Withdraw: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status != StatusSubmitted {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusWithdrawn); err != nil {
		return nil, fmt.Errorf("resignations: Withdraw: %w", err)
	}
	r.Status = StatusWithdrawn
	return r, nil
}

// Accept marks the resignation accepted AND updates employee status to 'resigned'
// with termination_date = last_working_date. Both operations are atomic.
func (s *serviceImpl) Accept(ctx context.Context, orgID, employeeID, ref, acceptedBy string) (*Resignation, error) {
	r, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("resignations: Accept: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status == StatusAccepted {
		return nil, ErrAlreadyAccepted
	}
	if r.Status != StatusSubmitted {
		return nil, ErrWrongStatus
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("resignations: Accept: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx,
		`UPDATE hrm_resignations SET status='accepted', accepted_at=NOW(), accepted_by=$1::uuid, updated_at=NOW() WHERE id=$2::uuid`,
		acceptedBy, r.ID,
	); err != nil {
		return nil, fmt.Errorf("resignations: Accept: update resignation: %w", err)
	}

	// Resolve the org's resigned status, inside the same transaction so a
	// missing status aborts the whole Accept rather than leaving the
	// resignation accepted against an unchanged employee.
	//
	// There is no 'resigned' status CATEGORY — the category CHECK allows only
	// active/inactive/on_leave/terminated. Migration 00053 models resignation
	// as a distinct status row NAMED 'Resigned' inside the 'terminated'
	// category, so the name match is what distinguishes it from 'Terminated'.
	// Falling back to the oldest terminated-category row keeps an org that
	// renamed its statuses working rather than failing outright.
	var statusID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM hrm_employee_statuses
		 WHERE org_id = $1::uuid AND category = 'terminated'
		 ORDER BY (LOWER(name) = 'resigned') DESC, created_at ASC
		 LIMIT 1`,
		r.OrgID,
	).Scan(&statusID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoResignedStatus
	}
	if err != nil {
		return nil, fmt.Errorf("resignations: Accept: resolve resigned status: %w", err)
	}

	// Record the last_working_date as termination_date. Employee remains
	// payroll-active until that date.
	//
	// This targets status_id, not a status text column: migration 00053
	// replaced hrm_employees.status with a status_id FK. This method wrote
	// the dropped column until that was caught, which made every Accept fail
	// with SQLSTATE 42703 and roll back the whole transaction.
	if _, err = tx.Exec(ctx,
		`UPDATE hrm_employees SET status_id=$1::uuid, termination_date=$2::date, updated_at=NOW()
		WHERE id=$3::uuid AND org_id=$4::uuid`,
		statusID, r.LastWorkingDate, r.EmployeeID, r.OrgID,
	); err != nil {
		return nil, fmt.Errorf("resignations: Accept: update employee: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("resignations: Accept: commit: %w", err)
	}

	now := time.Now()
	r.Status = StatusAccepted
	r.AcceptedAt = &now
	r.AcceptedBy = &acceptedBy
	return r, nil
}

func (s *serviceImpl) Reject(ctx context.Context, orgID, employeeID, ref string) (*Resignation, error) {
	r, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("resignations: Reject: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if r.Status != StatusSubmitted {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, r.ID, StatusRejected); err != nil {
		return nil, fmt.Errorf("resignations: Reject: %w", err)
	}
	r.Status = StatusRejected
	return r, nil
}
