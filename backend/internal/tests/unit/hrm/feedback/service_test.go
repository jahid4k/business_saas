// backend/internal/tests/unit/hrm/feedback/service_test.go
// Phase 5C 360 feedback: the anonymity contract, per-group suppression, and
// the coordination/content split.
package feedback_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/feedback"
)

func newSvc(allow bool) (feedback.Service, *stubRepo, *stubForms) {
	repo := newStubRepo()
	repo.employees[subjectEmp] = &feedback.EmployeeSubject{
		EmployeeID: subjectEmp, DisplayName: "Subject Person", UserID: strPtr(ownerUserID),
	}
	repo.employees[otherEmp] = &feedback.EmployeeSubject{
		EmployeeID: otherEmp, DisplayName: "Other Person", UserID: strPtr(otherUserID),
	}
	repo.employeeUsers[ownerUserID] = subjectEmp
	fs := newStubForms()
	return feedback.NewService(repo, &stubAuthorizer{allow: allow}, fs), repo, fs
}

func adminCaller() feedback.Caller {
	return feedback.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanCoordinate: true, CanManage: true}
}

// seedCycle creates an ACTIVE cycle with the given suppression threshold.
func seedCycle(t *testing.T, svc feedback.Service, minResponses int) *feedback.Cycle {
	t.Helper()
	ctx := context.Background()
	c, err := svc.CreateCycle(ctx, testOrg, ownerUserID, feedback.CreateCycleRequest{
		Name: "FY30 360 " + strings.Repeat("x", minResponses), PeriodStart: "2030-01-01",
		PeriodEnd: "2030-12-31", FormTemplateID: templateID, MinResponses: &minResponses,
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := svc.ActivateCycle(ctx, testOrg, c.ID); err != nil {
		t.Fatalf("activate cycle: %v", err)
	}
	return c
}

// askAndSubmit creates n requests in one relationship group and submits them
// all, which is what the suppression threshold counts.
func askAndSubmit(t *testing.T, svc feedback.Service, repo *stubRepo, cycleID string, rel feedback.Relationship, n int) {
	t.Helper()
	ctx := context.Background()

	specs := make([]feedback.RespondentSpec, 0, n)
	for i := 0; i < n; i++ {
		empID := repo.nextID("emp_resp")
		repo.employees[empID] = &feedback.EmployeeSubject{
			EmployeeID: empID, DisplayName: "Respondent " + empID, UserID: strPtr("usr_" + empID),
		}
		specs = append(specs, feedback.RespondentSpec{EmployeeID: &empID, Relationship: rel})
	}
	if _, err := svc.CreateRequests(ctx, testOrg, cycleID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp, Respondents: specs,
	}); err != nil {
		t.Fatalf("create %d %s requests: %v", n, rel, err)
	}

	for _, q := range repo.requests {
		if q.CycleID == cycleID && q.Relationship == rel && q.Status == feedback.RequestPending {
			if _, err := repo.SetRequestSubmitted(ctx, testOrg, q.ID); err != nil {
				t.Fatalf("submit: %v", err)
			}
		}
	}
}

// ============================================================
// The anonymity policy is derived, not stored
// ============================================================

// TestRelationship_IsAnonymous pins the single source of truth. Every
// suppression decision reads this, so it gets tested directly rather than
// only through its effects.
func TestRelationship_IsAnonymous(t *testing.T) {
	attributed := []feedback.Relationship{feedback.RelationshipSelf, feedback.RelationshipManager}
	for _, rel := range attributed {
		if rel.IsAnonymous() {
			t.Errorf("%s must be attributed by nature: a subject knows what they wrote and who their manager is", rel)
		}
	}
	anonymous := []feedback.Relationship{
		feedback.RelationshipPeer, feedback.RelationshipDirectReport, feedback.RelationshipExternal,
	}
	for _, rel := range anonymous {
		if !rel.IsAnonymous() {
			t.Errorf("%s must be anonymous", rel)
		}
	}
}

// TestAnonymousResponse_CarriesNoIdentity asserts the TYPE's shape, not a
// value's. This is the structural guard: if someone later "simplifies"
// AnonymousResponse into a *Request with fields blanked, or adds a
// respondent field, this fails loudly rather than silently leaking.
func TestAnonymousResponse_CarriesNoIdentity(t *testing.T) {
	forbidden := []string{
		"respondent", "user", "email", "name", "employee",
		"forminstance", "instance", "submittedat", "id",
	}
	assertNoForbiddenFields(t, reflect.TypeOf(feedback.AnonymousResponse{}), forbidden)
	assertNoForbiddenFields(t, reflect.TypeOf(feedback.AnonymousAnswer{}), forbidden)
}

func assertNoForbiddenFields(t *testing.T, typ reflect.Type, forbidden []string) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("%s.%s can name a respondent — this type must carry content only",
					typ.Name(), typ.Field(i).Name)
			}
		}
	}
}

