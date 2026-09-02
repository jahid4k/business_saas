// backend/internal/hrm/complaints/service.go
package complaints

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

type Service interface {
	List(ctx context.Context, orgID string, filter ComplaintListFilter) (*ComplaintListResponse, error)
	Get(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error)
	Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateComplaintRequest) (*Complaint, error)
	Update(ctx context.Context, orgID, employeeID, ref string, req UpdateComplaintRequest) (*Complaint, error)
	StartReview(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error)
	Assign(ctx context.Context, orgID, employeeID, ref string, req AssignRequest) (*Complaint, error)
	Resolve(ctx context.Context, orgID, employeeID, ref, resolvedBy string, req ResolveRequest) (*Complaint, error)
	Dismiss(ctx context.Context, orgID, employeeID, ref, resolvedBy string, req DismissRequest) (*Complaint, error)
	Withdraw(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) List(ctx context.Context, orgID string, filter ComplaintListFilter) (*ComplaintListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindAll(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("complaints: List: %w", err)
	}
	if list == nil {
		list = []*Complaint{}
	}
	total, err := s.repo.Count(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("complaints: List: count: %w", err)
	}
	return &ComplaintListResponse{Complaints: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error) {
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("complaints: Get: %w", err)
	}
	if c == nil {
		return nil, ErrNotFound
	}
	return c, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, employeeID, createdBy string, req CreateComplaintRequest) (*Complaint, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, ErrTitleRequired
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, ErrDescriptionRequired
	}
	if !req.ComplaintType.IsValid() {
		req.ComplaintType = TypeGeneral
	}
	if req.IncidentDate != nil {
		if _, err := time.Parse(dateLayout, *req.IncidentDate); err != nil {
			return nil, ErrInvalidDate
		}
	}
	c := &Complaint{
		OrgID: orgID, EmployeeID: employeeID,
		IsAnonymous: req.IsAnonymous, ComplaintType: req.ComplaintType,
		Title: req.Title, Description: req.Description,
		IncidentDate:      req.IncidentDate,
		AgainstEmployeeID: req.AgainstEmployeeID,
		AgainstDetails:    req.AgainstDetails,
		Status:            StatusSubmitted, CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("complaints: Create: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, employeeID, ref string, req UpdateComplaintRequest) (*Complaint, error) {
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("complaints: Update: %w", err)
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if c.Status == StatusResolved || c.Status == StatusDismissed || c.Status == StatusWithdrawn {
		return nil, ErrWrongStatus
	}
	if req.Title != nil {
		c.Title = *req.Title
	}
	if req.Description != nil {
		c.Description = *req.Description
	}
	if req.IncidentDate != nil {
		if _, err := time.Parse(dateLayout, *req.IncidentDate); err != nil {
			return nil, ErrInvalidDate
		}
		c.IncidentDate = req.IncidentDate
	}
	if req.AgainstDetails != nil {
		c.AgainstDetails = req.AgainstDetails
	}
	if req.DocumentID != nil {
		c.DocumentID = req.DocumentID
	}
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("complaints: Update: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) StartReview(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error) {
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("complaints: StartReview: %w", err)
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if c.Status != StatusSubmitted {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, c.ID, StatusUnderReview); err != nil {
		return nil, fmt.Errorf("complaints: StartReview: %w", err)
	}
	c.Status = StatusUnderReview
	return c, nil
}

func (s *serviceImpl) Assign(ctx context.Context, orgID, employeeID, ref string, req AssignRequest) (*Complaint, error) {
	if strings.TrimSpace(req.InvestigatorID) == "" {
		return nil, ErrInvestigatorRequired
	}
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("complaints: Assign: %w", err)
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if c.Status == StatusResolved || c.Status == StatusDismissed || c.Status == StatusWithdrawn {
		return nil, ErrWrongStatus
	}
	now := time.Now()
	c.InvestigatorID = &req.InvestigatorID
	c.InvestigationStartedAt = &now
	c.Status = StatusInvestigating
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("complaints: Assign: update: %w", err)
	}
	if err := s.repo.UpdateStatus(ctx, c.ID, StatusInvestigating); err != nil {
		return nil, fmt.Errorf("complaints: Assign: status: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) Resolve(ctx context.Context, orgID, employeeID, ref, resolvedBy string, req ResolveRequest) (*Complaint, error) {
	if strings.TrimSpace(req.Resolution) == "" {
		return nil, ErrResolutionRequired
	}
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("complaints: Resolve: %w", err)
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if c.Status == StatusResolved || c.Status == StatusDismissed || c.Status == StatusWithdrawn {
		return nil, ErrWrongStatus
	}
	now := time.Now()
	c.Resolution = &req.Resolution
	c.ResolutionAction = req.ResolutionAction
	c.ResolvedAt = &now
	c.ResolvedBy = &resolvedBy
	if req.DocumentID != nil {
		c.DocumentID = req.DocumentID
	}
	c.Status = StatusResolved
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("complaints: Resolve: update: %w", err)
	}
	if err := s.repo.UpdateStatus(ctx, c.ID, StatusResolved); err != nil {
		return nil, fmt.Errorf("complaints: Resolve: status: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) Dismiss(ctx context.Context, orgID, employeeID, ref, resolvedBy string, req DismissRequest) (*Complaint, error) {
	if strings.TrimSpace(req.Resolution) == "" {
		return nil, ErrResolutionRequired
	}
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("complaints: Dismiss: %w", err)
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if c.Status == StatusResolved || c.Status == StatusDismissed || c.Status == StatusWithdrawn {
		return nil, ErrWrongStatus
	}
	now := time.Now()
	c.Resolution = &req.Resolution
	c.ResolvedAt = &now
	c.ResolvedBy = &resolvedBy
	c.Status = StatusDismissed
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("complaints: Dismiss: update: %w", err)
	}
	if err := s.repo.UpdateStatus(ctx, c.ID, StatusDismissed); err != nil {
		return nil, fmt.Errorf("complaints: Dismiss: status: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) Withdraw(ctx context.Context, orgID, employeeID, ref string) (*Complaint, error) {
	c, err := s.repo.FindByRef(ctx, orgID, employeeID, ref)
	if err != nil {
		return nil, fmt.Errorf("complaints: Withdraw: %w", err)
	}
	if c == nil {
		return nil, ErrNotFound
	}
	if c.Status == StatusResolved || c.Status == StatusDismissed {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateStatus(ctx, c.ID, StatusWithdrawn); err != nil {
		return nil, fmt.Errorf("complaints: Withdraw: %w", err)
	}
	c.Status = StatusWithdrawn
	return c, nil
}
