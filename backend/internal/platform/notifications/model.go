package notifications

import (
	"time"

	"github.com/google/uuid"
)

// Channel types
const (
	ChannelEmail = "email"
	ChannelInApp = "in_app"
	ChannelPush  = "push"
)

// Event types
const (
	EventPasswordReset = "auth.password_reset"
	EventUserInvite    = "auth.user_invite"
)

// Statuses
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// NotificationPreference represents a user's opt-in/opt-out status for an event type on a channel
type NotificationPreference struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	EventType string    `json:"event_type" db:"event_type"`
	Channel   string    `json:"channel" db:"channel"`
	IsEnabled bool      `json:"is_enabled" db:"is_enabled"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Notification represents a dispatched notification in the database (primarily used for in-app tracking)
type Notification struct {
	ID           uuid.UUID `json:"id" db:"id"`
	OrgID        *uuid.UUID `json:"org_id" db:"org_id"`
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	EventType    string    `json:"event_type" db:"event_type"`
	Channel      string    `json:"channel" db:"channel"`
	Title        string    `json:"title" db:"title"`
	Body         string    `json:"body" db:"body"`
	ActionURL    *string   `json:"action_url" db:"action_url"`
	Metadata     *string   `json:"metadata" db:"metadata"` // JSON string
	Status       string    `json:"status" db:"status"`
	ErrorMessage *string   `json:"error_message" db:"error_message"`
	ReadAt       *time.Time `json:"read_at" db:"read_at"`
	SentAt       *time.Time `json:"sent_at" db:"sent_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// NotificationListResponse is the paginated in-app notification list for a user.
type NotificationListResponse struct {
	Notifications []*Notification `json:"notifications"`
	Total         int             `json:"total"`
	UnreadCount   int             `json:"unread_count"`
}

// DispatchRequest is the payload sent to the notification service to trigger a message
type DispatchRequest struct {
	OrgID        *uuid.UUID
	UserID       uuid.UUID
	UserEmail    string // Used as fallback if we don't look up user in notifications service
	EventType    string
	Channels     []string // Empty = all enabled channels
	Title        string
	Body         string
	ActionURL    *string
	Metadata     *string
	
	// Email-specific (for templates)
	TemplateName string
	TemplateData interface{}
}
