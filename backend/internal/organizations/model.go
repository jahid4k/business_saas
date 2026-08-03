// backend/internal/business/model.go
// NOTE: Package name is kept as business for backward compatibility with existing imports.
// The database and API semantics now use organizations.
package organizations

import "time"

type Business struct {
	ID        string     `db:"id" json:"id"`
	PublicID  string     `db:"public_id" json:"publicId"`
	Name      string     `db:"name" json:"name"`
	Slug      string     `db:"slug" json:"slug"`
	LegalName string     `db:"legal_name" json:"legalName,omitempty"`
	Type      string     `db:"type" json:"type,omitempty"`
	Industry  string     `db:"industry" json:"industry,omitempty"`
	Website   string     `db:"website" json:"website,omitempty"`
	LogoURL   string     `db:"logo_url" json:"logoURL,omitempty"`
	Country   string     `db:"country" json:"country,omitempty"`
	Timezone  string     `db:"timezone" json:"timezone"`
	Currency  string     `db:"currency" json:"currency"`
	MoneyRoundingScale int `db:"money_rounding_scale" json:"moneyRoundingScale"`
	MoneyRoundingMode  string `db:"money_rounding_mode" json:"moneyRoundingMode"`
	Status    string     `db:"status" json:"status"`
	CreatedAt time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time  `db:"updated_at" json:"updatedAt"`
	DeletedAt *time.Time `db:"deleted_at" json:"deletedAt,omitempty"`
}

type CreateBusinessRequest struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	LegalName string `json:"legalName"`
	Type      string `json:"type"`
	Industry  string `json:"industry"`
	Website   string `json:"website"`
	LogoURL   string `json:"logoURL"`
	Country   string `json:"country"`
	Timezone  string `json:"timezone"`
	Currency  string `json:"currency"`
}

type MembershipWithRole struct {
	Business *Business `json:"organization"`
	Role     string    `json:"role"`
	MemberID string    `json:"membershipId"`
}

type UpdateBusinessRequest struct {
	Name      string `json:"name"`
	LegalName string `json:"legalName"`
	Type      string `json:"type"`
	Industry  string `json:"industry"`
	Website   string `json:"website"`
	LogoURL   string `json:"logoURL"`
	Country   string `json:"country"`
	Timezone  string `json:"timezone"`
	Currency  string `json:"currency"`
}
