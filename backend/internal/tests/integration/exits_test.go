// backend/internal/tests/integration/exits_test.go
// hrm/exits against real Postgres. The claims needing a live schema: that
// clearance completion has no backing column, that the exit umbrella
// validates an FK-free source_id the database cannot, and that
// hrm_terminations.is_rehire_eligible — unread since migration 00034 —
// finally reaches a recruiter.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/exits"
	hrmrecruitment "github.com/mridha/businesssaas/internal/hrm/recruitment"
	hrmresignations "github.com/mridha/businesssaas/internal/hrm/resignations"
	hrmterminations "github.com/mridha/businesssaas/internal/hrm/terminations"
)

type exitFixture struct {
	orgID      string
	statusID   string
	ownerID    string
	employeeID string
}

// hrCaller is the ordinary HR caller: full org visibility and manage rights.
func hrCaller(userID string) exits.Caller {
	return exits.Caller{UserID: userID, Scope: authz.ScopeAll, CanManage: true}
}

func seedExitFixture(t *testing.T, env *testEnv) *exitFixture {
	t.Helper()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Departing Employee", nil)
	return &exitFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, employeeID: empID}
}

// seedResignationFor submits a resignation and returns its id.
func seedResignationFor(t *testing.T, env *testEnv, fx *exitFixture, lastWorkingDate string, waived bool) string {
	t.Helper()
	req := hrmresignations.SubmitResignationRequest{
		ResignationDate: "2026-03-01",
		ReasonCategory:  hrmresignations.ReasonCategory("personal"),
		IsNoticeWaived:  waived,
	}
	if lastWorkingDate != "" {
		req.LastWorkingDate = &lastWorkingDate
	}
	res, err := env.hrmResignationSvc.Submit(context.Background(), fx.orgID, fx.employeeID, fx.ownerID, req)
	if err != nil {
		t.Fatalf("submit resignation: %v", err)
	}
	return res.ID
}

// seedTerminationFor creates a termination, optionally flagged not-rehireable.
func seedTerminationFor(t *testing.T, env *testEnv, fx *exitFixture, employeeID string, rehireEligible bool) string {
	t.Helper()
	term, err := env.hrmTerminationSvc.Create(context.Background(), fx.orgID, employeeID, fx.ownerID,
		hrmterminations.CreateTerminationRequest{
			TerminationType:  hrmterminations.TerminationType("involuntary"),
			TerminationDate:  "2026-03-31",
			LastWorkingDate:  "2026-03-31",
			IsRehireEligible: &rehireEligible,
		})
	if err != nil {
		t.Fatalf("create termination: %v", err)
	}
	return term.ID
}

// ============================================================
// The structural claim
// ============================================================