// TestSubmittedRef_FormInstanceIDNotSerialisable pins the one field capable
// of defeating anonymity from outside this package: an instance id plus
// GET /forms/instances/:id names the respondent.
func TestSubmittedRef_FormInstanceIDNotSerialisable(t *testing.T) {
	f, ok := reflect.TypeOf(feedback.SubmittedRef{}).FieldByName("FormInstanceID")
	if !ok {
		t.Fatal("SubmittedRef.FormInstanceID is gone — update this test with the replacement")
	}
	if f.Tag.Get("json") != "-" {
		t.Errorf(`SubmittedRef.FormInstanceID must carry json:"-"; got %q`, f.Tag.Get("json"))
	}
}

// ============================================================
// Suppression, per relationship group
// ============================================================

// TestAggregate_SuppressesBelowThreshold is the headline rule.
func TestAggregate_SuppressesBelowThreshold(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	cycle := seedCycle(t, svc, 3)

	// Two peers: one short of the threshold.
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipPeer, 2)

	agg, err := svc.GetAggregate(ctx, testOrg, cycle.ID, subjectEmp, adminCaller())
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(agg.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(agg.Groups))
	}

	g := agg.Groups[0]
	if !g.Suppressed {
		t.Fatal("2 responses under a threshold of 3 must be suppressed")
	}
	// A suppressed group leaks NOTHING — not even its size. "One peer
	// responded but it is hidden" plus any knowledge of who was asked
	// narrows to a person.
	if g.ResponseCount != 0 {
		t.Errorf("a suppressed group must not report its count, got %d", g.ResponseCount)
	}
	if len(g.Responses) != 0 {
		t.Errorf("a suppressed group must carry no responses, got %d", len(g.Responses))
	}
	if g.AverageScore != nil {
		t.Errorf("a suppressed group must carry no score, got %v", g.AverageScore)
	}
	if g.MinResponses != 3 {
		t.Errorf("the threshold should be echoed so a client can explain the suppression, got %d", g.MinResponses)
	}
	// TotalResponses must exclude suppressed groups, or it can be differenced
	// against the visible ones to recover the hidden count.
	if agg.TotalResponses != 0 {
		t.Errorf("TotalResponses must exclude suppressed groups, got %d", agg.TotalResponses)
	}
}

func TestAggregate_RendersAtThreshold(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	cycle := seedCycle(t, svc, 3)
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipPeer, 3)

	agg, err := svc.GetAggregate(ctx, testOrg, cycle.ID, subjectEmp, adminCaller())
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	g := agg.Groups[0]
	if g.Suppressed {
		t.Fatal("3 responses at a threshold of 3 must render")
	}
	if g.ResponseCount != 3 {
		t.Errorf("expected 3 responses, got %d", g.ResponseCount)
	}
	if len(g.Responses) != 3 {
		t.Errorf("expected 3 response bodies, got %d", len(g.Responses))
	}
	if g.AverageScore == nil || !g.AverageScore.Equal(perfDec("80")) {
		t.Errorf("expected average score 80, got %v", g.AverageScore)
	}
	if agg.TotalResponses != 3 {
		t.Errorf("expected TotalResponses 3, got %d", agg.TotalResponses)
	}
}

