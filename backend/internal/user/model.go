// backend/internal/user/model.go
package user

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	StatusActive              = "active"
	StatusSuspended           = "suspended"
	StatusDeleted             = "deleted"
	StatusPendingVerification = "pending_verification"
)

// User maps to the updated SaaS users table.
// NEVER expose PasswordHash to API clients.
type User struct {
	ID               string          `db:"id"`
	PublicID         string          `db:"public_id"`
	Email            string          `db:"email"`
	PasswordHash     string          `db:"password_hash"`
	Username         string          `db:"username"`
	DisplayName      string          `db:"display_name"`
	FirstName        string          `db:"first_name"`
	LastName         string          `db:"last_name"`
	FullName         string          `db:"full_name"`
	PhotoURL         string          `db:"photo_url"`
	CoverPhotoURL    string          `db:"cover_photo_url"`
	Phone            string          `db:"phone"`
	PhoneVerified    bool            `db:"phone_verified"`
	EmailVerified    bool            `db:"email_verified"`
	EmailVerifiedAt  *time.Time      `db:"email_verified_at"`
	Country          string          `db:"country"`
	Timezone         string          `db:"timezone"`
	Locale           string          `db:"locale"`
	Language         string          `db:"language"`
	Currency         string          `db:"currency"`
	Status           string          `db:"status"`
	AccountType      string          `db:"account_type"`
	SuspendedAt      *time.Time      `db:"suspended_at"`
	SuspensionReason string          `db:"suspension_reason"`
	LoginRedirectURL string          `db:"login_redirect_url"`
	Shortcuts        []string        `db:"shortcuts"`
	Settings         json.RawMessage `db:"settings"`
	Preferences      json.RawMessage `db:"preferences"`
	Onboarding       json.RawMessage `db:"onboarding"`
	FeatureFlags     json.RawMessage `db:"feature_flags"`
	TwoFAEnabled     bool            `db:"two_fa_enabled"`
	LastLoginAt      *time.Time      `db:"last_login_at"`
	LastActivityAt   *time.Time      `db:"last_activity_at"`
	FailedLogins     int             `db:"failed_logins"`
	LockedUntil      *time.Time      `db:"locked_until"`
	CreatedAt        time.Time       `db:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"`
	DeletedAt        *time.Time      `db:"deleted_at"`
}

func (u *User) NormaliseForCreate() {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	u.Username = strings.TrimSpace(u.Username)
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)
	u.DisplayName = strings.TrimSpace(u.DisplayName)
	if u.DisplayName == "" {
		u.DisplayName = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}
	if u.FullName == "" {
		u.FullName = u.DisplayName
	}
	if u.Status == "" {
		u.Status = StatusActive
	}
	if u.AccountType == "" {
		u.AccountType = "saas_customer"
	}
	if u.Timezone == "" {
		u.Timezone = "UTC"
	}
	if u.Locale == "" {
		u.Locale = "en"
	}
	if u.Language == "" {
		u.Language = "en"
	}
	if u.Currency == "" {
		u.Currency = "USD"
	}
	if u.LoginRedirectURL == "" {
		u.LoginRedirectURL = "/dashboard"
	}
	if u.Shortcuts == nil {
		u.Shortcuts = []string{}
	}
	if len(u.Settings) == 0 {
		u.Settings = json.RawMessage(`{}`)
	}
	if len(u.Preferences) == 0 {
		u.Preferences = json.RawMessage(`{}`)
	}
	if len(u.Onboarding) == 0 {
		u.Onboarding = json.RawMessage(`{}`)
	}
	if len(u.FeatureFlags) == 0 {
		u.FeatureFlags = json.RawMessage(`{}`)
	}
}

// IsLocked returns true if the account is currently locked out.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && time.Now().Before(*u.LockedUntil)
}

func (u *User) IsLoginAllowed() bool {
	return u.Status == StatusActive && u.DeletedAt == nil && !u.IsLocked()
}

