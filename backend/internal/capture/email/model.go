package email

import "time"

type OrgInboundEmail struct {
	ID        string    `json:"id" db:"id"`
	OrgID     string    `json:"org_id" db:"org_id"`
	Address   string    `json:"address" db:"address"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type InboundEmailLog struct {
	ID           string         `json:"id" db:"id"`
	OrgID        *string        `json:"org_id,omitempty" db:"org_id"`
	ToAddress    string         `json:"to_address" db:"to_address"`
	FromAddress  string         `json:"from_address" db:"from_address"`
	Subject      *string        `json:"subject,omitempty" db:"subject"`
	RawPayload   map[string]any `json:"raw_payload" db:"raw_payload"`
	Processed    bool           `json:"processed" db:"processed"`
	ErrorMessage *string        `json:"error_message,omitempty" db:"error_message"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
}
