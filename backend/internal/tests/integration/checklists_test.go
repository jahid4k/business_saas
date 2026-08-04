// backend/internal/tests/integration/checklists_test.go
// Proves platform/checklists + hrm/onboarding against a real Postgres —
// specifically the properties a stub repo structurally cannot catch:
// constraints actually firing, transaction atomicity, ON DELETE SET NULL
// behavior against the deliberately-asymmetric CHECK constraints (see
// migration 00076's header), DATE round-tripping through pgx, and the
// employees.Create -> onboarding auto-hook wiring against a real employees
// repository. Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/auth"
	hrmemployees "github.com/mridha/businesssaas/internal/hrm/employees"
	"github.com/mridha/businesssaas/internal/platform/checklists"
)

// pgErrCode extracts the SQLSTATE from a (possibly wrapped) error, or ""
// if err is nil or not a *pgconn.PgError anywhere in its chain.
func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// ============================================================
// Constraints actually fire
// ============================================================

func TestIntegration_Checklists_ConstraintsFire(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	var tpl1ID string
	err := env.db.QueryRow(ctx,
		`INSERT INTO platform_checklist_templates (org_id, name, checklist_type, is_default, is_active, created_by)
		 VALUES ($1, 'T1', 'onboarding', TRUE, TRUE, $2) RETURNING id`,
		orgID, ownerID,
	).Scan(&tpl1ID)
	if err != nil {
		t.Fatalf("insert first default template: %v", err)
	}

	_, err = env.db.Exec(ctx,
		`INSERT INTO platform_checklist_templates (org_id, name, checklist_type, is_default, is_active, created_by)
		 VALUES ($1, 'T2', 'onboarding', TRUE, TRUE, $2)`,
		orgID, ownerID,
	)
	if code := pgErrCode(err); code != "23505" {
		t.Errorf("expected 23505 (uq_pchk_tpl_default) inserting a second default, got code=%q err=%v", code, err)
	}

	_, err = env.db.Exec(ctx,
		`INSERT INTO platform_checklist_template_items (template_id, title, owner_type) VALUES ($1, 'Bad item', 'role')`,
		tpl1ID,
	)
	if code := pgErrCode(err); code != "23514" {
		t.Errorf("expected 23514 (chk_pchk_tpl_item_role) for owner_type='role' with NULL owner_role, got code=%q err=%v", code, err)
	}

	var instItemID string
	err = env.db.QueryRow(ctx,
		`WITH inst AS (
			INSERT INTO platform_checklist_instances
			    (org_id, template_id, template_name, checklist_type, subject_type, subject_id, subject_label, anchor_date, status, created_by)
			VALUES ($1, $2, 'T1', 'onboarding', 'employee', gen_random_uuid(), 'Test Subject', CURRENT_DATE, 'in_progress', $3)
			RETURNING id
		)
		INSERT INTO platform_checklist_instance_items (instance_id, title, owner_type, status)
		SELECT id, 'Item', 'subject', 'pending' FROM inst
		RETURNING id`,
		orgID, tpl1ID, ownerID,
	).Scan(&instItemID)
	if err != nil {
		t.Fatalf("seed instance + item: %v", err)
	}

	_, err = env.db.Exec(ctx, `UPDATE platform_checklist_instance_items SET status = 'skipped' WHERE id = $1`, instItemID)
	if code := pgErrCode(err); code != "23514" {
		t.Errorf("expected 23514 (chk_pchk_item_skipped) for status='skipped' with NULL skip_reason, got code=%q err=%v", code, err)
	}
}

// ============================================================
// InsertInstanceWithItems atomicity
// ============================================================

