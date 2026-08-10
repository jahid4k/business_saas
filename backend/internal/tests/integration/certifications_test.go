// backend/internal/tests/integration/certifications_test.go
// Phase 6B certifications, the expiry sweep, and the skills taxonomy against
// real Postgres — what a stub cannot prove: that the sweep's date boundary is
// correct against CURRENT_DATE, that it does not re-notify, that the
// acknowledgement CHECK really accepts 'course_completion', and that the
// partial unique index frees a revoked credential for re-issue.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
	hrmcertifications "github.com/mridha/businesssaas/internal/hrm/certifications"
	hrmskills "github.com/mridha/businesssaas/internal/hrm/skills"
)

func certAdmin(userID string) hrmcertifications.Caller {
	return hrmcertifications.Caller{UserID: userID, Tier: authz.ScopeAll, CanManage: true}
}

func skillAdmin(userID string) hrmskills.Caller {
	return hrmskills.Caller{UserID: userID, Tier: authz.ScopeAll, CanManage: true}
}

type certFixture struct {
	orgID    string
	statusID string
	ownerID  string
	empID    string
	skill    *hrmskills.Skill
	cert     *hrmcertifications.Certification
}

func seedCertFixture(t *testing.T, env *testEnv) *certFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Holder", nil)

	sk, err := env.hrmSkillsSvc.CreateSkill(ctx, orgID, ownerID, hrmskills.CreateSkillRequest{
		Name: "First Aid " + uniqueSlug("s"),
	})
	if err != nil {
		t.Fatalf("create skill: %v", err)
	}
	months := 12
	cert, err := env.hrmCertSvc.Create(ctx, orgID, ownerID, hrmcertifications.CreateCertificationRequest{
		Name: "First Aid Certificate " + uniqueSlug("c"), ValidityMonths: &months, SkillID: &sk.ID,
	})
	if err != nil {
		t.Fatalf("create certification: %v", err)
	}
	return &certFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, empID: empID, skill: sk, cert: cert}
}

// issueWithExpiry issues a credential expiring `daysFromNow` days out. Negative
// values are in the past, which is how the sweep's boundaries get exercised.
func issueWithExpiry(t *testing.T, env *testEnv, fx *certFixture, empID string, daysFromNow int) string {
	t.Helper()
	ctx := context.Background()
	expires := time.Now().AddDate(0, 0, daysFromNow).Format("2006-01-02")
	issued := time.Now().AddDate(0, 0, -365).Format("2006-01-02")

	ec, err := env.hrmCertSvc.Issue(ctx, fx.orgID, certAdmin(fx.ownerID), hrmcertifications.IssueRequest{
		EmployeeID: empID, CertificationID: fx.cert.ID,
		IssuedOn: issued, ExpiresAt: &expires,
	})
	if err != nil {
		t.Fatalf("issue (expiry %+d days): %v", daysFromNow, err)
	}
	return ec.ID
}

