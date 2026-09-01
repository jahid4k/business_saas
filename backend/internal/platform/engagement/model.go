// backend/internal/platform/engagement/model.go
package engagement

import (
	"errors"
	"time"
)

// RelatedType defines the valid values for the polymorphic related_type column.
// Every current and future module registers its entity types here.
// Format: "<module>.<entity>" for module-specific entities,
//
//	"platform.<entity>" for cross-module platform entities.
type RelatedType string

const (
	// Platform entities
	RelatedContact RelatedType = "platform.contact"
	RelatedCompany RelatedType = "platform.company"

	// CRM entities
	RelatedCRMLead RelatedType = "crm.lead"
	RelatedCRMDeal RelatedType = "crm.deal"

	// Future — registered here to make the full picture visible
	// RelatedERPOrder    RelatedType = "erp.purchase_order"
	// RelatedHRMEmployee RelatedType = "hrm.employee"
)

func (r RelatedType) IsValid() bool {
	switch r {
	case RelatedContact, RelatedCompany, RelatedCRMLead, RelatedCRMDeal:
		return true
	}
	return false
}

// ============================================================
// Note
// ============================================================

// Note is a shared text annotation that can be attached to any entity
// across any module. The module field records which module created it,
// allowing filtered views per module while supporting a unified timeline.
type Note struct {
	ID          string      `db:"id"           json:"id"`
	PublicID    string      `db:"public_id"    json:"public_id"`
	OrgID       string      `db:"org_id"       json:"org_id"`
	Module      string      `db:"module"       json:"module"`
	Content     string      `db:"content"      json:"content"`
	RelatedType RelatedType `db:"related_type" json:"related_type,omitempty"`
	RelatedID   string      `db:"related_id"   json:"related_id,omitempty"`
	// Nullable because a SYSTEM-generated note has no human author. Every
	// internal/capture/* path creates leads with no acting user, and
	// leads.CreateLead's duplicate-capture path writes a note with that same
	// empty actor — created_by was NOT NULL, so those inserts failed and the
	// error was discarded by the caller. See migration 00120; the identical
	// fix for crm_leads.created_by is 00112.
	CreatedBy *string   `db:"created_by"   json:"created_by,omitempty"`
	CreatedAt time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt time.Time `db:"updated_at"   json:"updated_at"`
}

type CreateNoteRequest struct {
	Content     string `json:"content"`
	RelatedType string `json:"related_type"`
	RelatedID   string `json:"related_id"`
}

type UpdateNoteRequest struct {
	Content *string `json:"content"`
}

type NoteListResponse struct {
	Notes []*Note `json:"notes"`
	Total int     `json:"total"`
}

// ============================================================
// Task
// ============================================================

// TaskStatus defines the allowed values for task status.
type TaskStatus string

const (
	TaskStatusOpen      TaskStatus = "open"
	TaskStatusCompleted TaskStatus = "completed"
)

// TaskPriority defines the allowed values for task priority.
type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
)

func (p TaskPriority) IsValid() bool {
	switch p {
	case TaskPriorityLow, TaskPriorityMedium, TaskPriorityHigh:
		return true
	}
	return false
}

// Task is a shared work item that can be linked to any entity across any module.
type Task struct {
	ID          string       `db:"id"           json:"id"`
	PublicID    string       `db:"public_id"    json:"public_id"`
	OrgID       string       `db:"org_id"       json:"org_id"`
	Module      string       `db:"module"       json:"module"`
	Title       string       `db:"title"        json:"title"`
	Description *string      `db:"description"  json:"description,omitempty"`
	DueDate     *time.Time   `db:"due_date"     json:"due_date,omitempty"`
	Status      TaskStatus   `db:"status"       json:"status"`
	Priority    TaskPriority `db:"priority"     json:"priority"`
	RelatedType RelatedType  `db:"related_type" json:"related_type,omitempty"`
	RelatedID   string       `db:"related_id"   json:"related_id,omitempty"`
	AssignedTo  *string      `db:"assigned_to"  json:"assigned_to,omitempty"`
	CreatedBy   string       `db:"created_by"   json:"created_by"`
	CompletedAt *time.Time   `db:"completed_at" json:"completed_at,omitempty"`
	CreatedAt   time.Time    `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"   json:"updated_at"`
}

type CreateTaskRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	DueDate     *string `json:"due_date"`
	Priority    *string `json:"priority"`
	RelatedType string  `json:"related_type"`
	RelatedID   string  `json:"related_id"`
	AssignedTo  *string `json:"assigned_to"`
}

type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	DueDate     *string `json:"due_date"`
	Priority    *string `json:"priority"`
	AssignedTo  *string `json:"assigned_to"`
}

type AssignTaskRequest struct {
	AssignedTo string `json:"assigned_to"`
}

type TaskListResponse struct {
	Tasks []*Task `json:"tasks"`
	Total int     `json:"total"`
}

// ============================================================
// Activity
// ============================================================

// ActivityType defines the allowed types for activities.
type ActivityType string

const (
	ActivityTypeCall    ActivityType = "call"
	ActivityTypeEmail   ActivityType = "email"
	ActivityTypeMeeting ActivityType = "meeting"
	ActivityTypeNote    ActivityType = "note"
	ActivityTypeTask    ActivityType = "task"
	ActivityTypeOther   ActivityType = "other"
)