func TestIntegration_Checklists_InsertInstanceWithItems_AtomicOnFailure(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, _ := seedScopeTestOrg(t, env)

	repo := checklists.NewRepository(env.db)

	bogusUser := "00000000-0000-0000-0000-000000000000" // syntactically valid UUID, no matching users row -> FK violation
	inst := &checklists.Instance{
		OrgID: orgID, TemplateName: "Atomicity Test", ChecklistType: checklists.ChecklistTypeOnboarding,
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "11111111-1111-1111-1111-111111111111",
		SubjectLabel: "X", AnchorDate: time.Now(), Status: checklists.InstanceStatusInProgress, CreatedBy: bogusUser,
	}
	items := []*checklists.InstanceItem{
		{Title: "Item 1", OwnerType: checklists.OwnerTypeRole, OwnerRole: strp("hr"), Status: checklists.ItemStatusPending},
	}

	err := repo.InsertInstanceWithItems(ctx, inst, items)
	if err == nil {
		t.Fatal("expected InsertInstanceWithItems to fail (created_by references a nonexistent user)")
	}

	var count int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_checklist_instances WHERE org_id = $1`, orgID).Scan(&count); err != nil {
		t.Fatalf("count instances: %v", err)
	}
	if count != 0 {
		t.Errorf("expected zero surviving instance rows after a failed insert, got %d — the transaction did not roll back atomically", count)
	}
}

// ============================================================
// ON DELETE behaviours
// ============================================================

func TestIntegration_Checklists_DeleteUser_NullsReferencesWithoutErroring(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	userEmail := uniqueEmail("checklist-delete-target")
	u, err := env.authSvc.Signup(ctx, auth.SignupRequest{Email: userEmail, Password: "DeleteTargetPass123!"})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}

	tpl, err := env.checklistsSvc.CreateTemplate(ctx, orgID, ownerID, checklists.CreateTemplateRequest{
		Name: "Delete Target Test", ChecklistType: checklists.ChecklistTypeOnboarding,
		Items: []checklists.CreateTemplateItemRequest{
			{Title: "Subject item", OwnerType: checklists.OwnerTypeSubject},
			{Title: "Specific user item", OwnerType: checklists.OwnerTypeSpecificUser, OwnerUserID: &u.ID},
		},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	result, err := env.checklistsSvc.Instantiate(ctx, orgID, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "22222222-2222-2222-2222-222222222222",
		SubjectLabel: "X", SubjectUserID: &u.ID, AnchorDate: time.Now(), CreatedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	var subjectItemID, specificUserItemID string
	for _, it := range result.Items {
		if it.Title == "Subject item" {
			subjectItemID = it.ID
		} else {
			specificUserItemID = it.ID
		}
	}

	if _, err := env.checklistsSvc.CompleteItem(ctx, orgID, subjectItemID, u.ID, checklists.CompleteItemRequest{}); err != nil {
		t.Fatalf("complete item: %v", err)
	}

	// The proof: deleting the user must not error, despite it being
	// referenced by assignee_user_id, completed_by (instance items) and
	// owner_user_id (template item) — the CHECK/SET-NULL asymmetry
	// documented in migration 00076's header is safe.
	if _, err := env.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("SECURITY/SCHEMA: deleting a user referenced by checklist rows must not error, got: %v", err)
	}

	var assignee, completedBy *string
	if err := env.db.QueryRow(ctx,
		`SELECT assignee_user_id::text, completed_by::text FROM platform_checklist_instance_items WHERE id = $1`, subjectItemID,
	).Scan(&assignee, &completedBy); err != nil {
		t.Fatalf("query subject item: %v", err)
	}
	if assignee != nil || completedBy != nil {
		t.Errorf("expected assignee_user_id and completed_by to be nulled by ON DELETE SET NULL, got assignee=%v completed_by=%v", assignee, completedBy)
	}

	var specificAssignee *string
	if err := env.db.QueryRow(ctx,
		`SELECT assignee_user_id::text FROM platform_checklist_instance_items WHERE id = $1`, specificUserItemID,
	).Scan(&specificAssignee); err != nil {
		t.Fatalf("query specific_user item: %v", err)
	}
	if specificAssignee != nil {
		t.Errorf("expected specific_user item's assignee_user_id to be nulled, got %v", *specificAssignee)
	}

	var tplOwnerUserID *string
	if err := env.db.QueryRow(ctx,
		`SELECT owner_user_id::text FROM platform_checklist_template_items WHERE template_id = $1 AND title = 'Specific user item'`, tpl.ID,
	).Scan(&tplOwnerUserID); err != nil {
		t.Fatalf("query template item: %v", err)
	}
	if tplOwnerUserID != nil {
		t.Errorf("expected the template item's owner_user_id to be nulled too, got %v", *tplOwnerUserID)
	}
}

// ============================================================
// DATE round-trip through pgx
// ============================================================

func TestIntegration_Checklists_DueDate_RoundTripsThroughPostgres(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	tpl, err := env.checklistsSvc.CreateTemplate(ctx, orgID, ownerID, checklists.CreateTemplateRequest{
		Name: "Date Round Trip", ChecklistType: checklists.ChecklistTypeOnboarding,
		Items: []checklists.CreateTemplateItemRequest{
			{Title: "Month rollover", OwnerType: checklists.OwnerTypeSubject, DueOffsetDays: 1},
			{Title: "Year rollover", OwnerType: checklists.OwnerTypeSubject, DueOffsetDays: -5},
		},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	anchor := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	result, err := env.checklistsSvc.Instantiate(ctx, orgID, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "33333333-3333-3333-3333-333333333333",
		SubjectLabel: "X", AnchorDate: anchor, CreatedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	for _, it := range result.Items {
		var dbDate time.Time
		if err := env.db.QueryRow(ctx, `SELECT due_date FROM platform_checklist_instance_items WHERE id = $1`, it.ID).Scan(&dbDate); err != nil {
			t.Fatalf("query due_date for %q: %v", it.Title, err)
		}
		var want time.Time
		switch it.Title {
		case "Month rollover":
			want = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		case "Year rollover":
			want = time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC)
		}
		if !dbDate.Equal(want) {
			t.Errorf("%s: expected due_date %v round-tripped through Postgres, got %v", it.Title, want, dbDate)
		}
	}
}

// ============================================================
// End-to-end auto-hook (employees.Create -> onboarding -> checklists)
// ============================================================

func TestIntegration_Onboarding_AutoHook_EndToEnd(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	tpl, err := env.checklistsSvc.CreateTemplate(ctx, orgID, ownerID, checklists.CreateTemplateRequest{
		Name: "Default Onboarding", ChecklistType: checklists.ChecklistTypeOnboarding, IsDefault: true,
		Items: []checklists.CreateTemplateItemRequest{
			{Title: "Welcome email", OwnerType: checklists.OwnerTypeSubject, DueOffsetDays: -3},
		},
	})
	if err != nil {
		t.Fatalf("create default template: %v", err)
	}

	emp1, err := env.hrmEmpSvc.Create(ctx, orgID, ownerID, hrmemployees.CreateEmployeeRequest{
		FirstName: "Auto", HireDate: "2026-03-01",
	})
	if err != nil {
		t.Fatalf("create employee (with default template configured): %v", err)
	}

	subjType := checklists.SubjectTypeEmployee
	list, err := env.checklistsSvc.ListInstances(ctx, orgID, checklists.InstanceFilter{SubjectType: &subjType, SubjectID: &emp1.ID})
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(list.Instances) != 1 {
		t.Fatalf("expected exactly one onboarding instance for the new employee, got %d", len(list.Instances))
	}
	if list.Instances[0].TemplateName != "Default Onboarding" {
		t.Errorf("expected the instance to snapshot the default template's name, got %q", list.Instances[0].TemplateName)
	}

	// Unset the default — the next employee create must succeed with no
	// checklist produced, not an error.
	isDefaultFalse := false
	if _, err := env.checklistsSvc.UpdateTemplate(ctx, orgID, tpl.ID, checklists.UpdateTemplateRequest{IsDefault: &isDefaultFalse}); err != nil {
		t.Fatalf("unset default: %v", err)
	}

	emp2, err := env.hrmEmpSvc.Create(ctx, orgID, ownerID, hrmemployees.CreateEmployeeRequest{
		FirstName: "NoTemplate", HireDate: "2026-03-02",
	})
	if err != nil {
		t.Fatalf("create employee (no default template configured) should still succeed, got: %v", err)
	}

	list2, err := env.checklistsSvc.ListInstances(ctx, orgID, checklists.InstanceFilter{SubjectType: &subjType, SubjectID: &emp2.ID})
	if err != nil {
		t.Fatalf("list instances for second employee: %v", err)
	}
	if len(list2.Instances) != 0 {
		t.Errorf("expected zero checklist instances once the default is unset, got %d", len(list2.Instances))
	}
}

// TestIntegration_Onboarding_EmployeeCreate_SurvivesPoisonedHook proves the
// property against a REAL employees repository/Postgres transaction, not
// just the stub used by the unit test of the same name — that Create's own
// commit is unaffected by whatever the checklist hook does.
func TestIntegration_Onboarding_EmployeeCreate_SurvivesPoisonedHook(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	poisonedRepo := hrmemployees.NewRepository(env.db)
	poisonedAudit := audit.NewService(audit.NewRepository(env.db))
	poisonedSvc := hrmemployees.NewService(poisonedRepo, poisonedAudit, poisonedHook{})

	emp, err := poisonedSvc.Create(ctx, orgID, ownerID, hrmemployees.CreateEmployeeRequest{
		FirstName: "Poisoned", HireDate: "2026-03-01",
	})
	if err != nil {
		t.Fatalf("expected Create to succeed even though the checklist hook always errors, got %v", err)
	}

	var count int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_employees WHERE id = $1`, emp.ID).Scan(&count); err != nil {
		t.Fatalf("verify employee row: %v", err)
	}
	if count != 1 {
		t.Errorf("expected the employee row to be committed despite the poisoned hook, found count=%d", count)
	}
}

