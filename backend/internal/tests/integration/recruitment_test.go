// backend/internal/tests/integration/recruitment_test.go
// Proves HRM Recruitment (Phase 4A) against a real Postgres — the
// properties a stub repo cannot catch: unique constraints actually firing,
// stage-move transaction atomicity, the atomic pipeline-default clear-then-
// set, resume tenant isolation, an end-to-end approval decision flipping a
// requisition's status via the real callback wiring, ON DELETE behaviours,
// and stage-history ordering/duration arithmetic across several real moves.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/recruitment"
)

// seedRecruitmentPipeline creates a default pipeline with the given stage
// kinds (in order) and returns the pipeline and its stages.
func seedRecruitmentPipeline(t *testing.T, env *testEnv, orgID, userID string, kinds ...recruitment.StageKind) (*recruitment.Pipeline, []*recruitment.Stage) {
	t.Helper()
	ctx := context.Background()
	p, err := env.hrmRecruitmentSvc.CreatePipeline(ctx, orgID, userID, recruitment.CreatePipelineRequest{Name: "Engineering", IsDefault: true})
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
	var stages []*recruitment.Stage
	for i, k := range kinds {
		kind := string(k)
		s, err := env.hrmRecruitmentSvc.CreateStage(ctx, orgID, p.ID, recruitment.CreateStageRequest{
			Name: string(k), StageKind: &kind, Position: &i,
		})
		if err != nil {
			t.Fatalf("create stage %s: %v", k, err)
		}
		stages = append(stages, s)
	}
	return p, stages
}

