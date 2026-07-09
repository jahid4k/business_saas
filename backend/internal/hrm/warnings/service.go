// backend/internal/hrm/warnings/service.go
package warnings

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

// Service defines business logic for employee warning records.
type Service interface {
	List(ctx context.Context, orgID, employeeID, status string, activeOnly bool) (*WarningListResponse, error)
	Get(ctx context.Context, orgID, employeeID, ref string) (*EmployeeWarning, error)
	Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateWarningRequest) (*EmployeeWarning, error)
	Update(ctx context.Context, orgID, employeeID, ref string, req UpdateWarningRequest) (*EmployeeWarning, error)
	// Issue formally sends the warning to the employee.
	// Snapshots warning type config, sets response deadline, checks escalation rules.
	Issue(ctx context.Context, orgID, employeeID, ref, issuedBy string, req IssueRequest) (*EmployeeWarning, error)
	// Acknowledge is the employee confirming receipt.
	Acknowledge(ctx context.Context, orgID, employeeID, ref string, req AcknowledgeRequest) (*EmployeeWarning, error)
	// Appeal is the employee contesting the warning.
	Appeal(ctx context.Context, orgID, employeeID, ref string, req AppealRequest) (*EmployeeWarning, error)
	// Close is HR closing an issued/acknowledged/appealed warning.
	Close(ctx context.Context, orgID, employeeID, ref string, req CloseRequest) (*EmployeeWarning, error)
	Cancel(ctx context.Context, orgID, employeeID, ref string) (*EmployeeWarning, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service {
	return &serviceImpl{repo: repo, db: db}
}

func (s *serviceImpl) List(ctx context.Context, orgID, employeeID, status string, activeOnly bool) (*WarningListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, employeeID, status, activeOnly)
	if err != nil { return nil, fmt.Errorf("warnings: List: %w", err) }
	if list == nil { list = []*EmployeeWarning{} }
	return &WarningListResponse{Warnings: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, employeeID, ref string) (*EmployeeWarning, error) {
	w, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("warnings: Get: %w", err) }
	if w == nil { return nil, ErrNotFound }
	return w, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateWarningRequest) (*EmployeeWarning, error) {
	if strings.TrimSpace(req.WarningTypeID) == "" { return nil, ErrWarningTypeIDRequired }
	if strings.TrimSpace(req.Title) == "" { return nil, ErrTitleRequired }
	if strings.TrimSpace(req.Description) == "" { return nil, ErrDescriptionRequired }
	if strings.TrimSpace(req.IncidentDate) == "" { return nil, ErrIncidentDateRequired }
	if _, err := time.Parse(dateLayout, req.IncidentDate); err != nil { return nil, ErrInvalidDate }

	// Look up warning type config to snapshot
	var typeName string
	var severityLevel int
	var canRespond bool
	var responseWindowDays int
	err := s.db.QueryRow(ctx,
		`SELECT name, severity_level, employee_can_respond, response_window_days
		FROM hrm_warning_types WHERE id=$1::uuid AND org_id=$2::uuid AND is_active=TRUE`,
		req.WarningTypeID, orgID,
	).Scan(&typeName, &severityLevel, &canRespond, &responseWindowDays)
	if err != nil { return nil, ErrWarningTypeNotFound }

	witnessIDs := req.WitnessIDs
	if witnessIDs == nil { witnessIDs = []string{} }

	w := &EmployeeWarning{
		OrgID: orgID, EmployeeID: employeeID,
		WarningTypeID: req.WarningTypeID, WarningTypeName: typeName,
		SeverityLevel: severityLevel,
		Title: req.Title, Description: req.Description,
		IncidentDate: req.IncidentDate,
		IssuedBy: createdBy, WitnessIDs: witnessIDs,
		CanEmployeeRespond: canRespond, ResponseWindowDays: responseWindowDays,
		IsActive: false, // not active until formally issued
		Status: StatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, w); err != nil { return nil, fmt.Errorf("warnings: Create: %w", err) }
	return w, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, employeeID, ref string, req UpdateWarningRequest) (*EmployeeWarning, error) {
	w, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("warnings: Update: %w", err) }
	if w == nil { return nil, ErrNotFound }
	if w.Status != StatusDraft { return nil, ErrWrongStatus }
	if req.Title != nil { w.Title = *req.Title }
	if req.Description != nil { w.Description = *req.Description }
	if req.IncidentDate != nil {
		if _, err := time.Parse(dateLayout, *req.IncidentDate); err != nil { return nil, ErrInvalidDate }
		w.IncidentDate = *req.IncidentDate
	}
	if req.WitnessIDs != nil { w.WitnessIDs = req.WitnessIDs }
	if req.DocumentID != nil { w.DocumentID = req.DocumentID }
	if err := s.repo.Update(ctx, w); err != nil { return nil, fmt.Errorf("warnings: Update: %w", err) }
	return w, nil
}

