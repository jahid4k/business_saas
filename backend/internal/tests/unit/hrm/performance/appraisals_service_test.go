// backend/internal/tests/unit/hrm/performance/appraisals_service_test.go
// Phase 5B appraisal rules: the transition map, the calibration audit trail,
// and publish-immutability.
package performance_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/performance"
)

// newAppraisalSvc returns a service plus the stubs, so a test can drive the
// form engine's submitted-state — which is what the phase preconditions read.
func newAppraisalSvc(allow bool) (performance.Service, *stubRepo, *stubFormEngine) {
	repo := newStubRepo()
	repo.employeeUsers[ownerUserID] = ownerEmpID
	repo.employees[ownerEmpID] = true
	repo.employees[otherEmpID] = true
	repo.employeeSubjects[ownerEmpID] = &performance.EmployeeSubject{
		EmployeeID: ownerEmpID, DisplayName: "Own Employee",
		UserID: strPtr(ownerUserID), ManagerEmployeeID: strPtr(otherEmpID),
	}
	repo.employeeSubjects[otherEmpID] = &performance.EmployeeSubject{
		EmployeeID: otherEmpID, DisplayName: "Other Employee",
	}
	engine := newStubFormEngine()
	return performance.NewService(repo, &stubAuthorizer{allow: allow}, engine), repo, engine
}