// SafeUser is the public-safe representation expected by frontend/Auth.js.
type SafeUser struct {
	ID               string          `json:"id"`
	PublicID         string          `json:"publicId,omitempty"`
	Email            string          `json:"email,omitempty"`
	Username         string          `json:"username,omitempty"`
	DisplayName      string          `json:"displayName"`
	FirstName        string          `json:"firstName,omitempty"`
	LastName         string          `json:"lastName,omitempty"`
	FullName         string          `json:"fullName,omitempty"`
	PhotoURL         string          `json:"photoURL,omitempty"`
	CoverPhotoURL    string          `json:"coverPhotoURL,omitempty"`
	Phone            string          `json:"phone,omitempty"`
	PhoneVerified    bool            `json:"phoneVerified"`
	EmailVerified    bool            `json:"emailVerified"`
	EmailVerifiedAt  *time.Time      `json:"emailVerifiedAt,omitempty"`
	Country          string          `json:"country,omitempty"`
	Timezone         string          `json:"timezone"`
	Locale           string          `json:"locale"`
	Language         string          `json:"language"`
	Currency         string          `json:"currency"`
	Status           string          `json:"status"`
	AccountType      string          `json:"accountType"`
	LoginRedirectURL string          `json:"loginRedirectUrl"`
	Shortcuts        []string        `json:"shortcuts"`
	Settings         json.RawMessage `json:"settings"`
	Preferences      json.RawMessage `json:"preferences"`
	Onboarding       json.RawMessage `json:"onboarding"`
	FeatureFlags     json.RawMessage `json:"featureFlags"`
	TwoFAEnabled     bool            `json:"twoFactorEnabled"`
	LastLoginAt      *time.Time      `json:"lastLoginAt,omitempty"`
	LastActivityAt   *time.Time      `json:"lastActivityAt,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// ToSafe converts User to API-safe representation.
func (u *User) ToSafe() *SafeUser {
	settings := u.Settings
	if len(settings) == 0 {
		settings = json.RawMessage(`{}`)
	}
	preferences := u.Preferences
	if len(preferences) == 0 {
		preferences = json.RawMessage(`{}`)
	}
	onboarding := u.Onboarding
	if len(onboarding) == 0 {
		onboarding = json.RawMessage(`{}`)
	}
	featureFlags := u.FeatureFlags
	if len(featureFlags) == 0 {
		featureFlags = json.RawMessage(`{}`)
	}
	shortcuts := u.Shortcuts
	if shortcuts == nil {
		shortcuts = []string{}
	}

	return &SafeUser{
		ID:               u.ID,
		PublicID:         u.PublicID,
		Email:            u.Email,
		Username:         u.Username,
		DisplayName:      u.DisplayName,
		FirstName:        u.FirstName,
		LastName:         u.LastName,
		FullName:         u.FullName,
		PhotoURL:         u.PhotoURL,
		CoverPhotoURL:    u.CoverPhotoURL,
		Phone:            u.Phone,
		PhoneVerified:    u.PhoneVerified,
		EmailVerified:    u.EmailVerified,
		EmailVerifiedAt:  u.EmailVerifiedAt,
		Country:          u.Country,
		Timezone:         u.Timezone,
		Locale:           u.Locale,
		Language:         u.Language,
		Currency:         u.Currency,
		Status:           u.Status,
		AccountType:      u.AccountType,
		LoginRedirectURL: u.LoginRedirectURL,
		Shortcuts:        shortcuts,
		Settings:         settings,
		Preferences:      preferences,
		Onboarding:       onboarding,
		FeatureFlags:     featureFlags,
		TwoFAEnabled:     u.TwoFAEnabled,
		LastLoginAt:      u.LastLoginAt,
		LastActivityAt:   u.LastActivityAt,
		CreatedAt:        u.CreatedAt,
		UpdatedAt:        u.UpdatedAt,
	}
}

// UpdateProfileRequest is the body for PATCH /api/v1/users/me.
type UpdateProfileRequest struct {
	Username         string          `json:"username"`
	DisplayName      string          `json:"displayName"`
	FirstName        string          `json:"firstName"`
	LastName         string          `json:"lastName"`
	PhotoURL         string          `json:"photoURL"`
	CoverPhotoURL    string          `json:"coverPhotoURL"`
	Phone            string          `json:"phone"`
	Country          string          `json:"country"`
	Timezone         string          `json:"timezone"`
	Locale           string          `json:"locale"`
	Language         string          `json:"language"`
	Currency         string          `json:"currency"`
	LoginRedirectURL string          `json:"loginRedirectUrl"`
	Shortcuts        []string        `json:"shortcuts"`
	Settings         json.RawMessage `json:"settings"`
	Preferences      json.RawMessage `json:"preferences"`
	Onboarding       json.RawMessage `json:"onboarding"`
	FeatureFlags     json.RawMessage `json:"featureFlags"`
}

// AvatarRequest supports POST /api/v1/me/avatar with either a JSON URL
// or multipart file upload handled by the HTTP handler.
type AvatarRequest struct {
	PhotoURL string `json:"photoURL"`
}
