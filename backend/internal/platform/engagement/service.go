// backend/internal/platform/engagement/service.go
package engagement

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Service defines the business logic for all engagement records.
// Every method that creates a record takes a module string so the record
// is tagged with which module created it. This enables per-module filtering
// and the unified cross-module timeline.
type Service interface {
	// Timeline — unified view for any entity
	GetTimeline(ctx context.Context, orgID, relatedType, relatedID string) (*TimelineResponse, error)

	// Notes
	ListNotesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Note, error)
	GetNote(ctx context.Context, orgID, noteID string) (*Note, error)
	CreateNote(ctx context.Context, orgID, userID, module string, req CreateNoteRequest) (*Note, error)
	UpdateNote(ctx context.Context, orgID, noteID string, req UpdateNoteRequest) (*Note, error)
	DeleteNote(ctx context.Context, orgID, noteID string) error

	// Tasks
	ListTasksByOrg(ctx context.Context, orgID string) (*TaskListResponse, error)
	ListTasksByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Task, error)
	GetTask(ctx context.Context, orgID, taskID string) (*Task, error)
	CreateTask(ctx context.Context, orgID, userID, module string, req CreateTaskRequest) (*Task, error)
	UpdateTask(ctx context.Context, orgID, taskID string, req UpdateTaskRequest) (*Task, error)
	DeleteTask(ctx context.Context, orgID, taskID string) error
	CompleteTask(ctx context.Context, orgID, taskID string) (*Task, error)
	ReopenTask(ctx context.Context, orgID, taskID string) (*Task, error)
	AssignTask(ctx context.Context, orgID, taskID string, req AssignTaskRequest) (*Task, error)
	GetOverdueTasks(ctx context.Context, orgID string) ([]*Task, error)

	// Activities
	ListActivitiesByOrg(ctx context.Context, orgID string) (*ActivityListResponse, error)
	ListActivitiesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Activity, error)
	GetActivity(ctx context.Context, orgID, activityID string) (*Activity, error)
	CreateActivity(ctx context.Context, orgID, userID, module string, req CreateActivityRequest) (*Activity, error)
	UpdateActivity(ctx context.Context, orgID, activityID string, req UpdateActivityRequest) (*Activity, error)
	DeleteActivity(ctx context.Context, orgID, activityID string) error
	GetActivityCountByType(ctx context.Context, orgID string) (map[string]int, error)

	// Email Logs
	ListEmailLogsByOrg(ctx context.Context, orgID string) (*EmailLogListResponse, error)
	ListEmailLogsByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*EmailLog, error)
	GetEmailLog(ctx context.Context, orgID, emailID string) (*EmailLog, error)
	CreateEmailLog(ctx context.Context, orgID, userID, module string, req CreateEmailLogRequest) (*EmailLog, error)
	DeleteEmailLog(ctx context.Context, orgID, emailID string) error
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new engagement service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

// ============================================================
// Helpers
// ============================================================

// parseRelatedType converts the raw string from a request into the typed
// RelatedType constant and validates it against the registered values.
// Returns ErrRelatedTypeRequired if empty, ErrInvalidRelatedType if unrecognised.
func parseRelatedType(raw string) (RelatedType, error) {
	if strings.TrimSpace(raw) == "" {
		return "", ErrRelatedTypeRequired
	}
	rt := RelatedType(raw)
	if !rt.IsValid() {
		return "", ErrInvalidRelatedType
	}
	return rt, nil
}

// ============================================================
// Timeline
// ============================================================