// seedAppraisalCycle builds a scale with two levels, an ACTIVE appraisal
// cycle configured with both forms, and returns the cycle plus its levels.
func seedAppraisalCycle(t *testing.T, svc performance.Service) (*performance.AppraisalCycle, []*performance.RatingLevel) {
	t.Helper()
	ctx := context.Background()

	scale, err := svc.CreateScale(ctx, testOrg, ownerUserID, performance.CreateScaleRequest{
		Name: "Standard", IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create scale: %v", err)
	}
	var levels []*performance.RatingLevel
	for i, spec := range []struct {
		label string
		value string
	}{{"Below", "1"}, {"Meets", "3"}, {"Exceeds", "5"}} {
		order := i
		l, err := svc.CreateLevel(ctx, testOrg, scale.ID, performance.CreateLevelRequest{
			Label: spec.label, Value: dec(spec.value), DisplayOrder: &order,
		})
		if err != nil {
			t.Fatalf("create level %s: %v", spec.label, err)
		}
		levels = append(levels, l)
	}

	selfTmpl, mgrTmpl := "tmpl_self", "tmpl_manager"
	cycle, err := svc.CreateAppraisalCycle(ctx, testOrg, ownerUserID, performance.CreateAppraisalCycleRequest{
		Name: "FY30 Review", PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		RatingScaleID: scale.ID, SelfFormTemplateID: &selfTmpl, ManagerFormTemplateID: &mgrTmpl,
	})
	if err != nil {
		t.Fatalf("create appraisal cycle: %v", err)
	}
	if _, err := svc.ActivateAppraisalCycle(ctx, testOrg, cycle.ID); err != nil {
		t.Fatalf("activate cycle: %v", err)
	}
	return cycle, levels
}

func seedAppraisal(t *testing.T, svc performance.Service, cycleID string) *performance.AppraisalDetail {
	t.Helper()
	a, err := svc.InstantiateAppraisal(context.Background(), testOrg, cycleID, ownerUserID,
		performance.InstantiateAppraisalRequest{EmployeeID: ownerEmpID})
	if err != nil {
		t.Fatalf("instantiate appraisal: %v", err)
	}
	return a
}

// advanceTo walks an appraisal to the target phase, marking the forms
// submitted as the preconditions require.
func advanceTo(t *testing.T, svc performance.Service, engine *stubFormEngine, a *performance.AppraisalDetail, target performance.Phase) *performance.AppraisalDetail {
	t.Helper()
	ctx := context.Background()
	caller := adminCaller()

	steps := []performance.Phase{performance.PhaseSelfReview, performance.PhaseManagerReview, performance.PhaseCalibration}
	current := a
	for _, step := range steps {
		if step == performance.PhaseManagerReview {
			engine.markSubmitted(*current.SelfFormInstanceID, "80")
		}
		if step == performance.PhaseCalibration {
			engine.markSubmitted(*current.ManagerFormInstanceID, "90")
		}
		out, err := svc.AdvancePhase(ctx, testOrg, current.ID, caller, performance.AdvancePhaseRequest{
			ToPhase: string(step),
		})
		if err != nil {
			t.Fatalf("advance to %s: %v", step, err)
		}
		current = out
		if step == target {
			return current
		}
	}
	return current
}

// ============================================================
// The transition map
// ============================================================

// TestPhaseTransitions_MapIsTheSingleSourceOfTruth exercises the declarative
// graph directly. This is the deviation from house style, so the map itself
// gets tested rather than only its effects.
func TestPhaseTransitions_MapIsTheSingleSourceOfTruth(t *testing.T) {
	legal := []struct{ from, to performance.Phase }{
		{performance.PhaseDraft, performance.PhaseSelfReview},
		{performance.PhaseDraft, performance.PhaseCancelled},
		{performance.PhaseSelfReview, performance.PhaseManagerReview},
		{performance.PhaseManagerReview, performance.PhaseCalibration},
		// The backward sends that make a rejected review recoverable without
		// cancelling it.
		{performance.PhaseManagerReview, performance.PhaseSelfReview},
		{performance.PhaseCalibration, performance.PhaseManagerReview},
		{performance.PhaseCalibration, performance.PhasePublished},
		{performance.PhasePublished, performance.PhaseAcknowledged},
	}
	for _, tc := range legal {
		if !performance.CanTransition(tc.from, tc.to) {
			t.Errorf("expected %s → %s to be legal", tc.from, tc.to)
		}
	}

	illegal := []struct{ from, to performance.Phase }{
		{performance.PhaseDraft, performance.PhasePublished},        // no skipping the whole flow
		{performance.PhaseSelfReview, performance.PhaseCalibration}, // no skipping manager review
		{performance.PhasePublished, performance.PhaseCalibration},  // publication is irreversible
		{performance.PhasePublished, performance.PhaseCancelled},    // ... including by cancelling
		{performance.PhaseAcknowledged, performance.PhasePublished},
		{performance.PhaseCancelled, performance.PhaseDraft},
	}
	for _, tc := range illegal {
		if performance.CanTransition(tc.from, tc.to) {
			t.Errorf("expected %s → %s to be ILLEGAL", tc.from, tc.to)
		}
	}

	// Terminality is derived from the map, not declared separately, so the
	// two can never disagree.
	if !performance.PhaseAcknowledged.IsTerminal() || !performance.PhaseCancelled.IsTerminal() {
		t.Error("expected acknowledged and cancelled to be terminal")
	}
	if performance.PhasePublished.IsTerminal() {
		t.Error("published is not terminal — acknowledged follows it")
	}
}

func TestAdvancePhase_RejectsIllegalTransition(t *testing.T) {
	svc, _, engine := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)
	a := seedAppraisal(t, svc, cycle.ID)
	_ = engine

	// draft → calibration skips two phases.
	_, err := svc.AdvancePhase(context.Background(), testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseCalibration)})
	if !errors.Is(err, performance.ErrIllegalPhaseTransition) {
		t.Fatalf("expected ErrIllegalPhaseTransition, got %v", err)
	}
}

// TestAdvancePhase_RequiresFormsSubmitted pins the preconditions that sit on
// top of the map: legality is necessary but not sufficient.
func TestAdvancePhase_RequiresFormsSubmitted(t *testing.T) {
	ctx := context.Background()
	svc, _, engine := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)
	a := seedAppraisal(t, svc, cycle.ID)

	a, err := svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseSelfReview)})
	if err != nil {
		t.Fatalf("advance to self_review: %v", err)
	}

	// The self form has not been submitted.
	_, err = svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseManagerReview)})
	if !errors.Is(err, performance.ErrSelfReviewIncomplete) {
		t.Fatalf("expected ErrSelfReviewIncomplete, got %v", err)
	}

	engine.markSubmitted(*a.SelfFormInstanceID, "80")
	a, err = svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseManagerReview)})
	if err != nil {
		t.Fatalf("expected the move to succeed once the self form is submitted, got %v", err)
	}

	// Same again for the manager form before calibration.
	_, err = svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseCalibration)})
	if !errors.Is(err, performance.ErrManagerReviewIncomplete) {
		t.Fatalf("expected ErrManagerReviewIncomplete, got %v", err)
	}
}