// TestIntegration_Exits_NoStoredClearanceCompletion is migration 00114's
// central promise. hrm_terminations.exit_clearance_completed (00034) is
// exactly the denormalized boolean this avoids — it drifts the first time a
// clearance item is resolved without updating it.
//
// Introspecting information_schema is the only way to prove a column is
// ABSENT — the 6A/8A/8C precedent.
func TestIntegration_Exits_NoStoredClearanceCompletion(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	forbidden := []string{
		"clearance_completed", "is_cleared", "clearance_complete",
		"blocking_total", "outstanding_dues", "total_dues",
		"clearance_item_count", "resolved_item_count",
	}
	for _, col := range forbidden {
		var n int
		if err := env.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.columns
			  WHERE table_name = 'hrm_exits' AND column_name = $1`, col).Scan(&n); err != nil {
			t.Fatalf("introspect %s: %v", col, err)
		}
		if n != 0 {
			t.Errorf("hrm_exits.%s exists — clearance completion must be derived from the items", col)
		}
	}
}

// TestIntegration_Exits_ClearanceSummaryIsDerived proves the summary tracks
// the items on every read rather than being a snapshot taken once.
func TestIntegration_Exits_ClearanceSummaryIsDerived(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)

	sourceID := seedResignationFor(t, env, fx, "2026-03-31", false)
	e, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("create exit: %v", err)
	}

	amount := "40000.00"
	laptop, err := env.hrmExitsSvc.AddClearanceItem(ctx, fx.orgID, caller, e.ID, exits.CreateClearanceItemRequest{
		Department: "IT", Description: "Unreturned laptop", BlockingAmount: &amount,
	})
	if err != nil {
		t.Fatalf("add clearance item: %v", err)
	}
	if _, err := env.hrmExitsSvc.AddClearanceItem(ctx, fx.orgID, caller, e.ID, exits.CreateClearanceItemRequest{
		Department: "Facilities", Description: "Return access badge",
	}); err != nil {
		t.Fatalf("add second clearance item: %v", err)
	}

	got, err := env.hrmExitsSvc.Get(ctx, fx.orgID, caller, e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Clearance.TotalItems != 2 {
		t.Errorf("total items = %d, want 2", got.Clearance.TotalItems)
	}
	// Only the item with money BLOCKS; "return your badge" is incomplete but
	// owes nothing.
	if got.Clearance.BlockingItems != 1 {
		t.Errorf("blocking items = %d, want 1 — only a non-zero amount blocks", got.Clearance.BlockingItems)
	}
	if got.Clearance.OutstandingDues.String() != "40000" {
		t.Errorf("outstanding = %s, want 40000", got.Clearance.OutstandingDues)
	}
	if got.Clearance.IsComplete {
		t.Error("clearance reported complete with two unresolved items")
	}

	// Resolving changes the derived summary with no write to hrm_exits.
	if _, err := env.hrmExitsSvc.ResolveClearanceItem(ctx, fx.orgID, caller, e.ID, laptop.ID,
		exits.ResolveClearanceItemRequest{}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got, err = env.hrmExitsSvc.Get(ctx, fx.orgID, caller, e.ID)
	if err != nil {
		t.Fatalf("get after resolve: %v", err)
	}
	if got.Clearance.BlockingItems != 0 || !got.Clearance.OutstandingDues.IsZero() {
		t.Errorf("after resolving the only debt: blocking=%d outstanding=%s, want 0 and 0",
			got.Clearance.BlockingItems, got.Clearance.OutstandingDues)
	}
}

// TestIntegration_Exits_ResolvingDoesNotEraseTheAmount — a forgiven debt must
// still show what was forgiven. Rewriting blocking_amount to zero destroys
// the only record there was ever anything to forgive.
func TestIntegration_Exits_ResolvingDoesNotEraseTheAmount(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)
	sourceID := seedResignationFor(t, env, fx, "2026-03-31", false)
	e, _ := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: sourceID,
	})

	amount := "12500.50"
	item, err := env.hrmExitsSvc.AddClearanceItem(ctx, fx.orgID, caller, e.ID, exits.CreateClearanceItemRequest{
		Department: "Finance", Description: "Outstanding advance", BlockingAmount: &amount,
	})
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	resolved, err := env.hrmExitsSvc.ResolveClearanceItem(ctx, fx.orgID, caller, e.ID, item.ID,
		exits.ResolveClearanceItemRequest{WaiveAmount: true, ResolutionNote: strp("waived by CFO")})
	if err != nil {
		t.Fatalf("waive: %v", err)
	}
	if resolved.BlockingAmount.String() != "12500.5" {
		t.Errorf("blocking_amount = %s after a waiver, want it preserved at 12500.5",
			resolved.BlockingAmount)
	}
	if !resolved.IsResolved {
		t.Error("item not marked resolved")
	}

	// Resolving twice is refused rather than silently re-stamping who
	// resolved it and when.
	_, err = env.hrmExitsSvc.ResolveClearanceItem(ctx, fx.orgID, caller, e.ID, item.ID,
		exits.ResolveClearanceItemRequest{})
	if !errors.Is(err, exits.ErrAlreadyResolved) {
		t.Errorf("second resolve returned %v, want ErrAlreadyResolved", err)
	}
}

// ============================================================
// The FK-free source, validated in Go
// ============================================================

// TestIntegration_Exits_SourceMustBelongToTheEmployee is what replaces the
// foreign key migration 00114 deliberately does not have. Without this check
// an exit could point at another employee's resignation and settle the wrong
// person.
func TestIntegration_Exits_SourceMustBelongToTheEmployee(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)
	other := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Someone Else", nil)

	sourceID := seedResignationFor(t, env, fx, "2026-03-31", false)

	_, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: other, SourceType: "resignation", SourceID: sourceID,
	})
	if !errors.Is(err, exits.ErrSourceMismatch) {
		t.Errorf("creating an exit against another employee's resignation returned %v, want ErrSourceMismatch", err)
	}

	// A source id that does not exist at all is equally refused.
	_, err = env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation",
		SourceID: "11111111-1111-1111-1111-111111111111",
	})
	if !errors.Is(err, exits.ErrSourceMismatch) {
		t.Errorf("creating an exit against a non-existent resignation returned %v, want ErrSourceMismatch", err)
	}
}

// TestIntegration_Exits_OneLiveExitPerEmployee — two exits would each compute
// their own settlement for the same departure.
func TestIntegration_Exits_OneLiveExitPerEmployee(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)
	sourceID := seedResignationFor(t, env, fx, "2026-03-31", false)

	first, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("first exit: %v", err)
	}
	_, err = env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: sourceID,
	})
	if !errors.Is(err, exits.ErrExitAlreadyOpen) {
		t.Errorf("second exit returned %v, want ErrExitAlreadyOpen", err)
	}

	// Cancelling frees the employee for a new exit — a rehired employee who
	// later leaves again must not be permanently blocked.
	if _, err := env.hrmExitsSvc.Cancel(ctx, fx.orgID, caller, first.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: sourceID,
	}); err != nil {
		t.Errorf("a new exit after cancellation was refused: %v", err)
	}
}

// ============================================================
// Notice shortfall
// ============================================================

// TestIntegration_Exits_WaivedNoticeIsNeverAShortfall is the case that costs
// a departing person real money if it is wrong: the employer AGREED to forgo
// the notice, so there is nothing to recover.
func TestIntegration_Exits_WaivedNoticeIsNeverAShortfall(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)

	sourceID := seedResignationFor(t, env, fx, "2026-03-05", true)
	e, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("create exit: %v", err)
	}
	if e.NoticeShortfallDays != 0 {
		t.Errorf("shortfall = %d days on a WAIVED notice period, want 0 — this bills somebody for time the company forgave",
			e.NoticeShortfallDays)
	}
	if e.ExpectedLastWorkingDate != nil {
		t.Error("a waived notice period recorded an expected last working date")
	}
}

// TestIntegration_Exits_TerminationHasNoNoticeShortfall — the employer set
// the leaving date, so there is no entitlement to fall short of.
func TestIntegration_Exits_TerminationHasNoNoticeShortfall(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)

	termID := seedTerminationFor(t, env, fx, fx.employeeID, true)
	e, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "termination", SourceID: termID,
	})
	if err != nil {
		t.Fatalf("create exit from termination: %v", err)
	}
	if e.NoticeShortfallDays != 0 {
		t.Errorf("shortfall = %d on a termination, want 0", e.NoticeShortfallDays)
	}
}

// TestIntegration_Exits_ShortfallRecomputedOnDateCorrection — the date and
// the shortfall are one fact, so correcting the date must move the shortfall
// with it rather than leaving the two to disagree.
func TestIntegration_Exits_ShortfallRecomputedOnDateCorrection(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)

	sourceID := seedResignationFor(t, env, fx, "2026-03-31", false)
	e, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("create exit: %v", err)
	}
	if e.NoticeShortfallDays != 0 {
		t.Fatalf("shortfall = %d before any correction, want 0", e.NoticeShortfallDays)
	}

	// They actually left ten days early.
	updated, err := env.hrmExitsSvc.Update(ctx, fx.orgID, caller, e.ID, exits.UpdateExitRequest{
		LastWorkingDate: strp("2026-03-21"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.NoticeShortfallDays != 10 {
		t.Errorf("shortfall = %d after moving the date 10 days earlier, want 10", updated.NoticeShortfallDays)
	}
}

// ============================================================
// Rehire eligibility — giving a 00034 column its first reader
// ============================================================

// TestIntegration_Exits_TerminationSeedsRehireDecision proves
// hrm_terminations.is_rehire_eligible, unread since migration 00034, now
// reaches somewhere a recruiter can see it.
func TestIntegration_Exits_TerminationSeedsRehireDecision(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)

	termID := seedTerminationFor(t, env, fx, fx.employeeID, false) // NOT rehire eligible
	if _, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "termination", SourceID: termID,
	}); err != nil {
		t.Fatalf("create exit: %v", err)
	}

	re, err := env.hrmExitsSvc.GetRehire(ctx, fx.orgID, caller, fx.employeeID)
	if err != nil {
		t.Fatalf("get rehire: %v", err)
	}
	if re == nil {
		t.Fatal("no rehire decision was seeded from the termination")
	}
	if re.Status != exits.RehireNotEligible {
		t.Errorf("status = %s, want not_eligible seeded from is_rehire_eligible=false", re.Status)
	}

	// And an explicit decision overrides the seed rather than adding a second
	// row a recruiter would have to adjudicate between.
	if _, err := env.hrmExitsSvc.DecideRehire(ctx, fx.orgID, caller, fx.employeeID,
		exits.DecideRehireRequest{Status: "conditional", Reason: strp("reviewed on appeal")}); err != nil {
		t.Fatalf("decide rehire: %v", err)
	}
	re, err = env.hrmExitsSvc.GetRehire(ctx, fx.orgID, caller, fx.employeeID)
	if err != nil {
		t.Fatalf("get rehire after override: %v", err)
	}
	if re.Status != exits.RehireConditional {
		t.Errorf("status = %s after an explicit decision, want conditional", re.Status)
	}
	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_rehire_eligibility WHERE employee_id = $1`, fx.employeeID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("%d rehire rows for one employee, want exactly 1 standing decision", n)
	}
}

