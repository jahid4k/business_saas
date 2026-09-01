// backend/internal/hrm/holidays/service.go
package holidays

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

type Service interface {
	ListCalendars(ctx context.Context, orgID string, activeOnly bool) (*CalendarListResponse, error)
	GetCalendar(ctx context.Context, orgID, ref string) (*HolidayCalendar, error)
	CreateCalendar(ctx context.Context, orgID, createdBy string, req CreateCalendarRequest) (*HolidayCalendar, error)
	UpdateCalendar(ctx context.Context, orgID, ref string, req UpdateCalendarRequest) (*HolidayCalendar, error)
	DeleteCalendar(ctx context.Context, orgID, ref string) error

	ListHolidays(ctx context.Context, orgID, calendarRef string) (*HolidayListResponse, error)
	CreateHoliday(ctx context.Context, orgID, calendarRef string, req CreateHolidayRequest) (*Holiday, error)
	UpdateHoliday(ctx context.Context, orgID, calendarRef, holidayRef string, req UpdateHolidayRequest) (*Holiday, error)
	DeleteHoliday(ctx context.Context, orgID, calendarRef, holidayRef string) error

	AssignCalendar(ctx context.Context, orgID, createdBy string, req AssignCalendarRequest) (*CalendarAssignment, error)
	RemoveAssignment(ctx context.Context, orgID, ref string) error
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

func (s *serviceImpl) ListCalendars(ctx context.Context, orgID string, activeOnly bool) (*CalendarListResponse, error) {
	list, err := s.repo.FindAllCalendars(ctx, orgID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("holidays: ListCalendars: %w", err)
	}
	if list == nil {
		list = []*HolidayCalendar{}
	}
	return &CalendarListResponse{Calendars: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetCalendar(ctx context.Context, orgID, ref string) (*HolidayCalendar, error) {
	c, err := s.repo.FindCalendarByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("holidays: GetCalendar: %w", err)
	}
	if c == nil {
		return nil, ErrCalendarNotFound
	}
	h, err := s.repo.FindHolidays(ctx, c.ID)
	if err != nil {
		return nil, fmt.Errorf("holidays: GetCalendar holidays: %w", err)
	}
	c.Holidays = h
	return c, nil
}

func (s *serviceImpl) CreateCalendar(ctx context.Context, orgID, createdBy string, req CreateCalendarRequest) (*HolidayCalendar, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if req.Year < 2000 || req.Year > 2100 {
		return nil, ErrInvalidYear
	}
	if exists, _ := s.repo.CalendarNameExists(ctx, orgID, name, req.Year, ""); exists {
		return nil, ErrNameConflict
	}
	c := &HolidayCalendar{OrgID: orgID, Name: name, Description: req.Description, CountryCode: req.CountryCode, Year: req.Year, IsActive: true, CreatedBy: createdBy}
	if err := s.repo.CreateCalendar(ctx, c); err != nil {
		return nil, fmt.Errorf("holidays: CreateCalendar: %w", err)
	}
	c.Holidays = []*Holiday{}
	return c, nil
}

func (s *serviceImpl) UpdateCalendar(ctx context.Context, orgID, ref string, req UpdateCalendarRequest) (*HolidayCalendar, error) {
	c, err := s.repo.FindCalendarByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("holidays: UpdateCalendar: %w", err)
	}
	if c == nil {
		return nil, ErrCalendarNotFound
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return nil, ErrNameRequired
		}
		if exists, _ := s.repo.CalendarNameExists(ctx, orgID, n, c.Year, c.ID); exists {
			return nil, ErrNameConflict
		}
		c.Name = n
	}
	if req.Description != nil {
		c.Description = req.Description
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateCalendar(ctx, c); err != nil {
		return nil, fmt.Errorf("holidays: UpdateCalendar: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) DeleteCalendar(ctx context.Context, orgID, ref string) error {
	return s.repo.DeleteCalendar(ctx, orgID, ref)
}

func (s *serviceImpl) ListHolidays(ctx context.Context, orgID, calendarRef string) (*HolidayListResponse, error) {
	c, err := s.repo.FindCalendarByRef(ctx, orgID, calendarRef)
	if err != nil {
		return nil, fmt.Errorf("holidays: ListHolidays: %w", err)
	}
	if c == nil {
		return nil, ErrCalendarNotFound
	}
	list, err := s.repo.FindHolidays(ctx, c.ID)
	if err != nil {
		return nil, fmt.Errorf("holidays: ListHolidays: %w", err)
	}
	if list == nil {
		list = []*Holiday{}
	}
	return &HolidayListResponse{Holidays: list, Total: len(list)}, nil
}

func (s *serviceImpl) CreateHoliday(ctx context.Context, orgID, calendarRef string, req CreateHolidayRequest) (*Holiday, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrNameRequired
	}
	if strings.TrimSpace(req.Date) == "" {
		return nil, ErrDateRequired
	}
	if _, err := time.Parse(dateLayout, req.Date); err != nil {
		return nil, ErrInvalidDate
	}
	if !req.HolidayType.IsValid() {
		return nil, ErrInvalidHolidayType
	}
	c, err := s.repo.FindCalendarByRef(ctx, orgID, calendarRef)
	if err != nil || c == nil {
		return nil, ErrCalendarNotFound
	}
	h := &Holiday{CalendarID: c.ID, Name: strings.TrimSpace(req.Name), Date: req.Date, HolidayType: req.HolidayType, IsPaid: req.IsPaid, RepeatYearly: req.RepeatYearly}
	if err := s.repo.CreateHoliday(ctx, h); err != nil {
		if strings.Contains(err.Error(), "uq_hrm_hd_calendar_date") {
			return nil, ErrDateConflict
		}
		return nil, fmt.Errorf("holidays: CreateHoliday: %w", err)
	}
	return h, nil
}

func (s *serviceImpl) UpdateHoliday(ctx context.Context, orgID, calendarRef, holidayRef string, req UpdateHolidayRequest) (*Holiday, error) {
	c, err := s.repo.FindCalendarByRef(ctx, orgID, calendarRef)
	if err != nil || c == nil {
		return nil, ErrCalendarNotFound
	}
	h, err := s.repo.FindHolidayByRef(ctx, c.ID, holidayRef)
	if err != nil {
		return nil, fmt.Errorf("holidays: UpdateHoliday: %w", err)
	}
	if h == nil {
		return nil, ErrHolidayNotFound
	}
	if req.Name != nil {
		h.Name = strings.TrimSpace(*req.Name)
	}
	if req.Date != nil {
		if _, err := time.Parse(dateLayout, *req.Date); err != nil {
			return nil, ErrInvalidDate
		}
		h.Date = *req.Date
	}
	if req.HolidayType != nil {
		if !req.HolidayType.IsValid() {
			return nil, ErrInvalidHolidayType
		}
		h.HolidayType = *req.HolidayType
	}
	if req.IsPaid != nil {
		h.IsPaid = *req.IsPaid
	}
	if req.RepeatYearly != nil {
		h.RepeatYearly = *req.RepeatYearly
	}
	if err := s.repo.UpdateHoliday(ctx, h); err != nil {
		return nil, fmt.Errorf("holidays: UpdateHoliday: %w", err)
	}
	return h, nil
}

func (s *serviceImpl) DeleteHoliday(ctx context.Context, orgID, calendarRef, holidayRef string) error {
	c, err := s.repo.FindCalendarByRef(ctx, orgID, calendarRef)
	if err != nil || c == nil {
		return ErrCalendarNotFound
	}
	return s.repo.DeleteHoliday(ctx, c.ID, holidayRef)
}

func (s *serviceImpl) AssignCalendar(ctx context.Context, orgID, createdBy string, req AssignCalendarRequest) (*CalendarAssignment, error) {
	if !req.AssigneeType.IsValid() {
		return nil, ErrInvalidAssigneeType
	}
	if strings.TrimSpace(req.AssigneeID) == "" {
		return nil, ErrAssigneeIDRequired
	}
	if strings.TrimSpace(req.EffectiveDate) == "" {
		return nil, ErrEffectiveDateRequired
	}
	if _, err := time.Parse(dateLayout, req.EffectiveDate); err != nil {
		return nil, ErrEffectiveDateRequired
	}
	cal, err := s.repo.FindCalendarByRef(ctx, orgID, req.CalendarID)
	if err != nil || cal == nil {
		return nil, ErrCalendarNotFound
	}
	a := &CalendarAssignment{OrgID: orgID, CalendarID: cal.ID, AssigneeType: req.AssigneeType, AssigneeID: req.AssigneeID, EffectiveDate: req.EffectiveDate, CreatedBy: createdBy}
	if err := s.repo.UpsertAssignment(ctx, a); err != nil {
		return nil, fmt.Errorf("holidays: AssignCalendar: %w", err)
	}
	return a, nil
}

func (s *serviceImpl) RemoveAssignment(ctx context.Context, orgID, ref string) error {
	return s.repo.DeleteAssignment(ctx, orgID, ref)
}