// Issue formally issues a draft warning. Sets is_active=TRUE, issued_at=NOW(),
// computes response_deadline, and checks escalation rules.
func (s *serviceImpl) Issue(ctx context.Context, orgID, employeeID, ref, issuedBy string, req IssueRequest) (*EmployeeWarning, error) {
	w, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("warnings: Issue: %w", err) }
	if w == nil { return nil, ErrNotFound }
	if w.Status == StatusIssued || w.Status == StatusAcknowledged { return nil, ErrAlreadyIssued }
	if w.Status != StatusDraft && w.Status != StatusPendingApproval { return nil, ErrWrongStatus }

	now := time.Now()
	w.IssuedAt = &now
	w.IsActive = true
	w.Status = StatusIssued
	if req.DocumentID != nil { w.DocumentID = req.DocumentID }

	// Compute response deadline
	if w.CanEmployeeRespond && w.ResponseWindowDays > 0 {
		deadline := now.AddDate(0, 0, w.ResponseWindowDays).Format(dateLayout)
		w.ResponseDeadline = &deadline
	}

	// Compute expiry (from warning type's valid_duration_days)
	var validDurationDays int
	_ = s.db.QueryRow(ctx, `SELECT valid_duration_days FROM hrm_warning_types WHERE id=$1::uuid`, w.WarningTypeID).Scan(&validDurationDays)
	if validDurationDays > 0 {
		exp := now.AddDate(0, 0, validDurationDays).Format(dateLayout)
		w.ExpiresAt = &exp
	}

	if err := s.repo.Update(ctx, w); err != nil { return nil, fmt.Errorf("warnings: Issue: update: %w", err) }
	if err := s.repo.UpdateStatus(ctx, w.ID, StatusIssued); err != nil { return nil, fmt.Errorf("warnings: Issue: status: %w", err) }

	// Check escalation rules (log-only — never auto-create)
	s.checkEscalation(ctx, orgID, employeeID, w.WarningTypeID)

	return w, nil
}

// checkEscalation evaluates A3 escalation rules. Logs a warning but takes no hard action.
// The system flags HR — never auto-creates a new warning (ADR-0014 design decision).
func (s *serviceImpl) checkEscalation(ctx context.Context, orgID, employeeID, warningTypeID string) {
	// Load escalation rules for this warning type
	rows, err := s.db.Query(ctx,
		`SELECT trigger_count, within_days, action FROM hrm_warning_escalation_rules
		WHERE org_id=$1::uuid AND trigger_warning_type_id=$2::uuid AND is_active=TRUE`,
		orgID, warningTypeID)
	if err != nil { return }
	defer rows.Close()
	for rows.Next() {
		var triggerCount, withinDays int
		var action string
		if err := rows.Scan(&triggerCount, &withinDays, &action); err != nil { continue }
		count, err := s.repo.CountActiveByTypeAndEmployee(ctx, orgID, employeeID, warningTypeID, withinDays)
		if err != nil { continue }
		if count >= triggerCount {
			// In production: send a notification (email/webhook) per action type.
			// For now: structured log. Notification system is Group E scope.
			_ = action // "notify_hr" | "notify_management" | "flag_termination_review"
		}
	}
}

func (s *serviceImpl) Acknowledge(ctx context.Context, orgID, employeeID, ref string, req AcknowledgeRequest) (*EmployeeWarning, error) {
	w, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("warnings: Acknowledge: %w", err) }
	if w == nil { return nil, ErrNotFound }
	if w.Status != StatusIssued { return nil, ErrWrongStatus }

	now := time.Now()
	w.EmployeeRespondedAt = &now
	w.EmployeeResponse = req.Response
	w.Status = StatusAcknowledged
	if err := s.repo.Update(ctx, w); err != nil { return nil, fmt.Errorf("warnings: Acknowledge: update: %w", err) }
	if err := s.repo.UpdateStatus(ctx, w.ID, StatusAcknowledged); err != nil { return nil, fmt.Errorf("warnings: Acknowledge: status: %w", err) }
	return w, nil
}

func (s *serviceImpl) Appeal(ctx context.Context, orgID, employeeID, ref string, req AppealRequest) (*EmployeeWarning, error) {
	w, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("warnings: Appeal: %w", err) }
	if w == nil { return nil, ErrNotFound }
	if !w.CanEmployeeRespond { return nil, ErrCannotAppeal }
	if w.Status != StatusIssued && w.Status != StatusAcknowledged { return nil, ErrWrongStatus }
	if strings.TrimSpace(req.Reason) == "" { return nil, ErrDescriptionRequired }

	now := time.Now()
	w.AppealReason = &req.Reason
	w.AppealSubmittedAt = &now
	w.Status = StatusAppealed
	if err := s.repo.Update(ctx, w); err != nil { return nil, fmt.Errorf("warnings: Appeal: update: %w", err) }
	if err := s.repo.UpdateStatus(ctx, w.ID, StatusAppealed); err != nil { return nil, fmt.Errorf("warnings: Appeal: status: %w", err) }
	return w, nil
}

func (s *serviceImpl) Close(ctx context.Context, orgID, employeeID, ref string, req CloseRequest) (*EmployeeWarning, error) {
	w, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("warnings: Close: %w", err) }
	if w == nil { return nil, ErrNotFound }
	if w.Status == StatusClosed || w.Status == StatusCancelled || w.Status == StatusDraft { return nil, ErrWrongStatus }

	if w.Status == StatusAppealed {
		now := time.Now()
		w.AppealResolution = req.AppealResolution
		w.AppealResolvedAt = &now
	}
	w.Status = StatusClosed
	if err := s.repo.Update(ctx, w); err != nil { return nil, fmt.Errorf("warnings: Close: update: %w", err) }
	if err := s.repo.UpdateStatus(ctx, w.ID, StatusClosed); err != nil { return nil, fmt.Errorf("warnings: Close: status: %w", err) }
	return w, nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID, employeeID, ref string) (*EmployeeWarning, error) {
	w, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("warnings: Cancel: %w", err) }
	if w == nil { return nil, ErrNotFound }
	if w.Status == StatusClosed || w.Status == StatusCancelled { return nil, ErrWrongStatus }
	w.IsActive = false
	w.Status = StatusCancelled
	if err := s.repo.Update(ctx, w); err != nil { return nil, fmt.Errorf("warnings: Cancel: update: %w", err) }
	if err := s.repo.UpdateStatus(ctx, w.ID, StatusCancelled); err != nil { return nil, fmt.Errorf("warnings: Cancel: status: %w", err) }
	return w, nil
}