// TestAggregate_ManagerGroupNeverSuppressed pins the attributed-by-nature
// rule. There is exactly one manager, so a threshold of 3 could never be met
// — suppressing it would make the most actionable feedback in the cycle
// permanently unreadable while adding no privacy.
func TestAggregate_ManagerGroupNeverSuppressed(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	cycle := seedCycle(t, svc, 3)
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipManager, 1)

	agg, err := svc.GetAggregate(ctx, testOrg, cycle.ID, subjectEmp, adminCaller())
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	g := agg.Groups[0]
	if g.Suppressed {
		t.Fatal("manager feedback is attributed by nature and must never be suppressed")
	}
	if len(g.Responses) != 1 {
		t.Errorf("expected the single manager response to render, got %d", len(g.Responses))
	}
}

// TestAggregate_SuppressionAppliesToViewAll is the one people get wrong. An
// "admin sees everything" exception would make the promise to respondents
// false — and a promise of anonymity that is false for one role is false.
func TestAggregate_SuppressionAppliesToViewAll(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	cycle := seedCycle(t, svc, 3)
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipPeer, 1)

	admin := feedback.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanCoordinate: true, CanManage: true}
	agg, err := svc.GetAggregate(ctx, testOrg, cycle.ID, subjectEmp, admin)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if !agg.Groups[0].Suppressed {
		t.Fatal("view_all must not bypass suppression — anonymity is not a scope tier")
	}
}

// TestAggregate_SuppressionIsPerGroupNotCycleWide is the subtle one. Five
// total responses clears a cycle-wide threshold of 3, but the lone direct
// report is still individually identifiable the moment their group renders.
func TestAggregate_SuppressionIsPerGroupNotCycleWide(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	cycle := seedCycle(t, svc, 3)
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipPeer, 4)
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipDirectReport, 1)

	agg, err := svc.GetAggregate(ctx, testOrg, cycle.ID, subjectEmp, adminCaller())
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	byRel := map[feedback.Relationship]feedback.RelationshipGroup{}
	for _, g := range agg.Groups {
		byRel[g.Relationship] = g
	}
	if byRel[feedback.RelationshipPeer].Suppressed {
		t.Error("4 peers clears the threshold and must render")
	}
	if !byRel[feedback.RelationshipDirectReport].Suppressed {
		t.Error("a lone direct report must be suppressed even though the cycle has 5 responses overall")
	}
	if agg.TotalResponses != 4 {
		t.Errorf("TotalResponses must count only rendered groups, got %d", agg.TotalResponses)
	}
}

// ============================================================
// The coordination / content split
// ============================================================

// TestListRequests_RequiresCoordinate proves the service enforces the split
// itself rather than trusting the route gate.
func TestListRequests_RequiresCoordinate(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	cycle := seedCycle(t, svc, 1)
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipPeer, 1)

	viewer := feedback.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanCoordinate: false}
	if _, err := svc.ListRequests(ctx, testOrg, viewer, feedback.RequestListFilter{CycleID: cycle.ID}); !errors.Is(err, feedback.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied without coordinate, got %v", err)
	}

	coordinator := adminCaller()
	res, err := svc.ListRequests(ctx, testOrg, coordinator, feedback.RequestListFilter{CycleID: cycle.ID})
	if err != nil {
		t.Fatalf("coordinator list: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 request, got %d", res.Total)
	}
	// The coordination view names the respondent and carries no content —
	// RequestSummary has no answer field at all, which is the point.
	if res.Requests[0].RespondentName == "" {
		t.Error("the coordination view exists to name who was asked")
	}
}

// TestRequestSummary_CarriesNoContent is the mirror of the AnonymousResponse
// shape test: the identity-bearing type must never gain an answer field.
func TestRequestSummary_CarriesNoContent(t *testing.T) {
	forbidden := []string{"answer", "response", "score", "comment", "feedback", "forminstance"}
	assertNoForbiddenFields(t, reflect.TypeOf(feedback.RequestSummary{}), forbidden)
}

// ============================================================
// Request creation rules
// ============================================================

