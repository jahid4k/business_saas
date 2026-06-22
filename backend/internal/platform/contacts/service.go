// backend/internal/platform/contacts/service.go
package contacts

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/pkg/pagination"
)

// Service defines the business logic interface for contacts and companies.
type Service interface {
	// Contacts
	ListContacts(ctx context.Context, orgID string, p pagination.Params) (*ContactListResponse, error)
	GetContact(ctx context.Context, orgID, contactID string) (*Contact, error)
	CreateContact(ctx context.Context, orgID, userID string, req CreateContactRequest) (*Contact, error)
	// CreateContactTx inserts a contact inside an existing pgx.Tx.
	// Used by lead conversion to keep contact + deal + lead-status atomic.
	CreateContactTx(ctx context.Context, tx pgx.Tx, orgID, userID string, req CreateContactRequest) (*Contact, error)
	UpdateContact(ctx context.Context, orgID, contactID string, req UpdateContactRequest) (*Contact, error)
	DeleteContact(ctx context.Context, orgID, contactID string) error
	GetContactsByCompany(ctx context.Context, orgID, companyID string) ([]*Contact, error)

	// Companies
	ListCompanies(ctx context.Context, orgID string, p pagination.Params) (*CompanyListResponse, error)
	GetCompany(ctx context.Context, orgID, companyID string) (*Company, error)
	CreateCompany(ctx context.Context, orgID, userID string, req CreateCompanyRequest) (*Company, error)
	UpdateCompany(ctx context.Context, orgID, companyID string, req UpdateCompanyRequest) (*Company, error)
	DeleteCompany(ctx context.Context, orgID, companyID string) error
}

type serviceImpl struct {
	repo Repository
}

// NewService creates a new contacts service.
func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

// ============================================================
// Contacts
// ============================================================

func (s *serviceImpl) ListContacts(ctx context.Context, orgID string, p pagination.Params) (*ContactListResponse, error) {
	contacts, err := s.repo.FindContacts(ctx, orgID, p)
	if err != nil {
		return nil, fmt.Errorf("contacts: ListContacts: %w", err)
	}
	if contacts == nil {
		contacts = []*Contact{}
	}
	total, err := s.repo.CountContacts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("contacts: ListContacts: count: %w", err)
	}
	return &ContactListResponse{Contacts: contacts, Total: total}, nil
}

