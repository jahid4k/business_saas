// backend/internal/hrm/doctemplates/model.go
package doctemplates

import (
	"errors"
	"time"
)

type DocumentType string

const (
	DocTypeOfferLetter        DocumentType = "offer_letter"
	DocTypeContract           DocumentType = "contract"
	DocTypeWarningLetter      DocumentType = "warning_letter"
	DocTypePromotionLetter    DocumentType = "promotion_letter"
	DocTypeTransferLetter     DocumentType = "transfer_letter"
	DocTypeTerminationLetter  DocumentType = "termination_letter"
	DocTypeResignationAck     DocumentType = "resignation_acceptance"
	DocTypeExperienceLetter   DocumentType = "experience_letter"
	DocTypeNDA                DocumentType = "nda"
	DocTypePolicy             DocumentType = "policy"
	DocTypeCustom             DocumentType = "custom"
)

func (d DocumentType) IsValid() bool {
	switch d {
	case DocTypeOfferLetter, DocTypeContract, DocTypeWarningLetter, DocTypePromotionLetter,
		DocTypeTransferLetter, DocTypeTerminationLetter, DocTypeResignationAck,
		DocTypeExperienceLetter, DocTypeNDA, DocTypePolicy, DocTypeCustom:
		return true
	}
	return false
}

// DocumentTemplate is a reusable Markdown template with {{placeholder}} syntax.
type DocumentTemplate struct {
	ID                     string       `db:"id"                       json:"id"`
	PublicID               string       `db:"public_id"                json:"public_id"`
	OrgID                  string       `db:"org_id"                   json:"org_id"`
	Name                   string       `db:"name"                     json:"name"`
	DocumentType           DocumentType `db:"document_type"            json:"document_type"`
	Description            *string      `db:"description"              json:"description,omitempty"`
	BodyMarkdown           string       `db:"body_markdown"            json:"body_markdown"`
	AvailableVariables     []string     `db:"available_variables"      json:"available_variables"`
	RequiresAcknowledgement bool        `db:"requires_acknowledgement" json:"requires_acknowledgement"`
	IsActive               bool         `db:"is_active"                json:"is_active"`
	CreatedBy              string       `db:"created_by"               json:"created_by"`
	CreatedAt              time.Time    `db:"created_at"               json:"created_at"`
	UpdatedAt              time.Time    `db:"updated_at"               json:"updated_at"`
}

// PreviewResult holds the filled Markdown content after placeholder substitution.
type PreviewResult struct {
	FilledContent string            `json:"filled_content"`
	VariablesUsed []string          `json:"variables_used"`
	Missing       []string          `json:"missing,omitempty"`
}

type CreateDocumentTemplateRequest struct {
	Name                    string       `json:"name"`
	DocumentType            DocumentType `json:"document_type"`
	Description             *string      `json:"description"`
	BodyMarkdown            string       `json:"body_markdown"`
	AvailableVariables      []string     `json:"available_variables"`
	RequiresAcknowledgement bool         `json:"requires_acknowledgement"`
}

type UpdateDocumentTemplateRequest struct {
	Name                    *string      `json:"name"`
	DocumentType            *DocumentType`json:"document_type"`
	Description             *string      `json:"description"`
	BodyMarkdown            *string      `json:"body_markdown"`
	AvailableVariables      []string     `json:"available_variables"`
	RequiresAcknowledgement *bool        `json:"requires_acknowledgement"`
	IsActive                *bool        `json:"is_active"`
}

type PreviewTemplateRequest struct {
	Variables map[string]string `json:"variables"`
}

type DocumentTemplateListResponse struct {
	Templates []*DocumentTemplate `json:"templates"`
	Total     int                 `json:"total"`
}

var (
	ErrTemplateNotFound     = errors.New("document template not found")
	ErrNameRequired         = errors.New("name is required")
	ErrNameTooLong          = errors.New("name must not exceed 100 characters")
	ErrNameConflict         = errors.New("a template with this name already exists")
	ErrInvalidDocumentType  = errors.New("invalid document_type")
	ErrBodyRequired         = errors.New("body_markdown is required")
)
