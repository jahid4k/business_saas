// backend/internal/crm/templates/model.go
package templates

import (
	"errors"
	"time"
)

// TemplateType defines the allowed types for a CRM template.
type TemplateType string

const (
	TemplateTypeEmail TemplateType = "email"
	TemplateTypeNote  TemplateType = "note"
)

func (t TemplateType) IsValid() bool {
	switch t {
	case TemplateTypeEmail, TemplateTypeNote:
		return true
	}
	return false
}

// Template represents a snippet or email template.
type Template struct {
	ID        string       `db:"id"         json:"id"`
	PublicID  string       `db:"public_id"  json:"public_id"`
	OrgID     string       `db:"org_id"     json:"org_id"`
	Name      string       `db:"name"       json:"name"`
	Type      TemplateType `db:"type"       json:"type"`
	Subject   *string      `db:"subject"    json:"subject,omitempty"`
	Body      string       `db:"body"       json:"body"`
	CreatedBy string       `db:"created_by" json:"created_by"`
	CreatedAt time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt time.Time    `db:"updated_at" json:"updated_at"`
}

type CreateTemplateRequest struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Subject *string `json:"subject"`
	Body    string  `json:"body"`
}

func (r *CreateTemplateRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if !TemplateType(r.Type).IsValid() {
		return errors.New("invalid template type")
	}
	if r.Type == string(TemplateTypeEmail) && (r.Subject == nil || *r.Subject == "") {
		return errors.New("subject is required for email templates")
	}
	if r.Body == "" {
		return errors.New("body is required")
	}
	return nil
}

type UpdateTemplateRequest struct {
	Name    *string `json:"name"`
	Subject *string `json:"subject"`
	Body    *string `json:"body"`
}

type TemplateListResponse struct {
	Templates []*Template `json:"templates"`
	Total     int         `json:"total"`
}
