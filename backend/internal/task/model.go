// backend/internal/task/model.go
package task

import "time"

// TaskStatus defines the allowed values for a task's status field.
// Must match the CHECK constraint on tasks.status (migration 00013).
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusCancelled  TaskStatus = "cancelled"
)

// IsValid returns true when the status value is one of the allowed enum values.
func (s TaskStatus) IsValid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone, StatusCancelled:
		return true
	}
	return false
}

// SortField whitelists the columns List can order by.
type SortField string

const (
	SortByCreatedAt SortField = "created_at"
	SortByUpdatedAt SortField = "updated_at"
	SortByDueDate   SortField = "due_date"
	SortByTitle     SortField = "title"
	SortByStatus    SortField = "status"
)

func (f SortField) IsValid() bool {
	switch f {
	case SortByCreatedAt, SortByUpdatedAt, SortByDueDate, SortByTitle, SortByStatus:
		return true
	}
	return false
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Task is the core domain type for the task module.
// Every field maps directly to the tasks table column of the same name.
type Task struct {
	ID          string     `db:"id"          json:"id"`
	PublicID    string     `db:"public_id"   json:"publicId"`
	OrgID       string     `db:"org_id"      json:"organizationId"`
	Title       string     `db:"title"       json:"title"`
	Description string     `db:"description" json:"description"`
	Status      TaskStatus `db:"status"      json:"status"`
	DueDate     *time.Time `db:"due_date"    json:"dueDate,omitempty"`
	CreatedBy   *string    `db:"created_by"  json:"createdBy,omitempty"`
	AssignedTo  *string    `db:"assigned_to" json:"assignedTo,omitempty"`
	RelatedType *string    `db:"related_type" json:"relatedType,omitempty"`
	RelatedID   *string    `db:"related_id"   json:"relatedId,omitempty"`
	CreatedAt   time.Time  `db:"created_at"  json:"createdAt"`
	UpdatedAt   time.Time  `db:"updated_at"  json:"updatedAt"`
}

// ListFilter narrows List/Count to a subset of an organization's tasks.
// Zero values mean "no filter on this field".
type ListFilter struct {
	Status         TaskStatus
	AssignedTo     string
	InvolvedUserID string // Checks assigned_to = X OR created_by = X
	RelatedType    string
	RelatedID      string
	SortBy         SortField
	SortDesc       bool
	Limit          int
	Offset         int
}

// Normalise clamps pagination and applies defaults. Called by the service so
// the repository never has to guard against zero/negative/huge values.
func (f *ListFilter) Normalise() {
	if !f.SortBy.IsValid() {
		f.SortBy = SortByCreatedAt
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// CreateTaskRequest is the body for POST /organizations/:orgId/tasks.
type CreateTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`      // optional — defaults to "todo"
	DueDate     *string `json:"dueDate"`     // optional — RFC3339, e.g. "2026-07-01T00:00:00Z"
	AssignedTo  *string `json:"assignedTo"`  // optional — user id, public id, or email; must be an active org member
	RelatedType *string `json:"relatedType"` // optional
	RelatedID   *string `json:"relatedId"`   // optional
}

// UpdateTaskRequest is the body for PATCH /organizations/:orgId/tasks/:taskId.
// All fields are optional — only non-nil fields are applied.
//
// AssignedTo: nil = leave unchanged, "" = unassign, non-empty = reassign (must be an org member).
// DueDate:    nil = leave unchanged, "" = clear the due date, non-empty = set (RFC3339).
type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	DueDate     *string `json:"dueDate"`
	AssignedTo  *string `json:"assignedTo"`
	RelatedType *string `json:"relatedType"`
	RelatedID   *string `json:"relatedId"`
}

// TaskListResponse wraps the list response with pagination metadata.
type TaskListResponse struct {
	Tasks  []*Task `json:"tasks"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}