// TestAdvancePhase_SendBackDoesNotReDemandTheSelfForm covers the asymmetry
// that a naive precondition check gets wrong: manager_review → self_review is
// a send-back, and must not require the self form to be submitted again.
func TestAdvancePhase_SendBackDoesNotReDemandTheSelfForm(t *testing.T) {
	ctx := context.Background()
	svc, _, engine := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)
	a := advanceTo(t, svc, engine, seedAppraisal(t, svc, cycle.ID), performance.PhaseManagerReview)

	back, err := svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseSelfReview)})
	if err != nil {
		t.Fatalf("expected a send-back to self_review to succeed, got %v", err)
	}
	if back.Phase != performance.PhaseSelfReview {
		t.Errorf("expected phase self_review, got %s", back.Phase)
	}
}

// ============================================================
// Rating + calibration audit
// ============================================================

func TestSetRating_OnlyDuringManagerReview(t *testing.T) {
	ctx := context.Background()
	svc, _, engine := newAppraisalSvc(true)
	cycle, levels := seedAppraisalCycle(t, svc)
	a := seedAppraisal(t, svc, cycle.ID)

	// draft is too early.
	_, err := svc.SetRating(ctx, testOrg, a.ID, adminCaller(), performance.SetRatingRequest{
		RatingLevelID: levels[1].ID,
	})
	if !errors.Is(err, performance.ErrIllegalPhaseTransition) {
		t.Fatalf("expected a rating in draft to be refused, got %v", err)
	}

	a = advanceTo(t, svc, engine, a, performance.PhaseManagerReview)
	rated, err := svc.SetRating(ctx, testOrg, a.ID, adminCaller(), performance.SetRatingRequest{
		RatingLevelID: levels[1].ID,
	})
	if err != nil {
		t.Fatalf("set rating: %v", err)
	}
	// The FK and the snapshot move together — a FK without its snapshot would
	// lose the reading if the level were later renamed.
	if rated.FinalRatingLevelID == nil || *rated.FinalRatingLevelID != levels[1].ID {
		t.Error("expected the rating level FK to be set")
	}
	if rated.FinalRatingLabel == nil || *rated.FinalRatingLabel != "Meets" {
		t.Errorf("expected the label snapshot, got %v", rated.FinalRatingLabel)
	}
	if rated.FinalRatingValue == nil || !rated.FinalRatingValue.Equal(dec("3")) {
		t.Errorf("expected the value snapshot, got %v", rated.FinalRatingValue)
	}
}

