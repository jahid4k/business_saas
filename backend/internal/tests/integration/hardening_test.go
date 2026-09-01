// backend/internal/tests/integration/hardening_test.go
// The hardening pass: defects carried in the known-open list across Phases
// 7–9 and cleared together.
//
// Each of these guards a bug that was invisible in normal use — a new org that
// silently could not hire, and a note that silently failed to save.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/auth"
	hrmemployees "github.com/mridha/businesssaas/internal/hrm/employees"
	"github.com/mridha/businesssaas/internal/organizations"
)

// ============================================================
// A fresh organization can hire
// ============================================================

// TestIntegration_Hardening_NewOrgCanCreateAnEmployeeImmediately is the
// regression guard for a bug that made every API-created organization unable
// to hire its first employee.
//
// organizations.Create never seeded hrm_employee_statuses — only migration
// 00053's one-time backfill did — so POST /hrm/employees failed on a NOT NULL
// status_id. It has been in the known-open list since r18, and nearly every
// smoke run in this project worked around it with a manual INSERT.
//
// The test deliberately creates the org the way a real customer does and then
// hires, with no status setup in between.
func TestIntegration_Hardening_NewOrgCanCreateAnEmployeeImmediately(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner, err := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email: uniqueEmail("fresh-org-owner"), Password: "FreshOrgPass123!",
	})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, owner.ID) })

	org, err := env.orgSvc.Create(ctx, owner.ID, organizations.CreateBusinessRequest{
		Name: "Fresh Org", Slug: uniqueSlug("fresh-org"),
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	t.Cleanup(func() {
		_, _ = env.db.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, org.ID)
	})

	// No status setup at all — this is the whole point.
	emp, err := env.hrmEmpSvc.Create(ctx, org.ID, owner.ID, hrmemployees.CreateEmployeeRequest{
		FirstName: "First", HireDate: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("a brand-new organization could not create its first employee: %v", err)
	}
	if emp.ID == "" {
		t.Fatal("employee created with no id")
	}
}

// TestIntegration_Hardening_NewOrgGetsTheSameStatusesTheMigrationSeeds — an
// API-created org and a migration-seeded one must be indistinguishable, because
// HRM filters on CATEGORY and payroll's eligible-employee rule depends on
// those categories being right.
func TestIntegration_Hardening_NewOrgGetsTheSameStatusesTheMigrationSeeds(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner, err := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email: uniqueEmail("status-shape-owner"), Password: "StatusPass123!",
	})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, owner.ID) })
	org, err := env.orgSvc.Create(ctx, owner.ID, organizations.CreateBusinessRequest{
		Name: "Status Shape", Slug: uniqueSlug("status-shape"),
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	t.Cleanup(func() {
		_, _ = env.db.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, org.ID)
	})

	rows, err := env.db.Query(ctx,
		`SELECT name, category FROM hrm_employee_statuses WHERE org_id=$1 ORDER BY name`, org.ID)
	if err != nil {
		t.Fatalf("read statuses: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var name, category string
		if err := rows.Scan(&name, &category); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = category
	}

	want := map[string]string{
		"Active": "active", "Inactive": "inactive", "On Leave": "on_leave",
		// Two names, one category, deliberately — they are different words for
		// one lifecycle state and HRM reads the category.
		"Resigned": "terminated", "Terminated": "terminated",
	}
	if len(got) != len(want) {
		t.Fatalf("new org has %d statuses %v, want %d", len(got), got, len(want))
	}
	for name, category := range want {
		if got[name] != category {
			t.Errorf("status %q has category %q, want %q — payroll's eligible-employee "+
				"filter reads the category, not the name", name, got[name], category)
		}
	}
}

// TestIntegration_Hardening_BackfillRepairedExistingOrgs — the Go-side seeding
// only helps orgs created from now on. Migration 00120's backfill is what
// repairs the ones already created, which is all of them.
func TestIntegration_Hardening_BackfillRepairedExistingOrgs(t *testing.T) {
	env := newTestEnv(t)
	var orphaned int
	if err := env.db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM organizations o
		  WHERE NOT EXISTS (SELECT 1 FROM hrm_employee_statuses s WHERE s.org_id = o.id)`,
	).Scan(&orphaned); err != nil {
		t.Fatalf("count orgs without statuses: %v", err)
	}
	if orphaned != 0 {
		t.Errorf("%d organization(s) still have no employee statuses — those cannot hire", orphaned)
	}
}

// ============================================================
// System-generated notes
// ============================================================

// TestIntegration_Hardening_SystemGeneratedNoteIsRecorded closes 8D's sibling
// defect.
//
// 00112 made crm_leads.created_by nullable because capture paths have no
// acting user. The same empty actor reached engagement.CreateNote one call
// deeper, platform_notes.created_by was NOT NULL, and leads.CreateLead
// discarded the error with `_, _ =` — so a repeat inbound email from a known
// sender silently failed to record its duplicate-capture note.
//
// Two identical inbound emails is exactly how a real duplicate arrives.
func TestIntegration_Hardening_SystemGeneratedNoteIsRecorded(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, _ := seedScopeTestOrg(t, env)
	addr := seedInboundAddress(t, env, orgID, "sales", "")
	sender := "Repeat Sender <repeat@prospect.example>"

	if err := env.emailSvc.ProcessInboundWebhook(ctx,
		inboundPayload(addr, sender, "First enquiry", "Interested in pricing.")); err != nil {
		t.Fatalf("first webhook: %v", err)
	}
	if got := countLeads(t, env, orgID); got != 1 {
		t.Fatalf("first email produced %d leads, want 1; log says: %s", got, lastLogError(t, env, addr))
	}

	// The same sender again — this takes the duplicate-capture path, which is
	// where the note is written with no acting user.
	if err := env.emailSvc.ProcessInboundWebhook(ctx,
		inboundPayload(addr, sender, "Following up", "Any update?")); err != nil {
		t.Fatalf("second webhook: %v", err)
	}
	if got := countLeads(t, env, orgID); got != 1 {
		t.Errorf("the duplicate created %d leads, want still 1", got)
	}

	var notes int
	var createdBy *string
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_notes WHERE org_id=$1 AND related_type='crm.lead'`,
		orgID).Scan(&notes); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if notes != 1 {
		t.Fatalf("%d duplicate-capture notes recorded, want 1 — a system-generated note "+
			"is still failing silently", notes)
	}
	if err := env.db.QueryRow(ctx,
		`SELECT created_by::text FROM platform_notes WHERE org_id=$1 LIMIT 1`,
		orgID).Scan(&createdBy); err != nil {
		t.Fatalf("read note author: %v", err)
	}
	if createdBy != nil {
		t.Errorf("created_by = %q for a system-generated note, want NULL — attributing it "+
			"to a person would name someone who did not write it", *createdBy)
	}
}