// GetTimeline returns a unified chronological view of all engagement records
// for a given entity. Used for the activity feed on any contact, deal, lead, etc.
func (s *serviceImpl) GetTimeline(ctx context.Context, orgID, relatedType, relatedID string) (*TimelineResponse, error) {
	notes, err := s.repo.FindNotesByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetTimeline: notes: %w", err)
	}
	tasks, err := s.repo.FindTasksByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetTimeline: tasks: %w", err)
	}
	activities, err := s.repo.FindActivitiesByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetTimeline: activities: %w", err)
	}
	emails, err := s.repo.FindEmailLogsByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetTimeline: emails: %w", err)
	}

	var entries []*TimelineEntry
	for _, n := range notes {
		entries = append(entries, &TimelineEntry{
			Type: "note", ID: n.ID, Module: n.Module,
			Timestamp: n.CreatedAt, Data: n,
		})
	}
	for _, t := range tasks {
		entries = append(entries, &TimelineEntry{
			Type: "task", ID: t.ID, Module: t.Module,
			Timestamp: t.CreatedAt, Data: t,
		})
	}
	for _, a := range activities {
		entries = append(entries, &TimelineEntry{
			Type: "activity", ID: a.ID, Module: a.Module,
			Timestamp: a.OccurredAt, Data: a,
		})
	}
	for _, e := range emails {
		entries = append(entries, &TimelineEntry{
			Type: "email", ID: e.ID, Module: e.Module,
			Timestamp: e.CreatedAt, Data: e,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if entries == nil {
		entries = []*TimelineEntry{}
	}
	return &TimelineResponse{Entries: entries, Total: len(entries)}, nil
}

// ============================================================
// Notes
// ============================================================

func (s *serviceImpl) ListNotesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Note, error) {
	notes, err := s.repo.FindNotesByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListNotesByRelated: %w", err)
	}
	if notes == nil {
		notes = []*Note{}
	}
	return notes, nil
}

func (s *serviceImpl) GetNote(ctx context.Context, orgID, noteID string) (*Note, error) {
	n, err := s.repo.FindNoteByID(ctx, orgID, noteID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetNote: %w", err)
	}
	if n == nil {
		return nil, ErrNoteNotFound
	}
	return n, nil
}

func (s *serviceImpl) CreateNote(ctx context.Context, orgID, userID, module string, req CreateNoteRequest) (*Note, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, ErrContentRequired
	}
	if strings.TrimSpace(req.RelatedID) == "" {
		return nil, ErrRelatedIDRequired
	}
	rt, err := parseRelatedType(req.RelatedType)
	if err != nil {
		return nil, err
	}
	n := &Note{
		OrgID:       orgID,
		Module:      module,
		Content:     strings.TrimSpace(req.Content),
		RelatedType: rt,
		RelatedID:   req.RelatedID,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateNote(ctx, n); err != nil {
		return nil, fmt.Errorf("engagement: CreateNote: %w", err)
	}
	return n, nil
}

func (s *serviceImpl) UpdateNote(ctx context.Context, orgID, noteID string, req UpdateNoteRequest) (*Note, error) {
	n, err := s.repo.FindNoteByID(ctx, orgID, noteID)
	if err != nil {
		return nil, fmt.Errorf("engagement: UpdateNote: %w", err)
	}
	if n == nil {
		return nil, ErrNoteNotFound
	}
	if req.Content != nil && strings.TrimSpace(*req.Content) != "" {
		n.Content = strings.TrimSpace(*req.Content)
	}
	if err := s.repo.UpdateNote(ctx, n); err != nil {
		return nil, fmt.Errorf("engagement: UpdateNote: %w", err)
	}
	return n, nil
}

func (s *serviceImpl) DeleteNote(ctx context.Context, orgID, noteID string) error {
	return s.repo.DeleteNote(ctx, orgID, noteID)
}

// ============================================================
// Tasks
// ============================================================

func (s *serviceImpl) ListTasksByOrg(ctx context.Context, orgID string) (*TaskListResponse, error) {
	tasks, err := s.repo.FindTasksByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListTasksByOrg: %w", err)
	}
	if tasks == nil {
		tasks = []*Task{}
	}
	total, err := s.repo.CountTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListTasksByOrg: count: %w", err)
	}
	return &TaskListResponse{Tasks: tasks, Total: total}, nil
}

func (s *serviceImpl) ListTasksByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Task, error) {
	tasks, err := s.repo.FindTasksByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListTasksByRelated: %w", err)
	}
	if tasks == nil {
		tasks = []*Task{}
	}
	return tasks, nil
}

