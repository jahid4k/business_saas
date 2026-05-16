// backend/internal/task/service.go
package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Service defines the business logic interface for task operations.
type Service interface {
	List(ctx context.Context, businessID string) (*TaskListResponse, error)
	GetByID(ctx context.Context, businessID, taskID string) (*Task, error)
	Create(ctx context.Context, businessID, createdBy string, req CreateTaskRequest) (*Task, error)
	Update(ctx context.Context, businessID, taskID string, req UpdateTaskRequest) (*Task, error)
	Delete(ctx context.Context, businessID, taskID string) error
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new task service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

// ----------------------------------------------------------
// List
// ----------------------------------------------------------

// List returns all tasks for the business.
// Always returns an empty slice (not nil) so JSON encodes as [] not null.
func (s *serviceImpl) List(ctx context.Context, businessID string) (*TaskListResponse, error) {
	tasks, err := s.repo.FindAll(ctx, businessID)
	if err != nil {
		return nil, fmt.Errorf("task: List: %w", err)
	}

	// Never return nil slice — JSON must encode as [] not null
	if tasks == nil {
		tasks = []*Task{}
	}

	total, err := s.repo.Count(ctx, businessID)
	if err != nil {
		return nil, fmt.Errorf("task: List: count: %w", err)
	}

	return &TaskListResponse{Tasks: tasks, Total: total}, nil
}

// ----------------------------------------------------------
// GetByID
// ----------------------------------------------------------

// GetByID returns a task only if it belongs to the given business.
// Returns ErrNotFound when the task does not exist OR belongs to another business.
// The caller cannot tell the difference — this is intentional.
func (s *serviceImpl) GetByID(ctx context.Context, businessID, taskID string) (*Task, error) {
	t, err := s.repo.FindByID(ctx, businessID, taskID)
	if err != nil {
		return nil, fmt.Errorf("task: GetByID: %w", err)
	}
	if t == nil {
		return nil, ErrNotFound
	}
	return t, nil
}

// ----------------------------------------------------------
// Create
// ----------------------------------------------------------

// Create validates the request and inserts a new task.
func (s *serviceImpl) Create(ctx context.Context, businessID, createdBy string, req CreateTaskRequest) (*Task, error) {
	// Validate
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

	// Default status to "todo" if not provided
	status := TaskStatus(req.Status)
	if status == "" {
		status = StatusTodo
	}
	if !status.IsValid() {
		return nil, ErrInvalidStatus
	}

	t := &Task{
		BusinessID:  businessID,
		Title:       title,
		Description: description,
		Status:      status,
		CreatedBy:   createdBy,
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("task: Create: %w", err)
	}

	return t, nil
}

// ----------------------------------------------------------
// Update
// ----------------------------------------------------------

// Update applies partial updates to an existing task.
// Only non-nil fields in the request are changed.
// Verifies tenant ownership before updating.
func (s *serviceImpl) Update(ctx context.Context, businessID, taskID string, req UpdateTaskRequest) (*Task, error) {
	// Load existing — also enforces tenant isolation
	t, err := s.repo.FindByID(ctx, businessID, taskID)
	if err != nil {
		return nil, fmt.Errorf("task: Update: %w", err)
	}
	if t == nil {
		return nil, ErrNotFound
	}

	// Apply only the fields that were sent
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
		status := TaskStatus(*req.Status)
		if !status.IsValid() {
			return nil, ErrInvalidStatus
		}
		t.Status = status
	}

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("task: Update: %w", err)
	}

	return t, nil
}

// ----------------------------------------------------------
// Delete
// ----------------------------------------------------------

// Delete removes a task after verifying it belongs to the business.
// Returns ErrNotFound when the task does not exist or belongs to another business.
func (s *serviceImpl) Delete(ctx context.Context, businessID, taskID string) error {
	// FindByID first — enforces tenant isolation
	t, err := s.repo.FindByID(ctx, businessID, taskID)
	if err != nil {
		return fmt.Errorf("task: Delete: %w", err)
	}
	if t == nil {
		return ErrNotFound
	}

	if err := s.repo.Delete(ctx, businessID, taskID); err != nil {
		return fmt.Errorf("task: Delete: %w", err)
	}

	return nil
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

var ErrNotFound = errors.New("task not found")
var ErrTitleRequired = errors.New("title is required")
var ErrTitleTooLong = errors.New("title must not exceed 255 characters")
var ErrDescriptionTooLong = errors.New("description must not exceed 2000 characters")
var ErrInvalidStatus = errors.New("status must be one of: todo, in_progress, done")
