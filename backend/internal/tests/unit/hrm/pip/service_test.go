// backend/internal/tests/unit/hrm/pip/service_test.go
// Phase 5C PIP: the failed-plan handoff to terminations, the extension audit
// trail, and the close permission that 'manager' deliberately lacks.
package pip_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/pip"
)

const (
	testOrg     = "org_1"
	ownerUserID = "usr_owner"
	empID       = "emp_1"
	otherEmpID  = "emp_2"
)

// ── Stubs ────────────────────────────────────────────────────────────────────

type stubRepo struct {
	seq       int
	plans     map[string]*pip.PIP
	checkins  map[string][]*pip.Checkin
	employees map[string]*pip.EmployeeRef
}

var _ pip.Repository = (*stubRepo)(nil)

func newStubRepo() *stubRepo {
	return &stubRepo{
		plans:     map[string]*pip.PIP{},
		checkins:  map[string][]*pip.Checkin{},
		employees: map[string]*pip.EmployeeRef{},
	}
}

func (r *stubRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d", prefix, r.seq)
}

func (r *stubRepo) Find(_ context.Context, orgID string, f pip.ListFilter) ([]*pip.PIP, error) {
	out := make([]*pip.PIP, 0)
	for _, p := range r.plans {
		if p.OrgID != orgID {
			continue
		}
		if f.EmployeeID != "" && p.EmployeeID != f.EmployeeID {
			continue
		}
		if f.Status != "" && string(p.Status) != f.Status {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *stubRepo) Count(ctx context.Context, orgID string, f pip.ListFilter) (int, error) {
	out, err := r.Find(ctx, orgID, f)
	return len(out), err
}

func (r *stubRepo) FindByRef(_ context.Context, orgID, ref string) (*pip.PIP, error) {
	for _, p := range r.plans {
		if p.OrgID == orgID && (p.ID == ref || p.PublicID == ref) {
			return p, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) HasOpenPlan(_ context.Context, orgID, employeeID string) (bool, error) {
	for _, p := range r.plans {
		if p.OrgID == orgID && p.EmployeeID == employeeID && p.Status.IsOpen() {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) Create(_ context.Context, p *pip.PIP) error {
	p.ID = r.nextID("pip")
	p.PublicID = "pip_pub_" + p.ID
	p.Status = pip.StatusDraft
	p.OriginalEndDate = p.EndDate
	r.plans[p.ID] = p
	return nil
}

func (r *stubRepo) Update(_ context.Context, p *pip.PIP) error {
	if _, ok := r.plans[p.ID]; !ok {
		return pip.ErrNotFound
	}
	r.plans[p.ID] = p
	return nil
}

func (r *stubRepo) SetStatus(_ context.Context, orgID, id string, status pip.Status) (*pip.PIP, error) {
	p, ok := r.plans[id]
	if !ok || p.OrgID != orgID {
		return nil, pip.ErrNotFound
	}
	p.Status = status
	return p, nil
}

func (r *stubRepo) FindCheckins(_ context.Context, pipID string) ([]*pip.Checkin, error) {
	return r.checkins[pipID], nil
}

func (r *stubRepo) CreateCheckin(_ context.Context, ch *pip.Checkin) error {
	ch.ID = r.nextID("pipc")
	ch.PublicID = "pipc_pub_" + ch.ID
	ch.CheckedInAt = time.Now()
	r.checkins[ch.PIPID] = append(r.checkins[ch.PIPID], ch)
	return nil
}

func (r *stubRepo) ExtendWithCheckin(_ context.Context, orgID string, p *pip.PIP, ch *pip.Checkin) error {
	stored, ok := r.plans[p.ID]
	if !ok || stored.OrgID != orgID {
		return pip.ErrNotFound
	}
	stored.EndDate = p.EndDate
	stored.Status = pip.StatusExtended
	p.Status = pip.StatusExtended
	return r.CreateCheckin(context.Background(), ch)
}

func (r *stubRepo) CloseWithCheckin(_ context.Context, orgID string, p *pip.PIP, ch *pip.Checkin) error {
	stored, ok := r.plans[p.ID]
	if !ok || stored.OrgID != orgID {
		return pip.ErrNotFound
	}
	now := time.Now()
	stored.Status = pip.StatusClosed
	stored.Outcome = p.Outcome
	stored.ClosedAt = &now
	stored.ClosedBy = p.ClosedBy
	p.Status = pip.StatusClosed
	p.ClosedAt = &now
	return r.CreateCheckin(context.Background(), ch)
}

func (r *stubRepo) LinkTermination(_ context.Context, orgID, id, terminationID string) error {
	p, ok := r.plans[id]
	if !ok || p.OrgID != orgID {
		return pip.ErrNotFound
	}
	p.TerminationID = &terminationID
	return nil
}

func (r *stubRepo) FindEmployeeRef(_ context.Context, _, employeeRef string) (*pip.EmployeeRef, error) {
	if e, ok := r.employees[employeeRef]; ok {
		return e, nil
	}
	return nil, nil
}

type stubAuthorizer struct{ allow bool }

func (a *stubAuthorizer) AuthorizeRecordAccess(_ context.Context, tier authz.Scope, _, _, _ string) (bool, error) {
	if tier == authz.ScopeAll {
		return true, nil
	}
	return a.allow, nil
}

// stubTerminations records what the handoff asked for, and can be made to
// fail so the partial-success path is exercised.
type stubTerminations struct {
	seq      int
	fail     error
	requests []pip.DraftTerminationRequest
	orgIDs   []string
	empIDs   []string
}

var _ pip.TerminationCreator = (*stubTerminations)(nil)

func (s *stubTerminations) CreateDraftFromPIP(_ context.Context, orgID, employeeID, _ string, req pip.DraftTerminationRequest) (string, error) {
	if s.fail != nil {
		return "", s.fail
	}
	s.seq++
	s.requests = append(s.requests, req)
	s.orgIDs = append(s.orgIDs, orgID)
	s.empIDs = append(s.empIDs, employeeID)
	return fmt.Sprintf("term_%d", s.seq), nil
}

// ── Fixtures ─────────────────────────────────────────────────────────────────

func newSvc(allow bool) (pip.Service, *stubRepo, *stubTerminations) {
	repo := newStubRepo()
	repo.employees[empID] = &pip.EmployeeRef{
		EmployeeID: empID, DisplayName: "Under Review", ManagerEmployeeID: strPtr(otherEmpID),
	}
	repo.employees[otherEmpID] = &pip.EmployeeRef{EmployeeID: otherEmpID, DisplayName: "The Manager"}
	terms := &stubTerminations{}
	return pip.NewService(repo, &stubAuthorizer{allow: allow}, terms), repo, terms
}

func adminCaller() pip.Caller {
	return pip.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanManage: true, CanClose: true}
}

// managerCaller holds manage but NOT close — the grant 00091 gives 'manager'.
func managerCaller() pip.Caller {
	return pip.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanManage: true, CanClose: false}
}

func seedActivePlan(t *testing.T, svc pip.Service) *pip.Detail {
	t.Helper()
	ctx := context.Background()
	p, err := svc.Create(ctx, testOrg, ownerUserID, adminCaller(), pip.CreateRequest{
		EmployeeID: empID, Title: "Improve delivery predictability",
		Concerns: "Three consecutive missed commitments", SuccessCriteria: "Two consecutive on-time sprints",
		StartDate: "2030-01-01", EndDate: "2030-03-31",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	out, err := svc.Activate(ctx, testOrg, p.ID, adminCaller())
	if err != nil {
		t.Fatalf("activate plan: %v", err)
	}
	return out
}

func strPtr(s string) *string { return &s }

// ============================================================
// The failed-PIP handoff
// ============================================================

// TestClose_FailedCreatesDraftTerminationAndStops is the headline test. The
// build plan requires the handoff; the "and stops" half is what keeps a PIP
// from routing around the approval chain that gates dismissals.
func TestClose_FailedCreatesDraftTerminationAndStops(t *testing.T) {
	ctx := context.Background()
	svc, repo, terms := newSvc(true)
	p := seedActivePlan(t, svc)

	out, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
		Outcome: pip.OutcomeFailed, Note: "Criteria not met after full period",
	})
	if err != nil {
		t.Fatalf("close as failed: %v", err)
	}

	if out.Status != pip.StatusClosed {
		t.Errorf("expected the plan closed, got %s", out.Status)
	}
	if out.Outcome == nil || *out.Outcome != pip.OutcomeFailed {
		t.Errorf("expected outcome failed, got %v", out.Outcome)
	}
	if len(terms.requests) != 1 {
		t.Fatalf("expected exactly one draft termination, got %d", len(terms.requests))
	}
	if terms.empIDs[0] != empID {
		t.Errorf("draft termination raised against %s, want %s", terms.empIDs[0], empID)
	}
	// The plan's public id must appear in the reason, or the termination is
	// untraceable back to the process that produced it.
	if got := terms.requests[0].Reason; !strings.Contains(got, p.PublicID) {
		t.Errorf("termination reason %q should name the PIP %s", got, p.PublicID)
	}
	// The termination date defaults to when the plan ran out.
	if terms.requests[0].TerminationDate != "2030-03-31" {
		t.Errorf("termination date = %s, want the plan's end date", terms.requests[0].TerminationDate)
	}

	if out.TerminationID == nil {
		t.Fatal("the draft termination was not linked back onto the plan")
	}
	if stored := repo.plans[p.ID]; stored.TerminationID == nil {
		t.Error("the link was not persisted")
	}
}

// TestClose_NonFailedOutcomesCreateNoTermination pins the other half: a plan
// that succeeded or was abandoned must not produce a dismissal document.
func TestClose_NonFailedOutcomesCreateNoTermination(t *testing.T) {
	for _, outcome := range []pip.Outcome{pip.OutcomeSuccessful, pip.OutcomeAbandoned} {
		t.Run(string(outcome), func(t *testing.T) {
			ctx := context.Background()
			svc, _, terms := newSvc(true)
			p := seedActivePlan(t, svc)

			out, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
				Outcome: outcome, Note: "Closing",
			})
			if err != nil {
				t.Fatalf("close as %s: %v", outcome, err)
			}
			if len(terms.requests) != 0 {
				t.Errorf("a %s outcome must create no termination, got %d", outcome, len(terms.requests))
			}
			if out.TerminationID != nil {
				t.Errorf("a %s outcome must leave termination_id nil", outcome)
			}
		})
	}
}

// TestClose_HandoffFailureStillClosesThePlan pins the partial-success
// contract. The PIP closes first in its own transaction; if the downstream
// draft cannot be created, the plan is still closed and the caller is told
// so — reporting a plain error would imply nothing happened, and invite a
// retry that then fails with ErrAlreadyClosed.
func TestClose_HandoffFailureStillClosesThePlan(t *testing.T) {
	ctx := context.Background()
	svc, repo, terms := newSvc(true)
	terms.fail = errors.New("terminations unavailable")
	p := seedActivePlan(t, svc)

	out, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
		Outcome: pip.OutcomeFailed, Note: "Criteria not met",
	})
	if !errors.Is(err, pip.ErrTerminationHandoff) {
		t.Fatalf("expected ErrTerminationHandoff, got %v", err)
	}
	if !pip.IsHandoffFailure(err) {
		t.Error("IsHandoffFailure must recognise the sentinel a handler branches on")
	}
	if out == nil {
		t.Fatal("the plan must still be returned — it IS closed, and the caller needs to know")
	}
	if stored := repo.plans[p.ID]; stored.Status != pip.StatusClosed {
		t.Errorf("the plan must be closed despite the handoff failure, got %s", stored.Status)
	}
	if out.TerminationID != nil {
		t.Error("no termination was created, so nothing should be linked")
	}
}

