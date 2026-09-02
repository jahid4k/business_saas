// backend/internal/tests/integration/exit_offboarding_test.go
// Phase 9C: exit interviews, document issuance and access revocation.
//
// Three claims need a live run: that a manager cannot read what a departing
// employee said, that the relieving letter waits for clearance AND settlement
// while the experience letter does not, and that the revocation sweep is
// idempotent — it runs nightly and must not re-revoke.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/exits"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// hrCallerNoInterviews is an HR-ish caller WITHOUT interview_view — the
// manager case, which is the whole point of that permission existing.
func hrCallerNoInterviews(userID string) exits.Caller {
	return exits.Caller{UserID: userID, Scope: authz.ScopeAll, CanManage: true, CanViewInterviews: false}
}

func hrCallerWithInterviews(userID string) exits.Caller {
	return exits.Caller{UserID: userID, Scope: authz.ScopeAll, CanManage: true, CanViewInterviews: true}
}

// seedExitWithEmployee makes an exit whose employee has a real platform
// account, so access revocation has something to revoke.
func seedExitWithEmployee(t *testing.T, env *testEnv, lastWorkingDate string) (*exitFixture, string, *exits.Exit) {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	memberID := seedOrgMember(t, env, orgID, "member", "departing")
	empID := seedEmployee(t, env, orgID, statusID, ownerID, memberID, "Departing Person", nil)
	fx := &exitFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, employeeID: empID}

	var sourceID string
	if err := env.db.QueryRow(ctx,
		// resignation_date must not be AFTER last_working_date
		// (chk_hrm_res_dates), so it is derived from it rather than pinned to
		// today — these fixtures deliberately use past leaving dates.
		`INSERT INTO hrm_resignations (org_id, employee_id, resignation_date, notice_period_days,
		    last_working_date, reason_category, created_by)
		 VALUES ($1,$2,$3::date - INTERVAL '30 days',30,$3::date,'personal',$4) RETURNING id`,
		orgID, empID, lastWorkingDate, ownerID).Scan(&sourceID); err != nil {
		t.Fatalf("seed resignation: %v", err)
	}
	e, err := env.hrmExitsSvc.Create(ctx, orgID, hrCaller(ownerID), exits.CreateExitRequest{
		EmployeeID: empID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("create exit: %v", err)
	}
	return fx, memberID, e
}

// ============================================================
// Exit interview confidentiality
// ============================================================

