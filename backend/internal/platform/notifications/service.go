package notifications

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/google/uuid"
	"github.com/resend/resend-go/v2"
	"github.com/mridha/businesssaas/internal/config"
)

//go:embed templates/*.html
var templatesFS embed.FS
var tmpl *template.Template

func init() {
	var err error
	tmpl, err = template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		panic(fmt.Sprintf("notifications: failed to parse templates: %v", err))
	}
}

type Service interface {
	Dispatch(ctx context.Context, req DispatchRequest) error

	// ListInApp returns a user's in-app notifications, paginated, with an unread count.
	ListInApp(ctx context.Context, userID uuid.UUID, limit, offset int) (*NotificationListResponse, error)
	MarkRead(ctx context.Context, userID, notifID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	ListPreferences(ctx context.Context, userID uuid.UUID) ([]*NotificationPreference, error)
	UpdatePreference(ctx context.Context, userID uuid.UUID, eventType, channel string, enabled bool) error
}

type serviceImpl struct {
	repo         Repository
	resendClient *resend.Client
	fromEmail    string
}

func NewService(cfg config.NotificationsConfig, repo Repository) Service {
	return &serviceImpl{
		repo:         repo,
		resendClient: resend.NewClient(cfg.ResendAPIKey),
		fromEmail:    cfg.FromEmail,
	}
}

func (s *serviceImpl) Dispatch(ctx context.Context, req DispatchRequest) error {
	// 1. Get User Preferences for this EventType
	prefs, err := s.repo.GetPreferences(ctx, req.UserID, req.EventType)
	if err != nil {
		slog.Error("notifications: failed to get preferences", "err", err, "user_id", req.UserID)
		// We proceed anyway, assuming default is enabled
	}

	channelsToUse := req.Channels
	if len(channelsToUse) == 0 {
		// By default, attempt email and in-app
		channelsToUse = []string{ChannelEmail, ChannelInApp}
	}

	for _, ch := range channelsToUse {
		// If user explicitly disabled this channel for this event, skip it.
		// (Missing entry implies enabled by default in our model).
		if enabled, exists := prefs[ch]; exists && !enabled {
			continue
		}

		// Log it in DB first as pending
		notif := &Notification{
			OrgID:     req.OrgID,
			UserID:    req.UserID,
			EventType: req.EventType,
			Channel:   ch,
			Title:     req.Title,
			Body:      req.Body,
			ActionURL: req.ActionURL,
			Metadata:  req.Metadata,
			Status:    StatusPending,
		}

		if err := s.repo.LogNotification(ctx, notif); err != nil {
			slog.Error("notifications: failed to log notification", "err", err)
			continue
		}

		// Process delivery
		var deliveryErr error
		switch ch {
		case ChannelEmail:
			if req.UserEmail != "" {
				deliveryErr = s.sendEmail(ctx, req.UserEmail, req)
			} else {
				deliveryErr = fmt.Errorf("email address not provided")
			}
		case ChannelInApp:
			// In-app is "delivered" just by being in the database
			deliveryErr = nil
		default:
			deliveryErr = fmt.Errorf("unsupported channel: %s", ch)
		}

		// Update status
		status := StatusSent
		var errMsg *string
		if deliveryErr != nil {
			status = StatusFailed
			msg := deliveryErr.Error()
			errMsg = &msg
		}

		if updateErr := s.repo.UpdateNotificationStatus(ctx, notif.ID, status, errMsg); updateErr != nil {
			slog.Error("notifications: failed to update status", "err", updateErr, "id", notif.ID)
		}
	}

	return nil
}

func (s *serviceImpl) sendEmail(ctx context.Context, to string, req DispatchRequest) error {
	var htmlBody string
	if req.TemplateName != "" {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, req.TemplateName, req.TemplateData); err != nil {
			return fmt.Errorf("failed to render template: %w", err)
		}
		htmlBody = buf.String()
	} else {
		// Fallback to plain body
		htmlBody = req.Body
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: req.Title,
		Html:    htmlBody,
	}

	// We only send if we have an API key. 
	// If it's missing (e.g. dev environment without one), we just log it.
	if s.resendClient.ApiKey == "" {
		slog.Info("notifications: would send email (no API key)", "to", to, "subject", req.Title)
		return nil
	}

	_, err := s.resendClient.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}

	return nil
}

func (s *serviceImpl) ListInApp(ctx context.Context, userID uuid.UUID, limit, offset int) (*NotificationListResponse, error) {
	list, total, err := s.repo.FindByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("notifications: ListInApp: %w", err)
	}
	unread, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("notifications: ListInApp: count unread: %w", err)
	}
	return &NotificationListResponse{Notifications: list, Total: total, UnreadCount: unread}, nil
}

func (s *serviceImpl) MarkRead(ctx context.Context, userID, notifID uuid.UUID) error {
	return s.repo.MarkRead(ctx, userID, notifID)
}

func (s *serviceImpl) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *serviceImpl) ListPreferences(ctx context.Context, userID uuid.UUID) ([]*NotificationPreference, error) {
	list, err := s.repo.GetAllPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("notifications: ListPreferences: %w", err)
	}
	return list, nil
}

func (s *serviceImpl) UpdatePreference(ctx context.Context, userID uuid.UUID, eventType, channel string, enabled bool) error {
	return s.repo.UpsertPreference(ctx, userID, eventType, channel, enabled)
}
