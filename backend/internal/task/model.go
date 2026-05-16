// backend/internal/task/model.go
package task

import "time"

// TaskStatus defines the allowed values for a task's status field.
// Must match the task_status enum in migration 00006.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

// IsValid returns true when the status value is one of the allowed enum values.
func (s TaskStatus) IsValid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

// Task is the core domain type for the task module.
// Every field maps directly to the tasks table column of the same name.
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

// CreateTaskRequest is the body for POST /api/v1/tasks.
type CreateTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // optional — defaults to "todo"
}

// UpdateTaskRequest is the body for PATCH /api/v1/tasks/:id.
// All fields are optional — only non-nil fields are applied.
type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

// TaskListResponse wraps the list response with a total count.
type TaskListResponse struct {
	Tasks []*Task `json:"tasks"`
	Total int     `json:"total"`
}
