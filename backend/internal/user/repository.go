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

const (
	maxFailedLogins = 5
	lockDuration    = 15 * time.Minute
)

// Repository defines the data access interface for user operations.
type Repository interface {
	FindByID(ctx context.Context, userID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	UpdatePassword(ctx context.Context, id string, passwordHash string) error
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
			$19, $20, $21, COALESCE($22, ARRAY[]::TEXT[]),
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
		    shortcuts          = COALESCE($21, ARRAY[]::TEXT[]),
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

// UpdateSettings applies a partial profile update using a targeted SQL UPDATE,
// avoiding the read-modify-write race and the 3-query pattern of the old implementation.
// FIX: replaces the old FindByID → Update → FindByID pattern (3 round trips, stale-read risk).
func (r *repoImpl) UpdateSettings(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error) {
	const q = `
		UPDATE users
		SET username           = CASE WHEN $1 <> '' THEN $1           ELSE username           END,
		    display_name       = CASE WHEN $2 <> '' THEN $2           ELSE display_name       END,
		    first_name         = CASE WHEN $3 <> '' THEN $3           ELSE first_name         END,
		    last_name          = CASE WHEN $4 <> '' THEN $4           ELSE last_name          END,
		    photo_url          = CASE WHEN $5 <> '' THEN $5           ELSE photo_url          END,
		    cover_photo_url    = CASE WHEN $6 <> '' THEN $6           ELSE cover_photo_url    END,
		    phone              = CASE WHEN $7 <> '' THEN $7           ELSE phone              END,
		    country            = CASE WHEN $8 <> '' THEN $8           ELSE country            END,
		    timezone           = CASE WHEN $9 <> '' THEN $9           ELSE timezone           END,
		    locale             = CASE WHEN $10 <> '' THEN $10         ELSE locale             END,
		    language           = CASE WHEN $11 <> '' THEN $11         ELSE language           END,
		    currency           = CASE WHEN $12 <> '' THEN $12         ELSE currency           END,
		    login_redirect_url = CASE WHEN $13 <> '' THEN $13         ELSE login_redirect_url END,
		    shortcuts          = CASE WHEN $14::TEXT[] IS NOT NULL THEN $14 ELSE shortcuts    END,
		    settings           = CASE WHEN $15::JSONB IS NOT NULL THEN $15  ELSE settings     END,
		    preferences        = CASE WHEN $16::JSONB IS NOT NULL THEN $16  ELSE preferences  END,
		    onboarding         = CASE WHEN $17::JSONB IS NOT NULL THEN $17  ELSE onboarding   END,
		    feature_flags      = CASE WHEN $18::JSONB IS NOT NULL THEN $18  ELSE feature_flags END,
		    updated_at         = NOW()
		WHERE id = $19 AND deleted_at IS NULL
		RETURNING ` + userSelectColumns

	u, err := scanUser(r.db.QueryRow(ctx, q,
		strings.TrimSpace(req.Username),
		strings.TrimSpace(req.DisplayName),
		strings.TrimSpace(req.FirstName),
		strings.TrimSpace(req.LastName),
		strings.TrimSpace(req.PhotoURL),
		strings.TrimSpace(req.CoverPhotoURL),
		strings.TrimSpace(req.Phone),
		strings.TrimSpace(req.Country),
		strings.TrimSpace(req.Timezone),
		strings.TrimSpace(req.Locale),
		strings.TrimSpace(req.Language),
		strings.TrimSpace(req.Currency),
		strings.TrimSpace(req.LoginRedirectURL),
		req.Shortcuts,
		req.Settings,
		req.Preferences,
		req.Onboarding,
		req.FeatureFlags,
		userID,
	))
	if err != nil {
		return nil, fmt.Errorf("user: UpdateSettings: %w", err)
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
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
		SET failed_logins    = 0,
		    locked_until     = NULL,
		    last_login_at    = NOW(),
		    last_activity_at = NOW(),
		    updated_at       = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("user: RecordSuccessfulLogin: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	const q = `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, q, passwordHash, id)
	return err
}
