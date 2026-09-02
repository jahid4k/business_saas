package notifications

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetPreferences(ctx context.Context, userID uuid.UUID, eventType string) (map[string]bool, error)
	LogNotification(ctx context.Context, n *Notification) error
	UpdateNotificationStatus(ctx context.Context, id uuid.UUID, status string, errMsg *string) error

	FindByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Notification, int, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, userID, notifID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	GetAllPreferences(ctx context.Context, userID uuid.UUID) ([]*NotificationPreference, error)
	UpsertPreference(ctx context.Context, userID uuid.UUID, eventType, channel string, enabled bool) error
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) GetPreferences(ctx context.Context, userID uuid.UUID, eventType string) (map[string]bool, error) {
	query := `
		SELECT channel, is_enabled 
		FROM platform_notification_preferences 
		WHERE user_id = $1 AND event_type = $2
	`
	rows, err := r.db.Query(ctx, query, userID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefs := make(map[string]bool)
	for rows.Next() {
		var channel string
		var isEnabled bool
		if err := rows.Scan(&channel, &isEnabled); err != nil {
			return nil, err
		}
		prefs[channel] = isEnabled
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prefs, nil
}

func (r *repository) LogNotification(ctx context.Context, n *Notification) error {
	query := `
		INSERT INTO platform_notifications (
			org_id, user_id, event_type, channel, title, body, action_url, metadata, status, error_message, sent_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		n.OrgID, n.UserID, n.EventType, n.Channel, n.Title, n.Body, n.ActionURL, n.Metadata, n.Status, n.ErrorMessage, n.SentAt,
	).Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	return nil
}

func (r *repository) UpdateNotificationStatus(ctx context.Context, id uuid.UUID, status string, errMsg *string) error {
	query := `
		UPDATE platform_notifications
		SET status = $1, error_message = $2, sent_at = CASE WHEN $1 = 'sent' THEN NOW() ELSE sent_at END
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, status, errMsg, id)
	return err
}

func (r *repository) FindByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Notification, int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, org_id, user_id, event_type, channel, title, body, action_url, metadata,
			status, error_message, read_at, sent_at, created_at
		FROM platform_notifications
		WHERE user_id = $1 AND channel = 'in_app'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*Notification, 0)
	for rows.Next() {
		n := &Notification{}
		if err := rows.Scan(
			&n.ID, &n.OrgID, &n.UserID, &n.EventType, &n.Channel, &n.Title, &n.Body, &n.ActionURL, &n.Metadata,
			&n.Status, &n.ErrorMessage, &n.ReadAt, &n.SentAt, &n.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_notifications WHERE user_id = $1 AND channel = 'in_app'`,
		userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *repository) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_notifications WHERE user_id = $1 AND channel = 'in_app' AND read_at IS NULL`,
		userID,
	).Scan(&count)
	return count, err
}

func (r *repository) MarkRead(ctx context.Context, userID, notifID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE platform_notifications SET read_at = NOW() WHERE id = $1 AND user_id = $2 AND read_at IS NULL`,
		notifID, userID)
	return err
}

func (r *repository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE platform_notifications SET read_at = NOW() WHERE user_id = $1 AND channel = 'in_app' AND read_at IS NULL`,
		userID)
	return err
}

func (r *repository) GetAllPreferences(ctx context.Context, userID uuid.UUID) ([]*NotificationPreference, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, event_type, channel, is_enabled, updated_at
		FROM platform_notification_preferences
		WHERE user_id = $1
		ORDER BY event_type, channel`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*NotificationPreference, 0)
	for rows.Next() {
		p := &NotificationPreference{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.EventType, &p.Channel, &p.IsEnabled, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repository) UpsertPreference(ctx context.Context, userID uuid.UUID, eventType, channel string, enabled bool) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO platform_notification_preferences (user_id, event_type, channel, is_enabled)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, event_type, channel) DO UPDATE SET is_enabled = EXCLUDED.is_enabled, updated_at = NOW()`,
		userID, eventType, channel, enabled)
	return err
}
