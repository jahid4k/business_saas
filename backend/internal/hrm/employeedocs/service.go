// backend/internal/hrm/employeedocs/service.go
package employeedocs

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service interface {
	List(ctx context.Context, orgID string, filter DocListFilter) (*DocListResponse, error)
	Get(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error)
	Create(ctx context.Context, orgID, employeeID, issuedBy string, req CreateDocumentRequest) (*EmployeeDocument, error)
	// Send moves a draft document to sent status.
	Send(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error)
	// Acknowledge records employee acknowledgement.
	Acknowledge(ctx context.Context, orgID, employeeID, ref string, req AcknowledgeDocRequest) (*EmployeeDocument, error)
	// Decline records employee declining the document.
	Decline(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error)
	// Withdraw revokes a sent document.
	Withdraw(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}
func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) List(ctx context.Context, orgID string, filter DocListFilter) (*DocListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindAll(ctx, orgID, filter)
	if err != nil { return nil, fmt.Errorf("employeedocs: List: %w", err) }
	if list == nil { list = []*EmployeeDocument{} }
	total, err := s.repo.Count(ctx, orgID, filter)
	if err != nil { return nil, fmt.Errorf("employeedocs: List: count: %w", err) }
	return &DocListResponse{Documents: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error) {
	d, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("employeedocs: Get: %w", err) }
	if d == nil { return nil, ErrNotFound }
	return d, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, employeeID, issuedBy string, req CreateDocumentRequest) (*EmployeeDocument, error) {
	if strings.TrimSpace(req.Title) == "" { return nil, ErrTitleRequired }
	if strings.TrimSpace(req.FileURL) == "" { return nil, ErrFileURLRequired }
	if strings.TrimSpace(req.FileName) == "" { return nil, ErrFileNameRequired }
	if strings.TrimSpace(req.DocumentType) == "" { return nil, ErrDocTypeRequired }
	mime := req.MimeType
	if mime == "" { mime = "application/pdf" }
	d := &EmployeeDocument{
		OrgID: orgID, EmployeeID: employeeID,
		TemplateID: req.TemplateID, Title: req.Title,
		DocumentType: req.DocumentType, FileURL: req.FileURL,
		FileName: req.FileName, FileSizeBytes: req.FileSizeBytes,
		MimeType: mime, RelatedType: req.RelatedType, RelatedID: req.RelatedID,
		ExpiryDate: req.ExpiryDate,
		Status: StatusDraft, IssuedBy: &issuedBy, CreatedBy: issuedBy,
	}
	if err := s.repo.Create(ctx, d); err != nil { return nil, fmt.Errorf("employeedocs: Create: %w", err) }
	return d, nil
}

func (s *serviceImpl) Send(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error) {
	d, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("employeedocs: Send: %w", err) }
	if d == nil { return nil, ErrNotFound }
	if d.Status != StatusDraft { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, d.ID, StatusSent); err != nil { return nil, fmt.Errorf("employeedocs: Send: %w", err) }
	d.Status = StatusSent
	return d, nil
}

func (s *serviceImpl) Acknowledge(ctx context.Context, orgID, employeeID, ref string, req AcknowledgeDocRequest) (*EmployeeDocument, error) {
	d, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("employeedocs: Acknowledge: %w", err) }
	if d == nil { return nil, ErrNotFound }
	if d.Status == StatusAcknowledged { return nil, ErrAlreadyAcknowledged }
	if d.Status != StatusSent { return nil, ErrWrongStatus }
	if err := s.repo.Acknowledge(ctx, d.ID, req.Note, req.Signature); err != nil {
		return nil, fmt.Errorf("employeedocs: Acknowledge: %w", err)
	}
	d.Status = StatusAcknowledged
	return d, nil
}

func (s *serviceImpl) Decline(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error) {
	d, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("employeedocs: Decline: %w", err) }
	if d == nil { return nil, ErrNotFound }
	if d.Status != StatusSent { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, d.ID, StatusDeclined); err != nil { return nil, fmt.Errorf("employeedocs: Decline: %w", err) }
	d.Status = StatusDeclined
	return d, nil
}

func (s *serviceImpl) Withdraw(ctx context.Context, orgID, employeeID, ref string) (*EmployeeDocument, error) {
	d, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil { return nil, fmt.Errorf("employeedocs: Withdraw: %w", err) }
	if d == nil { return nil, ErrNotFound }
	if d.Status == StatusWithdrawn || d.Status == StatusAcknowledged { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, d.ID, StatusWithdrawn); err != nil { return nil, fmt.Errorf("employeedocs: Withdraw: %w", err) }
	d.Status = StatusWithdrawn
	return d, nil
}
