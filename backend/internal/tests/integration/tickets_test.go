// backend/internal/tests/integration/tickets_test.go
// platform/tickets against real Postgres. Three claims here can only be
// proved live: that no elapsed/paused/breached column exists, that the
// requester's comment read path never SELECTs an internal row, and that the
// pause ledger actually excludes paused time from the SLA clock.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/platform/tickets"
)

// ticketFixture is one org with an owner (full agent rights), a plain member
// who raises tickets, and a category.
type ticketFixture struct {
	orgID      string
	statusID   string
	ownerID    string
	memberID   string
	employeeID string
	categoryID string
}

// seedOrgMember signs up a user and gives them a membership at roleKey.
func seedOrgMember(t *testing.T, env *testEnv, orgID, roleKey, emailPrefix string) string {
	t.Helper()
	u, err := env.authSvc.Signup(context.Background(),
		auth.SignupRequest{Email: uniqueEmail(emailPrefix), Password: "TicketTestPass123!"})
	if err != nil {
		t.Fatalf("signup %s: %v", emailPrefix, err)
	}
	t.Cleanup(func() { cleanupUser(t, env, u.ID) })
	insertRawMembership(t, env, orgID, u.ID, nil, roleKey, "active", "accepted")
	return u.ID
}

func seedTicketFixture(t *testing.T, env *testEnv) *ticketFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	memberID := seedOrgMember(t, env, orgID, "member", "tkt-member")
	empID := seedEmployee(t, env, orgID, statusID, ownerID, memberID, "Ticket Requester", nil)

	cat, err := env.ticketsSvc.CreateCategory(ctx, orgID, ownerID, tickets.CreateCategoryRequest{
		Name: "IT Support " + uniqueSlug("tcat"),
	})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	return &ticketFixture{
		orgID: orgID, statusID: statusID, ownerID: ownerID,
		memberID: memberID, employeeID: empID, categoryID: cat.ID,
	}
}

func raiseTicket(t *testing.T, env *testEnv, fx *ticketFixture, subject string) *tickets.Ticket {
	t.Helper()
	tk, err := env.ticketsSvc.Create(context.Background(), fx.orgID, fx.memberID, tickets.CreateTicketRequest{
		RequesterID: fx.employeeID,
		CategoryID:  &fx.categoryID,
		Subject:     subject,
	})
	if err != nil {
		t.Fatalf("raise ticket %q: %v", subject, err)
	}
	return tk
}

// ============================================================
// The structural claims
// ============================================================

// TestIntegration_Tickets_NoStoredSLAColumnsExist is migration 00110's
// central promise: elapsed, paused and breached are computed from the event
// ledger on every read, never stored. A mutable paused_minutes counter shows
// the number but never how it got there, and drifts the first time a pause
// is recorded without updating it.
//
// Introspecting information_schema is the only way to prove a column is
// ABSENT — the 6A completion-percentage and 8A current-holder precedent.
func TestIntegration_Tickets_NoStoredSLAColumnsExist(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	forbidden := []struct{ table, column string }{
		{"platform_tickets", "elapsed_minutes"},
		{"platform_tickets", "paused_minutes"},
		{"platform_tickets", "paused_duration"},
		{"platform_tickets", "total_paused_seconds"},
		{"platform_tickets", "sla_breached"},
		{"platform_tickets", "is_breached"},
		{"platform_tickets", "first_response_due_at"},
		{"platform_tickets", "resolution_due_at"},
		{"platform_tickets", "comment_count"},
	}
	for _, f := range forbidden {
		var n int
		err := env.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.columns
			  WHERE table_name = $1 AND column_name = $2`, f.table, f.column).Scan(&n)
		if err != nil {
			t.Fatalf("introspect %s.%s: %v", f.table, f.column, err)
		}
		if n != 0 {
			t.Errorf("%s.%s exists — SLA figures must be derived from platform_ticket_sla_events, not stored",
				f.table, f.column)
		}
	}
}

// TestIntegration_Tickets_SLAEventsAreAppendOnly proves the ledger has no
// updated_at, so there is no shape in which an event is edited rather than
// superseded — the hrm_loan_recovery_events discipline.
func TestIntegration_Tickets_SLAEventsAreAppendOnly(t *testing.T) {
	env := newTestEnv(t)
	var n int
	err := env.db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM information_schema.columns
		  WHERE table_name = 'platform_ticket_sla_events' AND column_name = 'updated_at'`).Scan(&n)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if n != 0 {
		t.Error("platform_ticket_sla_events.updated_at exists — the ledger must be append-only")
	}
}

