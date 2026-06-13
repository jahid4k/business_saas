// backend/internal/task/service.go
package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
)

// Service defines the business logic interface for task operations.
type Service interface {
	List(ctx context.Context, orgID string, filter ListFilter) (*TaskListResponse, error)
	Get(ctx context.Context, orgID, taskRef string) (*Task, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateTaskRequest) (*Task, error)
	Update(ctx context.Context, orgID, taskRef string, req UpdateTaskRequest) (*Task, error)
	Delete(ctx context.Context, orgID, taskRef string) error
}

type serviceImpl struct {
	repo  Repository
	audit audit.Service
}

func NewService(repo Repository, auditSvc audit.Service) Service {
	return &serviceImpl{repo: repo, audit: auditSvc}
}

func (s *serviceImpl) List(ctx context.Context, orgID string, filter ListFilter) (*TaskListResponse, error) {
	filter.Normalise()

	tasks, err := s.repo.FindAll(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("task: List: %w", err)
	}
	if tasks == nil {
		tasks = []*Task{}
	}

	total, err := s.repo.Count(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("task: List: count: %w", err)
	}

	return &TaskListResponse{Tasks: tasks, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, taskRef string) (*Task, error) {
	t, err := s.repo.FindByRef(ctx, orgID, taskRef)
	if err != nil {
		return nil, fmt.Errorf("task: Get: %w", err)
	}
	if t == nil {
		return nil, ErrNotFound
	}
	return t, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateTaskRequest) (*Task, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	if len(title) > 255 {
		return nil, ErrTitleTooLong
	}

	description := strings.TrimSpace(req.Description)
	if len(description) > 2000 {
		return nil, ErrDescriptionTooLong
	}

	status := TaskStatus(strings.TrimSpace(req.Status))
	if status == "" {
		status = StatusTodo
	}
	if !status.IsValid() {
		return nil, ErrInvalidStatus
	}

	dueDate, err := parseDueDate(req.DueDate)
	if err != nil {
		return nil, err
	}

	t := &Task{
		OrgID:       orgID,
		Title:       title,
		Description: description,
		Status:      status,
		DueDate:     dueDate,
	}
	if createdBy != "" {
		t.CreatedBy = &createdBy
	}

	if req.AssignedTo != nil && strings.TrimSpace(*req.AssignedTo) != "" {
		assigneeID, err := s.repo.ResolveOrgMember(ctx, orgID, strings.TrimSpace(*req.AssignedTo))
		if err != nil {
			return nil, err
		}
		t.AssignedTo = &assigneeID
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("task: Create: %w", err)
	}

	s.audit.Log(ctx, audit.EventTaskCreated, createdBy, orgID, "", "", map[string]string{
		"task_id": t.ID, "title": t.Title,
	})

	return t, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, taskRef string, req UpdateTaskRequest) (*Task, error) {
	t, err := s.repo.FindByRef(ctx, orgID, taskRef)
	if err != nil {
		return nil, fmt.Errorf("task: Update: %w", err)
	}
	if t == nil {
		return nil, ErrNotFound
	}

	previousStatus := t.Status

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		if len(title) > 255 {
			return nil, ErrTitleTooLong
		}
		t.Title = title
	}

	if req.Description != nil {
		desc := strings.TrimSpace(*req.Description)
		if len(desc) > 2000 {
			return nil, ErrDescriptionTooLong
		}
		t.Description = desc
	}

	if req.Status != nil {
		status := TaskStatus(strings.TrimSpace(*req.Status))
		if !status.IsValid() {
			return nil, ErrInvalidStatus
		}
		t.Status = status
	}

	if req.DueDate != nil {
		dueDate, err := parseDueDate(req.DueDate)
		if err != nil {
			return nil, err
		}
		t.DueDate = dueDate // "" -> nil (cleared); RFC3339 string -> set
	}

	if req.AssignedTo != nil {
		ref := strings.TrimSpace(*req.AssignedTo)
		if ref == "" {
			t.AssignedTo = nil // unassign
		} else {
			assigneeID, err := s.repo.ResolveOrgMember(ctx, orgID, ref)
			if err != nil {
				return nil, err
			}
			t.AssignedTo = &assigneeID
		}
	}

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("task: Update: %w", err)
	}

	if t.Status != previousStatus {
		s.audit.Log(ctx, audit.EventTaskStatusChanged, "", orgID, "", "", map[string]string{
			"task_id": t.ID, "from": string(previousStatus), "to": string(t.Status),
		})
	}

	return t, nil
}

func (s *serviceImpl) Delete(ctx context.Context, orgID, taskRef string) error {
	t, err := s.repo.FindByRef(ctx, orgID, taskRef)
	if err != nil {
		return fmt.Errorf("task: Delete: %w", err)
	}
	if t == nil {
		return ErrNotFound
	}

	if err := s.repo.Delete(ctx, orgID, taskRef); err != nil {
		return fmt.Errorf("task: Delete: %w", err)
	}

	s.audit.Log(ctx, audit.EventTaskDeleted, "", orgID, "", "", map[string]string{
		"task_id": t.ID, "title": t.Title,
	})

	return nil
}

// parseDueDate converts an optional RFC3339 string into *time.Time.
//   - nil pointer            -> nil (no change / no due date)
//   - pointer to ""          -> nil (explicitly clear)
//   - pointer to valid RFC3339 -> parsed value
//   - anything else          -> ErrInvalidDueDate
func parseDueDate(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, ErrInvalidDueDate
	}
	return &t, nil
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

var (
	ErrNotFound           = errors.New("task not found")
	ErrTitleRequired      = errors.New("title is required")
	ErrTitleTooLong       = errors.New("title must not exceed 255 characters")
	ErrDescriptionTooLong = errors.New("description must not exceed 2000 characters")
	ErrInvalidStatus      = errors.New("status must be one of: todo, in_progress, done, cancelled")
	ErrInvalidDueDate     = errors.New("dueDate must be a valid RFC3339 timestamp")
	ErrAssigneeNotFound   = errors.New("assignedTo must be an active member of this organization")
)