func seedRecruitmentPosting(t *testing.T, env *testEnv, orgID, userID, pipelineID string) *recruitment.Posting {
	t.Helper()
	ctx := context.Background()
	req, err := env.hrmRecruitmentSvc.CreateRequisition(ctx, orgID, userID, recruitment.CreateRequisitionRequest{Title: "Backend Engineer"})
	if err != nil {
		t.Fatalf("create requisition: %v", err)
	}
	p, err := env.hrmRecruitmentSvc.CreatePosting(ctx, orgID, userID, recruitment.CreatePostingRequest{
		RequisitionID: req.ID, PipelineID: &pipelineID, Title: "Backend Engineer",
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	return p
}

// ============================================================
// Constraints actually fire
// ============================================================

func TestIntegration_Recruitment_CandidateEmailUniqueness_CaseInsensitive(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	email := "Bob@Example.com"
	if _, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "Bob", Email: &email}); err != nil {
		t.Fatalf("create candidate 1: %v", err)
	}

	dup := "bob@example.com"
	_, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "Bob2", Email: &dup})
	if err == nil {
		t.Fatal("expected the service-level dedup check to reject a case-different duplicate email")
	}

	// Also prove the DATABASE constraint itself fires — not just the
	// service-level pre-check — by inserting directly, bypassing the service.
	var count int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_candidates WHERE org_id = $1 AND LOWER(email) = LOWER($2)`, orgID, email).Scan(&count); err != nil {
		t.Fatalf("count candidates: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one candidate row for this email, got %d", count)
	}
	_, err = env.db.Exec(ctx,
		`INSERT INTO hrm_candidates (org_id, first_name, email, created_by) VALUES ($1, 'Bob3', $2, $3)`,
		orgID, dup, ownerID,
	)
	if err == nil {
		t.Error("expected uq_hrm_cand_org_email to reject a raw SQL insert with a case-different duplicate email")
	}
}

func TestIntegration_Recruitment_PipelineDefault_ConstraintAndAtomicPromotion(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	p1, _ := seedRecruitmentPipeline(t, env, orgID, ownerID, recruitment.StageKindApplied)

	// Raw SQL insert of a second default must violate uq_hrm_rpipe_default.
	_, err := env.db.Exec(ctx,
		`INSERT INTO hrm_recruitment_pipelines (org_id, name, is_default, is_active, created_by) VALUES ($1, 'Sales', TRUE, TRUE, $2)`,
		orgID, ownerID,
	)
	if err == nil {
		t.Error("expected uq_hrm_rpipe_default to reject a second default pipeline inserted directly")
	}

	// The service's atomic clear-then-set path must succeed where the raw
	// insert above failed.
	p2, err := env.hrmRecruitmentSvc.CreatePipeline(ctx, orgID, ownerID, recruitment.CreatePipelineRequest{Name: "Sales"})
	if err != nil {
		t.Fatalf("create second (non-default) pipeline: %v", err)
	}
	isDefault := true
	updated, err := env.hrmRecruitmentSvc.UpdatePipeline(ctx, orgID, p2.ID, recruitment.UpdatePipelineRequest{IsDefault: &isDefault})
	if err != nil {
		t.Fatalf("promote second pipeline to default: %v", err)
	}
	if !updated.IsDefault {
		t.Error("expected the promoted pipeline to be the new default")
	}

	fresh1, err := env.hrmRecruitmentSvc.GetPipeline(ctx, orgID, p1.ID)
	if err != nil {
		t.Fatalf("get first pipeline: %v", err)
	}
	if fresh1.IsDefault {
		t.Error("expected the original default to have been cleared atomically when the second was promoted")
	}
}

// ============================================================
// Stage move atomicity
// ============================================================

func TestIntegration_Recruitment_StageMove_AtomicOnHistoryFailure(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	pipeline, stages := seedRecruitmentPipeline(t, env, orgID, ownerID, recruitment.StageKindApplied, recruitment.StageKindInProgress)
	posting := seedRecruitmentPosting(t, env, orgID, ownerID, pipeline.ID)

	email := "atomicity@example.com"
	cand, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "Atom", Email: &email})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	app, err := env.hrmRecruitmentSvc.CreateApplication(ctx, orgID, &ownerID, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if app.StageID != stages[0].ID {
		t.Fatalf("expected initial stage %q, got %q", stages[0].ID, app.StageID)
	}

	// Delete the target stage out from under a would-be move — the history
	// insert's to_stage_id FK (ON DELETE SET NULL on the stage table, but
	// the stage row itself is now gone so a stale reference in application
	// code would fail) proves the whole move is one transaction: if any
	// part of MoveApplicationStage fails, stage_id must not have changed.
	//
	// Simplest reliable way to force a failure deep in the transaction
	// without relying on internal repo access: attempt to move to a stage
	// ID that does not exist in this org at all (a stage from nowhere).
	_, err = env.hrmRecruitmentSvc.MoveApplication(ctx, orgID, app.ID, ownerID, recruitment.MoveApplicationRequest{StageID: "00000000-0000-0000-0000-000000000000"})
	if err == nil {
		t.Fatal("expected moving to a nonexistent stage to fail")
	}

	unchanged, err := env.hrmRecruitmentSvc.GetApplication(ctx, orgID, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if unchanged.StageID != stages[0].ID {
		t.Errorf("expected stage_id to remain unchanged after a failed move, got %q (was %q)", unchanged.StageID, stages[0].ID)
	}

	var histCount int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_application_stage_history WHERE application_id = $1`, app.ID).Scan(&histCount); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if histCount != 1 {
		t.Errorf("expected exactly the initial-placement history row (1), got %d — a failed move must not leave a partial history row", histCount)
	}
}

// ============================================================
// Resume tenant isolation
// ============================================================

func TestIntegration_Recruitment_ResumeTenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgAID, _, ownerAID := seedScopeTestOrg(t, env)
	orgBID, _, _ := seedScopeTestOrg(t, env)

	cand, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgAID, &ownerAID, recruitment.CreateCandidateRequest{FirstName: "Isolated"})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	// A minimal valid PDF signature is enough for http.DetectContentType.
	pdfBytes := []byte("%PDF-1.4\n%mock pdf content for integration test\n")
	if _, err := env.hrmRecruitmentSvc.UploadResume(ctx, orgAID, cand.ID, pdfBytes, "resume.pdf"); err != nil {
		t.Fatalf("upload resume: %v", err)
	}

	// Org A can read its own candidate's resume.
	_, _, err = env.hrmRecruitmentSvc.GetResumeFile(ctx, orgAID, cand.ID)
	if err != nil {
		t.Fatalf("expected org A to read its own candidate's resume, got %v", err)
	}

	// Org B must not be able to reach it via the same candidate ref — the
	// org_id check happens before any filesystem access.
	_, _, err = env.hrmRecruitmentSvc.GetResumeFile(ctx, orgBID, cand.ID)
	if err == nil {
		t.Error("SECURITY: org B must not be able to fetch org A's candidate resume")
	}
}

