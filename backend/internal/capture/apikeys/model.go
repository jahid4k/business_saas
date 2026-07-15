package apikeys

import (
	"errors"
	"time"
)

var (
	ErrKeyNotFound = errors.New("api key not found")
	ErrKeyRevoked  = errors.New("api key has been revoked")
	ErrKeyExpired  = errors.New("api key has expired")
)

type Scope string

const (
	ScopeCaptureLeads Scope = "capture:leads"
)

// OrgAPIKey represents a generated API key for an organization.
type OrgAPIKey struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	Name           string     `json:"name"`
	KeyPrefix      string     `json:"key_prefix"`
	KeyHash        string     `json:"-"` // Never expose hash in JSON
	Scopes         []Scope    `json:"scopes"`
	AllowedOrigins []string   `json:"allowed_origins"`
	IsActive       bool       `json:"is_active"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
}

type CreateKeyRequest struct {
	Name           string     `json:"name"`
	Scopes         []Scope    `json:"scopes"`
	AllowedOrigins []string   `json:"allowed_origins"`
	ExpiresAt      *time.Time `json:"expires_at"`
}

type CreateKeyResponse struct {
	Key    *OrgAPIKey `json:"key"`
	RawKey string     `json:"raw_key"`
}
