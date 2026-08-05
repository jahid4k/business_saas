// backend/internal/tests/integration/recruitment_selection_test.go
// Proves HRM Recruitment (Phase 4B — interviews/scorecards/offers/referrals/
// hire) against a real Postgres — hire-conversion transaction atomicity, the
// source_candidate_id FK's ON DELETE SET NULL, the scorecard panelist unique
// constraint actually firing, an end-to-end offer approval flipping status
// via the real callback wiring, and the FOR UPDATE lock preventing a
// double-hire under concurrency.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	hrmemployees "github.com/mridha/businesssaas/internal/hrm/employees"
	"github.com/mridha/businesssaas/internal/hrm/recruitment"
)

// seedHiredApplication creates a pipeline (applied → hired), a candidate, a
// requisition, a posting, and an application already moved into the hired
// stage — the precondition HireApplication requires.
func seedHiredApplication(t *testing.T, env *testEnv, orgID, ownerID string) (app *recruitment.Application, cand *recruitment.Candidate, req *recruitment.Requisition) {
	t.Helper()
	ctx := context.Background()

	pipeline, stages := seedRecruitmentPipeline(t, env, orgID, ownerID, recruitment.StageKindApplied, recruitment.StageKindHired)
	req, err := env.hrmRecruitmentSvc.CreateRequisition(ctx, orgID, ownerID, recruitment.CreateRequisitionRequest{Title: "Backend Engineer"})
	if err != nil {
		t.Fatalf("create requisition: %v", err)
	}
	posting, err := env.hrmRecruitmentSvc.CreatePosting(ctx, orgID, ownerID, recruitment.CreatePostingRequest{
		RequisitionID: req.ID, PipelineID: &pipeline.ID, Title: "Backend Engineer",
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	email := uniqueEmail("hire-candidate")
	cand, err = env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "Hana", LastName: strPtrRecruitment("Kim"), Email: &email})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	app, err = env.hrmRecruitmentSvc.CreateApplication(ctx, orgID, &ownerID, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	app, err = env.hrmRecruitmentSvc.MoveApplication(ctx, orgID, app.ID, ownerID, recruitment.MoveApplicationRequest{StageID: stages[1].ID})
	if err != nil {
		t.Fatalf("move to hired stage: %v", err)
	}
	return app, cand, req
}

func strPtrRecruitment(s string) *string { return &s }

// ============================================================
// Hire conversion — atomicity
// ============================================================

func TestIntegration_Recruitment_HireApplication_Atomic_OnEmployeeInsertFailure(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	app, cand, req := seedHiredApplication(t, env, orgID, ownerID)

	// A department_id that does not exist violates hrm_employees'
	// REFERENCES hrm_departments(id) FK — this fails deep inside
	// CreateEmployeeTx's INSERT, after the transaction is already open.
	bogusDept := "00000000-0000-0000-0000-000000000000"
	_, err := env.hrmRecruitmentSvc.HireApplication(ctx, orgID, app.ID, ownerID, recruitment.HireApplicationRequest{DepartmentID: &bogusDept})
	if err == nil {
		t.Fatal("expected hiring with a nonexistent department_id to fail the employee insert")
	}

	// Nothing must have been written: no employee row for this candidate,
	// the application's converted_employee_id must remain unset, and the
	// requisition's filled_count must remain 0.
	var empCount int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_employees WHERE source_candidate_id = $1`, cand.ID).Scan(&empCount); err != nil {
		t.Fatalf("count employees: %v", err)
	}
	if empCount != 0 {
		t.Errorf("expected no employee row to exist after a failed hire, got %d", empCount)
	}

	unchangedApp, err := env.hrmRecruitmentSvc.GetApplication(ctx, orgID, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if unchangedApp.ConvertedEmployeeID != nil {
		t.Error("expected converted_employee_id to remain unset after a failed hire")
	}

	unchangedReq, err := env.hrmRecruitmentSvc.GetRequisition(ctx, orgID, req.ID)
	if err != nil {
		t.Fatalf("get requisition: %v", err)
	}
	if unchangedReq.FilledCount != 0 {
		t.Errorf("expected filled_count to remain 0 after a failed hire, got %d", unchangedReq.FilledCount)
	}
}

// ============================================================
// Hire conversion — happy path + source_candidate_id FK
// ============================================================

func TestIntegration_Recruitment_HireApplication_SourceCandidateID_RoundTripsAndNullsOnDelete(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	app, cand, req := seedHiredApplication(t, env, orgID, ownerID)

	res, err := env.hrmRecruitmentSvc.HireApplication(ctx, orgID, app.ID, ownerID, recruitment.HireApplicationRequest{})
	if err != nil {
		t.Fatalf("hire application: %v", err)
	}
	if res.EmployeeID == "" {
		t.Fatal("expected a non-empty employee id")
	}

	emp, err := env.hrmEmpSvc.Get(ctx, orgID, res.EmployeeID)
	if err != nil {
		t.Fatalf("get employee: %v", err)
	}
	if emp.SourceCandidateID == nil || *emp.SourceCandidateID != cand.ID {
		t.Errorf("expected source_candidate_id to round-trip to %q, got %v", cand.ID, emp.SourceCandidateID)
	}

	updatedReq, err := env.hrmRecruitmentSvc.GetRequisition(ctx, orgID, req.ID)
	if err != nil {
		t.Fatalf("get requisition: %v", err)
	}
	if updatedReq.FilledCount != 1 {
		t.Errorf("expected filled_count incremented to 1, got %d", updatedReq.FilledCount)
	}

	// Deleting the candidate must SET NULL on the employee's
	// source_candidate_id, not fail and not delete the employee.
	if _, err := env.db.Exec(ctx, `DELETE FROM hrm_candidates WHERE id = $1`, cand.ID); err != nil {
		t.Fatalf("SCHEMA: deleting a candidate referenced only by hrm_employees.source_candidate_id must not error, got: %v", err)
	}
	afterDelete, err := env.hrmEmpSvc.Get(ctx, orgID, res.EmployeeID)
	if err != nil {
		t.Fatalf("get employee after candidate delete: %v", err)
	}
	if afterDelete.SourceCandidateID != nil {
		t.Errorf("expected source_candidate_id to be nulled by ON DELETE SET NULL, got %v", *afterDelete.SourceCandidateID)
	}
}

// ============================================================
// Hire conversion — concurrency / FOR UPDATE lock
// ============================================================

func TestIntegration_Recruitment_HireApplication_ConcurrentCalls_OnlyOneEmployeeCreated(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	app, cand, _ := seedHiredApplication(t, env, orgID, ownerID)

	const attempts = 5
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = env.hrmRecruitmentSvc.HireApplication(ctx, orgID, app.ID, ownerID, recruitment.HireApplicationRequest{})
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly one of %d concurrent hire calls to succeed, got %d", attempts, successes)
	}

	var empCount int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_employees WHERE source_candidate_id = $1`, cand.ID).Scan(&empCount); err != nil {
		t.Fatalf("count employees: %v", err)
	}
	if empCount != 1 {
		t.Errorf("expected exactly one employee row despite %d concurrent hire attempts, got %d — the FOR UPDATE lock must have failed to serialize", attempts, empCount)
	}
}