// TestIntegration_Tickets_NoHrmReferences proves this platform table never
// gained an FK into hrm_*. requester_id is deliberately FK-free so widening
// requester_type to 'contact' is a CHECK change rather than an untangling.
func TestIntegration_Tickets_NoHrmReferences(t *testing.T) {
	env := newTestEnv(t)
	rows, err := env.db.Query(context.Background(),
		`SELECT ccu.table_name
		   FROM information_schema.table_constraints tc
		   JOIN information_schema.constraint_column_usage ccu
		     ON ccu.constraint_name = tc.constraint_name
		  WHERE tc.constraint_type = 'FOREIGN KEY'
		    AND tc.table_name LIKE 'platform_ticket%'`)
	if err != nil {
		t.Fatalf("introspect FKs: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if len(target) >= 4 && target[:4] == "hrm_" {
			t.Errorf("a platform_ticket* table has an FK to %s — platform must never reference hrm_*", target)
		}
	}
}

// ============================================================
// Internal comments — the confidentiality claim
// ============================================================

// TestIntegration_Tickets_InternalCommentsInvisibleToRequester is the claim
// the two-read-path design exists to make true. It calls the SERVICE as the
// requester, not a handler, so nothing about this depends on a handler
// remembering to strip a field.
func TestIntegration_Tickets_InternalCommentsInvisibleToRequester(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	tk := raiseTicket(t, env, fx, "Laptop will not boot")

	// The requester's own public comment.
	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.memberID, tk.ID,
		tickets.CreateCommentRequest{Body: "It started this morning."}); err != nil {
		t.Fatalf("requester comment: %v", err)
	}
	// An agent's public reply and an agent-to-agent internal note.
	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.CreateCommentRequest{Body: "We are looking into it."}); err != nil {
		t.Fatalf("agent public comment: %v", err)
	}
	internal := true
	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.CreateCommentRequest{Body: "Warranty expired, do not promise a replacement.", IsInternal: &internal}); err != nil {
		t.Fatalf("agent internal comment: %v", err)
	}

	asRequester, err := env.ticketsSvc.ListComments(ctx, fx.orgID, fx.memberID, tk.ID)
	if err != nil {
		t.Fatalf("list as requester: %v", err)
	}
	if len(asRequester) != 2 {
		t.Fatalf("requester sees %d comments, want 2 (both public ones)", len(asRequester))
	}
	for _, c := range asRequester {
		if c.IsInternal {
			t.Errorf("requester can see internal comment %s — the two read paths have collapsed", c.PublicID)
		}
	}

	asAgent, err := env.ticketsSvc.ListComments(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("list as agent: %v", err)
	}
	if len(asAgent) != 3 {
		t.Fatalf("agent sees %d comments, want 3 — an agent must see the internal note", len(asAgent))
	}
}

