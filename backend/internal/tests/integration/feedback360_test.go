// backend/internal/tests/integration/feedback360_test.go
// Phase 5C 360 feedback against real Postgres — what a stub cannot prove:
// that the content query genuinely selects no identity column, that the
// partial unique indexes distinguish internal from external respondents,
// that the scope CTE resolves a real reporting tree, and that suppression
// holds against the real form engine end to end.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mridha/businesssaas/internal/authz"
	hrmfeedback "github.com/mridha/businesssaas/internal/hrm/feedback"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// fbAdmin is a coordinator at the widest tier.
func fbAdmin(userID string) hrmfeedback.Caller {
	return hrmfeedback.Caller{
		UserID: userID, Tier: authz.ScopeAll, CanCoordinate: true, CanManage: true,
	}
}

// seedFeedbackTemplate builds a one-question scored form for the cycle.
func seedFeedbackTemplate(t *testing.T, env *testEnv, orgID, userID string) *forms.Template {
	t.Helper()
	ctx := context.Background()

	tmpl, err := env.formsSvc.CreateTemplate(ctx, orgID, userID, forms.CreateTemplateRequest{
		Name: "360 " + uniqueSlug("f"), FormType: string(forms.FormTypeFeedback360),
	})
	if err != nil {
		t.Fatalf("create 360 template: %v", err)
	}
	sec, err := env.formsSvc.CreateSection(ctx, orgID, tmpl.ID, forms.CreateSectionRequest{Title: "Overall"})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	weight := formDec("100")
	if _, err := env.formsSvc.CreateQuestion(ctx, orgID, sec.ID, forms.CreateQuestionRequest{
		QuestionText: "How well do they collaborate?", QuestionType: string(forms.QuestionScale),
		ScaleMin: intPtr(1), ScaleMax: intPtr(5), Weight: &weight, IsRequired: true,
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	return tmpl
}

type feedbackFixture struct {
	orgID    string
	statusID string
	ownerID  string
	subject  string
	cycle    *hrmfeedback.Cycle
}

func seedFeedbackFixture(t *testing.T, env *testEnv, minResponses int) *feedbackFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	subject := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Subject", nil)
	tmpl := seedFeedbackTemplate(t, env, orgID, ownerID)

	cycle, err := env.hrmFeedbackSvc.CreateCycle(ctx, orgID, ownerID, hrmfeedback.CreateCycleRequest{
		Name: "360 " + uniqueSlug("c"), PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		FormTemplateID: tmpl.ID, MinResponses: &minResponses,
	})
	if err != nil {
		t.Fatalf("create feedback cycle: %v", err)
	}
	if _, err := env.hrmFeedbackSvc.ActivateCycle(ctx, orgID, cycle.ID); err != nil {
		t.Fatalf("activate cycle: %v", err)
	}
	return &feedbackFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, subject: subject, cycle: cycle}
}

// askPeers creates n peer requests and answers+submits each through the real
// form engine, so the aggregate reads genuine scored responses.
func askPeers(t *testing.T, env *testEnv, fx *feedbackFixture, rel hrmfeedback.Relationship, n int) []string {
	t.Helper()
	ctx := context.Background()

	specs := make([]hrmfeedback.RespondentSpec, 0, n)
	empIDs := make([]string, 0, n)
	for i := 0; i < n; i++ {
		emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Respondent", nil)
		empIDs = append(empIDs, emp)
		e := emp
		specs = append(specs, hrmfeedback.RespondentSpec{EmployeeID: &e, Relationship: rel})
	}
	if _, err := env.hrmFeedbackSvc.CreateRequests(ctx, fx.orgID, fx.cycle.ID, fx.ownerID,
		hrmfeedback.CreateRequestsRequest{SubjectEmployeeID: fx.subject, Respondents: specs}); err != nil {
		t.Fatalf("create %d %s requests: %v", n, rel, err)
	}

	// Answer and submit each form, then mark the request submitted. The
	// respondents have no platform accounts, so the coordinator fills them in
	// — which is exactly the "external respondent" path the schema allows.
	rows, err := env.db.Query(ctx,
		`SELECT id, form_instance_id FROM hrm_feedback_requests
		  WHERE cycle_id = $1 AND subject_employee_id = $2 AND relationship = $3 AND status = 'pending'`,
		fx.cycle.ID, fx.subject, rel)
	if err != nil {
		t.Fatalf("load requests: %v", err)
	}
	type pair struct{ reqID, instID string }
	pairs := []pair{}
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.reqID, &p.instID); err != nil {
			rows.Close()
			t.Fatalf("scan: %v", err)
		}
		pairs = append(pairs, p)
	}
	rows.Close()

	for _, p := range pairs {
		answerAndSubmit(t, env, fx.orgID, p.instID, fx.ownerID, "4")
		if _, err := env.db.Exec(ctx,
			`UPDATE hrm_feedback_requests SET status='submitted', submitted_at=NOW() WHERE id=$1`, p.reqID); err != nil {
			t.Fatalf("mark submitted: %v", err)
		}
	}
	return empIDs
}