// TestCreateRequests_SubjectAndRespondentDoNotSwap pins the form-engine seam.
// Passing the respondent as the subject would file every response under the
// wrong person, and the form engine keeps them separate precisely so that
// cannot happen by accident.
func TestCreateRequests_SubjectAndRespondentDoNotSwap(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	cycle := seedCycle(t, svc, 1)

	respEmp := otherEmp
	if _, err := svc.CreateRequests(ctx, testOrg, cycle.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp,
		Respondents:       []feedback.RespondentSpec{{EmployeeID: &respEmp, Relationship: feedback.RelationshipPeer}},
	}); err != nil {
		t.Fatalf("create requests: %v", err)
	}
	_ = repo

	if len(fs.instantiated) != 1 {
		t.Fatalf("expected 1 form instantiation, got %d", len(fs.instantiated))
	}
	subj := fs.instantiated[0]
	if subj.SubjectID != subjectEmp {
		t.Errorf("form subject = %s, want the person being reviewed (%s)", subj.SubjectID, subjectEmp)
	}
	if subj.RespondentUserID == nil || *subj.RespondentUserID != otherUserID {
		t.Errorf("form respondent = %v, want the person answering (%s)", subj.RespondentUserID, otherUserID)
	}
	if subj.RespondentRole != string(feedback.RelationshipPeer) {
		t.Errorf("respondent role = %s, want peer", subj.RespondentRole)
	}
}

func TestCreateRequests_SelfMustBeTheSubject(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	cycle := seedCycle(t, svc, 1)

	wrong := otherEmp
	_, err := svc.CreateRequests(ctx, testOrg, cycle.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp,
		Respondents:       []feedback.RespondentSpec{{EmployeeID: &wrong, Relationship: feedback.RelationshipSelf}},
	})
	if !errors.Is(err, feedback.ErrSelfMismatch) {
		t.Fatalf("expected ErrSelfMismatch, got %v", err)
	}
}

func TestCreateRequests_RejectsDuplicateRespondent(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	cycle := seedCycle(t, svc, 1)

	respEmp := otherEmp
	spec := feedback.RespondentSpec{EmployeeID: &respEmp, Relationship: feedback.RelationshipPeer}
	if _, err := svc.CreateRequests(ctx, testOrg, cycle.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp, Respondents: []feedback.RespondentSpec{spec},
	}); err != nil {
		t.Fatalf("first ask: %v", err)
	}
	_, err := svc.CreateRequests(ctx, testOrg, cycle.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp, Respondents: []feedback.RespondentSpec{spec},
	})
	if !errors.Is(err, feedback.ErrDuplicateRequest) {
		t.Fatalf("expected ErrDuplicateRequest — one respondent must not count twice toward a threshold that requires distinct people; got %v", err)
	}
}

func TestCreateRequests_ExternalRespondentNeedsEmailAndName(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	cycle := seedCycle(t, svc, 1)

	_, err := svc.CreateRequests(ctx, testOrg, cycle.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp,
		Respondents:       []feedback.RespondentSpec{{Relationship: feedback.RelationshipExternal}},
	})
	if !errors.Is(err, feedback.ErrRespondentRequired) {
		t.Fatalf("expected ErrRespondentRequired, got %v", err)
	}

	email, name := "client@example.com", "External Client"
	if _, err := svc.CreateRequests(ctx, testOrg, cycle.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp,
		Respondents: []feedback.RespondentSpec{
			{Email: &email, Name: &name, Relationship: feedback.RelationshipExternal},
		},
	}); err != nil {
		t.Fatalf("external respondent with email and name should be accepted: %v", err)
	}
}

func TestCreateRequests_RejectsInactiveCycle(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, err := svc.CreateCycle(ctx, testOrg, ownerUserID, feedback.CreateCycleRequest{
		Name: "Draft cycle", PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		FormTemplateID: templateID,
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}

	respEmp := otherEmp
	_, err = svc.CreateRequests(ctx, testOrg, c.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp,
		Respondents:       []feedback.RespondentSpec{{EmployeeID: &respEmp, Relationship: feedback.RelationshipPeer}},
	})
	if !errors.Is(err, feedback.ErrCycleNotActive) {
		t.Fatalf("expected ErrCycleNotActive, got %v", err)
	}
}

// ============================================================
// Responding
// ============================================================

