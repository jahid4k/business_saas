// backend/internal/tests/unit/crm/leads_service_test.go
package crm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mridha/businesssaas/internal/crm/leads"
	crmsettings "github.com/mridha/businesssaas/internal/crm/settings"
	"github.com/mridha/businesssaas/internal/platform/contacts"
	"github.com/mridha/businesssaas/internal/platform/engagement"
)

// ── Stub lead repo ────────────────────────────────────────────────────────────

type stubLeadRepo struct {
	leads    map[string]*leads.Lead
	seq      int
	forceErr error
}

func newStubLeadRepo() *stubLeadRepo {
	return &stubLeadRepo{leads: map[string]*leads.Lead{}}
}

func (r *stubLeadRepo) nextID() string {
	r.seq++
	n := r.seq
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return "lead_" + s
}

func (r *stubLeadRepo) FindLeads(_ context.Context, orgID string) ([]*leads.Lead, error) {
	var out []*leads.Lead
	for _, l := range r.leads {
		if l.OrgID == orgID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *stubLeadRepo) CountLeads(_ context.Context, orgID string) (int, error) {
	count := 0
	for _, l := range r.leads {
		if l.OrgID == orgID {
			count++
		}
	}
	return count, nil
}

func (r *stubLeadRepo) FindLeadByID(_ context.Context, orgID, leadID string) (*leads.Lead, error) {
	if r.forceErr != nil {
		return nil, r.forceErr
	}
	l, ok := r.leads[leadID]
	if !ok || l.OrgID != orgID {
		return nil, nil
	}
	return l, nil
}

func (r *stubLeadRepo) FindLeadByEmail(_ context.Context, orgID, email string) (*leads.Lead, error) {
	if r.forceErr != nil {
		return nil, r.forceErr
	}
	for _, l := range r.leads {
		if l.OrgID == orgID && l.Email != nil && *l.Email == email {
			return l, nil
		}
	}
	return nil, nil
}

func (r *stubLeadRepo) CreateLead(_ context.Context, l *leads.Lead) error {
	l.ID = r.nextID()
	l.PublicID = "pub_" + l.ID
	l.CreatedAt = time.Now()
	l.UpdatedAt = time.Now()
	r.leads[l.ID] = l
	return nil
}

func (r *stubLeadRepo) UpdateLead(_ context.Context, l *leads.Lead) error {
	if _, ok := r.leads[l.ID]; !ok {
		return errors.New("lead not found")
	}
	l.UpdatedAt = time.Now()
	r.leads[l.ID] = l
	return nil
}

func (r *stubLeadRepo) UpdateLeadTx(_ context.Context, _ pgx.Tx, l *leads.Lead) error {
	return r.UpdateLead(context.Background(), l)
}

func (r *stubLeadRepo) SoftDeleteLead(_ context.Context, orgID, leadID string) error {
	l, ok := r.leads[leadID]
	if !ok || l.OrgID != orgID {
		return nil
	}
	delete(r.leads, leadID)
	return nil
}

func (r *stubLeadRepo) GetLeadsBySource(_ context.Context, _ string) ([]*leads.LeadsBySource, error) {
	return []*leads.LeadsBySource{}, nil
}

func (r *stubLeadRepo) GetLastAssignedLeadOwner(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (r *stubLeadRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	return &fakeTx{}, nil
}

// fakeTx is a minimal no-op pgx.Tx for pgx v5.
type fakeTx struct{}

func (t *fakeTx) Begin(ctx context.Context) (pgx.Tx, error)                    { return t, nil }
func (t *fakeTx) Commit(ctx context.Context) error                             { return nil }
func (t *fakeTx) Rollback(ctx context.Context) error                           { return nil }
func (t *fakeTx) Conn() *pgx.Conn                                              { return nil }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (t *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (t *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }

// ── Noop contact/deal creators ────────────────────────────────────────────────

type noopContactCreator struct{}

func (c *noopContactCreator) CreateContactTx(_ context.Context, _ pgx.Tx, orgID, userID string, req contacts.CreateContactRequest) (*contacts.Contact, error) {
	return &contacts.Contact{
		ID:        "contact_new",
		OrgID:     orgID,
		FirstName: req.FirstName,
		CreatedBy: userID,
		Status:    contacts.ContactStatusActive,
	}, nil
}

type noopDealCreator struct{}

func (d *noopDealCreator) CreateDealFromLeadTx(_ context.Context, _ pgx.Tx, _, _ string, _ *leads.Lead, _ leads.ConvertLeadRequest) (string, error) {
	return "deal_new", nil
}

// ── Noop settings and engagement ──────────────────────────────────────────────

type noopSettingsSvc struct{}

func (s *noopSettingsSvc) GetSettings(ctx context.Context, orgID string) (*crmsettings.CRMSettings, error) {
	return &crmsettings.CRMSettings{}, nil
}

type stubEngagementCreator struct {
	createdNotes []engagement.CreateNoteRequest
}

func (c *stubEngagementCreator) CreateNote(_ context.Context, _, _, _ string, req engagement.CreateNoteRequest) (*engagement.Note, error) {
	c.createdNotes = append(c.createdNotes, req)
	return &engagement.Note{ID: "note_new"}, nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func newSvc(repo leads.Repository, engagementSvc ...leads.EngagementCreator) leads.Service {
	var eSvc leads.EngagementCreator
	if len(engagementSvc) > 0 {
		eSvc = engagementSvc[0]
	} else {
		eSvc = &stubEngagementCreator{}
	}
	return leads.NewService(repo, &noopContactCreator{}, &noopDealCreator{}, &noopSettingsSvc{}, eSvc)
}

func sptr(s string) *string { return &s }

// ── CreateLead ────────────────────────────────────────────────────────────────

func TestCreateLead_Success(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	l, err := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{
		FirstName: "Alice",
	})
	if err != nil {
		t.Fatalf("CreateLead() error: %v", err)
	}
	if l.FirstName != "Alice" {
		t.Errorf("expected FirstName=Alice, got %q", l.FirstName)
	}
	if l.Status != leads.LeadStatusNew {
		t.Errorf("expected default status=new, got %q", l.Status)
	}
	if l.CreatedBy == nil || *l.CreatedBy != "user-1" {
		t.Errorf("expected CreatedBy=user-1, got %v", l.CreatedBy)
	}
}

// TestCreateLead_SystemCaptureHasNoCreator covers the caller every
// internal/capture/* path actually is: no acting user. created_by was
// uuid NOT NULL and each of those inserts died with 22P02, silently, into a
// log row — see migration 00112. An empty userID must become NULL, not the
// empty string.
func TestCreateLead_SystemCaptureHasNoCreator(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	source := "email"
	l, err := svc.CreateLead(context.Background(), "org-1", "", leads.CreateLeadRequest{
		FirstName:     "jane",
		CaptureSource: &source,
	})
	if err != nil {
		t.Fatalf("CreateLead() error: %v", err)
	}
	if l.CreatedBy != nil {
		t.Errorf("expected CreatedBy=nil for a system capture, got %q", *l.CreatedBy)
	}
	// capture_source is what records the origin instead.
	if l.CaptureSource == nil || *l.CaptureSource != "email" {
		t.Errorf("expected CaptureSource=email, got %v", l.CaptureSource)
	}
}

func TestCreateLead_FirstNameRequired(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	_, err := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{})
	if !errors.Is(err, leads.ErrFirstNameRequired) {
		t.Fatalf("expected ErrFirstNameRequired, got %v", err)
	}
}

func TestCreateLead_WhitespaceFirstNameRejected(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	_, err := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{
		FirstName: "   ",
	})
	if !errors.Is(err, leads.ErrFirstNameRequired) {
		t.Fatalf("expected ErrFirstNameRequired for whitespace name, got %v", err)
	}
}

func TestCreateLead_DuplicateEmail_AppendsNote(t *testing.T) {
	repo := newStubLeadRepo()
	engStub := &stubEngagementCreator{}
	svc := newSvc(repo, engStub)

	email := "test@example.com"
	captureSource := "web_form"
	// Create first lead
	created1, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{
		FirstName: "Original",
		Email:     &email,
	})

	// Attempt duplicate create
	created2, err := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{
		FirstName:     "Duplicate",
		Email:         &email,
		CaptureSource: &captureSource,
	})
	if err != nil {
		t.Fatalf("expected no error on duplicate email (dedup handled), got %v", err)
	}

	// Should return the original lead
	if created2.ID != created1.ID {
		t.Fatalf("expected duplicate creation to return original lead ID %s, got %s", created1.ID, created2.ID)
	}

	// Check that a note was appended
	if len(engStub.createdNotes) != 1 {
		t.Fatalf("expected 1 note created for deduplication, got %d", len(engStub.createdNotes))
	}
	note := engStub.createdNotes[0]
	if note.RelatedType != "lead" || note.RelatedID != created1.ID {
		t.Errorf("note has wrong related entity: %v", note)
	}
}

