// backend/internal/hrm/awards/model.go
package awards

import (
	"github.com/shopspring/decimal"
	"errors"
	"time"
)

type AwardType string
const (
	TypeSpotRecognition AwardType = "spot_recognition"
	TypePerformance     AwardType = "performance"
	TypeTenure          AwardType = "tenure"
	TypeTeam            AwardType = "team"
	TypeInnovation      AwardType = "innovation"
	TypeCustomerService AwardType = "customer_service"
	TypeCustom          AwardType = "custom"
)
func (t AwardType) IsValid() bool {
	switch t { case TypeSpotRecognition, TypePerformance, TypeTenure, TypeTeam, TypeInnovation, TypeCustomerService, TypeCustom: return true }
	return false
}

type AwardStatus string
const (
	StatusDraft          AwardStatus = "draft"
	StatusPendingApproval AwardStatus = "pending_approval"
	StatusApproved       AwardStatus = "approved"
	StatusIssued         AwardStatus = "issued"
	StatusCancelled      AwardStatus = "cancelled"
)

// Award is an employee recognition record.
type Award struct {
	ID                     string      `db:"id"                       json:"id"`
	PublicID               string      `db:"public_id"                json:"public_id"`
	OrgID                  string      `db:"org_id"                   json:"org_id"`
	EmployeeID             string      `db:"employee_id"              json:"employee_id"`
	AwardType              AwardType   `db:"award_type"               json:"award_type"`
	Title                  string      `db:"title"                    json:"title"`
	Description            string      `db:"description"              json:"description"`
	Points                 int         `db:"points"                   json:"points"`
	MonetaryValue          *decimal.Decimal    `db:"monetary_value"           json:"monetary_value,omitempty"`
	Currency               string      `db:"currency"                 json:"currency"`
	AwardDate              string      `db:"award_date"               json:"award_date"`
	IssuedBy               string      `db:"issued_by"                json:"issued_by"`
	ApprovalInstanceID     *string     `db:"approval_instance_id"     json:"approval_instance_id,omitempty"`
	CertificateDocumentID  *string     `db:"certificate_document_id"  json:"certificate_document_id,omitempty"`
	AnnouncementID         *string     `db:"announcement_id"          json:"announcement_id,omitempty"`
	Status                 AwardStatus `db:"status"                   json:"status"`
	IssuedAt               *time.Time  `db:"issued_at"                json:"issued_at,omitempty"`
	CreatedBy              string      `db:"created_by"               json:"created_by"`
	CreatedAt              time.Time   `db:"created_at"               json:"created_at"`
	UpdatedAt              time.Time   `db:"updated_at"               json:"updated_at"`
}

type CreateAwardRequest struct {
	EmployeeID    string    `json:"employee_id"`
	AwardType     AwardType `json:"award_type"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Points        *int      `json:"points"`
	MonetaryValue *decimal.Decimal  `json:"monetary_value"`
	Currency      *string   `json:"currency"`
	AwardDate     *string   `json:"award_date"`
}

type UpdateAwardRequest struct {
	Title                 *string  `json:"title"`
	Description           *string  `json:"description"`
	Points                *int     `json:"points"`
	MonetaryValue         *decimal.Decimal `json:"monetary_value"`
	AwardDate             *string  `json:"award_date"`
	CertificateDocumentID *string  `json:"certificate_document_id"`
}

// IssueRequest issues a draft/approved award to the employee.
type IssueRequest struct {
	CertificateDocumentID *string `json:"certificate_document_id"`
	CreateAnnouncement    bool    `json:"create_announcement"` // auto-create E2 announcement
	AnnouncementContent   *string `json:"announcement_content"`
}

type AwardListResponse struct {
	Awards []*Award `json:"awards"`
	Total  int      `json:"total"`
}

var (
	ErrNotFound            = errors.New("award not found")
	ErrEmployeeIDRequired  = errors.New("employee_id is required")
	ErrTitleRequired       = errors.New("title is required")
	ErrDescriptionRequired = errors.New("description is required")
	ErrInvalidAwardType    = errors.New("invalid award_type")
	ErrInvalidDate         = errors.New("date must be a valid YYYY-MM-DD")
	ErrWrongStatus         = errors.New("action not allowed in current award status")
	ErrAlreadyIssued       = errors.New("award has already been issued")
)