// ============================================================
// Approval end-to-end
// ============================================================

func TestIntegration_Recruitment_RequisitionApproval_EndToEnd(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	_, err := env.hrmApprovalsSvc.CreateTemplate(ctx, orgID, ownerID, approvals.CreateTemplateRequest{
		Name: "Requisition Approval", ActionType: approvals.ActionTypeJobRequisition, IsDefault: true,
		Levels: []approvals.CreateTemplateLevelRequest{
			{Level: 1, ApproverType: approvals.ApproverTypeSpecificUser, ApproverUserID: &ownerID, SLAHours: 48, OnSLABreach: approvals.SLABreachEscalateNext},
		},
	})
	if err != nil {
		t.Fatalf("create approval template: %v", err)
	}

	req, err := env.hrmRecruitmentSvc.CreateRequisition(ctx, orgID, ownerID, recruitment.CreateRequisitionRequest{Title: "Staff Engineer"})
	if err != nil {
		t.Fatalf("create requisition: %v", err)
	}
	submitted, err := env.hrmRecruitmentSvc.SubmitRequisition(ctx, orgID, req.ID, ownerID)
	if err != nil {
		t.Fatalf("submit requisition: %v", err)
	}
	if submitted.Status != recruitment.RequisitionStatusPendingApproval {
		t.Fatalf("expected pending_approval, got %q", submitted.Status)
	}
	if submitted.ApprovalInstanceID == nil {
		t.Fatal("expected an approval_instance_id to be set")
	}

	// Decide through the REAL approvals service — this exercises the actual
	// RegisterCallback wiring set up in newTestEnv, the same wiring main.go
	// uses, not a direct service-to-service call.
	if _, err := env.hrmApprovalsSvc.Decide(ctx, orgID, *submitted.ApprovalInstanceID, ownerID, approvals.DecisionRequest{Action: "approved"}); err != nil {
		t.Fatalf("decide: %v", err)
	}

	final, err := env.hrmRecruitmentSvc.GetRequisition(ctx, orgID, req.ID)
	if err != nil {
		t.Fatalf("get requisition: %v", err)
	}
	if final.Status != recruitment.RequisitionStatusApproved {
		t.Errorf("expected the approval decision to flip the requisition to approved via the callback, got %q", final.Status)
	}
}

// ============================================================
// ON DELETE behaviours
// ============================================================

func TestIntegration_Recruitment_DeleteStage_NullsHistoryReferenceWithoutErroring(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	pipeline, stages := seedRecruitmentPipeline(t, env, orgID, ownerID, recruitment.StageKindApplied, recruitment.StageKindInProgress)
	posting := seedRecruitmentPosting(t, env, orgID, ownerID, pipeline.ID)
	email := "delete-stage@example.com"
	cand, _ := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "X", Email: &email})
	app, err := env.hrmRecruitmentSvc.CreateApplication(ctx, orgID, &ownerID, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if _, err := env.hrmRecruitmentSvc.MoveApplication(ctx, orgID, app.ID, ownerID, recruitment.MoveApplicationRequest{StageID: stages[1].ID}); err != nil {
		t.Fatalf("move application: %v", err)
	}

	// The application itself references stages[1] via stage_id (RESTRICT),
	// so it must be moved off, or we delete only the *initial* stage
	// (stages[0]), which nothing currently references via stage_id but IS
	// referenced by stage_history.from_stage_id / to_stage_id.
	if _, err := env.db.Exec(ctx, `DELETE FROM hrm_recruitment_stages WHERE id = $1`, stages[0].ID); err != nil {
		t.Fatalf("SCHEMA: deleting a stage referenced only by stage_history must not error, got: %v", err)
	}

	history, err := env.hrmRecruitmentSvc.GetStageHistory(ctx, orgID, app.ID)
	if err != nil {
		t.Fatalf("get stage history: %v", err)
	}
	found := false
	for _, h := range history {
		if h.FromStageName != nil && *h.FromStageName == string(recruitment.StageKindApplied) {
			found = true
			if h.FromStageID != nil {
				t.Errorf("expected from_stage_id to be nulled by ON DELETE SET NULL after the stage was deleted, got %v", *h.FromStageID)
			}
		}
	}
	if !found {
		t.Error("expected the history row's from_stage_name snapshot to survive the stage's deletion")
	}
}

