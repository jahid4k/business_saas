// Package business manages workspaces/tenants in BusinessSAAS.
// Every piece of business data is scoped to a business_id.
// Cross-business data access is never permitted.
package business

import "time"

// Business represents a workspace/tenant in BusinessSAAS.
type Business struct {
	ID        string    `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	Slug      string    `db:"slug"       json:"slug"` // URL-safe unique identifier
	OwnerID   string    `db:"owner_id"   json:"owner_id"`
	IsActive  bool      `db:"is_active"  json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CreateBusinessRequest is the request body for POST /api/v1/businesses.
type CreateBusinessRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
	Slug string `json:"slug" validate:"required,min=2,max=50,alphanum"`
}