// ============================================================
// The anonymity boundary, against the real query
// ============================================================

// TestIntegration_Feedback_AggregateCarriesNoIdentity is the headline test.
// The unit tests prove the TYPE cannot name anyone; this proves the QUERY
// behind it does not fetch identity in the first place, and that the
// end-to-end aggregate a subject receives is free of it.
func TestIntegration_Feedback_AggregateCarriesNoIdentity(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFeedbackFixture(t, env, 3)

	respondents := askPeers(t, env, fx, hrmfeedback.RelationshipPeer, 3)

	agg, err := env.hrmFeedbackSvc.GetAggregate(ctx, fx.orgID, fx.cycle.ID, fx.subject, fbAdmin(fx.ownerID))
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(agg.Groups) != 1 || agg.Groups[0].Suppressed {
		t.Fatalf("expected one rendered peer group, got %+v", agg.Groups)
	}
	if agg.Groups[0].ResponseCount != 3 {
		t.Errorf("expected 3 responses, got %d", agg.Groups[0].ResponseCount)
	}

	// Serialise the whole aggregate and prove no respondent id appears
	// anywhere in it — the check that catches a leak through a field nobody
	// thought about.
	blob := mustJSON(t, agg)
	for _, empID := range respondents {
		if containsStr(blob, empID) {
			t.Errorf("aggregate leaked respondent employee id %s", empID)
		}
	}

	// And no form instance id: an instance id plus GET /forms/instances/:id
	// names the respondent, which is the leak that lives outside this module.
	var instIDs []string
	rows, err := env.db.Query(ctx,
		`SELECT form_instance_id::text FROM hrm_feedback_requests
		  WHERE cycle_id = $1 AND form_instance_id IS NOT NULL`, fx.cycle.ID)
	if err != nil {
		t.Fatalf("load instance ids: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		instIDs = append(instIDs, id)
	}
	for _, id := range instIDs {
		if containsStr(blob, id) {
			t.Errorf("aggregate leaked form instance id %s — a subject can read respondent_user_id from it", id)
		}
	}

	// The content is genuinely there, so the test is not passing vacuously.
	if len(agg.Groups[0].Responses) != 3 {
		t.Fatalf("expected 3 response bodies, got %d", len(agg.Groups[0].Responses))
	}
	if len(agg.Groups[0].Responses[0].Answers) == 0 {
		t.Error("responses carry no answers — the aggregate is empty, so the identity check proved nothing")
	}
	if agg.Groups[0].AverageScore == nil || !agg.Groups[0].AverageScore.Equal(perfDec("75")) {
		t.Errorf("expected average score 75 (4 of 1..5), got %v", agg.Groups[0].AverageScore)
	}
}

// TestIntegration_Feedback_SuppressionHoldsEndToEnd proves the threshold is
// applied against real submitted forms, not just stub counts.
func TestIntegration_Feedback_SuppressionHoldsEndToEnd(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFeedbackFixture(t, env, 3)

	askPeers(t, env, fx, hrmfeedback.RelationshipPeer, 2)

	agg, err := env.hrmFeedbackSvc.GetAggregate(ctx, fx.orgID, fx.cycle.ID, fx.subject, fbAdmin(fx.ownerID))
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	g := agg.Groups[0]
	if !g.Suppressed {
		t.Fatal("2 of 3 must be suppressed")
	}
	if g.ResponseCount != 0 || len(g.Responses) != 0 || g.AverageScore != nil {
		t.Errorf("a suppressed group leaked content: count=%d responses=%d score=%v",
			g.ResponseCount, len(g.Responses), g.AverageScore)
	}
	if agg.TotalResponses != 0 {
		t.Errorf("TotalResponses must exclude suppressed groups, got %d", agg.TotalResponses)
	}

	// One more response crosses the line.
	askPeers(t, env, fx, hrmfeedback.RelationshipPeer, 1)
	agg, err = env.hrmFeedbackSvc.GetAggregate(ctx, fx.orgID, fx.cycle.ID, fx.subject, fbAdmin(fx.ownerID))
	if err != nil {
		t.Fatalf("aggregate after third: %v", err)
	}
	if agg.Groups[0].Suppressed {
		t.Fatal("3 of 3 must render")
	}
}

// ============================================================
// The coordination path, against the real query
// ============================================================

// TestIntegration_Feedback_CoordinationNamesRespondentsAndNothingElse pins
// the other half of the split: the identity query returns names and status
// and no answer content at all.
func TestIntegration_Feedback_CoordinationNamesRespondents(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFeedbackFixture(t, env, 1)
	askPeers(t, env, fx, hrmfeedback.RelationshipPeer, 2)

	res, err := env.hrmFeedbackSvc.ListRequests(ctx, fx.orgID, fbAdmin(fx.ownerID),
		hrmfeedback.RequestListFilter{CycleID: fx.cycle.ID})
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("expected 2 requests, got %d", res.Total)
	}
	for _, r := range res.Requests {
		if r.RespondentName == "" {
			t.Error("the coordination view exists to name who was asked")
		}
		if r.Status != hrmfeedback.RequestSubmitted {
			t.Errorf("expected submitted, got %s", r.Status)
		}
	}

	// No answer text appears anywhere in the coordination payload.
	blob := mustJSON(t, res)
	if containsStr(blob, "collaborate") {
		t.Error("the coordination view leaked question or answer content")
	}
}