// TestIntegration_Tickets_RepositoryPublicPathNeverSelectsInternal goes below
// the service and asserts the repository method itself. If FindPublicComments
// ever loses its WHERE clause, the service could still look correct while
// having internal rows in memory — which is the leak this design forbids.
func TestIntegration_Tickets_RepositoryPublicPathNeverSelectsInternal(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	tk := raiseTicket(t, env, fx, "Repository path probe")

	internal := true
	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.CreateCommentRequest{Body: "internal only", IsInternal: &internal}); err != nil {
		t.Fatalf("internal comment: %v", err)
	}

	repo := tickets.NewRepository(env.db)
	public, err := repo.FindPublicComments(ctx, fx.orgID, tk.ID)
	if err != nil {
		t.Fatalf("FindPublicComments: %v", err)
	}
	if len(public) != 0 {
		t.Errorf("FindPublicComments returned %d rows for a ticket with only an internal comment", len(public))
	}
	all, err := repo.FindAllComments(ctx, fx.orgID, tk.ID)
	if err != nil {
		t.Fatalf("FindAllComments: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("FindAllComments returned %d rows, want 1", len(all))
	}
}

// TestIntegration_Tickets_MemberCannotWriteInternalComment proves .comment
// alone does not authorise an internal note. A requester who could author one
// would be writing something they cannot read back.
func TestIntegration_Tickets_MemberCannotWriteInternalComment(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	tk := raiseTicket(t, env, fx, "Internal write probe")

	internal := true
	_, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.memberID, tk.ID,
		tickets.CreateCommentRequest{Body: "sneaky", IsInternal: &internal})
	if err == nil {
		t.Fatal("a member wrote an internal comment — comment_internal is not granted to members")
	}
}

// ============================================================
// The pausable SLA clock, end to end
// ============================================================

