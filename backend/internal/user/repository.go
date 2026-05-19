// backend/internal/user/repository.go
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxFailedLogins = 5
const lockDuration = 15 * time.Minute

// Repository defines the data access interface for user operations.
type Repository interface {
	FindByID(ctx context.Context, userID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	UpdateSettings(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error)
	RecordFailedLogin(ctx context.Context, userID string) error
	RecordSuccessfulLogin(ctx context.Context, userID string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const userSelectColumns = `
	id, public_id,
	COALESCE(email, ''), COALESCE(password_hash, ''), COALESCE(username, ''),
	COALESCE(display_name, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(full_name, ''),
	COALESCE(photo_url, ''), COALESCE(cover_photo_url, ''),
	COALESCE(phone, ''), phone_verified,
	email_verified, email_verified_at,
	COALESCE(country, ''), timezone, locale, language, currency,
	status, account_type,
	suspended_at, COALESCE(suspension_reason, ''),
	login_redirect_url, shortcuts,
	settings, preferences, onboarding, feature_flags,
	two_fa_enabled,
	last_login_at, last_activity_at,
	failed_logins, locked_until,
	created_at, updated_at, deleted_at`

func scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	err := row.Scan(
		&u.ID, &u.PublicID,
		&u.Email, &u.PasswordHash, &u.Username,
		&u.DisplayName, &u.FirstName, &u.LastName, &u.FullName,
		&u.PhotoURL, &u.CoverPhotoURL,
		&u.Phone, &u.PhoneVerified,
		&u.EmailVerified, &u.EmailVerifiedAt,
		&u.Country, &u.Timezone, &u.Locale, &u.Language, &u.Currency,
		&u.Status, &u.AccountType,
		&u.SuspendedAt, &u.SuspensionReason,
		&u.LoginRedirectURL, &u.Shortcuts,
		&u.Settings, &u.Preferences, &u.Onboarding, &u.FeatureFlags,
		&u.TwoFAEnabled,
		&u.LastLoginAt, &u.LastActivityAt,
		&u.FailedLogins, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *repoImpl) FindByID(ctx context.Context, userID string) (*User, error) {
	q := `SELECT ` + userSelectColumns + ` FROM users WHERE id = $1 AND deleted_at IS NULL`
	u, err := scanUser(r.db.QueryRow(ctx, q, userID))
	if err != nil {
		return nil, fmt.Errorf("user: FindByID: %w", err)
	}
	return u, nil
}

func (r *repoImpl) FindByEmail(ctx context.Context, email string) (*User, error) {
	q := `SELECT ` + userSelectColumns + ` FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`
	u, err := scanUser(r.db.QueryRow(ctx, q, strings.ToLower(strings.TrimSpace(email))))
	if err != nil {
		return nil, fmt.Errorf("user: FindByEmail: %w", err)
	}
	return u, nil
}

func (r *repoImpl) Create(ctx context.Context, u *User) error {
	u.NormaliseForCreate()
	const q = `
		INSERT INTO users (
			email, password_hash, username, display_name, first_name, last_name, full_name,
			photo_url, cover_photo_url, phone, phone_verified,
			email_verified, email_verified_at, country, timezone, locale, language, currency,
			status, account_type, login_redirect_url, shortcuts,
			settings, preferences, onboarding, feature_flags
		)
		VALUES (
			NULLIF($1, ''), NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6, $7,
			NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), $11,
			$12, $13, NULLIF($14, ''), $15, $16, $17, $18,
			$19, $20, $21, $22,
			$23, $24, $25, $26
		)
		RETURNING id, public_id, created_at, updated_at`

	err := r.db.QueryRow(ctx, q,
		u.Email, u.PasswordHash, u.Username, u.DisplayName, u.FirstName, u.LastName, u.FullName,
		u.PhotoURL, u.CoverPhotoURL, u.Phone, u.PhoneVerified,
		u.EmailVerified, u.EmailVerifiedAt, u.Country, u.Timezone, u.Locale, u.Language, u.Currency,
		u.Status, u.AccountType, u.LoginRedirectURL, u.Shortcuts,
		u.Settings, u.Preferences, u.Onboarding, u.FeatureFlags,
	).Scan(&u.ID, &u.PublicID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("user: Create: %w", err)
	}
	return nil
}

func (r *repoImpl) Update(ctx context.Context, u *User) error {
	u.NormaliseForCreate()
	const q = `
		UPDATE users
		SET password_hash      = NULLIF($1, ''),
		    username           = NULLIF($2, ''),
		    display_name       = $3,
		    first_name         = $4,
		    last_name          = $5,
		    full_name          = $6,
		    photo_url          = NULLIF($7, ''),
		    cover_photo_url    = NULLIF($8, ''),
		    phone              = NULLIF($9, ''),
		    phone_verified     = $10,
		    email_verified     = $11,
		    email_verified_at  = $12,
		    country            = NULLIF($13, ''),
		    timezone           = $14,
		    locale             = $15,
		    language           = $16,
		    currency           = $17,
		    status             = $18,
		    account_type       = $19,
		    login_redirect_url = $20,
		    shortcuts          = $21,
		    settings           = $22,
		    preferences        = $23,
		    onboarding         = $24,
		    feature_flags      = $25,
		    failed_logins      = $26,
		    locked_until       = $27,
		    updated_at         = NOW()
		WHERE id = $28 AND deleted_at IS NULL
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		u.PasswordHash, u.Username, u.DisplayName, u.FirstName, u.LastName, u.FullName,
		u.PhotoURL, u.CoverPhotoURL, u.Phone, u.PhoneVerified,
		u.EmailVerified, u.EmailVerifiedAt, u.Country, u.Timezone, u.Locale, u.Language, u.Currency,
		u.Status, u.AccountType, u.LoginRedirectURL, u.Shortcuts,
		u.Settings, u.Preferences, u.Onboarding, u.FeatureFlags,
		u.FailedLogins, u.LockedUntil, u.ID,
	).Scan(&u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("user: Update: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateSettings(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error) {
	existing, err := r.FindByID(ctx, userID)
	if err != nil || existing == nil {
		return existing, err
	}
	if req.Username != "" {
		existing.Username = strings.TrimSpace(req.Username)
	}
	if req.DisplayName != "" {
		existing.DisplayName = strings.TrimSpace(req.DisplayName)
	}
	if req.FirstName != "" {
		existing.FirstName = strings.TrimSpace(req.FirstName)
	}
	if req.LastName != "" {
		existing.LastName = strings.TrimSpace(req.LastName)
	}
	if req.PhotoURL != "" {
		existing.PhotoURL = strings.TrimSpace(req.PhotoURL)
	}
	if req.CoverPhotoURL != "" {
		existing.CoverPhotoURL = strings.TrimSpace(req.CoverPhotoURL)
	}
	if req.Phone != "" {
		existing.Phone = strings.TrimSpace(req.Phone)
	}
	if req.Country != "" {
		existing.Country = strings.TrimSpace(req.Country)
	}
	if req.Timezone != "" {
		existing.Timezone = strings.TrimSpace(req.Timezone)
	}
	if req.Locale != "" {
		existing.Locale = strings.TrimSpace(req.Locale)
	}
	if req.Language != "" {
		existing.Language = strings.TrimSpace(req.Language)
	}
	if req.Currency != "" {
		existing.Currency = strings.TrimSpace(req.Currency)
	}
	if req.LoginRedirectURL != "" {
		existing.LoginRedirectURL = strings.TrimSpace(req.LoginRedirectURL)
	}
	if req.Shortcuts != nil {
		existing.Shortcuts = req.Shortcuts
	}
	if len(req.Settings) > 0 {
		existing.Settings = req.Settings
	}
	if len(req.Preferences) > 0 {
		existing.Preferences = req.Preferences
	}
	if len(req.Onboarding) > 0 {
		existing.Onboarding = req.Onboarding
	}
	if len(req.FeatureFlags) > 0 {
		existing.FeatureFlags = req.FeatureFlags
	}
	if err := r.Update(ctx, existing); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, userID)
}

func (r *repoImpl) RecordFailedLogin(ctx context.Context, userID string) error {
	const q = `
		UPDATE users
		SET failed_logins = failed_logins + 1,
		    locked_until = CASE
		        WHEN failed_logins + 1 >= $1 THEN NOW() + $2::INTERVAL
		        ELSE locked_until
		    END,
		    updated_at = NOW()
		WHERE id = $3`
	_, err := r.db.Exec(ctx, q, maxFailedLogins, fmt.Sprintf("%d minutes", int(lockDuration.Minutes())), userID)
	if err != nil {
		return fmt.Errorf("user: RecordFailedLogin: %w", err)
	}
	return nil
}

func (r *repoImpl) RecordSuccessfulLogin(ctx context.Context, userID string) error {
	const q = `
		UPDATE users
		SET failed_logins = 0,
		    locked_until = NULL,
		    last_login_at = NOW(),
		    last_activity_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("user: RecordSuccessfulLogin: %w", err)
	}
	return nil
}
