// backend/internal/crm/leads/service.go
package leads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/internal/platform/contacts"
	crmsettings "github.com/mridha/businesssaas/internal/crm/settings"
)

// ContactCreator is the subset of contacts.Service needed by lead conversion.
// Defined here to avoid importing the full contacts package interface and to
// allow passing a pgx.Tx for atomic writes.
type ContactCreator interface {
	CreateContactTx(ctx context.Context, tx pgx.Tx, orgID, userID string, req contacts.CreateContactRequest) (*contacts.Contact, error)
}

// DealCreator is the subset of deals.Service needed by lead conversion.
// Using an interface here breaks the import cycle: leads → deals would be
// circular since reports imports both.
// CreateDealFromLeadTx returns the new deal's ID and accepts a pgx.Tx so the
// insert participates in the same transaction as contact and lead updates.
type DealCreator interface {
	CreateDealFromLeadTx(ctx context.Context, tx pgx.Tx, orgID, userID string, lead *Lead, req ConvertLeadRequest) (string, error)
}

// Service defines the business logic for CRM leads.
type Service interface {
	ListLeads(ctx context.Context, orgID string) (*LeadListResponse, error)
	GetLead(ctx context.Context, orgID, leadID string) (*Lead, error)
	CreateLead(ctx context.Context, orgID, userID string, req CreateLeadRequest) (*Lead, error)
	UpdateLead(ctx context.Context, orgID, leadID string, req UpdateLeadRequest) (*Lead, error)
	DeleteLead(ctx context.Context, orgID, leadID string) error
	ConvertLead(ctx context.Context, orgID, leadID, userID string, req ConvertLeadRequest) (*ConvertLeadResponse, error)
	GetLeadsBySource(ctx context.Context, orgID string) ([]*LeadsBySource, error)
}

type SettingsService interface {
	GetSettings(ctx context.Context, orgID string) (*crmsettings.CRMSettings, error)
}

type serviceImpl struct {
	repo           Repository
	contactCreator ContactCreator
	dealCreator    DealCreator
	settingsSvc    SettingsService
}

func NewService(repo Repository, contactCreator ContactCreator, dealCreator DealCreator, settingsSvc SettingsService) Service {
	return &serviceImpl{
		repo:           repo,
		contactCreator: contactCreator,
		dealCreator:    dealCreator,
		settingsSvc:    settingsSvc,
	}
}

func (s *serviceImpl) ListLeads(ctx context.Context, orgID string) (*LeadListResponse, error) {
	leads, err := s.repo.FindLeads(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("leads: ListLeads: %w", err)
	}
	if leads == nil {
		leads = []*Lead{}
	}
	total, err := s.repo.CountLeads(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("leads: ListLeads: count: %w", err)
	}
	return &LeadListResponse{Leads: leads, Total: total}, nil
}

func (s *serviceImpl) GetLead(ctx context.Context, orgID, leadID string) (*Lead, error) {
	l, err := s.repo.FindLeadByID(ctx, orgID, leadID)
	if err != nil {
		return nil, fmt.Errorf("leads: GetLead: %w", err)
	}
	if l == nil {
		return nil, ErrLeadNotFound
	}
	return l, nil
}