// TestIntegration_Tickets_PauseExcludesElapsedTime is the ledger's whole
// purpose. It backdates the ticket and its events through raw SQL because the
// arithmetic only becomes observable over a span longer than a test can wait.
func TestIntegration_Tickets_PauseExcludesElapsedTime(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)

	// A 4-hour first-response / 8-hour resolution target for this category.
	if _, err := env.ticketsSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, tickets.CreateSLAPolicyRequest{
		CategoryID: &fx.categoryID, Priority: "normal",
		FirstResponseMinutes: 240, ResolutionMinutes: 480,
	}); err != nil {
		t.Fatalf("create policy: %v", err)
	}

	tk := raiseTicket(t, env, fx, "Pause arithmetic probe")
	if tk.SLAPolicyID == nil {
		t.Fatal("ticket got no SLA policy — ResolvePolicy did not match the category+priority row")
	}

	if _, err := env.ticketsSvc.Pause(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.PauseTicketRequest{Reason: strp("waiting on the requester")}); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if _, err := env.ticketsSvc.Resume(ctx, fx.orgID, fx.ownerID, tk.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}

	// Rewrite the timeline: raised 6h ago, paused from -5h to -2h (3 hours).
	// Wall clock 6h against a 4h target would be a breach; 3h of working
	// time is not.
	if _, err := env.db.Exec(ctx,
		`UPDATE platform_tickets SET created_at = NOW() - INTERVAL '6 hours' WHERE id = $1`, tk.ID); err != nil {
		t.Fatalf("backdate ticket: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`UPDATE platform_ticket_sla_events SET occurred_at = NOW() - INTERVAL '5 hours'
		  WHERE ticket_id = $1 AND event_type = 'pause'`, tk.ID); err != nil {
		t.Fatalf("backdate pause: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`UPDATE platform_ticket_sla_events SET occurred_at = NOW() - INTERVAL '2 hours'
		  WHERE ticket_id = $1 AND event_type = 'resume'`, tk.ID); err != nil {
		t.Fatalf("backdate resume: %v", err)
	}

	got, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FirstResponseSLA == nil {
		t.Fatal("no first-response SLA attached")
	}
	elapsed := got.FirstResponseSLA.Elapsed
	if elapsed < 2*time.Hour+50*time.Minute || elapsed > 3*time.Hour+10*time.Minute {
		t.Errorf("elapsed = %v, want ~3h (6h wall clock minus a 3h pause)", elapsed)
	}
	if got.FirstResponseSLA.Breached {
		t.Errorf("breached at %v against a 4h target — the pause was not excluded", elapsed)
	}
}

// TestIntegration_Tickets_TwoPausesBeatASingleCounter is the case a mutable
// paused_minutes column cannot represent honestly: the same ticket paused and
// resumed twice, where the total is only meaningful alongside its history.
func TestIntegration_Tickets_TwoPausesBeatASingleCounter(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)

	if _, err := env.ticketsSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, tickets.CreateSLAPolicyRequest{
		CategoryID: &fx.categoryID, Priority: "normal",
		FirstResponseMinutes: 240, ResolutionMinutes: 480,
	}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	tk := raiseTicket(t, env, fx, "Two pauses probe")

	for i := 0; i < 2; i++ {
		if _, err := env.ticketsSvc.Pause(ctx, fx.orgID, fx.ownerID, tk.ID, tickets.PauseTicketRequest{}); err != nil {
			t.Fatalf("pause %d: %v", i+1, err)
		}
		if _, err := env.ticketsSvc.Resume(ctx, fx.orgID, fx.ownerID, tk.ID); err != nil {
			t.Fatalf("resume %d: %v", i+1, err)
		}
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_ticket_sla_events WHERE ticket_id = $1`, tk.ID).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if n != 4 {
		t.Fatalf("ledger has %d events, want 4 — two pause/resume pairs", n)
	}

	// Ten hours ago, paused -9h→-7h and -5h→-4h: three hours excluded.
	stamp := func(eventType string, ord int, interval string) {
		if _, err := env.db.Exec(ctx,
			`UPDATE platform_ticket_sla_events SET occurred_at = NOW() - `+interval+`
			  WHERE id = (SELECT id FROM platform_ticket_sla_events
			               WHERE ticket_id = $1 AND event_type = $2
			               ORDER BY occurred_at OFFSET $3 LIMIT 1)`,
			tk.ID, eventType, ord); err != nil {
			t.Fatalf("stamp %s#%d: %v", eventType, ord, err)
		}
	}
	if _, err := env.db.Exec(ctx,
		`UPDATE platform_tickets SET created_at = NOW() - INTERVAL '10 hours' WHERE id = $1`, tk.ID); err != nil {
		t.Fatalf("backdate ticket: %v", err)
	}
	stamp("pause", 0, "INTERVAL '9 hours'")
	stamp("resume", 0, "INTERVAL '7 hours'")
	stamp("pause", 1, "INTERVAL '5 hours'")
	stamp("resume", 1, "INTERVAL '4 hours'")

	got, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	elapsed := got.ResolutionSLA.Elapsed
	if elapsed < 6*time.Hour+50*time.Minute || elapsed > 7*time.Hour+10*time.Minute {
		t.Errorf("elapsed = %v, want ~7h (10h wall clock minus two pauses totalling 3h)", elapsed)
	}
}

// TestIntegration_Tickets_DoublePauseRejected proves the service refuses a
// second pause rather than writing a ledger the arithmetic has to guess at.
func TestIntegration_Tickets_DoublePauseRejected(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	tk := raiseTicket(t, env, fx, "Double pause probe")

	if _, err := env.ticketsSvc.Pause(ctx, fx.orgID, fx.ownerID, tk.ID, tickets.PauseTicketRequest{}); err != nil {
		t.Fatalf("first pause: %v", err)
	}
	// Asserting the SPECIFIC sentinel, not merely that it errored: a paused
	// ticket sits in status 'paused', so a status guard placed ahead of the
	// ledger check answers this with ErrWrongStatus and leaves the caller
	// guessing. The live smoke run found exactly that.
	_, err := env.ticketsSvc.Pause(ctx, fx.orgID, fx.ownerID, tk.ID, tickets.PauseTicketRequest{})
	if !errors.Is(err, tickets.ErrAlreadyPaused) {
		t.Errorf("second pause returned %v, want ErrAlreadyPaused", err)
	}
	if _, err := env.ticketsSvc.Resume(ctx, fx.orgID, fx.ownerID, tk.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}
	_, err = env.ticketsSvc.Resume(ctx, fx.orgID, fx.ownerID, tk.ID)
	if !errors.Is(err, tickets.ErrNotPaused) {
		t.Errorf("second resume returned %v, want ErrNotPaused", err)
	}
}

