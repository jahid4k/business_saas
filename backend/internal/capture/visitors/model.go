package visitors

import "time"

type WebsiteVisitor struct {
	ID             string         `json:"id" db:"id"`
	OrgID          string         `json:"org_id" db:"org_id"`
	SessionID      string         `json:"session_id" db:"session_id"`
	IPAddress      *string        `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent      *string        `json:"user_agent,omitempty" db:"user_agent"`
	CompanyName    *string        `json:"company_name,omitempty" db:"company_name"`
	CompanyDomain  *string        `json:"company_domain,omitempty" db:"company_domain"`
	EnrichmentData map[string]any `json:"enrichment_data,omitempty" db:"enrichment_data"`
	LinkedLeadID   *string        `json:"linked_lead_id,omitempty" db:"linked_lead_id"`
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
}

type VisitorPageview struct {
	ID        string    `json:"id" db:"id"`
	VisitorID string    `json:"visitor_id" db:"visitor_id"`
	URL       string    `json:"url" db:"url"`
	Title     *string   `json:"title,omitempty" db:"title"`
	Referrer  *string   `json:"referrer,omitempty" db:"referrer"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type IdentifyRequest struct {
	SessionID string         `json:"session_id"`
	URL       string         `json:"url"`
	Title     *string        `json:"title,omitempty"`
	Referrer  *string        `json:"referrer,omitempty"`
	Traits    map[string]any `json:"traits,omitempty"` // For manual identification (email, name)
}
