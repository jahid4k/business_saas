// backend/internal/business/model.go
package business

import "time"

// Business represents a workspace/tenant in BusinessSAAS.
// Every piece of data in the system is scoped to a business_id.
type Business struct {
	ID        string    `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	Slug      string    `db:"slug"       json:"slug"`
	OwnerID   string    `db:"owner_id"   json:"owner_id"`
	IsActive  bool      `db:"is_active"  json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CreateBusinessRequest is the body for POST /api/v1/businesses.
type CreateBusinessRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// MembershipWithRole is the membership record enriched with role name.
// Returned when listing the user's businesses so the client knows
// what role they hold in each workspace.
type MembershipWithRole struct {
	Business *Business `json:"business"`
	Role     string    `json:"role"`
	MemberID string    `json:"membership_id"`
}
