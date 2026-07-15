// backend/internal/platform/contacts/model.go
package contacts

import (
	"errors"
	"time"

	"github.com/mridha/businesssaas/pkg/pagination"
)

// ============================================================
// Contact
// ============================================================

// ContactStatus defines allowed values for contact status.
type ContactStatus string

const (
	ContactStatusActive   ContactStatus = "active"
	ContactStatusInactive ContactStatus = "inactive"
	ContactStatusArchived ContactStatus = "archived"
)

func (s ContactStatus) IsValid() bool {
	switch s {
	case ContactStatusActive, ContactStatusInactive, ContactStatusArchived:
		return true
	}
	return false
}

// Contact is a shared platform entity representing a person.
// Used by CRM, HRM, ERP, and any future module that deals with people.
type Contact struct {
	ID        string        `db:"id"         json:"id"`
	PublicID  string        `db:"public_id"  json:"public_id"`
	OrgID     string        `db:"org_id"     json:"org_id"`
	FirstName string        `db:"first_name" json:"first_name"`
	LastName  *string       `db:"last_name"  json:"last_name,omitempty"`
	Email     *string       `db:"email"      json:"email,omitempty"`
	Phone     *string       `db:"phone"      json:"phone,omitempty"`
	Title     *string       `db:"title"      json:"title,omitempty"`
	CompanyID *string       `db:"company_id" json:"company_id,omitempty"`
	Source    *string       `db:"source"     json:"source,omitempty"`
	Status    ContactStatus `db:"status"     json:"status"`
	OwnerID   *string       `db:"owner_id"   json:"owner_id,omitempty"`
	CreatedBy string        `db:"created_by" json:"created_by"`
	CreatedAt time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt time.Time     `db:"updated_at" json:"updated_at"`
}

type CreateContactRequest struct {
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	Title     *string `json:"title"`
	CompanyID *string `json:"company_id"`
	Source    *string `json:"source"`
	OwnerID   *string `json:"owner_id"`
}

type UpdateContactRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	Title     *string `json:"title"`
	CompanyID *string `json:"company_id"`
	Source    *string `json:"source"`
	Status    *string `json:"status"`
	OwnerID   *string `json:"owner_id"`
}

// ContactListResponse is the paginated contacts list returned by the API.
type ContactListResponse struct {
	Contacts []*Contact      `json:"contacts"`
	Meta     pagination.Meta `json:"meta"`
	// Total is kept for backwards compatibility; prefer Meta.Total.
	Total int `json:"total"`
}

// ============================================================
// Company
// ============================================================

// CompanyStatus defines allowed values for company status.
type CompanyStatus string

const (
	CompanyStatusActive   CompanyStatus = "active"
	CompanyStatusInactive CompanyStatus = "inactive"
	CompanyStatusArchived CompanyStatus = "archived"
)

func (s CompanyStatus) IsValid() bool {
	switch s {
	case CompanyStatusActive, CompanyStatusInactive, CompanyStatusArchived:
		return true
	}
	return false
}

// Company is a shared platform entity representing an organisation/business.
// Used by CRM (customer), ERP (vendor/supplier), Accounting (billing entity), etc.
type Company struct {
	ID        string        `db:"id"         json:"id"`
	PublicID  string        `db:"public_id"  json:"public_id"`
	OrgID     string        `db:"org_id"     json:"org_id"`
	Name      string        `db:"name"       json:"name"`
	Domain    *string       `db:"domain"     json:"domain,omitempty"`
	Industry  *string       `db:"industry"   json:"industry,omitempty"`
	Website   *string       `db:"website"    json:"website,omitempty"`
	Phone     *string       `db:"phone"      json:"phone,omitempty"`
	Address   *string       `db:"address"    json:"address,omitempty"`
	Country   *string       `db:"country"    json:"country,omitempty"`
	Status    CompanyStatus `db:"status"     json:"status"`
	OwnerID   *string       `db:"owner_id"   json:"owner_id,omitempty"`
	CreatedBy string        `db:"created_by" json:"created_by"`
	CreatedAt time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt time.Time     `db:"updated_at" json:"updated_at"`
}

type CreateCompanyRequest struct {
	Name     string  `json:"name"`
	Domain   *string `json:"domain"`
	Industry *string `json:"industry"`
	Website  *string `json:"website"`
	Phone    *string `json:"phone"`
	Address  *string `json:"address"`
	Country  *string `json:"country"`
	OwnerID  *string `json:"owner_id"`
}

type UpdateCompanyRequest struct {
	Name     *string `json:"name"`
	Domain   *string `json:"domain"`
	Industry *string `json:"industry"`
	Website  *string `json:"website"`
	Phone    *string `json:"phone"`
	Address  *string `json:"address"`
	Country  *string `json:"country"`
	Status   *string `json:"status"`
	OwnerID  *string `json:"owner_id"`
}

type ListCompaniesQuery struct {
	Search *string
	Status *string
	Sort   *string
	Order  *string
	Limit  int
	Offset int
}

type CompanyListResponse struct {
	Companies []*Company      `json:"companies"`
	Meta      pagination.Meta `json:"meta"`
	// Total is kept for backwards compatibility; prefer Meta.Total.
	Total int `json:"total"`
}

// ============================================================
// Enrichment
// ============================================================

type EnrichedCompanyData struct {
	Name          string `json:"name"`
	Domain        string `json:"domain"`
	Industry      string `json:"industry"`
	Logo          string `json:"logo"`
	EmployeeCount int    `json:"employee_count"`
	EstimatedRev  string `json:"estimated_revenue"`
	Location      string `json:"location"`
	LinkedIn      string `json:"linkedin"`
	Twitter       string `json:"twitter"`
	Description   string `json:"description"`
}

// ============================================================
// Sentinel errors
// ============================================================

var (
	ErrContactNotFound   = errors.New("contact not found")
	ErrCompanyNotFound   = errors.New("company not found")
	ErrFirstNameRequired = errors.New("first_name is required")
	ErrNameRequired      = errors.New("name is required")
	ErrInvalidStatus     = errors.New("invalid status value")
	// ErrInvalidEmail is returned when a supplied email address fails basic validation.
	ErrInvalidEmail = errors.New("invalid email address")
)