// TestIntegration_Tickets_SLASerialisesAsMinutes pins the wire format. A bare
// time.Duration marshals to a raw nanosecond count, which every client then
// has to know to divide; minutes match the unit the policy is configured in.
func TestIntegration_Tickets_SLASerialisesAsMinutes(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)

	if _, err := env.ticketsSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, tickets.CreateSLAPolicyRequest{
		CategoryID: &fx.categoryID, Priority: "normal",
		FirstResponseMinutes: 240, ResolutionMinutes: 480,
	}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	tk := raiseTicket(t, env, fx, "Wire format probe")
	got, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	raw, err := json.Marshal(got.FirstResponseSLA)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, forbidden := range []string{"Elapsed", "Target", "Remaining"} {
		if _, ok := wire[forbidden]; ok {
			t.Errorf("%s is on the wire as a raw time.Duration", forbidden)
		}
	}
	if v, ok := wire["target_minutes"].(float64); !ok || int(v) != 240 {
		t.Errorf("target_minutes = %v, want 240", wire["target_minutes"])
	}
	if v, ok := wire["remaining_minutes"].(float64); !ok || int(v) != 239 {
		t.Errorf("remaining_minutes = %v, want 239 on a freshly raised ticket", wire["remaining_minutes"])
	}
	if got.FirstResponseSLA.TargetMinutes != int(got.FirstResponseSLA.Target/time.Minute) {
		t.Error("TargetMinutes and Target disagree — the pair must be derived, never assigned twice")
	}
}

// ============================================================
// Visibility narrowing — the platform stand-in for hrm scope tiers
// ============================================================

// TestIntegration_Tickets_MemberSeesOnlyTheirOwn proves the ListFilter
// narrowing. Without view_all a caller sees what they raised and what is
// assigned to them, and the two must agree: a filtered list is worthless if
// fetching the hidden ticket by id returns it anyway.
func TestIntegration_Tickets_MemberSeesOnlyTheirOwn(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	other := seedOrgMember(t, env, fx.orgID, "member", "tkt-other")
	otherEmp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, other, "Other Requester", nil)

	mine := raiseTicket(t, env, fx, "Mine")
	theirs, err := env.ticketsSvc.Create(ctx, fx.orgID, other, tickets.CreateTicketRequest{
		RequesterID: otherEmp, Subject: "Theirs",
	})
	if err != nil {
		t.Fatalf("create other ticket: %v", err)
	}

	res, err := env.ticketsSvc.List(ctx, fx.orgID, fx.memberID, tickets.ListFilter{})
	if err != nil {
		t.Fatalf("list as member: %v", err)
	}
	for _, tk := range res.Tickets {
		if tk.ID == theirs.ID {
			t.Error("a member's list included someone else's ticket")
		}
	}
	if res.Total != len(res.Tickets) {
		t.Errorf("Total %d disagrees with the %d rows returned — the count and list predicates have drifted",
			res.Total, len(res.Tickets))
	}

	// The single-row read must agree with the list.
	if _, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.memberID, theirs.ID); err == nil {
		t.Error("a member fetched someone else's ticket by id despite it being hidden from their list")
	}
	if _, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.memberID, mine.ID); err != nil {
		t.Errorf("a member could not fetch their own ticket: %v", err)
	}

	// An owner holds view_all and sees both.
	all, err := env.ticketsSvc.List(ctx, fx.orgID, fx.ownerID, tickets.ListFilter{})
	if err != nil {
		t.Fatalf("list as owner: %v", err)
	}
	seen := map[string]bool{}
	for _, tk := range all.Tickets {
		seen[tk.ID] = true
	}
	if !seen[mine.ID] || !seen[theirs.ID] {
		t.Error("an owner holding view_all did not see both tickets")
	}
}