// TestClose_RequiresCloseNotJustManage pins the permission split 00091 seeds:
// 'manager' runs the plan but does not get to pull the trigger.
func TestClose_RequiresCloseNotJustManage(t *testing.T) {
	ctx := context.Background()
	svc, _, terms := newSvc(true)
	p := seedActivePlan(t, svc)

	_, err := svc.Close(ctx, testOrg, p.ID, managerCaller(), pip.CloseRequest{
		Outcome: pip.OutcomeFailed, Note: "Not met",
	})
	if !errors.Is(err, pip.ErrCloseDenied) {
		t.Fatalf("expected ErrCloseDenied for a caller holding manage but not close, got %v", err)
	}
	if len(terms.requests) != 0 {
		t.Error("a denied close must not have reached the handoff")
	}
}

func TestClose_RequiresOutcomeAndNote(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	p := seedActivePlan(t, svc)

	if _, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
		Note: "no outcome given",
	}); !errors.Is(err, pip.ErrOutcomeRequired) {
		t.Errorf("expected ErrOutcomeRequired, got %v", err)
	}
	if _, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
		Outcome: pip.OutcomeFailed,
	}); !errors.Is(err, pip.ErrNoteRequired) {
		t.Errorf("expected ErrNoteRequired, got %v", err)
	}
	if _, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
		Outcome: pip.Outcome("nonsense"), Note: "x",
	}); !errors.Is(err, pip.ErrInvalidOutcome) {
		t.Errorf("expected ErrInvalidOutcome, got %v", err)
	}
}