// TestCalibrate_RequiresNoteAndRecordsBothRatings is the audit requirement the
// build plan calls mandatory.
func TestCalibrate_RequiresNoteAndRecordsBothRatings(t *testing.T) {
	ctx := context.Background()
	svc, _, engine := newAppraisalSvc(true)
	cycle, levels := seedAppraisalCycle(t, svc)
	a := advanceTo(t, svc, engine, seedAppraisal(t, svc, cycle.ID), performance.PhaseManagerReview)

	if _, err := svc.SetRating(ctx, testOrg, a.ID, adminCaller(), performance.SetRatingRequest{
		RatingLevelID: levels[1].ID, // Meets
	}); err != nil {
		t.Fatalf("set rating: %v", err)
	}
	engine.markSubmitted(*a.ManagerFormInstanceID, "90")
	a, err := svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseCalibration)})
	if err != nil {
		t.Fatalf("advance to calibration: %v", err)
	}

	// An unexplained override is refused.
	_, err = svc.Calibrate(ctx, testOrg, a.ID, adminCaller(), performance.CalibrateRequest{
		RatingLevelID: levels[2].ID, Note: "   ",
	})
	if !errors.Is(err, performance.ErrCalibrationNoteReq) {
		t.Fatalf("expected ErrCalibrationNoteReq, got %v", err)
	}

	out, err := svc.Calibrate(ctx, testOrg, a.ID, adminCaller(), performance.CalibrateRequest{
		RatingLevelID: levels[2].ID, Note: "Consistent over-delivery across the year",
	})
	if err != nil {
		t.Fatalf("calibrate: %v", err)
	}
	if out.FinalRatingLabel == nil || *out.FinalRatingLabel != "Exceeds" {
		t.Errorf("expected the calibrated label, got %v", out.FinalRatingLabel)
	}

	// The audit must answer "from what, to what, by whom, and why" from one
	// row — that is why calibration lives in phase history rather than its own
	// table.
	var calibration *performance.PhaseHistory
	for _, h := range out.History {
		if h.ToRatingLevelID != nil && *h.ToRatingLevelID == levels[2].ID {
			calibration = h
		}
	}
	if calibration == nil {
		t.Fatal("expected a phase-history row recording the calibration")
	}
	if calibration.FromRatingLabel == nil || *calibration.FromRatingLabel != "Meets" {
		t.Errorf("expected the PREVIOUS rating recorded, got %v", calibration.FromRatingLabel)
	}
	if calibration.Note == nil || *calibration.Note == "" {
		t.Error("expected the calibration note recorded")
	}
	if calibration.ChangedBy == nil {
		t.Error("expected the calibrating user recorded")
	}
}

func TestCalibrate_OnlyDuringCalibrationPhase(t *testing.T) {
	ctx := context.Background()
	svc, _, engine := newAppraisalSvc(true)
	cycle, levels := seedAppraisalCycle(t, svc)
	a := advanceTo(t, svc, engine, seedAppraisal(t, svc, cycle.ID), performance.PhaseManagerReview)

	_, err := svc.Calibrate(ctx, testOrg, a.ID, adminCaller(), performance.CalibrateRequest{
		RatingLevelID: levels[2].ID, Note: "too early",
	})
	if !errors.Is(err, performance.ErrNotInCalibration) {
		t.Fatalf("expected ErrNotInCalibration, got %v", err)
	}
}

// ============================================================
// Publish immutability
// ============================================================

func TestPublish_RequiresARating(t *testing.T) {
	svc, _, engine := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)
	a := advanceTo(t, svc, engine, seedAppraisal(t, svc, cycle.ID), performance.PhaseCalibration)

	_, err := svc.PublishAppraisal(context.Background(), testOrg, a.ID, adminCaller())
	if !errors.Is(err, performance.ErrRatingRequiredToPublish) {
		t.Fatalf("expected ErrRatingRequiredToPublish, got %v", err)
	}
}

