package settings

import "time"

type CRMSettings struct {
	OrgID               string    `db:"org_id" json:"org_id"`
	LeadRoutingEnabled  bool      `db:"lead_routing_enabled" json:"lead_routing_enabled"`
	RoundRobinAssignees []string  `db:"round_robin_assignees" json:"round_robin_assignees"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
}

type UpdateCRMSettingsRequest struct {
	LeadRoutingEnabled  *bool    `json:"lead_routing_enabled"`
	RoundRobinAssignees []string `json:"round_robin_assignees"`
}
