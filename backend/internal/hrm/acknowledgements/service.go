// backend/internal/hrm/acknowledgements/service.go
package acknowledgements

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

type Service interface {
	List(ctx context.Context, orgID, employeeID, ackType, status string) (*AckListResponse, error)
	ListByEntity(ctx context.Context, orgID, ackType, ackID string) (*AckListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Acknowledgement, error)
	// Create sends an acknowledgement request to an employee.
	Create(ctx context.Context, orgID, requestedBy string, req CreateAcknowledgementRequest) (*Acknowledgement, error)
	// Respond records the employee acknowledging.
	Respond(ctx context.Context, orgID, ref string, req RespondRequest) (*Acknowledgement, error)
	// Decline records the employee declining.
	Decline(ctx context.Context, orgID, ref string, req DeclineRequest) (*Acknowledgement, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

func (s *serviceImpl) List(ctx context.Context, orgID, employeeID, ackType, status string) (*AckListResponse, error) {
	list, err := s.repo.FindAll(ctx, orgID, employeeID, ackType, status)
	if err != nil {
		return nil, fmt.Errorf("acknowledgements: List: %w", err)
	}
	if list == nil {
		list = []*Acknowledgement{}
	}
	return &AckListResponse{Acknowledgements: list, Total: len(list)}, nil
}

func (s *serviceImpl) ListByEntity(ctx context.Context, orgID, ackType, ackID string) (*AckListResponse, error) {
	list, err := s.repo.FindByEntity(ctx, orgID, ackType, ackID)
	if err != nil {
		return nil, fmt.Errorf("acknowledgements: ListByEntity: %w", err)
	}
	if list == nil {
		list = []*Acknowledgement{}
	}
	return &AckListResponse{Acknowledgements: list, Total: len(list)}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Acknowledgement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("acknowledgements: Get: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	return a, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, requestedBy string, req CreateAcknowledgementRequest) (*Acknowledgement, error) {
	if strings.TrimSpace(req.EmployeeID) == "" {
		return nil, ErrEmployeeIDRequired
	}
	if strings.TrimSpace(req.EntityTitle) == "" {
		return nil, ErrEntityTitleRequired
	}
	if !req.AcknowledgeableType.IsValid() {
		return nil, ErrInvalidAckType
	}
	if strings.TrimSpace(req.AcknowledgeableID) == "" {
		return nil, ErrAckIDRequired
	}
	if req.ExpiresAt != nil {
		if _, err := time.Parse(dateLayout, *req.ExpiresAt); err != nil {
			return nil, ErrInvalidDate
		}
	}
	a := &Acknowledgement{
		OrgID: orgID, EmployeeID: req.EmployeeID,
		AcknowledgeableType: req.AcknowledgeableType, AcknowledgeableID: req.AcknowledgeableID,
		EntityTitle: req.EntityTitle, SignatureRequired: req.SignatureRequired,
		ExpiresAt: req.ExpiresAt,
		Status:    StatusPending, RequestedBy: requestedBy,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("acknowledgements: Create: %w", err)
	}
	return a, nil
}

func (s *serviceImpl) Respond(ctx context.Context, orgID, ref string, req RespondRequest) (*Acknowledgement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("acknowledgements: Respond: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.Status != StatusPending {
		return nil, ErrWrongStatus
	}
	if a.SignatureRequired && (req.SignatureData == nil || strings.TrimSpace(*req.SignatureData) == "") {
		return nil, ErrSignatureRequired
	}
	if err := s.repo.Acknowledge(ctx, a.ID, req.Notes, req.SignatureData); err != nil {
		return nil, fmt.Errorf("acknowledgements: Respond: %w", err)
	}
	now := time.Now()
	a.Status = StatusAcknowledged
	a.AcknowledgedAt = &now
	a.Notes = req.Notes
	a.SignatureData = req.SignatureData
	return a, nil
}

func (s *serviceImpl) Decline(ctx context.Context, orgID, ref string, req DeclineRequest) (*Acknowledgement, error) {
	a, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("acknowledgements: Decline: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if a.Status != StatusPending {
		return nil, ErrWrongStatus
	}
	if err := s.repo.Decline(ctx, a.ID, req.Reason); err != nil {
		return nil, fmt.Errorf("acknowledgements: Decline: %w", err)
	}
	now := time.Now()
	a.Status = StatusDeclined
	a.DeclinedAt = &now
	a.DeclineReason = &req.Reason
	return a, nil
}
