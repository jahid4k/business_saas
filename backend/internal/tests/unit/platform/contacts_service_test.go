// backend/internal/tests/unit/platform/contacts_service_test.go
// Platform contacts + companies service unit tests — no DB.
package platform

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mridha/businesssaas/internal/platform/contacts"
	"github.com/mridha/businesssaas/pkg/pagination"
)

// ── Stub contact repo ─────────────────────────────────────────────────────────

type stubContactRepo struct {
	contacts  map[string]*contacts.Contact
	companies map[string]*contacts.Company
	seq       int
}

func newStubContactRepo() *stubContactRepo {
	return &stubContactRepo{
		contacts:  map[string]*contacts.Contact{},
		companies: map[string]*contacts.Company{},
	}
}

func (r *stubContactRepo) nextID(prefix string) string {
	r.seq++
	s := ""
	n := r.seq
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return prefix + "_" + s
}

// ── Contact repo methods ──────────────────────────────────────────────────────

func (r *stubContactRepo) FindContacts(_ context.Context, orgID string, _ pagination.Params) ([]*contacts.Contact, error) {
	var out []*contacts.Contact
	for _, c := range r.contacts {
		if c.OrgID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *stubContactRepo) CountContacts(_ context.Context, orgID string) (int, error) {
	count := 0
	for _, c := range r.contacts {
		if c.OrgID == orgID {
			count++
		}
	}
	return count, nil
}

func (r *stubContactRepo) FindContactByID(_ context.Context, orgID, id string) (*contacts.Contact, error) {
	c, ok := r.contacts[id]
	if !ok || c.OrgID != orgID {
		return nil, nil
	}
	return c, nil
}

func (r *stubContactRepo) CreateContact(_ context.Context, c *contacts.Contact) error {
	c.ID = r.nextID("contact")
	c.PublicID = "pub_" + c.ID
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	r.contacts[c.ID] = c
	return nil
}

func (r *stubContactRepo) CreateContactTx(_ context.Context, _ pgx.Tx, c *contacts.Contact) error {
	return r.CreateContact(context.Background(), c)
}

func (r *stubContactRepo) UpdateContact(_ context.Context, c *contacts.Contact) error {
	if _, ok := r.contacts[c.ID]; !ok {
		return errors.New("not found")
	}
	r.contacts[c.ID] = c
	return nil
}

func (r *stubContactRepo) SoftDeleteContact(_ context.Context, orgID, id string) error {
	c, ok := r.contacts[id]
	if !ok || c.OrgID != orgID {
		return nil
	}
	delete(r.contacts, id)
	return nil
}

func (r *stubContactRepo) FindContactsByCompany(_ context.Context, orgID, companyID string) ([]*contacts.Contact, error) {
	var out []*contacts.Contact
	for _, c := range r.contacts {
		if c.OrgID == orgID && c.CompanyID != nil && *c.CompanyID == companyID {
			out = append(out, c)
		}
	}
	return out, nil
}

// ── Company repo methods ──────────────────────────────────────────────────────

func (r *stubContactRepo) FindCompanies(_ context.Context, orgID string, _ pagination.Params) ([]*contacts.Company, error) {
	var out []*contacts.Company
	for _, c := range r.companies {
		if c.OrgID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *stubContactRepo) CountCompanies(_ context.Context, orgID string) (int, error) {
	count := 0
	for _, c := range r.companies {
		if c.OrgID == orgID {
			count++
		}
	}
	return count, nil
}

func (r *stubContactRepo) FindCompanyByID(_ context.Context, orgID, id string) (*contacts.Company, error) {
	c, ok := r.companies[id]
	if !ok || c.OrgID != orgID {
		return nil, nil
	}
	return c, nil
}

func (r *stubContactRepo) CreateCompany(_ context.Context, c *contacts.Company) error {
	c.ID = r.nextID("company")
	c.PublicID = "pub_" + c.ID
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	r.companies[c.ID] = c
	return nil
}

func (r *stubContactRepo) UpdateCompany(_ context.Context, c *contacts.Company) error {
	if _, ok := r.companies[c.ID]; !ok {
		return errors.New("not found")
	}
	r.companies[c.ID] = c
	return nil
}

func (r *stubContactRepo) SoftDeleteCompany(_ context.Context, orgID, id string) error {
	c, ok := r.companies[id]
	if !ok || c.OrgID != orgID {
		return nil
	}
	delete(r.companies, id)
	return nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func newSvc(repo contacts.Repository) contacts.Service {
	return contacts.NewService(repo)
}

func defaultPage() pagination.Params {
	return pagination.Params{Limit: 50, Offset: 0}
}

// ── Contact tests ─────────────────────────────────────────────────────────────

func TestCreateContact_Success(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	c, err := svc.CreateContact(context.Background(), "org-1", "user-1", contacts.CreateContactRequest{
		FirstName: "Alice",
	})
	if err != nil {
		t.Fatalf("CreateContact() error: %v", err)
	}
	if c.FirstName != "Alice" {
		t.Errorf("expected FirstName=Alice, got %q", c.FirstName)
	}
	if c.Status != contacts.ContactStatusActive {
		t.Errorf("expected default status=active, got %q", c.Status)
	}
	if c.CreatedBy != "user-1" {
		t.Errorf("expected CreatedBy=user-1, got %q", c.CreatedBy)
	}
}

func TestCreateContact_FirstNameRequired(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	_, err := svc.CreateContact(context.Background(), "org-1", "user-1", contacts.CreateContactRequest{})
	if !errors.Is(err, contacts.ErrFirstNameRequired) {
		t.Fatalf("expected ErrFirstNameRequired, got %v", err)
	}
}

func TestCreateContact_InvalidEmail(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	badEmail := "not-an-email"
	_, err := svc.CreateContact(context.Background(), "org-1", "user-1", contacts.CreateContactRequest{
		FirstName: "Bob",
		Email:     &badEmail,
	})
	if !errors.Is(err, contacts.ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestCreateContact_ValidEmail(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	email := "bob@example.com"
	c, err := svc.CreateContact(context.Background(), "org-1", "user-1", contacts.CreateContactRequest{
		FirstName: "Bob",
		Email:     &email,
	})
	if err != nil {
		t.Fatalf("CreateContact() error: %v", err)
	}
	if c.Email == nil || *c.Email != "bob@example.com" {
		t.Errorf("expected email=bob@example.com, got %v", c.Email)
	}
}

func TestGetContact_CrossOrgReturnsNotFound(t *testing.T) {
	repo := newStubContactRepo()
	svc := newSvc(repo)
	c, _ := svc.CreateContact(context.Background(), "org-1", "user-1", contacts.CreateContactRequest{FirstName: "Carol"})

	_, err := svc.GetContact(context.Background(), "org-2", c.ID)
	if !errors.Is(err, contacts.ErrContactNotFound) {
		t.Fatalf("SECURITY: cross-org GetContact must return ErrContactNotFound, got %v", err)
	}
}

func TestDeleteContact_CrossOrgIsNoOp(t *testing.T) {
	repo := newStubContactRepo()
	svc := newSvc(repo)
	c, _ := svc.CreateContact(context.Background(), "org-1", "user-1", contacts.CreateContactRequest{FirstName: "Dave"})

	if err := svc.DeleteContact(context.Background(), "org-2", c.ID); err != nil {
		t.Fatalf("cross-org DeleteContact must not error, got: %v", err)
	}
	// Must survive
	got, _ := svc.GetContact(context.Background(), "org-1", c.ID)
	if got == nil {
		t.Error("SECURITY: contact was deleted from wrong org — must survive cross-org delete")
	}
}

func TestUpdateContact_CrossOrgReturnsNotFound(t *testing.T) {
	repo := newStubContactRepo()
	svc := newSvc(repo)
	c, _ := svc.CreateContact(context.Background(), "org-1", "user-1", contacts.CreateContactRequest{FirstName: "Eve"})

	updated := "Hacked"
	_, err := svc.UpdateContact(context.Background(), "org-2", c.ID, contacts.UpdateContactRequest{
		FirstName: &updated,
	})
	if !errors.Is(err, contacts.ErrContactNotFound) {
		t.Fatalf("SECURITY: cross-org UpdateContact must return ErrContactNotFound, got %v", err)
	}
}

func TestListContacts_EmptySliceNotNil(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	resp, err := svc.ListContacts(context.Background(), "org-empty", defaultPage())
	if err != nil {
		t.Fatalf("ListContacts() error: %v", err)
	}
	if resp.Contacts == nil {
		t.Error("expected non-nil empty slice for contacts")
	}
}

// ── Company tests ─────────────────────────────────────────────────────────────

func TestCreateCompany_Success(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	c, err := svc.CreateCompany(context.Background(), "org-1", "user-1", contacts.CreateCompanyRequest{
		Name: "Acme Corp",
	})
	if err != nil {
		t.Fatalf("CreateCompany() error: %v", err)
	}
	if c.Name != "Acme Corp" {
		t.Errorf("expected Name=Acme Corp, got %q", c.Name)
	}
	if c.Status != contacts.CompanyStatusActive {
		t.Errorf("expected default status=active, got %q", c.Status)
	}
}

func TestCreateCompany_NameRequired(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	_, err := svc.CreateCompany(context.Background(), "org-1", "user-1", contacts.CreateCompanyRequest{})
	if !errors.Is(err, contacts.ErrNameRequired) {
		t.Fatalf("expected ErrNameRequired, got %v", err)
	}
}

func TestGetCompany_CrossOrgReturnsNotFound(t *testing.T) {
	repo := newStubContactRepo()
	svc := newSvc(repo)
	c, _ := svc.CreateCompany(context.Background(), "org-1", "user-1", contacts.CreateCompanyRequest{Name: "TestCo"})

	_, err := svc.GetCompany(context.Background(), "org-2", c.ID)
	if !errors.Is(err, contacts.ErrCompanyNotFound) {
		t.Fatalf("SECURITY: cross-org GetCompany must return ErrCompanyNotFound, got %v", err)
	}
}

func TestDeleteCompany_CrossOrgIsNoOp(t *testing.T) {
	repo := newStubContactRepo()
	svc := newSvc(repo)
	c, _ := svc.CreateCompany(context.Background(), "org-1", "user-1", contacts.CreateCompanyRequest{Name: "SafeCo"})

	if err := svc.DeleteCompany(context.Background(), "org-2", c.ID); err != nil {
		t.Fatalf("cross-org DeleteCompany must not error, got: %v", err)
	}
	got, _ := svc.GetCompany(context.Background(), "org-1", c.ID)
	if got == nil {
		t.Error("SECURITY: company was deleted from wrong org — must survive cross-org delete")
	}
}

func TestListCompanies_EmptySliceNotNil(t *testing.T) {
	svc := newSvc(newStubContactRepo())
	resp, err := svc.ListCompanies(context.Background(), "org-empty", defaultPage())
	if err != nil {
		t.Fatalf("ListCompanies() error: %v", err)
	}
	if resp.Companies == nil {
		t.Error("expected non-nil empty slice for companies")
	}
}