func (s *serviceImpl) GetTask(ctx context.Context, orgID, taskID string) (*Task, error) {
	t, err := s.repo.FindTaskByID(ctx, orgID, taskID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetTask: %w", err)
	}
	if t == nil {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

func (s *serviceImpl) CreateTask(ctx context.Context, orgID, userID, module string, req CreateTaskRequest) (*Task, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, ErrTitleRequired
	}
	if strings.TrimSpace(req.RelatedID) == "" {
		return nil, ErrRelatedIDRequired
	}
	rt, err := parseRelatedType(req.RelatedType)
	if err != nil {
		return nil, err
	}
	priority := TaskPriorityMedium
	if req.Priority != nil && *req.Priority != "" {
		p := TaskPriority(*req.Priority)
		if !p.IsValid() {
			return nil, ErrInvalidPriority
		}
		priority = p
	}
	t := &Task{
		OrgID:       orgID,
		Module:      module,
		Title:       strings.TrimSpace(req.Title),
		Description: req.Description,
		Status:      TaskStatusOpen,
		Priority:    priority,
		RelatedType: rt,
		RelatedID:   req.RelatedID,
		AssignedTo:  req.AssignedTo,
		CreatedBy:   userID,
	}
	if req.DueDate != nil && *req.DueDate != "" {
		due, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			return nil, fmt.Errorf("engagement: CreateTask: invalid due_date (use RFC3339): %w", err)
		}
		t.DueDate = &due
	}
	if err := s.repo.CreateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("engagement: CreateTask: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) UpdateTask(ctx context.Context, orgID, taskID string, req UpdateTaskRequest) (*Task, error) {
	t, err := s.repo.FindTaskByID(ctx, orgID, taskID)
	if err != nil {
		return nil, fmt.Errorf("engagement: UpdateTask: %w", err)
	}
	if t == nil {
		return nil, ErrTaskNotFound
	}
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		t.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		t.Description = req.Description
	}
	if req.Priority != nil {
		p := TaskPriority(*req.Priority)
		if !p.IsValid() {
			return nil, ErrInvalidPriority
		}
		t.Priority = p
	}
	if req.AssignedTo != nil {
		t.AssignedTo = req.AssignedTo
	}
	if req.DueDate != nil && *req.DueDate != "" {
		due, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			return nil, fmt.Errorf("engagement: UpdateTask: invalid due_date: %w", err)
		}
		t.DueDate = &due
	}
	if err := s.repo.UpdateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("engagement: UpdateTask: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) DeleteTask(ctx context.Context, orgID, taskID string) error {
	return s.repo.DeleteTask(ctx, orgID, taskID)
}

func (s *serviceImpl) CompleteTask(ctx context.Context, orgID, taskID string) (*Task, error) {
	t, err := s.repo.FindTaskByID(ctx, orgID, taskID)
	if err != nil {
		return nil, fmt.Errorf("engagement: CompleteTask: %w", err)
	}
	if t == nil {
		return nil, ErrTaskNotFound
	}
	now := time.Now()
	t.Status = TaskStatusCompleted
	t.CompletedAt = &now
	if err := s.repo.UpdateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("engagement: CompleteTask: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) ReopenTask(ctx context.Context, orgID, taskID string) (*Task, error) {
	t, err := s.repo.FindTaskByID(ctx, orgID, taskID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ReopenTask: %w", err)
	}
	if t == nil {
		return nil, ErrTaskNotFound
	}
	t.Status = TaskStatusOpen
	t.CompletedAt = nil
	if err := s.repo.UpdateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("engagement: ReopenTask: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) AssignTask(ctx context.Context, orgID, taskID string, req AssignTaskRequest) (*Task, error) {
	if strings.TrimSpace(req.AssignedTo) == "" {
		return nil, fmt.Errorf("engagement: AssignTask: assigned_to is required")
	}
	t, err := s.repo.FindTaskByID(ctx, orgID, taskID)
	if err != nil {
		return nil, fmt.Errorf("engagement: AssignTask: %w", err)
	}
	if t == nil {
		return nil, ErrTaskNotFound
	}
	t.AssignedTo = &req.AssignedTo
	if err := s.repo.UpdateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("engagement: AssignTask: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) GetOverdueTasks(ctx context.Context, orgID string) ([]*Task, error) {
	tasks, err := s.repo.FindOverdueTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetOverdueTasks: %w", err)
	}
	if tasks == nil {
		tasks = []*Task{}
	}
	return tasks, nil
}

// ============================================================
// Activities
// ============================================================

func (s *serviceImpl) ListActivitiesByOrg(ctx context.Context, orgID string) (*ActivityListResponse, error) {
	activities, err := s.repo.FindActivitiesByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListActivitiesByOrg: %w", err)
	}
	if activities == nil {
		activities = []*Activity{}
	}
	total, err := s.repo.CountActivities(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListActivitiesByOrg: count: %w", err)
	}
	return &ActivityListResponse{Activities: activities, Total: total}, nil
}

func (s *serviceImpl) ListActivitiesByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*Activity, error) {
	activities, err := s.repo.FindActivitiesByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListActivitiesByRelated: %w", err)
	}
	if activities == nil {
		activities = []*Activity{}
	}
	return activities, nil
}