// TestIntegration_Exits_RehireFlagWarnsRecruitmentButDoesNotBlock is the
// consumer-owned-interface seam working end to end. A hard block would make a
// wrongly-flagged ex-employee unhireable with no override, so the candidate
// IS created and the warning rides along.
func TestIntegration_Exits_RehireFlagWarnsRecruitmentButDoesNotBlock(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)

	// A former employee with a work email, marked not-rehireable.
	email := uniqueEmail("former-staff")
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET work_email = $2 WHERE id = $1`, fx.employeeID, email); err != nil {
		t.Fatalf("set work email: %v", err)
	}
	termID := seedTerminationFor(t, env, fx, fx.employeeID, false)
	if _, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "termination", SourceID: termID,
	}); err != nil {
		t.Fatalf("create exit: %v", err)
	}

	// They apply again.
	cand, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, fx.orgID, &fx.ownerID,
		hrmrecruitment.CreateCandidateRequest{FirstName: "Returning", Email: &email})
	if err != nil {
		t.Fatalf("candidate creation was BLOCKED by a rehire flag: %v", err)
	}
	if cand.RehireFlag == nil {
		// attachRehireFlag logs and skips on a lookup error by design (a
		// failure must not block a recruiter), so a broken query is
		// indistinguishable from "no match" here. Call the checker directly
		// to say WHICH it is — the 8D lesson about tests that swallow the
		// reason they failed.
		re, lookupErr := env.hrmExitsSvc.CheckRehireEligibility(ctx, fx.orgID, email)
		t.Fatalf("no rehire warning surfaced on a candidate matching a not-rehireable former employee; "+
			"direct lookup returned re=%+v err=%v", re, lookupErr)
	}
	if cand.RehireFlag.Status != string(exits.RehireNotEligible) {
		t.Errorf("flag status = %s, want not_eligible", cand.RehireFlag.Status)
	}

	// A stranger gets no flag — noise a recruiter learns to ignore is worse
	// than no signal at all.
	strangerEmail := uniqueEmail("stranger")
	plain, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, fx.orgID, &fx.ownerID,
		hrmrecruitment.CreateCandidateRequest{FirstName: "Stranger", Email: &strangerEmail})
	if err != nil {
		t.Fatalf("create stranger candidate: %v", err)
	}
	if plain.RehireFlag != nil {
		t.Errorf("a candidate with no employment history was flagged: %+v", plain.RehireFlag)
	}
}

// TestIntegration_Exits_EligibleFormerEmployeeIsNotFlagged — only a negative
// decision is worth surfacing.
func TestIntegration_Exits_EligibleFormerEmployeeIsNotFlagged(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	caller := hrCaller(fx.ownerID)

	email := uniqueEmail("good-leaver")
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET work_email = $2 WHERE id = $1`, fx.employeeID, email); err != nil {
		t.Fatalf("set work email: %v", err)
	}
	termID := seedTerminationFor(t, env, fx, fx.employeeID, true) // rehire eligible
	if _, err := env.hrmExitsSvc.Create(ctx, fx.orgID, caller, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "termination", SourceID: termID,
	}); err != nil {
		t.Fatalf("create exit: %v", err)
	}

	cand, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, fx.orgID, &fx.ownerID,
		hrmrecruitment.CreateCandidateRequest{FirstName: "Good", Email: &email})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	if cand.RehireFlag != nil {
		t.Errorf("a rehire-ELIGIBLE former employee was flagged: %+v", cand.RehireFlag)
	}
}