func statusOf(t *testing.T, env *testEnv, id string) string {
	t.Helper()
	var s string
	if err := env.db.QueryRow(context.Background(),
		`SELECT status FROM hrm_employee_certifications WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return s
}

// ============================================================
// The expiry sweep — the highest-value feature in the phase
// ============================================================

// TestIntegration_Certifications_ExpirySweepBoundaries is the headline test.
//
// The boundary is the whole thing: a credential expiring TODAY is still valid
// today, so the sweep must use `expires_at < CURRENT_DATE` for expiry, not
// `<=`. For a safety certification, cutting somebody off a day early is a real
// operational error.
func TestIntegration_Certifications_ExpirySweepBoundaries(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCertFixture(t, env)

	// One employee per credential — the partial unique index allows only one
	// live credential per (employee, certification) pair.
	mk := func(name string, days int) string {
		emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", name, nil)
		return issueWithExpiry(t, env, fx, emp, days)
	}

	farFuture := mk("Far", 200) // well outside the 30-day window
	soon := mk("Soon", 10)      // inside the window
	edgeIn := mk("EdgeIn", 30)  // exactly at the window edge
	today := mk("Today", 0)     // expires TODAY — still valid
	yesterday := mk("Past", -1) // lapsed

	res, err := env.hrmCertSvc.SweepExpiries(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	t.Logf("sweep marked %d expiring, %d expired", res.MarkedExpiring, res.MarkedExpired)

	if got := statusOf(t, env, farFuture); got != "active" {
		t.Errorf("credential 200 days out = %s, want active", got)
	}
	if got := statusOf(t, env, soon); got != "expiring" {
		t.Errorf("credential 10 days out = %s, want expiring", got)
	}
	if got := statusOf(t, env, edgeIn); got != "expiring" {
		t.Errorf("credential exactly 30 days out = %s, want expiring (the window is inclusive)", got)
	}
	// The boundary that matters.
	if got := statusOf(t, env, today); got == "expired" {
		t.Error("a credential expiring TODAY was marked expired — it is still valid today")
	}
	if got := statusOf(t, env, yesterday); got != "expired" {
		t.Errorf("credential that lapsed yesterday = %s, want expired", got)
	}
}

// TestIntegration_Certifications_SweepDoesNotRenotify pins the
// expiry_notified_at guard. Without it the job would re-flag the same
// credential every night for a month, and the reminder becomes noise nobody
// reads.
func TestIntegration_Certifications_SweepDoesNotRenotify(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCertFixture(t, env)
	id := issueWithExpiry(t, env, fx, fx.empID, 10)

	first, err := env.hrmCertSvc.SweepExpiries(ctx)
	if err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if first.MarkedExpiring < 1 {
		t.Fatalf("first sweep marked %d expiring, want at least 1", first.MarkedExpiring)
	}
	if got := statusOf(t, env, id); got != "expiring" {
		t.Fatalf("status = %s, want expiring", got)
	}

	var notifiedAt *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT expiry_notified_at FROM hrm_employee_certifications WHERE id=$1`, id).Scan(&notifiedAt); err != nil {
		t.Fatalf("read notified_at: %v", err)
	}
	if notifiedAt == nil {
		t.Fatal("expiry_notified_at was not stamped")
	}

	// A second run must not touch it again.
	second, err := env.hrmCertSvc.SweepExpiries(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	var reNotified *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT expiry_notified_at FROM hrm_employee_certifications WHERE id=$1`, id).Scan(&reNotified); err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if !reNotified.Equal(*notifiedAt) {
		t.Errorf("the second sweep re-notified: %v → %v", notifiedAt, reNotified)
	}
	_ = second
}

// TestIntegration_Certifications_NeverExpiringIsUntouched — a NULL expiry means
// the credential does not expire, which must stay distinguishable from
// expiring today.
func TestIntegration_Certifications_NeverExpiringIsUntouched(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Holder", nil)

	// No validity_months → no derived expiry.
	cert, err := env.hrmCertSvc.Create(ctx, orgID, ownerID, hrmcertifications.CreateCertificationRequest{
		Name: "Lifetime " + uniqueSlug("c"),
	})
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	ec, err := env.hrmCertSvc.Issue(ctx, orgID, certAdmin(ownerID), hrmcertifications.IssueRequest{
		EmployeeID: empID, CertificationID: cert.ID, IssuedOn: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ec.ExpiresAt != nil {
		t.Fatalf("expected no expiry, got %v", ec.ExpiresAt)
	}

	if _, err := env.hrmCertSvc.SweepExpiries(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := statusOf(t, env, ec.ID); got != "active" {
		t.Errorf("a never-expiring credential became %s", got)
	}
}

// TestIntegration_Certifications_ExpiryDerivedFromValidityMonths pins the
// AddDate arithmetic — months are not a fixed number of hours, and raw
// duration maths drifts.
func TestIntegration_Certifications_ExpiryDerivedFromValidity(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCertFixture(t, env) // validity_months = 12

	ec, err := env.hrmCertSvc.Issue(ctx, fx.orgID, certAdmin(fx.ownerID), hrmcertifications.IssueRequest{
		EmployeeID: fx.empID, CertificationID: fx.cert.ID, IssuedOn: "2030-02-29",
	})
	// 2030 is not a leap year, so this is a deliberately invalid date the
	// parser must reject rather than silently roll to March 1st.
	if err == nil {
		t.Fatalf("expected an invalid date to be rejected, got %v", ec)
	}

	ec, err = env.hrmCertSvc.Issue(ctx, fx.orgID, certAdmin(fx.ownerID), hrmcertifications.IssueRequest{
		EmployeeID: fx.empID, CertificationID: fx.cert.ID, IssuedOn: "2030-03-15",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ec.ExpiresAt == nil {
		t.Fatal("expiry was not derived from validity_months")
	}
	if got := ec.ExpiresAt.Format("2006-01-02"); got != "2031-03-15" {
		t.Errorf("derived expiry = %s, want 2031-03-15", got)
	}

	// Changing the catalogue's validity must NOT move an issued credential.
	newMonths := 1
	if _, err := env.hrmCertSvc.Update(ctx, fx.orgID, fx.cert.ID, hrmcertifications.UpdateCertificationRequest{
		ValidityMonths: &newMonths,
	}); err != nil {
		t.Fatalf("update validity: %v", err)
	}
	var expires time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT expires_at FROM hrm_employee_certifications WHERE id=$1`, ec.ID).Scan(&expires); err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if got := expires.Format("2006-01-02"); got != "2031-03-15" {
		t.Errorf("issued credential's expiry moved to %s after a catalogue change", got)
	}
}

