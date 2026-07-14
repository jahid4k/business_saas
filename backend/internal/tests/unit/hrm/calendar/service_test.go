package calendar_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/calendar"
)

type stubRepo struct {
	events map[string]*calendar.CalendarEvent
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		events: make(map[string]*calendar.CalendarEvent),
	}
}

func (s *stubRepo) FindAll(ctx context.Context, orgID, eventType, status string, fromDate, toDate string) ([]*calendar.CalendarEvent, error) {
	var list []*calendar.CalendarEvent
	for _, e := range s.events {
		if e.OrgID == orgID {
			if eventType != "" && string(e.EventType) != eventType {
				continue
			}
			if status != "" && string(e.Status) != status {
				continue
			}
			list = append(list, e)
		}
	}
	return list, nil
}

func (s *stubRepo) FindByRef(ctx context.Context, orgID, ref string) (*calendar.CalendarEvent, error) {
	for _, e := range s.events {
		if e.OrgID == orgID && (e.ID == ref || e.PublicID == ref) {
			return e, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) Create(ctx context.Context, e *calendar.CalendarEvent) error {
	e.ID = "evt_" + time.Now().Format("20060102150405.000")
	s.events[e.ID] = e
	return nil
}

func (s *stubRepo) Update(ctx context.Context, e *calendar.CalendarEvent) error {
	s.events[e.ID] = e
	return nil
}

func (s *stubRepo) UpdateStatus(ctx context.Context, id string, status calendar.EventStatus) error {
	if e, ok := s.events[id]; ok {
		e.Status = status
	}
	return nil
}

func (s *stubRepo) GetTargetEmployeeIDs(ctx context.Context, orgID string, scopeType calendar.ScopeType, scopeIDs []string) ([]string, error) {
	// Return empty to avoid nil pointer panic on db.Exec in RequestRSVP
	return []string{}, nil
}

func TestCalendarService(t *testing.T) {
	repo := newStubRepo()
	// pass nil for pgxpool because our stub repo returns empty TargetEmployeeIDs,
	// so the db.Exec loop will not run.
	svc := calendar.NewService(repo, nil)
	ctx := context.Background()

	orgID := "org_1"
	createdBy := "user_1"

	// Create Event
	req := calendar.CreateEventRequest{
		Title:        "Team Meeting",
		EventType:    calendar.TypeTeamEvent,
		StartDate:    "2023-11-01",
		EndDate:      "2023-11-01",
		RequiresRSVP: true,
	}
	evt, err := svc.Create(ctx, orgID, createdBy, req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if evt.Title != "Team Meeting" {
		t.Errorf("Expected title 'Team Meeting', got %s", evt.Title)
	}

	// Update Event
	newTitle := "Updated Team Meeting"
	updateReq := calendar.UpdateEventRequest{
		Title: &newTitle,
	}
	updated, err := svc.Update(ctx, orgID, evt.ID, updateReq)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("Expected updated title")
	}

	// List
	list, err := svc.List(ctx, orgID, string(calendar.TypeTeamEvent), "", "", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if list.Total != 1 {
		t.Errorf("Expected 1 event")
	}

	// Get
	fetched, err := svc.Get(ctx, orgID, evt.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.ID != evt.ID {
		t.Errorf("Mismatch ID")
	}

	// Request RSVP
	_, err = svc.RequestRSVP(ctx, orgID, evt.ID, createdBy)
	if err != nil {
		t.Fatalf("RequestRSVP failed: %v", err)
	}

	// Cancel
	cancelled, err := svc.Cancel(ctx, orgID, evt.ID)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if cancelled.Status != calendar.StatusCancelled {
		t.Errorf("Expected status cancelled")
	}

	// Invalid Create
	invalidReq := calendar.CreateEventRequest{
		Title:     "", // empty title
		EventType: calendar.TypeTeamEvent,
	}
	_, err = svc.Create(ctx, orgID, createdBy, invalidReq)
	if err != calendar.ErrTitleRequired {
		t.Errorf("Expected ErrTitleRequired")
	}
}
