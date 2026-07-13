// backend/internal/hrm/calendar/service.go
package calendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

type Service interface {
	List(ctx context.Context, orgID, eventType, status, fromDate, toDate string) (*EventListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*CalendarEvent, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateEventRequest) (*CalendarEvent, error)
	Update(ctx context.Context, orgID, ref string, req UpdateEventRequest) (*CalendarEvent, error)
	Cancel(ctx context.Context, orgID, ref string) (*CalendarEvent, error)
	// RequestRSVP creates C4 acknowledgement requests for all target employees.
	RequestRSVP(ctx context.Context, orgID, ref, requestedBy string) (*CalendarEvent, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) List(ctx context.Context, orgID, eventType, status, fromDate, toDate string) (*EventListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, eventType, status, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("calendar: List: %w", err)
	}
	if list == nil {
		list = []*CalendarEvent{}
	}
	return &EventListResponse{Events: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*CalendarEvent, error) {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("calendar: Get: %w", err)
	}
	if e == nil {
		return nil, ErrNotFound
	}
	return e, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateEventRequest) (*CalendarEvent, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, ErrTitleRequired
	}
	if !req.EventType.IsValid() {
		return nil, ErrInvalidEventType
	}
	if strings.TrimSpace(req.StartDate) == "" {
		return nil, ErrStartDateRequired
	}
	if strings.TrimSpace(req.EndDate) == "" {
		return nil, ErrEndDateRequired
	}
	if _, err := time.Parse(dateLayout, req.StartDate); err != nil {
		return nil, ErrInvalidDate
	}
	if _, err := time.Parse(dateLayout, req.EndDate); err != nil {
		return nil, ErrInvalidDate
	}
	if req.EndDate < req.StartDate {
		return nil, ErrEndBeforeStart
	}
	scope := req.ScopeType
	if scope == "" {
		scope = ScopeOrganization
	}
	ids := req.ScopeIDs
	if ids == nil {
		ids = []string{}
	}

	e := &CalendarEvent{
		OrgID: orgID, Title: req.Title, Description: req.Description,
		EventType: req.EventType, StartDate: req.StartDate, EndDate: req.EndDate,
		IsAllDay: req.IsAllDay, StartTime: req.StartTime, EndTime: req.EndTime,
		Location: req.Location, ScopeType: scope, ScopeIDs: ids,
		RequiresRSVP: req.RequiresRSVP, RSVPDeadline: req.RSVPDeadline,
		OrganizerID: req.OrganizerID,
		Status:      StatusUpcoming, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, fmt.Errorf("calendar: Create: %w", err)
	}
	// Auto-request RSVP on create if required
	if e.RequiresRSVP {
		_, _ = s.requestRSVPInternal(ctx, orgID, e, createdBy)
	}
	return e, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdateEventRequest) (*CalendarEvent, error) {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("calendar: Update: %w", err)
	}
	if e == nil {
		return nil, ErrNotFound
	}
	if e.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	if req.Title != nil {
		e.Title = *req.Title
	}
	if req.Description != nil {
		e.Description = req.Description
	}
	if req.StartDate != nil {
		if _, err := time.Parse(dateLayout, *req.StartDate); err != nil {
			return nil, ErrInvalidDate
		}
		e.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		if _, err := time.Parse(dateLayout, *req.EndDate); err != nil {
			return nil, ErrInvalidDate
		}
		e.EndDate = *req.EndDate
	}
	if req.Location != nil {
		e.Location = req.Location
	}
	if req.RequiresRSVP != nil {
		e.RequiresRSVP = *req.RequiresRSVP
	}
	if req.RSVPDeadline != nil {
		e.RSVPDeadline = req.RSVPDeadline
	}
	if req.Status != nil {
		e.Status = *req.Status
	}
	if err := s.repo.Update(ctx, e); err != nil {
		return nil, fmt.Errorf("calendar: Update: %w", err)
	}
	return e, nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID, ref string) (*CalendarEvent, error) {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("calendar: Cancel: %w", err)
	}
	if e == nil {
		return nil, ErrNotFound
	}
	if e.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, e.ID, StatusCancelled); err != nil {
		return nil, fmt.Errorf("calendar: Cancel: %w", err)
	}
	e.Status = StatusCancelled
	return e, nil
}

func (s *serviceImpl) RequestRSVP(ctx context.Context, orgID, ref, requestedBy string) (*CalendarEvent, error) {
	e, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("calendar: RequestRSVP: %w", err)
	}
	if e == nil {
		return nil, ErrNotFound
	}
	return s.requestRSVPInternal(ctx, orgID, e, requestedBy)
}

func (s *serviceImpl) requestRSVPInternal(ctx context.Context, orgID string, e *CalendarEvent, requestedBy string) (*CalendarEvent, error) {
	empIDs, err := s.repo.GetTargetEmployeeIDs(ctx, orgID, e.ScopeType, e.ScopeIDs)
	if err != nil {
		return e, nil
	} // non-fatal
	for _, empID := range empIDs {
		_, _ = s.db.Exec(ctx,
			`INSERT INTO hrm_acknowledgements
			(org_id, employee_id, acknowledgeable_type, acknowledgeable_id,
			 entity_title, signature_required, expires_at, status, requested_by)
			VALUES ($1,$2::uuid,'calendar_event',$3::uuid,$4,FALSE,$5::date,'pending',$6::uuid)
			ON CONFLICT (employee_id, acknowledgeable_type, acknowledgeable_id, status) DO NOTHING`,
			orgID, empID, e.ID, e.Title, e.RSVPDeadline, requestedBy)
	}
	return e, nil
}
