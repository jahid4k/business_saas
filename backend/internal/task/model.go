// Package task implements the test CRUD module for permission validation.
// It demonstrates how every BusinessSAAS module should be structured:
// handler → service → repository, all scoped to a business_id.
package task

import "time"

// TaskStatus defines the allowed values for a task's status field.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

// Task is the core domain type for the task module.
type Task struct {
	ID          string     `db:"id"          json:"id"`
	BusinessID  string     `db:"business_id" json:"business_id"`
	Title       string     `db:"title"       json:"title"`
	Description string     `db:"description" json:"description"`
	Status      TaskStatus `db:"status"      json:"status"`
	CreatedBy   string     `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time  `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"  json:"updated_at"`
}

// CreateTaskRequest is the request body for POST /api/v1/tasks.
// Requires permission: tasks.create
type CreateTaskRequest struct {
	Title       string `json:"title"       validate:"required,min=1,max=255"`
	Description string `json:"description" validate:"max=2000"`
	Status      string `json:"status"      validate:"omitempty,oneof=todo in_progress done"`
}

// UpdateTaskRequest is the request body for PATCH /api/v1/tasks/:id.
// Requires permission: tasks.update
type UpdateTaskRequest struct {
	Title       *string `json:"title"       validate:"omitempty,min=1,max=255"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Status      *string `json:"status"      validate:"omitempty,oneof=todo in_progress done"`
}

// TaskListResponse wraps a paginated list of tasks.
type TaskListResponse struct {
	Tasks []*Task `json:"tasks"`
	Total int     `json:"total"`
}
