package holidays_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/holidays"
)

type stubRepo struct {
	calendars   map[string]*holidays.HolidayCalendar
	hols        map[string]*holidays.Holiday
	assignments map[string]*holidays.CalendarAssignment
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		calendars:   make(map[string]*holidays.HolidayCalendar),
		hols:        make(map[string]*holidays.Holiday),
		assignments: make(map[string]*holidays.CalendarAssignment),
	}
}

func (s *stubRepo) FindAllCalendars(ctx context.Context, orgID string, activeOnly bool) ([]*holidays.HolidayCalendar, error) {
	var list []*holidays.HolidayCalendar
	for _, c := range s.calendars {
		if c.OrgID == orgID {
			if !activeOnly || c.IsActive {
				list = append(list, c)
			}
		}
	}
	return list, nil
}

func (s *stubRepo) FindCalendarByRef(ctx context.Context, orgID, ref string) (*holidays.HolidayCalendar, error) {
	for _, c := range s.calendars {
		if c.OrgID == orgID && (c.ID == ref || c.PublicID == ref) {
			return c, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) CreateCalendar(ctx context.Context, c *holidays.HolidayCalendar) error {
	c.ID = "cal_" + time.Now().Format("20060102150405.000")
	s.calendars[c.ID] = c
	return nil
}

func (s *stubRepo) UpdateCalendar(ctx context.Context, c *holidays.HolidayCalendar) error {
	s.calendars[c.ID] = c
	return nil
}

func (s *stubRepo) DeleteCalendar(ctx context.Context, orgID, ref string) error {
	for id, c := range s.calendars {
		if c.OrgID == orgID && (c.ID == ref || c.PublicID == ref) {
			delete(s.calendars, id)
			return nil
		}
	}
	return holidays.ErrCalendarNotFound
}

func (s *stubRepo) CalendarNameExists(ctx context.Context, orgID, name string, year int, excludeID string) (bool, error) {
	for _, c := range s.calendars {
		if c.OrgID == orgID && c.Name == name && c.Year == year && c.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubRepo) FindHolidays(ctx context.Context, calendarID string) ([]*holidays.Holiday, error) {
	var list []*holidays.Holiday
	for _, h := range s.hols {
		if h.CalendarID == calendarID {
			list = append(list, h)
		}
	}
	return list, nil
}

func (s *stubRepo) FindHolidayByRef(ctx context.Context, calendarID, ref string) (*holidays.Holiday, error) {
	for _, h := range s.hols {
		if h.CalendarID == calendarID && (h.ID == ref || h.PublicID == ref) {
			return h, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) CreateHoliday(ctx context.Context, h *holidays.Holiday) error {
	h.ID = "hol_" + time.Now().Format("20060102150405.000")
	s.hols[h.ID] = h
	return nil
}

func (s *stubRepo) UpdateHoliday(ctx context.Context, h *holidays.Holiday) error {
	s.hols[h.ID] = h
	return nil
}

func (s *stubRepo) DeleteHoliday(ctx context.Context, calendarID, ref string) error {
	for id, h := range s.hols {
		if h.CalendarID == calendarID && (h.ID == ref || h.PublicID == ref) {
			delete(s.hols, id)
			return nil
		}
	}
	return holidays.ErrHolidayNotFound
}

func (s *stubRepo) FindAssignment(ctx context.Context, assigneeType, assigneeID string) (*holidays.CalendarAssignment, error) {
	for _, a := range s.assignments {
		if string(a.AssigneeType) == assigneeType && a.AssigneeID == assigneeID {
			return a, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) UpsertAssignment(ctx context.Context, a *holidays.CalendarAssignment) error {
	a.ID = "asg_" + time.Now().Format("20060102150405.000")
	s.assignments[a.ID] = a
	return nil
}

func (s *stubRepo) DeleteAssignment(ctx context.Context, orgID, ref string) error {
	for id, a := range s.assignments {
		if a.OrgID == orgID && (a.ID == ref || a.PublicID == ref) {
			delete(s.assignments, id)
			return nil
		}
	}
	return holidays.ErrAssignmentNotFound
}

func TestHolidayCalendarService(t *testing.T) {
	repo := newStubRepo()
	svc := holidays.NewService(repo)
	ctx := context.Background()

	orgID := "org_1"
	createdBy := "user_1"

	// Create Calendar
	req := holidays.CreateCalendarRequest{
		Name: "US Holidays",
		Year: 2024,
	}
	cal, err := svc.CreateCalendar(ctx, orgID, createdBy, req)
	if err != nil {
		t.Fatalf("CreateCalendar failed: %v", err)
	}
	if cal.Name != "US Holidays" {
		t.Errorf("Expected US Holidays, got %s", cal.Name)
	}

	// Update Calendar
	newName := "US Holidays 2024"
	updateReq := holidays.UpdateCalendarRequest{
		Name: &newName,
	}
	updated, err := svc.UpdateCalendar(ctx, orgID, cal.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateCalendar failed: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("Expected updated name")
	}

	// Create Holiday
	holReq := holidays.CreateHolidayRequest{
		Name:        "New Year",
		Date:        "2024-01-01",
		HolidayType: holidays.HolidayTypePublic,
		IsPaid:      true,
	}
	hol, err := svc.CreateHoliday(ctx, orgID, cal.ID, holReq)
	if err != nil {
		t.Fatalf("CreateHoliday failed: %v", err)
	}
	if hol.Name != "New Year" {
		t.Errorf("Expected New Year holiday")
	}

	// Assign Calendar
	assignReq := holidays.AssignCalendarRequest{
		CalendarID:    cal.ID,
		AssigneeType:  holidays.AssigneeOrganization,
		AssigneeID:    orgID,
		EffectiveDate: "2024-01-01",
	}
	asg, err := svc.AssignCalendar(ctx, orgID, createdBy, assignReq)
	if err != nil {
		t.Fatalf("AssignCalendar failed: %v", err)
	}
	if asg.CalendarID != cal.ID {
		t.Errorf("Assignment failed")
	}

	// List Holidays
	holList, err := svc.ListHolidays(ctx, orgID, cal.ID)
	if err != nil {
		t.Fatalf("ListHolidays failed: %v", err)
	}
	if holList.Total != 1 {
		t.Errorf("Expected 1 holiday")
	}

	// Delete Holiday
	err = svc.DeleteHoliday(ctx, orgID, cal.ID, hol.ID)
	if err != nil {
		t.Fatalf("DeleteHoliday failed: %v", err)
	}

	// Delete Assignment
	err = svc.RemoveAssignment(ctx, orgID, asg.ID)
	if err != nil {
		t.Fatalf("RemoveAssignment failed: %v", err)
	}

	// Delete Calendar
	err = svc.DeleteCalendar(ctx, orgID, cal.ID)
	if err != nil {
		t.Fatalf("DeleteCalendar failed: %v", err)
	}

	// Invalid Date
	holReqInvalid := holidays.CreateHolidayRequest{
		Name:        "Bad Date",
		Date:        "bad-date",
		HolidayType: holidays.HolidayTypePublic,
	}
	_, err = svc.CreateHoliday(ctx, orgID, cal.ID, holReqInvalid)
	if err != holidays.ErrInvalidDate && err != holidays.ErrCalendarNotFound {
		t.Errorf("Expected error for invalid date or calendar not found")
	}
}