func (s *serviceImpl) GetActivity(ctx context.Context, orgID, activityID string) (*Activity, error) {
	a, err := s.repo.FindActivityByID(ctx, orgID, activityID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetActivity: %w", err)
	}
	if a == nil {
		return nil, ErrActivityNotFound
	}
	return a, nil
}

func (s *serviceImpl) CreateActivity(ctx context.Context, orgID, userID, module string, req CreateActivityRequest) (*Activity, error) {
	if strings.TrimSpace(req.Type) == "" {
		return nil, ErrTypeRequired
	}
	aType := ActivityType(req.Type)
	if !aType.IsValid() {
		return nil, ErrInvalidActivityType
	}
	if strings.TrimSpace(req.Subject) == "" {
		return nil, ErrSubjectRequired
	}
	if strings.TrimSpace(req.RelatedID) == "" {
		return nil, ErrRelatedIDRequired
	}
	rt, err := parseRelatedType(req.RelatedType)
	if err != nil {
		return nil, err
	}
	occurredAt := time.Now()
	if req.OccurredAt != nil && *req.OccurredAt != "" {
		t, err := time.Parse(time.RFC3339, *req.OccurredAt)
		if err != nil {
			return nil, fmt.Errorf("engagement: CreateActivity: invalid occurred_at: %w", err)
		}
		occurredAt = t
	}
	a := &Activity{
		OrgID:        orgID,
		Module:       module,
		Type:         aType,
		Subject:      strings.TrimSpace(req.Subject),
		Description:  req.Description,
		Outcome:      req.Outcome,
		RelatedType:  rt,
		RelatedID:    req.RelatedID,
		OccurredAt:   occurredAt,
		DurationMins: req.DurationMins,
		CreatedBy:    userID,
	}
	if err := s.repo.CreateActivity(ctx, a); err != nil {
		return nil, fmt.Errorf("engagement: CreateActivity: %w", err)
	}
	return a, nil
}

func (s *serviceImpl) UpdateActivity(ctx context.Context, orgID, activityID string, req UpdateActivityRequest) (*Activity, error) {
	a, err := s.repo.FindActivityByID(ctx, orgID, activityID)
	if err != nil {
		return nil, fmt.Errorf("engagement: UpdateActivity: %w", err)
	}
	if a == nil {
		return nil, ErrActivityNotFound
	}
	if req.Type != nil {
		aType := ActivityType(*req.Type)
		if !aType.IsValid() {
			return nil, ErrInvalidActivityType
		}
		a.Type = aType
	}
	if req.Subject != nil && strings.TrimSpace(*req.Subject) != "" {
		a.Subject = strings.TrimSpace(*req.Subject)
	}
	if req.Description != nil {
		a.Description = req.Description
	}
	if req.Outcome != nil {
		a.Outcome = req.Outcome
	}
	if req.OccurredAt != nil && *req.OccurredAt != "" {
		t, err := time.Parse(time.RFC3339, *req.OccurredAt)
		if err != nil {
			return nil, fmt.Errorf("engagement: UpdateActivity: invalid occurred_at: %w", err)
		}
		a.OccurredAt = t
	}
	if req.DurationMins != nil {
		a.DurationMins = req.DurationMins
	}
	if err := s.repo.UpdateActivity(ctx, a); err != nil {
		return nil, fmt.Errorf("engagement: UpdateActivity: %w", err)
	}
	return a, nil
}

