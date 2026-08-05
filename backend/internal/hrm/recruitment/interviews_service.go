// backend/internal/hrm/recruitment/interviews_service.go
package recruitment

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// InterviewService is embedded into Service — see service.go.
type InterviewService interface {
	ListInterviews(ctx context.Context, orgID, applicationID string) ([]*Interview, error)
	GetInterview(ctx context.Context, orgID, ref string) (*Interview, error)
	CreateInterview(ctx context.Context, orgID, applicationRef, createdBy string, req CreateInterviewRequest) (*Interview, error)
	UpdateInterview(ctx context.Context, orgID, ref string, req UpdateInterviewRequest) (*Interview, error)
	DeleteInterview(ctx context.Context, orgID, ref string) error

	ListPanelists(ctx context.Context, orgID, interviewRef string) ([]*Panelist, error)
	AddPanelist(ctx context.Context, orgID, interviewRef string, req AddPanelistRequest) (*Panelist, error)
	RemovePanelist(ctx context.Context, orgID, interviewRef, employeeID string) error
}

func (s *serviceImpl) ListInterviews(ctx context.Context, orgID, applicationID string) ([]*Interview, error) {
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListInterviews: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	list, err := s.repo.FindInterviews(ctx, orgID, app.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListInterviews: %w", err)
	}
	if list == nil {
		list = []*Interview{}
	}
	return list, nil
}

func (s *serviceImpl) GetInterview(ctx context.Context, orgID, ref string) (*Interview, error) {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetInterview: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}
	panelists, err := s.repo.FindPanelists(ctx, i.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetInterview: panelists: %w", err)
	}
	i.Panelists = panelists
	return i, nil
}

func (s *serviceImpl) CreateInterview(ctx context.Context, orgID, applicationRef, createdBy string, req CreateInterviewRequest) (*Interview, error) {
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateInterview: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}

	scheduledAtStr := strings.TrimSpace(req.ScheduledAt)
	if scheduledAtStr == "" {
		return nil, ErrScheduledAtRequired
	}
	scheduledAt, err := time.Parse(time.RFC3339, scheduledAtStr)
	if err != nil {
		return nil, ErrInvalidScheduledAt
	}

	mode := InterviewModeVideo
	if req.Mode != nil && strings.TrimSpace(*req.Mode) != "" {
		mode = InterviewMode(strings.TrimSpace(*req.Mode))
		if !mode.IsValid() {
			return nil, ErrInvalidInterviewMode
		}
	}

	duration := 60
	if req.DurationMinutes != nil && *req.DurationMinutes > 0 {
		duration = *req.DurationMinutes
	}

	i := &Interview{
		OrgID: orgID, ApplicationID: app.ID, ScheduledAt: scheduledAt, DurationMinutes: duration,
		Mode: mode, Location: req.Location, MeetingURL: req.MeetingURL, Notes: req.Notes, CreatedBy: createdBy,
	}
	if err := s.repo.CreateInterview(ctx, i); err != nil {
		return nil, fmt.Errorf("recruitment: CreateInterview: %w", err)
	}
	return i, nil
}

func (s *serviceImpl) UpdateInterview(ctx context.Context, orgID, ref string, req UpdateInterviewRequest) (*Interview, error) {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateInterview: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}

	if req.ScheduledAt != nil {
		scheduledAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ScheduledAt))
		if err != nil {
			return nil, ErrInvalidScheduledAt
		}
		i.ScheduledAt = scheduledAt
	}
	if req.DurationMinutes != nil && *req.DurationMinutes > 0 {
		i.DurationMinutes = *req.DurationMinutes
	}
	if req.Mode != nil {
		mode := InterviewMode(strings.TrimSpace(*req.Mode))
		if !mode.IsValid() {
			return nil, ErrInvalidInterviewMode
		}
		i.Mode = mode
	}
	if req.Location != nil {
		i.Location = req.Location
	}
	if req.MeetingURL != nil {
		i.MeetingURL = req.MeetingURL
	}
	if req.Status != nil {
		status := InterviewStatus(strings.TrimSpace(*req.Status))
		if !status.IsValid() {
			return nil, ErrInvalidInterviewStatus
		}
		i.Status = status
	}
	if req.Outcome != nil {
		if strings.TrimSpace(*req.Outcome) == "" {
			i.Outcome = nil
		} else {
			outcome := InterviewOutcome(strings.TrimSpace(*req.Outcome))
			if !outcome.IsValid() {
				return nil, ErrInvalidInterviewOutcome
			}
			i.Outcome = &outcome
		}
	}
	if req.Notes != nil {
		i.Notes = req.Notes
	}

	if err := s.repo.UpdateInterview(ctx, i); err != nil {
		return nil, fmt.Errorf("recruitment: UpdateInterview: %w", err)
	}
	return i, nil
}

func (s *serviceImpl) DeleteInterview(ctx context.Context, orgID, ref string) error {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("recruitment: DeleteInterview: %w", err)
	}
	if i == nil {
		return ErrInterviewNotFound
	}
	if err := s.repo.DeleteInterview(ctx, orgID, i.ID); err != nil {
		return fmt.Errorf("recruitment: DeleteInterview: %w", err)
	}
	return nil
}

func (s *serviceImpl) ListPanelists(ctx context.Context, orgID, interviewRef string) ([]*Panelist, error) {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, interviewRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListPanelists: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}
	list, err := s.repo.FindPanelists(ctx, i.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListPanelists: %w", err)
	}
	if list == nil {
		list = []*Panelist{}
	}
	return list, nil
}

func (s *serviceImpl) AddPanelist(ctx context.Context, orgID, interviewRef string, req AddPanelistRequest) (*Panelist, error) {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, interviewRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: AddPanelist: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, ErrPanelistEmployeeID
	}

	existing, err := s.repo.FindPanelist(ctx, i.ID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: AddPanelist: check: %w", err)
	}
	if existing != nil {
		return nil, ErrPanelistAlreadyOnPanel
	}

	isLead := false
	if req.IsLead != nil {
		isLead = *req.IsLead
	}
	p := &Panelist{InterviewID: i.ID, EmployeeID: employeeID, PanelistRole: req.PanelistRole, IsLead: isLead}
	if err := s.repo.AddPanelist(ctx, p); err != nil {
		return nil, fmt.Errorf("recruitment: AddPanelist: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) RemovePanelist(ctx context.Context, orgID, interviewRef, employeeID string) error {
	i, err := s.repo.FindInterviewByRef(ctx, orgID, interviewRef)
	if err != nil {
		return fmt.Errorf("recruitment: RemovePanelist: %w", err)
	}
	if i == nil {
		return ErrInterviewNotFound
	}
	if err := s.repo.RemovePanelist(ctx, i.ID, employeeID); err != nil {
		return fmt.Errorf("recruitment: RemovePanelist: %w", err)
	}
	return nil
}