func (s *serviceImpl) CreateLead(ctx context.Context, orgID, userID string, req CreateLeadRequest) (*Lead, error) {
	if strings.TrimSpace(req.FirstName) == "" {
		return nil, ErrFirstNameRequired
	}

	ownerID := req.OwnerID
	if ownerID == nil && s.settingsSvc != nil {
		settings, err := s.settingsSvc.GetSettings(ctx, orgID)
		if err == nil && settings != nil && settings.LeadRoutingEnabled && len(settings.RoundRobinAssignees) > 0 {
			lastOwner, _ := s.repo.GetLastAssignedLeadOwner(ctx, orgID)
			nextOwner := settings.RoundRobinAssignees[0]
			if lastOwner != "" {
				for i, id := range settings.RoundRobinAssignees {
					if id == lastOwner {
						if i+1 < len(settings.RoundRobinAssignees) {
							nextOwner = settings.RoundRobinAssignees[i+1]
						}
						break
					}
				}
			}
			ownerID = &nextOwner
		}
	}

	l := &Lead{
		OrgID:       orgID,
		FirstName:   strings.TrimSpace(req.FirstName),
		LastName:    req.LastName,
		Email:       req.Email,
		Phone:       req.Phone,
		CompanyName: req.CompanyName,
		Title:       req.Title,
		Source:      req.Source,
		Status:      LeadStatusNew,
		OwnerID:     ownerID,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateLead(ctx, l); err != nil {
		return nil, fmt.Errorf("leads: CreateLead: %w", err)
	}
	return l, nil
}

func (s *serviceImpl) UpdateLead(ctx context.Context, orgID, leadID string, req UpdateLeadRequest) (*Lead, error) {
	l, err := s.repo.FindLeadByID(ctx, orgID, leadID)
	if err != nil {
		return nil, fmt.Errorf("leads: UpdateLead: %w", err)
	}
	if l == nil {
		return nil, ErrLeadNotFound
	}
	if req.FirstName != nil && strings.TrimSpace(*req.FirstName) != "" {
		l.FirstName = strings.TrimSpace(*req.FirstName)
	}
	if req.LastName != nil {
		l.LastName = req.LastName
	}
	if req.Email != nil {
		l.Email = req.Email
	}
	if req.Phone != nil {
		l.Phone = req.Phone
	}
	if req.CompanyName != nil {
		l.CompanyName = req.CompanyName
	}
	if req.Title != nil {
		l.Title = req.Title
	}
	if req.Source != nil {
		l.Source = req.Source
	}
	if req.OwnerID != nil {
		l.OwnerID = req.OwnerID
	}
	if req.Status != nil {
		st := LeadStatus(*req.Status)
		if !st.IsValid() {
			return nil, ErrInvalidStatus
		}
		l.Status = st
	}
	if err := s.repo.UpdateLead(ctx, l); err != nil {
		return nil, fmt.Errorf("leads: UpdateLead: %w", err)
	}
	return l, nil
}

func (s *serviceImpl) DeleteLead(ctx context.Context, orgID, leadID string) error {
	if err := s.repo.SoftDeleteLead(ctx, orgID, leadID); err != nil {
		return fmt.Errorf("leads: DeleteLead: %w", err)
	}
	return nil
}

// ConvertLead converts a lead into a contact and/or deal atomically.
//
// All three writes — contact insert, deal insert, lead status update — happen
// inside a single database transaction. If any step fails, every prior write
// in the same conversion is rolled back. Callers will never see a state where
// a contact exists but the lead is still marked "new".
func (s *serviceImpl) ConvertLead(ctx context.Context, orgID, leadID, userID string, req ConvertLeadRequest) (*ConvertLeadResponse, error) {
	l, err := s.repo.FindLeadByID(ctx, orgID, leadID)
	if err != nil {
		return nil, fmt.Errorf("leads: ConvertLead: %w", err)
	}
	if l == nil {
		return nil, ErrLeadNotFound
	}
	if l.Status == LeadStatusConverted {
		return nil, ErrLeadAlreadyConverted
	}

	// Open a transaction that wraps all conversion writes.
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("leads: ConvertLead: begin tx: %w", err)
	}
	// Rollback is a no-op after a successful Commit, so it is safe to defer.
	defer func() { _ = tx.Rollback(ctx) }()

	resp := &ConvertLeadResponse{Lead: l}

	// Step 1 — create contact (inside tx).
	if req.CreateContact && s.contactCreator != nil {
		c, err := s.contactCreator.CreateContactTx(ctx, tx, orgID, userID, contacts.CreateContactRequest{
			FirstName: l.FirstName,
			LastName:  l.LastName,
			Email:     l.Email,
			Phone:     l.Phone,
			OwnerID:   l.OwnerID,
		})
		if err != nil {
			return nil, fmt.Errorf("leads: ConvertLead: create contact: %w", err)
		}
		resp.ContactID = &c.ID
		l.ConvertedContactID = &c.ID
	}

	// Step 2 — create deal (inside same tx).
	if req.CreateDeal && s.dealCreator != nil && req.PipelineID != nil && req.StageID != nil {
		dealID, err := s.dealCreator.CreateDealFromLeadTx(ctx, tx, orgID, userID, l, req)
		if err != nil {
			return nil, fmt.Errorf("leads: ConvertLead: create deal: %w", err)
		}
		resp.DealID = &dealID
		l.ConvertedDealID = &dealID
	}

	// Step 3 — mark lead as converted (inside same tx).
	now := time.Now()
	l.Status = LeadStatusConverted
	l.ConvertedAt = &now
	if err := s.repo.UpdateLeadTx(ctx, tx, l); err != nil {
		return nil, fmt.Errorf("leads: ConvertLead: update lead: %w", err)
	}

	// Commit — all three writes succeed or none do.
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("leads: ConvertLead: commit: %w", err)
	}

	resp.Lead = l
	return resp, nil
}

func (s *serviceImpl) GetLeadsBySource(ctx context.Context, orgID string) ([]*LeadsBySource, error) {
	result, err := s.repo.GetLeadsBySource(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("leads: GetLeadsBySource: %w", err)
	}
	if result == nil {
		result = []*LeadsBySource{}
	}
	return result, nil
}