func (s *serviceImpl) GetContact(ctx context.Context, orgID, contactID string) (*Contact, error) {
	c, err := s.repo.FindContactByID(ctx, orgID, contactID)
	if err != nil {
		return nil, fmt.Errorf("contacts: GetContact: %w", err)
	}
	if c == nil {
		return nil, ErrContactNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateContact(ctx context.Context, orgID, userID string, req CreateContactRequest) (*Contact, error) {
	c, err := s.buildContact(orgID, userID, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateContact(ctx, c); err != nil {
		return nil, fmt.Errorf("contacts: CreateContact: %w", err)
	}
	return c, nil
}

// CreateContactTx inserts a contact within an existing pgx.Tx.
// The caller is responsible for committing or rolling back the transaction.
func (s *serviceImpl) CreateContactTx(ctx context.Context, tx pgx.Tx, orgID, userID string, req CreateContactRequest) (*Contact, error) {
	c, err := s.buildContact(orgID, userID, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateContactTx(ctx, tx, c); err != nil {
		return nil, fmt.Errorf("contacts: CreateContactTx: %w", err)
	}
	return c, nil
}

// buildContact validates the request and constructs a Contact ready for insert.
// Shared by CreateContact and CreateContactTx to avoid duplication.
func (s *serviceImpl) buildContact(orgID, userID string, req CreateContactRequest) (*Contact, error) {
	if strings.TrimSpace(req.FirstName) == "" {
		return nil, ErrFirstNameRequired
	}
	// Normalise and validate email when provided.
	var email *string
	if req.Email != nil {
		normalised := strings.ToLower(strings.TrimSpace(*req.Email))
		if normalised != "" {
			if err := validateEmail(normalised); err != nil {
				return nil, err
			}
			email = &normalised
		}
	}
	return &Contact{
		OrgID:     orgID,
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  req.LastName,
		Email:     email,
		Phone:     req.Phone,
		Title:     req.Title,
		CompanyID: req.CompanyID,
		Source:    req.Source,
		Status:    ContactStatusActive,
		OwnerID:   req.OwnerID,
		CreatedBy: userID,
	}, nil
}

func (s *serviceImpl) UpdateContact(ctx context.Context, orgID, contactID string, req UpdateContactRequest) (*Contact, error) {
	c, err := s.repo.FindContactByID(ctx, orgID, contactID)
	if err != nil {
		return nil, fmt.Errorf("contacts: UpdateContact: %w", err)
	}
	if c == nil {
		return nil, ErrContactNotFound
	}
	if req.FirstName != nil && strings.TrimSpace(*req.FirstName) != "" {
		c.FirstName = strings.TrimSpace(*req.FirstName)
	}
	if req.LastName != nil {
		c.LastName = req.LastName
	}
	if req.Email != nil {
		normalised := strings.ToLower(strings.TrimSpace(*req.Email))
		if normalised != "" {
			if err := validateEmail(normalised); err != nil {
				return nil, err
			}
			c.Email = &normalised
		} else {
			c.Email = nil
		}
	}
	if req.Phone != nil {
		c.Phone = req.Phone
	}
	if req.Title != nil {
		c.Title = req.Title
	}
	if req.CompanyID != nil {
		c.CompanyID = req.CompanyID
	}
	if req.Source != nil {
		c.Source = req.Source
	}
	if req.OwnerID != nil {
		c.OwnerID = req.OwnerID
	}
	if req.Status != nil {
		st := ContactStatus(*req.Status)
		if !st.IsValid() {
			return nil, ErrInvalidStatus
		}
		c.Status = st
	}
	if err := s.repo.UpdateContact(ctx, c); err != nil {
		return nil, fmt.Errorf("contacts: UpdateContact: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) DeleteContact(ctx context.Context, orgID, contactID string) error {
	if err := s.repo.SoftDeleteContact(ctx, orgID, contactID); err != nil {
		return fmt.Errorf("contacts: DeleteContact: %w", err)
	}
	return nil
}

func (s *serviceImpl) GetContactsByCompany(ctx context.Context, orgID, companyID string) ([]*Contact, error) {
	contacts, err := s.repo.FindContactsByCompany(ctx, orgID, companyID)
	if err != nil {
		return nil, fmt.Errorf("contacts: GetContactsByCompany: %w", err)
	}
	if contacts == nil {
		contacts = []*Contact{}
	}
	return contacts, nil
}

// ============================================================
// Companies
// ============================================================

func (s *serviceImpl) ListCompanies(ctx context.Context, orgID string, p pagination.Params) (*CompanyListResponse, error) {
	companies, err := s.repo.FindCompanies(ctx, orgID, p)
	if err != nil {
		return nil, fmt.Errorf("contacts: ListCompanies: %w", err)
	}
	if companies == nil {
		companies = []*Company{}
	}
	total, err := s.repo.CountCompanies(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("contacts: ListCompanies: count: %w", err)
	}
	return &CompanyListResponse{Companies: companies, Total: total}, nil
}

func (s *serviceImpl) GetCompany(ctx context.Context, orgID, companyID string) (*Company, error) {
	c, err := s.repo.FindCompanyByID(ctx, orgID, companyID)
	if err != nil {
		return nil, fmt.Errorf("contacts: GetCompany: %w", err)
	}
	if c == nil {
		return nil, ErrCompanyNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateCompany(ctx context.Context, orgID, userID string, req CreateCompanyRequest) (*Company, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrNameRequired
	}
	c := &Company{
		OrgID:     orgID,
		Name:      strings.TrimSpace(req.Name),
		Domain:    req.Domain,
		Industry:  req.Industry,
		Website:   req.Website,
		Phone:     req.Phone,
		Address:   req.Address,
		Country:   req.Country,
		Status:    CompanyStatusActive,
		OwnerID:   req.OwnerID,
		CreatedBy: userID,
	}
	if err := s.repo.CreateCompany(ctx, c); err != nil {
		return nil, fmt.Errorf("contacts: CreateCompany: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) UpdateCompany(ctx context.Context, orgID, companyID string, req UpdateCompanyRequest) (*Company, error) {
	c, err := s.repo.FindCompanyByID(ctx, orgID, companyID)
	if err != nil {
		return nil, fmt.Errorf("contacts: UpdateCompany: %w", err)
	}
	if c == nil {
		return nil, ErrCompanyNotFound
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		c.Name = strings.TrimSpace(*req.Name)
	}
	if req.Domain != nil {
		c.Domain = req.Domain
	}
	if req.Industry != nil {
		c.Industry = req.Industry
	}
	if req.Website != nil {
		c.Website = req.Website
	}
	if req.Phone != nil {
		c.Phone = req.Phone
	}
	if req.Address != nil {
		c.Address = req.Address
	}
	if req.Country != nil {
		c.Country = req.Country
	}
	if req.OwnerID != nil {
		c.OwnerID = req.OwnerID
	}
	if req.Status != nil {
		st := CompanyStatus(*req.Status)
		if !st.IsValid() {
			return nil, ErrInvalidStatus
		}
		c.Status = st
	}
	if err := s.repo.UpdateCompany(ctx, c); err != nil {
		return nil, fmt.Errorf("contacts: UpdateCompany: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) DeleteCompany(ctx context.Context, orgID, companyID string) error {
	if err := s.repo.SoftDeleteCompany(ctx, orgID, companyID); err != nil {
		return fmt.Errorf("contacts: DeleteCompany: %w", err)
	}
	return nil
}

// ============================================================
// Helpers
// ============================================================

// validateEmail performs a minimal structural check on a pre-normalised email.
// We do not use a regex library — a basic sanity check is sufficient; the
// database unique index provides the real duplicate guard.
func validateEmail(email string) error {
	at := strings.LastIndex(email, "@")
	if at < 1 || at == len(email)-1 {
		return ErrInvalidEmail
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") {
		return ErrInvalidEmail
	}
	return nil
}