// ── GetLead / Tenant isolation ────────────────────────────────────────────────

func TestGetLead_Success(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Bob"})
	got, err := svc.GetLead(context.Background(), "org-1", created.ID)
	if err != nil {
		t.Fatalf("GetLead() error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected ID=%s, got %s", created.ID, got.ID)
	}
}

func TestGetLead_CrossOrgReturnsNotFound(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Carol"})
	_, err := svc.GetLead(context.Background(), "org-2", created.ID)
	if !errors.Is(err, leads.ErrLeadNotFound) {
		t.Fatalf("SECURITY: cross-org GetLead must return ErrLeadNotFound, got %v", err)
	}
}

func TestGetLead_NotFound(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	_, err := svc.GetLead(context.Background(), "org-1", "nonexistent-id")
	if !errors.Is(err, leads.ErrLeadNotFound) {
		t.Fatalf("expected ErrLeadNotFound, got %v", err)
	}
}

// ── UpdateLead ────────────────────────────────────────────────────────────────

func TestUpdateLead_InvalidStatus(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Dave"})
	_, err := svc.UpdateLead(context.Background(), "org-1", created.ID, leads.UpdateLeadRequest{
		Status: sptr("invalid_status"),
	})
	if !errors.Is(err, leads.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestUpdateLead_CrossOrgReturnsNotFound(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Eve"})
	_, err := svc.UpdateLead(context.Background(), "org-2", created.ID, leads.UpdateLeadRequest{
		Status: sptr("contacted"),
	})
	if !errors.Is(err, leads.ErrLeadNotFound) {
		t.Fatalf("SECURITY: cross-org UpdateLead must return ErrLeadNotFound, got %v", err)
	}
}

// ── DeleteLead ────────────────────────────────────────────────────────────────

func TestDeleteLead_CrossOrgIsNoOp(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Frank"})
	if err := svc.DeleteLead(context.Background(), "org-2", created.ID); err != nil {
		t.Fatalf("cross-org DeleteLead must not error, got: %v", err)
	}
	got, err := svc.GetLead(context.Background(), "org-1", created.ID)
	if err != nil {
		t.Fatalf("GetLead() after cross-org delete error: %v", err)
	}
	if got == nil {
		t.Error("SECURITY: lead was deleted from wrong org — must survive cross-org delete")
	}
}

// ── ConvertLead ───────────────────────────────────────────────────────────────

func TestConvertLead_Success(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Grace"})
	resp, err := svc.ConvertLead(context.Background(), "org-1", created.ID, "user-1", leads.ConvertLeadRequest{
		CreateContact: true,
	})
	if err != nil {
		t.Fatalf("ConvertLead() error: %v", err)
	}
	if resp.Lead.Status != leads.LeadStatusConverted {
		t.Errorf("expected lead status=converted, got %q", resp.Lead.Status)
	}
	if resp.ContactID == nil {
		t.Error("expected contact ID in response when CreateContact=true")
	}
}

func TestConvertLead_AlreadyConverted_Rejected(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Hector"})
	_, _ = svc.ConvertLead(context.Background(), "org-1", created.ID, "user-1", leads.ConvertLeadRequest{CreateContact: true})
	_, err := svc.ConvertLead(context.Background(), "org-1", created.ID, "user-1", leads.ConvertLeadRequest{CreateContact: true})
	if !errors.Is(err, leads.ErrLeadAlreadyConverted) {
		t.Fatalf("expected ErrLeadAlreadyConverted on second conversion, got %v", err)
	}
}

// ── ListLeads ─────────────────────────────────────────────────────────────────

func TestListLeads_EmptySliceNotNil(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	resp, err := svc.ListLeads(context.Background(), "org-empty")
	if err != nil {
		t.Fatalf("ListLeads() error: %v", err)
	}
	if resp.Leads == nil {
		t.Error("expected non-nil empty slice for leads")
	}
	if resp.Total != 0 {
		t.Errorf("expected Total=0, got %d", resp.Total)
	}
}

func TestListLeads_OnlyReturnsOwnOrg(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	_, _ = svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Iris"})
	_, _ = svc.CreateLead(context.Background(), "org-2", "user-2", leads.CreateLeadRequest{FirstName: "Jack"})
	resp, _ := svc.ListLeads(context.Background(), "org-1")
	for _, l := range resp.Leads {
		if l.OrgID != "org-1" {
			t.Errorf("SECURITY: lead from org-2 appeared in org-1 list")
		}
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 lead in org-1, got %d", resp.Total)
	}
}

func TestUpdateLead_Success(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	created, _ := svc.CreateLead(context.Background(), "org-1", "user-1", leads.CreateLeadRequest{FirstName: "Dave"})

	newFirstName := "David"
	status := leads.LeadStatusContacted
	updated, err := svc.UpdateLead(context.Background(), "org-1", created.ID, leads.UpdateLeadRequest{
		FirstName: &newFirstName,
		Status:    (*string)(&status),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.FirstName != newFirstName {
		t.Errorf("expected FirstName=%q, got %q", newFirstName, updated.FirstName)
	}
	if updated.Status != leads.LeadStatusContacted {
		t.Errorf("expected Status=%q, got %q", leads.LeadStatusContacted, updated.Status)
	}
}

func TestGetLeadsBySource_Success(t *testing.T) {
	svc := newSvc(newStubLeadRepo())
	// Since our stub repo returns empty slice, we just test that the service calls it without error
	res, err := svc.GetLeadsBySource(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil {
		t.Error("expected non-nil result")
	}
}
