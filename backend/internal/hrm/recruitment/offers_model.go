// backend/internal/hrm/recruitment/offers_model.go
package recruitment

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type OfferStatus string

const (
	OfferStatusDraft           OfferStatus = "draft"
	OfferStatusPendingApproval OfferStatus = "pending_approval"
	OfferStatusApproved        OfferStatus = "approved"
	OfferStatusRejected        OfferStatus = "rejected"
	OfferStatusSent            OfferStatus = "sent"
	OfferStatusAccepted        OfferStatus = "accepted"
	OfferStatusDeclined        OfferStatus = "declined"
	OfferStatusRescinded       OfferStatus = "rescinded"
	OfferStatusExpired         OfferStatus = "expired"
)

// Offer is an approval-gated compensation offer for an application, reusing
// the hrm_approval_* engine exactly like hrm_job_requisitions.
type Offer struct {
	ID                 string           `db:"id"                    json:"id"`
	PublicID           string           `db:"public_id"             json:"public_id"`
	OrgID              string           `db:"org_id"                json:"org_id"`
	ApplicationID      string           `db:"application_id"        json:"application_id"`
	RequisitionID      string           `db:"requisition_id"        json:"requisition_id"`
	BaseSalary         *decimal.Decimal `db:"base_salary"           json:"base_salary,omitempty"`
	SalaryCurrency     string           `db:"salary_currency"       json:"salary_currency"`
	SigningBonus       *decimal.Decimal `db:"signing_bonus"         json:"signing_bonus,omitempty"`
	EquityDetails      *string          `db:"equity_details"        json:"equity_details,omitempty"`
	StartDate          *time.Time       `db:"start_date"            json:"start_date,omitempty"`
	ExpiresAt          *time.Time       `db:"expires_at"            json:"expires_at,omitempty"`
	Status             OfferStatus      `db:"status"                json:"status"`
	ApprovalInstanceID *string          `db:"approval_instance_id"  json:"approval_instance_id,omitempty"`
	DocumentID         *string          `db:"document_id"           json:"document_id,omitempty"`
	CreatedBy          string           `db:"created_by"            json:"created_by"`
	CreatedAt          time.Time        `db:"created_at"            json:"created_at"`
	UpdatedAt          time.Time        `db:"updated_at"            json:"updated_at"`
}

type CreateOfferRequest struct {
	RequisitionID  string           `json:"requisition_id"`
	BaseSalary     *decimal.Decimal `json:"base_salary"`
	SalaryCurrency *string          `json:"salary_currency"`
	SigningBonus   *decimal.Decimal `json:"signing_bonus"`
	EquityDetails  *string          `json:"equity_details"`
	StartDate      *string          `json:"start_date"` // ISO 8601 date
	ExpiresAt      *string          `json:"expires_at"` // RFC 3339
}

type UpdateOfferRequest struct {
	BaseSalary     *decimal.Decimal `json:"base_salary"`
	SalaryCurrency *string          `json:"salary_currency"`
	SigningBonus   *decimal.Decimal `json:"signing_bonus"`
	EquityDetails  *string          `json:"equity_details"`
	StartDate      *string          `json:"start_date"`
	ExpiresAt      *string          `json:"expires_at"`
}

var (
	ErrOfferNotFound            = errors.New("offer not found")
	ErrOfferRequisitionRequired = errors.New("requisition_id is required")
	ErrOfferWrongStatus         = errors.New("action not allowed in the offer's current status")
)
