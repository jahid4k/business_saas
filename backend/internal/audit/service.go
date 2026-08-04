// Package audit provides append-only audit logging for security-sensitive events.
//
// Events are written to the audit_logs table. This table is append-only:
// no records are ever updated or deleted, even if the referenced user or
// business is deleted.
//
// All event writes are best-effort and non-blocking — an audit write failure
// must never cause the originating request to fail.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// EventType classifies the audit event.
type EventType string

const (
	EventSignup               EventType = "auth.signup"
	EventLogin                EventType = "auth.login"
	EventLoginFailed          EventType = "auth.login_failed"
	EventLogout               EventType = "auth.logout"
	EventLogoutAll            EventType = "auth.logout_all"
	EventPasswordResetRequest EventType = "auth.password_reset_request"
	EventPasswordResetConfirm EventType = "auth.password_reset_confirm"
	EventBusinessCreated      EventType = "business.created"
	EventRoleAssigned         EventType = "authz.role_assigned"
	EventTaskCreated          EventType = "task.created"
	EventTaskStatusChanged    EventType = "task.status_changed"
	EventTaskDeleted          EventType = "task.deleted"

	// HRM events
	EventHRMEmployeeCreated      EventType = "hrm.employee.created"
	EventHRMEmployeeTerminated   EventType = "hrm.employee.terminated"
	EventHRMLeaveRequested       EventType = "hrm.leave.requested"
	EventHRMLeaveApproved        EventType = "hrm.leave.approved"
	EventHRMLeaveRejected        EventType = "hrm.leave.rejected"
	EventHRMLeaveBalanceAdjusted EventType = "hrm.leave.balance_adjusted"
	EventHRMLeaveEncashed        EventType = "hrm.leave.encashed"
	EventMemberPasswordReset     EventType = "authz.member_password_reset"
)

// Event represents a single audit log entry.
type Event struct {
	ID         string          `db:"id"`
	UserID     *string         `db:"user_id"`     // nil for anonymous events
	BusinessID *string         `db:"business_id"` // nil for pre-business events
	EventType  EventType       `db:"event_type"`
	Metadata   json.RawMessage `db:"metadata"` // arbitrary event data (JSONB)
	IPAddress  string          `db:"ip_address"`
	UserAgent  string          `db:"user_agent"`
	CreatedAt  time.Time       `db:"created_at"`
}

// Service defines the audit logging interface.
type Service interface {
	// Log writes an audit event. It is non-blocking and best-effort.
	// Errors are logged server-side but never returned to the caller.
	Log(ctx context.Context, event EventType, userID, businessID, ip, ua string, metadata any)
}

// Repository defines the data access interface for audit log writes.
type Repository interface {
	Insert(ctx context.Context, e *Event) error
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new audit service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

// Log writes an audit event asynchronously.
// The goroutine is fire-and-forget — the caller is never blocked.
func (s *serviceImpl) Log(ctx context.Context, eventType EventType, userID, businessID, ip, ua string, metadata any) {
	go func() {
		// Use a new background context — the request context may be cancelled
		// by the time the goroutine runs.
		bgCtx := context.Background()

		meta, err := json.Marshal(metadata)
		if err != nil {
			slog.Error("audit: failed to marshal metadata",
				slog.String("event", string(eventType)),
				slog.Any("error", err),
			)
			return
		}

		e := &Event{
			EventType: eventType,
			Metadata:  meta,
			IPAddress: ip,
			UserAgent: ua,
		}

		if userID != "" {
			e.UserID = &userID
		}
		if businessID != "" {
			e.BusinessID = &businessID
		}

		if err := s.repo.Insert(bgCtx, e); err != nil {
			// Log the failure but do NOT propagate — audit write failures
			// must never fail the originating request.
			slog.Error("audit: failed to write event",
				slog.String("event", string(eventType)),
				slog.Any("error", err),
			)
		}

		_ = ctx // suppress unused warning; bgCtx is intentional
	}()
}

// noopRepo is used as a placeholder until migrations and a real repo exist.