type poisonedHook struct{}

func (poisonedHook) OnEmployeeCreated(_ context.Context, _, _, _ string) error {
	return context.DeadlineExceeded // any non-nil error stands in for "the checklist engine is broken"
}

// ============================================================
// GetProgressBatch
// ============================================================

func TestIntegration_Checklists_GetProgressBatch(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	tpl, err := env.checklistsSvc.CreateTemplate(ctx, orgID, ownerID, checklists.CreateTemplateRequest{
		Name: "Batch Progress Test", ChecklistType: checklists.ChecklistTypeOnboarding,
		Items: []checklists.CreateTemplateItemRequest{
			{Title: "A", OwnerType: checklists.OwnerTypeSubject},
			{Title: "B", OwnerType: checklists.OwnerTypeSubject},
		},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	userEmail := uniqueEmail("progress-batch")
	u, err := env.authSvc.Signup(ctx, auth.SignupRequest{Email: userEmail, Password: "ProgressPass123!"})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, u.ID) })

	result1, err := env.checklistsSvc.Instantiate(ctx, orgID, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "44444444-4444-4444-4444-444444444444",
		SubjectLabel: "X", SubjectUserID: &u.ID, AnchorDate: time.Now(), CreatedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("instantiate 1: %v", err)
	}
	result2, err := env.checklistsSvc.Instantiate(ctx, orgID, tpl.ID, checklists.SubjectContext{
		SubjectType: checklists.SubjectTypeEmployee, SubjectID: "55555555-5555-5555-5555-555555555555",
		SubjectLabel: "Y", SubjectUserID: &u.ID, AnchorDate: time.Now(), CreatedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("instantiate 2: %v", err)
	}

	// Instance 1: complete both items (100%). Instance 2: leave untouched (0%).
	for _, it := range result1.Items {
		if _, err := env.checklistsSvc.CompleteItem(ctx, orgID, it.ID, u.ID, checklists.CompleteItemRequest{}); err != nil {
			t.Fatalf("complete item: %v", err)
		}
	}

	repo := checklists.NewRepository(env.db)
	progress, err := repo.GetProgressBatch(ctx, orgID, []string{result1.Instance.ID, result2.Instance.ID})
	if err != nil {
		t.Fatalf("GetProgressBatch: %v", err)
	}
	if len(progress) != 2 {
		t.Fatalf("expected progress for both instances, got %d entries", len(progress))
	}
	if progress[result1.Instance.ID].PercentDone != 100 {
		t.Errorf("expected instance 1 at 100%%, got %d", progress[result1.Instance.ID].PercentDone)
	}
	if progress[result2.Instance.ID].PercentDone != 0 {
		t.Errorf("expected instance 2 at 0%%, got %d", progress[result2.Instance.ID].PercentDone)
	}
}

func strp(s string) *string { return &s }