func TestClose_RejectsSecondClose(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	p := seedActivePlan(t, svc)

	if _, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
		Outcome: pip.OutcomeSuccessful, Note: "Met",
	}); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if _, err := svc.Close(ctx, testOrg, p.ID, adminCaller(), pip.CloseRequest{
		Outcome: pip.OutcomeFailed, Note: "second thoughts",
	}); !errors.Is(err, pip.ErrAlreadyClosed) {
		t.Fatalf("expected ErrAlreadyClosed, got %v", err)
	}
}

// ============================================================
// Extensions
// ============================================================

// TestExtend_RecordsBothDatesAndPreservesTheOriginal pins why
// original_end_date exists: a PIP whose end date silently moves is the
// documented failure mode of the whole instrument.
func TestExtend_RecordsBothDatesAndPreservesTheOriginal(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	p := seedActivePlan(t, svc)
	originalEnd := p.EndDate

	out, err := svc.Extend(ctx, testOrg, p.ID, adminCaller(), pip.ExtendRequest{
		NewEndDate: "2030-04-30", Note: "Partial progress; one more month",
	})
	if err != nil {
		t.Fatalf("extend: %v", err)
	}

	if !out.OriginalEndDate.Equal(originalEnd) {
		t.Errorf("original_end_date moved: %s, want %s", out.OriginalEndDate, originalEnd)
	}
	if out.EndDate.Format("2006-01-02") != "2030-04-30" {
		t.Errorf("end_date = %s, want 2030-04-30", out.EndDate)
	}
	if !out.WasExtended {
		t.Error("WasExtended must be true after an extension")
	}
	if out.Status != pip.StatusExtended {
		t.Errorf("expected status extended, got %s", out.Status)
	}

	var extension *pip.Checkin
	for _, ch := range out.Checkins {
		if ch.EntryType == pip.EntryExtension {
			extension = ch
		}
	}
	if extension == nil {
		t.Fatal("an extension must append a check-in — one cannot exist without a written reason")
	}
	if extension.PreviousEndDate == nil || !extension.PreviousEndDate.Equal(originalEnd) {
		t.Errorf("the extension entry lost the previous end date: %v", extension.PreviousEndDate)
	}
	if extension.NewEndDate == nil {
		t.Error("the extension entry lost the new end date")
	}
	if extension.Note == "" {
		t.Error("the extension reason was not recorded")
	}
}