// TestSubmitResponse_OnlyTheRespondent pins the narrowing the route gate
// cannot express: hrm.feedback.respond reaches every member, so "is this
// YOUR request" is decided here.
func TestSubmitResponse_OnlyTheRespondent(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	cycle := seedCycle(t, svc, 1)

	respEmp := otherEmp
	if _, err := svc.CreateRequests(ctx, testOrg, cycle.ID, ownerUserID, feedback.CreateRequestsRequest{
		SubjectEmployeeID: subjectEmp,
		Respondents:       []feedback.RespondentSpec{{EmployeeID: &respEmp, Relationship: feedback.RelationshipPeer}},
	}); err != nil {
		t.Fatalf("create requests: %v", err)
	}
	var reqID string
	for id := range repo.requests {
		reqID = id
	}

	wrongCaller := feedback.Caller{UserID: ownerUserID, Tier: authz.ScopeAll}
	if _, err := svc.SubmitResponse(ctx, testOrg, reqID, wrongCaller); !errors.Is(err, feedback.ErrNotRespondent) {
		t.Fatalf("expected ErrNotRespondent for someone else's request, got %v", err)
	}

	rightCaller := feedback.Caller{UserID: otherUserID, Tier: authz.ScopeAll}
	if _, err := svc.SubmitResponse(ctx, testOrg, reqID, rightCaller); err != nil {
		t.Fatalf("the actual respondent should be able to submit: %v", err)
	}
	if _, err := svc.SubmitResponse(ctx, testOrg, reqID, rightCaller); !errors.Is(err, feedback.ErrAlreadySubmitted) {
		t.Fatalf("expected ErrAlreadySubmitted on the second submit, got %v", err)
	}
}

// ============================================================
// Scope and cycle lifecycle
// ============================================================

func TestGetAggregate_DeniesOutOfScope(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(false) // authorizer refuses anything below ScopeAll
	cycle := seedCycle(t, svc, 1)
	askAndSubmit(t, svc, repo, cycle.ID, feedback.RelationshipPeer, 1)

	outsider := feedback.Caller{UserID: otherUserID, Tier: authz.ScopeOwn}
	if _, err := svc.GetAggregate(ctx, testOrg, cycle.ID, subjectEmp, outsider); !errors.Is(err, feedback.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

// TestUpdateCycle_ThresholdFrozenAfterClose stops a closed cycle's promise
// being weakened retroactively: lowering min_responses would unsuppress
// groups respondents answered under a stricter one.
func TestUpdateCycle_ThresholdFrozenAfterClose(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	cycle := seedCycle(t, svc, 5)
	if _, err := svc.CloseCycle(ctx, testOrg, cycle.ID); err != nil {
		t.Fatalf("close: %v", err)
	}

	lower := 1
	_, err := svc.UpdateCycle(ctx, testOrg, cycle.ID, feedback.UpdateCycleRequest{MinResponses: &lower})
	if !errors.Is(err, feedback.ErrCycleClosed) {
		t.Fatalf("expected ErrCycleClosed — a closed cycle's threshold is frozen; got %v", err)
	}
}

func TestCreateCycle_RejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)

	cases := []struct {
		name string
		req  feedback.CreateCycleRequest
		want error
	}{
		{"no name", feedback.CreateCycleRequest{
			PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31", FormTemplateID: templateID,
		}, feedback.ErrCycleNameRequired},
		{"no template", feedback.CreateCycleRequest{
			Name: "X", PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		}, feedback.ErrTemplateRequired},
		{"backwards period", feedback.CreateCycleRequest{
			Name: "Y", PeriodStart: "2030-12-31", PeriodEnd: "2030-01-01", FormTemplateID: templateID,
		}, feedback.ErrInvalidPeriod},
		{"zero threshold", feedback.CreateCycleRequest{
			Name: "Z", PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
			FormTemplateID: templateID, MinResponses: intPtr(0),
		}, feedback.ErrMinResponses},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreateCycle(ctx, testOrg, ownerUserID, tc.req); !errors.Is(err, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func intPtr(i int) *int { return &i }

func perfDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal literal in test: " + s)
	}
	return d
}
