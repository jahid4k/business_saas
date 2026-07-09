// backend/internal/hrm/awards/service.go
package awards

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

type Service interface {
	List(ctx context.Context, orgID, employeeID, status string) (*AwardListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Award, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateAwardRequest) (*Award, error)
	Update(ctx context.Context, orgID, ref string, req UpdateAwardRequest) (*Award, error)
	Submit(ctx context.Context, orgID, ref string) (*Award, error)
	// Issue formally awards the employee. Optionally creates an E2 announcement.
	Issue(ctx context.Context, orgID, ref, issuedBy string, req IssueRequest) (*Award, error)
	Cancel(ctx context.Context, orgID, ref string) (*Award, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}
func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) List(ctx context.Context, orgID, employeeID, status string) (*AwardListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, employeeID, status)
	if err != nil { return nil, fmt.Errorf("awards: List: %w", err) }
	if list == nil { list = []*Award{} }
	return &AwardListResponse{Awards: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Award, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("awards: Get: %w", err) }
	if a == nil { return nil, ErrNotFound }
	return a, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateAwardRequest) (*Award, error) {
	if strings.TrimSpace(req.EmployeeID) == "" { return nil, ErrEmployeeIDRequired }
	if strings.TrimSpace(req.Title) == "" { return nil, ErrTitleRequired }
	if strings.TrimSpace(req.Description) == "" { return nil, ErrDescriptionRequired }
	if !req.AwardType.IsValid() { return nil, ErrInvalidAwardType }

	awardDate := time.Now().Format(dateLayout)
	if req.AwardDate != nil && strings.TrimSpace(*req.AwardDate) != "" {
		if _, err := time.Parse(dateLayout, *req.AwardDate); err != nil { return nil, ErrInvalidDate }
		awardDate = *req.AwardDate
	}

	points := 0
	if req.Points != nil { points = *req.Points }
	currency := "BDT"
	if req.Currency != nil && *req.Currency != "" { currency = *req.Currency }

	a := &Award{
		OrgID: orgID, EmployeeID: req.EmployeeID,
		AwardType: req.AwardType, Title: req.Title, Description: req.Description,
		Points: points, MonetaryValue: req.MonetaryValue, Currency: currency,
		AwardDate: awardDate, IssuedBy: createdBy,
		Status: StatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, a); err != nil { return nil, fmt.Errorf("awards: Create: %w", err) }
	return a, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdateAwardRequest) (*Award, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("awards: Update: %w", err) }
	if a == nil { return nil, ErrNotFound }
	if a.Status != StatusDraft { return nil, ErrWrongStatus }
	if req.Title != nil { a.Title = *req.Title }
	if req.Description != nil { a.Description = *req.Description }
	if req.Points != nil { a.Points = *req.Points }
	if req.MonetaryValue != nil { a.MonetaryValue = req.MonetaryValue }
	if req.AwardDate != nil {
		if _, err := time.Parse(dateLayout, *req.AwardDate); err != nil { return nil, ErrInvalidDate }
		a.AwardDate = *req.AwardDate
	}
	if req.CertificateDocumentID != nil { a.CertificateDocumentID = req.CertificateDocumentID }
	if err := s.repo.Update(ctx, a); err != nil { return nil, fmt.Errorf("awards: Update: %w", err) }
	return a, nil
}

func (s *serviceImpl) Submit(ctx context.Context, orgID, ref string) (*Award, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("awards: Submit: %w", err) }
	if a == nil { return nil, ErrNotFound }
	if a.Status != StatusDraft { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, a.ID, StatusApproved); err != nil { return nil, fmt.Errorf("awards: Submit: %w", err) }
	a.Status = StatusApproved
	return a, nil
}

// Issue formally issues the award. When req.CreateAnnouncement=true,
// creates an hrm_announcements record (E2) and links it.
func (s *serviceImpl) Issue(ctx context.Context, orgID, ref, issuedBy string, req IssueRequest) (*Award, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("awards: Issue: %w", err) }
	if a == nil { return nil, ErrNotFound }
	if a.Status == StatusIssued { return nil, ErrAlreadyIssued }
	if a.Status != StatusDraft && a.Status != StatusApproved { return nil, ErrWrongStatus }

	now := time.Now()
	a.IssuedAt = &now
	a.Status = StatusIssued
	if req.CertificateDocumentID != nil { a.CertificateDocumentID = req.CertificateDocumentID }
	if err := s.repo.Update(ctx, a); err != nil { return nil, fmt.Errorf("awards: Issue: update: %w", err) }
	if err := s.repo.UpdateStatus(ctx, a.ID, StatusIssued); err != nil { return nil, fmt.Errorf("awards: Issue: status: %w", err) }

	// Create E2 announcement (optional)
	if req.CreateAnnouncement {
		content := fmt.Sprintf("Congratulations to %s for receiving the **%s** award!", a.EmployeeID, a.Title)
		if req.AnnouncementContent != nil { content = *req.AnnouncementContent }
		var annID string
		_ = s.db.QueryRow(ctx,
			`INSERT INTO hrm_announcements (org_id, title, content, category, scope_type, author_id, status, created_by)
			VALUES ($1,$2,$3,'award','organization',$4,'published',$4)
			RETURNING id::text`,
			orgID, "Award: "+a.Title, content, issuedBy,
		).Scan(&annID)
		if annID != "" {
			_ = s.repo.SetAnnouncementID(ctx, a.ID, annID)
			a.AnnouncementID = &annID
		}
	}
	return a, nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID, ref string) (*Award, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil { return nil, fmt.Errorf("awards: Cancel: %w", err) }
	if a == nil { return nil, ErrNotFound }
	if a.Status == StatusIssued || a.Status == StatusCancelled { return nil, ErrWrongStatus }
	if err := s.repo.UpdateStatus(ctx, a.ID, StatusCancelled); err != nil { return nil, fmt.Errorf("awards: Cancel: %w", err) }
	a.Status = StatusCancelled
	return a, nil
}