func TestExtend_MustMoveForward(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	p := seedActivePlan(t, svc)

	if _, err := svc.Extend(ctx, testOrg, p.ID, adminCaller(), pip.ExtendRequest{
		NewEndDate: "2030-02-01", Note: "shorten it",
	}); !errors.Is(err, pip.ErrExtensionBackwards) {
		t.Fatalf("expected ErrExtensionBackwards, got %v", err)
	}
}

func TestExtend_RequiresNote(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	p := seedActivePlan(t, svc)

	if _, err := svc.Extend(ctx, testOrg, p.ID, adminCaller(), pip.ExtendRequest{
		NewEndDate: "2030-04-30",
	}); !errors.Is(err, pip.ErrNoteRequired) {
		t.Fatalf("expected ErrNoteRequired, got %v", err)
	}
}

// TestUpdate_CannotMoveTheEndDate proves Extend is the ONLY path. If Update
// could move it, the written-reason requirement would be optional in
// practice, which is the same as absent.
func TestUpdate_CannotMoveTheEndDate(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	p := seedActivePlan(t, svc)
	before := p.EndDate

	out, err := svc.Update(ctx, testOrg, p.ID, adminCaller(), pip.UpdateRequest{
		Title: strPtr("Renamed plan"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !out.EndDate.Equal(before) {
		t.Errorf("end_date moved through Update: %s, want %s", out.EndDate, before)
	}
	if out.Title != "Renamed plan" {
		t.Errorf("the editable field did not change: %s", out.Title)
	}
}

// ============================================================
// Creation rules
// ============================================================

// TestCreate_OnePlanPerEmployee mirrors the partial unique index. Two
// overlapping plans make "did they pass" unanswerable.
func TestCreate_OnePlanPerEmployee(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	seedActivePlan(t, svc)

	_, err := svc.Create(ctx, testOrg, ownerUserID, adminCaller(), pip.CreateRequest{
		EmployeeID: empID, Title: "Second plan", Concerns: "More concerns",
		SuccessCriteria: "More criteria", StartDate: "2030-02-01", EndDate: "2030-05-31",
	})
	if !errors.Is(err, pip.ErrAlreadyOpen) {
		t.Fatalf("expected ErrAlreadyOpen, got %v", err)
	}
}

// TestCreate_SuccessCriteriaRequired pins the rule that a plan without
// stated criteria is unmeetable by construction.
func TestCreate_RequiresConcernsAndCriteria(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)

	base := pip.CreateRequest{
		EmployeeID: empID, Title: "Plan", Concerns: "Concerns",
		SuccessCriteria: "Criteria", StartDate: "2030-01-01", EndDate: "2030-03-31",
	}
	noConcerns := base
	noConcerns.Concerns = "   "
	if _, err := svc.Create(ctx, testOrg, ownerUserID, adminCaller(), noConcerns); !errors.Is(err, pip.ErrConcernsRequired) {
		t.Errorf("expected ErrConcernsRequired, got %v", err)
	}
	noCriteria := base
	noCriteria.SuccessCriteria = ""
	if _, err := svc.Create(ctx, testOrg, ownerUserID, adminCaller(), noCriteria); !errors.Is(err, pip.ErrCriteriaRequired) {
		t.Errorf("expected ErrCriteriaRequired, got %v", err)
	}
}

// TestCreate_FreezesTheManager pins the snapshot: a reorg mid-plan must not
// hand someone else's dismissal process to a manager who never opened it.
func TestCreate_FreezesTheManager(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	p := seedActivePlan(t, svc)

	if p.ManagerEmployeeID == nil || *p.ManagerEmployeeID != otherEmpID {
		t.Fatalf("manager not frozen at creation: %v", p.ManagerEmployeeID)
	}
	// The employee's manager changes afterwards; the plan must not follow.
	repo.employees[empID].ManagerEmployeeID = strPtr("emp_someone_else")
	out, err := svc.Get(ctx, testOrg, p.ID, adminCaller())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.ManagerEmployeeID == nil || *out.ManagerEmployeeID != otherEmpID {
		t.Errorf("the frozen manager moved with the reorg: %v", out.ManagerEmployeeID)
	}
}

// ============================================================
// Authorization
// ============================================================

// TestWrites_RequireManageAndScope pins the two-part narrowing. hrm.pips
// .manage is unscoped at the route, so the record check is the only thing
// stopping a view_team manager opening a plan outside their reporting line —
// and that middle case is the one people forget.
func TestWrites_RequireManageAndScope(t *testing.T) {
	ctx := context.Background()
	req := pip.CreateRequest{
		EmployeeID: empID, Title: "Plan", Concerns: "C", SuccessCriteria: "S",
		StartDate: "2030-01-01", EndDate: "2030-03-31",
	}

	t.Run("no manage", func(t *testing.T) {
		svc, _, _ := newSvc(true)
		caller := pip.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanManage: false}
		if _, err := svc.Create(ctx, testOrg, ownerUserID, caller, req); !errors.Is(err, pip.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied without manage, got %v", err)
		}
	})

	t.Run("manage but out of scope", func(t *testing.T) {
		svc, _, _ := newSvc(false) // authorizer refuses anything below ScopeAll
		caller := pip.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: true}
		if _, err := svc.Create(ctx, testOrg, ownerUserID, caller, req); !errors.Is(err, pip.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied for an employee outside the caller's tier, got %v", err)
		}
	})

	t.Run("manage and in scope", func(t *testing.T) {
		svc, _, _ := newSvc(true)
		caller := pip.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: true}
		if _, err := svc.Create(ctx, testOrg, ownerUserID, caller, req); err != nil {
			t.Errorf("a manager acting inside their tier should succeed: %v", err)
		}
	})
}