// TestIntegration_Offboarding_ManagerCannotReadInterviewResponses is the claim
// the whole feature rests on. An exit interview is worth conducting only if
// the departing employee believes their answers cannot reach the manager they
// are leaving — and a manager holds view_team over exits, so the protection
// has to come from a separate permission they simply lack.
func TestIntegration_Offboarding_ManagerCannotReadInterviewResponses(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx, _, e := seedExitWithEmployee(t, env, "2026-01-31")

	// A default exit-interview template, so the interview can actually be
	// SENT and therefore actually have responses. Without this the read would
	// fail with "not found" and the test would pass for the wrong reason —
	// proving nothing about confidentiality.
	tmpl, err := env.formsSvc.CreateTemplate(ctx, fx.orgID, fx.ownerID, forms.CreateTemplateRequest{
		Name: "Exit Interview " + uniqueSlug("form"), FormType: "exit_interview", IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create exit interview template: %v", err)
	}
	sec, err := env.formsSvc.CreateSection(ctx, fx.orgID, tmpl.ID, forms.CreateSectionRequest{
		Title: "Leaving",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	// A template with no questions cannot be instantiated, so the question is
	// what makes there be something to keep confidential at all.
	if _, err := env.formsSvc.CreateQuestion(ctx, fx.orgID, sec.ID, forms.CreateQuestionRequest{
		QuestionText: "Why are you leaving?", QuestionType: "text",
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	if _, err := env.hrmExitsSvc.ScheduleInterview(ctx, fx.orgID, hrCaller(fx.ownerID), e.ID,
		exits.ScheduleInterviewRequest{}); err != nil {
		t.Fatalf("schedule interview: %v", err)
	}
	sent, err := env.hrmExitsSvc.SendInterview(ctx, fx.orgID, hrCaller(fx.ownerID), e.ID)
	if err != nil {
		t.Fatalf("send interview: %v", err)
	}
	if sent.FormInstanceID == nil {
		t.Fatal("the interview was sent with no form instance — there would be nothing to protect")
	}

	// An HR caller WITH the key can read it. Asserting this first means the
	// denial below is genuinely about the permission and not about the data
	// being absent.
	if _, err := env.hrmExitsSvc.ReadInterviewResponses(ctx, fx.orgID,
		hrCallerWithInterviews(fx.ownerID), e.ID); err != nil {
		t.Fatalf("a caller WITH interview_view could not read the responses: %v", err)
	}

	// The same exit, the same live responses — but a caller without the key.
	_, err = env.hrmExitsSvc.ReadInterviewResponses(ctx, fx.orgID, hrCallerNoInterviews(fx.ownerID), e.ID)
	if !errors.Is(err, exits.ErrAccessDenied) {
		t.Errorf("reading responses without interview_view returned %v, want ErrAccessDenied — "+
			"a manager must never be able to read what a departing employee said about them", err)
	}

	// The LIFECYCLE is still visible to them: knowing an interview happened is
	// administrative, reading it is not.
	if _, err := env.hrmExitsSvc.GetInterview(ctx, fx.orgID, hrCallerNoInterviews(fx.ownerID), e.ID); err != nil {
		t.Errorf("the interview's lifecycle should be visible without interview_view: %v", err)
	}
}

// TestIntegration_Offboarding_InterviewIsNotSentBeforeItsDate — the timing IS
// the confidentiality mechanism, not a convenience. An interview answered
// while still on the payroll gets a different answer.
func TestIntegration_Offboarding_InterviewIsNotSentBeforeItsDate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	// Leaving well in the future, so the interview is not due.
	fx, _, e := seedExitWithEmployee(t, env, "2099-01-31")

	i, err := env.hrmExitsSvc.ScheduleInterview(ctx, fx.orgID, hrCaller(fx.ownerID), e.ID,
		exits.ScheduleInterviewRequest{})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	// Default is the day AFTER the last working date.
	if !i.ScheduledFor.After(time.Now()) {
		t.Errorf("scheduled_for = %s, want a future date (the day after last working date)", i.ScheduledFor)
	}

	_, err = env.hrmExitsSvc.SendInterview(ctx, fx.orgID, hrCaller(fx.ownerID), e.ID)
	if !errors.Is(err, exits.ErrInterviewNotDue) {
		t.Errorf("sending before the scheduled date returned %v, want ErrInterviewNotDue", err)
	}

	// And the sweep must not pick it up either.
	sent, err := env.hrmExitsSvc.RunInterviewSweep(ctx, time.Now())
	if err != nil {
		t.Fatalf("interview sweep: %v", err)
	}
	var status string
	if err := env.db.QueryRow(ctx,
		`SELECT status FROM hrm_exit_interviews WHERE id = $1`, i.ID).Scan(&status); err != nil {
		t.Fatalf("read interview: %v", err)
	}
	if status != string(exits.InterviewScheduled) {
		t.Errorf("status = %q after a sweep run (sent %d), want it left scheduled", status, sent)
	}
}

// TestIntegration_Offboarding_OneInterviewPerExit — a second would split one
// person's answers across two records and double-count them in any aggregate.
func TestIntegration_Offboarding_OneInterviewPerExit(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx, _, e := seedExitWithEmployee(t, env, "2026-01-31")

	if _, err := env.hrmExitsSvc.ScheduleInterview(ctx, fx.orgID, hrCaller(fx.ownerID), e.ID,
		exits.ScheduleInterviewRequest{}); err != nil {
		t.Fatalf("first schedule: %v", err)
	}
	_, err := env.hrmExitsSvc.ScheduleInterview(ctx, fx.orgID, hrCaller(fx.ownerID), e.ID,
		exits.ScheduleInterviewRequest{})
	if !errors.Is(err, exits.ErrInterviewExists) {
		t.Errorf("second schedule returned %v, want ErrInterviewExists", err)
	}
}

// TestIntegration_Offboarding_NoResponsesColumnExists — the form engine owns
// the answers. A second copy here would disagree with the first the moment
// one was edited.
func TestIntegration_Offboarding_NoResponsesColumnExists(t *testing.T) {
	env := newTestEnv(t)
	for _, col := range []string{"responses", "answers", "response_json", "feedback", "comments"} {
		var n int
		if err := env.db.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM information_schema.columns
			  WHERE table_name = 'hrm_exit_interviews' AND column_name = $1`, col).Scan(&n); err != nil {
			t.Fatalf("introspect %s: %v", col, err)
		}
		if n != 0 {
			t.Errorf("hrm_exit_interviews.%s exists — the form engine owns the responses", col)
		}
	}
}

// ============================================================
// Document issuance
// ============================================================

// TestIntegration_Offboarding_RelievingLetterWaitsButExperienceLetterDoesNot
// is the one place clearance and the settlement — tracked independently —
// have to agree. The experience letter states employment dates, which are
// true regardless of what is owed; withholding it would punish somebody for a
// money dispute by making them unemployable while it is resolved.
func TestIntegration_Offboarding_RelievingLetterWaitsButExperienceLetterDoesNot(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx, _, e := seedExitWithEmployee(t, env, "2026-01-31")
	caller := hrCaller(fx.ownerID)

	amount := "9000.00"
	if _, err := env.hrmExitsSvc.AddClearanceItem(ctx, fx.orgID, caller, e.ID,
		exits.CreateClearanceItemRequest{
			Department: "IT", Description: "Unreturned laptop", BlockingAmount: &amount,
		}); err != nil {
		t.Fatalf("add clearance item: %v", err)
	}

	docs, err := env.hrmExitsSvc.DocumentIssuanceEligibility(ctx, fx.orgID, caller, e.ID)
	if err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	byType := map[string]*exits.DocumentEligibility{}
	for _, d := range docs {
		byType[d.DocumentType] = d
	}

	if exp := byType["experience_letter"]; exp == nil || !exp.Eligible {
		t.Error("the experience letter is blocked — employment dates are true regardless of what is owed")
	}
	rel := byType["relieving_letter"]
	if rel == nil {
		t.Fatal("no relieving_letter eligibility reported")
	}
	if rel.Eligible {
		t.Error("the relieving letter was issuable with clearance outstanding")
	}
	if rel.Reason == "" {
		t.Error("no reason given — whoever is refused needs to know what is holding it up")
	}

	// Resolving clearance is not enough on its own: the settlement must also
	// be done, and it has not been run at all.
	items, _ := env.hrmExitsSvc.ListClearanceItems(ctx, fx.orgID, caller, e.ID)
	if _, err := env.hrmExitsSvc.ResolveClearanceItem(ctx, fx.orgID, caller, e.ID, items[0].ID,
		exits.ResolveClearanceItemRequest{}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	docs, err = env.hrmExitsSvc.DocumentIssuanceEligibility(ctx, fx.orgID, caller, e.ID)
	if err != nil {
		t.Fatalf("eligibility after clearance: %v", err)
	}
	for _, d := range docs {
		if d.DocumentType == "relieving_letter" && d.Eligible {
			t.Error("the relieving letter became issuable with clearance done but NO settlement run — " +
				"it is the document saying the org considers the person fully departed and owed nothing")
		}
	}
}

// TestIntegration_Offboarding_RelievingLetterDocTypeIsAcceptedByBothTables is
// the two-CHECK widening trap's regression guard. Migration 00118 widened
// hrm_document_templates AND hrm_employee_documents, whose vocabularies
// differ; widening only one leaves a template creatable but never issuable.
func TestIntegration_Offboarding_RelievingLetterDocTypeIsAcceptedByBothTables(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	var templateID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_document_templates (org_id, name, document_type, created_by)
		 VALUES ($1,'Relieving Letter','relieving_letter',$2) RETURNING id`,
		orgID, ownerID).Scan(&templateID); err != nil {
		t.Fatalf("hrm_document_templates rejected 'relieving_letter': %v", err)
	}

	statusID := ""
	if err := env.db.QueryRow(ctx,
		`SELECT id FROM hrm_employee_statuses WHERE org_id=$1 LIMIT 1`, orgID).Scan(&statusID); err != nil {
		t.Fatalf("read status: %v", err)
	}
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Doc Subject", nil)
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_employee_documents (org_id, employee_id, title, document_type,
		     file_url, file_name, mime_type, created_by)
		 VALUES ($1,$2,'Relieving Letter','relieving_letter','https://example.test/r.pdf',
		         'relieving.pdf','application/pdf',$3)`,
		orgID, empID, ownerID); err != nil {
		t.Fatalf("hrm_employee_documents rejected 'relieving_letter' — only ONE of the two "+
			"document_type CHECKs was widened: %v", err)
	}
}

// ============================================================
// Access revocation
// ============================================================

// TestIntegration_Offboarding_RevocationSweepSuspendsAndIsIdempotent. The
// sweep runs nightly, so re-revoking or erroring on an already-handled exit
// would turn into a permanent alarm.
func TestIntegration_Offboarding_RevocationSweepSuspendsAndIsIdempotent(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	// Left in the past, so the sweep should pick them up.
	fx, memberID, e := seedExitWithEmployee(t, env, "2020-06-30")

	var statusBefore string
	if err := env.db.QueryRow(ctx,
		`SELECT status FROM organization_members WHERE org_id=$1 AND user_id=$2`,
		fx.orgID, memberID).Scan(&statusBefore); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if statusBefore != "active" {
		t.Fatalf("membership starts %q, want active", statusBefore)
	}

	first, err := env.hrmExitsSvc.RunAccessRevocationSweep(ctx, time.Now())
	if err != nil {
		t.Fatalf("revocation sweep: %v", err)
	}
	if first < 1 {
		t.Fatalf("sweep revoked %d, want at least 1", first)
	}

	var statusAfter string
	if err := env.db.QueryRow(ctx,
		`SELECT status FROM organization_members WHERE org_id=$1 AND user_id=$2`,
		fx.orgID, memberID).Scan(&statusAfter); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if statusAfter != "suspended" {
		t.Errorf("membership = %q after revocation, want suspended", statusAfter)
	}
	// Suspended, NOT deleted — an admin has to be able to undo this.
	var memberRows int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM organization_members WHERE org_id=$1 AND user_id=$2`,
		fx.orgID, memberID).Scan(&memberRows); err != nil {
		t.Fatalf("count membership: %v", err)
	}
	if memberRows != 1 {
		t.Error("the membership row was deleted — revocation must be reversible")
	}
	// And the user account itself survives.
	var userRows int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, memberID).Scan(&userRows); err != nil {
		t.Fatalf("count user: %v", err)
	}
	if userRows != 1 {
		t.Error("the user account was deleted — revocation must never do that")
	}

	var revokedAt *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT access_revoked_at FROM hrm_exits WHERE id=$1`, e.ID).Scan(&revokedAt); err != nil {
		t.Fatalf("read exit: %v", err)
	}
	if revokedAt == nil {
		t.Fatal("access_revoked_at was never stamped — the sweep would retry this exit forever")
	}
	stampedAt := *revokedAt

	// IDEMPOTENCE: a second run must not touch it again.
	second, err := env.hrmExitsSvc.RunAccessRevocationSweep(ctx, time.Now())
	if err != nil {
		t.Fatalf("second sweep errored: %v", err)
	}
	var revokedAgain *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT access_revoked_at FROM hrm_exits WHERE id=$1`, e.ID).Scan(&revokedAgain); err != nil {
		t.Fatalf("read exit: %v", err)
	}
	if revokedAgain == nil || !revokedAgain.Equal(stampedAt) {
		t.Errorf("access_revoked_at moved from %v to %v on a second sweep (revoked %d) — "+
			"the sweep is not idempotent", stampedAt, revokedAgain, second)
	}
}

// TestIntegration_Offboarding_RevocationSkipsFutureLeavers — somebody serving
// notice must keep working until their last day.
func TestIntegration_Offboarding_RevocationSkipsFutureLeavers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx, memberID, e := seedExitWithEmployee(t, env, "2099-06-30")

	if _, err := env.hrmExitsSvc.RunAccessRevocationSweep(ctx, time.Now()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	var status string
	if err := env.db.QueryRow(ctx,
		`SELECT status FROM organization_members WHERE org_id=$1 AND user_id=$2`,
		fx.orgID, memberID).Scan(&status); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if status != "active" {
		t.Errorf("membership = %q for someone still serving notice, want active", status)
	}
	var revokedAt *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT access_revoked_at FROM hrm_exits WHERE id=$1`, e.ID).Scan(&revokedAt); err != nil {
		t.Fatalf("read exit: %v", err)
	}
	if revokedAt != nil {
		t.Error("a future leaver's access was revoked early")
	}
}

// TestIntegration_Offboarding_RevokeAccessNowIsImmediateAndRepeatable — a
// dismissal for cause does not wait for tonight's cron.
func TestIntegration_Offboarding_RevokeAccessNowIsImmediateAndRepeatable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	// Future last working date: the sweep would NOT have picked this up.
	fx, memberID, e := seedExitWithEmployee(t, env, "2099-06-30")
	caller := hrCaller(fx.ownerID)

	if _, err := env.hrmExitsSvc.RevokeAccessNow(ctx, fx.orgID, caller, e.ID); err != nil {
		t.Fatalf("revoke now: %v", err)
	}
	var status string
	if err := env.db.QueryRow(ctx,
		`SELECT status FROM organization_members WHERE org_id=$1 AND user_id=$2`,
		fx.orgID, memberID).Scan(&status); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if status != "suspended" {
		t.Errorf("membership = %q after an immediate revocation, want suspended", status)
	}

	// Repeating it is a no-op rather than an error — the manual path is as
	// idempotent as the sweep.
	if _, err := env.hrmExitsSvc.RevokeAccessNow(ctx, fx.orgID, caller, e.ID); err != nil {
		t.Errorf("repeating an immediate revocation errored: %v", err)
	}
}

// TestIntegration_Offboarding_RevocationStampsEvenWithNoPlatformAccount — an
// employee with no user account has nothing to revoke, but must still be
// stamped or the sweep retries them every night forever.
func TestIntegration_Offboarding_RevocationStampsEvenWithNoPlatformAccount(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	// No userID: an employee with no platform login.
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "No Account", nil)

	var sourceID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_resignations (org_id, employee_id, resignation_date, notice_period_days,
		    last_working_date, reason_category, created_by)
		 VALUES ($1,$2,DATE '2020-05-31',30,DATE '2020-06-30','personal',$3) RETURNING id`,
		orgID, empID, ownerID).Scan(&sourceID); err != nil {
		t.Fatalf("seed resignation: %v", err)
	}
	e, err := env.hrmExitsSvc.Create(ctx, orgID, hrCaller(ownerID), exits.CreateExitRequest{
		EmployeeID: empID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("create exit: %v", err)
	}

	if _, err := env.hrmExitsSvc.RunAccessRevocationSweep(ctx, time.Now()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	var revokedAt *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT access_revoked_at FROM hrm_exits WHERE id=$1`, e.ID).Scan(&revokedAt); err != nil {
		t.Fatalf("read exit: %v", err)
	}
	if revokedAt == nil {
		t.Error("an employee with no platform account was left unstamped — " +
			"the sweep would retry them every night forever")
	}
}
