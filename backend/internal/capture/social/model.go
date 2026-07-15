package social

import "time"

type SocialIntegration struct {
	ID            string         `json:"id" db:"id"`
	OrgID         string         `json:"org_id" db:"org_id"`
	Platform      string         `json:"platform" db:"platform"`
	PageID        string         `json:"page_id" db:"page_id"`
	FormID        *string        `json:"form_id,omitempty" db:"form_id"`
	AccessToken   string         `json:"access_token" db:"access_token"`
	FieldMappings map[string]any `json:"field_mappings,omitempty" db:"field_mappings"`
	IsActive      bool           `json:"is_active" db:"is_active"`
	ConnectedBy   string         `json:"connected_by" db:"connected_by"`
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
}

type SocialLeadLog struct {
	ID           string         `json:"id" db:"id"`
	OrgID        *string        `json:"org_id,omitempty" db:"org_id"`
	Platform     string         `json:"platform" db:"platform"`
	PageID       string         `json:"page_id" db:"page_id"`
	RawPayload   map[string]any `json:"raw_payload" db:"raw_payload"`
	Processed    bool           `json:"processed" db:"processed"`
	ErrorMessage *string        `json:"error_message,omitempty" db:"error_message"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
}
