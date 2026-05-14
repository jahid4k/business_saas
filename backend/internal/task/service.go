package task

import (
	"context"
	"errors"
	"fmt"
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

func (s *serviceImpl) List(_ context.Context, _ string) (*TaskListResponse, error) {
	return nil, errNotImplemented("List")
}

func (s *serviceImpl) GetByID(_ context.Context, _, _ string) (*Task, error) {
	return nil, errNotImplemented("GetByID")
}

func (s *serviceImpl) Create(_ context.Context, _, _ string, _ CreateTaskRequest) (*Task, error) {
	return nil, errNotImplemented("Create")
}

func (s *serviceImpl) Update(_ context.Context, _, _ string, _ UpdateTaskRequest) (*Task, error) {
	return nil, errNotImplemented("Update")
}

func (s *serviceImpl) Delete(_ context.Context, _, _ string) error {
	return errNotImplemented("Delete")
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

// ErrNotFound is returned when a task does not exist in the given business.
var ErrNotFound = errors.New("task not found")

func errNotImplemented(method string) error {
	return fmt.Errorf("task: %s: not yet implemented", method)
}