// TestPublish_FreezesScoresAndBlocksFurtherChange is the payslip pattern
// applied to appraisals: after publish the record must report the same
// numbers forever, so the figures are snapshotted rather than recomputed.
func TestPublish_FreezesScoresAndBlocksFurtherChange(t *testing.T) {
	ctx := context.Background()
	svc, _, engine := newAppraisalSvc(true)
	cycle, levels := seedAppraisalCycle(t, svc)
	a := advanceTo(t, svc, engine, seedAppraisal(t, svc, cycle.ID), performance.PhaseManagerReview)

	if _, err := svc.SetRating(ctx, testOrg, a.ID, adminCaller(), performance.SetRatingRequest{
		RatingLevelID: levels[1].ID,
	}); err != nil {
		t.Fatalf("set rating: %v", err)
	}
	engine.markSubmitted(*a.ManagerFormInstanceID, "90")
	a, _ = svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseCalibration)})

	published, err := svc.PublishAppraisal(ctx, testOrg, a.ID, adminCaller())
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Phase != performance.PhasePublished {
		t.Fatalf("expected phase published, got %s", published.Phase)
	}
	if published.SelfScore == nil || !published.SelfScore.Equal(dec("80")) {
		t.Errorf("expected the self score frozen at 80, got %v", published.SelfScore)
	}
	if published.ManagerScore == nil || !published.ManagerScore.Equal(dec("90")) {
		t.Errorf("expected the manager score frozen at 90, got %v", published.ManagerScore)
	}

	// Changing the underlying form score must NOT move a published appraisal.
	engine.scores[*a.ManagerFormInstanceID] = dec("10")
	after, err := svc.GetAppraisal(ctx, testOrg, a.ID, adminCaller())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if after.ManagerScore == nil || !after.ManagerScore.Equal(dec("90")) {
		t.Errorf("published appraisal must report its frozen score, got %v", after.ManagerScore)
	}

	// And no further edits are permitted.
	if _, err := svc.SetRating(ctx, testOrg, a.ID, adminCaller(), performance.SetRatingRequest{
		RatingLevelID: levels[0].ID,
	}); !errors.Is(err, performance.ErrAppraisalPublished) {
		t.Fatalf("expected ErrAppraisalPublished on re-rating, got %v", err)
	}
	if _, err := svc.Calibrate(ctx, testOrg, a.ID, adminCaller(), performance.CalibrateRequest{
		RatingLevelID: levels[0].ID, Note: "too late",
	}); !errors.Is(err, performance.ErrAppraisalPublished) {
		t.Fatalf("expected ErrAppraisalPublished on calibration, got %v", err)
	}
	// The only legal move out of published.
	if _, err := svc.AdvancePhase(ctx, testOrg, a.ID, adminCaller(),
		performance.AdvancePhaseRequest{ToPhase: string(performance.PhaseAcknowledged)}); err != nil {
		t.Fatalf("expected acknowledged to be reachable from published, got %v", err)
	}
}

// ============================================================
// Instantiation
// ============================================================

// TestInstantiate_FreezesManagerAndSplitsRespondents pins two decisions: the
// manager is snapshotted, and the self/manager forms go to DIFFERENT
// respondents — the split the form engine keeps separate columns for.
func TestInstantiate_FreezesManagerAndSplitsRespondents(t *testing.T) {
	svc, _, engine := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)
	a := seedAppraisal(t, svc, cycle.ID)

	if a.ManagerEmployeeIDSnapshot == nil || *a.ManagerEmployeeIDSnapshot != otherEmpID {
		t.Errorf("expected the manager frozen at instantiation, got %v", a.ManagerEmployeeIDSnapshot)
	}
	if a.SelfFormInstanceID == nil || a.ManagerFormInstanceID == nil {
		t.Fatal("expected both forms instantiated")
	}
	if len(engine.instantiated) != 2 {
		t.Fatalf("expected 2 form instantiations, got %d", len(engine.instantiated))
	}
	selfCtx, mgrCtx := engine.instantiated[0], engine.instantiated[1]
	if selfCtx.RespondentRole != "self" || mgrCtx.RespondentRole != "manager" {
		t.Errorf("expected self and manager respondent roles, got %q / %q", selfCtx.RespondentRole, mgrCtx.RespondentRole)
	}
	// Both forms are ABOUT the same employee...
	if selfCtx.SubjectID != ownerEmpID || mgrCtx.SubjectID != ownerEmpID {
		t.Error("expected both forms to have the appraisee as their subject")
	}
	// ... but answered by different people. The manager here has no linked
	// user account, which is a legitimate nil rather than an error.
	if selfCtx.RespondentUserID == nil || *selfCtx.RespondentUserID != ownerUserID {
		t.Errorf("expected the self form assigned to the appraisee, got %v", selfCtx.RespondentUserID)
	}
	if mgrCtx.RespondentUserID != nil {
		t.Errorf("expected a nil respondent for a manager with no account, got %v", mgrCtx.RespondentUserID)
	}
}

func TestInstantiate_RejectsDuplicateAndInactiveCycle(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)
	seedAppraisal(t, svc, cycle.ID)

	_, err := svc.InstantiateAppraisal(ctx, testOrg, cycle.ID, ownerUserID,
		performance.InstantiateAppraisalRequest{EmployeeID: ownerEmpID})
	if !errors.Is(err, performance.ErrAppraisalExists) {
		t.Fatalf("expected ErrAppraisalExists, got %v", err)
	}

	if _, err := svc.CloseAppraisalCycle(ctx, testOrg, cycle.ID); err != nil {
		t.Fatalf("close cycle: %v", err)
	}
	_, err = svc.InstantiateAppraisal(ctx, testOrg, cycle.ID, ownerUserID,
		performance.InstantiateAppraisalRequest{EmployeeID: otherEmpID})
	if !errors.Is(err, performance.ErrAppraisalCycleNotActive) {
		t.Fatalf("expected ErrAppraisalCycleNotActive, got %v", err)
	}
}

