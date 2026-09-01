// backend/internal/hrm/announcements/service.go
package announcements

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service interface {
	List(ctx context.Context, orgID, category, status string) (*AnnouncementListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Announcement, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateAnnouncementRequest) (*Announcement, error)
	Update(ctx context.Context, orgID, ref string, req UpdateAnnouncementRequest) (*Announcement, error)
	// Publish moves to published and, when requires_acknowledgement=true,
	// creates C4 acknowledgement requests for each target employee.
	Publish(ctx context.Context, orgID, ref, publishedBy string) (*Announcement, error)
	Schedule(ctx context.Context, orgID, ref string) (*Announcement, error)
	Archive(ctx context.Context, orgID, ref string) (*Announcement, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) List(ctx context.Context, orgID, category, status string) (*AnnouncementListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, category, status)
	if err != nil {
		return nil, fmt.Errorf("announcements: List: %w", err)
	}
	if list == nil {
		list = []*Announcement{}
	}
	return &AnnouncementListResponse{Announcements: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Announcement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("announcements: Get: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	return a, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateAnnouncementRequest) (*Announcement, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, ErrTitleRequired
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, ErrContentRequired
	}
	if req.Category != "" && !req.Category.IsValid() {
		return nil, ErrInvalidCategory
	}
	cat := req.Category
	if cat == "" {
		cat = CatGeneral
	}
	scope := req.ScopeType
	if scope == "" {
		scope = ScopeOrganization
	}
	ids := req.ScopeIDs
	if ids == nil {
		ids = []string{}
	}

	a := &Announcement{
		OrgID: orgID, Title: req.Title, Content: req.Content,
		Category: cat, ScopeType: scope, ScopeIDs: ids,
		ScheduledAt: req.ScheduledAt, ExpiresAt: req.ExpiresAt,
		RequiresAcknowledgement: req.RequiresAcknowledgement,
		AcknowledgementDeadline: req.AcknowledgementDeadline,
		IsPinned:                req.IsPinned,
		AuthorID:                createdBy, Status: StatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("announcements: Create: %w", err)
	}
	return a, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdateAnnouncementRequest) (*Announcement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("announcements: Update: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.Status == StatusPublished || a.Status == StatusArchived {
		return nil, ErrWrongStatus
	}
	if req.Title != nil {
		a.Title = *req.Title
	}
	if req.Content != nil {
		a.Content = *req.Content
	}
	if req.Category != nil {
		a.Category = *req.Category
	}
	if req.ScopeIDs != nil {
		a.ScopeIDs = req.ScopeIDs
	}
	if req.ScheduledAt != nil {
		a.ScheduledAt = req.ScheduledAt
	}
	if req.ExpiresAt != nil {
		a.ExpiresAt = req.ExpiresAt
	}
	if req.RequiresAcknowledgement != nil {
		a.RequiresAcknowledgement = *req.RequiresAcknowledgement
	}
	if req.AcknowledgementDeadline != nil {
		a.AcknowledgementDeadline = req.AcknowledgementDeadline
	}
	if req.IsPinned != nil {
		a.IsPinned = *req.IsPinned
	}
	if req.PinOrder != nil {
		a.PinOrder = *req.PinOrder
	}
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("announcements: Update: %w", err)
	}
	return a, nil
}

// Publish publishes the announcement immediately.
// If requires_acknowledgement=true, creates C4 acknowledgement requests
// for each target employee (acknowledgeable_type='announcement').
func (s *serviceImpl) Publish(ctx context.Context, orgID, ref, publishedBy string) (*Announcement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("announcements: Publish: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.Status == StatusPublished {
		return nil, ErrAlreadyPublished
	}
	if a.Status == StatusArchived {
		return nil, ErrWrongStatus
	}

	var pub interface{} = time.Now()
	if err := s.repo.UpdateStatus(ctx, a.ID, StatusPublished, &pub); err != nil {
		return nil, fmt.Errorf("announcements: Publish: %w", err)
	}
	now := time.Now()
	a.Status = StatusPublished
	a.PublishedAt = &now

	// C4 integration: create acknowledgement requests for each target employee
	if a.RequiresAcknowledgement {
		empIDs, err := s.repo.GetTargetEmployeeIDs(ctx, orgID, a.ScopeType, a.ScopeIDs)
		if err == nil && len(empIDs) > 0 {
			for _, empID := range empIDs {
				// Direct insert into hrm_acknowledgements (C4 table)
				_, _ = s.db.Exec(ctx,
					`INSERT INTO hrm_acknowledgements
					(org_id, employee_id, acknowledgeable_type, acknowledgeable_id,
					 entity_title, signature_required, expires_at, status, requested_by)
					VALUES ($1,$2::uuid,'announcement',$3::uuid,$4,FALSE,$5::date,'pending',$6::uuid)
					ON CONFLICT (employee_id, acknowledgeable_type, acknowledgeable_id, status) DO NOTHING`,
					orgID, empID, a.ID, a.Title,
					a.AcknowledgementDeadline, publishedBy)
			}
		}
	}
	return a, nil
}

func (s *serviceImpl) Schedule(ctx context.Context, orgID, ref string) (*Announcement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("announcements: Schedule: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.Status != StatusDraft {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, a.ID, StatusScheduled, nil); err != nil {
		return nil, fmt.Errorf("announcements: Schedule: %w", err)
	}
	a.Status = StatusScheduled
	return a, nil
}

func (s *serviceImpl) Archive(ctx context.Context, orgID, ref string) (*Announcement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("announcements: Archive: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.Status == StatusArchived {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, a.ID, StatusArchived, nil); err != nil {
		return nil, fmt.Errorf("announcements: Archive: %w", err)
	}
	a.Status = StatusArchived
	return a, nil
}