// ============================================================
// Issue, revoke, re-issue
// ============================================================

// TestIntegration_Certifications_RevokeFreesReissue — which is what makes the
// live-credential index partial rather than absolute, and is the renewal path.
func TestIntegration_Certifications_RevokeFreesReissue(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCertFixture(t, env)
	caller := certAdmin(fx.ownerID)

	first, err := env.hrmCertSvc.Issue(ctx, fx.orgID, caller, hrmcertifications.IssueRequest{
		EmployeeID: fx.empID, CertificationID: fx.cert.ID, IssuedOn: "2030-01-01",
	})
	if err != nil {
		t.Fatalf("first issue: %v", err)
	}

	if _, err := env.hrmCertSvc.Issue(ctx, fx.orgID, caller, hrmcertifications.IssueRequest{
		EmployeeID: fx.empID, CertificationID: fx.cert.ID, IssuedOn: "2030-01-02",
	}); !errors.Is(err, hrmcertifications.ErrAlreadyHeld) {
		t.Fatalf("expected ErrAlreadyHeld, got %v", err)
	}

	if _, err := env.hrmCertSvc.Revoke(ctx, fx.orgID, first.ID, caller, hrmcertifications.RevokeRequest{}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := env.hrmCertSvc.Issue(ctx, fx.orgID, caller, hrmcertifications.IssueRequest{
		EmployeeID: fx.empID, CertificationID: fx.cert.ID, IssuedOn: "2030-01-03",
	}); err != nil {
		t.Errorf("a revoked credential should free the employee for re-issue: %v", err)
	}
}

// TestIntegration_Certifications_IssueGrantsTheLinkedSkill is the in-phase
// consumer that justifies building the skills taxonomy in Phase 6 rather than
// deferring the whole thing to Phase 10.
func TestIntegration_Certifications_IssueGrantsTheLinkedSkill(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCertFixture(t, env)

	ec, err := env.hrmCertSvc.Issue(ctx, fx.orgID, certAdmin(fx.ownerID), hrmcertifications.IssueRequest{
		EmployeeID: fx.empID, CertificationID: fx.cert.ID, IssuedOn: "2030-01-01",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	res, err := env.hrmSkillsSvc.ListEmployeeSkills(ctx, fx.orgID, skillAdmin(fx.ownerID),
		hrmskills.EmployeeSkillListFilter{EmployeeID: fx.empID})
	if err != nil {
		t.Fatalf("list employee skills: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("expected the credential to record 1 skill, got %d", res.Total)
	}
	got := res.Skills[0]
	if got.SkillID != fx.skill.ID {
		t.Errorf("recorded skill = %s, want %s", got.SkillID, fx.skill.ID)
	}
	if got.Source != hrmskills.SourceCertification {
		t.Errorf("source = %s, want certification — provenance is the point of the column", got.Source)
	}
	if got.SourceCertificationID == nil || *got.SourceCertificationID != ec.ID {
		t.Errorf("source certification id = %v, want %s", got.SourceCertificationID, ec.ID)
	}
}

// ============================================================
// The widened acknowledgement CHECK
// ============================================================

// TestIntegration_Certifications_CourseCompletionAckAccepted proves migration
// 00094's CHECK widening took, and that it did not turn the constraint into a
// no-op.
func TestIntegration_Certifications_CourseCompletionAckAccepted(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCertFixture(t, env)

	var ackID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_acknowledgements
		    (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, requested_by)
		 VALUES ($1,$2,'course_completion',$3,'Security Basics',$4) RETURNING id`,
		fx.orgID, fx.empID, fx.cert.ID, fx.ownerID).Scan(&ackID); err != nil {
		t.Fatalf("the acknowledgeable_type CHECK rejected 'course_completion': %v", err)
	}

	// 'appraisal' from Phase 5B must still be accepted — the widening added to
	// the list rather than replacing it.
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_acknowledgements
		    (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, requested_by)
		 VALUES ($1,$2,'appraisal',$3,'Prior phase',$4)`,
		fx.orgID, fx.empID, fx.cert.ID, fx.ownerID); err != nil {
		t.Errorf("the widening dropped 'appraisal': %v", err)
	}

	// And nonsense is still refused.
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_acknowledgements
		    (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, requested_by)
		 VALUES ($1,$2,'not_a_real_type',$3,'Nope',$4)`,
		fx.orgID, fx.empID, fx.cert.ID, fx.ownerID); err == nil {
		t.Error("the widened CHECK now accepts anything")
	}
}

// ============================================================
// Skills taxonomy
// ============================================================

// TestIntegration_Skills_DeleteRefusedWhenHeld guards against silent data
// loss: hrm_employee_skills.skill_id is ON DELETE CASCADE, so deleting a
// taxonomy entry would erase every employee's record of holding it.
func TestIntegration_Skills_DeleteRefusedWhenHeld(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCertFixture(t, env)

	if _, err := env.hrmCertSvc.Issue(ctx, fx.orgID, certAdmin(fx.ownerID), hrmcertifications.IssueRequest{
		EmployeeID: fx.empID, CertificationID: fx.cert.ID, IssuedOn: "2030-01-01",
	}); err != nil {
		t.Fatalf("issue: %v", err)
	}

	if err := env.hrmSkillsSvc.DeleteSkill(ctx, fx.orgID, fx.skill.ID); !errors.Is(err, hrmskills.ErrSkillInUse) {
		t.Fatalf("expected ErrSkillInUse, got %v", err)
	}

	// The records are all still there.
	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_employee_skills WHERE skill_id=$1`, fx.skill.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected the employee skill to survive, got %d rows", n)
	}
}

func TestIntegration_Skills_ScopeTiers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	managerEmp := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Manager", nil)
	reportEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Report", &managerEmp)
	strangerEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Stranger", nil)

	sk, err := env.hrmSkillsSvc.CreateSkill(ctx, orgID, ownerID, hrmskills.CreateSkillRequest{
		Name: "Go " + uniqueSlug("s"),
	})
	if err != nil {
		t.Fatalf("create skill: %v", err)
	}
	for _, emp := range []string{managerEmp, reportEmp, strangerEmp} {
		if _, err := env.hrmSkillsSvc.GrantSkill(ctx, orgID, skillAdmin(ownerID), hrmskills.GrantSkillRequest{
			EmployeeID: emp, SkillID: sk.ID,
		}); err != nil {
			t.Fatalf("grant to %s: %v", emp, err)
		}
	}

	cases := []struct {
		tier authz.Scope
		want int
	}{
		{authz.ScopeOwn, 1},
		{authz.ScopeTeam, 2},
		{authz.ScopeAll, 3},
	}
	for _, tc := range cases {
		caller := hrmskills.Caller{UserID: ownerID, Tier: tc.tier}
		res, err := env.hrmSkillsSvc.ListEmployeeSkills(ctx, orgID, caller, hrmskills.EmployeeSkillListFilter{
			SkillID: sk.ID,
		})
		if err != nil {
			t.Fatalf("list at tier %v: %v", tc.tier, err)
		}
		if res.Total != tc.want {
			t.Errorf("tier %v: expected %d, got %d", tc.tier, tc.want, res.Total)
		}
	}
}

func TestIntegration_Certifications_ScopeTiersAndTenancy(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fxA := seedCertFixture(t, env)
	fxB := seedCertFixture(t, env)

	if _, err := env.hrmCertSvc.Get(ctx, fxB.orgID, fxA.cert.ID); !errors.Is(err, hrmcertifications.ErrNotFound) {
		t.Errorf("org B reached org A's certification: %v", err)
	}
	if _, err := env.hrmSkillsSvc.GetSkill(ctx, fxB.orgID, fxA.skill.ID); !errors.Is(err, hrmskills.ErrSkillNotFound) {
		t.Errorf("org B reached org A's skill: %v", err)
	}

	// Scope tiers over the credentials list.
	// fxA.empID is already linked to ownerID, so it IS the caller's own record.
	// Linking a second employee to the same user would make scope.Predicate's
	// ScopeOwn subquery return two rows — see the note in the Phase 6 docs.
	reportEmp := seedEmployee(t, env, fxA.orgID, fxA.statusID, fxA.ownerID, "", "Rep", &fxA.empID)
	for i, emp := range []string{fxA.empID, reportEmp} {
		if _, err := env.hrmCertSvc.Issue(ctx, fxA.orgID, certAdmin(fxA.ownerID), hrmcertifications.IssueRequest{
			EmployeeID: emp, CertificationID: fxA.cert.ID,
			IssuedOn: fmt.Sprintf("2030-01-%02d", i+1),
		}); err != nil {
			t.Fatalf("issue to %s: %v", emp, err)
		}
	}

	own := hrmcertifications.Caller{UserID: fxA.ownerID, Tier: authz.ScopeOwn}
	res, err := env.hrmCertSvc.ListEmployeeCertifications(ctx, fxA.orgID, own,
		hrmcertifications.EmployeeCertificationListFilter{CertificationID: fxA.cert.ID})
	if err != nil {
		t.Fatalf("list at view_own: %v", err)
	}
	if res.Total != 1 {
		t.Errorf("view_own returned %d credentials, want 1", res.Total)
	}
}
