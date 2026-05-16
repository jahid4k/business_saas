// backend/internal/user/model.go
package user

import "time"

// User maps directly to the users table.
// NEVER expose PasswordHash to API clients.
type User struct {
	ID           string     `db:"id"`
	Email        string     `db:"email"`
	PasswordHash string     `db:"password_hash"`
	FirstName    string     `db:"first_name"`
	LastName     string     `db:"last_name"`
	IsVerified   bool       `db:"is_verified"`
	IsActive     bool       `db:"is_active"`
	FailedLogins int        `db:"failed_logins"`
	LockedUntil  *time.Time `db:"locked_until"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

// IsLocked returns true if the account is currently locked out.
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// SafeUser is the public-safe representation — no password hash, no internal fields.
type SafeUser struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	IsVerified bool      `json:"is_verified"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToSafe converts a User to its public-safe representation.
func (u *User) ToSafe() *SafeUser {
	return &SafeUser{
		ID:         u.ID,
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		IsVerified: u.IsVerified,
		CreatedAt:  u.CreatedAt,
	}
}

// UpdateProfileRequest is the body for PATCH /api/v1/users/me.
type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
