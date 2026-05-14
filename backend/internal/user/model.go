// Package user manages user profile data.
// Authentication (credentials, sessions) lives in the auth package.
// This package owns: profile fields, email verification state, account status.
package user

import "time"

// User represents a registered account in BusinessSAAS.
type User struct {
	ID           string     `db:"id" json:"id"`
	Email        string     `db:"email" json:"email"`
	PasswordHash string     `db:"password_hash" json:"-"` // never serialised to JSON
	FirstName    string     `db:"first_name" json:"first_name"`
	LastName     string     `db:"last_name" json:"last_name"`
	IsVerified   bool       `db:"is_verified" json:"is_verified"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	FailedLogins int        `db:"failed_logins" json:"-"`
	LockedUntil  *time.Time `db:"locked_until" json:"-"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

// IsLocked returns true if the account is currently under a lockout period.
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// FullName returns the user's display name.
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

// SafeUser is the subset of User that is safe to return in API responses.
// It deliberately omits password hash, failed login count, and lock fields.
type SafeUser struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	IsVerified bool      `json:"is_verified"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToSafe converts a full User to a SafeUser for API responses.
func (u *User) ToSafe() SafeUser {
	return SafeUser{
		ID:         u.ID,
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		IsVerified: u.IsVerified,
		IsActive:   u.IsActive,
		CreatedAt:  u.CreatedAt,
	}
}

// UpdateProfileRequest is the request body for PATCH /api/v1/users/me.
type UpdateProfileRequest struct {
	FirstName string `json:"first_name" validate:"omitempty,min=1,max=100"`
	LastName  string `json:"last_name"  validate:"omitempty,min=1,max=100"`
}