func (s *serviceImpl) DeleteActivity(ctx context.Context, orgID, activityID string) error {
	return s.repo.DeleteActivity(ctx, orgID, activityID)
}

func (s *serviceImpl) GetActivityCountByType(ctx context.Context, orgID string) (map[string]int, error) {
	result, err := s.repo.GetActivityCountByType(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetActivityCountByType: %w", err)
	}
	if result == nil {
		result = map[string]int{}
	}
	return result, nil
}

// ============================================================
// Email Logs
// ============================================================

func (s *serviceImpl) ListEmailLogsByOrg(ctx context.Context, orgID string) (*EmailLogListResponse, error) {
	logs, err := s.repo.FindEmailLogsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListEmailLogsByOrg: %w", err)
	}
	if logs == nil {
		logs = []*EmailLog{}
	}
	total, err := s.repo.CountEmailLogs(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListEmailLogsByOrg: count: %w", err)
	}
	return &EmailLogListResponse{EmailLogs: logs, Total: total}, nil
}

func (s *serviceImpl) ListEmailLogsByRelated(ctx context.Context, orgID, relatedType, relatedID string) ([]*EmailLog, error) {
	logs, err := s.repo.FindEmailLogsByRelated(ctx, orgID, relatedType, relatedID)
	if err != nil {
		return nil, fmt.Errorf("engagement: ListEmailLogsByRelated: %w", err)
	}
	if logs == nil {
		logs = []*EmailLog{}
	}
	return logs, nil
}

func (s *serviceImpl) GetEmailLog(ctx context.Context, orgID, emailID string) (*EmailLog, error) {
	e, err := s.repo.FindEmailLogByID(ctx, orgID, emailID)
	if err != nil {
		return nil, fmt.Errorf("engagement: GetEmailLog: %w", err)
	}
	if e == nil {
		return nil, ErrEmailLogNotFound
	}
	return e, nil
}

func (s *serviceImpl) CreateEmailLog(ctx context.Context, orgID, userID, module string, req CreateEmailLogRequest) (*EmailLog, error) {
	if strings.TrimSpace(req.Subject) == "" {
		return nil, ErrSubjectRequired
	}
	if strings.TrimSpace(req.FromEmail) == "" {
		return nil, ErrFromEmailRequired
	}
	if strings.TrimSpace(req.ToEmail) == "" {
		return nil, ErrToEmailRequired
	}
	if strings.TrimSpace(req.RelatedID) == "" {
		return nil, ErrRelatedIDRequired
	}
	rt, err := parseRelatedType(req.RelatedType)
	if err != nil {
		return nil, err
	}
	direction := "outbound"
	if req.Direction != nil && *req.Direction != "" {
		direction = *req.Direction
	}
	status := "sent"
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}
	now := time.Now()
	e := &EmailLog{
		OrgID:       orgID,
		Module:      module,
		Subject:     strings.TrimSpace(req.Subject),
		Body:        req.Body,
		FromEmail:   strings.TrimSpace(req.FromEmail),
		ToEmail:     strings.TrimSpace(req.ToEmail),
		Direction:   direction,
		Status:      status,
		RelatedType: rt,
		RelatedID:   req.RelatedID,
		SentAt:      &now,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateEmailLog(ctx, e); err != nil {
		return nil, fmt.Errorf("engagement: CreateEmailLog: %w", err)
	}
	return e, nil
}

func (s *serviceImpl) DeleteEmailLog(ctx context.Context, orgID, emailID string) error {
	return s.repo.DeleteEmailLog(ctx, orgID, emailID)
}