// ============================================================
// Scorecards — unique constraint fires
// ============================================================

func TestIntegration_Recruitment_Scorecard_UniqueConstraint_FiresOnDuplicateRawInsert(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	pipeline, _ := seedRecruitmentPipeline(t, env, orgID, ownerID, recruitment.StageKindApplied)
	posting := seedRecruitmentPosting(t, env, orgID, ownerID, pipeline.ID)
	email := uniqueEmail("scorecard-candidate")
	cand, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "S", Email: &email})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	app, err := env.hrmRecruitmentSvc.CreateApplication(ctx, orgID, &ownerID, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	interview, err := env.hrmRecruitmentSvc.CreateInterview(ctx, orgID, app.ID, ownerID, recruitment.CreateInterviewRequest{ScheduledAt: "2030-01-01T10:00:00Z"})
	if err != nil {
		t.Fatalf("create interview: %v", err)
	}

	panelistEmail := uniqueEmail("panelist")
	panelist, err := env.hrmEmpSvc.Create(ctx, orgID, ownerID, hrmemployees.CreateEmployeeRequest{
		FirstName: "Pan", LastName: strPtrRecruitment("Elist"), Email: &panelistEmail, HireDate: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("create panelist employee: %v", err)
	}

	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_interview_scorecards (interview_id, panelist_employee_id) VALUES ($1, $2)`,
		interview.ID, panelist.ID,
	); err != nil {
		t.Fatalf("first raw scorecard insert: %v", err)
	}
	_, err = env.db.Exec(ctx,
		`INSERT INTO hrm_interview_scorecards (interview_id, panelist_employee_id) VALUES ($1, $2)`,
		interview.ID, panelist.ID,
	)
	if err == nil {
		t.Error("expected uq_hrm_sc_interview_panelist to reject a second raw insert for the same interview+panelist")
	}
}

// ============================================================
// Offer approval end-to-end
// ============================================================

func TestIntegration_Recruitment_OfferApproval_EndToEnd(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	_, err := env.hrmApprovalsSvc.CreateTemplate(ctx, orgID, ownerID, approvals.CreateTemplateRequest{
		Name: "Offer Approval", ActionType: approvals.ActionTypeOffer, IsDefault: true,
		Levels: []approvals.CreateTemplateLevelRequest{
			{Level: 1, ApproverType: approvals.ApproverTypeSpecificUser, ApproverUserID: &ownerID, SLAHours: 48, OnSLABreach: approvals.SLABreachEscalateNext},
		},
	})
	if err != nil {
		t.Fatalf("create approval template: %v", err)
	}

	pipeline, _ := seedRecruitmentPipeline(t, env, orgID, ownerID, recruitment.StageKindApplied)
	posting := seedRecruitmentPosting(t, env, orgID, ownerID, pipeline.ID)
	email := uniqueEmail("offer-candidate")
	cand, err := env.hrmRecruitmentSvc.CreateCandidate(ctx, orgID, &ownerID, recruitment.CreateCandidateRequest{FirstName: "O", Email: &email})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	app, err := env.hrmRecruitmentSvc.CreateApplication(ctx, orgID, &ownerID, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	// Recover the requisition ID that seedRecruitmentPosting created via the
	// posting it returned.
	fullPosting, err := env.hrmRecruitmentSvc.GetPosting(ctx, orgID, posting.ID)
	if err != nil {
		t.Fatalf("get posting: %v", err)
	}

	offer, err := env.hrmRecruitmentSvc.CreateOffer(ctx, orgID, app.ID, ownerID, recruitment.CreateOfferRequest{RequisitionID: fullPosting.RequisitionID})
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	submitted, err := env.hrmRecruitmentSvc.SubmitOffer(ctx, orgID, offer.ID, ownerID)
	if err != nil {
		t.Fatalf("submit offer: %v", err)
	}
	if submitted.Status != recruitment.OfferStatusPendingApproval {
		t.Fatalf("expected pending_approval, got %q", submitted.Status)
	}
	if submitted.ApprovalInstanceID == nil {
		t.Fatal("expected an approval_instance_id to be set")
	}

	// Decide through the REAL approvals service — exercises the
	// RegisterCallback("offer", ...) wiring set up in newTestEnv.
	if _, err := env.hrmApprovalsSvc.Decide(ctx, orgID, *submitted.ApprovalInstanceID, ownerID, approvals.DecisionRequest{Action: "approved"}); err != nil {
		t.Fatalf("decide: %v", err)
	}

	final, err := env.hrmRecruitmentSvc.GetOffer(ctx, orgID, offer.ID)
	if err != nil {
		t.Fatalf("get offer: %v", err)
	}
	if final.Status != recruitment.OfferStatusApproved {
		t.Errorf("expected the approval decision to flip the offer to approved via the callback, got %q", final.Status)
	}
}
