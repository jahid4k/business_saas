package email

import "time"

// Destination decides which module an inbound email becomes. Defaults to
// 'lead' so every address registered before Phase 8D keeps behaving exactly
// as it did — see migration 00112.
type Destination string

const (
	DestinationLead   Destination = "lead"
	DestinationTicket Destination = "ticket"
)

func (d Destination) IsValid() bool {
	return d == DestinationLead || d == DestinationTicket
}

type OrgInboundEmail struct {
	ID          string      `json:"id" db:"id"`
	OrgID       string      `json:"org_id" db:"org_id"`
	Address     string      `json:"address" db:"address"`
	Destination Destination `json:"destination" db:"destination"`
	IsActive    bool        `json:"is_active" db:"is_active"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
}

// InboundRoute is what an address resolves to: which org, and which module.
// GetOrgByAddress used to return just an org id, which was sufficient while
// there was exactly one destination.
type InboundRoute struct {
	OrgID       string
	Destination Destination
}

// EmployeeRequester is the sender resolved to somebody who can actually own
// a ticket. Resolved by THIS package's repository rather than by
// platform/tickets, which must never reference hrm_* — the 7D
// benefits.FindEmployeeIDByUserID precedent: resolving your own subject is
// the consuming package's job.
type EmployeeRequester struct {
	UserID     string
	EmployeeID string
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
