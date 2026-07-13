// backend/internal/hrm/employeedocs/model.go
//
// C3: Employee Document Management.
// Uses existing hrm_employee_documents table from migration 00024 (Group A4).
// No new migration required — this is the Go package layer for that table.
package employeedocs

import (
	"errors"
	"time"
)

type DocStatus string
const (
	StatusDraft        DocStatus = "draft"
	StatusSent         DocStatus = "sent"
	StatusAcknowledged DocStatus = "acknowledged"
	StatusDeclined     DocStatus = "declined"
	StatusExpired      DocStatus = "expired"
	StatusWithdrawn    DocStatus = "withdrawn"
	StatusSuperseded   DocStatus = "superseded"
)

// EmployeeDocument represents a document instance for an employee.
// It may be generated from a template (template_id set) or directly uploaded (template_id nil).
type EmployeeDocument struct {
	ID                 string     `db:"id"                  json:"id"`
	PublicID           string     `db:"public_id"           json:"public_id"`
	OrgID              string     `db:"org_id"              json:"org_id"`
	EmployeeID         string     `db:"employee_id"         json:"employee_id"`
	TemplateID         *string    `db:"template_id"         json:"template_id,omitempty"`
	Title              string     `db:"title"               json:"title"`
	DocumentType       string     `db:"document_type"       json:"document_type"`
	FileURL            string     `db:"file_url"            json:"file_url"`
	FileName           string     `db:"file_name"           json:"file_name"`
	FileSizeBytes      *int64     `db:"file_size_bytes"     json:"file_size_bytes,omitempty"`
	MimeType           string     `db:"mime_type"           json:"mime_type"`
	GeneratedContent   *string    `db:"generated_content"   json:"generated_content,omitempty"`
	RelatedType        *string    `db:"related_type"        json:"related_type,omitempty"`
	RelatedID          *string    `db:"related_id"          json:"related_id,omitempty"`
	Version            int        `db:"version"             json:"version"`
	SupersededBy       *string    `db:"superseded_by"       json:"superseded_by,omitempty"`
	BulkSendBatchID    *string    `db:"bulk_send_batch_id"  json:"bulk_send_batch_id,omitempty"`
	ExpiryDate         *string    `db:"expiry_date"         json:"expiry_date,omitempty"`
	Status             DocStatus  `db:"status"              json:"status"`
	IssuedBy           *string    `db:"issued_by"           json:"issued_by,omitempty"`
	SentAt             *time.Time `db:"sent_at"             json:"sent_at,omitempty"`
	AcknowledgedAt     *time.Time `db:"acknowledged_at"     json:"acknowledged_at,omitempty"`
	AcknowledgementNote *string   `db:"acknowledgement_note" json:"acknowledgement_note,omitempty"`
	CreatedBy          string     `db:"created_by"          json:"created_by"`
	CreatedAt          time.Time  `db:"created_at"          json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"          json:"updated_at"`
}

// CreateDocumentRequest creates an employee document from a template or as a direct upload.
type CreateDocumentRequest struct {
	TemplateID   *string `json:"template_id"`    // nil = direct upload
	Title        string  `json:"title"`
	DocumentType string  `json:"document_type"`
	FileURL      string  `json:"file_url"`
	FileName     string  `json:"file_name"`
	FileSizeBytes *int64 `json:"file_size_bytes"`
	MimeType     string  `json:"mime_type"`
	RelatedType  *string `json:"related_type"`   // warning|promotion|transfer|...
	RelatedID    *string `json:"related_id"`
	ExpiryDate   *string `json:"expiry_date"`
}

type AcknowledgeDocRequest struct {
	Note      *string `json:"note"`
	Signature *string `json:"signature"` // base64 or typed name
}

type DocListResponse struct {
	Documents []*EmployeeDocument `json:"documents"`
	Total     int                 `json:"total"`
}

var (
	ErrNotFound      = errors.New("document not found")
	ErrTitleRequired = errors.New("title is required")
	ErrFileURLRequired = errors.New("file_url is required")
	ErrFileNameRequired = errors.New("file_name is required")
	ErrDocTypeRequired  = errors.New("document_type is required")
	ErrWrongStatus   = errors.New("action not allowed in current document status")
	ErrAlreadyAcknowledged = errors.New("document has already been acknowledged")
)