// TestIntegration_Tickets_AssigneeGainsVisibility proves assignment widens
// reach: an agent without view_all still sees what has been handed to them.
func TestIntegration_Tickets_AssigneeGainsVisibility(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	agent := seedOrgMember(t, env, fx.orgID, "manager", "tkt-agent")

	tk := raiseTicket(t, env, fx, "Assignment visibility probe")
	if _, err := env.ticketsSvc.Get(ctx, fx.orgID, agent, tk.ID); err == nil {
		t.Fatal("a manager without view_all saw an unassigned ticket they did not raise")
	}
	if _, err := env.ticketsSvc.Assign(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.AssignTicketRequest{AssigneeUserID: agent}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, err := env.ticketsSvc.Get(ctx, fx.orgID, agent, tk.ID); err != nil {
		t.Errorf("the assignee could not read their own assigned ticket: %v", err)
	}
}

// ============================================================
// Sensitive categories
// ============================================================

// TestIntegration_Tickets_SensitiveCategoryRestrictsAssignee is why sensitive
// categories exist: a harassment queue must not be assignable to the general
// helpdesk.
func TestIntegration_Tickets_SensitiveCategoryRestrictsAssignee(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	sensitive := true
	role := "owner"

	cat, err := env.ticketsSvc.CreateCategory(ctx, fx.orgID, fx.ownerID, tickets.CreateCategoryRequest{
		Name: "Grievance " + uniqueSlug("tcat"), IsSensitive: &sensitive, RestrictedRole: &role,
	})
	if err != nil {
		t.Fatalf("create sensitive category: %v", err)
	}
	tk, err := env.ticketsSvc.Create(ctx, fx.orgID, fx.memberID, tickets.CreateTicketRequest{
		RequesterID: fx.employeeID, CategoryID: &cat.ID, Subject: "Confidential",
	})
	if err != nil {
		t.Fatalf("raise sensitive ticket: %v", err)
	}

	manager := seedOrgMember(t, env, fx.orgID, "manager", "tkt-mgr")
	_, err = env.ticketsSvc.Assign(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.AssignTicketRequest{AssigneeUserID: manager})
	if err == nil {
		t.Error("a sensitive ticket was assigned to a manager outside the restricted 'owner' role")
	}

	// The owner does hold the restricted role, so the same call succeeds.
	if _, err := env.ticketsSvc.Assign(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.AssignTicketRequest{AssigneeUserID: fx.ownerID}); err != nil {
		t.Errorf("assigning to a holder of the restricted role failed: %v", err)
	}
}

// TestIntegration_Tickets_SensitiveCategoryNeedsARole proves the pairing is
// enforced in Go rather than as a CHECK — a sensitive category with no
// restricted role restricts nothing while looking like it does.
func TestIntegration_Tickets_SensitiveCategoryNeedsARole(t *testing.T) {
	env := newTestEnv(t)
	fx := seedTicketFixture(t, env)
	sensitive := true
	_, err := env.ticketsSvc.CreateCategory(context.Background(), fx.orgID, fx.ownerID,
		tickets.CreateCategoryRequest{Name: "Broken " + uniqueSlug("tcat"), IsSensitive: &sensitive})
	if err == nil {
		t.Error("a sensitive category was created with no restricted_role")
	}
}

// ============================================================
// Lifecycle
// ============================================================

// TestIntegration_Tickets_FirstResponseStampedByAgentReplyOnly proves the SLA
// measures what the requester actually received: their own follow-up is not a
// response, and an internal note is not one either.
func TestIntegration_Tickets_FirstResponseStampedByAgentReplyOnly(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	tk := raiseTicket(t, env, fx, "First response probe")

	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.memberID, tk.ID,
		tickets.CreateCommentRequest{Body: "Any update?"}); err != nil {
		t.Fatalf("requester follow-up: %v", err)
	}
	after, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if after.FirstResponseAt != nil {
		t.Error("the requester's own comment stamped first_response_at")
	}

	internal := true
	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.CreateCommentRequest{Body: "check the warranty", IsInternal: &internal}); err != nil {
		t.Fatalf("internal note: %v", err)
	}
	after, err = env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if after.FirstResponseAt != nil {
		t.Error("an internal note stamped first_response_at — the requester never saw it")
	}

	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.ownerID, tk.ID,
		tickets.CreateCommentRequest{Body: "We have ordered a replacement."}); err != nil {
		t.Fatalf("agent reply: %v", err)
	}
	after, err = env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if after.FirstResponseAt == nil {
		t.Error("an agent's public reply did not stamp first_response_at")
	}
}