// ============================================================
// Scope tiers and tenant isolation
// ============================================================

// TestIntegration_Exits_ScopeTiersNarrowVisibility — a departing employee must
// see their own exit and nobody else's, and the list and the single-row read
// must agree.
func TestIntegration_Exits_ScopeTiersNarrowVisibility(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExitFixture(t, env)
	hr := hrCaller(fx.ownerID)

	// A second employee, linked to a real user, with their own exit.
	memberID := seedOrgMember(t, env, fx.orgID, "member", "exit-self")
	selfEmp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, memberID, "Self Server", nil)

	otherSource := seedResignationFor(t, env, fx, "2026-03-31", false)
	othersExit, err := env.hrmExitsSvc.Create(ctx, fx.orgID, hr, exits.CreateExitRequest{
		EmployeeID: fx.employeeID, SourceType: "resignation", SourceID: otherSource,
	})
	if err != nil {
		t.Fatalf("create other exit: %v", err)
	}

	selfTerm := seedTerminationFor(t, env, fx, selfEmp, true)
	myExit, err := env.hrmExitsSvc.Create(ctx, fx.orgID, hr, exits.CreateExitRequest{
		EmployeeID: selfEmp, SourceType: "termination", SourceID: selfTerm,
	})
	if err != nil {
		t.Fatalf("create own exit: %v", err)
	}

	self := exits.Caller{UserID: memberID, Scope: authz.ScopeOwn, CanManage: false}
	res, err := env.hrmExitsSvc.List(ctx, fx.orgID, self, exits.ListFilter{})
	if err != nil {
		t.Fatalf("list as self: %v", err)
	}
	for _, e := range res.Exits {
		if e.ID == othersExit.ID {
			t.Error("a view_own caller saw someone else's exit")
		}
	}
	if res.Total != len(res.Exits) {
		t.Errorf("Total %d disagrees with %d rows — count and list predicates have drifted",
			res.Total, len(res.Exits))
	}
	if _, err := env.hrmExitsSvc.Get(ctx, fx.orgID, self, othersExit.ID); err == nil {
		t.Error("a view_own caller fetched someone else's exit by id")
	}
	if _, err := env.hrmExitsSvc.Get(ctx, fx.orgID, self, myExit.ID); err != nil {
		t.Errorf("a departing employee could not read their own exit: %v", err)
	}

	// And view_own does not confer write rights.
	if _, err := env.hrmExitsSvc.Cancel(ctx, fx.orgID, self, myExit.ID); !errors.Is(err, exits.ErrAccessDenied) {
		t.Errorf("a view_own caller cancelling their own exit returned %v, want ErrAccessDenied", err)
	}
}

// TestIntegration_Exits_TenantIsolation — clearance items have no org_id of
// their own and are guarded only by the JOIN through hrm_exits.
func TestIntegration_Exits_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	a := seedExitFixture(t, env)
	b := seedExitFixture(t, env)

	sourceID := seedResignationFor(t, env, a, "2026-03-31", false)
	e, err := env.hrmExitsSvc.Create(ctx, a.orgID, hrCaller(a.ownerID), exits.CreateExitRequest{
		EmployeeID: a.employeeID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("create exit: %v", err)
	}

	if _, err := env.hrmExitsSvc.Get(ctx, b.orgID, hrCaller(b.ownerID), e.ID); err == nil {
		t.Error("org B read org A's exit")
	}
	if _, err := env.hrmExitsSvc.AddClearanceItem(ctx, b.orgID, hrCaller(b.ownerID), e.ID,
		exits.CreateClearanceItemRequest{Department: "IT", Description: "cross-tenant"}); err == nil {
		t.Error("org B attached a clearance item to org A's exit")
	}
}