// ============================================================
// The partial unique indexes
// ============================================================

// TestIntegration_Feedback_DuplicateRespondentBlocked exercises both partial
// indexes: internal respondents keyed by employee id, external ones by email.
// A single index over COALESCE of the two would collide every external
// respondent onto one key, which is why there are two.
func TestIntegration_Feedback_DuplicateRespondentBlocked(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFeedbackFixture(t, env, 1)

	internal := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Internal", nil)
	emailA, nameA := "clienta@example.com", "Client A"
	emailB, nameB := "clientb@example.com", "Client B"

	ask := func(specs ...hrmfeedback.RespondentSpec) error {
		_, err := env.hrmFeedbackSvc.CreateRequests(ctx, fx.orgID, fx.cycle.ID, fx.ownerID,
			hrmfeedback.CreateRequestsRequest{SubjectEmployeeID: fx.subject, Respondents: specs})
		return err
	}

	if err := ask(
		hrmfeedback.RespondentSpec{EmployeeID: &internal, Relationship: hrmfeedback.RelationshipPeer},
		hrmfeedback.RespondentSpec{Email: &emailA, Name: &nameA, Relationship: hrmfeedback.RelationshipExternal},
		hrmfeedback.RespondentSpec{Email: &emailB, Name: &nameB, Relationship: hrmfeedback.RelationshipExternal},
	); err != nil {
		t.Fatalf("first batch: %v", err)
	}

	// Two DISTINCT external respondents must both have landed — this is what
	// a COALESCE-based single index would have broken.
	var externals int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_feedback_requests
		  WHERE cycle_id = $1 AND respondent_employee_id IS NULL`, fx.cycle.ID).Scan(&externals); err != nil {
		t.Fatalf("count externals: %v", err)
	}
	if externals != 2 {
		t.Errorf("expected 2 distinct external respondents, got %d", externals)
	}

	if err := ask(hrmfeedback.RespondentSpec{EmployeeID: &internal, Relationship: hrmfeedback.RelationshipPeer}); !errors.Is(err, hrmfeedback.ErrDuplicateRequest) {
		t.Errorf("expected ErrDuplicateRequest for a repeat internal respondent, got %v", err)
	}
	if err := ask(hrmfeedback.RespondentSpec{Email: &emailA, Name: &nameA, Relationship: hrmfeedback.RelationshipExternal}); !errors.Is(err, hrmfeedback.ErrDuplicateRequest) {
		t.Errorf("expected ErrDuplicateRequest for a repeat external respondent, got %v", err)
	}
}

// TestIntegration_Feedback_SelfCheckConstraint proves the DB refuses a
// mislabelled self relationship even if the service check were bypassed — a
// self response treated as attributed when it is really a peer's would
// deanonymise that peer.
func TestIntegration_Feedback_SelfCheckConstraint(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFeedbackFixture(t, env, 1)
	other := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Not The Subject", nil)

	_, err := env.db.Exec(ctx,
		`INSERT INTO hrm_feedback_requests
		    (org_id, cycle_id, subject_employee_id, respondent_employee_id,
		     respondent_name, relationship, requested_by)
		 VALUES ($1,$2,$3,$4,'Mislabelled','self',$5)`,
		fx.orgID, fx.cycle.ID, fx.subject, other, fx.ownerID)
	if err == nil {
		t.Fatal("chk_hrm_fbr_self accepted a self relationship whose respondent is not the subject")
	}
}

// ============================================================
// Scope tiers over a real reporting tree
// ============================================================

func TestIntegration_Feedback_ScopeTiers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	managerEmp := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Manager", nil)
	reportEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Report", &managerEmp)
	strangerEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Stranger", nil)

	tmpl := seedFeedbackTemplate(t, env, orgID, ownerID)
	min := 1
	cycle, err := env.hrmFeedbackSvc.CreateCycle(ctx, orgID, ownerID, hrmfeedback.CreateCycleRequest{
		Name: "Scoped 360 " + uniqueSlug("c"), PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		FormTemplateID: tmpl.ID, MinResponses: &min,
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := env.hrmFeedbackSvc.ActivateCycle(ctx, orgID, cycle.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// One peer ask about each of the three subjects.
	for _, subj := range []string{managerEmp, reportEmp, strangerEmp} {
		peer := seedEmployee(t, env, orgID, statusID, ownerID, "", "Peer", nil)
		p := peer
		if _, err := env.hrmFeedbackSvc.CreateRequests(ctx, orgID, cycle.ID, ownerID,
			hrmfeedback.CreateRequestsRequest{
				SubjectEmployeeID: subj,
				Respondents:       []hrmfeedback.RespondentSpec{{EmployeeID: &p, Relationship: hrmfeedback.RelationshipPeer}},
			}); err != nil {
			t.Fatalf("ask about %s: %v", subj, err)
		}
	}

	cases := []struct {
		tier authz.Scope
		want int
	}{
		{authz.ScopeOwn, 1},  // asks about the manager themselves
		{authz.ScopeTeam, 2}, // own + direct report
		{authz.ScopeAll, 3},  // the whole org
	}
	for _, tc := range cases {
		caller := hrmfeedback.Caller{UserID: ownerID, Tier: tc.tier, CanCoordinate: true}
		res, err := env.hrmFeedbackSvc.ListRequests(ctx, orgID, caller,
			hrmfeedback.RequestListFilter{CycleID: cycle.ID})
		if err != nil {
			t.Fatalf("list at tier %v: %v", tc.tier, err)
		}
		if res.Total != tc.want {
			t.Errorf("tier %v: expected %d requests, got %d", tc.tier, tc.want, res.Total)
		}
	}

	// The content path narrows the same way.
	own := hrmfeedback.Caller{UserID: ownerID, Tier: authz.ScopeOwn}
	if _, err := env.hrmFeedbackSvc.GetAggregate(ctx, orgID, cycle.ID, strangerEmp, own); !errors.Is(err, hrmfeedback.ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied reading a stranger's feedback at view_own, got %v", err)
	}
}

// ============================================================
// Tenant isolation
// ============================================================

func TestIntegration_Feedback_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fxA := seedFeedbackFixture(t, env, 1)
	fxB := seedFeedbackFixture(t, env, 1)

	if _, err := env.hrmFeedbackSvc.GetCycle(ctx, fxB.orgID, fxA.cycle.ID); !errors.Is(err, hrmfeedback.ErrCycleNotFound) {
		t.Errorf("org B reached org A's feedback cycle: %v", err)
	}
	if _, err := env.hrmFeedbackSvc.GetAggregate(ctx, fxB.orgID, fxA.cycle.ID, fxA.subject,
		fbAdmin(fxB.ownerID)); !errors.Is(err, hrmfeedback.ErrCycleNotFound) {
		t.Errorf("org B read an aggregate from org A's cycle: %v", err)
	}
}

// mustJSON serialises a value so a test can assert an identifier appears
// nowhere in the payload — the check that catches a leak through a field
// nobody thought to look at.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func containsStr(haystack, needle string) bool {
	return needle != "" && strings.Contains(haystack, needle)
}