// TestIntegration_Tickets_RequesterMayCloseTheirOwn proves "never mind,
// sorted it myself" does not require agent rights, while closing someone
// else's does.
func TestIntegration_Tickets_RequesterMayCloseTheirOwn(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	tk := raiseTicket(t, env, fx, "Self close probe")

	closed, err := env.ticketsSvc.Close(ctx, fx.orgID, fx.memberID, tk.ID)
	if err != nil {
		t.Fatalf("requester closing their own ticket: %v", err)
	}
	if closed.Status != tickets.StatusClosed || closed.ClosedAt == nil {
		t.Errorf("status = %s, closed_at set = %v; want closed with a timestamp", closed.Status, closed.ClosedAt != nil)
	}
	if _, err := env.ticketsSvc.AddComment(ctx, fx.orgID, fx.memberID, tk.ID,
		tickets.CreateCommentRequest{Body: "one more thing"}); err == nil {
		t.Error("a comment was accepted on a closed ticket")
	}
}

// TestIntegration_Tickets_PolicyIsPinnedAtCreation proves a later policy edit
// cannot retroactively breach a ticket raised under the old target — the
// snapshot discipline 7B and 7D apply to prices, applied here to a target.
func TestIntegration_Tickets_PolicyIsPinnedAtCreation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)

	pol, err := env.ticketsSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, tickets.CreateSLAPolicyRequest{
		CategoryID: &fx.categoryID, Priority: "normal",
		FirstResponseMinutes: 240, ResolutionMinutes: 480,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	tk := raiseTicket(t, env, fx, "Policy pinning probe")
	if tk.SLAPolicyID == nil || *tk.SLAPolicyID != pol.ID {
		t.Fatalf("ticket pinned policy %v, want %s", tk.SLAPolicyID, pol.ID)
	}

	// A brand-new, much tighter policy must not capture the existing ticket.
	if _, err := env.ticketsSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, tickets.CreateSLAPolicyRequest{
		CategoryID: &fx.categoryID, Priority: "urgent",
		FirstResponseMinutes: 5, ResolutionMinutes: 10,
	}); err != nil {
		t.Fatalf("create tighter policy: %v", err)
	}
	got, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SLAPolicyID == nil || *got.SLAPolicyID != pol.ID {
		t.Errorf("ticket now points at %v, want the pinned %s", got.SLAPolicyID, pol.ID)
	}
	if got.FirstResponseSLA == nil || got.FirstResponseSLA.Target != 240*time.Minute {
		t.Errorf("target = %v, want 4h from the pinned policy", got.FirstResponseSLA)
	}
}

