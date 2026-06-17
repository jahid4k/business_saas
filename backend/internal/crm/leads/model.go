// backend/internal/crm/leads/model.go
package leads

import (
	"errors"
	"time"
)

// LeadStatus defines the allowed status values for a lead.
type LeadStatus string

const (
	LeadStatusNew         LeadStatus = "new"
	LeadStatusContacted   LeadStatus = "contacted"
	LeadStatusQualified   LeadStatus = "qualified"
	LeadStatusUnqualified LeadStatus = "unqualified"
	LeadStatusConverted   LeadStatus = "converted"
)

func (s LeadStatus) IsValid() bool {
	switch s {
	case LeadStatusNew, LeadStatusContacted, LeadStatusQualified,
		LeadStatusUnqualified, LeadStatusConverted:
		return true
	}
	return false
}

// Lead represents a sales prospect before conversion.
type Lead struct {
	ID                 string     `db:"id"                   json:"id"`
	PublicID           string     `db:"public_id"            json:"public_id"`
	OrgID              string     `db:"org_id"               json:"org_id"`
	FirstName          string     `db:"first_name"           json:"first_name"`
	LastName           *string    `db:"last_name"            json:"last_name,omitempty"`
	Email              *string    `db:"email"                json:"email,omitempty"`
	Phone              *string    `db:"phone"                json:"phone,omitempty"`
	CompanyName        *string    `db:"company_name"         json:"company_name,omitempty"`
	Title              *string    `db:"title"                json:"title,omitempty"`
	Source             *string    `db:"source"               json:"source,omitempty"`
	Status             LeadStatus `db:"status"               json:"status"`
	ConvertedAt        *time.Time `db:"converted_at"         json:"converted_at,omitempty"`
	ConvertedContactID *string    `db:"converted_contact_id" json:"converted_contact_id,omitempty"`
	ConvertedDealID    *string    `db:"converted_deal_id"    json:"converted_deal_id,omitempty"`
	OwnerID            *string    `db:"owner_id"             json:"owner_id,omitempty"`
	CreatedBy          string     `db:"created_by"           json:"created_by"`
	CreatedAt          time.Time  `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"           json:"updated_at"`
}

type CreateLeadRequest struct {
	FirstName   string  `json:"first_name"`
	LastName    *string `json:"last_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	CompanyName *string `json:"company_name"`
	Title       *string `json:"title"`
	Source      *string `json:"source"`
	OwnerID     *string `json:"owner_id"`
}

type UpdateLeadRequest struct {
	FirstName   *string `json:"first_name"`
	LastName    *string `json:"last_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	CompanyName *string `json:"company_name"`
	Title       *string `json:"title"`
	Source      *string `json:"source"`
	Status      *string `json:"status"`
	OwnerID     *string `json:"owner_id"`
}

// ConvertLeadRequest defines what to optionally create from a lead.
type ConvertLeadRequest struct {
	CreateContact bool     `json:"create_contact"`
	CreateDeal    bool     `json:"create_deal"`
	DealTitle     *string  `json:"deal_title"`
	PipelineID    *string  `json:"pipeline_id"`
	StageID       *string  `json:"stage_id"`
	DealValue     *float64 `json:"deal_value"`
}

// ConvertLeadResponse holds the IDs of newly created records.
type ConvertLeadResponse struct {
	Lead      *Lead   `json:"lead"`
	ContactID *string `json:"contact_id,omitempty"`
	DealID    *string `json:"deal_id,omitempty"`
}

type LeadListResponse struct {
	Leads []*Lead `json:"leads"`
	Total int     `json:"total"`
}

// LeadsBySource is used in reports.
type LeadsBySource struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

// Sentinel errors
var (
	ErrLeadNotFound         = errors.New("lead not found")
	ErrFirstNameRequired    = errors.New("first_name is required")
	ErrInvalidStatus        = errors.New("invalid status value")
	ErrLeadAlreadyConverted = errors.New("lead has already been converted")
)
