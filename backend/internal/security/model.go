// backend/internal/security/model.go
package security

import "time"

type SessionView struct {
	ID             string     `json:"id"`
	PublicID       string     `json:"publicId"`
	UserID         string     `json:"userId"`
	UserPublicID   string     `json:"userPublicId"`
	Email          string     `json:"email"`
	DisplayName    string     `json:"displayName"`
	UserAgent      string     `json:"userAgent,omitempty"`
	IPAddress      string     `json:"ipAddress,omitempty"`
	Country        string     `json:"country,omitempty"`
	City           string     `json:"city,omitempty"`
	Region         string     `json:"region,omitempty"`
	LastActivityAt time.Time  `json:"lastActivityAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	RevokedAt      *time.Time `json:"revokedAt,omitempty"`
	IsActive       bool       `json:"isActive"`
}

type LoginEventView struct {
	ID            string    `json:"id"`
	PublicID      string    `json:"publicId"`
	UserID        string    `json:"userId,omitempty"`
	UserPublicID  string    `json:"userPublicId,omitempty"`
	Email         string    `json:"email,omitempty"`
	Provider      string    `json:"provider"`
	Status        string    `json:"status"`
	FailureReason string    `json:"failureReason,omitempty"`
	IPAddress     string    `json:"ipAddress,omitempty"`
	UserAgent     string    `json:"userAgent,omitempty"`
	Country       string    `json:"country,omitempty"`
	City          string    `json:"city,omitempty"`
	Region        string    `json:"region,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}