// TestIntegration_Tickets_CategorySpecificPolicyBeatsTheDefault proves the
// resolution order: the org-wide row (NULL category_id) is a fallback, not a
// competitor.
func TestIntegration_Tickets_CategorySpecificPolicyBeatsTheDefault(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)

	if _, err := env.ticketsSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, tickets.CreateSLAPolicyRequest{
		Priority: "normal", FirstResponseMinutes: 999, ResolutionMinutes: 1999,
	}); err != nil {
		t.Fatalf("create default policy: %v", err)
	}
	specific, err := env.ticketsSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, tickets.CreateSLAPolicyRequest{
		CategoryID: &fx.categoryID, Priority: "normal",
		FirstResponseMinutes: 30, ResolutionMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create specific policy: %v", err)
	}

	tk := raiseTicket(t, env, fx, "Policy precedence probe")
	if tk.SLAPolicyID == nil || *tk.SLAPolicyID != specific.ID {
		t.Errorf("pinned %v, want the category-specific %s", tk.SLAPolicyID, specific.ID)
	}

	// A ticket with no category can only match the org-wide default.
	uncategorised, err := env.ticketsSvc.Create(ctx, fx.orgID, fx.memberID, tickets.CreateTicketRequest{
		RequesterID: fx.employeeID, Subject: "No category",
	})
	if err != nil {
		t.Fatalf("create uncategorised: %v", err)
	}
	if uncategorised.SLAPolicyID == nil || *uncategorised.SLAPolicyID == specific.ID {
		t.Errorf("an uncategorised ticket pinned %v — it must fall back to the org-wide default",
			uncategorised.SLAPolicyID)
	}
}

// ============================================================
// Conversion
// ============================================================

// TestIntegration_Tickets_ConversionIsOneWay proves a second conversion is
// refused rather than silently overwriting: two complaints believing they own
// the same ticket, with only the later recorded, is worse than an error.
func TestIntegration_Tickets_ConversionIsOneWay(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	tk := raiseTicket(t, env, fx, "Conversion probe")

	target := "11111111-1111-1111-1111-111111111111"
	if err := env.ticketsSvc.MarkConverted(ctx, fx.orgID, fx.ownerID, tk.ID, "complaint", target); err != nil {
		t.Fatalf("mark converted: %v", err)
	}
	got, err := env.ticketsSvc.Get(ctx, fx.orgID, fx.ownerID, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != tickets.StatusConverted || got.ConvertedToID == nil || *got.ConvertedToID != target {
		t.Errorf("status = %s, converted_to_id = %v; want converted pointing at %s",
			got.Status, got.ConvertedToID, target)
	}

	second := "22222222-2222-2222-2222-222222222222"
	if err := env.ticketsSvc.MarkConverted(ctx, fx.orgID, fx.ownerID, tk.ID, "complaint", second); err == nil {
		t.Error("a ticket was converted twice")
	}
	if err := env.ticketsSvc.MarkConverted(ctx, fx.orgID, fx.ownerID, tk.ID, "lead", second); err == nil {
		t.Error("an unsupported conversion target was accepted")
	}
}

// ============================================================
// Tenant isolation
// ============================================================

// TestIntegration_Tickets_TenantIsolation proves the org filter reaches the
// child tables too, which have no org_id of their own and are only guarded by
// the JOIN through platform_tickets.
func TestIntegration_Tickets_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	a := seedTicketFixture(t, env)
	b := seedTicketFixture(t, env)

	tk := raiseTicket(t, env, a, "Org A ticket")

	if _, err := env.ticketsSvc.Get(ctx, b.orgID, b.ownerID, tk.ID); err == nil {
		t.Error("org B read org A's ticket")
	}
	repo := tickets.NewRepository(env.db)
	if err := repo.CreateComment(ctx, b.orgID, &tickets.Comment{
		TicketID: tk.ID, AuthorUserID: b.ownerID, Body: "cross-tenant",
	}); err == nil {
		t.Error("org B attached a comment to org A's ticket")
	}
	if err := repo.CreateSLAEvent(ctx, b.orgID, tk.ID, "pause", nil, b.ownerID); err == nil {
		t.Error("org B wrote an SLA event on org A's ticket")
	}
}