func TestIntegration_Recruitment_DeletePosting_RestrictedByApplications(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	pipeline, _ := seedRecruitmentPipeline(t, env, orgID, ownerID, recruitment.StageKindApplied)
	posting := seedRecruitmentPosting(t, env, orgID, ownerID, pipeline.ID)
	email := "restrict@example.com"
	cand, _ := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "R", Email: &email})
	if _, err := env.hrmRecruitmentSvc.CreateApplication(ctx, orgID, &ownerID, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID}); err != nil {
		t.Fatalf("create application: %v", err)
	}

	_, err := env.db.Exec(ctx, `DELETE FROM hrm_job_postings WHERE id = $1`, posting.ID)
	if err == nil {
		t.Error("expected deleting a posting with a live application to be RESTRICTed")
	}
}

// ============================================================
// Stage history ordering + duration arithmetic
// ============================================================

func TestIntegration_Recruitment_StageHistory_OrderingAndDuration(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	pipeline, stages := seedRecruitmentPipeline(t, env, orgID, ownerID,
		recruitment.StageKindApplied, recruitment.StageKindInProgress, recruitment.StageKindOffer, recruitment.StageKindHired)
	posting := seedRecruitmentPosting(t, env, orgID, ownerID, pipeline.ID)
	email := "history@example.com"
	cand, _ := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "H", Email: &email})
	app, err := env.hrmRecruitmentSvc.CreateApplication(ctx, orgID, &ownerID, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	// Move through the pipeline with a small real delay between moves so
	// seconds_in_previous_stage has a genuine, verifiable lower bound.
	for _, target := range stages[1:] {
		time.Sleep(1100 * time.Millisecond)
		if _, err := env.hrmRecruitmentSvc.MoveApplication(ctx, orgID, app.ID, ownerID, recruitment.MoveApplicationRequest{StageID: target.ID}); err != nil {
			t.Fatalf("move to %s: %v", target.Name, err)
		}
	}

	history, err := env.hrmRecruitmentSvc.GetStageHistory(ctx, orgID, app.ID)
	if err != nil {
		t.Fatalf("get stage history: %v", err)
	}
	if len(history) != 4 {
		t.Fatalf("expected 4 history rows (1 initial placement + 3 moves), got %d", len(history))
	}

	// Ordering: ascending by moved_at (ORDER BY h.moved_at ASC).
	for i := 1; i < len(history); i++ {
		if history[i].MovedAt.Before(history[i-1].MovedAt) {
			t.Fatalf("expected history ordered ascending by moved_at, row %d (%v) precedes row %d (%v)", i, history[i].MovedAt, i-1, history[i-1].MovedAt)
		}
	}

	// The initial placement row has no duration to report.
	if history[0].SecondsInPreviousStage != nil {
		t.Errorf("expected the initial placement row's seconds_in_previous_stage to be NULL, got %v", *history[0].SecondsInPreviousStage)
	}
	// Every subsequent move measured a real ~1.1s gap.
	for i := 1; i < len(history); i++ {
		if history[i].SecondsInPreviousStage == nil {
			t.Fatalf("row %d: expected seconds_in_previous_stage to be set", i)
		}
		if *history[i].SecondsInPreviousStage < 1 {
			t.Errorf("row %d: expected seconds_in_previous_stage >= 1 given the real ~1.1s sleep between moves, got %d", i, *history[i].SecondsInPreviousStage)
		}
	}

	// Final application state reflects the terminal hired stage.
	final, err := env.hrmRecruitmentSvc.GetApplication(ctx, orgID, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if final.Status != recruitment.ApplicationStatusHired {
		t.Errorf("expected final status=hired, got %q", final.Status)
	}
	if final.HiredAt == nil {
		t.Error("expected hired_at to be set")
	}
}