func (t ActivityType) IsValid() bool {
	switch t {
	case ActivityTypeCall, ActivityTypeEmail, ActivityTypeMeeting,
		ActivityTypeNote, ActivityTypeTask, ActivityTypeOther:
		return true
	}
	return false
}

// Activity is a shared event record (call, meeting, email, etc.) that can be
// linked to any entity across any module.
type Activity struct {
	ID           string       `db:"id"            json:"id"`
	PublicID     string       `db:"public_id"     json:"public_id"`
	OrgID        string       `db:"org_id"        json:"org_id"`
	Module       string       `db:"module"        json:"module"`
	Type         ActivityType `db:"type"          json:"type"`
	Subject      string       `db:"subject"       json:"subject"`
	Description  *string      `db:"description"   json:"description,omitempty"`
	Outcome      *string      `db:"outcome"       json:"outcome,omitempty"`
	RelatedType  RelatedType  `db:"related_type"  json:"related_type,omitempty"`
	RelatedID    string       `db:"related_id"    json:"related_id,omitempty"`
	OccurredAt   time.Time    `db:"occurred_at"   json:"occurred_at"`
	DurationMins *int         `db:"duration_mins" json:"duration_mins,omitempty"`
	CreatedBy    string       `db:"created_by"    json:"created_by"`
	CreatedAt    time.Time    `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at"    json:"updated_at"`
}

type CreateActivityRequest struct {
	Type         string  `json:"type"`
	Subject      string  `json:"subject"`
	Description  *string `json:"description"`
	Outcome      *string `json:"outcome"`
	RelatedType  string  `json:"related_type"`
	RelatedID    string  `json:"related_id"`
	OccurredAt   *string `json:"occurred_at"`
	DurationMins *int    `json:"duration_mins"`
}

type UpdateActivityRequest struct {
	Type         *string `json:"type"`
	Subject      *string `json:"subject"`
	Description  *string `json:"description"`
	Outcome      *string `json:"outcome"`
	OccurredAt   *string `json:"occurred_at"`
	DurationMins *int    `json:"duration_mins"`
}

type ActivityListResponse struct {
	Activities []*Activity `json:"activities"`
	Total      int         `json:"total"`
}

// ============================================================
// EmailLog
// ============================================================

// EmailLog records inbound and outbound emails linked to any entity.
type EmailLog struct {
	ID          string      `db:"id"           json:"id"`
	PublicID    string      `db:"public_id"    json:"public_id"`
	OrgID       string      `db:"org_id"       json:"org_id"`
	Module      string      `db:"module"       json:"module"`
	Subject     string      `db:"subject"      json:"subject"`
	Body        *string     `db:"body"         json:"body,omitempty"`
	FromEmail   string      `db:"from_email"   json:"from_email"`
	ToEmail     string      `db:"to_email"     json:"to_email"`
	Direction   string      `db:"direction"    json:"direction"`
	Status      string      `db:"status"       json:"status"`
	RelatedType RelatedType `db:"related_type" json:"related_type,omitempty"`
	RelatedID   string      `db:"related_id"   json:"related_id,omitempty"`
	SentAt      *time.Time  `db:"sent_at"      json:"sent_at,omitempty"`
	CreatedBy   string      `db:"created_by"   json:"created_by"`
	CreatedAt   time.Time   `db:"created_at"   json:"created_at"`
}

type CreateEmailLogRequest struct {
	Subject     string  `json:"subject"`
	Body        *string `json:"body"`
	FromEmail   string  `json:"from_email"`
	ToEmail     string  `json:"to_email"`
	Direction   *string `json:"direction"`
	Status      *string `json:"status"`
	RelatedType string  `json:"related_type"`
	RelatedID   string  `json:"related_id"`
}

type EmailLogListResponse struct {
	EmailLogs []*EmailLog `json:"email_logs"`
	Total     int         `json:"total"`
}

// ============================================================
// Timeline — unified view across all engagement types
// ============================================================

// TimelineEntry is a single entry in a unified timeline response.
// The Type field tells the client which concrete type to render.
type TimelineEntry struct {
	Type      string      `json:"type"` // "note" | "task" | "activity" | "email"
	ID        string      `json:"id"`
	Module    string      `json:"module"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// TimelineResponse is the unified timeline for any entity.
type TimelineResponse struct {
	Entries []*TimelineEntry `json:"entries"`
	Total   int              `json:"total"`
}

// ============================================================
// Sentinel errors
// ============================================================

var (
	ErrNoteNotFound        = errors.New("note not found")
	ErrTaskNotFound        = errors.New("task not found")
	ErrActivityNotFound    = errors.New("activity not found")
	ErrEmailLogNotFound    = errors.New("email log not found")
	ErrContentRequired     = errors.New("content is required")
	ErrTitleRequired       = errors.New("title is required")
	ErrSubjectRequired     = errors.New("subject is required")
	ErrTypeRequired        = errors.New("type is required")
	ErrFromEmailRequired   = errors.New("from_email is required")
	ErrToEmailRequired     = errors.New("to_email is required")
	ErrInvalidPriority     = errors.New("invalid priority value")
	ErrInvalidActivityType = errors.New("invalid activity type")
	ErrRelatedTypeRequired = errors.New("related_type is required")
	ErrRelatedIDRequired   = errors.New("related_id is required")
	ErrInvalidRelatedType  = errors.New("invalid related_type value")
)