// ============================================================
// Scope + configuration guards
// ============================================================

// TestGetAppraisal_DeniesOutOfScope is this module's named failure mode:
// appraisal draft leakage.
func TestGetAppraisal_DeniesOutOfScope(t *testing.T) {
	svc, _, _ := newAppraisalSvc(false) // authorizer denies
	cycle, _ := seedAppraisalCycle(t, svc)
	a := seedAppraisal(t, svc, cycle.ID)

	caller := performance.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam}
	_, err := svc.GetAppraisal(context.Background(), testOrg, a.ID, caller)
	if !errors.Is(err, performance.ErrAppraisalAccessDenied) {
		t.Fatalf("expected ErrAppraisalAccessDenied, got %v", err)
	}
}

func TestCreateAppraisalCycle_RejectsScaleWithNoLevels(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAppraisalSvc(true)

	scale, err := svc.CreateScale(ctx, testOrg, ownerUserID, performance.CreateScaleRequest{Name: "Empty"})
	if err != nil {
		t.Fatalf("create scale: %v", err)
	}
	tmpl := "tmpl_self"
	_, err = svc.CreateAppraisalCycle(ctx, testOrg, ownerUserID, performance.CreateAppraisalCycleRequest{
		Name: "No levels", PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		RatingScaleID: scale.ID, SelfFormTemplateID: &tmpl,
	})
	if !errors.Is(err, performance.ErrScaleNoLevels) {
		t.Fatalf("expected ErrScaleNoLevels, got %v", err)
	}
}

func TestCreateAppraisalCycle_RequiresAtLeastOneForm(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)

	_, err := svc.CreateAppraisalCycle(ctx, testOrg, ownerUserID, performance.CreateAppraisalCycleRequest{
		Name: "Formless", PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		RatingScaleID: cycle.RatingScaleID,
	})
	if !errors.Is(err, performance.ErrFormTemplateRequired) {
		t.Fatalf("expected ErrFormTemplateRequired, got %v", err)
	}
}

// TestSetRating_RejectsLevelFromAnotherScale guards a rating that would be
// numerically meaningless against the cycle's scale.
func TestSetRating_RejectsLevelFromAnotherScale(t *testing.T) {
	ctx := context.Background()
	svc, _, engine := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)
	a := advanceTo(t, svc, engine, seedAppraisal(t, svc, cycle.ID), performance.PhaseManagerReview)

	other, err := svc.CreateScale(ctx, testOrg, ownerUserID, performance.CreateScaleRequest{Name: "Other scale"})
	if err != nil {
		t.Fatalf("create other scale: %v", err)
	}
	foreign, err := svc.CreateLevel(ctx, testOrg, other.ID, performance.CreateLevelRequest{
		Label: "Foreign", Value: decimal.NewFromInt(9),
	})
	if err != nil {
		t.Fatalf("create foreign level: %v", err)
	}

	_, err = svc.SetRating(ctx, testOrg, a.ID, adminCaller(), performance.SetRatingRequest{
		RatingLevelID: foreign.ID,
	})
	if !errors.Is(err, performance.ErrRatingLevelWrongScale) {
		t.Fatalf("expected ErrRatingLevelWrongScale, got %v", err)
	}
}

func TestDeleteScale_BlockedWhileACycleUsesIt(t *testing.T) {
	svc, _, _ := newAppraisalSvc(true)
	cycle, _ := seedAppraisalCycle(t, svc)

	if err := svc.DeleteScale(context.Background(), testOrg, cycle.RatingScaleID); !errors.Is(err, performance.ErrScaleInUse) {
		t.Fatalf("expected ErrScaleInUse, got %v", err)
	}
}