func TestGet_DeniesOutOfScope(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(false)
	p := seedActivePlan(t, svc)

	outsider := pip.Caller{UserID: "usr_stranger", Tier: authz.ScopeOwn}
	if _, err := svc.Get(ctx, testOrg, p.ID, outsider); !errors.Is(err, pip.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

// ============================================================
// Check-ins and lifecycle
// ============================================================

func TestAddCheckin_RequiresActivePlan(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)

	p, err := svc.Create(ctx, testOrg, ownerUserID, adminCaller(), pip.CreateRequest{
		EmployeeID: empID, Title: "Draft plan", Concerns: "C", SuccessCriteria: "S",
		StartDate: "2030-01-01", EndDate: "2030-03-31",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Still a draft.
	if _, err := svc.AddCheckin(ctx, testOrg, p.ID, adminCaller(), pip.CheckinRequest{
		Note: "early note",
	}); !errors.Is(err, pip.ErrNotActive) {
		t.Fatalf("expected ErrNotActive on a draft plan, got %v", err)
	}

	if _, err := svc.Activate(ctx, testOrg, p.ID, adminCaller()); err != nil {
		t.Fatalf("activate: %v", err)
	}
	progress := pip.ProgressPartial
	out, err := svc.AddCheckin(ctx, testOrg, p.ID, adminCaller(), pip.CheckinRequest{
		Progress: &progress, Note: "Halfway, mixed results",
	})
	if err != nil {
		t.Fatalf("checkin: %v", err)
	}
	if len(out.Checkins) != 1 {
		t.Fatalf("expected 1 check-in, got %d", len(out.Checkins))
	}
	if out.Checkins[0].EntryType != pip.EntryReview {
		t.Errorf("expected a review entry, got %s", out.Checkins[0].EntryType)
	}
}

// TestCancel_IsDistinctFromAbandoned pins the distinction: cancelling says
// the plan should not have been opened, abandoning says it ran and was
// dropped. Conflating them loses the difference.
func TestCancel_IsDistinctFromAbandoned(t *testing.T) {
	ctx := context.Background()
	svc, _, terms := newSvc(true)
	p := seedActivePlan(t, svc)

	out, err := svc.Cancel(ctx, testOrg, p.ID, adminCaller())
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if out.Status != pip.StatusCancelled {
		t.Errorf("expected cancelled, got %s", out.Status)
	}
	if out.Outcome != nil {
		t.Errorf("cancelling records no outcome, got %v", out.Outcome)
	}
	if len(terms.requests) != 0 {
		t.Error("cancelling must never reach the terminations handoff")
	}
	// And a cancelled plan frees the employee for a new one.
	if _, err := svc.Create(ctx, testOrg, ownerUserID, adminCaller(), pip.CreateRequest{
		EmployeeID: empID, Title: "Fresh start", Concerns: "C", SuccessCriteria: "S",
		StartDate: "2030-04-01", EndDate: "2030-06-30",
	}); err != nil {
		t.Errorf("a cancelled plan should free the employee for a new one: %v", err)
	}
}
